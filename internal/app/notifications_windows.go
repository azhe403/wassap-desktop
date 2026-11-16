//go:build windows

package app

import (
	"syscall"
	"unsafe"

	webview2 "wassap-desktop/internal/platform/windows/webview2"
)

// Force-foreground sequence for the notification click path.
//
// Windows grants the right to activate a window (SetForegroundWindow) to the
// process that received the most recent input. Clicking a desktop
// notification is input to the WebView2 browser process that owns the toast,
// not to this process, so a plain SetForegroundWindow from the notification
// click handler silently fails and the window stays behind. Attaching this
// thread's input to the foreground thread first makes the call stick; this is
// the standard pre-Vista-era workaround that still works on modern Windows.
// Unlike clicking the tray icon - which is input directly to this process and
// needs no such trick - there is no sanctioned way to earn the foreground
// right from a notification click.

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	kernel32      = syscall.NewLazyDLL("kernel32.dll")

	procGetCurrentThreadId      = kernel32.NewProc("GetCurrentThreadId")
	procGetForegroundWindow     = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput       = user32.NewProc("AttachThreadInput")
	procShowWindow              = user32.NewProc("ShowWindow")
	procBringWindowToTop        = user32.NewProc("BringWindowToTop")
	procSetForegroundWindow     = user32.NewProc("SetForegroundWindow")
	procSetActiveWindow         = user32.NewProc("SetActiveWindow")
)

const swShow = 5 // SW_SHOW

// platformBringToFront shows the hidden/minimised main window, if any, and
// forcibly pulls it to the foreground using the attach-thread-input trick.
// Called on the notification activation path after wailsRuntime.WindowShow.
func (a *App) platformBringToFront() {
	h, ok := a.host.(*webview2.Host)
	if !ok {
		return
	}
	hwnd := h.ParentHandle()
	if hwnd == 0 {
		return
	}

	curThread, _, _ := procGetCurrentThreadId.Call()
	fg, _, _ := procGetForegroundWindow.Call()

	// Identify the thread that currently owns the foreground window so its
	// input queue can be attached to. Best effort: on failure the plain
	// SetForegroundWindow below still runs.
	var fgPid uint32
	fgThread, _, _ := procGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&fgPid)))
	attached := fgThread != 0 && fgThread != curThread
	if attached {
		procAttachThreadInput.Call(curThread, fgThread, 1)
	}

	procShowWindow.Call(hwnd, swShow)
	procBringWindowToTop.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	procSetActiveWindow.Call(hwnd)

	if attached {
		procAttachThreadInput.Call(curThread, fgThread, 0)
	}
}
