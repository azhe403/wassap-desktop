# Pecut 4 — Account Switcher UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the account sidebar and account switching experience inside a single desktop window, integrating with the existing AccountService and ProfileManager to switch between isolated WebView2 browser profiles without separate desktop windows, iframes, or reverse proxies.

**Architecture:** One main desktop window running Wails v2.17.0. The left region is a fixed-width vertical sidebar rendered by Wails (Vanilla HTML/CSS/JS). The right region is the content area where the active account's isolated WhatsApp Web session is embedded as a native Win32 child window (`WS_CHILD`) hosting `edge.Chromium`. Account switching safely deactivates and closes the previous WebView2 environment, locks/releases profiles cleanly, activates the requested account's isolated User Data Folder (`profiles/<id>/EBWebView`), and updates persistent active account configuration in `~/.local/config/wassap-desktop/config.json`.

**Tech Stack:** Go 1.26, Wails v2.17.0, `github.com/wailsapp/go-webview2` v1.0.22, Win32 User32/Kernel32 APIs, Vanilla HTML5, CSS3, JavaScript (ES modules).

**Spec:** Pecut 4 — Account Switcher UI

## Global Constraints

- Exactly one main desktop window; switching must never create additional top-level desktop windows or taskbar entries.
- One active WebView at a time for WhatsApp Web; previous account session must be cleanly deactivated and closed before new session opens.
- Complete profile isolation: each account retains its dedicated User Data Folder (`profiles/<id>/EBWebView`) with separate cookies, LocalStorage LevelDB, and cache.
- No embedding WhatsApp Web inside an iframe.
- No reverse proxy; navigation must be direct to `https://web.whatsapp.com`.
- No React, Vue, or heavy UI frameworks; use existing Vanilla HTML/CSS/JS frontend stack.
- No Python anywhere in scripts, tooling, or tests.
- Standalone config persistence for active account ID in `~/.local/config/wassap-desktop/config.json` with atomic writes.
- Never persist browser session data or credentials in application configuration.
- Preserve backward compatibility for Pecut 1 (`--pecut1`) and Pecut 2 profile isolation tests.

## Review Focus

1. **Active account deleted from another session or external edit:** If `config.json` references an account ID that no longer exists in `accounts.json`, the app must fall back gracefully to the first available account, or no active account if empty, without crashing or throwing unhandled errors.
2. **Concurrent switching requests:** If the user rapidly clicks Account B then Account C while switching is in progress, the second request must be rejected or queued safely to avoid race conditions or dual WebView2 initialization in the child window.
3. **Attempt to delete currently active account:** Removing an active account must be blocked by the UI and backend with a clear message requiring the user to switch to another account first.
4. **WebView visibility during modal dialogs:** When opening "Add Account" or "Rename" dialogs, the native child WebView2 (which sits above web content in Win32 z-order) must be hidden or managed so it does not occlude the modal or intercept its keyboard and mouse focus.
5. **Window resizing synchronization:** When the main desktop window is resized or maximized, the native child window must synchronously resize to fill `(x = sidebarWidth, y = 0, width = clientWidth - sidebarWidth, height = clientHeight)` without flicker or visual tearing.

---

### Task 1: Active Account Configuration & Persistence

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `internal/appdirs/appdirs.go` (ensure config path helpers)

**Interfaces:**
- Consumes: `appdirs.Dirs.Config`
- Produces:
  ```go
  type AppConfig struct {
      ActiveAccountID string `json:"activeAccountId"`
  }
  type Store interface {
      Load() (*AppConfig, error)
      Save(cfg *AppConfig) error
      SetActiveAccount(accountID string) error
      GetActiveAccount() (string, error)
  }
  func NewStore(configPath string) Store
  ```

- [ ] **Step 1: Write failing unit tests for ConfigStore**

Write tests in `internal/config/config_test.go` checking:
- Default config when file does not exist (returns empty `ActiveAccountID` without error).
- Saving and loading `ActiveAccountID`.
- Atomic write guarantee (temporary file replaced via `os.Rename`).
- Handling corrupt JSON (returns error without panic).
- Directory auto-creation when `config` directory does not yet exist.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/config`
Expected: FAIL with package not found or undefined types.

- [ ] **Step 3: Implement `internal/config/config.go`**

Implement `Store` and `JSONStore` using `sync.RWMutex`, atomic file write via tempfile + `Sync()` + `os.Rename`, and clean error handling.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/config`
Expected: PASS.

---

### Task 2: Host Child Window & Native Embedding Support

**Files:**
- Modify: `internal/platform/windows/webview2/host.go`
- Modify: `internal/platform/windows/webview2/win32_windows.go`
- Modify: `internal/platform/windows/webview2/host_integration_test.go`

**Interfaces:**
- Consumes: Win32 User32 APIs (`CreateWindowExW`, `SetParent`, `SetWindowPos`, `ShowWindow`, `GetClientRect`).
- Produces:
  ```go
  type Host struct {
      ...
      ParentHWND   uintptr
      SidebarWidth int
      ...
  }
  func (h *Host) SetParentWindow(hwnd uintptr, sidebarWidth int)
  func (h *Host) UpdateBounds(width, height int) error
  func (h *Host) SetVisible(visible bool) error
  ```

