# Pecut 4 — Account Switcher UI Specification

## Executive Summary

Pecut 4 implements the single-window account switching experience for Wassap Desktop. Users can now manage multiple isolated WhatsApp Web accounts from a compact vertical sidebar, switch profiles in-place within the main desktop window, and persist their active selection across application restarts.

## Architecture Decisions

### Main Window & Native Child Embedding

Wails v2.17.0 locks a single `edge.Chromium` instance to the main window's lifetime and user data folder. To preserve the requirement of **exactly one main desktop window**, the active account's WebView2 session is embedded as a Win32 `WS_CHILD` window inside the main Wails window, positioned to the right of the 68px sidebar. 

When the user switches accounts, the previous session is cleanly closed (flushing its isolated profile storage) and the next session is instantiated on the same embedded child HWND. No new desktop windows are created during the switching lifecycle.

### Active Account Persistence

`activeAccountId` is stored atomically in `~/.local/config/wassap-desktop/config.json`. On startup, the last active account is automatically selected. If the saved account no longer exists, the app gracefully falls back to the first available account.

## Implementation Details

### Backend

- **`internal/config`**: New atomic JSON configuration store.
- **`internal/platform/windows/webview2`**: Host supports child window attachment (`SetParentWindow`) alongside the legacy standalone PoC. Sessions support dynamic resizing (`UpdateBounds`) and modal visibility toggling (`SetVisible`).
- **`app.go`**: `SwitchAccount()` coordinate profile teardown/activation, active state memory + disk persistence, and active-account deletion blocking.

### Frontend

- **`frontend/src/app.css`**: Polished dark theme, 68px fixed sidebar, circular gradient avatars with initials, unread indicator placeholders, active state pill.
- **`frontend/src/components/*`**: Sidebar renderer, custom right-click context menu, accessible modal dialogs for Add/Rename/Remove flows.
- **`frontend/src/main.js`**: Lifecycle bootstrap, resize observer, switching state coordination, and Wails bindings integration.

## Testing Results

All Go test suites pass cleanly:
```
$ go test -v ./...
```
- `internal/config`: 4/4 tests PASS
- `internal/platform/windows/webview2`: 3/3 tests PASS
- `app.go` root package: 6/6 tests PASS

Frontend build:
```
$ cd frontend && npm run build
✓ 10 modules transformed.
✓ 2.45s build complete
```

## Known Limitations & Trade-offs

- **Manual window HWND probing**: The Windows host probes the main Wails window handle via `EnumWindows` + `GetWindowThreadProcessId`. This is robust in single-process Wails apps, but assumes standard process isolation.
- **Frontend resize events**: Native child window tracking relies on frontend `window.resize` debouncing rather than Wails `OnSize` callbacks, which may introduce ~50ms delay on very fast maximization animations.
- **No native context menu styling**: The right-click menu is a pure DOM element, meaning it respects WebView2 rendering but is subject to standard CSS limits rather than OS-styled menus.

## Recommendation for Pecut 5

- **Unread indicators**: Integrate WhatsApp Web message polling events via `WebView2.WebMessageReceived` to light up the green unread badges in the sidebar.
- **Notification Center**: Implement a Wails `Notification` binding or native Win32 toast notifications for received WhatsApp messages.
- **Settings UI**: Wire the `⚙️` settings button to an actual settings modal (e.g. enabling/disabling auto-launch, default account selection, theme overrides).
