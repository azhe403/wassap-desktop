# Account Avatar Sync — Sidebar Follows the WhatsApp Profile Photo

**Date:** 2026-11-16
**Status:** Proposed
**Scope:** `internal/avatar/` (new), `internal/platform/windows/webview2/` (inject + message), `app_windows.go` / `app.go` (wiring + bindings), `frontend/src/components/sidebar.js`, `frontend/src/components/profiles-page.js`, `frontend/src/main.js`, `frontend/src/utils/` (shared avatar helper)

## Intent

The sidebar shows initials with a deterministic color for each account. The user wants that circle to follow the account's actual WhatsApp profile photo, so multiple accounts are distinguishable at a glance by their pictures.

There is no WhatsApp API access. The only place the photo exists is inside the isolated WebView2 session per account that loads `https://web.whatsapp.com`. The app already ships two inject-and-report pipelines into that page (notification account label, privacy blur), so this feature is a third one with an image payload instead of text.

## Goals

- Sidebar avatar circle renders the account's WhatsApp profile photo once the session has loaded it.
- Profile Management page static avatar uses the same image.
- Avatar refreshes on its own when the user changes their photo in WhatsApp.
- Any failure (page not loaded, selector gone, decode error, oversized image) falls back silently to the current initials rendering.

## Non-goals

- No group or contact avatars; only the self ("me") photo.
- No manual upload / override (the user explicitly chose auto-sync only).
- No tray icon or notification image changes.
- No encryption at rest beyond the existing 0700 profile-directory convention; the avatar is a local cache of something the user can already see.

## Anti-goals

- Scraping other people's photos. The injected script only reads the self avatar.
- Blocking or delaying session setup, switching, or shutdown on avatar work. All avatar handling is best-effort and off the critical path.

## Constraints

- WhatsApp Web DOM selectors are unofficial and change without notice. The self-avatar targets, in priority order:
  1. `[data-testid="navbar-item-me-tab-photo"] img` (current navbar self photo)
  2. `[data-testid="navbar-item-me-tab-photo"] svg image` (vector variant, href attribute)
  3. `[data-testid="profile-drawer"] [data-testid="profile-pic-picker"] img` (drawer fallback)
  All current-version-verified by the privacy blur selector history (`selectors/avatars.js`), which tracks the same element for blurring.
- Media CDN URLs are signed and expire. Bytes must be fetched **inside the page** (`fetch(url)` → `FileReader` → base64), not by Go. `data:` and `blob:` URLs are posted as-is after the same fetch.
- Wails frontend cannot read arbitrary file paths in `<img src>`. Images reach the frontend as base64 data URIs through a bound method, like update status flows through events.
- Sessions run as hidden `WS_CHILD` windows when not active, but JavaScript keeps running, so hidden sessions can scrape too.

## Approaches considered

1. **In-page fetch, post base64 to host (picked).** Injected script finds the self-avatar element, fetches the URL in page context, base64-encodes, posts `{kind:"avatar-update", account, dataUrl}` over `window.chrome.webview.postMessage`. Host validates, hashes, writes `avatars/<id>.<ext>`, emits `account-avatar:changed`. Works for expiring CDN URLs and for blob/data hrefs. Cost: base64 payload (~30–150 KB) per photo change; message channel already proven by the privacy blur pipe.
2. **Post the URL, Go downloads it.** Smaller messages, but signed URLs expire and Go would need the session's cookies; blob/data hrefs (vector avatar variant) are useless outside the page. Rejected.
3. **Canvas screenshot crop of the avatar element.** Works without reading URLs, but cross-origin images taint the canvas, and cropping coordinates are brittle. Rejected.

## Design

### 0. Spike first (decided 2026-11-16)

A throwaway diagnostic build ships before the real pipeline. The injected script (or a temporary `c.Init` prelude) only observes and logs — no writes:

- What the self-avatar container actually renders: element types (`img` vs `svg image`), URL schemes of `src`/`href` (`https:` media CDN, `blob:`, `data:`), and whether a placeholder element is rendered when no photo is set.
- Whether in-page `fetch(url)` returns non-empty bytes (`r.blob().then(b => b.size)`), for each URL scheme observed.

Results decide (a) the real selector set, (b) the transport (direct base64 post if fetch is readable; a canvas read-back fallback only for verifiably untainted cases; otherwise stop and reconsider), and (c) the placeholder exclusion rule. The spike is deleted or reduced to the real scanner afterwards; nothing it logs contains message content, only element metadata.

### 1. Injected script (document-created, alongside the privacy prelude)

Installed via `c.Init(script)` in `runSession` next to `injectAccountLabel`, stamped with `ACCOUNT_ID` like the notification wrapper. Contents:

