package account

import (
	"fmt"
	"time"
)

// Account represents a WhatsApp account managed by Wassap Desktop.
// Each account connects to an isolated browser profile managed by ProfileManager.
type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NewAccount validates the account name, generates a stable lowercase ULID,
// and returns an initialized Account struct.
func NewAccount(name string) (*Account, error) {
	validName, err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	id, err := NewULID()
	if err != nil {
		return nil, fmt.Errorf("account: failed to generate id: %w", err)
	}

	now := time.Now().UTC()
	return &Account{
		ID:        id,
		Name:      validName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
