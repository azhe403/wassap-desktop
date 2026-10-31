//go:build windows

// Package oslock provides the Windows cross-process lock used to guarantee that
// a browser profile is in use by exactly one window, in exactly one process.
//
// The lock is a Win32 LockFileEx exclusive lock held on the file itself. An OS
// lock is released when the process dies, so a crash never leaves a profile
// permanently blocked, unlike a "locked" marker file.
package oslock

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"

	"wassap-desktop/internal/browser"
)

// File is one held lock.
type File struct {
	file *os.File
}

// Lock opens path, creating it when needed, and takes an exclusive lock.
//
// It returns an error wrapping browser.ErrProfileBusy when another open handle
// already holds the lock.
func Lock(path string) (browser.Lock, error) {
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		return nil, fmt.Errorf("oslock: cannot prepare %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("oslock: cannot open %s: %w", path, err)
	}
	ol := windows.Overlapped{}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&ol,
	)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, fmt.Errorf("%w: %s", browser.ErrProfileBusy, path)
		}
		return nil, fmt.Errorf("oslock: cannot lock %s: %w", path, err)
	}
	return &File{file: f}, nil
}

// TryLock implements browser.Lock: Lock already takes the lock, so the
// behaviour is the same.
func (l *File) TryLock(string) error { return nil }

// Unlock releases the lock and closes the handle. Closing alone would also
// release it, but the explicit unlock makes the intent obvious and fails loudly
// if the handle was already lost.
func (l *File) Unlock() error {
	ol := windows.Overlapped{}
	err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &ol)
	closeErr := l.file.Close()
	if err != nil {
		return fmt.Errorf("oslock: cannot unlock %s: %w", l.file.Name(), err)
	}
	return closeErr
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
