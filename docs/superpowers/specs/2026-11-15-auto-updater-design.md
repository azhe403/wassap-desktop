# Wassap Desktop Auto Updater — Design

**Date:** 2026-11-15
**Status:** Approved by user 2026-11-15 ("gas")
**Scope:** CI release workflow (version stamping), `internal/updater` (new package), `internal/config` (new fields), `app.go` (bindings + startup hook), frontend Settings/About + update notice, `main.go` (startup sweep)

---

## 1. Intent & Why

Wassap Desktop is distributed as raw binaries on GitHub Releases (repo `azhe403/wassap-desktop`): a rolling pre-release `nightly` updated on every push to `main`, and stable releases from `v*` tags. Users who want the newest build today must download and replace the exe by hand.

This design adds an **opt-in auto updater**: it checks GitHub Releases for a newer build, can download it in the background, and applies it on restart via a rename-aside swap performed by the running binary itself. No separate updater project or helper binary.

Public behavior committed to by the user (2026-11-15):

- **Auto-update is OFF by default.** The app never downloads anything a user did not enable or click.
- **Channel defaults to `stable`.** The user can switch to `nightly` in Settings.
- When auto-update is ON: new version is downloaded silently in the background, and **"Restart to update"** applies the update and relaunches the app automatically (via a relauncher inside the same binary — see §7.1). The app never restarts itself without an explicit user action.
- When auto-update is OFF (default): if a newer version exists, show it in **Settings → About/Software Update** with an **"Update"** button, plus a small badge dot on the sidebar entry. The notice never downloads anything by itself and never renders over an open WhatsApp session.
- Channel semantics follow an **ahead-only** rule (user decision, 2026-11-15): an update is offered only when the target ref (the stable release tag, or `main` for nightly) contains commits the running build does not. The updater never installs an older build; switching channels may simply offer nothing.
- Phase 1 support: **Windows** (amd64 + arm64). Linux/macOS report "unsupported" from the updater in this phase; the code is structured so they can be enabled later.

## 2. Goals / Non-goals / Anti-goals

**Goals**
- Opt-in auto update with a stable/nightly channel switch.
- Background download + user-confirmed restart-to-apply.
- Survive partial failure: verify digest before apply; roll back the rename if the swap fails; never leave the app unstartable.
- Zero extra projects, binaries, or CI artifacts. Only `-ldflags` are added to the existing build.

**Non-goals**
- Linux/macOS apply paths (check may report unsupported; Phase 1 Windows-only).
- Delta patches / differential updates.
- Code signing. CI does not sign today; SmartScreen may warn on the replaced exe. Known limitation, not addressed here.
- Silent self-restart. Never happens.
- Downgrade support. Switching channels never installs an older build; the user reinstalls manually if they want that.

**Anti-goals**
- No telemetry, no analytics.
- No update checks when the running build carries no version stamp (dev builds): the check stays completely silent.
- No update from anywhere except the official GitHub Releases of this repo.

## 3. Constraints

- Distro is raw `.exe` downloaded from Releases — no installer, no Add/Remove Programs, no guaranteed install location. The exe may live in any folder, including a non-writable one (e.g. `Program Files`). The design must degrade to a clear error in that case.
- GitHub Releases assets already exist and are correctly named:
  - `wassap-desktop-windows-amd64.exe`
  - `wassap-desktop-windows-arm64.exe`
  - (Linux/macOS assets exist but are out of scope for apply in Phase 1.)
- GitHub releases API, unauthenticated: 60 requests/hour per IP. Throttle to one check per 12h; user-initiated "Check now" bypasses the throttle.
- The app currently has **no version stamping**. Update detection is impossible without it, so stamping lands first.
- Frontend is vanilla JS modules under `frontend/src/`; Wails bindings are called as `window.go.main.App.*`; no test runner exists for the frontend.

## 4. Approaches (decided)

| # | Approach | Trade-off | Verdict |
|---|----------|-----------|---------|
| A | **Rename-aside swap in-process**: running exe renames itself to `*.old-<pid>`, the staged new exe is copied into place, new process spawned, old file swept on next launch. Rollback = rename back on failure. | Small, no new artifacts; Windows locks writes but not renames on a running exe. Requires the exe folder to be user-writable. | **Chosen** |
| B | Separate `updater.exe` helper that waits for exit, swaps, relaunches | Robust swap; but a second binary in the CI matrix, AV-suspicious file-replacer behavior, Linux/macOS still need their own paths | Rejected for Phase 1 |
| C | Move to NSIS/Inno installer + silent installer update | Standard UX, Add/Remove Programs; but rewrites the distribution model (CI rework, makensis), far beyond an auto updater | Rejected; possible future project |

