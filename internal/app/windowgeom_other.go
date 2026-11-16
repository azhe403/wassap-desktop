//go:build !windows

package app

import (
	"wassap-desktop/internal/config"
)

// startGeometryPolling is a no-op off Windows; window geometry is not
// persisted there yet. done is closed immediately so callers can wait on it.
func startGeometryPolling(stop <-chan struct{}, done chan struct{}, onSample func(x, y, w, h int, maximised bool), restore *config.WindowState) {
	close(done)
}
