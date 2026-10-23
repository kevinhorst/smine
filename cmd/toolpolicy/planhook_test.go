package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/planrules"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	otherPlan   = "plans/2026-09-28-other-1a2b3c4/2026-09-28-other-wip-1a2b3c4/design/other.md"
	planContent = "# Probe\n\n## Changes\nnone\n"
	probePlan   = "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/probe.md"
)

func planGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_, err := shell.Run(context.Background(), root, "git", args...)
	require.NoError(t, err)
}

// planRepo is a git repo whose main branch holds one commit and whose
// checked-out branch claude/work starts there; the path is symlink-resolved
// so it equals git's own --show-toplevel.
func planRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	planGit(t, root, "init", "-q", "-b", "main")
	planGit(t, root, "config", "user.email", "test@example.com")
	planGit(t, root, "config", "user.name", "test")
	writePlanFile(t, root, "README.md", "base\n")
	planGit(t, root, "add", ".")
	planGit(t, root, "commit", "-q", "-m", "base")
	planGit(t, root, "checkout", "-q", "-b", "claude/work")
	return root
}

// frozenGrants are the grants of a session anchored at root whose guard holds
// one record of the probe plan dir with rules.
func frozenGrants(t *testing.T, root string, rules ...string) *Grants {
	t.Helper()
	input := &HookInput{Cwd: root, TranscriptPath: planTranscript(t, root)}
	grants, err := LoadGrants(
		input,
		defaultHookConfig(t),
		fixtureHome(t),
		"",
		"",
		frozenLedger(t, root, rules...),
	)
	require.NoError(t, err)
	return grants
}

// frozenLedger is a frozen ledger holding one record of the probe plan dir
// with rules, keyed to root's main checkout.
func frozenLedger(t *testing.T, root string, rules ...string) string {
	t.Helper()
	ledger := filepath.Join(t.TempDir(), planrules.LedgerName)
	record := frozenRecord(rules...)
	record.Repo = gitroot.MainRoot(root)
	require.NoError(t, appendRecord(ledger, record))
	return ledger
}

// frozenRecord is a record of the probe plan dir holding rules, each with
// itself as target.
func frozenRecord(rules ...string) *planrules.Record {
	file := &planrules.File{Checked: []string{}, Rules: make([]*planrules.Rule, 0, len(rules))}
	for _, rule := range rules {
		file.Rules = append(file.Rules, &planrules.Rule{NeededFor: "probe", Rule: rule, Target: rule})
	}

	record := &planrules.Record{File: file, Plan: planrules.PlanDir(probePlan), SessionId: "design"}
	return record
}

