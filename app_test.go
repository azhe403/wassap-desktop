package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wassap-desktop/internal/appdirs"
)

func TestAppHandler_LegacyModeRedirects(t *testing.T) {
	dirs := appdirs.Under(t.TempDir())
	app := NewApp(dirs, true) // legacyMode = true
	h := app.handler()
	if h == nil {
		t.Fatal("expected handler in legacy mode, got nil")
	}

	for _, target := range []string{"/", "/index.html", "/some/asset.js"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://wails.localhost"+target, nil))

		if rec.Code != http.StatusFound {
			t.Errorf("%s: status = %d, want %d", target, rec.Code, http.StatusFound)
		}
		if loc := rec.Header().Get("Location"); loc != whatsappURL {
			t.Errorf("%s: Location = %q, want %q", target, loc, whatsappURL)
		}
	}
}

func TestAppHandler_Pecut2ServesAssets(t *testing.T) {
	dirs := appdirs.Under(t.TempDir())
	app := NewApp(dirs, false) // legacyMode = false
	h := app.handler()
	if h != nil {
		t.Errorf("expected nil handler in Pecut 2 mode (to serve embedded assets), got %v", h)
	}
}

func TestApp_OpenAndCloseAccount(t *testing.T) {
	dirs := appdirs.Under(t.TempDir())
	app := NewApp(dirs, false)

	id, err := app.OpenAccount("account-a")
	if err != nil {
		t.Fatalf("OpenAccount failed: %v", err)
	}
	if id != "account-a" {
		t.Errorf("got %q, want account-a", id)
	}

	if err := app.CloseAccount("account-a"); err != nil {
		t.Fatalf("CloseAccount failed: %v", err)
	}
	time.Sleep(1 * time.Second)
}

