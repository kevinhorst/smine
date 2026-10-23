//go:build !darwin && !windows

package server

const (
	fixInstallToken  = "run `claude setup-token`, then add the token on the Profile page (Account tokens) — or write it to ~/.config/claude-routine/token (0600)"
	verifyPathPrefix = ""
)

func verifyBashPath() string {
	return "bash"
}
