package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/planrules"
)

func fixtureHome(t *testing.T) string {
	t.Helper()
	home, err := filepath.Abs("testdata/home")
	require.NoError(t, err)
	return home
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	project, err := filepath.Abs("testdata/project")
	require.NoError(t, err)
	return project
}

func ruleStrings(rules []*Rule) []string {
	raw := make([]string, 0, len(rules))
	for _, rule := range rules {
		raw = append(raw, rule.Raw)
	}
	return raw
}

func TestLoadGrants(t *testing.T) {
	type testCase struct {
		_id                    string
		_expectedAllow         []string
		_expectedApprovals     map[string]bool
		_expectedDeny          []string
		_expectedDesignPrompts []string
		_expectedDirs          []string
		_expectedHuman         bool
		_expectedRecords       []*planrules.Record
		_expectedSkillRule     []string

		frozenLedger string
		input        *HookInput
	}

	t.Setenv(headlessEnv, "")
	home := fixtureHome(t)
	project := fixtureProject(t)
	noRecords := []*planrules.Record{}
	projectRecord := frozenRecord("Bash(lsof *)")
	projectRecord.Repo = gitroot.MainRoot(project)
	otherPlanRecord := frozenRecord()
	otherPlanRecord.Plan = "plans/2026-09-28-other-1a2b3c4/2026-09-28-other-wip-1a2b3c4"
	otherPlanRecord.Repo = gitroot.MainRoot(project)
	otherRepoRecord := frozenRecord("Bash(kill *)")
	otherRepoRecord.Repo = t.TempDir()
	recordsLedger := filepath.Join(t.TempDir(), planrules.LedgerName)
	for _, record := range []*planrules.Record{projectRecord, otherRepoRecord, otherPlanRecord} {
		require.NoError(t, appendRecord(recordsLedger, record))
	}
	broken, err := filepath.Abs("testdata/broken")
	require.NoError(t, err)
	layeredAllow := []string{
		"Bash(git status*)",
		"Edit",
		"Write",
		"WebFetch(domain:code.claude.com)",
		"mcp__ccd_session",
		"mcp__serena__find_symbol",
		"Bash(gh *)",
		"Bash(make audit*)",
		"Bash(go test *)",
	}
	layeredDeny := []string{"Bash(git stash*)", "WebFetch(domain:evil.example)"}
	layeredDirs := []string{filepath.Join(home, "extra")}
	admitted := t.TempDir()
	writeAllowLayer(t, admitted, "Skill(smine-orchestrate)")
	admittedAllow := append(slices.Clone(layeredAllow[:7]), "Skill(smine-orchestrate)")
	scannedApprovals := map[string]bool{
		"Makefile":            true,
		"nonsense":            true,
		"settings.local.json": false,
	}

	tests := make([]*testCase, 0)

	// four-layers-unioned
	tests = append(tests, &testCase{
		_id:                    "four-layers-unioned",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project},
	})

	// missing-layers-skipped
	tests = append(tests, &testCase{
		_id:                    "missing-layers-skipped",
		_expectedAllow:         layeredAllow[:7],
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny[:1],
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: filepath.Join(project, "nowhere")},
	})

	// unparseable-layer-skipped
	tests = append(tests, &testCase{
		_id:                    "unparseable-layer-skipped",
		_expectedAllow:         layeredAllow[:7],
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny[:1],
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: broken},
	})

	// skill-tool-use-unadmitted-no-grant
	tests = append(tests, &testCase{
		_id:                    "skill-tool-use-unadmitted-no-grant",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-skill-tooluse.jsonl"},
	})

	// skill-tool-use-settings-grant
	tests = append(tests, &testCase{
		_id:                    "skill-tool-use-settings-grant",
		_expectedAllow:         admittedAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny[:1],
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{"Edit", "Read", "Bash(git *)", "Bash(mv *)"},

		frozenLedger: "",
		input:        &HookInput{Cwd: admitted, TranscriptPath: "testdata/transcript-skill-tooluse.jsonl"},
	})

	// prompt-start-grant
	tests = append(tests, &testCase{
		_id:                    "prompt-start-grant",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{"Read", "Bash(jq *)"},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-prompt-start.jsonl"},
	})

	// command-name-grant
	tests = append(tests, &testCase{
		_id:                    "command-name-grant",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{"Read", "Bash(jq *)", "Edit", "Read", "Bash(git *)", "Bash(mv *)"},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-command-name.jsonl"},
	})

	// subagent-inherits-skill-grants
	tests = append(tests, &testCase{
		_id:                    "subagent-inherits-skill-grants",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{"Read", "Bash(jq *)", "Edit", "Read", "Bash(git *)", "Bash(mv *)"},

		frozenLedger: "",
		input:        &HookInput{AgentId: "agent-1", Cwd: project, TranscriptPath: "testdata/transcript-command-name.jsonl"},
	})

	// missing-transcript-no-grants
	tests = append(tests, &testCase{
		_id:                    "missing-transcript-no-grants",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/does-not-exist.jsonl"},
	})

	// approvals-scanned
	tests = append(tests, &testCase{
		_id:                    "approvals-scanned",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     scannedApprovals,
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-approvals.jsonl"},
	})

	// approvals-behind-harness-preamble
	tests = append(tests, &testCase{
		_id:                    "approvals-behind-harness-preamble",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{"Makefile": true, "other": true, "settings.local.json": false},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: []string{""},
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-approvals-preamble.jsonl"},
	})

	// approvals-mid-turn-queued
	tests = append(tests, &testCase{
		_id:                    "approvals-mid-turn-queued",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{"Makefile": true, "settings.local.json": true},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-approvals-queued.jsonl"},
	})

	// approvals-aliases
	tests = append(tests, &testCase{
		_id:                    "approvals-aliases",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{"-S": true, "settings.local.json": true},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-approvals-aliases.jsonl"},
	})

	// headless-no-human
	tests = append(tests, &testCase{
		_id:                    "headless-no-human",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-headless.jsonl"},
	})

	// design-prompt-command-name
	tests = append(tests, &testCase{
		_id:                    "design-prompt-command-name",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: []string{"refine probe.md", ""},
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-design-command-name.jsonl"},
	})

	// design-prompt-prompt-start
	tests = append(tests, &testCase{
		_id:                    "design-prompt-prompt-start",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: []string{"change probe.md"},
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-design-prompt-start.jsonl"},
	})

	// design-prompt-skill-call-not-counted
	tests = append(tests, &testCase{
		_id:                    "design-prompt-skill-call-not-counted",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         true,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-design-skill-call.jsonl"},
	})

	// design-prompt-peer-origin-not-counted
	tests = append(tests, &testCase{
		_id:                    "design-prompt-peer-origin-not-counted",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-design-peer.jsonl"},
	})

	// design-prompt-sdk-origin-not-counted
	tests = append(tests, &testCase{
		_id:                    "design-prompt-sdk-origin-not-counted",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: "",
		input:        &HookInput{Cwd: project, TranscriptPath: "testdata/transcript-design-sdk.jsonl"},
	})

	// records-of-this-repo
	tests = append(tests, &testCase{
		_id:                    "records-of-this-repo",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       []*planrules.Record{projectRecord, otherPlanRecord},
		_expectedSkillRule:     []string{},

		frozenLedger: recordsLedger,
		input:        &HookInput{Cwd: project},
	})

	// records-missing-ledger
	tests = append(tests, &testCase{
		_id:                    "records-missing-ledger",
		_expectedAllow:         layeredAllow,
		_expectedApprovals:     map[string]bool{},
		_expectedDeny:          layeredDeny,
		_expectedDesignPrompts: nil,
		_expectedDirs:          layeredDirs,
		_expectedHuman:         false,
		_expectedRecords:       noRecords,
		_expectedSkillRule:     []string{},

		frozenLedger: filepath.Join(t.TempDir(), planrules.LedgerName),
		input:        &HookInput{Cwd: project},
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
				test.frozenLedger,
			)
			require.NoError(t, err)
			assert.Equal(t, test._expectedDesignPrompts, grants.freeze.designPrompts)
			assert.Equal(t, test._expectedRecords, grants.freeze.records)
			assert.Equal(t, test._expectedApprovals, grants.Approvals)
			assert.Equal(t, test._expectedAllow, ruleStrings(grants.Allow))
			assert.Equal(t, test._expectedDeny, ruleStrings(grants.Deny))
			assert.Equal(t, test._expectedDirs, grants.AdditionalDirectories)
			assert.Equal(t, test._expectedHuman, grants.HasHumanTurn)
			assert.Equal(t, test._expectedSkillRule, ruleStrings(grants.SkillAllow))
		})
	}
}

