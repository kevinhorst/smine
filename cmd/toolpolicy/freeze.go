package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/planrules"
	"github.com/kevinhorst/smine/internal/toolpolicy"
	"github.com/kevinhorst/smine/internal/transcript"
)

type freezeGuard struct {
	designPrompts  []string
	ledger         string
	records        []*planrules.Record
	root           string
	sectionedPlans []string
	sessionId      string
}

func newFreezeGuard(anchor string, designPrompts []string, ledger string, sectionedPlans []string, sessionId string) *freezeGuard {
	guard := &freezeGuard{
		designPrompts:  designPrompts,
		ledger:         ledger,
		records:        make([]*planrules.Record, 0),
		sectionedPlans: sectionedPlans,
		sessionId:      sessionId,
	}
	if anchor == "" {
		return guard
	}

	guard.records = planrules.LoadRecords(ledger, gitroot.MainRoot(anchor))
	guard.root = gitroot.CheckoutRoot(anchor)
	return guard
}

func (g *freezeGuard) fileCause(causes *hookconfig.ToolpolicyCauses, cwd string, target string) string {
	isUnanchored := g == nil || g.root == ""
	if isUnanchored {
		return ""
	}

	relative, err := filepath.Rel(resolvePath(g.root, ""), resolvePath(cwd, target))
	if err != nil {
		return ""
	}

	slashed := filepath.ToSlash(relative)
	plan := path.Dir(slashed)
	isRulesFile := path.Base(slashed) == planrules.FileName && planrules.IsPlanDir(plan)
	if !isRulesFile {
		return ""
	}
	if cause := g.planCause(causes, plan); cause != "" {
		return causes.RulesFile + ": " + cause
	}
	return ""
}

func (g *freezeGuard) namesPlan(plan string) bool {
	fileName := planrules.PlanFileName(plan)
	return slices.ContainsFunc(g.designPrompts, func(args string) bool {
		return slices.ContainsFunc(strings.Fields(args), func(word string) bool {
			return path.Base(filepath.ToSlash(word)) == fileName
		})
	})
}

func (g *freezeGuard) planCause(causes *hookconfig.ToolpolicyCauses, plan string) string {
	if g.spentFreezes() >= len(g.designPrompts) {
		return causes.FreezeBudget
	}

	isFrozen := planrules.Latest(plan, g.records) != nil
	if isFrozen && !g.namesPlan(plan) {
		return causes.FreezeReopen
	}
	return ""
}

func (g *freezeGuard) planDirOf(planFile string) string {
	relative, err := filepath.Rel(resolvePath(g.root, ""), resolvePath(g.root, planFile))
	if err != nil {
		return ""
	}
	return planrules.PlanDir(filepath.ToSlash(relative))
}

// spentFreezes counts the ledger's records this session wrote.
func (g *freezeGuard) spentFreezes() int {
	spent := 0
	for _, record := range g.records {
		if record.SessionId == g.sessionId {
			spent++
		}
	}
	return spent
}

func appendRecord(ledger string, record *planrules.Record) error {
	if ledger == "" {
		return errors.New("appendRecord: No frozen ledger configured")
	}
	if err := os.MkdirAll(filepath.Dir(ledger), ledgerDirMode); err != nil {
		return fmt.Errorf("appendRecord: Failed to create ledger dir: %w", err)
	}

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("appendRecord: Failed to encode record: %w", err)
	}

	file, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ledgerFileMode)
	if err != nil {
		return fmt.Errorf("appendRecord: Failed to open ledger: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appendRecord: Failed to write ledger: %w", err)
	}
	return nil
}

func designPrompts(scan *transcriptScan, designSkill string) []string {
	if os.Getenv(headlessEnv) != headlessEnvValue {
		return scan.designPrompts
	}

	skill, args, isCommand := transcript.SlashCommand(scan.firstPrompt)
	isDesignRun := isCommand && skill == designSkill
	if !isDesignRun {
		return nil
	}
	return []string{args}
}

