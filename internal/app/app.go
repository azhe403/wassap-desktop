package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/avatar"
	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/config"
	"wassap-desktop/internal/platform/procstats"
	"wassap-desktop/internal/privacyblur"
	"wassap-desktop/internal/storage"
	"wassap-desktop/internal/updater"

	"github.com/wailsapp/wails/v2/pkg/options"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// whatsappURL is the official WhatsApp Web address.
const whatsappURL = "https://web.whatsapp.com"

// HostController defines the opener and session host interface.
type HostController interface {
	browser.Opener
	CloseAll() error
	UpdateBounds(width, height int) error
	SetVisible(visible bool) error
	SetActiveSession(id browser.ProfileID)
	EvalAll(script string)
	SetPrivacyDelegate(prelude func() string, onWrite func(map[string]any))
	SetAvatarDelegate(onUpdate func(browser.ProfileID, string))
	SetSmartRAMSaver(enabled bool)
}

// App is the desktop shell application and Wails binding target.
type App struct {
	ctx        context.Context
	dirs       appdirs.Dirs
	legacyMode bool
	build      VersionInfo

	manager     *browser.Manager
	host        HostController
	inspector   browser.Inspector
	accountSvc  account.Service
	configStore config.Store
	avatarStore *avatar.Store

	mu             sync.Mutex
	currentProfile string
	activeAccount  string
	switching      bool
	closeToTray    bool
	smartRAMSaver  bool
	forceQuit      bool
	quitting       bool
	windowState    *config.WindowState

	geomPending *config.WindowState
	geomTimer   *time.Timer
	geomStop    chan struct{}
	geomDone    chan struct{}

	statsMu        sync.Mutex
	statsLastTicks map[uint32]uint64
	statsLastAt    time.Time

	tray trayBackend

	// broadcastMu serializes the privacy blur broadcast (save → read-latest →
	// EvalAll → event) across the two write paths (settings save and in-page
	// Alt+/), so no session ever observes a payload older than the store.
	broadcastMu sync.Mutex

	checker    *updater.Checker
	downloader *updater.Downloader
	updateMu   sync.Mutex
	dlState    dlDownloadState
	lastStatus updater.Status
}

// trayBackend is the platform tray implementation. On Windows it is a real
// system-tray controller (internal/platform/windows/tray); on other platforms
// it is a no-op. The tray owns a hidden top-level window.
type trayBackend interface {
	// Setup creates the tray icon on its dedicated hidden top-level window.
	Setup(hwnd uintptr)
	// Remove destroys the tray icon and its owner window.
	Remove()
	// Ready reports whether the tray icon is registered with the shell.
	Ready() bool
}

