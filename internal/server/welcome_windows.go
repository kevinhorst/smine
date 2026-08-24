//go:build windows

package server

import (
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	fixInstallClaude = "install Claude Desktop or the claude CLI, then log off and back on (or rerun configserver.exe -install) so the PATH entry is picked up"
	fixInstallToken  = "run `claude setup-token` in Git Bash, then save the token via the Routines page Configure widget — or write it to %USERPROFILE%\\.config\\claude-routine\\token"
	fixLoadRoutine   = "rerun configserver.exe -install (elevated once) — it registers the logon task; then restart the config server"
	fixPeekFragment  = "rerun configserver.exe -install — it deploys the peek-mcp fragment into %USERPROFILE%\\.claude.json"
	fixPeekReachable = "check peek-mcp.exe in %LOCALAPPDATA%\\smine\\bin, then restart the config server — it spawns peek itself"
	fixSyncAssets    = "rerun configserver.exe -install (or install.ps1 from the repo root)"
	verifyPathPrefix = ""
)

func claudeCandidates() []string {
	binDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "smine", "bin")
	candidates := []string{
		filepath.Join(binDir, "claude"),
		filepath.Join(binDir, "claude.exe"),
	}
	return candidates
}

func platformDepChecks() []setupCheck {
	return nil
}

func verifyBashPath() string {
	return shell.BashPath()
}
