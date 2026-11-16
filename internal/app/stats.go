package app

import (
	"runtime"
	"strings"
	"time"

	"wassap-desktop/internal/browser"
	"wassap-desktop/internal/platform/procstats"
)

// ProfileResourceUsage is the live footprint of one account's isolated
// browser profile.
type ProfileResourceUsage struct {
	ID              string  `json:"id"`
	Running         bool    `json:"running"`
	ProcessCount    int     `json:"processCount"`
	WorkingSetBytes uint64  `json:"workingSetBytes"`
	CPUPercent      float64 `json:"cpuPercent"`
}

// ResourceUsage is a point-in-time sample of what Wassap Desktop costs in
// memory and CPU right now: the app itself (including the shell's own
// WebView2 engine), each open account profile, and the total.
type ResourceUsage struct {
	AppWorkingSetBytes   uint64                 `json:"appWorkingSetBytes"`
	AppCPUPercent        float64                `json:"appCPUPercent"`
	TotalWorkingSetBytes uint64                 `json:"totalWorkingSetBytes"`
	TotalCPUPercent      float64                `json:"totalCPUPercent"`
	Profiles             []ProfileResourceUsage `json:"profiles"`
}

// GetResourceUsage samples every process this app and its WebView2 sessions
// own. CPU percentages are deltas against the previous call — the frontend
// polls on an interval, so the first sample reports 0.
func (a *App) GetResourceUsage() ResourceUsage {
	snap, err := procstats.Snapshot()
	selfPID, selfWS, selfTicks := procstats.SelfStats()
	if err != nil {
		// Process accounting is best-effort; report the app bucket only.
		snap = nil
	}

	// Map each account's EBWebView user data folder to its account ID so a
	// webview process can be attributed to the account it serves.
	layout := a.manager.Layout()
	userDataByAccount := map[string]string{}
	var accountIDs []string
	if a.accountSvc != nil {
		if accounts, err := a.accountSvc.List(); err == nil {
			for _, acc := range accounts {
				id, err := browser.ParseProfileID(acc.ID)
				if err != nil {
					continue
				}
				accountIDs = append(accountIDs, acc.ID)
				userDataByAccount[normalizePath(layout.UserDataDir(id))] = acc.ID
			}
		}
	}

	a.statsMu.Lock()
	last := a.statsLastTicks
	lastAt := a.statsLastAt
	now := time.Now()
	elapsed := now.Sub(lastAt).Seconds()
	cores := float64(runtime.NumCPU())
	cpuPercent := func(pid uint32, ticks uint64) float64 {
		if last == nil || elapsed <= 0 {
			return 0
		}
		prev, ok := last[pid]
		if !ok || ticks < prev {
			return 0
		}
		return float64(ticks-prev) / (elapsed * 1e7 * cores) * 100
	}

	type sums struct {
		ws   uint64
		tick uint64
		cpu  float64
		n    int
	}
	perAccount := map[string]*sums{}
	appWS := selfWS
	appCPU := cpuPercent(selfPID, selfTicks)

	current := map[uint32]uint64{selfPID: selfTicks}
	for _, p := range snap {
		current[p.PID] = p.CPUTicks
		attributed := ""
		cmd := normalizePath(p.CommandLine)
		if cmd != "" {
			for dir, id := range userDataByAccount {
				if strings.Contains(cmd, dir) {
					attributed = id
					break
				}
			}
		}
		cpu := cpuPercent(p.PID, p.CPUTicks)
		if attributed == "" {
			// WebView2 engine not tied to an account profile: the shell's own
			// UI engine. It counts toward the app bucket.
			appWS += p.WorkingSetBytes
			appCPU += cpu
			continue
		}
		s := perAccount[attributed]
		if s == nil {
			s = &sums{}
			perAccount[attributed] = s
		}
		s.ws += p.WorkingSetBytes
		s.cpu += cpu
		s.n++
	}

	a.statsLastTicks = current
	a.statsLastAt = now
	a.statsMu.Unlock()

	usage := ResourceUsage{
		AppWorkingSetBytes: appWS,
		AppCPUPercent:      appCPU,
	}
	for _, id := range accountIDs {
		p := ProfileResourceUsage{ID: id}
		if s, ok := perAccount[id]; ok {
			p.Running = true
			p.ProcessCount = s.n
			p.WorkingSetBytes = s.ws
			p.CPUPercent = s.cpu
		}
		usage.TotalWorkingSetBytes += p.WorkingSetBytes
		usage.TotalCPUPercent += p.CPUPercent
		usage.Profiles = append(usage.Profiles, p)
	}
	usage.TotalWorkingSetBytes += usage.AppWorkingSetBytes
	usage.TotalCPUPercent += usage.AppCPUPercent
	return usage
}

func normalizePath(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "/", `\`)
	return s
}
