//go:build windows

package webview2

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

// iCoreWebView2NavigationCompletedEventArgsVtbl mirrors the COM vtable of
// ICoreWebView2NavigationCompletedEventArgs. wailsapp/go-webview2 declares the
// GetIsSuccess slot but exports no accessor for it, and adding one would mean
// forking the binding. Declaring the slots here is enough: the interface itself
// is the same shape, so its pointer can be used directly.
type iCoreWebView2NavigationCompletedEventArgsVtbl struct {
	QueryInterface    uintptr
	AddRef            uintptr
	Release           uintptr
	GetIsSuccess      uintptr
	GetWebErrorStatus uintptr
	GetNavigationId   uintptr
}

type iCoreWebView2NavigationCompletedEventArgs struct {
	vtbl *iCoreWebView2NavigationCompletedEventArgsVtbl
}

// IsSuccess reads CoreWebView2NavigationCompletedEventArgs.IsSuccess.
func IsSuccess(args *edge.ICoreWebView2NavigationCompletedEventArgs) (bool, error) {
	if args == nil {
		return false, ErrNilCallbackArgs
	}
	a := (*iCoreWebView2NavigationCompletedEventArgs)(unsafe.Pointer(args))
	var result int32
	hr, _, _ := syscall.SyscallN(a.vtbl.GetIsSuccess, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&result)))
	if err := hrToError(hr); err != nil {
		return false, err
	}
	return result != 0, nil
}

// WebErrorStatus reads CoreWebView2NavigationCompletedEventArgs.WebErrorStatus.
func WebErrorStatus(args *edge.ICoreWebView2NavigationCompletedEventArgs) (uint32, error) {
	if args == nil {
		return 0, ErrNilCallbackArgs
	}
	a := (*iCoreWebView2NavigationCompletedEventArgs)(unsafe.Pointer(args))
	var result uint32
	hr, _, _ := syscall.SyscallN(a.vtbl.GetWebErrorStatus, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&result)))
	if err := hrToError(hr); err != nil {
		return 0, err
	}
	return result, nil
}

func hrToError(hr uintptr) error {
	if windows.Handle(hr) == windows.S_OK {
		return nil
	}
	return windows.Errno(hr)
}
