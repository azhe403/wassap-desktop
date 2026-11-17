# Account Avatar Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make each account's sidebar avatar follow its WhatsApp profile photo, with initials as a silent fallback.

**Architecture:** A document-created script injected into every WebView2 WhatsApp session finds the self ("me") avatar, fetches its bytes in-page, and posts a data URI to Go over the existing `chrome.webview.postMessage` channel. Go validates, dedupes, and stores it under `~/.local/share/wassap-desktop/avatars/<id>.<ext>`, emits `account-avatar:changed`, and exposes `GetAccountAvatar(id)` to the frontend, which renders the image in the sidebar and Profile Management page. Photo removal posts a tombstone that deletes the cached file.

**Tech Stack:** Go (stdlib `image/jpeg`, `image/png`, `crypto/sha256`, `encoding/base64`), Wails v2 bindings + `wailsRuntime.EventsEmit`, WebView2/Win32 (`c.Init`, `MessageCallback`), vanilla JS frontend in Vite.

**Spec:** `docs/superpowers/specs/2026-11-16-account-avatar-sync-design.md`

## Global Constraints

- No new Go dependencies. WebP is explicitly rejected by validation (stdlib has no decoder).
- The `Account` struct and `accounts.json` are unchanged. Avatar presence is a filesystem property.
- Self avatar only. Never scrape contact, group, or status photos.
- Account id comes from the WebView2 session closure (`s.profile`), never from page payload. Validate it before it becomes a filename: `^[a-z0-9_-]{1,64}$`.
- No `visibilityState` gate in the injected script; background sessions are throttled by Chromium but not paused by our code.
- Avatar work is best-effort: any failure logs and falls back to initials; never blocks session setup, switching, shutdown, or UI render.
- The repo checkout currently has unrelated staged changes (a refactor moving `app.go` → `internal/app/`, plus `hooks.go`). This plan targets the **current** paths (`internal/app/...`). Before every commit: run `git status --short`, stage only files named in that task, and STOP if unrelated staged work would be swept in. Never use `--no-verify`.
- Do not push. No commit delays (AZ convention: one logical change per commit, no artificial sleeps).

## Review Focus

- WA renders a placeholder image when no photo is set → must show app initials, not WA's grey silhouette. Pinned in Task 1 observations and Task 3 script assertions (placeholder exclusion from the spike).
- Expired signed URL / non-CORS `fetch` returns zero bytes → no crash, initials persist. Pinned in Task 1 observations and Task 2 validation tests.
- User clears their WA photo → cached image deleted, initials return. Pinned in Task 2 `Clear` test and Task 3 tombstone message test.
- Hidden/background sessions are `SW_HIDE`'d → script must keep running there. Pinned in Task 3 assertion that the generated script contains no `visibilityState`.
- Account removed from app → avatar files removed; malicious account id cannot escape the avatars dir. Pinned in Task 2 unsafe-id test and Task 4 `RemoveAccount` wiring.

---

### Task 1: Spike — observe the self-avatar in a real session (throwaway)

**Files:**
- Create (temporary): `internal/platform/windows/webview2/avatar_spike_windows.go`
- Modify (temporary): `internal/platform/windows/webview2/host.go`, `internal/platform/windows/webview2/permissions_windows.go`, `internal/platform/windows/webview2/win32_windows.go`
- Record: `docs/superpowers/specs/2026-11-16-account-avatar-sync-design.md` (new "Spike results" section)

**Interfaces:**
- Produces: observed values for (a) self-avatar element type and URL scheme when a photo is set, (b) `fetch(url)` readable byte count per scheme, (c) what the same container renders when no photo is set. These decide the selector set, transport, and placeholder exclusion in Task 3.

- [ ] **Step 1: Add a throwaway diagnostic injection**

In `avatar_spike_windows.go` expose `func avatarDiagnosticScript() string` returning JS that, every 5 s, resolves the self-avatar from `[data-testid="navbar-item-me-tab-photo"]` (`img` → `currentSrc || src`; `svg image` → `href` / `xlink:href`), records `tag`, `scheme`, `naturalWidth`, and `fetch(url).then(r => r.blob()).then(b => b.size)` (catch → `-1`), then posts `JSON.stringify({kind:'avatar-diagnostic', ...})`. Wire it as a `c.Init` prelude in `setUpWebView` next to `injectAccountLabel`, and log each `avatar-diagnostic` message in `MessageCallback` as `webview2: avatar-diagnostic: <raw JSON>`. Do not write any files. This is throwaway code: no tests, no binding changes.

