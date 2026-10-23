//go:build !darwin && !windows

package main

import (
	"context"
	"io"

	"github.com/kevinhorst/smine/internal/trash"
)

// finishServices does nothing: Linux has no autostart and configserver -install prints the start command (concept D16).
func finishServices(ctx context.Context, out io.Writer, port, repoDir string) error {
	return nil
}

func prepareServices(ctx context.Context, out io.Writer, port, repoDir string, ask func(question string) bool, store *trash.Store) error {
	return nil
}
