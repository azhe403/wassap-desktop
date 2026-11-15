package updater

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotBeforeAnyCheckIsUnknown(t *testing.T) {
	f := &fakeGitHub{}
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	st := c.Snapshot()
	if st.State != StateUnknown {
		t.Errorf("state = %q, want unknown", st.State)
	}
	if st.CurrentVersion != "0.1.0" {
		t.Errorf("current = %q", st.CurrentVersion)
	}
	if st.Channel != "stable" {
		t.Errorf("channel = %q, want stable", st.Channel)
	}
}

func TestSnapshotAfterSuccessReflectsCache(t *testing.T) {
	f := &fakeGitHub{stableJSON: stableRel, compareAheadByOf: map[string]int{"v0.2.0": 42}}
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c, cachePath, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	if _, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false); err != nil {
		t.Fatal(err)
	}
	_ = cachePath
	snap := c.Snapshot()
	if snap.State != StateAvailable {
		t.Errorf("state = %q, want available", snap.State)
	}
	if snap.Channel != "stable" || snap.ReleaseURL == "" {
		t.Errorf("snapshot should carry cached fields, got %+v", snap)
	}
}

func TestSnapshotAfterFailureCarriesMessage(t *testing.T) {
	f := &fakeGitHub{latestCode: 500, compareCode: 500}
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c, _, _ := newChecker(t, f, "0.1.0", "oldcommit", now)

	if _, err := c.Check(context.Background(), Prefs{Channel: "stable"}, false); err != nil {
		t.Fatal(err)
	}
	snap := c.Snapshot()
	if snap.Message == "" {
		t.Error("snapshot must carry the failure message for the UI")
	}
}
