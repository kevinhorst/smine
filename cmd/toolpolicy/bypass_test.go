package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/planrules"
)

const (
	bypassOtherDir = "plans/2026-10-02-other-1a2b3c4/2026-10-02-other-wip-1a2b3c4"
	bypassProbeDir = "plans/2026-10-02-probe-7ea30b5/2026-10-02-probe-wip-7ea30b5"
	// codeGatePlanContent is a plan whose Changes hold an entry: a plan at its
	// code gate.
	codeGatePlanContent = "# Probe\n\n## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `probe.go` | new | [Probe unit](#probe-unit-new) |\n\n" +
		"### Probe unit (new)\n\nlocation: `probe.go`\nphase: 1\n"
)

// bypassProbePlan is the probe plan dir's plan file.
var bypassProbePlan = bypassProbeDir + "/design/probe.md"

// allowList is the settings file's allow list; a missing file is empty.
func allowList(t *testing.T, path string) []string {
	t.Helper()
	_, allow, err := allowedRules(path)
	require.NoError(t, err)
	return allow
}

// bypassGrants is LoadGrants with the error required away.
func bypassGrants(t *testing.T, input *HookInput, home string, ledger string) *Grants {
	t.Helper()
	grants, err := LoadGrants(
		input,
		defaultHookConfig(t),
		home,
		"",
		"",
		ledger,
	)
	require.NoError(t, err)
	return grants
}

// bypassHeadlessGrants loads grants with the headless marker set, as a
// headless session's gate reads it, and clears the marker again.
func bypassHeadlessGrants(t *testing.T, input *HookInput, home string, ledger string) *Grants {
	t.Helper()
	t.Setenv(headlessEnv, headlessEnvValue)
	grants := bypassGrants(t, input, home, ledger)
	t.Setenv(headlessEnv, "")
	return grants
}

// bypassLine is one user prompt line with the given origin stamp; "" leaves
// the stamp out.
func bypassLine(t *testing.T, origin string, text string) string {
	t.Helper()
	encoded, err := json.Marshal(text)
	require.NoError(t, err)

	stamp := ""
	if origin != "" {
		stamp = `"turnOrigin":"` + origin + `",`
	}
	return `{"type":"user",` + stamp + `"message":{"role":"user","content":` + string(encoded) + `}}`
}

// bypassRecord is a ledger record of plan in root's main checkout, written by
// sessionId and holding rows.
func bypassRecord(root string, plan string, sessionId string, rows ...*planrules.Rule) *planrules.Record {
	file := &planrules.File{Checked: []string{"go test ./..."}, Rules: rows}
	if rows == nil {
		file.Rules = []*planrules.Rule{}
	}

	record := &planrules.Record{File: file, Plan: plan, Repo: gitroot.MainRoot(root), SessionId: sessionId}
	return record
}

