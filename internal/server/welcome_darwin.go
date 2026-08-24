//go:build darwin

package server

import "os/exec"

const (
	fixInstallClaude = "install the claude CLI (native installer or npm), then restart the config server so it sees the new PATH"
	fixInstallDeps   = "brew install flock coreutils"
	fixInstallToken  = "run `claude setup-token`, then save the token via the Routines page Configure widget — or write it to ~/.config/claude-routine/token (0600)"
	fixLoadRoutine   = "restart the config server (launchctl kickstart -k gui/$UID/com.smine.configserver) — it bootstraps enabled routines at startup"
	fixPeekFragment  = "run cmd/sync/sync_settings.sh — it deploys the peek-mcp fragment into ~/.claude.json"
	fixPeekReachable = "install peek-mcp (go install github.com/kevinhorst/peek-mcp@latest), then restart the config server — it spawns peek itself"
	fixSyncAssets    = "run ./install.sh from the repo root (or the three cmd/sync scripts)"
	// verifyPathPrefix mirrors run.sh, which prepends the homebrew bin dir —
	// the launchd server PATH lacks it.
	verifyPathPrefix = `export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"; `
)

func claudeCandidates() []string {
	candidates := []string{
		"/opt/homebrew/bin/claude",
		"/usr/local/bin/claude",
	}
	return candidates
}

func platformDepChecks() []setupCheck {
	check := setupCheck{
		Id:    "wrapper-tools",
		Fix:   fixInstallDeps,
		Group: "claude runtime",
		Name:  "wrapper tools",
		Ok:    true,
	}
	missing := ""
	for _, tool := range []string{"flock", "timeout"} {
		if _, err := exec.LookPath(tool); err != nil {
			missing += tool + " "
		}
	}
	if missing != "" {
		check.Detail = missing + "not found — the routine wrapper needs both"
		check.Ok = false
		return []setupCheck{check}
	}

	check.Detail = "flock and timeout available"
	return []setupCheck{check}
}

func verifyBashPath() string {
	return "bash"
}
