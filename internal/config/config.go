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

// WindowState records the last known geometry of the main window so it can be
// restored on the next launch. It is nil until the window has been moved,
// resized or closed at least once.
//
// Units are deliberately asymmetric by platform convention: X/Y are raw
// screen pixels (the argument SetWindowPos takes), Width/Height are logical
// 96-DPI pixels (the unit Wails window options and WindowGetSize use).
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// AppConfig represents persisted application configuration.
type AppConfig struct {
	ActiveAccountID string       `json:"activeAccountId,omitempty"`
	CloseToTray     bool         `json:"closeToTray"`
	WindowState     *WindowState `json:"windowState,omitempty"`
	// AutoUpdate is opt-in: the app never downloads anything unless the user enables it.
	AutoUpdate bool `json:"autoUpdate"`
	// UpdateChannel is "stable" (default) or "nightly".
	UpdateChannel string `json:"updateChannel,omitempty"`
	// PrivacyBlur is the privacy blur section; nil when never written.
	PrivacyBlur *PrivacyBlurConfig `json:"privacyBlur,omitempty"`
}

// PrivacyBlurConfig holds the privacy blur settings. Its keys intentionally
// mirror the upstream engine's DEFAULT_SETTINGS names so the vendored engine
// and this store stay trivially comparable. All zero values are invalid in
// practice; use NormalizePrivacyBlurConfig.
type PrivacyBlurConfig struct {
	Enabled           bool    `json:"enabled"`
	PrivacyMode       string  `json:"privacyMode"` // "blur" | "redacted"
	BlurAmount        int     `json:"blurAmount"`  // px, 1..10
	Animation         bool    `json:"animation"`
	AnimationDuration float64 `json:"animationDuration"` // seconds, 0.05..2
	InputOpacity      int     `json:"inputOpacity"`     // 0..100
	UnblurLastN       bool    `json:"unblurLastN"`
	UnblurLastNCount  int     `json:"unblurLastNCount"` // 1..10
	BlurAvatars       bool    `json:"blurAvatars"`
	BlurNames         bool    `json:"blurNames"`
	BlurPreviews      bool    `json:"blurPreviews"`
	BlurTextChats     bool    `json:"blurTextChats"`
	BlurStickers      bool    `json:"blurStickers"`
	BlurMediaPreview  bool    `json:"blurMediaPreview"`
	BlurMediaGallery  bool    `json:"blurMediaGallery"`
	BlurInput         bool    `json:"blurInput"`
}

// DefaultPrivacyBlurConfig is the out-of-the-box section: everything blurred,
// master switch off. The master switch never defaults on — a desktop client
// must not change what the user's screen shows uninvited.
func DefaultPrivacyBlurConfig() PrivacyBlurConfig {
	return PrivacyBlurConfig{
		Enabled:           false,
		PrivacyMode:       "blur",
		BlurAmount:        3,
		Animation:         true,
		AnimationDuration: 0.25,
		InputOpacity:      30,
		UnblurLastN:       false,
		UnblurLastNCount:  3,
		BlurAvatars:       true,
		BlurNames:         true,
		BlurPreviews:      true,
		BlurTextChats:     true,
		BlurStickers:      true,
		BlurMediaPreview:  true,
		BlurMediaGallery:  true,
		BlurInput:         true,
	}
}