Reason for A: it is the smallest change that fully covers the Windows replace-binary case and matches the existing raw-binary release model. (User decision, 2026-11-15.)

## 5. Version stamping (prerequisite)

Two package-level vars in `main.go`:

```go
var version = "dev"  // -X main.version=...
var commit  = ""     // -X main.commit=...
```

CI changes in `.github/workflows/build.yml`:

- On tag `v*`: `-ldflags "-X main.version=${TAG#v} -X main.commit=$GITHUB_SHA"` (e.g. `0.2.0`).
- On `main` (nightly): `-ldflags "-X main.version=0.0.0-nightly.${GITHUB_SHA::7} -X main.commit=$GITHUB_SHA"`.
- Local `wails build` / `wails dev` without ldflags: `version` stays `"dev"`; the updater never acts on dev builds.

About view replaces the hardcoded `v0.4.0` with a new binding `GetVersion()` returning `{version, commit}` — the About text is currently wrong regardless (tag is v0.1.0), so this fixes a stale claim rather than introducing drift.

## 6. Architecture

```
                 GitHub Releases API (public)
        stable: GET /repos/azhe403/wassap-desktop/releases/latest
        nightly: GET /repos/azhe403/wassap-desktop/releases/tags/nightly
        nightly freshness: GET /compare/<commit>...main (ahead_by > 0 ⇒ new build)
                        │ HTTPS, 10s timeout, User-Agent: wassap-desktop
                        ▼
┌────────────────────────────────────────────────────────┐
│                  internal/updater                      │
│  Checker   – channel selection, ahead-only offer gate,  │
│              throttle, cache.json                       │
│  Assets    – platform/arch → asset name                │
│  Download  – temp+rename into ~/.cache/wassap-desktop/ │
│              updates/, SHA-256 vs asset digest          │
│  Apply     – pre-stage .new-<pid> + two-rename swap    │
│  Relaunch  – hidden --relaunch-after-update <pid> mode │
│  Sweep     – restore-if-exe-missing + remove *.old-*   │
└───────┬───────────────────────────▲────────────────────┘
        │ bound calls               │ wails EventsEmit("update:status")
        ▼                           │
┌────────────────────────────────────────────────────────┐
│                 Frontend (vanilla JS)                  │
│  Settings → "Software Update" group:                   │
│    toggle auto-update (default OFF)                    │
│    channel select: stable (default) / nightly          │
│    "Check for updates" button + status line            │
│  About: real version via GetVersion()                  │
│  Notice: Settings/About status row + sidebar badge dot  │
│    OFF  → "Update available" + Update / Dismiss buttons│
│    ON   → hidden until download completes:             │
│           sidebar badge + "Restart to update" row      │
│  NO floating pill: the DOM is covered by native       │
│  WebView2 HWNDs during sessions, so any DOM pill is    │
│  invisible where users actually are. Notice lives in  │
│  Settings/About + a sidebar badge dot.                │
└────────────────────────────────────────────────────────┘
```

## 7. Components

### 7.1 `internal/updater` package

- **Semver** (`semver.go`): lenient compare. Strip leading `v`, strip `+` build metadata; compare numeric core per segment; a stable tag outranks a pre-release on the same core; numeric pre-release suffixes compare numerically (`nightly.12` > `nightly.9`). Unparsable segments degrade to 0 rather than erroring. `IsNewer(candidate, current)`. Dev builds (`version == "dev"`) short-circuit to "no update".
- **Checker** (`checker.go`): `Check(ctx)` (throttled) and `CheckNow(ctx)` (user-initiated, bypasses throttle). Channel from config:
  - `stable`: use `/releases/latest` to find the stable tag and its compare ref; update available iff `ahead_by > 0` for `/compare/<commit>...<tag>`. If the running build has no embedded commit (dev/unstamped), stay silent.
  - `nightly`: use `/releases/tags/nightly` for the download asset + URL, and `/compare/<commit>...main` for freshness: update available iff `ahead_by > 0`.
  - **Offer gate is `ahead_by` for both channels** (user decision, 2026-11-15): the updater never offers a build whose content is not ahead of the running commit — no downgrades through channel switches, ever. Semver compare remains for display and tag parsing only. AHEAD-only comparison needs the embedded `commit`, which both CI stamps provide.
  - If a compare call fails, fail-soft to the cached status.
  - Throttle: one check per 12h (persisted `checked_at`); startup check honored only if cache is stale. On error, retry no sooner than 1h.
  - Result cached at `~/.cache/wassap-desktop/update-check.json`:
    `{checkedAt, channel, currentVersion, latestVersion, updateAvailable, releaseUrl, publishedAt}` — safe write via temp + rename; corrupt/missing cache degrades to "unknown".