// bypassTranscript writes a transcript whose first line sets the cwd to root,
// followed by lines.
func bypassTranscript(t *testing.T, root string, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	first := `{"type":"system","cwd":"` + root + `"}`
	content := strings.Join(append([]string{first}, lines...), "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// designLine is a human-typed /fdesign line in the Desktop command-name shape.
func designLine(args string) string {
	content := "<command-message>fdesign</command-message>\n<command-name>/fdesign</command-name>\n<command-args>" + args + "</command-args>"
	encoded, _ := json.Marshal(content)
	return `{"type":"user","turnOrigin":"human","message":{"role":"user","content":` + string(encoded) + `}}`
}

// planCheckLine is an assistant line whose Bash call runs the section check on file.
func planCheckLine(t *testing.T, file string) string {
	t.Helper()
	encoded, err := json.Marshal("smine-rules plan-sections " + file)
	require.NoError(t, err)
	return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"check","name":"Bash","input":{"command":` + string(encoded) + `}}]}}`
}

// writeFrozenLedger writes a ledger holding records and returns its path; no
// records leave the ledger absent, as on a machine that never froze. Lines are
// counted by ledgerLines in decide_test.go.
func writeFrozenLedger(t *testing.T, records ...*planrules.Record) string {
	t.Helper()
	ledger := filepath.Join(t.TempDir(), planrules.LedgerName)
	for _, record := range records {
		require.NoError(t, appendRecord(ledger, record))
	}
	return ledger
}

// writeRulesFile writes the probe plan dir's rules file holding rows.
func writeRulesFile(t *testing.T, root string, rows ...*planrules.Rule) {
	t.Helper()
	file := &planrules.File{Checked: []string{}, Rules: rows}
	encoded, err := json.Marshal(file)
	require.NoError(t, err)
	writePlanFile(t, root, bypassProbeDir+"/"+planrules.FileName, string(encoded))
}

// TestPlanRulesBypass_Freeze drives toolpolicy freeze through every known way
// to get a plan's rules frozen without a design prompt that covers them.
func TestPlanRulesBypass_Freeze(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAllow    []string
		_expectedDecision string
		_expectedLines    int
		_expectedReason   string

		grants       *Grants
		headless     string
		input        *HookInput
		ledger       string
		settingsPath string
		statePath    string
	}

	t.Setenv(headlessEnv, "")
	policy := defaultToolpolicy(t)
	home := fixtureHome(t)
	stateDir := t.TempDir()
	probeDir := bypassProbeDir
	lsofRow := &planrules.Rule{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"}
	freezeInput := func(root string, sessionId string, command string, transcriptPath string) *HookInput {
		input := &HookInput{
			Cwd:            root,
			PermissionMode: permissionModeAcceptEdits,
			SessionId:      sessionId,
			ToolInput:      ToolInput{Command: command},
			ToolName:       toolNameBash,
			TranscriptPath: transcriptPath,
		}
		return input
	}
	tests := make([]*testCase, 0)

	// design-prompt-freezes
	root := planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger := writeFrozenLedger(t)
	input := freezeInput(root, "design-prompt-freezes", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "design-prompt-freezes",
		_expectedAllow:    []string{"Bash(lsof *)"},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeApplied + " — " + probeDir + " holds 1 rule(s): Bash(lsof *)",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "design-prompt-freezes.json"),
	})

	// empty-set-freezes
	root = planRepo(t)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "empty-set-freezes", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "empty-set-freezes",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeApplied + " — " + probeDir + " holds 0 rule(s)",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "empty-set-freezes.json"),
	})

	// headless-first-prompt-freezes
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "headless-first-prompt-freezes", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "sdk", "/fdesign "+bypassProbePlan)))
	tests = append(tests, &testCase{
		_id:               "headless-first-prompt-freezes",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeApplied + " — " + probeDir + " holds 1 rule(s): Bash(lsof *). not applied in a headless session",

		grants:       bypassHeadlessGrants(t, input, home, ledger),
		headless:     headlessEnvValue,
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "headless-first-prompt-freezes.json"),
	})

	// frozen-plan-named-refreezes
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "earlier-design"))
	input = freezeInput(root, "frozen-plan-named-refreezes", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine(bypassProbePlan)))
	tests = append(tests, &testCase{
		_id:               "frozen-plan-named-refreezes",
		_expectedAllow:    []string{"Bash(lsof *)"},
		_expectedDecision: decisionDeny,
		_expectedLines:    2,
		_expectedReason:   causeFreezeApplied + " — " + probeDir + " holds 1 rule(s): Bash(lsof *)",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "frozen-plan-named-refreezes.json"),
	})

	// no-design-prompt
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "no-design-prompt", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "human", "freeze the probe plan")))
	tests = append(tests, &testCase{
		_id:               "no-design-prompt",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "no-design-prompt.json"),
	})

	// agent-called-fdesign
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	skillCall := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Skill","input":{"skill":"fdesign","args":"` + bypassProbePlan + `"}}]}}`
	input = freezeInput(root, "agent-called-fdesign", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "human", "plan the probe"), skillCall))
	tests = append(tests, &testCase{
		_id:               "agent-called-fdesign",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "agent-called-fdesign.json"),
	})

	// scheduled-prompt
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	scheduled := bypassLine(t, "", "<command-message>fdesign</command-message>\n<command-name>/fdesign</command-name>\n<command-args>"+bypassProbePlan+"</command-args>")
	input = freezeInput(root, "scheduled-prompt", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "human", "hi"), scheduled))
	tests = append(tests, &testCase{
		_id:               "scheduled-prompt",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "scheduled-prompt.json"),
	})

	// peer-prompt
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	peer := `{"type":"attachment","attachment":{"type":"queued_command","prompt":"/fdesign ` + bypassProbePlan + `","commandMode":"prompt","origin":{"kind":"peer","from":"uds:/tmp/cc-socks-502/1.sock"},"isMeta":true}}`
	input = freezeInput(root, "peer-prompt", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "human", "hi"), peer))
	tests = append(tests, &testCase{
		_id:               "peer-prompt",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "peer-prompt.json"),
	})

	// sdk-prompt-not-headless
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "sdk-prompt-not-headless", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "sdk", "/fdesign "+bypassProbePlan)))
	tests = append(tests, &testCase{
		_id:               "sdk-prompt-not-headless",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "sdk-prompt-not-headless.json"),
	})

	// headless-first-prompt-other-skill
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "headless-first-prompt-other-skill", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, bypassLine(t, "sdk", "/fimplement "+bypassProbePlan), bypassLine(t, "sdk", "/fdesign "+bypassProbePlan)))
	tests = append(tests, &testCase{
		_id:               "headless-first-prompt-other-skill",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassHeadlessGrants(t, input, home, ledger),
		headless:     headlessEnvValue,
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "headless-first-prompt-other-skill.json"),
	})

	// second-freeze-on-one-prompt
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, bypassOtherDir, "second-freeze-on-one-prompt"))
	input = freezeInput(root, "second-freeze-on-one-prompt", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "second-freeze-on-one-prompt",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "second-freeze-on-one-prompt.json"),
	})

	// invented-plan-dir-after-freeze
	root = planRepo(t)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "invented-plan-dir-after-freeze"))
	input = freezeInput(root, "invented-plan-dir-after-freeze", "toolpolicy freeze plans/2026-10-02-invented-0000000/2026-10-02-invented-wip-0000000", bypassTranscript(t, root, designLine(bypassProbePlan)))
	tests = append(tests, &testCase{
		_id:               "invented-plan-dir-after-freeze",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeBudget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "invented-plan-dir-after-freeze.json"),
	})

	// frozen-plan-not-named
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "earlier-design"))
	input = freezeInput(root, "frozen-plan-not-named", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("refine the other plan")))
	tests = append(tests, &testCase{
		_id:               "frozen-plan-not-named",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    1,
		_expectedReason:   causeFreezeReopen,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "frozen-plan-not-named.json"),
	})

	// rule-wider-than-target
	root = planRepo(t)
	writeRulesFile(t, root, &planrules.Rule{NeededFor: "V1", Rule: "Bash(lsof*)", Target: "lsof -i :6101"})
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "rule-wider-than-target", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "rule-wider-than-target",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   planrules.FileName + " rule",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "rule-wider-than-target.json"),
	})

	// rule-without-target
	root = planRepo(t)
	writeRulesFile(t, root, &planrules.Rule{NeededFor: "V1", Rule: "Bash(lsof *)", Target: ""})
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "rule-without-target", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "rule-without-target",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   "is not the gate's rule for its target",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "rule-without-target.json"),
	})

	// wrapper-rule
	root = planRepo(t)
	writeRulesFile(t, root, &planrules.Rule{NeededFor: "V1", Rule: "Bash(xargs *)", Target: "xargs lsof"})
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "wrapper-rule", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "wrapper-rule",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   planrules.FileName + " rules rejected",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "wrapper-rule.json"),
	})

	// gate-binary-rule
	root = planRepo(t)
	writeRulesFile(t, root, &planrules.Rule{NeededFor: "V1", Rule: "Bash(toolpolicy freeze *)", Target: "toolpolicy freeze " + bypassOtherDir})
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "gate-binary-rule", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "gate-binary-rule",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   planrules.FileName + " rules rejected",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "gate-binary-rule.json"),
	})

	// unknown-key
	root = planRepo(t)
	writePlanFile(t, root, probeDir+"/"+planrules.FileName, `{"checked":[],"rules":[],"extra":["Bash(kill *)"]}`)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "unknown-key", "toolpolicy freeze "+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "unknown-key",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   planrules.FileName + " does not load",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "unknown-key.json"),
	})

	// target-outside-worktree
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "target-outside-worktree", "toolpolicy freeze ../"+probeDir, bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "target-outside-worktree",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeTarget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "target-outside-worktree.json"),
	})

	// target-design-dir
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "target-design-dir", "toolpolicy freeze "+probeDir+"/design", bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "target-design-dir",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeTarget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "target-design-dir.json"),
	})

	// target-done-plan-dir
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-10-02-probe-7ea30b5/2026-10-02-probe-done-7ea30b5/"+planrules.FileName, `{"checked":[],"rules":[{"needed_for":"V1","rule":"Bash(lsof *)","target":"lsof -i :6101"}]}`)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "target-done-plan-dir", "toolpolicy freeze plans/2026-10-02-probe-7ea30b5/2026-10-02-probe-done-7ea30b5", bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "target-done-plan-dir",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeTarget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "target-done-plan-dir.json"),
	})

	// target-old-depth-plan-dir
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-10-02-probe-wip-7ea30b5/"+planrules.FileName, `{"checked":[],"rules":[{"needed_for":"V1","rule":"Bash(lsof *)","target":"lsof -i :6101"}]}`)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "target-old-depth-plan-dir", "toolpolicy freeze plans/2026-10-02-probe-wip-7ea30b5", bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "target-old-depth-plan-dir",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeTarget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "target-old-depth-plan-dir.json"),
	})

	// target-feature-folder
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "target-feature-folder", "toolpolicy freeze plans/2026-10-02-probe-7ea30b5", bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "target-feature-folder",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeFreezeTarget,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "target-feature-folder.json"),
	})

	// piped-call-usage-deny
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "piped-call-usage-deny", "toolpolicy freeze "+probeDir+" | cat", bypassTranscript(t, root, designLine("")))
	tests = append(tests, &testCase{
		_id:               "piped-call-usage-deny",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeSettingsUsage,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "piped-call-usage-deny.json"),
	})

	// anchor-removed
	root = planRepo(t)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = freezeInput(root, "anchor-removed", "toolpolicy freeze "+probeDir, bypassTranscript(t, filepath.Join(root, "removed"), designLine("")))
	tests = append(tests, &testCase{
		_id:               "anchor-removed",
		_expectedAllow:    []string{},
		_expectedDecision: decisionDeny,
		_expectedLines:    0,
		_expectedReason:   causeAnchorRemoved,

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "anchor-removed.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(headlessEnv, test.headless)
			verdict := Decide(test.input, policy, test.grants, home, test.statePath)

			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Contains(t, verdict.Reason, test._expectedReason)
			assert.Equal(t, test._expectedAllow, allowList(t, test.settingsPath))
			assert.Equal(t, test._expectedLines, ledgerLines(t, test.ledger))
		})
	}
}

