//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// dialogAttr never sets SW_HIDE, unlike HideWindow, so the child's GUI dialog shows.
func dialogAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
