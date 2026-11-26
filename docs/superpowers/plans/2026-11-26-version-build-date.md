# Version & Build Date Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stamp a UTC build date into Wassap Desktop binaries and show an installed-vs-available version/date comparison in Settings when an update exists.

**Architecture:** CI adds one ldflag; `main.go` passes the raw stamp into `app.VersionInfo`. The updater's existing GitHub release payload already carries `published_at`; the checker keeps it on `Status` and in the JSON cache, the app binding formats it to RFC3339, and the vanilla-JS Settings view renders two read-only text rows. No changes to the update decision, download, swap, or relaunch paths.

**Tech Stack:** Go 1.26 / Wails v2.17.0, `golang.org/x/sys/windows` (existing), vanilla JS + CSS, GitHub Actions ldflags.

**Spec:** `docs/superpowers/specs/2026-11-26-version-build-date-design.md`

## Global Constraints

- Auto-update remains off by default; channel remains stable by default; ahead-only offer gate unchanged; Windows-only apply remains Phase 1.
- Dates are UTC everywhere. Wire format RFC3339 (`2026-11-26T08:12:00Z`). Display format fixed client-side: `26 Nov 2026 08:12 UTC`. Missing date renders `date unavailable`, never a fabricated value.
- Build stamp flag is exactly `-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)` appended to the existing ldflags string in the Build step (which already sets `shell: bash`).
- Dev builds have empty `buildDate` and empty `commit`; do not print a bogus line in About or `--version`.
- No new release asset, manifest, HTTP request, or CI job. No new colors, icons, or cards in the UI; existing `app.css` tokens only.
- One logical change per commit; conventional commit messages.

## Review Focus

1. **Old cached `update-check.json` without the new publishedAt field** → app must show "date unavailable", not crash or show a zero date. Pinned by Task 2 cache roundtrip + Snapshot tests (zero time accepted, no panic).
2. **Failed check after a successful one** → the available release's date must survive from cache (error fallback must not blank it). Pinned by Task 2 `statusOnError`/fresh-cache test asserting `LatestBuildDate` preserved.
3. **Nightly version identity** (`nightly 2026-11-26T08:12Z`) must be shown verbatim as the version, with its date parsed separately; no `v`-prefix on the nightly identity. Pinned by Task 1 checker test + Task 4 UI code inspection (`npm run build`, no test runner in frontend).
4. **Dev build (no stamps)** → About shows no date line and `--version` keeps its existing shape. Pinned by Task 1 `--version` smoke check and Task 4 conditional rendering.
5. **Comparison rows must not add dead controls or change the update actions** → rows are read-only text; Update / Restart / Dismiss buttons keep their existing dispatch. Pinned by Task 4 code inspection and `npm run build`; no new interactive elements.

---

### Task 1: CI build-date stamp + version plumbing

**Files:**
- Modify: `.github/workflows/build.yml` (Build step, current ldflags line)
- Modify: `main.go` (buildDate var, `--version` block, `app.New` call near line 126)
- Modify: `internal/app/update_binding.go` (`VersionInfo` struct near line 36)

**Interfaces:**
- Consumes: existing `version`, `commit` vars in `main.go`.
- Produces: `VersionInfo{Version, Commit, BuildDate string json:"buildDate"}`; `main.buildDate` available to `--version`.

- [ ] **Step 1: Add the stamp to CI ldflags** — in `.github/workflows/build.yml`, append `-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)` inside the existing `-ldflags "..."` argument. Leave the `VER` selection logic and the matrix untouched.

- [ ] **Step 2: Add `var buildDate = ""`** in `main.go` beside `version`/`commit`, with a one-line comment noting CI sets it via ldflags and dev builds leave it empty.

- [ ] **Step 3: Pass it to the binding** — change the `app.New(...)` call at `main.go:126` to `app.New(dirs, legacyMode, app.VersionInfo{Version: version, Commit: commit, BuildDate: buildDate})`, and add `BuildDate string \`json:"buildDate"\`` to `VersionInfo` in `internal/app/update_binding.go`.

- [ ] **Step 4: Extend `--version`** in `main.go`: when `buildDate != ""`, print `wassap-desktop <version> (<commit-or-dev>) built <buildDate>`; when empty, keep the current exact output (`wassap-desktop dev (dev)` for unstamped builds).

- [ ] **Step 5: Verify** — `go build -ldflags "-X main.version=0.2.0 -X main.commit=abc1234 -X main.buildDate=2026-11-26T08:12:00Z" -o build/bin/wassap-desktop.exe .` then run `--version`; expected `wassap-desktop 0.2.0 (abc1234) built 2026-11-26T08:12:00Z`. Also run without flags into a second output path and confirm `wassap-desktop dev (dev)`. Delete both temp binaries after.

