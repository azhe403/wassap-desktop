# Pecut 2 — implementation plan

Findings from the Wails v2.17.0 / go-webview2 v1.0.22 source inspection that drive
this plan:

- `options.App.Windows.WebviewUserDataPath` is read exactly once, on the single
  `edge.Chromium` instance Wails creates in `Frontend.Run`. One Wails process =
  one WebView2 environment = one user data folder. Wails cannot re-point it.
- `webviewloader` exposes `WithAdditionalBrowserArguments` (and
  `WithExclusiveUserDataFolderAccess`), but `edge.Chromium` neither passes nor
  exposes them, and `webviewloader.option` is unexported. Named profiles are
  unreachable through Wails.
- The Wails UI goroutine is not the Win32 message-loop thread (`winc.RunMainLoop`
  spins without `runtime.LockOSThread`), so `WindowSetSize`/`WindowSetPosition`
  cannot steer an embedded WebView2 window reliably.
- `webviewloader.GetAvailableCoreWebView2BrowserVersionString("")` is the
  supported runtime probe: it returns an empty string when WebView2 is missing.

Steps:

- [ ] 1. `internal/browser`: `ProfileManager` interface, `ProfileID` validation,
      layout, disk-backed `Manager`.
- [ ] 2. `internal/browser`: process marker + PID liveness, profile lock files,
      cross-process exclusivity probes for a user data folder.
- [ ] 3. `internal/platform/windows/webview2`: `Host`, `Window`, `Session` built on
      `wailsapp/go-webview2` (promoted to a direct dependency, not forked).
- [ ] 4. Headless integration test driving the real WebView2 runtime twice against
      one profile to prove persistence and deletion isolation.
- [ ] 5. Root package: PoC app with Open A / Open B / Close / Open A+B / Delete,
      wrapping the legacy single-profile UDF so Pecut 1 keeps working.
- [ ] 6. Docs: README, Pecut 2 report, POC manifest + gate for Windows Credential
      Manager verification of ISO A / ISO B.
- [ ] 7. `go vet ./...`, `go test ./...`, `wails build`, report.
