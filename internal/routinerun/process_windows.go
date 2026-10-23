package routinerun

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/kevinhorst/smine/internal/routines"
)

const createNoWindow = 0x08000000

// containTree uses a kill-on-close job so every descendant dies with the run.
func containTree() func() {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		log.Printf("containTree: Failed to create job: %v", err)
		return func() {}
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(job)
		log.Printf("containTree: Failed to set job limits: %v", err)
		return func() {}
	}

	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		windows.CloseHandle(job)
		log.Printf("containTree: Failed to join job: %v", err)
		return func() {}
	}
	return func() { windows.CloseHandle(job) }
}

func isolateGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | createNoWindow,
		HideWindow:    true,
	}
}

func killGroup(pid int) {
	if _, err := routines.TerminateTree(pid); err != nil {
		killProcess(pid)
	}
}

func killProcess(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = process.Kill()
}

// redirectRunLogs exists because Task Scheduler captures no output itself.
func redirectRunLogs(label string) func() {
	logDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-routine", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		log.Printf("redirectRunLogs: %v", err)
		return func() {}
	}

	stdout, err := os.Create(filepath.Join(logDir, label+".out.log"))
	if err != nil {
		log.Printf("redirectRunLogs: %v", err)
		return func() {}
	}
	stderr, err := os.Create(filepath.Join(logDir, label+".err.log"))
	if err != nil {
		stdout.Close()
		log.Printf("redirectRunLogs: %v", err)
		return func() {}
	}

	os.Stdout, os.Stderr = stdout, stderr
	log.SetOutput(stderr)
	return func() {
		stdout.Close()
		stderr.Close()
	}
}

func terminateGroup(pid int) {
	if _, err := routines.TerminateTree(pid); err != nil {
		killProcess(pid)
	}
}
