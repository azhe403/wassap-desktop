# Wassap Desktop

A multi-account WhatsApp Web desktop shell built with Wails v2, Go and Microsoft Edge WebView2 on Windows.

## Current Status (Pecut 2)

**Pecut 2 — WebView2 Multi-Profile Isolation Proof of Concept**: Demonstrates that multiple WhatsApp accounts can use completely isolated, persistent browser profiles on WebView2.

- **Account A** and **Account B** use separate, isolated User Data Folders.
- Independent cookie jars, `localStorage` leveldb, and `IndexedDB`.
- Complete persistence across application restarts.
- Full Pecut 1 backward compatibility is maintained via `--pecut1` or the UI.

---

## Architecture

Wassap Desktop uses **Separate WebView2 User Data Folders (UDF)** per account.

```text
~/.local/share/wassap-desktop/
├── profiles/
│   ├── account-a/
│   │   ├── EBWebView/       <-- Isolated WhatsApp Session A
│   │   └── profile.lock     <-- Win32 cross-process lock
│   └── account-b/
│       ├── EBWebView/       <-- Isolated WhatsApp Session B
│       └── profile.lock
├── profiles.json            <-- Profile manifest
└── EBWebView/               <-- Legacy Pecut 1 profile
```

### Why Separate User Data Folders?

- **Physical Isolation**: Each account has its own SQLite cookie database and LevelDB storage trees. No data or cookies can leak between accounts.
- **Process Isolation**: Separate `msedgewebview2.exe` processes avoid shared state or cascading crashes.
- **Safe Deletion**: Deleting one account cleans up only its own directory, leaving other accounts untouched.

---

## Running the Application

### Development Mode

```sh
wails dev
```

### Production Build

```sh
wails build
```

Binary will be built to: `build/bin/wassap-desktop.exe`.

### Running Pecut 2 PoC

```sh
.\build\bin\wassap-desktop.exe
```

The test interface provides:
- **Open Account A**: Opens a window navigating to `https://web.whatsapp.com` with Profile A.
- **Open Account B**: Opens a window navigating to `https://web.whatsapp.com` with Profile B.
- **Close Current WebView**: Closes the active session cleanly and flushes storage.
- **Evidence Inspector**: Shows on-disk cookies and storage status.

### Running Pecut 1 Legacy Mode

```sh
.\build\bin\wassap-desktop.exe --pecut1
```

Launches directly into the single WhatsApp Web session preserved at `~/.local/share/wassap-desktop/EBWebView/`.

---

## Running Tests

```sh
go test -v ./...
```

Includes unit tests for profile management, path resolution, and an end-to-end integration test (`TestSeparateUserDataFoldersIsolateStorage`) that executes real WebView2 headless sessions to verify storage persistence, isolation, and deletion safety.

---

## Documentation

- [Pecut 1 Specification](docs/specs/pecut-1-single-whatsapp-session.md)
- [Pecut 2 Specification & Verification Report](docs/specs/pecut-2-profile-isolation.md)
