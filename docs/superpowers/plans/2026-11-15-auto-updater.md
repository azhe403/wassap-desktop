# Wassap Desktop Auto Updater Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An opt-in auto updater for Wassap Desktop that detects newer builds on GitHub Releases, downloads them in the background when the user opts in, and applies them on an explicit "Restart to update" via an in-process rename-aside swap with a same-binary relauncher.

**Architecture:** A new `internal/updater` package (GitHub client, channel checker with an ahead-only offer gate, SHA-256-verified downloader, rename-aside applier, startup sweep) stamped with a version/commit via CI `ldflags`, exposed to the vanilla-JS frontend through Wails bindings; the notice surface is the Settings → Software Update row plus a sidebar badge dot — never a floating overlay (native WebView2 HWNDs cover the DOM during sessions).

**Tech Stack:** Go (Wails v2.17.0), `github.com/wailsapp/wails/v2/pkg/runtime`, `golang.org/x/sys/windows`, vanilla JS + CSS for the frontend, GitHub Actions for stamping.

**Spec:** `docs/superpowers/specs/2026-11-15-auto-updater-design.md` — the plan argues from it; read it first.

## Global Constraints

- Auto-update is **OFF by default**; channel defaults to **`stable`**; user may switch to **`nightly`** in Settings.
- Offer gate is **ahead-only for both channels** via `/compare/<commit>...<ref>` `ahead_by > 0` (stable ref = tag from `/releases/latest`, nightly ref = `main`). Never installs older content; channel switches may offer nothing.
- No floating pill / overlay: the DOM is covered by native WebView2 child HWNDs. Notice = Settings → Software Update row + sidebar badge dot only.
- Phase 1: **Windows-only apply** (amd64 + arm64). Linux/macOS return `unsupported` from the updater.
- Relaunch via same-binary `--relaunch-after-update <pid>` hidden mode; it waits for the parent process to **fully exit** before starting the new exe. No helper project or second binary.
- GitHub Releases of `azhe403/wassap-desktop` are the only update source. Unauthenticated API. One check per 12h; on error retry no sooner than 1h; startup check only if cache stale (plus jitter).
- Dev builds without a version stamp (`version == "dev"` / no commit) never act; the check is fully silent.
- No telemetry; the app never self-restarts outside the user-confirmed apply path; `dist` never downloads without explicit user action when auto-update is OFF.
- Repo conventions: conventional commits, one logical change per commit (`feat(updater): …`, `feat(ci): …`, `docs: …`). Go tests live next to the package (`internal/updater/*_test.go`, `internal/config/config_test.go` pattern).
- Version display must come from the `GetVersion()` binding; the hardcoded `v0.4.0` in `settings-page.js` is removed by this plan.

## Review Focus

The five inputs/conditions most likely to bite users, and the task whose tests pin each:

1. **Relauncher spawns the new exe while the old one still holds the mutex or its WebView2 profiles** → new instance dies instantly or opens broken profiles. Pin: `relauncher waits for full parent exit` — test with a real child process (Task 5); plus manual checklist (Task 9).
2. **Crash/power-loss between the two apply renames** → exe gone, app never starts again. Pin: startup sweep renames the matching `.old-*` back when the exe is missing — restore test (Task 5); failure table rewritten in Task 9 docs/spec are already aligned.
3. **Channel flip to `stable` from a nightly that is ahead of the stable tag** → semver would offer a "downgrade". Pin: `ahead_by` gate tests, both directions (Task 3).
4. **Truncated/corrupted download staged as an update** → applied binary is broken. Pin: digest verify at download **and** re-verify before the swap; `.part` deleted on any failure (Tasks 4 and 5).
5. **Corrupt `~/.cache/wassap-desktop/update-check.json` at every startup** → panic loop or GitHub hammering. Pin: corrupt/missing cache degrades to "unknown" with no panic and no HTTP call when cache is fresh (Tasks 2 and 3).

---

### Task 1: CI version stamping + `--version` flag

**Files:**
- Modify: `.github/workflows/build.yml` (all four matrix builds)
- Modify: `main.go` (version vars + `--version` handling at the very top of `main()`)

**Interfaces:**
- Produces: `main.version` (string, default `"dev"`), `main.commit` (string, default `""`) package vars; later tasks read them from `package main`.

