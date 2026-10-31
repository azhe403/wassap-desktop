package browser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// diskApplier implements Applier for a Chromium-style user data folder on a
// local filesystem. The proof of concept uses it for WebView2, whose user data
// folder is laid out as <user data folder>/Default/... .
//
// The staging steps are plain directory moves, which keeps both the previous
// state and the uncommitted state complete at every point: a crash leaves a
// profile that Recover can finish, never a half-copied one.
type diskApplier struct{}

// NewDiskApplier returns the default Applier.
func NewDiskApplier() Applier { return diskApplier{} }

func (diskApplier) LoadMemory(path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	memory := memoryPathFor(path)
	backup := backupPathFor(path)
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(memory), 0o700); err != nil {
		return err
	}
	if err := writeMarker(markerPathFor(path), markerOpen); err != nil {
		return err
	}
	if !dirExists(path) {
		// Nothing to preserve; the browser starts a fresh in-memory profile.
		return os.MkdirAll(memory, 0o700)
	}
	// Move the persisted state out of the way first: this is the step that makes
	// "on disk" mean "previous state" for the whole session.
	if err := os.Rename(path, backup); err != nil {
		return fmt.Errorf("browser: cannot move %s aside: %w", path, err)
	}
	if err := os.MkdirAll(memory, 0o700); err != nil {
		return err
	}
	return copyTree(backup, memory)
}

func (diskApplier) CommitMemory(path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	memory := memoryPathFor(path)
	if !dirExists(memory) {
		return fmt.Errorf("browser: no in-memory storage at %s", memory)
	}
	if err := writeMarker(markerPathFor(path), markerClean); err != nil {
		return err
	}
	if dirExists(path) {
		return fmt.Errorf("browser: refusing to overwrite %s", path)
	}
	return os.Rename(memory, path)
}

func (diskApplier) RestoreBackup(path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	backup := backupPathFor(path)
	if !dirExists(backup) {
		return nil
	}
	if dirExists(path) {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return os.Rename(backup, path)
}

func (diskApplier) DropBackup(path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	backup := backupPathFor(path)
	if !dirExists(backup) {
		return nil
	}
	return os.RemoveAll(backup)
}

func (diskApplier) HasStorage(path string) bool { return dirExists(path) }

func (diskApplier) LockFile(path string) string { return filepath.Join(path, "LOCK") }

func (diskApplier) MarkerFile(path string) string { return markerPathFor(path) }

func (diskApplier) Purge(path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	// Remove the per-profile staging directory too, so no copy of the storage
	// outlives the profile.
	return os.RemoveAll(filepath.Dir(filepath.Dir(path)))
}

// The staging layout is fixed by the manager: <staging>/<id>/<memory|backup>/EBWebView.
// The paths are derived from the user data folder so the applier needs no extra
// state.
func memoryPathFor(userData string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(userData)), "memory", userDataDirName)
}

func backupPathFor(userData string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(userData)), "backup", userDataDirName)
}

// The marker lives beside the user data folder, never inside it: WebView2 owns
// everything inside the folder and a foreign file there would be cleaned up.
func markerPathFor(userData string) string {
	return filepath.Join(filepath.Dir(userData), MarkerName)
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// copyTree copies a directory tree, preserving file modes and modification
// times. Symbolic links are copied as their target content; Chromium does not
// use links inside its user data folder in practice.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyFile(path, target, info)
	})
}

func copyFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
