package app

import "testing"

func TestClassifyProcessLine(t *testing.T) {
	dataRoot := `c:\users\me\.local\share\wassap-desktop`
	accounts := map[string]string{
		`c:\users\me\.local\share\wassap-desktop\profiles\account-a\ebwebview`: "account-a",
		`c:\users\me\.local\share\wassap-desktop\profiles\account-b\ebwebview`: "account-b",
	}

	tests := []struct {
		name     string
		cmd      string
		wantID   string
		wantOurs bool
	}{
		{
			name:     "account engine",
			cmd:      `"C:\Program Files (x86)\Microsoft\EdgeWebView\Application\1\msedgewebview2.exe" --user-data-dir="C:\Users\me\.local\share\wassap-desktop\profiles\account-a\EBWebView\EBWebView" --embedded-browser-webview=1`,
			wantID:   "account-a",
			wantOurs: true,
		},
		{
			name:     "shell ui engine",
			cmd:      `msedgewebview2.exe --user-data-dir="C:\Users\me\.local\share\wassap-desktop\wails-shell\run-1234\EBWebView"`,
			wantID:   "",
			wantOurs: true,
		},
		{
			name:     "foreign webview app",
			cmd:      `msedgewebview2.exe --user-data-dir="C:\Users\me\AppData\Local\Packages\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\LocalState\EBWebView"`,
			wantID:   "",
			wantOurs: false,
		},
		{
			name:     "no command line",
			cmd:      "",
			wantID:   "",
			wantOurs: false,
		},
		{
			name:     "slashes and case are normalized",
			cmd:      `msedgewebview2.exe --user-data-dir="C:/Users/ME/.local/share/WASSAP-DESKTOP/profiles/account-b/EBWebView"`,
			wantID:   "account-b",
			wantOurs: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ours := classifyProcessLine(tc.cmd, dataRoot, accounts)
			if ours != tc.wantOurs {
				t.Fatalf("ours = %v, want %v", ours, tc.wantOurs)
			}
			if id != tc.wantID {
				t.Fatalf("id = %q, want %q", id, tc.wantID)
			}
		})
	}
}
