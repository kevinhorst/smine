// Command skillcontext injects the context entries a skill declares when the skill is invoked.
package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

// HookInput is the UserPromptSubmit / PostToolUse envelope; unread fields stay undeclared.
type HookInput struct {
	AgentId       string    `json:"agent_id"`
	Cwd           string    `json:"cwd"`
	HookEventName string    `json:"hook_event_name"`
	Prompt        string    `json:"prompt"`
	SessionId     string    `json:"session_id"`
	ToolInput     ToolInput `json:"tool_input"`
	ToolName      string    `json:"tool_name"`
	UserInput     string    `json:"user_input"`
}

func (i *HookInput) invocation() (Invocation, bool) {
	switch i.HookEventName {
	case eventPromptSubmit:
		return promptInvocation(cmp.Or(i.Prompt, i.UserInput))
	case eventPostToolUse:
		if i.ToolName != toolSkill || i.ToolInput.Skill == "" {
			return Invocation{}, false
		}

		call := Invocation{Args: i.ToolInput.Args, Skill: i.ToolInput.Skill}
		return call, true
	}
	return Invocation{}, false
}

// HookOutput adds the injection as context on a Skill call.
type HookOutput struct {
	HookSpecificOutput HookSpecificOutput `json:"hookSpecificOutput"`
}

type HookSpecificOutput struct {
	AdditionalContext string `json:"additionalContext"`
	HookEventName     string `json:"hookEventName"`
}

// Invocation is one skill call: the skill's name and its invocation text.
type Invocation struct {
	Args  string
	Skill string
}

func (c *Invocation) isSkipped(helpPattern *regexp.Regexp, noContextPattern *regexp.Regexp) bool {
	return noContextPattern.MatchString(c.Args) || helpPattern.MatchString(c.Args)
}

// ToolInput holds the Skill tool's arguments.
type ToolInput struct {
	Args  string `json:"args"`
	Skill string `json:"skill"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillcontext: isOff: %v\n", err)
	}
	return isOff
}

func promptInvocation(prompt string) (Invocation, bool) {
	command, isSlash := strings.CutPrefix(prompt, "/")
	if !isSlash {
		return Invocation{}, false
	}

	skill := command
	if end := strings.IndexFunc(command, unicode.IsSpace); end >= 0 {
		skill = command[:end]
	}
	if skill == "" {
		return Invocation{}, false
	}

	call := Invocation{Args: prompt, Skill: skill}
	return call, true
}

func run(args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("skillcontext", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", "", "Claude Code home directory (default ~/.claude)")
	skillsDir := flags.String("skills", "", "deployed skills directory (default <claude home>/skills)")
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

	cfg, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	helpPattern, err := regexp.Compile(cfg.Skillcontext.HelpPattern)
	if err != nil {
		fmt.Fprintf(stderr, "run: Invalid skillcontext.help_pattern: %v\n", err)
		return 1
	}

	noContextPattern, err := regexp.Compile(cfg.Skillcontext.NoContextPattern)
	if err != nil {
		fmt.Fprintf(stderr, "run: Invalid skillcontext.no_context_pattern: %v\n", err)
		return 1
	}

	call, isInvocation := input.invocation()
	if !isInvocation || call.isSkipped(helpPattern, noContextPattern) {
		return 0
	}
	if input.Cwd == "" {
		return 0
	}
	if isOff(home, input.SessionId, sessionstate.InjectSkill) {
		return 0
	}

	addition, err := inject(home, cfg, input, &call, cmp.Or(*skillsDir, filepath.Join(home, "skills")))
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	if addition == "" {
		return 0
	}

	if err := writeOutput(addition, input.HookEventName, stdout); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	return 0
}

func writeOutput(addition string, event string, stdout io.Writer) error {
	if event == eventPromptSubmit {
		if _, err := fmt.Fprintln(stdout, addition); err != nil {
			return fmt.Errorf("writeOutput: Failed to write output: %w", err)
		}
		return nil
	}

	output := &HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			AdditionalContext: addition,
			HookEventName:     eventPostToolUse,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("writeOutput: Failed to encode output: %w", err)
	}
	return nil
}