// NormalizePrivacyBlurConfig clamps every numeric field and fixes the mode
// enum. Unknown modes fall back to "blur".
func NormalizePrivacyBlurConfig(c PrivacyBlurConfig) PrivacyBlurConfig {
	if c.PrivacyMode != "redacted" {
		c.PrivacyMode = "blur"
	}
	c.BlurAmount = clampInt(c.BlurAmount, 1, 10)
	c.InputOpacity = clampInt(c.InputOpacity, 0, 100)
	c.UnblurLastNCount = clampInt(c.UnblurLastNCount, 1, 10)
	switch {
	case c.AnimationDuration < 0.05:
		c.AnimationDuration = 0.05
	case c.AnimationDuration > 2:
		c.AnimationDuration = 2
	}
	return c
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Store defines access to application configuration.
type Store interface {
	Load() (*AppConfig, error)
	Save(cfg *AppConfig) error
	SetActiveAccount(accountID string) error
	GetActiveAccount() (string, error)
	SetCloseToTray(enabled bool) error
	GetCloseToTray() (bool, error)
	SetWindowState(state WindowState) error
	GetWindowState() (*WindowState, error)
	SetAutoUpdate(enabled bool) error
	GetAutoUpdate() (bool, error)
	SetUpdateChannel(channel string) error
	GetUpdateChannel() (string, error)
	GetPrivacyBlurSettings() (*PrivacyBlurConfig, error)
	SetPrivacyBlurSettings(cfg PrivacyBlurConfig) error
	MergePrivacyBlurSettings(patch map[string]any) (*PrivacyBlurConfig, error)
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

// UpdateChannels are the update channels the app understands.
const (
	UpdateChannelStable  = "stable"
	UpdateChannelNightly = "nightly"
)

// IsUpdateChannel reports whether ch is a known update channel.
func IsUpdateChannel(ch string) bool {
	return ch == UpdateChannelStable || ch == UpdateChannelNightly
}

// SetAutoUpdate updates and persists the auto-update preference. It is off by default.
func (s *JSONStore) SetAutoUpdate(enabled bool) error {
	return s.update(func(cfg *AppConfig) { cfg.AutoUpdate = enabled })
}

// GetAutoUpdate loads and returns the auto-update preference.
func (s *JSONStore) GetAutoUpdate() (bool, error) {
	cfg, err := s.Load()
	if err != nil {
		return false, err
	}
	return cfg.AutoUpdate, nil
}

// SetUpdateChannel updates and persists the update channel. Only "stable" and
// "nightly" are accepted; anything else returns an error and persists nothing.
func (s *JSONStore) SetUpdateChannel(channel string) error {
	if !IsUpdateChannel(channel) {
		return fmt.Errorf("config: unknown update channel %q (want stable or nightly)", channel)
	}
	return s.update(func(cfg *AppConfig) { cfg.UpdateChannel = channel })
}

// GetUpdateChannel loads and returns the update channel. A missing value
// defaults to "stable".
func (s *JSONStore) GetUpdateChannel() (string, error) {
	cfg, err := s.Load()
	if err != nil {
		return "", err
	}
	if cfg.UpdateChannel == "" {
		return UpdateChannelStable, nil
	}
	return cfg.UpdateChannel, nil
}

// SetWindowState persists the main window geometry.
func (s *JSONStore) SetWindowState(state WindowState) error {
	return s.update(func(cfg *AppConfig) { cfg.WindowState = &state })
}

// GetWindowState loads and returns the last persisted window geometry. It
// returns (nil, nil) when no state has been saved yet.
func (s *JSONStore) GetWindowState() (*WindowState, error) {
	cfg, err := s.Load()
	if err != nil {
		return nil, err
	}
	return cfg.WindowState, nil
}

// GetPrivacyBlurSettings loads the privacy blur section. It returns
// (nil, nil) when the section has never been written.
func (s *JSONStore) GetPrivacyBlurSettings() (*PrivacyBlurConfig, error) {
	cfg, err := s.Load()
	if err != nil {
		return nil, err
	}
	return cfg.PrivacyBlur, nil
}

// SetPrivacyBlurSettings normalizes cfg and replaces the whole section
// atomically.
func (s *JSONStore) SetPrivacyBlurSettings(cfg PrivacyBlurConfig) error {
	return s.update(func(c *AppConfig) {
		normalized := NormalizePrivacyBlurConfig(cfg)
		c.PrivacyBlur = &normalized
	})
}

// MergePrivacyBlurSettings applies the recognized keys of patch to the stored
// section (or to the defaults when unwritten) under the store's write lock,
// and returns the merged, normalized result. Unrecognized keys and values of
// the wrong type are ignored; out-of-range numbers are clamped. Concurrent
// callers serialize on the lock instead of clobbering each other.
func (s *JSONStore) MergePrivacyBlurSettings(patch map[string]any) (*PrivacyBlurConfig, error) {
	if err := s.update(func(c *AppConfig) {
		base := DefaultPrivacyBlurConfig()
		if c.PrivacyBlur != nil {
			base = *c.PrivacyBlur
		}
		merged := applyPrivacyBlurPatch(base, patch)
		c.PrivacyBlur = &merged
	}); err != nil {
		return nil, err
	}
	return s.GetPrivacyBlurSettings()
}

// applyPrivacyBlurPatch applies recognized keys of patch to base and returns
// the normalized result. patch values of the wrong type are ignored.
func applyPrivacyBlurPatch(base PrivacyBlurConfig, patch map[string]any) PrivacyBlurConfig {
	for key, value := range patch {
		switch key {
		case "enabled":
			if v, ok := value.(bool); ok { base.Enabled = v }
		case "privacyMode":
			if v, ok := value.(string); ok { base.PrivacyMode = v }
		case "blurAmount":
			if v, ok := asIntPatch(value); ok { base.BlurAmount = v }
		case "animation":
			if v, ok := value.(bool); ok { base.Animation = v }
		case "animationDuration":
			if v, ok := value.(float64); ok { base.AnimationDuration = v }
		case "inputOpacity":
			if v, ok := asIntPatch(value); ok { base.InputOpacity = v }
		case "unblurLastN":
			if v, ok := value.(bool); ok { base.UnblurLastN = v }
		case "unblurLastNCount":
			if v, ok := asIntPatch(value); ok { base.UnblurLastNCount = v }
		case "blurAvatars":
			if v, ok := value.(bool); ok { base.BlurAvatars = v }
		case "blurNames":
			if v, ok := value.(bool); ok { base.BlurNames = v }
		case "blurPreviews":
			if v, ok := value.(bool); ok { base.BlurPreviews = v }
		case "blurTextChats":
			if v, ok := value.(bool); ok { base.BlurTextChats = v }
		case "blurStickers":
			if v, ok := value.(bool); ok { base.BlurStickers = v }
		case "blurMediaPreview":
			if v, ok := value.(bool); ok { base.BlurMediaPreview = v }
		case "blurMediaGallery":
			if v, ok := value.(bool); ok { base.BlurMediaGallery = v }
		case "blurInput":
			if v, ok := value.(bool); ok { base.BlurInput = v }
		}
	}
	return NormalizePrivacyBlurConfig(base)
}

// asIntPatch accepts the numeric types a JSON decoder produces (float64) and
// int/int64 for hand-built patches, truncating toward zero.
func asIntPatch(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
