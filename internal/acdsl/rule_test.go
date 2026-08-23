package acdsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marker builds a marker line at runtime so this test file's own source
// never carries a line-leading marker the real tree scan would pick up.
func marker(rest string) string {
	return "//" + "acdsl:" + rest
}

func TestParseRulesValidMarkerWithParams(t *testing.T) {
	lines := map[string][]string{
		"a.go": {
			"package a",
			marker(`X-001 forbid-call anchor="\.go$" anchor-not="_test\.go$" pkg="os/exec" func="Command" why="no raw exec"`),
		},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	rule := rules[0]
	assert.Equal(t, "X-001", rule.Id)
	assert.Equal(t, "forbid-call", rule.Verifier)
	assert.Equal(t, `\.go$`, rule.Anchor)
	assert.Equal(t, `_test\.go$`, rule.AnchorNot)
	assert.Equal(t, "no raw exec", rule.Why)
	assert.Equal(t, map[string]string{"pkg": "os/exec", "func": "Command"}, rule.Params)
	assert.Equal(t, "a.go", rule.File)
	assert.Equal(t, 2, rule.Line)
}

func TestParseRulesEmptyAttribute(t *testing.T) {
	// pass-empty
	lines := map[string][]string{
		"a.go": {marker(`X-001 fake anchor="^plans/" empty="ok" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.True(t, rules[0].AllowEmpty)
	assert.NotContains(t, rules[0].Params, "empty")

	// fail-bad-value
	lines = map[string][]string{
		"a.go": {marker(`X-002 fake anchor="^plans/" empty="yes" why="w"`)},
	}
	_, violations = ParseRules(lines)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], `empty must be "ok" when given`)
}

func TestParseRulesNeedsAttribute(t *testing.T) {
	// pass-two-paths
	lines := map[string][]string{
		"a.go": {marker(`X-001 fake anchor="^skills/" needs="context/context.json, proposals/schema.json" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.Equal(t, []string{"context/context.json", "proposals/schema.json"}, rules[0].Needs)
	assert.NotContains(t, rules[0].Params, "needs")

	// fail-empty-value
	lines = map[string][]string{
		"a.go": {marker(`X-002 fake anchor="^skills/" needs="" why="w"`)},
	}
	_, violations = ParseRules(lines)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "needs must name at least one path when given")
}

func TestParseRulesMissingAnchorIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call why="w"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "anchor is required")
}

func TestParseRulesMissingWhyIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "why is required")
}

func TestParseRulesShortMarkerIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "marker needs")
}