- **Assets** (`assets.go`): `windows/amd64 → wassap-desktop-windows-amd64.exe`, `windows/arm64 → wassap-desktop-windows-arm64.exe`. Anything else → `unsupported`.
- **Download** (`download.go`): streams to `~/.cache/wassap-desktop/updates/<asset>.part` then renames to `<asset>`. Verifies SHA-256 against the GitHub asset `digest` field (`sha256:<hex>`) when present; mismatch → delete, hard error. Absent digest → log warning, proceed (HTTPS from the official repo is the transport trust anchor). Checks free disk space first via `golang.org/x/sys/windows.GetDiskFreeSpaceEx` when on Windows; refusal below 2× the asset size. (x/sys is already an indirect dep; promote to direct if needed.)
- **Apply** (`apply_windows.go`, `apply_other.go`): three phases, all inside the running binary.
  1. **Pre-stage**: copy the staged file to `<exedir>/<name>.new-<pid>` (same directory, same volume) and re-verify its SHA-256. The old exe stays intact and running throughout.
  2. **Relaunch**: spawn `<exe> --relaunch-after-update <own pid>` detached. The relauncher is the same binary in a hidden mode (no new project, no new artifact): it waits for the parent process to exit fully (`OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject`), then starts `<exe>` with no arguments and exits. Waiting for full process exit — not just for swap start — means the WebView2 teardown in `OnShutdown` has completed and the new instance opens the profiles after the old one released them.
  3. **Swap**: `os.Rename(exe, exe.old-<pid>)` then `os.Rename(<name>.new-<pid>, exe)` — two renames, no copy between them, so the exe is missing only for the nanoseconds between two metadata operations. On failure of the first rename: delete `.new-*`, return the error, app keeps running. On failure of the second: rename `.old-*` back. Then force-quit via the existing `forceQuit`/`quitApp` path, which bypasses the close-to-tray veto in `onBeforeClose`.
- **Sweep** (`sweep.go`): on startup, (a) if `<exedir>/<name>` is missing and exactly one `<name>.old-*` exists next to it, rename it back (crash between the two apply renames) — log a warning; (b) otherwise remove `<name>.old-*` older than 1 hour. Best-effort, errors logged and ignored.
- **Relauncher** (`relaunch_other.go` handles non-Windows as no-op/unsupported): `main.go` checks `--relaunch-after-update <pid>` before doing anything else. When set, it waits for `<pid>` to exit (`OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject`, capped at 60s), starts `os.Executable()` with no args, and exits — it never reaches `wails.Run` and never touches the single-instance lock, sidestepping the Wails mutex race entirely.

### 7.2 Config (`internal/config`)

`AppConfig` gains:

```go
AutoUpdate    bool   `json:"autoUpdate"`              // default false
UpdateChannel string `json:"updateChannel,omitempty"` // default "stable"
```

`Store` gains `GetAutoUpdate/SetAutoUpdate`, `GetUpdateChannel/SetUpdateChannel` (validates `stable|nightly`). Missing `updateChannel` in an existing file → `stable`. Existing config files need no migration.

### 7.3 App bindings (`app.go`)

