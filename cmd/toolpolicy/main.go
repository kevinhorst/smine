// Command toolpolicy decides every Claude Code tool call as a closed-world hook gate.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/desktop"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/trash"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	isInterfaces := len(args) > 0 && args[0] == commandInterfaces
	if isInterfaces {
		return runInterfaces(args[1:], stdout, stderr)
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	flags := flag.NewFlagSet("toolpolicy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gateRepo := flags.String("gate-repo", "", "absolute main checkout of the gate's own source repo: its linked worktrees may not run the policy's operator paths (empty: guard off)")
	ledgerPath := flags.String("ledger", "", "path to the denial ledger (JSONL, appended)")
	reposPath := flags.String("repos", filepath.Join(stateDir, config.StateRegistryFile), "repo registry backing the @repos skill path grant (empty: no grant)")
	if err := flags.Parse(args); err != nil {
		return usageExitCode
	}
	if *gateRepo != "" {
		// the hook command carries it with forward slashes; the worktree roots it is compared with are native
		*gateRepo = filepath.Clean(*gateRepo)
	}

	claudeHome, err := config.ClaudeHomeDir("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := hookconfig.Load(claudeHome)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	policy := &cfg.Toolpolicy

	policy.ReadFence = filepath.SplitList(os.Getenv(config.ReadFenceEnv))

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "run: Failed to resolve home: %v\n", err)
		return 1
	}

	if flags.Arg(0) == commandCheck {
		return runCheck(
			flags.Args()[1:],
			cfg,
			home,
			*gateRepo,
			*reposPath,
			stdout,
			stderr,
		)
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}

	grants, err := LoadGrants(
		input,
		cfg,
		home,
		*gateRepo,
		*reposPath,
		frozenLedgerPath(*ledgerPath),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	grants.trash = trash.New(stateDir, 0)

	if configDir, err := os.UserConfigDir(); err == nil {
		added := sessionDirectories(os.Getenv(desktop.HostSessionEnv), desktop.SessionStoreDir(configDir))
		grants.AdditionalDirectories = append(grants.AdditionalDirectories, added...)
	}
	state := statePath(*ledgerPath, input.SessionId)
	reconcileSettingsMessages(input, grants, cfg.Context.ContextDir, state, stdout)

	switch input.HookEventName {
	case hookEventDirectoryAdded:
		return 0
	case hookEventUserPromptSubmit, hookEventPostToolUse:
		return runPlanHook(input, policy, grants, state, stdout)
	case hookEventPermissionRequest:
		if err := answerPermissionRequest(input, policy, grants, home, state, stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	verdict := deferAllowToAutoMode(input, Decide(input, policy, grants, home, state))
	isGateAnswer := verdict.IsApplied || verdict.IsReport
	isLedgerDeny := verdict.Decision == decisionDeny && !isGateAnswer
	if isLedgerDeny && *ledgerPath != "" {
		appendDenial(*ledgerPath, grants.anchorRoot(input), input, verdict, time.Now())
	}

	if verdict.Decision == decisionNone {
		return 0
	}

	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            hookEventPreToolUse,
			PermissionDecision:       verdict.Decision,
			PermissionDecisionReason: verdict.Reason,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "run: Failed to encode decision: %v\n", err)
		return 1
	}
	return 0
}
