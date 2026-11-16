# Upstream sync — whatsapp-privacy-blur engine

- Source: https://github.com/Nyihui/WhatsappPrivacyBlur-Extension
- Vendored: v3.2.0 @ 82cc20e (2026-11-14), MIT — see LICENSE in this directory.
- Vendored verbatim: `upstream/selectors/*.js` (9 files), `upstream/content.js`, `upstream/LICENSE`.
- Adaptation lives only in `../adapter.js` (our `chrome.*` shim). Never patch the upstream files; bump them by re-copying.
- Re-sync: clone upstream, checkout the tag/commit to vendor, copy the files listed above, update the version line here, run
  `go test ./internal/privacyblur/`, and fix only `adapter.js` if the API surface the engine uses changed (see content.js bootstrap).
- Upstream features intentionally NOT vendored: `ui/` (in-page quick settings panel), `minimizer/`, `icons/`, `manifest.json`, `build.js`, shortcut rebinding UI.
