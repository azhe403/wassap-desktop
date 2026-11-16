//go:build !windows

package app

// WaitForPID is a no-op off Windows; the updater is Windows-only this phase.
func WaitForPID(pid int) error {
	return nil
}