// New creates a new App application struct. build carries the build-time
// stamp injected by the composition root (ldflags -X main.version=...).
func New(dirs appdirs.Dirs, legacyMode bool, build VersionInfo) *App {
	manager, host, insp := initPlatform(dirs)

	repoPath := filepath.Join(dirs.Data, "accounts.json")
	repo, repoErr := storage.NewJSONAccountRepository(repoPath)
	var accountSvc account.Service
	if repoErr != nil {
		// A corrupt or unreadable repository disables account bindings instead of
		// letting them panic. The file on disk is preserved for inspection.
		fmt.Fprintf(os.Stderr, "storage: failed to initialize accounts repository: %v\n", repoErr)
	} else {
		accountSvc = account.NewService(repo, manager)
	}

	cfgPath := filepath.Join(dirs.Config, "config.json")
	cfgStore := config.NewStore(cfgPath)
	closeToTray, err := cfgStore.GetCloseToTray()
	if err != nil {
		log.Printf("app: cannot load close-to-tray preference, defaulting to off: %v", err)
	}
	smartRAMSaver, err := cfgStore.GetSmartRAMSaver()
	if err != nil {
		log.Printf("app: cannot load smart RAM saver preference, defaulting to on: %v", err)
		smartRAMSaver = true
	}

	app := &App{
		dirs:          dirs,
		legacyMode:    legacyMode,
		build:         build,
		manager:       manager,
		host:          host,
		inspector:     insp,
		accountSvc:    accountSvc,
		configStore:   cfgStore,
		avatarStore:   avatar.NewStore(dirs.Data),
		closeToTray:   closeToTray,
		smartRAMSaver: smartRAMSaver,
	}
	windowState, wsErr := cfgStore.GetWindowState()
	if wsErr != nil {
		log.Printf("app: cannot load window state, starting with defaults: %v", wsErr)
	} else {
		app.windowState = windowState
	}
	app.tray = newTrayBackend(app)
	app.wirePlatformNotifications()

	// Privacy blur: install the delegate before any session can be opened.
	// The prelude closure re-reads config.json at every call (including each
	// navigation re-apply) — a tiny file, well within navigation budget.
	if app.host != nil {
		app.host.SetPrivacyDelegate(
			func() string { return privacyblur.PreludeScript(mustJSON(app.effectivePrivacyBlur())) },
			app.handlePrivacyBlurWrite,
		)
		app.host.SetAvatarDelegate(app.handleAvatarUpdate)
		app.host.SetSmartRAMSaver(smartRAMSaver)
	}

	// Auto updater: passive and fail-soft until the user opts in. The
	// checker never touches the network until startup's kick fires.
	updatesDir := filepath.Join(dirs.Cache, "updates")
	if err := os.MkdirAll(updatesDir, 0o700); err != nil {
		log.Printf("updater: cannot create updates dir: %v", err)
	}
	app.checker = updater.NewChecker(updater.NewClient(), filepath.Join(dirs.Cache, "update-check.json"), build.Version, build.Commit)
	app.downloader = updater.NewDownloader(updatesDir)
	app.lastStatus = app.checker.Snapshot()

	// Restore active account from config on startup
	if accountSvc != nil {
		accounts, _ := accountSvc.List()
		if len(accounts) > 0 {
			savedActive, _ := cfgStore.GetActiveAccount()
			found := false
			for _, a := range accounts {
				if a.ID == savedActive {
					found = true
					break
				}
			}
			if found {
				app.activeAccount = savedActive
			} else {
				// Fallback to first valid account
				app.activeAccount = accounts[0].ID
				_ = cfgStore.SetActiveAccount(accounts[0].ID)
			}
		}
	}

	return app
}

// mustJSON marshals v with encoding/json, which also escapes <, > and &, so
// the output is safe to embed in a one-line JavaScript assignment.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("privacy blur: cannot marshal settings: %v", err))
	}
	return string(b)
}

// effectivePrivacyBlur reads the section from config.json on every call and
// returns the effective settings (defaults filled when never written).
func (a *App) effectivePrivacyBlur() config.PrivacyBlurConfig {
	got, err := a.configStore.GetPrivacyBlurSettings()
	if err != nil || got == nil {
		return config.DefaultPrivacyBlurConfig()
	}
	return config.NormalizePrivacyBlurConfig(*got)
}

// handlePrivacyBlurWrite merges a page-originated patch (e.g. the engine's
// Alt+/ handler) into the stored settings under the store's lock, then
// broadcasts the latest stored state to every open session.
func (a *App) handlePrivacyBlurWrite(values map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("privacy blur: write hook panicked: %v", r)
		}
	}()
	if _, err := a.configStore.MergePrivacyBlurSettings(values); err != nil {
		log.Printf("privacy blur: merge failed: %v", err)
		return
	}
	a.broadcastPrivacyBlur()
}

// broadcastPrivacyBlur applies the LATEST stored privacy blur settings to
// every open session and emits privacy-blur:changed. The payload is re-read
// from the store under broadcastMu, so broadcasts from concurrent writers
// converge instead of interleaving stale and fresh snapshots per session.
func (a *App) broadcastPrivacyBlur() {
	a.broadcastMu.Lock()
	defer a.broadcastMu.Unlock()
	cfg := a.effectivePrivacyBlur()
	if a.host != nil {
		a.host.EvalAll(privacyblur.ApplyScript(mustJSON(cfg)))
	}
	a.emitPrivacyBlurChanged(cfg)
}

