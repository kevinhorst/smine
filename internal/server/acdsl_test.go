package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acdslTestDeclarations is assembled so no source line starts with the live
// marker — DiscoverRules scans .go files line-wise, string literals included.
var acdslTestDeclarations = "//" + `acdsl:TSV-001 gofmt anchor="\.go$" why="fixture rule (ACTION-IMPL-INTEG-005)"` + "\n" +
	"//" + `acdsl:TSV-002 gofmt anchor="\.go$" lifetime="task" why="task fixture"` + "\n" +
	"//" + `acdsl:TSV-003 gofmt reach="global" anchor="\.go$" why="global fixture"` + "\n"

// newAcdslTestRepo materializes a minimal git repo carrying one doctrine and
// one task rule plus a pack registry, and chdirs into it — the acdsl
// handlers resolve everything against the working directory.
func newAcdslTestRepo(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "rules.acdsl"), []byte(acdslTestDeclarations), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "registry.json"), []byte(`{"gofmt": {"argv": ["true"], "timeout_s": 5, "description": "noop"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "context", "context.json"), []byte(`{"entries": [{"id": "ACTION-IMPL-INTEG-005"}, {"id": "ACTION-IMPL-INTEG-001"}]}`), 0o644))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), root, "git", args...)
		require.NoError(t, err)
	}
	t.Chdir(root)
	return newTestServer(t, &Options{
		ContextDir:        "context",
		AcdslVerdictsPath: filepath.Join(root, "verdicts.jsonl"),
	})
}

func TestContextAcdslRendersRulesAndCoverage(t *testing.T) {
	server := newAcdslTestRepo(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "TSV-001")
	assert.Contains(t, body, "coverage")
	assert.Contains(t, body, "(1 of 2 entries)")
	assert.Contains(t, body, "no runs")
}

func TestContextAcdslReachFilterNarrowsRows(t *testing.T) {
	server := newAcdslTestRepo(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?reach=global", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "TSV-003")
	assert.NotContains(t, body, `>TSV-001<`)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?reach=smine", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, `>TSV-001<`)
	assert.NotContains(t, body, `>TSV-003<`)
}

func TestAcdslToggleRouteGone(t *testing.T) {
	server := newAcdslTestRepo(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/context/acdsl/rules/TSV-001/toggle", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestAcdslStatsGroupsDeliveryArms(t *testing.T) {
	server := newAcdslTestRepo(t)
	records := strings.Join([]string{
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "violations", "rules": [{"id": "TSV-001", "projected": true, "violations": 2}]}`,
		`{"ts": "2099-01-01T01:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": false, "violations": 0}]}`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(server.acdslVerdictsPath, []byte(records), 0o644))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?since=all", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "while projected: red <strong>100%</strong> (1) · 1 runs")
	assert.Contains(t, body, "while gate-only: red <strong>0%</strong> (0) · 1 runs")
}

func seedVerdicts(t *testing.T, server *Server, lines []string) {
	t.Helper()
	require.NoError(t, os.WriteFile(server.acdslVerdictsPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))
}

func getAcdsl(t *testing.T, server *Server) string {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context?since=all", nil))
	require.Equal(t, http.StatusOK, response.Code)
	return response.Body.String()
}

func TestAcdslEvictionCandidate(t *testing.T) {
	server := newAcdslTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join("acdsl", "testdata", "TSV-001"), 0o755))

	// Eviction demands a long, entirely clean projected window: any red is
	// proof the projection earns its tokens.
	var clean []string
	for range 300 {
		clean = append(clean, `{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": true, "violations": 0}]}`)
	}
	seedVerdicts(t, server, clean)
	assert.Contains(t, getAcdsl(t, server), "eviction candidate")

	// Below the run threshold: no flag.
	seedVerdicts(t, server, clean[:299])
	assert.NotContains(t, getAcdsl(t, server), "eviction candidate")

	// A single red in the window keeps the projection, whatever the rate.
	oneRed := append(clean[:299],
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "violations", "rules": [{"id": "TSV-001", "projected": true, "violations": 1}]}`)
	seedVerdicts(t, server, oneRed)
	assert.NotContains(t, getAcdsl(t, server), "eviction candidate")
}

func TestAcdslEvictionCandidateRequiresFixtures(t *testing.T) {
	server := newAcdslTestRepo(t)

	var clean []string
	for range 300 {
		clean = append(clean, `{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": true, "violations": 0}]}`)
	}
	seedVerdicts(t, server, clean)
	assert.NotContains(t, getAcdsl(t, server), "eviction candidate")
}

