package browser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"wassap-desktop/internal/browser/cookiestore"
)

// Errors returned by Manager. They are wrapped, so use errors.Is.
var (
	// ErrProfileExists is returned by Create for an id that already exists.
	ErrProfileExists = errors.New("browser: profile already exists")
	// ErrProfileNotFound is returned when a profile does not exist on disk.
	ErrProfileNotFound = errors.New("browser: profile not found")
	// ErrProfileOpen is returned when a profile must be closed first.
	ErrProfileOpen = errors.New("browser: profile is open")
	// ErrProfileBusy is returned when another window (in this or another
	// process) holds the profile.
	ErrProfileBusy = errors.New("browser: profile is in use")
	// ErrSessionEnded is returned by Session operations after Close.
	ErrSessionEnded = errors.New("browser: session has ended")
)

// ManagerInterface is the profile management abstraction. The concrete
// implementation is Manager; the interface exists so the app layer can be
// tested without a browser engine.
//
// Create and Open both reserve the profile before touching storage, so a second
// caller - including a second process - gets ErrProfileBusy instead of a second
// browser sharing the same user data folder. Delete refuses to touch a profile
// that is open anywhere, and nothing here ever deletes a profile on its own.
type ManagerInterface interface {
	Create(accountID string) error
	Open(accountID string) error
	Close(accountID string) error
	Delete(accountID string) error
}

// Lock is the cross-process exclusivity primitive the manager needs. The
// Windows implementation uses an OS file lock, so the guarantee holds between
// processes too.
type Lock interface {
	// TryLock returns ErrProfileBusy when the path is already held.
	TryLock(path string) error
	// Unlock releases the lock and the handle.
	Unlock() error
}

// LockFunc opens the lock file at path and locks it, returning ErrProfileBusy
// when another process holds it.
type LockFunc func(path string) (Lock, error)

// Option configures a Manager.
type Option func(*Manager)

// WithClock replaces the time source, for tests.
func WithClock(now func() time.Time) Option {
	return func(m *Manager) { m.now = now }
}

// WithLock replaces the lock implementation, for tests.
func WithLock(fn LockFunc) Option {
	return func(m *Manager) { m.locker = fn }
}

// Manager owns the profile life cycle. It is safe for concurrent use, but it
// serialises every state change, because opening and closing a profile is a
// multi-step filesystem operation.
type Manager struct {
	layout    Layout
	applier   Applier
	opener    Opener
	inspector Inspector
	locker    LockFunc
	now       func() time.Time

	mu      sync.Mutex
	entries map[ProfileID]*entry
}

// entry is the in-memory state of one profile.
type entry struct {
	id      ProfileID
	probe   Probe
	session Session
	state   ProfileState
	closing chan struct{}
	// lastErr is the error of the most recent failed transition, kept so a
	// half-applied profile can be reported and recovered from.
	lastErr error
}

// ManagerDeps are the collaborators a Manager needs.
type ManagerDeps struct {
	Layout    Layout
	Applier   Applier
	Opener    Opener
	Inspector Inspector
	Lock      LockFunc
	Now       func() time.Time
}

