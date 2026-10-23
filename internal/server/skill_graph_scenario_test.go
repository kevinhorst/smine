package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

// decodeScenario reads a scenario response body; a body that is not the
// scenario JSON decodes to the zero result.
func decodeScenario(response *httptest.ResponseRecorder) *skillmodel.ScenarioResult {
	result := &skillmodel.ScenarioResult{}
	if err := json.Unmarshal(response.Body.Bytes(), result); err != nil {
		return &skillmodel.ScenarioResult{}
	}
	return result
}

func TestHandleSkillGraphScenario(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedBody        string
		_expectedCode        int
		_expectedContentType string
		_expectedFlipped     bool
		_expectedOn          []string
		_expectedReplaced    map[string]string
		_expectedRound001    string
		_expectedRound002    string
		_expectedRouteState  string

		handler  http.Handler
		request  *http.Request
		response *httptest.ResponseRecorder
	}

	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	provideDemoGraphSkill(t, skillsRepo)

	// A model carrying a comment does not load.
	brokenDir := filepath.Join(skillsRepo, "broken-skill")
	writeSkillModel(t, brokenDir, "---\nname: broken-skill\ndescription: d\nversion: 1.0\n---\n\n# Broken\n", demoGraphChange)
	model, err := os.ReadFile(skillgraph.ModelPath(brokenDir))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(brokenDir), append([]byte("// note\n"), model...), 0o644))
	server := newTestServer(t, &Options{SkillsHome: filepath.Join(dir, "home"), SkillsRepo: skillsRepo})
	scenarioPath := "/scripts/skills/repo/" + demoGraphSkill + "/graph/scenario"
	nightlyRoute := "nightly-route"
	nightlyReplaced := map[string]string{"SKILL-DEMOSKILL-ROUND-001": nightlyRoute, "SKILL-DEMOSKILL-ROUND-002": nightlyRoute}
	flipNightly := url.Values{"flip": {"mode = nightly"}}.Encode()
	flipLateAtom := url.Values{"flip": {"late_atom"}}.Encode()

	tests := make([]*testCase, 0)

	// no-on
	tests = append(tests, &testCase{
		_id:                  "no-on",
		_expectedBody:        "",
		_expectedCode:        http.StatusOK,
		_expectedContentType: "application/json",
		_expectedFlipped:     false,
		_expectedOn:          nil,
		_expectedReplaced:    map[string]string{},
		_expectedRound001:    skillmodel.StatusUnknown,
		_expectedRound002:    skillmodel.StatusActive,
		_expectedRouteState:  skillmodel.StatusInactive,

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, scenarioPath, nil),
		response: httptest.NewRecorder(),
	})

	// on-nightly
	tests = append(tests, &testCase{
		_id:                  "on-nightly",
		_expectedBody:        "",
		_expectedCode:        http.StatusOK,
		_expectedContentType: "application/json",
		_expectedFlipped:     false,
		_expectedOn:          []string{"arg:nightly"},
		_expectedReplaced:    nightlyReplaced,
		_expectedRound001:    skillmodel.StatusInactive,
		_expectedRound002:    skillmodel.StatusInactive,
		_expectedRouteState:  skillmodel.StatusActive,

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, scenarioPath+"?on=arg:nightly", nil),
		response: httptest.NewRecorder(),
	})

	// flip-mode-nightly
	tests = append(tests, &testCase{
		_id:                  "flip-mode-nightly",
		_expectedBody:        "",
		_expectedCode:        http.StatusOK,
		_expectedContentType: "application/json",
		_expectedFlipped:     true,
		_expectedOn:          []string{"arg:nightly"},
		_expectedReplaced:    nightlyReplaced,
		_expectedRound001:    skillmodel.StatusInactive,
		_expectedRound002:    skillmodel.StatusInactive,
		_expectedRouteState:  skillmodel.StatusActive,

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, scenarioPath+"?"+flipNightly, nil),
		response: httptest.NewRecorder(),
	})

	// flip-late-atom
	tests = append(tests, &testCase{
		_id:                  "flip-late-atom",
		_expectedBody:        "",
		_expectedCode:        http.StatusOK,
		_expectedContentType: "application/json",
		_expectedFlipped:     false,
		_expectedOn:          nil,
		_expectedReplaced:    map[string]string{},
		_expectedRound001:    skillmodel.StatusUnknown,
		_expectedRound002:    skillmodel.StatusActive,
		_expectedRouteState:  skillmodel.StatusInactive,

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, scenarioPath+"?"+flipLateAtom, nil),
		response: httptest.NewRecorder(),
	})

	// unknown-on-ignored
	tests = append(tests, &testCase{
		_id:                  "unknown-on-ignored",
		_expectedBody:        "",
		_expectedCode:        http.StatusOK,
		_expectedContentType: "application/json",
		_expectedFlipped:     false,
		_expectedOn:          nil,
		_expectedReplaced:    map[string]string{},
		_expectedRound001:    skillmodel.StatusUnknown,
		_expectedRound002:    skillmodel.StatusActive,
		_expectedRouteState:  skillmodel.StatusInactive,

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, scenarioPath+"?on=arg:bogus", nil),
		response: httptest.NewRecorder(),
	})

	// unknown-skill-404
	tests = append(tests, &testCase{
		_id:                  "unknown-skill-404",
		_expectedBody:        "",
		_expectedCode:        http.StatusNotFound,
		_expectedContentType: "text/plain; charset=utf-8",
		_expectedFlipped:     false,
		_expectedOn:          nil,
		_expectedReplaced:    nil,
		_expectedRound001:    "",
		_expectedRound002:    "",
		_expectedRouteState:  "",

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/no-such-skill/graph/scenario", nil),
		response: httptest.NewRecorder(),
	})

	// broken-model-409
	tests = append(tests, &testCase{
		_id:                  "broken-model-409",
		_expectedBody:        "comment",
		_expectedCode:        http.StatusConflict,
		_expectedContentType: "text/plain; charset=utf-8",
		_expectedFlipped:     false,
		_expectedOn:          nil,
		_expectedReplaced:    nil,
		_expectedRound001:    "",
		_expectedRound002:    "",
		_expectedRouteState:  "",

		handler:  server.Handler(),
		request:  httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/broken-skill/graph/scenario", nil),
		response: httptest.NewRecorder(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.handler.ServeHTTP(test.response, test.request)
			result := decodeScenario(test.response)
			assert.Equal(t, test._expectedCode, test.response.Code)
			assert.Equal(t, test._expectedContentType, test.response.Header().Get("Content-Type"))
			assert.Contains(t, test.response.Body.String(), test._expectedBody)
			assert.Equal(t, test._expectedFlipped, result.Flipped)
			assert.Equal(t, test._expectedOn, result.On)
			assert.Equal(t, test._expectedReplaced, result.Replaced)
			assert.Equal(t, test._expectedRound001, result.Entries["SKILL-DEMOSKILL-ROUND-001"])
			assert.Equal(t, test._expectedRound002, result.Entries["SKILL-DEMOSKILL-ROUND-002"])
			assert.Equal(t, test._expectedRouteState, result.States[nightlyRoute])
		})
	}
}
