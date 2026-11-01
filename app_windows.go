//go:build windows

package main

import (
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"

	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/platform/windows/oslock"
	webview2 "wassap-desktop/internal/platform/windows/webview2"
	"wassap-desktop/internal/platform/windows/webview2/inspector"
)

// sidebarWidthPx must match the frontend's --sidebar-width CSS value.
const sidebarWidthPx = 68

var (
	u32                 = syscall.NewLazyDLL("user32.dll")
	procEnumWindows     = u32.NewProc("EnumWindows")
	procGetWindowPID    = u32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible = u32.NewProc("IsWindowVisible")
	procGetClientRect   = u32.NewProc("GetClientRect")
)

// findMainWindowHWND returns the visible top-level window of this process with
// the largest client area. Picking by size avoids matching WebView2's helper
// windows, which are visible but have no client area.
func findMainWindowHWND(timeout time.Duration) uintptr {
	currentPid := uint32(os.Getpid())
	deadline := time.Now().Add(timeout)
	for {
		var best uintptr
		bestArea := 0
		cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var pid uint32
			procGetWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			if pid != currentPid {
				return 1
			}
			if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
				return 1
			}
			var clientRect struct {
				Left, Top, Right, Bottom int32
			}
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&clientRect)))
			area := int(clientRect.Right) * int(clientRect.Bottom)
			if area > bestArea {
				bestArea = area
				best = hwnd
			}
			return 1
		})
		procEnumWindows.Call(cb, 0)
		if best != 0 && bestArea > 0 {
			return best
		}
		if time.Now().After(deadline) {
			return 0
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// attachMainWindow runs at startup and attaches the host to the main window once it is visible.
func (a *App) attachMainWindow() {
	if a.host == nil {
		return
	}
	go func() {
		hwnd := findMainWindowHWND(15 * time.Second)
		if hwnd == 0 {
			log.Printf("webview2: main window not found within 15s; sessions will open as top-level windows")
			return
		}
		if h, ok := a.host.(*webview2.Host); ok {
			h.SetParentWindow(hwnd, sidebarWidthPx)
			log.Printf("webview2: attached main window handle %d", hwnd)
		}
	}()
}

// ensureMainWindow blocks until the parent is attached; called before opening a session.
func (a *App) ensureMainWindow() {
	if h, ok := a.host.(*webview2.Host); ok && h.ParentHWND != 0 {
		return
	}
	hwnd := findMainWindowHWND(5 * time.Second)
	if hwnd == 0 {
		log.Printf("webview2: main window not found; opening session as top-level fallback")
		return
	}
	if h, ok := a.host.(*webview2.Host); ok {
		h.SetParentWindow(hwnd, sidebarWidthPx)
		log.Printf("webview2: attached main window handle %d", hwnd)
	}
}

func initPlatform(dirs appdirs.Dirs) (*browser.Manager, HostController, browser.Inspector) {
	layout := browser.NewLayout(dirs.Data)
	host := webview2.NewHost("Wassap Desktop", whatsappURL)
	insp := inspector.New(layout)

	deps := browser.ManagerDeps{
		Layout:    layout,
		Applier:   browser.NewDiskApplier(),
		Opener:    host,
		Inspector: insp,
		Lock:      oslock.Lock,
	}
	manager := browser.NewManager(deps)
	return manager, host, insp
}
