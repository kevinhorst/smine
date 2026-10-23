//go:build !darwin && !windows

package main

import "context"

// play is silent off darwin and windows: no player is assumed on a linux host.
func play(ctx context.Context, path string) error {
	return nil
}
