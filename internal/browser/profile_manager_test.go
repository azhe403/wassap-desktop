package browser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeLock is an in-memory LockFunc for tests: no OS file locks involved.
type fakeLock struct {
	held map[string]bool
}

func newFakeLock() (map[string]bool, LockFunc) {
	held := make(map[string]bool)
	return held, func(path string) (Lock, error) {
		if held[path] {
			return nil, ErrProfileBusy
		}
		held[path] = true
		return &fakeLockHandle{held: held, path: path}, nil
	}
}

type fakeLockHandle struct {
	held map[string]bool
	path string
}

func (l *fakeLockHandle) TryLock(string) error { return nil }
func (l *fakeLockHandle) Unlock() error {
	delete(l.held, l.path)
	return nil
}

// fakeApplier implements Applier purely in memory so manager tests do not
// touch a real Chromium layout.
type fakeApplier struct {
	mu        chan struct{}
	has       map[string]bool
	hasMem    map[string]bool
	hasBackup map[string]bool
	locked    map[string]bool
	commitErr error
}

func newFakeApplier() *fakeApplier {
	return &fakeApplier{
		mu:        make(chan struct{}, 1),
		has:       make(map[string]bool),
		hasMem:    make(map[string]bool),
		hasBackup: make(map[string]bool),
		locked:    make(map[string]bool),
	}
}

func (f *fakeApplier) begin() { f.mu <- struct{}{} }
func (f *fakeApplier) end()   { <-f.mu }

func (f *fakeApplier) LoadMemory(path string) error {
	f.begin()
	defer f.end()
	f.hasMem[path] = f.has[path]
	f.hasBackup[path] = f.has[path]
	f.has[path] = false
	return nil
}

func (f *fakeApplier) CommitMemory(path string) error {
	f.begin()
	defer f.end()
	if f.commitErr != nil {
		return f.commitErr
	}
	f.has[path] = true
	return nil
}

func (f *fakeApplier) RestoreBackup(path string) error {
	f.begin()
	defer f.end()
	f.has[path] = f.hasBackup[path]
	f.hasBackup[path] = false
	f.hasMem[path] = false
	return nil
}

func (f *fakeApplier) DropBackup(path string) error {
	f.begin()
	defer f.end()
	f.hasBackup[path] = false
	return nil
}

func (f *fakeApplier) HasStorage(path string) bool {
	f.begin()
	defer f.end()
	return f.has[path]
}

func (f *fakeApplier) LockFile(path string) string { return path + "/LOCK" }
func (f *fakeApplier) MarkerFile(path string) string {
	return filepath.Join(filepath.Dir(path), MarkerName)
}
func (f *fakeApplier) Purge(path string) error {
	f.begin()
	defer f.end()
	f.has[path] = false
	return nil
}

// fakeOpener records the Probe it was handed and reports clean exits.
type fakeOpener struct {
	probe   Probe
	calls   int
	fails   bool
}

func (o *fakeOpener) OpenProfile(id ProfileID, p Probe) (Session, error) {
	o.calls++
	o.probe = p
	if o.fails {
		return nil, ErrSessionEnded
	}
	return &fakeSession{id: id, p: p}, nil
}

type fakeSession struct {
	id  ProfileID
	p   Probe
	clo int
}

func (s *fakeSession) Profile() ProfileID            { return s.id }
func (s *fakeSession) Path() string                  { return s.p.Path }
func (s *fakeSession) Outcome() Outcome               { return OpenedClean }
func (s *fakeSession) Show() error                    { return nil }
func (s *fakeSession) Focus() error                   { return nil }
func (s *fakeSession) Alive() bool                    { return s.clo == 0 }
func (s *fakeSession) Close() error { s.clo++; return nil }

