package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")

	checkedAt := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	in := UpdateCheckCache{
		CheckedAt:        checkedAt,
		Channel:          "nightly",
		CurrentVersion:   "0.0.0-nightly.abc1234",
		LatestVersion:    "nightly (latest main)",
		UpdateAvailable:  true,
		ReleaseURL:       "https://github.com/azhe403/wassap-desktop/releases/tag/nightly",
		StagedAsset:      "wassap-desktop-windows-amd64.exe",
		StagedDigest:     "sha256:deadbeef",
		DismissedVersion: "v0.2.0",
	}

	c := &Cache{Path: path}
	if err := c.Store(in); err != nil {
		t.Fatalf("Store: %v", err)
	}
	out, err := c.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !out.CheckedAt.Equal(checkedAt) {
		t.Errorf("CheckedAt = %v, want %v", out.CheckedAt, checkedAt)
	}
	if out.Channel != in.Channel || out.CurrentVersion != in.CurrentVersion ||
		out.LatestVersion != in.LatestVersion || out.UpdateAvailable != in.UpdateAvailable ||
		out.ReleaseURL != in.ReleaseURL || out.StagedAsset != in.StagedAsset ||
		out.StagedDigest != in.StagedDigest || out.DismissedVersion != in.DismissedVersion {
		t.Errorf("round trip mismatch:\ngot  %+v\nwant %+v", out, in)
	}
}

func TestCacheMissingFileReturnsZeroValue(t *testing.T) {
	c := &Cache{Path: filepath.Join(t.TempDir(), "nope.json")}
	got, err := c.Load()
	if err != nil {
		t.Fatalf("missing file must not error, got: %v", err)
	}
	if got != (UpdateCheckCache{}) {
		t.Errorf("missing file must degrade to zero value, got %+v", got)
	}
}

func TestCacheCorruptFileReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	if err := os.WriteFile(path, []byte(`{"nope`), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Cache{Path: path}
	got, err := c.Load()
	if err != nil {
		t.Fatalf("corrupt file must not error, got: %v", err)
	}
	if got != (UpdateCheckCache{}) {
		t.Errorf("corrupt file must degrade to zero value, got %+v", got)
	}
}

func TestCacheStoreLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := &Cache{Path: path}
	if err := c.Store(UpdateCheckCache{Channel: "stable"}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || len(e.Name()) > len("update-check.json") && e.Name()[:len("update-check.json")] == "update-check.json" && e.Name() != "update-check.json" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}

func TestCacheStoreOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update-check.json")
	c := &Cache{Path: path}
	if err := c.Store(UpdateCheckCache{Channel: "stable", UpdateAvailable: false}); err != nil {
		t.Fatal(err)
	}
	if err := c.Store(UpdateCheckCache{Channel: "nightly", UpdateAvailable: true}); err != nil {
		t.Fatal(err)
	}
	got, err := c.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Channel != "nightly" || !got.UpdateAvailable {
		t.Errorf("second store did not overwrite: %+v", got)
	}
}

func TestDismissedMatchesVersion(t *testing.T) {
	c := UpdateCheckCache{DismissedVersion: "v0.2.0"}
	if !c.Dismissed("v0.2.0") {
		t.Error("dismissed version must match")
	}
	if c.Dismissed("v0.3.0") {
		t.Error("a different version must not be dismissed")
	}
	if c.Dismissed("") {
		t.Error("empty version must not be dismissed")
	}
}

func TestUpdateCheckCacheJSONFieldNames(t *testing.T) {
	// The cache format is consumed by future humans debugging issues; keep the
	// wire shape stable and snake_case-free (camelCase, matching config.json).
	data, err := json.Marshal(UpdateCheckCache{CheckedAt: time.Unix(0, 0).UTC(), Channel: "stable", DismissedVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"checkedAt", "dismissedVersion", "channel"} {
		if _, ok := m[want]; !ok {
			t.Errorf("expected camelCase key %q in %s", want, data)
		}
	}
}
