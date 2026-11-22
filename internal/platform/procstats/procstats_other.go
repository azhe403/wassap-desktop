//go:build !windows

package procstats

// Snapshot lists nothing on platforms without process accounting.
type Proc struct {
	PID             uint32
	Name            string
	CommandLine     string
	WorkingSetBytes uint64
	CPUTicks        uint64
}

func Snapshot() ([]Proc, error) { return nil, nil }

func SelfStats() (pid uint32, workingSetBytes uint64, cpuTicks uint64) { return 0, 0, 0 }

func TrimProcessWorkingSet(pid uint32) error { return nil }

func TrimUserDataWorkingSet(userDataDir string) (int, error) { return 0, nil }

