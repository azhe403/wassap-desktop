//go:build windows

package webview2

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"

	"wassap-desktop/internal/browser"
)

// The bindings below are kept deliberately small: user32 and kernel32 symbols
// that Go's standard library does not export. Nothing here is WebView2 specific;
// the WebView2 work itself goes through wailsapp/go-webview2.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")

	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetWindowTextW   = user32.NewProc("SetWindowTextW")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procUnregisterClassW = user32.NewProc("UnregisterClassW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")
	procCallWindowProcW  = user32.NewProc("CallWindowProcW")
)

const (
	// wmInvokeCallback carries a callback queued by Session.call. PostMessage and
	// WM_USER are both process-local, so the value only has to avoid the
	// messages user32 already defines.
	wmInvokeCallback = 0x8000
	wmClose          = 0x0010
	wmNCDestroy      = 0x0082

	wsOverlappedWindow = 0x00CF0000
	wsClipSiblings     = 0x04000000
	wsClipChildren     = 0x02000000
	swShow             = 5
	colorWindow        = 5
	idcArrow           = 32512
	csOwndc            = 0x0020
	csHredraw          = 0x0002
)

// ErrNilCallbackArgs is returned by the navigation helpers for a nil pointer.
var ErrNilCallbackArgs = errors.New("webview2: nil event args")

// Rect is the outer window rectangle in pixels.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

// sessions maps a window handle to its session. A sync.Map is used because the
// window procedure runs on arbitrary threads and must never block on a mutex
// held by code that is waiting for a callback to complete.
var sessions sync.Map // uintptr -> *Session

func lockOSThread()   { runtime.LockOSThread() }
func unlockOSThread() { runtime.UnlockOSThread() }

func initCOM() error {
	// COINIT_APARTMENTTHREADED = 0x2
	hr, _, _ := procCoInitializeEx.Call(0, 2)
	if hr == 0 || hr == 0x00000001 { // S_OK or S_FALSE
		return nil
	}
	return fmt.Errorf("CoInitializeEx returned 0x%08x", uint32(hr))
}

func uninitCOM() {
	procCoUninitialize.Call()
}

func (s *Session) createWindow() (uintptr, error) {
	className, err := syscall.UTF16PtrFromString("WassapWebView2Session_" + string(s.profile))
	if err != nil {
		return 0, err
	}
	title, err := syscall.UTF16PtrFromString(s.host.Title + " - " + string(s.profile))
	if err != nil {
		return 0, err
	}

	instance, _, _ := procGetModuleHandleW.Call(0)
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	wc := wndClassEx{
		size:       uint32(unsafe.Sizeof(wndClassEx{})),
		style:      csOwndc | csHredraw,
		wndProc:    syscall.NewCallback(windowProc),
		instance:   instance,
		cursor:     cursor,
		background: colorWindow + 1,
		className:  className,
	}
	if atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return 0, fmt.Errorf("webview2: cannot register window class for %s: %v", s.profile, regErr)
	}
	s.className = className
	s.instance = instance

	hwnd, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow|wsClipSiblings|wsClipChildren,
		uintptr(s.host.Bounds.X), uintptr(s.host.Bounds.Y),
		uintptr(s.host.Bounds.Width), uintptr(s.host.Bounds.Height),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), instance)
		s.className = nil
		return 0, fmt.Errorf("webview2: cannot create window for %s: %v", s.profile, createErr)
	}
	s.hwnd = hwnd
	sessions.Store(hwnd, s)
	// Record the thread that now owns the window. Session.call compares against
	// it so that a call made from this thread runs inline instead of deadlocking
	// on a message the thread itself would have to dispatch.
	s.threadID = currentThreadID()
	return hwnd, nil
}

func (s *Session) destroyClassName() {
	if s.className == nil {
		return
	}
	procUnregisterClassW.Call(uintptr(unsafe.Pointer(s.className)), s.instance)
	s.className = nil
}

