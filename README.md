# Wassap Desktop

A multi-account WhatsApp Web desktop application built with **Wails v2.17.0**, **Go**, and **Vanilla HTML/CSS/JS** on Windows, using native **WebView2** engine bindings for fully isolated, persistent browser profiles.

## Features

- **Multi-account switching**: Manage multiple WhatsApp Web accounts inside a single desktop window.
- **Native profile isolation**: Each account uses its own isolated WebView2 user data folder (`~/.local/share/wassap-desktop/profiles/<id>/EBWebView`).
- **Single main window architecture**: Account switching happens inside the existing Wails window using native `WS_CHILD` embedded WebView2 — no separate top-level desktop windows, iframes, or reverse proxies.
- **Persistent active account state**: Last active account is automatically restored when the app restarts.

## Architecture

- **Pecut 1**: Single WhatsApp Web session with persistent storage.
- **Pecut 2**: Proof of concept for multi-account profile isolation using separate WebView2 User Data Folders.
- **Pecut 3**: Account management backend (domain model, validation, JSON persistence, Wails bindings).
- **Pecut 4**: Account Switcher UI — vertical sidebar, desktop-first modal dialogs, embedded native WebView2 switching.

## Directory Layout

```text
~/.local/share/wassap-desktop/
├── accounts.json                  # Account metadata (atomic JSON writes)
├── profiles/
│   └── <ulid>/
│       ├── EBWebView/             # Isolated WebView2 storage per account
│       └── profile.lock
├── profiles.json                  # Profile manifest
└── EBWebView/                     # Legacy Pecut 1 shared profile (untouched)

~/.local/config/wassap-desktop/
└── config.json                    # Application settings (activeAccountId)
```

## Running the App

### Legacy Pecut 1 Mode

```bash
go run . --pecut1
```

### Main Desktop UI (Pecut 4)

```bash
go run .
```

Run `wails dev` for live hot-reloading during development.

### Production Build

```bash
wails build
```

## Updating

Wassap Desktop can check the [GitHub Releases](https://github.com/azhe403/wassap-desktop/releases) page for newer builds and apply them in place — no reinstall needed.

- **Auto-update is OFF by default.** Nothing is downloaded unless you turn it on in **Settings → Software Update** when you want it.
- **Channels:** `Stable` (default) follows tagged releases (`v*`); `Nightly` follows `main` and may break things. Switching channels never downgrades the app — an update is offered only when the channel's build contains commits your running build does not.
- **Applying an update:** when a newer build is found, enable **Auto-update** (or click **Update**), then click **Restart to update** when it has downloaded. The app swaps the binary and relaunches itself. Until you click, nothing changes.
- **Windows only.** Linux/macOS report "not supported" for now.
- **Versions ≤ v0.1.0 have no updater** and don't know about this feature; their first update is a manual download from the Releases page. Every release after that can update itself.
- Development builds (no version stamp) disable the updater entirely.
- **Version and build date:** Settings → Software Update shows the installed and available version with their build/publish dates side by side when an update exists; the About row shows the installed build date. Dates are UTC.

## Testing

```bash
go test ./...
cd frontend && npm run build
```

## Privacy Blur

Wassap Desktop can blur sensitive WhatsApp content until you hover it — names, profile photos, message previews, bubbles, stickers, media and the typing field — via **Settings → Privacy Blur** or the shield button in the sidebar (`Alt+/` toggles in a chat). Everything runs locally; nothing is sent anywhere.

## Credits

The privacy blur engine is adapted from **WhatsApp Privacy Blur** by Nyihui —
[Firefox Add-on](https://addons.mozilla.org/en-US/firefox/addon/whatsapp-privacy-blur/) · [source](https://github.com/Nyihui/WhatsappPrivacyBlur-Extension) — MIT licensed.
Its selector library and blur engine are vendored **unmodified** under `internal/privacyblur/assets/upstream/` (v3.2.0 @ `82cc20e`); Wassap Desktop adds its own adapter (`assets/adapter.js`) and native controls instead of the add-on's in-page panel.

## Supported Platforms

Windows 10/11 with WebView2 Runtime installed.
