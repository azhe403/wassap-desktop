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
└── config.json                  app configuration (e.g. activeAccountId)
```

- Account IDs are 26-char lowercase Crockford Base32 ULIDs (time-sortable,
  path-safe by `browser.ParseProfileID` rules).
- Each account's browser state lives only in its own `profiles/<ulid>/`
  directory. Deleting one account moves its directory to `runtime/trash/`,
  purges it, and leaves other profiles untouched.
- `config.json` persists UI preferences without storing any browser session data.

## Package Map

| Package | Responsibility |
|---|---|
| `internal/appdirs` | Resolve + ensure `~/.local/share|config|cache/wassap-desktop` |
| `internal/account` | `Account` model, ULID generation, name validation, `AccountService` |
| `internal/storage` | `JSONAccountRepository` — atomic, concurrent-safe JSON persistence |
| `internal/browser` | Platform-independent `ProfileManager` (create/open/close/delete, locks, crash recovery) |
| `internal/platform/windows` | WebView2 host + Win32 locks + storage inspector |
| `internal/config` | Atomic JSON app configuration store (`config.json`) |
| `app.go` / `main.go` | Wails facade: thin bindings, asset serving, legacy-mode redirect, window embedding |

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
