package skillmodel

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipWithoutZ3 skips a test that needs the z3 binary.
func skipWithoutZ3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not installed — brew install z3")
	}
}

// coreModel names one transition, one invariant and one soft clause — the
// entries core tokens may map back to.
func coreModel() *Model {
	model := &Model{
		Invariants:  []Invariant{{Id: "SKILL-A-RULES-001", Expr: "a"}},
		Soft:        []Invariant{{Id: "SKILL-A-QUAL-001", Expr: "a"}},
		Transitions: []Transition{{Id: "SKILL-A-PROC-001", Kind: KindStep}},
	}
	return model
}

func TestParseVerdicts(t *testing.T) {
	type testCase struct {
		_expected   []string
		_id         string
		_shouldPass bool

		output string
	}

	tests := make([]*testCase, 0)

	// error-before-verdict-fails
	tests = append(tests, &testCase{_expected: nil, _id: "error-before-verdict-fails", _shouldPass: false, output: "(error \"line 3: unknown constant x\")\nsat\n"})

	// error-after-verdict-ignored
	tests = append(tests, &testCase{_expected: []string{"unsat"}, _id: "error-after-verdict-ignored", _shouldPass: true, output: "unsat\n(a b)\n(error \"model is not available\")\n"})

	// three-verdicts-in-order
	tests = append(tests, &testCase{_expected: []string{"sat", "unsat", "unknown"}, _id: "three-verdicts-in-order", _shouldPass: true, output: "sat\nunsat\nunknown\n"})

	// no-output
	tests = append(tests, &testCase{_expected: nil, _id: "no-output", _shouldPass: true, output: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdicts, err := parseVerdicts(test.output)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, verdicts)
		})
	}
}

func TestRunZ3_MissingZ3(t *testing.T) {
	t.Setenv("PATH", "")
	_, _, err := runZ3(context.Background(), SmtDoc{Checks: 1, Text: "(check-sat)\n"})
	assert.ErrorContains(t, err, "z3 binary not found")
}

func TestRunZ3(t *testing.T) {
	skipWithoutZ3(t)

	t.Run("count-mismatch", func(t *testing.T) {
		_, _, err := runZ3(context.Background(), SmtDoc{Checks: 2, Text: "(check-sat)\n"})
		assert.ErrorContains(t, err, "Expected 2 verdicts, got 1: sat")
	})

	t.Run("undeclared-atom", func(t *testing.T) {
		doc := SmtDoc{Checks: 1, Text: "(declare-const a Bool)\n(declare-const ob.0 Bool)\n(assert (=> ob.0 (and undeclared a)))\n(check-sat-assuming (ob.0))\n"}
		_, _, err := runZ3(context.Background(), doc)
		assert.ErrorContains(t, err, "Error before the first verdict")
	})

	t.Run("cancelled-ctx", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, err := runZ3(ctx, SmtDoc{Checks: 1, Text: "(check-sat)\n"})
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("two-checks", func(t *testing.T) {
		doc := SmtDoc{Checks: 2, Text: "(declare-const a Bool)\n(assert (not a))\n(check-sat-assuming (a))\n(check-sat-assuming ((not a)))\n"}
		verdicts, _, err := runZ3(context.Background(), doc)
		require.NoError(t, err)
		assert.Equal(t, []string{VerdictUnsat, VerdictSat}, verdicts)
	})
}

func TestCoreEntryIds(t *testing.T) {
	type testCase struct {
		_expected []string
		_id       string

		output string
	}

	tests := make([]*testCase, 0)

	// step-suffix
	tests = append(tests, &testCase{_expected: []string{"SKILL-A-RULES-001"}, _id: "step-suffix", output: "unsat\n(SKILL_A_RULES_001.s3)\n"})

	// post-prefix
	tests = append(tests, &testCase{_expected: []string{"SKILL-A-PROC-001"}, _id: "post-prefix", output: "unsat\n(post.SKILL_A_PROC_001.s2)\n"})

	// stopblock-prefix
	tests = append(tests, &testCase{_expected: []string{"SKILL-A-PROC-001"}, _id: "stopblock-prefix", output: "unsat\n(stopblock.SKILL_A_PROC_001.s0)\n"})

	// reach-prefix
	tests = append(tests, &testCase{_expected: []string{"SKILL-A-PROC-001"}, _id: "reach-prefix", output: "unsat\n(reach.SKILL_A_PROC_001)\n"})

	// non-entries-dropped
	tests = append(tests, &testCase{_expected: nil, _id: "non-entries-dropped", output: "unsat\n(vocab.exclusive0.a.b halt.s2 frame.x.s1 init.x query.negation reach.assume0)\n"})

	// dedupe-model-order
	tests = append(tests, &testCase{
		_expected: []string{"SKILL-A-PROC-001", "SKILL-A-RULES-001", "SKILL-A-QUAL-001"},
		_id:       "dedupe-model-order",
		output:    "unsat\n(SKILL_A_QUAL_001 SKILL_A_RULES_001.s1 post.SKILL_A_PROC_001.s0 SKILL_A_RULES_001.s2)\n",
	})

	// error-after-core
	tests = append(tests, &testCase{_expected: []string{"SKILL-A-RULES-001"}, _id: "error-after-core", output: "unsat\n(SKILL_A_RULES_001)\n(error \"model is not available\")\n"})

	// sat-output
	tests = append(tests, &testCase{_expected: nil, _id: "sat-output", output: "sat\n(error \"unsat core is not available\")\n"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, coreEntryIds(coreModel(), test.output))
		})
	}
}

