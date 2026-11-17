package avatar

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestStorePutWritesFileAndReturnsHash(t *testing.T) {
	s := NewStore(t.TempDir())
	uri := pngDataURI(t, 64, 64)

	hash, changed, err := s.Put("a01", uri)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first Put must report changed")
	}
	if len(hash) != 64 {
		t.Fatalf("hash = %q, want 64 hex chars", hash)
	}
	if got := s.Get("a01"); got != uri {
		t.Fatalf("Get = %q, want stored data URI", got)
	}
	path := filepath.Join(s.dir, "a01.png")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("avatar file missing: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("avatar is not a regular file: %v", info.Mode())
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("avatar mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestStorePutRejectsInvalidPayloads(t *testing.T) {
	big := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0}, 2*1024*1024+1))
	tests := []struct {
		name string
		uri  string
	}{
		{"webp-mime", "data:image/webp;base64,AA=="},
		{"malformed-base64", "data:image/png;base64,!!!not-base64!!!"},
		{"oversized", big},
		{"too-small", pngDataURI(t, 16, 16)},
		{"not-a-data-uri", "https://example.com/photo.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore(t.TempDir())
			if _, _, err := s.Put("a01", tc.uri); err == nil {
				t.Fatal("Put must reject invalid payload")
			}
			if got := s.Get("a01"); got != "" {
				t.Fatalf("invalid payload stored: %q", got)
			}
		})
	}
}

func TestStorePutSameBytesDoesNotRewrite(t *testing.T) {
	s := NewStore(t.TempDir())
	uri := pngDataURI(t, 64, 64)
	if _, _, err := s.Put("a01", uri); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.dir, "a01.png")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	_, changed, err := s.Put("a01", uri)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("identical bytes must not report changed")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical bytes must not rewrite the file")
	}
}

func TestStoreClearRemovesAllExtensionsForID(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, _, err := s.Put("a01", pngDataURI(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	jpg := filepath.Join(s.dir, "a01.jpg")
	if err := os.WriteFile(jpg, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Clear("a01"); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".png", ".jpg"} {
		p := filepath.Join(s.dir, "a01"+ext)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("file %s still exists after Clear", p)
		}
	}
	if got := s.Get("a01"); got != "" {
		t.Fatalf("Get after Clear = %q, want empty", got)
	}
	if err := s.Clear("a01"); err != nil {
		t.Fatalf("Clear on missing avatar must be nil, got %v", err)
	}
}

func TestStoreRejectsUnsafeID(t *testing.T) {
	s := NewStore(t.TempDir())
	for _, id := range []string{"../escape", "a/b", "", strings.Repeat("x", 65)} {
		if _, _, err := s.Put(id, pngDataURI(t, 64, 64)); err == nil {
			t.Errorf("Put(%q) must reject unsafe id", id)
		}
	}
	outside := filepath.Join(s.dir, "escape.png")
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Errorf("unsafe id wrote outside avatars dir: %s", outside)
	}
}

func TestStoreGetReturnsEmptyWhenAbsent(t *testing.T) {
	s := NewStore(t.TempDir())
	if got := s.Get("missing"); got != "" {
		t.Fatalf("Get = %q, want empty", got)
	}
}