func (a *App) emitPrivacyBlurChanged(cfg config.PrivacyBlurConfig) {
	if a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "privacy-blur:changed", cfg)
}

// GetPrivacyBlurSettings returns the effective privacy blur settings.
func (a *App) GetPrivacyBlurSettings() config.PrivacyBlurConfig {
	return a.effectivePrivacyBlur()
}

// SetPrivacyBlurSettings replaces the whole privacy blur section, persists
// it, broadcasts the latest state to every open session live, and emits
// privacy-blur:changed.
func (a *App) SetPrivacyBlurSettings(cfg config.PrivacyBlurConfig) error {
	cfg = config.NormalizePrivacyBlurConfig(cfg)
	if err := a.configStore.SetPrivacyBlurSettings(cfg); err != nil {
		return err
	}
	a.broadcastPrivacyBlur()
	return nil
}

// ListAccounts returns all registered accounts.
func (a *App) ListAccounts() ([]account.Account, error) {
	if a.accountSvc == nil {
		return nil, fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.List()
}

// avatarChanged is the payload of the account-avatar:changed Wails event.
// An empty hash means the photo was cleared and the UI should use initials.
type avatarChanged struct {
	ID   string `json:"id"`
	Hash string `json:"hash"`
}

// handleAvatarUpdate stores a profile photo posted by a WebView2 session. An
// empty data URI is the photo-cleared signal from the page.
func (a *App) handleAvatarUpdate(id browser.ProfileID, dataURI string) {
	accountID := string(id)
	if accountID == "" || a.avatarStore == nil {
		return
	}
	if dataURI == "" {
		if err := a.avatarStore.Clear(accountID); err != nil {
			log.Printf("avatar: cannot clear %s: %v", accountID, err)
			return
		}
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "account-avatar:changed", avatarChanged{ID: accountID})
		}
		return
	}
	hash, changed, err := a.avatarStore.Put(accountID, dataURI)
	if err != nil {
		log.Printf("avatar: cannot store %s: %v", accountID, err)
		return
	}
	if !changed || a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "account-avatar:changed", avatarChanged{ID: accountID, Hash: hash})
}

// GetAccountAvatar returns the cached profile photo as a data URI, or "" when
// the account has none. Bound to the frontend.
func (a *App) GetAccountAvatar(id string) string {
	if a.avatarStore == nil {
		return ""
	}
	return a.avatarStore.Get(id)
}

