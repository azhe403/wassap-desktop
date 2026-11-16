//go:build windows

package app

import (
	"os/exec"
	"testing"
	"time"
)

func TestWaitForPIDOnDeadProcessReturnsPromptly(t *testing.T) {
	// A process that exits at once; WaitForPID must not error.
	cmd := exec.Command("cmd", "/c", "exit /b 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("could not run helper process: %v", err)
	}
	pid := cmd.Process.Pid
	start := time.Now()
	if err := WaitForPID(pid); err != nil {
		t.Fatalf("WaitForPID on a dead process = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("WaitForPID blocked for %s on a dead process", elapsed)
	}
}

func TestWaitForPIDBlocksUntilChildExits(t *testing.T) {
	// ping for ~2 seconds as a harmless long-lived child.
	cmd := exec.Command("ping", "-n", "2", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start ping: %v", err)
	}
	pid := cmd.Process.Pid

	start := time.Now()
	if err := WaitForPID(pid); err != nil {
		t.Fatalf("WaitForPID = %v, want nil", err)
	}
	elapsed := time.Since(start)
	_ = cmd.Wait() // reap

	if elapsed < 500*time.Millisecond {
		t.Fatalf("WaitForPID returned after %s; the child lived longer", elapsed)
	}
}

func TestWaitForPIDRejectsInvalidPid(t *testing.T) {
	if err := WaitForPID(0); err == nil {
		t.Fatal("pid 0 must be rejected")
	}
}
