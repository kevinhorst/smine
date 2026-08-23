package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPermRule(t *testing.T) {
	for _, testCase := range []struct {
		text string
		want bool
	}{
		{"Bash(jq *)", true},
		{"Bash(git commit -m *)", true},
		{"WebFetch(domain:x.dev)", true},
		{"Read(//Users/x/**)", true},
		{"mcp__serena__find_symbol", true},
		{"Load(path)", false},
		{"jq", false},
		{"Bash()", false},
		{"mcp__", false},
	} {
		assert.Equal(t, testCase.want, isPermRule(testCase.text), testCase.text)
	}
}

func TestRenderDocMarkdownDecorates(t *testing.T) {
	// Fixture: allow ls+cat, ask rm, disabled-allow jq (writePermissionSettings).
	_, server := writePermissionSettings(t)

	doc := "uses `Bash(ls *)`, `Bash(rm *)`, `Bash(jq *)`, `Bash(rg *)`, plain `settings.json`."
	body, err := server.renderDocMarkdown([]byte(doc))
	require.NoError(t, err)
	html := string(body)

	// allowed → passive badge, no button for that rule
	assert.Contains(t, html, ">allowed</span>")
	// ask → passive badge
	assert.Contains(t, html, ">ask</span>")
	// parked → enable button
	assert.Contains(t, html, ">enable</button>")
	// absent → + allow button with the QueryEscaped POST URL
	// html/template entity-escapes the QueryEscape "+" inside the attribute.
	assert.Contains(t, html, "rule=Bash%28rg&#43;%2A%29")
	assert.Contains(t, html, "+ allow")
	// ordinary code span stays a stock <code> element
	assert.Contains(t, html, "<code>settings.json</code>")
	assert.NotContains(t, html, `settings.json</code><button`)
}

func TestRenderDocMarkdownEscapes(t *testing.T) {
	_, server := writePermissionSettings(t)

	body, err := server.renderDocMarkdown([]byte("run `Bash(echo \"<b>\")` and `<i>plain</i>`"))
	require.NoError(t, err)
	html := string(body)

	assert.NotContains(t, html, "<b>")
	assert.NotContains(t, html, "<i>")
}

func TestRenderDocMarkdownDegrades(t *testing.T) {
	// A directory at the settings path makes config.Load fail → plain render.
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.Mkdir(settingsPath, 0o755))
	server := newTestServer(t, &Options{SettingsPath: settingsPath})

	doc := "`Bash(rg *)` stays undecorated"
	body, err := server.renderDocMarkdown([]byte(doc))
	require.NoError(t, err)

	plain, err := renderMarkdown([]byte(doc))
	require.NoError(t, err)
	assert.Equal(t, plain, body)
}