// TestPlanRulesBypass_Hook drives the /fimplement hook through every known
// way to get rules applied that the plan's frozen record does not hold.
func TestPlanRulesBypass_Hook(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedAllow []string
		_expectedLines int
		_expectedOut   string

		grants       *Grants
		headless     string
		input        *HookInput
		ledger       string
		settingsPath string
		statePath    string
	}

	t.Setenv(headlessEnv, "")
	home := fixtureHome(t)
	stateDir := t.TempDir()
	policy := defaultToolpolicy(t)

	probeDir := bypassProbeDir
	lsofRow := &planrules.Rule{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"}
	killRow := &planrules.Rule{NeededFor: "V1", Rule: "Bash(kill *)", Target: "kill 1"}
	appliedReport := func(root string, applied string) string {
		return planHookHeader + "\n- plan: " + filepath.Join(root, bypassProbePlan) + "\n- " + applied + "\n- checked: go test ./..."
	}
	notFrozenReport := func(root string) string {
		return planHookHeader + "\n- plan: " + filepath.Join(root, bypassProbePlan) + "\n- Permissions: none applied — " + probeDir + " is not frozen; a design run freezes it: toolpolicy freeze " + probeDir + "\n"
	}
	contextJson := func(report string) string {
		output := &HookOutput{HookSpecificOutput: HookDecision{AdditionalContext: report, HookEventName: hookEventPostToolUse}}
		encoded, err := json.Marshal(output)
		require.NoError(t, err)
		return string(encoded) + "\n"
	}
	promptInput := func(root string) *HookInput {
		input := &HookInput{
			Cwd:            root,
			HookEventName:  hookEventUserPromptSubmit,
			Prompt:         "/fimplement probe.md",
			SessionId:      "implement",
			TranscriptPath: bypassTranscript(t, root, bypassLine(t, "human", "/fimplement probe.md")),
		}
		return input
	}
	designRunPrompt := func(root string, sessionId string) *HookInput {
		input := &HookInput{
			Cwd:            root,
			HookEventName:  hookEventUserPromptSubmit,
			Prompt:         "/fimplement",
			SessionId:      sessionId,
			TranscriptPath: bypassTranscript(t, root, designLine(""), planCheckLine(t, bypassProbePlan)),
		}
		return input
	}
	lsofApplied := "Permissions: appended 1 rule(s), 0 already present — Bash(lsof *)"
	tests := make([]*testCase, 0)

	// record-applies
	root := planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	ledger := writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	input := promptInput(root)
	tests = append(tests, &testCase{
		_id:            "record-applies",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut:   appliedReport(root, lsofApplied) + "\n",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "record-applies.json"),
	})

	// no-record-applies-nothing
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "no-record-applies-nothing",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "no-record-applies-nothing.json"),
	})

	// file-edited-after-freeze-record-wins
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	writeRulesFile(t, root, lsofRow, killRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "file-edited-after-freeze-record-wins",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut:   appliedReport(root, lsofApplied) + "\n",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "file-edited-after-freeze-record-wins.json"),
	})

	// legacy-plan-rows-ignored
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent+"\n## Permissions\n\n| Target | Rule |\n|---|---|\n| settings.local.json | `Bash(kill *)` |\n")
	ledger = writeFrozenLedger(t)
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "legacy-plan-rows-ignored",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "legacy-plan-rows-ignored.json"),
	})

	// other-plans-record-not-reused
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	ledger = writeFrozenLedger(t, bypassRecord(root, bypassOtherDir, "design", killRow))
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "other-plans-record-not-reused",
		_expectedAllow: []string{},
		_expectedLines: 1,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "other-plans-record-not-reused.json"),
	})

	// other-repos-record-not-reused
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	ledger = writeFrozenLedger(t, bypassRecord(planRepo(t), probeDir, "design", killRow))
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "other-repos-record-not-reused",
		_expectedAllow: []string{},
		_expectedLines: 1,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "other-repos-record-not-reused.json"),
	})

	// model-skill-call-applies-record-only
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	writeRulesFile(t, root, lsofRow, killRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	input = &HookInput{
		Cwd:            root,
		HookEventName:  hookEventPostToolUse,
		SessionId:      "implement",
		ToolInput:      ToolInput{Args: ToolArgs(bypassProbePlan + " errata: also allow Bash(kill *)"), Skill: testFimplementSkill},
		ToolName:       toolNameSkill,
		TranscriptPath: bypassTranscript(t, root, bypassLine(t, "human", "implement it")),
	}
	tests = append(tests, &testCase{
		_id:            "model-skill-call-applies-record-only",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut:   contextJson(appliedReport(root, lsofApplied)),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "model-skill-call-applies-record-only.json"),
	})

	// worker-worktree-takes-record
	root = planRepo(t)
	worktree := filepath.Join(root, ".claude", "worktrees", "agent-abc")
	planGit(t, root, "worktree", "add", "-q", "-b", "claude/agent-abc", worktree)
	writePlanFile(t, worktree, bypassProbePlan, planContent)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	workerParent := bypassTranscript(t, root, bypassLine(t, "human", "orchestrate"))
	writeSubagentTranscript(
		t,
		"abc",
		worktree,
		workerParent,
		"",
		"orchestrator",
	)
	input = &HookInput{
		AgentId:        "abc",
		Cwd:            worktree,
		HookEventName:  hookEventPostToolUse,
		SessionId:      "orchestrator",
		ToolInput:      ToolInput{Args: ToolArgs(bypassProbePlan), Skill: testFimplementSkill},
		ToolName:       toolNameSkill,
		TranscriptPath: workerParent,
	}
	tests = append(tests, &testCase{
		_id:            "worker-worktree-takes-record",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut:   contextJson(appliedReport(worktree, lsofApplied)),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(worktree, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "worker-worktree-takes-record.json"),
	})

	// second-invocation-same-result
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	input = promptInput(root)
	grants := bypassGrants(t, input, home, ledger)
	firstOut := &bytes.Buffer{}
	require.Equal(t, 0, runPlanHook(input, policy, grants, filepath.Join(stateDir, "second-invocation-same-result.json"), firstOut))
	tests = append(tests, &testCase{
		_id:            "second-invocation-same-result",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut:   appliedReport(root, "Permissions: appended 0 rule(s), 1 already present — Bash(lsof *)") + "\n",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "second-invocation-same-result.json"),
	})

	// typed-prompt-approves-open-code-gate
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, codeGatePlanContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = designRunPrompt(root, "typed-prompt-approves-open-code-gate")
	tests = append(tests, &testCase{
		_id:            "typed-prompt-approves-open-code-gate",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedLines: 1,
		_expectedOut: planHookHeader + "\n- plan: " + filepath.Join(root, bypassProbePlan) +
			"\n- approval: this typed /fimplement approves the code gate — " + probeDir + " frozen with 1 rule(s)" +
			"\n- " + lsofApplied + "\n- checked: \n",

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "typed-prompt-approves-open-code-gate.json"),
	})

	// typed-prompt-at-design-gate-not-approved
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = designRunPrompt(root, "typed-prompt-at-design-gate-not-approved")
	tests = append(tests, &testCase{
		_id:            "typed-prompt-at-design-gate-not-approved",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "typed-prompt-at-design-gate-not-approved.json"),
	})

	// typed-prompt-without-design-prompt-not-approved
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, codeGatePlanContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "typed-prompt-without-design-prompt-not-approved",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "typed-prompt-without-design-prompt-not-approved.json"),
	})

	// typed-prompt-design-budget-spent-not-approved
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, codeGatePlanContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t, bypassRecord(root, bypassOtherDir, "typed-prompt-design-budget-spent-not-approved"))
	input = designRunPrompt(root, "typed-prompt-design-budget-spent-not-approved")
	tests = append(tests, &testCase{
		_id:            "typed-prompt-design-budget-spent-not-approved",
		_expectedAllow: []string{},
		_expectedLines: 1,
		_expectedOut:   notFrozenReport(root),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "typed-prompt-design-budget-spent-not-approved.json"),
	})

	// typed-prompt-wide-rule-not-frozen
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, codeGatePlanContent)
	writeRulesFile(t, root, &planrules.Rule{NeededFor: "V1", Rule: "Bash(lsof*)", Target: "lsof -i :6101"})
	ledger = writeFrozenLedger(t)
	input = designRunPrompt(root, "typed-prompt-wide-rule-not-frozen")
	tests = append(tests, &testCase{
		_id:            "typed-prompt-wide-rule-not-frozen",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut: planHookHeader + "\n- plan: " + filepath.Join(root, bypassProbePlan) +
			"\n- approval: this typed /fimplement approves the code gate, but " + probeDir + " did not freeze — " +
			planrules.FileName + ` rules rejected: isSelfServiceRule: Rule "Bash(lsof*)" names no literal program` +
			strings.TrimPrefix(notFrozenReport(root), planHookHeader+"\n- plan: "+filepath.Join(root, bypassProbePlan)),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "typed-prompt-wide-rule-not-frozen.json"),
	})

	// skill-call-never-approves
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, codeGatePlanContent)
	writeRulesFile(t, root, lsofRow)
	ledger = writeFrozenLedger(t)
	input = &HookInput{
		Cwd:            root,
		HookEventName:  hookEventPostToolUse,
		SessionId:      "skill-call-never-approves",
		ToolInput:      ToolInput{Args: ToolArgs(bypassProbePlan), Skill: testFimplementSkill},
		ToolName:       toolNameSkill,
		TranscriptPath: bypassTranscript(t, root, designLine(""), planCheckLine(t, filepath.Join(root, bypassProbePlan))),
	}
	tests = append(tests, &testCase{
		_id:            "skill-call-never-approves",
		_expectedAllow: []string{},
		_expectedLines: 0,
		_expectedOut:   contextJson(strings.TrimSuffix(notFrozenReport(root), "\n")),

		grants:       bypassGrants(t, input, home, ledger),
		headless:     "",
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "skill-call-never-approves.json"),
	})

	// headless-applies-nothing
	root = planRepo(t)
	writePlanFile(t, root, bypassProbePlan, planContent)
	ledger = writeFrozenLedger(t, bypassRecord(root, probeDir, "design", lsofRow))
	input = promptInput(root)
	tests = append(tests, &testCase{
		_id:            "headless-applies-nothing",
		_expectedAllow: []string{},
		_expectedLines: 1,
		_expectedOut:   "",

		grants:       bypassHeadlessGrants(t, input, home, ledger),
		headless:     headlessEnvValue,
		input:        input,
		ledger:       ledger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "headless-applies-nothing.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(headlessEnv, test.headless)
			stdout := &bytes.Buffer{}
			exit := runPlanHook(test.input, policy, test.grants, test.statePath, stdout)

			assert.Equal(t, 0, exit)
			assert.Equal(t, test._expectedOut, stdout.String())
			assert.Equal(t, test._expectedAllow, allowList(t, test.settingsPath))
			assert.Equal(t, test._expectedLines, ledgerLines(t, test.ledger))
		})
	}
}
