// Command worktreewriteguard keeps a worktree-anchored session's writes inside its own worktree.
package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
)

// HookDecision is the PreToolUse permission verdict.
type HookDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// HookInput is the PreToolUse envelope; unread fields stay undeclared.
type HookInput struct {
	Cwd       string    `json:"cwd"`
	ToolInput ToolInput `json:"tool_input"`
}

// HookOutput wraps the deny verdict; an allow writes nothing.
type HookOutput struct {
	HookSpecificOutput HookDecision `json:"hookSpecificOutput"`
}

type ToolInput struct {
	FilePath     string `json:"file_path"`
	NotebookPath string `json:"notebook_path"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

// denyReason returns why the write is refused, empty when it passes.
func denyReason(cwd string, path string, protectedPaths string, section *hookconfig.Worktreewriteguard) string {
	if cwd == "" || path == "" {
		return ""
	}

	target := path
	if !strings.HasPrefix(path, "/") {
		target = cwd + "/" + path
	}

	if root := config.ProtectedRoot(target, protectedPaths, section.PoolSegment); root != "" {
		return fmt.Sprintf(section.DenyTexts[denyTextProtected], root)
	}

	mainRoot, rest, isPoolSession := strings.Cut(cwd, section.PoolSegment)
	if !isPoolSession {
		return ""
	}

	name, _, _ := strings.Cut(rest, "/")
	worktreeRoot := mainRoot + section.PoolSegment + name
	if strings.Contains(target, parentSegment) {
		return fmt.Sprintf(section.DenyTexts[denyTextParent], worktreeRoot)
	}

	if strings.HasPrefix(target, worktreeRoot+"/") {
		return ""
	}

	if strings.HasPrefix(target, mainRoot+"/") {
		return fmt.Sprintf(section.DenyTexts[denyTextOutside], worktreeRoot, target)
	}
	return ""
}

func run(args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("worktreewriteguard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", "", "Claude Code home directory (default ~/.claude)")
	if err := flags.Parse(args); err != nil {
		return usageExitCode
	}

	home, err := config.ClaudeHomeDir(*claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	cfg, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	if len(cfg.Worktreewriteguard.DenyTexts) != denyTextCount {
		fmt.Fprintf(stderr, "run: %s: worktreewriteguard.deny_texts holds %d texts, not %d\n", hookconfig.Path(home), len(cfg.Worktreewriteguard.DenyTexts), denyTextCount)
		return 1
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}

	path := cmp.Or(input.ToolInput.FilePath, input.ToolInput.NotebookPath)
	reason := denyReason(input.Cwd, path, os.Getenv(config.ProtectedPathsKey), &cfg.Worktreewriteguard)
	if reason == "" {
		return 0
	}

	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       decisionDeny,
			PermissionDecisionReason: reason,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "run: Failed to encode verdict: %v\n", err)
		return 1
	}
	return 0
}