- [ ] **Step 2: Run the real app and capture observations**

Run: `wails dev` (or the existing build command from `README.md`) and open at least one logged-in account; then, if possible, a second account with no photo set. Watch stderr for `avatar-diagnostic:` lines. Expected: JSON lines containing `tag`, `scheme`, `fetchSize`, `naturalWidth` for the photo case and for the no-photo case. If no lines appear after 30 s, check that the prelude was installed and that the page finished loading before concluding anything.

- [ ] **Step 3: Record results and remove the spike**

Append a "Spike results" section to the design doc with the observed element type, URL scheme(s), readable fetch sizes, and the no-photo markup. Delete `avatar_spike_windows.go` and revert only the three temporary edits from Step 1 (by editing them back, not by checking out files that other work may have touched). If `fetchSize` is `-1` for every scheme observed, STOP and report: the chosen transport is unverified and needs a new design decision before Task 2.

- [ ] **Step 4: Commit the recorded findings**

```bash
git add docs/superpowers/specs/2026-11-16-account-avatar-sync-design.md
git commit -m "docs: record avatar spike observations"
```

(Only if the section is non-empty and the spike code is gone.)

### Task 2: Avatar store (validation, dedupe, clear)

**Files:**
- Create: `internal/avatar/store.go`, `internal/avatar/store_test.go`

**Interfaces:**
- Produces:
  - `avatar.NewStore(dataDir string) *Store` (creates `<dataDir>/avatars/` lazily on first write)
  - `avatar.Store.Get(id string) string` → data URI or `""`
  - `avatar.Store.Put(id, dataURI string) (hash string, changed bool, err error)`
  - `avatar.Store.Clear(id string) error`
  - IDs valid per `^[a-z0-9_-]{1,64}$`; files `<dir>/avatars/<id>.png` or `.jpg`; hash = SHA-256 hex of decoded bytes; atomic write via temp file + rename.

- [ ] **Step 1: Write failing tests in `internal/avatar/store_test.go`**

```
TestStorePutWritesFileAndReturnsHash:
  fixture = pngDataURI(t, 64, 64) generated with image/png + base64
  got := s.Put("a01", fixture); assert err nil, changed true, len(got.hash)==64,
  s.Get("a01") == fixture, file exists with mode 0600

TestStorePutRejectsInvalidPayloads (table, each expects err != nil and no file):
  "data:image/webp;base64,..." (WebP excluded)
  "data:image/png;base64,!!!not-base64!!!"
  "data:image/png;base64,<> 2MB+ of zero bytes"
  pngDataURI(t, 16, 16) (too small: <32px)
  "https://example.com/photo.jpg" (not a data URI)

TestStorePutSameBytesDoesNotRewrite:
  Put twice; second changed == false; file modtime unchanged

TestStoreClearRemovesAllExtensionsForID:
  create both <id>.jpg and <id>.png files via Put plus os.WriteFile;
  Clear(id); assert both gone, Get(id) == "", Clear on missing id is nil

TestStoreRejectsUnsafeID:
  Put("../escape", pngDataURI(...)) → error; no file outside avatars dir

TestStoreGetReturnsEmptyWhenAbsent:
  Get("missing") == ""
```

Helper `pngDataURI(t *testing.T, w, h int) string` encodes an `image.NewRGBA` with `png.Encode` and prefixes `data:image/png;base64,`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/avatar/ -v`
Expected: FAIL — package undefined.

- [ ] **Step 3: Implement `internal/avatar/store.go`**

Implement exactly the four signatures from Interfaces. Validate id first; validate MIME against `jpeg|png` only; `base64.StdEncoding.DecodeString`; size cap 2 MB; `image.DecodeConfig` (import `image/jpeg` and `image/png` for registration) with `Width >= 32 && Height >= 32`; hash decoded bytes; compare with the existing file's hash (decode-and-hash existing file — no sidecar); on same hash return `(hash, false, nil)`; otherwise write a temp file with mode 0600 in the avatars dir and rename to `<id>.<ext>`, removing any older extension variant for that id. `Get` reads the existing file and returns the data URI with the correct MIME, or `""` on any error. `Clear` removes `.png` and `.jpg` for the id and ignores `os.IsNotExist`. All failures return descriptive errors; none panic.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/avatar/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/avatar/store.go internal/avatar/store_test.go
git commit -m "feat(avatar): add validated local avatar store"
```

