// Command planmodecontext delivers the plan context pack once per plan-mode stretch of the root agent.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

// contextIndex is the slice of the context index the hook reads.
type contextIndex struct {
	Entries []contextdocs.RuleEntry `json:"entries"`
}

// HookInput is the UserPromptSubmit / PostToolUse envelope; unread fields stay undeclared.
type HookInput struct {
	AgentId        string `json:"agent_id"`
	Cwd            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	PermissionMode string `json:"permission_mode"`
	SessionId      string `json:"session_id"`
	ToolName       string `json:"tool_name"`
}

// Validate requires the fields the injection path and the index lookup hang on.
func (i *HookInput) Validate() error {
	// session_id
	if i.SessionId == "" {
		return errors.New("HookInput.Validate: Missing field session_id")
	}

	// cwd
	if i.Cwd == "" {
		return errors.New("HookInput.Validate: Missing field cwd")
	}
	return nil
}

func (i *HookInput) isPlanMode() bool {
	isEnterCall := i.HookEventName == eventPostToolUse && i.ToolName == toolEnterPlanMode
	return i.PermissionMode == permissionModePlan || isEnterCall
}

// HookOutput adds the pointer to the plan injection as context.
type HookOutput struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

type HookSpecificOutput struct {
	AdditionalContext string `json:"additionalContext"`
	HookEventName     string `json:"hookEventName"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

func claim(content string, injection string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(injection), 0o755); err != nil {
		return false, fmt.Errorf("claim: Failed to create %s: %w", filepath.Dir(injection), err)
	}

	file, err := os.OpenFile(injection, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim: Failed to create %s: %w", injection, err)
	}

	_, writeErr := file.WriteString(content)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return false, errors.Join(fmt.Errorf("claim: Failed to write %s: %w", injection, err), removeStale(injection))
	}
	return true, nil
}

func deliver(cfg *hookconfig.Config, input *HookInput, injection string) (string, error) {
	if !input.isPlanMode() {
		return "", removeStale(injection)
	}

	if isFile(injection) {
		return "", nil
	}

	index := indexPath(cfg.Context.ContextDir, input.Cwd)
	if index == "" {
		return "", nil
	}

	section := &cfg.Planmodecontext
	content, err := pack(index, section)
	if err != nil {
		return "", fmt.Errorf("deliver: %w", err)
	}

	if content == "" {
		return "", nil
	}

	isClaimed, err := claim(content, injection)
	if err != nil {
		return "", fmt.Errorf("deliver: %w", err)
	}

	if !isClaimed {
		return "", nil
	}

	lines := strings.Count(content, "\n")
	pointer := fmt.Sprintf(section.PointerFormat, section.Banner, injection, len(content), lines)
	instructions := contextrequired.ReadInstructions(lines, injection, 1, cfg.Globalcontext.ReadBudget)
	return pointer + "\n" + strings.Join(instructions, "\n"), nil
}

func indexPath(contextDir string, cwd string) string {
	dirs := []string{contextDir, fallbackContextDir}
	for _, dir := range dirs {
		path := filepath.Join(cwd, dir, contextdocs.ContextFileName)
		if isFile(path) {
			return path
		}
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "planmodecontext: isOff: %v\n", err)
	}
	return isOff
}

// namedSetting answers a plan-mode prompt without a context dir with the line naming the setting.
func namedSetting(input *HookInput, stderr io.Writer, stdout io.Writer) int {
	if !input.isPlanMode() {
		return 0
	}
	return writeOutputOrFail(noContextDirFormat, input.HookEventName, stdout, stderr)
}

func pack(index string, section *hookconfig.Planmodecontext) (string, error) {
	data, err := os.ReadFile(index)
	if err != nil {
		return "", fmt.Errorf("pack: Failed to read %s: %w", index, err)
	}

	decoded := &contextIndex{}
	if err := json.Unmarshal(data, decoded); err != nil {
		return "", fmt.Errorf("pack: Failed to decode %s: %w", index, err)
	}

	entries := &strings.Builder{}
	for _, entry := range decoded.Entries {
		if strings.HasPrefix(entry.Id, section.EntryIdPrefix) {
			fmt.Fprintf(entries, section.EntryFormat, entry.Id, entry.Content.Statement)
		}
	}

	if entries.Len() == 0 {
		return "", nil
	}

	content := fmt.Sprintf(section.PackFormat, section.Banner, section.Heading, entries.String(), section.Trailer)
	return content, nil
}

func removeStale(injection string) error {
	err := os.Remove(injection)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("removeStale: Failed to remove %s: %w", injection, err)
	}
	return nil
}

func run(args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("planmodecontext", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", "", "Claude Code home directory (default ~/.claude)")
	if err := flags.Parse(args); err != nil {
		return failureExitCode
	}

	home, err := config.ClaudeHomeDir(*claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return failureExitCode
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return failureExitCode
	}

	if err := input.Validate(); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return failureExitCode
	}

	cfg, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return failureExitCode
	}

	if cfg.Planmodecontext.RootOnly && input.AgentId != "" {
		return 0
	}
	if isOff(home, input.SessionId, sessionstate.InjectPlan) {
		return 0
	}
	if cfg.Context.ContextDir == "" {
		return namedSetting(input, stderr, stdout)
	}

	injection := filepath.Join(contextrequired.Dir(home, input.SessionId), contextrequired.PlanModeFile)
	addition, err := deliver(cfg, input, injection)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return failureExitCode
	}

	if addition == "" {
		return 0
	}
	return writeOutputOrFail(addition, input.HookEventName, stdout, stderr)
}

func writeOutput(addition string, event string, stdout io.Writer) error {
	output := &HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			AdditionalContext: addition,
			HookEventName:     event,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("writeOutput: Failed to encode output: %w", err)
	}
	return nil
}

// writeOutputOrFail writes the addition and returns run's exit code.
func writeOutputOrFail(addition string, event string, stdout io.Writer, stderr io.Writer) int {
	if err := writeOutput(addition, event, stdout); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return failureExitCode
	}
	return 0
}
