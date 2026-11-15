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

func TestResetDownloadStateIfStale(t *testing.T) {
	cases := []struct {
		name      string
		dl        dlDownloadState
		latest    string
		wantDL    dlDownloadState
		wantReset bool
	}{
		{
			name:   "same version staged stays staged",
			dl:     dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedSHA: "aaa", StagedVersion: "v0.2.0"},
			latest: "v0.2.0",
			wantDL: dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedSHA: "aaa", StagedVersion: "v0.2.0"},
			wantReset: false,
		},
		{
			name:      "newer release invalidates staged build",
			dl:        dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedSHA: "aaa", StagedVersion: "v0.2.0"},
			latest:    "v0.3.0",
			wantDL:    dlDownloadState{},
			wantReset: true,
		},
		{
			name:      "newer nightly identity invalidates staged nightly",
			dl:        dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedVersion: "nightly 2026-11-15T07:00Z"},
			latest:    "nightly 2026-11-15T09:00Z",
			wantDL:    dlDownloadState{},
			wantReset: true,
		},
		{
			name:      "mid-flight download is not torn down",
			dl:        dlDownloadState{State: "downloading", StagedVersion: "v0.2.0", Done: 10},
			latest:    "v0.3.0",
			wantDL:    dlDownloadState{State: "downloading", StagedVersion: "v0.2.0", Done: 10},
			wantReset: false,
		},
		{
			name:      "missing staged version resets nothing (conservative)",
			dl:        dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe"},
			latest:    "v0.3.0",
			wantDL:    dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe"},
			wantReset: false,
		},
		{
			name:      "empty latest resolution resets nothing",
			dl:        dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedVersion: "v0.2.0"},
			latest:    "",
			wantDL:    dlDownloadState{State: "downloaded", StagedPath: "/x/wassap-desktop.exe", StagedVersion: "v0.2.0"},
			wantReset: false,
		},
		{
			name:      "idle state untouched",
			dl:        dlDownloadState{State: "idle"},
			latest:    "v0.3.0",
			wantDL:    dlDownloadState{State: "idle"},
			wantReset: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reset := resetDownloadStateIfStale(tc.dl, tc.latest)
			if reset != tc.wantReset {
				t.Errorf("reset = %v, want %v", reset, tc.wantReset)
			}
			if got != tc.wantDL {
				t.Errorf("dl = %+v, want %+v", got, tc.wantDL)
			}
		})
	}
}
