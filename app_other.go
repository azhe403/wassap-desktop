//go:build !windows

package main

import (
	"errors"

	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/browser"
)

type stubHost struct{}

func (s *stubHost) OpenProfile(id browser.ProfileID, probe browser.Probe) (browser.Session, error) {
	return nil, errors.New("platform not supported yet in Pecut 2")
}

func (s *stubHost) CloseAll() error {
	return nil
}

func (s *stubHost) UpdateBounds(width, height int) error {
	return nil
}

func (s *stubHost) SetVisible(visible bool) error {
	return nil
}

func (s *stubHost) SetActiveSession(id browser.ProfileID) {}

func (s *stubHost) EvalAll(script string) {}

func (s *stubHost) SetPrivacyDelegate(prelude func() string, apply func() string, onWrite func(map[string]any)) {
}

type stubInspector struct{}

func (s *stubInspector) Inspect(id browser.ProfileID) ([]browser.Evidence, error) {
	return nil, nil
}

func initPlatform(dirs appdirs.Dirs) (*browser.Manager, HostController, browser.Inspector) {
	layout := browser.NewLayout(dirs.Data)
	host := &stubHost{}
	insp := &stubInspector{}

	deps := browser.ManagerDeps{
		Layout:    layout,
		Applier:   browser.NewDiskApplier(),
		Opener:    host,
		Inspector: insp,
	}
	manager := browser.NewManager(deps)
	return manager, host, insp
}

func (a *App) attachMainWindow() {}

func (a *App) ensureMainWindow() {}

// wirePlatformNotifications is a no-op off Windows; notifications are a
// WebView2 feature.
func (a *App) wirePlatformNotifications() {}

// platformBringToFront is a no-op off Windows.
func (a *App) platformBringToFront() {}
