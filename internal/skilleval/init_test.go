package skilleval

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
)

const (
	initTestBaseSha = "b4981b88d52deaa69d6ed1d60eb51b57ead47ba0"
	initTestDate    = "2026-10-02"
	initTestHeadSha = "a36dd5b46f542542cb58401ef7cf2d7452eb28f9"
	initTestOverlay = "diff --git a/x b/x\n"
	initTestRunName = initTestDate + "-a36dd5"
	initTestSkillMd = "---\nname: demo\nversion: " + initTestVersion + "\n---\n"
	initTestToken   = "eval"
	initTestVersion = "1.4.0"
)

// initDefinition is a minimal valid definition for skill.
func initDefinition(skill string) *evals.Definition {
	definition := &evals.Definition{
		Budgets:       evals.DefinitionBudgets{ArmTimeoutS: 600, ArmUsd: 10},
		Environment:   evals.DefinitionEnvironment{Range: evals.DefinitionRange{BaseSha: initTestBaseSha, HeadSha: initTestHeadSha}},
		Invocation:    evals.DefinitionInvocation{Text: "run"},
		SchemaVersion: definitionSchema,
		Skill:         skill,
	}
	return definition
}

// initErrorText is err's message, empty for nil.
func initErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// initSentinel is the init refusal err wraps, nil for none.
func initSentinel(err error) error {
	for _, sentinel := range []error{ErrClosedArg, ErrNoDefinition, ErrNoGolden, ErrOverWindow, ErrUnknownToken} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	return nil
}

func initTestDemoSpec() *evals.Definition {
	definition := &evals.Definition{
		Budgets: evals.DefinitionBudgets{ArmTimeoutS: 7200, ArmUsd: 30, WindowUsd: 150},
		Check:   []string{"make", "audit"},
		Environment: evals.DefinitionEnvironment{
			Overlay:    "eval/context.patch",
			Range:      evals.DefinitionRange{BaseSha: initTestBaseSha, HeadSha: initTestHeadSha},
			TargetRepo: "peek-mcp",
		},
		Invocation:    evals.DefinitionInvocation{Text: "run"},
		Reference:     "9e49be5e",
		SchemaVersion: definitionSchema,
		Scorer:        evals.DefinitionScorer{Command: []string{"make", "score"}},
		Skill:         "demo",
	}
	return definition
}

// manifestArms are the distinct configuration keys and the blind labels of
// runDir's run.json, each sorted; both empty without one.
func manifestArms(runDir string) ([]string, []string) {
	keys := make([]string, 0)
	labels := make([]string, 0)
	manifest, err := loadManifest(runDir)
	if err != nil {
		return keys, labels
	}

	for _, arm := range manifest.Arms {
		labels = append(labels, arm.Label)
		if !slices.Contains(keys, arm.Key) {
			keys = append(keys, arm.Key)
		}
	}
	slices.Sort(keys)
	slices.Sort(labels)
	return keys, labels
}

// setupInitHome points HOME at a temp dir holding a deployed SKILL.md for each
// skill and the token store labels eval and spare.
func setupInitHome(t *testing.T, skillNames ...string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(windowBudgetEnv, "")
	for _, skill := range skillNames {
		writeTestFile(t, filepath.Join(home, ".claude", "skills", skill, skillMdFile), initTestSkillMd)
	}
	for _, label := range []string{initTestToken, "spare"} {
		writeTestFile(t, filepath.Join(home, ".config", "claude-routine", "tokens", label), "token")
	}
}

// writeInitSkill writes a repo skill dir under skillsRoot at relative holding
// definition as its eval/definition.json.
func writeInitSkill(t *testing.T, definition *evals.Definition, relative, skillsRoot string) string {
	t.Helper()
	skillDir := filepath.Join(skillsRoot, relative)
	writeDefinitionFile(t, skillDir, definition)
	return skillDir
}