func enforceSkills(rules []*GateRules) []string {
	skills := make([]string, 0, len(rules))
	for _, rule := range rules {
		skills = append(skills, rule.Skill)
	}
	return skills
}

// writeSkillFixture deploys a fixture skill into home: a SKILL.md with one
// allowed-tools line and, when enforce is not empty, its enforce.json.
func writeSkillFixture(t *testing.T, allowedTools string, enforce string, home string, name string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "skills", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: " + name + "\ndescription: fixture\nallowed-tools: " + allowedTools + "\n---\n\n# fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
	if enforce == "" {
		return
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, enforceFile), []byte(enforce), 0o644))
}

// chainHome is a home deploying the chain fixtures: chain grants link, link
// grants jq and carries a gate rule; jq and smine-orchestrate match
// testdata/home.
func chainHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	writeSkillFixture(t, "Skill(link)", "", home, "chain")
	writeSkillFixture(t, "Read, Bash(jq *)", "", home, "jq")
	writeSkillFixture(t, "Skill(jq), Bash(date)", `{"gate":{"planModeDenied":["Workflow"]}}`, home, "link")
	writeSkillFixture(t, "Edit, Read, Bash(git *), Bash(mv *)", "", home, "smine-orchestrate")
	return home
}

func TestLoadGrants_ManagedSettingsPathFromConfig(t *testing.T) {
	cfg := defaultHookConfig(t)
	managed := filepath.Join(t.TempDir(), "managed-settings.json")
	require.NoError(t, os.WriteFile(managed, []byte(`{"permissions": {"allow": ["Bash(managed-probe *)"]}}`), 0o644))
	cfg.Toolpolicy.ManagedSettingsPath = managed
	input := &HookInput{Cwd: t.TempDir()}

	grants, err := LoadGrants(
		input,
		cfg,
		t.TempDir(),
		"",
		"",
		"",
	)

	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(managed-probe *)"}, ruleStrings(grants.Allow))
}

