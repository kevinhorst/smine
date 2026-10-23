package contextdocs

import (
	"regexp"
	"strings"
)

// DefaultRole is the AGENTS.md role line when the deploy record names none.
const DefaultRole = "You are a development agent for this repository."

// The leading comment documents the template, not the deployed copy.
var templateCommentPattern = regexp.MustCompile(`(?s)\A<!--.*?-->\n*`)

// A language block spans its own marker lines; the closing marker's name is not checked.
var langBlockPattern = regexp.MustCompile(`(?ms)^\{\{LANG:([a-z_]+)\}\}\n(.*?)^\{\{/LANG:[a-z_]+\}\}\n`)

// RenderAgentsTemplate expands context/AGENTS.md: comment dropped, placeholders replaced, selected language blocks unwrapped, the others removed.
func RenderAgentsTemplate(template string, role string, contextDir string, langs []string) string {
	selected := toSet(langs)
	content := templateCommentPattern.ReplaceAllString(template, "")
	content = strings.ReplaceAll(content, "{{ROLE}}", role)
	content = strings.ReplaceAll(content, "{{CONTEXT_DIR}}", contextDir)
	return langBlockPattern.ReplaceAllStringFunc(content, func(block string) string {
		match := langBlockPattern.FindStringSubmatch(block)
		if selected[match[1]] {
			return match[2]
		}
		return ""
	})
}