- Resolves the self-avatar URL using the constraint selector list above; `img.currentSrc || img.src`, or `svg image.getAttribute('href')` / `xlink:href`.
- Keeps `lastUrl` in memory. When the URL changes (avatar set or changed), fetches it in page context and posts one message:
  `window.chrome.webview.postMessage(JSON.stringify({kind:'avatar-update', account: ACCOUNT_ID, dataUrl: '<data:image/...;base64,...>'}))`
- Scan triggers: a `setInterval` (starting at 30 s), a `MutationObserver` on `document.body` with a 2 s debounce, and once on `load`. **No `visibilityState` gate** (decided 2026-11-16): inactive sessions are `SW_HIDE`'d and Chromium treats hidden pages as background, so a gate would pause the scrape for exactly the accounts the sidebar shows. Background timer throttling is accepted as-is; the active session stays fresh, others refresh on a slower cadence.
- Tombstone path (decided 2026-11-16): when a previously-resolved URL disappears from the DOM (photo cleared in WA), the script posts `{kind:'avatar-clear', account: ACCOUNT_ID}` once; `lastUrl` remembers whether a URL was seen before. Host deletes `avatars/<id>.*` (file + sidecar) and emits `account-avatar:changed` with an empty hash; frontend falls back to initials.
- Fingerprint = resolved URL string plus `naturalWidth`. On repeated identical URLs, nothing is posted. The host additionally dedupes by content hash, so redundant posts are harmless.
- Placeholder exclusion (decided 2026-11-16, pending spike confirmation): the spike (see below) records what the self-avatar container renders when no photo is set. The script must not post WA's placeholder artwork as if it were a profile photo — e.g. skip URLs/data-URIs the spike identifies as placeholders, and treat an undecodable/`<32px` result as "no photo" (→ initials).

### 2. Host side (Go, Windows/WebView2)

