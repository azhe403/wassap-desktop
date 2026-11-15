package updater

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UpdateCheckCache is the persisted result of the last successful (or
// attempted) update check. It is machine-local state under
// ~/.cache/wassap-desktop/; missing or corrupt files degrade to the zero
// value rather than to errors.
type UpdateCheckCache struct {
	CheckedAt        time.Time `json:"checkedAt,omitempty"`
	Channel          string    `json:"channel,omitempty"`
	CurrentVersion   string    `json:"currentVersion,omitempty"`
	LatestVersion    string    `json:"latestVersion,omitempty"`
	UpdateAvailable  bool      `json:"updateAvailable"`
	ReleaseURL       string    `json:"releaseUrl,omitempty"`
	StagedAsset      string    `json:"stagedAsset,omitempty"`
	StagedDigest     string    `json:"stagedDigest,omitempty"`
	DismissedVersion string    `json:"dismissedVersion,omitempty"`
}

// Dismissed reports whether the given latest-version value was dismissed by
// the user. A different (newer) version re-arms the notice.
func (c UpdateCheckCache) Dismissed(v string) bool {
	return v != "" && c.DismissedVersion == v
}

// Cache reads and writes the update-check file at Path with atomic replace.
type Cache struct {
	Path string
}

// Load reads the cache. A missing or corrupt file returns the zero value and
// no error.
func (c *Cache) Load() (UpdateCheckCache, error) {
	data, err := os.ReadFile(c.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return UpdateCheckCache{}, nil
		}
		return UpdateCheckCache{}, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var cfg UpdateCheckCache
	if err := json.Unmarshal(data, &cfg); err != nil {
		return UpdateCheckCache{}, nil
	}
	return cfg, nil
}

// Store atomically persists the cache: temp file + rename, mode 0600.
func (c *Cache) Store(cfg UpdateCheckCache) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("updater: marshal error: %w", err)
	}

	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("updater: mkdir error: %w", err)
	}

	var rb [8]byte
	_, _ = rand.Read(rb[:])
	tmpPath := fmt.Sprintf("%s.tmp.%s", c.Path, hex.EncodeToString(rb[:]))

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("updater: tempfile open error: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("updater: write error: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("updater: sync error: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("updater: close error: %w", err)
	}
	if err := os.Rename(tmpPath, c.Path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("updater: rename error: %w", err)
	}
	return nil
}
