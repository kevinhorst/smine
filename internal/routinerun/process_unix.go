//go:build !windows

package routinerun

import (
	"errors"
	"log"
	"os/exec"
	"syscall"
)

// containTree is a no-op on unix: cells are process groups the runner signals itself.
func containTree() func() {
	return func() {}
}

// redirectRunLogs is a no-op on unix: launchd writes StandardOutPath and StandardErrorPath itself.
func redirectRunLogs(label string) func() {
	return func() {}
}

// isolateGroup lets a deadline or the server's Kill op signal the whole cell tree.
func isolateGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(pid int) {
	signalGroup(pid, syscall.SIGKILL)
}

// signalGroup signals a cell's process group; a vanished group is fine.
func signalGroup(pid int, signal syscall.Signal) {
	err := syscall.Kill(-pid, signal)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		log.Printf("signalGroup: Group %d: Failed to send %v: %v", pid, signal, err)
	}
}

func terminateGroup(pid int) {
	signalGroup(pid, syscall.SIGTERM)
}
