package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/planrules"
	"github.com/kevinhorst/smine/internal/plans"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/toolpolicy"
	"github.com/kevinhorst/smine/internal/transcript"
	"github.com/kevinhorst/smine/internal/trash"
)

func gateApproval(input *HookInput, policy *toolpolicy.Policy, grants *Grants, planFile string) string {
	guard := grants.freeze
	dir := guard.planDirOf(planFile)
	isTyped := input.HookEventName == hookEventUserPromptSubmit
	isFrozen := planrules.Latest(dir, guard.records) != nil
	if !isTyped || isFrozen {
		return ""
	}

	markdown, err := os.ReadFile(planFile)
	isAtCodeGate := err == nil && plans.Gate(string(markdown)) == plans.GateCode
	if !isAtCodeGate || guard.planCause(&policy.Causes, dir) != "" {
		return ""
	}

	record, cause := recordFreeze(grants.anchorRoot(input), guard, input, policy, dir)
	if cause != "" {
		return "approval: this typed /fimplement approves the code gate, but " + dir + " did not freeze — " + cause
	}
	return fmt.Sprintf("approval: this typed /fimplement approves the code gate — %s frozen with %d rule(s)", dir, len(record.Rules))
}

func applyRecords(anchor string, records []*planrules.Record, statePath string, policy *toolpolicy.Policy, keep *trash.Store) []string {
	target := filepath.Join(anchor, ".claude", unlockSettings)
	settings, allow, err := allowedRules(target)
	if err != nil {
		return repeatedLine(len(records), "Permissions: none applied — "+unlockSettings+" does not load, the user fixes it: "+err.Error())
	}

	summaries := make([]string, 0, len(records))
	appended := make([]string, 0)
	for _, record := range records {
		summary, fresh := recordSummary(record, append(slices.Clone(allow), appended...), policy)
		summaries = append(summaries, summary)
		appended = append(appended, fresh...)
	}
	if len(appended) == 0 {
		return summaries
	}

	state, err := sessionstate.Load(statePath)
	if err != nil {
		return repeatedLine(len(records), "Permissions: none applied — "+policy.Causes.StateUnavailable+err.Error())
	}

	state.SettingsLocal = sessionstate.SelfServiceState{IsReverted: false, LastAppended: appended}
	if err := sessionstate.Save(statePath, state); err != nil {
		return repeatedLine(len(records), "Permissions: none applied — tool-policy state unwritable: "+err.Error())
	}
	if err := writePermissionList(target, settings, allowKey, append(allow, appended...), keep); err != nil {
		return repeatedLine(len(records), "Permissions: none applied — "+unlockSettings+" write failed after validation: "+err.Error())
	}
	return summaries
}

func recordSummary(record *planrules.Record, allow []string, policy *toolpolicy.Policy) (string, []string) {
	rows := record.RuleStrings()
	if len(rows) == 0 {
		return "Permissions: " + record.Plan + " is frozen with no rule", nil
	}

	fresh, err := checkNewRules(rows, allow, policy)
	if err != nil {
		return "Permissions: none applied — frozen rule rejected: " + err.Error() + "; report it to the user", nil
	}

	summary := fmt.Sprintf("Permissions: appended %d rule(s), %d already present — %s", len(fresh), len(rows)-len(fresh), strings.Join(rows, " "))
	return summary, fresh
}

func repeatedLine(count int, line string) []string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = line
	}
	return lines
}

func explicitPlan(args string, cwd string, root string) (string, string) {
	for word := range strings.FieldsSeq(args) {
		if !strings.HasSuffix(word, ".md") {
			continue
		}

		relative, err := filepath.Rel(resolvePath(root, ""), resolvePath(cwd, word))
		isPlanPath := err == nil && planrules.IsPlanFile(filepath.ToSlash(relative)) && isFile(filepath.Join(root, relative))
		if isPlanPath {
			return filepath.Join(root, relative), ""
		}
		if strings.ContainsRune(word, filepath.Separator) {
			continue
		}

		named := namedPlans(word, root)
		switch len(named) {
		case 0:
			continue
		case 1:
			return filepath.Join(root, named[0]), ""
		}
		return "", "plan not resolved — several wip plan dirs hold " + word + ": " + strings.Join(named, ", ") +
			"; ask the user which one, they send /fimplement <plan file path>"
	}
	return "", ""
}

