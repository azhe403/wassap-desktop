//go:build windows

// Package webview2 implements the Windows browser host used by Wassap Desktop.
//
// It is the only package that talks to WebView2 and Win32. Everything above it
// works against the interfaces in internal/browser.
//
// # Why this package exists instead of a second wails.Run
//
// Wails v2.17.0 creates exactly one WebView2 environment per process, from a
// single edge.Chromium instance, and reads options.App.Windows.WebviewUserDataPath
// once while creating it (internal/frontend/desktop/windows/frontend.go,
// setupChromium). There is no API to re-point that environment, to create a
// second one, or to select a named profile: webviewloader exposes
// WithAdditionalBrowserArguments and WithExclusiveUserDataFolderAccess, but
// edge.Chromium neither passes them nor re-exports them.
//
// This package therefore drives wailsapp/go-webview2 - the same binding Wails
// itself uses, at the same version - directly. It is not a fork: go-webview2 is
// imported as an ordinary module.
//
// # Why the host owns its own window
//
// The WebView2 environment callback is delivered by the thread's Win32 message
// loop, and go-webview2's Embed spins that loop until the environment is ready.
// Wails runs its message loop on one goroutine (winc.RunMainLoop, which does not
// LockOSThread) and dispatches its own UI calls across goroutines. Creating an
// environment there would couple its lifetime to Wails' window. Instead every
// session gets a dedicated OS thread with its own window, message loop and
// WebView2 environment, and the Wails window stays a plain control panel.
package webview2

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"

	"wassap-desktop/internal/browser"
)

// ErrNoRuntime is returned by CheckRuntime when WebView2 is not installed.
var ErrNoRuntime = errors.New("webview2: the WebView2 runtime is not installed")

// CheckRuntime reports whether the WebView2 runtime is available and which
// version. It starts no browser, so it is safe to call at any time.
func CheckRuntime() (string, error) {
	version, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoRuntime, err)
	}
	if version == "" {
		return "", ErrNoRuntime
	}
	return version, nil
}

// ErrProfileHasLiveBrowser is returned when the engine's own lock file shows a
// browser already running on the user data folder.
var ErrProfileHasLiveBrowser = errors.New("webview2: user data folder already in use")

// Host starts browser sessions. One Host serves every profile in the process;
// sessions are independent and may be open at the same time.
type Host struct {
	// Title is the window caption prefix. The account id is appended.
	Title string
	// StartURL is the page every session navigates to.
	StartURL string
	// Bounds are the initial outer window bounds in pixels.
	Bounds Rect
	// CloseTimeout bounds a single session shutdown.
	CloseTimeout time.Duration
	// CloseGrace is how long a session waits for the engine to flush its
	// profile before the controller is closed anyway.
	CloseGrace time.Duration

	mu       sync.Mutex
	sessions map[*Session]struct{}
}

// NewHost returns a Host with defaults for the proof of concept.
func NewHost(title, startURL string) *Host {
	return &Host{
		Title:        title,
		StartURL:     startURL,
		Bounds:       Rect{X: 120, Y: 80, Width: 1180, Height: 820},
		CloseTimeout: 30 * time.Second,
		CloseGrace:   10 * time.Second,
		sessions:     make(map[*Session]struct{}),
	}
}

// OpenProfile implements browser.Opener. It blocks until the WebView2
// environment exists and the start URL has been requested, or until start-up
// fails, and reports start-up failures as errors instead of exiting the process.
func (h *Host) OpenProfile(id browser.ProfileID, probe browser.Probe) (browser.Session, error) {
	if probe.Path == "" {
		return nil, errors.New("webview2: profile has no user data folder")
	}
	// WebView2 aborts start-up, with a message box, when the user data folder
	// does not exist. Wails documents the same requirement.
	if err := os.MkdirAll(probe.Path, 0o700); err != nil {
		return nil, fmt.Errorf("webview2: cannot create user data folder %s: %w", probe.Path, err)
	}

	s := &Session{
		host:     h,
		profile:  id,
		path:     probe.Path,
		outcome:  browser.OpenedClean,
		requests: make(chan openRequest, 1),
		events:   make(chan SessionEvent, 32),
		done:     make(chan struct{}),
	}
	if probe.Exited && storagePresent(probe.Path) {
		s.outcome = browser.OpenedMemory
	}

	h.mu.Lock()
	h.sessions[s] = struct{}{}
	h.mu.Unlock()

	go s.run()

	select {
	case req := <-s.requests:
		if req.err != nil {
			return nil, req.err
		}
		return s, nil
	case <-time.After(h.timeout()):
		_ = s.Close()
		return nil, fmt.Errorf("webview2: timed out starting a browser for %s", id)
	}
}

