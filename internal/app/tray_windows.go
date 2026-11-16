//go:build windows

package app

import (
	"log"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	traypkg "wassap-desktop/internal/platform/windows/tray"
)

// newTrayBackend builds the Windows tray backend with hooks into the app.
// The hooks keep all Win32 specifics in internal/platform/windows/tray and
// prevent package main's facade from importing tray internals.
func newTrayBackend(a *App) trayBackend {
	return traypkg.NewController(traypkg.Hooks{
		ShowWindow: a.showFromBackground,
		OpenSettings: func() {
			a.mu.Lock()
			ctx := a.ctx
			a.mu.Unlock()
			if ctx != nil {
				wailsRuntime.EventsEmit(ctx, "open-settings")
			}
		},
		HideSession: func() {
			if err := a.SetWebViewVisible(false); err != nil {
				log.Printf("tray: cannot hide the native session: %v", err)
			}
		},
		Quit: a.quitApp,
	})
}
