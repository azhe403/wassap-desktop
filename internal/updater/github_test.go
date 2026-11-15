package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const releaseJSON = `{
	"tag_name": "v0.2.0",
	"html_url": "https://github.com/azhe403/wassap-desktop/releases/tag/v0.2.0",
	"published_at": "2026-11-15T06:03:19Z",
	"prerelease": false,
	"assets": [
		{"name": "wassap-desktop-windows-amd64.exe", "browser_download_url": "https://example.com/wd-amd64.exe", "digest": "sha256:aaa111"},
		{"name": "wassap-desktop-windows-arm64.exe", "browser_download_url": "https://example.com/wd-arm64.exe", "digest": "sha256:bbb222"},
		{"name": "wassap-desktop-linux-amd64", "browser_download_url": "https://example.com/wd-linux", "digest": "sha256:ccc333"}
	]
}`

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestReleaseParsesTagURLAndAssets(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if ua := r.Header.Get("User-Agent"); ua != "wassap-desktop" {
			t.Errorf("User-Agent = %q, want wassap-desktop", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseJSON))
	})
	c := NewClient()
	c.baseURL = srv.URL

	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel == nil {
		t.Fatal("expected release, got nil")
	}
	if rel.TagName != "v0.2.0" {
		t.Errorf("tag = %q", rel.TagName)
	}
	if rel.HTMLURL != "https://github.com/azhe403/wassap-desktop/releases/tag/v0.2.0" {
		t.Errorf("url = %q", rel.HTMLURL)
	}
	if len(rel.Assets) != 3 {
		t.Fatalf("assets = %d, want 3", len(rel.Assets))
	}
	if rel.Assets[0].Name != "wassap-desktop-windows-amd64.exe" || rel.Assets[0].Digest != "sha256:aaa111" {
		t.Errorf("asset = %+v", rel.Assets[0])
	}
	if rel.PublishedAt.Year() != 2026 {
		t.Errorf("publishedAt = %v", rel.PublishedAt)
	}
}

func TestLatestRelease404MeansNoStableYet(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	c := NewClient()
	c.baseURL = srv.URL
	rel, err := c.LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("404 must not be an error, got %v", err)
	}
	if rel != nil {
		t.Errorf("404 must yield nil release, got %+v", rel)
	}
}

func TestReleaseByTagParses(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/tags/nightly") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Replace(releaseJSON, `"v0.2.0"`, `"nightly"`, 1)))
	})
	c := NewClient()
	c.baseURL = srv.URL
	rel, err := c.ReleaseByTag(context.Background(), "nightly")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "nightly" {
		t.Errorf("tag = %q", rel.TagName)
	}
}

func TestAheadByParsesCompareResponse(t *testing.T) {
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/compare/") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Path, "abc123...main") {
			t.Errorf("compare path %q must contain base...head", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ahead_by": 42, "status": "ahead"})
	})
	c := NewClient()
	c.baseURL = srv.URL
	n, err := c.AheadBy(context.Background(), "abc123", "main")
	if err != nil {
		t.Fatal(err)
	}
	if n != 42 {
		t.Errorf("ahead_by = %d, want 42", n)
	}
}

func TestNon200IsAnErrorWithStatus(t *testing.T) {
	for _, code := range []int{403, 429, 500, 503} {
		srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		})
		c := NewClient()
		c.baseURL = srv.URL
		if _, err := c.LatestRelease(context.Background()); err == nil {
			t.Errorf("status %d must produce an error", code)
		} else if !strings.Contains(err.Error(), fmt.Sprint(code)) {
			t.Errorf("error %v should carry the status code", err)
		}
	}
}
