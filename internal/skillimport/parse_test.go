package skillimport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const entriesFixture = "---\nname: demo\nversion: 1.0\n---\n" +
	"# Demo\n\n" +
	"## Intake\n\n" +
	"**SKILL-DEMO-INTAKE-001** `[step]` **Feature name**\n\n" +
	"Ask for the feature name.\n\n" +
	"* Why: nothing else is known yet.\n" +
	"* Applies: no name in the prompt.\n\n" +
	"**SKILL-DEMO-INTAKE-002** `[prose]` **Short prose**\n\n" +
	"Keep prose short.\n" +
	"Wrapped continuation of the statement.\n\n" +
	"**SKILL-DEMO-INTAKE-002a** `[prose]` **Split sibling**\n\n" +
	"Letter-suffixed split entry.\n\n" +
	"## Templates\n\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` **Overview template**\n\n" +
	"Overview template.\n\n" +
	"````markdown\n# Concept\n```json\n{}\n```\n````\n\n" +
	"```markdown\n**SKILL-DEMO-INTAKE-099** `[step]` **Fenced**\n\nInside a fence, never parsed.\n```\n"

func TestParseEntries(t *testing.T) {
	entries, violations := ParseEntries(strings.Split(entriesFixture, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 4)

	first := entries[0]
	assert.Equal(t, "SKILL-DEMO-INTAKE-001", first.Id)
	assert.Equal(t, "DEMO", first.Skill)
	assert.Equal(t, "INTAKE", first.Topic)
	assert.Equal(t, 1, first.Number)
	assert.Empty(t, first.Suffix)
	assert.Equal(t, "step", first.Class)
	assert.Equal(t, "step", first.Role)
	assert.Equal(t, "prose", first.Channel)
	assert.Equal(t, "Feature name", first.Title)
	assert.Equal(t, "Ask for the feature name.", first.Statement)
	assert.Equal(t, "nothing else is known yet.", first.Why)
	assert.Equal(t, "no name in the prompt.", first.Applies)
	assert.False(t, first.Payload)
	assert.Equal(t, 8, first.Line)
	assert.Equal(t, 14, first.End)

	second := entries[1]
	assert.Equal(t, "SKILL-DEMO-INTAKE-002", second.Id)
	assert.Equal(t, "Keep prose short. Wrapped continuation of the statement.", second.Statement)

	split := entries[2]
	assert.Equal(t, "SKILL-DEMO-INTAKE-002a", split.Id)
	assert.Equal(t, 2, split.Number)
	assert.Equal(t, "a", split.Suffix)

	payload := entries[3]
	assert.Equal(t, "SKILL-DEMO-TPL-001", payload.Id)
	assert.True(t, payload.Payload)
	assert.Equal(t, "payload", payload.Role)
	assert.Empty(t, payload.Channel)
}

func TestParseEntriesSectionAndRefs(t *testing.T) {
	body := "## Dispatch\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
		"RESOLVE the base per [Guard](#skill-demo-proc-002) and [Guard](#skill-demo-proc-002).\n\n" +
		"* param: split case rides via [Split](#skill-demo-proc-003a).\n\n" +
		"**SKILL-DEMO-PROC-002** `[rule]` **Guard**\n\n" +
		"Premise pin MUST hold.\n\n" +
		"## Round\n\n" +
		"**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\n" +
		"Template with a fenced link that never counts:\n\n" +
		"```markdown\n[Fenced](#skill-demo-proc-001)\n```\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 3)

	first := entries[0]
	assert.Equal(t, "Dispatch", first.Section)
	assert.Equal(t, []string{"SKILL-DEMO-PROC-002", "SKILL-DEMO-PROC-003a"}, first.Refs)

	second := entries[1]
	assert.Equal(t, "Dispatch", second.Section)
	assert.Empty(t, second.Refs)

	payload := entries[2]
	assert.Equal(t, "Round", payload.Section)
	assert.Empty(t, payload.Refs)
}

func TestParseEntriesOntologyRoles(t *testing.T) {
	for _, role := range []string{"verify", "def", "rule", "substep", "spawn"} {
		body := "**SKILL-DEMO-EVAL-001** `[" + role + "]` **New role**\n\nOne point.\n"
		entries, violations := ParseEntries(strings.Split(body, "\n"))
		require.Empty(t, violations, role)
		require.Len(t, entries, 1, role)
		assert.Equal(t, role, entries[0].Role, role)
	}
}

func TestParseEntriesParamNotARole(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-001** `[param]` **Param as role**\n\nParam is a bullet register, never a role.\n"
	_, violations := ParseEntries(strings.Split(body, "\n"))
	require.NotEmpty(t, violations)
	assert.Contains(t, violations[0], `unknown tag "param"`)
}

func TestParseEntriesMechanismCustomTag(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-004** `[step][hook:read-gate]` **Gated write**\n\n" +
		"The read-gate denies the write until covered.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Equal(t, "step", entries[0].Role)
	// A mechanism tag carrying a colon is a custom tag, not the bare hook
	// channel — Channel stays the default and no Hook pointer is required.
	assert.Equal(t, "prose", entries[0].Channel)
	assert.Empty(t, entries[0].Hook)
}

func TestParseEntriesTwoTagSlot(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-001** `[step][schema]` **Emits JSON**\n\n" +
		"Emits schema-bound JSON.\n\n" +
		"* Schema: reference/schema.json\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Equal(t, "step", entries[0].Role)
	assert.Equal(t, "schema", entries[0].Channel)
	assert.Equal(t, "reference/schema.json", entries[0].Schema)
}

func TestParseEntriesHookChannel(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-004** `[step][hook]` **Gated write**\n\n" +
		"The read-gate denies the write until covered.\n\n" +
		"* Hook: cmd/hooks/read-gate.sh\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Equal(t, "step", entries[0].Role)
	assert.Equal(t, "hook", entries[0].Channel)
	assert.Equal(t, "cmd/hooks/read-gate.sh", entries[0].Hook)
}

func TestParseEntriesBulletContinuation(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-005** `[step]` **Wrapped bullet**\n\n" +
		"Statement.\n\n" +
		"* Applies: a bullet whose text wraps\n" +
		"  onto an indented continuation line.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Equal(t, "a bullet whose text wraps onto an indented continuation line.", entries[0].Applies)
}

func TestParseEntriesFormalClauses(t *testing.T) {
	body := "`{pred: not asks_user and (gate_pauses => interactive)}`\\\n" +
		"**SKILL-DEMO-EVAL-010** `[rule]` **Never prompts**\n\n" +
		"Review NEVER prompts.\n\n" +
		"`{pred: round_started}`\\\n" +
		"`{post: tools_loaded and round_ready}`\\\n" +
		"**SKILL-DEMO-EVAL-011** `[verify]` **Tools loaded**\n\n" +
		"Before round: VERIFY tools loaded; load them.\n\n" +
		"`{post: report_written}`\\\n" +
		"**SKILL-DEMO-EVAL-012** `[step]` **Report**\n\n" +
		"WRITE the report.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 3)
	assert.Equal(t, "not asks_user and (gate_pauses => interactive)", entries[0].Pred)
	assert.Empty(t, entries[0].Post)
	assert.Equal(t, "round_started", entries[1].Pred)
	assert.Equal(t, "tools_loaded and round_ready", entries[1].Post)
	assert.Empty(t, entries[2].Pred)
	assert.Equal(t, "report_written", entries[2].Post)
	// The formal line belongs to the entry's block.
	assert.Equal(t, 0, entries[0].Line)
}

func TestParseEntriesFormalClauseOrphan(t *testing.T) {
	body := "`{pred: base_red}`\n\n" +
		"**SKILL-DEMO-EVAL-012** `[stop]` **Base red**\n\n" +
		"If base red ⇒ STOP: report.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].Pred)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "pred clause line is not directly above an entry headline")
}

func TestParseEntriesLegacyClauseBullet(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-013** `[stop]` **Base red**\n\n" +
		"If base red ⇒ STOP: report.\n\n" +
		"* pred: base_red\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].Pred)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "clause bullet — formal clauses live on the")
}

func TestParseEntriesFormalClauseRoleViolation(t *testing.T) {
	body := "`{pred: round_started}`\\\n" +
		"**SKILL-DEMO-EVAL-014** `[def]` **Round**\n\n" +
		"A round is one review pass.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, entries, 1)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "formal clause on role \"def\"")
}

