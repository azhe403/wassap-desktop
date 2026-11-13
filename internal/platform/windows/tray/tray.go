//go:build windows

// Package tray implements the Wassap Desktop system tray icon and its hidden
// owner window.
//
// It is deliberately independent of the main application package: everything
// it needs from the application is handed in as Hooks (closures) at
// construction, so this package never imports package main and never leaks
// Wails-bindable methods onto the App facade.
//
// The tray icon is registered on a dedicated hidden top-level window running
// its own locked OS thread, so shell notifications never depend on the Wails
// window procedure or any subclassing. The window stays invisible for the
// process lifetime; it is a top-level window (not a message-only HWND_MESSAGE
// window) so it receives shell broadcasts such as TaskbarCreated, and so
// SetForegroundWindow works on it as the popup menu's owner window.
// The wndproc looks up the owning controller through a registry keyed on
// HWND, so the message thread never touches the controller's mutex.
package tray

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Win32 constants for Shell_NotifyIcon and tray menu.
const (
	nimAdd    = 0x00000000
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmUser          = 0x0400 // WM_USER + 0; the shell sends it as NIN_SELECT

	wmDestroy = 0x0002
	wmClose   = 0x0010

	// wmTrayCallback must not collide with Wails' internal invoke callback (0x8000).
	wmTrayCallback = 0x8001

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	mfString    = 0x0000
	mfSeparator = 0x0800

	idOpen     = 1
	idSettings = 2
	idQuit     = 3

	// wsPopup is the WS_POPUP style: a top-level window with no caption. The
	// tray owner window is created hidden (never WS_VISIBLE), so it does not
	// appear in the taskbar or Alt+Tab, but it still receives shell broadcasts.
	wsPopup = 0x80000000

	// swRestore shows and un-minimises a window.
	swRestore = 9

	// wmQuit posts WM_QUIT (0x0012) to the message loop, ending the pump.
	wmQuit = 0x0012
)

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconExW   = shell32.NewProc("ExtractIconExW")

	k32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = k32.NewProc("GetModuleHandleW")

	trayUser32               = syscall.NewLazyDLL("user32.dll")
	procRegisterClassExW     = trayUser32.NewProc("RegisterClassExW")
	procCreateWindowExW      = trayUser32.NewProc("CreateWindowExW")
	procDefWindowProcW       = trayUser32.NewProc("DefWindowProcW")
	procGetMessageW          = trayUser32.NewProc("GetMessageW")
	procTranslateMessage     = trayUser32.NewProc("TranslateMessage")
	procDispatchMessageW     = trayUser32.NewProc("DispatchMessageW")
	procDestroyWindow        = trayUser32.NewProc("DestroyWindow")
	procCreatePopupMenu      = trayUser32.NewProc("CreatePopupMenu")
	procAppendMenuW          = trayUser32.NewProc("AppendMenuW")
	procSetMenuDefaultItem   = trayUser32.NewProc("SetMenuDefaultItem")
	procTrackPopupMenu       = trayUser32.NewProc("TrackPopupMenu")
	procDestroyMenu          = trayUser32.NewProc("DestroyMenu")
	procGetCursorPos         = trayUser32.NewProc("GetCursorPos")
	procSetForegroundWindow  = trayUser32.NewProc("SetForegroundWindow")
	procShowWindow           = trayUser32.NewProc("ShowWindow")
	procPostMessageW         = trayUser32.NewProc("PostMessageW")
	procRegisterWindowMessageW = trayUser32.NewProc("RegisterWindowMessageW")
	procDestroyIcon           = trayUser32.NewProc("DestroyIcon")
)

// taskbarCreatedMsg is the shell broadcast message sent after Explorer
// restarts. A tray icon registered before the restart vanishes; the standard
// remedy is to re-add it on this message.
var taskbarCreatedMsg = func() uint32 {
	name, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	msg, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
	return uint32(msg)
}()

// NOTIFYICONDATAW — full Windows structure (952 bytes on 64-bit).
// A truncated struct makes Shell_NotifyIconW reject the call with
// "unspecified error", and the icon never receives any mouse messages.
type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeoutOrVer    uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
}

type trayWndClass struct {
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

type trayMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ X, Y int32 }
	private uint32
}

