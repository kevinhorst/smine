package skilleval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
)

func candidatesJson(t *testing.T, candidates []map[string]json.RawMessage) string {
	t.Helper()
	data, err := json.Marshal(candidates)
	require.NoError(t, err)
	return string(data)
}

func TestExtractCandidates(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		artifactsPath string
		findings      *evals.DefinitionFindings
	}

	ledgerFindings := &evals.DefinitionFindings{ArrayKey: "", ClaimKeys: []string{"claim"}, Glob: "known.json"}
	reviewFindings := &evals.DefinitionFindings{ArrayKey: "ranked_plan", ClaimKeys: []string{"claim", "finding"}, Glob: "round-*/review.json"}
	tests := make([]*testCase, 0)

	// ledger-top-level-array
	ledgerDir := t.TempDir()
	writeTestFile(t, filepath.Join(ledgerDir, "known.json"), `[
		{"finding_id":"correctness-MAJ-1","file":"a.go","line":3,"severity":"MAJOR","fix":"guard nil","claim":"nil map write","route":"auto-fix","fix_state":"fixed","round":1,"effect":"incorrect","exposure":"rare"},
		{"finding_id":"tests-MIN-1","file":"b.go","line":9,"severity":"MINOR","claim":"no test","route":"human","fix_state":"open","round":2}
	]`)
	tests = append(tests, &testCase{
		_id: "ledger-top-level-array",
		_expected: `[
			{"claim":"nil map write","effect":"incorrect","exposure":"rare","file":"a.go","finding_id":"r1-correctness-MAJ-1","fix":"guard nil","fix_state":"fixed","line":3,"round":1,"route":"auto-fix","severity":"MAJOR"},
			{"claim":"no test","file":"b.go","finding_id":"r2-tests-MIN-1","fix_state":"open","line":9,"round":2,"route":"human","severity":"MINOR"}
		]`,
		_shouldPass: true,

		artifactsPath: ledgerDir,
		findings:      ledgerFindings,
	})

	// array-key-document
	reviewDir := t.TempDir()
	writeTestFile(t, filepath.Join(reviewDir, "round-1", "review.json"), `{"ranked_plan":[
		{"finding_id":"F1","file":"a.go","line":3,"severity":"MAJOR","claim":"nil map write","direction":"correctness"},
		{"finding_id":"F2","file":"b.go","line":9,"severity":"NIT","claim":null,"finding":"typo in log prefix"}
	]}`)
	tests = append(tests, &testCase{
		_id: "array-key-document",
		_expected: `[
			{"claim":"nil map write","file":"a.go","finding_id":"F1","line":3,"severity":"MAJOR"},
			{"claim":"typo in log prefix","file":"b.go","finding_id":"F2","line":9,"severity":"NIT"}
		]`,
		_shouldPass: true,

		artifactsPath: reviewDir,
		findings:      reviewFindings,
	})

	// round-prefixed-id
	roundDir := t.TempDir()
	writeTestFile(t, filepath.Join(roundDir, "known.json"), `[
		{"finding_id":"tests-MIN-1","claim":"first round","round":1},
		{"finding_id":"tests-MIN-1","claim":"second round","round":2}
	]`)
	tests = append(tests, &testCase{
		_id: "round-prefixed-id",
		_expected: `[
			{"claim":"first round","finding_id":"r1-tests-MIN-1","round":1},
			{"claim":"second round","finding_id":"r2-tests-MIN-1","round":2}
		]`,
		_shouldPass: true,

		artifactsPath: roundDir,
		findings:      ledgerFindings,
	})

	// no-round-keeps-id
	noRoundDir := t.TempDir()
	writeTestFile(t, filepath.Join(noRoundDir, "known.json"), `[{"finding_id":"tests-MIN-1","claim":"no round"}]`)
	tests = append(tests, &testCase{
		_id:         "no-round-keeps-id",
		_expected:   `[{"claim":"no round","finding_id":"tests-MIN-1"}]`,
		_shouldPass: true,

		artifactsPath: noRoundDir,
		findings:      ledgerFindings,
	})

	// fix-state-copied
	fixStateDir := t.TempDir()
	writeTestFile(t, filepath.Join(fixStateDir, "known.json"), `[{"finding_id":"F1","claim":"c","fix_state":"applied","round":1,"applied":true,"fixed":true}]`)
	tests = append(tests, &testCase{
		_id:         "fix-state-copied",
		_expected:   `[{"claim":"c","finding_id":"r1-F1","fix_state":"applied","round":1}]`,
		_shouldPass: true,

		artifactsPath: fixStateDir,
		findings:      ledgerFindings,
	})

	// fix-state-absent
	noFixStateDir := t.TempDir()
	writeTestFile(t, filepath.Join(noFixStateDir, "known.json"), `[{"finding_id":"F1","claim":"c","round":1}]`)
	tests = append(tests, &testCase{
		_id:         "fix-state-absent",
		_expected:   `[{"claim":"c","finding_id":"r1-F1","round":1}]`,
		_shouldPass: true,

		artifactsPath: noFixStateDir,
		findings:      ledgerFindings,
	})

	// malformed-round
	malformedDir := t.TempDir()
	writeTestFile(t, filepath.Join(malformedDir, "known.json"), `[{"finding_id":"F1","claim":"c","round":"one"}]`)
	tests = append(tests, &testCase{
		_id:         "malformed-round",
		_expected:   `null`,
		_shouldPass: false,

		artifactsPath: malformedDir,
		findings:      ledgerFindings,
	})

	// empty-ledger
	emptyDir := t.TempDir()
	writeTestFile(t, filepath.Join(emptyDir, "known.json"), `[]`)
	tests = append(tests, &testCase{
		_id:         "empty-ledger",
		_expected:   `[]`,
		_shouldPass: true,

		artifactsPath: emptyDir,
		findings:      ledgerFindings,
	})

	// no-artifact
	tests = append(tests, &testCase{
		_id:         "no-artifact",
		_expected:   `[]`,
		_shouldPass: true,

		artifactsPath: t.TempDir(),
		findings:      ledgerFindings,
	})

	// newest-of-two
	newestDir := t.TempDir()
	newerPath := filepath.Join(newestDir, "round-1", "review.json")
	olderPath := filepath.Join(newestDir, "round-2", "review.json")
	writeTestFile(t, newerPath, `{"ranked_plan":[{"finding_id":"NEW","claim":"from the newest artifact"}]}`)
	writeTestFile(t, olderPath, `{"ranked_plan":[{"finding_id":"OLD","claim":"from the older artifact"}]}`)
	newerTime := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)
	olderTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(newerPath, newerTime, newerTime))
	require.NoError(t, os.Chtimes(olderPath, olderTime, olderTime))
	tests = append(tests, &testCase{
		_id:         "newest-of-two",
		_expected:   `[{"claim":"from the newest artifact","finding_id":"NEW"}]`,
		_shouldPass: true,

		artifactsPath: newestDir,
		findings:      reviewFindings,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			candidates, err := extractCandidates(test.artifactsPath, test.findings)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.JSONEq(t, test._expected, candidatesJson(t, candidates))
		})
	}
}
