# Wassap Desktop Privacy Blur - Design

**Date:** 2026-11-16
**Status:** Approved by user 2026-11-16 ("Gas, approve", design-gate page). Awaiting implementation via plan.
**Scope:** `internal/privacyblur` (new package, vendored engine), `internal/config` (new section + store methods), `app.go` (bindings + startup hook), `app_other.go` (stub), `internal/platform/windows/webview2` (prelude hook, `EvalAll`, `privacy-blur-set` message kind), frontend (Settings section + sidebar shield toggle + state event), `README.md` (credits), `docs` (this file).

---

## 1. Intent & Why

The Firefox add-on **WhatsApp Privacy Blur** (`addons.mozilla.org/en-US/firefox/addon/whatsapp-privacy-blur/`) blurs sensitive WhatsApp Web content with hover-to-reveal: names, photos, previews, message text, media. The user asked to adopt it into Wassap Desktop ("adopsi", with credits in the README). Source of truth for the engine: `github.com/Nyihui/WhatsappPrivacyBlur-Extension` (MIT).

Scope decisions (user, 2026-11-16):

- **Engine + app-native controls.** The blur engine is adopted. The controls live in Wassap Desktop's own chrome: a **Privacy Blur** section in Settings and a shield toggle in the sidebar. No in-page floating shield button, no in-page quick-settings panel.
- **No sidebar minimizer.** The add-on's `Alt+.` chat-list collapse is not ported.
- **Credits in README** (mandated by the user; MIT requires retaining the notice anyway).

The blur itself is delivered as an injected script into the native WebView2 sessions, the exact analog of a browser content script: WebView2's `AddScriptToExecuteOnDocumentCreated` (go-webview2 `Chromium.Init`, already used by the app to tag notifications with the account label) and `ExecuteScript` (`Chromium.Eval`, fire-and-forget, available for live updates).

## 2. Goals / Non-goals / Anti-goals

**Goals**

- Blur/redact, with hover-to-reveal, for: profile pictures and status rings; contact and group names; chat-list previews; message text in the open conversation; stickers; media previews (images/videos/documents); the fullscreen media gallery; the text input (opacity instead of blur).
- Master on/off switch; per-target toggles; adjustable blur strength; **Blur** vs **Redacted** (solid black) mode; animated hover transition.
- **Unblur last N messages** (1-10) in the active chat, matching the add-on.
- In-page `Alt+/` master toggle that stays in sync with the app's persisted config.
- Settings persist globally in `config.json` and apply to all sessions immediately (live via `ExecuteScript`) and to every session opened later (via the document-created prelude).

**Non-goals**

- The add-on's sidebar minimizer (`Alt+.`). Skipped by user decision 2026-11-16.
- The add-on's in-page quick-settings panel and floating shield button. Replaced by app-native Settings + sidebar toggle.
- A shortcut-rebinding UI. `Alt+/` is fixed in v1.
- Per-account privacy settings. One global profile for all accounts.
- Non-Windows paths (the app itself is Windows-only today; the stub keeps other platforms building).

**Anti-goals**

- Zero network. The engine is 100% local; nothing is sent anywhere.
- No behavioral fork of the upstream engine. Upstream files are vendored **byte-identical**; all adaptation happens in one adapter script of our own. Upstream selector/engine fixes must be re-appliable by replacing files verbatim.
- No surprise blur on first run: the master switch **defaults to OFF** (my call; see §8).
- No layout changes to the WhatsApp page beyond what the upstream engine does.

## 3. Constraints

- Sessions are created by `internal/platform/windows/webview2`; the only injection points are `Chromium.Init` (document-created) and `Chromium.Eval` (live). Confirmed in go-webview2 v1.0.22 (`pkg/edge/chromium.go`): `Init` calls `AddScriptToExecuteOnDocumentCreated(script, nil)`, `Eval` calls `ExecuteScript(script, nil)`.
- Document-created scripts must carry the boot settings themselves; live changes ride `Eval`.
- WhatsApp Web is an SPA; the upstream MutationObserver design handles in-page navigation. Init runs on every document creation, which covers full reloads.
- WebView2 pages already have `window.chrome` (with `chrome.webview` for postMessage). The adapter **augments** it; it must never replace `window.chrome.webview`.
- Frontend is vanilla JS under `frontend/src/`, no test runner; Go has `go test`.
- MIT: the upstream license text is vendored alongside the engine files and referenced from the README.

