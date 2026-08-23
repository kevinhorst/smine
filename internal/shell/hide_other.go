//go:build !windows

package shell

import "os/exec"

// HideWindow is a no-op outside Windows — console windows are a Windows
// concept; see hide_windows.go.
func HideWindow(cmd *exec.Cmd) {}