func TestInit(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedErrText     string
		_expectedHasSkillDir bool
		_expectedKeys        []string
		_expectedLabels      []string
		_expectedRunDir      string
		_expectedSentinel    error
		_shouldPass          bool

		request *InitRequest
	}

	setupInitHome(t, "demo", "paired", "single", "needsprereq", "minted", "shortref")
	skillsRoot := newTempDir(t)
	demoDir := writeInitSkill(t, initTestDemoSpec(), "demo", skillsRoot)
	writeTestFile(t, filepath.Join(demoDir, "eval", "context.patch"), initTestOverlay)
	paired := initDefinition("paired")
	paired.Replicas = 2
	writeInitSkill(t, paired, filepath.Join("group", "paired"), skillsRoot)
	writeInitSkill(t, initDefinition("single"), "single", skillsRoot)
	writeInitSkill(t, initDefinition("twice"), "twice", skillsRoot)
	writeInitSkill(t, initDefinition("twice"), filepath.Join("group", "twice"), skillsRoot)
	needsPrereq := initDefinition("needsprereq")
	needsPrereq.Prerequisites = []string{"eval/missing.patch"}
	writeInitSkill(t, needsPrereq, "needsprereq", skillsRoot)
	minted := initDefinition("minted")
	minted.Goldens = []evals.DefinitionGolden{{File: "eval/golden/g.json", Id: "g"}}
	writeInitSkill(t, minted, "minted", skillsRoot)
	writeInitSkill(t, initDefinition("undeployed"), "undeployed", skillsRoot)
	shortRef := initDefinition("shortref")
	shortRef.Environment.CheckoutRef = "abc"
	writeInitSkill(t, shortRef, "shortref", skillsRoot)

	tests := make([]*testCase, 0)

	// happy-path
	happyRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "happy-path",
		_expectedErrText:     "",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{"opus-high"},
		_expectedLabels:      []string{"A"},
		_expectedRunDir:      filepath.Join(happyRoot, "demo", initTestRunName),
		_expectedSentinel:    nil,
		_shouldPass:          true,

		request: &InitRequest{Date: initTestDate, EvalsRoot: happyRoot, Replicas: 0, Skill: "demo", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// replicas-arg-three
	replicasRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "replicas-arg-three",
		_expectedErrText:     "",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{"opus-high"},
		_expectedLabels:      []string{"A", "B", "C"},
		_expectedRunDir:      filepath.Join(replicasRoot, "demo", initTestRunName),
		_expectedSentinel:    nil,
		_shouldPass:          true,

		request: &InitRequest{Date: initTestDate, EvalsRoot: replicasRoot, Replicas: 3, Skill: "demo", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// definition-replicas-two
	pairedRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "definition-replicas-two",
		_expectedErrText:     "",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{"opus-high"},
		_expectedLabels:      []string{"A", "B"},
		_expectedRunDir:      filepath.Join(pairedRoot, "paired", initTestRunName),
		_expectedSentinel:    nil,
		_shouldPass:          true,

		request: &InitRequest{Date: initTestDate, EvalsRoot: pairedRoot, Replicas: 0, Skill: "paired", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// no-replicas-declared-one
	singleRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "no-replicas-declared-one",
		_expectedErrText:     "",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{"opus-high"},
		_expectedLabels:      []string{"A"},
		_expectedRunDir:      filepath.Join(singleRoot, "single", initTestRunName),
		_expectedSentinel:    nil,
		_shouldPass:          true,

		request: &InitRequest{Date: initTestDate, EvalsRoot: singleRoot, Replicas: 0, Skill: "single", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// second-init-same-day
	secondRoot := newTempDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(secondRoot, "demo", initTestRunName), 0o755))
	tests = append(tests, &testCase{
		_id:                  "second-init-same-day",
		_expectedErrText:     "",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{"opus-high"},
		_expectedLabels:      []string{"A"},
		_expectedRunDir:      filepath.Join(secondRoot, "demo", initTestRunName+"-2"),
		_expectedSentinel:    nil,
		_shouldPass:          true,

		request: &InitRequest{Date: initTestDate, EvalsRoot: secondRoot, Replicas: 0, Skill: "demo", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// no-definition
	absentRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "no-definition",
		_expectedErrText:     "absent",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    ErrNoDefinition,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: absentRoot, Replicas: 0, Skill: "absent", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// both-layout-levels
	twiceRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "both-layout-levels",
		_expectedErrText:     "2 skill dirs match",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    nil,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: twiceRoot, Replicas: 0, Skill: "twice", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// invalid-skill-name
	invalidRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "invalid-skill-name",
		_expectedErrText:     "Invalid skill name",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    nil,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: invalidRoot, Replicas: 0, Skill: "../x", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// missing-prerequisite
	prereqRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "missing-prerequisite",
		_expectedErrText:     "eval/missing.patch",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    nil,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: prereqRoot, Replicas: 0, Skill: "needsprereq", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// minted-golden-missing
	mintedRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "minted-golden-missing",
		_expectedErrText:     "eval/golden/g.json is not minted",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    ErrNoGolden,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: mintedRoot, Replicas: 0, Skill: "minted", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// unknown-token
	unknownTokenRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "unknown-token",
		_expectedErrText:     "stored labels: eval, spare",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    ErrUnknownToken,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: unknownTokenRoot, Replicas: 0, Skill: "demo", SkillsRoot: skillsRoot, Token: "other"},
	})

	// empty-token
	emptyTokenRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "empty-token",
		_expectedErrText:     "",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    ErrUnknownToken,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: emptyTokenRoot, Replicas: 0, Skill: "demo", SkillsRoot: skillsRoot, Token: ""},
	})

	// missing-deployed-skill-md
	undeployedRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "missing-deployed-skill-md",
		_expectedErrText:     "copyFile",
		_expectedHasSkillDir: true,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    nil,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: undeployedRoot, Replicas: 0, Skill: "undeployed", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// short-checkout-ref
	shortRefRoot := newTempDir(t)
	tests = append(tests, &testCase{
		_id:                  "short-checkout-ref",
		_expectedErrText:     "shortRef",
		_expectedHasSkillDir: false,
		_expectedKeys:        []string{},
		_expectedLabels:      []string{},
		_expectedRunDir:      "",
		_expectedSentinel:    nil,
		_shouldPass:          false,

		request: &InitRequest{Date: initTestDate, EvalsRoot: shortRefRoot, Replicas: 0, Skill: "shortref", SkillsRoot: skillsRoot, Token: initTestToken},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			runDir, err := Init(test.request)
			keys, labels := manifestArms(runDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedRunDir, runDir)
			assert.Equal(t, test._expectedSentinel, initSentinel(err))
			assert.Contains(t, initErrorText(err), test._expectedErrText)
			assert.Equal(t, test._expectedHasSkillDir, pathExists(filepath.Join(test.request.EvalsRoot, test.request.Skill)))
			assert.Equal(t, test._expectedKeys, keys)
			assert.Equal(t, test._expectedLabels, labels)
		})
	}

	// happy-path run.json, SKILL.md and overlay copies
	evalsRoot := newTempDir(t)
	runDir, err := Init(&InitRequest{Date: initTestDate, EvalsRoot: evalsRoot, Replicas: 0, Skill: "demo", SkillsRoot: skillsRoot, Token: initTestToken})
	require.NoError(t, err)
	expected := &RunManifest{
		Arms: []ManifestArm{
			{
				Id:              "opus-high-r1",
				DisabledServers: []string{},
				Effort:          "high",
				Key:             "opus-high",
				Label:           "A",
				Model:           "opus",
				OverlayPatch:    "",
				Replica:         1,
				Worktree:        "",
			},
		},
		At:             initTestHeadSha,
		BudgetUsd:      30,
		Check:          []string{"make", "audit"},
		CheckTimeoutS:  1800,
		Date:           initTestDate,
		Findings:       evals.DefinitionFindings{},
		Golden:         nil,
		Judge:          "opus",
		Layer:          LayerEval,
		Mode:           ModeNightly,
		OverlayPatch:   filepath.Join(runDir, overlayFile),
		Overrides:      nil,
		PresetSkillDir: demoDir,
		Prompt:         "/demo run",
		Reference:      "9e49be5e",
		Repo:           "",
		RepoName:       "peek-mcp",
		Scorer:         []string{"make", "score"},
		Skill:          "demo",
		SkillMd:        filepath.Join(runDir, skillMdFile),
		SkillVersion:   initTestVersion,
		TimeoutS:       7200,
		TokenLabel:     initTestToken,
	}

	manifest, err := loadManifest(runDir)
	require.NoError(t, err)
	skillCopy, err := os.ReadFile(filepath.Join(runDir, skillMdFile))
	require.NoError(t, err)
	overlayCopy, err := os.ReadFile(filepath.Join(runDir, overlayFile))
	require.NoError(t, err)
	assert.Equal(t, expected, manifest)
	assert.Equal(t, initTestSkillMd, string(skillCopy))
	assert.Equal(t, initTestOverlay, string(overlayCopy))
	assert.DirExists(t, filepath.Join(runDir, runsDir))
}

