# Privacy Blur engine — maintainer's guide

A working map of the vendored WhatsApp Privacy Blur engine, our adapter, and the exact
procedure for syncing a new upstream version. Read this before touching anything under
`internal/privacyblur/`.

The engine is MIT-licensed upstream: [WhatsApp Privacy Blur](https://addons.mozilla.org/en-US/firefox/addon/whatsapp-privacy-blur/) by
Nyihui, source at [Nyihui/WhatsappPrivacyBlur-Extension](https://github.com/Nyihui/WhatsappPrivacyBlur-Extension).
The version vendored here is **v3.2.0 @ `82cc20e`** (2026-11-14). Its files are copied
**byte-for-byte** — no edits, no formatting, no "small fixes". All adaptation lives in
our own `assets/adapter.js`.

## 1. What the feature does

A script injected into the WhatsApp Web document blurs sensitive elements until the
pointer hovers over them, using CSS `filter: blur(...)` (or solid black "redacted" mode).
Nothing is read from or sent off the page: the script only adds classes and CSS to the
DOM. Settings live in the app's `config.json`, are global across accounts, and default
to **off**. `Alt+/` toggles the master switch inside the chat.

Native controls live in the app shell, not in the page: the **Privacy Blur** panel in
Settings and the shield button in the sidebar footer. The add-on's own in-page panel,
floating shield button, shortcuts UI and sidebar minimizer are deliberately not ported.

## 2. File map

| File | Responsibility | Sync concern |
|---|---|---|
| `assets/upstream/selectors/index.js` | Creates `window.WA_BLUR_RULES = []`. | Must load first. |
| `assets/upstream/selectors/avatars.js` | `blurAvatars`; profile photos/status rings, `blurMultiplier: 5` on fullscreen overlays; `hoverParentTargets`/`hoverHasTargets`. | Selector-heavy, updated upstream for WhatsApp v2.3000.x. |
| `assets/upstream/selectors/names.js` | `blurNames`; contact/group names, usernames, phone, headers. | Same. |
| `assets/upstream/selectors/text.js` | `blurTextChats`; bubbles, pinned messages, group descriptions, story captions. | Largest rule set; most likely to rot. |
| `assets/upstream/selectors/previews.js` | `blurPreviews`; last-message previews in the chat list. | Same. |
| `assets/upstream/selectors/input.js` | `blurInput`; composer + search bar. Uses `property: 'opacity'`, not blur. | Only rule with a non-filter property. |
| `assets/upstream/selectors/media-preview.js` | `blurMediaPreview`; images, video thumbs, documents, link previews, media hub. | Has `hoverParentTargets`. |
| `assets/upstream/selectors/media-gallery.js` | `blurMediaGallery`; fullscreen media, `blurMultiplier: 5`, story hover rules. | Same. |
| `assets/upstream/selectors/misc.js` | `blurStickers` (`blurMultiplier: 2`) and the `noTransition` special rule. | `special` rules need CSS-builder awareness. |
| `assets/upstream/content.js` | Everything else: defaults, shortcut matcher, CSS generator, DOM observer, unblur-last-N, bootstrap. | Check its `chrome.*` callsites and global functions on every sync. |
| `assets/upstream/LICENSE` | MIT text. | Must stay with the vendor. |
| `assets/adapter.js` (ours) | Fake `chrome.*` API, boot object, live-apply entry point, startup DOM prime. | The only place we adapt upstream. |
| `assets/SYNC.md` (ours) | Pin, license, re-sync notes. | Update version/commit on every sync. |
| `privacyblur.go` | Embeds assets; `PreludeScript(json)` and `ApplyScript(json)`; `manifestOrder`. | If upstream adds/renames a selector file, update `manifestOrder` and the test's file list. |
| `privacyblur_test.go` | Sentinel/order/guard tests over the embedded files. | Extend when the vendored file set changes. |
| `internal/config/config.go` | `PrivacyBlurConfig` with all 16 settings keys, clamping and atomic merge. | New upstream toggle → new key here + Settings row. |
| `internal/platform/windows/webview2/win32_windows.go` | Evaluates the prelude on `NavigationCompleted`; broadcasts live updates via `EvalAll`. | Injection lifecycle; see §5. |
| `app.go` | Wails bindings, `privacy-blur:changed` event, in-page write path. | Nothing upstream-specific here. |
| `frontend/src/components/settings-page.js`, `app.css`, `components/sidebar.js` | Native Settings panel, shield button, state sync. | New upstream toggle → new row. |

## 3. How the engine works

**Rule shape.** Each entry pushed into `window.WA_BLUR_RULES` looks like:

```js
{
  key: 'blurTextChats',          // matches the setting key
  className: 'wa-blur-text',
  label: 'All Messages In Chat',
  property: 'filter',            // or 'opacity' for the typing field
  blurMultiplier: 1,             // multiplies --wa-blur-amount
  targets: [ 'selector | cmd:arg | ...', ... ],
  hoverParentTargets: [ { hoverParent: '...', child: '> img' } ],   // optional
  hoverHasTargets: [ { blurTarget: '...', hoverTrigger: '...' } ],  // optional
}
```

**CSS generation (`buildCSS`).** For every active rule, the upstream generator emits
three groups of selectors: the blurred form, the `:hover` form, and override forms for
elements carrying `.wa-unblur-override`. The blur value is
`var(--wa-blur-amount)` (or `calc(var(--wa-blur-amount) * N)` when a multiplier is set).
Hover restores `filter: none` with an optional transition; `privacyMode: 'redacted'`
swaps in `brightness(0)` + black. The `opacity` rule instead fades to
`var(--wa-input-opacity)` and rises to `1` on hover. Variables are set on
`document.documentElement` by `applySettings`.

**Selector pipeline (`evaluatePipeline`).** A target string is `triggerSelector` plus
`|`-separated commands. Commands:

- `has:> span[...]` / `not-has:...` — existence checks; leading ones are folded into
  native CSS `:has(...)` / `:not(:has(...))` for speed;
- `closest:selector` — move up to the nearest ancestor;
- `up:N` — walk N parents;
- `find:> child` — query descendants from the current element.

Container gates (`CONTAINER_GATES` in content.js) skip queries scoped to drawers/modals
that are not mounted.

**Marker classes.** Pipeline results receive a generated class
`wa-js-target-<key>-<ruleIndex>-<index>`. The CSS rules target that class. Hover wiring
for `hoverHasTargets` attaches `mouseenter`/`mouseleave` listeners that toggle
`.wa-unblur-override` on the corresponding blur targets.

**DOM updates.** A `MutationObserver` on `document.body`/documentElement calls
`scheduleDomUpdates()`, which defers `applyDomUpdates()` to `requestAnimationFrame`.
That runs `applyUnblurLastN()` (marks the newest N `[data-testid="msg-container"]`
messages and grouped stickers with `.wa-unblur-override`) then `applyJsTargets()`.
`manageUnblurObserver()` starts/stops the observer when `enabled` changes.

**Shortcut.** `window.WAPanel.isShortcutMatch` implements modifier combos and sequential
chords. In our build, only `Alt+/` (master toggle) is live. It flips `enabled` through
`chrome.storage.local.set`, which our adapter turns into a host message.

## 4. Our adapter contract

These are the **only** `chrome.*` callsites in upstream v3.2.0 (line numbers in
`assets/upstream/content.js`). On sync, re-run the grep and compare:

| Line | Upstream call | Our implementation |
|---|---|---|
| 273 | `chrome.storage.local.set({ unblurLastNCount: 10 })` | Merges into boot, fires `onChanged`, posts `privacy-blur-set` (a no-op clamp from our UI, terminates harmlessly). |
| 606 | `chrome.storage.local.get(DEFAULT_SETTINGS, applySettings)` | Object-defaults merge: keys come from the defaults object; `boot[key]` wins when present, including `false`/`0`. |
| 609 | `chrome.runtime.onMessage.addListener(...)` | No-op sink (the add-on's popup is not vendored; nothing publishes to it). |
| 617 | `chrome.storage.onChanged.addListener(...)` | Registers a listener; fired by our fake `set`. |
| 619 | `chrome.storage.local.get(...)` again | Same as above. |
| 630 | `chrome.storage.local.set({ enabled: !isEnabled })` | Merges and posts `{kind:'privacy-blur-set', values:{enabled:...}}` via `window.chrome.webview.postMessage`. |

Additional adapter responsibilities:

- defines `window.__WA_PRIVACY_BOOT` from the settings JSON baked into the prelude;
- defines `window.__wassapPrivacyApply(json)` for live updates — parses, merges into
  boot, then calls the global `applySettings(...)` declared by upstream content.js;
- **augments**, never replaces, `window.chrome` — WebView2 owns `chrome.webview`, which
  the notification wrapper also uses;
- primes `applyUnblurLastN()`/`applyJsTargets()` on `DOMContentLoaded` plus 250 ms/1000 ms
  timer passes, because `requestAnimationFrame` can stall before the first composed
  WebView2 frame (see §7);
- wraps every hook in try/catch — a broken privacy layer must never break WhatsApp.

Settings keys (mirroring upstream `DEFAULT_SETTINGS`, clamped in `internal/config`):

`enabled` (bool, default false), `privacyMode` (`blur`|`redacted`), `blurAmount` (1–10),
`animation`, `animationDuration` (0.05–2), `inputOpacity` (0–100), `unblurLastN`,
`unblurLastNCount` (1–10), `blurAvatars`, `blurNames`, `blurPreviews`, `blurTextChats`,
`blurStickers`, `blurMediaPreview`, `blurMediaGallery`, `blurInput` (all bool, default true).

Not vendored: `minimizerEnabled`, `shortcutToggleMinimizer`, `shortcutToggleShield`
rebinding metadata — `Alt+/` is fixed.

## 5. Injection lifecycle in Wassap Desktop

1. `NewApp` installs one host delegate: a `Prelude` closure that reads
   `config.json` fresh on **every** call, plus the in-page write hook. There is
   no `Apply` delegate: live updates are built by the app with `ApplyScript(json)`
   and broadcast through `EvalAll`.
2. `Session.setUpWebView` does **not** call `c.Init` for privacy blur. Registration via
   `AddScriptToExecuteOnDocumentCreated` is asynchronous and lost the race against the
   first `Navigate()`, leaving the page with no engine until a settings toggle fired an
   `Eval`. Instead...
3. `NavigationCompletedCallback` calls `c.Eval(prelude())` for each successful
   navigation. The full engine (adapter + upstream files in manifest order) runs with
   current settings against a complete DOM — upstream's first pass is direct, no rAF.
4. Settings changes in the app call `SetPrivacyBlurSettings` → save → `EvalAll(ApplyScript(json))`
   → the page's `__wassapPrivacyApply` re-applies settings. `broadcastMu` serializes
   broadcasts so two sessions cannot diverge.
5. `Alt+/` in the page → fake `set` → `postMessage` → host merges the patch under the
   store lock → re-broadcast + `privacy-blur:changed` event → sidebar and Settings sync.
6. `__wassapPrivacyApply` never writes back, so step 4 does not loop.

## 6. Re-sync runbook

1. **Fetch upstream.** `git clone https://github.com/Nyihui/WhatsappPrivacyBlur-Extension.git`,
   check out the release/tag you want. Read `CHANGELOG.md`, `manifest.json`, and the
   diff of `selectors/` and `content.js` since the pin recorded in `assets/SYNC.md`.
2. **Copy verbatim.** Replace `selectors/*.js`, `content.js`, `LICENSE` under
   `assets/upstream/`. Never hand-edit these files. Keep the directory layout.
3. **Check the load order.** Upstream `manifest.json` → `content_scripts[0].js` must
   match `manifestOrder` in `privacyblur.go` and the `need` list in
   `privacyblur_test.go` (index.js first, content.js last, selectors in between).
4. **Check the adapter surface.** `grep -n "chrome\." content.js` — every callsite must
   be covered by `assets/adapter.js`. Also check for `window.WAPanel.*` fields and
   globals the adapter calls (`applySettings`, `applyJsTargets`, `applyUnblurLastN`).
   Extend only the adapter; if a call genuinely cannot be shimmed, document it here.
5. **Check settings surface.** New rule `key`s need a matching field in
   `PrivacyBlurConfig` and a row in the Settings → Privacy Blur panel. New `property`
   values (e.g. beyond `filter`/`opacity`) need CSS-builder review. Update
   `Sync.md`'s version line and README Credits.
6. **Test.** `go test ./internal/privacyblur/ ./internal/config/ -count=1`,
   `go build ./...`, and `npm run build` if UI changed. The sentinel/order tests catch
   truncated or reordered vendors; `TestPreludePrimesInitialDOM` catches loss of the
   cold-start prime.
7. **Verify in real WebView2** (no WhatsApp login needed): recreate the throwaway harness —
   a Go test in `internal/platform/windows/webview2` starts an `httptest` server serving
   a minimal DOM such as
   `<div data-testid="msg-container"><div><span data-testid="selectable-text">x</span></div></div>`,
   registers `Host.SetPrivacyDelegate` with a prelude closure returning
   `privacyblur.PreludeScript(<enabled settings JSON>)` and a write hook, opens a
   probe profile, and the page reports computed `getComputedStyle(el).filter`
   back to Go via an image beacon or POST. Expected: class `wa-js-target-blurTextChats-3-0`
   and `filter: blur(3px)` without any manual toggle. Run it **before** deleting the
   harness; it is the only test that exercises the real injection path. (Full run takes
   ~20 s and briefly opens a WebView2 window.)
8. **Commit.** Two logical commits per repo convention: engine/vendor change, then
   adapter/docs/credits change. Take a commit lease before staging and release after.

## 7. Known traps and fixes already paid for

- **rAF stall at startup.** Upstream defers its first DOM pass to
  `requestAnimationFrame`; before the first composed WebView2 frame the callback may
  never fire. Evidence: `rafCalls=1 rafFired=0 moFires=2` with no blur class. Adapter
  timer primes fix it.
- **`Init` loses the first document.** `AddScriptToExecuteOnDocumentCreated` is async;
  the first `Navigate()` can beat registration and the page gets no engine at all.
  Evidence: in-page beacon arrived, injected-script beacon did not. Evaluating the
  prelude on `NavigationCompleted` fixes it.
- **Selector rot.** WhatsApp Web (v2.3000.x) changes `data-testid` values; upstream
  keeps version-dated comments in selector files. When blur silently stops working on
  one surface, diff that surface's selector file first.
- **CRLF.** No `.gitattributes`, `core.autocrlf=true`; vendored files check out with
  CRLF. Tests do not byte-hash them; JS tolerates CRLF. Keep re-copies as-is.
- **Upstream's own `unblurLastNCount` clamp** posts a redundant host message; it
  terminates and is harmless.
- **Native UI vs upstream UI:** do not port `ui/`, `minimizer/`, `icons/`, or shortcut
  rebinding — Settings and the sidebar already own those surfaces.

## 8. Glossary

- **rule** — one entry in `WA_BLUR_RULES` (a toggle key + selectors + options).
- **target / pipeline** — CSS selector plus `|`-separated DOM traversal commands.
- **marker class** — generated `wa-js-target-*` class that the CSS actually matches.
- **override** — `.wa-unblur-override`, added on hover or for the newest N messages.
- **boot / prelude** — settings JSON + adapter + upstream files, evaluated per document.
- **apply script** — compact `__wassapPrivacyApply(json)` payload for live updates.
- **broadcast** — `EvalAll` of the apply script to every open session.
