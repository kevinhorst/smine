package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	goosWindows = "windows"
	// installTimeout bounds one configserver run including the peek download, skill sync and service restarts.
	installTimeout = 30 * time.Minute
	// tokenDirPerm and tokenFilePerm keep the headless token owner-only.
	tokenDirPerm  = 0o700
	tokenFilePerm = 0o600
)

var (
	// goBinaries are built by every install: the server, the scheduler-run routine runner, and the acdsl and rules CLIs hooks call.
	goBinaries = []string{"configserver", "routine", "acdsl", "rules"}
	// guiBinaries run detached on Windows (no console window).
	guiBinaries       = []string{"configserver", "routine"}
	goRunConfigserver = []string{"go", "run", "./cmd/configserver"}
	rustBinaries      = []rustBinary{
		{component: components.Agentwt, name: "agentwt"},
		{component: components.Heartbeat, name: "heartbeat"},
	}
)

// rustBinary is a crate whose binary one component installs.
type rustBinary struct {
	component components.ID
	name      string
}

func builtConfigserver(goos, repoDir string) []string {
	command := []string{filepath.Join(repoDir, "bin", "configserver"+exeSuffix(goos))}
	return command
}

// buildBinaries builds a Rust binary only when its component is selected (concept D19).
func buildBinaries(ctx context.Context, goos string, out io.Writer, repoDir string, selected []components.ID) error {
	for _, name := range goBinaries {
		fmt.Fprintf(out, "-> Building %s ...\n", name)
		if output, err := shell.RunSync(ctx, repoDir, "go", goBuildArgs(goos, name)...); err != nil {
			return fmt.Errorf("buildBinaries: %s: %w\n%s", name, err, output)
		}
	}

	for _, rust := range rustBinaries {
		if !slices.Contains(selected, rust.component) {
			continue
		}

		if err := buildRust(ctx, goos, out, repoDir, rust.name); err != nil {
			return fmt.Errorf("buildBinaries: %w", err)
		}
	}
	return nil
}

// buildRust renames the binary into bin/ for a fresh inode, since macOS kills later execs of a signed binary rewritten under a running vnode (Makefile rust-build).
func buildRust(ctx context.Context, goos string, out io.Writer, repoDir, name string) error {
	fmt.Fprintf(out, "-> Building %s ...\n", name)
	manifest := filepath.Join("cmd", name, "Cargo.toml")
	if output, err := shell.RunSync(ctx, repoDir, "cargo", "build", "--release", "--locked", "--manifest-path", manifest); err != nil {
		return fmt.Errorf("buildRust: %s: %w\n%s", name, err, output)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "cmd", name, "target", "release", name+exeSuffix(goos)))
	if err != nil {
		return fmt.Errorf("buildRust: %w", err)
	}

	if err := fsx.WriteFileAtomic(data, filepath.Join(repoDir, "bin", name+exeSuffix(goos)), 0o755); err != nil {
		return fmt.Errorf("buildRust: %w", err)
	}
	return nil
}

// checkPrerequisites covers only what the selection needs (concept D19) and stops on a missing hard one before any write; a release archive builds nothing.
func checkPrerequisites(ctx context.Context, isPayload bool, opts *options, prompter *prompter, selected []components.ID) error {
	isRust := slices.Contains(selected, components.Agentwt) || slices.Contains(selected, components.Heartbeat)
	if isRust && !isPayload && !isOnPath("cargo") {
		return errors.New("checkPrerequisites: cargo not found — agentwt and heartbeat build with Rust: install rustup (rustup default stable), or untick them")
	}

	if slices.Contains(selected, components.Skills) && !isOnPath("jq") {
		fmt.Fprintln(prompter.out, "WARN: jq not found — the skills call it; install it before the first session")
	}

	if slices.Contains(selected, components.Mining) && !isOnPath("shellcheck") {
		fmt.Fprintln(prompter.out, "WARN: shellcheck not found — the acdsl lint gate of the mining runs needs it")
	}

	isTokenWanted := slices.Contains(selected, components.Routines) || slices.Contains(selected, components.Mining)
	if isTokenWanted && !opts.isYes {
		return offerToken(prompter)
	}
	return nil
}

func exeSuffix(goos string) string {
	if goos == goosWindows {
		return ".exe"
	}
	return ""
}

func goBuildArgs(goos, name string) []string {
	args := []string{"build"}
	if goos == goosWindows && slices.Contains(guiBinaries, name) {
		args = append(args, "-ldflags", "-H=windowsgui")
	}
	return append(args, "-o", filepath.Join("bin", name+exeSuffix(goos)), "./cmd/"+name)
}

func isOnPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// offerToken asks once when none is stored; Enter skips, and routine runs exit 78 until one exists.
func offerToken(prompter *prompter) error {
	tokenDir, err := routines.TokenDir()
	if err != nil {
		return fmt.Errorf("offerToken: %w", err)
	}

	labels, err := routines.ListTokenLabels(tokenDir)
	isLabeled := err == nil && len(labels) > 0
	if isLabeled || fsx.HasContent(routines.LegacyTokenPath(tokenDir)) {
		return nil
	}

	answer := prompter.ask("Routine OAuth token (run 'claude setup-token' to get one; Enter to skip)")
	token := strings.Join(strings.Fields(answer), "")
	if token == "" {
		return nil
	}

	if err := os.MkdirAll(tokenDir, tokenDirPerm); err != nil {
		return fmt.Errorf("offerToken: %w", err)
	}

	if err := fsx.WriteFileAtomic([]byte(token), routines.LegacyTokenPath(tokenDir), tokenFilePerm); err != nil {
		return fmt.Errorf("offerToken: %w", err)
	}

	fmt.Fprintf(prompter.out, "-> Token written: %s\n", routines.LegacyTokenPath(tokenDir))
	return nil
}

// prepareCheckout is the pre-apply half: checkout repository, service preparation, build (a release archive skips it); git setup is configserver -install's.
func prepareCheckout(ctx context.Context, goos string, isPayload bool, out io.Writer, port, repoDir string, selected []components.ID, ask func(question string) bool, store *trash.Store) error {
	if err := prepareServices(ctx, out, port, repoDir, ask, store); err != nil {
		return fmt.Errorf("prepareCheckout: %w", err)
	}

	if !isPayload {
		if err := buildBinaries(ctx, goos, out, repoDir, selected); err != nil {
			return fmt.Errorf("prepareCheckout: %w", err)
		}
	}
	return nil
}

// runConfigserver runs the preview through go run and the apply through the built binary.
func runConfigserver(ctx context.Context, args, command []string, out io.Writer, repoDir string) error {
	runCtx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	invocation := append(slices.Clone(command[1:]), args...)
	process := exec.CommandContext(runCtx, command[0], invocation...)
	process.Dir = repoDir
	process.Stdin = os.Stdin
	process.Stdout = out
	process.Stderr = os.Stderr
	if err := process.Run(); err != nil {
		return fmt.Errorf("runConfigserver: %s: %w", strings.Join(command, " "), err)
	}
	return nil
}
