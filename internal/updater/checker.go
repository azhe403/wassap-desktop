package updater

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State is the update lifecycle's coarse status as seen by the frontend.
type State string

const (
	StateUpToDate    State = "up-to-date"
	StateAvailable   State = "available"
	StateUnsupported State = "unsupported"
	StateError       State = "error"
	StateUnknown     State = "unknown"
)

// Status is what the UI needs to decide what to show. It is JSON-marshaled
// into the `update:status` Wails event verbatim.
type Status struct {
	State           State     `json:"state"`
	Channel         string    `json:"channel"`
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	ReleaseURL      string    `json:"releaseURL"`
	Message         string    `json:"message,omitempty"`
	CheckedAt       time.Time `json:"checkedAt"`
	LatestBuildDate time.Time `json:"latestBuildDate"`
	Dismissed       bool      `json:"dismissed"`
}

// Prefs is the user's update preference, resolved by the binding layer.
type Prefs struct {
	AutoUpdate bool
	Channel    string // "stable" or "nightly"
}

// Checker performs throttled update checks against GitHub releases.
// The offer gate is ahead-only: a release counts as new only when the ref it
// lives on (tag for stable, main for nightly) contains commits the running
// build does not. It never suggests a downgrade.
type Checker struct {
	client    *Client
	cachePath string
	version   string
	commit    string

	now func() time.Time // test seam

	mu          sync.Mutex
	lastRelease *Release // stashed for Asset(); guarded
}

// NewChecker builds a Checker persisting its cache at cachePath.
func NewChecker(client *Client, cachePath, version, commit string) *Checker {
	return &Checker{client: client, cachePath: cachePath, version: version, commit: commit, now: time.Now}
}