func TestInit_railroadReviewDefinition(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	setupInitHome(t, "railroad-review")
	goldenOverlay, err := os.ReadFile(filepath.Join(repoRoot, "skills", "quality", "railroad-review", "eval", "golden", "smine-1.1.4-1.2.1.g2.overlay.patch"))
	require.NoError(t, err)
	evalsRoot := newTempDir(t)

	runDir, err := Init(&InitRequest{
		Date:       "2026-09-28",
		EvalsRoot:  evalsRoot,
		Golden:     GoldenProduce,
		Replicas:   0,
		Skill:      "railroad-review",
		SkillsRoot: filepath.Join(repoRoot, "skills"),
		Token:      initTestToken,
	})
	require.NoError(t, err)
	manifest, err := loadManifest(runDir)
	require.NoError(t, err)
	overlayCopy, err := os.ReadFile(manifest.OverlayPatch)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(evalsRoot, "railroad-review", "2026-09-28-1f725c"), runDir)
	assert.Equal(t, "1f725cedc5c8b092c8a3d7e408793ef72840ec7e", manifest.At)
	assert.Contains(t, manifest.Prompt, "autofix-level: none")
	assert.Contains(t, manifest.Prompt, "exclude: sessions/ evals/ proposals/ plans/ ")
	assert.Contains(t, manifest.Prompt, "prebuilt: bin/acdsl bin/verifiers")
	assert.Contains(t, manifest.Prompt, "scope: branch[b4981b88d52deaa69d6ed1d60eb51b57ead47ba0]")
	assert.Equal(t, "context/review-index.json", manifest.ContextIndex)
	expectedPrebuild := &evals.DefinitionPrebuild{Command: []string{"make", "build-audit"}, Outputs: []string{"bin/acdsl", "bin/verifiers"}}
	assert.Equal(t, expectedPrebuild, manifest.Prebuild)
	assert.InDelta(t, 1000, manifest.BudgetUsd, 0)
	require.NotNil(t, manifest.Golden)
	assert.Equal(t, GoldenProduce, manifest.Golden.Kind)
	assert.Equal(t, 7200, manifest.TimeoutS)
	assert.Equal(t, 1800, manifest.CheckTimeoutS)
	assert.True(t, filepath.IsAbs(manifest.PresetSkillDir))
	assert.Equal(t, filepath.Join(repoRoot, "skills", "quality", "railroad-review"), manifest.PresetSkillDir)
	assert.Equal(t, string(goldenOverlay), string(overlayCopy))
	assert.Empty(t, manifest.Check)
	assert.Empty(t, manifest.Reference)
}

