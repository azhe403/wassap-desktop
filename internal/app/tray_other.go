//go:build !windows

package app

// noopTray is a no-op tray backend for platforms without a system tray
// implementation. Ready() is false, so close-to-tray can never engage and
// the window always closes normally.
type noopTray struct{}

func (noopTray) Setup(uintptr) {}

func (noopTray) Remove() {}

func (noopTray) Ready() bool { return false }

// newTrayBackend returns the no-op tray backend.
func newTrayBackend(a *App) trayBackend { return noopTray{} }