// CloseAll closes every session.
func (h *Host) CloseAll() error {
	h.mu.Lock()
	all := make([]*Session, 0, len(h.sessions))
	for s := range h.sessions {
		all = append(all, s)
	}
	h.mu.Unlock()

	var firstErr error
	for _, s := range all {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *Host) timeout() time.Duration {
	if h.CloseTimeout > 0 {
		return h.CloseTimeout
	}
	return 30 * time.Second
}

func (h *Host) grace() time.Duration {
	if h.CloseTimeout > 0 {
		return h.CloseGrace
	}
	return 10 * time.Second
}

func (h *Host) forget(s *Session) {
	h.mu.Lock()
	delete(h.sessions, s)
	h.mu.Unlock()
}

// SessionEventKind classifies a lifecycle event.
type SessionEventKind int

const (
	// EventReady means the environment was created and the start URL is loading.
	EventReady SessionEventKind = iota
	// EventStartFailed means start-up failed.
	EventStartFailed
	// EventClosed means the window is gone, so the profile is free again.
	EventClosed
	// EventProcessFailed means a WebView2 process failed; see Err for severity.
	EventProcessFailed
)

// SessionEvent is a lifecycle event.
type SessionEvent struct {
	Session browser.ProfileID
	Kind    SessionEventKind
	Err     error
}

type openRequest struct {
	err error
}

type callback struct {
	fn     func() error
	result chan error
}

// Session is one window with one WebView2 environment and one user data
// folder. All Win32 and COM work happens on the session's own OS thread.
type Session struct {
	host     *Host
	profile  browser.ProfileID
	path     string
	outcome  browser.Outcome
	requests chan openRequest
	events   chan SessionEvent
	done     chan struct{}

	// OnEvent is called from arbitrary threads for lifecycle events and must
	// not block.
	OnEvent func(SessionEvent)

	hwnd     uintptr
	instance uintptr
	className *uint16
	threadID  uint32
	chromium *edge.Chromium
	runtimeVersion string

	callbacks atomic.Pointer[callback]

	closeOnce sync.Once
	closed    chan struct{}
	closeErr  error
	closeMu   sync.Mutex

	aliveMu sync.Mutex
	alive   bool
}

// Profile implements browser.Session.
func (s *Session) Profile() browser.ProfileID { return s.profile }

// Path implements browser.Session.
func (s *Session) Path() string { return s.path }

// Outcome implements browser.Session.
func (s *Session) Outcome() browser.Outcome { return s.outcome }

// RuntimeVersion is the WebView2 runtime the session started with.
func (s *Session) RuntimeVersion() string { return s.runtimeVersion }

// Alive implements browser.Session.
func (s *Session) Alive() bool {
	s.aliveMu.Lock()
	defer s.aliveMu.Unlock()
	return s.alive
}

func (s *Session) setAlive(v bool) {
	s.aliveMu.Lock()
	s.alive = v
	s.aliveMu.Unlock()
}

// Events exposes the session's lifecycle events.
func (s *Session) Events() <-chan SessionEvent { return s.events }

func (s *Session) emit(kind SessionEventKind, err error) {
	ev := SessionEvent{Session: s.profile, Kind: kind, Err: err}
	if s.OnEvent != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("webview2: session event hook panicked: %v", r)
				}
			}()
			s.OnEvent(ev)
		}()
	}
	select {
	case s.events <- ev:
	default:
	}
}

// Show brings the window to the front.
func (s *Session) Show() error {
	if !s.Alive() {
		return browser.ErrSessionEnded
	}
	go func() {
		if err := s.call(func() error { return showWindow(s.hwnd) }); err != nil {
			log.Printf("webview2: cannot show window for %s: %v", s.profile, err)
		}
	}()
	return nil
}

