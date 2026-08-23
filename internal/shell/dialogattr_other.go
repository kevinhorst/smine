//go:build !windows

package shell

import "os/exec"

// dialogAttr is a no-op outside Windows — console suppression is a Windows
// concept; see dialogattr_windows.go.
func dialogAttr(cmd *exec.Cmd) {}
