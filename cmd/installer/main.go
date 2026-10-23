// Command installer is the smine source installer for macOS, Linux and Windows; it hands over to configserver -install (master C6).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/trash"
)

// Port defaults, the integer form of config.DefaultHeartbeatPort, config.DefaultPeekPort and config.DefaultPeekControlPort.
const (
	defaultHeartbeatPort   = 6101
	defaultPeekControlPort = 42442
	defaultPeekPort        = 4242
)

// errDeclined is the answer No at the confirmation, errInterrupted a Ctrl-C during the dialogue: nothing was written either way.
var (
	errDeclined    = errors.New("declined")
	errInterrupted = errors.New("interrupted")
)

// options.explicit names the flags the caller set, so only those reach configserver -install and a re-run keeps stored ports.
type options struct {
	addr                string
	componentsArg       string
	explicit            map[string]bool
	heartbeatPort       int
	initWelcome         bool
	isPreview           bool
	isResetSkills       bool
	isYes               bool
	peekControlPort     int
	peekPort            int
	presentationProfile string
	skillsArg           string
}

func main() {
	opts := &options{explicit: map[string]bool{}}
	flags := flag.NewFlagSet("installer", flag.ExitOnError)
	flags.StringVar(&opts.componentsArg, "components", "", "component ids, comma-separated, or minimal | full")
	flags.StringVar(&opts.skillsArg, "skills", "", "all | none | chosen:<leaf>,<leaf>")
	flags.BoolVar(&opts.isYes, "yes", false, "no prompts; missing flags take the stored record, else Minimal")
	flags.BoolVar(&opts.isPreview, "preview", false, "print the plan, write nothing")
	flags.BoolVar(&opts.isResetSkills, "reset-skills", false, "forget the confirmed skill selection: ask the skill question again (with -yes: only clear the confirmation)")
	flags.StringVar(&opts.presentationProfile, "presentation-profile", os.Getenv("SMINE_PRESENTATION_PROFILE"), "presentation profile id installed into the global context dir")
	flags.StringVar(&opts.addr, "addr", "127.0.0.1:6001", "config server listen address")
	flags.IntVar(&opts.peekPort, "peek-port", defaultPeekPort, "peek MCP port")
	flags.IntVar(&opts.peekControlPort, "peek-control-port", defaultPeekControlPort, "peek control port")
	flags.IntVar(&opts.heartbeatPort, "heartbeat-port", defaultHeartbeatPort, "heartbeat daemon port")
	flags.BoolVar(&opts.initWelcome, "init-welcome", false, "always show the Welcome nav entry")
	_ = flags.Parse(os.Args[1:])
	flags.Visit(func(set *flag.Flag) { opts.explicit[set.Name] = true })

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, opts, newPrompter(ctx.Done(), os.Stdin, os.Stdout))
	cancel()
	os.Exit(code)
}

func run(ctx context.Context, opts *options, prompter *prompter) int {
	err := runInstallation(ctx, opts, prompter)
	if errors.Is(err, errDeclined) {
		fmt.Fprintln(prompter.out, "nothing written")
		return 0
	}
	if errors.Is(err, errInterrupted) {
		fmt.Fprintln(prompter.out, "\ninterrupted — nothing written")
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "installer: %v\n", err)
		return 1
	}

	return 0
}

// installRoot is the extracted release archive holding this binary as bin/installer, else the working directory of a go run.
func installRoot() (string, error) {
	executable, err := os.Executable()
	if err == nil {
		root := filepath.Dir(filepath.Dir(executable))
		if install.HasPayload(root) {
			return root, nil
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("installRoot: %w", err)
	}
	return dir, nil
}

// runInstallation follows concept Flow 1; nothing is written before the preview is confirmed.
func runInstallation(ctx context.Context, opts *options, prompter *prompter) error {
	repoDir, err := installRoot()
	if err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	env, err := install.NewEnv(repoDir, io.Discard)
	if err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}
	if env.IsPayload {
		if err := clearQuarantine(prompter.out, repoDir); err != nil {
			return fmt.Errorf("runInstallation: %w", err)
		}
	}

	previewCommand := goRunConfigserver
	if env.IsPayload {
		previewCommand = builtConfigserver(env.GOOS, repoDir)
	}

	stored, isFound, err := install.Load(env)
	if err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	result, err := choose(prompter, opts, stored, isFound, env.GOOS, repoDir)
	if err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}
	if ctx.Err() != nil {
		return errInterrupted
	}

	args := installArgs(opts, result)
	previewArgs := append(slices.Clone(args), "-preview")
	if opts.isPreview {
		return runConfigserver(ctx, previewArgs, previewCommand, prompter.out, repoDir)
	}

	if err := checkPrerequisites(ctx, env.IsPayload, opts, prompter, result.ids); err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	if !opts.isYes {
		if err := runConfigserver(ctx, previewArgs, previewCommand, prompter.out, repoDir); err != nil {
			return fmt.Errorf("runInstallation: %w", err)
		}

		if !prompter.confirm("Apply these steps?") {
			return errDeclined
		}
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	ask := func(question string) bool { return opts.isYes || prompter.confirm(question) }
	port := serverPort(opts, env)
	if err := prepareCheckout(ctx, env.GOOS, env.IsPayload, prompter.out, port, repoDir, result.ids, ask, trash.New(stateDir, 0)); err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	if err := runConfigserver(ctx, args, builtConfigserver(env.GOOS, repoDir), prompter.out, repoDir); err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	if err := finishServices(ctx, prompter.out, port, repoDir); err != nil {
		return fmt.Errorf("runInstallation: %w", err)
	}

	fmt.Fprintf(prompter.out, "-> Open http://127.0.0.1:%s/welcome — the browser asks for the access token as the password\n", port)
	return nil
}
