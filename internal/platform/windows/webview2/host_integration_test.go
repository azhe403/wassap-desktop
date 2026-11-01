//go:build windows

package webview2_test

import (
	"syscall"
	"testing"
	"unsafe"

	"wassap-desktop/internal/browser"
	webview2 "wassap-desktop/internal/platform/windows/webview2"
)

// TestHost_OpenProfileFailsCleanlyWithoutRuntimeStart verifies that the proof
// of concept's session layer reports a failed WebView2 start-up as an error
// instead of exiting the whole process (the default go-webview2 behaviour).
func TestHost_OpenProfileFailsCleanlyWithoutRuntimeStart(t *testing.T) {
	if _, err := webview2.CheckRuntime(); err != nil {
		t.Skipf("WebView2 runtime unavailable: %v", err)
	}

	host := webview2.NewHost("PocProbe", "about:blank")
	probe := browser.Probe{Path: `C:\Program Files\WindowsApps\__RESERVED_BY_POC__`}

	_, err := host.OpenProfile("probe", probe)
	if err == nil {
		t.Fatalf("OpenProfile on a reserved path succeeded; start-up must fail")
	}
	t.Logf("start-up failed as intended: %v", err)
}

func TestHost_ChildWindowConfiguration(t *testing.T) {
	user32 := syscall.NewLazyDLL("user32.dll")
	procCreateWindowExW := user32.NewProc("CreateWindowExW")
	procDestroyWindow := user32.NewProc("DestroyWindow")

	// Create a dummy parent window
	title, _ := syscall.UTF16PtrFromString("TestParentWindow")
	cls, _ := syscall.UTF16PtrFromString("STATIC")
	parentHwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(title)),
		0x00CF0000|0x10000000, // WS_OVERLAPPEDWINDOW | WS_VISIBLE
		100, 100, 800, 600,
		0, 0, 0, 0,
	)
	if parentHwnd == 0 {
		t.Skip("cannot create test parent window")
	}
	defer procDestroyWindow.Call(parentHwnd)

	host := webview2.NewHost("TestChild", "about:blank")
	host.SetParentWindow(parentHwnd, 68)

	if host.ParentHWND != parentHwnd {
		t.Errorf("expected ParentHWND=%d, got %d", parentHwnd, host.ParentHWND)
	}
	if host.SidebarWidth != 68 {
		t.Errorf("expected SidebarWidth=68, got %d", host.SidebarWidth)
	}
}
