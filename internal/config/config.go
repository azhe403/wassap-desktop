package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrCorruptConfig indicates the configuration file exists but cannot be parsed.
var ErrCorruptConfig = errors.New("config: file is corrupt")

// AppConfig represents persisted application configuration.
type AppConfig struct {
	ActiveAccountID string `json:"activeAccountId,omitempty"`
	CloseToTray     bool   `json:"closeToTray"`
}

// Store defines access to application configuration.
type Store interface {
	Load() (*AppConfig, error)
	Save(cfg *AppConfig) error
	SetActiveAccount(accountID string) error
	GetActiveAccount() (string, error)
	SetCloseToTray(enabled bool) error
	GetCloseToTray() (bool, error)
}

// JSONStore is a thread-safe, atomic file-backed configuration store.
type JSONStore struct {
	path string
	mu   sync.RWMutex
}

// NewStore returns a Store reading and writing to the specified JSON path.
func NewStore(path string) Store {
	return &JSONStore{path: path}
}

// Load reads and parses the configuration file. If the file does not exist,
// an empty AppConfig is returned without error.
func (s *JSONStore) Load() (*AppConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadLocked()
}

func (s *JSONStore) loadLocked() (*AppConfig, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return &AppConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: read error: %w", err)
	}

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptConfig, err)
	}
	return &cfg, nil
}

// Save atomically writes the configuration to disk.
func (s *JSONStore) Save(cfg *AppConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(cfg)
}

func (s *JSONStore) saveLocked(cfg *AppConfig) error {
	if cfg == nil {
		cfg = &AppConfig{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal error: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: mkdir error: %w", err)
	}

	var rb [8]byte
	_, _ = rand.Read(rb[:])
	tmpPath := fmt.Sprintf("%s.tmp.%s", s.path, hex.EncodeToString(rb[:]))

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: tempfile open error: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: write error: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: sync error: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: close error: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("config: rename error: %w", err)
	}
	return nil
}

// update applies fn to the current configuration and persists the result
// atomically while holding the store's write lock, so concurrent updates
// cannot silently overwrite each other.
func (s *JSONStore) update(fn func(*AppConfig)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.loadLocked()
	if err != nil {
		return err
	}
	fn(cfg)
	return s.saveLocked(cfg)
}

// SetActiveAccount updates and persists the active account ID.
func (s *JSONStore) SetActiveAccount(accountID string) error {
	return s.update(func(cfg *AppConfig) { cfg.ActiveAccountID = accountID })
}

// GetActiveAccount loads and returns the active account ID.
func (s *JSONStore) GetActiveAccount() (string, error) {
	cfg, err := s.Load()
	if err != nil {
		return "", err
	}
	return cfg.ActiveAccountID, nil
}

// SetCloseToTray updates and persists the close-to-tray preference.
func (s *JSONStore) SetCloseToTray(enabled bool) error {
	return s.update(func(cfg *AppConfig) { cfg.CloseToTray = enabled })
}

// GetCloseToTray loads and returns the close-to-tray preference.
func (s *JSONStore) GetCloseToTray() (bool, error) {
	cfg, err := s.Load()
	if err != nil {
		return false, err
	}
	return cfg.CloseToTray, nil
}
