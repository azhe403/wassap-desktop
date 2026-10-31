# Wassap Desktop

A multi-account WhatsApp Web desktop shell built with Wails v2, Go and the native
Windows WebView2.

**Current scope (Pecut 1):** a single WhatsApp Web session, displayed directly in
the native WebView, with a login that survives restarts. Multi-account support is
not implemented yet.

## How it works

Wassap Desktop does not bundle, proxy or reimplement WhatsApp Web. The native
WebView is pointed at `https://web.whatsapp.com` and that is the whole UI.

The desktop shell exists only to give that page a persistent browser profile, so
the linked-device session is restored on the next launch.

There is no iframe, no reverse proxy and no unofficial WhatsApp API. The WebView
loads WhatsApp Web as a normal top-level navigation.

### Start-up sequence

Wails always navigates the WebView to `http://wails.localhost/` on start-up. That
is not configurable, so the app works with it instead of fighting it:

1. `main.go` resolves and creates the application directories, then passes the
   data directory to Wails as `WebviewUserDataPath`.
2. Wails navigates to `http://wails.localhost/`. Because no `Assets` are
   registered, every request reaches the `Handler` in `app.go`, which answers
   with a `302` redirect to `https://web.whatsapp.com`. The WebView follows it
   once, and WhatsApp Web stays loaded.

There is deliberately no script-based navigation (no `OnDomReady` hook). An
earlier version called `window.location.replace` on every DOM-ready event, which
ran again after each WhatsApp page load and caused a reload loop. The embedded
`frontend/dist` is still built to keep the standard Wails pipeline working, but
it is never served in production.

## Application directories

The layout is identical on Windows, Linux and macOS, and is always resolved
against the current user's home directory (`os.UserHomeDir`, which uses
`%USERPROFILE%` on Windows and `$HOME` elsewhere):

| Purpose | Path |
| --- | --- |
| Data (WebView2 profile / WhatsApp session) | `~/.local/share/wassap-desktop/` |
| Config | `~/.config/wassap-desktop/` |
| Cache | `~/.cache/wassap-desktop/` |

`os.UserConfigDir` and `os.UserCacheDir` are deliberately **not** used, because
they resolve to platform-specific locations (`%APPDATA%`, `~/Library/...`,
`$XDG_*`) and would break the single documented convention.

Resolution lives in `internal/appdirs` and is covered by unit tests. The data and
config directories are created before `wails.Run`; the cache directory is left
alone because it holds only regenerable data.

## Persistent WebView2 storage

This was the main thing to verify, because Wails' defaults are misleading.

- Wails v2.17.0 exposes the WebView2 user data folder as
  `options.Windows.WebviewUserDataPath`. `setupChromium` assigns it straight to
  the `go-webview2` Chromium `DataPath`.
- If it is left empty, `go-webview2` falls back to
  `filepath.Join(os.Getenv("AppData"), <exe name>)` — i.e. a directory named
  after the executable, `.exe` included, under `%APPDATA%`. So the default is
  **not** a `.local/share` path, and it also changes between `wails dev`
  (`wassap-desktop-dev.exe`) and a production build (`wassap-desktop.exe`),
  which silently splits the session in two.
- The value is passed to `CreateCoreWebView2EnvironmentWithOptions`, and
  WebView2 requires the folder to already exist. If it does not, WebView2 fails
  the environment creation and Wails shows a message box and exits. Creating the
  directory is therefore done in `main.go` before `wails.Run`, not hoped for.
- Persistence is not something the app turns on: a real user data folder is what
  makes cookies, `localStorage` and `IndexedDB` survive. Because it is now a
  fixed, explicit path, the session is shared by `wails dev` and production
  builds alike.

**Actual browser data directory used:**

```
~/.local/share/wassap-desktop/EBWebView/
```

That `EBWebView` folder is created by WebView2 itself and contains the standard
Chromium profile (`Default/Cookies`, `Default/Local Storage`,
`Default/IndexedDB`, `Local State`, `lockfile`, caches, …). WhatsApp Web's
session lives in that profile.

### Platform notes

- **Windows:** fully functional. The data folder can be set exactly as above.
  Note that Wails accepts the path without creating it, and that the folder must
  exist *and* be writable — a read-only folder fails at start-up with a WebView2
  error rather than a clear message.
- **Linux/macOS:** the directory convention and `internal/appdirs` are
  platform-independent, but `WebviewUserDataPath` is a Windows-only option. Wails
  on Linux uses WebKitGTK (via `webkit2gtk`), and on macOS uses WKWebView, each
  with its own storage mechanism; on those platforms this option is ignored and
  the persistent store is managed by the platform webview. Setting it is
  harmless there, but it does not control storage on those platforms. Prompt 1
  targets Windows/WebView2 first.
- **`wails dev`:** functions properly. Note that because WebView2 places an
  exclusive lock on the user data folder (`EBWebView\lockfile`), running `wails dev`
  while a production build is already open will cause the dev binary to exit
  immediately. Close any running instance before launching dev mode. Since WhatsApp
  Web is remote, frontend hot reload does not apply to it, but Go file changes
  will trigger rebuilds normally.

## Development

```sh
wails dev      # live development
wails build    # production build -> build/bin/wassap-desktop.exe
go test ./...  # unit tests
```

On this machine `wails` is installed at `%USERPROFILE%\go\bin\wails.exe`, which
is not always on `PATH`; add that directory or invoke it by full path.

## Known limitations

- **One account only.** Multi-account management is out of scope for this step.
- **The login is not migrated.** Before this change the session lived in
  `%APPDATA%\wassap-desktop-dev.exe`. It is not copied to the new location, so
  the first launch after upgrading asks you to scan the QR code again. The old
  directory is left untouched and can be deleted manually.
- **First start-up is a cold browser profile.** WebView2 has to initialise its
  storage, so the first load is noticeably slower than later ones.
- **`frontend/dist` must exist to build.** `go:embed all:frontend/dist` makes the
  Go build depend on the frontend build, which is the normal Wails layout. The
  assets are not served, but the directory cannot be removed without also
  changing `wails.json`.
- **Upgrading WebView2 or changing the data path** can invalidate the stored
  profile and require a fresh login.
- **No hardening beyond the defaults.** Popups, downloads and permission prompts
  are handled by WebView2 defaults. `wails dev` also enables DevTools.
- **No session recovery UI.** If WhatsApp Web's stored session is rejected by
  WhatsApp, the app simply shows the QR screen again.
