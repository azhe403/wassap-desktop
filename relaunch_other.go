//go:build !windows

package main

// waitForPID is a no-op off Windows; the updater is Windows-only this phase.
func waitForPID(pid int) error {
	return nil
}
