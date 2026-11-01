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
	"unsafe"

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

	// ParentHWND is the main desktop window handle. When non-zero,
	// sessions are created as embedded WS_CHILD windows inside it.
	ParentHWND uintptr
	// SidebarWidth is the pixel width of the sidebar on the left.
	SidebarWidth int

	// activeID is the profile whose session window is currently visible;
	// every other open session stays alive but hidden behind it.
	activeID browser.ProfileID

	mu       sync.Mutex
	sessions map[*Session]struct{}
}

// SetParentWindow configures the parent window and sidebar offset, and hooks
// the parent's native Win32 WM_SIZE messages for instant real-time resizing.
func (h *Host) SetParentWindow(hwnd uintptr, sidebarWidth int) {
	h.mu.Lock()
	h.ParentHWND = hwnd
	h.SidebarWidth = sidebarWidth
	h.mu.Unlock()

	SubclassParentWindow(hwnd, h)
}

// ResizeChildren immediately resizes every child window to match the parent
// window's client dimensions. Because this runs directly in the parent's native
// Win32 WM_SIZE proc, it updates synchronously on every single frame as the user
// drags the window border.
func (h *Host) ResizeChildren(parentWidth, parentHeight int) {
	h.mu.Lock()
	sidebarWidth := h.SidebarWidth
	h.mu.Unlock()

	w := parentWidth - sidebarWidth
	curH := parentHeight
	if w < 0 {
		w = 0
	}
	if curH < 0 {
		curH = 0
	}

	for s := range h.sessionsSnapshot() {
		if s.hwnd != 0 {
			moveWindow(s.hwnd, sidebarWidth, 0, w, curH)
		}
	}
}

// UpdateBounds resizes and repositions any open session inside/over the parent window.
func (h *Host) UpdateBounds(width, height int) error {
	h.mu.Lock()
	pHwnd := h.ParentHWND
	h.mu.Unlock()

	if pHwnd != 0 {
		var clientRect struct {
			Left, Top, Right, Bottom int32
		}
		procGetClientRect.Call(pHwnd, uintptr(unsafe.Pointer(&clientRect)))
		h.ResizeChildren(int(clientRect.Right), int(clientRect.Bottom))
	} else {
		h.ResizeChildren(width, height)
	}
	return nil
}

// SetVisible shows or hides open sessions (used during modal dialogs).
func (h *Host) SetVisible(visible bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.sessions {
		_ = s.call(func() error {
			if s.hwnd != 0 {
				if visible {
					_ = showWindow(s.hwnd)
				} else {
					hideWindow(s.hwnd)
				}
			}
			return nil
		})
	}
	return nil
}

// SetActiveSession makes the session for id visible and hides every other open
// session, so the app can switch accounts by swapping visibility instead of
// tearing engines down.
func (h *Host) SetActiveSession(id browser.ProfileID) {
	h.mu.Lock()
	h.activeID = id
	sessions := make([]*Session, 0, len(h.sessions))
	for s := range h.sessions {
		sessions = append(sessions, s)
	}
	h.mu.Unlock()

	var active *Session
	for _, s := range sessions {
		if s.profile == id {
			active = s
		}
	}
	for _, s := range sessions {
		if s == active {
			_ = s.Show()
		} else {
			_ = s.Hide()
		}
	}
}

// ActiveSessionID returns the profile id currently shown; empty when none.
func (h *Host) ActiveSessionID() browser.ProfileID {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activeID
}

// sessionsSnapshot returns a copy of the open sessions so the caller can
// mutate visibility without holding the host lock.
func (h *Host) sessionsSnapshot() map[*Session]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[*Session]struct{}, len(h.sessions))
	for s := range h.sessions {
		out[s] = struct{}{}
	}
	return out
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
	// Remember which session is visible; the run loop opens this window
	// hidden when another one is already active, so switching accounts only
	// swaps visibility instead of destroying engines.
	startHidden := h.ParentHWND != 0 && h.activeID != "" && h.activeID != id
	h.mu.Unlock()
	s.startHidden = startHidden

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
	// startHidden is set by OpenProfile: the session's window is created
	// hidden because another session is currently active.
	startHidden bool
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

// Show brings the window to the front and focuses it. Synchronous: the switch
// back must complete before the UI drops its "switching" state.
func (s *Session) Show() error {
	if !s.Alive() {
		return browser.ErrSessionEnded
	}
	return s.call(func() error {
		if err := showWindow(s.hwnd); err != nil {
			return err
		}
		if err := setForeground(s.hwnd); err != nil {
			log.Printf("webview2: cannot focus window for %s: %v", s.profile, err)
		}
		if s.chromium != nil {
			s.chromium.Focus()
		}
		return nil
	})
}

// Hide keeps the engine running but makes the session window invisible, so
// another session can take the embedded region without this one losing state.
func (s *Session) Hide() error {
	if !s.Alive() {
		return browser.ErrSessionEnded
	}
	return s.call(func() error {
		hideWindow(s.hwnd)
		return nil
	})
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
		log.Printf("webview2: closing session %s hwnd=%d", s.profile, s.hwnd)
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
		log.Printf("webview2: setUpWebView failed for %s: %v", s.profile, err)
		s.requests <- openRequest{err: err}
		s.emit(EventStartFailed, err)
		destroyWindow(hwnd)
		close(s.done)
		return
	}

	_ = setWindowTitle(hwnd, fmt.Sprintf("%s - %s", s.host.Title, s.profile))
	if s.startHidden {
		hideWindow(hwnd)
		log.Printf("webview2: opening %s hidden (another session is active)", s.profile)
	} else {
		_ = showWindow(hwnd)
	}
	if s.chromium != nil {
		_ = s.chromium.Show()
		s.chromium.Resize()
		s.chromium.Focus()
	}

	s.setAlive(true)
	s.requests <- openRequest{}
	s.emit(EventReady, nil)

	// If this session has a parent Wails window, attach it now as a real Win32 child (WS_CHILD).
	// Because SetParent is called AFTER WebView2's controller has already initialised its COM
	// and HWND surface, WebView2 works completely, and Windows automatically moves & clips it
	// synchronously with the parent window whenever the user drags or resizes the main window!
	if s.host.ParentHWND != 0 {
		// Change style from WS_POPUP to WS_CHILD (GWL_STYLE = -16 = 0xFFFFFFF0)
		var gwlStyle uintptr = 0xFFFFFFF0
		currentStyle, _, _ := procGetWindowLongPtr.Call(hwnd, gwlStyle)
		newStyle := (currentStyle &^ wsPopup) | wsChild
		procSetWindowLongPtr.Call(hwnd, gwlStyle, newStyle)

		// Set Wails window as native parent
		procSetParent.Call(hwnd, s.host.ParentHWND)

		// Reposition child in client coordinates of parent (x = 68, y = 0)
		var clientRect struct {
			Left, Top, Right, Bottom int32
		}
		procGetClientRect.Call(s.host.ParentHWND, uintptr(unsafe.Pointer(&clientRect)))
		w := int(clientRect.Right) - s.host.SidebarWidth
		h := int(clientRect.Bottom)
		if w < 0 {
			w = 0
		}
		if h < 0 {
			h = 0
		}
		moveWindow(hwnd, s.host.SidebarWidth, 0, w, h)
		if s.chromium != nil {
			s.chromium.Resize()
		}
		log.Printf("webview2: successfully re-parented %s (hwnd=%d) as WS_CHILD of %d at pos=(%d,0) size=%dx%d",
			s.profile, hwnd, s.host.ParentHWND, s.host.SidebarWidth, w, h)
	}

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
