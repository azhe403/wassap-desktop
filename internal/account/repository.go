package account

import "errors"

// Repository errors returned by account storage.
var (
	// ErrAccountNotFound is returned when an account with a given ID does not exist.
	ErrAccountNotFound = errors.New("account: account not found")

	// ErrDuplicateAccountID is returned when attempting to insert an account whose ID already exists.
	ErrDuplicateAccountID = errors.New("account: duplicate account id")

	// ErrCorruptData is returned when the storage file exists but cannot be parsed.
	ErrCorruptData = errors.New("account: metadata file is corrupt")
)

// Repository defines the persistent storage operations for accounts.
type Repository interface {
	// List returns all accounts in creation/insertion order.
	List() ([]Account, error)

	// Get retrieves an account by ID, returning ErrAccountNotFound if absent.
	Get(id string) (*Account, error)

	// Save inserts or updates an account.
	Save(acc Account) error

	// Delete removes an account by ID, returning ErrAccountNotFound if absent.
	Delete(id string) error
}