// NewManager returns a Manager for the given dependencies. The manager performs
// no filesystem mutation until it is used, so constructing it during startup
// never deletes or rewrites a profile.
func NewManager(deps ManagerDeps, opts ...Option) *Manager {
	m := &Manager{
		layout:    deps.Layout,
		applier:   deps.Applier,
		opener:    deps.Opener,
		inspector: deps.Inspector,
		locker:    deps.Lock,
		now:       deps.Now,
		entries:   make(map[ProfileID]*entry),
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.now == nil {
		m.now = time.Now
	}
	return m
}

// Layout exposes the directories in use.
func (m *Manager) Layout() Layout { return m.layout }

// Create makes a new, empty profile. It never opens a browser and never
// overwrites an existing profile.
func (m *Manager) Create(accountID string) error {
	id, err := ParseProfileID(accountID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	dir := m.layout.ProfilePath(id)
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		return fmt.Errorf("%w: %s", ErrProfileExists, id)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("browser: cannot inspect profile %s: %w", id, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("browser: cannot create profile %s: %w", id, err)
	}

	e := &entry{id: id, probe: Probe{Path: m.layout.UserDataDir(id)}}
	m.entries[id] = e
	if err := m.saveManifestLocked(); err != nil {
		return err
	}
	return nil
}

// Open starts a browser session for the profile.
//
// The profile is reserved first, then the previous run's leftovers are resolved,
// then the engine is started against a private user data folder. On any failure
// the on-disk state is put back the way it was.
func (m *Manager) Open(accountID string) error {
	id, err := ParseProfileID(accountID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if e, ok := m.entries[id]; ok && e.session != nil && e.state == StateOpen && e.session.Alive() {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s is already open", ErrProfileOpen, id)
	}
	m.mu.Unlock()

	// Reserve the profile across processes before anything else happens.
	locks, err := m.acquireLocks(id)
	if err != nil {
		return err
	}
	releaseLocks := func() {
		for _, l := range locks {
			_ = l.Unlock()
		}
	}
	defer releaseLocks()

	if _, err := os.Stat(m.layout.ProfilePath(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrProfileNotFound, id)
		}
		return fmt.Errorf("browser: cannot inspect profile %s: %w", id, err)
	}

	// Refuse to open a profile whose lock file is still held by another
	// process: the manager holds its own lock, so an existing engine lock
	// means someone else is using this folder without honouring ours.
	if m.locker != nil {
		if _, err := os.Stat(m.applier.LockFile(m.layout.UserDataDir(id))); err == nil {
			return fmt.Errorf("%w: %s", ErrProfileBusy, id)
		}
	}

	// Recover from a previous instance that did not close cleanly. This runs
	// before any inference, so a stale memory folder is never taken for a
	// stored one.
	if err := m.Recover(id); err != nil {
		return err
	}

	probe, err := m.probe(id)
	if err != nil {
		return err
	}

	outcome := OpenedMemory
	// When nothing was ever committed, the engine starts a fresh profile.
	if !m.applier.HasStorage(probe.Path) {
		outcome = OpenedClean
	}

	if outcome == OpenedMemory {
		if err := m.applier.LoadMemory(probe.Path); err != nil {
			return fmt.Errorf("browser: cannot load profile %s into memory: %w", id, err)
		}
	}

	m.mu.Lock()
	e := m.entries[id]
	if e == nil {
		e = &entry{id: id}
		m.entries[id] = e
	}
	e.state = StateClosing
	e.closing = make(chan struct{})
	close(e.closing)
	e.closing = nil
	e.probe = Probe{Path: probe.Path, Exited: probe.Exited}
	m.mu.Unlock()

	memoryProbe := Probe{Path: m.layout.MemoryDir(id)}
	session, err := m.opener.OpenProfile(id, memoryProbe)
	if err != nil {
		// Put the previous state back so a failed open cannot cost the user
		// their session.
		if outcome == OpenedMemory {
			_ = m.applier.RestoreBackup(probe.Path)
		}
		m.mu.Lock()
		e.state = StateClosed
		e.session = nil
		e.lastErr = err
		m.mu.Unlock()
		return fmt.Errorf("browser: cannot open profile %s: %w", id, err)
	}

	m.mu.Lock()
	e.session = session
	e.state = StateOpen
	e.lastErr = nil
	m.mu.Unlock()
	return nil
}

// Close tears down the session and commits its storage.
func (m *Manager) Close(accountID string) error {
	id, err := ParseProfileID(accountID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	e := m.entries[id]
	if e == nil || e.session == nil || e.state == StateClosed {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s is not open", ErrSessionEnded, id)
	}
	session := e.session
	e.state = StateClosing
	e.closing = make(chan struct{})
	done := e.closing
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		e.state = StateClosed
		if e.closing == done {
			close(e.closing)
			e.closing = nil
		}
		e.session = nil
		m.mu.Unlock()
	}()

	if err := session.Close(); err != nil {
		// The engine did not shut down cleanly. Keep the previous state and
		// let Recover handle the leftovers on the next open.
		m.mu.Lock()
		e.lastErr = err
		m.mu.Unlock()
		return err
	}

	if err := m.commit(id, e.probe.Path); err != nil {
		m.mu.Lock()
		e.lastErr = err
		m.mu.Unlock()
		return err
	}
	return nil
}

// Delete removes a profile and every byte of storage it created. It refuses to
// delete a profile that is open, in this process or another one.
func (m *Manager) Delete(accountID string) error {
	id, err := ParseProfileID(accountID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	if e := m.entries[id]; e != nil && (e.state == StateOpen || e.state == StateClosing) {
		m.mu.Unlock()
		return fmt.Errorf("%w: %s is open", ErrProfileOpen, id)
	}
	m.mu.Unlock()

	locks, err := m.acquireLocks(id)
	if err != nil {
		return err
	}
	defer func() {
		for _, l := range locks {
			_ = l.Unlock()
		}
	}()

	dir := m.layout.ProfilePath(id)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrProfileNotFound, id)
		}
		return fmt.Errorf("browser: cannot inspect profile %s: %w", id, err)
	}
	if _, err := os.Stat(m.applier.LockFile(m.layout.UserDataDir(id))); err == nil {
		return fmt.Errorf("%w: %s is open in another process", ErrProfileBusy, id)
	}

	// Move first, then purge. The rename is atomic, so a crash leaves either a
	// complete profile or a complete trash entry, never a half-deleted one.
	trash := m.layout.TrashDir(id)
	if err := os.RemoveAll(trash); err != nil {
		return fmt.Errorf("browser: cannot clear trash for %s: %w", id, err)
	}
	if err := os.MkdirAll(m.layout.TrashRoot, 0o700); err != nil {
		return fmt.Errorf("browser: cannot prepare trash: %w", err)
	}
	if err := os.Rename(dir, trash); err != nil {
		return fmt.Errorf("browser: cannot move profile %s to trash: %w", id, err)
	}
	for _, extra := range []string{m.layout.StagingDir(id)} {
		_ = os.RemoveAll(extra)
	}

	m.mu.Lock()
	delete(m.entries, id)
	err = m.saveManifestLocked()
	m.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(trash); err != nil {
		return fmt.Errorf("browser: profile %s was removed but its trash could not be purged: %w", id, err)
	}
	return nil
}