func TestContextRuleReachHandler(t *testing.T) {
	server := newAcdslTestRepo(t)
	post := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/context/rule/TSV-001/reach", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}
	rulesFile := func() string {
		data, err := os.ReadFile(filepath.Join("acdsl", "rules.acdsl"))
		require.NoError(t, err)
		return string(data)
	}

	// Named reach: attr inserted, 303 back to the detail page.
	response := post("reach=aqms,peek-mcp")
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/context/rule/TSV-001", response.Header().Get("Location"))
	assert.Contains(t, rulesFile(), `acdsl:TSV-001 gofmt reach="aqms,peek-mcp" anchor`)

	// Empty input disables: reach rewritten to none in place.
	response = post("reach=")
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Contains(t, rulesFile(), `acdsl:TSV-001 gofmt reach="none" anchor`)

	// Invalid value: detail page rendered with the error, file untouched.
	response = post("reach=aqms,global")
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "invalid reach")
	assert.Contains(t, rulesFile(), `reach="none"`)

	// Unknown rule id: 404.
	request := httptest.NewRequest(http.MethodPost, "/context/rule/TSV-999/reach", strings.NewReader("reach=smine"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestContextEntryReachHandler(t *testing.T) {
	server := newAcdslTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join("context", "facts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("context", "facts", "repo.md"),
		[]byte("**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: global\n"), 0o644))

	request := httptest.NewRequest(http.MethodPost, "/context/entry/FACT-REPO-STACK-001/reach", strings.NewReader("reach=smine"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/context/entry/FACT-REPO-STACK-001", response.Header().Get("Location"))
	source, err := os.ReadFile(filepath.Join("context", "facts", "repo.md"))
	require.NoError(t, err)
	assert.Contains(t, string(source), "* Reach: smine")
	// The handler regenerates context.json from the markdown sources.
	generated, err := os.ReadFile(filepath.Join("context", "context.json"))
	require.NoError(t, err)
	assert.Contains(t, string(generated), `"reach": "smine"`)
}

func TestBuildReachOptionsMultiSelect(t *testing.T) {
	server := newAcdslTestRepo(t)

	selected := func(options []reachOption) []string {
		var names []string
		for _, option := range options {
			if option.Selected {
				names = append(names, option.Value)
			}
		}
		return names
	}

	// A list value marks each member; an unknown name is appended checked so
	// it survives the next save.
	options := server.buildReachOptions("smine,unregistered")
	assert.ElementsMatch(t, []string{"smine", "unregistered"}, selected(options))

	// The singleton values still mark exactly themselves.
	assert.Equal(t, []string{reach.None}, selected(server.buildReachOptions(reach.None)))
	assert.Equal(t, []string{reach.Global}, selected(server.buildReachOptions(reach.Global)))
}

func TestContextReachBulkHandler(t *testing.T) {
	server := newAcdslTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join("context", "facts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("context", "facts", "repo.md"),
		[]byte("**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: global\n"), 0o644))
	post := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/context/reach/bulk", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response
	}

	// Mixed ids: the rule's marker attr and the entry's Reach bullet both
	// rewrite to the joined list; context.json regenerates.
	response := post("id=TSV-001&id=FACT-REPO-STACK-001&reach=aqms&reach=peek-mcp&aspect=TSV")
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/context?aspect=TSV", response.Header().Get("Location"))
	rules, err := os.ReadFile(filepath.Join("acdsl", "rules.acdsl"))
	require.NoError(t, err)
	assert.Contains(t, string(rules), `acdsl:TSV-001 gofmt reach="aqms,peek-mcp" anchor`)
	source, err := os.ReadFile(filepath.Join("context", "facts", "repo.md"))
	require.NoError(t, err)
	assert.Contains(t, string(source), "* Reach: aqms,peek-mcp")
	generated, err := os.ReadFile(filepath.Join("context", "context.json"))
	require.NoError(t, err)
	assert.Contains(t, string(generated), `"reach": "aqms,peek-mcp"`)

	// Invalid reach or no selection: redirect back with the error, no writes.
	response = post("id=TSV-001&reach=global&reach=aqms")
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Contains(t, response.Header().Get("Location"), "err=")
	response = post("reach=global")
	require.Equal(t, http.StatusSeeOther, response.Code)
	assert.Contains(t, response.Header().Get("Location"), "err=")
	rules, err = os.ReadFile(filepath.Join("acdsl", "rules.acdsl"))
	require.NoError(t, err)
	assert.Contains(t, string(rules), `reach="aqms,peek-mcp"`)

	// An id that is neither a discovered rule nor a known entry: 500, same
	// as the single-entry handler's unknown-entry path.
	response = post("id=TSV-999&reach=global")
	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestAcdslDisabledRuleShowsBadgeAndNoCandidate(t *testing.T) {
	server := newAcdslTestRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join("acdsl", "testdata", "TSV-001"), 0o755))
	disabled := strings.Replace(acdslTestDeclarations,
		`acdsl:TSV-001 gofmt anchor`, `acdsl:TSV-001 gofmt reach="none" anchor`, 1)
	require.NoError(t, os.WriteFile(filepath.Join("acdsl", "rules.acdsl"), []byte(disabled), 0o644))

	var clean []string
	for range 300 {
		clean = append(clean, `{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": true, "violations": 0}]}`)
	}
	seedVerdicts(t, server, clean)

	body := getAcdsl(t, server)
	assert.Contains(t, body, "disabled")
	assert.NotContains(t, body, "eviction candidate")
}

