//go:build darwin

package main

import (
	"context"
	"fmt"
	"os/exec"
)

func play(ctx context.Context, path string) error {
	if err := exec.CommandContext(ctx, afplayBinary, path).Run(); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	return nil
}
