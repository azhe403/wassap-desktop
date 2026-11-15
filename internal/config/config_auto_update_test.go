package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"wassap-desktop/internal/config"
)

func TestJSONStore_AutoUpdateRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	auto, err := store.GetAutoUpdate()
	if err != nil {
		t.Fatalf("GetAutoUpdate() unexpected error: %v", err)
	}
	if auto != false {
		t.Errorf("expected auto-update off by default, got %v", auto)
	}

	if err := store.SetAutoUpdate(true); err != nil {
		t.Fatalf("SetAutoUpdate(true) failed: %v", err)
	}
	auto, err = store.GetAutoUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !auto {
		t.Error("expected auto-update on after round trip")
	}

	if err := store.SetAutoUpdate(false); err != nil {
		t.Fatal(err)
	}
	auto, err = store.GetAutoUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if auto {
		t.Error("expected auto-update off after round trip")
	}
}

func TestJSONStore_UpdateChannelRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	channel, err := store.GetUpdateChannel()
	if err != nil {
		t.Fatal(err)
	}
	if channel != "stable" {
		t.Errorf("expected default channel stable, got %q", channel)
	}

	if err := store.SetUpdateChannel("nightly"); err != nil {
		t.Fatal(err)
	}
	channel, err = store.GetUpdateChannel()
	if err != nil {
		t.Fatal(err)
	}
	if channel != "nightly" {
		t.Errorf("expected nightly after round trip, got %q", channel)
	}

	if err := store.SetUpdateChannel("stable"); err != nil {
		t.Fatal(err)
	}
}

func TestJSONStore_SetUpdateChannelRejectsUnknownChannel(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	if err := store.SetUpdateChannel("beta"); err == nil {
		t.Fatal("expected an error for an unknown channel")
	}
	if err := store.SetUpdateChannel("STABLE"); err == nil {
		t.Fatal("channel validation must be strict (case-sensitive)")
	}
	channel, err := store.GetUpdateChannel()
	if err != nil {
		t.Fatal(err)
	}
	if channel != "stable" {
		t.Errorf("rejected channel must not be persisted, got %q", channel)
	}
}

func TestJSONStore_UpdatePrefsPreserveCloseToTray(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	store := config.NewStore(configPath)

	if err := store.SetCloseToTray(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoUpdate(true); err != nil {
		t.Fatal(err)
	}
	if err := store.SetUpdateChannel("nightly"); err != nil {
		t.Fatal(err)
	}

	tray, err := store.GetCloseToTray()
	if err != nil {
		t.Fatal(err)
	}
	if !tray {
		t.Error("close-to-tray must survive auto-update preference writes")
	}
	auto, err := store.GetAutoUpdate()
	if err != nil {
		t.Fatal(err)
	}
	if !auto {
		t.Error("auto-update must survive channel writes")
	}
}

func TestJSONStore_MissingChannelDefaultsToStable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Hand-write a config that predates the updateChannel field.
	if err := os.WriteFile(configPath, []byte(`{"closeToTray": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.NewStore(configPath)
	channel, err := store.GetUpdateChannel()
	if err != nil {
		t.Fatal(err)
	}
	if channel != "stable" {
		t.Errorf("missing updateChannel must default to stable, got %q", channel)
	}
	tray, err := store.GetCloseToTray()
	if err != nil {
		t.Fatal(err)
	}
	if !tray {
		t.Error("pre-existing closeToTray must be preserved")
	}
}
