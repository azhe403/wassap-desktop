package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"wassap-desktop/internal/account"
)

// JSONAccountRepository is an atomic, concurrent-safe JSON implementation of account.Repository.
type JSONAccountRepository struct {
	filePath string
	mu       sync.RWMutex
	accounts []account.Account
}

// NewJSONAccountRepository initializes the repository from the given file path.
// If the file does not exist, an empty repository is initialized.
// If the file exists but contains corrupted or invalid JSON, it returns account.ErrCorruptData.
func NewJSONAccountRepository(filePath string) (*JSONAccountRepository, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("storage: failed to create directory %q: %w", dir, err)
	}

	repo := &JSONAccountRepository{
		filePath: filePath,
		accounts: make([]account.Account, 0),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return repo, nil
		}
		return nil, fmt.Errorf("storage: failed to read accounts file: %w", err)
	}

	// If file is empty, consider it empty account list
	if len(data) == 0 {
		return repo, nil
	}

	var loaded []account.Account
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("%w: %v", account.ErrCorruptData, err)
	}

	repo.accounts = loaded
	return repo, nil
}

// List returns a copy of all stored accounts in insertion order.
func (r *JSONAccountRepository) List() ([]account.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]account.Account, len(r.accounts))
	copy(result, r.accounts)
	return result, nil
}

// Get returns an account by ID, or account.ErrAccountNotFound if absent.
func (r *JSONAccountRepository) Get(id string) (*account.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, acc := range r.accounts {
		if acc.ID == id {
			copyAcc := acc
			return &copyAcc, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", account.ErrAccountNotFound, id)
}

// Save inserts a new account or updates an existing one in-place, persisting atomically.
func (r *JSONAccountRepository) Save(acc account.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	updated := false
	for i, existing := range r.accounts {
		if existing.ID == acc.ID {
			r.accounts[i] = acc
			updated = true
			break
		}
	}

	if !updated {
		r.accounts = append(r.accounts, acc)
	}

	return r.persistLocked()
}

// Delete removes an account by ID, persisting the update atomically.
func (r *JSONAccountRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	foundIndex := -1
	for i, acc := range r.accounts {
		if acc.ID == id {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return fmt.Errorf("%w: %s", account.ErrAccountNotFound, id)
	}

	r.accounts = append(r.accounts[:foundIndex], r.accounts[foundIndex+1:]...)
	return r.persistLocked()
}

// persistLocked writes the current in-memory accounts atomically to disk.
// It writes to a temporary file in the same directory, flushes, closes, and renames.
func (r *JSONAccountRepository) persistLocked() error {
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("storage: cannot create directory: %w", err)
	}

	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	tmpFile := filepath.Join(dir, fmt.Sprintf(".accounts.tmp-%s.json", hex.EncodeToString(randBytes[:])))

	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("storage: failed to create temporary file: %w", err)
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r.accounts); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("storage: failed to encode JSON: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("storage: failed to sync temporary file: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("storage: failed to close temporary file: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tmpFile, r.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("storage: atomic rename failed: %w", err)
	}

	return nil
}
