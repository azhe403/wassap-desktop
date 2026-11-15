//go:build windows

package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestWaitForPIDOnDeadProcessReturnsPromptly(t *testing.T) {
	// A process that exits at once; waitForPID must not error.
	cmd := exec.Command("cmd", "/c", "exit /b 0")
	if err := cmd.Run(); err != nil {
		t.Fatalf("could not run helper process: %v", err)
	}
	pid := cmd.Process.Pid
	start := time.Now()
	if err := waitForPID(pid); err != nil {
		t.Fatalf("waitForPID on a dead process = %v, want nil", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("waitForPID blocked for %s on a dead process", elapsed)
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
	if err := waitForPID(pid); err != nil {
		t.Fatalf("waitForPID = %v, want nil", err)
	}
	elapsed := time.Since(start)
	_ = cmd.Wait() // reap

	if elapsed < 500*time.Millisecond {
		t.Fatalf("waitForPID returned after %s; the child lived longer", elapsed)
	}
}

func TestWaitForPIDRejectsInvalidPid(t *testing.T) {
	if err := waitForPID(0); err == nil {
		t.Fatal("pid 0 must be rejected")
	}
}
