//go:build windows

package main

import (
	"log"
	"math"
	"syscall"
	"time"
	"unsafe"

	"wassap-desktop/internal/config"
)

// Restore of the saved window position, clamped so the window never ends up
// on a monitor that is no longer connected (the classic way windows become
// unreachable after a display change). Wails' own WindowSetPosition adds the
// monitor work-area origin to the given coordinates, which corrupts absolute
// positions on secondary monitors, so the restore goes through SetWindowPos
// directly with raw screen pixels.
var (
	um32                 = syscall.NewLazyDLL("user32.dll")
	procMonitorFromPoint = um32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW  = um32.NewProc("GetMonitorInfoW")
	procSetWindowPosAbs  = um32.NewProc("SetWindowPos")
)

const (
	monitorDefaultToNearest = 2
	swpNoSize               = 0x0001
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
)

var (
	pmGetWindowRect   = um32.NewProc("GetWindowRect")
	pmIsZoomed        = um32.NewProc("IsZoomed")
	pmIsIconic        = um32.NewProc("IsIconic")
	pmGetDpiForWindow = um32.NewProc("GetDpiForWindow")
)

type geomSample struct {
	x, y, w, h int
	maximised  bool
}

// sampleWindowGeometry reads the window's geometry once. Width/height are
// returned in logical (96 DPI) pixels, the units Wails uses for window
// options and WindowGetSize; position stays raw screen pixels, which is what
// SetWindowPos takes. Minimised windows are reported as not observable.
func sampleWindowGeometry(hwnd uintptr) *geomSample {
	if iconic, _, _ := pmIsIconic.Call(hwnd); iconic != 0 {
		return nil
	}
	var r win32Rect
	if ret, _, _ := pmGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ret == 0 {
		return nil
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return nil
	}
	zoomed, _, _ := pmIsZoomed.Call(hwnd)
	dpi, _, _ := pmGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	lw := int(math.Round(float64(w) * 96 / float64(dpi)))
	lh := int(math.Round(float64(h) * 96 / float64(dpi)))
	return &geomSample{x: int(r.Left), y: int(r.Top), w: lw, h: lh, maximised: zoomed != 0}
}

// startGeometryPolling polls the main window's geometry and feeds it to
// onSample until the stop channel is closed; it closes done when the
// goroutine exits. If restore is non-nil and not maximised, the saved
// position is applied once, the first time the window becomes observable,
// clamped to a connected monitor. It exists because window messages
// (WM_SIZE/WM_MOVE) never legitimately reach our subclass here: with the
// embedded WS_CHILD sessions, the winc window ignores them — polling is the
// only reliable way to observe geometry changes. Polling is local Win32
// reads at 4 Hz, negligible next to the 250 ms UI tick.
func startGeometryPolling(stop <-chan struct{}, done chan struct{}, onSample func(x, y, w, h int, maximised bool), restore *config.WindowState) {
	go func() {
		defer close(done)
		hwnd := findMainWindowHWND(15 * time.Second)
		if hwnd == 0 {
			log.Printf("window-geometry: main window not found; window geometry will not be remembered")
			return
		}
		// Apply the saved position exactly once, on the first observable
		// sample. A maximised window is not repositioned: its rect belongs to
		// the shell, so moving it would not help (the pre-maximise rect is not
		// addressable via window options; that is a Wails limitation).
		applyRestore := restore != nil && !restore.Maximised
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		var last *geomSample
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s := sampleWindowGeometry(hwnd)
				if s == nil {
					continue
				}
				if applyRestore {
					applySavedPosition(hwnd, restore.X, restore.Y)
					applyRestore = false
				}
				if last != nil && *last == *s {
					continue
				}
				last = s
				onSample(s.x, s.y, s.w, s.h, s.maximised)
			}
		}
	}()
}

type win32Rect struct {
	Left, Top, Right, Bottom int32
}

type win32MonitorInfo struct {
	cbSize    uint32
	rcMonitor win32Rect
	rcWork    win32Rect
	dwFlags   uint32
}

// pointInWorkArea reports whether pt lies inside the given monitor's work area.
func pointInWorkArea(work win32Rect, x, y int) bool {
	return x >= int(work.Left) && x < int(work.Right) && y >= int(work.Top) && y < int(work.Bottom)
}

// clampToVisibleMonitor moves (x, y) onto the nearest monitor's work area when
// the saved point is no longer on any connected display. When the point is
// visible but the window would only partly hang off-screen, the top-left is
// kept: Windows clamps off-screen edges itself during activation.
func clampToVisibleMonitor(x, y int) (int, int) {
	// POINT is two int32s passed by value in one 64-bit register on amd64.
	packed := uintptr(uint32(int32(x))) | uintptr(uint32(int32(y)))<<32
	hmon, _, _ := procMonitorFromPoint.Call(packed, monitorDefaultToNearest)
	if hmon == 0 {
		return x, y
	}
	var mi win32MonitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	if ret, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ret == 0 {
		return x, y
	}
	if pointInWorkArea(mi.rcWork, x, y) {
		return x, y
	}
	cx := int(mi.rcWork.Left) + 40
	cy := int(mi.rcWork.Top) + 40
	log.Printf("window-geometry: saved position (%d,%d) is off-screen; clamping to (%d,%d)", x, y, cx, cy)
	return cx, cy
}

// applySavedPosition moves the window to the saved position, clamped so it
// never ends up unreachable after a monitor change.
func applySavedPosition(hwnd uintptr, x, y int) {
	if hwnd == 0 {
		return
	}
	cx, cy := clampToVisibleMonitor(x, y)
	procSetWindowPosAbs.Call(hwnd, 0, uintptr(int32(cx)), uintptr(int32(cy)), 0, 0,
		uintptr(swpNoSize|swpNoZOrder|swpNoActivate))
}
