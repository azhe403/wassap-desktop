package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func sha256HexOf(t *testing.T, b []byte) string {
	t.Helper()
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestCopyStagedAndSwapHappyPath(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "target.exe", "old-binary-bytes")
	staged := writeFile(t, dir, "staged.exe", "new-binary-bytes")
	digest := sha256HexOf(t, []byte("new-binary-bytes"))

	newPath, err := CopyStaged(staged, exe, digest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(filepath.Base(newPath), "target.exe.new-") {
		t.Errorf("newPath = %q, want target.exe.new-<pid>", newPath)
	}
	if filepath.Dir(newPath) != dir {
		t.Errorf("newPath must live next to the exe for same-volume rename, got %q", newPath)
	}

	if err := Swap(exe, newPath, os.Rename); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary-bytes" {
		t.Errorf("exe content = %q, want the new binary", got)
	}
	olds, err := filepath.Glob(filepath.Join(dir, "target.exe.old-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(olds) != 1 {
		t.Fatalf("expected exactly one .old-* file, found %v", olds)
	}
	oldBytes, err := os.ReadFile(olds[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(oldBytes) != "old-binary-bytes" {
		t.Errorf("old file content = %q, want the original binary", oldBytes)
	}
}

func TestSwapFirstRenameFailureKeepsEverything(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "target.exe", "old-binary-bytes")
	staged := writeFile(t, dir, "staged.exe", "new-binary-bytes")
	digest := sha256HexOf(t, []byte("new-binary-bytes"))

	newPath, err := CopyStaged(staged, exe, digest)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	failFirst := func(a, b string) error {
		calls++
		if calls == 1 {
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}

	err = Swap(exe, newPath, failFirst)
	if err == nil {
		t.Fatal("expected an error from the failing rename")
	}
	exeBytes, readErr := os.ReadFile(exe)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(exeBytes) != "old-binary-bytes" {
		t.Errorf("exe must be untouched on first-rename failure, got %q", exeBytes)
	}
	// The .new-* file is left for the caller to remove.
	if _, statErr := os.Stat(newPath); statErr != nil {
		t.Errorf(".new-* must be left for the caller to clean up: %v", statErr)
	}
}

func TestSwapSecondRenameFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "target.exe", "old-binary-bytes")
	staged := writeFile(t, dir, "staged.exe", "new-binary-bytes")
	digest := sha256HexOf(t, []byte("new-binary-bytes"))

	newPath, err := CopyStaged(staged, exe, digest)
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	failSecond := func(a, b string) error {
		calls++
		if calls == 2 {
			return errors.New("disk full")
		}
		return os.Rename(a, b)
	}

	err = Swap(exe, newPath, failSecond)
	if err == nil {
		t.Fatal("expected an error from the failing second rename")
	}
	// Rollback must have restored the original exe.
	exeBytes, readErr := os.ReadFile(exe)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(exeBytes) != "old-binary-bytes" {
		t.Errorf("exe must be restored after rollback, got %q", exeBytes)
	}
	if !strings.Contains(err.Error(), ".old-") {
		t.Errorf("error should name the preserved .old-* path, got %q", err)
	}
}

func TestCopyStagedDigestMismatchLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	exe := writeFile(t, dir, "target.exe", "old-binary-bytes")
	staged := writeFile(t, dir, "staged.exe", "new-binary-bytes")

	_, err := CopyStaged(staged, exe, sha256HexOf(t, []byte("something else")))
	if err == nil {
		t.Fatal("digest mismatch must be an error")
	}
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".new-") {
			t.Errorf(".new-* file left behind: %s", e.Name())
		}
	}
}
