package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestApp_AccountManagementLifecycle(t *testing.T) {
	tempHome := t.TempDir()
	dirs := appdirs.Under(tempHome)
	app := NewApp(dirs, false)

	// 1. Create two accounts
	acc1, err := app.CreateAccount("Personal Account")
	if err != nil {
		t.Fatalf("CreateAccount acc1 failed: %v", err)
	}
	if acc1.Name != "Personal Account" || len(acc1.ID) != 26 {
		t.Fatalf("unexpected acc1: %+v", acc1)
	}

	acc2, err := app.CreateAccount("Work Account")
	if err != nil {
		t.Fatalf("CreateAccount acc2 failed: %v", err)
	}

	// 2. List accounts
	list, err := app.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(list))
	}

	// 3. Rename account
	if err := app.RenameAccount(acc1.ID, "Family Account"); err != nil {
		t.Fatalf("RenameAccount failed: %v", err)
	}

	// 4. Get account
	got, err := app.GetAccount(acc1.ID)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if got.Name != "Family Account" {
		t.Fatalf("expected Family Account, got %s", got.Name)
	}

	// 5. Simulate application restart
	appRestarted := NewApp(dirs, false)
	reloadedList, err := appRestarted.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts after restart failed: %v", err)
	}
	if len(reloadedList) != 2 {
		t.Fatalf("expected 2 accounts after restart, got %d", len(reloadedList))
	}
	if reloadedList[0].Name != "Family Account" || reloadedList[1].Name != "Work Account" {
		t.Fatalf("reloaded accounts mismatch: %+v", reloadedList)
	}

	// 6. Remove acc1: acc2 and its profile must remain 100% intact
	if err := appRestarted.RemoveAccount(acc1.ID); err != nil {
		t.Fatalf("RemoveAccount acc1 failed: %v", err)
	}

	listAfterRemove, err := appRestarted.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts after remove failed: %v", err)
	}
	if len(listAfterRemove) != 1 || listAfterRemove[0].ID != acc2.ID {
		t.Fatalf("expected only acc2 remaining, got %+v", listAfterRemove)
	}

	// Verify acc2 can still be retrieved
	if _, err := appRestarted.GetAccount(acc2.ID); err != nil {
		t.Fatalf("acc2 should still exist: %v", err)
	}
}

func TestApp_CorruptAccountsJSONDisablesAccountBindings(t *testing.T) {
	tempHome := t.TempDir()
	dirs := appdirs.Under(tempHome)

	// Corrupt accounts.json so repository initialization fails.
	if err := os.MkdirAll(dirs.Data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirs.Data, "accounts.json"), []byte("{ not json "), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp(dirs, false)

	// Every account binding must fail gracefully, not panic.
	if _, err := app.ListAccounts(); err == nil {
		t.Fatal("ListAccounts: expected error with corrupt repository, got nil")
	}
	if _, err := app.GetAccount("01m6ad2v4mgwf313dm0tb6f5mf"); err == nil {
		t.Fatal("GetAccount: expected error, got nil")
	}
	if _, err := app.CreateAccount("X"); err == nil {
		t.Fatal("CreateAccount: expected error, got nil")
	}
	if err := app.RenameAccount("01m6ad2v4mgwf313dm0tb6f5mf", "X"); err == nil {
		t.Fatal("RenameAccount: expected error, got nil")
	}
	if err := app.RemoveAccount("01m6ad2v4mgwf313dm0tb6f5mf"); err == nil {
		t.Fatal("RemoveAccount: expected error, got nil")
	}

	// Corrupt file must remain untouched on disk.
	content, err := os.ReadFile(filepath.Join(dirs.Data, "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "{ not json " {
		t.Fatalf("corrupt file was modified: %s", string(content))
	}
}


