package updater

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var exeOldPatternCache = map[string]*regexp.Regexp{}

func exeOldPattern(base string) *regexp.Regexp {
	p, ok := exeOldPatternCache[base]
	if !ok {
		p = regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `\.old-([0-9]+)$`)
		exeOldPatternCache[base] = p
	}
	return p
}

type exeCandidate struct {
	path    string
	modTime time.Time
}

func exeOldCandidates(dir, base string) []exeCandidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("updater: cannot list %s: %v", dir, err)
		return nil
	}
	pattern := exeOldPattern(base)
	var out []exeCandidate
	for _, e := range entries {
		if e.IsDir() || !pattern.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, exeCandidate{path: filepath.Join(dir, e.Name()), modTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modTime.After(out[j].modTime) }) // newest first
	return out
}

// RestoreExe renames the newest <exeBase>.old-* sibling of exePath back to
// exePath when — and only when — the exe itself is missing. It returns
// restored=true when a binary was restored. If the exe is present it is a
// no-op. The relauncher invokes this before starting the new instance: the
// crash window between the two apply renames is recoverable from here even
// when the main binary cannot start.
func RestoreExe(exePath string) (bool, error) {
	if _, err := os.Stat(exePath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("updater: sweep: stat %s: %w", exePath, err)
	}
	dir, base := filepath.Dir(exePath), filepath.Base(exePath)
	cands := exeOldCandidates(dir, base)
	if len(cands) == 0 {
		return false, nil
	}
	log.Printf("updater: exe %s is missing; restoring %s (crash during update?)", exePath, cands[0].path)
	if err := os.Rename(cands[0].path, exePath); err != nil {
		return false, fmt.Errorf("updater: cannot restore %s: %w", cands[0].path, err)
	}
	return true, nil
}

// StartupSweep repairs and cleans update leftovers next to the exe. See
// RestoreExe for the restore half; the sweep removes matching <exeBase>.old-*
// files older than 1 hour when the exe is present. Younger ones are kept; the
// running instance may still be executing them.
func StartupSweep(exePath string) error {
	restored, err := RestoreExe(exePath)
	if err != nil {
		return err
	}
	if restored {
		return nil
	}

	dir := filepath.Dir(exePath)
	base := filepath.Base(exePath)
	olds := exeOldCandidates(dir, base)
	if len(olds) == 0 {
		return nil
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
