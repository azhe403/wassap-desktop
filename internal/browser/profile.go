// Package browser defines the platform-independent browser profile model used to
// give every WhatsApp account its own persistent browser storage.
//
// The package deliberately knows nothing about WebView2, Wails or Windows. It
// owns three things and nothing else:
//
//   - what a profile is on disk (an account id and a directory layout),
//   - which profiles currently exist and which are in use by a live session,
//   - which directory a profile's browser engine must use.
//
// Everything that actually talks to a browser engine lives behind the Renderer
// interfaces below and is supplied by the platform layer.
package browser

// ProfileID is a validated account identifier. The zero value is not a valid
// ID; use ParseProfileID or the IDs returned by Manager.List.
type ProfileID string

// String implements fmt.Stringer.
func (id ProfileID) String() string { return string(id) }

// ProfileState reports what a profile is doing right now.
type ProfileState int

const (
	// StateClosed means the profile exists on disk but has no live session.
	StateClosed ProfileState = iota
	// StateOpen means a session holds the profile and the browser engine is
	// alive for it.
	StateOpen
	// StateClosing means a session is tearing down and the profile must not be
	// opened again until it reaches StateClosed.
	StateClosing
)

// String implements fmt.Stringer.
func (s ProfileState) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Outcome is the result of opening a profile.
type Outcome int

const (
	// OpenedMemory means the browser storage was loaded into memory and the
	// previous on-disk state was kept as a backup.
	OpenedMemory Outcome = iota
	// OpenedClean means no usable storage existed and a fresh profile was
	// started.
	OpenedClean
)

// String implements fmt.Stringer.
func (o Outcome) String() string {
	if o == OpenedMemory {
		return "opened with existing storage"
	}
	return "opened with clean storage"
}

// Probe is the engine's own view of a profile the caller is about to open.
type Probe struct {
	// Path is the directory the engine uses for this profile.
	Path string
	// Locked reports that the engine's exclusive lock file already exists, which
	// means the engine considers the profile to be in use somewhere.
	Locked bool
	// Exited indicates that the previous engine instance for this profile ended
	// by shutting its environment down cleanly.
	Exited bool
}

// Session is one live browser instance bound to one profile.
type Session interface {
	// Profile is the profile the session was opened for.
	Profile() ProfileID
	// Path is the user data folder the session's engine uses.
	Path() string
	// Outcome reports whether stored state or clean state was loaded.
	Outcome() Outcome
	// Show brings the session's window to the front.
	Show() error
	// Hide keeps the engine running but makes the session's window invisible,
	// so another session can take over the embedded region without this
	// account losing its state.
	Hide() error
	// Focus gives the session's window keyboard focus.
	Focus() error
	// Close tears the session down. It blocks until the browser has finished
	// flushing and releasing its storage, or until CloseTimeout elapses.
	Close() error
	// Alive reports whether the session can still be used.
	Alive() bool
}

// Opener starts a browser session for a profile. Implementations are the only
// place that initialises a browser engine.
type Opener interface {
	OpenProfile(id ProfileID, probe Probe) (Session, error)
}

// Evidence is one fact observed in a profile's storage. It exists so the UI can
// show what is actually in a profile instead of asking the user to trust it.
type Evidence struct {
	// Key is a short stable identifier, for example "cookies".
	Key string
	// Label is a human readable description.
	Label string
	// Value is the observed value.
	Value string
	// High means the fact strengthens the case for isolation; Low means it is
	// neutral; Warn means it is consistent with shared state.
	Relevance string
}

// Inspector reads evidence from a profile's storage.
type Inspector interface {
	Inspect(id ProfileID) ([]Evidence, error)
}

// Applier is the storage life cycle the manager needs from the engine's
// directory layout. The manager drives these steps in this order so that a
// crash at any point leaves the profile in a state the next startup can reason
// about.
type Applier interface {
	// LoadMemory moves the persisted browsing state into a fully in-memory copy
	// and keeps the original as a backup. After it returns the engine's storage
	// directory holds no persisted session state.
	LoadMemory(path string) error
	// CommitMemory writes the in-memory copy back to the engine's storage
	// directory and removes the backup.
	CommitMemory(path string) error
	// RestoreBackup puts the backup back in place and drops any uncommitted
	// state.
	RestoreBackup(path string) error
	// DropBackup deletes the backup.
	DropBackup(path string) error
	// HasStorage reports whether the engine already provisioned storage.
	HasStorage(path string) bool
	// LockFile is the file the engine's exclusive lock lives in.
	LockFile(path string) string
	// MarkerFile is the file the platform layer uses to record how the previous
	// engine instance ended.
	MarkerFile(path string) string
	// Purge deletes every file the engine created for the profile.
	Purge(path string) error
}
