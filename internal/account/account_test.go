package account_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"wassap-desktop/internal/account"
	"wassap-desktop/internal/browser"
)

func TestNewULID_Valid(t *testing.T) {
	id1, err := account.NewULID()
	if err != nil {
		t.Fatalf("unexpected error generating ULID: %v", err)
	}

	if len(id1) != 26 {
		t.Fatalf("expected 26 characters, got %d (%s)", len(id1), id1)
	}

	// Must be lowercase
	if id1 != strings.ToLower(id1) {
		t.Fatalf("expected lowercase ULID, got %s", id1)
	}

	// Must only contain valid Crockford Base32 lowercase characters (0-9, a-z excluding i, l, o, u)
	const crockfordBase32 = "0123456789abcdefghjkmnpqrstvwxyz"
	for _, c := range id1 {
		if !strings.ContainsRune(crockfordBase32, c) {
			t.Fatalf("invalid Crockford base32 character %c in %s", c, id1)
		}
	}

	// Verify two consecutive IDs are unique and ordered
	time.Sleep(2 * time.Millisecond)
	id2, err := account.NewULID()
	if err != nil {
		t.Fatalf("unexpected error generating second ULID: %v", err)
	}
	if id1 == id2 {
		t.Fatalf("expected distinct ULIDs, got duplicate: %s", id1)
	}
	if id1 >= id2 {
		t.Fatalf("expected id1 < id2 for monotonic time, got %s >= %s", id1, id2)
	}
}

func TestParseULID_Validation(t *testing.T) {
	valid, err := account.NewULID()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := account.ParseULID(valid)
	if err != nil {
		t.Fatalf("expected valid ULID to parse, got: %v", err)
	}
	if parsed != valid {
		t.Fatalf("expected %s, got %s", valid, parsed)
	}

	// Rejections
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"uppercase", strings.ToUpper(valid)},
		{"too short", valid[:25]},
		{"too long", valid + "a"},
		{"invalid char", "0123456789abcdefghij012345"}, // 'i' is not in Crockford
		{"whitespace", " " + valid[1:]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := account.ParseULID(tc.raw)
			if err == nil {
				t.Fatalf("expected error for %s (%q), got nil", tc.name, tc.raw)
			}
			if !errors.Is(err, account.ErrInvalidAccountID) {
				t.Fatalf("expected ErrInvalidAccountID, got %v", err)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid simple", "Personal", "Personal", false},
		{"valid with spaces trimmed", "  Work Account  ", "Work Account", false},
		{"empty", "", "", true},
		{"whitespace only", "   \t\n  ", "", true},
		{"too long", strings.Repeat("a", 101), "", true},
		{"max length allowed", strings.Repeat("a", 100), strings.Repeat("a", 100), false},
		{"contains control chars", "Personal\x00Account", "", true},
		{"contains newline", "Line1\nLine2", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := account.ValidateName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.input)
				}
				if !errors.Is(err, account.ErrInvalidAccountName) {
					t.Fatalf("expected ErrInvalidAccountName, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %q: %v", tc.input, err)
				}
				if got != tc.want {
					t.Fatalf("expected %q, got %q", tc.want, got)
				}
			}
		})
	}
}

func TestNewAccount(t *testing.T) {
	acc, err := account.NewAccount("Marketing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acc.Name != "Marketing" {
		t.Fatalf("expected name Marketing, got %s", acc.Name)
	}
	if len(acc.ID) != 26 {
		t.Fatalf("expected 26-char ULID, got %s", acc.ID)
	}
	if acc.CreatedAt.IsZero() || acc.UpdatedAt.IsZero() {
		t.Fatalf("expected non-zero timestamps, got %v, %v", acc.CreatedAt, acc.UpdatedAt)
	}

	// Must be 100% compliant with browser.ParseProfileID
	profileID, err := browser.ParseProfileID(acc.ID)
	if err != nil {
		t.Fatalf("account ID %q failed browser.ParseProfileID: %v", acc.ID, err)
	}
	if string(profileID) != acc.ID {
		t.Fatalf("profileID %s != acc.ID %s", profileID, acc.ID)
	}
}
