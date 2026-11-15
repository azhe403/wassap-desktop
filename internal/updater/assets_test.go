package updater

import "testing"

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch string
		wantName     string
		wantOK       bool
	}{
		{"windows", "amd64", "wassap-desktop-windows-amd64.exe", true},
		{"windows", "arm64", "wassap-desktop-windows-arm64.exe", true},
		{"linux", "amd64", "", false},
		{"linux", "arm64", "", false},
		{"darwin", "amd64", "", false},
		{"darwin", "universal", "", false},
		{"windows", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		name, ok := AssetName(tc.goos, tc.goarch)
		if name != tc.wantName || ok != tc.wantOK {
			t.Errorf("AssetName(%q, %q) = (%q, %v), want (%q, %v)", tc.goos, tc.goarch, name, ok, tc.wantName, tc.wantOK)
		}
	}
}
