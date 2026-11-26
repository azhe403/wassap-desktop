package updater

import (
	"context"
	"os"
	"testing"
	"time"
)

var wantStablePublish = time.Date(2026, 11, 15, 6, 3, 19, 0, time.UTC)
var wantNightlyPublish = time.Date(2026, 11, 15, 7, 0, 0, 0, time.UTC)

func TestStableCheckCarriesPublishTime(t *testing.T) {
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 7}}
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC))

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LatestBuildDate.Equal(wantStablePublish) {
		t.Errorf("LatestBuildDate = %v, want %v", st.LatestBuildDate, wantStablePublish)
	}
}

func TestNightlyCheckCarriesPublishTime(t *testing.T) {
	f := &fakeGitHub{nightlyJSON: nightlyRel, compareAheadByOf: map[string]int{"main": 3}}
	c, _, _ := newChecker(t, f, "0.0.0-nightly.old", "oldcommit", time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC))

	st, err := c.Check(context.Background(), Prefs{Channel: "nightly"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LatestBuildDate.Equal(wantNightlyPublish) {
		t.Errorf("LatestBuildDate = %v, want %v", st.LatestBuildDate, wantNightlyPublish)
	}
}

func TestSnapshotKeepsPublishTime(t *testing.T) {
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 7}}
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC))
	if _, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false); err != nil {
		t.Fatal(err)
	}
	snap := c.Snapshot()
	if !snap.LatestBuildDate.Equal(wantStablePublish) {
		t.Errorf("Snapshot.LatestBuildDate = %v, want %v", snap.LatestBuildDate, wantStablePublish)
	}
}

func TestFailedCheckPreservesPublishTime(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	f := &fakeGitHub{latestCode: 500, compareCode: 500}
	c, cachePath, _ := newChecker(t, f, "0.1.0", "oldcommit", now)
	cache := &Cache{Path: cachePath}
	if err := cache.Store(UpdateCheckCache{
		CheckedAt:       now.Add(-13 * time.Hour),
		LastAttempt:     now.Add(-13 * time.Hour),
		Channel:         "stable",
		UpdateAvailable: true,
		LatestVersion:   "v0.2.0",
		PublishedAt:     wantStablePublish,
	}); err != nil {
		t.Fatal(err)
	}

	st, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LatestBuildDate.Equal(wantStablePublish) {
		t.Errorf("failed check lost the known publish time: %v, want %v", st.LatestBuildDate, wantStablePublish)
	}
}

func TestCacheRoundTripIncludesPublishedAt(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: dir + "/update-check.json"}
	in := UpdateCheckCache{Channel: "stable", LatestVersion: "v0.2.0", PublishedAt: wantStablePublish}
	if err := c.Store(in); err != nil {
		t.Fatal(err)
	}
	got, err := c.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.PublishedAt.Equal(wantStablePublish) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, wantStablePublish)
	}
}

func TestCacheWithoutPublishedAtIsZeroNotError(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/update-check.json"
	// A cache written by the previous version has no publishedAt key.
	if err := os.WriteFile(path, []byte(`{"checkedAt":"2026-11-15T12:00:00Z","channel":"stable","latestVersion":"v0.2.0","updateAvailable":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (&Cache{Path: path}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if !got.PublishedAt.IsZero() {
		t.Errorf("missing publishedAt must stay zero, got %v", got.PublishedAt)
	}
}
