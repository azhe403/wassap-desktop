# Pecut 3 — Account Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a robust account management domain model, atomic JSON persistence, service orchestration with browser profile isolation rollback, and Wails application bindings.

**Architecture:** A domain-driven layered architecture where `internal/account` defines the pure domain model, validation, ULID generation, and service interface; `internal/storage` implements atomic JSON persistence with concurrency safety; `internal/browser` provides profile isolation via `ManagerInterface`; and `app.go` exposes thin Wails bindings while preserving Pecut 1 and Pecut 2 features.

**Tech Stack:** Go 1.26, Wails v2.17.0, JSON, Crockford Base32 ULID, sync/mutex, atomic filesystem operations (temp file rename).

**Spec:** Pecut 3 — Account Management specification (prompt).

## Global Constraints

- Storage location: `~/.local/share/wassap-desktop/accounts.json` (`filepath.Join(dirs.Data, "accounts.json")`).
- Account ID: 26-character lowercase Crockford Base32 ULID, immutable, safe for filesystem path (`browser.ParseProfileID`).
- Name: Editable, validated (non-empty trimmed, max 100 characters, no control characters).
- No cookies, session tokens, or browser data stored in account metadata.
- JSON repository must use atomic file replacement and handle missing files gracefully.
- Corrupted `accounts.json` must return an error without silently overwriting or wiping existing files.
- Thread-safe repository read/write operations.
- Account ordering must be preserved as created.
- Profile creation rollback: If persistence fails during account creation, the created browser profile must be purged immediately to prevent orphan profiles.
- Profile deletion safety: Active/open browser profiles must not be deleted.
- No final account sidebar UI yet.
- Preserve Pecut 1 legacy mode (`--pecut1`) and Pecut 2 test interface.
- Zero unnecessary dependencies.

## Review Focus

1. *Corrupted accounts.json file on disk*: Startup or repository load must fail with an explicit error and NEVER overwrite or erase corrupted data with an empty list.
2. *Concurrent reads and writes to repository*: Repository methods must be guarded against race conditions when multiple calls occur simultaneously.
3. *Rollback on persistence failure during Create*: If `ProfileManager.Create` succeeds but `Repository.Save` fails, the profile directory must be cleaned up via `ProfileManager.Delete` without leaving an orphan profile.
4. *Active profile deletion guard*: Attempting to remove an account whose profile is open/busy must fail and leave both profile storage and account metadata intact.
5. *Case-sensitivity and path safety of ULID*: ULID generated must be lowercase to be 100% compliant with `browser.ParseProfileID` validation rules.

---

### Task 1: Account Domain Model, ULID Generator, and Validation

**Files:**
- Create: `internal/account/account.go`
- Create: `internal/account/ulid.go`
- Create: `internal/account/validation.go`
- Test: `internal/account/account_test.go`

**Interfaces:**
- Produces:
  - `type Account struct { ID string, Name string, CreatedAt time.Time, UpdatedAt time.Time }`
  - `func NewAccount(name string) (*Account, error)`
  - `func ValidateName(name string) (string, error)`
  - `func NewULID() (string, error)`
  - `func ParseULID(raw string) (string, error)`
  - Errors: `ErrInvalidAccountID`, `ErrInvalidAccountName`

- [ ] **Step 1: Write the failing unit tests for Account, ULID, and Validation**