// Recover resolves leftovers from an instance that did not close cleanly.
//
// It is idempotent and never touches a profile that is held by a live process.
// The caller must hold the profile lock.
func (m *Manager) Recover(id ProfileID) error {
	if _, err := os.Stat(m.layout.ProfilePath(id)); err != nil {
		return nil
	}
	if !m.applier.HasStorage(m.layout.MemoryDir(id)) {
		// Nothing was staged, so there is nothing to put back.
		return nil
	}
	if m.applier.HasStorage(m.layout.BackupDir(id)) {
		if m.applier.HasStorage(m.layout.UserDataDir(id)) {
			// Uncommitted changes from the crashed run; the backup is the
			// state that was actually committed.
			if err := m.applier.RestoreBackup(m.layout.UserDataDir(id)); err != nil {
				return fmt.Errorf("browser: cannot restore backup for %s: %w", id, err)
			}
		} else {
			_ = m.applier.DropBackup(m.layout.UserDataDir(id))
		}
	}
	return m.discardStorage(id)
}

// List returns every profile, sorted by id, with its in-memory state.
func (m *Manager) List() []Info {
	m.mu.Lock()
	states := make(map[ProfileID]entrySnapshot, len(m.entries))
	for id, e := range m.entries {
		states[id] = entrySnapshot{state: e.state, opened: e.session != nil, err: e.lastErr}
	}
	m.mu.Unlock()

	des, err := os.ReadDir(m.layout.ProfilesRoot)
	if err != nil {
		return nil
	}
	var ids []ProfileID
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		id, err := ParseProfileID(de.Name())
		if err != nil {
			// A directory whose name is not a valid id was never created by
			// this manager; leave it alone and keep it out of the list.
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := make([]Info, 0, len(ids))
	for _, id := range ids {
		info := Info{
			ID:        id,
			Path:      m.layout.ProfilePath(id),
			UserData:  m.layout.UserDataDir(id),
			State:     StateClosed,
			CreatedAt: manifestTime(m.layout.ManifestPath, id),
			Layout:    "one user data folder per account",
		}
		if s, ok := states[id]; ok {
			info.State = s.state
			info.Open = s.opened
			info.LastError = errorString(s.err)
		}
		info.Storage = m.storageSummary(id)
		out = append(out, info)
	}
	return out
}

// Info is one row of the profile list.
type Info struct {
	ID        ProfileID    `json:"id"`
	Path      string       `json:"path"`
	UserData  string       `json:"userData"`
	State     ProfileState `json:"state"`
	Open      bool         `json:"open"`
	Storage   StorageInfo  `json:"storage"`
	CreatedAt string       `json:"createdAt"`
	LastError string       `json:"lastError"`
	Layout    string       `json:"layout"`
}

// StorageSummary reads a profile's storage without opening it.
func (m *Manager) storageSummary(id ProfileID) StorageInfo {
	var s StorageInfo
	path := m.layout.UserDataDir(id)
	if !m.applier.HasStorage(path) {
		return s
	}
	s.Persisted = true
	s.Bytes = dirSize(path)
	s.Partial = readMarker(m.applier.MarkerFile(path)) != markerClean &&
		!fileExists(m.applier.LockFile(path))
	hosts, err := cookiestore.Hosts(filepath.Join(path, "Default", "Network", "Cookies"))
	if err != nil {
		s.Cookies = -1
		return s
	}
	s.Cookies = len(hosts)
	return s
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// StorageInfo reports what a closed profile has on disk.
type StorageInfo struct {
	// Persisted is true when the engine's user data folder exists.
	Persisted bool `json:"persisted"`
	// Bytes is the total size of the persisted user data folder.
	Bytes int64 `json:"bytes"`
	// Cookies is the number of host entries in the engine's cookie database, or
	// -1 when it cannot be read.
	Cookies int `json:"cookies"`
	// Partial is true when a previous session did not shut down cleanly.
	Partial bool `json:"partial"`
}

type entrySnapshot struct {
	state  ProfileState
	opened bool
	err    error
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Inspect returns evidence read from a profile's storage.
func (m *Manager) Inspect(accountID string) ([]Evidence, error) {
	id, err := ParseProfileID(accountID)
	if err != nil {
		return nil, err
	}
	if m.inspector == nil {
		return nil, nil
	}
	return m.inspector.Inspect(id)
}

// acquireLocks takes the manager's lock for the profile, and the lock that
// guards the engine's own lock file. The second lock closes the window between
// "the engine's LOCK is absent" and "the engine has created it".
func (m *Manager) acquireLocks(id ProfileID) ([]Lock, error) {
	if m.locker == nil {
		return nil, nil
	}
	own, err := m.locker(filepath.Join(m.layout.ProfilePath(id), "profile.lock"))
	if err != nil {
		return nil, err
	}
	engine, err := m.locker(m.applier.LockFile(m.layout.UserDataDir(id)))
	if err != nil {
		_ = own.Unlock()
		return nil, err
	}
	return []Lock{own, engine}, nil
}

func (m *Manager) probe(id ProfileID) (Probe, error) {
	p := Probe{Path: m.layout.UserDataDir(id)}
	p.Locked = fileExists(m.applier.LockFile(p.Path))
	p.Exited = readMarker(m.applier.MarkerFile(p.Path)) == markerClean
	return p, nil
}

// commit persists a session that shut down cleanly, then drops every temporary
// copy. The engine writes its clean-shutdown marker before this runs, so a crash
// between the commit and the cleanup leaves a profile that Recover can finish.
func (m *Manager) commit(id ProfileID, path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	if err := m.applier.CommitMemory(path); err != nil {
		return fmt.Errorf("browser: cannot commit profile %s: %w", id, err)
	}
	if err := m.applier.DropBackup(path); err != nil {
		return fmt.Errorf("browser: cannot drop backup for %s: %w", id, err)
	}
	if err := os.RemoveAll(m.layout.StagingDir(id)); err != nil {
		return fmt.Errorf("browser: cannot clear staging for %s: %w", id, err)
	}
	return nil
}

// discardStorage removes any staged copy and the engine's user data folder,
// leaving the profile directory itself in place.
func (m *Manager) discardStorage(id ProfileID) error {
	if err := os.RemoveAll(m.layout.StagingDir(id)); err != nil {
		return fmt.Errorf("browser: cannot discard staged data for %s: %w", id, err)
	}
	if err := m.applier.Purge(m.layout.UserDataDir(id)); err != nil {
		return fmt.Errorf("browser: cannot purge profile %s: %w", id, err)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// manifest is the content of profiles.json. It is bookkeeping only: the
// directories on disk are the source of truth, so a lost or corrupt manifest
// never loses a session.
type manifest struct {
	Version  int                 `json:"version"`
	Profiles map[string]manifestEntry `json:"profiles"`
}

type manifestEntry struct {
	CreatedAt string `json:"createdAt"`
}

func (m *Manager) saveManifestLocked() error {
	des, err := os.ReadDir(m.layout.ProfilesRoot)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("browser: cannot read profiles directory: %w", err)
	}
	mf := manifest{Version: 1, Profiles: map[string]manifestEntry{}}
	old := readManifest(m.layout.ManifestPath)
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		if _, err := ParseProfileID(de.Name()); err != nil {
			continue
		}
		created := m.now().UTC().Format(time.RFC3339)
		if prev, ok := old.Profiles[de.Name()]; ok && prev.CreatedAt != "" {
			created = prev.CreatedAt
		}
		mf.Profiles[de.Name()] = manifestEntry{CreatedAt: created}
	}
	body, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("browser: cannot encode manifest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.layout.ManifestPath), 0o700); err != nil {
		return fmt.Errorf("browser: cannot prepare manifest directory: %w", err)
	}
	tmp := m.layout.ManifestPath + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return fmt.Errorf("browser: cannot write manifest: %w", err)
	}
	if err := os.Rename(tmp, m.layout.ManifestPath); err != nil {
		return fmt.Errorf("browser: cannot replace manifest: %w", err)
	}
	return nil
}

func readManifest(path string) manifest {
	var mf manifest
	body, err := os.ReadFile(path)
	if err != nil {
		return manifest{Profiles: map[string]manifestEntry{}}
	}
	if err := json.Unmarshal(body, &mf); err != nil || mf.Profiles == nil {
		return manifest{Profiles: map[string]manifestEntry{}}
	}
	return mf
}

func manifestTime(path string, id ProfileID) string {
	return readManifest(path).Profiles[string(id)].CreatedAt
}
