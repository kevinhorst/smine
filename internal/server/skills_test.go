package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillsStateFilter(t *testing.T) {
	writeTestSkill := func(t *testing.T, root, name string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: 1.0\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeTestSkill(t, skillsRepo, "repo-skill")
	writeTestSkill(t, skillsRepo, "both-skill")
	writeTestSkill(t, skillsHome, "both-skill")
	writeTestSkill(t, skillsHome, "home-skill")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	t.Run("all-states-with-counts", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/repo-skill")
		assert.Contains(t, body, "/home-skill")
		assert.Contains(t, body, "/both-skill")
		assert.Contains(t, body, "All (3)")
		assert.Contains(t, body, "Synced (1)")
		assert.Contains(t, body, "Active only (1)")
		assert.Contains(t, body, "Repo only (1)")
		assert.Contains(t, body, "v1.0")
		assert.Contains(t, body, "0 invocations")
		assert.NotContains(t, body, "local")
		// The list re-pulls itself after a sync op (repo-op trigger).
		assert.Contains(t, body, `id="skills-list" hx-get="/scripts/skills"`)
		assert.Contains(t, body, `hx-trigger="repo-op from:body"`)
		// Synced cards link to the repo variant (D8).
		assert.Contains(t, body, "/scripts/skills/repo/both-skill")
		assert.NotContains(t, body, "/scripts/skills/home/both-skill")
	})

	t.Run("state-filter-selects-subset", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills?state=active", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/home-skill")
		assert.NotContains(t, body, "/repo-skill")
		assert.NotContains(t, body, "/both-skill")
		// Counts stay global under a filter; the re-pull URL keeps the state.
		assert.Contains(t, body, "All (3)")
		assert.Contains(t, body, `id="skills-list" hx-get="/scripts/skills?state=active"`)
	})

	t.Run("invalid-state-falls-back-to-all", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills?state=bogus", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "/repo-skill")
		assert.Contains(t, body, "/home-skill")
		assert.Contains(t, body, "/both-skill")
	})
}

func TestMergeSkills(t *testing.T) {
	list := []skills.Skill{
		{Name: "both", Origin: skills.OriginHome, Path: "/home/both"},
		{Name: "both", Origin: skills.OriginRepo, Path: "/repo/both"},
		{Name: "home-only", Origin: skills.OriginHome},
		{Name: "repo-only", Origin: skills.OriginRepo},
	}

	cards := mergeSkills(list)
	require.Len(t, cards, 3)
	assert.Equal(t, "both", cards[0].Skill.Name)
	assert.Equal(t, skillStateSynced, cards[0].State)
	// The synced pair folds to the repo variant (D8).
	assert.Equal(t, skills.OriginRepo, cards[0].Skill.Origin)
	assert.Equal(t, "/repo/both", cards[0].Skill.Path)
	assert.Equal(t, skillStateActive, cards[1].State)
	assert.Equal(t, "home-only", cards[1].Skill.Name)
	assert.Equal(t, skillStateRepo, cards[2].State)
	assert.Equal(t, "repo-only", cards[2].Skill.Name)
	// equal versions carry no drift marker
	assert.Empty(t, cards[0].HomeVersion)

	drifted := mergeSkills([]skills.Skill{
		{Name: "bumped", Origin: skills.OriginHome, Version: "1.1"},
		{Name: "bumped", Origin: skills.OriginRepo, Version: "1.2"},
	})
	require.Len(t, drifted, 1)
	assert.Equal(t, skillStateSynced, drifted[0].State)
	assert.Equal(t, "1.1", drifted[0].HomeVersion)
	assert.Equal(t, "1.2", drifted[0].Skill.Version)

	assert.Empty(t, mergeSkills(nil))
}

func TestSkillsSyncRendersStubOutput(t *testing.T) {
	syncScripts := t.TempDir()
	stub := "#!/bin/sh\necho \"skill-sync $*\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(syncScripts, "sync_skills.sh"), []byte(stub), 0o755))
	server := newTestServer(t, &Options{SyncScriptsDir: syncScripts})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/scripts/skills/sync", url.Values{}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "skill-sync")
	assert.NotContains(t, response.Body.String(), "--prune")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/scripts/skills/sync", url.Values{"prune": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "skill-sync --prune")
}

func TestSkillsGroupingAndBadges(t *testing.T) {
	writeTestSkill := func(t *testing.T, root, name string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeTestSkill(t, skillsRepo, "fam/fam-a")
	writeTestSkill(t, skillsRepo, "fam/fam-b")
	writeTestSkill(t, skillsRepo, "solo")
	writeTestSkill(t, skillsHome, "fam-a")
	writeTestSkill(t, skillsHome, "extra")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	t.Run("groups-and-state-badges", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "<summary>fam (2)")
		assert.NotContains(t, body, "<summary>solo (1)")
		assert.Contains(t, body, "/solo")
		assert.Contains(t, body, ">Synced<")
		assert.Contains(t, body, ">Active only<")
		assert.Contains(t, body, ">Repo only<")
		// Family summary carries the invocation sum; no drift, no alert.
		assert.Contains(t, body, "0 invocations")
		assert.NotContains(t, body, "Update needed")
	})
}

func TestSkillGroups(t *testing.T) {
	list := []skillCard{
		{Invocations: 4, Skill: skills.Skill{Group: "smine", Name: "smine"}},
		{HomeVersion: "1.1", Invocations: 2, Skill: skills.Skill{Group: "smine", Name: "smine-memory", Version: "1.2"}},
		{Skill: skills.Skill{Group: "smine", Name: "smine-context"}},
		{Invocations: 7, Skill: skills.Skill{Name: "jq"}},
		{Skill: skills.Skill{Group: "skillroutine", Name: "skillroutine-create"}},
		{Skill: skills.Skill{Group: "skillroutine", Name: "skillroutine-eval"}},
		{Skill: skills.Skill{Group: "concept", Name: "clarify"}},
		{Skill: skills.Skill{Group: "concept", Name: "concept"}},
	}

	groups := skillGroups(list)
	require.Len(t, groups, 4)
	// Multi-member groups first, alphabetical within (concept, skillroutine, smine), then single-member jq.
	assert.Equal(t, "concept", groups[0].Family)
	assert.Len(t, groups[0].Skills, 2)
	assert.Equal(t, "skillroutine", groups[1].Family)
	assert.Len(t, groups[1].Skills, 2)
	// Aggregates: invocations summed, drifted members counted (D3).
	assert.Equal(t, "smine", groups[2].Family)
	assert.Len(t, groups[2].Skills, 3)
	assert.Equal(t, 6, groups[2].Invocations)
	assert.Equal(t, 1, groups[2].UpdatesNeeded)
	assert.Equal(t, "jq", groups[3].Family)
	assert.Len(t, groups[3].Skills, 1)
	assert.Equal(t, 7, groups[3].Invocations)
	assert.Empty(t, skillGroups(nil))
}

func TestSkillsIndexVersionDriftBadge(t *testing.T) {
	writeVersioned := func(t *testing.T, root, name, version string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: " + version + "\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillsHome := filepath.Join(dir, "home")
	writeVersioned(t, skillsRepo, "bumped-skill", "1.2")
	writeVersioned(t, skillsHome, "bumped-skill", "1.1")
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// One card, still under the Synced tab; the badge carries the drift (D7).
	assert.Contains(t, body, "Synced (1)")
	assert.Contains(t, body, "v1.1 → v1.2")
	assert.NotContains(t, body, `>v1.2</span>`)
	assert.Contains(t, body, ">Update needed<")
}
