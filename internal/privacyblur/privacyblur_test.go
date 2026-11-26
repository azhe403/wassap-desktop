package privacyblur

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreludeContainsAllUpstreamFilesInOrder(t *testing.T) {
	pre := PreludeScript(`{"enabled":true}`)
	need := []string{
		"selectors/index.js", "selectors/avatars.js", "selectors/names.js",
		"selectors/text.js", "selectors/previews.js", "selectors/input.js",
		"selectors/media-preview.js", "selectors/media-gallery.js", "selectors/misc.js",
		"content.js",
	}
	last := -1
	for _, f := range need {
		body, err := os.ReadFile(filepath.Join("assets", "upstream", filepath.FromSlash(f)))
		if err != nil {
			t.Fatalf("vendored file missing: %v", err)
		}
		idx := strings.Index(pre, string(body))
		if idx < 0 {
			t.Fatalf("prelude does not contain %s", f)
		}
		if idx <= last {
			t.Fatalf("prelude order wrong at %s", f)
		}
		last = idx
	}
}

func TestPreludeEmbedsBootJSONBeforeAdapter(t *testing.T) {
	pre := PreludeScript(`{"enabled":true,"blurAmount":7}`)
	boot := strings.Index(pre, `window.__WA_PRIVACY_BOOT = {"enabled":true,"blurAmount":7};`)
	adapter := strings.Index(pre, "window.__wassapPrivacyApply")
	content := strings.Index(pre, "WAPanel = window.WAPanel || {}")
	if boot < 0 {
		t.Fatal("boot JSON line missing")
	}
	if adapter <= boot || content <= adapter {
		t.Fatalf("order must be boot(%d) < adapter(%d) < content(%d)", boot, adapter, content)
	}
}

func TestApplyScriptCarriesCurrentJSON(t *testing.T) {
	got := ApplyScript(`{"enabled":false,"blurAmount":9}`)
	if got != `window.__wassapPrivacyApply({"enabled":false,"blurAmount":9});` {
		t.Fatalf("unexpected apply script: %s", got)
	}
}

// Guard only: the real regression was verified with a browser harness at
// document_start (rAF never fired; adapter primes the initial DOM by timer).
func TestPreludePrimesInitialDOM(t *testing.T) {
	pre := PreludeScript(`{"enabled":true}`)
	if !strings.Contains(pre, "primeInitialDom") || !strings.Contains(pre, "DOMContentLoaded") {
		t.Fatal("prelude lost the startup DOM prime — blur will not appear until a setting is toggled")
	}
}

func TestUpstreamSentinels(t *testing.T) {
	cj, err := os.ReadFile(filepath.Join("assets", "upstream", "content.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cj), "window.WAPanel.DEFAULT_SETTINGS") {
		t.Fatal("content.js sentinel missing — truncated vendor?")
	}
	se, err := os.ReadFile(filepath.Join("assets", "upstream", "selectors", "index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(se), "WA_BLUR_RULES") {
		t.Fatal("selectors/index.js sentinel missing")
	}
}