func TestLoadGrants_SkillChain(t *testing.T) {
	input := &HookInput{Cwd: t.TempDir(), TranscriptPath: "testdata/transcript-skill-chain.jsonl"}
	grants, err := LoadGrants(
		input,
		defaultHookConfig(t),
		chainHome(t),
		"",
		"",
		"",
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"link"}, enforceSkills(grants.Enforce))
	assert.Equal(t, []string{"Skill(link)", "Skill(jq)", "Bash(date)"}, ruleStrings(grants.SkillAllow))
}

func TestLoadGrants_PromptNamedSkill(t *testing.T) {
	input := &HookInput{Cwd: t.TempDir(), TranscriptPath: "testdata/transcript-skill-prompt-named.jsonl"}
	grants, err := LoadGrants(
		input,
		defaultHookConfig(t),
		chainHome(t),
		"",
		"",
		"",
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"jq", "chain"}, grants.ActiveSkills)
	assert.Equal(t, []string{"link"}, grants.PromptSkills)
	assert.Equal(t, []string{"Read", "Bash(jq *)", "Skill(link)"}, ruleStrings(grants.SkillAllow))
}

func TestGrants_ActivateSkills(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedActive    []string
		_expectedEnforce   []string
		_expectedSkillRule []string

		events []skillEvent
		grants *Grants
	}

	home := chainHome(t)
	chainRules := []string{"Skill(link)"}
	linkRules := []string{"Skill(jq)", "Bash(date)"}
	jqRules := []string{"Read", "Bash(jq *)"}
	orchestrateRules := []string{"Edit", "Read", "Bash(git *)", "Bash(mv *)"}
	typedChain := skillEvent{isToolCall: false, skill: "chain"}
	typedJq := skillEvent{isToolCall: false, skill: "jq"}
	typedOrchestrate := skillEvent{isToolCall: false, skill: "smine-orchestrate"}
	callJq := skillEvent{isToolCall: true, skill: "jq"}
	callLink := skillEvent{isToolCall: true, skill: "link"}
	callOrchestrate := skillEvent{isToolCall: true, skill: "smine-orchestrate"}
	namedCallOrchestrate := skillEvent{isPromptNamed: true, isToolCall: true, skill: "smine-orchestrate"}

	tests := make([]*testCase, 0)

	// empty-events
	tests = append(tests, &testCase{
		_id:                "empty-events",
		_expectedActive:    []string{},
		_expectedEnforce:   []string{},
		_expectedSkillRule: []string{},

		events: nil,
		grants: &Grants{},
	})

	// typed-activates
	tests = append(tests, &testCase{
		_id:                "typed-activates",
		_expectedActive:    []string{"jq"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: jqRules,

		events: []skillEvent{typedJq},
		grants: &Grants{},
	})

	// typed-unknown-skill-grants-nothing
	tests = append(tests, &testCase{
		_id:                "typed-unknown-skill-grants-nothing",
		_expectedActive:    []string{"ghost"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: []string{},

		events: []skillEvent{{isToolCall: false, skill: "ghost"}},
		grants: &Grants{},
	})

	// repeat-not-duplicated
	tests = append(tests, &testCase{
		_id:                "repeat-not-duplicated",
		_expectedActive:    []string{"jq"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: jqRules,

		events: []skillEvent{typedJq, typedJq, callJq},
		grants: &Grants{},
	})

	// call-unadmitted-ignored
	tests = append(tests, &testCase{
		_id:                "call-unadmitted-ignored",
		_expectedActive:    []string{},
		_expectedEnforce:   []string{},
		_expectedSkillRule: []string{},

		events: []skillEvent{callOrchestrate},
		grants: &Grants{},
	})

	// call-unadmitted-no-enforce
	tests = append(tests, &testCase{
		_id:                "call-unadmitted-no-enforce",
		_expectedActive:    []string{},
		_expectedEnforce:   []string{},
		_expectedSkillRule: []string{},

		events: []skillEvent{callLink},
		grants: &Grants{},
	})

	// call-prompt-named-admitted
	tests = append(tests, &testCase{
		_id:                "call-prompt-named-admitted",
		_expectedActive:    []string{"smine-orchestrate"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: orchestrateRules,

		events: []skillEvent{namedCallOrchestrate},
		grants: &Grants{},
	})

	// call-after-prompt-named-stays-active
	tests = append(tests, &testCase{
		_id:                "call-after-prompt-named-stays-active",
		_expectedActive:    []string{"smine-orchestrate"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: orchestrateRules,

		events: []skillEvent{namedCallOrchestrate, callOrchestrate},
		grants: &Grants{},
	})

	// call-settings-admitted
	tests = append(tests, &testCase{
		_id:                "call-settings-admitted",
		_expectedActive:    []string{"smine-orchestrate"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: orchestrateRules,

		events: []skillEvent{callOrchestrate},
		grants: &Grants{Allow: parseRules([]string{"Skill(smine-orchestrate)"}, "fixture")},
	})

	// call-settings-other-skill-ignored
	tests = append(tests, &testCase{
		_id:                "call-settings-other-skill-ignored",
		_expectedActive:    []string{},
		_expectedEnforce:   []string{},
		_expectedSkillRule: []string{},

		events: []skillEvent{callOrchestrate},
		grants: &Grants{Allow: parseRules([]string{"Skill(jq)", "Bash(git *)"}, "fixture")},
	})

	// call-chain-admitted
	tests = append(tests, &testCase{
		_id:                "call-chain-admitted",
		_expectedActive:    []string{"chain", "link", "jq"},
		_expectedEnforce:   []string{"link"},
		_expectedSkillRule: slices.Concat(chainRules, linkRules, jqRules),

		events: []skillEvent{typedChain, callLink, callJq},
		grants: &Grants{},
	})

	// call-before-grant-ignored
	tests = append(tests, &testCase{
		_id:                "call-before-grant-ignored",
		_expectedActive:    []string{"chain", "link"},
		_expectedEnforce:   []string{"link"},
		_expectedSkillRule: slices.Concat(chainRules, linkRules),

		events: []skillEvent{typedChain, callJq, callLink},
		grants: &Grants{},
	})

	// call-retry-after-grant
	tests = append(tests, &testCase{
		_id:                "call-retry-after-grant",
		_expectedActive:    []string{"chain", "link", "jq"},
		_expectedEnforce:   []string{"link"},
		_expectedSkillRule: slices.Concat(chainRules, linkRules, jqRules),

		events: []skillEvent{typedChain, callJq, callLink, callJq},
		grants: &Grants{},
	})

	// typed-after-ignored-call
	tests = append(tests, &testCase{
		_id:                "typed-after-ignored-call",
		_expectedActive:    []string{"smine-orchestrate"},
		_expectedEnforce:   []string{},
		_expectedSkillRule: orchestrateRules,

		events: []skillEvent{callOrchestrate, typedOrchestrate},
		grants: &Grants{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.grants.activateSkills(test.events, home)
			assert.Equal(t, test._expectedActive, test.grants.ActiveSkills)
			assert.Equal(t, test._expectedEnforce, enforceSkills(test.grants.Enforce))
			assert.Equal(t, test._expectedSkillRule, ruleStrings(test.grants.SkillAllow))
		})
	}
}

// writeAllowLayer writes dir's settings.local.json allowing one rule.
func writeAllowLayer(t *testing.T, dir string, rule string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".claude"), 0o755))
	layer := `{"permissions":{"allow":["` + rule + `"]}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".claude", "settings.local.json"), []byte(layer), 0o644))
}

// writeCwdTranscript writes a transcript whose first line records cwd.
func writeCwdTranscript(t *testing.T, path string, cwd string) {
	t.Helper()
	line := `{"type":"user","cwd":"` + cwd + `","message":{"content":"hi"}}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(line), 0o644))
}

