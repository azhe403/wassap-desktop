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
```

- Account IDs are 26-char lowercase Crockford Base32 ULIDs (time-sortable,
  path-safe by `browser.ParseProfileID` rules).
- Each account's browser state lives only in its own `profiles/<ulid>/`
  directory. Deleting one account moves its directory to `runtime/trash/`,
  purges it, and leaves other profiles untouched.

## Package Map

| Package | Responsibility |
|---|---|
| `internal/appdirs` | Resolve + ensure `~/.local/share|config|cache/wassap-desktop` |
| `internal/account` | `Account` model, ULID generation, name validation, `AccountService` |
| `internal/storage` | `JSONAccountRepository` — atomic, concurrent-safe JSON persistence |
| `internal/browser` | Platform-independent `ProfileManager` (create/open/close/delete, locks, crash recovery) |
| `internal/platform/windows` | WebView2 host + Win32 locks + storage inspector |
| `app.go` / `main.go` | Wails facade: thin bindings, asset serving, legacy-mode redirect |

## Account Lifecycle

```text
CreateAccount(name)
  -> validate name, generate ULID
  -> ProfileManager.Create(id)        profile dir on disk
  -> repository.Save(account)         accounts.json
  -> on save failure: ProfileManager.Delete(id)  (no orphan profiles)

RemoveAccount(id)
  -> reject if profile open/busy (ErrAccountBusy)
  -> ProfileManager.Delete(id)
  -> repository.Delete(id)
```
