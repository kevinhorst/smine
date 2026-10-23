// Command globalcontext injects the always-read global content into every session and subagent.
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
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

// HookInput is the SessionStart / SubagentStart envelope; fields the hook never inspects stay undeclared.
type HookInput struct {
	Cwd           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	SessionId     string `json:"session_id"`
}

// Validate requires the fields the injection file path and the repo-local dir hang on.
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

// HookOutput adds the global content, or the pointer to its injection file, as context.
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

func deliver(claudeHome string, content string, sessionId string, section *hookconfig.Globalcontext) (string, error) {
	injection := filepath.Join(contextrequired.Dir(claudeHome, sessionId), contextrequired.GlobalFile)
	if len(content) <= section.InlineCap {
		return content, removeStale(injection)
	}

	if err := writeInjection(content, injection); err != nil {
		return "", err
	}

	lines := strings.Count(content, "\n")
	pointer := fmt.Sprintf(section.PointerFormat, section.Banner, injection, len(content), lines)
	instructions := contextrequired.ReadInstructions(lines, injection, 1, section.ReadBudget)
	return pointer + "\n" + strings.Join(instructions, "\n"), nil
}

func gather(files []string, banner string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	builder := &strings.Builder{}
	builder.WriteString(banner + "\n\n")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("gather: Failed to read %s: %w", file, err)
		}

		fmt.Fprintf(builder, "===== %s =====\n%s\n", file, data)
	}
	return builder.String(), nil
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "globalcontext: isOff: %v\n", err)
	}
	return isOff
}

func markdownFiles(dirs []string) ([]string, error) {
	files := make([]string, 0)
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return nil, fmt.Errorf("markdownFiles: Failed to glob %s: %w", dir, err)
		}

		files = append(files, matches...)
	}
	return files, nil
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

func render(dirs []string, claudeHome string, sessionId string, section *hookconfig.Globalcontext) (string, error) {
	files, err := markdownFiles(dirs)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}

	content, err := gather(files, section.Banner)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}

	addition, err := deliver(claudeHome, content, sessionId, section)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	return addition, nil
}

func run(args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("globalcontext", flag.ContinueOnError)
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

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}

	if err := input.Validate(); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	cfg, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	if isOff(home, input.SessionId, sessionstate.InjectGlobal) {
		return 0
	}

	isContextDirSet := cfg.Context.ContextDir != ""
	dirs := []string{filepath.Join(home, "context", "global")}
	if isContextDirSet {
		dirs = append(dirs, filepath.Join(input.Cwd, cfg.Context.ContextDir, "global"))
	}

	addition, err := render(dirs, home, input.SessionId, &cfg.Globalcontext)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	if !isContextDirSet {
		addition = withNotice(addition)
	}
	if addition == "" {
		return 0
	}
	return writeOutputOrFail(addition, input.HookEventName, stdout, stderr)
}

// withNotice appends the unset-context-dir line after the global content, so the content's banner still leads.
func withNotice(addition string) string {
	if addition == "" {
		return noContextDirFormat
	}
	return addition + "\n" + noContextDirFormat
}

func writeInjection(content string, injection string) error {
	if err := os.MkdirAll(filepath.Dir(injection), 0o755); err != nil {
		return fmt.Errorf("writeInjection: Failed to create %s: %w", filepath.Dir(injection), err)
	}

	if err := fsx.WriteFileAtomic([]byte(content), injection, 0o644); err != nil {
		return fmt.Errorf("writeInjection: Failed to write %s: %w", injection, err)
	}
	return nil
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
		return 1
	}
	return 0
}
