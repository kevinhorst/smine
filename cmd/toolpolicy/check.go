package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/planrules"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

// The anchor, settings layers and repo paths stay: they locate the worktree and authorize nothing.
func checkGrants(base *Grants, home string, skillNames []string) *Grants {
	grants := *base
	grants.Approvals = map[string]bool{}
	grants.HasHumanTurn = false
	grants.SkillAllow = make([]*Rule, 0)
	for _, skill := range skillNames {
		if manifest := readSkillManifest(home, skill, base.values); manifest != nil {
			grants.SkillAllow = append(grants.SkillAllow, manifest.Rules...)
		}
	}
	return &grants
}

func checkProbe(input *HookInput, policy *toolpolicy.Policy, target string) *HookInput {
	probe := &HookInput{Cwd: input.Cwd, PermissionMode: permissionModeAcceptEdits, ScratchpadDir: input.ScratchpadDir}
	isUrl := strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://")
	isToolName := !strings.ContainsAny(target, " \t") && policy.ClassOf(target) == toolpolicy.ClassRuleBearing
	switch {
	case isUrl:
		probe.ToolName = toolNameWebFetch
		probe.ToolInput.Url = target
	case isToolName:
		probe.ToolName = target
	default:
		probe.ToolName = toolNameBash
		probe.ToolInput.Command = target
	}
	return probe
}

// checkReport is the numbered verdict list for targets; skill flags lead.
func checkReport(input *HookInput, policy *toolpolicy.Policy, base *Grants, home string, args []string) (string, error) {
	skillNames, targets := splitCheckArgs(args)
	if len(targets) == 0 {
		return "", fmt.Errorf("checkReport: %s", policy.Causes.SettingsUsage)
	}

	grants := checkGrants(base, home, skillNames)
	lines := make([]string, 0, len(targets))
	for index, target := range targets {
		lines = append(lines, fmt.Sprintf("%d. %s → %s", index+1, target, checkTarget(input, policy, grants, home, target)))
	}
	return strings.Join(lines, "\n"), nil
}

func checkTarget(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string, target string) string {
	probe := checkProbe(input, policy, target)
	if isSelfServiceCommand(probe.ToolInput.Command) {
		return "not checkable: toolpolicy commands are gate commands"
	}

	verdict := Decide(probe, policy, grants, home, "")
	if verdict.Decision != decisionDeny {
		return "admitted"
	}
	if verdict.Cause != policy.Causes.NoAllowRule {
		return "denied: " + verdict.Cause + " — no allow rule lifts this; reshape it (split the call, use a make target, stay inside the session roots) and check again"
	}
	if err := isSelfServiceRule(verdict.CandidateRule, policy.WrapperCommands); err != nil {
		return "denied: no allow rule matches; candidate rule " + verdict.CandidateRule +
			" — not self-serviceable (" + err.Error() + "): reshape it, or propose it for the global template"
	}
	return "denied: no allow rule matches; candidate rule " + verdict.CandidateRule +
		" — self-serviceable: list it in the plan dir's " + planrules.FileName + " with this target"
}

func decideCheckCommand(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string, args []string) Verdict {
	report, err := checkReport(input, policy, grants, home, args)
	if err != nil {
		return grants.denyVerdict(policy, toolNameBash, input, policy.Causes.SettingsUsage)
	}

	verdict := Verdict{
		Cause:    causeCheckReport,
		Decision: decisionDeny,
		IsReport: true,
		Reason:   causeCheckReport + " — this call is refused only so Claude Code does not run it. Do not retry.\n" + report,
	}
	return verdict
}

func runCheck(args []string, cfg *hookconfig.Config, home string, gateRepo string, reposPath string, stdout io.Writer, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "runCheck: Failed to resolve cwd: %v\n", err)
		return 1
	}

	input := &HookInput{Cwd: cwd}
	base, err := LoadGrants(
		input,
		cfg,
		home,
		gateRepo,
		reposPath,
		"",
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	report, err := checkReport(input, &cfg.Toolpolicy, base, home, args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return usageExitCode
	}
	fmt.Fprintln(stdout, report)
	return 0
}

// splitCheckArgs separates the leading --skill <leaf> pairs from the targets.
func splitCheckArgs(args []string) ([]string, []string) {
	skillNames := make([]string, 0)
	for len(args) >= 2 && args[0] == checkSkillFlag {
		skillNames = append(skillNames, args[1])
		args = args[2:]
	}
	return skillNames, args
}
