//go:build !windows

package shell

import "os/exec"

func HideWindow(cmd *exec.Cmd) {}
