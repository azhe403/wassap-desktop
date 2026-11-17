// Package avatar stores WhatsApp profile photos as a local cache, keyed by
// account id. Photos arrive from the WebView2 page as data URIs; the store
// validates them before anything touches disk and falls back to "no avatar"
// on any failure.
package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxAvatarBytes = 2 * 1024 * 1024

var validID = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// Store keeps avatar files under <dataDir>/avatars/.
type Store struct {
	dir string
}

// NewStore returns a store rooted at the given application data directory.
// The avatars subdirectory is created lazily on the first write.
func NewStore(dataDir string) *Store {
	return &Store{dir: filepath.Join(dataDir, "avatars")}
}

// Get returns the stored data URI for id, or "" when no valid avatar exists.
func (s *Store) Get(id string) string {
	if !validID.MatchString(id) {
		return ""
	}
	for ext, mime := range map[string]string{".png": "image/png", ".jpg": "image/jpeg"} {
		path := filepath.Join(s.dir, id+ext)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
	}
	return ""
}

// Put validates and stores a data URI, returning the content hash. changed is
// false when the account already has byte-identical avatar data.
func (s *Store) Put(id, dataURI string) (hash string, changed bool, err error) {
	if !validID.MatchString(id) {
		return "", false, fmt.Errorf("avatar: invalid account id %q", id)
	}
	decoded, mime, err := decodeDataURI(dataURI)
	if err != nil {
		return "", false, err
	}
	if len(decoded) > maxAvatarBytes {
		return "", false, fmt.Errorf("avatar: payload too large (%d bytes)", len(decoded))
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		return "", false, fmt.Errorf("avatar: undecodable image: %w", err)
	}
	if cfg.Width < 32 || cfg.Height < 32 {
		return "", false, fmt.Errorf("avatar: image too small (%dx%d)", cfg.Width, cfg.Height)
	}

	sum := sha256.Sum256(decoded)
	hash = hex.EncodeToString(sum[:])
	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	}
	if existing, e := os.ReadFile(filepath.Join(s.dir, id+ext)); e == nil {
		old := sha256.Sum256(existing)
		if hex.EncodeToString(old[:]) == hash {
			return hash, false, nil
		}
	}

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", false, fmt.Errorf("avatar: cannot create store: %w", err)
	}
	tmp, err := os.CreateTemp(s.dir, ".tmp-")
	if err != nil {
		return "", false, fmt.Errorf("avatar: cannot create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", false, fmt.Errorf("avatar: cannot set file mode: %w", err)
	}
	if _, err := tmp.Write(decoded); err != nil {
		tmp.Close()
		return "", false, fmt.Errorf("avatar: cannot write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", false, fmt.Errorf("avatar: cannot close: %w", err)
	}
	path := filepath.Join(s.dir, id+ext)
	if err := os.Rename(tmpName, path); err != nil {
		return "", false, fmt.Errorf("avatar: cannot install file: %w", err)
	}
	// Remove the other extension so Get can never see two versions.
	for _, stale := range []string{".png", ".jpg"} {
		if stale != ext {
			_ = os.Remove(filepath.Join(s.dir, id+stale))
		}
	}
	return hash, true, nil
}

// Clear removes every avatar file for id. Missing files are not an error.
func (s *Store) Clear(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("avatar: invalid account id %q", id)
	}
	for _, ext := range []string{".png", ".jpg"} {
		err := os.Remove(filepath.Join(s.dir, id+ext))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("avatar: cannot clear %s: %w", id, err)
		}
	}
	return nil
}

func decodeDataURI(uri string) ([]byte, string, error) {
	const prefix = "data:"
	if !strings.HasPrefix(uri, prefix) {
		return nil, "", fmt.Errorf("avatar: not a data URI")
	}
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, "", fmt.Errorf("avatar: malformed data URI")
	}
	meta := uri[len(prefix):comma]
	if !strings.HasSuffix(meta, ";base64") {
		return nil, "", fmt.Errorf("avatar: data URI is not base64")
	}
	mime := strings.TrimSuffix(meta, ";base64")
	switch mime {
	case "image/jpeg", "image/png":
	default:
		return nil, "", fmt.Errorf("avatar: unsupported MIME %q", mime)
	}
	b, err := base64.StdEncoding.DecodeString(uri[comma+1:])
	if err != nil {
		return nil, "", fmt.Errorf("avatar: invalid base64: %w", err)
	}
	return b, mime, nil
}
