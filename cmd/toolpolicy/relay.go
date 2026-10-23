package main

import (
	"strings"

	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/toolpolicy"
	"github.com/kevinhorst/smine/internal/transcript"
)

func isRelayPrimer(script string) bool {
	return strings.Join(strings.Fields(script), " ") == strings.Join(strings.Fields(relayPrimerScript), " ")
}

func relayCause(causes *hookconfig.ToolpolicyCauses, input *HookInput, grants *Grants) string {
	script := input.ToolInput.Script
	switch {
	case input.ToolName != toolNameWorkflow:
		return ""
	case isRelayPrimer(script):
		return ""
	case strings.Contains(script, relayMarker):
		return causes.WorkflowMarker
	case !grants.HasHumanTurn:
		return ""
	case grants.LastTurnOrigin == transcript.OriginTaskNotification:
		return ""
	}
	return causes.WorkflowRelay
}

func relayVerdict(policy *toolpolicy.Policy, cause string) Verdict {
	verdict := Verdict{
		Cause:    cause,
		Decision: decisionDeny,
		Reason:   "tool-policy: denied " + toolNameWorkflow + " — " + cause + ". " + policy.Steers.Relay + relayPrimerScript,
	}
	return verdict
}
