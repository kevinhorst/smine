package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const formatFixtureArgsTable = `---
name: x
---
# x

## Args

| Arg | Form | Default | Effect |
| :-- | :--- | :------ | :----- |
| ` + "`alpha`" + ` | number | ` + "`1`" + ` | first arg — TOPIC-001 |
| ` + "`beta`" + ` | ` + "`a\\|b`" + ` | ` + "`a`" + ` | second arg |

## Next
`

func TestParseArgsSection(t *testing.T) {
	spec := ParseArgsSection(strings.Split(formatFixtureArgsTable, "\n"))
	assert.True(t, spec.Exists)
	assert.True(t, spec.Table)
	assert.Len(t, spec.Rows, 2)
	assert.Equal(t, "alpha", spec.Rows[0].Name)
	assert.Equal(t, "beta", spec.Rows[1].Name)
	assert.Len(t, spec.Rows[1].Cells, 4)
	assert.Equal(t, ``+"`a\\|b`"+``, spec.Rows[1].Cells[1], "escaped pipe survives cell split")

	bullets := "# x\n\n## Args\n\n- `one`: doc.\n- two: doc.\n"
	spec = ParseArgsSection(strings.Split(bullets, "\n"))
	assert.True(t, spec.Exists)
	assert.False(t, spec.Table)
	assert.Len(t, spec.Rows, 2)
	assert.Equal(t, "one", spec.Rows[0].Name)
	assert.Equal(t, "two", spec.Rows[1].Name)

	none := ParseArgsSection(strings.Split("# x\n\nbody\n", "\n"))
	assert.False(t, none.Exists)
}

func TestCheckArgsTable(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		src string
	}

	tests := make([]*testCase, 0)

	// table-conformant
	tests = append(tests, &testCase{
		_id:       "table-conformant",
		_expected: 0,

		src: formatFixtureArgsTable,
	})

	// bullets-over-fallback
	tests = append(tests, &testCase{
		_id:       "bullets-over-fallback",
		_expected: 1,

		src: "# x\n\n## Args\n\n- a: doc.\n- b: doc.\n- c: doc.\n",
	})

	// bullets-within-fallback
	tests = append(tests, &testCase{
		_id:       "bullets-within-fallback",
		_expected: 0,

		src: "# x\n\n## Args\n\n- a: doc.\n- b: doc.\n",
	})

	// unbackticked-arg-and-long-effect
	tests = append(tests, &testCase{
		_id:       "unbackticked-arg-and-long-effect",
		_expected: 2,

		src: "# x\n\n## Args\n\n| Arg | Form | Default | Effect |\n| :-- | :-- | :-- | :-- |\n| alpha | n | 1 | " + strings.Repeat("long ", 40) + " |\n",
	})

	// no-args-section
	tests = append(tests, &testCase{
		_id:       "no-args-section",
		_expected: 0,

		src: "# x\n\nbody\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := CheckArgsTable(ParseArgsSection(strings.Split(test.src, "\n")))
			assert.Len(t, violations, test._expected)
		})
	}
}

func TestCheckArgumentHint(t *testing.T) {
	spec := ParseArgsSection(strings.Split(formatFixtureArgsTable, "\n"))

	assert.Empty(t, CheckArgumentHint("x", "alpha: 1 beta: a", 6, spec))
	assert.Len(t, CheckArgumentHint("x", "", 0, spec), 1, "missing hint on arg-bearing skill")
	assert.Len(t, CheckArgumentHint("x", "/x alpha: 1 beta: a", 6, spec), 1, "leading /x command duplicates the skill name")
	assert.Len(t, CheckArgumentHint("x", "alpha: 1", 6, spec), 1, "beta not set")
	assert.Empty(t, CheckArgumentHint("x", "", 0, ArgsSpec{}), "no args, no hint required")
}

func TestCheckEnumerations(t *testing.T) {
	clean := "---\nallowed-tools: a, b, c, d, e, f, g, h\n---\n# x\n\nshort `a, b, c` span.\n\n```\na, b, c, d, e, f, g, h as fence data\n```\n"
	assert.Empty(t, CheckEnumerations(strings.Split(clean, "\n")))

	dirty := "# x\n\nkeys `a, b, c, d, e, f, g` inline.\n"
	violations := CheckEnumerations(strings.Split(dirty, "\n"))
	assert.Len(t, violations, 1)
	assert.Equal(t, 3, violations[0].Line)
}

