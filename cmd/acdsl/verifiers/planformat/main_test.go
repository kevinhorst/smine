package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/plans"
)

func TestCheckFile(t *testing.T) {
	type testCase struct {
		_id         string
		_violations int

		file        string
		headings    []heading
		permissions []heading
		unit        *plans.Unit
	}

	catalog := []heading{{text: "TLDR", line: 1}, {text: "Current state [AGENT-ONLY]", line: 7}, {text: "Changelog", line: 12}}
	withPermissions := []heading{{text: "TLDR", line: 1}, {text: "Permissions", line: 5}, {text: "Current state [AGENT-ONLY]", line: 7}, {text: "Changelog", line: 12}}
	reordered := []heading{{text: "TLDR", line: 1}, {text: "Changes", line: 3}, {text: "Scope", line: 5}, {text: "Changelog", line: 7}}
	table := "## Changes\n\n| File | Kind | Entry |\n| :--- | :--- | :--- |\n| `a.go` | new | A |\n\n"
	proseFirst := "### Docs [AGENT-ONLY]\nlocation: `a.go`\n\n### Code\nlocation: `a.go`\n"
	tests := make([]*testCase, 0)

	// permissions-section
	tests = append(tests, &testCase{
		_id:         "permissions-section",
		_violations: 1,

		file:        "plans/2026-10-02-x-abc1234/2026-10-02-x-wip-abc1234/design/x.md",
		headings:    withPermissions,
		permissions: []heading{{text: "## Permissions", line: 5}},
	})

	// permissions-subheading
	tests = append(tests, &testCase{
		_id:         "permissions-subheading",
		_violations: 1,

		file:        "plans/2026-10-02-x-abc1234/2026-10-02-x-wip-abc1234/design/x.md",
		headings:    catalog,
		permissions: []heading{{text: "#### Permissions", line: 10}},
	})

	// permissions-section-old-plan-dir
	tests = append(tests, &testCase{
		_id:         "permissions-section-old-plan-dir",
		_violations: 2,

		file:        "plans/2026-09-01-x-wip-abc1234/design/x.md",
		headings:    withPermissions,
		permissions: []heading{{text: "## Permissions", line: 5}},
	})

	// no-permissions-section
	tests = append(tests, &testCase{
		_id:         "no-permissions-section",
		_violations: 0,

		file:        "plans/2026-10-02-x-abc1234/2026-10-02-x-wip-abc1234/design/x.md",
		headings:    catalog,
		permissions: []heading{},
	})

	// phased-overlap
	tests = append(tests, &testCase{
		_id:         "phased-overlap",
		_violations: 1,

		file:        "plan.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges(table + "### A\nlocation: `a.go`\nphase: 1\n\n### B\nlocation: `a.go`\nphase: 1\n"),
	})

	// legacy-unit
	tests = append(tests, &testCase{
		_id:         "legacy-unit",
		_violations: 0,

		file:        "plan.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges(table + "### A\nlocation: `a.go`\n"),
	})

	// plan-without-changes
	tests = append(tests, &testCase{
		_id:         "plan-without-changes",
		_violations: 0,

		file:        "plans/2026-10-02-x-abc1234/2026-10-02-x-wip-abc1234/design/x.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges("## TLDR\n\nx\n\n## Changelog\n\n- created\n"),
	})

	// agent-marker-missing
	tests = append(tests, &testCase{
		_id:         "agent-marker-missing",
		_violations: 1,

		file:        "plan.md",
		headings:    []heading{{text: "TLDR", line: 1}, {text: "Current state", line: 3}, {text: "Changelog", line: 5}},
		permissions: []heading{},
	})

	// marker-on-human-section
	tests = append(tests, &testCase{
		_id:         "marker-on-human-section",
		_violations: 1,

		file:        "plan.md",
		headings:    []heading{{text: "TLDR", line: 1}, {text: "Scope [AGENT-ONLY]", line: 3}, {text: "Changelog", line: 5}},
		permissions: []heading{},
	})

	// marker-on-override
	tests = append(tests, &testCase{
		_id:         "marker-on-override",
		_violations: 0,

		file:        "plan.md",
		headings:    []heading{{text: "TLDR", line: 1}, {text: "Ruled out [AGENT-ONLY]", line: 3}, {text: "Changelog", line: 5}},
		permissions: []heading{},
	})

	// order-checked
	tests = append(tests, &testCase{
		_id:         "order-checked",
		_violations: 1,

		file:        "plans/2026-10-04-new-abc1234/2026-10-04-new-wip-abc1234/design/new.md",
		headings:    reordered,
		permissions: []heading{},
	})

	// order-exempt-before-cutoff
	tests = append(tests, &testCase{
		_id:         "order-exempt-before-cutoff",
		_violations: 0,

		file:        "plans/2026-10-03-new-abc1234/2026-10-03-new-wip-abc1234/design/new.md",
		headings:    reordered,
		permissions: []heading{},
	})

	// order-exempt-old-plan-dir
	tests = append(tests, &testCase{
		_id:         "order-exempt-old-plan-dir",
		_violations: 1,

		file:        "plans/2026-09-01-old-wip-abc1234/design/old.md",
		headings:    reordered,
		permissions: []heading{},
	})

	// entry-order-checked
	tests = append(tests, &testCase{
		_id:         "entry-order-checked",
		_violations: 1,

		file:        "plans/2026-10-04-new-abc1234/2026-10-04-new-wip-abc1234/design/new.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges(table + proseFirst),
	})

	// entry-order-exempt-before-cutoff
	tests = append(tests, &testCase{
		_id:         "entry-order-exempt-before-cutoff",
		_violations: 0,

		file:        "plans/2026-10-03-new-abc1234/2026-10-03-new-wip-abc1234/design/new.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges(table + proseFirst),
	})

	// entry-order-draft-path
	tests = append(tests, &testCase{
		_id:         "entry-order-draft-path",
		_violations: 1,

		file:        "plan.md",
		headings:    catalog,
		permissions: []heading{},
		unit:        plans.ParseChanges(table + proseFirst),
	})

	// marker-checked-old-plan-dir
	tests = append(tests, &testCase{
		_id:         "marker-checked-old-plan-dir",
		_violations: 2,

		file:        "plans/2026-09-30-old-wip-abc1234/design/old.md",
		headings:    []heading{{text: "TLDR", line: 1}, {text: "Current state", line: 3}, {text: "Changelog", line: 5}},
		permissions: []heading{},
	})

	// old-depth-plan-file
	tests = append(tests, &testCase{
		_id:         "old-depth-plan-file",
		_violations: 1,

		file:        "plans/2026-10-02-x-wip-abc1234/design/x.md",
		headings:    catalog,
		permissions: []heading{},
	})

	// old-depth-companion-file
	tests = append(tests, &testCase{
		_id:         "old-depth-companion-file",
		_violations: 1,

		file:        "plans/2026-10-02-x-wip-abc1234/design/notes.md",
		headings:    catalog,
		permissions: []heading{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			out := &bytes.Buffer{}
			assert.Equal(t, test._violations, checkFile(test.file, test.headings, test.permissions, test.unit, out), out.String())
		})
	}
}
