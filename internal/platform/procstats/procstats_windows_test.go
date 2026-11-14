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
