package avatar

import (
	"strings"
	"testing"
)

func TestDocumentScriptTargetsSelfPhotoAndUsesSpikeTransport(t *testing.T) {
	script := DocumentScript("a01")

	for _, want := range []string{
		`"a01"`,
		`[data-testid="message-yourself-row"]`,
		`avatar-update`,
		`avatar-clear`,
		`cdn\.whatsapp\.net`,
		`pps.whatsapp.net`,
		`static.whatsapp.net`,
		`MutationObserver`,
		`setInterval`,
		`window.chrome.webview.postMessage`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q", want)
		}
	}
}

func TestDocumentScriptSkipsVisibilityGateAndPostsOnlyOnFingerprintChange(t *testing.T) {
	script := DocumentScript("a01")

	if strings.Contains(script, "visibilityState") {
		t.Error("script must not pause for hidden sessions")
	}
	if !strings.Contains(script, "lastUrl !== url") {
		t.Error("script must compare the new URL with the last seen URL before posting")
	}
	if !strings.Contains(script, "lastUrl && !url") {
		t.Error("script must detect a cleared photo (previous URL disappeared)")
	}
	if !strings.Contains(script, `default-contact-refreshed`) || !strings.Contains(script, `default-group-refreshed`) {
		t.Error("script must exclude WA's default silhouette so photoless accounts keep initials")
	}
}
