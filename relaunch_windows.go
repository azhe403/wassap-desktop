//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

const synchronize = 0x00100000
const waitTimeout = 0x00000102 // WAIT_TIMEOUT

// waitForPID blocks until the process with the given pid exits, or 60
// seconds elapse, whichever comes first. It is used by the relaunch helper:
// the new instance may only start after the old one has fully exited (past
// WebView2 teardown), otherwise the single-instance lock kills it and the
// profiles are still held.
func waitForPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	h, err := windows.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		// The process is most likely already gone; that is not an error for
		// our purposes — the caller can start the new instance.
		return nil
	}
	defer windows.CloseHandle(h)
	r, err := windows.WaitForSingleObject(h, 60*1000)
	if err != nil {
		return err
	}
	if r == waitTimeout {
		return fmt.Errorf("timed out waiting for pid %d to exit", pid)
	}
	return nil
}