// newTestManager wires a Manager to the fakes, with a t.TempDir layout.
func newTestManager(t *testing.T) (*Manager, *fakeApplier, *fakeOpener, map[string]bool) {
	t.Helper()
	layout := NewLayout(t.TempDir())
	ap := newFakeApplier()
	op := &fakeOpener{}
	held, locker := newFakeLock()
	m := NewManager(ManagerDeps{
		Layout:  layout,
		Applier: ap,
		Opener:  op,
		Lock:    locker,
		Now:     func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
	return m, ap, op, held
}

func TestParseProfileID_Valid(t *testing.T) {
	for _, in := range []string{"account-1", "a", "abc-xyz", "user-1"} {
		if _, err := ParseProfileID(in); err != nil {
			t.Errorf("ParseProfileID(%q) error = %v", in, err)
		}
	}
}

func TestParseProfileID_Invalid(t *testing.T) {
	for _, in := range []string{
		"",       // empty
		"AB",     // uppercase
		"a/b",    // path separator
		"..",     // parent reference
		"a b",    // space
		"a\\b",   // windows separator
		"nul",    // reserved
		"CON",    // reserved upper
		"a\tb",   // control char
	} {
		if _, err := ParseProfileID(in); err == nil {
			t.Errorf("ParseProfileID(%q) = nil error, want error", in)
		}
	}
}

func TestNewProfileID_Valid(t *testing.T) {
	id, err := NewProfileID()
	if err != nil {
		t.Fatalf("NewProfileID() error = %v", err)
	}
	if _, err := ParseProfileID(string(id)); err != nil {
		t.Errorf("NewProfileID() = %q is not valid: %v", id, err)
	}
}

func TestManager_CreateOpenCloseDelete(t *testing.T) {
	m, _, _, _ := newTestManager(t)

	// Create
	if err := m.Create("account-1"); err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if err := m.Create("account-1"); !errors.Is(err, ErrProfileExists) {
		t.Errorf("Create duplicate = %v, want ErrProfileExists", err)
	}

	// Open
	if err := m.Open("account-1"); err != nil {
		t.Fatalf("Open error = %v", err)
	}

	// Open again while open must be refused
	if err := m.Open("account-1"); !errors.Is(err, ErrProfileOpen) {
		t.Errorf("second Open = %v, want ErrProfileOpen", err)
	}

	// Close
	if err := m.Close("account-1"); err != nil {
		t.Fatalf("Close error = %v", err)
	}
	if err := m.Close("account-1"); !errors.Is(err, ErrSessionEnded) {
		t.Errorf("second Close = %v, want ErrSessionEnded", err)
	}

	// Open after Close must work again
	if err := m.Open("account-1"); err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	if err := m.Close("account-1"); err != nil {
		t.Fatalf("close again error = %v", err)
	}

	// Delete
	if err := m.Delete("account-1"); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if err := m.Delete("account-1"); !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("delete missing = %v, want ErrProfileNotFound", err)
	}
}

func TestManager_DeleteWhileOpen_Refused(t *testing.T) {
	m, _, _, _ := newTestManager(t)
	if err := m.Create("account-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Open("account-1"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Delete("account-1"); !errors.Is(err, ErrProfileOpen) {
		t.Errorf("Delete while open = %v, want ErrProfileOpen", err)
	}
}

func TestManager_LockedProfileRefused(t *testing.T) {
	m, _, _, held := newTestManager(t)
	if err := m.Create("account-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Mark the profile's lock file as held by another process.
	held[filepath.Join(m.Layout().ProfilePath("account-1"), "profile.lock")] = true
	// The manager must refuse to open.
	err := m.Open("account-1")
	if !errors.Is(err, ErrProfileBusy) {
		t.Errorf("Open on locked profile = %v, want ErrProfileBusy", err)
	}
}

func TestManager_InvalidIDRefused(t *testing.T) {
	m, _, _, _ := newTestManager(t)
	for _, id := range []string{"", "A/B", "..", "nul", "a b"} {
		if err := m.Create(id); !errors.Is(err, ErrInvalidProfileID) {
			t.Errorf("Create(%q) = %v, want ErrInvalidProfileID", id, err)
		}
		if err := m.Open(id); !errors.Is(err, ErrInvalidProfileID) {
			t.Errorf("Open(%q) = %v, want ErrInvalidProfileID", id, err)
		}
	}
}

func TestManager_StorageSummary(t *testing.T) {
	m, ap, _, _ := newTestManager(t)
	_ = m.Create("account-1")
	_ = m.Create("account-2")

	// Simulate a commit on account-1 only.
	ap.has[m.Layout().UserDataDir("account-1")] = true
	ap.hasBackup[m.Layout().UserDataDir("account-1")] = false
	ap.hasMem[m.Layout().UserDataDir("account-1")] = false
	_ = writeMarker(ap.MarkerFile(m.Layout().UserDataDir("account-1")), markerClean)

	infos := m.List()
	if len(infos) != 2 {
		t.Fatalf("List() = %d profiles, want 2: %+v", len(infos), infos)
	}
	byID := map[ProfileID]Info{}
	for _, i := range infos {
		byID[i.ID] = i
	}
	if !byID["account-1"].Storage.Persisted {
		t.Errorf("account-1.Storage.Persisted = false, want true")
	}
	if byID["account-2"].Storage.Persisted {
		t.Errorf("account-2.Storage.Persisted = true, want false")
	}
	if byID["account-1"].Storage.Partial {
		t.Errorf("account-1.Storage.Partial = true, want false (clean marker set)")
	}
}

func TestManager_LegacyWrapsInterface(t *testing.T) {
	var _ ManagerInterface = (*Manager)(nil)
	_ = os.MkdirAll
}
