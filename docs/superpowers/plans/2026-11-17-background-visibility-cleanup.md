# Background Session Visibility + Shell Profile Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make hidden WebView2 sessions controller-invisible so Chromium throttles them like background tabs (messages/notifications keep flowing), and delete stale `wails-shell/run-*` user-data folders at startup.

**Architecture:** Two independent small changes. (1) `Session.Hide`/`Session.Show` in `internal/platform/windows/webview2/host.go` additionally call the WebView2 controller visibility (`PutIsVisible`, exposed by go-webview2 as `Chromium.Hide()`/`Chromium.Show()`), best-effort with logging. (2) A new `shell_cleanup.go` in `package main` removes `run-<pid>` folders under `dirs.Data/wails-shell` whose pid is not the current process; called from `main()` before `wails.Run`.

**Tech Stack:** Go (Windows-first, cross-compile clean), Wails v2.17.0, github.com/wailsapp/go-webview2 v1.0.22.

**Spec:** `docs/superpowers/specs/2026-11-17-background-visibility-cleanup-design.md`

## Global Constraints

- Hard requirement from the user: hidden accounts MUST keep receiving WhatsApp Web messages and notifications — no `TrySuspend`, no disposal, no engine teardown for background sessions.
- No new config keys, no UI changes, no new dependencies.
- Controller visibility is best-effort: failures are logged, never fatal, never block the switch path.
- Show must re-instate full rendering (`PutIsVisible(true)`); Hide must not tear anything down.
- Cleanup must never delete the current process's `run-<pid>` folder.
- Every `run-*` deletion failure is logged and swallowed (WebView2 may hold handles on crashed sessions).
- Tests stay green: `go test ./...`.

## Review Focus

1. **Hide path leaves page visible** — user expects hidden accounts to stop burning CPU/GPU but not to go offline. Pinned by Task 1's verification checklist against `GetResourceUsage()` and a live notification; unit-testable only to the point of wiring (no real engine in tests).
2. **Show path forgets `PutIsVisible(true)`** — user expects switching back to a hidden account to restore full rendering instantly; a missing Show leaves the page throttled forever. Pinned by Task 1 (same checklist: switch back, verify full render restored) and code review.
3. **Cleanup deletes the folder in use by a live browser process from a previous run** — user expects the app to never corrupt a running WebView2 profile. Deletion failures are already swallowed; pinned by Task 2's test asserting non-current-pid folders are attempted and current-pid folder is never touched, plus spec note that best-effort is acceptable.
4. **Cleanup runs before `appdirs.Ensure`** (folder may not exist) — user expects no crash on first run. Pinned by Task 2's missing-directory test (no error, no panic).
5. **Cross-compile breakage on non-Windows platforms** — CI/other platforms must still build; pinned by no new build tags (cleanup is pure stdlib path handling in `package main`) and `GOOS=linux go build ./...` in verification.

---

### Task 1: Controller visibility for hidden sessions

**Files:**
- Modify: `internal/platform/windows/webview2/host.go:575-601` (`Session.Hide` and `Session.Show`)
- Test: `internal/platform/windows/webview2/visibility_windows_test.go` (read for style; no new unit test — requires a live engine)

**Interfaces:**
- Consumes: `Session.chromium` (type `*edge.Chromium`, set in `setUpWebView`; may be nil only before setup completes), existing `hideWindow(hwnd)`/`showWindow(hwnd)` Win32 helpers in `win32_windows.go`.
- Produces: `Session.Hide()` additionally hides the WebView2 controller; `Session.Show()` additionally shows it. No signature changes.

- [ ] **Step 1: Modify `Session.Show`** to call `s.chromium.Show()` after `showWindow(s.hwnd)`, guarded by `s.chromium != nil`; log via `log.Printf` on error (do not return it):

```go
if s.chromium != nil {
    if err := s.chromium.Show(); err != nil {
        log.Printf("webview2: cannot mark controller visible for %s: %v", s.profile, err)
    }
}
```

Place the call inside the existing `s.call(func() error { ... })` closure alongside the existing `setForeground` block, before the `Focus()` call.

- [ ] **Step 2: Modify `Session.Hide`** to call `s.chromium.Hide()` after `hideWindow(s.hwnd)`, same guard and log-and-continue style:

```go
if s.chromium != nil {
    if err := s.chromium.Hide(); err != nil {
        log.Printf("webview2: cannot mark controller hidden for %s: %v", s.profile, err)
    }
}
```

- [ ] **Step 3: Build and run the Go test suite**

Run: `go build ./...; go test ./...`
Expected: build succeeds, all tests PASS.

- [ ] **Step 4: Manual verification (dev build, 2 accounts)**

