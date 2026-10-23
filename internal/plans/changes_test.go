package plans

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// changesEntry builds one ### entry; an empty location, phase or drivers
// omits its line.
func changesEntry(drivers string, heading string, location string, phase string) string {
	lines := []string{entryPrefix + heading}
	if location != "" {
		lines = append(lines, "location: "+location)
	}
	if phase != "" {
		lines = append(lines, "phase: "+phase)
	}
	if drivers != "" {
		lines = append(lines, "drivers: "+drivers)
	}
	return strings.Join(lines, "\n") + "\n"
}

// changesUnit builds a Changes unit: its opening heading, a planned-files
// table listing files, then the entries.
func changesUnit(heading string, files []string, entries ...string) string {
	lines := []string{heading, "", "| File | Kind | Entry |", "| :--- | :--- | :--- |"}
	for _, file := range files {
		lines = append(lines, "| `"+file+"` | new | x |")
	}
	lines = append(lines, "")
	lines = append(lines, entries...)
	return strings.Join(lines, "\n")
}

// driversSection builds a ## Drivers section with an Impact and a Check
// column holding rows.
func driversSection(rows ...string) string {
	lines := []string{"## Drivers", "", "| ID | Origin | Observed | Wanted | Impact | Check |", "| :-- | :-- | :-- | :-- | :-- | :-- |"}
	lines = append(lines, rows...)
	lines = append(lines, "", "")
	return strings.Join(lines, "\n")
}

