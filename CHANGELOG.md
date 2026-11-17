# Changelog

All notable changes to Wassap Desktop are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - 2026-11-17

### Added

- **Account avatar sync**: each account's sidebar and Profile Management avatar now follows its WhatsApp profile photo. A script injected into the isolated WebView2 session reads the self photo, caches it locally under `~/.local/share/wassap-desktop/avatars/`, and pushes updates to the UI; initials are used when no photo is available.
- **Auto updater**: ahead-only stable/nightly version checker with a throttled cache, verified background downloads with `.part` staging, rename-aside swap, startup sweep and relaunch mode. Added auto-update preferences and channel selection, Wails bindings and startup check, update settings with an About/version section and sidebar badge, and CI stamping of version and commit into release binaries.
- **Privacy blur**: vendored upstream blur engine with an adapter and script builders, persisted blur settings with atomic merge, per-session prelude injection and `eval-all`, cross-session broadcast on save, an in-page `Alt+/` write path, frontend settings section and sidebar shield toggle, and upstream attribution.
- **Background session lifecycle**: WebView2 controllers are hidden for `startHidden`/background sessions and stale `wails-shell/run-*` directories are swept at startup.
- **Settings page**: reorganized into tabbed panels with third-party notices.

### Changed

- **Shell structure**: moved the Wails shell implementation from the repository root into `internal/app`, leaving a thin `main.go` at the root. Generated JS bindings moved from `window.go.main.App` to `window.go.app.App`.
- Added `.openchamber/screenshots` to `.gitignore`.

### Fixed

- Avatar sync rejects WhatsApp's default silhouette (vendored `default-contact-refreshed` / `default-group-refreshed` selectors) so photoless accounts keep the app's initials.
- Auto updater discards a staged build that was superseded while downloading and applies the findings of the final review.
- Privacy blur broadcasts converge on the latest stored settings across sessions.
- Shell-profile cleanup runs only in the lock winner; background controllers are hidden consistently.
- Regenerated Wails bindings after the privacy blur settings change.
- CI zips the macOS `.app` bundle before publishing release assets.

[Unreleased]: https://github.com/azhe403/wassap-desktop/compare/v0.1.0...HEAD