func freezePlanRules(input *HookInput, policy *toolpolicy.Policy, grants *Grants, statePath string, args []string) Verdict {
	anchor := grants.anchorRoot(input)
	if anchor == "" {
		return grants.denyVerdict(policy, toolNameBash, input, policy.Causes.AnchorRemoved)
	}
	if len(args) != 1 {
		return grants.denyVerdict(policy, toolNameBash, input, policy.Causes.SettingsUsage)
	}

	guard := grants.freeze
	plan := planDirPath(input.Cwd, guard.root, args[0])
	if plan == "" {
		return grants.denyVerdict(policy, toolNameBash, input, policy.Causes.FreezeTarget)
	}
	if cause := guard.planCause(&policy.Causes, plan); cause != "" {
		return grants.denyVerdict(policy, toolNameBash, input, cause)
	}

	record, cause := recordFreeze(anchor, guard, input, policy, plan)
	if cause != "" {
		return grants.denyVerdict(policy, toolNameBash, input, cause)
	}

	outcome := "not applied in a headless session"
	if os.Getenv(headlessEnv) != headlessEnvValue {
		outcome = applyRecords(anchor, []*planrules.Record{record}, statePath, policy, grants.trash)[0]
	}
	return frozenVerdict(outcome, record)
}

func frozenLedgerPath(ledgerPath string) string {
	if ledgerPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(ledgerPath), planrules.LedgerName)
}

func frozenVerdict(outcome string, record *planrules.Record) Verdict {
	verdict := Verdict{
		Cause:     causeFreezeApplied,
		Decision:  decisionDeny,
		IsApplied: true,
		Reason: fmt.Sprintf(
			"%s — %s holds %d rule(s): %s. %s. Recorded by the gate; the rules file is write-protected from now on. This call is refused only so Claude Code does not run it. Do not retry.",
			causeFreezeApplied,
			record.Plan,
			len(record.Rules),
			strings.Join(record.RuleStrings(), " "),
			outcome,
		),
	}
	return verdict
}

func loadRulesFile(location string) (*planrules.File, error) {
	file, err := planrules.LoadFile(location)
	if errors.Is(err, os.ErrNotExist) {
		empty := &planrules.File{Checked: []string{}, Rules: []*planrules.Rule{}}
		return empty, nil
	}
	return file, err
}

func planDirPath(cwd string, root string, target string) string {
	relative, err := filepath.Rel(resolvePath(root, ""), resolvePath(cwd, target))
	if err != nil {
		return ""
	}

	slashed := filepath.ToSlash(relative)
	if !planrules.IsPlanDir(slashed) {
		return ""
	}
	return slashed
}

func recordFreeze(anchor string, guard *freezeGuard, input *HookInput, policy *toolpolicy.Policy, plan string) (*planrules.Record, string) {
	file, err := loadRulesFile(filepath.Join(guard.root, filepath.FromSlash(plan), planrules.FileName))
	if err != nil {
		return nil, planrules.FileName + " does not load: " + err.Error()
	}
	if cause := ruleRowsCause(file, input, policy); cause != "" {
		return nil, cause
	}

	record := &planrules.Record{
		File:      file,
		Plan:      plan,
		Repo:      gitroot.MainRoot(anchor),
		SessionId: input.SessionId,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := appendRecord(guard.ledger, record); err != nil {
		return nil, "frozen ledger unwritable: " + err.Error()
	}

	guard.records = append(guard.records, record)
	return record, ""
}

func ruleRowsCause(file *planrules.File, input *HookInput, policy *toolpolicy.Policy) string {
	rules := file.RuleStrings()
	if len(rules) == 0 {
		return ""
	}
	if _, err := checkNewRules(rules, nil, policy); err != nil {
		return planrules.FileName + " rules rejected: " + err.Error()
	}

	for _, row := range file.Rules {
		probe := checkProbe(input, policy, row.Target)
		expected := NarrowestRule(probe.ToolName, &probe.ToolInput, policy.GitAdmittedFlags)
		if row.Rule != expected {
			return fmt.Sprintf("%s rule %q is not the gate's rule for its target %q — expected %s", planrules.FileName, row.Rule, row.Target, expected)
		}
	}
	return ""
}
