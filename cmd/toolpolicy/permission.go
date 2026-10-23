package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/kevinhorst/smine/internal/toolpolicy"
)

// A self-service command is answered at PreToolUse and never reaches a request.
func answerPermissionRequest(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string, statePath string, stdout io.Writer) error {
	isSelfService := input.ToolName == toolNameBash && isSelfServiceCommand(input.ToolInput.Command)
	if isSelfService {
		return nil
	}

	verdict := deferAllowToAutoMode(input, Decide(input, policy, grants, home, statePath))
	answer := &PermissionAnswer{Behavior: verdict.Decision}
	switch verdict.Decision {
	case decisionNone:
		return nil
	case decisionDeny:
		answer.Message = verdict.Reason
	}

	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			Decision:      answer,
			HookEventName: hookEventPermissionRequest,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("answerPermissionRequest: Failed to encode answer: %w", err)
	}
	return nil
}