- [ ] **Step 1: Add the ldflags computation to the Build step** in `.github/workflows/build.yml`, before the `wails build` line, for every matrix job (single shared snippet works on all runners; on Windows git-bash is the shell):

```bash
if [[ "${GITHUB_REF}" == refs/tags/v* ]]; then
  VER="${GITHUB_REF_NAME#v}"
else
  VER="0.0.0-nightly.${GITHUB_SHA::7}"
fi
echo "LDFLAGS=-X main.version=${VER} -X main.commit=${GITHUB_SHA}" >> "$GITHUB_ENV"
```
  and change the build command to `wails build -platform ${{ matrix.platform }} -o ${{ matrix.name }} ${{ matrix.tags }} -ldflags "${LDFLAGS}"`. `${{ matrix.tags }}` stays untouched.

- [ ] **Step 2: Add the two vars** to `main.go` above `func main()`:

```go
// version and commit are stamped at build time via -ldflags "-X main.version=… -X main.commit=…".
var version = "dev"
var commit = ""
```

- [ ] **Step 3: Add a `--version` flag** at the very top of `main()`, before `flag.Parse()`: if any arg equals `--version` or `-v`, print `wassap-desktop <version> (<commit-or-dev>)` and `os.Exit(0)`. It must run before any directory creation so it is safe on any machine.

- [ ] **Step 4: Verify** — `wails build -platform windows/amd64 -o wassap-desktop.exe -ldflags "-X main.version=0.0.0-test -X main.commit=abc1234"` (or `go build -ldflags … .` if wails CLI is unavailable); run `.\build\bin\wassap-desktop.exe --version`; expected output `wassap-desktop 0.0.0-test (abc1234)`. Also `git diff --check`.

- [ ] **Step 5: Commit** `feat(ci): stamp version and commit into release binaries`

---

### Task 2: `internal/updater` — semver, assets, cache

**Files:**
- Create: `internal/updater/semver.go`, `internal/updater/semver_test.go`
- Create: `internal/updater/assets.go`, `internal/updater/assets_test.go`
- Create: `internal/updater/cache.go`, `internal/updater/cache_test.go`

**Interfaces:**
- Produces: `Compare(a, b string) int`, `IsNewer(candidate, current string) bool`; `AssetName(goos, goarch string) (string, bool)`; `Cache` with `Load() (UpdateCheckCache, error)` / `Store(UpdateCheckCache) error`; `UpdateCheckCache{CheckedAt time.Time, Channel string, CurrentVersion string, LatestVersion string, UpdateAvailable bool, ReleaseURL string, StagedAsset string, StagedDigest string, DismissedVersion string}`.

- [ ] **Step 1: Write failing tests for `Compare`** in `semver_test.go`. Cases (assert sign): `v0.2.0 > v0.1.0`; `v0.1.0 > v0.1.0-pre`; `v0.1.0-pre.12 > v0.1.0-pre.9`; `v0.1.0+build > v0.1.0` is 0; equal with `v` prefix both sides; garbage segments degrade to 0 (no panic).

- [ ] **Step 2: Run** `go test ./internal/updater/ -run TestCompare -v` — expect FAIL (undefined).

- [ ] **Step 3: Implement** `Compare(a, b string) int` and `IsNewer` in `semver.go`: strip leading `v`, split off `+` metadata, split core on `-` for the pre-release suffix; compare numeric core segments; a stable (no-suffix) version outranks a prerelease on the same core; numeric suffix segments compare numerically with fallback to byte compare on non-numeric segments. No panics — unparsable segments count as 0.

- [ ] **Step 4: Write failing tests for assets** in `assets_test.go`: `("windows","amd64") → "wassap-desktop-windows-amd64.exe", true`; arm64 likewise; `("linux","amd64") → ok`; `("darwin","universal"?) → false` (updater unsupported); unknown arch → false.

- [ ] **Step 5: Implement `AssetName(goos, goarch string) (string, bool)`** mapping exactly `windows/amd64→wassap-desktop-windows-amd64.exe`, `windows/arm64→wassap-desktop-windows-arm64.exe`; everything else returns `("", false)`. The spec deliberately leaves Linux/macOS for a later phase.

- [ ] **Step 6: Write failing tests for the cache** in `cache_test.go`: roundtrip write→read preserves all fields incl. `DismissedVersion`; missing file → zero value, `err == nil`; corrupt JSON → zero value, no panic, no error (degrades to unknown); `Store` leaves no `.tmp` files behind in the directory.

