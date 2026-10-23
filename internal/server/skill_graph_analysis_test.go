package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

// graphFixture is one server over a fresh demo skill: the repo root and the
// skill dir.
type graphFixture struct {
	server     *Server
	skillDir   string
	skillsRepo string
}

// newGraphFixture writes the demo skill, a deployed home copy with a model
// and the plain annotation-tier skill, and starts a server over them.
func newGraphFixture(t *testing.T) *graphFixture {
	t.Helper()
	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillDir := provideDemoGraphSkill(t, skillsRepo)
	providePlainSkill(t, skillsRepo)
	skillsHome := filepath.Join(dir, "home")
	homeModel, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(provideDeployedSkill(t, skillDir, skillsHome), homeModel))
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})
	return &graphFixture{server: server, skillDir: skillDir, skillsRepo: skillsRepo}
}

func TestSplitFindings(t *testing.T) {
	errorFinding := skillmodel.Finding{Check: "c", Message: "m", Severity: skillmodel.SeverityError}
	warningFinding := skillmodel.Finding{Check: "c", Message: "m", Severity: skillmodel.SeverityWarning}

	type testCase struct {
		_id               string
		_expectedBlocking []skillmodel.Finding
		_expectedWarnings []skillmodel.Finding

		findings []skillmodel.Finding
	}

	tests := make([]*testCase, 0)

	// error-blocks
	tests = append(tests, &testCase{_id: "error-blocks", _expectedBlocking: []skillmodel.Finding{errorFinding}, _expectedWarnings: nil, findings: []skillmodel.Finding{errorFinding}})

	// warning-warns
	tests = append(tests, &testCase{_id: "warning-warns", _expectedBlocking: nil, _expectedWarnings: []skillmodel.Finding{warningFinding}, findings: []skillmodel.Finding{warningFinding}})

	// mixed
	tests = append(tests, &testCase{_id: "mixed", _expectedBlocking: []skillmodel.Finding{errorFinding}, _expectedWarnings: []skillmodel.Finding{warningFinding}, findings: []skillmodel.Finding{warningFinding, errorFinding}})

	// empty
	tests = append(tests, &testCase{_id: "empty", _expectedBlocking: nil, _expectedWarnings: nil, findings: nil})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			blocking, warnings := splitFindings(test.findings)
			assert.Equal(t, test._expectedBlocking, blocking)
			assert.Equal(t, test._expectedWarnings, warnings)
		})
	}
}

func TestGraphRemovedRoutes(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedCode int

		handler  http.Handler
		request  *http.Request
		response *httptest.ResponseRecorder
	}

	fixture := newGraphFixture(t)
	graphPath := "/scripts/skills/repo/" + demoGraphSkill + "/graph"

	tests := make([]*testCase, 0)

	// preview-404
	tests = append(tests, &testCase{
		_id:           "preview-404",
		_expectedCode: http.StatusNotFound,

		handler:  fixture.server.Handler(),
		request:  httptest.NewRequest(http.MethodPost, graphPath+"/preview", strings.NewReader("{}")),
		response: httptest.NewRecorder(),
	})

	// save-404
	tests = append(tests, &testCase{
		_id:           "save-404",
		_expectedCode: http.StatusNotFound,

		handler:  fixture.server.Handler(),
		request:  httptest.NewRequest(http.MethodPost, graphPath+"/save", strings.NewReader("{}")),
		response: httptest.NewRecorder(),
	})

	// query-404
	tests = append(tests, &testCase{
		_id:           "query-404",
		_expectedCode: http.StatusNotFound,

		handler:  fixture.server.Handler(),
		request:  httptest.NewRequest(http.MethodPost, graphPath+"/query", strings.NewReader("{}")),
		response: httptest.NewRecorder(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.handler.ServeHTTP(test.response, test.request)
			assert.Equal(t, test._expectedCode, test.response.Code)
		})
	}
}

func TestHandleSkillGraphFindings(t *testing.T) {
	get := func(t *testing.T, fixture *graphFixture) string {
		t.Helper()
		response := httptest.NewRecorder()
		fixture.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo-skill/graph/findings", nil))
		require.Equal(t, http.StatusOK, response.Code)
		return response.Body.String()
	}

	t.Run("no-z3-unavailable", func(t *testing.T) {
		fixture := newGraphFixture(t)
		t.Setenv("PATH", t.TempDir())
		assert.Contains(t, get(t, fixture), "battery unavailable")
	})

	t.Run("clean-skill", func(t *testing.T) {
		requireZ3(t)
		body := get(t, newGraphFixture(t))
		assert.Contains(t, body, "battery:")
		assert.Contains(t, body, `class="st-findings-data"`)
		assert.NotContains(t, body, "<details")
	})
}