// Hooks are the closures the tray needs from the application. The main
// package builds them against the App struct; the tray package never
// imports package main.
type Hooks struct {
	// ShowWindow brings the window back: unminimise, show, and make the
	// active session visible.
	ShowWindow func()
	// OpenSettings shows the window and asks the frontend to open the settings view.
	OpenSettings func()
	// HideSession hides the native session (settings page/modal owns the area).
	HideSession func()
	// Quit terminates the application.
	Quit func()
}

// Controller owns all tray state for one application instance. Keeping it
// per-instance (instead of package-level globals) makes the state testable
// and eliminates the data race between the setup goroutine, the Win32
// message-pump thread, and the teardown path. The controller's mutable
// fields are guarded by its mutex; the Win32 wndproc looks up the owning
// controller through a global read-write-locked registry keyed on HWND, so
// the message thread never touches the controller's mutex.
type Controller struct {
	mu      sync.Mutex
	hooks   Hooks
	// appHwnd is read from the tray message thread; atomic so the message
	// thread never has to wait on the setup goroutine's mutex.
	appHwnd atomic.Uintptr
	hwnd    uintptr // our hidden top-level window; 0 until created
	added   bool    // icon registered with shell
	icon    uintptr // retained for re-adding the icon after an Explorer restart
}

// NewController creates an empty controller bound to hooks.
func NewController(hooks Hooks) *Controller {
	return &Controller{hooks: hooks}
}

// notifyIcon builds the Shell_NotifyIconW payload for the tray icon.
func notifyIcon(hwnd, icon uintptr) notifyIconData {
	nid := notifyIconData{
		hWnd:             hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayCallback,
		hIcon:            icon,
	}
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	copy(nid.szTip[:], syscall.StringToUTF16("Wassap Desktop"))
	return nid
}

// addIconLocked registers the icon with the shell. The caller must hold c.mu.
func (c *Controller) addIconLocked() (bool, error) {
	nid := notifyIcon(c.hwnd, c.icon)
	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if ret == 0 {
		return false, err
	}
	return true, nil
}

// Setup initialises the hidden top-level window and registers the tray icon.
// Safe to call once; subsequent calls are no-ops until the icon is removed.
func (c *Controller) Setup(hwnd uintptr) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hwnd != 0 {
		return // already set up
	}
	c.appHwnd.Store(hwnd)

	if err := c.ensureTrayWindowLocked(); err != nil {
		log.Printf("tray: message window failed: %v", err)
		trayLog("message window failed: %v", err)
		return
	}

	c.icon = loadAppIcon()

	ok, addErr := c.addIconLocked()
	if !ok {
		log.Printf("tray: Shell_NotifyIconW(NIM_ADD) failed: %v", addErr)
		trayLog("NIM_ADD failed: %v hwnd=%d icon=%d", addErr, c.hwnd, c.icon)
		// Roll the handle back so a later Setup() can retry.
		procPostMessageW.Call(c.hwnd, wmClose, 0, 0)
		if c.icon != 0 {
			procDestroyIcon.Call(c.icon)
		}
		c.hwnd = 0
		c.icon = 0
		return
	}
	c.added = true
	trayRegistryRegister(c.hwnd, c)
	log.Printf("tray: system tray icon added (hwnd=%d)", c.hwnd)
	trayLog("icon added hwnd=%d icon=%d", c.hwnd, c.icon)
}

// Remove tears down the tray icon and destroys the hidden top-level window.
// Safe to call multiple times; only the first call does real work.
func (c *Controller) Remove() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hwnd == 0 {
		return
	}
	trayRegistryUnregister(c.hwnd)
	if c.added {
		nid := notifyIconData{hWnd: c.hwnd, uID: 1}
		nid.cbSize = uint32(unsafe.Sizeof(nid))
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		c.added = false
	}
	procPostMessageW.Call(c.hwnd, wmClose, 0, 0)
	if c.icon != 0 {
		procDestroyIcon.Call(c.icon)
		c.icon = 0
	}
	c.hwnd = 0
}

// Ready reports whether the tray icon is currently registered with the shell.
// It is used to gate close-to-tray: hiding the window without a working tray
// would leave the app invisible with no way to quit.
func (c *Controller) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.added
}

