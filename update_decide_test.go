package main

import (
	"testing"

	"wassap-desktop/internal/updater"
)

func TestShouldAutoDownloadTruthTable(t *testing.T) {
	cases := []struct {
		name       string
		autoUpdate bool
		state      updater.State
		dismissed  bool
		want       bool
	}{
		{"opted in and available", true, updater.StateAvailable, false, true},
		{"opted in but dismissed", true, updater.StateAvailable, true, false},
		{"opted in but up to date", true, updater.StateUpToDate, false, false},
		{"opted out, available", false, updater.StateAvailable, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAutoDownload(tc.autoUpdate, updater.Status{State: tc.state, Dismissed: tc.dismissed})
			if got != tc.want {
				t.Errorf("shouldAutoDownload(%v, state=%s, dismissed=%v) = %v, want %v", tc.autoUpdate, tc.state, tc.dismissed, got, tc.want)
			}
		})
	}
}
