//go:build !darwin

package main

import "io"

// clearQuarantine is macOS Gatekeeper's concern alone.
func clearQuarantine(out io.Writer, repoDir string) error {
	return nil
}