func TestParseRulesDuplicateIdAcrossFilesIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" why="w"`)},
		"b.go": {marker(`X-001 forbid-call anchor="x" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Len(t, rules, 1)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "duplicate rule id X-001")
	assert.Contains(t, violations[0], "a.go:1")
}

func TestDeclarationMarkerPerSyntax(t *testing.T) {
	cases := map[string][2]string{
		"rules.acdsl": {"//" + "acdsl:", ""},
		"a.go":        {"//" + "acdsl:", ""},
		"run.sh":      {"#" + "acdsl:", ""},
		"job.py":      {"#" + "acdsl:", ""},
		"Makefile":    {"#" + "acdsl:", ""},
		"query.sql":   {"--" + "acdsl:", ""},
		"doc.md":      {"<!--" + "acdsl:", "-->"},
		"page.html":   {"<!--" + "acdsl:", "-->"},
	}
	for path, want := range cases {
		prefix, terminator, ok := declarationMarker(path)
		require.Truef(t, ok, "path %s", path)
		assert.Equalf(t, want[0], prefix, "path %s", path)
		assert.Equalf(t, want[1], terminator, "path %s", path)
	}

	_, _, ok := declarationMarker("data.json")
	assert.False(t, ok)
}

func TestParseRulesHashAndSqlMarkers(t *testing.T) {
	lines := map[string][]string{
		"run.sh":    {"#!/bin/sh", "#" + `acdsl:X-001 forbid-call anchor="\.sh$" why="shell rule"`},
		"query.sql": {"--" + `acdsl:X-002 forbid-call anchor="\.sql$" why="sql rule"`},
		"Makefile":  {"#" + `acdsl:X-003 forbid-call anchor="^Makefile$" why="make rule"`},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 3)
	assert.Equal(t, "X-003", rules[0].Id)
	assert.Equal(t, `\.sql$`, rules[1].Anchor)
	assert.Equal(t, "shell rule", rules[2].Why)
}

func TestParseRulesMarkdownBlockMarker(t *testing.T) {
	lines := map[string][]string{
		"doc.md": {
			"# Title",
			"<!--" + `acdsl:X-001 forbid-call anchor="\.md$" why="md rule"` + " -->",
			"<!--" + `acdsl:X-002 forbid-call anchor="\.md$" why="glued terminator"` + "-->",
		},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 2)
	assert.Equal(t, "md rule", rules[0].Why)
	assert.Equal(t, "glued terminator", rules[1].Why)
}

func TestParseRulesMarkdownIgnoresLineCommentForm(t *testing.T) {
	// A fenced //acdsl: example in a doc is quoted text, never a declaration —
	// markdown declares only through its own block-comment form.
	lines := map[string][]string{
		"doc.md": {"```go", marker(`X-001 forbid-call anchor="x" why="w"`), "```"},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	assert.Empty(t, violations)
}

func TestParseRulesUnknownExtensionIgnored(t *testing.T) {
	lines := map[string][]string{
		"data.json": {marker(`X-001 forbid-call anchor="x" why="w"`)},
		"notes.txt": {"#" + `acdsl:X-002 forbid-call anchor="x" why="w"`},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	assert.Empty(t, violations)
}

func TestParseRulesDuplicateIdAcrossSyntaxesIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go":   {marker(`X-001 forbid-call anchor="x" why="w"`)},
		"run.sh": {"#" + `acdsl:X-001 forbid-call anchor="x" why="w"`},
	}
	rules, violations := ParseRules(lines)
	assert.Len(t, rules, 1)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "duplicate rule id X-001")
}

func TestParseRulesIgnoresOrdinaryComments(t *testing.T) {
	lines := map[string][]string{
		"a.go": {
			"package a",
			"// a normal comment",
			"var x = 1 // trailing comment",
		},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	assert.Empty(t, violations)
}

func TestUnquoteFieldKeepsRegexpEscapes(t *testing.T) {
	assert.Equal(t, `\.go$`, unquoteField(`"\.go$"`))
	assert.Equal(t, `a"b`, unquoteField(`"a\"b"`))
	assert.Equal(t, `a\b`, unquoteField(`"a\\b"`))
}

func TestParseRulesProjectedFalse(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" projected="false" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.False(t, rules[0].Projected)
	assert.NotContains(t, rules[0].Params, "projected")
}

func TestParseRulesProjectedDefaultsTrue(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.True(t, rules[0].Projected)
}

func TestParseRulesProjectedInvalidIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" projected="maybe" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], `projected must be "true" or "false"`)
}

func TestParseRulesLifetimeDefaultsToDoctrine(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.Equal(t, LifetimeDoctrine, rules[0].Lifetime)
}

func TestParseRulesTaskLifetimeAnywhere(t *testing.T) {
	// Lifetime is plain data — a task entry is legal in any declaration file.
	lines := map[string][]string{
		"tasks/contract.acdsl": {marker(`TASK-X-001 symbol-exists anchor="x" lifetime="task" symbol="Ping" why="w"`)},
		"a.go":                 {marker(`TASK-X-002 symbol-exists anchor="x" lifetime="task" symbol="Pong" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 2)
	assert.Equal(t, LifetimeTask, rules[0].Lifetime)
	assert.Equal(t, LifetimeTask, rules[1].Lifetime)
	assert.Equal(t, map[string]string{"symbol": "Pong"}, rules[0].Params)
}

func TestParseRulesInvalidLifetimeIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" lifetime="forever" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "lifetime must be doctrine or task")
}

func TestParseRulesReachDefaultsToThisRepo(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.Equal(t, "smine", rules[0].Reach)
}

func TestParseRulesReachGlobalAndListParsed(t *testing.T) {
	lines := map[string][]string{
		"a.go": {
			marker(`X-001 forbid-call anchor="x" reach="global" why="w"`),
			marker(`X-002 forbid-call anchor="x" reach="aqms,peek-mcp" why="w"`),
		},
	}
	rules, violations := ParseRules(lines)
	require.Empty(t, violations)
	require.Len(t, rules, 2)
	assert.Equal(t, "global", rules[0].Reach)
	assert.Equal(t, "aqms,peek-mcp", rules[1].Reach)
	// reach is a first-class field, never verifier argv
	assert.Empty(t, rules[0].Params)
	assert.Empty(t, rules[1].Params)
}

func TestParseRulesInvalidReachIsViolation(t *testing.T) {
	lines := map[string][]string{
		"a.go": {marker(`X-001 forbid-call anchor="x" reach="bogus name" why="w"`)},
	}
	rules, violations := ParseRules(lines)
	assert.Empty(t, rules)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "reach must be global, none, or a comma-separated repo-name list")
}

func TestFilterIdSelectsOneRule(t *testing.T) {
	rules := []Rule{
		{Id: "D-001", Lifetime: LifetimeDoctrine},
		{Id: "TASK-X-001", Lifetime: LifetimeTask},
	}

	one, err := FilterId(rules, "TASK-X-001")
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "TASK-X-001", one[0].Id)

	_, err = FilterId(rules, "NOPE-999")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rule with id")
}

func TestFilterLifetimeSelectors(t *testing.T) {
	rules := []Rule{
		{Id: "D-001", Lifetime: LifetimeDoctrine},
		{Id: "TASK-X-001", Lifetime: LifetimeTask},
	}

	doctrine, err := FilterLifetime(rules, LifetimeDoctrine)
	require.NoError(t, err)
	require.Len(t, doctrine, 1)
	assert.Equal(t, "D-001", doctrine[0].Id)

	task, err := FilterLifetime(rules, LifetimeTask)
	require.NoError(t, err)
	require.Len(t, task, 1)
	assert.Equal(t, "TASK-X-001", task[0].Id)

	all, err := FilterLifetime(rules, "all")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	_, err = FilterLifetime(rules, "sometimes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown selector")
}