// readd re-registers the tray icon with the shell. Called when the shell
// broadcasts the TaskbarCreated message (Explorer restart), which drops every
// previously registered icon. On failure c.added is cleared so Ready()
// reports the tray as unusable; otherwise the app could hide its window with
// no icon and no way back.
func (c *Controller) readd() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hwnd == 0 || c.icon == 0 {
		c.added = false
		return
	}
	ok, _ := c.addIconLocked()
	if !ok {
		log.Printf("tray: NIM_ADD on TaskbarCreated failed")
		c.added = false
		return
	}
	c.added = true
	log.Printf("tray: tray icon re-added after shell restart")
}

// ensureTrayWindowLocked registers the window class and creates the
// hidden top-level owner window on a locked OS thread that runs its own message pump.
// The caller must hold c.mu.
func (c *Controller) ensureTrayWindowLocked() error {
	if c.hwnd != 0 {
		return nil
	}

	ready := make(chan uintptr, 1)
	fail := make(chan error, 1)
	// The controller is captured in the goroutine closure, which creates the
	// window and then runs the message pump on it. It never touches c.hwnd:
	// the caller assigns c.hwnd after this function returns. The message
	// pump only uses the immutable HWND handed to the wndproc.
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		hInst, _, _ := procGetModuleHandleW.Call(0)
		className, _ := syscall.UTF16PtrFromString("WassapTrayMessageWindow")
		wc := trayWndClass{
			size:      uint32(unsafe.Sizeof(trayWndClass{})),
			wndProc:   trayWndProcPtr,
			instance:  hInst,
			className: className,
		}
		// ERROR_CLASS_ALREADY_EXISTS (1410): the class is already registered
		// (e.g. by a previous instance or an earlier Setup of this controller).
		// Reusing it is fine — the class is our private tray owner class.
		const errorClassAlreadyExists = 1410
		if atom, _, callErr := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
			if errno, ok := callErr.(syscall.Errno); !ok || errno != errorClassAlreadyExists {
				fail <- fmt.Errorf("RegisterClassExW: %v", callErr)
				return
			}
		}

		hwnd, _, callErr := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			0,
			wsPopup,
			0, 0, 0, 0,
			0, // top-level window: no parent, so shell broadcasts reach it
			0, hInst, 0,
		)
		if hwnd == 0 {
			fail <- fmt.Errorf("CreateWindowExW: %v", callErr)
			return
		}
		ready <- hwnd

		var m trayMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()

	select {
	case hwnd := <-ready:
		if hwnd == 0 {
			return fmt.Errorf("tray window handle is zero")
		}
		c.hwnd = hwnd
		return nil
	case err := <-fail:
		return err
	}
}

// trayWndProcPtr is the native callback registered with RegisterClassExW.
// It is a package-level syscall.NewCallback, resolved once at package init.
// The wndproc looks up the owning controller through the registry keyed on
// HWND, so it never touches the controller's mutex.
var trayWndProcPtr = syscall.NewCallback(trayWndProc)

