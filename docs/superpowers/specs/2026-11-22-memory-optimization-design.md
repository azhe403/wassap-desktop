# Memory Optimization: Chromium Flags & Working Set Trimming - Design

Date: 2026-11-22
Status: Approved

## Intent

Wassap Desktop runs multiple isolated WebView2 environments for WhatsApp Web accounts.
Each WebView2 instance consumes 200MB - 400MB of RAM.
The goal is to reduce RAM footprint significantly without disconnecting background accounts or dropping notifications:

1. **Chromium low-memory flags:** Pass `--enable-low-end-device-mode`, `--js-flags=--max-old-space-size=384`, and `--renderer-process-limit=1` to each account's WebView2 session so Chromium caps heap usage and aggressively collects memory.
2. **Win32 Working Set Trimming:** When a background session is hidden (`Session.Hide()`), trim its processes' working set via Win32 `SetProcessWorkingSetSize(h, ^uintptr(0), ^uintptr(0))` or `EmptyWorkingSet(h)`. Windows pushes idle pages to standby, reducing private RAM by 50-80% while keeping WebSockets and notifications alive.

## Goals

- Configure `AdditionalBrowserArgs` in `internal/platform/windows/webview2/win32_windows.go`.
- Add a helper to trim working set memory for all `msedgewebview2.exe` processes belonging to a given session's user data directory (or process tree) when `Session.Hide()` is invoked.
- Ensure cross-platform compile cleanliness with `//go:build windows` and `//go:build !windows` stubs.
- No regression in WhatsApp Web message delivery, notifications, or switching responsiveness.
- All existing tests pass.

## Non-goals

- Disposing or hibernating background sessions (`TrySuspend` or `Close`), which would break notifications.
- Mutating user settings or adding UI controls for these automatic optimizations.