Write tests in `internal/account/account_test.go`:
- `TestNewULID_Valid`: Generates 26-character lowercase Crockford Base32 string, monotonic timestamp order.
- `TestParseULID_Validation`: Rejects uppercase, wrong length, non-base32 chars.
- `TestValidateName`: Rejects empty, whitespace-only, overly long (>100 chars), strings with control chars, and accepts trimmed valid names.
- `TestNewAccount`: Creates Account with valid lowercase ULID, valid Name, non-zero CreatedAt and UpdatedAt.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/account`
Expected: FAIL (package does not exist yet)

- [ ] **Step 3: Implement ULID generator, Validation, and Account model**

Implement:
- `internal/account/ulid.go`: Standard 128-bit ULID (48-bit millisecond timestamp + 80-bit crypto/rand entropy), encoded with Crockford Base32 alphabet (`0123456789abcdefghjkmnpqrstvwxyz`, lowercase).
- `internal/account/validation.go`: Validate account name (trim whitespace, check 1..100 chars, ensure no unicode control/invalid chars), and validate ULID.
- `internal/account/account.go`: `Account` struct with JSON tags `id`, `name`, `createdAt`, `updatedAt`, `NewAccount(name string) (*Account, error)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/account`
Expected: PASS

- [ ] **Step 5: Verify compatibility with `browser.ParseProfileID`**

Write a test verifying that `browser.ParseProfileID(account.ID)` returns no error for generated accounts.
Run: `go test -v ./internal/account`
Expected: PASS

---

### Task 2: Account Repository & JSON Persistence

**Files:**
- Create: `internal/account/repository.go`
- Create: `internal/storage/account_json_repository.go`
- Test: `internal/storage/account_json_repository_test.go`

**Interfaces:**
- Consumes: `Account` from `internal/account`
- Produces:
  - `internal/account.Repository` interface:
    - `List() ([]Account, error)`
    - `Get(id string) (*Account, error)`
    - `Save(acc Account) error`
    - `Delete(id string) error`
  - `internal/storage.NewJSONAccountRepository(filePath string) (*JSONAccountRepository, error)`
  - Errors: `internal/account.ErrAccountNotFound`, `internal/account.ErrDuplicateAccountID`, `internal/account.ErrCorruptData`

- [ ] **Step 1: Write the failing tests for JSONAccountRepository**

Write tests in `internal/storage/account_json_repository_test.go`:
- `TestJSONRepository_EmptyFile`: Missing file returns empty account slice without error.
- `TestJSONRepository_SaveAndGet`: Saves accounts, preserves order, retrieves by ID.
- `TestJSONRepository_Update`: Saving an existing account updates it in place without altering list position or ID.
- `TestJSONRepository_Delete`: Deletes account by ID, returns `ErrAccountNotFound` for non-existent ID.
- `TestJSONRepository_AtomicWrite`: Proves file writes are atomic (no partial/corrupt files left).
- `TestJSONRepository_CorruptedJSON`: Proves malformed JSON returns `ErrCorruptData` and does not overwrite existing file on disk.
- `TestJSONRepository_ConcurrentAccess`: Concurrent reads and writes with goroutines do not race.
- `TestJSONRepository_AutoCreateDirectory`: Proves parent directories are automatically created if they don't exist.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/storage`
Expected: FAIL

- [ ] **Step 3: Implement JSONAccountRepository**

Implement:
- `internal/account/repository.go`: `Repository` interface and repository errors (`ErrAccountNotFound`, `ErrDuplicateAccountID`, `ErrCorruptData`).
- `internal/storage/account_json_repository.go`:
  - `sync.RWMutex` protecting in-memory slice and file I/O.
  - Startup loader: if file does not exist, return empty slice. If file contains invalid JSON, return wrapped `ErrCorruptData`.
  - Atomic saver: write indented JSON to `filepath.Join(dir, "accounts.json.tmp."+random)`, `file.Sync()`, `file.Close()`, `os.Rename(tmp, target)`.
  - Directory creation with `os.MkdirAll(dir, 0o700)`.
  - Order preservation in slice.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/storage`
Expected: PASS

---

### Task 3: Account Service & ProfileManager Integration

**Files:**
- Create: `internal/account/service.go`
- Test: `internal/account/service_test.go`

**Interfaces:**
- Consumes:
  - `internal/account.Repository`
  - `internal/browser.ManagerInterface` (or equivalent ProfileManager interface)
- Produces:
  - `internal/account.Service` interface:
    - `List() ([]Account, error)`
    - `Get(id string) (*Account, error)`
    - `Create(name string) (*Account, error)`
    - `Rename(id string, name string) error`
    - `Remove(id string) error`
  - `internal/account.NewService(repo Repository, profiles browser.ManagerInterface) Service`
  - Errors: `ErrAccountNotFound`, `ErrAccountBusy`, `ErrProfileCreationFailed`, `ErrProfileDeletionFailed`

- [ ] **Step 1: Write mock ProfileManager and failing unit tests for AccountService**

Write tests in `internal/account/service_test.go`:
- `TestService_CreateSuccess`: Validates name, generates ULID, calls `profiles.Create(id)`, saves to repo, returns account.
- `TestService_CreateInvalidName`: Rejects invalid names without calling profile manager or repo.
- `TestService_CreateProfileFailure`: If `profiles.Create(id)` fails, repo is not touched and error is returned.
- `TestService_CreatePersistenceFailure_Rollback`: If repo save fails, `profiles.Delete(id)` is called to roll back profile directory so no orphaned profile remains.
- `TestService_List`: Returns all accounts from repo.
- `TestService_Get`: Returns account by ID, or `ErrAccountNotFound`.
- `TestService_Rename`: Updates name and UpdatedAt timestamp, preserves ID, updates repo.
- `TestService_RenameNotFound`: Returns `ErrAccountNotFound` for unknown ID.
- `TestService_RemoveSuccess`: Finds account, ensures profile closed/deleted via `profiles.Delete(id)`, removes from repo.
- `TestService_RemoveActiveProfile_Refused`: If profile is currently open/busy (mock returns `browser.ErrProfileOpen`), `Remove` fails, profile is not deleted, metadata is NOT removed.
- `TestService_RemoveProfileDeletionFailure`: If `profiles.Delete` fails with an unexpected error, metadata is NOT removed from repo.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/account -run TestService`
Expected: FAIL