1. `wails dev`, open two accounts, switch A → B.
2. Call `GetResourceUsage()` (frontend devtools or stats page) — account A's CPU drops materially while hidden.
3. From a phone, send a WhatsApp message to account A — desktop notification still fires while B is active.
4. Switch back to A — page restores full rendering (no stale throttling, no blank surface).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/windows/webview2/host.go
git commit -m "feat(webview2): mark controller hidden for background sessions"
```

---

### Task 2: Stale shell `run-*` profile cleanup at startup

**Files:**
- Create: `shell_cleanup.go` (package main)
- Create: `shell_cleanup_test.go` (package main)
- Modify: `main.go` (`main()`: call `cleanupStaleShellProfiles` after `appdirs.Ensure`, before the log file is opened)

**Interfaces:**
- Consumes: `appdirs.Dirs` (only `dirs.Data string`), `os.Getpid()`.
- Produces: `cleanupStaleShellProfiles(dataDir string, selfPID int) error` — removes every `run-*` directory directly under `filepath.Join(dataDir, "wails-shell")` whose numeric suffix differs from `selfPID`; skips (does not fail) entries that are not directories or do not match `run-<digits>`; returns nil unless the shell base directory cannot be read for a reason other than "not exist".

- [ ] **Step 1: Write the failing test** in `shell_cleanup_test.go`

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupStaleShellProfiles(t *testing.T) {
	cases := []struct {
		name    string
		setup   []string // relative paths to create under the wails-shell dir
		selfPID int
		wantGone   []string
		wantStay   []string
		wantErr    bool
	}{
		{
			name:    "stale pid dir is removed, current pid dir stays",
			setup:   []string{"run-111", "run-222"},
			selfPID: 222,
			wantGone: []string{"run-111"},
			wantStay: []string{"run-222"},
		},
		{
			name:    "non run-* entries are untouched",
			setup:   []string{"keep.txt", "run-notanumber"},
			selfPID: 222,
			wantStay: []string{"keep.txt", "run-notanumber"},
		},
		{
			name:    "missing wails-shell directory is not an error",
			setup:   nil,
			selfPID: 222,
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			shellDir := filepath.Join(root, "wails-shell")
			for _, rel := range tc.setup {
				if filepath.Ext(rel) == ".txt" {
					os.MkdirAll(shellDir, 0o700)
					os.WriteFile(filepath.Join(shellDir, rel), []byte("x"), 0o600)
					continue
				}
				os.MkdirAll(filepath.Join(shellDir, rel), 0o700)
			}
			err := cleanupStaleShellProfiles(root, tc.selfPID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			for _, rel := range tc.wantGone {
				if _, statErr := os.Stat(filepath.Join(shellDir, rel)); !os.IsNotExist(statErr) {
					t.Errorf("expected %s to be removed", rel)
				}
			}
			for _, rel := range tc.wantStay {
				if _, statErr := os.Stat(filepath.Join(shellDir, rel)); statErr != nil {
					t.Errorf("expected %s to stay: %v", rel, statErr)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestCleanupStaleShellProfiles -v`
Expected: FAIL — `undefined: cleanupStaleShellProfiles`

- [ ] **Step 3: Implement `cleanupStaleShellProfiles` in `shell_cleanup.go`**

Signature: `func cleanupStaleShellProfiles(dataDir string, selfPID int) error`. Read `dataDir/wails-shell`; if `os.IsNotExist`, return nil. For each entry: skip unless it is a directory matching `^run-([0-9]+)$` with pid != selfPID; `os.RemoveAll` it; on error log `log.Printf("shell-cleanup: cannot remove %s: %v", path, err)` and continue. Return nil in all such cases (best-effort per spec); return a non-nil error only if the base directory cannot be read.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test . -run TestCleanupStaleShellProfiles -v`
Expected: PASS

- [ ] **Step 5: Wire into `main()` in `main.go`**

After the `appdirs.Ensure(dirs)` block (before the log-file open, so removal is "one of the first things we do"), add:

```go
if err := cleanupStaleShellProfiles(dirs.Data, os.Getpid()); err != nil {
    log.Printf("shell-cleanup: %v", err)
}
```

(`log` is already imported; `os` is already imported.)

- [ ] **Step 6: Full verification**

Run: `go test ./...; GOOS=linux go build ./...`
Expected: all tests PASS; linux build succeeds (no platform-specific leakage).

- [ ] **Step 7: Commit**

```bash
git add shell_cleanup.go shell_cleanup_test.go main.go
git commit -m "feat: sweep stale wails-shell run-* profiles at startup"
```

---

### Task 3: Commit the design and plan docs

**Files:**
- Commit: `docs/superpowers/specs/2026-11-17-background-visibility-cleanup-design.md`
- Commit: `docs/superpowers/plans/2026-11-17-background-visibility-cleanup.md`

- [ ] **Step 1: Commit the docs**

```bash
git add docs/superpowers/specs/2026-11-17-background-visibility-cleanup-design.md docs/superpowers/plans/2026-11-17-background-visibility-cleanup.md
git commit -m "docs: add background visibility + shell cleanup design and plan"
```