func TestRenderInvocation(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         string
		_expectedSentinel error
		_shouldPass       bool

		def       *evals.Definition
		golden    *ManifestGolden
		overrides []evals.DefinitionArg
	}

	reviewArgs := []evals.DefinitionArg{
		{Name: "scope", Value: "branch"},
		{Name: "autofix-level", Value: "full"},
		{Name: "rounds", Value: "1"},
		{Name: "verbose"},
	}
	produceArgs := []evals.DefinitionArg{
		{Name: "autofix-level", Value: "none"},
		{Name: "known", Value: "{seed}"},
	}
	reviewGoldens := []evals.DefinitionGolden{{Produce: evals.DefinitionGoldenRun{Args: produceArgs}}}
	reviewInvocation := evals.DefinitionInvocation{Args: reviewArgs, Open: []string{"rounds"}}
	tests := make([]*testCase, 0)

	// kind-args-merged
	tests = append(tests, &testCase{
		_id:               "kind-args-merged",
		_expected:         "/review scope: branch autofix-level: none rounds: 1 verbose known: {seed}",
		_expectedSentinel: nil,
		_shouldPass:       true,

		def:       &evals.Definition{Goldens: reviewGoldens, Invocation: reviewInvocation, Skill: "review"},
		golden:    &ManifestGolden{Kind: GoldenProduce},
		overrides: nil,
	})

	// set-replaces-in-place
	tests = append(tests, &testCase{
		_id:               "set-replaces-in-place",
		_expected:         "/review scope: branch autofix-level: full rounds: 2 verbose",
		_expectedSentinel: nil,
		_shouldPass:       true,

		def:       &evals.Definition{Goldens: reviewGoldens, Invocation: reviewInvocation, Skill: "review"},
		golden:    &ManifestGolden{Kind: GoldenTest},
		overrides: []evals.DefinitionArg{{Name: "rounds", Value: "2"}},
	})

	// closed-set-refused
	tests = append(tests, &testCase{
		_id:               "closed-set-refused",
		_expected:         "",
		_expectedSentinel: ErrClosedArg,
		_shouldPass:       false,

		def:       &evals.Definition{Goldens: reviewGoldens, Invocation: reviewInvocation, Skill: "review"},
		golden:    &ManifestGolden{Kind: GoldenTest},
		overrides: []evals.DefinitionArg{{Name: "scope", Value: "wip"}},
	})

	// placeholders-and-handshake
	prebuild := &evals.DefinitionPrebuild{Command: []string{"make", "build-audit"}, Outputs: []string{"bin/acdsl", "bin/verifiers"}}
	excludedRange := evals.DefinitionRange{BaseSha: "b4981b88", ExcludePaths: []string{"sessions/", "plans/"}, HeadSha: "1f725ced"}
	placeholderArgs := []evals.DefinitionArg{
		{Name: "scope", Value: "branch[{base}]"},
		{Name: "head", Value: "{head}"},
		{Name: "exclude", Value: "{exclude}"},
		{Name: "prebuilt", Value: "{prebuilt}"},
		{Name: "run-id", Value: "{runId}"},
	}
	tests = append(tests, &testCase{
		_id:               "placeholders-and-handshake",
		_expected:         "/review the branch scope: branch[b4981b88] head: 1f725ced exclude: sessions/ plans/ prebuilt: bin/acdsl bin/verifiers run-id: {runId} " + handshake,
		_expectedSentinel: nil,
		_shouldPass:       true,

		def: &evals.Definition{
			Environment: evals.DefinitionEnvironment{Prebuild: prebuild, Range: excludedRange},
			Findings:    evals.DefinitionFindings{Glob: "known.json"},
			Invocation:  evals.DefinitionInvocation{Args: placeholderArgs, Text: "the branch"},
			Skill:       "review",
		},
		golden:    nil,
		overrides: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			prompt, err := renderInvocation(test.def, test.golden, test.overrides)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedSentinel, initSentinel(err))
			assert.Equal(t, test._expected, prompt)
		})
	}
}

