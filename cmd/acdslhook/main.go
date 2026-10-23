// Command acdslhook delivers a file's governing ACDSL rules as context after a Read.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/shell"
)

// HookContext is the PostToolUse context injection.
type HookContext struct {
	AdditionalContext string `json:"additionalContext"`
	HookEventName     string `json:"hookEventName"`
}

// HookInput is the Read envelope; fields the hook never inspects stay undeclared.
type HookInput struct {
	Cwd       string    `json:"cwd"`
	SessionId string    `json:"session_id"`
	ToolInput ToolInput `json:"tool_input"`
}

func (i *HookInput) relativeTarget() string {
	if i.Cwd == "" || i.ToolInput.FilePath == "" {
		return ""
	}

	info, err := os.Stat(filepath.Join(i.Cwd, registryPath))
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}

	rel, isInside := strings.CutPrefix(i.ToolInput.FilePath, i.Cwd+"/")
	if !isInside {
		return ""
	}
	return rel
}

// HookOutput wraps the context injection.
type HookOutput struct {
	HookSpecificOutput HookContext `json:"hookSpecificOutput"`
}

// ToolInput holds the read file.
type ToolInput struct {
	FilePath string `json:"file_path"`
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

// acdslCommand returns the argv prefix of the repo's own acdsl; nil when no binary exists and the go run fallback is off.
func acdslCommand(cwd string, section *hookconfig.Acdslhook) []string {
	for _, name := range section.Binaries {
		candidate := filepath.Join(cwd, "bin", name)
		if isExecutable(candidate) {
			return []string{candidate}
		}
	}
	if !section.GoRunFallback {
		return nil
	}
	return []string{"go", "run", "./cmd/acdsl"}
}

func deliverContext(ctx context.Context, cwd string, rel string, section *hookconfig.Acdslhook, stdout io.Writer) error {
	rules := runAcdsl(ctx, cwd, rel, section)
	if rules == "" {
		return nil
	}

	text := fmt.Sprintf(section.ContextText, rel, rules)
	output := &HookOutput{
		HookSpecificOutput: HookContext{
			AdditionalContext: text,
			HookEventName:     eventPostToolUse,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("deliverContext: Failed to encode context: %w", err)
	}
	return nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}

	hasExecutableBit := info.Mode().Perm()&executableBits != 0
	return hasExecutableBit || runtime.GOOS == windowsOs
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "acdslhook: isOff: %v\n", err)
	}
	return isOff
}

func run(ctx context.Context, args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("acdslhook", flag.ContinueOnError)
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

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}
	if isOff(home, input.SessionId, sessionstate.InjectAcdsl) {
		return 0
	}

	rel := input.relativeTarget()
	if rel == "" {
		return 0
	}

	if err := deliverContext(ctx, input.Cwd, rel, &cfg.Acdslhook, stdout); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	return 0
}

func runAcdsl(ctx context.Context, cwd string, rel string, section *hookconfig.Acdslhook) string {
	prefix := acdslCommand(cwd, section)
	if prefix == nil {
		return ""
	}

	runCtx, cancel := context.WithTimeout(ctx, section.TimeoutMs.Duration())
	defer cancel()

	projectArgs := []string{"project", contextFlag, rel}
	argv := slices.Concat(prefix, projectArgs)
	command := exec.CommandContext(runCtx, argv[0], argv[1:]...)
	command.Dir = cwd
	shell.HideWindow(command)
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(output), "\n")
}
