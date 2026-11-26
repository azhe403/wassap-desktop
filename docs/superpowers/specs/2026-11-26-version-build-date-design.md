# Version & Build Date Display — Design

**Date:** 2026-11-26
**Status:** Approved by user ("yg rekomend aja" = go with the recommendation), 2026-11-26

**Scope:** CI ldflags stamping, `main.go` version vars, `internal/app` binding contract, `internal/updater` status/cache propagation, frontend Settings/About. No change to the update decision logic (ahead-only gate, download, swap, relaunch).

## Intent & Why

The updater already stamps a version (`vX.Y.Z` for tags, `0.0.0-nightly.<sha7>` for main) and a commit, but has no build date. On the nightly channel two builds can look nearly identical in the Settings row (`0.0.0-nightly.a1b2c3d` vs `0.0.0-nightly.f9e8d7c`), and the row currently shows only the *available* version, so a user cannot tell "what I run" from "what is offered" at a glance. Adding a build date to the installed binary and showing an explicit installed-vs-available comparison when an update exists removes that ambiguity.

## Goals / Non-goals

- Goals: stamp a UTC build date at CI build time; surface it in `--version` and About; surface the available release's publish time (already in the GitHub API) through the updater status; show both installed and available version + date side by side when an update is available/downloaded.
- Non-goals: no new release asset or manifest; no CI pipeline restructuring; no change to channel rules or update safety; no local-timezone conversion (UTC everywhere, matching existing RFC3339 payloads).
- Anti-goals: no fabricated numbers or relative "x days ago" text; no new decoration in the UI; no new update-check requests.

## Constraints

- Repo moved since the original updater work: bindings now live in `internal/app` (app.New receives `app.VersionInfo{Version, Commit}`); `main.go` still owns the raw ldflag vars and `--version` handling.
- `internal/updater/github.go` already parses `published_at` into `Release.PublishedAt`; the checker discards it today.
- `UpdateCheckCache` is persisted JSON; adding a field is backward compatible (old caches lack it, degrade to "date unavailable").
- Frontend has no test runner; `npm run build` is the only automated check.

## Approaches (considered)

1. **`published_at` from GitHub for the available build + CI-stamped date for the installed build** — chosen. The API call already happens, so the remote date is free; the local date needs one extra ldflag. Cost: remote date is publish time, a few minutes after compilation.
2. `published_at` only, no local stamp — the installed side would have no date at all, defeating the comparison.
3. Stamped date in both binaries via a release manifest/sidecar — most accurate, but rewrites CI/release assets and the updater's download/selection path. Rejected: out of proportion for a display change.

## Design

- **CI** (`.github/workflows/build.yml`, Build step): add `-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)` to the existing ldflags string. Local/dev builds keep `buildDate == ""`.
- **`main.go`**: `var buildDate = ""`; pass it through `app.VersionInfo{Version, Commit, BuildDate}`; `--version` prints `wassap-desktop <version> (<commit>) built <buildDate>` when present, unchanged output when absent.
- **`internal/app`**: `VersionInfo` gains `BuildDate string json:"buildDate"`; `UpdateStatus` gains `LatestBuildDate string json:"latestBuildDate,omitempty"` (RFC3339 UTC, empty when unknown). `statusToUpdateStatus` copies the checker's new field.
- **`internal/updater`**: `Status` gains `LatestBuildDate time.Time`; checker sets it from `Release.PublishedAt` in both channel branches; `UpdateCheckCache` gains `publishedAt time.Time json:"publishedAt,omitempty"` and the checker restores it into `Snapshot`/`statusFromCache` so the date survives restart and error fallbacks. No new endpoint or HTTP request.
- **Frontend**:
  - About row: under the existing version text, show `build <date UTC>` when `GetVersion().buildDate` is present; no text when absent (dev build).
  - Settings → Software Update status row: add a compact two-row comparison, shown only when state is `available` or `downloaded` and a latest version exists:
    ```text
    Installed   v0.1.0                   built 15 Nov 2026 06:03 UTC
    Available   0.0.0-nightly.a1b2c3d    published 26 Nov 2026 08:12 UTC
    ```
    Dates are formatted client-side from RFC3339 with a fixed UTC suffix; missing dates render `date unavailable`. The existing status-line sentences and buttons remain unchanged, except the `unsupported` sentence: `cf323bf` intentionally clarifies it to "This is a development build. Install a release from GitHub to enable updates." (`StateUnsupported` is emitted only when the build lacks a version/commit stamp, so the sentence is accurate.) Use the existing `app.css` tokens and the row's typography; no new colors, no cards, no icons.
- **Docs**: README's "Updating" section gets one line about where version/build date are shown; the 2026-11-15 auto-updater spec gets a short note recording the added stamp and status field (living-doc obligation).

## Verification

- Go tests: cache roundtrip includes `publishedAt`; checker tests assert `Status.LatestBuildDate` comes from the release JSON for both channels and survives the cache/snapshot path.
- `go build ./...`, `npm --prefix frontend run build`, and the existing filtered `go test .` for package-main changes. Machine-local WebView2 suites are intermittently flaky for environmental reasons (pre-existing).
- Manual (user-owned): with a nightly update available, confirm Settings shows both rows with distinct dates and About shows the installed build date; `--version` on a stamped binary prints the build date.

## Left to the builder

- Exact HTML class names for the comparison rows and whether the About build date is a second line or appended to the same line (both use existing tokens).
- The `--version` output wording when `commit` is present but `buildDate` is absent, and vice versa.
