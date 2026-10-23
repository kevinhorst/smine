package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

// chainModel is a three-step model whose complete bound is 3: RUN-001 and
// RUN-002 each grant one atom, RUN-003 needs both.
func chainModel() *skillmodel.Model {
	vocab := &skillmodel.Vocabulary{Sorts: map[string][]skillmodel.Atom{"state": {{Id: "a"}, {Id: "b"}}}}
	model := &skillmodel.Model{
		IsStrict: true,
		Sections: []skillmodel.Section{{Id: "run", Title: "Run"}},
		Transitions: []skillmodel.Transition{
			{Id: "SKILL-DEMO-RUN-001", Kind: skillmodel.KindStep, Post: []string{"a"}, Section: "run", Text: "t"},
			{Id: "SKILL-DEMO-RUN-002", Kind: skillmodel.KindStep, Post: []string{"b"}, Section: "run", Text: "t"},
			{Id: "SKILL-DEMO-RUN-003", Kind: skillmodel.KindStep, Pred: "a and b", Section: "run", Text: "t"},
		},
		Vocabulary: vocab,
	}
	return model
}

// saveFixture saves a two-step demo skill — RUN-001 grants tools_loaded,
// RUN-002 is guarded by pred — into a nested temp dir with no vocabulary.cue
// above it, and returns the skill dir. Non-nil atoms opt it into the model
// tier.
func saveFixture(t *testing.T, atoms map[string]*skillgraph.Atom, pred string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "group", "demo")
	err := os.MkdirAll(dir, 0o755)
	require.NoError(t, err)

	grant := &skillgraph.Entry{
		Id:        "SKILL-DEMO-RUN-001",
		Post:      []string{"tools_loaded"},
		Role:      skillgraph.RoleStep,
		Statement: "Load the tools.",
		Title:     "Tool load",
	}
	guarded := &skillgraph.Entry{
		Id:        "SKILL-DEMO-RUN-002",
		Pred:      pred,
		Role:      skillgraph.RoleStep,
		Statement: "Run the tools.",
		Title:     "Tool run",
	}
	skill := &skillgraph.Skill{
		Atoms:       atoms,
		Changelog:   []skillgraph.ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: []skillgraph.Field{{Key: "name", Value: "demo"}, {Key: "version", Value: "1.0"}},
		Sections: []*skillgraph.Section{{
			Blocks: []skillgraph.Block{{Entry: grant}, {Entry: guarded}},
			Title:  "Run",
		}},
	}
	if atoms != nil {
		skill.Tier = skillgraph.TierStrict
	}
	err = skillgraph.Save(dir, skill)
	require.NoError(t, err)
	return dir
}

// modelAtoms opts saveFixture into the model tier — saved with tier strict:
// RUN-001 produces the atom.
func modelAtoms() map[string]*skillgraph.Atom {
	atoms := map[string]*skillgraph.Atom{"tools_loaded": {Sort: "state"}}
	return atoms
}

// writeQueryFile writes one query file into a temp dir and returns its path.
func writeQueryFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "query.json")
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
	return path
}

