//go:build windows

package webview2

import (
	"syscall"
	"unsafe"
)

// ChildWindowInfo describes one child HWND of a parent window.
type ChildWindowInfo struct {
	Class        string
	Visible      bool
	Rect         Rect
	ParentIsMain bool
}

var (
	procGetWindowRect_  = user32.NewProc("GetWindowRect")
	procGetClassNameW   = user32.NewProc("GetClassNameW")
	procGetParent_      = user32.NewProc("GetParent")
	procGetDpiForWindow = user32.NewProc("GetDpiForWindow")
	procGetClientRect_  = user32.NewProc("GetClientRect")
)

// ChildWindowsOf returns every direct child window of parent.
func ChildWindowsOf(parent uintptr) []ChildWindowInfo {
	var out []ChildWindowInfo
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		buf := make([]uint16, 256)
		procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		vis, _, _ := procIsWindowVisible_.Call(hwnd)
		var r struct{ Left, Top, Right, Bottom int32 }
		procGetWindowRect_.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		p, _, _ := procGetParent_.Call(hwnd)
		out = append(out, ChildWindowInfo{
			Class:        syscall.UTF16ToString(buf),
			Visible:      vis != 0,
			Rect:         Rect{X: int(r.Left), Y: int(r.Top), Width: int(r.Right - r.Left), Height: int(r.Bottom - r.Top)},
			ParentIsMain: p == parent,
		})
		return 1
	})
	procEnumChildWindows.Call(parent, cb, 0)
	return out
}

// ClientWidth returns the client width of hwnd in physical pixels.
func ClientWidth(hwnd uintptr) int {
	var r struct{ Left, Top, Right, Bottom int32 }
	procGetClientRect_.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Right)
}

// ClientHeight returns the client height of hwnd in physical pixels.
func ClientHeight(hwnd uintptr) int {
	var r struct{ Left, Top, Right, Bottom int32 }
	procGetClientRect_.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Bottom)
}

// WindowDPI returns the effective DPI of hwnd (96 = 100% scaling).
func WindowDPI(hwnd uintptr) int {
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	return int(dpi)
}
