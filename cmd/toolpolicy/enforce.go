package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// writeToolNames are the tools whose target file the writer rules judge.
var writeToolNames = []string{toolNameEdit, toolNameNotebookEdit, toolNameWrite}

type GateRules struct {
	PlanModeDenied []string `json:"planModeDenied"`
	RootDenied     []string `json:"rootDenied"`
	Skill          string   `json:"-"`
	WorkflowOnly   []string `json:"workflowOnly"`
	Writers        []Writer `json:"writers"`
}

// Writer names the agent types allowed to write files matching Glob.
type Writer struct {
	AgentTypes []string `json:"agentTypes"`
	Glob       string   `json:"glob"`
}

func loadGateRules(home string, skill string) *GateRules {
	content, err := os.ReadFile(filepath.Join(home, ".claude", "skills", skill, enforceFile))
	if err != nil {
		return nil
	}

	document := &struct {
		Gate *GateRules `json:"gate"`
	}{}
	if err := json.Unmarshal(content, document); err != nil {
		return &GateRules{RootDenied: []string{"*"}, Skill: skill + " (unreadable " + enforceFile + ")", Writers: []Writer{{Glob: "*"}}}
	}
	if document.Gate == nil {
		return nil
	}

	document.Gate.Skill = skill
	return document.Gate
}

func enforceVerdict(tool string, cause string) Verdict {
	return Verdict{
		Cause:    cause,
		Decision: decisionDeny,
		Reason:   "tool-policy: denied " + tool + " — " + cause + ". No allow rule lifts this; follow the skill's protocol instead.",
	}
}

func enforceCause(input *HookInput, rules []*GateRules) string {
	isRoot := input.AgentId == ""
	isWrite := slices.Contains(writeToolNames, input.ToolName)
	target := writeTarget(input)
	for _, rule := range rules {
		isPlanDenied := input.PermissionMode == permissionModePlan && slices.Contains(rule.PlanModeDenied, input.ToolName)
		if isPlanDenied {
			return fmt.Sprintf("enforce(%s): %s is not allowed in plan mode", rule.Skill, input.ToolName)
		}

		isHandSpawn := input.ToolName == toolNameAgent && matchesAnyGlob(rule.WorkflowOnly, input.ToolInput.SubagentType)
		if isHandSpawn {
			return fmt.Sprintf("enforce(%s): %s agents are spawned only by the skill's Workflow", rule.Skill, input.ToolInput.SubagentType)
		}

		if !isWrite {
			continue
		}

		if isRoot && matchesAnyGlob(rule.RootDenied, target) {
			return fmt.Sprintf("enforce(%s): %s is written only by the skill's tooling, never by the dispatcher", rule.Skill, target)
		}

		for _, writer := range rule.Writers {
			isForeign := globMatch(writer.Glob, target) && !slices.Contains(writer.AgentTypes, input.AgentType)
			if isForeign {
				return fmt.Sprintf("enforce(%s): %s is written only by %v", rule.Skill, target, writer.AgentTypes)
			}
		}
	}
	return ""
}

// writeTarget is the file a write tool call targets.
func writeTarget(input *HookInput) string {
	if input.ToolInput.FilePath != "" {
		return input.ToolInput.FilePath
	}
	return input.ToolInput.NotebookPath
}

// matchesAnyGlob reports whether text matches one of the gate globs.
func matchesAnyGlob(patterns []string, text string) bool {
	return text != "" && slices.ContainsFunc(patterns, func(pattern string) bool {
		return globMatch(pattern, text)
	})
}
