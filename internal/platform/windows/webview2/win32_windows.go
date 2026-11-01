//go:build windows

package webview2

import (
	"errors"
	"fmt"
	"log"
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
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
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
	procGetClientRect    = user32.NewProc("GetClientRect")
	procMoveWindow       = user32.NewProc("MoveWindow")
	procEnumChildWindows = user32.NewProc("EnumChildWindows")
	procIsWindowVisible_ = user32.NewProc("IsWindowVisible")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongW")
	procGetWindowLongPtr = user32.NewProc("GetWindowLongW")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
	procSetParent        = user32.NewProc("SetParent")
	procSetWindowPos     = user32.NewProc("SetWindowPos")

	comctl32                 = syscall.NewLazyDLL("comctl32.dll")
	procSetWindowSubclass    = comctl32.NewProc("SetWindowSubclass")
	procDefSubclassProc      = comctl32.NewProc("DefSubclassProc")
	procRemoveWindowSubclass = comctl32.NewProc("RemoveWindowSubclass")
)

const (
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpNoCopyBits = 0x0100
)

const (
	// wmInvokeCallback carries a callback queued by Session.call. PostMessage and
	// WM_USER are both process-local, so the value only has to avoid the
	// messages user32 already defines.
	wmInvokeCallback = 0x8000
	wmSize           = 0x0005
	wmSetFocus       = 0x0007
	wmClose          = 0x0010
	wmNCDestroy      = 0x0082

	wsOverlappedWindow = 0x00CF0000
	wsPopup            = 0x80000000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsClipSiblings     = 0x04000000
	wsClipChildren     = 0x02000000
	swShow             = 5
	swHide             = 0
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
	// Dark WhatsApp background #111b21: COLORREF is 0x00BBGGRR -> 0x00211B11
	darkBrush, _, _ := procCreateSolidBrush.Call(0x00211B11)

	wc := wndClassEx{
		size:       uint32(unsafe.Sizeof(wndClassEx{})),
		style:      csOwndc | csHredraw,
		wndProc:    syscall.NewCallback(windowProc),
		instance:   instance,
		cursor:     cursor,
		background: darkBrush,
		className:  className,
	}
	if atom, _, regErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return 0, fmt.Errorf("webview2: cannot register window class for %s: %v", s.profile, regErr)
	}
	s.className = className
	s.instance = instance

	var style uintptr = wsOverlappedWindow | wsClipSiblings | wsClipChildren
	var parentHwnd uintptr = 0
	x := s.host.Bounds.X
	y := s.host.Bounds.Y
	w := s.host.Bounds.Width
	h := s.host.Bounds.Height

	if s.host.ParentHWND != 0 {
		// Use WS_POPUP owned by ParentHWND so the window has its own clean Win32
		// message pump and COM apartment without cross-thread WS_CHILD rendering bugs.
		// It has no borders, no titlebar, no taskbar button, and docks precisely over
		// the main window's content region.
		style = wsPopup | wsVisible | wsClipSiblings | wsClipChildren
		parentHwnd = s.host.ParentHWND

		var parentScreenRect struct {
			Left, Top, Right, Bottom int32
		}
		procGetWindowRect.Call(parentHwnd, uintptr(unsafe.Pointer(&parentScreenRect)))

		var clientRect struct {
			Left, Top, Right, Bottom int32
		}
		procGetClientRect.Call(parentHwnd, uintptr(unsafe.Pointer(&clientRect)))

		// Calculate client origin in screen coordinates (accounting for titlebar and borders)
		borderX := int(parentScreenRect.Right-parentScreenRect.Left-clientRect.Right) / 2
		titleY := int(parentScreenRect.Bottom-parentScreenRect.Top-clientRect.Bottom) - borderX

		x = int(parentScreenRect.Left) + borderX + s.host.SidebarWidth
		y = int(parentScreenRect.Top) + titleY
		w = int(clientRect.Right) - s.host.SidebarWidth
		h = int(clientRect.Bottom)
		if w <= 0 {
			// The parent window has not been laid out yet; start with a sane size.
			w = 1000
		}
		if h <= 0 {
			h = 700
		}
		log.Printf("webview2: popup window for %s -> owner=%d screenPos=(%d,%d) size=%dx%d",
			s.profile, parentHwnd, x, y, w, h)
	}

	hwnd, _, createErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style,
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		parentHwnd, 0, instance, 0,
	)
	if hwnd != 0 {
		vis, _, _ := procIsWindowVisible_.Call(hwnd)
		log.Printf("webview2: created hwnd=%d for %s visible=%v style=0x%x", hwnd, s.profile, vis != 0, style)
	}
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
	// Set default background to WhatsApp dark #111b21 (alpha=255, R=17, G=27, B=33)
	// so the WebView2 compositor buffer never flashes white during resize or navigation.
	_ = SetControllerBackgroundColor(c.GetController(), 0xFF, 17, 27, 33)

	_ = c.Show()
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
	case 0x0014: // WM_ERASEBKGND: suppress default erase to eliminate white flicker on resize
		return 1
	case wmInvokeCallback:
		if s, ok := sessions.Load(hwnd); ok {
			s.(*Session).serveCallback()
		}
		return 0
	case wmSize:
		if s, ok := sessions.Load(hwnd); ok {
			if sess, ok := s.(*Session); ok && sess.chromium != nil {
				sess.chromium.Resize()
			}
		}
		return 0
	case wmSetFocus:
		if s, ok := sessions.Load(hwnd); ok {
			if sess, ok := s.(*Session); ok && sess.chromium != nil {
				sess.chromium.Focus()
			}
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

func hideWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}

func moveWindow(hwnd uintptr, x, y, width, height int) {
	if hwnd == 0 {
		return
	}
	procMoveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 1)
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

// parentHosts maps a parent window HWND to its Host.
var (
	parentHosts  sync.Map // uintptr -> *Host
	subclassOnce sync.Once
	subclassProc uintptr
)

func parentWindowSubclassProc(hwnd uintptr, uMsg uint32, wParam, lParam, uIdSubclass, dwRefData uintptr) uintptr {
	if uMsg == wmSize {
		if wParam != 1 { // not SIZE_MINIMIZED
			width := int(lParam & 0xFFFF)
			height := int((lParam >> 16) & 0xFFFF)
			if val, ok := parentHosts.Load(hwnd); ok {
				if h, ok := val.(*Host); ok {
					h.ResizeChildren(width, height)
				}
			}
		}
	} else if uMsg == wmNCDestroy {
		const subclassID = 0x57415353
		procRemoveWindowSubclass.Call(hwnd, subclassProc, subclassID)
		parentHosts.Delete(hwnd)
	}
	ret, _, _ := procDefSubclassProc.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return ret
}

func setWindowPosFast(hwnd uintptr, x, y, width, height int) {
	if hwnd == 0 {
		return
	}
	procSetWindowPos.Call(
		hwnd,
		0,
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		swpNoZOrder|swpNoActivate|swpNoCopyBits,
	)
}

// SubclassParentWindow hooks the parent window's native Win32 WM_SIZE messages
// so embedded child sessions resize synchronously at 60fps as the user drags
// the window borders.
func SubclassParentWindow(hwnd uintptr, h *Host) {
	if hwnd == 0 {
		return
	}
	parentHosts.Store(hwnd, h)
	subclassOnce.Do(func() {
		subclassProc = syscall.NewCallback(parentWindowSubclassProc)
	})
	const subclassID = 0x57415353 // "WASS"
	procSetWindowSubclass.Call(hwnd, subclassProc, subclassID, 0)
}