- `wireNotificationClickMessage` grows a `case "avatar-update"` and `case "avatar-clear"`. Like the notification-click pipe, the account id comes from the session closure (`s.profile`), never from the page payload — the payload's `account` field is ignored and the id is sanitized before it touches the filesystem.
- Payload validation, in order: `dataUrl` matches `^data:image/(jpeg|png);base64,`, decoded size ≤ 2 MB, image dimensions ≥ 32 px (decode via `image.DecodeConfig`). WebP is **excluded** (decided 2026-11-16, my call): the stdlib has no WebP decoder and adding `golang.org/x/image` for one header check is not worth the dependency; WhatsApp profile photos are JPEG. Any failure: log and drop.
- `avatar-clear`: delete `avatars/<id>.*` (image + sidecar), emit `account-avatar:changed` with empty hash. Idempotent when nothing exists.
- Content dedupe: SHA-256 of decoded bytes compared against the previously stored image (re-decode the existing file or keep a `.sha256` sidecar — builder's choice). Same hash → no write, no event.
- Write to `<Data>/avatars/<id>.<ext>` (ext from MIME; jpeg/png) via temp file + rename, replacing any prior file for that account. `<Data>` comes from `appdirs.Dirs`, i.e. `~/.local/share/wassap-desktop/avatars/`.
- Emit `account-avatar:changed` with `{id, hash}` via `wailsRuntime.EventsEmit`. The WebView2 layer holds no app context: like the privacy-blur pipe, `App` registers an avatar-write handler on the host (`SetAvatarWriteHandler`-style, set in `startup`), which does validate/store/emit.

### 3. Wails bindings (app.go)

- `GetAccountAvatar(id string) string` — returns the stored data URI for the account, or `""` when absent. Reads and re-encodes on call (files are small); frontend caches per hash.
- No change to `Account` struct or `accounts.json`: avatar presence is a filesystem property, so a repo migration is avoided and `RemoveAccount` cleanup stays one place.
- `RemoveAccount` additionally deletes `avatars/<id>.*` (best-effort, logs on failure).
- Non-Windows stub (`app_other.go`) gets no-op equivalents only if the bindings reference platform code; the avatar store itself is platform-neutral, so bindings live in `app.go` and work everywhere. The injected script only ever runs under WebView2 anyway.

### 4. Frontend

- New `frontend/src/utils/avatar-image.js`: `fetchAvatarMap()` → `api.GetAccountAvatar(id)` per account, returns `Map<id, dataUri>`; caches by hash; also `applyAvatars(container)` used by both sidebar and profiles page.
- `sidebar.js`: when a data URI exists for an account, the `.account-avatar-btn` renders `<img>` (object-fit: cover, border-radius inherit-ish, 48×48) in place of the initials `<span>`; `getAvatarColor` background stays as fallback frame. Unread badge, switching overlay, and active pill are unchanged and layer above the image.
- `profiles-page.js`: static avatars get the same treatment.
- `main.js`: on boot after `refreshAccounts`, fetch the map once and re-render. Subscribe to `account-avatar:changed` (next to the existing `update:status` / `privacy-blur:changed` subscriptions) → refetch just that id and re-render sidebar.
- `app.css`: `.account-avatar-img { width:100%; height:100%; border-radius:50%; object-fit:cover; }` plus skeleton/alt handling.

### 5. Failure modes

- Account never opened → no avatar file → initials (expected).
- Photo cleared in WA → tombstone message deletes the cached file → initials.
- Selector drift / WA rewrite → scan finds nothing → initials; debug log behind existing logging, no user-facing error.
- Oversized or non-image payload, or WebP → dropped at host validation → initials persist.
- Avatar file corrupt on read → binding returns `""` → initials.
- Account deleted → files removed; event re-render removes `<img>`.

## Approaches to the freshness question (decided)

MutationObserver (2 s debounce) + a 30 s interval, with **no visibilityState gate**, was picked (2026-11-16, user) over "active session only" (fails goal 3 for the sidebar's other avatars) and over "gate on visibility" (Chromium hides `SW_HIDE` child windows, pausing the pipeline for exactly those accounts). Background timer throttling is accepted; `visibilitychange` is no longer referenced by the design.

## Spike results (2026-11-17, observed on live WA Web in account-a)

- `[data-testid="navbar-item-me-tab-photo"]` **does not exist** in the current WhatsApp Web build (197 `data-testid` elements probed, no match). It cannot be the primary selector.
- The reliable self-photo element is `[data-testid="message-yourself-row"] img` — the "Message yourself" chat row uses the account's own profile photo. Observed: `IMG`, `https://media-cgk2-1.cdn.whatsapp.net/...`, `naturalWidth=96`, URL length 233.
- The first image inside `[data-testid="chatlist-header"]` is **not** the self photo: it is `https://static.whatsapp.net/...`, `naturalWidth=84` (app chrome/logo). It must be excluded.
- In-page `fetch` readability is **confirmed**: the CDN self-photo URL yielded a 2905-byte blob from page context; the static logo yielded 3086 bytes. Transport by in-page fetch → base64 is viable.
- No-photo/placeholder case was not observed directly (only account-a was open; UI automation could not target account-b's sidebar button). Placeholder exclusion is therefore rule-based, not observed: accept self-photo candidates only from media hosts (`*.cdn.whatsapp.net`, `pps.whatsapp.net`), require `naturalWidth >= 32`, and reject `static.whatsapp.net`, `data:`, and `blob:` sources. This excludes both the WA logo and typical inline placeholder artwork.
- Post-review hardening (2026-11-17): the injected script also rejects any self-row containing `[data-testid*="default-contact-refreshed"]` or `[data-testid*="default-group-refreshed"]`. Those testids are documented in the vendored privacy-blur selectors as WA's default silhouette; if WA ever serves that silhouette from a media host, the sidebar keeps the app's initials instead of showing it.

Consequence for implementation: the primary selector list in §Constraints is superseded by the spike results above; `message-yourself-row img` is primary, `navbar-item-me-tab-photo` is retained only as a harmless fallback for older builds.

## Smoke test results (2026-11-17, built dev binary against the real profiles)

- Fresh account before any cached photo → initials: PASS. account-b rendered "A0" initials until its photo was fetched.
- Opened account → photo appears: PASS. `avatars/account-a.jpg` (2905 bytes, matching the spike's fetched blob size) written at 08:45:21; the sidebar showed the photo instead of initials.
- Background/non-active session also scrapes: PASS. `avatars/account-b.jpg` (1317 bytes) was written at 08:47:52 while the other account was active.
- Change photo in WA → updates within ~30 s: NOT VERIFIED live (no automation for WhatsApp Web profile editing). The fingerprint/refresh path is pinned by unit tests on the generated script and store dedupe.
- Clear photo in WA → initials return: NOT VERIFIED live. Tombstone is covered by unit tests (`avatar-clear` message → host `Store.Clear` → event with empty hash → frontend deletes the data URI).
- Delete account → avatar files removed: NOT VERIFIED live. `RemoveAccount` calls `avatarStore.Clear` and the store's clear test passes.
- Privacy blur on → no interaction: NOT VERIFIED visually; the two features touch different DOM surfaces and no error was logged.
- No avatar-related errors appeared in the app log during the run.

## What's left to the builder

- Exact selector ordering and debounce constants; tune against the real page if the interval proves chatty.
- Whether the `.sha256` sidecar or re-decode-and-compare is cheaper to keep consistent; either satisfies the dedupe requirement.
- CSS details to make `<img>` behave exactly like the initials span inside `.account-avatar-btn`.
- Manual smoke test matrix: fresh account (initials), after first open (photo appears), change photo in WA (updates ≤ ~30 s), delete account (files gone), privacy blur on (no interaction).
