// Package privacyblur embeds the vendored WhatsApp Privacy Blur engine and
// the Wassap Desktop adapter that bridges it to the app's config store. The
// engine is MIT-licensed upstream; see assets/SYNC.md.
//
// The package is deliberately platform-neutral: it only builds script text.
// The WebView2 host (internal/platform/windows/webview2) decides how and
// when to feed it to the page.
package privacyblur

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed all:assets
var assets embed.FS

// manifestOrder is the load order from upstream manifest.json (document_start).
var manifestOrder = []string{
	"selectors/index.js", "selectors/avatars.js", "selectors/names.js",
	"selectors/text.js", "selectors/previews.js", "selectors/input.js",
	"selectors/media-preview.js", "selectors/media-gallery.js", "selectors/misc.js",
	"content.js",
}

// PreludeScript returns the full engine script for one session: the boot
// settings object, the adapter, then every upstream file in manifest order.
// The WebView2 host evaluates this on NavigationCompleted (not via Init —
// see internal/platform/windows/webview2). settingsJSON must come from
// encoding/json (it escapes <, > and &).
func PreludeScript(settingsJSON string) string {
	var b strings.Builder
	b.WriteString("window.__WA_PRIVACY_BOOT = ")
	b.WriteString(settingsJSON)
	b.WriteString(";\n")
	mustWrite(&b, "assets/adapter.js")
	for _, rel := range manifestOrder {
		b.WriteString("\n/* vendored: " + rel + " */\n")
		mustWrite(&b, "assets/upstream/"+rel)
	}
	return b.String()
}

// ApplyScript returns a live-update payload for Chromium.Eval. It expects
// the page to have been booted with a PreludeScript from this package.
func ApplyScript(settingsJSON string) string {
	return "window.__wassapPrivacyApply(" + settingsJSON + ");"
}

func mustWrite(b *strings.Builder, name string) {
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		panic("privacyblur: embedded asset missing: " + name)
	}
	b.Write(data)
}
