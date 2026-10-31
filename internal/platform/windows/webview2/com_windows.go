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
