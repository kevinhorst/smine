package contextdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrequired"
)

const (
	budgetKindGuide = "guide"
	budgetKindPack  = "always pack"
)

func TestValidateBudgets(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		contextDir string
		set        RuleSet
	}

	tests := make([]*testCase, 0)

	// pass-under-budget
	under := provideBudgetContext(t, 100, 100)
	tests = append(tests, &testCase{
		_id:       "pass-under-budget",
		_expected: []string{},

		contextDir: under,
		set:        provideBudgetSet(t, under),
	})

	// fail-guide-over-budget
	bigGuide := provideBudgetContext(t, contextrequired.ReadBudgetBytes+1, 100)
	tests = append(tests, &testCase{
		_id:       "fail-guide-over-budget",
		_expected: []string{budgetKindGuide},

		contextDir: bigGuide,
		set:        provideBudgetSet(t, bigGuide),
	})

	// fail-pack-over-budget
	bigPack := provideBudgetContext(t, 100, contextrequired.ReadBudgetBytes+1)
	tests = append(tests, &testCase{
		_id:       "fail-pack-over-budget",
		_expected: []string{budgetKindPack},

		contextDir: bigPack,
		set:        provideBudgetSet(t, bigPack),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, budgetKinds(ValidateBudgets(test.contextDir, test.set)))
		})
	}
}

// budgetKinds names what each budget violation is about, so a case asserts kinds instead of message text.
func budgetKinds(violations []string) []string {
	kinds := make([]string, 0, len(violations))
	for _, violation := range violations {
		kind := budgetKindGuide
		if strings.HasPrefix(violation, budgetKindPack+":") {
			kind = budgetKindPack
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

func provideBudgetSet(t *testing.T, contextDir string) RuleSet {
	set, err := ParseContext(contextDir, false)
	require.NoError(t, err)
	return set
}

func provideBudgetContext(t *testing.T, guideBytes int, statementBytes int) string {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "rules"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0o755))

	guideHead := "# GO CODE STYLE\n\n**Files:** `*.go`\n\n"
	guidePadding := strings.Repeat("x", max(guideBytes-len(guideHead)-1, 0))
	guideFile := guideHead + guidePadding + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules", "go.md"), []byte(guideFile), 0o644))

	statement := "Padded statement " + strings.Repeat("x", statementBytes) + "."
	actionsFile := "**ACTION-IMPL-EXEC-001** `[prose]` — " + statement + "\n\n* Applies: always.\n* Reach: always\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "implementing.md"), []byte(actionsFile), 0o644))
	return dir
}