func TestEmitDocRejects(t *testing.T) {
	type testCase struct {
		_contains string
		_id       string

		bound       int
		consistency bool
		queryPath   string
		reach       string
	}

	tests := make([]*testCase, 0)

	// fail-zero-query-bound
	tests = append(tests, &testCase{
		_contains: "bound must be >= 1; omit for the default",
		_id:       "fail-zero-query-bound",

		queryPath: writeQueryFile(t, `{"name":"zero","reach":"SKILL-DEMO-RUN-003","bound":0,"expect":"unsat"}`),
	})

	// fail-negative-flag-bound
	tests = append(tests, &testCase{
		_contains: "bound must be >= 1; omit for the default",
		_id:       "fail-negative-flag-bound",

		bound: -1,
		reach: "SKILL-DEMO-RUN-003",
	})

	// fail-no-mode
	tests = append(tests, &testCase{
		_contains: "one of --consistency, --query, --reach is required",
		_id:       "fail-no-mode",
	})

	// fail-unparsable-assume
	tests = append(tests, &testCase{
		_contains: "emitReach: assume[0]",
		_id:       "fail-unparsable-assume",

		queryPath: writeQueryFile(t, `{"name":"bad","reach":"SKILL-DEMO-RUN-003","assume":["a and"],"expect":"unsat"}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := emitDoc(chainModel(), test.consistency, test.queryPath, test.reach, test.bound)
			assert.ErrorContains(t, err, test._contains)
		})
	}
}

func TestEmitDocBound(t *testing.T) {
	type testCase struct {
		_expectedBound         int
		_expectedCompleteBound int
		_id                    string

		bound       int
		consistency bool
		queryPath   string
		reach       string
	}

	tests := make([]*testCase, 0)

	// omitted-query-bound
	tests = append(tests, &testCase{
		_expectedBound:         3,
		_expectedCompleteBound: 3,
		_id:                    "omitted-query-bound",

		queryPath: writeQueryFile(t, `{"name":"plain","reach":"SKILL-DEMO-RUN-003","expect":"sat"}`),
	})

	// query-bound-below
	tests = append(tests, &testCase{
		_expectedBound:         1,
		_expectedCompleteBound: 3,
		_id:                    "query-bound-below",

		queryPath: writeQueryFile(t, `{"name":"low","reach":"SKILL-DEMO-RUN-003","bound":1,"expect":"unsat"}`),
	})

	// flag-default
	tests = append(tests, &testCase{
		_expectedBound:         3,
		_expectedCompleteBound: 3,
		_id:                    "flag-default",

		reach: "SKILL-DEMO-RUN-003",
	})

	// flag-bound
	tests = append(tests, &testCase{
		_expectedBound:         2,
		_expectedCompleteBound: 3,
		_id:                    "flag-bound",

		bound: 2,
		reach: "SKILL-DEMO-RUN-003",
	})

	// consistency
	tests = append(tests, &testCase{
		_expectedBound:         0,
		_expectedCompleteBound: 0,
		_id:                    "consistency",

		consistency: true,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			emitted, err := emitDoc(chainModel(), test.consistency, test.queryPath, test.reach, test.bound)
			require.NoError(t, err)
			assert.Equal(t, test._expectedBound, emitted.bound)
			assert.Equal(t, test._expectedCompleteBound, emitted.completeBound)
		})
	}
}

func TestEmission_BoundWarning(t *testing.T) {
	type testCase struct {
		_expected string
		_id       string

		emitted *emission
	}

	tests := make([]*testCase, 0)

	// below-complete
	tests = append(tests, &testCase{
		_expected: "skillmodel: warning: bound 1 is below the complete bound 3 — an unsat verdict is not a proof",
		_id:       "below-complete",
		emitted:   &emission{bound: 1, completeBound: 3},
	})

	// at-complete
	tests = append(tests, &testCase{_expected: "", _id: "at-complete", emitted: &emission{bound: 3, completeBound: 3}})

	// static-document
	tests = append(tests, &testCase{_expected: "", _id: "static-document", emitted: &emission{}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.emitted.boundWarning())
		})
	}
}

func TestEmission_ReachLine(t *testing.T) {
	type testCase struct {
		_expected string
		_id       string

		emitted *emission
		verdict string
	}

	tests := make([]*testCase, 0)

	// sat
	tests = append(tests, &testCase{_expected: "reachable", _id: "sat", emitted: &emission{bound: 1, completeBound: 3}, verdict: skillmodel.VerdictSat})

	// unsat-at-complete
	tests = append(tests, &testCase{_expected: "DEAD — unreachable within the complete bound", _id: "unsat-at-complete", emitted: &emission{bound: 3, completeBound: 3}, verdict: skillmodel.VerdictUnsat})

	// unsat-above-complete
	tests = append(tests, &testCase{_expected: "DEAD — unreachable within the complete bound", _id: "unsat-above-complete", emitted: &emission{bound: 5, completeBound: 3}, verdict: skillmodel.VerdictUnsat})

	// unsat-below-complete
	tests = append(tests, &testCase{_expected: "unreachable within bound 1", _id: "unsat-below-complete", emitted: &emission{bound: 1, completeBound: 3}, verdict: skillmodel.VerdictUnsat})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.emitted.reachLine(test.verdict))
		})
	}
}

func TestRunBundle(t *testing.T) {
	// bundle-usage-error
	assert.Equal(t, exitError, runBundle(nil))
}

func TestRunCheck(t *testing.T) {
	type testCase struct {
		_expected int
		_id       string

		args []string
	}

	annotationDir := saveFixture(t, nil, "tools_loaded")
	unparsableDir := saveFixture(t, nil, "tools_loaded and")
	modelDir := saveFixture(t, modelAtoms(), "tools_loaded")
	t.Setenv("PATH", "")
	tests := make([]*testCase, 0)

	// pass-annotation-tier
	tests = append(tests, &testCase{_expected: exitClean, _id: "pass-annotation-tier", args: []string{annotationDir}})

	// fail-unparsable-pred
	tests = append(tests, &testCase{_expected: exitViolations, _id: "fail-unparsable-pred", args: []string{unparsableDir}})

	// fail-model-tier-without-z3
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-model-tier-without-z3", args: []string{modelDir}})

	// fail-usage-no-dir
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-usage-no-dir", args: nil})

	// fail-missing-skill-cue
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-missing-skill-cue", args: []string{t.TempDir()}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runCheck(test.args))
		})
	}
}

func TestRunQuery(t *testing.T) {
	type testCase struct {
		_expected int
		_id       string

		args []string
	}

	annotationDir := saveFixture(t, nil, "tools_loaded")
	modelDir := saveFixture(t, modelAtoms(), "tools_loaded")
	t.Setenv("PATH", "")
	tests := make([]*testCase, 0)

	// fail-missing-expr
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-missing-expr", args: []string{modelDir}})

	// fail-missing-dir
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-missing-dir", args: []string{"--expr", "tools_loaded"}})

	// fail-annotation-tier
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-annotation-tier", args: []string{"--expr", "tools_loaded", annotationDir}})

	// fail-model-tier-without-z3
	tests = append(tests, &testCase{_expected: exitError, _id: "fail-model-tier-without-z3", args: []string{"--expr", "tools_loaded", modelDir}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runQuery(test.args))
		})
	}
}

func TestRunModelTier(t *testing.T) {
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not installed — brew install z3")
	}

	modelDir := saveFixture(t, modelAtoms(), "tools_loaded")
	assert.Equal(t, exitClean, runCheck([]string{modelDir}), "check")
	assert.Equal(t, exitClean, runQuery([]string{"--expr", "tools_loaded or not tools_loaded", modelDir}), "query-holds")
	assert.Equal(t, exitClean, runQuery([]string{"--expr", "tools_loaded", modelDir}), "query-counterexample")
}