func fimplementArgs(input *HookInput, fimplementSkill string) (string, bool) {
	switch input.HookEventName {
	case hookEventUserPromptSubmit:
		if isWrappedPrompt(input.Prompt) {
			return "", false
		}
		return transcript.SlashWordArgs(fimplementSkill, input.Prompt)
	case hookEventPostToolUse:
		isFimplementCall := input.ToolName == toolNameSkill && input.ToolInput.Skill == fimplementSkill
		return strings.TrimSpace(string(input.ToolInput.Args)), isFimplementCall
	}
	return "", false
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isWrappedPrompt(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	return strings.HasPrefix(trimmed, "<") && !strings.HasPrefix(trimmed, pastedContentOpen)
}

func namedPlans(name string, root string) []string {
	matches, err := filepath.Glob(filepath.Join(root, "plans", "*", "*-wip-*", "design", name))
	if err != nil {
		return nil
	}

	named := make([]string, 0, len(matches))
	for _, match := range matches {
		relative, err := filepath.Rel(root, match)
		if err != nil {
			continue
		}
		if planrules.IsPlanFile(filepath.ToSlash(relative)) {
			named = append(named, relative)
		}
	}
	return named
}

func resolvePlan(cwd string, anchor string, args string, guard *freezeGuard) (string, string) {
	root := gitroot.CheckoutRoot(anchor)
	plan, note := explicitPlan(args, cwd, root)
	isSettled := plan != "" || note != ""
	if isSettled {
		return plan, note
	}

	if checked := sessionCheckedPlan(root, guard); checked != "" {
		return checked, ""
	}
	return "", "no plan named and none section-checked in this session — best-effort mode; name it: /fimplement <plan file>"
}

func sessionCheckedPlan(root string, guard *freezeGuard) string {
	for _, checked := range slices.Backward(guard.sectionedPlans) {
		relative, err := filepath.Rel(resolvePath(root, ""), resolvePath(root, checked))
		isPlanFile := err == nil && planrules.IsPlanFile(filepath.ToSlash(relative))
		if isPlanFile && isFile(filepath.Join(root, relative)) {
			return filepath.Join(root, relative)
		}
	}
	return ""
}

func planHookReport(input *HookInput, policy *toolpolicy.Policy, grants *Grants, statePath string, args string) []string {
	lines := []string{planHookHeader}
	anchor := grants.anchorRoot(input)
	if anchor == "" {
		return append(lines, "- plan: not resolved — "+policy.Causes.AnchorRemoved, "- Permissions: none applied")
	}

	plan, note := resolvePlan(input.Cwd, anchor, args, grants.freeze)
	if plan == "" {
		return append(lines, "- plan: "+note, "- Permissions: none applied")
	}

	plans := []string{plan}
	if derived := planrules.DerivedPlans(plan); len(derived) > 0 {
		lines = append(lines, "- master: "+plan+" — applying its "+strconv.Itoa(len(derived))+" derived plans")
		plans = derived
	}

	approvals := make([]string, 0, len(plans))
	records := make([]*planrules.Record, 0, len(plans))
	for _, planFile := range plans {
		approvals = append(approvals, gateApproval(input, policy, grants, planFile))
		if record := planrules.Latest(grants.freeze.planDirOf(planFile), grants.freeze.records); record != nil {
			records = append(records, record)
		}
	}
	applied := applyRecords(anchor, records, statePath, policy, grants.trash)
	return append(lines, planLines(plans, approvals, records, applied, grants.freeze)...)
}

func planLines(plans []string, approvals []string, records []*planrules.Record, applied []string, guard *freezeGuard) []string {
	lines := make([]string, 0)
	next := 0
	for i, plan := range plans {
		lines = append(lines, "- plan: "+plan)
		if approvals[i] != "" {
			lines = append(lines, "- "+approvals[i])
		}

		dir := guard.planDirOf(plan)
		isFrozen := next < len(records) && records[next].Plan == dir
		if !isFrozen {
			lines = append(lines, "- Permissions: none applied — "+dir+" is not frozen; a design run freezes it: toolpolicy freeze "+dir)
			continue
		}

		lines = append(lines, "- "+applied[next], "- checked: "+strings.Join(records[next].Checked, ", "))
		next++
	}
	return lines
}

func runPlanHook(input *HookInput, policy *toolpolicy.Policy, grants *Grants, statePath string, stdout io.Writer) int {
	args, isInvocation := fimplementArgs(input, policy.FimplementSkill)
	if !isInvocation || os.Getenv(headlessEnv) == headlessEnvValue {
		return 0
	}

	report := strings.Join(planHookReport(input, policy, grants, statePath, args), "\n")
	if input.HookEventName == hookEventUserPromptSubmit {
		fmt.Fprintln(stdout, report)
		return 0
	}

	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			AdditionalContext: report,
			HookEventName:     hookEventPostToolUse,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "runPlanHook: Failed to encode context: %v\n", err)
		return 1
	}
	return 0
}
