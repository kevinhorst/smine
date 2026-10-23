//go:build !darwin

package server

// restartServerAgent is a no-op: only darwin's LaunchAgent yields restart-required, the Windows logon task and Linux manual start need none.
func restartServerAgent(repoDir string) error {
	return nil
}