func TestParseEntriesSectionKind(t *testing.T) {
	body := "---\nname: demo\nformat: 2\n---\n" +
		"## Dispatch\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
		"RESOLVE the base.\n\n" +
		"## Nightly route\n" +
		"`{section: route, pred: mode = nightly, between: PROC-001..end}`\n\n" +
		"**SKILL-DEMO-NIGHT-001** `[step]` **Route it**\n\n" +
		"ROUTE the run.\n\n" +
		"## Reference\n" +
		"`{section: context}`\n\n" +
		"**SKILL-DEMO-REF-001** `[rule]` **Standing**\n\n" +
		"Runs MUST hold.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 3)
	assert.Empty(t, entries[0].SectionKind)
	assert.Equal(t, skillgraph.SectionKindRoute, entries[1].SectionKind)
	assert.Equal(t, "mode = nightly", entries[1].SectionPred)
	assert.Equal(t, []string{"SKILL-DEMO-PROC-001", "end"}, entries[1].SectionBetween)
	assert.Equal(t, skillgraph.SectionKindContext, entries[2].SectionKind)
	assert.Empty(t, entries[2].SectionPred)
}

// routeAnchorBody renders a fixture with two phases (Work: WORK-001, WORK-002;
// Done: DONE-001), a def in Work, and one route section carrying the clause.
func routeAnchorBody(clause string) string {
	return "---\nname: demo\nformat: 2\n---\n" +
		"## Work\n\n" +
		"**SKILL-DEMO-WORK-000** `[def]` **Term**\n\nA term.\n\n" +
		"**SKILL-DEMO-WORK-001** `[step]` **First**\n\nRUN one.\n\n" +
		"**SKILL-DEMO-WORK-002** `[step]` **Second**\n\nRUN two.\n\n" +
		"## Done\n\n" +
		"**SKILL-DEMO-DONE-001** `[step]` **Finish**\n\nFINISH it.\n\n" +
		"## Route\n" +
		"`{section: route, pred: fast" + clause + "}`\n\n" +
		"**SKILL-DEMO-FAST-001** `[step]` **Fast**\n\nRUN fast.\n"
}

