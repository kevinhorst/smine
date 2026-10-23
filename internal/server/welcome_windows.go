//go:build windows

package server

import (
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	fixInstallToken  = "run `claude setup-token` in Git Bash, then add the token on the Profile page (Account tokens) — or write it to %USERPROFILE%\\.config\\claude-routine\\token"
	verifyPathPrefix = ""
)

func verifyBashPath() string {
	return shell.BashPath()
}