// Asset returns the release asset matching goos/goarch from the last
// successful check, or false when no check has produced one or the platform
// is unsupported.
func (c *Checker) Asset(goos, goarch string) (Asset, bool) {
	name, ok := AssetName(goos, goarch)
	if !ok {
		return Asset{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastRelease == nil {
		return Asset{}, false
	}
	for _, a := range c.lastRelease.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Check performs one update check honoring the throttle. force bypasses it
// (the user clicked "Check for updates"). It never panics and never surfaces
// a network error as a Go error — failures arrive in Status.Message.
func (c *Checker) Check(ctx context.Context, p Prefs, force bool) (Status, error) {
	if c.version == "" || c.version == "dev" || c.commit == "" {
		return Status{State: StateUnsupported, Channel: p.Channel, CurrentVersion: c.version, CheckedAt: c.now()}, nil
	}
	if p.Channel != "stable" && p.Channel != "nightly" {
		p.Channel = "stable"
	}

	cached, err := c.cache().Load()
	if err != nil {
		cached = UpdateCheckCache{}
	}
	if !force && fresh(cached, c.now()) {
		return statusFromCache(cached), nil
	}

	st, rel, err := c.checkNow(ctx, p, cached)
	if err != nil {
		// Keep the previous successful result in the cache; just record the
		// failed attempt so the next check backs off for 1h.
		failed := cached
		failed.LastAttempt = c.now()
		failed.LastError = err.Error()
		_ = c.cache().Store(failed)
		return statusOnError(err, cached, p.Channel, c.version, c.now()), nil
	}

	res := cached
	res.Channel = p.Channel
	res.CurrentVersion = c.version
	res.LatestVersion = st.LatestVersion
	res.UpdateAvailable = st.State == StateAvailable
	res.ReleaseURL = st.ReleaseURL
	res.PublishedAt = st.LatestBuildDate
	res.CheckedAt = c.now()
	res.LastAttempt = c.now()
	res.LastError = ""
	res.StagedAsset, res.StagedDigest = "", "" // new release invalidates any staged download
	_ = c.cache().Store(res)

	c.mu.Lock()
	c.lastRelease = rel
	c.mu.Unlock()
	return st, nil
}

func (c *Checker) cache() *Cache { return &Cache{Path: c.cachePath} }

// Snapshot reads the cached status without touching the network. Before the
// first check completes it reports StateUnknown. Used to give the UI an
// initial paint without waiting for a check result.
func (c *Checker) Snapshot() Status {
	cached, err := c.cache().Load()
	if err != nil {
		cached = UpdateCheckCache{}
	}
	empty := cached.LastAttempt.IsZero() && cached.CheckedAt.IsZero() &&
		cached.LastError == "" && cached.CurrentVersion == "" && cached.Channel == ""
	if empty {
		return Status{State: StateUnknown, Channel: "stable", CurrentVersion: c.version}
	}
	st := statusFromCache(cached)
	st.CurrentVersion = c.version
	if st.Channel == "" {
		st.Channel = "stable"
	}
	return st
}

// fresh reports whether the last attempt is recent enough to skip the network.
func fresh(c UpdateCheckCache, now time.Time) bool {
	if c.LastAttempt.IsZero() {
		return false
	}
	window := 12 * time.Hour
	if c.LastError != "" {
		window = time.Hour
	}
	return now.Sub(c.LastAttempt) < window
}

func statusFromCache(c UpdateCheckCache) Status {
	st := Status{
		Channel:         c.Channel,
		CurrentVersion:  c.CurrentVersion,
		LatestVersion:   c.LatestVersion,
		ReleaseURL:      c.ReleaseURL,
		CheckedAt:       c.CheckedAt,
		LatestBuildDate: c.PublishedAt,
	}
	if c.LastError != "" {
		st.State = errorStateFor(c)
		st.Message = c.LastError
		st.CheckedAt = c.CheckedAt // preserve last success time
	} else if c.UpdateAvailable {
		st.State = StateAvailable
	} else {
		st.State = StateUpToDate
	}
	st.Dismissed = c.Dismissed(st.LatestVersion)
	return st
}

func statusOnError(cause error, cached UpdateCheckCache, channel, version string, now time.Time) Status {
	st := Status{
		Channel:         channel,
		CurrentVersion:  version,
		LatestVersion:   cached.LatestVersion,
		ReleaseURL:      cached.ReleaseURL,
		Message:         cause.Error(),
		CheckedAt:       cached.CheckedAt,   // keep the last successful check's time
		LatestBuildDate: cached.PublishedAt, // keep the known publish time across failures
		Dismissed:       cached.Dismissed(cached.LatestVersion),
	}
	if cached.CheckedAt.IsZero() {
		st.State = StateUpToDate
		st.CheckedAt = now
		return st
	}
	st.State = StateError
	return st
}

func errorStateFor(c UpdateCheckCache) State {
	if !c.CheckedAt.IsZero() {
		return StateError
	}
	return StateUpToDate
}

func (c *Checker) checkNow(ctx context.Context, p Prefs, cached UpdateCheckCache) (Status, *Release, error) {
	switch p.Channel {
	case "nightly":
		return c.checkNightly(ctx, p, cached)
	default:
		return c.checkStable(ctx, p, cached)
	}
}

func (c *Checker) checkStable(ctx context.Context, p Prefs, cached UpdateCheckCache) (Status, *Release, error) {
	rel, err := c.client.LatestRelease(ctx)
	if err != nil {
		return Status{}, nil, err
	}
	if rel == nil {
		return Status{
			State:          StateUpToDate,
			Channel:        p.Channel,
			CurrentVersion: c.version,
			Message:        "no stable release yet",
			CheckedAt:      c.now(),
		}, nil, nil
	}
	ahead, err := c.client.AheadBy(ctx, c.commit, rel.TagName)
	if err != nil {
		return Status{}, nil, err
	}
	st := Status{
		Channel:         p.Channel,
		CurrentVersion:  c.version,
		LatestVersion:   rel.TagName,
		ReleaseURL:      rel.HTMLURL,
		CheckedAt:       c.now(),
		LatestBuildDate: rel.PublishedAt.UTC(),
		Dismissed:       cached.Dismissed(rel.TagName),
	}
	if ahead > 0 {
		st.State = StateAvailable
	} else {
		st.State = StateUpToDate
	}
	return st, rel, nil
}

func (c *Checker) checkNightly(ctx context.Context, p Prefs, cached UpdateCheckCache) (Status, *Release, error) {
	rel, err := c.client.ReleaseByTag(ctx, "nightly")
	if err != nil {
		return Status{}, nil, err
	}
	if rel == nil {
		return Status{}, nil, errors.New("updater: nightly release not found")
	}
	ahead, err := c.client.AheadBy(ctx, c.commit, "main")
	if err != nil {
		return Status{}, nil, err
	}
	// The nightly release is re-created on every push to main, so the
	// published timestamp works as a per-build identity for dismissals.
	latest := "nightly " + rel.PublishedAt.UTC().Format("2006-01-02T15:04Z")
	st := Status{
		Channel:         p.Channel,
		CurrentVersion:  c.version,
		LatestVersion:   latest,
		ReleaseURL:      rel.HTMLURL,
		CheckedAt:       c.now(),
		LatestBuildDate: rel.PublishedAt.UTC(),
		Dismissed:       cached.Dismissed(latest),
	}
	if ahead > 0 {
		st.State = StateAvailable
	} else {
		st.State = StateUpToDate
	}
	return st, rel, nil
}