func TestCheckWindow(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedErrText      string
		_expectedIsOverWindow bool
		_shouldPass           bool

		budgets  *evals.DefinitionBudgets
		env      string
		runCount int
	}

	tests := make([]*testCase, 0)

	// under-window
	tests = append(tests, &testCase{
		_id:                   "under-window",
		_expectedErrText:      "",
		_expectedIsOverWindow: false,
		_shouldPass:           true,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 30, WindowUsd: 100},
		env:      "",
		runCount: 3,
	})

	// equal-to-window
	tests = append(tests, &testCase{
		_id:                   "equal-to-window",
		_expectedErrText:      "",
		_expectedIsOverWindow: false,
		_shouldPass:           true,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 100},
		env:      "",
		runCount: 2,
	})

	// over-window
	tests = append(tests, &testCase{
		_id:                   "over-window",
		_expectedErrText:      "3 runs × $50 = $150 over the $100 window",
		_expectedIsOverWindow: true,
		_shouldPass:           false,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 100},
		env:      "",
		runCount: 3,
	})

	// window-zero-default-passes
	tests = append(tests, &testCase{
		_id:                   "window-zero-default-passes",
		_expectedErrText:      "",
		_expectedIsOverWindow: false,
		_shouldPass:           true,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 0},
		env:      "",
		runCount: 3,
	})

	// window-zero-default-over
	tests = append(tests, &testCase{
		_id:                   "window-zero-default-over",
		_expectedErrText:      "over the $150 window",
		_expectedIsOverWindow: true,
		_shouldPass:           false,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 0},
		env:      "",
		runCount: 4,
	})

	// env-raises-window
	tests = append(tests, &testCase{
		_id:                   "env-raises-window",
		_expectedErrText:      "",
		_expectedIsOverWindow: false,
		_shouldPass:           true,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 100},
		env:      "1000",
		runCount: 3,
	})

	// malformed-env
	tests = append(tests, &testCase{
		_id:                   "malformed-env",
		_expectedErrText:      windowBudgetEnv,
		_expectedIsOverWindow: false,
		_shouldPass:           false,

		budgets:  &evals.DefinitionBudgets{ArmUsd: 50, WindowUsd: 100},
		env:      "lots",
		runCount: 1,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(windowBudgetEnv, test.env)
			err := checkWindow(test.budgets, test.runCount)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedIsOverWindow, errors.Is(err, ErrOverWindow))
			assert.Contains(t, initErrorText(err), test._expectedErrText)
		})
	}
}

