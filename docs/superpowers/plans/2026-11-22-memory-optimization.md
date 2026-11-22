# Memory Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce memory footprint of Wassap Desktop WebView2 sessions via Chromium low-memory flags and Win32 background working set trimming.

**Architecture:** 
1. In `internal/platform/windows/webview2/win32_windows.go`, configure `c.AdditionalBrowserArgs` with low-memory Chromium switches.
2. In `internal/platform/procstats`, implement `TrimProcessWorkingSet(pid uint32)` and `TrimUserDataWorkingSet(userDataDir string)` for Windows, with cross-platform stubs for other OSes.
3. In `internal/platform/windows/webview2/host.go`, trigger `TrimUserDataWorkingSet(s.path)` when a session is hidden in `Session.Hide()`.

**Tech Stack:** Go, Win32 PSAPI (`EmptyWorkingSet`), Chromium / WebView2 (`AdditionalBrowserArgs`).

**Spec:** `docs/superpowers/specs/2026-11-22-memory-optimization-design.md`

## Global Constraints

- Never break background message reception or notifications for hidden accounts.
- Cross-platform compile cleanliness: builds on Windows and Linux (`//go:build windows` and `//go:build !windows`).
- All existing tests pass (`go test ./...`).
- Non-blocking: trimming must not delay the Win32 UI switch thread.

## Review Focus

1. **Non-Windows compile failure**: Non-Windows platforms must compile without unresolved symbols. Pinned by Task 2 tests and `GOOS=linux go build ./...`.
2. **Invalid handle / access denied on protected PIDs**: `TrimProcessWorkingSet` must handle OpenProcess failure gracefully and return/swallow error. Pinned by Task 2 unit tests.
3. **Empty or missing user data directory**: `TrimUserDataWorkingSet("")` must safely no-op without scanning or trimming arbitrary processes. Pinned by Task 2 unit tests.
4. **Race with immediate switch-back**: If user switches back before/during trim, `Show()` restores full rendering and pages fault back in seamlessly without error.
5. **Chromium argument formatting**: Arguments must match Chromium switch syntax (strings without quotes).

---

### Task 1: Add low-memory Chromium arguments to WebView2 sessions

**Files:**
- Modify: `internal/platform/windows/webview2/win32_windows.go:275-296`
- Test: `internal/platform/windows/webview2/win32_windows.go` (verified via `go test ./...`)

**Interfaces:**
- Consumes: `c.AdditionalBrowserArgs []string` on `edge.Chromium`.
- Produces: `AdditionalBrowserArgs` populated with `--enable-low-end-device-mode`, `--js-flags=--max-old-space-size=384`, and `--renderer-process-limit=1`.

- [ ] **Step 1: Update `setUpWebView` in `win32_windows.go` to set `AdditionalBrowserArgs`**

```go
c.AdditionalBrowserArgs = []string{
    "--enable-low-end-device-mode",
    "--js-flags=--max-old-space-size=384",
    "--renderer-process-limit=1",
}
```

- [ ] **Step 2: Verify build and test**

Run: `go test ./internal/platform/windows/webview2/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/platform/windows/webview2/win32_windows.go
git commit -m "perf(webview2): pass low-memory flags to chromium sessions"
```

---

### Task 2: Implement Working Set Trimming in `procstats`

**Files:**
- Modify: `internal/platform/procstats/procstats_windows.go`
- Modify: `internal/platform/procstats/procstats_other.go`
- Modify: `internal/platform/procstats/procstats_windows_test.go`

**Interfaces:**
- Produces:
  - `TrimProcessWorkingSet(pid uint32) error`
  - `TrimUserDataWorkingSet(userDataDir string) (int, error)`

- [ ] **Step 1: Write test for `TrimProcessWorkingSet` and `TrimUserDataWorkingSet`**

In `procstats_windows_test.go`:
- Test trimming self process `TrimProcessWorkingSet(uint32(os.Getpid()))` succeeds.
- Test `TrimUserDataWorkingSet("")` safely returns 0 without error.

- [ ] **Step 2: Run test to see it fail (symbols not defined)**

Run: `go test ./internal/platform/procstats`
Expected: compile error / undefined symbols.

- [ ] **Step 3: Implement `TrimProcessWorkingSet` and `TrimUserDataWorkingSet`**

In `procstats_windows.go`:
- Bind `procEmptyWorkingSet = psapi.NewProc("EmptyWorkingSet")`.
- Implement `TrimProcessWorkingSet(pid uint32) error`.
- Implement `TrimUserDataWorkingSet(userDataDir string) (int, error)` using `Snapshot()` to match processes containing `userDataDir` in their command line.

In `procstats_other.go`:
- Implement stub `TrimProcessWorkingSet(pid uint32) error { return nil }`.
- Implement stub `TrimUserDataWorkingSet(userDataDir string) (int, error) { return 0, nil }`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/platform/procstats`
Expected: PASS

- [ ] **Step 5: Verify cross-platform compilation**

Run: `GOOS=linux go build ./internal/platform/procstats/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/platform/procstats/procstats_windows.go internal/platform/procstats/procstats_other.go internal/platform/procstats/procstats_windows_test.go
git commit -m "feat(procstats): add working set trimming for webview processes"
```

---

### Task 3: Hook Working Set Trimming to `Session.Hide`

**Files:**
- Modify: `internal/platform/windows/webview2/host.go:603-614` (`Session.Hide`)

**Interfaces:**
- Consumes: `procstats.TrimUserDataWorkingSet(s.path)`

- [ ] **Step 1: Trigger `TrimUserDataWorkingSet` in `Session.Hide()`**

In `host.go`:
```go
if s.path != "" {
    go func(dir string) {
        time.Sleep(500 * time.Millisecond)
        if count, err := procstats.TrimUserDataWorkingSet(dir); err == nil && count > 0 {
            log.Printf("webview2: trimmed working set for %s (%d processes)", s.profile, count)
        }
    }(s.path)
}
```

- [ ] **Step 2: Run all tests and cross-compile check**

Run: `go test ./...`
Run: `GOOS=linux go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/platform/windows/webview2/host.go
git commit -m "perf(webview2): trim background session working set on hide"
```
