//go:build !windows

package shell

import "os/exec"

func dialogAttr(cmd *exec.Cmd) {}
