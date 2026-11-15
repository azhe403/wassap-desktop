package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

var payload = bytes.Repeat([]byte("wassap-desktop-update-test-payload "), 512) // ~18 KB

func hexSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func newDownloadServer(t *testing.T, served []byte, contentLength int64, digest string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(served)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func digestOf(b []byte) string { return "sha256:" + hexSum(b) }

func TestDownloadStagesAndVerifies(t *testing.T) {
	dir := t.TempDir()
	srv := newDownloadServer(t, payload, int64(len(payload)), "")
	defer srv.Close()

	d := NewDownloader(dir)
	d.freeBytes = func(string) (uint64, error) { return 1 << 40, nil }

	var progressCalls []int64
	res, err := d.Download(context.Background(), Asset{
		Name:               "wassap-desktop-windows-amd64.exe",
		BrowserDownloadURL: srv.URL + "/asset",
		Digest:             digestOf(payload),
	}, func(done, total int64) { progressCalls = append(progressCalls, done) })
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(dir, "wassap-desktop-windows-amd64.exe"); res.Path != want {
		t.Errorf("path = %q, want %q", res.Path, want)
	}
	if got := res.SHA256; got != hexSum(payload) {
		t.Errorf("SHA256 = %q, want %q", got, hexSum(payload))
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("staged bytes differ from served bytes")
	}
	if len(progressCalls) == 0 {
		t.Fatal("progress callback was never called")
	}
	if progressCalls[len(progressCalls)-1] != int64(len(payload)) {
		t.Errorf("final done = %d, want %d", progressCalls[len(progressCalls)-1], len(payload))
	}
	for i := 1; i < len(progressCalls); i++ {
		if progressCalls[i] < progressCalls[i-1] {
			t.Fatalf("progress not monotonic: %v", progressCalls)
		}
	}
}

func TestDownloadDigestMismatchDeletesFiles(t *testing.T) {
	dir := t.TempDir()
	srv := newDownloadServer(t, payload, int64(len(payload)), "")
	defer srv.Close()

	d := NewDownloader(dir)
	d.freeBytes = func(string) (uint64, error) { return 1 << 40, nil }

	_, err := d.Download(context.Background(), Asset{
		Name:               "wassap-desktop-windows-amd64.exe",
		BrowserDownloadURL: srv.URL + "/asset",
		Digest:             "sha256:" + hexSum([]byte("different bytes")),
	}, nil)
	if err == nil {
		t.Fatal("digest mismatch must be an error")
	}
	assertDirEmpty(t, dir)
}

func TestDownloadTruncatedBodyRemovesPartFile(t *testing.T) {
	dir := t.TempDir()
	half := payload[:len(payload)/2]
	// Server promises full length, serves only half.
	srv := newDownloadServer(t, half, int64(len(payload)), "")
	defer srv.Close()

	d := NewDownloader(dir)
	d.freeBytes = func(string) (uint64, error) { return 1 << 40, nil }

	_, err := d.Download(context.Background(), Asset{
		Name:               "wassap-desktop-windows-amd64.exe",
		BrowserDownloadURL: srv.URL + "/asset",
		Digest:             digestOf(payload),
	}, nil)
	if err == nil {
		t.Fatal("truncated body must be an error")
	}
	assertDirEmpty(t, dir)
}

func TestDownloadWithoutDigestSkipsVerification(t *testing.T) {
	dir := t.TempDir()
	srv := newDownloadServer(t, payload, int64(len(payload)), "")
	defer srv.Close()

	d := NewDownloader(dir)
	d.freeBytes = func(string) (uint64, error) { return 1 << 40, nil }

	res, err := d.Download(context.Background(), Asset{
		Name:               "wassap-desktop-windows-amd64.exe",
		BrowserDownloadURL: srv.URL + "/asset",
		Digest:             "",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA256 != hexSum(payload) {
		t.Errorf("SHA256 should still be computed for the caller: %q", res.SHA256)
	}
}

func TestDownloadRefusedWhenDiskTooSmall(t *testing.T) {
	dir := t.TempDir()
	srv := newDownloadServer(t, payload, int64(len(payload)), "")
	defer srv.Close()

	d := NewDownloader(dir)
	d.freeBytes = func(string) (uint64, error) { return 1, nil } // 1 byte free

	_, err := d.Download(context.Background(), Asset{
		Name:               "wassap-desktop-windows-amd64.exe",
		BrowserDownloadURL: srv.URL + "/asset",
		Digest:             digestOf(payload),
	}, nil)
	if err == nil {
		t.Fatal("insufficient disk space must be refused")
	}
	assertDirEmpty(t, dir)
}

func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty dir, found: %v", entries)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	bare := hexSum(payload)
	for _, want := range []string{bare, "sha256:" + bare} {
		if err := VerifyFileSHA256(path, want); err != nil {
			t.Errorf("VerifyFileSHA256(%q) = %v, want nil", want, err)
		}
	}
	if err := VerifyFileSHA256(path, "sha256:"+hexSum([]byte("other"))); err == nil {
		t.Error("mismatch must be an error")
	}
	if err := VerifyFileSHA256(path, "sha256:zzzznothex"); err == nil {
		t.Error("garbage digest must be an error, not a panic")
	}
}