### Task 3: Inject-and-report pipeline (WebView2)

**Files:**
- Create: `internal/avatar/script.go`, `internal/avatar/script_test.go`, `internal/platform/windows/webview2/permissions_avatar_windows_test.go`
- Modify: `internal/platform/windows/webview2/host.go`, `internal/platform/windows/webview2/permissions_windows.go`, `internal/platform/windows/webview2/win32_windows.go`

**Interfaces:**
- Consumes: Task 1's observed selector/scheme/placeholder facts.
- Produces:
  - `avatar.DocumentScript(accountID string) string` — document-created JS
  - `Host.SetAvatarDelegate(onUpdate func(browser.ProfileID, string))` and `Host.avatarOnUpdate() func(browser.ProfileID, string)`; empty data URI means clear
  - Page messages `{"kind":"avatar-update","values":{"dataUrl":"data:image/...;base64,..."}}` and `{"kind":"avatar-clear"}`

- [ ] **Step 1: Write failing Go tests**

`internal/avatar/script_test.go`:
```
TestDocumentScriptIncludesAccountIDAndMessageKinds:
  script := DocumentScript("a01")
  assert contains JSON-escaped "a01", 'avatar-update', 'avatar-clear'

TestDocumentScriptUsesSpikeSelectorsAndExcludesPlaceholders:
  assert script contains the selector strings chosen from the spike results
  assert script contains the exclusion rule recorded in the spike (e.g. skip the
  observed placeholder signature and any image whose naturalWidth < 32)
  assert script does NOT contain 'visibilityState'

TestDocumentScriptPostsOnlyOnFingerprintChange:
  assert script contains a fingerprint comparison against lastUrl and that
  posting is inside the "changed" branch (string assertions on the generated source)
```

`internal/platform/windows/webview2/permissions_avatar_windows_test.go` (`//go:build windows`):
```
TestAvatarMessagesReachDelegateWithSessionProfile:
  host := NewHost("t", ""); captured := []; host.SetAvatarDelegate(func(id browser.ProfileID, uri string) { captured = append(...) })
  s := &Session{host: host, profile: "a01"}; c := &edge.Chromium{}
  wireNotificationClickMessage(s, c)
  c.MessageCallback(`{"kind":"avatar-update","account":"spoofed","values":{"dataUrl":"data:image/png;base64,AA=="}}`, nil, nil)
  c.MessageCallback(`{"kind":"avatar-clear"}`, nil, nil)
  assert captured == [{browser.ProfileID("a01"), "data:image/png;base64,AA=="}, {browser.ProfileID("a01"), ""}]

TestAvatarMessagesIgnoreMalformedJSON:
  same setup; c.MessageCallback("not json", nil, nil); assert no delegate call
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/avatar/ ./internal/platform/windows/webview2/ -v`
Expected: FAIL — `DocumentScript` undefined; delegate methods undefined.

- [ ] **Step 3: Implement `internal/avatar/script.go`**

Define `DocumentScript(accountID string) string` returning an IIFE with the account id embedded via `strconv.Quote`. JS behavior: resolve the self-avatar with the spike-verified selectors; skip placeholder signatures and `<32px` images; fingerprint = URL + naturalWidth; on change, `fetch(url)` → `r.blob()` → `FileReader.readAsDataURL` → post `avatar-update`; if the previously seen URL disappears, post `avatar-clear` once and reset `lastUrl`; triggers = initial run, `MutationObserver` on `document.body` with 2 s debounce, `setInterval` 30 s. Guard with a `window.__wassapAvatarSync` flag like the notification wrapper so re-injection is a no-op. No `visibilityState` reference. All JS errors caught and ignored.

- [ ] **Step 4: Wire the host**

