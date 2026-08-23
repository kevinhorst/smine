//go:build windows

package shell

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW: the child gets no console at all — combined with
// HideWindow this keeps console-subsystem children (powershell, git, bash)
// from popping windows when the parent is a GUI-subsystem binary.
const createNoWindow = 0x08000000

// HideWindow marks cmd to run without a console window. The detached server
// and routinewrap are built -H=windowsgui; without this every child spawn
// would open a fresh console window on the desktop.
func HideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
