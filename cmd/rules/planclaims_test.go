package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunPlanClaims runs plan-claims on args and returns its exit code and everything it printed to stdout.
func captureRunPlanClaims(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runPlanClaims(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(printed)
}

func TestRunPlanClaims(t *testing.T) {
	type testCase struct {
		_id             string
		_expected       int
		_expectedStdout string

		args []string
	}

	dir := t.TempDir()
	tests := make([]*testCase, 0)

	// two-assumptions-two-decisions
	decisions := "## Decisions\n\n| ID | Adds | Facts [AGENT-ONLY] |\n| :-- | :-- | :-- |\n| D1 | **Adds** nothing<br>**Reuses** x | [A1](#assumptions), [F1](#baseline) |\n| D2 | **Adds** nothing<br>**Reuses** x | [A1](#assumptions) |\n\n"
	assumptions := "## Assumptions\n\n| ID | Assumption | Reality (brief) | Reality [AGENT-ONLY] | Location [AGENT-ONLY] |\n| :-- | :-- | :-- | :-- | :-- |\n| A1 | premise one | **B** | reality one | a.go:1 |\n| A2 | premise two | **B** | reality two | b.go:2 |\n"
	linked := providePlanSectionsFile(t, dir, "linked.md", "# Plan\n\n"+decisions+assumptions)
	tests = append(tests, &testCase{
		_id:       "two-assumptions-two-decisions",
		_expected: exitClean,
		_expectedStdout: `{
  "claims": [
    {
      "claim_id": "A1",
      "evidence": "a.go:1",
      "source": "premise one — decisions: D1, D2",
      "text": "reality one"
    },
    {
      "claim_id": "A2",
      "evidence": "b.go:2",
      "source": "premise two — decisions: ",
      "text": "reality two"
    }
  ],
  "schema": 1
}
`,

		args: []string{linked},
	})

	// not-applicable-assumptions
	notApplicable := providePlanSectionsFile(t, dir, "not-applicable.md", "# Plan\n\n## Assumptions\n\nN/A — the request states no premise\n")
	tests = append(tests, &testCase{
		_id:             "not-applicable-assumptions",
		_expected:       exitClean,
		_expectedStdout: "{\n  \"claims\": [],\n  \"schema\": 1\n}\n",

		args: []string{notApplicable},
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:             "missing-file",
		_expected:       exitError,
		_expectedStdout: "",

		args: []string{filepath.Join(dir, "missing.md")},
	})

	// no-argument
	tests = append(tests, &testCase{
		_id:             "no-argument",
		_expected:       exitError,
		_expectedStdout: "",

		args: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, stdout := captureRunPlanClaims(t, test.args)
			assert.Equal(t, test._expected, code)
			assert.Equal(t, test._expectedStdout, stdout)
		})
	}
}
