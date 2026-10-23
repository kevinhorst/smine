package plans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func decisionsSection(header string, row string) string {
	return "## Decisions\n\n" + header + "\n| :-- | :-- | :-- |\n" + row + "\n"
}

func TestParseDecisions(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []*Decision

		markdown string
	}

	header := "| ID | Brief | Adds |"
	tests := make([]*testCase, 0)

	// pass-counts-and-reuse
	tests = append(tests, &testCase{
		_id:       "pass-counts-and-reuse",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{"checks": 1, "files": 2}, Line: 5, Problems: nil}},

		markdown: decisionsSection(header, "| D1 | b | **Adds** 1 check, 2 files.<br>**Reuses** `plainLines`. |"),
	})

	// pass-nothing
	tests = append(tests, &testCase{
		_id:       "pass-nothing",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{}, Line: 5, Problems: nil}},

		markdown: decisionsSection(header, "| D1 | b | **Adds** nothing.<br>**Reuses** the trust route. |"),
	})

	// fail-no-adds-cell
	tests = append(tests, &testCase{
		_id:       "fail-no-adds-cell",
		_expected: []*Decision{{Id: "D1", Adds: nil, Line: 5, Problems: []string{"has no Adds cell"}}},

		markdown: decisionsSection("| ID | Brief | Why |", "| D1 | b | w |"),
	})

	// fail-word-outside-vocabulary
	tests = append(tests, &testCase{
		_id:       "fail-word-outside-vocabulary",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{"flags": 1}, Line: 5, Problems: []string{`counts "2 widgets", which is no kind of the vocabulary`}}},

		markdown: decisionsSection(header, "| D1 | b | **Adds** 1 flag, 2 widgets.<br>**Reuses** the gate. |"),
	})

	// fail-no-reuses-clause
	tests = append(tests, &testCase{
		_id:       "fail-no-reuses-clause",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{"exit codes": 2}, Line: 5, Problems: []string{"Adds cell names no carrier after **Reuses**"}}},

		markdown: decisionsSection(header, "| D1 | b | **Adds** 2 exit codes.<br>**Removes** the flag. |"),
	})

	// pass-facts-cell
	tests = append(tests, &testCase{
		_id:       "pass-facts-cell",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{}, Facts: "[A1](#assumptions), [F2](#baseline)", Line: 5, Problems: nil}},

		markdown: decisionsSection("| ID | Adds | Facts [AGENT-ONLY] |", "| D1 | **Adds** nothing<br>**Reuses** the gate | [A1](#assumptions), [F2](#baseline) |"),
	})

	// pass-no-facts-column
	tests = append(tests, &testCase{
		_id:       "pass-no-facts-column",
		_expected: []*Decision{{Id: "D1", Adds: map[string]int{}, Facts: "", Line: 5, Problems: nil}},

		markdown: decisionsSection(header, "| D1 | b | **Adds** nothing<br>**Reuses** the gate |"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseDecisions(test.markdown))
		})
	}
}

func TestAddsLine(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		decisions []*Decision
	}

	tests := make([]*testCase, 0)

	// totals-in-vocabulary-order
	tests = append(tests, &testCase{
		_id:       "totals-in-vocabulary-order",
		_expected: "adds: 3 files, 3 flags, 2 checks",

		decisions: []*Decision{{Id: "D1", Adds: map[string]int{"checks": 2, "files": 2}}, {Id: "D2", Adds: map[string]int{"files": 1, "flags": 3}}},
	})

	// singular-for-one
	tests = append(tests, &testCase{
		_id:       "singular-for-one",
		_expected: "adds: 1 exit code, 1 agent step",

		decisions: []*Decision{{Id: "D1", Adds: map[string]int{"agent steps": 1, "exit codes": 1}}},
	})

	// nothing
	tests = append(tests, &testCase{
		_id:       "nothing",
		_expected: "adds: nothing",

		decisions: []*Decision{{Id: "D1", Adds: map[string]int{}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, AddsLine(test.decisions))
		})
	}
}