// planTranscript is a one-line transcript whose first cwd is dir.
func planTranscript(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	line := `{"type":"user","turnOrigin":"human","cwd":"` + dir + `","message":{"role":"user","content":"hi"}}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(line), 0o644))
	return path
}

func writePlanFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestFimplementArgs(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedArgs       string
		_expectedInvocation bool

		input *HookInput
	}

	tests := make([]*testCase, 0)

	// prompt-bare
	tests = append(tests, &testCase{
		_id:                 "prompt-bare",
		_expectedArgs:       "",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement"},
	})

	// prompt-with-args
	tests = append(tests, &testCase{
		_id:                 "prompt-with-args",
		_expectedArgs:       probePlan + " errata: none",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement " + probePlan + " errata: none"},
	})

	// prompt-args-span-lines
	tests = append(tests, &testCase{
		_id:                 "prompt-args-span-lines",
		_expectedArgs:       probePlan + "\n" + otherPlan,
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement " + probePlan + "\n" + otherPlan},
	})

	// prompt-args-on-later-line
	tests = append(tests, &testCase{
		_id:                 "prompt-args-on-later-line",
		_expectedArgs:       probePlan,
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement\n \n " + probePlan + "\n"},
	})

	// prompt-other-skill-suffix-no-match
	tests = append(tests, &testCase{
		_id:                 "prompt-other-skill-suffix-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplementx " + probePlan},
	})

	// prompt-other-skill-no-match
	tests = append(tests, &testCase{
		_id:                 "prompt-other-skill-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "/fdesign " + probePlan},
	})

	// prompt-leading-space
	tests = append(tests, &testCase{
		_id:                 "prompt-leading-space",
		_expectedArgs:       probePlan,
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "  /fimplement " + probePlan},
	})

	// prompt-word-on-later-line
	tests = append(tests, &testCase{
		_id:                 "prompt-word-on-later-line",
		_expectedArgs:       probePlan + "\nerrata: none",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "agreed\n\nnot worth a plan\n\n/fimplement " + probePlan + "\nerrata: none\n"},
	})

	// prompt-word-mid-sentence
	tests = append(tests, &testCase{
		_id:                 "prompt-word-mid-sentence",
		_expectedArgs:       "then /done --close",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "looks good, run `/fimplement`, then /done --close"},
	})

	// prompt-word-in-pasted-block
	tests = append(tests, &testCase{
		_id:                 "prompt-word-in-pasted-block",
		_expectedArgs:       "adhoc\n</pasted_content id=\"1\">",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "\n\n<pasted_content id=\"1\">\n/fimplement adhoc\n</pasted_content id=\"1\">\n"},
	})

	// prompt-path-no-match
	tests = append(tests, &testCase{
		_id:                 "prompt-path-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "read skills/feature/fimplement and /skills/fimplement/skill.cue"},
	})

	// prompt-cross-session-wrapper-no-match
	tests = append(tests, &testCase{
		_id:                 "prompt-cross-session-wrapper-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventUserPromptSubmit, Prompt: "<cross-session-message>\n/fimplement " + probePlan + "\n</cross-session-message>"},
	})

	// skill-call-with-args
	tests = append(tests, &testCase{
		_id:                 "skill-call-with-args",
		_expectedArgs:       probePlan + " errata: none",
		_expectedInvocation: true,

		input: &HookInput{HookEventName: hookEventPostToolUse, ToolInput: ToolInput{Args: " " + probePlan + " errata: none ", Skill: testFimplementSkill}, ToolName: toolNameSkill},
	})

	// skill-call-other-skill-no-match
	tests = append(tests, &testCase{
		_id:                 "skill-call-other-skill-no-match",
		_expectedArgs:       probePlan,
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventPostToolUse, ToolInput: ToolInput{Args: probePlan, Skill: "fdesign"}, ToolName: toolNameSkill},
	})

	// posttooluse-non-skill-no-match
	tests = append(tests, &testCase{
		_id:                 "posttooluse-non-skill-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventPostToolUse, ToolInput: ToolInput{Command: "/fimplement"}, ToolName: toolNameBash},
	})

	// pretooluse-no-match
	tests = append(tests, &testCase{
		_id:                 "pretooluse-no-match",
		_expectedArgs:       "",
		_expectedInvocation: false,

		input: &HookInput{HookEventName: hookEventPreToolUse, ToolInput: ToolInput{Args: probePlan, Skill: testFimplementSkill}, ToolName: toolNameSkill},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			args, isInvocation := fimplementArgs(test.input, testFimplementSkill)
			assert.Equal(t, test._expectedArgs, args)
			assert.Equal(t, test._expectedInvocation, isInvocation)
		})
	}
}

func TestResolvePlan(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedNote string
		_expectedPlan string

		args  string
		guard *freezeGuard
		root  string
	}

	noneNote := "no plan named and none section-checked in this session — best-effort mode; name it: /fimplement <plan file>"
	checkedGuard := &freezeGuard{records: []*planrules.Record{}, sectionedPlans: []string{probePlan}, sessionId: "design"}
	designGuard := &freezeGuard{records: []*planrules.Record{frozenRecord()}, sessionId: "design"}
	otherSessionGuard := &freezeGuard{records: []*planrules.Record{frozenRecord()}, sessionId: "implement"}
	emptyGuard := &freezeGuard{records: []*planrules.Record{}, sessionId: "design"}

	tests := make([]*testCase, 0)

	// explicit-arg-wins
	root := planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-arg-wins",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, otherPlan),

		args:  otherPlan + " errata: none",
		guard: emptyGuard,
		root:  root,
	})

	// first-plan-word-wins-across-lines
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "first-plan-word-wins-across-lines",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  "errata: none\n" + probePlan + "\n" + otherPlan,
		guard: emptyGuard,
		root:  root,
	})

	// bare-name-resolves
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "bare-name-resolves",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  "probe.md errata: none",
		guard: emptyGuard,
		root:  root,
	})

	// bare-name-in-two-dirs-lists-both
	root = planRepo(t)
	secondProbePlan := "plans/2026-09-28-probe-7ea30b5/2026-09-29-probe-wip-9f8e7d6/design/probe.md"
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, secondProbePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "bare-name-in-two-dirs-lists-both",
		_expectedNote: "plan not resolved — several wip plan dirs hold probe.md: " + probePlan + ", " + secondProbePlan + "; ask the user which one, they send /fimplement <plan file path>",
		_expectedPlan: "",

		args:  "probe.md",
		guard: emptyGuard,
		root:  root,
	})

	// bare-name-in-feature-folder
	root = planRepo(t)
	derivedPlan := "plans/2026-09-28-merge-strategy-1a2b3c4/2026-09-28-probe-wip-7ea30b5/design/probe.md"
	writePlanFile(t, root, derivedPlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "bare-name-in-feature-folder",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, derivedPlan),

		args:  "probe.md",
		guard: emptyGuard,
		root:  root,
	})

	// old-depth-plan-ignored
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-09-28-probe-wip-7ea30b5/design/probe.md", planContent)
	tests = append(tests, &testCase{
		_id:           "old-depth-plan-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "probe.md",
		guard: emptyGuard,
		root:  root,
	})

	// bare-name-unknown-falls-through
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "bare-name-unknown-falls-through",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  "missing.md",
		guard: checkedGuard,
		root:  root,
	})

	// explicit-non-plan-md-ignored
	root = planRepo(t)
	notesFile := "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/notes.md"
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, notesFile, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-non-plan-md-ignored",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  notesFile,
		guard: checkedGuard,
		root:  root,
	})

	// explicit-outside-checkout-ignored
	root = planRepo(t)
	outsideRoot := planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, outsideRoot, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-outside-checkout-ignored",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  filepath.Join(outsideRoot, otherPlan),
		guard: checkedGuard,
		root:  root,
	})

	// explicit-missing-file-falls-through
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-missing-file-falls-through",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args:  otherPlan,
		guard: checkedGuard,
		root:  root,
	})

	// session-freeze-ignored
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "session-freeze-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: designGuard,
		root:  root,
	})

	// explicit-arg-beats-session-freeze
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-arg-beats-session-freeze",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, otherPlan),

		args:  otherPlan,
		guard: designGuard,
		root:  root,
	})

	// other-session-freeze-ignored
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "other-session-freeze-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: otherSessionGuard,
		root:  root,
	})

	// legacy-names-ignored
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/raw.md", planContent)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/refined.md", planContent)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/change-x.md", planContent)
	tests = append(tests, &testCase{
		_id:           "legacy-names-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: emptyGuard,
		root:  root,
	})

	// foreign-stem-ignored
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/notes.md", planContent)
	tests = append(tests, &testCase{
		_id:           "foreign-stem-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: emptyGuard,
		root:  root,
	})

	// several-plans-none-named
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "several-plans-none-named",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: emptyGuard,
		root:  root,
	})

	// non-design-file-ignored
	root = planRepo(t)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/concept/concept.md", planContent)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/exploration.md", planContent)
	tests = append(tests, &testCase{
		_id:           "non-design-file-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args:  "",
		guard: emptyGuard,
		root:  root,
	})

	// session-checked-plan
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "session-checked-plan",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, otherPlan),

		args: "",
		guard: &freezeGuard{
			records:        []*planrules.Record{},
			sectionedPlans: []string{filepath.Join(root, probePlan), otherPlan},
			sessionId:      "design",
		},
		root: root,
	})

	// checked-plan-beats-session-freeze
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "checked-plan-beats-session-freeze",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, otherPlan),

		args: "",
		guard: &freezeGuard{
			records:        []*planrules.Record{frozenRecord()},
			sectionedPlans: []string{filepath.Join(root, otherPlan)},
			sessionId:      "design",
		},
		root: root,
	})

	// explicit-arg-beats-checked-plan
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, otherPlan, planContent)
	tests = append(tests, &testCase{
		_id:           "explicit-arg-beats-checked-plan",
		_expectedNote: "",
		_expectedPlan: filepath.Join(root, probePlan),

		args: probePlan,
		guard: &freezeGuard{
			records:        []*planrules.Record{},
			sectionedPlans: []string{filepath.Join(root, otherPlan)},
			sessionId:      "design",
		},
		root: root,
	})

	// checked-plan-missing-ignored
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "checked-plan-missing-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args: "",
		guard: &freezeGuard{
			records:        []*planrules.Record{frozenRecord()},
			sectionedPlans: []string{filepath.Join(root, otherPlan)},
			sessionId:      "design",
		},
		root: root,
	})

	// checked-non-plan-files-ignored
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	writePlanFile(t, root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/notes.md", planContent)
	outsideRoot = planRepo(t)
	writePlanFile(t, outsideRoot, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:           "checked-non-plan-files-ignored",
		_expectedNote: noneNote,
		_expectedPlan: "",

		args: "",
		guard: &freezeGuard{
			records: []*planrules.Record{},
			sectionedPlans: []string{
				filepath.Join(root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/notes.md"),
				filepath.Join(root, "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5", planrules.FileName),
				filepath.Join(outsideRoot, probePlan),
			},
			sessionId: "design",
		},
		root: root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			plan, note := resolvePlan(test.root, test.root, test.args, test.guard)
			assert.Equal(t, test._expectedPlan, plan)
			assert.Equal(t, test._expectedNote, note)
		})
	}
}

// linePrefixes cuts each line to the length of its expected prefix, so a
// table can pin the stable head of a line whose tail carries an OS error.
func linePrefixes(lines []string, prefixes []string) []string {
	cut := make([]string, 0, len(lines))
	for i, line := range lines {
		if i < len(prefixes) && len(line) > len(prefixes[i]) {
			line = line[:len(prefixes[i])]
		}
		cut = append(cut, line)
	}
	return cut
}

func TestApplyRecords(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      []string
		_expectedAllow []string
		_expectedState *sessionstate.SessionState

		anchor    string
		records   []*planrules.Record
		statePath string
	}

	base := t.TempDir()
	withProbe := `{"permissions": {"allow": ["Bash(ls-probe *)"]}}`
	lsofState := &sessionstate.SessionState{SettingsLocal: sessionstate.SelfServiceState{IsReverted: false, LastAppended: []string{"Bash(lsof *)"}}}

	tests := make([]*testCase, 0)

	// appends-fresh
	anchor := selfServiceCwd(t, base, "appends-fresh", withProbe)
	record := frozenRecord("Bash(lsof *)", "WebFetch(domain:docs.rs)")
	tests = append(tests, &testCase{
		_id:            "appends-fresh",
		_expected:      []string{"Permissions: appended 2 rule(s), 0 already present — Bash(lsof *) WebFetch(domain:docs.rs)"},
		_expectedAllow: []string{"Bash(ls-probe *)", "Bash(lsof *)", "WebFetch(domain:docs.rs)"},
		_expectedState: &sessionstate.SessionState{SettingsLocal: sessionstate.SelfServiceState{IsReverted: false, LastAppended: []string{"Bash(lsof *)", "WebFetch(domain:docs.rs)"}}},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "appends-fresh.json"),
	})

	// already-present-counted
	anchor = selfServiceCwd(t, base, "already-present-counted", withProbe)
	record = frozenRecord("Bash(ls-probe *)")
	tests = append(tests, &testCase{
		_id:            "already-present-counted",
		_expected:      []string{"Permissions: appended 0 rule(s), 1 already present — Bash(ls-probe *)"},
		_expectedAllow: []string{"Bash(ls-probe *)"},
		_expectedState: &sessionstate.SessionState{},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "already-present-counted.json"),
	})

	// no-rows
	anchor = selfServiceCwd(t, base, "no-rows", withProbe)
	record = frozenRecord()
	tests = append(tests, &testCase{
		_id:            "no-rows",
		_expected:      []string{"Permissions: " + planrules.PlanDir(probePlan) + " is frozen with no rule"},
		_expectedAllow: []string{"Bash(ls-probe *)"},
		_expectedState: &sessionstate.SessionState{},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "no-rows.json"),
	})

	// wrapper-row-rejects-all
	anchor = selfServiceCwd(t, base, "wrapper-row-rejects-all", withProbe)
	record = frozenRecord("Bash(lsof *)", "Bash(xargs *)")
	tests = append(tests, &testCase{
		_id:            "wrapper-row-rejects-all",
		_expected:      []string{"Permissions: none applied — frozen rule rejected: isSelfServiceRule: Program \"xargs\" is a wrapper — the rule would admit every command; report it to the user"},
		_expectedAllow: []string{"Bash(ls-probe *)"},
		_expectedState: &sessionstate.SessionState{},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "wrapper-row-rejects-all.json"),
	})

	// repeated-row-rejects-all
	anchor = selfServiceCwd(t, base, "repeated-row-rejects-all", withProbe)
	record = frozenRecord("Bash(lsof *)", "Bash(lsof *)")
	tests = append(tests, &testCase{
		_id:            "repeated-row-rejects-all",
		_expected:      []string{"Permissions: none applied — frozen rule rejected: checkNewRules: Rule \"Bash(lsof *)\" is repeated; report it to the user"},
		_expectedAllow: []string{"Bash(ls-probe *)"},
		_expectedState: &sessionstate.SessionState{},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "repeated-row-rejects-all.json"),
	})

	// broken-local-file
	anchor = selfServiceCwd(t, base, "broken-local-file", "{")
	record = frozenRecord("Bash(lsof *)")
	tests = append(tests, &testCase{
		_id:            "broken-local-file",
		_expected:      []string{"Permissions: none applied — settings.local.json does not load, the user fixes it: "},
		_expectedAllow: nil,
		_expectedState: &sessionstate.SessionState{},

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "broken-local-file.json"),
	})

	// records-last-appended
	anchor = selfServiceCwd(t, base, "records-last-appended", "")
	record = frozenRecord("Bash(lsof *)")
	tests = append(tests, &testCase{
		_id:            "records-last-appended",
		_expected:      []string{"Permissions: appended 1 rule(s), 0 already present — Bash(lsof *)"},
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedState: lsofState,

		anchor:    anchor,
		records:   []*planrules.Record{record},
		statePath: filepath.Join(base, "state", "records-last-appended.json"),
	})

	// several-records-one-revert
	anchor = selfServiceCwd(t, base, "several-records-one-revert", withProbe)
	tests = append(tests, &testCase{
		_id: "several-records-one-revert",
		_expected: []string{
			"Permissions: appended 1 rule(s), 0 already present — Bash(lsof *)",
			"Permissions: appended 1 rule(s), 1 already present — Bash(lsof *) WebFetch(domain:docs.rs)",
		},
		_expectedAllow: []string{"Bash(ls-probe *)", "Bash(lsof *)", "WebFetch(domain:docs.rs)"},
		_expectedState: &sessionstate.SessionState{SettingsLocal: sessionstate.SelfServiceState{IsReverted: false, LastAppended: []string{"Bash(lsof *)", "WebFetch(domain:docs.rs)"}}},

		anchor:    anchor,
		records:   []*planrules.Record{frozenRecord("Bash(lsof *)"), frozenRecord("Bash(lsof *)", "WebFetch(domain:docs.rs)")},
		statePath: filepath.Join(base, "state", "several-records-one-revert.json"),
	})

	// rejected-record-others-apply
	anchor = selfServiceCwd(t, base, "rejected-record-others-apply", withProbe)
	tests = append(tests, &testCase{
		_id: "rejected-record-others-apply",
		_expected: []string{
			"Permissions: none applied — frozen rule rejected: isSelfServiceRule: Program \"xargs\" is a wrapper",
			"Permissions: appended 1 rule(s), 0 already present — Bash(lsof *)",
		},
		_expectedAllow: []string{"Bash(ls-probe *)", "Bash(lsof *)"},
		_expectedState: lsofState,

		anchor:    anchor,
		records:   []*planrules.Record{frozenRecord("Bash(xargs *)"), frozenRecord("Bash(lsof *)")},
		statePath: filepath.Join(base, "state", "rejected-record-others-apply.json"),
	})

	policy := defaultToolpolicy(t)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			lines := applyRecords(test.anchor, test.records, test.statePath, policy, nil)
			_, allow, _ := allowedRules(filepath.Join(test.anchor, ".claude", unlockSettings))
			state, err := sessionstate.Load(test.statePath)
			assert.Equal(t, test._expected, linePrefixes(lines, test._expected))
			assert.Equal(t, test._expectedAllow, allow)
			require.NoError(t, err)
			assert.Equal(t, test._expectedState, state)
		})
	}
}

func TestRunPlanHook(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedAllow []string
		_expectedExit  int
		_expectedOut   string

		input        *HookInput
		ledger       string
		settingsPath string
		statePath    string
	}

	t.Setenv(headlessEnv, "")
	home := fixtureHome(t)
	stateDir := t.TempDir()
	policy := defaultToolpolicy(t)

	rowReport := func(root string) string {
		return planHookHeader + "\n- plan: " + filepath.Join(root, probePlan) + "\n- Permissions: appended 1 rule(s), 0 already present — Bash(lsof *)\n- checked: "
	}
	contextJson := func(report string) string {
		output := &HookOutput{HookSpecificOutput: HookDecision{AdditionalContext: report, HookEventName: hookEventPostToolUse}}
		encoded, err := json.Marshal(output)
		require.NoError(t, err)
		return string(encoded) + "\n"
	}

	tests := make([]*testCase, 0)

	// non-fimplement-silent
	root := planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:            "non-fimplement-silent",
		_expectedAllow: []string{},
		_expectedExit:  0,
		_expectedOut:   "",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fdesign", TranscriptPath: planTranscript(t, root)},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "non-fimplement-silent.json"),
	})

	// prompt-prints-plain-report
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:            "prompt-prints-plain-report",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut:   rowReport(root) + "\n",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement", SessionId: "design", TranscriptPath: bypassTranscript(t, root, planCheckLine(t, probePlan))},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "prompt-prints-plain-report.json"),
	})

	// skill-call-emits-additional-context-json
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:            "skill-call-emits-additional-context-json",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut:   contextJson(rowReport(root)),

		input: &HookInput{
			Cwd:            root,
			HookEventName:  hookEventPostToolUse,
			ToolInput:      ToolInput{Args: probePlan, Skill: testFimplementSkill},
			ToolName:       toolNameSkill,
			TranscriptPath: planTranscript(t, root),
		},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "skill-call-emits-additional-context-json.json"),
	})

	// skill-call-agent-worktree-anchor
	root = planRepo(t)
	worktree := filepath.Join(root, ".claude", "worktrees", "agent-abc")
	planGit(t, root, "worktree", "add", "-q", "-b", "claude/agent-abc", worktree)
	writePlanFile(t, worktree, probePlan, planContent)
	workerParent := planTranscript(t, root)
	writeSubagentTranscript(
		t,
		"abc",
		worktree,
		workerParent,
		"",
		"orchestrator",
	)
	tests = append(tests, &testCase{
		_id:            "skill-call-agent-worktree-anchor",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut:   contextJson(rowReport(worktree)),

		input: &HookInput{
			AgentId:        "abc",
			Cwd:            worktree,
			HookEventName:  hookEventPostToolUse,
			SessionId:      "orchestrator",
			ToolInput:      ToolInput{Args: probePlan + " errata: none", Skill: testFimplementSkill},
			ToolName:       toolNameSkill,
			TranscriptPath: workerParent,
		},
		ledger:       frozenLedger(t, worktree, "Bash(lsof *)"),
		settingsPath: filepath.Join(worktree, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "skill-call-agent-worktree-anchor.json"),
	})

	// anchor-removed-applies-nothing
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:            "anchor-removed-applies-nothing",
		_expectedAllow: []string{},
		_expectedExit:  0,
		_expectedOut:   planHookHeader + "\n- plan: not resolved — " + causeAnchorRemoved + "\n- Permissions: none applied\n",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement", TranscriptPath: planTranscript(t, filepath.Join(root, "removed"))},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "anchor-removed-applies-nothing.json"),
	})

	// writes-anchor-not-cwd
	root = planRepo(t)
	drifted := t.TempDir()
	writePlanFile(t, root, probePlan, planContent)
	tests = append(tests, &testCase{
		_id:            "writes-anchor-not-cwd",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut:   rowReport(root) + "\n",

		input: &HookInput{
			Cwd:            drifted,
			HookEventName:  hookEventUserPromptSubmit,
			Prompt:         "/fimplement " + filepath.Join(root, probePlan),
			TranscriptPath: planTranscript(t, root),
		},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "writes-anchor-not-cwd.json"),
	})

	// not-frozen-line
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	probeDir := planrules.PlanDir(probePlan)
	tests = append(tests, &testCase{
		_id:            "not-frozen-line",
		_expectedAllow: []string{},
		_expectedExit:  0,
		_expectedOut: planHookHeader + "\n- plan: " + filepath.Join(root, probePlan) + "\n- Permissions: none applied — " + probeDir +
			" is not frozen; a design run freezes it: toolpolicy freeze " + probeDir + "\n",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement probe.md", TranscriptPath: planTranscript(t, root)},
		ledger:       filepath.Join(t.TempDir(), planrules.LedgerName),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "not-frozen-line.json"),
	})

	// report-carries-checked-line
	root = planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	checkedRecord := frozenRecord("Bash(lsof *)")
	checkedRecord.Checked = []string{"go test ./...", "git status"}
	checkedRecord.Repo = gitroot.MainRoot(root)
	checkedLedger := filepath.Join(t.TempDir(), planrules.LedgerName)
	require.NoError(t, appendRecord(checkedLedger, checkedRecord))
	tests = append(tests, &testCase{
		_id:            "report-carries-checked-line",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut:   rowReport(root) + "go test ./..., git status\n",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement probe.md", TranscriptPath: planTranscript(t, root)},
		ledger:       checkedLedger,
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "report-carries-checked-line.json"),
	})

	// master-applies-derived
	root = planRepo(t)
	masterPlan := "plans/2026-09-28-probe-7ea30b5/2026-09-28-whole-wip-7ea30b5/design/whole.md"
	writePlanFile(t, root, masterPlan, "# Whole\n\nrole: `master`, target: `opus`\n")
	alphaPlan := "plans/2026-09-28-probe-7ea30b5/2026-09-28-alpha-wip-7ea30b5/design/alpha.md"
	alphaDir := planrules.PlanDir(alphaPlan)
	writePlanFile(t, root, alphaPlan, "# Alpha\n\nrole: `derived`, master: [whole](../../2026-09-28-whole-wip-7ea30b5/design/whole.md)\n")
	writePlanFile(t, root, probePlan, "# Probe\n\nrole: `derived`, master: [whole](../../2026-09-28-whole-wip-7ea30b5/design/whole.md), target: `opus`\n")
	tests = append(tests, &testCase{
		_id:            "master-applies-derived",
		_expectedAllow: []string{"Bash(lsof *)"},
		_expectedExit:  0,
		_expectedOut: planHookHeader + "\n- master: " + filepath.Join(root, masterPlan) + " — applying its 2 derived plans" +
			"\n- plan: " + filepath.Join(root, alphaPlan) + "\n- Permissions: none applied — " + alphaDir + " is not frozen; a design run freezes it: toolpolicy freeze " + alphaDir +
			strings.TrimPrefix(rowReport(root), planHookHeader) + "\n",

		input:        &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement whole.md", TranscriptPath: planTranscript(t, root)},
		ledger:       frozenLedger(t, root, "Bash(lsof *)"),
		settingsPath: filepath.Join(root, ".claude", unlockSettings),
		statePath:    filepath.Join(stateDir, "master-applies-derived.json"),
	})

	cfg := defaultHookConfig(t)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			grants, err := LoadGrants(
				test.input,
				cfg,
				home,
				"",
				"",
				test.ledger,
			)
			require.NoError(t, err)
			stdout := &bytes.Buffer{}
			exit := runPlanHook(test.input, policy, grants, test.statePath, stdout)
			_, allow, err := allowedRules(test.settingsPath)
			require.NoError(t, err)
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedOut, stdout.String())
			assert.Equal(t, test._expectedAllow, allow)
		})
	}
}

func TestRunPlanHook_Headless(t *testing.T) {
	t.Setenv(headlessEnv, headlessEnvValue)
	root := planRepo(t)
	writePlanFile(t, root, probePlan, planContent)
	grants := frozenGrants(t, root, "Bash(lsof *)")
	statePath := filepath.Join(t.TempDir(), "headless.json")
	policy := defaultToolpolicy(t)

	prompt := &HookInput{Cwd: root, HookEventName: hookEventUserPromptSubmit, Prompt: "/fimplement"}
	skill := &HookInput{Cwd: root, HookEventName: hookEventPostToolUse, ToolInput: ToolInput{Args: probePlan, Skill: testFimplementSkill}, ToolName: toolNameSkill}

	promptOut := &bytes.Buffer{}
	skillOut := &bytes.Buffer{}
	assert.Equal(t, 0, runPlanHook(prompt, policy, grants, statePath, promptOut))
	assert.Equal(t, 0, runPlanHook(skill, policy, grants, statePath, skillOut))

	assert.Empty(t, promptOut.String())
	assert.Empty(t, skillOut.String())
	assert.NoFileExists(t, filepath.Join(root, ".claude", unlockSettings))
}
