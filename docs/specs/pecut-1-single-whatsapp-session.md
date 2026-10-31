# Pecut 1 — Single WhatsApp Web with Persistent Session

## Context

We are building Wassap Desktop, a multi-account WhatsApp Web desktop application.

Current stack:
- Wails v2.17.0
- Go
- Vanilla HTML, CSS, and TypeScript/JavaScript
- Native WebView2 on Windows

The Wails development environment is now running successfully.

## Objective

Implement the smallest working version of Wassap Desktop that displays WhatsApp Web directly inside the application and preserves its login session between application restarts.

## Requirements

1. Navigate the native WebView directly to:
   `https://web.whatsapp.com`

2. Do not use iframe, reverse proxy, or unofficial WhatsApp APIs.

3. Configure persistent browser storage.

4. Use these application directories:
   - Data: `~/.local/share/wassap-desktop/`
   - Config: `~/.config/wassap-desktop/`
   - Cache: `~/.cache/wassap-desktop/`

5. Keep the directory convention consistent across Windows, Linux, and macOS. Resolve `~` using the current user's home directory.

6. Do not implement multi-account management yet.

7. Do not introduce React, Vue, Electron, or unnecessary dependencies.

## Important technical investigation

Before implementing browser storage:

- Inspect how Wails v2.17.0 configures WebView2.
- Determine where its persistent browser data is stored.
- Verify whether the application can explicitly configure the WebView2 user data directory.
- Do not assume that creating a directory automatically makes WebView2 use it.
- Document any platform-specific limitations.

## Acceptance Criteria

- [ ] Application opens WhatsApp Web directly.
- [ ] WhatsApp Web can load normally.
- [ ] User can log in.
- [ ] Closing and reopening the application preserves the login session.
- [ ] Browser data is stored persistently.
- [ ] No unrelated multi-account functionality is introduced.
- [ ] `wails dev` works.
- [ ] Production build succeeds.

## Implementation Rules

- Inspect the existing project before modifying files.
- Make the smallest necessary changes.
- Keep implementation simple and maintainable.
- Add tests for application path resolution where practical.
- Do not modify unrelated files.
- Do not create unnecessary Git commits.
- Update README with implementation details and known limitations.

At the end, report:
1. Files created or modified.
2. How persistent WebView storage is configured.
3. Actual browser data directory.
4. Build and test results.
5. Remaining limitations.

---

## Status (2026-10-31)

Status per acceptance criterion, based on what has actually been verified:

- [x] **Opens WhatsApp Web directly.** Verified by screenshot: the window shows the WhatsApp "Scan to log in" screen.
- [ ] **WhatsApp Web loads normally.** Partly verified. The page renders, but a reload loop was reported. The cause (a `window.location.replace` in an `OnDomReady` hook) was removed and the build passes. Stability has not yet been confirmed in the user's window.
- [ ] **User can log in.** Not tested. No QR scan has been performed.
- [ ] **Session survives restart.** Not proven. The cookie database persisted across three launch/exit cycles, but without a real login there is no session to preserve.
- [x] **Browser data stored persistently.** Verified. The profile is at `~/.local/share/wassap-desktop/EBWebView/`, with `Default/Network/Cookies`, `Local Storage`, `IndexedDB`, and `Local State`.
- [x] **No multi-account functionality.** Verified. None was added.
- [x] **`wails dev` works.** Verified. `wails dev` builds and launches the development binary with WebView2 environment created and watcher active. The earlier exit was caused by a concurrent production binary holding a lock on the WebView2 profile directory (`~/.local/share/wassap-desktop/EBWebView/lockfile`). When no instance is running, `wails dev` runs properly.
- [x] **Production build succeeds.** Verified with `wails build`, which produced `build/bin/wassap-desktop.exe`.

### Test results

- `go vet ./...`: passes.
- `go test ./...`: passes (`internal/appdirs` and the root package).

### Open items

1. Confirm the window stays on WhatsApp Web without reloading. (Reload loop fix applied: removed `OnDomReady` replacement hook; redirect via HTTP 302 handler now drives navigation).
2. Log in with a QR scan, then restart and confirm the session is kept.
3. Decide whether to migrate the old `%APPDATA%\wassap-desktop-dev.exe` profile. The user chose a fresh login and to leave the old data untouched.