func TestParseEntriesRouteAnchors(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedBetween   []string
		_expectedReplaces  []string
		_expectedViolation string

		clause string
	}

	tests := make([]*testCase, 0)

	// route-without-anchor
	tests = append(tests, &testCase{
		_id:                "route-without-anchor",
		_expectedBetween:   nil,
		_expectedReplaces:  nil,
		_expectedViolation: "route section without anchor",

		clause: "",
	})

	// route-with-both
	tests = append(tests, &testCase{
		_id:                "route-with-both",
		_expectedBetween:   []string{"SKILL-DEMO-WORK-002", "SKILL-DEMO-DONE-001"},
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-001"},
		_expectedViolation: "both replaces and between",

		clause: ", replaces: WORK-001, between: WORK-002..DONE-001",
	})

	// replaces-id
	tests = append(tests, &testCase{
		_id:                "replaces-id",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-001"},
		_expectedViolation: "",

		clause: ", replaces: WORK-001",
	})

	// replaces-range-plus-id
	tests = append(tests, &testCase{
		_id:                "replaces-range-plus-id",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-001..SKILL-DEMO-WORK-002", "SKILL-DEMO-DONE-001"},
		_expectedViolation: "",

		clause: ", replaces: WORK-001..WORK-002 + DONE-001",
	})

	// between-ids
	tests = append(tests, &testCase{
		_id:                "between-ids",
		_expectedBetween:   []string{"SKILL-DEMO-WORK-002", "SKILL-DEMO-DONE-001"},
		_expectedReplaces:  nil,
		_expectedViolation: "",

		clause: ", between: WORK-002..DONE-001",
	})

	// between-start-end
	tests = append(tests, &testCase{
		_id:                "between-start-end",
		_expectedBetween:   []string{"start", "end"},
		_expectedReplaces:  nil,
		_expectedViolation: "",

		clause: ", between: start..end",
	})

	// malformed-replaces-item
	tests = append(tests, &testCase{
		_id:                "malformed-replaces-item",
		_expectedBetween:   nil,
		_expectedReplaces:  nil,
		_expectedViolation: "malformed replaces item",

		clause: ", replaces: work-one",
	})

	// start-inside-replaces
	tests = append(tests, &testCase{
		_id:                "start-inside-replaces",
		_expectedBetween:   nil,
		_expectedReplaces:  nil,
		_expectedViolation: "malformed replaces item",

		clause: ", replaces: start..WORK-001",
	})

	// malformed-between
	tests = append(tests, &testCase{
		_id:                "malformed-between",
		_expectedBetween:   nil,
		_expectedReplaces:  nil,
		_expectedViolation: "malformed between",

		clause: ", between: WORK-001",
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:                "unknown-id",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-009"},
		_expectedViolation: "no such entry",

		clause: ", replaces: WORK-009",
	})

	// def-anchor
	tests = append(tests, &testCase{
		_id:                "def-anchor",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-000"},
		_expectedViolation: "a def is never an anchor",

		clause: ", replaces: WORK-000",
	})

	// anchor-inside-route
	tests = append(tests, &testCase{
		_id:                "anchor-inside-route",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-FAST-001"},
		_expectedViolation: "inside the route itself",

		clause: ", replaces: FAST-001",
	})

	// range-across-sections
	tests = append(tests, &testCase{
		_id:                "range-across-sections",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-WORK-001..SKILL-DEMO-DONE-001"},
		_expectedViolation: "",

		clause: ", replaces: WORK-001..DONE-001",
	})

	// range-out-of-order
	tests = append(tests, &testCase{
		_id:                "range-out-of-order",
		_expectedBetween:   nil,
		_expectedReplaces:  []string{"SKILL-DEMO-DONE-001..SKILL-DEMO-WORK-001"},
		_expectedViolation: "range out of order",

		clause: ", replaces: DONE-001..WORK-001",
	})

	// between-reentry-before-split
	tests = append(tests, &testCase{
		_id:                "between-reentry-before-split",
		_expectedBetween:   []string{"SKILL-DEMO-DONE-001", "SKILL-DEMO-WORK-001"},
		_expectedReplaces:  nil,
		_expectedViolation: "reentry at or before the split",

		clause: ", between: DONE-001..WORK-001",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			entries, violations := ParseEntries(strings.Split(routeAnchorBody(test.clause), "\n"))
			route := entries[len(entries)-1]
			assert.Equal(t, test._expectedBetween, route.SectionBetween)
			assert.Equal(t, test._expectedReplaces, route.SectionReplaces)
			assert.Equal(t, test._expectedViolation == "", len(violations) == 0, "violations = %v", violations)
			assert.True(t, test._expectedViolation == "" || strings.Contains(strings.Join(violations, "\n"), test._expectedViolation), "violations = %v", violations)
		})
	}
}

