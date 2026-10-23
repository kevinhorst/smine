package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/plans"
)

const (
	phaseTableHead      = "| Phase | Entries | Runs | Needs |\n| :-- | :-- | :-- | :-- |\n"
	phaseTableOne       = phaseTableHead + "| **Phase: 1** | [A](#a) | alone | — |\n\n"
	phaseTableTwo       = phaseTableHead + "| **Phase: 1** | [A](#a)<br>[B](#b) | parallel | — |\n\n"
	planSectionsHead    = "# Plan\n\nroute: `change`\n\n"
	sectionsAdds        = "adds: nothing\n"
	sectionsDrivers     = "## Drivers\n\n| ID | Impact |\n| :-- | :-- |\n| R1 | behavioral |\n| R2 | no-change |\n\n"
	sectionsBody        = sectionsDrivers + "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n" + phaseTableOne + "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n"
	sectionsDriverLines = "R1 covered (1)\nR2 no-change\n"
)

// captureRunPlanSections runs plan-sections on args and returns its exit
// code and everything it printed to stdout.
func captureRunPlanSections(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runPlanSections(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(printed)
}

// lineOf is the 1-based number of the first line of content starting with
// prefix.
func lineOf(content string, prefix string) int {
	return slices.IndexFunc(strings.Split(content, "\n"), func(line string) bool {
		return strings.HasPrefix(line, prefix)
	}) + 1
}

// providePlanSectionsFile writes content as the plan file name in dir and
// returns its path.
func providePlanSectionsFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestPlanSectionsLine(t *testing.T) {
	// design-gate
	assert.Equal(t, "plan-sections: ok — design gate", planSectionsLine(plans.GateDesign))

	// code-gate
	assert.Equal(t, "plan-sections: ok — code gate", planSectionsLine(plans.GateCode))
}

func TestRunPlanSections(t *testing.T) {
	type testCase struct {
		_id             string
		_expected       int
		_expectedStdout string

		args []string
	}

	dir := t.TempDir()
	twoFiles := "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n| `b.go` | new | B |\n\n" + phaseTableTwo
	tests := make([]*testCase, 0)

	// clean-plan
	clean := providePlanSectionsFile(t, dir, "clean.md", planSectionsHead+sectionsDrivers+twoFiles+"### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n\n### B\nlocation: `b.go`\nphase: 1\ndrivers: R1\n")
	tests = append(tests, &testCase{
		_id:             "clean-plan",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\n" + sectionsAdds + "R1 covered (2)\nR2 no-change\n",

		args: []string{clean},
	})

	// overlapping-plan
	overlappingContent := planSectionsHead + sectionsDrivers + twoFiles + "### A\nlocation: `a.go`, `b.go`\nphase: 1\ndrivers: R1\n\n### B\nlocation: `b.go`\nphase: 1\ndrivers: R1\n"
	overlapping := providePlanSectionsFile(t, dir, "overlapping.md", overlappingContent)
	tests = append(tests, &testCase{
		_id:             "overlapping-plan",
		_expected:       exitViolations,
		_expectedStdout: fmt.Sprintf("%s:%d: Changes: phase 1: b.go is touched by \"A\" and \"B\" — entries of one phase touch disjoint files\n", overlapping, lineOf(overlappingContent, "### B")),

		args: []string{overlapping},
	})

	// driver-lines-after-ok-line
	driverLines := providePlanSectionsFile(t, dir, "driver-lines.md", planSectionsHead+sectionsBody)
	tests = append(tests, &testCase{
		_id:             "driver-lines-after-ok-line",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\n" + sectionsAdds + sectionsDriverLines,

		args: []string{driverLines},
	})

	// no-driver-lines-at-design-gate
	designGate := providePlanSectionsFile(t, dir, "design-gate.md", planSectionsHead+sectionsDrivers)
	tests = append(tests, &testCase{
		_id:             "no-driver-lines-at-design-gate",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — design gate\n" + sectionsAdds,

		args: []string{designGate},
	})

	// uncovered-driver-exits-with-violation
	uncoveredContent := planSectionsHead + "## Drivers\n\n| ID | Impact |\n| :-- | :-- |\n| R1 | behavioral |\n| R3 | behavioral |\n\n## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n" + phaseTableOne + "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n"
	uncovered := providePlanSectionsFile(t, dir, "uncovered.md", uncoveredContent)
	tests = append(tests, &testCase{
		_id:             "uncovered-driver-exits-with-violation",
		_expected:       exitViolations,
		_expectedStdout: fmt.Sprintf("%s:%d: Drivers: driver R3 has no Changes entry — name it on an entry's drivers: line\n", uncovered, lineOf(uncoveredContent, "| R3 ")),

		args: []string{uncovered},
	})

	// ok-line-is-first-line
	okFirst := providePlanSectionsFile(t, dir, "ok-first.md", planSectionsHead+"## Drivers\n\n| ID | Impact | Check |\n| :-- | :-- | :-- |\n| R1 | behavioral | kept |\n| R2 | behavioral | rejected — out of scope |\n\n## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n"+phaseTableOne+"### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n")
	tests = append(tests, &testCase{
		_id:             "ok-line-is-first-line",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\n" + sectionsAdds + "R1 covered (1)\nR2 rejected\n",

		args: []string{okFirst},
	})

	// entry-order-violation
	hotBehindCode := planSectionsHead + sectionsDrivers + twoFiles + "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n\n### B\nlocation: `b.go`\nhot: validation\nphase: 1\ndrivers: R1\n"
	orderViolation := providePlanSectionsFile(t, dir, "entry-order.md", hotBehindCode)
	tests = append(tests, &testCase{
		_id:             "entry-order-violation",
		_expected:       exitViolations,
		_expectedStdout: fmt.Sprintf("%s:%d: Changes: entry \"B\" stands behind an entry of a later group — order is hot entries, other code, then %s entries\n", orderViolation, lineOf(hotBehindCode, "### B"), plans.AgentOnlyMarker),

		args: []string{orderViolation},
	})

	// entry-order-exempt-plan-dir
	exemptDesign := filepath.Join(dir, "plans", "2026-10-02-x-f1a2b3c", "2026-10-02-x-wip-abc1234", "design")
	require.NoError(t, os.MkdirAll(exemptDesign, 0o755))
	orderExempt := providePlanSectionsFile(t, exemptDesign, "x.md", hotBehindCode)
	tests = append(tests, &testCase{
		_id:             "entry-order-exempt-plan-dir",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\nR1 covered (2)\nR2 no-change\n",

		args: []string{orderExempt},
	})

	// adds-line-printed-after-ok-line
	decisionsWithAdds := "## Decisions\n\n| ID | Brief | Adds |\n| :-- | :-- | :-- |\n| D1 | b | **Adds** 1 check, 2 files.<br>**Reuses** `plainLines`. |\n| D2 | b | **Adds** nothing.<br>**Reuses** the gate. |\n\n"
	addsLine := providePlanSectionsFile(t, dir, "adds-line.md", planSectionsHead+decisionsWithAdds+sectionsBody)
	tests = append(tests, &testCase{
		_id:             "adds-line-printed-after-ok-line",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\nadds: 2 files, 1 check\n" + sectionsDriverLines,

		args: []string{addsLine},
	})

	// old-plan-dir-not-checked
	oldDesign := filepath.Join(dir, "plans", "2026-10-05-y-f1a2b3c", "2026-10-05-y-wip-abc1234", "design")
	require.NoError(t, os.MkdirAll(oldDesign, 0o755))
	unchecked := planSectionsHead + "## Decisions\n\n| ID | Brief |\n| :-- | :-- |\n| D1 | b |\n\n" + sectionsDrivers + "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n"
	oldPlan := providePlanSectionsFile(t, oldDesign, "y.md", unchecked)
	tests = append(tests, &testCase{
		_id:             "old-plan-dir-not-checked",
		_expected:       exitClean,
		_expectedStdout: "plan-sections: ok — code gate\n" + sectionsDriverLines,

		args: []string{oldPlan},
	})

	// expected-table-printed-on-mismatch
	fiveFiles := "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n| `b.go` | new | B |\n| `c.go` | new | C |\n| `d.go` | new | D |\n| `e.go` | new | E |\n\n"
	fiveEntries := "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\n\n### B\nlocation: `b.go`\nphase: 1\ndrivers: R1\n\n### C\nlocation: `c.go`\nphase: 1\ndrivers: R1\n\n### D\nlocation: `d.go`\nphase: 1\ndrivers: R1\n\n### E\nlocation: `e.go`\nphase: 1\ndrivers: R1\n"
	mismatchContent := planSectionsHead + sectionsDrivers + fiveFiles + fiveEntries
	mismatch := providePlanSectionsFile(t, dir, "mismatch.md", mismatchContent)
	changesLine := lineOf(mismatchContent, "## Changes")
	expectedTable := phaseTableHead + "| **Phase: 1** | [A](#a)<br>[B](#b)<br>[C](#c)<br>[D](#d)<br>[E](#e) | parallel | |\n"
	tests = append(tests, &testCase{
		_id:             "expected-table-printed-on-mismatch",
		_expected:       exitViolations,
		_expectedStdout: fmt.Sprintf("%s:%d: Changes: phase 1 holds 5 entries — at most 4, an entry is one agent's work package\n%s:%d: Changes: phase table (Phase | Entries | Runs | Needs) missing under the planned-files table\n", mismatch, changesLine, mismatch, changesLine) + expectedTable,

		args: []string{mismatch},
	})

	// no-argument
	tests = append(tests, &testCase{
		_id:             "no-argument",
		_expected:       exitError,
		_expectedStdout: "",

		args: nil,
	})

	// unreadable-path
	tests = append(tests, &testCase{
		_id:             "unreadable-path",
		_expected:       exitError,
		_expectedStdout: "",

		args: []string{filepath.Join(dir, "missing.md")},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, stdout := captureRunPlanSections(t, test.args)
			assert.Equal(t, test._expected, code)
			assert.Equal(t, test._expectedStdout, stdout)
		})
	}
}
