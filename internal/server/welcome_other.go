//go:build !darwin && !windows

package server

const (
	fixInstallClaude = "install the claude CLI and restart the config server"
	fixInstallToken  = "run `claude setup-token`, then save the token via the Routines page Configure widget — or write it to ~/.config/claude-routine/token (0600)"
	fixLoadRoutine   = "restart the config server — it bootstraps enabled routines at startup"
	fixPeekFragment  = "run cmd/sync/sync_settings.sh — it deploys the peek-mcp fragment into ~/.claude.json"
	fixPeekReachable = "install peek-mcp, then restart the config server — it spawns peek itself"
	fixSyncAssets    = "run the cmd/sync scripts from the repo root"
	verifyPathPrefix = ""
)

func claudeCandidates() []string {
	return nil
}

func platformDepChecks() []setupCheck {
	return nil
}

func verifyBashPath() string {
	return "bash"
}
