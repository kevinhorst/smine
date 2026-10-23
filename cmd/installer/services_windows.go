//go:build windows

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/trash"
)

// serverTaskName is the config server's logon task under \smine\.
const serverTaskName = "configserver"

// finishServices: configserver -install verifies the logon task serves.
func finishServices(ctx context.Context, out io.Writer, port, repoDir string) error {
	return nil
}

// prepareServices stops the running server, which file-locks bin\configserver.exe during the build (install.ps1 step 3).
func prepareServices(ctx context.Context, out io.Writer, port, repoDir string, ask func(question string) bool, store *trash.Store) error {
	if output, err := routines.StopTask(ctx, serverTaskName); err != nil {
		return fmt.Errorf("prepareServices: %w\n%s", err, output)
	}
	return nil
}
