package contextdocs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const agentsTemplate = "<!-- template marker\nspanning lines -->\n\n# Agents\n\n{{ROLE}}\n\n- `{{CONTEXT_DIR}}/actions/`\n{{LANG:go}}\n- Go: {{CONTEXT_DIR}}/rules/go.md\n{{/LANG:go}}\n{{LANG:python}}\n- Python: {{CONTEXT_DIR}}/rules/python.md\n{{/LANG:python}}\n- tail\n"

func TestRenderAgentsTemplate(t *testing.T) {
	out := RenderAgentsTemplate(agentsTemplate, "You are a bot.", "docs", []string{"go"})

	assert.Equal(t, "# Agents\n\nYou are a bot.\n\n- `docs/actions/`\n- Go: docs/rules/go.md\n- tail\n", out)
}

func TestRenderAgentsTemplateNoLanguages(t *testing.T) {
	out := RenderAgentsTemplate(agentsTemplate, DefaultRole, "context", nil)

	assert.Equal(t, "# Agents\n\nYou are a development agent for this repository.\n\n- `context/actions/`\n- tail\n", out)
}

func TestRenderAgentsTemplateWithoutComment(t *testing.T) {
	out := RenderAgentsTemplate("# Agents\n{{ROLE}}\n", "r", "docs", nil)

	assert.Equal(t, "# Agents\nr\n", out)
}
