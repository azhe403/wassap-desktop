package app

import "wassap-desktop/internal/updater"

// shouldAutoDownload decides whether an available update may be fetched
// without further user action. The user must have opted in, the release must
// actually be available, and the user must not have dismissed this version.
func shouldAutoDownload(autoUpdate bool, st updater.Status) bool {
	return autoUpdate && st.State == updater.StateAvailable && !st.Dismissed
}

// resetDownloadStateIfStale drops the staged in-memory download state when a
// fresher check resolved a *different* latest version than the one staged.
// Without this, clicking "Restart to update" under a "v0.3.0 downloaded"
// label would install the previously staged v0.2.0. Mid-flight downloads and
// states missing a staged-version identity are left alone.
func resetDownloadStateIfStale(dl dlDownloadState, newLatest string) (dlDownloadState, bool) {
	if dl.State != "downloaded" || dl.StagedVersion == "" || newLatest == "" {
		return dl, false
	}
	if dl.StagedVersion == newLatest {
		return dl, false
	}
	return dlDownloadState{}, true
}
