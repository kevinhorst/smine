//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW keeps console-subsystem children from popping windows under a GUI-subsystem parent.
const createNoWindow = 0x08000000

// HideWindow matters because the server and the routine runner are built -H=windowsgui.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
