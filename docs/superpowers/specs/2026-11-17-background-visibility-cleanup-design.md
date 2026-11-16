# Background Session Visibility + Shell Profile Cleanup - Design

Date: 2026-11-17
Status: Proposed (awaiting sign-off)

## Intent

Wassap Desktop keeps every account's WebView2 engine alive in the background after an
account switch. Hidden sessions are only `SW_HIDE`d at the Win32 level; the WebView2
controller is never told the content is invisible, so Chromium treats each background
account as a fully visible page: rAF loops, unthrottled timers, full rendering. That
burns CPU and memory for accounts nobody is looking at, while the user's hard
requirement is that background accounts keep receiving WhatsApp Web messages and
notifications.

Two changes, both small:

1. Make hidden sessions controller-invisible (`ICoreWebView2Controller.PutIsVisible(false)`),
   so Chromium throttles rendering/timers for them like a background tab.
   JavaScript and WebSockets keep running: messages and notifications still arrive.
2. Delete stale `wails-shell/run-*` WebView2 user-data folders left behind by previous
   launches of the shell. Pure disk hygiene for the fresh-per-launch shell profile.

## Goals

- `Session.Hide`: after `hideWindow(hwnd)`, call `s.chromium.Hide()` (already exists in
  go-webview2 as `PutIsVisible(false)`), guarded by `s.chromium != nil` and logged on
  failure. Visibility is best-effort, mirroring the rest of the session layer.
- `Session.Show`: after `showWindow(hwnd)`, call `s.chromium.Show()`
  (`PutIsVisible(true)`), same guard and logging, so the page becomes "visible" again
  and Chromium restores full rendering.
- Startup cleanup in `main()` before `wails.Run` (after `appdirs.Ensure`): scan
  `filepath.Join(dirs.Data, "wails-shell")`, remove every `run-<pid>` entry whose pid
  is not the current process. Deletion failures are logged and ignored
  (WebView2 may still hold handles). The current process's folder is never removed.
- No UI, no config keys, no behaviour change for the active session.

## Non-goals

- `TrySuspend`/hibernation of background accounts (kills message delivery; rejected).
- `Manager.Close` disposal / Memory Saver sweeper (the 2026-11-13 design; separate,
  unimplemented, and unwanted under "stay online").
- Switching the shell to a stable user-data folder (the `run-<pid>` workaround avoids a
  known WebView2 in-use-folder failure; not revisited here).
- Additional browser arguments / GPU flags.

## Accepted tradeoffs

- Controller `IsVisible=false` makes the page hidden: rAF stops and timers throttle, but
  network events, message handlers and the Notification API keep working. If some
  WhatsApp Web code path depends on "visible" for sync to progress, it degrades to
  background-tab behaviour - same as any browser tab in the background.
- Stale `run-*` deletion is best-effort; folders held open by a still-running crashed
  browser process survive and are retried on the next launch.

## Verification

1. `go build` / `wails build` (or dev run) with 2 accounts; switch between them.
2. Observe via `GetResourceUsage()` (existing stats) that the hidden account's CPU
   drops; Task Manager shows reduced working set / GPU usage for background engines.
3. Send a message to the hidden account from a phone; confirm the desktop notification
   still fires while another account is active.
4. Trigger the cleanup by launching the app after previous runs; confirm old `run-*`
   folders are gone and the log shows the cleanup line.
5. Existing Go test suite stays green (`go test ./...`).