func TestLoadGrants_AnchorAndRepos(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedAllow      []string
		_expectedAnchorRoot string
		_expectedGateRepo   string
		_expectedRepoPaths  []string

		gateRepo  string
		input     *HookInput
		reposPath string
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	home := filepath.Join(base, "home")
	mainCheckout := filepath.Join(base, "repo")
	worktree := filepath.Join(mainCheckout, ".claude", "worktrees", "wt")
	lane := filepath.Join(mainCheckout, ".claude", "worktrees", "wf_r-1")
	foreign := filepath.Join(base, "foreign")
	other := filepath.Join(base, "other")
	for _, dir := range []string{home, filepath.Join(mainCheckout, ".git", "worktrees", "wt"), worktree, lane, foreign, other} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	pointer := "gitdir: " + filepath.Join(mainCheckout, ".git", "worktrees", "wt") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o644))
	writeAllowLayer(t, mainCheckout, "Bash(uptime)")
	writeAllowLayer(t, worktree, "Bash(date)")
	writeAllowLayer(t, lane, "Bash(whoami)")

	anchoredTranscript := filepath.Join(base, "anchored.jsonl")
	writeCwdTranscript(t, anchoredTranscript, worktree)
	writeSubagentTranscript(
		t,
		"w1",
		lane,
		anchoredTranscript,
		"wf_r",
		"s1",
	)
	removedTranscript := filepath.Join(base, "removed.jsonl")
	writeCwdTranscript(t, removedTranscript, filepath.Join(mainCheckout, ".claude", "worktrees", "gone"))

	foreignRepos := filepath.Join(base, "repos-foreign.json")
	foreignJson := `{"repos":[{"name":"foreign","path":"` + foreign + `"},{"name":"other","path":"` + other + `"}]}`
	require.NoError(t, os.WriteFile(foreignRepos, []byte(foreignJson), 0o644))
	ownRepos := filepath.Join(base, "repos-own.json")
	ownJson := `{"repos":[{"name":"repo","path":"` + mainCheckout + `"},{"name":"foreign","path":"` + foreign + `"}]}`
	require.NoError(t, os.WriteFile(ownRepos, []byte(ownJson), 0o644))

	tests := make([]*testCase, 0)

	// layers-from-anchor-not-drifted-cwd
	tests = append(tests, &testCase{
		_id:                 "layers-from-anchor-not-drifted-cwd",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  nil,

		gateRepo:  "",
		input:     &HookInput{Cwd: mainCheckout, TranscriptPath: anchoredTranscript},
		reposPath: "",
	})

	// removed-anchor-no-project-layers
	tests = append(tests, &testCase{
		_id:                 "removed-anchor-no-project-layers",
		_expectedAllow:      []string{},
		_expectedAnchorRoot: "",
		_expectedGateRepo:   "",
		_expectedRepoPaths:  nil,

		gateRepo:  "",
		input:     &HookInput{Cwd: mainCheckout, TranscriptPath: removedTranscript},
		reposPath: "",
	})

	// repos-flag-loads-paths
	tests = append(tests, &testCase{
		_id:                 "repos-flag-loads-paths",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  []string{foreign, other},

		gateRepo:  "",
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: foreignRepos,
	})

	// repos-flag-missing-file-no-grant
	tests = append(tests, &testCase{
		_id:                 "repos-flag-missing-file-no-grant",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  nil,

		gateRepo:  "",
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: filepath.Join(base, "missing.json"),
	})

	// repos-flag-empty
	tests = append(tests, &testCase{
		_id:                 "repos-flag-empty",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  nil,

		gateRepo:  "",
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: "",
	})

	// repos-own-repo-excluded
	tests = append(tests, &testCase{
		_id:                 "repos-own-repo-excluded",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  []string{foreign},

		gateRepo:  "",
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: ownRepos,
	})

	// gate-repo-flag-cleaned
	tests = append(tests, &testCase{
		_id:                 "gate-repo-flag-cleaned",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   mainCheckout,
		_expectedRepoPaths:  nil,

		gateRepo:  mainCheckout + string(filepath.Separator),
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: "",
	})

	// registry-dir-not-gate-repo
	tests = append(tests, &testCase{
		_id:                 "registry-dir-not-gate-repo",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedGateRepo:   "",
		_expectedRepoPaths:  []string{foreign},

		gateRepo:  "",
		input:     &HookInput{Cwd: worktree, TranscriptPath: anchoredTranscript},
		reposPath: ownRepos,
	})

	// workflow-subagent-layers-from-own-worktree
	tests = append(tests, &testCase{
		_id:                 "workflow-subagent-layers-from-own-worktree",
		_expectedAllow:      []string{"Bash(whoami)"},
		_expectedAnchorRoot: lane,
		_expectedRepoPaths:  nil,

		input:     &HookInput{AgentId: "w1", Cwd: worktree, SessionId: "s1", TranscriptPath: anchoredTranscript},
		reposPath: "",
	})

	// subagent-envelope-cwd-not-trusted
	tests = append(tests, &testCase{
		_id:                 "subagent-envelope-cwd-not-trusted",
		_expectedAllow:      []string{"Bash(date)"},
		_expectedAnchorRoot: worktree,
		_expectedRepoPaths:  nil,

		input:     &HookInput{AgentId: "w2", Cwd: lane, SessionId: "s1", TranscriptPath: anchoredTranscript},
		reposPath: "",
	})

	cfg := defaultHookConfig(t)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			grants, err := LoadGrants(
				test.input,
				cfg,
				home,
				test.gateRepo,
				test.reposPath,
				"",
			)
			require.NoError(t, err)
			assert.Equal(t, test._expectedAllow, ruleStrings(grants.Allow))
			assert.Equal(t, test._expectedAnchorRoot, grants.anchorRoot(test.input))
			assert.Equal(t, test._expectedGateRepo, grants.gateRepo)
			assert.Equal(t, test._expectedRepoPaths, grants.repoPaths)
		})
	}
}

