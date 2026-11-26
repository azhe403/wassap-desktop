package app

import (
	"testing"
	"time"

	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/updater"
)

// Dismissing a notice must not discard the publish time that the comparison
// row needs: the checker deliberately carries it through cache/error paths.
func TestDismissUpdateNoticePreservesLatestBuildDate(t *testing.T) {
	published := time.Date(2026, 11, 26, 8, 12, 0, 0, time.UTC)
	app := &App{
		dirs: appdirs.Dirs{Cache: t.TempDir()},
		lastStatus: updater.Status{
			State:           updater.StateAvailable,
			Channel:         "nightly",
			CurrentVersion:  "0.0.0-nightly.a1b2c3d",
			LatestVersion:   "v0.2.0",
			LatestBuildDate: published,
		},
	}

	if err := app.DismissUpdateNotice("v0.2.0"); err != nil {
		t.Fatal(err)
	}

	got := app.lastStatusSnapshot()
	if !got.Dismissed {
		t.Error("notice was not dismissed")
	}
	if !got.LatestBuildDate.Equal(published) {
		t.Errorf("LatestBuildDate = %v, want %v", got.LatestBuildDate, published)
	}
	out := statusToUpdateStatus(got)
	want := published.Format(time.RFC3339)
	if out.LatestBuildDate != want {
		t.Errorf("binding LatestBuildDate = %q, want %q", out.LatestBuildDate, want)
	}
}
