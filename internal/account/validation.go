package account

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrInvalidAccountName is returned when an account name fails validation.
var ErrInvalidAccountName = errors.New("account: invalid account name")

const (
	// MaxAccountNameLength is the maximum allowed characters in an account name.
	MaxAccountNameLength = 100
)

// ValidateName checks that name is valid and returns the trimmed string.
// Requirements:
// - Non-empty after trimming whitespace.
// - Maximum length 100 characters.
// - No control characters or null bytes.
func ValidateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: name cannot be empty", ErrInvalidAccountName)
	}

	runes := []rune(trimmed)
	if len(runes) > MaxAccountNameLength {
		return "", fmt.Errorf("%w: name exceeds %d characters", ErrInvalidAccountName, MaxAccountNameLength)
	}

	for _, r := range runes {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: name contains control characters", ErrInvalidAccountName)
		}
	}

	return trimmed, nil
}