## 4. Approaches

| # | Approach | Trade-off | Verdict |
|---|----------|-----------|---------|
| A | **Vendor the upstream engine byte-identical + one adapter** that fakes the tiny `chrome.*` API surface the engine uses (`chrome.storage.local.get/set`, `chrome.storage.onChanged`, `chrome.runtime.onMessage`). Settings live in the app config; the adapter bridges them. | Zero drift from upstream; re-syncing = copying files over; ~80 lines of our code. Requires the fake `chrome` to be airtight against `window.chrome.webview`. | **Chosen** |
| B | Rewrite the blur engine natively | Full control; but the selector library (`selectors/*.js`, ~15 KB) is the fragile, most-maintained part and would rot in a fork nobody upstream. | Rejected |
| C | Patch upstream files to remove `chrome.*` | Smaller adapter, but every upstream update becomes a merge conflict, violating the anti-goal above. | Rejected |

### 4.1 How the vendored engine is wired (Option A)

One concatenated document-created script, built by `internal/privacyblur`, injected via a single `c.Init(...)` right after the existing account-label injection and before navigation:

1. **Boot prelude** (ours): `window.__WA_PRIVACY_BOOT = <json.Marshal(settings)>;` followed by the adapter:
   - augments `window.chrome` with a fake `chrome.storage.local` whose `get(defaults, cb)` returns `Object.assign({}, defaults, window.__WA_PRIVACY_BOOT)` and whose `set(obj, cb)` merges `obj` into the boot object, fires registered `onChanged` listeners, and posts `{kind:'privacy-blur-set', values: obj}` to the host via `window.chrome.webview.postMessage`;
   - fake `chrome.storage.onChanged.addListener` / `chrome.runtime.onMessage.addListener` (the latter is a no-op sink; nothing publishes to it in v1);
   - defines `window.__wassapPrivacyApply = function (settingsJson) { ... }` which parses the JSON, merges into the boot object, and applies via the global `applySettings(...)` the upstream content.js declares (function declarations in classic scripts are globals);
   - defines `window.__wassapPrivacyGet()` returning the current boot object (used by debugging only).
2. **Upstream files, byte-identical, in manifest order**: `selectors/index.js`, `selectors/avatars.js`, `selectors/names.js`, `selectors/text.js`, `selectors/previews.js`, `selectors/input.js`, `selectors/media-preview.js`, `selectors/media-gallery.js`, `selectors/misc.js`, `content.js`.
3. Explicitly **not** vendored: `ui/*` (in-page panel), `minimizer/*`, `icons/*`, `manifest.json`, `build.js`, tests. `content.js` references none of these at runtime except `window.WA_BLUR_RULES` and `window.WAPanel`, both provided by the selectors files.

**Reload drift (builder-check finding, my call):** `AddScriptToExecuteOnDocumentCreated` bakes the boot JSON at session creation, so a full page load inside a long-lived session (hard reload, renderer crash recovery) would re-seed from stale settings. Fix: the existing `NavigationCompletedCallback` in `setUpWebView` additionally calls `h.EvalAll(privacyblur.ApplyScript(freshJSON))` after every navigation. The baked prelude still matters (first paint, before any navigation event); the navigation re-apply closes the drift. Cosmetic cost: a brief unblurred flash on hard reload, accepted.

The upstream `content.js` keyboard handler flips `enabled` via `chrome.storage.local.set({enabled: ...})`; our fake `set` posts `{kind:'privacy-blur-set', values:{enabled: ...}}` to the host. The host (§4.3) persists it and broadcasts the full merged settings back to every session via `Eval` → `__wassapPrivacyApply`. The initiating page re-applies the same value; `applySettings` is idempotent (guarded by its own change detection).

### 4.2 Config

`config.AppConfig` gains one section:

