package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreExe(t *testing.T) {
	t.Run("single candidate restored", func(t *testing.T) {
		dir := t.TempDir()
		old := filepath.Join(dir, "app.exe.old-123")
		if err := os.WriteFile(old, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		exe := filepath.Join(dir, "app.exe")
		restored, err := RestoreExe(exe)
		if err != nil {
			t.Fatal(err)
		}
		if !restored {
			t.Fatal("expected restored=true")
		}
		got, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "original" {
			t.Errorf("restored content = %q", got)
		}
	})

	t.Run("newest of several candidates is restored", func(t *testing.T) {
		dir := t.TempDir()
		older := filepath.Join(dir, "app.exe.old-111")
		newer := filepath.Join(dir, "app.exe.old-222")
		if err := os.WriteFile(older, []byte("old-binary"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(newer, []byte("new-binary"), 0o600); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		_ = os.Chtimes(older, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
		_ = os.Chtimes(newer, now, now)
		exe := filepath.Join(dir, "app.exe")
		restored, err := RestoreExe(exe)
		if err != nil {
			t.Fatal(err)
		}
		if !restored {
			t.Fatal("expected restored=true")
		}
		got, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new-binary" {
			t.Errorf("must restore the newest candidate, got %q", got)
		}
	})

	t.Run("exe present means no-op", func(t *testing.T) {
		dir := t.TempDir()
		exe := filepath.Join(dir, "app.exe")
		if err := os.WriteFile(exe, []byte("current"), 0o600); err != nil {
			t.Fatal(err)
		}
		restored, err := RestoreExe(exe)
		if err != nil {
			t.Fatal(err)
		}
		if restored {
			t.Error("no restore needed when the exe is present")
		}
	})

	t.Run("no candidates means no restore", func(t *testing.T) {
		dir := t.TempDir()
		restored, err := RestoreExe(filepath.Join(dir, "app.exe"))
		if err != nil {
			t.Fatal(err)
		}
		if restored {
			t.Error("nothing to restore must not report restored")
		}
	})

	t.Run("non-matching siblings are ignored", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"other.exe.old-1", "app.exe.bak", "app.exe.old-", "app.exe.old-12x"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		restored, err := RestoreExe(filepath.Join(dir, "app.exe"))
		if err != nil {
			t.Fatal(err)
		}
		if restored {
			t.Error("non-matching files must not be restored")
		}
	})
}

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