func TestAcdslReprojectionCandidate(t *testing.T) {
	server := newAcdslTestRepo(t)
	flipped := strings.Replace(acdslTestDeclarations,
		`why="fixture rule (ACTION-IMPL-INTEG-005)"`,
		`why="fixture rule (ACTION-IMPL-INTEG-005)" projected="false"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join("acdsl", "rules.acdsl"), []byte(flipped), 0o644))

	// A single red in the gate-only arm is enough — any failure while
	// unprojected warrants projecting the prose back.
	runs := []string{
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": false, "violations": 0}]}`,
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": false, "violations": 0}]}`,
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": false, "violations": 0}]}`,
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": false, "violations": 0}]}`,
		`{"ts": "2099-01-01T00:00:00Z", "outcome": "violations", "rules": [{"id": "TSV-001", "projected": false, "violations": 1}]}`,
	}
	seedVerdicts(t, server, runs)
	assert.Contains(t, getAcdsl(t, server), "re-projection candidate")

	// All-clean gate-only arm: nothing to re-project for.
	seedVerdicts(t, server, runs[:4])
	assert.NotContains(t, getAcdsl(t, server), "re-projection candidate")
}

func TestAcdslRedRunsOnRuleDetail(t *testing.T) {
	// The list page no longer renders a recent-runs section; the run data
	// feeds the rule detail's red-runs list.
	server := newAcdslTestRepo(t)
	seedVerdicts(t, server, []string{
		`{"ts": "2099-01-01T00:00:00Z", "branch": "claude/older", "outcome": "clean", "rules": [{"id": "TSV-001", "projected": true, "violations": 0}]}`,
		`{"ts": "2099-01-02T00:00:00Z", "branch": "claude/newer", "outcome": "violations", "rules": [{"id": "TSV-001", "projected": true, "violations": 2}]}`,
	})
	body := getAcdsl(t, server)
	assert.NotContains(t, body, "recent gate runs")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/rule/TSV-001", nil))
	require.Equal(t, http.StatusOK, response.Code)
	detail := response.Body.String()
	assert.Contains(t, detail, "red runs")
	assert.Contains(t, detail, "claude/newer")
	assert.NotContains(t, detail, "claude/older")
}

func TestAspectDeleteBlockedByAcdslFamily(t *testing.T) {
	contextDir := writeContextFixture(t)
	server := newAcdslTestRepo(t)
	server.contextDir = contextDir

	withFamily := acdslTestDeclarations +
		"//" + `acdsl:ACDSL-INTEG-001 gofmt anchor="\.go$" why="family fixture"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join("acdsl", "rules.acdsl"), []byte(withFamily), 0o644))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/context/aspects/INTEG", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "in use: ACDSL-INTEG-001")
	raw, err := os.ReadFile(filepath.Join(contextDir, "context.json"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "INTEG")
}

func TestBuildContextRulePageVerifier(t *testing.T) {
	server := newAcdslTestRepo(t)

	data, found, err := server.buildContextRulePage(context.Background(), "TSV-001")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, data.VerifierFound)
	assert.Equal(t, "gofmt", data.Row.Rule.Verifier)
	assert.Equal(t, []string{"true"}, data.Verifier.Argv)
	assert.Equal(t, "noop", data.Verifier.Description)

	_, found, err = server.buildContextRulePage(context.Background(), "NOPE-999")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestContextRuleShowsVerifier(t *testing.T) {
	server := newAcdslTestRepo(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/context/rule/TSV-001", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "noop")       // registry description
	assert.Contains(t, body, "timeout 5s") // registry timeout
	assert.Contains(t, body, `id="rule-fixtures"`)
}

func TestContextRuleFixturesRunNoFixtures(t *testing.T) {
	server := newAcdslTestRepo(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/context/rule/TSV-001/fixtures/run", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()
	assert.Contains(t, body, `id="rule-fixtures"`)
	assert.Contains(t, body, "no fixtures for this rule")
}
