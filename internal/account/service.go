package account

import (
	"errors"
	"fmt"
	"time"

	"wassap-desktop/internal/browser"
)

// Service errors returned by AccountService operations.
var (
	// ErrAccountBusy is returned when attempting to remove an account whose browser profile is open or locked.
	ErrAccountBusy = errors.New("account: account profile is currently in use")

	// ErrProfileCreationFailed is returned when the browser profile could not be created on disk.
	ErrProfileCreationFailed = errors.New("account: failed to create browser profile")

	// ErrProfileDeletionFailed is returned when the browser profile could not be purged.
	ErrProfileDeletionFailed = errors.New("account: failed to delete browser profile")
)

// ProfileManager defines the browser profile lifecycle operations required by AccountService.
// This interface matches browser.ManagerInterface directly.
type ProfileManager interface {
	Create(accountID string) error
	Open(accountID string) error
	Close(accountID string) error
	Delete(accountID string) error
}

// Service defines the business operations for managing WhatsApp accounts.
type Service interface {
	List() ([]Account, error)
	Get(id string) (*Account, error)
	Create(name string) (*Account, error)
	Rename(id string, name string) error
	Remove(id string) error
}

type accountService struct {
	repo     Repository
	profiles ProfileManager
}

// NewService creates a new account service orchestrating metadata persistence and browser profiles.
func NewService(repo Repository, profiles ProfileManager) Service {
	return &accountService{
		repo:     repo,
		profiles: profiles,
	}
}

// List returns all accounts in creation order.
func (s *accountService) List() ([]Account, error) {
	return s.repo.List()
}

// Get returns an account by ID.
func (s *accountService) Get(id string) (*Account, error) {
	return s.repo.Get(id)
}

// Create validates the name, generates a ULID, provisions an isolated browser profile,
// and persists account metadata.
// If metadata persistence fails, the newly provisioned browser profile is deleted to prevent orphan directories.
func (s *accountService) Create(name string) (*Account, error) {
	validName, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	id, err := NewULID()
	if err != nil {
		return nil, fmt.Errorf("account: failed to generate id: %w", err)
	}

	if err := s.profiles.Create(id); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProfileCreationFailed, err)
	}

	now := time.Now().UTC()
	acc := &Account{
		ID:        id,
		Name:      validName,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Save(*acc); err != nil {
		// Rollback: delete created browser profile to avoid orphan folders on disk
		_ = s.profiles.Delete(id)
		return nil, fmt.Errorf("account: failed to persist metadata: %w", err)
	}

	return acc, nil
}

// Rename updates the name and timestamp of an existing account while preserving its ID and browser profile.
func (s *accountService) Rename(id string, name string) error {
	validName, err := ValidateName(name)
	if err != nil {
		return err
	}

	acc, err := s.repo.Get(id)
	if err != nil {
		return err
	}

	acc.Name = validName
	acc.UpdatedAt = time.Now().UTC()

	return s.repo.Save(*acc)
}

// Remove verifies the account exists, ensures its browser profile is not active,
// deletes the browser profile, and purges the account metadata.
// It refuses to delete an account whose profile is open or in use.
func (s *accountService) Remove(id string) error {
	acc, err := s.repo.Get(id)
	if err != nil {
		return err
	}

	// Delete browser profile first.
	// If the profile is open or locked, ProfileManager.Delete returns ErrProfileOpen / ErrProfileBusy.
	if err := s.profiles.Delete(acc.ID); err != nil {
		if errors.Is(err, browser.ErrProfileOpen) || errors.Is(err, browser.ErrProfileBusy) {
			return fmt.Errorf("%w: profile %s is in use: %v", ErrAccountBusy, id, err)
		}
		return fmt.Errorf("%w: %v", ErrProfileDeletionFailed, err)
	}

	// Only delete metadata after browser profile is cleanly removed
	return s.repo.Delete(acc.ID)
}