- [ ] **Step 1: Write unit test verifying Host with ParentHWND creates WS_CHILD window**

In `internal/platform/windows/webview2/host_integration_test.go`:
- Test that when `ParentHWND != 0`, `createWindow()` creates a child window (`WS_CHILD | WS_VISIBLE | WS_CLIPSIBLINGS | WS_CLIPCHILDREN`) positioned at `x = sidebarWidth, y = 0`.
- Test that when `ParentHWND == 0`, `createWindow()` retains backward-compatible `WS_OVERLAPPEDWINDOW` behavior for Pecut 2 tests.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestHost_ChildWindow ./internal/platform/windows/webview2`
Expected: FAIL with missing fields or methods.

- [ ] **Step 3: Implement child window creation and layout management**

In `win32_windows.go` and `host.go`:
- Update `Session.createWindow()` to branch on `s.host.ParentHWND`. If `ParentHWND != 0`, use style `WS_CHILD | WS_VISIBLE | WS_CLIPSIBLINGS | WS_CLIPCHILDREN`, parent `s.host.ParentHWND`, and bounds derived from `s.host.SidebarWidth` and `GetClientRect(parentHWND)`.
- Implement `UpdateBounds(width, height int)` on `Host` and `Session` using `SetWindowPos` and `s.chromium.Resize()`.
- Implement `SetVisible(visible bool)` on `Host` and `Session` using `ShowWindow(SW_SHOW / SW_HIDE)` and `controller.SetIsVisible`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/platform/windows/webview2`
Expected: PASS (both new child window test and existing isolation tests pass).

---

### Task 3: App Layer Integration & Account Switching Orchestration

**Files:**
- Modify: `app.go`
- Modify: `app_windows.go`
- Modify: `app_other.go`
- Modify: `app_test.go`

**Interfaces:**
- Consumes: `internal/account/service.go`, `internal/config/config.go`, `internal/browser/profile_manager.go`, `internal/platform/windows/webview2/host.go`
- Produces new Wails bindings:
  ```go
  func (a *App) GetActiveAccountID() string
  func (a *App) SwitchAccount(accountID string) error
  func (a *App) SetWebViewVisible(visible bool) error
  func (a *App) NotifyWindowResize(width, height int) error
  ```

- [ ] **Step 1: Write failing integration tests for App switching and active account persistence**

In `app_test.go`:
- Test initial startup loads `activeAccountId` from config.
- Test fallback to first account when saved `activeAccountId` was deleted.
- Test `SwitchAccount(id)` validates existence, closes previous session, opens new session, updates `activeAccountId`, and saves to config.
- Test `SwitchAccount(id)` rejects duplicate or concurrent switching requests with clear error.
- Test removing active account is rejected until switched.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestApp_SwitchAccount ./...`
Expected: FAIL.

- [ ] **Step 3: Implement orchestration in `app.go` and `app_windows.go`**

- Initialize `config.Store` in `NewApp`.
- Detect main Wails HWND on startup in `app_windows.go` and bind it to `Host.SetParentWindow(mainHwnd, 68)`.
- Implement `SwitchAccount` with mutex protection, switching state tracking, graceful deactivation of current session, activation of new profile, and persistence of `activeAccountId`.
- Implement `RemoveAccount` check: if `id == a.currentProfile`, return error `account is currently active; switch to another account first`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./...`
Expected: PASS.

---

### Task 4: Frontend UI — Sidebar Layout, Avatars, Badges & Empty State

**Files:**
- Modify: `frontend/index.html`
- Modify: `frontend/src/app.css`
- Modify: `frontend/src/style.css`
- Create: `frontend/src/components/sidebar.js`
- Create: `frontend/src/utils/avatar.js`

**Interfaces:**
- Produces:
  ```javascript
  // avatar.js
  export function getInitials(name);
  export function getAvatarColor(nameOrId);
  // sidebar.js
  export function renderSidebar(accounts, activeId, switchingId, callbacks);
  export function renderEmptyState(container, onAddAccount);
  ```

- [ ] **Step 1: Implement avatar initials and deterministic color generator**

Create `frontend/src/utils/avatar.js`:
- Extract 1-2 uppercase letters from account name (e.g., "Personal Account" -> "PA", "Work" -> "W").
- Compute deterministic, accessible background hue from account name/ID with readable contrast.

- [ ] **Step 2: Implement desktop-first CSS layout in `frontend/src/app.css`**

- Set layout: fixed left sidebar (width: 68px, background: `#111b21` or theme neutral, vertical column, flex).
- Account avatar buttons: 48x48px rounded-full with smooth hover, active vertical bar/pill indicator on the left border.
- Unread indicator badge placeholder (e.g. green dot or count badge).
- Bottom group: Add Account (`+`) button with tooltip, Settings (`⚙`) button.
- Content area: `#content-area` occupying `calc(100vw - 68px)`.
- Clean empty state view with illustration/icon, welcoming text, and "+ Add Account" CTA when account list is empty.

- [ ] **Step 3: Implement sidebar rendering logic**