- [ ] **Step 7: Implement `cache.go`**: `UpdateCheckCache` struct with `json:"checkedAt,omitempty"` etc.; `Load` reads `filepath` once; missing-or-corrupt → `UpdateCheckCache{}` + nil error; `Store` writes via temp file + rename (same pattern as `internal/config/config.go: saveLocked`, mode 0600). Dismissals: `DismissedVersion string` field; helper `(c UpdateCheckCache) Dismissed(v string) bool { return c.DismissedVersion == v }`.

- [ ] **Step 8: Run all new tests** — `go test ./internal/updater/ -v` — expect PASS.

- [ ] **Step 9: Commit** `feat(updater): semver compare, asset mapping and update-check cache`

---

### Task 3: `internal/updater` — GitHub client + ahead-only checker

**Files:**
- Create: `internal/updater/github.go`, `internal/updater/github_test.go`
- Create: `internal/updater/checker.go`, `internal/updater/checker_test.go`

**Interfaces:**
- Consumes: `Cache`, `UpdateCheckCache`, `IsNewer` (for display only) from Task 2.
- Produces (needed by Tasks 4–7):

```go
type Asset struct{ Name, BrowserDownloadURL, Digest string }
type Release struct{ TagName string; HTMLURL string; PublishedAt time.Time; Assets []Asset }
type Client struct{ /* baseURL string, repo string, hc *http.Client, ua string */ }
func NewClient() *Client                       // baseURL https://api.github.com, repo azhe403/wassap-desktop
func (c *Client) LatestRelease(ctx context.Context) (*Release, error)   // GET /releases/latest; 404 → (nil, nil)
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (*Release, error)
func (c *Client) AheadBy(ctx context.Context, base, head string) (int, error) // GET /compare/base...head → ahead_by

type State string
const ( StateUpToDate State = "up-to-date"; StateAvailable State = "available"; StateUnsupported State = "unsupported"; StateError State = "error" )
type Status struct{ State State; Channel, CurrentVersion, LatestVersion, ReleaseURL, Message string; CheckedAt time.Time; Dismissed bool }
type Prefs struct{ AutoUpdate bool; Channel string }  // Channel ∈ {"stable","nightly"}
type Checker struct{ /* client *Client, cachePath string, version, commit string, now func() time.Time */ }
func NewChecker(client *Client, cachePath, version, commit string) *Checker
func (c *Checker) Check(ctx context.Context, p Prefs, force bool) (Status, error)
// Asset returns the asset matching goos/goarch from the last successful Check, or false.
func (c *Checker) Asset(goos, goarch string) (Asset, bool)
```

- [ ] **Step 1: Write failing httptest-based tests** in `github_test.go`: `LatestRelease` parses tag/url/assets (name, browser_download_url, digest); 404 → nil release; `ReleaseByTag("nightly")` parses the same shape; `AheadBy` parses `ahead_by`; non-200 → error with status code; every request carries `User-Agent: wassap-desktop`.

- [ ] **Step 2: Implement `github.go`**: path-style URL joining (`baseURL + "/repos/" + repo + "/releases/latest"`), 10s timeout, decode into private structs then map to exported ones; `AheadBy` URL is `/compare/<base>...<head>` (three dots); treat HTTP 403/429 as ordinary error.

- [ ] **Step 3: Write failing checker tests** in `checker_test.go` using an `httptest.Server` with a hit counter, plus `now` injection:
  - stable, running commit `abc`, tag `v0.2.0` ahead (`ahead_by: 42`) → `StateAvailable`, `LatestVersion=="v0.2.0"`;
  - stable, tag `v0.2.0` NOT ahead of running commit (`ahead_by: 0` — running nightly contains the tag) → `StateUpToDate`;
  - nightly, `ahead_by` vs `main` is 0 → up-to-date; `> 0` → available, `LatestVersion=="nightly (latest main)"`;
  - dev build (`version=="dev"`) → `StateUnsupported`, and **zero HTTP hits**; unstamped commit (empty) → same;
  - `force=false` + fresh cache (`CheckedAt` now) → no HTTP at all, cached status returned;
  - cache stale → check runs, cache rewritten with new `CheckedAt`;
  - any HTTP failure → `StateError` + cached `UpdateAvailable` value surfaces as the Status if one existed, otherwise `up-to-date` with `Message` set; no panic; retry timestamp respected (a second immediate call does not hit the network again);
  - dismissal: cached `DismissedVersion` equal to resolved latest version → Status.Dismissed true (state stays `available`); different/newer → false.

