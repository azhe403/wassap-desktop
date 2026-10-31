package browser

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ErrInvalidProfileID is returned for account ids that are not safe to use as
// part of a filesystem path.
var ErrInvalidProfileID = errors.New("browser: invalid profile id")

// MaxProfileIDLength is the longest accepted account id. It keeps the whole
// profile path well inside MAX_PATH for the default layout.
const MaxProfileIDLength = 64

// profileIDPattern restricts account ids to lowercase letters, digits and
// hyphens. Anything else would either be a path separator, a Windows-reserved
// character, a way to express "..", or a case-folding collision.
var profileIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// windowsReservedNames cannot be used as a directory name on Windows, with or
// without an extension.
var windowsReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ParseProfileID validates raw and returns it as a ProfileID.
//
// A valid id is non-empty, at most MaxProfileIDLength characters, consists of
// lowercase letters, digits and single hyphens, and is not a Windows reserved
// device name. Validation happens before any filesystem call, so an id can
// never escape the profiles directory.
func ParseProfileID(raw string) (ProfileID, error) {
	if raw == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidProfileID)
	}
	if len(raw) > MaxProfileIDLength {
		return "", fmt.Errorf("%w: %q is longer than %d characters", ErrInvalidProfileID, raw, MaxProfileIDLength)
	}
	if raw != strings.ToLower(raw) {
		return "", fmt.Errorf("%w: %q must be lowercase", ErrInvalidProfileID, raw)
	}
	if !profileIDPattern.MatchString(raw) {
		return "", fmt.Errorf("%w: %q may only contain a-z, 0-9 and single hyphens", ErrInvalidProfileID, raw)
	}
	if windowsReservedNames[strings.SplitN(raw, "-", 2)[0]] {
		return "", fmt.Errorf("%w: %q is a reserved device name", ErrInvalidProfileID, raw)
	}
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("%w: %q contains whitespace or control characters", ErrInvalidProfileID, raw)
		}
	}
	return ProfileID(raw), nil
}

// NewProfileID returns a random, valid account id. It is used by the proof of
// concept; a real account layer would keep its own stable ids and call
// ParseProfileID on them.
func NewProfileID() (ProfileID, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("browser: cannot generate profile id: %w", err)
	}
	return ProfileID("account-" + hex.EncodeToString(buf[:])), nil
}

// userDataDirName is the directory the browser engine creates inside a profile
// directory. WebView2 always uses this name for its user data folder contents.
const userDataDirName = "EBWebView"

// ProfilePath returns the profile's own directory, which contains both the
// engine's user data folder and the manager's own bookkeeping files.
func (d Layout) ProfilePath(id ProfileID) string {
	return filepath.Join(d.ProfilesRoot, string(id))
}

// UserDataDir returns the user data folder handed to the browser engine for a
// profile.
func (d Layout) UserDataDir(id ProfileID) string {
	return filepath.Join(d.ProfilePath(id), userDataDirName)
}

// Layout is the on-disk map of the profile subsystem.
//
//	<ProfilesRoot>/<id>/                 profile directory, created by the manager
//	<ProfilesRoot>/<id>/EBWebView/       WebView2 user data folder
//	<ProfilesRoot>/<id>/EBWebView/LOCK   WebView2's own exclusive lock
//	<profiles.json>                      profiles known to this installation
//
// The temporary locations exist because WebView2 can neither be moved nor
// copied safely while it is running: a profile is opened by moving its storage
// directory into a temporary location and the browser is pointed there.
type Layout struct {
	// ProfilesRoot is the directory holding one directory per profile.
	ProfilesRoot string
	// ManifestPath is the JSON file listing every profile.
	ManifestPath string
	// StagingRoot holds on-memory copies of profiles that are open, plus the
	// previous on-disk state as a backup.
	StagingRoot string
	// TrashRoot holds profiles that are being deleted.
	TrashRoot string
}

// NewLayout derives the layout from the application data directory:
//
//	<data>/profiles/         profile directories
//	<data>/profiles.json     manifest
//	<data>/runtime/staging/  memory and backup copies of open profiles
//	<data>/runtime/trash/    profiles being deleted
func NewLayout(dataRoot string) Layout {
	runtimeRoot := filepath.Join(dataRoot, "runtime")
	return Layout{
		ProfilesRoot: filepath.Join(dataRoot, "profiles"),
		ManifestPath: filepath.Join(dataRoot, "profiles.json"),
		StagingRoot:  filepath.Join(runtimeRoot, "staging"),
		TrashRoot:    filepath.Join(runtimeRoot, "trash"),
	}
}

// MemoryDir is the user data folder an open profile's browser is pointed at.
func (d Layout) MemoryDir(id ProfileID) string {
	return filepath.Join(d.StagingRoot, string(id), "memory", userDataDirName)
}

// BackupDir is where the persisted user data folder is parked while a profile
// is open.
func (d Layout) BackupDir(id ProfileID) string {
	return filepath.Join(d.StagingRoot, string(id), "backup", userDataDirName)
}

// StagingDir is the per-profile temporary directory.
func (d Layout) StagingDir(id ProfileID) string {
	return filepath.Join(d.StagingRoot, string(id))
}

// TrashDir is where a profile is moved while it is being deleted.
func (d Layout) TrashDir(id ProfileID) string {
	return filepath.Join(d.TrashRoot, string(id))
}