func TestParseChanges(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Unit

		markdown string
	}

	table := "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n"
	tests := make([]*testCase, 0)

	// header
	tests = append(tests, &testCase{
		_id: "header",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", Line: 9, Phase: 1, PhaseText: "1"}},
			Line:       3,
			TableFiles: []string{"a.go"},
			TableLine:  5,
		},

		markdown: "# Plan\n\n" + table + "### A\nlocation: `a.go`\nphase: 1\n",
	})

	// fenced-heading
	tests = append(tests, &testCase{
		_id: "fenced-heading",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\n```go\n### not a heading\n```\n",
	})

	// line-suffix
	tests = append(tests, &testCase{
		_id: "line-suffix",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", Line: 7, Phase: 2, PhaseText: "2"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go:3` | new | A |\n\n### A\nlocation: `a.go:12-40`\nphase: 2\n",
	})

	// multi-span-cell
	tests = append(tests, &testCase{
		_id: "multi-span-cell",
		_expected: &Unit{
			Line:       1,
			TableFiles: []string{"a.go", "b.go"},
			TableLine:  3,
		},

		markdown: "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go`, `b.go` | new | A |\n",
	})

	// no-changes-section
	tests = append(tests, &testCase{
		_id:       "no-changes-section",
		_expected: nil,

		markdown: "# Plan\n\n## TLDR\n\n### A\nlocation: `a.go`\n",
	})

	// addenda-section-ignored
	tests = append(tests, &testCase{
		_id: "addenda-section-ignored",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\n\n## Addenda\n\n### A1\n#### B\nlocation: `b.go`\nphase: 1\n",
	})

	// second-changes-heading-ignored
	tests = append(tests, &testCase{
		_id: "second-changes-heading-ignored",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\n\n## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `b.go` | new | B |\n\n### B\nlocation: `b.go`\nphase: 1\n",
	})

	// drivers-line-plain-ids
	tests = append(tests, &testCase{
		_id: "drivers-line-plain-ids",
		_expected: &Unit{
			Entries:    []*Entry{{Drivers: []string{"R1", "R2"}, Files: []string{"a.go"}, HasDrivers: true, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1, R2\n",
	})

	// drivers-line-linked-ids
	tests = append(tests, &testCase{
		_id: "drivers-line-linked-ids",
		_expected: &Unit{
			Entries:    []*Entry{{Drivers: []string{"R1", "R3"}, Files: []string{"a.go"}, HasDrivers: true, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\ndrivers: [R1](#drivers), [R3](#drivers)\n",
	})

	// drivers-line-twice-first-wins
	tests = append(tests, &testCase{
		_id: "drivers-line-twice-first-wins",
		_expected: &Unit{
			Entries:    []*Entry{{Drivers: []string{"R1"}, Files: []string{"a.go"}, HasDrivers: true, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\ndrivers: R1\ndrivers: R2\n",
	})

	// drivers-line-without-id
	tests = append(tests, &testCase{
		_id: "drivers-line-without-id",
		_expected: &Unit{
			Entries:    []*Entry{{Drivers: nil, Files: []string{"a.go"}, HasDrivers: true, HasLocation: true, Heading: "A", Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nphase: 1\ndrivers: none\n",
	})

	// hot-line
	tests = append(tests, &testCase{
		_id: "hot-line",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "A", IsHot: true, Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### A\nlocation: `a.go`\nhot: validation\nphase: 1\n",
	})

	// agent-only-heading
	tests = append(tests, &testCase{
		_id: "agent-only-heading",
		_expected: &Unit{
			Entries:    []*Entry{{Files: []string{"a.go"}, HasLocation: true, Heading: "Prose (modified) [AGENT-ONLY]", IsAgentOnly: true, Line: 7, Phase: 1, PhaseText: "1"}},
			Line:       1,
			TableFiles: []string{"a.go"},
			TableLine:  3,
		},

		markdown: table + "### Prose (modified) [AGENT-ONLY]\nlocation: `a.go`\nphase: 1\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseChanges(test.markdown))
		})
	}
}

func TestParseDrivers(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []*Driver

		markdown string
	}

	tests := make([]*testCase, 0)

	// five-column-table
	tests = append(tests, &testCase{
		_id:       "five-column-table",
		_expected: []*Driver{{Id: "R1", Line: 5}, {Id: "R2", Line: 6}},

		markdown: "## Drivers\n\n| ID | Origin | Observed | Wanted | Impact |\n| :-- | :-- | :-- | :-- | :-- |\n| R1 | o | b | w | behavioral |\n| R2 | o | b | w | contract-touching |\n",
	})

	// six-column-table
	tests = append(tests, &testCase{
		_id:       "six-column-table",
		_expected: []*Driver{{Id: "R1", Line: 5}, {Id: "R2", IsRejected: true, Line: 6}},

		markdown: driversSection("| R1 | o | b | w | behavioral | ok |", "| R2 | o | b | w | behavioral | rejected |"),
	})

	// no-drivers-section
	tests = append(tests, &testCase{
		_id:       "no-drivers-section",
		_expected: nil,

		markdown: "# Plan\n\n## TLDR\n\n| ID | Origin | Impact |\n| :-- | :-- | :-- |\n| R1 | o | behavioral |\n",
	})

	// fenced-table-ignored
	tests = append(tests, &testCase{
		_id:       "fenced-table-ignored",
		_expected: []*Driver{{Id: "R1", IsNoChange: true, Line: 9}},

		markdown: "## Drivers\n\n```\n| ID | Impact |\n| R9 | behavioral |\n```\n| ID | Origin | Impact |\n| :-- | :-- | :-- |\n| R1 | o | no-change |\n",
	})

	// non-driver-row-skipped
	tests = append(tests, &testCase{
		_id:       "non-driver-row-skipped",
		_expected: []*Driver{{Id: "R1", Line: 6}},

		markdown: driversSection("| — | o | b | w | behavioral | ok |", "| R1 | o | b | w | behavioral | ok |", "| R1a | o | b | w | behavioral | ok |"),
	})

	// impact-no-change
	tests = append(tests, &testCase{
		_id:       "impact-no-change",
		_expected: []*Driver{{Id: "R1", IsNoChange: true, Line: 5}},

		markdown: driversSection("| R1 | o | b | w | no-change | ok |"),
	})

	// check-rejected-with-reason
	tests = append(tests, &testCase{
		_id:       "check-rejected-with-reason",
		_expected: []*Driver{{Id: "R1", IsRejected: true, Line: 5}},

		markdown: driversSection("| R1 | o | b | w | behavioral | rejected — duplicate of R2 |"),
	})

	// pipe-in-origin-cell-stays-strict
	tests = append(tests, &testCase{
		_id:       "pipe-in-origin-cell-stays-strict",
		_expected: []*Driver{{Id: "R1", Line: 5}},

		markdown: driversSection("| R1 | \"a | b\" | b | w | no-change | ok |"),
	})

	// table-in-other-section-ignored
	tests = append(tests, &testCase{
		_id:       "table-in-other-section-ignored",
		_expected: []*Driver{{Id: "R1", Line: 11}},

		markdown: "## Assumptions\n\n| Impact | ID |\n| :-- | :-- |\n| no-change | R5 |\n\n## Drivers\n\n| ID | Origin | Impact |\n| :-- | :-- | :-- |\n| R1 | o | behavioral |\n\n## Decisions\n\n| ID | Problem |\n| :-- | :-- |\n| R2 | x |\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseDrivers(test.markdown))
		})
	}
}

func TestCheckDrivers(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		markdown string
	}

	behavioral := "| R1 | o | b | w | behavioral | ok |"
	behavioralSecond := "| R2 | o | b | w | behavioral | ok |"
	tests := make([]*testCase, 0)

	// pass-all-covered
	tests = append(tests, &testCase{
		_id:         "pass-all-covered",
		_violations: 0,

		markdown: driversSection(behavioral, behavioralSecond) + changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("R1", "A", "`a.go`", "1"), changesEntry("R2", "B", "`b.go`", "1")),
	})

	// pass-one-entry-serves-two
	tests = append(tests, &testCase{
		_id:         "pass-one-entry-serves-two",
		_violations: 0,

		markdown: driversSection(behavioral, behavioralSecond) + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1, R2", "A", "`a.go`", "1")),
	})

	// fail-driver-without-entry
	tests = append(tests, &testCase{
		_id:         "fail-driver-without-entry",
		_violations: 1,

		markdown: driversSection(behavioral, behavioralSecond) + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1")),
	})

	// fail-entry-without-drivers-line
	tests = append(tests, &testCase{
		_id:         "fail-entry-without-drivers-line",
		_violations: 1,

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("R1", "A", "`a.go`", "1"), changesEntry("", "B", "`b.go`", "1")),
	})

	// fail-entry-names-no-id
	tests = append(tests, &testCase{
		_id:         "fail-entry-names-no-id",
		_violations: 1,

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("R1", "A", "`a.go`", "1"), changesEntry("none", "B", "`b.go`", "1")),
	})

	// fail-unknown-driver-id
	tests = append(tests, &testCase{
		_id:         "fail-unknown-driver-id",
		_violations: 1,

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1, R7", "A", "`a.go`", "1")),
	})

	// pass-no-change-row
	tests = append(tests, &testCase{
		_id:         "pass-no-change-row",
		_violations: 0,

		markdown: driversSection(behavioral, "| R2 | o | b | w | no-change | ok |") + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1")),
	})

	// pass-rejected-row
	tests = append(tests, &testCase{
		_id:         "pass-rejected-row",
		_violations: 0,

		markdown: driversSection(behavioral, "| R2 | o | b | w | behavioral | rejected — out of scope |") + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1")),
	})

	// fail-no-driver-rows
	tests = append(tests, &testCase{
		_id:         "fail-no-driver-rows",
		_violations: 1,

		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckDrivers(ParseDrivers(test.markdown), ParseChanges(test.markdown))

			assert.Len(t, violations, test._violations, "%v", violations)
		})
	}
}

func TestCheckOrder(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		markdown string
	}

	hotLine := "hot: validation\n"
	code := changesEntry("", "Code", "`c.go`", "1")
	codeSecond := changesEntry("", "Code second", "`d.go`", "1")
	hot := changesEntry("", "Hot", "`h.go`", "1") + hotLine
	prose := changesEntry("", "Prose "+AgentOnlyMarker, "`p.md`", "1")
	proseSecond := changesEntry("", "Prose second "+AgentOnlyMarker, "`q.md`", "1")
	tests := make([]*testCase, 0)

	// pass-hot-code-prose
	tests = append(tests, &testCase{
		_id:         "pass-hot-code-prose",
		_violations: 0,

		markdown: changesUnit("## Changes", nil, hot, code, prose),
	})

	// pass-code-only
	tests = append(tests, &testCase{
		_id:         "pass-code-only",
		_violations: 0,

		markdown: changesUnit("## Changes", nil, code, codeSecond),
	})

	// pass-prose-only
	tests = append(tests, &testCase{
		_id:         "pass-prose-only",
		_violations: 0,

		markdown: changesUnit("## Changes", nil, prose, proseSecond),
	})

	// pass-no-entries
	tests = append(tests, &testCase{
		_id:         "pass-no-entries",
		_violations: 0,

		markdown: changesUnit("## Changes", []string{"a.go"}),
	})

	// fail-hot-behind-code
	tests = append(tests, &testCase{
		_id:         "fail-hot-behind-code",
		_violations: 1,

		markdown: changesUnit("## Changes", nil, code, hot),
	})

	// fail-hot-behind-prose
	tests = append(tests, &testCase{
		_id:         "fail-hot-behind-prose",
		_violations: 1,

		markdown: changesUnit("## Changes", nil, prose, hot),
	})

	// fail-code-behind-prose
	tests = append(tests, &testCase{
		_id:         "fail-code-behind-prose",
		_violations: 1,

		markdown: changesUnit("## Changes", nil, prose, code),
	})

	// fail-marked-entry-with-hot-line
	tests = append(tests, &testCase{
		_id:         "fail-marked-entry-with-hot-line",
		_violations: 1,

		markdown: changesUnit("## Changes", nil, code, prose+hotLine),
	})

	// fail-two-late-entries
	tests = append(tests, &testCase{
		_id:         "fail-two-late-entries",
		_violations: 2,

		markdown: changesUnit("## Changes", nil, prose, code, hot),
	})

	// pass-marker-without-space
	tests = append(tests, &testCase{
		_id:         "pass-marker-without-space",
		_violations: 0,

		markdown: changesUnit("## Changes", nil, changesEntry("", "X"+AgentOnlyMarker, "`x.go`", "1"), code),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckOrder(ParseChanges(test.markdown))

			assert.Len(t, violations, test._violations, "%v", violations)
		})
	}
}

func TestDriverLines(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		markdown string
	}

	behavioral := "| R1 | o | b | w | behavioral | ok |"
	tests := make([]*testCase, 0)

	// covered-count
	tests = append(tests, &testCase{
		_id:       "covered-count",
		_expected: []string{"R1 covered (2)"},

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("R1", "A", "`a.go`", "1"), changesEntry("R1", "B", "`b.go`", "1")),
	})

	// duplicate-id-counts-once
	tests = append(tests, &testCase{
		_id:       "duplicate-id-counts-once",
		_expected: []string{"R1 covered (1)"},

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1, [R1](#drivers)", "A", "`a.go`", "1")),
	})

	// no-change
	tests = append(tests, &testCase{
		_id:       "no-change",
		_expected: []string{"R1 no-change"},

		markdown: driversSection("| R1 | o | b | w | no-change | ok |") + changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1")),
	})

	// rejected
	tests = append(tests, &testCase{
		_id:       "rejected",
		_expected: []string{"R1 rejected"},

		markdown: driversSection("| R1 | o | b | w | behavioral | rejected — out of scope |") + changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1")),
	})

	// no-entry
	tests = append(tests, &testCase{
		_id:       "no-entry",
		_expected: []string{"R1 no entry"},

		markdown: driversSection(behavioral) + changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, DriverLines(ParseDrivers(test.markdown), ParseChanges(test.markdown)))
		})
	}
}