- [ ] **Step 3: Implement AccountService**

Implement `internal/account/service.go`:
- Validation and ULID generation during `Create`.
- Orchestration: `profiles.Create(id)` -> `repo.Save(acc)`. On repo error, roll back with `profiles.Delete(id)`.
- `Rename(id, name)`: Validates new name, checks account exists, updates `Name` and `UpdatedAt = time.Now()`, saves via `repo.Save(acc)`.
- `Remove(id)`: Checks account exists. Calls `profiles.Delete(id)`. If `profiles.Delete` returns `ErrProfileOpen` or error, abort and return `ErrAccountBusy` or wrapped error. Then remove from `repo.Delete(id)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/account -run TestService`
Expected: PASS

---

### Task 4: Wails Application Integration & End-to-End Persistence

**Files:**
- Modify: `app.go`
- Modify: `main.go`
- Test: `app_test.go`

**Interfaces:**
- Produces Wails bindings on `*App`:
  - `ListAccounts() ([]account.Account, error)`
  - `GetAccount(id string) (*account.Account, error)`
  - `CreateAccount(name string) (*account.Account, error)`
  - `RenameAccount(id string, name string) error`
  - `RemoveAccount(id string) error`

- [ ] **Step 1: Write integration tests in `app_test.go`**

Add tests to `app_test.go`:
- `TestApp_AccountManagementLifecycle`:
  - Call `CreateAccount("Work Account")` -> verify returned account and profile created.
  - Call `ListAccounts()` -> verify list contains "Work Account".
  - Call `RenameAccount(id, "Personal Account")` -> verify name updated.
  - Call `GetAccount(id)` -> verify updated account returned.
  - Simulate app restart with same `dirs`: create new App instance -> call `ListAccounts()` -> verify "Personal Account" persists across restart!
  - Call `RemoveAccount(id)` -> verify account removed from repository and profile deleted.
  - Verify legacy Pecut 1 and Pecut 2 methods still compile and pass.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestApp_AccountManagementLifecycle`
Expected: FAIL (`CreateAccount` not defined on `App`)

- [ ] **Step 3: Wire AccountService and Repository into App**

In `app.go`:
- Add `accountService account.Service` field to `App`.
- In `NewApp(dirs appdirs.Dirs, legacyMode bool)`:
  - Construct `storage.NewJSONAccountRepository(filepath.Join(dirs.Data, "accounts.json"))`.
  - Construct `account.NewService(repo, manager)`.
- Expose thin delegate methods:
  - `ListAccounts() ([]account.Account, error)`
  - `GetAccount(id string) (*account.Account, error)`
  - `CreateAccount(name string) (*account.Account, error)`
  - `RenameAccount(id string, name string) error`
  - `RemoveAccount(id string) error`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -run TestApp`
Expected: PASS

---

### Task 5: Full Test Suite, Build Verification & Documentation

**Files:**
- Modify: `README.md`
- Create: `docs/specs/pecut-3-account-management.md`

- [ ] **Step 1: Run all tests in the workspace**

Run: `go test -v ./...`
Expected: ALL PASS

- [ ] **Step 2: Run `go vet`**

Run: `go vet ./...`
Expected: No warnings

- [ ] **Step 3: Verify Wails build**

Run: `wails build` (or `go build -o build/bin/wassap-desktop.exe .`)
Expected: Clean compilation and build success

- [ ] **Step 4: Update Documentation**

Update `README.md` and create `docs/specs/pecut-3-account-management.md` documenting:
- Account domain architecture and ULID format.
- JSON repository storage and atomic write mechanics.
- AccountService lifecycle and rollback behavior.
- Wails bindings exposed.
- Verification results.
