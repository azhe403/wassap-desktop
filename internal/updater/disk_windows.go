//go:build windows

package updater

import (
	"golang.org/x/sys/windows"
)

func defaultFreeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeAvailable uint64
	// GetDiskFreeSpaceExW reports free bytes to the calling user; enough for
	// a pre-flight check.
	if err := windows.GetDiskFreeSpaceEx(p, &freeAvailable, nil, nil); err != nil {
		return 0, err
	}
	return freeAvailable, nil
}
