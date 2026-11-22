//go:build windows

package procstats

import (
	"strings"
	"testing"
)

func TestSnapshotSmoke(t *testing.T) {
	snap, err := Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	for _, p := range snap {
		if p.PID == 0 {
			t.Errorf("zero pid for entry %+v", p)
		}
		if p.Name == "" {
			t.Errorf("empty name for pid %d", p.PID)
		}
		if p.Name == "msedgewebview2.exe" && p.WorkingSetBytes == 0 {
			t.Logf("webview process %d had zero working set (may be exiting)", p.PID)
		}
	}
	selfPID, selfWS, selfTicks := SelfStats()
	if selfPID == 0 {
		t.Errorf("self pid is 0")
	}
	if selfWS == 0 {
		t.Errorf("self working set is 0")
	}
	if selfTicks == 0 {
		t.Errorf("self cpu ticks are 0")
	}
	t.Logf("processes: %d, self ws=%dMB ticks=%d", len(snap), selfWS/1024/1024, selfTicks)

	bridges := 0
	for _, p := range snap {
		if strings.Contains(strings.ToLower(p.CommandLine), "ebwebview") {
			bridges++
		}
	}
	t.Logf("webview processes with EBWebView user-data-dir: %d", bridges)
}

func TestTrimProcessWorkingSet(t *testing.T) {
	selfPID, _, _ := SelfStats()
	if err := TrimProcessWorkingSet(selfPID); err != nil {
		t.Fatalf("TrimProcessWorkingSet(selfPID) failed: %v", err)
	}

	count, err := TrimUserDataWorkingSet("")
	if err != nil {
		t.Fatalf("TrimUserDataWorkingSet(\"\") returned error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 trimmed processes for empty dir, got %d", count)
	}

	load, err := SystemMemoryLoad()
	if err != nil {
		t.Fatalf("SystemMemoryLoad() failed: %v", err)
	}
	if load == 0 || load > 100 {
		t.Errorf("SystemMemoryLoad() expected 1..100, got %d", load)
	}
	t.Logf("system memory load: %d%%", load)
}