```json
"privacyBlur": {
  "enabled": false,
  "privacyMode": "blur",          // "blur" | "redacted"
  "blurAmount": 3,                // px, 1..10
  "animation": true,
  "animationDuration": 0.25,
  "inputOpacity": 30,             // 0..100
  "unblurLastN": false,
  "unblurLastNCount": 3,          // 1..10
  "blurAvatars": true,
  "blurNames": true,
  "blurPreviews": true,
  "blurTextChats": true,
  "blurStickers": true,
  "blurMediaPreview": true,
  "blurMediaGallery": true,
  "blurInput": true
}
```

- `*PrivacyBlurConfig` pointer on `AppConfig`, `json:"privacyBlur,omitempty"`; nil means "never written" → effective defaults (all per-target ON, `enabled` false, mode `blur`).
- `Store` interface gains `GetPrivacyBlurSettings() (*PrivacyBlurConfig, error)`, `SetPrivacyBlurSettings(cfg PrivacyBlurConfig) error` (full-object replace, validates+clamps), and `MergePrivacyBlurSettings(patch map[string]any) (*PrivacyBlurConfig, error)` (applies recognized keys atomically under `update()`; last-writer-wins per key, never clobbers untouched keys). `JSONStore` implements all three (atomic write, same as existing setters). Validation: clamps blurAmount 1..10, inputOpacity 0..100, unblurLastNCount 1..10, privacyMode ∈ {blur, redacted}; unknown patch keys are ignored.
- Keys intentionally mirror the upstream `DEFAULT_SETTINGS` names minus drops (`minimizerEnabled`, `shortcutToggleMinimizer`, `shortcutToggleShield` rebinding metadata), so re-syncing semantics stays obvious.

### 4.3 Host: prelude hook + live broadcast

`internal/platform/windows/webview2` changes:

