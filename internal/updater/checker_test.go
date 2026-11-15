package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub routes the three endpoints the checker uses and counts hits.
type fakeGitHub struct {
	hits        *int64
	stableJSON  string
	nightlyJSON string
	latestCode  int
	nightlyCode int
	compareJSON string
	compareCode      int
	compareAheadByOf map[string]int
}

func (f *fakeGitHub) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(f.hits, 1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			if f.latestCode == 404 {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(f.stableJSON))
		case strings.HasSuffix(r.URL.Path, "/releases/tags/nightly"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(f.nightlyJSON))
		case strings.Contains(r.URL.Path, "/compare/"):
			if f.compareCode != 0 && f.compareCode != 200 {
				w.WriteHeader(f.compareCode)
				_, _ = w.Write([]byte(`{"message":"nope"}`))
				return
			}
			head := r.URL.Path[strings.LastIndex(r.URL.Path, "...")+3:]
			n := f.compareAheadByOf[head]
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"ahead_by": %d, "status": "ahead"}`, n)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const stableRel = `{
	"tag_name": "v0.2.0",
	"html_url": "https://github.com/azhe403/wassap-desktop/releases/tag/v0.2.0",
	"published_at": "2026-11-15T06:03:19Z",
	"assets": [
		{"name": "wassap-desktop-windows-amd64.exe", "browser_download_url": "https://example.com/wd-amd64.exe", "digest": "sha256:aaa111"}
	]
}`

const nightlyRel = `{
	"tag_name": "nightly",
	"html_url": "https://github.com/azhe403/wassap-desktop/releases/tag/nightly",
	"published_at": "2026-11-15T07:00:00Z",
	"assets": [
		{"name": "wassap-desktop-windows-amd64.exe", "browser_download_url": "https://example.com/wd-nightly.exe", "digest": "sha256:eee555"}
	]
}`

func newChecker(t *testing.T, f *fakeGitHub, version, commit string, now time.Time) (*Checker, string, *int64) {
	t.Helper()
	hits := new(int64)
	f.hits = hits
	srv := f.server(t)
	dir := t.TempDir()
	cachePath := dir + string(os.PathSeparator) + "update-check.json"
	c := NewChecker(NewClient(), cachePath, version, commit)
	c.client.baseURL = srv.URL
	c.now = func() time.Time { return now }
	return c, cachePath, hits
}

func TestStableAheadOffersUpdate(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, nightlyJSON: nightlyRel, compareAheadByOf: map[string]int{"v0.2.0": 42}}
	c, cachePath, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	st, err := c.Check(context.Background(), Prefs{AutoUpdate: false, Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateAvailable {
		t.Errorf("state = %q, want available (%s)", st.State, st.Message)
	}
	if st.LatestVersion != "v0.2.0" {
		t.Errorf("latest = %q", st.LatestVersion)
	}
	if st.ReleaseURL == "" {
		t.Error("release URL must be carried for the download flow")
	}

	// Successful check rewrites the cache.
	cache := &Cache{Path: cachePath}
	got, err := cache.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdateAvailable || got.Channel != "stable" || !got.CheckedAt.Equal(now) || got.StagedAsset != "" {
		t.Errorf("cache after success = %+v", got)
	}
	if got.LastAttempt.IsZero() || got.LastError != "" {
		t.Errorf("cache must record a successful attempt, got %+v", got)
	}
}

func TestStableNotAheadKeepsQuiet(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, nightlyJSON: nightlyRel, compareAheadByOf: map[string]int{"v0.2.0": 0}}
	c, _, _ := newChecker(t, f, "0.0.0-nightly.abc1234", "abc1234", now) // nightly build that contains the tag

	st, err := c.Check(context.Background(), Prefs{AutoUpdate: false, Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateUpToDate {
		t.Errorf("state = %q, want up-to-date (a nightly containing v0.2.0 must not be offered it)", st.State)
	}
}

func TestNightlyChannelAheadOffers(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, nightlyJSON: nightlyRel, compareAheadByOf: map[string]int{"main": 3}}
	c, _, _ := newChecker(t, f, "0.0.0-nightly.old1234", "old1234", now)

	st, err := c.Check(context.Background(), Prefs{AutoUpdate: false, Channel: "nightly"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateAvailable {
		t.Errorf("state = %q, want available", st.State)
	}
	if !strings.HasPrefix(st.LatestVersion, "nightly ") {
		t.Errorf("latest = %q, want a per-build nightly identity (so dismissals expire on each rebuild)", st.LatestVersion)
	}
}

func TestNightlyChannelNotAheadKeepsQuiet(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, nightlyJSON: nightlyRel, compareAheadByOf: map[string]int{"main": 0}}
	c, _, _ := newChecker(t, f, "0.0.0-nightly.def5678", "def5678", now)

	st, err := c.Check(context.Background(), Prefs{AutoUpdate: false, Channel: "nightly"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateUpToDate {
		t.Errorf("state = %q, want up-to-date", st.State)
	}
}

func TestDevBuildNeverTouchesNetwork(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	for _, stamp := range [][2]string{{"dev", ""}, {"0.1.0", ""}, {"", "abc"}} {
		f := &fakeGitHub{}
		c, _, hits := newChecker(t, f, stamp[0], stamp[1], now)
		st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, true) // even forced
		if err != nil {
			t.Fatal(err)
		}
		if st.State != StateUnsupported {
			t.Errorf("version=%q commit=%q → state = %q, want unsupported", stamp[0], stamp[1], st.State)
		}
		if got := atomic.LoadInt64(hits); got != 0 {
			t.Errorf("unstamped builds must make zero HTTP requests, made %d", got)
		}
	}
}

func TestFreshCacheSkipsNetwork(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 42}}
	c, cachePath, hits := newChecker(t, f, "0.1.0", "oldcommit", now)

	cache := &Cache{Path: cachePath}
	if err := cache.Store(UpdateCheckCache{
		CheckedAt:       now.Add(-1 * time.Hour),
		LastAttempt:     now.Add(-1 * time.Hour),
		Channel:         "stable",
		CurrentVersion:  "0.1.0",
		LatestVersion:   "v0.2.0",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/azhe403/wassap-desktop/releases/tag/v0.2.0",
		DismissedVersion: "v0.2.0",
	}); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(hits) != 0 {
		t.Errorf("fresh cache must skip the network, made %d requests", *hits)
	}
	if !st.Dismissed {
		t.Error("cached dismissal must surface")
	}
	if st.State != StateAvailable {
		t.Errorf("cached availability must surface, got %q", st.State)
	}
}

func TestStaleCacheChecksAndRewrites(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 7}}
	c, cachePath, hits := newChecker(t, f, "0.1.0", "oldcommit", now)

	cache := &Cache{Path: cachePath}
	if err := cache.Store(UpdateCheckCache{CheckedAt: now.Add(-13 * time.Hour), LastAttempt: now.Add(-13 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(hits) == 0 {
		t.Error("stale cache must trigger a network check")
	}
	got, err := cache.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.CheckedAt.Equal(now) {
		t.Errorf("cache CheckedAt = %v, want %v", got.CheckedAt, now)
	}
	_ = st
}

func TestHTTPFailureFallsBackAndBackoffs(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: "", latestCode: 500, compareCode: 500, compareAheadByOf: map[string]int{}}
	c, cachePath, hits := newChecker(t, f, "0.1.0", "oldcommit", now)

	// Seed a prior successful check.
	cache := &Cache{Path: cachePath}
	if err := cache.Store(UpdateCheckCache{
		CheckedAt:       now.Add(-13 * time.Hour),
		LastAttempt:     now.Add(-13 * time.Hour),
		Channel:         "stable",
		UpdateAvailable: true,
		LatestVersion:   "v0.2.0",
	}); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateError {
		t.Errorf("state = %q, want error with cached availability surfaced", st.State)
	}
	if st.LatestVersion != "v0.2.0" {
		t.Errorf("cached latest must surface on error, got %q", st.LatestVersion)
	}
	if st.Message == "" {
		t.Error("message must explain the failure")
	}

	// A second immediate call must not hit the network again (1h backoff).
	before := atomic.LoadInt64(hits)
	if _, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false); err != nil {
		t.Fatal(err)
	}
	if after := atomic.LoadInt64(hits); after != before {
		t.Errorf("failed check must not retry inside 1h: hits %d → %d", before, after)
	}
}

func TestHTTPFailureNoPriorSuccessIsUpToDateWithMessage(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{latestCode: 500, compareCode: 500}
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateUpToDate {
		t.Errorf("state = %q, want up-to-date when nothing is cached", st.State)
	}
	if st.Message == "" {
		t.Error("message must explain the failure")
	}
}

func TestDismissalSurfacesPerVersion(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 42}}
	c, cachePath, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	cache := &Cache{Path: cachePath}
	if err := cache.Store(UpdateCheckCache{LastAttempt: now.Add(-13 * time.Hour), CheckedAt: now.Add(-13 * time.Hour), DismissedVersion: "v0.2.0"}); err != nil {
		t.Fatal(err)
	}
	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Dismissed {
		t.Error("dismissal matching the resolved version must surface")
	}

	// A different dismissed version re-arms the notice.
	if err := cache.Store(UpdateCheckCache{LastAttempt: now.Add(-13 * time.Hour), CheckedAt: now.Add(-13 * time.Hour), DismissedVersion: "v0.3.0"}); err != nil {
		t.Fatal(err)
	}
	st, err = c.Check(context.Background(), Prefs{Channel: "stable"}, true) // force: recheck
	if err != nil {
		t.Fatal(err)
	}
	if st.Dismissed {
		t.Error("dismissal of a different version must not suppress the notice")
	}
}

func TestCheckerAssetFromLastCheck(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 42}}
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	if _, ok := c.Asset("windows", "amd64"); ok {
		t.Error("Asset before any check must report false")
	}
	if _, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false); err != nil {
		t.Fatal(err)
	}
	asset, ok := c.Asset("windows", "amd64")
	if !ok {
		t.Fatal("Asset after a successful stable check must resolve")
	}
	if asset.BrowserDownloadURL != "https://example.com/wd-amd64.exe" || asset.Digest != "sha256:aaa111" {
		t.Errorf("asset = %+v", asset)
	}
	if _, ok := c.Asset("darwin", "universal"); ok {
		t.Error("unsupported platform resolves false")
	}
}
