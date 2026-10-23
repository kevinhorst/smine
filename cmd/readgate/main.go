// Command readgate denies an agent's tool call until it has read every file it owes in full.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/readledger"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

// HookDecision is the PreToolUse permission verdict.
type HookDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// HookInput is the envelope of every event readgate handles; unread fields stay undeclared.
type HookInput struct {
	AgentId       string       `json:"agent_id"`
	Cwd           string       `json:"cwd"`
	HookEventName string       `json:"hook_event_name"`
	Prompt        string       `json:"prompt"`
	SessionId     string       `json:"session_id"`
	Source        string       `json:"source"`
	ToolInput     ToolInput    `json:"tool_input"`
	ToolName      string       `json:"tool_name"`
	ToolResponse  ToolResponse `json:"tool_response"`
}

// Validate requires the session id every ledger and injection path hangs on.
func (i *HookInput) Validate() error {
	// session_id
	if i.SessionId == "" {
		return errors.New("HookInput.Validate: Missing field session_id")
	}
	return nil
}

func (i *HookInput) isManifestWrite() bool {
	return i.isWrite() && strings.HasSuffix(i.ToolInput.FilePath, reviewscope.LaneManifestSuffix)
}

func (i *HookInput) isReadingAny(owed []string) bool {
	return i.ToolName == toolRead && slices.Contains(owed, i.ToolInput.FilePath)
}

func (i *HookInput) isWrite() bool {
	return i.ToolName == toolWrite || i.ToolName == toolEdit
}

func (i *HookInput) touchedPaths() []string {
	switch i.ToolName {
	case toolRead, toolEdit, toolWrite:
		return nonEmpty(i.ToolInput.FilePath)
	case toolBash:
		return bashTokens(i.ToolInput.Command)
	}
	return nil
}

// HookOutput wraps the deny verdict; an allow writes nothing.
type HookOutput struct {
	HookSpecificOutput HookDecision `json:"hookSpecificOutput"`
}

type ResponseFile struct {
	FilePath  string `json:"filePath"`
	NumLines  int    `json:"numLines"`
	StartLine int    `json:"startLine"`
}

// ToolInput holds the tool arguments the gate and the ledger read.
type ToolInput struct {
	Args      string `json:"args"`
	Command   string `json:"command"`
	Content   string `json:"content"`
	FilePath  string `json:"file_path"`
	NewString string `json:"new_string"`
	Skill     string `json:"skill"`
}

// ToolResponse models only a Read's file object; the harness also sends strings, arrays and other objects.
type ToolResponse struct {
	File ResponseFile `json:"file"`
}

func (r *ToolResponse) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	object := toolResponseObject{}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("ToolResponse.UnmarshalJSON: Failed to decode result object: %w", err)
	}

	r.File = object.File
	return nil
}

// toolResponseObject decodes ToolResponse's object form without recursing into its unmarshaler.
type toolResponseObject struct {
	File ResponseFile `json:"file"`
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

// decide runs the gate and writes its deny verdict; an allow writes nothing, and without a context dir every call is allowed.
func decide(ctx context.Context, claudeHome string, hookConfig *hookconfig.Config, input *HookInput, ledger *readledger.Ledger, stdout io.Writer) error {
	if hookConfig.Context.ContextDir == "" {
		return nil
	}

	off := offFeatures(claudeHome, input.SessionId)
	reason, err := NewGate(claudeHome, hookConfig, input, ledger, off).Decide(ctx)
	if err != nil {
		return fmt.Errorf("decide: %w", err)
	}

	if reason == "" {
		return nil
	}

	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       decisionDeny,
			PermissionDecisionReason: reason,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("decide: Failed to encode verdict: %w", err)
	}
	return nil
}

func dispatch(ctx context.Context, claudeHome string, hookConfig *hookconfig.Config, input *HookInput, stderr io.Writer, stdout io.Writer) error {
	ledger := readledger.New(input.AgentId, claudeHome, hookConfig.Readgate.LedgerRetentionMs.Duration(), input.SessionId)
	switch input.HookEventName {
	case eventPreToolUse:
		return decide(ctx, claudeHome, hookConfig, input, ledger, stdout)
	case eventPostToolUse:
		return record(input, ledger)
	case eventPromptSubmit:
		return ledger.RecordPrompt(input.Prompt)
	case eventSessionStart:
		logNoContextDir(hookConfig, stderr)
		return ledger.Reset(input.Source)
	}
	return nil
}

// logNoContextDir names the unset setting once per session start; the gate allows every call until it is set.
func logNoContextDir(hookConfig *hookconfig.Config, stderr io.Writer) {
	if hookConfig.Context.ContextDir == "" {
		fmt.Fprintln(stderr, noContextDirFormat)
	}
}

// offFeatures reads the session's switch record; an unreadable record is logged and switches nothing off.
func offFeatures(claudeHome string, sessionId string) []sessionstate.Feature {
	off, err := sessionstate.Off(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId))
	if err != nil {
		fmt.Fprintf(os.Stderr, "readgate: offFeatures: %v\n", err)
	}
	return off
}

// nonEmpty wraps a path as a list, empty for "".
func nonEmpty(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

func record(input *HookInput, ledger *readledger.Ledger) error {
	switch input.ToolName {
	case toolRead:
		file := input.ToolResponse.File
		return ledger.RecordRead(file.FilePath, file.NumLines, file.StartLine)
	case toolSkill:
		return ledger.RecordSkill(input.ToolInput.Args, input.ToolInput.Skill)
	}
	return nil
}

func run(ctx context.Context, args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("readgate", flag.ContinueOnError)
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

	hookConfig, err := hookconfig.Load(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	if err := dispatch(ctx, home, hookConfig, input, stderr, stdout); err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	return 0
}