- [ ] **Step 4: Run** `go test ./internal/updater/ -run 'Checker|GitHub|AheadBy|Latest' -v` — expect FAIL.

- [ ] **Step 5: Implement `checker.go`**: `Check` (1) resolves stamps (`dev`/empty commit → unsupported, no I/O); (2) loads cache; fresh & !force → return cached status; (3) channel branch — stable: `LatestRelease` (nil → up-to-date with Message "no stable release yet") then `AheadBy(commit, tag)`; nightly: `ReleaseByTag("nightly")` for URL+assets, then `AheadBy(commit, "main")`; (4) any error → write nothing except a `CheckedAt=now-1h` throttling marker? **No** — on error return Status with error but do NOT bump `CheckedAt`, return cached values where present; (5) success: build Status, preserve `DismissedVersion` from cache, write cache atomically, AND stash the last-used `*Release` in the Checker so `Asset(goos, goarch)` can return the right file (resolve the asset by calling `AssetName` from Task 2 — that import cycle is fine, same package). Throttle: 12h default (`now-12h` cutoff), injected `now` for tests.

- [ ] **Step 6: Run** the package tests — expect PASS.

- [ ] **Step 7: Commit** `feat(updater): ahead-only stable/nightly checker with throttled cache`

---

### Task 4: `internal/updater` — downloader with SHA-256 verification

**Files:**
- Create: `internal/updater/download.go`, `internal/updater/download_test.go`

**Interfaces:**
- Consumes: `Asset` from Task 3.
- Produces (needed by Tasks 5 and 7):

```go
type DownloadResult struct{ Path, SHA256, AssetName string }
type Downloader struct{ /* … */ }
func NewDownloader(updateDir string) *Downloader       // updateDir e.g. ~/.cache/wassap-desktop/updates
func (d *Downloader) Download(ctx context.Context, asset Asset, progress func(done, total int64)) (DownloadResult, error)
func VerifyFileSHA256(path, want string) error          // want may carry "sha256:" prefix or raw hex
```

- [ ] **Step 1: Write failing tests** in `download_test.go`:
  - httptest serves valid bytes with digest `sha256:<hex>` → file staged at `updateDir/<asset>`, `SHA256` matches, progress invoked with increasing done values ending at total;
  - digest mismatch → error, staged file **and** `.part` both absent;
  - server writes half the body then claims Content-Length mismatch (or connection closes early) → error, `.part` removed, no staged file;
  - digest empty on the Asset → download proceeds (warning logged), result returned without verification;
  - `VerifyFileSHA256` accepts both `sha256:hex` and bare hex; rejects mismatch.

- [ ] **Step 2: Run** `go test ./internal/updater/ -run 'Download|VerifyFile' -v` — expect FAIL.

- [ ] **Step 3: Implement `download.go`**: stream `resp.Body` (10 bytes buffer progress callback throttled to ≥250ms inside the caller or here — here) to `<updateDir>/<name>.part`; after `EOF`, `VerifyFileSHA256` if the Asset digest is non-empty (strip `sha256:` prefix); rename to final path only on success; on ANY error remove both files. Free-disk check: on Windows call `windows.GetDiskFreeSpaceEx`; refuse below 2× expected size (`ContentLength`, fallback 100 MB); behind a `freeBytes func(path string) (uint64, error)` seam defaulting to the platform impl — tests inject a tiny quota to exercise refusal.

- [ ] **Step 4: Run** the tests — expect PASS. **Windows free-disk test runs only under `runtime.GOOS == "windows"`**; other platforms skip.

- [ ] **Step 5: Commit** `feat(updater): verified background downloader with .part staging`

---

### Task 5: `internal/updater` — rename-aside swap, relauncher, startup sweep

