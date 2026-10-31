// Package appdirs resolves the directories Wassap Desktop uses for persistent
// state.
//
// The layout is identical on Windows, Linux and macOS and is always resolved
// against the current user's home directory:
//
//	~/.local/share/wassap-desktop/  data   (WebView2 user data / WhatsApp session)
//	~/.config/wassap-desktop/       config
//	~/.cache/wassap-desktop/        cache
//
// This deliberately does not use os.UserConfigDir or os.UserCacheDir, because
// those produce per-platform locations (%APPDATA%, ~/Library/..., $XDG_*) and
// would break the single documented convention.
package appdirs

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNoHomeDir is returned when the current user's home directory cannot be
// determined.
var ErrNoHomeDir = errors.New("appdirs: cannot determine user home directory")

// AppName is the directory name used for every root.
const AppName = "wassap-desktop"

// Dirs holds the resolved application directories.
type Dirs struct {
	// Home is the current user's home directory.
	Home string
	// Data is ~/.local/share/wassap-desktop.
	Data string
	// Config is ~/.config/wassap-desktop.
	Config string
	// Cache is ~/.cache/wassap-desktop.
	Cache string
}

// Resolve returns the application directories for the current user.
func Resolve() (Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Dirs{}, ErrNoHomeDir
	}
	if home == "" {
		return Dirs{}, ErrNoHomeDir
	}
	return Under(home), nil
}

// Under returns the application directories rooted at the given home
// directory. It performs no filesystem access and does not require the
// directories to exist.
func Under(home string) Dirs {
	return Dirs{
		Home:   home,
		Data:   filepath.Join(home, ".local", "share", AppName),
		Config: filepath.Join(home, ".config", AppName),
		Cache:  filepath.Join(home, ".cache", AppName),
	}
}

// Ensure creates the data and config directories if they do not exist yet.
//
// The cache directory is intentionally left alone: it holds only regenerable
// data, so creating it eagerly buys nothing.
func Ensure(d Dirs) error {
	for _, dir := range []string{d.Data, d.Config} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}