In `frontend/src/components/sidebar.js`:
- Render account items preserving order.
- Set `aria-current="page"` and active class on active account.
- Show spinner/pulsing indicator if account is switching.
- Render tooltip with account display name on hover.

---

### Task 5: Frontend UI — Modal Dialogs, Context Menu & Accessibility

**Files:**
- Create: `frontend/src/components/modal.js`
- Create: `frontend/src/components/context-menu.js`
- Modify: `frontend/src/app.css`

**Interfaces:**
- Produces:
  ```javascript
  export function showAddAccountModal({ onConfirm, onCancel });
  export function showRenameModal({ currentName, onConfirm, onCancel });
  export function showConfirmModal({ title, message, onConfirm, onCancel });
  export function showContextMenu({ x, y, account, onRename, onRemove });
  ```

- [ ] **Step 1: Implement accessible modal dialog component**

In `frontend/src/components/modal.js`:
- Trap keyboard focus inside dialog (Tab, Shift+Tab).
- Close on Escape or cancel button.
- Validate non-empty name before submitting.
- Call `api.SetWebViewVisible(false)` when modal opens, and `api.SetWebViewVisible(true)` when modal closes, ensuring dialog is 100% visible and responsive.

- [ ] **Step 2: Implement custom right-click context menu**

In `frontend/src/components/context-menu.js`:
- Position context menu near clicked avatar without overflowing viewport.
- Options: "Rename Account" and "Remove Account".
- Close when clicking outside or pressing Escape.
- Keyboard navigation (ArrowUp, ArrowDown, Enter).

- [ ] **Step 3: Connect Rename and Remove flows**

- Rename: open modal, validate name, call `api.RenameAccount(id, newName)`, refresh sidebar.
- Remove: if active, show error toast/modal "Please switch to another account before removing this one." Otherwise, show confirm dialog, call `api.RemoveAccount(id)`, refresh sidebar.

---

### Task 6: Frontend UI — Account Switching Flow & Lifecycle Coordination

**Files:**
- Modify: `frontend/src/main.js`
- Create: `frontend/src/state.js`
- Modify: `frontend/index.html`

**Interfaces:**
- Produces centralized application state:
  ```javascript
  // state.js
  export const state = {
      accounts: [],
      activeAccountId: null,
      switchingAccountId: null,
      isLoading: true,
      error: null,
  };
  ```

- [ ] **Step 1: Implement state management and switching coordinator**

In `frontend/src/state.js` and `frontend/src/main.js`:
- On app load:
  1. Fetch `ListAccounts()`.
  2. Fetch `GetActiveAccountID()`.
  3. If accounts exist and `activeAccountId` is valid, mark as active.
  4. If `activeAccountId` is empty but accounts exist, backend auto-selects first account.
  5. If accounts list is empty, show empty state in content area.
- On account avatar click:
  1. If clicked account is already active or switching is in progress, ignore.
  2. Set `switchingAccountId = targetId`, update sidebar visually.
  3. Call `api.SwitchAccount(targetId)`.
  4. On success: `activeAccountId = targetId`, `switchingAccountId = null`, re-render sidebar.
  5. On failure: revert `switchingAccountId = null`, display error banner/toast.

- [ ] **Step 2: Connect window resize observer**

In `frontend/src/main.js`:
- Observe window resize events and notify backend via `api.NotifyWindowResize(width, height)` with debouncing so the native child WebView2 matches the content area dimensions.

---

### Task 7: Comprehensive Verification & Automated Tests

**Files:**
- Modify: `app_test.go`
- Modify: `internal/account/service_test.go`
- Modify: `internal/storage/repo_test.go`

- [ ] **Step 1: Run complete Go test suite**

Run: `go test -v ./...`
Verify:
- Account creation, list, rename, delete tests pass.
- Active account persistence and fallback tests pass.
- Concurrent switching prevention tests pass.
- Child window creation and bounds tests pass.
- Profile isolation and storage tests pass.

- [ ] **Step 2: Test frontend build with Vite / npm**

Run: `cd frontend && npm run build`
Verify:
- Clean build with zero warnings or errors.
- Bundle output in `frontend/dist/`.

- [ ] **Step 3: Test Wails build**

Run: `wails build`
Verify:
- Production binary builds cleanly.

---

### Task 8: Documentation & Architecture Update

**Files:**
- Modify: `README.md`
- Modify: `docs/architecture.md`
- Create: `docs/specs/pecut-4-account-switcher-ui.md`

- [ ] **Step 1: Update `docs/architecture.md` with Pecut 4 architecture**

Document:
- Main window layout with native child window embedding.
- Single desktop window architecture and Win32 child window lifecycle.
- Active account persistence schema in `~/.local/config/wassap-desktop/config.json`.
- Account switching sequence and profile isolation guarantees.

- [ ] **Step 2: Update `README.md` with usage instructions**

- Add instructions for running Pecut 4 account switcher.
- Document keyboard shortcuts and account management workflows.

- [ ] **Step 3: Document Pecut 4 report in `docs/specs/pecut-4-account-switcher-ui.md`**

Complete the 8-point report required by the specification.
