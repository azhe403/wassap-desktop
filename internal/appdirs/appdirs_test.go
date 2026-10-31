package appdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnder_UsesFixedHomeRelativeLayout(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "tester")
	d := Under(home)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"home", d.Home, home},
		{"data", d.Data, filepath.Join(home, ".local", "share", "wassap-desktop")},
		{"config", d.Config, filepath.Join(home, ".config", "wassap-desktop")},
		{"cache", d.Cache, filepath.Join(home, ".cache", "wassap-desktop")},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

// The layout must be identical on every OS: no %APPDATA%, no ~/Library, no
// $XDG_DATA_HOME. Only the path separator may differ.
func TestUnder_LayoutIsPlatformIndependent(t *testing.T) {
	home := t.TempDir()
	d := Under(home)

	for _, dir := range []string{d.Data, d.Config, d.Cache} {
		rel, err := filepath.Rel(home, dir)
		if err != nil {
			t.Fatalf("Rel(%q, %q) = %v", home, dir, err)
		}
		switch rel {
		case filepath.Join(".local", "share", "wassap-desktop"),
			filepath.Join(".config", "wassap-desktop"),
			filepath.Join(".cache", "wassap-desktop"):
		default:
			t.Errorf("unexpected relative layout %q", rel)
		}
	}
}

func TestUnder_DoesNotCreateDirectories(t *testing.T) {
	home := t.TempDir()
	d := Under(home)

	if _, err := os.Stat(d.Data); !os.IsNotExist(err) {
		t.Fatalf("Under must not touch the filesystem, but %q exists", d.Data)
	}
}

func TestResolve_UsesUserHomeDir(t *testing.T) {
	// t.Setenv restores the original value automatically.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	d, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	wantHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}
	if d.Home != wantHome {
		t.Errorf("Home = %q, want %q", d.Home, wantHome)
	}
	if d != Under(wantHome) {
		t.Errorf("Resolve() = %+v, want %+v", d, Under(wantHome))
	}
}

func TestEnsure_CreatesDataAndConfig(t *testing.T) {
	d := Under(t.TempDir())

	if err := Ensure(d); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	for _, dir := range []string{d.Data, d.Config} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected %q to exist: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%q is not a directory", dir)
		}
	}
}

func TestEnsure_IsIdempotent(t *testing.T) {
	d := Under(t.TempDir())

	for i := 0; i < 3; i++ {
		if err := Ensure(d); err != nil {
			t.Fatalf("Ensure() call %d error = %v", i+1, err)
		}
	}
}

func TestEnsure_DoesNotRequireHomeToExist(t *testing.T) {
	// A home directory that does not exist yet must still work, as long as it
	// is creatable.
	d := Under(filepath.Join(t.TempDir(), "brand-new-home"))

	if err := Ensure(d); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if _, err := os.Stat(d.Data); err != nil {
		t.Fatalf("expected %q to exist: %v", d.Data, err)
	}
}

func TestEnsure_ReportsErrorForUncreatablePath(t *testing.T) {
	// A regular file where a parent directory is expected cannot be created.
	home := t.TempDir()
	blocker := filepath.Join(home, ".local")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := Ensure(Under(home)); err == nil {
		t.Fatal("Ensure() = nil, want error")
	}
}