func TestPathRuleRoots(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		repoPaths []string
		spec      string
	}

	tests := make([]*testCase, 0)

	// home-recursive
	tests = append(tests, &testCase{
		_id:       "home-recursive",
		_expected: []string{"/h/GolandProjects/agent-plans"},

		repoPaths: nil,
		spec:      "~/GolandProjects/agent-plans/**",
	})

	// absolute-recursive
	tests = append(tests, &testCase{
		_id:       "absolute-recursive",
		_expected: []string{"/opt/data"},

		repoPaths: nil,
		spec:      "//opt/data/**",
	})

	// repos-symbolic
	tests = append(tests, &testCase{
		_id:       "repos-symbolic",
		_expected: []string{"/r/a", "/r/b"},

		repoPaths: []string{"/r/a", "/r/b"},
		spec:      "@repos/**",
	})

	// repos-symbolic-empty-registry
	tests = append(tests, &testCase{
		_id:       "repos-symbolic-empty-registry",
		_expected: nil,

		repoPaths: nil,
		spec:      "@repos/**",
	})

	// home-non-recursive
	tests = append(tests, &testCase{
		_id:       "home-non-recursive",
		_expected: nil,

		repoPaths: nil,
		spec:      "~/GolandProjects/agent-plans",
	})

	// relative
	tests = append(tests, &testCase{
		_id:       "relative",
		_expected: nil,

		repoPaths: nil,
		spec:      "docs/**",
	})

	// repos-non-recursive
	tests = append(tests, &testCase{
		_id:       "repos-non-recursive",
		_expected: nil,

		repoPaths: []string{"/r/a"},
		spec:      "@repos",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, pathRuleRoots("/h", test.repoPaths, test.spec))
		})
	}
}