func TestBlindLabels(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		count int
	}

	tests := make([]*testCase, 0)

	// zero-empty
	tests = append(tests, &testCase{
		_id:         "zero-empty",
		_expected:   []string{},
		_shouldPass: true,

		count: 0,
	})

	// three-permutation
	tests = append(tests, &testCase{
		_id:         "three-permutation",
		_expected:   []string{"A", "B", "C"},
		_shouldPass: true,

		count: 3,
	})

	// twenty-six-full-alphabet
	tests = append(tests, &testCase{
		_id:         "twenty-six-full-alphabet",
		_expected:   []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"},
		_shouldPass: true,

		count: 26,
	})

	// twenty-seven-err
	tests = append(tests, &testCase{
		_id:         "twenty-seven-err",
		_expected:   nil,
		_shouldPass: false,

		count: 27,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			labels, err := blindLabels(test.count)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.ElementsMatch(t, test._expected, labels)
		})
	}
}

func TestConfigKey(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		disabledServers []string
		effort          string
		model           string
	}

	tests := make([]*testCase, 0)

	// opus-high
	tests = append(tests, &testCase{
		_id:       "opus-high",
		_expected: "opus-high",

		disabledServers: nil,
		effort:          "high",
		model:           "opus",
	})

	// goland-disabled
	tests = append(tests, &testCase{
		_id:       "goland-disabled",
		_expected: "opus-high-no-goland",

		disabledServers: []string{"goland"},
		effort:          "high",
		model:           "opus",
	})

	// full-model-id-verbatim
	tests = append(tests, &testCase{
		_id:       "full-model-id-verbatim",
		_expected: "claude-opus-5-5[1m]-high",

		disabledServers: nil,
		effort:          "high",
		model:           "claude-opus-5-5[1m]",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, configKey(test.disabledServers, test.effort, test.model))
		})
	}
}
