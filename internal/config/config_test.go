package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"wassap-desktop/internal/config"
)

func TestJSONStore_MissingFileReturnsDefault(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.ActiveAccountID != "" {
		t.Errorf("expected empty ActiveAccountID, got %q", cfg.ActiveAccountID)
	}

	active, err := store.GetActiveAccount()
	if err != nil {
		t.Fatalf("GetActiveAccount() unexpected error: %v", err)
	}
	if active != "" {
		t.Errorf("expected empty active account, got %q", active)
	}

	tray, err := store.GetCloseToTray()
	if err != nil {
		t.Fatalf("GetCloseToTray() unexpected error: %v", err)
	}
	if tray != false {
		t.Errorf("expected CloseToTray false by default, got %v", tray)
	}
}

func TestJSONStore_SaveAndLoadCloseToTray(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	if err := store.SetCloseToTray(true); err != nil {
		t.Fatalf("SetCloseToTray(true) failed: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.CloseToTray {
		t.Errorf("expected CloseToTray true, got false")
	}

	val, err := store.GetCloseToTray()
	if err != nil {
		t.Fatalf("GetCloseToTray() failed: %v", err)
	}
	if !val {
		t.Errorf("expected GetCloseToTray() true, got false")
	}

	if err := store.SetCloseToTray(false); err != nil {
		t.Fatalf("SetCloseToTray(false) failed: %v", err)
	}
	val, _ = store.GetCloseToTray()
	if val {
		t.Errorf("expected GetCloseToTray() false, got true")
	}
}

func TestJSONStore_SaveAndLoadActiveAccount(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	const expectedID = "01m6ad2v4mgwf313dm0tb6f5mf"
	if err := store.SetActiveAccount(expectedID); err != nil {
		t.Fatalf("SetActiveAccount() failed: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.ActiveAccountID != expectedID {
		t.Errorf("expected %q, got %q", expectedID, cfg.ActiveAccountID)
	}

	active, err := store.GetActiveAccount()
	if err != nil {
		t.Fatalf("GetActiveAccount() failed: %v", err)
	}
	if active != expectedID {
		t.Errorf("expected %q, got %q", expectedID, active)
	}
}

func TestJSONStore_CorruptJSONReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte("{ bad json "), 0o600); err != nil {
		t.Fatal(err)
	}

	store := config.NewStore(configPath)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error on corrupt json, got nil")
	}
}

func TestJSONStore_AutoCreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nested", "sub", "config.json")
	store := config.NewStore(configPath)

	if err := store.SetActiveAccount("01m6ad2v4mgwf313dm0tb6f5mf"); err != nil {
		t.Fatalf("SetActiveAccount() failed with nested path: %v", err)
	}

	active, err := store.GetActiveAccount()
	if err != nil {
		t.Fatalf("GetActiveAccount() failed: %v", err)
	}
	if active != "01m6ad2v4mgwf313dm0tb6f5mf" {
		t.Errorf("expected active account, got %q", active)
	}
}
