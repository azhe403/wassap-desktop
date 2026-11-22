//go:build windows

// Package procstats reads per-process resource usage on Windows without
// spawning anything: Toolhelp32 for the process list, NtQueryInformationProcess
// plus PEB reads for the command line (used to attribute WebView2 processes to
// the account whose user data folder they serve), GetProcessTimes for CPU and
// psapi for the working set. PEB offsets are the x64/arm64 layout; a wrong
// read simply yields an empty command line, never a crash.
package procstats

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Proc is one live process with everything the app needs for usage
// accounting and per-account attribution.
type Proc struct {
	PID             uint32
	Name            string // exe file name, lower case
	CommandLine     string
	WorkingSetBytes uint64
	CPUTicks        uint64 // kernel + user time, in 100-nanosecond ticks
}

var (
	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	ntdll                         = windows.NewLazySystemDLL("ntdll.dll")
	psapi                         = windows.NewLazySystemDLL("psapi.dll")
	procGlobalMemoryStatusEx      = kernel32.NewProc("GlobalMemoryStatusEx")
	procNtQueryInformationProcess = ntdll.NewProc("NtQueryInformationProcess")
	procGetProcessMemoryInfo      = psapi.NewProc("GetProcessMemoryInfo")
	procEmptyWorkingSet           = psapi.NewProc("EmptyWorkingSet")
)

const (
	processBasicInformationClass = 0
	processQueryLimited          = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	processVMRead                = 0x0010 // PROCESS_VM_READ

	// x64/arm64 PEB layout: PEB.ProcessParameters, then
	// RTL_USER_PROCESS_PARAMETERS.CommandLine (a UNICODE_STRING whose buffer
	// pointer sits at +0x08 inside the string).
	pebProcessParametersOffset         = 0x20
	processParametersCommandLineOffset = 0x70
)

type processBasicInformation struct {
	reserved1       uintptr
	pebBaseAddress  uintptr
	reserved2       [2]uintptr
	uniqueProcessID uintptr
	reserved3       uintptr
}

type unicodeString struct {
	Length        uint16
	MaximumLength uint16
	_             [4]byte
	Buffer        uintptr
}

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

type memoryStatusEx struct {
	cbSize               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// Snapshot lists every live WebView2 process and our own executable. It skips
// anything it cannot open — access denied on a system process is normal.
func Snapshot() ([]Proc, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := windows.Process32First(snap, &e); err != nil {
		return nil, err
	}

	out := make([]Proc, 0, 32)
	for {
		name := strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))
		if name == "msedgewebview2.exe" || name == selfExeName() {
			if s, ok := probe(e.ProcessID, name); ok {
				out = append(out, s)
			}
		}
		err = windows.Process32Next(snap, &e)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SelfStats returns the PID, working set and CPU ticks of this process.
// The pseudo-handle works for both queries.
func SelfStats() (pid uint32, workingSetBytes uint64, cpuTicks uint64) {
	h, err := windows.GetCurrentProcess()
	if err != nil {
		return windows.GetCurrentProcessId(), 0, 0
	}
	return windows.GetCurrentProcessId(), readWorkingSet(h), readCPUTicks(h)
}

func selfExeName() string {
	p, err := os.Executable()
	if err != nil {
		return "wassap-desktop.exe"
	}
	return strings.ToLower(filepath.Base(p))
}

func probe(pid uint32, name string) (Proc, bool) {
	h, err := windows.OpenProcess(processQueryLimited|processVMRead, false, pid)
	if err != nil {
		// PEB read not possible; still report usage with a reduced handle.
		h, err = windows.OpenProcess(processQueryLimited, false, pid)
		if err != nil {
			return Proc{}, false
		}
		defer windows.CloseHandle(h)
		return Proc{
			PID:             pid,
			Name:            name,
			WorkingSetBytes: readWorkingSet(h),
			CPUTicks:        readCPUTicks(h),
		}, true
	}
	defer windows.CloseHandle(h)
	return Proc{
		PID:             pid,
		Name:            name,
		CommandLine:     readCommandLine(h),
		WorkingSetBytes: readWorkingSet(h),
		CPUTicks:        readCPUTicks(h),
	}, true
}

func readCommandLine(h windows.Handle) string {
	var pbi processBasicInformation
	var retLen uint32
	r, _, _ := procNtQueryInformationProcess.Call(
		uintptr(h),
		processBasicInformationClass,
		uintptr(unsafe.Pointer(&pbi)),
		unsafe.Sizeof(pbi),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if r != 0 || pbi.pebBaseAddress == 0 {
		return ""
	}

	var params uintptr
	if err := windows.ReadProcessMemory(h, pbi.pebBaseAddress+pebProcessParametersOffset, (*byte)(unsafe.Pointer(&params)), unsafe.Sizeof(params), nil); err != nil || params == 0 {
		return ""
	}

	var us unicodeString
	if err := windows.ReadProcessMemory(h, params+processParametersCommandLineOffset, (*byte)(unsafe.Pointer(&us)), unsafe.Sizeof(us), nil); err != nil || us.Length == 0 || us.Buffer == 0 {
		return ""
	}

	buf := make([]uint16, int(us.Length)/2)
	var read uintptr
	if err := windows.ReadProcessMemory(h, us.Buffer, (*byte)(unsafe.Pointer(&buf[0])), uintptr(us.Length), &read); err != nil {
		return ""
	}
	return strings.TrimRight(windows.UTF16ToString(buf), "\x00")
}

func readCPUTicks(h windows.Handle) uint64 {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return 0
	}
	kernel64 := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	user64 := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	return kernel64 + user64
}

func readWorkingSet(h windows.Handle) uint64 {
	var pmc processMemoryCounters
	pmc.cb = uint32(unsafe.Sizeof(pmc))
	r, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&pmc)), uintptr(pmc.cb))
	if r == 0 {
		return 0
	}
	return uint64(pmc.workingSetSize)
}

// TrimProcessWorkingSet empties as many pages as possible from the process working set.
func TrimProcessWorkingSet(pid uint32) error {
	const (
		processSetQuota          = 0x0100
		processQueryInformation  = 0x0400
	)
	h, err := windows.OpenProcess(processSetQuota|processQueryInformation, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	r, _, err := procEmptyWorkingSet.Call(uintptr(h))
	if r == 0 {
		return err
	}
	return nil
}

// TrimUserDataWorkingSet finds all webview processes attributed to the given user data directory
// and trims their working sets. Returns the number of processes trimmed.
func TrimUserDataWorkingSet(userDataDir string) (int, error) {
	if userDataDir == "" {
		return 0, nil
	}
	normTarget := strings.ToLower(filepath.Clean(userDataDir))
	snap, err := Snapshot()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, p := range snap {
		if p.Name != "msedgewebview2.exe" {
			continue
		}
		cmd := strings.ToLower(filepath.Clean(p.CommandLine))
		if strings.Contains(cmd, normTarget) {
			if err := TrimProcessWorkingSet(p.PID); err == nil {
				count++
			}
		}
	}
	return count, nil
}

// SystemMemoryLoad returns the current percentage of physical memory in use (0-100).
func SystemMemoryLoad() (uint32, error) {
	var ms memoryStatusEx
	ms.cbSize = uint32(unsafe.Sizeof(ms))
	r, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r == 0 {
		return 0, err
	}
	return ms.memoryLoad, nil
}


