package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const entriesFixture = "---\nname: demo\nversion: 1.0\n---\n" +
	"# Demo\n\n" +
	"## Intake\n\n" +
	"**SKILL-DEMO-INTAKE-001** `[step]` — Ask for the feature name.\n\n" +
	"* Why: nothing else is known yet.\n" +
	"* Applies: no name in the prompt.\n\n" +
	"**SKILL-DEMO-INTAKE-002** `[review]` — Keep prose short.\n" +
	"Prose that ends the block.\n\n" +
	"## Templates\n\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` — Overview template.\n\n" +
	"````markdown\n# Concept\n```json\n{}\n```\n````\n\n" +
	"```markdown\n**SKILL-DEMO-INTAKE-099** `[step]` — Inside a fence, never parsed.\n```\n"

func TestParseEntries(t *testing.T) {
	entries, violations := ParseEntries(strings.Split(entriesFixture, "\n"))
	require.Empty(t, violations)
	require.Len(t, entries, 3)

	first := entries[0]
	assert.Equal(t, "SKILL-DEMO-INTAKE-001", first.Id)
	assert.Equal(t, "DEMO", first.Skill)
	assert.Equal(t, "INTAKE", first.Topic)
	assert.Equal(t, 1, first.Number)
	assert.Equal(t, "step", first.Class)
	assert.Equal(t, "Ask for the feature name.", first.Statement)
	assert.Equal(t, "nothing else is known yet.", first.Why)
	assert.Equal(t, "no name in the prompt.", first.Applies)
	assert.False(t, first.Payload)
	assert.Equal(t, 8, first.Line)
	assert.Equal(t, 12, first.End)

	second := entries[1]
	assert.Equal(t, "SKILL-DEMO-INTAKE-002", second.Id)
	assert.Equal(t, second.Line+1, second.End)

	payload := entries[2]
	assert.Equal(t, "SKILL-DEMO-TPL-001", payload.Id)
	assert.True(t, payload.Payload)
	assert.Equal(t, 26, payload.End)
}

func TestParseEntriesViolations(t *testing.T) {
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
		body:       "**SKILL-DEMO-X-001** `[step]` — Topic too short.\n",
	})

	// unknown-class
	tests = append(tests, &testCase{
		_id:        "unknown-class",
		_violation: `unknown class "shout"`,
		body:       "**SKILL-DEMO-INTAKE-001** `[shout]` — Loud.\n",
	})

	// payload-without-fence
	tests = append(tests, &testCase{
		_id:        "payload-without-fence",
		_violation: "not followed by a fenced block",
		body:       "**SKILL-DEMO-TPL-001** `[payload]` — Template.\n\nProse instead of a fence.\n",
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

func TestResolveDisable(t *testing.T) {
	entries, _ := ParseEntries(strings.Split(entriesFixture, "\n"))

	exact, err := ResolveDisable(entries, []string{"SKILL-DEMO-INTAKE-002"})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"SKILL-DEMO-INTAKE-002": true}, exact)

	glob, err := ResolveDisable(entries, []string{" SKILL-DEMO-INTAKE-* ", ""})
	require.NoError(t, err)
	assert.Len(t, glob, 2)

	_, err = ResolveDisable(entries, []string{"SKILL-DEMO-NOPE-*"})
	assert.Error(t, err)
}

func TestSkillIdName(t *testing.T) {
	assert.Equal(t, "PACKAGECOMMIT", SkillIdName("package-commit"))
	assert.Equal(t, "CONCEPT", SkillIdName("concept"))
}