- `GetVersion() (VersionInfo, error)` — `{version, commit}`.
- `GetUpdateStatus() (UpdateStatus, error)` — cached check result + current download state (`idle|downloading|downloaded|applying|error|unsupported`) + progress (bytes/total) when downloading.
- `GetAutoUpdate() bool` / `SetAutoUpdate(enabled bool) error`.
- `GetUpdateChannel() string` / `SetUpdateChannel(channel string) error`.
- `CheckForUpdates() (UpdateStatus, error)` — user-initiated, bypasses throttle.
- `DownloadUpdate() error` — starts background download (no-op if none available / already downloading). Mirrors what the auto path does.
- `RestartToApplyUpdate() error` — pre-stages `.new-<pid>`, spawns the relauncher (`--relaunch-after-update <pid>`), performs the two-rename swap, force-quits via the `forceQuit` path.
- `DismissUpdateNotice(version string) error` — persists a per-version dismissal (stored in config-adjacent JSON or the update cache; per-version; a newer version re-arms the notice).

Startup hook (`startup`): after context is saved, kick off `go a.updater.Start(ctx)` which (a) sweeps stale `.old-*` files, (b) if cache stale, performs a throttled check in the background, (c) starts a 12h ticker, (d) on every status change, `wailsRuntime.EventsEmit(ctx, "update:status", status)`. All paths fail-soft: log and continue.

If auto-update is ON and a check finds an update, the checker auto-starts the download and emits `downloading` → `downloaded`. If OFF, it only emits `available` and downloads nothing.

### 7.4 Frontend

- **Settings → Software Update** (new group, same `settings-group` pattern):
  - Toggle "Auto-update" (default off) with description: checks for new versions and downloads them automatically; installs on restart.
  - Channel `<select>`: `Stable (recommended)` / `Nightly (bleeding edge, built from main)`.
  - Row: current version (from `GetVersion()`), channel, last checked time, status line, and a "Check for updates" button.
- **About**: version text bound to `GetVersion()` instead of hardcoded.
- **Notice** — user decision (2026-11-15): **no floating pill**. The DOM is covered by native WebView2 child HWNDs while a WhatsApp session is open, so a DOM overlay is invisible exactly when it matters. The notice lives in two places only:
  1. **Settings → About/Software Update status row**: shows current version, channel, last check, and context action — [Update] when `available` (OFF path), [Restart to update] when `downloaded`, or a progress line while `downloading`. Update → `DownloadUpdate()`; Restart → `RestartToApplyUpdate()`.
  2. **Sidebar badge**: a small dot on the Settings/sidebar entry whenever `available` or `downloaded` is active and not dismissed — the only ambient signal, visible because the sidebar is always DOM-owned chrome. Driven by `EventsOn("update:status")`.
  - Dismiss is per-version (`DismissUpdateNotice`); a newer version re-arms both the row action and the badge.
  - No changes to account/session/WebView2 visibility code.

### 7.5 Release workflow

`.github/workflows/build.yml`: compute `LDFLAGS` per matrix entry:

```bash
REF="${GITHUB_REF}"
if [[ "$GITHUB_REF" == refs/tags/v* ]]; then VER="${GITHUB_REF_NAME#v}";
else VER="0.0.0-nightly.${GITHUB_SHA::7}"; fi
wails build ... -ldflags "-X main.version=${VER} -X main.commit=${GITHUB_SHA}"
```

