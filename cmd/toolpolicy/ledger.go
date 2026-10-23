package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Denial struct {
	AgentId       string `json:"agent_id,omitempty"`
	AgentType     string `json:"agent_type,omitempty"`
	Anchor        string `json:"anchor,omitempty"`
	CandidateRule string `json:"candidate_rule"`
	Cause         string `json:"cause"`
	Cwd           string `json:"cwd"`
	Input         string `json:"input"`
	ScratchpadDir string `json:"scratchpad_dir,omitempty"`
	SessionId     string `json:"session_id"`
	Timestamp     string `json:"ts"`
	Tool          string `json:"tool"`
}

func appendDenial(path string, anchor string, input *HookInput, verdict Verdict, now time.Time) {
	if err := os.MkdirAll(filepath.Dir(path), ledgerDirMode); err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: appendDenial: Failed to create ledger dir: %v\n", err)
		return
	}

	denial := Denial{
		AgentId:       input.AgentId,
		AgentType:     input.AgentType,
		Anchor:        anchor,
		CandidateRule: verdict.CandidateRule,
		Cause:         verdict.Cause,
		Cwd:           input.Cwd,
		Input:         inputSummary(input.ToolName, &input.ToolInput),
		ScratchpadDir: input.ScratchpadDir,
		SessionId:     input.SessionId,
		Timestamp:     now.UTC().Format(time.RFC3339),
		Tool:          input.ToolName,
	}
	line, err := json.Marshal(denial)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: appendDenial: Failed to encode denial: %v\n", err)
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ledgerFileMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: appendDenial: Failed to open ledger: %v\n", err)
		return
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: appendDenial: Failed to write ledger: %v\n", err)
	}
}

func inputSummary(toolName string, input *ToolInput) string {
	switch {
	case toolName == toolNameBash:
		return input.Command
	case toolName == toolNameWebFetch:
		return urlHost(input.Url)
	case toolName == toolNameSkill:
		return input.Skill
	case toolName == toolNameAgent:
		return input.SubagentType
	case strings.HasPrefix(toolName, mcpPrefix):
		return toolName
	case input.FilePath != "":
		return input.FilePath
	case input.NotebookPath != "":
		return input.NotebookPath
	}
	return input.Path
}
