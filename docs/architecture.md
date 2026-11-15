# Architecture Reference

Fixed technical reference for the current state of Wassap Desktop. Specs in
`docs/specs/` describe how each pecut was achieved; this document describes
what exists today.

## Data Layout

```text
~/.local/share/wassap-desktop/
├── accounts.json                account metadata (JSON list, atomic writes)
├── profiles/
│   └── <ulid>/
│       ├── EBWebView/           WebView2 user data folder (cookies, localStorage, IndexedDB, ...)
│       └── profile.lock         Win32 cross-process lock
├── profiles.json                profile manifest (Pecut 2 bookkeeping)
├── runtime/
│   ├── staging/                 in-memory + backup copies of open profiles
│   └── trash/                   profiles being deleted
└── EBWebView/                   legacy Pecut 1 single-session profile

~/.local/config/wassap-desktop/
└── config.json                  app configuration (e.g. activeAccountId, windowState)

~/.cache/wassap-desktop/
└── wassap-desktop.log           persistent mirror of the app log (appended)
```

- Account IDs are 26-char lowercase Crockford Base32 ULIDs (time-sortable,
  path-safe by `browser.ParseProfileID` rules).
- Each account's browser state lives only in its own `profiles/<ulid>/`
  directory. Deleting one account moves its directory to `runtime/trash/`,
  purges it, and leaves other profiles untouched.
- `config.json` persists UI preferences (active account, close-to-tray, and the main window's geometry under `windowState`) without storing any browser session data.

## Package Map

| Package | Responsibility |
|---|---|
| `internal/appdirs` | Resolve + ensure `~/.local/share|config|cache/wassap-desktop` |
| `internal/account` | `Account` model, ULID generation, name validation, `AccountService` |
| `internal/storage` | `JSONAccountRepository` — atomic, concurrent-safe JSON persistence |
| `internal/browser` | Platform-independent `ProfileManager` (create/open/close/delete, locks, crash recovery) |
| `internal/platform/windows` | WebView2 host + Win32 locks + storage inspector + system tray (`tray/`) |
| `internal/config` | Atomic JSON app configuration store (`config.json`) |
| `app.go` / `main.go` | Wails facade: thin bindings, asset serving, legacy-mode redirect, window embedding |
| `windowgeom_windows.go` / `windowgeom_other.go` | Win32 main-window geometry: poll, clamp/restore, `SetWindowPos` (no-op off Windows) |

## Account Lifecycle

```text
CreateAccount(name)
  -> validate name, generate ULID
  -> ProfileManager.Create(id)        profile dir on disk
  -> repository.Save(account)         accounts.json
  -> on save failure: ProfileManager.Delete(id)  (no orphan profiles)

RemoveAccount(id)
  -> reject if profile open/busy (ErrAccountBusy)
  -> reject if account is currently active
  -> ProfileManager.Delete(id)
  -> repository.Delete(id)
```

## Single-Window Embedded WebView Architecture (Pecut 4)

Wassap Desktop preserves exactly one desktop window. Account switching occurs in-place inside that single Wails window:

1. **Sidebar (HTML/CSS)**: Rendered by Wails in a fixed-width vertical region.
2. **Active WebView (Native)**: The currently active account's isolated `edge.Chromium` WebView2 session is embedded as a Win32 `WS_CHILD` window inside the main Wails window, covering the right-hand content region.
3. **Synchronized Resizing**: The frontend observes window resizes and calls `NotifyWindowResize`, which repositions the native child window to match.
4. **Modal Dialogs**: During rename/add/delete dialogs, the native child WebView is hidden to ensure full interactivity with the sidebar UI.

Active account selection is restored from `config.json` at startup, with a fallback to the first valid account when the persisted reference becomes stale.

## Window Geometry & Restore

The main window's size, position and maximised state are remembered across restarts in `config.json` under `windowState` (`x`, `y`, `width`, `height`, `maximised`). Units are asymmetric by Wails convention: width/height are logical 96-DPI pixels (the unit of Wails window options), x/y are raw screen pixels. Window size and the maximised state go through the Wails window options before creation; the position is applied once the polling loop sees the window (clamped to the nearest connected monitor's work area, so the window cannot open off-screen after a monitor is removed). The restore-position step does not run for a maximised window — its rect belongs to the shell, and the pre-maximise position is not addressable through window options (a Wails limitation); un-maximising returns to Wails' default position.

Geometry changes are observed by polling `GetWindowRect`/`IsZoomed` every 250 ms (debounced 400 ms before writes), not via window messages: the `SetWindowSubclass` attempt on the main window fails silently (`ret == 0`), so `WM_SIZE`/`WM_MOVE` never reach app code. Live child-session resizing is driven independently by the frontend's `window.resize` → `NotifyWindowResize` → `Host.UpdateBounds` path and is unaffected.

## System Tray & Close-to-Tray

The app registers a Windows system tray icon (`internal/platform/windows/tray`) on a dedicated hidden top-level window with its own locked thread (a top-level window receives `TaskbarCreated` broadcasts; a message-only window does not). The icon survives Explorer restarts: on `TaskbarCreated` the shell dropped the old icon, and the tray re-registers it. A single-instance lock (`options.SingleInstanceLock`, UUID `3f7f3db3-008c-4e3d-a134-0d66e741a2f1`) guarantees exactly one running instance; a second launch foregrounds the first (`OnSecondInstanceLaunch` → `App.showFromBackground`).

When close-to-tray is enabled in Settings, closing the window hides it instead of exiting - but only if the tray icon is actually registered (`tray.Ready()`); otherwise the window closes normally so the app can never be left hidden with no way to quit.

A real quit no longer blocks on session teardown: `OnBeforeClose` hides the window immediately, flushes pending window geometry, and runs `CloseCurrent` in a background goroutine; only after the teardown completes does it release the close to Wails (`quitting` disarms the first close; the second call lets it proceed). Wails then destroys the parent window and `shutdown` runs. The process exits only after the session flush, so embedded WebView2 child windows are never orphaned mid-teardown (an orphan would stall every later close with a full timeout). "Quit" in the tray menu takes the same path; on quit the sessions are torn down in parallel (worst case one teardown timeout, not two). Close-to-tray behavior is unchanged: the X button with close-to-tray enabled still just hides to the tray.
