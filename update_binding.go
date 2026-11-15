package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"wassap-desktop/internal/updater"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// dlDownloadState is the in-memory download lifecycle, kept separately from
// the checker's status: the checker owns "is there an update", this owns
// "are we fetching it / is it staged". StagedVersion binds the staged file
// to the release identity it was downloaded from; a later check resolving a
// different identity invalidates it.
type dlDownloadState struct {
	State         string // idle | downloading | downloaded | error
	Done          int64
	Total         int64
	StagedPath    string
	StagedSHA     string
	StagedVersion string
	Message       string
}

// VersionInfo is what the About view shows.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// UpdateStatus is the JSON contract between the updater and the frontend
// (bindings and the `update:status` Wails event).
type UpdateStatus struct {
	State          string `json:"state"`
	Channel        string `json:"channel"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseURL"`
	Message        string `json:"message,omitempty"`
	CheckedAt      string `json:"checkedAt,omitempty"`
	ProgressDone   int64  `json:"progressDone,omitempty"`
	ProgressTotal  int64  `json:"progressTotal,omitempty"`
	Dismissed      bool   `json:"dismissed"`
}

func statusToUpdateStatus(st updater.Status) UpdateStatus {
	out := UpdateStatus{
		State:          string(st.State),
		Channel:        st.Channel,
		CurrentVersion: st.CurrentVersion,
		LatestVersion:  st.LatestVersion,
		ReleaseURL:     st.ReleaseURL,
		Message:        st.Message,
		Dismissed:      st.Dismissed,
	}
	if !st.CheckedAt.IsZero() {
		out.CheckedAt = st.CheckedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// loadPrefs reads auto-update preference and channel from the config store.
func (a *App) loadPrefs() (updater.Prefs, error) {
	p := updater.Prefs{Channel: "stable"}
	auto, err := a.configStore.GetAutoUpdate()
	if err != nil {
		return p, err
	}
	p.AutoUpdate = auto
	channel, err := a.configStore.GetUpdateChannel()
	if err != nil {
		return p, err
	}
	p.Channel = channel
	return p, nil
}

// currentUpdateStatus merges the checker's cached status with the download
// lifecycle. It never touches the network.
func (a *App) currentUpdateStatus() UpdateStatus {
	a.updateMu.Lock()
	dl := a.dlState
	st := a.lastStatus
	a.updateMu.Unlock()

	out := statusToUpdateStatus(st)
	switch dl.State {
	case "downloading":
		out.State = "downloading"
		out.ProgressDone = dl.Done
		out.ProgressTotal = dl.Total
	case "downloaded":
		out.State = "downloaded"
		out.ProgressTotal = dl.Total
	case "error":
		out.State = "error"
		out.Message = dl.Message
	case "", "idle":
	}
	if out.State == "" {
		out.State = "unknown"
	}
	return out
}

func (a *App) setLastStatus(st updater.Status) {
	a.updateMu.Lock()
	a.lastStatus = st
	a.updateMu.Unlock()
}

func (a *App) setDLState(dl dlDownloadState) {
	a.updateMu.Lock()
	a.dlState = dl
	a.updateMu.Unlock()
}

// GetVersion returns the build-stamped version and commit.
func (a *App) GetVersion() VersionInfo {
	return VersionInfo{Version: version, Commit: commit}
}

// GetUpdateStatus returns the last known update status. It never performs a
// network request; use CheckForUpdates for that.
func (a *App) GetUpdateStatus() UpdateStatus {
	return a.currentUpdateStatus()
}

// GetAutoUpdate reports whether automatic update downloads are enabled.
func (a *App) GetAutoUpdate() bool {
	auto, err := a.configStore.GetAutoUpdate()
	if err != nil {
		log.Printf("updater: cannot read auto-update preference: %v", err)
		return false
	}
	return auto
}

// SetAutoUpdate persists the auto-update preference. When enabled and an
// undismissed update is already known, it starts the download immediately.
func (a *App) SetAutoUpdate(enabled bool) error {
	if err := a.configStore.SetAutoUpdate(enabled); err != nil {
		return err
	}
	if enabled {
		a.updateMu.Lock()
		st := a.lastStatus
		a.updateMu.Unlock()
		if shouldAutoDownload(true, st) {
			go a.startDownload()
		}
	}
	return nil
}

// GetUpdateChannel returns the configured update channel ("stable" or "nightly").
func (a *App) GetUpdateChannel() string {
	channel, err := a.configStore.GetUpdateChannel()
	if err != nil {
		log.Printf("updater: cannot read update channel: %v", err)
		return "stable"
	}
	return channel
}

// SetUpdateChannel persists the update channel ("stable" or "nightly").
func (a *App) SetUpdateChannel(channel string) error {
	prev := a.GetUpdateChannel()
	if err := a.configStore.SetUpdateChannel(channel); err != nil {
		return err
	}
	// A channel switch is an explicit user action: bypass the throttle so the
	// status row reflects the newly selected channel now, not in 12h.
	if channel != prev {
		go a.updateCheck(context.Background(), true)
	}
	return nil
}

// CheckForUpdates performs a throttled check with the throttle bypassed
// (the user clicked a button). It updates the cache, emits `update:status`,
// and returns the resulting status.
func (a *App) CheckForUpdates() UpdateStatus {
	status := a.updateCheck(context.Background(), true)
	return status
}

// DownloadUpdate starts downloading the staged update in the background.
// It errors when no update is available or a download is already in flight.
func (a *App) DownloadUpdate() error {
	a.updateMu.Lock()
	dl := a.dlState
	st := a.lastStatus
	a.updateMu.Unlock()

	if dl.State == "downloading" {
		return errors.New("download already in progress")
	}
	if dl.State == "downloaded" {
		return nil // already staged; harmless no-op
	}
	if st.State != updater.StateAvailable {
		return errors.New("no update available to download")
	}
	go a.startDownload()
	return nil
}

// RestartToApplyUpdate applies the staged update: it copies the new binary
// next to the current one, spawns the relauncher, swaps the exe, and quits.
// The app quits only after a successful swap.
func (a *App) RestartToApplyUpdate() error {
	a.updateMu.Lock()
	dl := a.dlState
	a.updateMu.Unlock()

	if dl.State != "downloaded" || dl.StagedPath == "" {
		return errors.New("no staged update; download first")
	}
	// Belt-and-braces: refuse to install a staged build the world has moved
	// past, even if the completion-time discard never got a chance to run.
	if staged := dl.StagedVersion; staged != "" {
		if latest := a.lastStatusSnapshot().LatestVersion; latest != "" && latest != staged {
			return fmt.Errorf("staged update %s is superseded by %s; download the latest first", staged, latest)
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	newPath, err := updater.CopyStaged(dl.StagedPath, exe, dl.StagedSHA)
	if err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("stage for apply failed: %w", err)
	}

	cmd := exec.Command(exe, "--relaunch-after-update", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("cannot spawn the relauncher: %w", err)
	}

	if err := updater.Swap(exe, newPath, os.Rename); err != nil {
		_ = cmd.Process.Kill()
		_ = os.Remove(newPath)
		return fmt.Errorf("apply failed: %w", err)
	}

	a.mu.Lock()
	a.forceQuit = true
	a.mu.Unlock()
	a.quitApp()
	return nil
}

// DismissUpdateNotice records the dismissed version. A newer version
// re-arms the notice automatically.
func (a *App) DismissUpdateNotice(version string) error {
	cachePath := filepath.Join(a.dirs.Cache, "update-check.json")
	cache := &updater.Cache{Path: cachePath}
	cached, err := cache.Load()
	if err != nil {
		return err
	}
	cached.DismissedVersion = version
	if err := cache.Store(cached); err != nil {
		return err
	}
	st := a.lastStatusSnapshot()
	st.Dismissed = version == st.LatestVersion
	a.setLastStatus(updater.Status{
		State:          st.State,
		Channel:        st.Channel,
		CurrentVersion: st.CurrentVersion,
		LatestVersion:  st.LatestVersion,
		ReleaseURL:     st.ReleaseURL,
		Message:        st.Message,
		CheckedAt:      st.CheckedAt,
		Dismissed:      st.Dismissed,
	})
	a.emitUpdateStatus()
	return nil
}

func (a *App) lastStatusSnapshot() updater.Status {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	return a.lastStatus
}

// updateCheck performs one throttled (or forced) check, merges the result,
// emits `update:status`, and auto-downloads when the user opted in.
func (a *App) updateCheck(ctx context.Context, force bool) UpdateStatus {
	prefs, err := a.loadPrefs()
	if err != nil {
		log.Printf("updater: cannot load preferences: %v", err)
		return a.currentUpdateStatus()
	}
	st, err := a.checker.Check(ctx, prefs, force)
	if err != nil {
		log.Printf("updater: check failed: %v", err)
		// Checker never returns an error today; treat as a no-op.
		return a.currentUpdateStatus()
	}
	// I-1: a fresher check that resolved a different latest version must
	// invalidate a previously staged download — otherwise "Restart to update"
	// announces the new build while installing the old one.
	a.updateMu.Lock()
	resetDL, reset := resetDownloadStateIfStale(a.dlState, st.LatestVersion)
	oldPath := a.dlState.StagedPath
	a.dlState = resetDL
	a.updateMu.Unlock()
	if reset && oldPath != "" {
		if err := os.Remove(oldPath); err != nil {
			log.Printf("updater: cannot remove superseded staged file %s: %v", oldPath, err)
		}
	}
	a.setLastStatus(st)
	a.emitUpdateStatus()
	if shouldAutoDownload(prefs.AutoUpdate, st) {
		go a.startDownload()
	}
	return statusToUpdateStatus(st)
}

// updateOnStartup sweeps update leftovers, then checks once (if the cache is
// stale) and keeps checking every 12h until the app exits.
func (a *App) updateOnStartup(ctx context.Context) {
	if exe, err := os.Executable(); err == nil {
		if err := updater.StartupSweep(exe); err != nil {
			log.Printf("updater: startup sweep: %v", err)
		}
	} else {
		log.Printf("updater: cannot resolve executable path: %v", err)
	}

	// Initial cache paint so the UI has something to show before the first
	// check completes.
	a.setLastStatus(a.checker.Snapshot())
	a.emitUpdateStatus()

	_ = a.updateCheck(ctx, false)
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.updateCheck(ctx, false)
		}
	}
}

// startDownload resolves and downloads the update in the background.
func (a *App) startDownload() {
	asset, ok := a.checker.Asset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		log.Printf("updater: no asset for %s/%s", runtime.GOOS, runtime.GOARCH)
		return
	}
	stagedVersion := a.lastStatusSnapshot().LatestVersion

	a.setDLState(dlDownloadState{State: "downloading", StagedVersion: stagedVersion})
	a.emitUpdateStatus()

	res, err := a.downloader.Download(context.Background(), asset, func(done, total int64) {
		a.setDLState(dlDownloadState{State: "downloading", Done: done, Total: total, StagedVersion: stagedVersion})
		a.emitUpdateStatus()
	})
	if err != nil {
		log.Printf("updater: download failed: %v", err)
		a.setDLState(dlDownloadState{State: "error", Message: err.Error()})
		a.emitUpdateStatus()
		return
	}

	// The release can move while a download is in flight. If a fresher check
	// already resolved a different latest version, installing what we just
	// staged would be the exact mismatch this path exists to prevent:
	// discard it instead of staging it as "new".
	if _, superseded := resetDownloadStateIfStale(
		dlDownloadState{State: "downloaded", StagedVersion: stagedVersion},
		a.lastStatusSnapshot().LatestVersion,
	); superseded {
		log.Printf("updater: discarding staged %s — a fresher release is already known", stagedVersion)
		_ = os.Remove(res.Path)
		a.setDLState(dlDownloadState{})
		a.emitUpdateStatus()
		return
	}

	// Persist the staged asset metadata so a restart can apply it even if the
	// app closed between download and restart.
	cachePath := filepath.Join(a.dirs.Cache, "update-check.json")
	cache := &updater.Cache{Path: cachePath}
	if cached, err := cache.Load(); err == nil {
		cached.StagedAsset = res.AssetName
		cached.StagedDigest = res.SHA256
		if err := cache.Store(cached); err != nil {
			log.Printf("updater: cannot persist staged asset metadata: %v", err)
		}
	}

	done := int64(0)
	if info, err := os.Stat(res.Path); err == nil {
		done = info.Size()
	}
	a.setDLState(dlDownloadState{State: "downloaded", Done: done, Total: done, StagedPath: res.Path, StagedSHA: res.SHA256, StagedVersion: stagedVersion})
	a.emitUpdateStatus()
}

// emitUpdateStatus marshals UpdateStatus into the `update:status` event.
func (a *App) emitUpdateStatus() {
	ctx := a.ctxRef()
	if ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(ctx, "update:status", a.currentUpdateStatus())
}

func (a *App) ctxRef() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
}