func trayWndProc(hwnd uintptr, uMsg uint32, wParam, lParam uintptr) uintptr {
	if taskbarCreatedMsg != 0 && uMsg == taskbarCreatedMsg {
		// Explorer restarted; the shell dropped all tray icons. Re-add ours.
		if c := trayRegistryLookup(hwnd); c != nil {
			c.readd()
		}
		return 0
	}
	switch uMsg {
	case wmTrayCallback:
		// Look up the owning controller by HWND; the message thread never reads
		// the controller's mutable fields, it only passes the immutable HWND
		// it was given.
		if c := trayRegistryLookup(hwnd); c != nil {
			c.dispatchTrayMessage(hwnd, lParam)
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostMessageW.Call(hwnd, wmQuit, 0, 0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return ret
}

// dispatchTrayMessage is called from the Win32 message thread. It passes the
// immutable HWND to handlers; it never holds the controller's mutex or reads
// c.hwnd.
func (c *Controller) dispatchTrayMessage(hwnd uintptr, lParam uintptr) {
	trayLog("tray message lParam=%#x", lParam)
	switch lParam {
	case wmLButtonUp, wmLButtonDblClk, wmUser:
		c.activate()
	case wmRButtonUp:
		c.showTrayMenu(hwnd)
	}
}

// activate brings the window back: the hidden top-level owner window is the
// correct owner to foreground (SetForegroundWindow), then the app is asked to
// restore the Wails window and the active session.
func (c *Controller) activate() {
	c.appHwndShow()
	c.hooks.ShowWindow()
}

// appHwndShow shows and focuses the Wails window; it is the Win32 counterpart
// to the Wails runtime calls in the ShowWindow hook.
func (c *Controller) appHwndShow() {
	hwnd := c.appHwnd.Load()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, swRestore)
		procSetForegroundWindow.Call(hwnd)
	}
}

func (c *Controller) showTrayMenu(hwnd uintptr) {
	hmenu, _, _ := procCreatePopupMenu.Call()
	if hmenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hmenu)

	openLabel, _ := syscall.UTF16PtrFromString("Open Wassap")
	settingsLabel, _ := syscall.UTF16PtrFromString("Settings")
	quitLabel, _ := syscall.UTF16PtrFromString("Quit")

	procAppendMenuW.Call(hmenu, mfString, idOpen, uintptr(unsafe.Pointer(openLabel)))
	procAppendMenuW.Call(hmenu, mfString, idSettings, uintptr(unsafe.Pointer(settingsLabel)))
	procAppendMenuW.Call(hmenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hmenu, mfString, idQuit, uintptr(unsafe.Pointer(quitLabel)))
	procSetMenuDefaultItem.Call(hmenu, idOpen, 0)

	var pt struct{ X, Y int32 }
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Win32 requires the owner window to be foreground, otherwise the menu
	// stays on screen after the user clicks elsewhere.
	procSetForegroundWindow.Call(hwnd)

	cmd, _, _ := procTrackPopupMenu.Call(
		hmenu,
		tpmReturnCmd|tpmRightButton,
		uintptr(pt.X), uintptr(pt.Y),
		0,
		hwnd,
		0,
	)
	procPostMessageW.Call(hwnd, 0, 0, 0)

	switch cmd {
	case idOpen:
		c.activate()
	case idSettings:
		// The settings page owns the content area: hide the native session
		// before the window appears so the page shows, not the webview.
		c.hooks.HideSession()
		c.activate()
		c.hooks.OpenSettings()
	case idQuit:
		c.hooks.Quit()
	}
}

// loadAppIcon extracts the first icon embedded in this executable.
func loadAppIcon() uintptr {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}
	path, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return 0
	}
	var large, small uintptr
	n, _, _ := procExtractIconExW.Call(
		uintptr(unsafe.Pointer(path)),
		0,
		uintptr(unsafe.Pointer(&large)),
		uintptr(unsafe.Pointer(&small)),
		1,
	)
	if n == 0 {
		return 0
	}
	return large
}

// trayLog mirrors tray diagnostics to a file: production builds have no
// console, so a silent failure here is otherwise invisible.
func trayLog(format string, args ...any) {
	path := filepath.Join(os.TempDir(), "wassap-tray.log")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, format+"\n", args...)
}

// trayRegistry is a per-process registry mapping HWND → *Controller,
// used by the Win32 wndproc to find the owning controller without any
// package-level mutable pointer. It is populated during Setup and cleared
// during Remove; lookups from the message thread are safe because the map
// is guarded by an RWMutex.
type trayRegistry struct {
	mu     sync.RWMutex
	byHwnd map[uintptr]*Controller
}

var globalTrayRegistry = &trayRegistry{byHwnd: make(map[uintptr]*Controller)}

func trayRegistryRegister(hwnd uintptr, c *Controller) {
	globalTrayRegistry.mu.Lock()
	globalTrayRegistry.byHwnd[hwnd] = c
	globalTrayRegistry.mu.Unlock()
}

func trayRegistryUnregister(hwnd uintptr) {
	globalTrayRegistry.mu.Lock()
	delete(globalTrayRegistry.byHwnd, hwnd)
	globalTrayRegistry.mu.Unlock()
}

func trayRegistryLookup(hwnd uintptr) *Controller {
	globalTrayRegistry.mu.RLock()
	defer globalTrayRegistry.mu.RUnlock()
	return globalTrayRegistry.byHwnd[hwnd]
}