// GetAccount returns an account by ID.
func (a *App) GetAccount(id string) (*account.Account, error) {
	if a.accountSvc == nil {
		return nil, fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.Get(id)
}

// CreateAccount creates a new account with an isolated browser profile.
func (a *App) CreateAccount(name string) (*account.Account, error) {
	if a.accountSvc == nil {
		return nil, fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.Create(name)
}

// RenameAccount changes the display name of an existing account.
func (a *App) RenameAccount(id string, name string) error {
	if a.accountSvc == nil {
		return fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.Rename(id, name)
}

// RemoveAccount deletes an account and its isolated browser profile. A session
// kept alive in the background by account switching is closed first so the
// profile directory is not locked.
func (a *App) RemoveAccount(id string) error {
	a.mu.Lock()
	if a.activeAccount == id || a.currentProfile == id {
		a.mu.Unlock()
		return fmt.Errorf("account is currently active; switch to another account first")
	}
	a.mu.Unlock()

	if a.accountSvc == nil {
		return fmt.Errorf("account service not initialized")
	}

	// Close a background session for this account if one is still running,
	// then delete. The engine may still hold file handles for a short time
	// after its window is destroyed, so retry the deletion until the OS
	// releases the profile directory.
	if a.manager != nil && a.manager.IsOpen(id) {
		if err := a.manager.Close(id); err != nil {
			log.Printf("remove: closing session for %q failed: %v", id, err)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for {
		if lastErr = a.accountSvc.Remove(id); lastErr == nil {
			if a.avatarStore != nil {
				if err := a.avatarStore.Clear(id); err != nil {
					log.Printf("remove: cannot clear avatar for %q: %v", id, err)
				}
			}
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

// sessionAlive reports whether the manager currently holds a live session.
// Callers must not hold a.mu.
func (a *App) sessionAlive(accountID string) bool {
	if a.manager == nil || accountID == "" {
		return false
	}
	return a.manager.IsOpen(accountID)
}

// GetActiveAccountID returns the currently active account ID.
func (a *App) GetActiveAccountID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.activeAccount
}

// SwitchAccount switches the active session to the target account.
func (a *App) SwitchAccount(accountID string) error {
	a.mu.Lock()
	if a.switching {
		a.mu.Unlock()
		return fmt.Errorf("account switching already in progress")
	}
	// Only short-circuit when the session for this account is genuinely running.
	if a.activeAccount == accountID && a.currentProfile == accountID && a.sessionAlive(accountID) {
		a.mu.Unlock()
		return nil
	}
	a.switching = true
	current := a.currentProfile
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.switching = false
		a.mu.Unlock()
	}()

	// 1. Validate account exists
	if a.accountSvc != nil {
		if _, err := a.accountSvc.Get(accountID); err != nil {
			return fmt.Errorf("cannot switch to account %q: %w", accountID, err)
		}
	}

	// 2. Hide the previous session's window without destroying it, so every
	// opened account stays alive in the background and switching back is
	// instant. The engine keeps rendering and the profile flushes on app exit.
	if current != "" && current != accountID {
		log.Printf("switch: hiding live session %q", current)
		if err := a.manager.HideSession(current); err != nil {
			log.Printf("switch: hiding %q failed: %v", current, err)
		}
	}

	// 3. Ensure the native parent window is attached before embedding
	a.ensureMainWindow()

	// 4. Activate requested account profile: show it if it is already open,
	// otherwise start the engine for it.
	if a.manager.IsOpen(accountID) {
		if err := a.manager.ShowSession(accountID); err != nil {
			return fmt.Errorf("failed to show live session %q: %w", accountID, err)
		}
		log.Printf("switch: re-showing live session %q", accountID)
	} else {
		_ = a.manager.Create(accountID)
		log.Printf("switch: activating profile %q", accountID)
		if err := a.manager.Open(accountID); err != nil {
			return fmt.Errorf("failed to activate profile %q: %w", accountID, err)
		}
		log.Printf("switch: profile %q active", accountID)
	}

	// 5. Make the newly active session the visible one across all open
	// sessions, and mark it as active so the next open starts hidden.
	a.host.SetActiveSession(browser.ProfileID(accountID))

	// 4. Update memory and persisted config state
	a.mu.Lock()
	a.currentProfile = accountID
	a.activeAccount = accountID
	a.mu.Unlock()

	if a.configStore != nil {
		_ = a.configStore.SetActiveAccount(accountID)
	}

	return nil
}

// SetWebViewVisible controls the native child WebView visibility (e.g. during modals).
func (a *App) SetWebViewVisible(visible bool) error {
	if a.host != nil {
		return a.host.SetVisible(visible)
	}
	return nil
}

// NotifyWindowResize updates the child WebView dimensions when the desktop window resizes.
func (a *App) NotifyWindowResize(width, height int) error {
	if a.host != nil {
		return a.host.UpdateBounds(width, height)
	}
	return nil
}

// GetCloseToTray returns whether the app should minimize to tray on window close.
// GetCloseToTray returns whether the app should minimize to tray on window close.
func (a *App) GetCloseToTray() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.closeToTray
}

// SetCloseToTray updates and persists the close-to-tray preference.
func (a *App) SetCloseToTray(enabled bool) error {
	a.mu.Lock()
	a.closeToTray = enabled
	a.mu.Unlock()

	if a.configStore != nil {
		return a.configStore.SetCloseToTray(enabled)
	}
	return nil
}

// GetSmartRAMSaver returns whether smart RAM saver is enabled.
func (a *App) GetSmartRAMSaver() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.smartRAMSaver
}

// SetSmartRAMSaver updates and persists the smart RAM saver preference.
// The host is updated so sessions opened from now on get the right Chromium
// flags. Already-open engines keep running untouched; per the no-unload rule
// they are never closed and recreated just to apply a setting.
func (a *App) SetSmartRAMSaver(enabled bool) error {
	a.mu.Lock()
	a.smartRAMSaver = enabled
	a.mu.Unlock()

	if a.configStore != nil {
		if err := a.configStore.SetSmartRAMSaver(enabled); err != nil {
			return err
		}
	}
	if a.host != nil {
		a.host.SetSmartRAMSaver(enabled)
	}
	return nil
}

// noteWindowGeometry records the latest known window geometry and persists it
// after a short quiet period, so dragging or resizing the window does not
// hammer the config file. Called from the geometry polling loop
// (windowgeom_windows.go) whenever the sampled geometry changes.
func (a *App) noteWindowGeometry(x, y, w, h int, maximised bool) {
	a.mu.Lock()
	a.geomPending = &config.WindowState{X: x, Y: y, Width: w, Height: h, Maximised: maximised}
	if a.geomTimer != nil {
		a.geomTimer.Stop()
	}
	a.geomTimer = time.AfterFunc(400*time.Millisecond, a.flushWindowGeometry)
	a.mu.Unlock()
}

// flushWindowGeometry writes the pending window geometry to the config store.
// When the window is maximised only the maximised flag is updated and the last
// known normal bounds are kept, so un-maximising returns the user to the size
// they actually had instead of the maximised rectangle.
func (a *App) flushWindowGeometry() {
	a.mu.Lock()
	p := a.geomPending
	a.geomPending = nil
	if a.geomTimer != nil {
		a.geomTimer.Stop()
		a.geomTimer = nil
	}
	a.mu.Unlock()

	if p == nil || a.configStore == nil {
		return
	}
	if p.Maximised {
		cur, err := a.configStore.GetWindowState()
		if err != nil {
			// Read failure: persist the maximised geometry anyway so the
			// flag is not silently dropped.
			log.Printf("app: cannot load stored window state (%v); storing maximised bounds as-is", err)
		} else if cur != nil {
			// Keep the last known normal bounds so un-maximising returns
			// the user to the size they actually had.
			p.X, p.Y, p.Width, p.Height = cur.X, cur.Y, cur.Width, cur.Height
		} else {
			// First-ever save of a maximised window: persist as-is; without
			// stored bounds there is no normal geometry to preserve yet.
			log.Printf("app: no normal bounds on file; storing maximised geometry as-is")
		}
	}
	if err := a.configStore.SetWindowState(*p); err != nil {
		log.Printf("app: cannot persist window geometry: %v", err)
	} else {
		log.Printf("app: window geometry saved (x=%d y=%d %dx%d maximised=%v)", p.X, p.Y, p.Width, p.Height, p.Maximised)
	}
}

// onBeforeClose is called by Wails before closing the main window.
// If close-to-tray is enabled and the tray icon is actually registered (and
// the exit was not forced via tray "Quit"), it hides the window and prevents
// app exit; otherwise it allows normal exit so the app is never left hidden
// with no way to quit.
//
// Exit flushes WebView2 sessions BEFORE Wails destroys its window. Once the
// parent window is gone, the embedded child session windows are orphaned and
// their session threads can no longer respond to teardown callbacks — every
// teardown would wait out the call timeout and the app would appear to hang.
// shutdown() repeats CloseCurrent as a safety net; it is a no-op for sessions
// already closed.
func (a *App) onBeforeClose(ctx context.Context) (prevent bool) {
	// Read the flags under the mutex only; WindowHide marshals to the Wails
	// runtime and may block, so it must not run while we hold a.mu.
	a.mu.Lock()
	forceQuit := a.forceQuit
	closeToTray := a.closeToTray
	quitting := a.quitting
	a.mu.Unlock()
	log.Printf("beforeclose: enter (forceQuit=%v closeToTray=%v quitting=%v)", forceQuit, closeToTray, quitting)

	// Flush pending geometry before deciding; the window still exists here
	// and its latest bounds are the ones the user left it in.
	flushStart := time.Now()
	a.flushWindowGeometry()
	log.Printf("beforeclose: geometry flush took %s", time.Since(flushStart))

	// Hide to tray only on an ordinary close with the preference enabled;
	// tray "Quit" (forceQuit) really exits.
	if closeToTray && a.trayReady() && !forceQuit && !quitting {
		wailsRuntime.WindowHide(ctx)
		return true
	}

	// The teardown path for a real quit. Sessions are flushed BEFORE Wails
	// destroys its window (once the parent goes, embedded child windows are
	// orphaned and their teardown stalls), but a wedged WebView2 session can
	// hold this path hostage for a long time — observed at 33s per session on
	// both sessions every time. Doing that inline froze the window for over a
	// minute. So hide the window now, tear down in the background, and only
	// then let Wails destroy the window. The process still exits only after
	// the flush completes, preserving the guarantee above.
	a.mu.Lock()
	if a.quitting {
		// Reached via quitApp once teardown is done; allow the close.
		a.mu.Unlock()
		return false
	}
	a.quitting = true
	a.mu.Unlock()

	wailsRuntime.WindowHide(ctx)
	go func() {
		closeStart2 := time.Now()
		_ = a.CloseCurrent()
		log.Printf("beforeclose: background CloseCurrent took %s", time.Since(closeStart2))
		a.quitApp()
	}()
	return true
}

// quitApp marks forceQuit and exits the application cleanly.
func (a *App) quitApp() {
	a.mu.Lock()
	a.forceQuit = true
	ctx := a.ctx
	a.mu.Unlock()

	if ctx != nil {
		wailsRuntime.Quit(ctx)
	}
}

// showFromBackground brings the main window back to the foreground and
// restores the active session's visibility. It is used by the tray "Open"
// action and when a second instance of the application is launched while a
// first instance is hidden to the tray.
func (a *App) showFromBackground() {
	a.mu.Lock()
	ctx := a.ctx
	act := a.activeAccount
	a.mu.Unlock()
	if ctx != nil {
		// WindowUnminimise is deliberately not called: Wails implements it
		// with WM_SYSCOMMAND(SC_RESTORE), which un-maximises a maximised
		// window to the normal width/height. WindowShow alone restores a
		// minimised window and leaves a maximised one maximised.
		wailsRuntime.WindowShow(ctx)
	}
	if a.host != nil && act != "" {
		a.host.SetActiveSession(browser.ProfileID(act))
	}
}

// onNotificationActivate is called when the user clicks a desktop
// notification created by one of the session's pages. It brings the main
// window to the front and tells the frontend which account the notification
// came from; the frontend then runs its usual switch flow so the sidebar and
// the visible session stay consistent. The page's own onclick handler also
// runs and navigates the session to the chat.
func (a *App) onNotificationActivate(id string) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	// WindowUnminimise is deliberately not called: Wails implements it with
	// WM_SYSCOMMAND(SC_RESTORE), which un-maximises a maximised window to
	// the normal width/height. WindowShow alone restores a minimised window
	// and leaves a maximised one maximised.
	wailsRuntime.WindowShow(ctx)
	// A notification-click is input to the WebView2 browser process, not to
	// this one, so SetForegroundWindow alone cannot win foreground rights.
	// platformBringToFront uses the attach-thread-input trick to force the
	// window to the front on Windows (no-op elsewhere).
	a.platformBringToFront()
	wailsRuntime.EventsEmit(ctx, "notification:activate", id)
}

// onSecondInstanceLaunch is called by the single-instance lock in the already
// running instance when the user launches the app again. The second process
// exits by itself; this brings the first instance to the front.
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	log.Printf("app: second instance launch intercepted (args=%v); restoring first instance", data.Args)
	a.showFromBackground()
}

// trayReady reports whether the tray icon is registered and usable.
func (a *App) trayReady() bool {
	if a.tray == nil {
		return false
	}
	return a.tray.Ready()
}

// retryTraySetup attempts tray registration once more if it never succeeded
// (e.g. the main window was not visible when attachMainWindow ran, or
// Shell_NotifyIconW rejected the icon). Setup() is idempotent once the icon
// is registered, so this is safe to call from multiple probe points.
func (a *App) retryTraySetup(hwnd uintptr) {
	if hwnd == 0 || a.tray == nil {
		return
	}
	if a.tray.Ready() {
		return
	}
	a.tray.Setup(hwnd)
}

// startup is called when the app starts. The context is saved so we can call
// the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	// Sweep WebView2 profile folders the shell left behind on previous
	// launches (run-<pid>). startup only runs in the single-instance lock
	// winner, so this never touches the live profile of a running instance.
	// Best-effort: failures are logged, never fatal.
	if err := cleanupStaleShellProfiles(a.dirs.Data, os.Getpid()); err != nil {
		log.Printf("shell-cleanup: %v", err)
	}
	// Auto updater runs in the background; it never blocks startup and never
	// downloads anything without the user opting in.
	go a.updateOnStartup(ctx)
	// Smart RAM saver periodically trims background sessions when system RAM load is high.
	go a.smartRAMSaverLoop(ctx)
	// Position restore happens in the geometry polling loop (windowgeom_*.go):
	// the window may not be visible yet when OnStartup runs, and the loop
	// applies the saved position once the window is observable.
}

func (a *App) smartRAMSaverLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !a.GetSmartRAMSaver() {
				continue
			}
			load, err := procstats.SystemMemoryLoad()
			if err != nil || load < 70 {
				continue
			}
			a.trimBackgroundSessions()
		}
	}
}

