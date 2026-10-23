package plans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const assumptionsHeader = "| ID | Assumption | Reality (brief) | Reality [AGENT-ONLY] | Location [AGENT-ONLY] |"

func assumptionsSection(header string, rows string) string {
	return "## Assumptions\n\n" + header + "\n| :-- | :-- | :-- | :-- | :-- |\n" + rows + "\n"
}

func TestParseAssumptions(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []*Assumption

		markdown string
	}

	tests := make([]*testCase, 0)

	// two-rows-with-all-cells
	tests = append(tests, &testCase{
		_id: "two-rows-with-all-cells",
		_expected: []*Assumption{
			{Id: "A1", Assumption: `"premise one"`, Line: 5, Location: "a.go:1", Reality: "long reality one"},
			{Id: "A2", Assumption: `"premise two"`, Line: 6, Location: "b.go:2-3", Reality: "long reality two"},
		},

		markdown: assumptionsSection(assumptionsHeader, "| A1 | \"premise one\" | **Brief** | long reality one | a.go:1 |\n| A2 | \"premise two\" | **Brief** | long reality two | b.go:2-3 |"),
	})

	// no-agent-tier-columns
	tests = append(tests, &testCase{
		_id:       "no-agent-tier-columns",
		_expected: []*Assumption{{Id: "A1", Assumption: `"premise"`, Line: 5, Location: "", Reality: ""}},

		markdown: assumptionsSection("| ID | Assumption | Reality (brief) |", "| A1 | \"premise\" | **Brief** |"),
	})

	// not-applicable-section
	tests = append(tests, &testCase{
		_id:       "not-applicable-section",
		_expected: nil,

		markdown: "## Assumptions\n\nN/A — the request states no premise\n\n## Drivers\n",
	})

	// non-assumption-id-skipped
	tests = append(tests, &testCase{
		_id:       "non-assumption-id-skipped",
		_expected: []*Assumption{{Id: "A1", Assumption: `"premise"`, Line: 6, Location: "a.go", Reality: "reality"}},

		markdown: assumptionsSection(assumptionsHeader, "| D1 | \"decision\" | **Brief** | other | c.go |\n| A1 | \"premise\" | **Brief** | reality | a.go |"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseAssumptions(test.markdown))
		})
	}
}
