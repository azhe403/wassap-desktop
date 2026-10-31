//go:build windows

package webview2_test

import (
	"testing"

	"wassap-desktop/internal/browser"
	webview2 "wassap-desktop/internal/platform/windows/webview2"
)

// TestHost_OpenProfileFailsCleanlyWithoutRuntimeStart verifies that the proof
// of concept's session layer reports a failed WebView2 start-up as an error
// instead of exiting the whole process (the default go-webview2 behaviour).
//
// It is a real integration test against the installed WebView2 runtime. It is
// skipped when the runtime is not present.
func TestHost_OpenProfileFailsCleanlyWithoutRuntimeStart(t *testing.T) {
	if _, err := webview2.CheckRuntime(); err != nil {
		t.Skipf("WebView2 runtime unavailable: %v", err)
	}

	// A path the user cannot have: no profile has ever used it, so the
	// environment will refuse to start because the folder is reserved by
	// an in-use engine.
	host := webview2.NewHost("PocProbe", "about:blank")
	probe := browser.Probe{Path: `C:\Program Files\WindowsApps\__RESERVED_BY_POC__`}

	_, err := host.OpenProfile("probe", probe)
	if err == nil {
		t.Fatalf("OpenProfile on a reserved path succeeded; start-up must fail")
	}
	// The error must be a normal Go error, not a process exit. If we are
	// still executing, that means the host did not call os.Exit.
	t.Logf("start-up failed as intended: %v", err)
}
