package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skillsTestEnv builds a repo skills root with one skill (changelog optional),
// an evals dir, an examples dir, and a sessions store with invoked_skills.
func skillsTestEnv(t *testing.T, withChangelog bool) *Server {
	t.Helper()

	skillsRepo := t.TempDir()
	skillDir := filepath.Join(skillsRepo, "demo")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	manifest := "---\nname: demo\ndescription: demo skill\nversion: 1.1\nallowed-tools: Bash(jq *), Read\n---\n\n# Demo\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o644))
	if withChangelog {
		changelog := `[{"version": "1.1", "date": "2026-07-10", "text": "added X"}]`
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "changelog.json"), []byte(changelog), 0o644))
	}

	evalsDir := t.TempDir()
	evalSkillDir := filepath.Join(evalsDir, "demo")
	require.NoError(t, os.MkdirAll(evalSkillDir, 0o755))
	evalContent := `{
		"eval": {"skill": "demo", "date": "2026-07-11", "notes": "baseline"},
		"runs": [{"id": "run-1", "model": {"id": "claude-fable-5", "effort": "high"}}],
		"totals": [{"runId": "run-1", "axis": "compliance", "raw": 22, "max": 25, "pct": 88}]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(evalSkillDir, "eval-1.json"), []byte(evalContent), 0o644))

	examplesDir := t.TempDir()
	exampleSkillDir := filepath.Join(examplesDir, "demo")
	require.NoError(t, os.MkdirAll(exampleSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(exampleSkillDir, "input.md"), []byte("example\n"), 0o644))

	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "personal", "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0o755))
	batch := `{
		"batch": {"scope": "personal", "file": "b1.md", "number": 1, "analyzedDate": "2026-07-10"},
		"sessions": [{"id": "00000000-0000-0000-0000-000000000001", "title": "s1", "invoked_skills": ["demo"]}]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch-01.json"), []byte(batch), 0o644))

	return newTestServer(t, &Options{
		EvalsDir:    evalsDir,
		ExamplesDir: examplesDir,
		SessionsDir: sessionsDir,
		SkillsHome:  filepath.Join(t.TempDir(), "empty"),
		SkillsRepo:  skillsRepo,
	})
}

func TestSkillDetailRendersAllBlocks(t *testing.T) {
	server := skillsTestEnv(t, true)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, "Version history")
	assert.Contains(t, body, "allowed-tools: Bash(jq *), Read")
	assert.Contains(t, body, "added X")
	assert.Contains(t, body, "Eval runs")
	assert.Contains(t, body, "claude-fable-5")
	assert.Contains(t, body, "Examples")
	assert.Contains(t, body, "input.md")
	assert.Contains(t, body, "eval manifest stub")
	assert.Contains(t, body, "Invocations (1)")
	assert.Contains(t, body, "/sessions/personal/1?session=00000000-0000-0000-0000-000000000001")
}

func TestSkillDetailWithoutChangelogHidesBlock(t *testing.T) {
	server := skillsTestEnv(t, false)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "Version history")
}

func TestSkillDetailMalformedChangelogRendersError(t *testing.T) {
	server := skillsTestEnv(t, false)
	skillDir := filepath.Join(server.skillsRepo, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "changelog.json"), []byte("{nope"), 0o644))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "Failed to parse")
}

func TestSkillFileServesExample(t *testing.T) {
	server := skillsTestEnv(t, false)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo/file?f=input.md", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "example")
}

func addSkill(t *testing.T, server *Server, name string) {
	t.Helper()
	skillDir := filepath.Join(server.skillsRepo, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	manifest := "---\nname: " + name + "\ndescription: x\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o644))
}

func TestSkillsIndexInlineUsage(t *testing.T) {
	server := skillsTestEnv(t, false)
	addSkill(t, server, "aaa-unused")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// usage stats render inline in the one grouped view; no sort tabs exist.
	assert.Contains(t, body, "1 invocation")
	assert.Contains(t, body, "0 invocations")
	assert.Contains(t, body, "last used 2026-07-10")
	assert.NotContains(t, body, "sort=invocations")
	assert.NotContains(t, body, "sort=last-used")
}