func TestParseEntriesSectionKindViolations(t *testing.T) {
	unknown := "## X\n`{section: chapter}`\n"
	_, violations := ParseEntries(strings.Split(unknown, "\n"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], `unknown section kind "chapter"`)

	predOnContext := "## X\n`{section: context, pred: nightly}`\n"
	_, violations = ParseEntries(strings.Split(predOnContext, "\n"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "only route sections carry a pred")

	anchorOnPhase := "## X\n`{section: phase, between: start..end}`\n"
	_, violations = ParseEntries(strings.Split(anchorOnPhase, "\n"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "only route sections carry one")

	orphan := "Prose.\n\n`{section: context}`\n"
	_, violations = ParseEntries(strings.Split(orphan, "\n"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "not directly under a section heading")
}

func TestParseEntriesClauseMissingHardBreak(t *testing.T) {
	body := "`{pred: round_started}`\n" +
		"**SKILL-DEMO-EVAL-021** `[step]` **No break**\n\n" +
		"RUN it.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, entries, 1)
	assert.Equal(t, "round_started", entries[0].Pred)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "trailing backslash hard break")
}

func TestParseEntriesCombinedClauseViolation(t *testing.T) {
	body := "`{pred: nightly}{post: done}`\n" +
		"**SKILL-DEMO-EVAL-020** `[step]` **Combined**\n\n" +
		"RUN it.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].Pred)
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "combined {pred}{post} line")
}

func TestParseEntriesSectionOrder(t *testing.T) {
	body := "---\nname: demo\nformat: 2\n---\n" +
		"## Dispatch\n\n" +
		"**SKILL-DEMO-PROC-001** `[step]` **Resolve base**\n\n" +
		"RESOLVE the base.\n\n" +
		"**SKILL-DEMO-TERM-001** `[def]` **Round**\n\n" +
		"A round is one pass.\n"
	_, violations := ParseEntries(strings.Split(body, "\n"))
	require.Len(t, violations, 1)
	assert.Contains(t, violations[0], "breaks the section order")

	// Format 1 is exempt.
	legacy := strings.Replace(body, "format: 2", "format: 1", 1)
	_, violations = ParseEntries(strings.Split(legacy, "\n"))
	assert.Empty(t, violations)
}

func TestParseEntriesQualityAndCustomTags(t *testing.T) {
	body := "**SKILL-DEMO-EVAL-002** `[quality][todo:unmeasured]` **Batches calls**\n\n" +
		"Batch independent tool calls into one message.\n"
	entries, violations := ParseEntries(strings.Split(body, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Equal(t, "quality", entries[0].Role)
	assert.Equal(t, "prose", entries[0].Channel)
	assert.Equal(t, []string{"todo:unmeasured"}, entries[0].Custom)
	assert.Equal(t, "quality+todo:unmeasured", entries[0].Class)

	prose := "**SKILL-DEMO-EVAL-003** `[prose][todo:schema-needed]` **Vocabulary**\n\n" +
		"Closed disposition set.\n"
	entries, violations = ParseEntries(strings.Split(prose, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 1)
	assert.Empty(t, entries[0].Role)
	assert.Equal(t, "prose", entries[0].Channel)
	assert.Equal(t, []string{"todo:schema-needed"}, entries[0].Custom)
}

func TestParseEntriesCustomTagViolations(t *testing.T) {
	type testCase struct {
		_id        string
		_violation string
		body       string
	}
	tests := []*testCase{
		{
			_id:        "empty-custom-value",
			_violation: "has no value after the colon",
			body:       "**SKILL-DEMO-EVAL-001** `[quality][todo:]` **Bad**\n\nBad.\n",
		},
		{
			_id:        "custom-before-role",
			_violation: "must come before any custom tag",
			body:       "**SKILL-DEMO-EVAL-001** `[todo:x][quality]` **Bad**\n\nBad.\n",
		},
		{
			_id:        "custom-before-channel",
			_violation: "must come before any custom tag",
			body:       "**SKILL-DEMO-EVAL-001** `[quality][todo:x][schema]` **Bad**\n\nBad.\n",
		},
	}
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, violations := ParseEntries(strings.Split(test.body, "\n"))
			require.Len(t, violations, 1)
			assert.Contains(t, violations[0], test._violation)
		})
	}
}

func TestParseEntriesFormatTwo(t *testing.T) {
	frontmatter := "---\nname: demo\nversion: 1.0\nformat: 2\n---\n"
	statement200 := strings.Repeat("x", 128) + "\n" + strings.Repeat("x", 71)
	statement201 := strings.Repeat("x", 128) + "\n" + strings.Repeat("x", 72)
	bullet160 := "* " + strings.Repeat("y", 126) + "\n  " + strings.Repeat("y", 33)
	bullet161 := "* " + strings.Repeat("y", 126) + "\n  " + strings.Repeat("y", 34)

	type testCase struct {
		_id         string
		_violations int
		_violation  string
		body        string
	}
	tests := []*testCase{
		{
			_id:         "statement-at-200-clean",
			_violations: 0,
			body:        frontmatter + "**SKILL-DEMO-EVAL-001** `[step]` **Cap edge**\n\n" + statement200 + "\n",
		},
		{
			_id:         "statement-201-flagged",
			_violations: 1,
			_violation:  "exceeds 200 runes",
			body:        frontmatter + "**SKILL-DEMO-EVAL-001** `[step]` **Over cap**\n\n" + statement201 + "\n",
		},
		{
			_id:         "bullet-at-160-clean",
			_violations: 0,
			body:        frontmatter + "**SKILL-DEMO-EVAL-001** `[step]` **Bullet edge**\n\nStatement.\n\n" + bullet160 + "\n",
		},
		{
			_id:         "bullet-161-flagged",
			_violations: 1,
			_violation:  "exceeds 160 runes",
			body:        frontmatter + "**SKILL-DEMO-EVAL-001** `[step]` **Over bullet**\n\nStatement.\n\n" + bullet161 + "\n",
		},
		{
			_id:         "why-flagged",
			_violations: 1,
			_violation:  "Why bullet",
			body:        frontmatter + "**SKILL-DEMO-EVAL-001** `[step]` **Why left**\n\nStatement.\n\n* Why: rationale.\n",
		},
	}
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, violations := ParseEntries(strings.Split(test.body, "\n"))
			require.Len(t, violations, test._violations)
			if test._violation != "" {
				assert.Contains(t, violations[0], test._violation)
			}
		})
	}
}