**Files:**
- Create: `internal/updater/apply.go`, `internal/updater/apply_test.go`
- Create: `internal/updater/sweep.go`, `internal/updater/sweep_test.go`
- Modify: `main.go` — handle `--relaunch-after-update <pid>` before anything else in `main()`
- Create: `relaunch_windows.go` — `waitForPID(pid int) error` via `OpenProcess(SYNCHRONIZE)` + `WaitForSingleObject`, 60s cap
- Create: `relaunch_other.go` — same signature, returns nil immediately (updater is Windows-only this phase)

**Interfaces:**
- Produces (needed by Task 7):

```go
// CopyStaged copies stagedPath → <dirOf(exePath)>/<exeBase>.new-<pid> and re-verifies the digest.
func CopyStaged(stagedPath, exePath, wantSHA256 string) (newPath string, err error)
// Swap atomically replaces exePath: rename(exe → <exe>.old-<pid>), rename(newPath → exe). rename is a test seam.
func Swap(exePath, newPath string, rename func(a, b string) error) error // on 2nd-rename failure: rollback
// StartupSweep: restore-if-missing, else delete matching <exeBase>.old-* older than 1h.
func StartupSweep(exePath string) error
```

- [ ] **Step 1: Write failing tests** in `apply_test.go`:
  - happy path on tempdir: seed `target.exe` with bytes "old", stage copy of "new" (via `CopyStaged` with a real digest), `Swap` with real rename → `target.exe` == "new", `target.exe.old-<pid>` == "old";
  - first rename fails (injected rename returns error on 1st call) → `.new-*` still removable by caller (assert it exists; caller cleans), exe untouched;
  - second rename fails (injected rename fails on 2nd call) → old restored (`target.exe` == "old"), error returned;
  - `CopyStaged` digest mismatch → error, no `.new-*` file left.
  Tests must never touch `os.Executable()`; always take explicit paths.

- [ ] **Step 2: Run** `go test ./internal/updater/ -run 'Apply|Swap|Staged' -v` — expect FAIL.

- [ ] **Step 3: Implement**: `CopyStaged` streams bytes + computes SHA-256 while copying; caller passes expected digest — signature `CopyStaged(stagedPath, exePath string, wantSHA256 string) (string, error)`; on mismatch remove `.new-*` and error out. `Swap` does `rename(exe, exe.old-<pid>)` then `rename(newPath, exe)`; on 2nd failure: `rename(old, exe)` rollback then return a wrapped error naming the kept `.old-*` path. `pid` = `os.Getpid()`.

- [ ] **Step 4: Sweep tests** in `sweep_test.go`: (a) exe missing + one `app.exe.old-99` present → restored to `app.exe`, warning path; (b) exe present + `.old-*` older than 1h (mtime set via `os.Chtimes`) → deleted; (c) `.old-*` younger than 1h → kept; (d) unrelated files (`other.old-1`, `app.exe.bak`) never touched.

- [ ] **Step 5: Implement `sweep.go`**: match `<exeBase>.old-<number>` in `filepath.Dir(exePath)`; restore exactly one candidate when exe is missing (rename), else sweep stale (>1h). Restore is the crash-window closer.

- [ ] **Step 6: Verify the review-focus scenario with a real child process** — `apply` never self-relaunches; that is `main.go`'s job: alongside the `--version` check from Task 1 (i.e. after it, still before `appdirs.Ensure` and `wails.Run`, and before any directory creation), `if len(os.Args) == 3 && os.Args[1] == "--relaunch-after-update"` → `waitForPID(atoi(Args[2]))`, then `exec.Command(exe).Start()`, `os.Exit(0)`. `relaunch_windows.go` uses `OpenProcess(0x00100000 /* SYNCHRONIZE */)` + `WaitForSingleObject(h, 60_000)`; `relaunch_other.go` exits immediately. Sanity-check manually: spawn `wassap-desktop.exe --relaunch-after-update <currentPid>` from a shell after a harmless process, confirm it starts nothing until the pid exits.

- [ ] **Step 7: Run** all package tests — expect PASS.

- [ ] **Step 8: Commit** `feat(updater): rename-aside swap, startup sweep and relaunch mode`

---

### Task 6: config store — auto-update prefs

**Files:**
- Modify: `internal/config/config.go` (`AppConfig`, `Store` interface, `JSONStore`)
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `GetAutoUpdate() (bool, error)`, `SetAutoUpdate(bool) error`, `GetUpdateChannel() (string, error)`, `SetUpdateChannel(string) error` (validates `stable|nightly`; stores `stable` when the field is absent).

