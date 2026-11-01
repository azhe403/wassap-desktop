package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/appdirs"
	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/storage"
)

// whatsappURL is the official WhatsApp Web address.
const whatsappURL = "https://web.whatsapp.com"

// HostController defines the opener and session host interface.
type HostController interface {
	browser.Opener
	CloseAll() error
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

	mu             sync.Mutex
	currentProfile string
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

	app := &App{
		dirs:       dirs,
		legacyMode: legacyMode,
		manager:    manager,
		host:       host,
		inspector:  insp,
		accountSvc: accountSvc,
	}

	// Ensure the default accounts A and B exist in the manager
	_ = manager.Create("account-a")
	_ = manager.Create("account-b")

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

// RemoveAccount deletes an account and its isolated browser profile.
func (a *App) RemoveAccount(id string) error {
	if a.accountSvc == nil {
		return fmt.Errorf("account service not initialized")
	}
	return a.accountSvc.Remove(id)
}

// startup is called when the app starts. The context is saved so we can call
// the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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

	if err := a.manager.Open(accountID); err != nil {
		return "", fmt.Errorf("failed to open profile %q: %w", accountID, err)
	}

	a.currentProfile = accountID
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

// CloseCurrent closes the current active WebView.
func (a *App) CloseCurrent() error {
	a.mu.Lock()
	current := a.currentProfile
	a.mu.Unlock()

	if current == "" {
		// Close any open session from the host
		return a.host.CloseAll()
	}
	return a.CloseAccount(current)
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
