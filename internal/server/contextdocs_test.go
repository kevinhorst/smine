package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

func TestEnforcementTitleAndRoleTitle(t *testing.T) {
	for _, channel := range []string{channelGate, channelHook, channelLint, channelSchema, channelProse} {
		assert.NotEmpty(t, enforcementTitle(channel), channel)
	}
	assert.Empty(t, enforcementTitle("manual"))

	for _, role := range []string{roleStep, roleStop, rolePayload} {
		assert.NotEmpty(t, roleTitle(role), role)
	}
	assert.Empty(t, roleTitle("bogus"))
}

// writeRegistryFixture returns a repos.json path with one repo "demo" at
// targetPath, so context tests can select it in the target dropdown.
func writeRegistryFixture(t *testing.T, targetPath string) string {
	t.Helper()
	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "demo", "path": ` + jsonString(t, targetPath) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))
	return reposPath
}

func writeContextFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), contextdocs.SourceContextDir)
	require.NoError(t, os.MkdirAll(filepath.Join(src, "actions"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "rules"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("<!-- template marker -->\n# Agents"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "rules", "plan.md"), []byte("# Plan Format"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "rules", "go.md"), []byte("# Go Style"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "rules", "notes.txt"), []byte("plain"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "actions", "navigation.md"),
		[]byte("**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n"), 0644))
	contextJson := `{"entries": [
		{"id": "ACTION-NAV-001", "kind": "action", "origin": "baseline", "scope": "NAV", "content": {"statement": "Never scan.", "applies": "everywhere."}, "enforcement": "prose", "reach": "global", "version": "1.0", "source": "context/actions/navigation.md"},
		{"id": "RULE-NAV-INTEG-001", "kind": "rule", "scope": "NAV", "topic": "INTEG", "content": {"statement": "Names are verbose."}, "enforcement": "prose", "reach": "smine", "version": "1.0", "source": "context/rules/go.md"},
		{"id": "FACT-NAV-001", "kind": "fact", "scope": "NAV", "content": {"statement": "Go services."}, "reach": "global", "version": "1.0", "source": "context/facts/repo.md"}],
		"aspects": [{"name": "INTEG", "scope": "Data integrity", "class": "topic"}, {"name": "NAV", "scope": "Navigation", "class": "scope"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(src, "context.json"), []byte(contextJson), 0644))
	aspects := []contextdocs.RuleAspect{
		{Name: "INTEG", Scope: "Data integrity", Class: "topic"},
		{Name: "NAV", Scope: "Navigation", Class: "scope"},
	}
	require.NoError(t, contextdocs.SaveAspects(src, aspects))
	return src
}

func TestContextIndex(t *testing.T) {
	server := newTestServer(t, &Options{
		ContextDir: writeContextFixture(t),
		ReposPath:  writeRegistryFixture(t, t.TempDir()),
	})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// One page: the acdsl section and the prose kind sections render together.
	assert.NotContains(t, body, `class="tabs"`)
	assert.Contains(t, body, `id="acdsl-section"`)
	assert.Contains(t, body, `action="/context/reach/bulk"`)
	// Kind sections in fixed order, one collapsible scope group each — no raw
	// .md file-list cards anymore.
	assert.Contains(t, body, "<h2>actions</h2>")
	assert.Contains(t, body, "<h2>rules</h2>")
	assert.Contains(t, body, "<h2>facts</h2>")
	assert.Less(t, strings.Index(body, "<h2>actions</h2>"), strings.Index(body, "<h2>rules</h2>"))
	assert.Contains(t, body, `NAV <span class="badge badge-dim">`)
	assert.Contains(t, body, `href="/context/entry/ACTION-NAV-001"`)
	assert.Contains(t, body, `href="/context/actions/navigation.md"`)
	assert.Contains(t, body, "v1.0")
	assert.NotContains(t, body, `class="label" href="/context/rules/plan.md"`)
	assert.Contains(t, body, "template marker")
	assert.Contains(t, body, `name="name"`)
	assert.Contains(t, body, `<option value="demo">demo</option>`)
	assert.Contains(t, body, `name="context-dir"`)
	assert.NotContains(t, body, `name="lang"`)
	assert.NotContains(t, body, `name="symlink"`)
	assert.Contains(t, body, `id="aspect-bar"`)
	assert.Contains(t, body, ">NAV (0/3)</a>")
	assert.Contains(t, body, ">INTEG (1)</a>")
	assert.Contains(t, body, `hx-post="/context/aspects"`)
	assert.NotContains(t, body, `<details class="section" open`)

	// Topic chip navigation: the matching scope group opens and filters to
	// the topic; every other group stays rendered but collapsed.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?aspect=NAV&topic=INTEG", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, "RULE-NAV-INTEG-001")
	assert.Contains(t, body, `<details class="section" open`)
	assert.Contains(t, body, `href="/context/entry/ACTION-NAV-001"`)

	// Reach dimension: the repo section filters entries orthogonally to scope.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?reach=smine", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, "RULE-NAV-INTEG-001")
	assert.NotContains(t, body, `href="/context/entry/ACTION-NAV-001"`)
	assert.Contains(t, body, ">smine (0/1)</a>")
	assert.Contains(t, body, ">global (0/2)</a>")
}

func TestContextFilterURL(t *testing.T) {
	filter := contextFilter{Aspect: "GOLANG", Layer: "acdsl", Lifetime: "task", Reach: "go", Topic: "ERR"}
	assert.Equal(t, "/context?aspect=GOLANG&layer=acdsl&lifetime=task&reach=go&topic=ERR", filter.URL("/context"))
	assert.True(t, filter.Active())
	assert.Equal(t, "/context", contextFilter{}.URL("/context"))
	assert.False(t, contextFilter{}.Active())

	// changing the scope clears the topic; other axes are preserved
	next := filter.with("aspect", "PLAN")
	assert.Equal(t, "PLAN", next.Aspect)
	assert.Empty(t, next.Topic)
	assert.Equal(t, "go", next.Reach)

	cleared := filter.with("reach", "")
	assert.Empty(t, cleared.Reach)
	assert.Equal(t, "ERR", cleared.Topic)
}

func TestContextIndexLayerAndLifetimeFilters(t *testing.T) {
	server := newTestServer(t, &Options{
		ContextDir: writeContextFixture(t),
		ReposPath:  writeRegistryFixture(t, t.TempDir()),
	})

	get := func(target string) string {
		t.Helper()
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		require.Equal(t, http.StatusOK, response.Code)
		return response.Body.String()
	}

	// layer=fact hides the other kinds, the acdsl section and the baseline;
	// the matching group renders open.
	body := get("/context?layer=fact")
	assert.Contains(t, body, "<h2>facts</h2>")
	assert.NotContains(t, body, "<h2>actions</h2>")
	assert.NotContains(t, body, "<h2>rules</h2>")
	assert.NotContains(t, body, `id="acdsl-section"`)
	assert.NotContains(t, body, "template marker")
	assert.Contains(t, body, `<details class="section" open`)

	// layer=acdsl hides the prose body entirely, baseline and global with it
	body = get("/context?layer=acdsl")
	assert.Contains(t, body, `id="acdsl-section"`)
	assert.NotContains(t, body, `id="prose-body"`)
	assert.NotContains(t, body, "<h2>baseline</h2>")
	assert.NotContains(t, body, "<h2>global</h2>")

	// unfiltered: baseline renders before global, global before acdsl
	body = get("/context")
	baselineAt := strings.Index(body, "<h2>baseline</h2>")
	globalAt := strings.Index(body, "<h2>global</h2>")
	acdslAt := strings.Index(body, `id="acdsl-section"`)
	require.NotEqual(t, -1, baselineAt)
	require.NotEqual(t, -1, globalAt)
	require.NotEqual(t, -1, acdslAt)
	assert.Less(t, baselineAt, globalAt)
	assert.Less(t, globalAt, acdslAt)
	assert.Contains(t, body, `action="/context/global/toggle"`)
	// the global toggle form closes before the bulk form opens, so every item
	// checkbox below belongs to bulk-reach
	toggleAt := strings.Index(body, `action="/context/global/toggle"`)
	bulkFormAt := strings.Index(body, `<form id="bulk-reach"`)
	require.NotEqual(t, -1, bulkFormAt)
	assert.Less(t, toggleAt, bulkFormAt)
	assert.Less(t, bulkFormAt, acdslAt)
	// the reserved options carry the exclusive marker, repo names do not
	assert.Contains(t, body, `value="global" data-exclusive`)
	assert.Contains(t, body, `value="none" data-exclusive`)
	assert.Contains(t, body, `value="smine" form="bulk-reach"`)
	assert.NotContains(t, body, `value="smine" data-exclusive`)

	// the filter card carries layer and lifetime chip rows
	body = get("/context")
	assert.Contains(t, body, ">acdsl</a>")
	assert.Contains(t, body, ">task</a>")
	assert.Contains(t, body, ">doctrine</a>")

	// an active reach drops scopes with nothing under it from the bar
	body = get("/context?reach=go")
	assert.NotContains(t, body, ">NAV (0/0)</a>")
}

func TestContextAspects(t *testing.T) {
	readAspectsFile := func(t *testing.T, contextDir string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(contextDir, "context.json"))
		require.NoError(t, err)
		return string(data)
	}

	t.Run("add-persists-sorted", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"arch"}, "scope": {"Architecture"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "ARCH")

		file := readAspectsFile(t, contextDir)
		assert.Contains(t, file, `"ARCH"`)
		assert.Less(t, strings.Index(file, `"ARCH"`), strings.Index(file, `"INTEG"`))
	})

	t.Run("add-duplicate-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"NAV"}, "scope": {"again"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "already exists")
	})

	t.Run("add-invalid-name-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		values := url.Values{"name": {"n0pe!"}, "scope": {"x"}}
		server.Handler().ServeHTTP(response, formPost("/context/aspects", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "2-12 letters")
		assert.NotContains(t, readAspectsFile(t, contextDir), "N0PE")
	})

	t.Run("add-empty-scope-rejected", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/context/aspects", url.Values{"name": {"PERF"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "scope is required")
	})

	t.Run("delete-unused-removes", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/INTEG", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, readAspectsFile(t, contextDir), "INTEG")
	})

	t.Run("delete-in-use-blocked-with-ids", func(t *testing.T) {
		contextDir := writeContextFixture(t)
		server := newTestServer(t, &Options{ContextDir: contextDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/NAV", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "in use: ACTION-NAV-001")
		assert.Contains(t, readAspectsFile(t, contextDir), "NAV")
	})

	t.Run("delete-unknown-400", func(t *testing.T) {
		server := newTestServer(t, &Options{ContextDir: writeContextFixture(t)})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/GHOST", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestContextDoc(t *testing.T) {
	server := newTestServer(t, &Options{ContextDir: writeContextFixture(t)})

	t.Run("valid-doc-renders-markdown", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/rules/go.md", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "<h1>Go Style</h1>")
		assert.Contains(t, response.Body.String(), "context/rules/go.md")
	})

	t.Run("unknown-file-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/rules/nope.md", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("on-disk-but-unscanned-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/rules/notes.txt", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.NotContains(t, response.Body.String(), "plain")
	})
}

func TestContextSync(t *testing.T) {
	target := t.TempDir()
	_, err := shell.Run(context.Background(), target, "git", "init", "-q")
	require.NoError(t, err)
	stateDir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	server := newTestServer(t, &Options{
		ContextDir: writeContextFixture(t),
		ReposPath:  writeRegistryFixture(t, target),
		StateDir:   stateDir,
	})

	t.Run("default-builds-the-record-only", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{"context-dir": {"docs"}, "name": {"demo"}}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "overlay.patch")
		assert.Contains(t, response.Body.String(), "not written (write to repo off)")
		assert.FileExists(t, contextdocs.DeployRecordPath(stateDir, "demo"))
		assert.NoFileExists(t, filepath.Join(target, "docs", "context.json"))
	})

	t.Run("write-to-repo-writes-the-two-files", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{"context-dir": {"docs"}, "name": {"demo"}, "write": {"on"}}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.FileExists(t, filepath.Join(target, "docs", "context.json"))
		assert.FileExists(t, filepath.Join(target, "docs", "aspects.json"))
		assert.NoFileExists(t, filepath.Join(target, "AGENTS.md"))
		index, err := contextdocs.LoadContextFile(filepath.Join(target, "docs", "context.json"))
		require.NoError(t, err)
		require.NotNil(t, index.Deploy)
		assert.True(t, index.Deploy.WriteRepo)
		assert.Len(t, index.Entries, 2)
	})

	t.Run("markdown-projects-the-tree", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{"context-dir": {"docs"}, "name": {"demo"}, "write": {"on"}, "markdown": {"on"}}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		require.Equal(t, http.StatusOK, response.Code)
		assert.FileExists(t, filepath.Join(target, "AGENTS.md"))
		assert.FileExists(t, filepath.Join(target, "docs", "actions", "navigation.md"))
	})

	t.Run("sub-option-without-write-is-bad-request", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{"context-dir": {"docs"}, "name": {"demo"}, "markdown": {"on"}}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("bad-context-dir-is-bad-request", func(t *testing.T) {
		response := httptest.NewRecorder()
		values := url.Values{"context-dir": {"../x"}, "name": {"demo"}}
		server.Handler().ServeHTTP(response, formPost("/context/sync", values))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("unknown-name-is-bad-request", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/context/sync", url.Values{"name": {"ghost"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestReachChipsSingletonsAndLists(t *testing.T) {
	assert.Equal(t, []string{reach.Global}, reachChips(reach.Global))
	assert.Equal(t, []string{reach.Always}, reachChips(reach.Always))
	assert.Equal(t, []string{"work", "peek-mcp"}, reachChips("work, peek-mcp"))
	assert.Nil(t, reachChips(reach.None))
}

func TestReachBarEntriesOrdersReservedFirst(t *testing.T) {
	entryCounts := map[string]int{
		"work":       1,
		reach.Always: 1,
		reach.Global: 1,
		"aqms":       1,
		"smine":      1,
	}
	chips := reachBarEntries(nil, entryCounts)
	names := make([]string, 0, len(chips))
	for _, chip := range chips {
		names = append(names, chip.Name)
	}
	assert.Equal(t, []string{reach.Global, reach.Always, "smine", "aqms", "work"}, names)
}