- [ ] **Step 1: Write failing tests**: roundtrip autoUpdate true/false; roundtrip channel `nightly`; `SetUpdateChannel("beta")` → error; a config.json with no `updateChannel` key → `GetUpdateChannel()` returns `"stable"`; existing `closeToTray` preserved on roundtrip.

- [ ] **Step 2: Run** `go test ./internal/config/ -v` — expect FAIL.

- [ ] **Step 3: Implement**: add to `AppConfig`: `AutoUpdate bool json:"autoUpdate"`, `UpdateChannel string json:"updateChannel,omitempty"`. Interface + `JSONStore` implementations following the existing `update`/`GetCloseToTray` pattern verbatim. Validation inside `SetUpdateChannel` returns an error for anything but `stable|nightly`.

- [ ] **Step 4: Run** — expect PASS.

- [ ] **Step 5: Commit** `feat(config): auto-update preference and channel`

---

### Task 7: app bindings + startup wiring

**Files:**
- Modify: `app.go` — new fields on `App` (`updater *updater.Checker`, `downloader *updater.Downloader`, `updateMu sync.Mutex`, `dlState` struct{state string, done, total int64, stagedPath, stagedSHA string, err string}), new bound methods, `startup()` kick, `GetCloseToTray`-style accessors
- Modify: `app_test.go` if it exercises `NewApp` (otherwise create the helper as a free function + test)

**Interfaces:**
- Consumes: everything from Tasks 2–6.
- Produces (frontend contract — do not rename):

```go
type VersionInfo struct{ Version, Commit string }
type UpdateStatus struct{ State, Channel, CurrentVersion, LatestVersion, ReleaseURL, Message string
  CheckedAt string; ProgressDone, ProgressTotal int64; Dismissed bool }
func (a *App) GetVersion() VersionInfo
func (a *App) GetUpdateStatus() UpdateStatus            // last known; never hits network
func (a *App) GetAutoUpdate() bool
func (a *App) SetAutoUpdate(enabled bool) error
func (a *App) GetUpdateChannel() string
func (a *App) SetUpdateChannel(channel string) error
func (a *App) CheckForUpdates() UpdateStatus            // force-checks, updates cache, emits event
func (a *App) DownloadUpdate() error                    // starts bg download if available; else error
func (a *App) RestartToApplyUpdate() error              // full apply; quits only on success
func (a *App) DismissUpdateNotice(version string) error // writes DismissedVersion into cache
```

- [ ] **Step 1: Pure helper + test first**: add `shouldAutoDownload(p config prefs mirror, st updater.Status) bool` as a free function (`autoUpdate && st.State == updater.StateAvailable && !st.Dismissed`); test the 4 truth-table rows in a new `update_decide_test.go`.

- [ ] **Step 2: Wire the checker into `NewApp`**: after `cfgStore` is built, construct `updater.NewChecker(updater.NewClient(), filepath.Join(dirs.Cache, "update-check.json"), version, commit)` and `updater.NewDownloader(filepath.Join(dirs.Cache, "updates"))`. Cache dir needs `os.MkdirAll(dirs.Cache/updates)` — `Ensure` intentionally skips cache, so create it lazily here (best effort).

- [ ] **Step 3: Bound methods** following the `GetCloseToTray` pattern: `GetVersion` reads the two package vars. `GetUpdateStatus` returns `dlState` merged over the last cached check (checker does the load; App caches last `Status` in memory, initialised on first call or from cache at startup). `SetAutoUpdate` persists; if enabled && cached status available && not dismissed → kick `a.startDownload()`. `CheckForUpdates` calls `Check(force=true)` with prefs from the store, merges into `dlState`, emits `update:status`, returns it. `DownloadUpdate` errors unless state is available/idle: resolves the asset via `checker.Asset(runtime.GOOS, runtime.GOARCH)` from the last successful check, starts a goroutine: win-only progress callbacks (≥250ms throttle) emit `update:status` with `State="downloading"`, final emits `State="downloaded"`; on error emits `State="error"` + message. `RestartToApplyUpdate`: (1) reads `dlState.stagedPath`; if empty → error `no staged update` — **before any quit**; (2) `updater.CopyStaged(staged, exe, s.stagedSHA)`; (3) `cmd := exec.Command(exe, "--relaunch-after-update", strconv.Itoa(os.Getpid()))` + `cmd.Start()`; on error → remove `.new-*`, return, **do not quit**; (4) `updater.Swap(exe, newPath, os.Rename)`; on error kill relauncher (`cmd.Process.Kill()`), attempt second-rename rollback (already in `Swap`), remove `.new-*`, return error, **do not quit**; (5) on success set `a.forceQuit = true` and call `a.quitApp()`.

