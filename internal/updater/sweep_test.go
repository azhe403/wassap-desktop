package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepRestoresMissingExeFromOld(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "app.exe.old-1234")
	if err := os.WriteFile(old, []byte("original-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "app.exe")

	if err := StartupSweep(exe); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original-binary" {
		t.Errorf("exe restored from .old-* must hold the original binary, got %q", got)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the .old-* file must have been consumed by the restore")
	}
}

func TestSweepDeletesOldFilesWhenExePresent(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(exe, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, "app.exe.old-9999")
	if err := os.WriteFile(stale, []byte("ancient"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	if err := StartupSweep(exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale .old-* must be removed when the exe is present")
	}
	keep := filepath.Join(dir, "app.exe.old-1111")
	if err := os.WriteFile(keep, []byte("fresh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := StartupSweep(exe); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("a young .old-* file must be kept (the current instance may still be running)")
	}
}

func TestSweepLeavesUnrelatedFilesAlone(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "app.exe")
	if err := os.WriteFile(exe, []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelated := []string{
		"other.exe.old-1",
		"app.exe.bak",
		"app.exe.old-",   // missing pid
		"app.exe.old-12x", // non-numeric pid
		"README.txt",
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range unrelated {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	if err := StartupSweep(exe); err != nil {
		t.Fatal(err)
	}
	for _, name := range unrelated {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("unrelated file %s was touched: %v", name, err)
		}
	}
}
