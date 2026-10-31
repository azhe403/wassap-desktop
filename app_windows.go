//go:build windows

package main

import (
	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/platform/windows/oslock"
	webview2 "wassap-desktop/internal/platform/windows/webview2"
	"wassap-desktop/internal/platform/windows/webview2/inspector"
)

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
