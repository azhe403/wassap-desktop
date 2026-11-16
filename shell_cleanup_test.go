package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupStaleShellProfiles(t *testing.T) {
	cases := []struct {
		name     string
		setup    []string // relative entries to create under the wails-shell dir
		selfPID  int
		wantGone []string
		wantStay []string
		wantErr  bool
	}{
		{
			name:     "stale pid dir is removed, current pid dir stays",
			setup:    []string{"run-111", "run-222"},
			selfPID:  222,
			wantGone: []string{"run-111"},
			wantStay: []string{"run-222"},
		},
		{
			name:     "non run-* entries are untouched",
			setup:    []string{"keep.txt", "run-notanumber"},
			selfPID:  222,
			wantStay: []string{"keep.txt", "run-notanumber"},
		},
		{
			name:    "missing wails-shell directory is not an error",
			setup:   nil,
			selfPID: 222,
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			shellDir := filepath.Join(root, "wails-shell")
			for _, rel := range tc.setup {
				if filepath.Ext(rel) == ".txt" {
					if err := os.MkdirAll(shellDir, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(shellDir, rel), []byte("x"), 0o600); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.MkdirAll(filepath.Join(shellDir, rel), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			err := cleanupStaleShellProfiles(root, tc.selfPID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			for _, rel := range tc.wantGone {
				if _, statErr := os.Stat(filepath.Join(shellDir, rel)); !os.IsNotExist(statErr) {
					t.Errorf("expected %s to be removed", rel)
				}
			}
			for _, rel := range tc.wantStay {
				if _, statErr := os.Stat(filepath.Join(shellDir, rel)); statErr != nil {
					t.Errorf("expected %s to stay: %v", rel, statErr)
				}
			}
		})
	}
}