func TestCheckContextIds(t *testing.T) {
	clean := "---\nacdsl-context: RULE-PLAN-*\n---\n# x\n\nsee [the rule]($AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md) and MODES-002.\n\n```\nRULE-PLAN-038 as data\n```\n"
	assert.Empty(t, CheckContextIds(strings.Split(clean, "\n")))

	dirty := "---\nname: x\n---\n# x\n\nper RULE-PLAN-038 this fails.\n"
	violations := CheckContextIds(strings.Split(dirty, "\n"))
	assert.Len(t, violations, 1)
	assert.Equal(t, 6, violations[0].Line)
}

func TestIsGrantablePathRule(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		item string
	}

	tests := make([]*testCase, 0)

	// home-recursive-ok
	tests = append(tests, &testCase{
		_id:       "home-recursive-ok",
		_expected: true,

		item: "Read(~/GolandProjects/agent-plans/**)",
	})

	// abs-recursive-ok
	tests = append(tests, &testCase{
		_id:       "abs-recursive-ok",
		_expected: true,

		item: "Edit(//opt/data/**)",
	})

	// repos-ok
	tests = append(tests, &testCase{
		_id:       "repos-ok",
		_expected: true,

		item: "Read(@repos/**)",
	})

	// repos-named-flagged
	tests = append(tests, &testCase{
		_id:       "repos-named-flagged",
		_expected: false,

		item: "Read(@repos/peek-mcp/**)",
	})

	// relative-flagged
	tests = append(tests, &testCase{
		_id:       "relative-flagged",
		_expected: false,

		item: "Read(docs/**)",
	})

	// non-recursive-flagged
	tests = append(tests, &testCase{
		_id:       "non-recursive-flagged",
		_expected: false,

		item: "Write(~/GolandProjects/agent-plans)",
	})

	// skill-variable-root
	tests = append(tests, &testCase{
		_id:       "skill-variable-root",
		_expected: true,

		item: "Read($SKILL_PLANARCHIVE_ARCHIVE/**)",
	})

	// skill-variable-non-recursive-flagged
	tests = append(tests, &testCase{
		_id:       "skill-variable-non-recursive-flagged",
		_expected: false,

		item: "Edit($SKILL_PLANARCHIVE_ARCHIVE)",
	})

	// bash-unaffected
	tests = append(tests, &testCase{
		_id:       "bash-unaffected",
		_expected: true,

		item: "Bash(git status *)",
	})

	// plain-tool-unaffected
	tests = append(tests, &testCase{
		_id:       "plain-tool-unaffected",
		_expected: true,

		item: "Read",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsGrantablePathRule(test.item))
		})
	}
}

func TestToolsSorted(t *testing.T) {
	assert.True(t, ToolsSorted("Edit, Read, Write, Bash(git diff*), Bash(git log*), Bash(jq *)"))
	assert.False(t, ToolsSorted("Read, Edit"), "plain tools out of order")
	assert.False(t, ToolsSorted("Bash(jq *), Read"), "plain tools come first")
	assert.False(t, ToolsSorted("Bash(make build*), Bash(git diff*)"), "groups out of order")
	assert.True(t, ToolsSorted(""))
}

func TestFormatInvocationLines(t *testing.T) {
	hint := `scheduler rewrite directions: tests security lanes: 2[opus:medium] focus: "goroutine lifecycle" expect-base-red nightly run-id: eval-a1`
	names := []string{"directions", "lanes", "focus", "expect-base-red", "nightly", "run-id"}
	expected := []string{
		"scheduler rewrite",
		"directions: tests security",
		"lanes: 2[opus:medium]",
		`focus: "goroutine lifecycle"`,
		"expect-base-red",
		"nightly",
		"run-id: eval-a1",
	}
	assert.Equal(t, expected, FormatInvocationLines(hint, names))
	assert.Nil(t, FormatInvocationLines("", nil))
}
