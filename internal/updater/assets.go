package updater

// AssetName returns the GitHub Release asset for a platform/arch pair. Only
// Windows is supported in phase 1; everything else reports false and the
// updater stays silent there.
func AssetName(goos, goarch string) (string, bool) {
	switch {
	case goos == "windows" && goarch == "amd64":
		return "wassap-desktop-windows-amd64.exe", true
	case goos == "windows" && goarch == "arm64":
		return "wassap-desktop-windows-arm64.exe", true
	default:
		return "", false
	}
}