// Focus brings the window to the foreground and moves keyboard focus into the
// web view.
func (s *Session) Focus() error {
	if !s.Alive() {
		return browser.ErrSessionEnded
	}
	return s.call(func() error {
		if err := setForeground(s.hwnd); err != nil {
			return err
		}
		if s.chromium != nil {
			s.chromium.Focus()
		}
		return nil
	})
}

// Close destroys the window and blocks until the session thread has released the
// WebView2 controller and environment, or until the close timeout elapses.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.closed = make(chan struct{})
		go func() {
			defer close(s.closed)
			select {
			case <-s.done:
			case <-time.After(s.host.timeout()):
				s.setCloseErr(fmt.Errorf("webview2: %s did not shut down within %s", s.profile, s.host.timeout()))
				// Last resort: destroy the window anyway so the process does not
				// leak it, and let the manager recover the profile on next open.
				_ = s.call(func() error { destroyWindow(s.hwnd); return nil })
			}
		}()
		if err := s.call(func() error { destroyWindow(s.hwnd); return nil }); err != nil {
			s.setCloseErr(err)
		}
		<-s.closed
		s.host.forget(s)
	})
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.closeErr
}

func (s *Session) setCloseErr(err error) {
	s.closeMu.Lock()
	if s.closeErr == nil {
		s.closeErr = err
	}
	s.closeMu.Unlock()
}

// run owns the OS thread for the session's whole life.
func (s *Session) run() {
	lockOSThread()
	defer unlockOSThread()

	if err := initCOM(); err != nil {
		s.requests <- openRequest{err: fmt.Errorf("webview2: cannot initialize COM: %w", err)}
		close(s.done)
		return
	}
	defer uninitCOM()

	hwnd, err := s.createWindow()
	if err != nil {
		s.requests <- openRequest{err: err}
		close(s.done)
		return
	}
	s.hwnd = hwnd

	if err := s.setUpWebView(); err != nil {
		s.requests <- openRequest{err: err}
		s.emit(EventStartFailed, err)
		destroyWindow(hwnd)
		close(s.done)
		return
	}

	_ = setWindowTitle(hwnd, fmt.Sprintf("%s - %s", s.host.Title, s.profile))
	_ = showWindow(hwnd)
	if s.chromium != nil {
		_ = s.chromium.Show()
		s.chromium.Resize()
		s.chromium.Focus()
	}

	s.setAlive(true)
	s.requests <- openRequest{}
	s.emit(EventReady, nil)

	s.messageLoop()
}

// messageLoop dispatches messages until the window is destroyed or WM_QUIT
// arrives, then waits for the environment to shut down.
func (s *Session) messageLoop() {
	for {
		m, ok := getMessage()
		if !ok {
			break
		}
		translateMessage(m)
		dispatchMessage(m)
	}
	s.teardown()
}

// teardown releases the controller and then the environment, in that order, as
// go-webview2 documents: the controller must go first, and the environment's
// Close then flushes the profile and ends the browser process.
func (s *Session) teardown() {
	s.setAlive(false)
	if s.chromium != nil {
		_ = CloseController(s.chromium.GetController())
		_ = CloseEnvironment(s.chromium.Environment())
	}
	s.destroyClassName()
	close(s.done)
	s.host.forget(s)
	s.emit(EventClosed, nil)
}

// call runs f on the session's UI thread and returns its error.
func (s *Session) call(f func() error) error {
	if s.hwnd == 0 {
		return browser.ErrSessionEnded
	}
	if s.threadID != 0 && s.threadID == currentThreadID() {
		return f()
	}
	result := make(chan error, 1)
	s.callbacks.Store(&callback{fn: f, result: result})
	if !postMessage(s.hwnd, wmInvokeCallback, 0, 0) {
		s.callbacks.CompareAndSwap(s.callbacks.Load(), nil)
		return errors.New("webview2: cannot post to the session window")
	}
	return <-result
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// storagePresent reports whether the engine's user data folder already holds
// profile storage, which is how an in-memory copy is distinguished from a fresh
// profile.
func storagePresent(path string) bool {
	info, err := os.Stat(filepath.Join(path, "Default"))
	return err == nil && info.IsDir()
}
