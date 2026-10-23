//go:build windows

package routines

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// terminatedExitStatus is the SIGTERM exit (128+15) the results rows record for a killed run.
const terminatedExitStatus = 143

var (
	ntdll            = windows.NewLazySystemDLL("ntdll.dll")
	ntSuspendProcess = ntdll.NewProc("NtSuspendProcess")
	ntResumeProcess  = ntdll.NewProc("NtResumeProcess")
)

// pid is a Windows process id as the kernel types it; the package's int pids convert once, at the exported verbs.
type pid = uint32

// descendantsOf lists root first, then every process under it, depth-first through the parent map.
func descendantsOf(children map[pid][]pid, root pid) []pid {
	return walkTree(children, root, map[pid]bool{})
}

// walkTree visits each pid once: Windows reuses the pid of a dead parent, so a snapshot's parent map can loop.
func walkTree(children map[pid][]pid, id pid, seen map[pid]bool) []pid {
	if seen[id] {
		return nil
	}
	seen[id] = true

	tree := []pid{id}
	for _, child := range children[id] {
		tree = append(tree, walkTree(children, child, seen)...)
	}
	return tree
}

// eachProcess visits every entry of the snapshot; the end of the list is not an error.
func eachProcess(snapshot windows.Handle, visit func(entry *windows.ProcessEntry32)) error {
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err := windows.Process32First(snapshot, &entry)
	for err == nil {
		visit(&entry)
		err = windows.Process32Next(snapshot, &entry)
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil
	}
	return fmt.Errorf("eachProcess: %w", err)
}

// processTree is the live tree under rootPid, rootPid included, from one Toolhelp snapshot.
func processTree(rootPid int) ([]pid, error) {
	children, err := snapshotChildren()
	if err != nil {
		return nil, fmt.Errorf("processTree: %w", err)
	}
	return descendantsOf(children, pid(rootPid)), nil
}

// signalProcesses applies an ntdll routine (NTSTATUS 0 = success) to every process it can open with access; it returns how many succeeded.
func signalProcesses(access uint32, pids []pid, routine *windows.LazyProc) int {
	signalled := 0
	for _, id := range pids {
		handle, err := windows.OpenProcess(access, false, id)
		if err != nil {
			continue
		}

		status, _, _ := routine.Call(uintptr(handle))
		if status == 0 {
			signalled++
		}
		_ = windows.CloseHandle(handle)
	}
	return signalled
}

// snapshotChildren maps every parent pid to its children as the kernel lists them now.
func snapshotChildren() (map[pid][]pid, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshotChildren: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	children := map[pid][]pid{}
	err = eachProcess(snapshot, func(entry *windows.ProcessEntry32) {
		parent, child := entry.ParentProcessID, entry.ProcessID
		children[parent] = append(children[parent], child)
	})
	if err != nil {
		return nil, fmt.Errorf("snapshotChildren: %w", err)
	}
	return children, nil
}

// terminateProcesses ends every process it can open with the SIGTERM exit status; it returns how many ended.
func terminateProcesses(pids []pid) int {
	ended := 0
	for _, id := range pids {
		handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, id)
		if err != nil {
			continue
		}

		if windows.TerminateProcess(handle, terminatedExitStatus) == nil {
			ended++
		}
		_ = windows.CloseHandle(handle)
	}
	return ended
}

// ResumeTree is the SIGCONT of Windows: NtResumeProcess over rootPid's tree; the count is what resumed.
func ResumeTree(rootPid int) (int, error) {
	tree, err := processTree(rootPid)
	if err != nil {
		return 0, fmt.Errorf("ResumeTree: %w", err)
	}
	return signalProcesses(windows.PROCESS_SUSPEND_RESUME, tree, ntResumeProcess), nil
}

// SuspendTree is the SIGSTOP of Windows: NtSuspendProcess over rootPid's tree; the count is what suspended.
func SuspendTree(rootPid int) (int, error) {
	tree, err := processTree(rootPid)
	if err != nil {
		return 0, fmt.Errorf("SuspendTree: %w", err)
	}
	return signalProcesses(windows.PROCESS_SUSPEND_RESUME, tree, ntSuspendProcess), nil
}

// TerminateTree ends rootPid's tree; the count is what TerminateProcess ended.
func TerminateTree(rootPid int) (int, error) {
	tree, err := processTree(rootPid)
	if err != nil {
		return 0, fmt.Errorf("TerminateTree: %w", err)
	}
	return terminateProcesses(tree), nil
}
