package contextdocs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideFilterAspects() []RuleAspect {
	return []RuleAspect{
		{Name: "GOLANG", Scope: "Go", Class: "scope", Lang: "go"},
		{Name: "PYTHON", Scope: "Python", Class: "scope", Lang: "python"},
		{Name: "IMPL", Scope: "Implementing", Class: "scope"},
		{Name: "INTEG", Scope: "Integrity", Class: "topic"},
	}
}

func TestFilterRulesFileDropsSmineReachEntriesWithBullets(t *testing.T) {
	content := "# Chapter\n\n" +
		"**ACTION-IMPL-001** `[prose]` — Keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[prose]` — Drop.\n\n* Applies: always.\n* Reach: smine\n\n" +
		"**ACTION-IMPL-003** `[prose]` — Keep too.\n\n* Applies: always.\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), nil)
	assert.Contains(t, out, "ACTION-IMPL-001")
	assert.NotContains(t, out, "ACTION-IMPL-002")
	assert.NotContains(t, out, "Reach: smine")
	assert.Contains(t, out, "ACTION-IMPL-003")
}

func TestFilterRulesFileListReachCoversOnlyNamedTargets(t *testing.T) {
	content := "**ACTION-IMPL-001** `[prose]` — Listed.\n\n* Applies: always.\n* Reach: work, peek-mcp\n"
	kept := FilterRulesFile(content, "work", provideFilterAspects(), nil)
	assert.Contains(t, kept, "ACTION-IMPL-001")
	dropped := FilterRulesFile(content, "otherrepo", provideFilterAspects(), nil)
	assert.NotContains(t, dropped, "ACTION-IMPL-001")
}

func TestFilterRulesFileLangBoundScopes(t *testing.T) {
	content := "**RULE-GOLANG-INTEG-001** `[prose]` — Go rule.\n\n* Applies: go files.\n\n" +
		"**RULE-PYTHON-INTEG-001** `[prose]` — Python rule.\n\n* Applies: py files.\n\n" +
		"**ACTION-IMPL-001** `[prose]` — Lang-free.\n\n* Applies: always.\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), []string{"go"})
	assert.Contains(t, out, "RULE-GOLANG-INTEG-001")
	assert.NotContains(t, out, "RULE-PYTHON-INTEG-001")
	assert.Contains(t, out, "ACTION-IMPL-001")
}

func TestFilterRulesFileKeepsMarkedEntriesVerbatim(t *testing.T) {
	content := "**ACTION-IMPL-001** `[prose]` `[DoD]` — Marked keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[prose]` `[DoD]` — Marked drop.\n\n* Applies: always.\n* Reach: smine\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), nil)
	assert.Contains(t, out, "**ACTION-IMPL-001** `[prose]` `[DoD]` — Marked keep.")
	assert.NotContains(t, out, "ACTION-IMPL-002")
}

func TestFilterRulesFilePassesNonEntryContentVerbatim(t *testing.T) {
	content := "# Heading\n\nProse paragraph.\n\n```markdown\n**ACTION-IMPL-099** `[prose]` — Example in fence.\n\n* Reach: smine\n```\n\n" +
		"## Tombstones\n\n| Retired | Replacement | Date |\n| ACTION-IMPL-009 | — | 2026-01-01 |\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), nil)
	assert.Equal(t, content, out)
}

func TestFilterRulesFileDropsContinuationOfDroppedEntry(t *testing.T) {
	content := "**ACTION-IMPL-001** `[prose]` — Keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[prose]` — Drop.\n\n* Applies: always.\n* Reach: smine\n\n" +
		"Continuation prose of the dropped entry.\n\n```go\nexample()\n```\n\n* A late bullet of the dropped entry.\n\n" +
		"**ACTION-IMPL-003** `[prose]` — Keep too.\n\n* Applies: always.\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), nil)
	assert.NotContains(t, out, "Continuation prose")
	assert.NotContains(t, out, "example()")
	assert.NotContains(t, out, "late bullet")
	assert.Contains(t, out, "ACTION-IMPL-003")
}

func TestFilterRulesFileOutputReparsesGreen(t *testing.T) {
	content := "**ACTION-IMPL-001** `[prose]` — Keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[prose]` — Drop.\n\n* Applies: always.\n* Reach: smine\n\n" +
		"**RULE-PYTHON-INTEG-001** `[prose]` — Drop lang.\n\n* Applies: py.\n"
	out := FilterRulesFile(content, "work", provideFilterAspects(), []string{"go"})
	dir := writeRulesFixture(t, map[string]string{"chapter.md": out})
	set, err := ParseRulesDir(dir, false)
	require.NoError(t, err)
	require.Len(t, set.Entries, 1)
	assert.Equal(t, "ACTION-IMPL-001", set.Entries[0].Id)
	assert.False(t, strings.Contains(out, "\n\n\n"), "no double blank lines in filtered output")
}

func TestIsReachable(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		reachValue string
		self       string
	}

	tests := make([]*testCase, 0)

	// own-name
	tests = append(tests, &testCase{
		_id:       "own-name",
		_expected: true,

		reachValue: "smine",
		self:       "smine",
	})

	// list-naming-self-with-spaces
	tests = append(tests, &testCase{
		_id:       "list-naming-self-with-spaces",
		_expected: true,

		reachValue: "backend, smine",
		self:       "smine",
	})

	// global
	tests = append(tests, &testCase{
		_id:       "global",
		_expected: true,

		reachValue: "global",
		self:       "smine",
	})

	// foreign-name
	tests = append(tests, &testCase{
		_id:       "foreign-name",
		_expected: false,

		reachValue: "backend",
		self:       "smine",
	})

	// none
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: false,

		reachValue: "none",
		self:       "smine",
	})

	// always
	tests = append(tests, &testCase{
		_id:       "always",
		_expected: false,

		reachValue: "always",
		self:       "smine",
	})

	// no-repo-named-keeps-foreign
	tests = append(tests, &testCase{
		_id:       "no-repo-named-keeps-foreign",
		_expected: true,

		reachValue: "backend",
		self:       "",
	})

	// no-repo-named-keeps-none
	tests = append(tests, &testCase{
		_id:       "no-repo-named-keeps-none",
		_expected: true,

		reachValue: "none",
		self:       "",
	})

	// entry-without-reach
	tests = append(tests, &testCase{
		_id:       "entry-without-reach",
		_expected: true,

		reachValue: "",
		self:       "smine",
	})

	// blank-reach
	tests = append(tests, &testCase{
		_id:       "blank-reach",
		_expected: true,

		reachValue: "  ",
		self:       "smine",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsReachable(test.reachValue, test.self))
		})
	}
}
