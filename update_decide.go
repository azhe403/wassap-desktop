package main

import "wassap-desktop/internal/updater"

// shouldAutoDownload decides whether an available update may be fetched
// without further user action. The user must have opted in, the release must
// actually be available, and the user must not have dismissed this version.
func shouldAutoDownload(autoUpdate bool, st updater.Status) bool {
	return autoUpdate && st.State == updater.StateAvailable && !st.Dismissed
}