In `host.go`, add `onAvatarUpdate func(browser.ProfileID, string)` next to the privacy hooks; `SetAvatarDelegate(fn func(browser.ProfileID, string))` stores it under the existing mutex; `avatarOnUpdate()` returns it. In `permissions_windows.go`, extend the existing `MessageCallback` switch with `case "avatar-update"` (read `msg.Values["dataUrl"]` as string; skip empty) and `case "avatar-clear"` (empty URI), calling `s.host.avatarOnUpdate()` with `s.profile` on a goroutine wrapped in the same `recover()` pattern as the privacy hook. In `win32_windows.go` `setUpWebView`, install `avatar.DocumentScript(string(s.profile))` with `c.Init(...)` next to `injectAccountLabel`; log failures, never fail session setup.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/avatar/ ./internal/platform/windows/webview2/ -v`
Expected: all PASS (non-Windows hosts: run the avatar package tests; webview2 tests are Windows-only).

- [ ] **Step 6: Commit**

```bash
git add internal/avatar/script.go internal/avatar/script_test.go internal/platform/windows/webview2/
git commit -m "feat(avatar): inject self-avatar scraper into WebView2 sessions"
```

### Task 4: App binding, event, and removal cleanup

**Files:**
- Modify: `internal/app/app.go`, `internal/app/app_other.go`, `internal/app/app_privacy_test.go` (fakeHost), Create: `internal/app/app_avatar_test.go`

**Interfaces:**
- Consumes: `avatar.NewStore`, `Store.Get/Put/Clear` (Task 2); `Host.SetAvatarDelegate` (Task 3).
- Produces:
  - `App.GetAccountAvatar(id string) string` (Wails binding)
  - `App.handleAvatarUpdate(id browser.ProfileID, dataURI string)` (delegate target)
  - Wails event `account-avatar:changed` payload `{id: string, hash: string}` (`hash: ""` = cleared)

- [ ] **Step 1: Write failing tests in `internal/app/app_avatar_test.go`**

```
TestHandleAvatarUpdateStoresAndClears:
  app := &App{dirs: appdirs.Under(t.TempDir()), avatarStore: avatar.NewStore(...)}
  app.handleAvatarUpdate("a01", pngDataURI(...)); assert avatar file exists
  app.handleAvatarUpdate("a01", ""); assert file gone

TestHandleAvatarUpdateRejectsInvalidPayload:
  app.handleAvatarUpdate("a01", "data:image/webp;base64,AA=="); assert no file

TestGetAccountAvatarReturnsEmptyForMissing:
  app.GetAccountAvatar("missing") == ""
```

Update `fakeHost` in `app_privacy_test.go` with `func (f *fakeHost) SetAvatarDelegate(func(browser.ProfileID, string)) {}`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -v`
Expected: FAIL — `avatarStore` field / `handleAvatarUpdate` / `GetAccountAvatar` undefined.

- [ ] **Step 3: Implement in `internal/app/app.go`**

