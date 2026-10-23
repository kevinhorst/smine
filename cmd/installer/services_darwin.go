//go:build darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	serverBinary  = "configserver"
	serveAttempts = 10
	serveInterval = 500 * time.Millisecond
	stopAttempts  = 50
	stopInterval  = 100 * time.Millisecond
)

// smineLabelPrefix is the ownership proof of a LaunchAgent: every agent smine writes carries it.
const smineLabelPrefix = "com.smine."

// lookupListener, signalProcess and stopAgent are the process seams; tests swap them so no real process or agent is touched.
var (
	lookupListener = listenerCommand
	signalProcess  = syscall.Kill
	stopAgent      = routines.Stop
)

// finishServices checks that this checkout's binary serves the port, since a foreign or stale holder makes every later symptom old state.
func finishServices(ctx context.Context, out io.Writer, port, repoDir string) error {
	expected := filepath.Join(repoDir, "bin", serverBinary)
	for range serveAttempts {
		time.Sleep(serveInterval)
		if _, command := listenerCommand(ctx, port); command == expected {
			fmt.Fprintf(out, "configserver LaunchAgent serving :%s from %s (logs: ~/Library/Logs/smine-configserver.*.log)\n", port, expected)
			return nil
		}
	}
	return fmt.Errorf("finishServices: :%s is not served by %s — check ~/Library/Logs/smine-configserver.err.log", port, expected)
}

// listenerCommand returns zero and empty without a listener, which lsof reports as exit 1.
func listenerCommand(ctx context.Context, port string) (int, string) {
	output, err := shell.Run(ctx, "", "lsof", "-tiTCP:"+port, "-sTCP:LISTEN")
	if err != nil {
		return 0, ""
	}

	first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	pid, err := strconv.Atoi(first)
	if err != nil {
		return 0, ""
	}

	command, err := shell.Run(ctx, "", "ps", "-o", "comm=", "-p", strconv.Itoa(pid))
	if err != nil {
		return pid, ""
	}
	return pid, strings.TrimSpace(command)
}

// prepareServices stashes smine's own legacy configserver LaunchAgents under another label and asks before stopping another checkout's server on the port (master C16, C17).
func prepareServices(ctx context.Context, out io.Writer, port, repoDir string, ask func(question string) bool, store *trash.Store) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("prepareServices: %w", err)
	}

	plists, err := filepath.Glob(filepath.Join(home, "Library", "LaunchAgents", "*.plist"))
	if err != nil {
		return fmt.Errorf("prepareServices: %w", err)
	}
	for _, plist := range plists {
		if err := removeLegacyAgent(ctx, out, plist, store); err != nil {
			return fmt.Errorf("prepareServices: %w", err)
		}
	}
	return stopForeignListener(ctx, out, port, repoDir, ask)
}

func removeLegacyAgent(ctx context.Context, out io.Writer, plist string, store *trash.Store) error {
	label := strings.TrimSuffix(filepath.Base(plist), ".plist")
	if label == install.ServerAgentLabel || label == install.HeartbeatAgentLabel {
		return nil
	}
	data, err := os.ReadFile(plist)
	if err != nil || !bytes.Contains(data, []byte("bin/configserver")) {
		return nil
	}
	if !strings.HasPrefix(label, smineLabelPrefix) {
		fmt.Fprintf(out, "-> Leaving %s: it runs a configserver but is not a smine agent (no %s prefix)\n", label, smineLabelPrefix)
		return nil
	}

	fmt.Fprintf(out, "-> Stashing legacy configserver agent %s ...\n", label)
	_, _ = stopAgent(ctx, label)
	entry := &trash.Entry{Action: "installer", Reason: "legacy configserver agent " + label}
	if _, err := store.Move(plist, entry); err != nil {
		return fmt.Errorf("removeLegacyAgent: %w", err)
	}
	return nil
}

// stopForeignListener refuses a port holder that is not a configserver and asks before stopping another checkout's; this checkout's own is left to finishServices.
func stopForeignListener(ctx context.Context, out io.Writer, port, repoDir string, ask func(question string) bool) error {
	pid, command := lookupListener(ctx, port)
	if pid == 0 || command == filepath.Join(repoDir, "bin", serverBinary) {
		return nil
	}
	if filepath.Base(command) != serverBinary {
		return fmt.Errorf("stopForeignListener: Port %s is held by %s (pid %d), not a configserver — stop it yourself or pick another port", port, command, pid)
	}
	if !ask(fmt.Sprintf("Port %s is held by another checkout's configserver (%s, pid %d). Stop it?", port, command, pid)) {
		return fmt.Errorf("stopForeignListener: Port %s stays with %s (pid %d) — declined", port, command, pid)
	}

	fmt.Fprintf(out, "-> Stopping %s on :%s (pid %d) ...\n", command, port, pid)
	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("stopForeignListener: %w", err)
	}
	for range stopAttempts {
		if signalProcess(pid, 0) != nil {
			return nil
		}
		time.Sleep(stopInterval)
	}
	return fmt.Errorf("stopForeignListener: pid %d still holds :%s", pid, port)
}