- [ ] **Step 4: Startup kick** in `startup()`: `go a.updateOnStartup(ctx)` — `updater.StartupSweep(exe)`; if cache stale, `Check(force=false)` with prefs; on available & auto-update on → `startDownload()`; then a 12h ticker repeating the same (respecting autoUpdate each time); every status change emits `wailsRuntime.EventsEmit(ctx, "update:status", <UpdateStatus JSON>)`. All failures log and continue. Resolve `exe` via `os.Executable()` once.

- [ ] **Step 5: Events payload**: send the same `UpdateStatus` JSON the bindings return; `CheckedAt` as RFC3339 string; progress ints; frontend consumes `state` as its primary switch.

- [ ] **Step 6: Build + unit tests** — `go build ./...`, `go test ./...` — expect PASS. (No Wails runtime in tests; bindings are thin pass-throughs to tested units.)

- [ ] **Step 7: Commit** `feat(updater): wails bindings and startup check`

---

### Task 8: frontend — Software Update section, badge, About version

**Files:**
- Modify: `frontend/src/components/settings-page.js`
- Modify: `frontend/src/components/sidebar.js` (badge dot inside `#btn-settings`)
- Modify: `frontend/src/main.js` (`state.updateBadge`, `EventsOn("update:status")`, re-apply badge after `render()`, re-render settings page if visible on status events)
- Modify: `frontend/src/app.css`

**Interfaces:**
- Consumes the Task 7 bindings via `window.go.main.App.*` (the `window.runtime.EventsOn/EventsOff` pair for `update:status`).

- [ ] **Step 1: Settings markup** — add a `<div class="settings-section-title">Software Update</div>` block **after** General, **before** About, containing: a toggle row `Auto-update` (`#setting-auto-update`, desc: "Checks for new releases and downloads them automatically. Installs only on restart."); a channel row (`#setting-update-channel` `<select class="settings-select">` with options `stable` — "Stable (recommended)" and `nightly` — "Nightly (built from main, bleeding edge)"); and a status row `.update-status-row` with: current version (`#update-current-version`), last-checked text (`#update-last-checked`), status line (`#update-status-text`), a `Check for updates` button (`#btn-update-check`), and a primary action button (`#btn-update-action`) that is hidden/shown per state ("Update" → `DownloadUpdate()`; "Restart to update" → `RestartToApplyUpdate()`; disabled "Checking…" while `checking`).
- About row: replace literal `v0.4.0` with `<span id="about-version" …>` filled from `GetVersion()` (`v<version>`, title carries `commit`).

- [ ] **Step 2: Settings logic** — mirror the `GetCloseToTray` pattern at the bottom of `renderSettingsPage`: load `GetAutoUpdate()` → checkbox; load `GetUpdateChannel()` → select; render the status from `GetUpdateStatus()`. Handlers: checkbox change → `SetAutoUpdate(checked)` (revert on error); select change → `SetUpdateChannel(value)` (revert + console error); `#btn-update-check` → `CheckForUpdates()` then `renderUpdateStatus()`; `#btn-update-action` dispatches based on `data-action` set by `renderUpdateStatus()` (`download` → `DownloadUpdate()` then local state `downloading`; `restart` → `RestartToApplyUpdate()`; on error show inline text `#update-status-text` with `textContent`). `renderUpdateStatus(s)` is a local function mapping `state` → row text/button: `up-to-date` "You're on the latest version."; `available` (OFF) "v<latest> is available."; `available`+auto "Downloading…"; `downloading` "Downloading… <done>%" (guard divide-by-zero); `downloaded` "v<latest> downloaded. Restart to apply."; `error` "<message>"; `unsupported` "Update checks aren't supported on this build."; unchecked badge dot handled by main.js.

