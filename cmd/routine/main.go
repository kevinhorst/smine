// Command routine runs local routines, fleet worker cells and the smine bootstrap.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/routinerun"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

// Exit statuses of the dispatcher.
const (
	exitConfig       = 78
	exitStartFailure = 127
	exitUsage        = 2
	usage            = "usage: routine run <routine-dir> | cell -timeout <s> -- <cmd...> | worker -name <name> -skill <leaf> (-base <ref> | -resume <session id>) [-args <text>] [-attended] [-keep-stopped] [-model <m>] [-effort <e>] [-budget-usd <n>] [-token <label>] [-timeout <s>] | bootstrap | allowed-tools <skill> | publish-style <staged-file> | smine-mcp-config | interfaces"
)

const defaultWorkerTimeoutS = 14400

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	code := dispatch(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func dispatch(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "allowed-tools":
		return printAllowedTools(args[1:])
	case "bootstrap":
		return runBootstrap(ctx, args[1:])
	case "cell":
		return runCell(ctx, args[1:])
	case "interfaces":
		return printInterfaces(args[1:])
	case "publish-style":
		return publishStyle(args[1:])
	case "run":
		return runRoutine(ctx, args[1:])
	case "smine-mcp-config":
		return printSmineMcpConfig(args[1:])
	case "worker":
		return runWorker(ctx, args[1:])
	default:
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}
}

func printAllowedTools(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}
	fmt.Fprint(os.Stdout, skills.AllowedTools(filepath.Join(skills.DefaultHomePath(), args[0])))
	return 0
}

func printSmineMcpConfig(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	mcpConfig, err := routinerun.SmineMcpConfig(config.DefaultClaudeJsonPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitConfig
	}
	fmt.Fprint(os.Stdout, mcpConfig)
	return 0
}

func publishStyle(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitConfig
	}

	if err := routinerun.PublishStyleProfile(args[0], trash.New(stateDir, 0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitConfig
	}
	return 0
}

func runBootstrap(ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitConfig
	}
	return routinerun.NewBootstrap(repoRoot, os.Stderr, os.Stdout).Run(ctx)
}

func runCell(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("cell", flag.ExitOnError)
	timeoutS := flags.Int("timeout", 0, "cell deadline in seconds")
	_ = flags.Parse(args)
	command := flags.Args()
	if *timeoutS <= 0 || len(command) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	release := routinerun.PreventSleep()
	defer release()
	spec := &routinerun.CellSpec{
		Args:     command,
		CellPids: os.Getenv("ROUTINE_CELL_PIDS"),
		Stdout:   os.Stdout,
		Timeout:  time.Duration(*timeoutS) * time.Second,
	}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitStartFailure
	}
	return status
}

func runRoutine(ctx context.Context, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	runner, err := routinerun.NewRunner(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitConfig
	}
	return runner.Run(ctx)
}

func runWorker(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("worker", flag.ExitOnError)
	attended := flags.Bool("attended", false, "start the cell without the headless marker")
	base := flags.String("base", "", "ref the worker branch forks from")
	budgetUsd := flags.String("budget-usd", "", "spend cap of the cell")
	effort := flags.String("effort", "", "effort level of the cell")
	keepStopped := flags.Bool("keep-stopped", false, "keep the worktree of a stopped cell for -resume")
	model := flags.String("model", "", "model of the cell")
	name := flags.String("name", "", "worker name: worktree dir and claude/<name> branch")
	resume := flags.String("resume", "", "session id of this worker's stopped cell; continues it in its kept worktree")
	skill := flags.String("skill", "", "skill leaf the cell's one prompt invokes")
	skillArgs := flags.String("args", "", "text after /<skill> in the prompt; the whole prompt of a resumed cell")
	timeoutS := flags.Int("timeout", defaultWorkerTimeoutS, "cell deadline in active seconds")
	token := flags.String("token", "", "token label; empty uses the launching session's login")
	_ = flags.Parse(args)

	required := []string{*base + *resume, *name, *skill}
	if slices.Contains(required, "") {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	commonDir, err := shell.Run(ctx, ".", "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		fmt.Fprintln(os.Stderr, "routine worker: not inside a git repository")
		return exitConfig
	}

	release := routinerun.PreventSleep()
	defer release()
	spec := &routinerun.WorkerSpec{
		Attended:    *attended,
		Base:        *base,
		BudgetUsd:   *budgetUsd,
		Effort:      *effort,
		KeepStopped: *keepStopped,
		Model:       *model,
		Name:        *name,
		Repo:        filepath.Dir(strings.TrimSpace(commonDir)),
		Resume:      *resume,
		Skill:       *skill,
		SkillArgs:   *skillArgs,
		Timeout:     time.Duration(*timeoutS) * time.Second,
		TokenLabel:  *token,
	}
	result, err := routinerun.RunWorker(ctx, spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitStartFailure
	}

	line, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitStartFailure
	}
	fmt.Fprintln(os.Stdout, string(line))
	return result.ExitStatus
}