func (a *App) trimBackgroundSessions() {
	a.mu.Lock()
	active := a.activeAccount
	a.mu.Unlock()

	if a.accountSvc == nil {
		return
	}
	accounts, err := a.accountSvc.List()
	if err != nil {
		return
	}
	layout := a.manager.Layout()

	for _, acc := range accounts {
		if acc.ID == active {
			continue
		}
		id, err := browser.ParseProfileID(acc.ID)
		if err != nil {
			continue
		}
		userDataDir := layout.UserDataDir(id)
		if count, err := procstats.TrimUserDataWorkingSet(userDataDir); err == nil && count > 0 {
			log.Printf("smart-ram-saver: trimmed %d processes for background account %s (system RAM load >= 70%%)", count, acc.ID)
		}
	}
}

// shutdown is called when the app is closing. It tears down the system tray
// and flushes all browser sessions.
func (a *App) shutdown(ctx context.Context) {
	shutdownStart := time.Now()
	log.Printf("shutdown: start")
	a.mu.Lock()
	if a.geomStop != nil {
		close(a.geomStop)
		done := a.geomDone
		a.geomStop = nil
		a.geomDone = nil
		a.mu.Unlock()
		// Wait for the polling goroutine to exit before the final flush, so a
		// pending geometry write cannot be armed after shutdown proceeds.
		if done != nil {
			<-done
		}
	} else {
		a.mu.Unlock()
	}
	flushStart := time.Now()
	a.flushWindowGeometry()
	log.Printf("shutdown: geometry flush took %s", time.Since(flushStart))
	if a.tray != nil {
		trayStart := time.Now()
		a.tray.Remove()
		log.Printf("shutdown: tray removed in %s", time.Since(trayStart))
	}
	closeStart := time.Now()
	_ = a.CloseCurrent()
	log.Printf("shutdown: CloseCurrent took %s", time.Since(closeStart))
	log.Printf("shutdown: complete in %s", time.Since(shutdownStart))
}