func TestCheckUnit(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		isStrict bool
		markdown string
	}

	tests := make([]*testCase, 0)

	// pass-disjoint-phase
	tests = append(tests, &testCase{
		_id:         "pass-disjoint-phase",
		_violations: 0,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("", "A", "`a.go`", "1"), changesEntry("", "B", "`b.go`", "1")),
	})

	// pass-shared-across-phases
	tests = append(tests, &testCase{
		_id:         "pass-shared-across-phases",
		_violations: 0,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1"), changesEntry("", "B", "`a.go`", "2")),
	})

	// fail-overlap
	tests = append(tests, &testCase{
		_id:         "fail-overlap",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1"), changesEntry("", "B", "`a.go`", "1")),
	})

	// fail-missing-phase-strict
	tests = append(tests, &testCase{
		_id:         "fail-missing-phase-strict",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "")),
	})

	// pass-legacy-lenient
	tests = append(tests, &testCase{
		_id:         "pass-legacy-lenient",
		_violations: 0,

		isStrict: false,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "")),
	})

	// fail-brace-span
	tests = append(tests, &testCase{
		_id:         "fail-brace-span",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", nil, changesEntry("", "A", "`x/{a,b}.go`", "1")),
	})

	// fail-directory-span
	tests = append(tests, &testCase{
		_id:         "fail-directory-span",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", nil, changesEntry("", "A", "`internal/plans/`", "1")),
	})

	// fail-table-extra
	tests = append(tests, &testCase{
		_id:         "fail-table-extra",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go", "b.go"}, changesEntry("", "A", "`a.go`", "1")),
	})

	// fail-location-extra
	tests = append(tests, &testCase{
		_id:         "fail-location-extra",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`, `b.go`", "1")),
	})

	// fail-no-table-strict
	tests = append(tests, &testCase{
		_id:         "fail-no-table-strict",
		_violations: 1,

		isStrict: true,
		markdown: "## Changes\n\n" + changesEntry("", "A", "`a.go`", "1"),
	})

	// fail-phase-zero
	tests = append(tests, &testCase{
		_id:         "fail-phase-zero",
		_violations: 1,

		isStrict: true,
		markdown: changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "0")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckUnit(ParseChanges(test.markdown), test.isStrict)

			assert.Len(t, violations, test._violations, "%v", violations)
		})
	}
}

func TestCheckPhases(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		markdown string
	}

	phaseHead := "| Phase | Entries | Runs | Needs |\n| :-- | :-- | :-- | :-- |\n"
	files := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	entryA := changesEntry("", "A", "`a.go`", "1")
	entryB := changesEntry("", "B", "`b.go`", "1")
	entryBLate := changesEntry("", "B", "`b.go`", "2")
	fiveEntries := []string{entryA, entryB, changesEntry("", "C", "`c.go`", "1"), changesEntry("", "D", "`d.go`", "1"), changesEntry("", "E", "`e.go`", "1")}
	tests := make([]*testCase, 0)

	// pass-table-matches
	tests = append(tests, &testCase{
		_id:         "pass-table-matches",
		_violations: 0,

		markdown: changesUnit("## Changes", files[:2], phaseHead+"| **Phase: 1** | [B](#b)<br>[A](#a) | parallel | — |\n", entryA, entryB),
	})

	// fail-five-entries-in-phase
	fiveTable := phaseHead + "| **Phase: 1** | [A](#a)<br>[B](#b)<br>[C](#c)<br>[D](#d)<br>[E](#e) | parallel | — |\n"
	tests = append(tests, &testCase{
		_id:         "fail-five-entries-in-phase",
		_violations: 1,

		markdown: changesUnit("## Changes", files, append([]string{fiveTable}, fiveEntries...)...),
	})

	// fail-table-missing
	tests = append(tests, &testCase{
		_id:         "fail-table-missing",
		_violations: 1,

		markdown: changesUnit("## Changes", files[:2], entryA, entryB),
	})

	// fail-entry-in-wrong-row
	tests = append(tests, &testCase{
		_id:         "fail-entry-in-wrong-row",
		_violations: 1,

		markdown: changesUnit("## Changes", files[:2], phaseHead+"| **Phase: 1** | [B](#b) | alone | — |\n| **Phase: 2** | [A](#a) | alone | Phase: 1 |\n", entryA, entryBLate),
	})

	// fail-runs-alone-for-two
	tests = append(tests, &testCase{
		_id:         "fail-runs-alone-for-two",
		_violations: 1,

		markdown: changesUnit("## Changes", files[:2], phaseHead+"| **Phase: 1** | [A](#a)<br>[B](#b) | alone | — |\n", entryA, entryB),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckPhases(ParseChanges(test.markdown))

			assert.Len(t, violations, test._violations, "%v", violations)
		})
	}
}

func TestCheckPlan(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		markdown string
	}

	head := "# Plan\n\nroute: `change`\n\n"
	drivers := driversSection("| R1 | o | b | w | behavioral | ok |")
	uncoveredDrivers := driversSection("| R1 | o | b | w | behavioral | ok |", "| R2 | o | b | w | behavioral | ok |")
	covered := changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1"))
	tests := make([]*testCase, 0)

	// no-changes
	tests = append(tests, &testCase{
		_id:         "no-changes",
		_violations: 0,

		markdown: head + uncoveredDrivers + "## TLDR\n",
	})

	// covered
	tests = append(tests, &testCase{
		_id:         "covered",
		_violations: 0,

		markdown: head + drivers + covered,
	})

	// phased-overlap
	tests = append(tests, &testCase{
		_id:         "phased-overlap",
		_violations: 1,

		markdown: head + drivers + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "1"), changesEntry("R1", "B", "`a.go`", "1")),
	})

	// entries-strict-unphased
	tests = append(tests, &testCase{
		_id:         "entries-strict-unphased",
		_violations: 1,

		markdown: head + drivers + changesUnit("## Changes", []string{"a.go"}, changesEntry("R1", "A", "`a.go`", "")),
	})

	// entries-uncovered-driver
	tests = append(tests, &testCase{
		_id:         "entries-uncovered-driver",
		_violations: 1,

		markdown: head + uncoveredDrivers + covered,
	})

	// without-entries-skips-drivers
	tests = append(tests, &testCase{
		_id:         "without-entries-skips-drivers",
		_violations: 0,

		markdown: head + uncoveredDrivers + changesUnit("## Changes", []string{"a.go"}),
	})

	// master-split-table-skips-drivers
	tests = append(tests, &testCase{
		_id:         "master-split-table-skips-drivers",
		_violations: 0,

		markdown: "# Plan\n\nrole: `master`\n\n" + uncoveredDrivers + "## Changes\n\n| Derived plan | Carries | Decisions | Depends on |\n| :-- | :-- | :-- | :-- |\n| plan-a | `a.go` | D1 | — |\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckPlan(ParseDrivers(test.markdown), ParseChanges(test.markdown))

			assert.Len(t, violations, test._violations, "%v", violations)
		})
	}
}

func TestGate(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		markdown string
	}

	head := "# Plan\n\nroute: `change`\n\n"
	header := changesUnit("## Changes", []string{"a.go"}, changesEntry("", "A", "`a.go`", "1"))
	tests := make([]*testCase, 0)

	// no-changes-section
	tests = append(tests, &testCase{
		_id:       "no-changes-section",
		_expected: GateDesign,

		markdown: head + "## TLDR\n",
	})

	// changes-heading-without-table
	tests = append(tests, &testCase{
		_id:       "changes-heading-without-table",
		_expected: GateDesign,

		markdown: head + "## Changes\n\n" + changesEntry("", "A", "`a.go`", "1"),
	})

	// table-without-entries
	tests = append(tests, &testCase{
		_id:       "table-without-entries",
		_expected: GateDesign,

		markdown: head + changesUnit("## Changes", []string{"a.go"}),
	})

	// with-entries
	tests = append(tests, &testCase{
		_id:       "with-entries",
		_expected: GateCode,

		markdown: head + header,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Gate(test.markdown))
		})
	}
}

func TestTableCells(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		row string
	}

	tests := make([]*testCase, 0)

	// plain-row
	tests = append(tests, &testCase{
		_id:       "plain-row",
		_expected: []string{"A1", "x", "y"},

		row: "| A1 | x | y |",
	})

	// escaped-pipe-stays-in-cell
	tests = append(tests, &testCase{
		_id:       "escaped-pipe-stays-in-cell",
		_expected: []string{"A3", "curl x | jq '.runs[]'", "z"},

		row: `| A3 | curl x \| jq '.runs[]' | z |`,
	})

	// empty-cells
	tests = append(tests, &testCase{
		_id:       "empty-cells",
		_expected: []string{"D1", "", "z"},

		row: "| D1 |  | z |",
	})

	// no-outer-pipes
	tests = append(tests, &testCase{
		_id:       "no-outer-pipes",
		_expected: []string{"a", "b"},

		row: "a | b",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, tableCells(test.row))
		})
	}
}
