package reviewscope

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
)

var (
	laneDiffFiles = []string{"docs/guide.md", "docs/hooks/guard.md", "internal/a.go", "internal/a_test.go", "skills/x/SKILL.md"}
	laneTracked   = []string{"internal/a.go", "internal/a_test.go"}
)

// laneExpectation collects the plan and the on-disk manifests one
// PlanLanes call is expected to produce.
type laneExpectation struct {
	manifests map[string]*LaneManifest
	plan      *LanePlan
}

func newLaneExpectation() *laneExpectation {
	plan := &LanePlan{Lanes: []Lane{}, MissingContext: []string{}, Skipped: []SkippedLane{}}
	return &laneExpectation{manifests: map[string]*LaneManifest{}, plan: plan}
}

// lane expects one lane and its manifest next to the artifact.
func (e *laneExpectation) lane(request *LaneRequest, slug, chunk, direction string, files, partners []string, index int, walk *contextdocs.Walk) *laneExpectation {
	artifact := filepath.Join(request.RoundDir, slug+".json")
	root := ""
	if request.WorktreeRoot != "" {
		root = filepath.Join(request.WorktreeRoot, slug)
	}

	e.manifests[artifact+LaneManifestSuffix] = &LaneManifest{
		Artifact:  artifact,
		Direction: direction,
		Files:     append(slices.Clone(files), partners...),
		Head:      request.Head,
		Partners:  partners,
		Root:      root,
		Walk:      walk,
	}
	lane := Lane{Artifact: artifact, Chunk: chunk, Direction: direction, Files: len(files), Index: index, Manifest: artifact + LaneManifestSuffix, Slug: slug}
	e.plan.Lanes = append(e.plan.Lanes, lane)
	return e
}

// provideChecklistWalk is a group-first walk as a checklist agent writes it.
func provideChecklistWalk() *contextdocs.Walk {
	return &contextdocs.Walk{Groups: []contextdocs.WalkGroup{{Id: "contract-1", Items: []string{"signature holds"}}}, Shape: contextdocs.WalkShapeGroupFirst}
}

// provideLaneRequest is a one-lane request over laneDiffFiles with every
// direction's walk source present, writing into a fresh round dir.
func provideLaneRequest(t *testing.T, directions ...string) *LaneRequest {
	tokens := map[string][]string{}
	for _, direction := range []string{directionCodeStyle, directionCritical, directionDataIntegrity, directionSecurity, directionTests} {
		tokens[direction] = []string{"RULE-GOLANG-*"}
	}

	entry := contextdocs.RuleEntry{Id: "RULE-GOLANG-NAME-001", Source: "rules/go.md"}
	return &LaneRequest{
		Ceiling:      10,
		Checklists:   map[string]*contextdocs.Walk{directionContracts: provideChecklistWalk(), directionCorrectness: provideChecklistWalk()},
		DiffFiles:    laneDiffFiles,
		Directions:   directions,
		Done:         map[string][]string{},
		Head:         "0123456789abcdef0123456789abcdef01234567",
		Lanes:        1,
		Pairs:        BuildTestPairs(laneDiffFiles, laneTracked),
		RoundDir:     filepath.Join(t.TempDir(), "round-1"),
		RuleSet:      contextdocs.RuleSet{Entries: []contextdocs.RuleEntry{entry}},
		Tokens:       tokens,
		WorktreeRoot: "",
	}
}

// provideRuleWalk is the file-first walk BuildWalk derives from the request's
// one rule entry.
func provideRuleWalk() *contextdocs.Walk {
	return &contextdocs.Walk{Groups: []contextdocs.WalkGroup{{Id: "RULE-GOLANG-NAME", Items: []string{"RULE-GOLANG-NAME-001"}}}, Shape: contextdocs.WalkShapeFileFirst}
}

// readManifests decodes every lane manifest in the round dir, keyed by path.
func readManifests(t *testing.T, roundDir string) map[string]*LaneManifest {
	paths, err := filepath.Glob(filepath.Join(roundDir, "*"+LaneManifestSuffix))
	require.NoError(t, err)

	manifests := map[string]*LaneManifest{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)

		manifest := &LaneManifest{}
		require.NoError(t, json.Unmarshal(data, manifest))
		manifests[path] = manifest
	}
	return manifests
}