// domReady is called when the frontend DOM is fully loaded. It attaches the
// native child window embedding to Wails's main window before any session opens.
func (a *App) domReady(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	if a.geomStop == nil {
		a.geomStop = make(chan struct{})
		a.geomDone = make(chan struct{})
		startGeometryPolling(a.geomStop, a.geomDone, a.noteWindowGeometry, a.windowState)
	}
	a.mu.Unlock()
	a.attachMainWindow()
}

// OpenAccountA opens Account A in its isolated WebView2 window.
func (a *App) OpenAccountA() (string, error) {
	return a.OpenAccount("account-a")
}

// OpenAccountB opens Account B in its isolated WebView2 window.
func (a *App) OpenAccountB() (string, error) {
	return a.OpenAccount("account-b")
}

// OpenAccount opens a specific account by ID.
func (a *App) OpenAccount(accountID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure profile exists
	_ = a.manager.Create(accountID)

	if a.manager.IsOpen(accountID) {
		if err := a.manager.ShowSession(accountID); err != nil {
			return "", fmt.Errorf("failed to show live session %q: %w", accountID, err)
		}
	} else if err := a.manager.Open(accountID); err != nil {
		return "", fmt.Errorf("failed to open profile %q: %w", accountID, err)
	}

	a.currentProfile = accountID
	a.host.SetActiveSession(browser.ProfileID(accountID))
	return accountID, nil
}

