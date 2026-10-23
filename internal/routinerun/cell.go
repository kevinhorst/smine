package routinerun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kevinhorst/smine/internal/routines"
)

// killGrace is the TERM-to-KILL escalation window, cmd.WaitDelay after the deadline's cancel.
const killGrace = 30 * time.Second

const (
	headlessEnv    = headlessEnvKey + "=1"
	headlessEnvKey = "SMINE_HEADLESS"
)

type CellSpec struct {
	Args     []string
	CellPids string // empty = unregistered
	Dir      string
	Stderr   io.Writer // nil = os.Stderr
	Stdout   io.Writer
	Timeout  time.Duration
}

func cellStatus(ctx context.Context, cellErr, waitErr error) int {
	switch {
	case ctx.Err() != nil:
		return exitCancelled
	case cellErr != nil:
		return exitDeadline
	case waitErr == nil:
		return exitOk
	}
	return exitCode(waitErr)
}

func exitCode(waitErr error) int {
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) && exitErr.ExitCode() >= 0 {
		return exitErr.ExitCode()
	}
	return exitSoftware
}

// registerCell feeds the registry the server's Kill op and the runner's cancellation read.
func registerCell(path string, pid int) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("registerCell: %s: %v", path, err)
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("registerCell: %s: Failed to open: %v", path, err)
		return
	}
	defer file.Close()

	if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
		log.Printf("registerCell: %s: Failed to write: %v", path, err)
	}
}

// terminateRegistered also reaches cells nested under a shell pipeline.
func terminateRegistered(path string) {
	for _, pid := range routines.ReadCellPids(path) {
		terminateGroup(pid)
	}
}

// RunCell's error covers start failures only; the status is 124 on deadline, 143 on cancel.
func RunCell(ctx context.Context, spec *CellSpec) (int, error) {
	cellCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cellCtx, spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = append(os.Environ(), headlessEnv)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	if spec.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	isolateGroup(cmd)
	cmd.Cancel = func() error {
		terminateGroup(cmd.Process.Pid)
		return nil
	}
	cmd.WaitDelay = killGrace
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("RunCell: Failed to start %s: %w", spec.Args[0], err)
	}
	registerCell(spec.CellPids, cmd.Process.Pid)

	waitErr := cmd.Wait()
	if cellCtx.Err() != nil {
		killGroup(cmd.Process.Pid)
	}
	return cellStatus(ctx, cellCtx.Err(), waitErr), nil
}