- `Host` gains: `Prelude func() string` (set by the app before any session opens; returning "" disables injection), `Apply func() string` (returns a fresh `ExecuteScript` payload that re-applies the current settings; called after each navigation to close the document-created staleness gap), and `EvalAll(script string)` which marshals `chromium.Eval(script)` through each session's existing `s.call` so the call lands on the session thread, fire-and-forget with the same timeout semantics as everything else.
- `setUpWebView`: after `injectAccountLabel` / `wireNotificationClickMessage` and before `c.Navigate`, if `h.Prelude != nil`, build the prelude and `c.Init(prelude)`. Failure is logged and never fails session setup (privacy is best-effort, like the label). `NavigationCompletedCallback` additionally re-applies: if `h.Apply != nil`, call `s.chromium.Eval(h.Apply())` (the callback already runs on the session's UI thread, so no marshalling is needed; `ExecuteScript` is queued asynchronously). Other sessions are unaffected — only the navigated session re-seeds.
- `wireNotificationClickMessage` (kept) and a sibling `wirePrivacyBlurMessage(s, c)` share the `MessageCallback` plumbing: messages with `kind == "privacy-blur-set"` call a new `Host.OnPrivacySettingsWrite func(values map[string]any)` hook. Everything else is ignored as today. (One callback slot exists on `Chromium`; the wire function simply routes both kinds. If `OnPrivacySettingsWrite` is nil, the kind is dropped.)

`HostController` in `app.go` gains `EvalAll(script string)`; `app_other.go` gains the matching no-op.

### 4.4 App bindings & event

`App` in `app.go`:

- New field `privacyBlur *config.PrivacyBlurConfig` (loaded at startup), guarded by the existing config store's own locking.
- Startup wiring: `host.Prelude = func() string { return privacyblur.PreludeScript(a.currentPrivacyBlurJSON()) }` (set before any session is opened; sessions read the JSON at creation), and `host.OnPrivacySettingsWrite = a.handlePrivacyBlurWrite`.
- `handlePrivacyBlurWrite(values map[string]any)`: merge recognized keys into the config struct, save, then `EvalAll(privacyblur.ApplyScript(json))` to re-broadcast the full object to every session, then emit Wails event **`privacy-blur:changed`** with the merged settings. Panics recovered; errors logged.
- New Wails bindings:
  - `GetPrivacyBlurSettings() config.PrivacyBlurConfig` — returns the effective config (defaults filled when never written).
  - `SetPrivacyBlurSettings(cfg config.PrivacyBlurConfig) error` — **full-object replace**, not a merge patch: the frontend always round-trips the whole struct it got from `Get` (absent JS fields would otherwise decode as zero values), the backend validates + clamps, persists atomically, then broadcasts via `EvalAll` and emits `privacy-blur:changed`.
- `handlePrivacyBlurWrite(values map[string]any)` (in-page `Alt+/` path) does **not** read-merge-write across store calls. The `Store` interface gains a third method, `MergePrivacyBlurSettings(patch map[string]any) (*PrivacyBlurConfig, error)`, which applies recognized keys under the store's `update()` lock (clamping as in Set) and returns the merged config. Two writes racing (Alt+/ in one session vs. a Settings-page save, or two Alt+/ presses) serialize instead of clobbering. Unrecognized keys are ignored.

### 4.5 Frontend

- `frontend/src/components/settings-page.js`: new **Privacy Blur** section between *General* and *Software Update*. Rows (existing `settings-toggle-row` / `settings-select` / slider patterns):
  1. Master **Privacy Blur** toggle (`setting-privacy-blur-enabled`) with desc: "Blur chats and contacts until you hover them. Applies to every account. `Alt+/` toggles in-page."
  2. **Mode**: segmented control `Blur` / `Redacted` (`privacyMode`).
  3. **Blur strength**: range 1-10 px (`blurAmount`).
  4. Per-target toggles (face the master when OFF): Chat list names & photos (`blurAvatars`, `blurNames`), Chat list previews (`blurPreviews`), Message bubbles (`blurTextChats`), Stickers (`blurStickers`), Media previews (`blurMediaPreview`), Media gallery (`blurMediaGallery`), Typing field opacity (`blurInput` + `inputOpacity` 0-100 slider, shown inline).
  5. **Keep last N messages visible**: toggle (`unblurLastN`) + select 1-10 (`unblurLastNCount`).
  6. Footer note: "Changes apply immediately to open sessions. Everything stays on this device."
  - All rows disabled visually when the master is OFF (dim, `pointer-events: none` on the group), matching how the master governs.
- `frontend/src/components/sidebar.js`: a **shield** button in `.sidebar-footer` above the add/settings row, inline SVG (stroke-based, `currentColor`), `aria-pressed`, `.active` state styling in `app.css` (accent foreground + tinted background). Click → toggles master via the same binding path as the Settings checkbox. `title` reflects state ("Privacy Blur: on" / "off").
- `frontend/src/main.js`: `EventsOn('privacy-blur:changed', render shield state + settings rows if present)`; sidebar button calls the toggle path; initial state fetched via `GetPrivacyBlurSettings()` after bindings are ready; `renderSidebar` call sites pass shield state handlers.
- `frontend/src/app.css`: `.sidebar-shield-btn` (+ `.active`), `settings-privacy-*` styles consistent with existing tokens.
- Shortcut legend: nothing new on screen; `Alt+/` documented in the section's master-row description.

### 4.6 Packaging, license, credits

- New package `internal/privacyblur` with `assets/upstream/` holding the nine upstream files **byte-identical** (`selectors/*.js`, `content.js`) plus the upstream `LICENSE`, and `adapter.js` (ours). `go:embed`s them; `PreludeScript(settingsJSON string)` and `ApplyScript(settingsJSON string)` are pure functions of the JSON.
- A golden check in `internal/privacyblur/upstream_test.go` asserts the vendored files are non-empty and contain expected sentinel strings (e.g. `window.WAPanel.DEFAULT_SETTINGS` in content.js, `WA_BLUR_RULES` in selectors/index.js); a comment block at the top of `assets/upstream/` (separate `SYNC.md`) records the upstream repo, commit hash vendored, and the re-sync procedure (replace files, re-run tests, bump the hash).
- `README.md` gains a **Credits** section: name, link to the add-on page and the source repository, MIT license note, "vendored unmodified under `internal/privacyblur/assets/upstream/`".
- No icons are copied (UI is native; the shield is drawn as SVG).

## 5. Data flow (happy path)

1. Session setup: WebView2 page created → document-created script runs → adapter seeds `chrome.*` fakes from `window.__WA_PRIVACY_BOOT` → upstream engine reads storage → `applySettings` paints blur CSS before first meaningful frame.
2. User clicks sidebar shield: frontend flips `enabled` on the full settings object it loaded via `GetPrivacyBlurSettings()` → `SetPrivacyBlurSettings(fullObject)` → config saved → `EvalAll("window.__wassapPrivacyApply(...)")` → every open session's DOM updates live; Wails event syncs all windows (settings page if open, sidebar icon).
3. User hits `Alt+/` inside a session: engine flips `enabled` → fake `set` → postMessage `privacy-blur-set` → `MergePrivacyBlurSettings` (atomic in-store) → host re-broadcasts the merged object to sessions + emits event → sidebar and Settings stay truthful.
4. User hard-reloads a session (Ctrl+R / crash recovery): the new document boots from `__WA_PRIVACY_BOOT` (current at session start) — if settings changed since, `NavigationCompletedCallback` immediately re-applies the fresh object via `Eval(ApplyScript(...))`.

## 6. Testing & verification

- `internal/privacyblur`: unit tests — Prelude includes every upstream file in order; settings JSON is HTML/JS-escaped safely (`json.Marshal` + `<` → `\u003c` is already what `encoding/json` does); ApplyScript shape; SYNC.md sentinel check.
- `internal/config`: roundtrip + clamp tests for the new section; nil-section default behavior; corrupt-file behavior unchanged; `MergePrivacyBlurSettings` — partial patches keep untouched keys, clamp out-of-range values, serialize concurrent merges (race test with `-race`).
- `go build ./...`, `go vet ./...`, `go test ./...` green.
- Frontend has no runner; manual QA checklist (also in the PR description): login fresh → no blur with master OFF; enable via sidebar → blur appears immediately; hover reveals each target class; `Alt+/` toggles; mode Redacted blacks out; unblur last N keeps newest bubbles visible; restart app → state persisted; Settings rows disabled when master OFF; no `[WebView2 Error]` in the log.

## 7. Size & risk

A single coherent feature, roughly 10-14 files, no new dependencies, one new package. Main risk is **selector rot** (WhatsApp changes its DOM and blur rules silently stop matching) — accepted upstream reality; the fix is re-syncing `assets/upstream/` from the MIT repo, which the design keeps trivial.

## 8. Open calls made (my call, correct me)

- Master default OFF. The add-on defaults ON; a desktop client changing the user's screen uninvited is worse than one click to enable. Sidebar placement keeps that click cheap.
- Global setting across all accounts (matches how every other app setting works here).
- `Alt+/` fixed, not rebindable in v1.
- English Settings labels, matching the rest of the Settings page.
- Redacted mode kept (free once the engine is in).
- In-page panel/floating shield/minimizer not ported (user decision, 2026-11-16).
- Privacy controls do not get their own sidebar entry; they live in Settings + one sidebar footer button.
- `SetPrivacyBlurSettings` is a full-object replace; the frontend always round-trips the struct from `Get`. (Builder-check finding; alternative merge-patch was rejected — absent JS fields would zero out settings silently.)
- In-page write path merges via a new atomic `MergePrivacyBlurSettings` store method instead of read-modify-write across calls. (Builder-check finding; otherwise concurrent Alt+/ and Settings saves clobber each other.)
- Navigation re-apply via `NavigationCompletedCallback` closes the document-created staleness gap; the baked boot prelude stays for first paint. (Builder-check finding.) Upstream vendored at v3.2.0 @ `82cc20e0`; `SYNC.md` records it. Real-WebView2 test suites are flaky on this machine, so tests stay sentinel/deterministic — no gating on spinning a live engine.

## 9. Out of scope / follow-ups

- Shortcut rebinding UI; per-account overrides; sync of upstream re-releases into CI (watch the upstream repo instead); porting the quick-settings panel if the user later wants it.
- `app_other.go` gets compile-only stubs, no behavior.