func TestForeignRepoPaths(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		anchor string
		paths  []string
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	mainCheckout := filepath.Join(base, "repo")
	worktree := filepath.Join(mainCheckout, ".claude", "worktrees", "wt")
	foreign := filepath.Join(base, "foreign")
	require.NoError(t, os.MkdirAll(filepath.Join(mainCheckout, ".git", "worktrees", "wt"), 0o755))
	require.NoError(t, os.MkdirAll(worktree, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(foreign, ".git"), 0o755))
	pointer := "gitdir: " + filepath.Join(mainCheckout, ".git", "worktrees", "wt") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o644))

	tests := make([]*testCase, 0)

	// own-main-checkout-dropped
	tests = append(tests, &testCase{
		_id:       "own-main-checkout-dropped",
		_expected: []string{foreign},

		anchor: mainCheckout,
		paths:  []string{mainCheckout, foreign},
	})

	// own-repo-from-worktree-anchor-dropped
	tests = append(tests, &testCase{
		_id:       "own-repo-from-worktree-anchor-dropped",
		_expected: []string{foreign},

		anchor: worktree,
		paths:  []string{foreign, mainCheckout},
	})

	// foreign-kept
	tests = append(tests, &testCase{
		_id:       "foreign-kept",
		_expected: []string{foreign},

		anchor: worktree,
		paths:  []string{foreign},
	})

	// empty-anchor-unchanged
	tests = append(tests, &testCase{
		_id:       "empty-anchor-unchanged",
		_expected: []string{mainCheckout, foreign},

		anchor: "",
		paths:  []string{mainCheckout, foreign},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, foreignRepoPaths(test.anchor, test.paths))
		})
	}
}