func TestParseEntriesViolations(t *testing.T) {
	statement320 := strings.Repeat("x", 128) + "\n" + strings.Repeat("x", 128) + "\n" + strings.Repeat("x", 64)

	type testCase struct {
		_id        string
		_violation string
		body       string
	}
	tests := make([]*testCase, 0)

	// malformed-headline
	tests = append(tests, &testCase{
		_id:        "malformed-headline",
		_violation: "malformed entry line",
		body:       "**SKILL-DEMO-X-001** `[step]` **Short topic**\n\nTopic too short.\n",
	})

	// unknown-tag
	tests = append(tests, &testCase{
		_id:        "unknown-tag",
		_violation: `unknown tag "shout"`,
		body:       "**SKILL-DEMO-INTAKE-001** `[shout]` **Loud one**\n\nLoud.\n",
	})

	// payload-without-fence
	tests = append(tests, &testCase{
		_id:        "payload-without-fence",
		_violation: "not followed by a fenced block",
		body:       "**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\nTemplate.\n\nProse after a blank line is not a fence.\n",
	})

	// text-after-title (legacy dash / inline statement on the headline)
	tests = append(tests, &testCase{
		_id:        "text-after-title",
		_violation: "text after the title",
		body:       "**SKILL-DEMO-INTAKE-001** `[step]` **Old form** — Statement crammed on the headline.\n",
	})

	// missing-blank (no blank line between headline and statement)
	tests = append(tests, &testCase{
		_id:        "missing-blank",
		_violation: "a blank line separates the headline",
		body:       "**SKILL-DEMO-INTAKE-001** `[step]` **No blank**\nStatement on the very next line.\n",
	})

	// line-over-128
	tests = append(tests, &testCase{
		_id:        "line-over-128",
		_violation: "exceeds 128 chars",
		body:       "**SKILL-DEMO-INTAKE-001** `[step]` **Wide line**\n\n" + strings.Repeat("x", 129) + "\n",
	})

	// title-too-long
	tests = append(tests, &testCase{
		_id:        "title-too-long",
		_violation: "exceeds 3 words",
		body:       "**SKILL-DEMO-INTAKE-001** `[step]` **A very long title**\n\nStatement.\n",
	})

	// statement-over-cap
	tests = append(tests, &testCase{
		_id:        "statement-over-cap",
		_violation: "split into SKILL-DEMO-INTAKE-001a/SKILL-DEMO-INTAKE-001b",
		body:       "**SKILL-DEMO-INTAKE-001** `[step]` **Long statement**\n\n" + statement320 + "\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, violations := ParseEntries(strings.Split(test.body, "\n"))
			require.Len(t, violations, 1)
			assert.Contains(t, violations[0], test._violation)
		})
	}
}