// CloseAccount closes the specified account's session.
func (a *App) CloseAccount(accountID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.manager.Close(accountID); err != nil {
		return fmt.Errorf("failed to close profile %q: %w", accountID, err)
	}

	if a.currentProfile == accountID {
		a.currentProfile = ""
	}
	return nil
}

// CloseCurrent closes all live WebView2 sessions, flushing every profile to disk.
func (a *App) CloseCurrent() error {
	log.Printf("close-current: start")
	a.mu.Lock()
	a.currentProfile = ""
	a.mu.Unlock()

	if a.manager != nil {
		log.Printf("close-current: closing manager sessions")
		closeStart := time.Now()
		if err := a.manager.CloseAll(); err != nil {
			log.Printf("close-current: manager CloseAll error: %v", err)
		}
		log.Printf("close-current: manager sessions closed in %s", time.Since(closeStart))
	}
	if a.host != nil {
		log.Printf("close-current: closing host sessions")
		closeStart := time.Now()
		if err := a.host.CloseAll(); err != nil {
			log.Printf("close-current: host CloseAll error: %v", err)
		}
		log.Printf("close-current: host sessions closed in %s", time.Since(closeStart))
	}
	return nil
}

// CurrentProfile returns the ID of the current open profile.
func (a *App) CurrentProfile() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.currentProfile
}

