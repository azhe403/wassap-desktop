# Wassap Desktop

WhatsApp Web di desktop, multi-account. Tiap akun dapat profil browser
yang terpisah dan persisten.

**Stack:** Wails v2.17 + Go, frontend vanilla HTML/JS/CSS, WebView2 (Windows).

## Build & Run

```sh
wails dev          # development
wails build        # produksi -> build/bin/wassap-desktop.exe
```

```sh
.\build\bin\wassap-desktop.exe             # mode multi-akun (test interface)
.\build\bin\wassap-desktop.exe --pecut1    # legacy single-session
```

## Tests

```sh
go test ./...
```

## Kondisi Saat Ini

- Multi-akun dengan profil browser terpisah & persisten ✅
- Manajemen akun (backend) ✅
- Sidebar UI akun belum — pemakaian masih lewat test interface

## Dokumentasi

- [Architecture Reference](docs/architecture.md) — layout data, struktur paket, lifecycle akun
- [Specs](docs/specs/) — single session, isolasi profil, manajemen akun
- [Plans](docs/plans/) — rencana implementasi
