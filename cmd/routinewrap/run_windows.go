//go:build windows

package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
)

// backstopDefaultS is the plan-D20 default: above any stage-budget sum in
// use, still short of the next nightly window.
const backstopDefaultS = 46800

// exit codes mirror run.sh: 0 skip-if-running, 75 group-lock timeout.
func run(dir string) int {
	// Absolute-ize: the bash child gets this path as its script argument
	// while ALSO having it as cwd — a relative dir doubles up
	// (routines/x/routines/x/run.sh) and bash finds nothing.
	dir, err := filepath.Abs(dir)
	if err != nil {
		log.Printf("routinewrap: %v", err)
		return 2
	}
	label, env, err := routines.PlistMeta(dir)
	if err != nil {
		log.Printf("routinewrap: %v", err)
		return 2
	}
	for key, value := range env {
		os.Setenv(key, value)
	}
	os.Setenv("ROUTINE_LOCKS_HELD", "1")

	selfLock, ok, err := lockFile(filepath.Join(dir, ".lock"), 0)
	if err != nil || !ok {
		log.Printf("routinewrap: already running or lock failed: %v", err)
		return 0
	}
	defer selfLock.Close()

	group := env["ROUTINE_GROUP"]
	if group == "" {
		group = filepath.Base(dir)
	}
	groupLock, ok, err := lockFile(filepath.Join(worktreeRoot(env), "."+group+".lock"), 2*time.Hour)
	if err != nil || !ok {
		log.Printf("routinewrap: group lock timeout: %s (%v)", group, err)
		return 75
	}
	defer groupLock.Close()

	release := preventSleep()
	defer release()

	timeoutS := backstopDefaultS
	if value, err := strconv.Atoi(env["ROUTINE_TIMEOUT_S"]); err == nil && value > 0 {
		timeoutS = value
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()

	return runBash(ctx, dir, label)
}

// worktreeRoot mirrors routines/_lib/worktree.sh: ROUTINE_WT_ROOT or
// ~/.cache/claude-routine/worktrees.
func worktreeRoot(env map[string]string) string {
	if root := env["ROUTINE_WT_ROOT"]; root != "" {
		return root
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "claude-routine", "worktrees")
	}
	return filepath.Join(home, ".cache", "claude-routine", "worktrees")
}

// lockFile opens (creating) path and takes an exclusive LockFileEx. wait=0
// fails immediately (self-lock); wait>0 polls every 10s until the deadline
// (group lock, 2h — flock -w parity). The OS releases the lock on process
// death, so no stale-lock protocol exists. ok=false means held elsewhere.
func lockFile(path string, wait time.Duration) (*os.File, bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	deadline := time.Now().Add(wait)
	for {
		overlapped := new(windows.Overlapped)
		err = windows.LockFileEx(windows.Handle(file.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0, 1, 0, overlapped)
		if err == nil {
			return file, true, nil
		}
		if time.Now().After(deadline) {
			file.Close()
			return nil, false, nil
		}
		time.Sleep(10 * time.Second)
	}
}

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// preventSleep asserts a system wake-lock for the run — the caffeinate -is
// analog; the returned release drops it. WakeToRun in the task XML is the
// wake-from-sleep backstop.
func preventSleep() func() {
	setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired))
	return func() { setThreadExecutionState.Call(uintptr(esContinuous)) }
}

// runBash executes run.sh under Git Bash inside a job object with
// kill-on-close, so the backstop deadline (ctx) tears down the whole claude
// process tree — taskkill-style tree walks race newly spawned children; a job
// object does not. Output goes to the per-routine log files (plan D17). The
// suspended-start -> assign-to-job -> resume sequence closes the window where
// an early-spawning child escapes the job.
func runBash(ctx context.Context, dir, label string) int {
	logDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-routine", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Printf("routinewrap: logdir: %v", err)
		return 2
	}
	stdout, err := os.Create(filepath.Join(logDir, label+".out.log"))
	if err != nil {
		log.Printf("routinewrap: out log: %v", err)
		return 2
	}
	defer stdout.Close()
	stderr, err := os.Create(filepath.Join(logDir, label+".err.log"))
	if err != nil {
		log.Printf("routinewrap: err log: %v", err)
		return 2
	}
	defer stderr.Close()

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		log.Printf("routinewrap: job object: %v", err)
		return 2
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		log.Printf("routinewrap: job limits: %v", err)
		return 2
	}

	bash := shell.BashPath()
	if bash == "" {
		windows.CloseHandle(job)
		log.Printf("routinewrap: Git Bash not found (set SMINE_BASH)")
		return 2
	}
	cmd := exec.Command(bash, filepath.ToSlash(filepath.Join(dir, "run.sh")))
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = stdout, stderr
	// CREATE_NO_WINDOW: routinewrap is windowsgui-subsystem, so bash would
	// otherwise allocate a fresh console window at every routine fire.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_SUSPENDED | 0x08000000,
	}
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		log.Printf("routinewrap: start: %v", err)
		return 2
	}

	process, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
	if err == nil {
		windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	if err := resumeMainThread(uint32(cmd.Process.Pid)); err != nil {
		windows.CloseHandle(job) // kills the suspended child too
		log.Printf("routinewrap: resume: %v", err)
		return 2
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		windows.CloseHandle(job) // KILL_ON_JOB_CLOSE tears the tree down
		<-done
		log.Printf("routinewrap: backstop deadline hit for %s", label)
		return 1
	case err := <-done:
		windows.CloseHandle(job)
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		if err != nil {
			log.Printf("routinewrap: wait: %v", err)
			return 2
		}
		return 0
	}
}

// resumeMainThread resumes the suspended process's threads (a fresh process
// has exactly one) via a toolhelp snapshot.
func resumeMainThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err := windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		if err != nil {
			return err
		}
	}
	return nil
}
