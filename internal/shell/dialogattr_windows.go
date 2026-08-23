//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// dialogAttr suppresses only the console window (CREATE_NO_WINDOW) — unlike
// HideWindow it never sets STARTF_USESHOWWINDOW/SW_HIDE, so the child's GUI
// dialog shows normally.
func dialogAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
