package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const renderFixture = "---\nname: demo\nversion: 1.0\n---\n" +
	"# Demo\n\n" +
	"Intro prose stays.\n\n" +
	"## Intake\n\n" +
	"**SKILL-DEMO-INTAKE-001** `[step]` — First.\n\n" +
	"* Why: reason.\n\n" +
	"**SKILL-DEMO-INTAKE-002** `[step]` — Second.\n\n" +
	"## Writing\n\n" +
	"### Style\n\n" +
	"**SKILL-DEMO-WRITING-001** `[review]` — Short prose.\n\n" +
	"### Kept\n\n" +
	"Prose that stays.\n\n" +
	"## Templates\n\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` — Template.\n\n" +
	"```markdown\n# T\n```\n\n" +
	"## Model\n\n- Suggested: any\n"

func TestRenderSkillNoTokensIsIdentity(t *testing.T) {
	out, err := RenderSkill(renderFixture, nil)
	require.NoError(t, err)
	assert.Equal(t, renderFixture, out)
}

func TestRenderSkillDropsEntryAndKeepsNeighbours(t *testing.T) {
	out, err := RenderSkill(renderFixture, []string{"SKILL-DEMO-INTAKE-001"})
	require.NoError(t, err)
	assert.NotContains(t, out, "SKILL-DEMO-INTAKE-001")
	assert.NotContains(t, out, "* Why: reason.")
	assert.Contains(t, out, "**SKILL-DEMO-INTAKE-002** `[step]` — Second.")
	assert.Contains(t, out, "## Intake\n\n**SKILL-DEMO-INTAKE-002**")
	assert.True(t, strings.HasPrefix(out, "---\nname: demo\nversion: 1.0\n---\n# Demo\n"), "frontmatter and H1 untouched")
}

func TestRenderSkillDropsPayloadThroughFence(t *testing.T) {
	out, err := RenderSkill(renderFixture, []string{"SKILL-DEMO-TPL-*"})
	require.NoError(t, err)
	assert.NotContains(t, out, "```markdown")
	assert.NotContains(t, out, "## Templates")
	assert.Contains(t, out, "## Model")
}

func TestRenderSkillGlobEmptiesSection(t *testing.T) {
	out, err := RenderSkill(renderFixture, []string{"SKILL-DEMO-INTAKE-*"})
	require.NoError(t, err)
	assert.NotContains(t, out, "## Intake")
	assert.Contains(t, out, "Intro prose stays.")
	assert.Contains(t, out, "## Writing")
}

func TestRenderSkillEmptiedSubsectionOnly(t *testing.T) {
	out, err := RenderSkill(renderFixture, []string{"SKILL-DEMO-WRITING-001"})
	require.NoError(t, err)
	assert.NotContains(t, out, "### Style")
	assert.Contains(t, out, "## Writing")
	assert.Contains(t, out, "### Kept")
}

func TestRenderSkillEmptiedSubsectionsCascadeToParent(t *testing.T) {
	fixture := renderFixture + "\n## Modes\n\n### A\n\n" +
		"**SKILL-DEMO-MODEA-001** `[step]` — A.\n\n### B\n\n" +
		"**SKILL-DEMO-MODEB-001** `[step]` — B.\n\n## Closing\n\nBye.\n"
	out, err := RenderSkill(fixture, []string{"SKILL-DEMO-MODEA-*", "SKILL-DEMO-MODEB-*"})
	require.NoError(t, err)
	assert.NotContains(t, out, "### A")
	assert.NotContains(t, out, "### B")
	assert.NotContains(t, out, "## Modes")
	assert.Contains(t, out, "## Closing")
}

func TestRenderSkillUnknownTokenErrors(t *testing.T) {
	_, err := RenderSkill(renderFixture, []string{"SKILL-DEMO-NOPE-001"})
	assert.Error(t, err)
}