(Windows jobs run in bash via git-bash on the runner, so the same shell snippet works; or a tiny `Compute ldflags` step with pwsh — implementer's call, keep one dialect.)

## 8. Data flow

**Check (automatic, throttled)** → cache write → `EventsEmit(update:status)` → Settings status row + sidebar badge re-render.
**Auto-update ON**: `available` → checker starts `DownloadUpdate` itself → `downloading` (progress ticks every ~256 KiB or 250 ms) → `downloaded` → Settings row offers "Restart to update", badge stays on.
**Auto-update OFF**: `available` → Settings row offers [Update] [Dismiss]; nothing downloads until user clicks.
**Apply**: `RestartToApplyUpdate` → pre-stage `.new-<pid>` (copy + verify) → spawn relauncher → two-rename swap → `forceQuit` → parent exits after WebView2 teardown → relauncher observes full process death → starts `<exe>` (new version, no args) → new instance acquires the single-instance lock with zero contention.
**Bootstrap**: builds ≤ v0.1.0 have no updater — their first update is manual via the Releases page. README gains an "Updating" section stating this once.

## 9. Failure modes & mitigations

| Failure | Behavior |
|---|---|
| No network / GitHub 403/429 | Last cached status served; nothing surfaced as an error dialog; retry ≥ 1h |
| Corrupt cache / missing digest | Cache → "unknown"; digest absent → warning + proceed; digest mismatch → hard fail, file deleted |
| Exe folder not writable (e.g. Program Files) | Swap fails cleanly, error names the cause and the Releases fallback; app keeps running |
| Crash mid-apply | The missing-exe case is closed: startup sweep renames the matching `.old-*` back if `<name>` is absent. Between the two renames no file copy happens, so the window is nanoseconds and always recoverable on next launch |
| New binary is broken | Same exposure as the raw-download path; `update-check.json` records what was applied; user reinstalls from Releases |
| Two updates race | Pre-stage + atomic rename swap; a second click sees either `downloaded` (no-op) or an in-progress state; single-instance lock serializes app instances |
| Nightly freshness compare diverges after force-push | `ahead_by > 0` may stay true; acceptable, user can always switch channel |
| Downgrade via channel switch | Never possible: the offer gate is `ahead_by > 0` for BOTH channels (user decision, 2026-11-15) |

## 10. Testing

Go unit tests (`internal/updater/*_test.go`, package `updater`):
- Apply: pre-stage copy → verify → two-rename swap → rollback paths (first-rename failure cleans `.new-*`; second-rename failure restores `.old-*`). Sweep: restore-if-exe-missing + `.old-*` cleanup.
- Relauncher: `--relaunch-after-update <pid>` waits for exit, spawns new exe, never reaches `wails.Run`. Tested with a real child process.
- Channel selection: ahead-only gate for stable and nightly; a running nightly whose history contains the stable tag must NOT offer that tag; ahead of main offers nightly; dev build short-circuits.
- Throttle: fresh cache → no HTTP call; stale/forced (`CheckNow`) → HTTP call; corrupt cache → unknown, no panic.
- Assets: platform/arch mapping; unsupported on other GOOS.
- Download: digest match/mismatch (httptest server), `.part` cleanup on failure.
- Config: new fields round-trip; channel validation rejects junk; missing field → defaults.

Manual verification checklist (in PR description):
- Fresh install of stamped build: About shows real version; Settings shows Software Update; auto-update default off; channel default stable.
- Check now against stable release: reports correctly; Settings row + sidebar badge appear when the target ref is ahead; dismiss suppresses until something newer is ahead.
- Enable auto-update on a machine with an older stamped build: download completes; Settings row offers "Restart to update"; restart applies and relaunches automatically; new build reports new version; `.old-*` swept.
- Force a failing swap (folder made read-only): error is actionable; old exe still runs; `.new-*` cleaned up.
- Nightly channel on a commit behind main: update offered; switch back to stable: no downgrade offered (ahead-only gate).

## 11. Explicit calls made by the author (not user-decided)

- Ahead-only offer gate for both channels via `/compare/<commit>...<ref>` `ahead_by > 0` (stable ref = tag from `/releases/latest`, nightly ref = `main`) — user decision, 2026-11-15. Semver compare is display/tag-parsing only.
- Apply mechanics: pre-stage `.new-<pid>` + two-rename swap + startup sweep restore-if-exe-missing — author's call, closes the crash window the builder check found.
- Same-binary relauncher `--relaunch-after-update <pid>` waiting for full parent process exit (not a helper project) — user decision, 2026-11-15.
- Notice surface: Settings/About row + sidebar badge, no floating pill over WebView2 sessions — user decision, 2026-11-15.
- 12h throttle + 1h backoff on error; jitter on startup check.
- Update cache lives at `~/.cache/wassap-desktop/update-check.json`; dismissals are per-version in the same cache file.
- Download directory `~/.cache/wassap-desktop/updates/`.
- Free-disk check 2× asset size, Windows-only, via `golang.org/x/sys/windows`.
- Apply quits via the existing `forceQuit`/`quitApp` path (bypasses close-to-tray); the app never self-restarts outside this user-confirmed path.
- Phase 1 ships with Linux/macOS reporting `unsupported`; no apply path there.
- No code signing; SmartScreen friction documented as known limitation.
- First update for ≤ v0.1.0 users is manual (bootstrap problem, unavoidable).

## 12. Left to the builder

- Exact UI copy and the sidebar badge's styling/placement.
- Progress event granularity and debounce.
- Whether the `Compute ldflags` step is bash or pwsh (keep one shell dialect in CI).
- Whether `DismissUpdateNotice` persists inside `update-check.json` or alongside it.
