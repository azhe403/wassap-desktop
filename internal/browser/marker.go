package browser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Marker content written to the profile's marker file when the engine reports a
// clean environment shutdown.
const (
	markerClean = "clean"
	markerOpen  = "open"
)

// MarkerName and LockFileName are the files the browser engine keeps at the top
// of its user data folder. Both are documented WebView2 artefacts: LOCK is the
// engine's exclusive lock and is only present while an environment is alive.
const (
	MarkerName = "wassap-session"
)

// ErrPartialProfile reports a profile whose staged storage could not be fully
// recovered.
var ErrPartialProfile = errors.New("browser: profile storage is only partially recovered")

// validatePath guards every filesystem operation that takes a path built from a
// profile. It rejects empty paths and paths whose base name is not the engine's
// user data folder, which catches a layout mix-up before it can delete the wrong
// directory.
func validatePath(path string) error {
	if path == "" {
		return errors.New("browser: empty storage path")
	}
	if base := filepath.Base(path); base != userDataDirName {
		return fmt.Errorf("browser: refusing to operate on %q: base name is not %q", path, userDataDirName)
	}
	return nil
}

// readMarker returns the marker content, or "" when it is absent.
func readMarker(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

// writeMarker replaces the marker atomically.
func writeMarker(path, value string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(value), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