// ListProfiles returns status and storage information for all profiles.
func (a *App) ListProfiles() []browser.Info {
	return a.manager.List()
}

// InspectProfile returns detailed storage evidence for a profile.
func (a *App) InspectProfile(accountID string) ([]browser.Evidence, error) {
	return a.manager.Inspect(accountID)
}

// DeleteProfile deletes a profile and its persistent storage.
func (a *App) DeleteProfile(accountID string) error {
	a.mu.Lock()
	if a.currentProfile == accountID {
		a.currentProfile = ""
	}
	a.mu.Unlock()

	return a.manager.Delete(accountID)
}

// OpenLegacyPecut1 opens a session using the legacy Pecut 1 storage path.
func (a *App) OpenLegacyPecut1() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	legacyID := "legacy-pecut1"
	_ = a.manager.Create(legacyID)

	// Point legacy session to dirs.Data
	legacyPath := filepath.Join(a.dirs.Data, "EBWebView")
	_ = os.MkdirAll(legacyPath, 0o700)

	probe := browser.Probe{
		Path:   legacyPath,
		Exited: true,
	}

	_, err := a.host.OpenProfile(browser.ProfileID(legacyID), probe)
	if err != nil {
		return fmt.Errorf("failed to open legacy session: %w", err)
	}

	a.currentProfile = legacyID
	return nil
}

// handler answers the startup request.
// In legacy Pecut 1 mode, it redirects to web.whatsapp.com directly.
// In Pecut 2 mode, it returns nil so Wails serves the embedded frontend assets.
func (a *App) handler() http.Handler {
	if a.legacyMode {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, whatsappURL, http.StatusFound)
		})
	}
	return nil
}
