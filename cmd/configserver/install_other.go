//go:build !windows

package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/routines"
)

// ensureCredential creates the access credential once and prints the token.
func ensureCredential(credentialPath string) error {
	if _, err := os.Stat(credentialPath); err == nil {
		return nil
	}
	return runNewCredential(credentialPath, os.Stdout)
}

// finishInstall always restarts on darwin: the installer rebuilt bin/configserver and bin/heartbeat, so a loaded agent still serves the old binary.
func finishInstall(ctx context.Context, env install.Env, record install.Record, request *installRequest, results []install.Result) error {
	if env.GOOS != "darwin" {
		return restartServer(ctx, env, results)
	}
	if install.Selects(record, env.GOOS, components.Heartbeat) {
		if err := install.RestartAgent(ctx, install.HeartbeatAgentLabel, install.HeartbeatAgentPlist(env)); err != nil {
			return err
		}
	}
	return install.RestartAgent(ctx, install.ServerAgentLabel, install.ServerAgentPlist(env))
}

// prepareInstall: nothing precedes the engine outside Windows.
func prepareInstall(ctx context.Context, repoDir string, record install.Record, request *installRequest) (int, bool) {
	return 0, false
}

// restartServer serves -apply, which builds nothing: a changed or unloaded LaunchAgent restarts, Linux has none (concept D16).
func restartServer(ctx context.Context, env install.Env, results []install.Result) error {
	if env.GOOS != "darwin" {
		fmt.Printf("no autostart on %s — start the server with: ./bin/configserver -addr 127.0.0.1:%s\n", env.GOOS, env.Values.ConfigServerPort)
		return nil
	}

	isRestartDue := slices.ContainsFunc(results, func(result install.Result) bool {
		return result.Action == install.ActionRestartRequired
	})
	isLoaded, err := routines.IsLoaded(ctx, install.ServerAgentLabel)
	isRunning := err == nil && isLoaded
	if !isRestartDue && isRunning {
		return nil
	}
	return install.RestartAgent(ctx, install.ServerAgentLabel, install.ServerAgentPlist(env))
}
