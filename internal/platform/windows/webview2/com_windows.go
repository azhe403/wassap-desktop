//go:build windows

package webview2

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

// wailsapp/go-webview2 declares the ICoreWebView2Environment and
// ICoreWebView2Controller vtables but exports no Close method on either: its
// environment Close awaits a completed handler and is not wrapped, and the
// controller Close is not wrapped at all. Shutdown has to be explicit, because
// closing the controller is what flushes the profile to disk and closing the
// environment is what ends the browser process and releases the user data
// folder's LOCK. Both calls below are plain COM calls on the interface pointers
// go-webview2 already returns, so the binding is used, not forked.

// iCoreWebView2EnvironmentVtbl mirrors ICoreWebView2Environment. wailsapp's own
// struct stops after RemoveNewBrowserVersionAvailable, so Close is declared here
// at its real index.
type iCoreWebView2EnvironmentVtbl struct {
	QueryInterface                   uintptr
	AddRef                           uintptr
	Release                          uintptr
	CreateCoreWebView2Controller     uintptr
	CreateWebResourceResponse        uintptr
	GetBrowserVersionString          uintptr
	AddNewBrowserVersionAvailable    uintptr
	RemoveNewBrowserVersionAvailable uintptr
	Close                            uintptr
}

type iCoreWebView2Environment struct {
	vtbl *iCoreWebView2EnvironmentVtbl
}

// iCoreWebView2ControllerVtbl mirrors ICoreWebView2Controller up to Close.
type iCoreWebView2ControllerVtbl struct {
	QueryInterface                    uintptr
	AddRef                            uintptr
	Release                           uintptr
	GetIsVisible                      uintptr
	PutIsVisible                      uintptr
	GetBounds                         uintptr
	PutBounds                         uintptr
	GetZoomFactor                     uintptr
	PutZoomFactor                     uintptr
	AddZoomFactorChanged              uintptr
	RemoveZoomFactorChanged           uintptr
	SetBoundsAndZoomFactor            uintptr
	MoveFocus                         uintptr
	AddMoveFocusRequested             uintptr
	RemoveMoveFocusRequested          uintptr
	AddGotFocus                       uintptr
	RemoveGotFocus                    uintptr
	AddLostFocus                      uintptr
	RemoveLostFocus                   uintptr
	AddAcceleratorKeyPressed          uintptr
	RemoveAcceleratorKeyPressed       uintptr
	GetParentWindow                   uintptr
	PutParentWindow                   uintptr
	NotifyParentWindowPositionChanged uintptr
	Close                             uintptr
	GetCoreWebView2                   uintptr
}

type iCoreWebView2Controller struct {
	vtbl *iCoreWebView2ControllerVtbl
}

// CloseEnvironment calls ICoreWebView2Environment::Close on an environment
// obtained from edge.Chromium.Environment. The exit code is the value
// CoreWebView2Environment.Close(exitCode) expects; an explicit shutdown uses
// S_OK. This call is what ends the browser process and releases the user data
// folder's LOCK.
func CloseEnvironment(env *edge.ICoreWebView2Environment) error {
	if env == nil {
		return ErrNilCallbackArgs
	}
	e := (*iCoreWebView2Environment)(unsafe.Pointer(env))
	hr, _, _ := syscall.Syscall(e.vtbl.Close,
		1, uintptr(unsafe.Pointer(e)), uintptr(windows.S_OK), 0)
	return hrToError(hr)
}

// CloseController calls ICoreWebView2Controller::Close on a controller obtained
// from edge.Chromium.GetController. It is what flushes the profile to disk.
func CloseController(c *edge.ICoreWebView2Controller) error {
	if c == nil {
		return ErrNilCallbackArgs
	}
	ctrl := (*iCoreWebView2Controller)(unsafe.Pointer(c))
	hr, _, _ := syscall.Syscall(ctrl.vtbl.Close, 1, uintptr(unsafe.Pointer(ctrl)), 0, 0)
	return hrToError(hr)
}

var iidICoreWebView2Controller2 = windows.GUID{
	Data1: 0xc39e72d7,
	Data2: 0xedd3,
	Data3: 0x4595,
	Data4: [8]byte{0x9e, 0x5d, 0x76, 0xb8, 0x64, 0x2d, 0x63, 0xea},
}

type iCoreWebView2Controller2Vtbl struct {
	iCoreWebView2ControllerVtbl
	GetDefaultBackgroundColor uintptr
	PutDefaultBackgroundColor uintptr
}

type iCoreWebView2Controller2 struct {
	vtbl *iCoreWebView2Controller2Vtbl
}

// SetControllerBackgroundColor sets the default background color of the WebView2 controller.
// Setting it to a dark color prevents the white flash during resize or page load.
func SetControllerBackgroundColor(c *edge.ICoreWebView2Controller, a, r, g, b byte) error {
	if c == nil {
		return ErrNilCallbackArgs
	}
	ctrl := (*iCoreWebView2Controller)(unsafe.Pointer(c))
	var ctrl2 *iCoreWebView2Controller2
	hr, _, _ := syscall.SyscallN(
		ctrl.vtbl.QueryInterface,
		uintptr(unsafe.Pointer(ctrl)),
		uintptr(unsafe.Pointer(&iidICoreWebView2Controller2)),
		uintptr(unsafe.Pointer(&ctrl2)),
	)
	if hr != 0 || ctrl2 == nil {
		return hrToError(hr)
	}
	defer func() {
		syscall.SyscallN(ctrl2.vtbl.Release, uintptr(unsafe.Pointer(ctrl2)))
	}()

	// COREWEBVIEW2_COLOR struct (4 bytes: A, R, G, B) is passed directly in register
	color := uint32(a) | (uint32(r) << 8) | (uint32(g) << 16) | (uint32(b) << 24)
	hr, _, _ = syscall.SyscallN(
		ctrl2.vtbl.PutDefaultBackgroundColor,
		uintptr(unsafe.Pointer(ctrl2)),
		uintptr(color),
	)
	return hrToError(hr)
}

// NotifyPositionChanged informs WebView2 that the parent window moved or resized,
// keeping the DirectComposition visual tree and swapchain in sync without latency.
func NotifyPositionChanged(c *edge.ICoreWebView2Controller) error {
	if c == nil {
		return ErrNilCallbackArgs
	}
	ctrl := (*iCoreWebView2Controller)(unsafe.Pointer(c))
	hr, _, _ := syscall.SyscallN(ctrl.vtbl.NotifyParentWindowPositionChanged, uintptr(unsafe.Pointer(ctrl)))
	return hrToError(hr)
}
