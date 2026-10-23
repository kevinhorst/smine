package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// BatchAction is one action of a Browser pane batch call.
type BatchAction struct {
	Input BatchActionInput `json:"input"`
}

// BatchActionInput is the slice of a batch action's input the gate inspects.
type BatchActionInput struct {
	Url string `json:"url"`
}

// Any other shape decodes to none, so no MCP tool's own actions field fails the envelope decode.
type BatchActions []BatchAction

func (a *BatchActions) UnmarshalJSON(data []byte) error {
	actions := make([]BatchAction, 0)
	isActionList := json.Unmarshal(data, &actions) == nil
	if !isActionList {
		actions = nil
	}
	*a = actions
	return nil
}

type HookDecision struct {
	AdditionalContext        string            `json:"additionalContext,omitempty"`
	Decision                 *PermissionAnswer `json:"decision,omitempty"`
	HookEventName            string            `json:"hookEventName"`
	PermissionDecision       string            `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string            `json:"permissionDecisionReason,omitempty"`
}

type HookInput struct {
	AgentId        string    `json:"agent_id"`
	AgentType      string    `json:"agent_type"`
	Cwd            string    `json:"cwd"`
	Directory      string    `json:"directory"`
	HookEventName  string    `json:"hook_event_name"`
	PermissionMode string    `json:"permission_mode"`
	Prompt         string    `json:"prompt"`
	ScratchpadDir  string    `json:"scratchpad_dir"`
	SessionId      string    `json:"session_id"`
	ToolInput      ToolInput `json:"tool_input"`
	ToolName       string    `json:"tool_name"`
	TranscriptPath string    `json:"transcript_path"`
}

type HookOutput struct {
	HookSpecificOutput HookDecision `json:"hookSpecificOutput"`
}

type PermissionAnswer struct {
	Behavior string `json:"behavior"`
	Message  string `json:"message,omitempty"`
}

type ToolArgs string

func (a *ToolArgs) UnmarshalJSON(data []byte) error {
	text := ""
	if err := json.Unmarshal(data, &text); err == nil {
		*a = ToolArgs(text)
		return nil
	}

	compact := &bytes.Buffer{}
	if err := json.Compact(compact, data); err != nil {
		return fmt.Errorf("ToolArgs.UnmarshalJSON: Failed to compact args: %w", err)
	}
	*a = ToolArgs(compact.String())
	return nil
}

type ToolInput struct {
	Actions      BatchActions `json:"actions"`
	Args         ToolArgs     `json:"args"`
	Command      string       `json:"command"`
	FilePath     string       `json:"file_path"`
	Model        string       `json:"model"`
	NotebookPath string       `json:"notebook_path"`
	Path         string       `json:"path"`
	Pattern      string       `json:"pattern"`
	Script       string       `json:"script"`
	ScriptPath   string       `json:"scriptPath"`
	Skill        string       `json:"skill"`
	SubagentType string       `json:"subagent_type"`
	Url          string       `json:"url"`
}

type Verdict struct {
	CandidateRule string `json:"candidate_rule,omitempty"`
	Cause         string `json:"cause,omitempty"`
	Decision      string `json:"decision"`
	IsApplied     bool   `json:"is_applied,omitempty"`
	IsReport      bool   `json:"is_report,omitempty"`
	Reason        string `json:"reason"`
}