// setUpWebView creates the WebView2 environment, controller and web view for the
// session window. It runs on the session's UI thread.
func (s *Session) setUpWebView() error {
	version, err := CheckRuntime()
	if err != nil {
		return err
	}
	s.runtimeVersion = version

	c := edge.NewChromium()
	// Without this, every WebView2 failure calls os.Exit(1): go-webview2's
	// default handler prints "[WebView2 Error]" and exits the whole process.
	// The proof of concept must survive a failed profile.
	failure := make(chan error, 8)
	report := func(err error) {
		if err == nil {
			return
		}
		select {
		case failure <- err:
		default:
		}
	}
	c.SetErrorCallback(report)
	c.DataPath = s.path
	c.ProcessFailedCallback = func(_ *edge.ICoreWebView2, args *edge.ICoreWebView2ProcessFailedEventArgs) {
		if args == nil {
			report(fmt.Errorf("webview2: unknown process failure for %s", s.profile))
			return
		}
		kind, err := args.GetProcessFailedKind()
		if err != nil {
			report(fmt.Errorf("webview2: a process for %s failed: %w", s.profile, err))
			return
		}
		err = processFailureError(s.profile, kind)
		report(err)
		s.emit(EventProcessFailed, err)
	}
	c.NavigationCompletedCallback = func(_ *edge.ICoreWebView2, args *edge.ICoreWebView2NavigationCompletedEventArgs) {
		// Navigation failures are reported by the page itself; only the engine
		// dying is surfaced, through the process-failed callback.
		if args == nil {
			return
		}
		if ok, err := IsSuccess(args); err == nil && !ok {
			status, _ := WebErrorStatus(args)
			s.emit(EventProcessFailed, fmt.Errorf("webview2: %s did not load the start page (web error %d)", s.profile, status))
		}
	}

	if !c.Embed(s.hwnd) {
		return fmt.Errorf("webview2: cannot embed a web view for %s", s.profile)
	}
	select {
	case err := <-failure:
		return err
	default:
	}
	if c.GetController() == nil {
		return fmt.Errorf("webview2: the web view for %s did not initialise", s.profile)
	}

	s.chromium = c
	c.Resize()
	if s.host.StartURL != "" {
		c.Navigate(s.host.StartURL)
	}
	return nil
}

// processFailureError turns a WebView2 process-failed kind into an error.
// Only the browser process exiting is fatal for the session; WebView2 recreates
// failed renderers itself.
func processFailureError(id browser.ProfileID, kind edge.COREWEBVIEW2_PROCESS_FAILED_KIND) error {
	if kind == edge.COREWEBVIEW2_PROCESS_FAILED_KIND_BROWSER_PROCESS_EXITED {
		return fmt.Errorf("webview2: the browser process for %s exited", id)
	}
	return nil
}

func windowProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmInvokeCallback:
		if s, ok := sessions.Load(hwnd); ok {
			s.(*Session).serveCallback()
		}
		return 0
	case wmClose:
		destroyWindow(hwnd)
		return 0
	case wmNCDestroy:
		sessions.Delete(hwnd)
		postQuitMessage(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func (s *Session) serveCallback() {
	req := s.callbacks.Swap(nil)
	if req == nil {
		return
	}
	var err error
	if req.fn != nil {
		err = req.fn()
	}
	req.result <- err
}

// postCallbackToCurrentThread is unused: Session.call performs the thread check
// directly.

func currentThreadID() uint32 {
	id, _, _ := procGetCurrentThread.Call()
	return uint32(id)
}

func destroyWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procDestroyWindow.Call(hwnd)
}

func showWindow(hwnd uintptr) error {
	if hwnd == 0 {
		return browser.ErrSessionEnded
	}
	procShowWindow.Call(hwnd, swShow)
	return nil
}

func setWindowTitle(hwnd uintptr, title string) error {
	p, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(p)))
	return nil
}

func setForeground(hwnd uintptr) error {
	if hwnd == 0 {
		return browser.ErrSessionEnded
	}
	procSetForeground.Call(hwnd)
	return nil
}

func postMessage(hwnd uintptr, message uint32, wParam, lParam uintptr) bool {
	ret, _, _ := procPostMessageW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret != 0
}

func postQuitMessage(code int) {
	procPostQuitMessage.Call(uintptr(code))
}

func getMessage() (*msg, bool) {
	m := &msg{}
	ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(m)), 0, 0, 0)
	if int32(ret) <= 0 {
		return nil, false
	}
	return m, true
}

func translateMessage(m *msg) {
	procTranslateMessage.Call(uintptr(unsafe.Pointer(m)))
}

func dispatchMessage(m *msg) {
	procDispatchMessageW.Call(uintptr(unsafe.Pointer(m)))
}
