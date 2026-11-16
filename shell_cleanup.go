package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// staleShellProfile matches the wails-shell profile folders the shell creates
// on every launch (run-<pid>, see shellUserDataPath in main.go).
var staleShellProfile = regexp.MustCompile(`^run-([0-9]+)$`)

// cleanupStaleShellProfiles removes run-* WebView2 user-data folders left
// behind under dataDir/wails-shell by previous launches of the shell. The
// current process's folder (selfPID) is never touched. Deletion failures are
// logged and swallowed: a crashed browser process may still hold handles, and
// the folder is retried on the next launch. A missing shell directory is not
// an error (first run). A non-nil error is returned only when the shell
// directory itself cannot be read.
func cleanupStaleShellProfiles(dataDir string, selfPID int) error {
	shellDir := filepath.Join(dataDir, "wails-shell")
	entries, err := os.ReadDir(shellDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", shellDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m := staleShellProfile.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if pid == selfPID {
			continue
		}
		path := filepath.Join(shellDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			log.Printf("shell-cleanup: cannot remove %s: %v", path, err)
			continue
		}
		log.Printf("shell-cleanup: removed stale %s", path)
	}
	return nil
}
