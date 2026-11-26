# Upstream sync - whatsapp-privacy-blur engine

**Maintainer's guide (architecture, adapter contract, full re-sync runbook): `../../docs/privacy-blur-engine.md`.**

- Source: https://github.com/Nyihui/WhatsappPrivacyBlur-Extension
- Vendored: v3.2.0 @ 82cc20e (2026-11-14), MIT — see LICENSE in this directory.
- Vendored verbatim: `upstream/selectors/*.js` (9 files), `upstream/content.js`, `upstream/LICENSE`.
- Adaptation lives only in `../adapter.js` (our `chrome.*` shim). Never patch the upstream files; bump them by re-copying.
- Re-sync: clone upstream, checkout the tag/commit to vendor, copy the files listed above, update the version line here, run
  `go test ./internal/privacyblur/`, fix only `adapter.js` if the API surface the engine uses changed (see content.js bootstrap),
  then verify `manifestOrder` in `privacyblur.go` still matches upstream's `manifest.json` `content_scripts[0].js` array (order is
  asserted only by the sentinel test for content.js; selectors' relative order is not).
- Repo note: this repository has no `.gitattributes` and `core.autocrlf=true`, so vendored files check out with CRLF on Windows.
  That is safe for the JS engine and for the embed tests (nothing byte-hashes vendored files); keep vendored LF/CRLF as-is when re-copying.
- Upstream features intentionally NOT vendored: `ui/` (in-page quick settings panel), `minimizer/`, `icons/`, `manifest.json`, `build.js`, shortcut rebinding UI.