- [ ] **Step 3: Sidebar badge** — inside the `#btn-settings` button add `<span class="update-badge" aria-hidden="true"></span>`; CSS: absolute positioned dot reusing the `.unread-badge` recipe (`var(--badge-bg)`), hidden by default (`display: none`), `.update-badge.visible { display: block; }`; `#btn-settings { position: relative; }`.

- [ ] **Step 4: main.js wiring** — `state.updateBadge = false`. After every `render()`: `document.querySelector('#btn-settings .update-badge')?.classList.toggle('visible', state.updateBadge)`. On `window.runtime.EventsOn('update:status', (s) => { state.updateBadge = (s.state === 'available' || s.state === 'downloaded') && !s.dismissed; toggle badge; if (state.currentView === 'settings') renderSettingsPage(...) again with same handlers; })`. Also initial status fetch at boot: `window.go.main.App.GetUpdateStatus().then(s => { …apply badge… })`. `EventsOff` not needed (listener lives for app lifetime).

- [ ] **Step 5: CSS** — append to `app.css` under the Settings section: `.settings-select` (same shell as `.settings-checkbox` sizing, `#111b21` bg, `var(--border-color)` border, `var(--text-primary)`, padding 8px 10px, border-radius 6px), `.update-status-row` (reuse `.settings-toggle-row` look), `.update-status-actions` (flex gap 8px), `.update-status-text` (font-size 12px; color `var(--text-secondary)`; `.error` → `var(--danger-red)`), `.update-badge` (see Step 3), and `.btn-primary` already exists? — if not, reuse `#btn-settings`-like accent: define minimal `.update-action-btn` (background `var(--accent-emerald)`, white text, 8px radius, padding 8px 14px, cursor pointer; `:disabled` opacity 0.6).

- [ ] **Step 6: Build** — `npm run build` in `frontend/` (node_modules present) — expect success; `wails dev` visual pass is deferred to the manual checklist.

- [ ] **Step 7: Commit** `feat(updater): software update settings, about version and sidebar badge`

---

### Task 9: docs, manual verification, README

**Files:**
- Modify: `README.md` — new "Updating" section: what auto-update is, default off, channels, first update from ≤ v0.1.0 is manual, dev builds self-disable, which platforms are supported (Windows), and where the setting lives.
- Create/Update: nothing else; spec already committed (see commit below).

**Manual verification checklist (run in order, record results in the PR/commit body):**

- [ ] Fresh stamped build: `wassap-desktop.exe --version` prints stamped `version (commit)`; Settings → Software Update shows row; auto-update default OFF; channel default "Stable".
- [ ] "Check for updates" on stable (run a build whose commit is behind the v0.1.0 tag's history — e.g. an older local build): row shows update available; sidebar dot appears; [Update] downloads (digest verified) → row switches to "Restart to update"; [Restart to update] → swap happens, app relaunches itself, new instance runs the new version, `--version` confirms, `*.old-*` gone on next start.
- [ ] Same flow end-to-end with `SetAutoUpdate(true)` before the check: no clicks needed until the restart button.
- [ ] Channel switch to Nightly on a commit behind main → offered; switch back to Stable while running a post-v0.1.0 nightly → **not offered** (ahead-only gate).
- [ ] Make `build\bin` read-only (Properties) → apply: actionable error, app keeps running, no `.new-*` left in the folder.
- [ ] Delete the exe next to a `wassap-desktop.exe.old-<pid>` → next launch restores it (sweep restore path).
- [ ] Corrupt `~/.cache/wassap-desktop/update-check.json` to `{"nope` → app starts, check degrades silently to error/unknown, no crash, no repeated HTTP hammering within 12h.
- [ ] README render pass; confirm the About row shows the real version not `v0.4.0`.

## Self-review notes (author checklist, run before handing out)

- Spec coverage: stamping (T1), updater package (T2–T5), config (T6), bindings+startup (T7), frontend (T8), README+manual (T9), all spec sections mapped. Bootstrap limitation and Linux/macOS unsupported are surfaced in spec/README, not hidden.
- Review Focus coverage: (1) T5 Step 6 + T9 checklist item; (2) T5 Steps 4–5 + test; (3) T3 Step 3 tests; (4) T4 verify-on-download + T5 Step 1 `CopyStaged` re-verify + T7 Step 3 before-quit gate; (5) T2 corrupt-cache test + T3 fresh-cache-no-HTTP test + T7 quit-only-after-swap.
