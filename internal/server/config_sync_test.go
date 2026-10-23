package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeClaudeTemplate writes a Claude settings template dir under dir whose
// base.json holds base, and returns the template dir.
func writeClaudeTemplate(t *testing.T, dir, base string) string {
	t.Helper()
	templateDir := filepath.Join(dir, "settings", "claude_code")
	require.NoError(t, os.MkdirAll(filepath.Join(templateDir, config.ComponentFragmentDir), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, config.BaseSettingsFile), []byte(base), 0644))
	return templateDir
}

// writeControlPortRecord saves an install record under claudeHome whose
// peek.control_port is 42542.
func writeControlPortRecord(t *testing.T, claudeHome string) {
	t.Helper()
	env := install.Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS}
	record := install.FullRecord(env.GOOS)
	controlPort, _ := components.LookupSetting(components.Peek, "control_port")
	value, err := components.ParseText(&controlPort, "42542")
	require.NoError(t, err)
	require.NoError(t, install.SetOverride(&record, components.Peek, "control_port", value))
	require.NoError(t, install.Save(env, record))
}

// writeEngineFixtures isolates HOME and builds a server over a checkout whose
// template holds base and one gate fragment, with an install record setting
// peek.control_port=42542 and live settings holding live; it returns the
// live settings path.
func writeEngineFixtures(t *testing.T, base, gateFragment, live string) (livePath string, server *Server) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	livePath = filepath.Join(home, ".claude", "settings.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(livePath), 0755))
	require.NoError(t, os.WriteFile(livePath, []byte(live), 0644))

	repoDir := t.TempDir()
	templateDir := writeClaudeTemplate(t, repoDir, base)
	gatePath := filepath.Join(templateDir, config.ComponentFragmentDir, "gate.json")
	require.NoError(t, os.WriteFile(gatePath, []byte(gateFragment), 0644))
	writeControlPortRecord(t, filepath.Dir(livePath))

	options := &Options{ClaudeFragmentPath: templateDir, RepoDir: repoDir, SettingsPath: livePath}
	return livePath, newTestServer(t, options)
}

// newSectionServer builds a server whose live settings and repo template
// hold the given JSON — the fixture for the section overridden badges (D1).
func newSectionServer(t *testing.T, live, fragment string) *Server {
	t.Helper()
	dir := t.TempDir()
	livePath := filepath.Join(dir, "settings.json")
	require.NoError(t, os.WriteFile(livePath, []byte(live), 0644))
	return newTestServer(t, &Options{ClaudeFragmentPath: writeClaudeTemplate(t, dir, fragment), SettingsPath: livePath})
}

// writeMarkerFixtures builds a server whose live file and repo fragment hold
// the given content, with an install record in the settings dir — the
// server's Claude home — setting peek.control_port=42542. A name ending in .toml wires the Codex
// pair, anything else the Claude live file and template.
func writeMarkerFixtures(t *testing.T, name, live, fragment string) (livePath, fragmentPath string, server *Server) {
	t.Helper()
	dir := t.TempDir()
	livePath = filepath.Join(dir, "live-"+name)
	require.NoError(t, os.WriteFile(livePath, []byte(live), 0644))
	writeControlPortRecord(t, dir)

	options := &Options{SettingsPath: filepath.Join(dir, "settings.json")}
	if strings.HasSuffix(name, ".toml") {
		fragmentPath = filepath.Join(dir, "fragment-"+name)
		require.NoError(t, os.WriteFile(fragmentPath, []byte(fragment), 0644))
		options.CodexFragmentPath, options.CodexPath = fragmentPath, livePath
	} else {
		fragmentPath = writeClaudeTemplate(t, dir, fragment)
		options.ClaudeFragmentPath, options.SettingsPath = fragmentPath, livePath
	}
	return livePath, fragmentPath, newTestServer(t, options)
}

// homeNative is the home directory as the {{HOME}} marker expands it.
func homeNative(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	return filepath.ToSlash(home)
}

// assertSectionBadge asserts the fragment carries the OOB summary span and
// whether the overridden badge is inside it.
func assertSectionBadge(t *testing.T, server *Server, path, section string, overridden bool) {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", path, nil))
	require.Equal(t, 200, response.Code, response.Body.String())

	body := response.Body.String()
	spanAt := strings.Index(body, `id="ovr-`+section+`" hx-swap-oob="true"`)
	require.Greater(t, spanAt, -1, body)
	span := body[spanAt:]
	span = span[:strings.Index(span, "</span>")+len("</span>")]
	if overridden {
		assert.Contains(t, span, "overridden</span>")
	} else {
		assert.NotContains(t, span, "overridden</span>")
	}
}

// stashedBases lists the base name of every file in the server's trash.
func stashedBases(t *testing.T, server *Server) []string {
	t.Helper()
	stashes, err := server.trash.List()
	require.NoError(t, err)

	var bases []string
	for _, stash := range stashes {
		for _, rel := range stashRels(stash) {
			bases = append(bases, path.Base(rel))
		}
	}
	return bases
}

