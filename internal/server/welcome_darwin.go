//go:build darwin

package server

const (
	fixInstallToken = "run `claude setup-token`, then add the token on the Profile page (Account tokens) — or write it to ~/.config/claude-routine/token (0600)"
	// Mirrors run.sh's homebrew PATH prefix, which the launchd server PATH lacks.
	verifyPathPrefix = `export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"; `
)

func verifyBashPath() string {
	return "bash"
}