- [ ] **Step 6: Commit** `feat(ci): stamp build date into release binaries`

---

### Task 2: updater status + cache carry the release publish time

**Files:**
- Modify: `internal/updater/checker.go` (Status struct near line 23, success path ~line 115, `statusFromCache` ~line 169, `statusOnError` ~line 190, `checkStable` ~line 242, `checkNightly` ~line 269, `Snapshot`)
- Modify: `internal/updater/cache.go` (`UpdateCheckCache` struct near line 22)
- Test: `internal/updater/checker_test.go`, `internal/updater/cache_test.go`

**Interfaces:**
- Consumes: `github.go Release.PublishedAt time.Time` (already parsed from `published_at`).
- Produces: `Status.LatestBuildDate time.Time`; `UpdateCheckCache.PublishedAt time.Time json:"publishedAt,omitempty"`.

- [ ] **Step 1: Write failing tests** in `checker_test.go` and `cache_test.go`:
  - `TestStableAheadOffersUpdate` (or a new test): after a check against the canned `stableRel` JSON (`published_at: 2026-11-15T06:03:19Z`), assert `st.LatestBuildDate.Equal(time.Date(2026,11,15,6,3,19,0,time.UTC))`.
  - `TestNightlyChannelAheadOffers`: assert `st.LatestBuildDate` equals the canned nightly `published_at` (`2026-11-15T07:00:00Z`).
  - `TestSnapshotAfterSuccessReflectsCache`: assert `c.Snapshot().LatestBuildDate` equals the same timestamp (survives cache roundtrip).
  - `TestHTTPFailureFallsBackAndBackoffs`: seed a cache with a `PublishedAt`; after a failing check assert returned `Status.LatestBuildDate` is unchanged.
  - `TestCacheRoundTrip`: include `PublishedAt` in the input struct and assert equality after Load.
  - `TestCacheMissingFileReturnsZeroValue` must still pass with the new field absent.

- [ ] **Step 2: Run** `go test ./internal/updater/ -run 'Checker|Snapshot|Cache' -v` — expect FAIL (undefined fields).

- [ ] **Step 3: Implement**: add `LatestBuildDate time.Time \`json:"latestBuildDate"\`` to `Status`; add `PublishedAt time.Time \`json:"publishedAt,omitempty"\`` to `UpdateCheckCache`. In `checkStable`/`checkNightly`, set `LatestBuildDate: rel.PublishedAt.UTC()`. In the checker success path, persist `res.PublishedAt = st.LatestBuildDate` (preserving the previous value when a check yields no release). In `statusFromCache`, set `st.LatestBuildDate = cached.PublishedAt`. In `statusOnError`, keep whatever `cached.PublishedAt` provides. `Snapshot` already delegates to `statusFromCache`, so no separate change.

- [ ] **Step 4: Run** `go test ./internal/updater/ -count=1` — expect PASS.

- [ ] **Step 5: Commit** `feat(updater): carry release publish time through status and cache`

---

### Task 3: app binding exposes latest build date

**Files:**
- Modify: `internal/app/update_binding.go` (`UpdateStatus` struct near line 43, `statusToUpdateStatus` near line 56)

**Interfaces:**
- Consumes: `updater.Status.LatestBuildDate time.Time` from Task 2.
- Produces: `UpdateStatus.LatestBuildDate string json:"latestBuildDate,omitempty"` in RFC3339 UTC, consumed by Task 4.

- [ ] **Step 1: Add the field** to `UpdateStatus`: `LatestBuildDate string \`json:"latestBuildDate,omitempty"\``.

- [ ] **Step 2: Populate it** in `statusToUpdateStatus`: when `st.LatestBuildDate` is non-zero, set `out.LatestBuildDate = st.LatestBuildDate.UTC().Format(time.RFC3339)` (same pattern as the existing `CheckedAt` formatting); leave empty otherwise.

- [ ] **Step 3: Verify** — `go build ./...` and `go vet ./...` (only the two known pre-existing webview2 `unsafe.Pointer` vet notes may appear). No new unit test needed: the pure formatting path is covered by Task 2's time assertions and this function has no branches beyond the zero check.

- [ ] **Step 4: Commit** `feat(updater): expose latest build date in update status binding`

---

### Task 4: frontend comparison rows + About build date

**Files:**
- Modify: `frontend/src/components/settings-page.js` (About row near line 270, `GetVersion` handler near line 679, Software Update status row near line 246, `renderUpdateStatus` near line 807)
- Modify: `frontend/src/app.css` (add styles beside the `.update-status-*` block)

**Interfaces:**
- Consumes: `GetVersion() → {version, commit, buildDate}`, `GetUpdateStatus() → UpdateStatus` with `latestBuildDate`.

