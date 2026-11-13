//go:build windows

package webview2

import (
	"testing"

	"wassap-desktop/internal/browser"
)

// TestShouldShowSessionOnlyActiveSession locks the rule that decides which
// session window is visible. Every open account is embedded at the same
// position, so showing more than the active session stacks the accounts on
// top of each other and makes account navigation ambiguous.
func TestShouldShowSessionOnlyActiveSession(t *testing.T) {
	const (
		active = browser.ProfileID("account-a")
		other  = browser.ProfileID("account-b")
	)

	cases := []struct {
		name    string
		visible bool
		profile browser.ProfileID
		active  browser.ProfileID
		want    bool
	}{
		{"hidden hides the active session", false, active, active, false},
		{"hidden hides other sessions", false, other, active, false},
		{"visible shows the active session", true, active, active, true},
		{"visible hides other sessions", true, other, active, false},
		{"no active session yet shows no session", true, other, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldShowSession(tc.visible, tc.profile, tc.active)
			if got != tc.want {
				t.Errorf("shouldShowSession(visible=%v, profile=%q, active=%q) = %v, want %v",
					tc.visible, tc.profile, tc.active, got, tc.want)
			}
		})
	}
}
