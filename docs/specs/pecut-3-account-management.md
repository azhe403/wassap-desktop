# Pecut 3 — Account Management Specification

## Context

Wassap Desktop is a multi-account WhatsApp Web desktop application built with Wails v2.17.0, Go, Vanilla HTML/CSS/JavaScript, and native WebView2 on Windows.

Previous milestones achieved:
- **Pecut 1**: Single WhatsApp Web session with persistent storage under `~/.local/share/wassap-desktop/EBWebView/`.
- **Pecut 2**: Profile isolation proof of concept using separate User Data Folders (UDFs) managed by `ProfileManager`, with automated verification proving full isolation and safe lifecycle operations.

## Objective

Design and implement a maintainable account management system that manages multiple WhatsApp accounts and connects each account to its own isolated browser profile. This milestone focuses strictly on the backend, domain model, persistence, service orchestration, and Wails application bindings. (The final account sidebar UI is deferred to subsequent phases).

---

## 1. Architecture Overview

The system follows clean, layered architecture separating domain rules, storage mechanisms, browser profile isolation, and desktop host bindings:

```text
┌────────────────────────────────────────────────────────┐
│                   Wails Host App                       │
│  (app.go: ListAccounts, GetAccount, CreateAccount, ...) │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│                    Account Service                     │
│               (internal/account/service.go)            │
│  - Name validation & ULID creation                     │
│  - Orchestration & atomic rollback on failure          │
│  - Active profile deletion protection                  │
└──────────────┬──────────────────────────┬──────────────┘
               │                          │
┌──────────────▼──────────────┐ ┌─────────▼──────────────┐
│     Account Repository      │ │     Profile Manager    │
│  (internal/account/repo.go) │ │ (internal/browser/...) │
│  - JSON atomic persistence  │ │ - Separate UDF layout  │
│  - Concurrency safety (RWMutex)│ - Win32 locks & probes │
│  - Ordering preservation    │ │ - Safe disk lifecycle  │
└─────────────────────────────┘ └────────────────────────┘
```

---

## 2. Domain Model & Validation (`internal/account`)