func TestHandleSkillGraphDiff(t *testing.T) {
	get := func(t *testing.T, fixture *graphFixture) string {
		t.Helper()
		response := httptest.NewRecorder()
		fixture.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo-skill/graph/diff", nil))
		require.Equal(t, http.StatusOK, response.Code)
		return response.Body.String()
	}
	git := func(t *testing.T, dir string, args ...string) {
		t.Helper()
		base := []string{"-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}
		_, err := shell.Run(context.Background(), dir, "git", append(base, args...)...)
		require.NoError(t, err)
	}
	commit := func(t *testing.T, fixture *graphFixture) {
		t.Helper()
		git(t, fixture.skillsRepo, "init", "-q")
		git(t, fixture.skillsRepo, "add", ".")
		git(t, fixture.skillsRepo, "commit", "-q", "-m", "fixture")
	}

	t.Run("no-head-note", func(t *testing.T) {
		assert.Contains(t, get(t, newGraphFixture(t)), "no git HEAD")
	})

	t.Run("model-files-only", func(t *testing.T) {
		fixture := newGraphFixture(t)
		commit(t, fixture)
		skill, err := skillgraph.Load(fixture.skillDir)
		require.NoError(t, err)
		ref, found := skill.Entry("SKILL-DEMOSKILL-ROUND-004")
		require.True(t, found)
		ref.Entry.Title = "Late grant"
		require.NoError(t, skillgraph.Save(fixture.skillDir, skill))
		body := get(t, fixture)
		assert.Contains(t, body, "Late grant")
		assert.Contains(t, body, skillgraph.ModelFile(filepath.Base(fixture.skillDir)))
		assert.NotContains(t, body, ">SKILL.md<")
	})

	t.Run("no-stale-card", func(t *testing.T) {
		fixture := newGraphFixture(t)
		commit(t, fixture)
		require.NoError(t, os.WriteFile(filepath.Join(fixture.skillDir, skillgraph.FileManifest), []byte("hand edit\n"), 0o644))
		body := get(t, fixture)
		assert.NotContains(t, body, "st-stale")
		assert.NotContains(t, body, "hand edit")
		assert.Contains(t, body, "no change")
	})
}

func TestSkillGraphFindingsPartial(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedListItem string
		_expectedSummary  string
		_hasFindingList   bool
		_isErrorMarked    bool

		response *httptest.ResponseRecorder
		view     *graphFindingsView
	}

	fixture := newGraphFixture(t)
	entryError := skillmodel.Finding{Check: "consistency", EntryIds: []string{"SKILL-DEMOSKILL-PROC-001"}, Message: "no run satisfies the model", Severity: skillmodel.SeverityError}
	entryErrorItem := `<li class="st-finding-error"><code>consistency</code> no run satisfies the model <button type="button" class="st-subst-link" data-open-entry="SKILL-DEMOSKILL-PROC-001">SKILL-DEMOSKILL-PROC-001</button></li>`
	orphanError := skillmodel.Finding{Check: "consistency", Message: "the preds contradict each other", Severity: skillmodel.SeverityError}

	tests := make([]*testCase, 0)

	// clean
	tests = append(tests, &testCase{
		_id:               "clean",
		_expectedListItem: "",
		_expectedSummary:  "battery: clean",
		_hasFindingList:   false,
		_isErrorMarked:    false,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{},
	})

	// warnings-counted-not-listed
	tests = append(tests, &testCase{
		_id:               "warnings-counted-not-listed",
		_expectedListItem: "",
		_expectedSummary:  "battery: 0 error(s) · 2 warning(s)",
		_hasFindingList:   false,
		_isErrorMarked:    false,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{WarningCount: 2},
	})

	// errors-listed
	tests = append(tests, &testCase{
		_id:               "errors-listed",
		_expectedListItem: entryErrorItem,
		_expectedSummary:  "battery: 1 error(s) · 0 warning(s)",
		_hasFindingList:   true,
		_isErrorMarked:    true,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{Errors: []skillmodel.Finding{entryError}},
	})

	// error-without-entry-listed
	tests = append(tests, &testCase{
		_id:               "error-without-entry-listed",
		_expectedListItem: `<li class="st-finding-error"><code>consistency</code> the preds contradict each other</li>`,
		_expectedSummary:  "battery: 1 error(s) · 0 warning(s)",
		_hasFindingList:   true,
		_isErrorMarked:    true,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{Errors: []skillmodel.Finding{orphanError}},
	})

	// mixed
	tests = append(tests, &testCase{
		_id:               "mixed",
		_expectedListItem: entryErrorItem,
		_expectedSummary:  "battery: 1 error(s) · 1 warning(s)",
		_hasFindingList:   true,
		_isErrorMarked:    true,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{Errors: []skillmodel.Finding{entryError}, WarningCount: 1},
	})

	// unavailable
	tests = append(tests, &testCase{
		_id:               "unavailable",
		_expectedListItem: "",
		_expectedSummary:  "battery unavailable — z3 not found on PATH",
		_hasFindingList:   false,
		_isErrorMarked:    false,

		response: httptest.NewRecorder(),
		view:     &graphFindingsView{Error: "z3 not found on PATH"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fixture.server.renderFragment(test.response, tmplSkillGraphFindings, test.view)
			assert.Equal(t, http.StatusOK, test.response.Code)
			assert.Contains(t, test.response.Body.String(), test._expectedSummary)
			assert.Contains(t, test.response.Body.String(), test._expectedListItem)
			assert.Equal(t, test._hasFindingList, strings.Contains(test.response.Body.String(), "st-finding-list"))
			assert.Equal(t, test._isErrorMarked, strings.Contains(test.response.Body.String(), "st-battery-error"))
			assert.NotContains(t, test.response.Body.String(), "<details")
			assert.NotContains(t, test.response.Body.String(), "st-finding-warning")
		})
	}
}
