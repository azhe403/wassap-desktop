# Memory Saver — Design

Date: 2026-11-13
Status: Approved approach (A), pending design sign-off

## Intent

Wassap Desktop keeps every account's WebView2 engine alive in the background after an
account switch, so switching back is instant. Each background engine plus its in-memory
profile is expensive. The user wants a way to reclaim that memory: after a background
account has not been accessed for a configurable limit (default 5 minutes), its browser
session is disposed automatically. The feature is toggleable on/off in Settings and is
off by default.

## Goals

- New **Memory Saver** toggle in the Settings page. Default: **off**.
- New **limit** control (up/down stepper, minutes, range 1–60, default **5**) next to the
  toggle.
- When on: a backend sweeper disposes any open session that is NOT the active account
  and whose last access is older than the limit.
- "Accessed" means: the user switched to (or opened) that account. Background activity
  inside the WhatsApp Web page does not count, and does not need to.
- Disposal is `Manager.Close`: flush the in-memory profile to disk, then tear the engine
  down. No data is lost; reopening the account just takes longer (full page load).
- Switching back to a disposed account uses the existing `Open` path and the existing
  loading state in the UI. Nothing new to build for that path.

## Non-goals / anti-goals

- Never dispose the currently active account.
- No WhatsApp-message-based keep-alive (no peeking into the webview).
- No per-account opt-out, no notifications/toasts on auto-dispose, no tick-interval
  setting, no hours unit.
- Toggle off does not resurrect disposed accounts; it only stops future disposals.
- Toggle on while a session is already idle beyond the limit disposes it on the next
  tick (within 30 s), not instantly.
- Idle timestamps are in-memory; nothing to restore after a restart (no sessions are
  open at startup).

Accepted tradeoff: a disposed background account stops receiving WhatsApp Web messages
and notifications until it is opened again. The user asked for the disposal explicitly.

## Approach (picked: A — backend sweeper)

Rejected:

- **B. Frontend timer** — webview JS does not own session state; timers drift across
  re-renders/settings views and cannot be unit-tested against the engine. Fragile.
- **C. Idle timeout inside `browser.Manager`** — Manager has no concept of the active
  account or user preferences; it would need an injected clock, callback, and config,
  leaking app policy into the profile/engine layer.

## Design

### Config (`internal/config/config.go`)

Add to `AppConfig`:

```go
MemorySaver        bool `json:"memorySaver"`        // default false (zero value)
MemorySaverMinutes int  `json:"memorySaverMinutes"` // missing/0 -> 5; clamp 1..60
```

Store interface gains `SetMemorySaver/GetMemorySaver` and
`SetMemorySaverMinutes/GetMemorySaverMinutes`, mirroring the CloseToTray pattern.
`GetMemorySaverMinutes` applies the default-and-clamp so every caller gets a sane
value without duplicating the rule.

### App (`app.go`)

- New in-memory state under `a.mu`: `memorySaver bool`, `memorySaverMinutes int`,
  `lastAccess map[string]time.Time`, `disposing map[string]chan struct{}`.
- Loaded at `NewApp` from the config store (like `closeToTray`).
- `SwitchAccount` and `OpenAccount` set `lastAccess[target] = time.Now()` on success.
- New `Manager` method `OpenIDs() []string`: IDs of entries with `StateOpen` and an
  alive session. No disk I/O.
- Sweeper goroutine started in `startup()`, stops when the Wails context is done.
  Every 30 s:
  1. Lock `a.mu`. Skip if `!memorySaver`, `switching` is true, or no open sessions.
  2. Candidates = open IDs except `activeAccount`. For an ID with no `lastAccess`
     entry yet, set it to now and skip (first sight arms the clock).
  3. For each candidate with `now-lastAccess > limit`: create `ch`, store in
     `disposing[id] = ch`, collect. Unlock `a.mu`.
  4. For each collected candidate: `manager.Close(id)` (slow path, never under
     `a.mu`), then lock `a.mu`, close `ch`, remove `disposing[id]`, and:
     - on success: `delete(lastAccess, id)`, log;
     - on failure: `lastAccess[id] = time.Now()` (back off one full interval), log.
- Race with switching: `SwitchAccount`/`OpenAccount`, right after taking `a.mu` and
  before setting `switching = true`, check `disposing[target]`; if present, unlock,
  wait on the channel with a 15 s timeout, re-lock, then proceed. On timeout, log and
  proceed anyway (a busy profile will surface through the existing Open error path).
  This serialises disposal vs. switch-back without holding `a.mu` during teardown.
- `RemoveAccount` needs no change: if it races a disposal it may call `Close` on an
  already-closed session; that error path already exists and is non-fatal.

### Settings UI (`frontend/src/components/settings-page.js`, `frontend/src/main.js`)

- New row in the General group, above About: **Memory Saver** toggle, description
  "Dispose the browser session of accounts you have not used to free memory.
  Reopening an account takes a few seconds to reload."
- New row: **Dispose after** with an up/down stepper (−/+ buttons and a `N min` label,
  1–60, step 1), wired to the minutes setting.
- Handlers `onMemorySaverChange` / `onMemorySaverMinutesChange` in `main.js` follow the
  existing `handleCloseToTrayChange` pattern: call the Wails binding, toast on
  success/failure, revert the control on failure.
- Wails bindings (`GetMemorySaver`, `SetMemorySaver`, `GetMemorySaverMinutes`,
  `SetMemorySaverMinutes`) are regenerated by `wails dev` / build; the `wailsjs` files
  are generated artefacts.

### Tests

- `internal/config`: round-trip of both new keys; missing → (false, 5); minutes of 0,
  negative, >60 → clamped to 1..60 with 5 as the empty-file default.
- New `sweeper.go` in `package main`: pure selection function
  `disposableSessions(openIDs []string, lastAccess map[string]time.Time, active string, limit time.Duration, now time.Time) []string`
  — table-tested: active account excluded, within-limit kept, over-limit disposed,
  unknown IDs arm-not-dispose, and a `disposing` overlap is skipped.
- App-level test for the wait/race path is not practical with a real engine; correctness
  there rests on the mutex discipline above and manual dev-mode verification.

## Manual verification

1. `wails dev`, create/switch 2 accounts, enable Memory Saver with limit = 1 minute.
2. Leave account A in background, confirm via logs it is disposed after the interval,
   memory drops in Task Manager, and switching back to A reloads WhatsApp Web.
3. Toggle off mid-interval; confirm no further disposals happen.
4. Set limit to 1, toggle off then on with idle sessions past the limit; confirm
   disposal happens on the next tick.