### Account Struct
```go
type Account struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

### Identifier Requirements:
- **ULID (Universally Unique Lexicographically Sortable Identifier)**: 128-bit identifier composed of a 48-bit millisecond timestamp and 80 bits of cryptographic entropy (`crypto/rand`).
- **Encoding**: 26 characters of Crockford Base32 (`0123456789abcdefghjkmnpqrstvwxyz`).
- **Case**: Strictly lowercase to guarantee 100% path safety and conformance with `browser.ParseProfileID` validation rules (`^[a-z0-9]+(-[a-z0-9]+)*$`).
- **Immutability**: `Account.ID` is permanently fixed at creation and never changes during renames or edits.

### Account Name Validation Rules:
- Whitespace is automatically trimmed from both ends.
- Trimmed name must be non-empty (minimum 1 character).
- Maximum length is 100 characters.
- Must not contain ASCII or Unicode control characters (e.g. `\x00`, `\n`, `\r`, etc.).

### Security & Isolation Boundary:
- No session tokens, cookies, auth credentials, or web state are stored in the account metadata.
- All browser-level state resides exclusively in the isolated directory managed by `ProfileManager`.

---

## 3. Account Repository & Storage (`internal/storage`)

### Storage Location
`~/.local/share/wassap-desktop/accounts.json` (resolved via `internal/appdirs`).

### JSON Persistence Schema
```json
[
  {
    "id": "01jar9w8y9e0e37z667gbb0h7w",
    "name": "Personal Account",
    "createdAt": "2026-11-01T08:00:00Z",
    "updatedAt": "2026-11-01T08:00:00Z"
  },
  {
    "id": "01jar9w8y9e0e37z667gbb0h7x",
    "name": "Work Account",
    "createdAt": "2026-11-01T08:05:00Z",
    "updatedAt": "2026-11-01T08:10:00Z"
  }
]
```

### Reliability Guarantees:
1. **Startup Discovery**: Missing files do not cause errors; the repository initializes an empty in-memory slice `[]Account{}`.
2. **Corrupted Data Protection**: If `accounts.json` contains invalid JSON or corrupted bytes, the repository returns `ErrCorruptData` and halts startup without silently wiping or overwriting the file.
3. **Atomic File Replacement**: File writes create a sibling temporary file (`accounts.json.tmp.<rand>`), flush with `file.Sync()`, close the descriptor, and atomically replace the destination via `os.Rename`.
4. **Directory Auto-Creation**: Parent directories are created with `0o700` permissions on demand.
5. **Concurrency Safety**: Synchronized using a `sync.RWMutex` protecting concurrent read and write operations across goroutines.
6. **Order Preservation**: Accounts maintain their original creation order in the slice.

---

## 4. Account Service (`internal/account/service.go`)

### Interface
```go
type Service interface {
    List() ([]Account, error)
    Get(id string) (*Account, error)
    Create(name string) (*Account, error)
    Rename(id string, name string) error
    Remove(id string) error
}
```

### Operation Semantics:

#### 1. `Create(name string) (*Account, error)`
1. Validate `name` using `ValidateName`.
2. Generate a new lowercase ULID.
3. Create the profile directory and bookkeeping via `ProfileManager.Create(id)`.
4. Construct `Account` with UTC timestamps and persist via `Repository.Save(account)`.
5. **Rollback Guarantee**: If `Repository.Save` fails, the service immediately calls `ProfileManager.Delete(id)` to purge the newly created profile directory, ensuring no orphaned profile directories are left on disk.

#### 2. `Rename(id string, name string) error`
1. Validate `name`.
2. Retrieve the existing account from `Repository.Get(id)`. If missing, return `ErrAccountNotFound`.
3. Update `account.Name` and set `account.UpdatedAt = time.Now().UTC()`.
4. Preserve `account.ID`, `account.CreatedAt`, and the underlying browser profile.
5. Persist the updated account via `Repository.Save(account)`.

#### 3. `Remove(id string) error`
1. Verify the account exists via `Repository.Get(id)`.
2. Request profile deletion via `ProfileManager.Delete(id)`.
   - If the profile is open or locked, `ProfileManager.Delete` returns `ErrProfileOpen` / `ErrProfileBusy`. The service aborts immediately with `ErrAccountBusy`.
   - The account metadata is NOT removed if profile deletion fails.
3. Remove account metadata from `Repository.Delete(id)`.

---

## 5. Application Integration (`app.go`)

Expose thin Wails bindings on `*App` delegating directly to `AccountService`:
- `ListAccounts() ([]account.Account, error)`
- `GetAccount(id string) (*account.Account, error)`
- `CreateAccount(name string) (*account.Account, error)`
- `RenameAccount(id string, name string) error`
- `RemoveAccount(id string) error`

Domain logic remains completely decoupled from Wails runtime packages.
Pecut 1 (`--pecut1`) single session redirect and Pecut 2 profile isolation testing methods remain intact and operational.

---

## 6. Testing Strategy

1. **Unit Tests (`internal/account`)**:
   - Monotonic ULID generation and Crockford Base32 encoding.
   - Rejection of invalid IDs, uppercase letters, and non-Crockford characters.
   - Compatibility validation between generated account IDs and `browser.ParseProfileID`.
   - Name trimming, length restrictions, and control-character rejection.
   - Service unit tests with mock `ProfileManager` covering creation rollback on failure and active profile deletion protection.

2. **Storage Tests (`internal/storage`)**:
   - Missing file initialization.
   - Atomic replacement verification.
   - Corrupted file error and preservation.
   - Multi-goroutine concurrent access safety.
   - Auto-creation of nested directory paths.

3. **End-to-End Wails Integration Tests (`app_test.go`)**:
   - Full lifecycle: Create, List, Rename, Get, App restart reload, Remove.
   - Verification that removing one account leaves other accounts and profiles untouched.

---

## 7. Acceptance Criteria Checklist

- [ ] Account model and validation implemented.
- [ ] Lowercase Crockford Base32 ULID generator implemented.
- [ ] Atomic JSON repository implemented under `~/.local/share/wassap-desktop/accounts.json`.
- [ ] `AccountService` implemented with profile creation rollback on storage failure.
- [ ] Active browser profile deletion blocked.
- [ ] Thin Wails bindings exposed on `*App`.
- [ ] Account metadata persists across application restarts.
- [ ] Removing one account does not affect other accounts or profiles.
- [ ] Pecut 1 and Pecut 2 backward compatibility maintained.
- [ ] All unit and integration tests pass cleanly.
