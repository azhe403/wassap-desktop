package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/config"
	"wassap-desktop/internal/storage"
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
}

// App is the desktop shell application and Wails binding target.
type App struct {
	ctx        context.Context
	dirs       appdirs.Dirs
	legacyMode bool

	manager   *browser.Manager
	host      HostController
	inspector browser.Inspector
	accountSvc account.Service
	configStore config.Store

	mu             sync.Mutex
	currentProfile string
	activeAccount  string
	switching      bool
}

// NewApp creates a new App application struct.
func NewApp(dirs appdirs.Dirs, legacyMode bool) *App {
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

	app := &App{
		dirs:        dirs,
		legacyMode:  legacyMode,
		manager:     manager,
		host:        host,
		inspector:   insp,
		accountSvc:  accountSvc,
		configStore: cfgStore,
	}

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

// ListAccounts returns all registered accounts.
func (a *App) ListAccounts() ([]account.Account, error) {
	if a.accountSvc == nil {
		return nil, fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.List()
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

// startup is called when the app starts. The context is saved so we can call
// the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// domReady is called when the frontend DOM is fully loaded. It attaches the
// native child window embedding to Wails's main window before any session opens.
func (a *App) domReady(ctx context.Context) {
	a.ctx = ctx
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
	a.mu.Lock()
	a.currentProfile = ""
	a.mu.Unlock()

	if a.manager != nil {
		_ = a.manager.CloseAll()
	}
	if a.host != nil {
		return a.host.CloseAll()
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
