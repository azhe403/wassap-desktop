package updater

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// StartupSweep repairs and cleans update leftovers next to the exe.
//
// Restore: if the exe is missing but exactly one <exeBase>.old-<pid> exists
// (crash between the two swap renames), it is renamed back — without this the
// install could never start again. Log a warning either way.
//
// Sweep: <exeBase>.old-<pid> files older than 1 hour are deleted. Younger
// ones are kept; the running instance may still be executing them.
func StartupSweep(exePath string) error {
	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	oldPattern := regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `\.old-([0-9]+)$`)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("updater: sweep: %w", err)
	}

	_, statErr := os.Stat(exePath)
	exeExists := statErr == nil

	type candidate struct {
		path    string
		modTime time.Time
	}
	var olds []candidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if !oldPattern.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		olds = append(olds, candidate{path: full, modTime: info.ModTime()})
	}

	if !exeExists && len(olds) == 1 {
		log.Printf("updater: exe missing but %s exists; restoring it (crash during update?)", olds[0].path)
		return os.Rename(olds[0].path, exePath)
	}
	if !exeExists && len(olds) > 1 {
		// Restore the most recently modified candidate.
		newest := olds[0]
		for _, c := range olds[1:] {
			if c.modTime.After(newest.modTime) {
				newest = c
			}
		}
		log.Printf("updater: exe missing and %d candidates exist; restoring newest %s", len(olds), newest.path)
		return os.Rename(newest.path, exePath)
	}

	cutoff := time.Now().Add(-time.Hour)
	for _, c := range olds {
		if c.modTime.Before(cutoff) {
			if err := os.Remove(c.path); err != nil {
				log.Printf("updater: sweep: cannot remove %s: %v", c.path, err)
			}
		}
	}
	return nil
}