func TestPlanLanes(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedManifests map[string]*LaneManifest
		_expectedPlan      *LanePlan
		_shouldPass        bool

		request *LaneRequest
	}

	noPartners := []string{}
	tests := make([]*testCase, 0)

	// relevance-code-style
	request := provideLaneRequest(t, directionCodeStyle)
	expected := newLaneExpectation().lane(request, "code-style-lane1", "", directionCodeStyle, laneDiffFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-code-style",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-contracts
	request = provideLaneRequest(t, directionContracts)
	expected = newLaneExpectation().lane(request, "contracts-lane1", "", directionContracts, laneDiffFiles, noPartners, 1, provideChecklistWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-contracts",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-correctness
	request = provideLaneRequest(t, directionCorrectness)
	productionFiles := []string{"docs/guide.md", "docs/hooks/guard.md", "internal/a.go", "skills/x/SKILL.md"}
	expected = newLaneExpectation().lane(request, "correctness-lane1", "", directionCorrectness, productionFiles, noPartners, 1, provideChecklistWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-correctness",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-critical
	request = provideLaneRequest(t, directionCritical)
	expected = newLaneExpectation().lane(request, "critical-lane1", "", directionCritical, laneDiffFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-critical",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-data-integrity
	request = provideLaneRequest(t, directionDataIntegrity)
	expected = newLaneExpectation().lane(request, "data-integrity-lane1", "", directionDataIntegrity, laneDiffFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-data-integrity",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-security
	request = provideLaneRequest(t, directionSecurity)
	securityFiles := []string{"docs/hooks/guard.md", "internal/a.go", "internal/a_test.go", "skills/x/SKILL.md"}
	expected = newLaneExpectation().lane(request, "security-lane1", "", directionSecurity, securityFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-security",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-special-focus
	request = provideLaneRequest(t, directionSpecialFocus)
	expected = newLaneExpectation().lane(request, "special-focus-lane1", "", directionSpecialFocus, laneDiffFiles, noPartners, 1, nil)
	tests = append(tests, &testCase{
		_id:                "relevance-special-focus",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// relevance-tests
	request = provideLaneRequest(t, directionTests)
	testsFiles := []string{"internal/a.go", "internal/a_test.go", "skills/x/SKILL.md"}
	expected = newLaneExpectation().lane(request, "tests-lane1", "", directionTests, testsFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "relevance-tests",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// contracts-ignores-chunks
	request = provideLaneRequest(t, directionContracts, directionCritical)
	request.Chunks = []Chunk{{Files: []string{"internal/a.go"}, Name: "Core Files"}, {Files: []string{"docs/guide.md"}, Name: "docs"}}
	expected = newLaneExpectation().
		lane(request, "contracts-lane1", "", directionContracts, laneDiffFiles, noPartners, 1, provideChecklistWalk()).
		lane(request, "critical-core-files-lane1", "Core Files", directionCritical, []string{"internal/a.go"}, noPartners, 1, provideRuleWalk()).
		lane(request, "critical-docs-lane1", "docs", directionCritical, []string{"docs/guide.md"}, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "contracts-ignores-chunks",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// tests-lane-partners
	request = provideLaneRequest(t, directionTests)
	request.DiffFiles = []string{"internal/a.go"}
	request.Pairs = BuildTestPairs(request.DiffFiles, laneTracked)
	expected = newLaneExpectation().lane(request, "tests-lane1", "", directionTests, []string{"internal/a.go"}, []string{"internal/a_test.go"}, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "tests-lane-partners",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// coverage-done-excluded
	request = provideLaneRequest(t, directionCritical)
	request.Done = map[string][]string{directionCritical: {"docs/guide.md", "internal/a.go"}, directionTests: {"docs/hooks/guard.md"}}
	openFiles := []string{"docs/hooks/guard.md", "internal/a_test.go", "skills/x/SKILL.md"}
	expected = newLaneExpectation().lane(request, "critical-lane1", "", directionCritical, openFiles, noPartners, 1, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "coverage-done-excluded",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// empty-lane-skipped
	request = provideLaneRequest(t, directionCorrectness)
	request.DiffFiles = []string{"internal/a_test.go"}
	request.Pairs = BuildTestPairs(request.DiffFiles, laneTracked)
	expected = newLaneExpectation()
	expected.plan.Skipped = []SkippedLane{{Chunk: "", Direction: directionCorrectness, Reason: "no relevant files left"}}
	tests = append(tests, &testCase{
		_id:                "empty-lane-skipped",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// group-first-missing-checklist-error
	request = provideLaneRequest(t, directionContracts)
	request.Checklists = map[string]*contextdocs.Walk{}
	tests = append(tests, &testCase{
		_id:                "group-first-missing-checklist-error",
		_expectedManifests: map[string]*LaneManifest{},
		_expectedPlan:      nil,
		_shouldPass:        false,

		request: request,
	})

	// rule-direction-empty-walk-missing-context
	request = provideLaneRequest(t, directionCritical, directionSpecialFocus)
	request.Tokens = map[string][]string{}
	expected = newLaneExpectation().lane(request, "special-focus-lane1", "", directionSpecialFocus, laneDiffFiles, noPartners, 1, nil)
	expected.plan.MissingContext = []string{directionCritical}
	tests = append(tests, &testCase{
		_id:                "rule-direction-empty-walk-missing-context",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// special-focus-no-walk
	request = provideLaneRequest(t, directionSpecialFocus)
	request.Checklists = map[string]*contextdocs.Walk{}
	request.Tokens = map[string][]string{directionSpecialFocus: {"RULE-GOLANG-*"}}
	expected = newLaneExpectation().lane(request, "special-focus-lane1", "", directionSpecialFocus, laneDiffFiles, noPartners, 1, nil)
	tests = append(tests, &testCase{
		_id:                "special-focus-no-walk",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// unknown-direction-error
	request = provideLaneRequest(t, directionCritical, "style")
	tests = append(tests, &testCase{
		_id:                "unknown-direction-error",
		_expectedManifests: map[string]*LaneManifest{},
		_expectedPlan:      nil,
		_shouldPass:        false,

		request: request,
	})

	// explicit-root-per-lane
	request = provideLaneRequest(t, directionCritical)
	request.Lanes = 2
	request.WorktreeRoot = "/wt"
	expected = newLaneExpectation().
		lane(request, "critical-lane1", "", directionCritical, laneDiffFiles, noPartners, 1, provideRuleWalk()).
		lane(request, "critical-lane2", "", directionCritical, laneDiffFiles, noPartners, 2, provideRuleWalk())
	tests = append(tests, &testCase{
		_id:                "explicit-root-per-lane",
		_expectedManifests: expected.manifests,
		_expectedPlan:      expected.plan,
		_shouldPass:        true,

		request: request,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			plan, err := PlanLanes(test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedPlan, plan)
			assert.Equal(t, test._expectedManifests, readManifests(t, test.request.RoundDir))
		})
	}
}

func TestLanePlan_WriteKnownView(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedIdFloor   map[string]int
		_expectedKnown     []KnownEntry
		_expectedKnownPath string
		_shouldPass        bool

		handedInPath string
		plan         *LanePlan
		roundDir     string
	}

	tests := make([]*testCase, 0)

	// handed-in-then-own
	artifactsDir := t.TempDir()
	roundDir := filepath.Join(artifactsDir, "round-2")
	require.NoError(t, os.Mkdir(roundDir, 0o755))
	ownRows := []KnownEntry{{File: "a.go", FindingId: "tests-MIN-3-r1"}}
	require.NoError(t, writeJson(filepath.Join(artifactsDir, knownFile), ownRows))
	handedInRows := []KnownEntry{
		{File: "b.go", FindingId: "tests-MAJ-7-r2"},
		{File: "c.go", FindingId: "correctness-NIT-2-r1"},
		{File: "d.go", FindingId: "legacy-1"},
	}
	handedInPath := filepath.Join(t.TempDir(), "seed.json")
	require.NoError(t, writeJson(handedInPath, handedInRows))
	tests = append(tests, &testCase{
		_id:                "handed-in-then-own",
		_expectedIdFloor:   map[string]int{"correctness": 2, "tests": 7},
		_expectedKnown:     append(slices.Clone(handedInRows), ownRows...),
		_expectedKnownPath: filepath.Join(roundDir, knownFile),
		_shouldPass:        true,

		handedInPath: handedInPath,
		plan:         &LanePlan{},
		roundDir:     roundDir,
	})

	// nothing-known
	roundDir = filepath.Join(t.TempDir(), "round-1")
	require.NoError(t, os.Mkdir(roundDir, 0o755))
	tests = append(tests, &testCase{
		_id:                "nothing-known",
		_expectedIdFloor:   map[string]int{},
		_expectedKnown:     nil,
		_expectedKnownPath: "",
		_shouldPass:        true,

		handedInPath: "",
		plan:         &LanePlan{},
		roundDir:     roundDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.plan.WriteKnownView(test.handedInPath, test.roundDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedKnownPath, test.plan.KnownPath)
			assert.Equal(t, test._expectedIdFloor, test.plan.IdFloor)
			assert.Equal(t, test._expectedKnown, readKnownFile(t, filepath.Join(test.roundDir, knownFile)))
		})
	}
}