func TestSolve(t *testing.T) {
	t.Run("multi-check-refused", func(t *testing.T) {
		_, err := Solve(context.Background(), SmtDoc{Checks: 2, Text: "(check-sat)\n(check-sat)\n"}, coreModel())
		assert.ErrorContains(t, err, "Expected a single-check document, got 2 checks")
	})

	t.Run("unsat-core", func(t *testing.T) {
		skipWithoutZ3(t)
		model := &Model{
			Invariants: []Invariant{{Id: "SKILL-A-RULES-001", Expr: "budget >= 0"}, {Id: "SKILL-A-RULES-002", Expr: "budget < 0"}},
			Vocabulary: &Vocabulary{Sorts: map[string][]Atom{"count": {{Id: "budget", Kind: "int"}}}},
		}
		doc, err := EmitConsistency(model)
		require.NoError(t, err)
		result, err := Solve(context.Background(), doc, model)
		require.NoError(t, err)
		assert.Equal(t, VerdictUnsat, result.Verdict)
		assert.Equal(t, []string{"SKILL-A-RULES-001", "SKILL-A-RULES-002"}, result.Core)
		assert.Empty(t, result.Scenario)
	})

	t.Run("sat-scenario", func(t *testing.T) {
		skipWithoutZ3(t)
		model := &Model{Vocabulary: &Vocabulary{Sorts: map[string][]Atom{"state": {{Id: "b"}, {Id: "a"}}}}}
		doc := SmtDoc{Checks: 1, Text: "(declare-const a Bool)\n(declare-const b Bool)\n(assert (and a (not b)))\n(check-sat)\n(get-model)\n"}
		result, err := Solve(context.Background(), doc, model)
		require.NoError(t, err)
		assert.Equal(t, VerdictSat, result.Verdict)
		assert.Equal(t, []string{"a=true", "b=false"}, result.Scenario)
		assert.Empty(t, result.Core)
	})

	t.Run("unknown", func(t *testing.T) {
		skipWithoutZ3(t)
		doc := SmtDoc{Checks: 1, Text: "(set-option :timeout 1)\n(declare-const x Int)\n(declare-const y Int)\n(declare-const z Int)\n" +
			"(assert (= (+ (* x x x) (* y y y) (* z z z)) 33))\n(check-sat)\n"}
		result, err := Solve(context.Background(), doc, coreModel())
		require.NoError(t, err)
		assert.Equal(t, VerdictUnknown, result.Verdict)
		assert.Empty(t, result.Core)
		assert.Empty(t, result.Scenario)
	})
}