Add `SetAvatarDelegate(func(browser.ProfileID, string))` to `HostController`; add `avatarStore *avatar.Store` to `App`; initialize `avatar.NewStore(dirs.Data)` in `NewApp`; install `app.handleAvatarUpdate` via `app.host.SetAvatarDelegate` in `NewApp` beside `SetPrivacyDelegate`. Implement `handleAvatarUpdate`: ignore empty ids; `""` data URI → `avatarStore.Clear(id)` → emit `{id, hash:""}`; otherwise `Put` → on error log and return; on `changed == false` return; emit `{id, hash}` with `wailsRuntime.EventsEmit` (guard `a.ctx != nil` so tests don't need a Wails context). Implement `GetAccountAvatar(id string) string` returning `avatarStore.Get(id)`. In `RemoveAccount`, after a successful `accountSvc.Remove(id)`, call `a.avatarStore.Clear(id)` (best-effort, log failure). Add the no-op `SetAvatarDelegate` to `stubHost` in `app_other.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -v` and `go build ./...`
Expected: all PASS, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/app/app.go internal/app/app_other.go internal/app/app_privacy_test.go internal/app/app_avatar_test.go
git commit -m "feat(avatar): wire avatar store into app binding and events"
```

### Task 5: Frontend rendering

**Files:**
- Modify: `frontend/src/main.js`, `frontend/src/components/sidebar.js`, `frontend/src/components/profiles-page.js`, `frontend/src/state.js`, `frontend/src/app.css`, generated `frontend/wailsjs/go/app/App.js`, `frontend/wailsjs/go/app/App.d.ts`
- Create: `frontend/src/utils/avatar-image.js`

**Interfaces:**
- Consumes: `GetAccountAvatar(id)` binding and `account-avatar:changed` event (Task 4).
- Produces:
  - `avatarImageHTML(id, avatars, fallbackHTML, imgClass)` → `<img>` markup when `avatars[id]` exists, else the unchanged fallback (initials)
  - `state.avatars` — plain object `id → dataURI`
  - `renderSidebar(container, accounts, activeId, switchingId, handlers, avatars)`, `renderProfilesPage(container, accounts, activeId, handlers, avatars)`

- [ ] **Step 1: Regenerate Wails bindings**

Run: `wails generate module` (repo root). Expected: `frontend/wailsjs/go/app/App.js` and `App.d.ts` contain `GetAccountAvatar`. If the CLI reports success but the files are unchanged, STOP and report rather than hand-editing generated code.

- [ ] **Step 2: Add the shared helper and state**

In `frontend/src/utils/avatar-image.js` export `avatarImageHTML(id, avatars, fallbackHTML, imgClass)` returning `<img class="${imgClass}" src="${escaped dataURI}" alt="" />` when present (escape quotes from the data URI — base64 is safe but keep the function defensive), else `fallbackHTML`. Export `fetchAvatars(api, ids)` → `Promise<object>` calling `api.GetAccountAvatar(id)` for each id in parallel, keeping only non-empty results. Add `avatars: {}` to `state` in `state.js`.

- [ ] **Step 3: Render images in both surfaces**

In `sidebar.js`, add the `avatars` parameter and wrap the existing `<span>${initials}</span>` with `avatarImageHTML(acc.id, avatars, initialsHTML, 'account-avatar-img')`; keep color background, unread badge, switching overlay, and active pill unchanged. In `profiles-page.js`, add the `avatars` parameter, pass it from `renderCard` to `avatarImageHTML` for `.account-avatar-static`. In `app.css`, add `.account-avatar-img { width: 100%; height: 100%; border-radius: 50%; object-fit: cover; }` and the equivalent for `.account-avatar-static img`; ensure the initials `<span>` is hidden when an image is present via `.account-avatar-btn:has(.account-avatar-img) span:not(.unread-badge), ... { display: none; }` (or a class-based rule if `:has` is unavailable in the embedded WebView).

- [ ] **Step 4: Wire load + live update in `main.js`**

After `refreshAccounts()` populates `state.accounts`, call `fetchAvatars(getApp(), state.accounts.map(a => a.id)).then(m => { state.avatars = m; render(); })`. Add an `EventsOn('account-avatar:changed', ({ id, hash }) => ...)` subscription next to `privacy-blur:changed`: empty `hash` → `delete state.avatars[id]`; otherwise refetch that id via `fetchAvatars` and set `state.avatars[id]`. Pass `state.avatars` as the extra argument to `renderSidebar` and `renderProfilesPage`. Keep failure paths silent (catch → leave initials).

- [ ] **Step 5: Build and verify**

Run: `cd frontend && npm run build`
Expected: build succeeds with no import errors. Then run the real app and confirm: sidebar circle shows the photo after opening an account, switches back to initials when the photo is cleared in WA, and the Profile Management page shows the same image. If `GetAccountAvatar` is missing from `window.go` at runtime, report the binding-namespace mismatch (the existing frontend uses `window.go.main.App` while the generated file targets `window.go.app.App`) instead of hand-editing generated code.

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/wailsjs/go/app/App.js frontend/wailsjs/go/app/App.d.ts
git commit -m "feat(frontend): render account profile photos in sidebar"
```

### Task 6: Full smoke test and documentation

**Files:**
- Modify: `docs/superpowers/specs/2026-11-16-account-avatar-sync-design.md` (smoke matrix results), `README.md` (only if a user-facing note is warranted)

**Interfaces:**
- Consumes: all earlier tasks.

- [ ] **Step 1: Run the smoke matrix in the real app**

For each row, record PASS/FAIL with a short note in the design doc:
1. Fresh account never opened → initials
2. Open logged-in account → photo appears within ~30 s
3. Change photo in WA (active account) → photo updates within ~30 s
4. Clear photo in WA → initials return
5. Delete account in Profile Management → `avatars/<id>.*` gone
6. Privacy blur enabled → no visual conflict on WA page; sidebar photo unaffected
7. Background (non-active) account with a changed photo → eventual update expected, slower due to Chromium throttling

- [ ] **Step 2: Run the full verification suite**

Run: `go test ./...` and `cd frontend && npm run build`
Expected: all PASS.

- [ ] **Step 3: Commit the smoke results**

```bash
git add docs/superpowers/specs/2026-11-16-account-avatar-sync-design.md
git commit -m "docs: record avatar sync smoke test results"
```