func TestConfigSync(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	confirmedForm := url.Values{"confirmed": {"on"}}
	forceLive := `{"model": "live", "stale": true, "env": {"MINE": "y"}, "permissions": {"allow": ["Read"], "additionalDirectories": ["/old"]}, "hooks": {"Stop": [{"hooks": []}]}}`

	t.Run("claude-apply-writes-base-and-owning-fragment", func(t *testing.T) {
		live := `{"model": "live", "cleanupPeriodDays": 180, "env": {"OWNED": "changed", "MINE": "x"}, "permissions": {"allow": ["Bash(x *)", "Read"], "defaultMode": "plan"}, "hooks": {"Stop": [{"hooks": []}]}}`
		livePath, server := writeEngineFixtures(t,
			`{"model": "fragment", "stale": true, "env": {"OTEL": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`,
			`{"permissions": {"allow": ["Bash(x *)"]}, "env": {"OWNED": "original"}}`,
			live)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "base cleanupPeriodDays")
		assert.Contains(t, response.Body.String(), "gate env.OWNED")
		assert.Contains(t, response.Body.String(), "base stale removed")

		base, err := os.ReadFile(filepath.Join(server.repoDir, "settings", "claude_code", config.BaseSettingsFile))
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "live", "cleanupPeriodDays": 180, "env": {"MINE": "x"}, "permissions": {"allow": ["Read"], "defaultMode": "plan"}}`, string(base))

		gate, err := os.ReadFile(filepath.Join(server.repoDir, "settings", "claude_code", config.ComponentFragmentDir, "gate.json"))
		require.NoError(t, err)
		assert.JSONEq(t, `{"permissions": {"allow": ["Bash(x *)"]}, "env": {"OWNED": "changed"}}`, string(gate))

		unchanged, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, live, string(unchanged))
	})

	t.Run("claude-apply-keeps-markers", func(t *testing.T) {
		_, server := writeEngineFixtures(t,
			`{"env": {"OTEL": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`,
			`{}`,
			`{"env": {"OTEL": "http://127.0.0.1:42542/otlp"}, "model": "live"}`)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		base, err := os.ReadFile(filepath.Join(server.repoDir, "settings", "claude_code", config.BaseSettingsFile))
		require.NoError(t, err)
		assert.JSONEq(t, `{"env": {"OTEL": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}, "model": "live"}`, string(base))
	})

	t.Run("claude-apply-twice-no-changes", func(t *testing.T) {
		_, server := writeEngineFixtures(t, `{"model": "fragment"}`, `{}`, `{"model": "live"}`)
		first := httptest.NewRecorder()
		server.Handler().ServeHTTP(first, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, first.Code, first.Body.String())

		second := httptest.NewRecorder()
		server.Handler().ServeHTTP(second, formPost("/api/config/claude/sync/apply", nil))
		require.Equal(t, 200, second.Code, second.Body.String())
		assert.Contains(t, second.Body.String(), "(no changes)")
	})

	t.Run("claude-revert-runs-settings-engine", func(t *testing.T) {
		livePath, server := writeEngineFixtures(t,
			`{"model": "fragment", "env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`,
			`{"permissions": {"allow": ["Bash(x *)"]}}`,
			`{"model": "live"}`)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Equal(t, "config-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), "Bash(x *)")

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "live", "env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:42542/otlp"}, "permissions": {"allow": ["Bash(x *)"]}}`, string(live))
	})

	t.Run("claude-force-rebuilds-live-from-template", func(t *testing.T) {
		livePath, server := writeEngineFixtures(t,
			`{"model": "fragment", "env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`,
			`{"permissions": {"allow": ["Bash(x *)"]}}`,
			forceLive)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/force", confirmedForm))
		require.Equal(t, 200, response.Code, response.Body.String())
		assert.Contains(t, response.Body.String(), "settings.json rebuilt from the template")

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.JSONEq(t, `{"model": "fragment", "env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:42542/otlp"}, "permissions": {"allow": ["Bash(x *)"]}}`, string(live))
		assert.Contains(t, stashedBases(t, server), "settings.json")
	})

	t.Run("claude-force-refused-without-confirm", func(t *testing.T) {
		livePath, server := writeEngineFixtures(t, `{"model": "fragment"}`, `{}`, forceLive)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/force", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "preview first")

		unchanged, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, forceLive, string(unchanged))
	})

	t.Run("claude-force-preview-lists-drops", func(t *testing.T) {
		livePath, server := writeEngineFixtures(t, `{"model": "fragment"}`, `{"permissions": {"allow": ["Bash(x *)"]}}`, forceLive)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/force/preview", nil))
		require.Equal(t, 200, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, "<code>stale</code>")
		assert.Contains(t, body, "<code>MINE</code>")
		assert.Contains(t, body, "<code>permissions.allow Read</code>")
		assert.Contains(t, body, "<code>Stop (unnamed group)</code>")
		assert.Contains(t, body, `hx-post="/api/config/claude/sync/force"`)
		assert.Contains(t, body, `<input type="hidden" name="confirmed" value="on">`)

		unchanged, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, forceLive, string(unchanged))
	})

	t.Run("codex-force-preview-404", func(t *testing.T) {
		_, _, server := writeMarkerFixtures(t, "config.toml", "model = \"live\"\n", "model = \"fragment\"\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/sync/force/preview", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("codex-sync-keeps-previous", func(t *testing.T) {
		livePath, _, server := writeMarkerFixtures(t, "config.toml", "model = \"live\"\n", "model = \"fragment\"\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/sync/revert", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, "model = \"fragment\"\n", string(live))
		assert.Contains(t, stashedBases(t, server), filepath.Base(livePath))
	})

	t.Run("claude-revert-twice-no-changes", func(t *testing.T) {
		_, server := writeEngineFixtures(t, `{"model": "fragment"}`, `{"permissions": {"allow": ["Bash(x *)"]}}`, `{}`)
		first := httptest.NewRecorder()
		server.Handler().ServeHTTP(first, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, first.Code, first.Body.String())

		second := httptest.NewRecorder()
		server.Handler().ServeHTTP(second, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, second.Code, second.Body.String())
		assert.Contains(t, second.Body.String(), "(no changes)")
	})

	t.Run("claude-revert-refuses-invalid-fragment", func(t *testing.T) {
		live := `{"model": "live"}`
		livePath, server := writeEngineFixtures(t, `{"model": "fragment"}`, `{"permissions": `, live)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/claude/sync/revert", nil))
		require.Equal(t, 200, response.Code)
		assert.Contains(t, response.Body.String(), "gate.json")

		unchanged, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, live, string(unchanged))
	})

	t.Run("codex-revert-expands-markers", func(t *testing.T) {
		home := homeNative(t)
		livePath, _, server := writeMarkerFixtures(t, "config.toml",
			"notify = [\"/old/notify\"]\n",
			"notify = [\"{{HOME}}/.codex/notify\"]\n")

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/sync/revert", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		live, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, "notify = [\""+home+"/.codex/notify\"]\n", string(live))
	})

	t.Run("codex-apply-collapses-markers", func(t *testing.T) {
		home := homeNative(t)
		_, fragmentPath, server := writeMarkerFixtures(t, "config.toml",
			"notify = [\""+home+"/.codex/notify\"]\nmodel = \"live\"\n",
			"notify = [\"{{HOME}}/.codex/notify\"]\nmodel = \"fragment\"\n")

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/sync/apply", nil))
		require.Equal(t, 200, response.Code, response.Body.String())

		fragment, err := os.ReadFile(fragmentPath)
		require.NoError(t, err)
		assert.Equal(t, "notify = [\"{{HOME}}/.codex/notify\"]\nmodel = \"live\"\n", string(fragment))
	})

	t.Run("codex-marker-fragment-not-overridden", func(t *testing.T) {
		home := homeNative(t)
		_, _, server := writeMarkerFixtures(t, "config.toml",
			"notify = [\""+home+"/.codex/notify\"]\n",
			"notify = [\"{{HOME}}/.codex/notify\"]\n")

		rows, err := server.codexRows()
		require.NoError(t, err)
		index := slices.IndexFunc(rows, func(row configRow) bool { return row.Key == "notify" })
		require.GreaterOrEqual(t, index, 0)
		assert.False(t, rows[index].Overridden)
	})

	t.Run("marker-fragment-matches-expanded-live", func(t *testing.T) {
		live := `{"env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:42542/otlp"}}`
		fragment := `{"env": {"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}}`
		_, _, server := writeMarkerFixtures(t, "settings.json", live, fragment)
		assertSectionBadge(t, server, "/api/env", "env", false)
	})

	t.Run("home-marker-permissions-not-overridden", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		dir := filepath.ToSlash(home) + "/GolandProjects/smine"
		live := `{"permissions": {"additionalDirectories": ["` + dir + `"]}}`
		fragment := `{"permissions": {"additionalDirectories": ["{{HOME}}/GolandProjects/smine"]}}`
		server := newSectionServer(t, live, fragment)
		assertSectionBadge(t, server, "/api/permissions", "permissions", false)
	})

	t.Run("unknown-target-404", func(t *testing.T) {
		server := newTestServer(t, &Options{})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/bogus/sync/apply", nil))
		assert.Equal(t, 404, response.Code)
	})

	t.Run("codex-missing-source-errors-without-partial-write", func(t *testing.T) {
		live := "model = \"live\"\n"
		livePath, fragmentPath, server := writeMarkerFixtures(t, "config.toml", live, "model = \"fragment\"\n")
		require.NoError(t, os.Remove(fragmentPath))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/config/codex/sync/revert", nil))
		// Op errors render inside the result block, not as HTTP errors.
		require.Equal(t, 200, response.Code)
		assert.Contains(t, response.Body.String(), "handleConfigSyncOp")

		unchanged, err := os.ReadFile(livePath)
		require.NoError(t, err)
		assert.Equal(t, live, string(unchanged))
	})
}
