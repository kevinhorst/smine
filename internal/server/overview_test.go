package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverview(t *testing.T) {
	t.Run("tiles-render-with-fixtures", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")
		require.NoError(t, os.WriteFile(settingsPath, []byte(`{
			"model": "opus",
			"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]},
			"disabledMcpjsonServers": ["idle-server"]
		}`), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.disabled.json"), []byte(`{}`), 0644))

		claudeJsonPath := filepath.Join(dir, ".claude.json")
		require.NoError(t, os.WriteFile(claudeJsonPath, []byte(`{
			"mcpServers": {"goland": {}, "idle-server": {}}
		}`), 0644))

		checklistPath := filepath.Join(dir, "checklist.md")
		require.NoError(t, os.WriteFile(checklistPath, []byte(checklistFixture), 0644))

		skillsRepo := filepath.Join(dir, "skills-repo")
		require.NoError(t, os.MkdirAll(filepath.Join(skillsRepo, "jq", "workflows"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, "jq", "SKILL.md"),
			[]byte("---\nname: jq\ndescription: d\n---\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, "jq", "workflows", "run.js"),
			[]byte("export const meta = {}\n"), 0644))

		contextDir := filepath.Join(dir, "context")
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "rules"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "style"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "rules", "navigation.md"), []byte("# x\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "style", "go.md"), []byte("# x\n"), 0644))

		sessionsDir := filepath.Join(dir, "sessions")
		jsonDir := filepath.Join(sessionsDir, "personal", "json")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch2.json"), []byte(`{
			"batch": {"scope": "personal", "number": 2, "file": "batch2.md", "analyzedDate": "2026-07-17"},
			"sessions": [{
				"id": "ghi789",
				"title": "Third session",
				"frustration": [
					{"quote": "no", "trigger": "t"},
					{"quote": "still no", "trigger": "t"}
				]
			}]
		}`), 0644))

		proposalsDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(proposalsDir, 0755))
		proposalsJSON := `{"kind": "skills", "groups": [{"title": "New skills", "proposals": [
			{"id": "a", "title": "a", "status": "proposed"},
			{"id": "b", "title": "b"}
		]}]}`
		require.NoError(t, os.WriteFile(filepath.Join(proposalsDir, "skills.json"), []byte(proposalsJSON), 0644))

		server := newTestServer(t, &Options{
			ChecklistPath:  checklistPath,
			ClaudeJsonPath: claudeJsonPath,
			ContextDir:     contextDir,
			SessionsDir:    sessionsDir,
			SettingsPath:   settingsPath,
			SkillsHome:     filepath.Join(dir, "skills-home"),
			SkillsRepo:     skillsRepo,
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, "Sessions analyzed")
		assert.Contains(t, body, "1/1 enabled")            // hooks tile
		assert.Contains(t, body, "1/2 enabled")            // MCP tile: idle-server disabled in live list
		assert.Contains(t, body, "goland")                 // MCP detail
		assert.Contains(t, body, "Frustration / Positive") // merged entries card
		assert.Contains(t, body, `class="split-frustration"`)
		assert.Contains(t, body, `class="split-positive"`)
		assert.NotContains(t, body, "Positive entries")
		assert.Contains(t, body, "Proposals") // proposals tile
		assert.Contains(t, body, "1 skills")  // status-bearing entries only
		assert.Contains(t, body, "opus")      // model tile
		assert.NotContains(t, body, "Skills out of sync")
		assert.Contains(t, body, ">0/1<")                         // merged skills tile: 0 active / 1 repo
		assert.Contains(t, body, "active / repo · 1 out of sync") // repo-only jq has no home twin
		assert.Contains(t, body, "Workflows")
		assert.Contains(t, body, "skill workflow scripts")
		assert.Contains(t, body, "rules · style") // context tile detail: pack groups
		assert.Contains(t, body, "Tools")         // tools tile
		assert.Contains(t, body, "prune-jetbrains · secretscan")
		assert.Contains(t, body, `data-tile-id="skills"`)
		assert.Contains(t, body, "Repos") // repo tile (empty registry)
		assert.Contains(t, body, "0 worktrees")
		assert.Contains(t, body, "Routines") // routine tile (no routines dir)
		assert.Contains(t, body, "0/0 active")
		assert.Contains(t, body, "0/1 done")                    // checklist fixture: entry 8 Open
		assert.Contains(t, body, `href="/sessions/personal/2"`) // last analysis link: batch 2 is newest
		assert.Contains(t, body, `href="/sessions/personal/2?dimension=frustration&amp;open=ghi789#session-ghi789"`)
		assert.Contains(t, body, `href="/sessions/personal/1?dimension=positive&amp;open=def456#session-def456"`)
		assert.Contains(t, body, "last signal 2026-07-17") // merged card carries the newest signal batch date

		// default row order: sessions row, config row, repo row, orga row
		assert.Less(t, strings.Index(body, `data-tile-id="sessions"`), strings.Index(body, `data-tile-id="hooks"`))
		assert.Less(t, strings.Index(body, `data-tile-id="hooks"`), strings.Index(body, `data-tile-id="skills"`))
		assert.Less(t, strings.Index(body, `data-tile-id="skills"`), strings.Index(body, `data-tile-id="proposals"`))
		assert.Contains(t, body, "Sentiment trend")
		assert.Contains(t, body, "trend-frustration")
		assert.Contains(t, body, "&#43;1 vs batch 1") // "+1": batch1 1 entry, batch2 2 entries; template escapes the plus
	})

	t.Run("missing-checklist-degrades-tile", func(t *testing.T) {
		server := newTestServer(t, &Options{
			ChecklistPath: filepath.Join(t.TempDir(), "nope.md"),
			SessionsDir:   t.TempDir(),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "—")
	})

	t.Run("empty-everything-still-200", func(t *testing.T) {
		dir := t.TempDir()
		checklistPath := filepath.Join(dir, "checklist.md")
		require.NoError(t, os.WriteFile(checklistPath, []byte(checklistFixture), 0644))
		server := newTestServer(t, &Options{
			ChecklistPath: checklistPath,
			SessionsDir:   filepath.Join(dir, "no-sessions"),
			SkillsHome:    filepath.Join(dir, "no-home"),
			SkillsRepo:    filepath.Join(dir, "no-repo"),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "0/0 enabled")
		assert.NotContains(t, response.Body.String(), "Sentiment trend")
		// no batches → no analyzedDate → the merged card carries no detail
		assert.NotContains(t, response.Body.String(), "last signal")
	})
}