- [ ] **Step 1: About** — add `<span id="about-build-date">` under the existing `#about-version` text (same `.settings-toggle-text` column), empty by default. In the `api.GetVersion().then(...)` handler, set its text to `build <formatted>` when `v.buildDate` is present, empty otherwise. Keep the `#about-version` text logic as-is (it already handles `dev`).

- [ ] **Step 2: Comparison block** — inside the `.update-status-row`, add a read-only two-row element hidden by default:
  ```html
  <div class="update-compare" id="update-compare" hidden>
      <div class="update-compare-row"><span class="update-compare-label">Installed</span><span id="update-compare-installed"></span></div>
      <div class="update-compare-row"><span class="update-compare-label">Available</span><span id="update-compare-available"></span></div>
  </div>
  ```
  Add a module-level helper `formatBuildUTC(iso)` returning `DD Mon YYYY HH:MM UTC` from an RFC3339 string via `new Date(iso).toUTCString()`-style formatting with an explicit ` UTC` suffix, and `date unavailable` for empty/invalid input. In `renderUpdateStatus(container, s)`: when `s.state === 'available' || s.state === 'downloaded'` and `s.latestVersion` is non-empty, fill installed as `v${s.currentVersion} · ${formatBuildUTC(<installed stamp>)}` and available as `${s.latestVersion} · ${formatBuildUTC(s.latestBuildDate)}`, then show the block; otherwise hide it. The installed build date comes from the `GetVersion()` result, cached in a module-scoped variable `let installedBuildDate = ''` and set in the same `GetVersion` handler from Step 1 (call `renderUpdateStatus(container, container.__updateStatus)` after it resolves so the comparison populates even when status arrived first).

- [ ] **Step 3: CSS** — append to `app.css` beside `.update-status-text`: `.update-compare` (flex column, gap 4px, margin-top 10px, font-size 12px), `.update-compare-row` (flex, gap 10px, justify-content space-between, max-width 100%), `.update-compare-label` (color `var(--text-secondary)`, min-width 64px), and reuse `var(--text-primary)` for values. No new colors, borders, icons, or shadows. The `[hidden]` attribute handles visibility; no `display` rule may override it.

- [ ] **Step 4: Build** — `npm --prefix frontend run build` — expect success with no console errors; then `wails generate module` is NOT needed (no new binding methods).

- [ ] **Step 5: Commit** `feat(updater): show installed vs available version and build dates`

---

### Task 5: docs + verification

**Files:**
- Modify: `README.md` (Updating section)
- Modify: `docs/superpowers/specs/2026-11-15-auto-updater-design.md` (§5 version stamping: mention `buildDate` ldflag; §7.3/§7.4: mention `LatestBuildDate` field and the comparison row)
- Modify: the plan doc itself if any behavior drifted (living-doc obligation)

**Manual verification (user-owned, do not run the live app unprompted):**
- With a nightly pre-release available, Settings row shows two distinct rows: installed version + build date, available version + publish date.
- About shows the installed build date; `--version` on a stamped binary includes `built <timestamp>`.
- Old cache file without `publishedAt` → "date unavailable", no crash; error state preserves the last known date.

- [ ] **Step 1: Update README** — one line in "Updating": version and build date are shown in Settings → Software Update (installed vs available) and on the About row.

- [ ] **Step 2: Update the earlier spec** — record the added `buildDate` stamp, `Status.LatestBuildDate`, `UpdateCheckCache.PublishedAt`, and the comparison row, with a "2026-11-26 update" note. Do not rewrite the rest of the spec.

- [ ] **Step 3: Final verification** — `go build ./...`, `go test ./internal/updater/ ./internal/config/ -count=1`, and `npm --prefix frontend run build` all green; `git status --short` shows only intended files. The user-owned manual checklist above is reported in the final message, not executed here.

- [ ] **Step 4: Commit** `docs: version and build date display`

---

## Self-review notes (author checklist, run before handing out)

- Spec coverage: stamping (T1), date propagation/cache (T2), binding (T3), UI (T4), docs + manual (T5). No spec section unmapped.
- Step scan: each step is a single action with a checkable result; no speculative bodies. The frontend formatter and CSS details are left to the implementer where the spec fixed only the output format.
- Type consistency: `Status.LatestBuildDate time.Time` → `UpdateCheckCache.PublishedAt time.Time` → `UpdateStatus.LatestBuildDate string` → JS `s.latestBuildDate`. `VersionInfo.BuildDate string` → `v.buildDate` → `installedBuildDate`.
- Review Focus: five entries mapped to Task 2 (cache/snapshot/error), Task 1 (nightly identity + dev build), Task 4 (read-only rows, build). The frontend has no unit runner; browser build + code inspection is the honest bar and is called out in the task steps.
