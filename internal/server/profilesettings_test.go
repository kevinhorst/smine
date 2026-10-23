package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newProfileServer builds a server whose presentation profile starts as
// existing (empty = no file, the developer default).
func newProfileServer(t *testing.T, existing string) *Server {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "presentation-profile.md")
	if existing != "" {
		require.NoError(t, os.WriteFile(path, []byte(existing), 0o644))
	}
	return newTestServer(t, &Options{
		PresentationPath: path,
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
}

func TestServer_HandleProfilePage(t *testing.T) {
	server := newProfileServer(t, "")

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Profile")
	assert.Contains(t, body, "style_content")
	assert.Contains(t, body, "set at install")
	assert.NotContains(t, body, `name="audience"`)
	assert.NotContains(t, body, `name="proposals"`)
	assert.Contains(t, body, `hx-post="/profile/test"`)
	assert.NotRegexp(t, `name="dev_mode"[^>]*disabled`, body)
	assert.NotContains(t, body, `name="plan"`)
	assert.Contains(t, body, `hx-post="/profile/tokens"`)
	assert.Contains(t, body, `name="presentation_content"`)
	assert.Less(t, strings.Index(body, `class="meta profile-path"`), strings.Index(body, `name="language"`))
}

func TestServer_HandleProfilePageCasual(t *testing.T) {
	server := newProfileServer(t, "---\nlanguage: de\naudience: casual\n---\n")

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Regexp(t, `name="dev_mode"[^>]*disabled`, recorder.Body.String())
}

func TestServer_HandleProfileTestWithoutToken(t *testing.T) {
	options := &Options{TokenDir: filepath.Join(t.TempDir(), "tokens")}
	server := newTestServer(t, options)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/profile/test", url.Values{}))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `<span class="badge badge-error">failed</span>`)
	assert.Contains(t, body, "no token file to verify — save one first")
	assert.Contains(t, body, smineNightlyModel+" · token file")
}

func TestServer_StyleTestTarget(t *testing.T) {
	type testCase struct {
		_id       string
		_expected styleTestTarget

		server *Server
	}

	tokenDirWith := func(labelFile string, labelValue string, legacyValue string) string {
		tokenDir := filepath.Join(t.TempDir(), "claude-routine", "tokens")
		require.NoError(t, os.MkdirAll(tokenDir, 0o700))
		if legacyValue != "" {
			require.NoError(t, os.WriteFile(routines.LegacyTokenPath(tokenDir), []byte(legacyValue), 0o600))
		}
		if labelFile != "" {
			require.NoError(t, os.WriteFile(filepath.Join(tokenDir, labelFile), []byte(labelValue), 0o600))
		}
		return tokenDir
	}
	routinesWith := func(envKey string, envValue string, routineName string) string {
		routinesDir := t.TempDir()
		routineDir := filepath.Join(routinesDir, routineName)
		require.NoError(t, os.MkdirAll(routineDir, 0o755))
		manifest := []byte(`{"kind":"smine-nightly","prompt":"/smine --nightly"}`)
		require.NoError(t, os.WriteFile(filepath.Join(routineDir, "routine.json"), manifest, 0o644))
		plist := []byte(testPlistWithEnv(routineDir, envKey, envValue))
		require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine-handler-test.plist"), plist, 0o644))
		return routinesDir
	}
	serverWith := func(routinesDir string, tokenDir string) *Server {
		options := &Options{RoutinesDir: routinesDir, TokenDir: tokenDir}
		return newTestServer(t, options)
	}
	tests := make([]*testCase, 0)

	// no-token-no-routine
	tokenDir := tokenDirWith("", "", "")
	tests = append(tests, &testCase{
		_id: "no-token-no-routine",
		_expected: styleTestTarget{
			Detail:    "no token file to verify — save one first",
			Model:     smineNightlyModel,
			TokenName: "token file",
			TokenPath: "",
		},

		server: serverWith(t.TempDir(), tokenDir),
	})

	// legacy-token-no-routine
	tokenDir = tokenDirWith("", "", "sk-legacy")
	tests = append(tests, &testCase{
		_id: "legacy-token-no-routine",
		_expected: styleTestTarget{
			Detail:    "",
			Model:     smineNightlyModel,
			TokenName: "token file",
			TokenPath: routines.LegacyTokenPath(tokenDir),
		},

		server: serverWith(t.TempDir(), tokenDir),
	})

	// routine-model-override
	tokenDir = tokenDirWith("", "", "sk-legacy")
	tests = append(tests, &testCase{
		_id: "routine-model-override",
		_expected: styleTestTarget{
			Detail:    "",
			Model:     "claude-sonnet-5",
			TokenName: "token file",
			TokenPath: routines.LegacyTokenPath(tokenDir),
		},

		server: serverWith(routinesWith("ROUTINE_MODEL", "claude-sonnet-5", nightlyRoutineName), tokenDir),
	})

	// routine-labeled-token
	tokenDir = tokenDirWith("work", "sk-work", "")
	tests = append(tests, &testCase{
		_id: "routine-labeled-token",
		_expected: styleTestTarget{
			Detail:    "",
			Model:     smineNightlyModel,
			TokenName: "work",
			TokenPath: filepath.Join(tokenDir, "work"),
		},

		server: serverWith(routinesWith("ROUTINE_TOKEN", "work", nightlyRoutineName), tokenDir),
	})

	// routine-label-missing-file — the default pick stays, here: no token
	tokenDir = tokenDirWith("", "", "")
	tests = append(tests, &testCase{
		_id: "routine-label-missing-file",
		_expected: styleTestTarget{
			Detail:    "no token file to verify — save one first",
			Model:     smineNightlyModel,
			TokenName: "token file",
			TokenPath: "",
		},

		server: serverWith(routinesWith("ROUTINE_TOKEN", "gone", nightlyRoutineName), tokenDir),
	})

	// routine-label-empty-file — an empty labeled file falls back to the legacy file
	tokenDir = tokenDirWith("work", "", "sk-legacy")
	tests = append(tests, &testCase{
		_id: "routine-label-empty-file",
		_expected: styleTestTarget{
			Detail:    "",
			Model:     smineNightlyModel,
			TokenName: "token file",
			TokenPath: routines.LegacyTokenPath(tokenDir),
		},

		server: serverWith(routinesWith("ROUTINE_TOKEN", "work", nightlyRoutineName), tokenDir),
	})

	// other-routine-ignored — only smine-nightly configures the pick
	tokenDir = tokenDirWith("", "", "sk-legacy")
	tests = append(tests, &testCase{
		_id: "other-routine-ignored",
		_expected: styleTestTarget{
			Detail:    "",
			Model:     smineNightlyModel,
			TokenName: "token file",
			TokenPath: routines.LegacyTokenPath(tokenDir),
		},

		server: serverWith(routinesWith("ROUTINE_MODEL", "claude-sonnet-5", "demo"), tokenDir),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.server.styleTestTarget())
		})
	}
}

func TestProfileTestAnswer(t *testing.T) {
	type testCase struct {
		_id       string
		_expected profileTestView

		output string
	}

	unreadable := profileTestView{Detail: "claude answered, but the response envelope was unreadable"}
	tests := make([]*testCase, 0)

	// no-json
	tests = append(tests, &testCase{
		_id:       "no-json",
		_expected: unreadable,

		output: "bash: claude: command not found",
	})

	// malformed-json
	tests = append(tests, &testCase{
		_id:       "malformed-json",
		_expected: unreadable,

		output: `{"is_error": false, "result": `,
	})

	// claude-error
	tests = append(tests, &testCase{
		_id:       "claude-error",
		_expected: profileTestView{Detail: "claude reported an error: Credit balance is too low"},

		output: `{"is_error": true, "result": "Credit balance is too low"}`,
	})

	// answer-after-preamble — log lines before the envelope are skipped
	tests = append(tests, &testCase{
		_id:       "answer-after-preamble",
		_expected: profileTestView{Answer: "Gestern wurden 12 Sitzungen ausgewertet.", Ok: true},

		output: "warning: slow start\n" + `{"is_error": false, "result": "Gestern wurden 12 Sitzungen ausgewertet."}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, profileTestAnswer(test.output))
		})
	}
}

func TestServer_HandleProfileSave(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAudience string
		_expectedDevMode  bool
		_expectedStatus   int
		_shouldPersist    bool

		form   url.Values
		server *Server
	}

	casualProfile := "---\nlanguage: de\naudience: casual\n---\n"
	tests := make([]*testCase, 0)

	// pass-valid-developer
	tests = append(tests, &testCase{
		_id:               "pass-valid-developer",
		_expectedAudience: "",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"language": {"en"}, "auto_merge": {"on"}},
		server: newProfileServer(t, ""),
	})

	// pass-developer-automerge-off — a non-default developer combo persists
	tests = append(tests, &testCase{
		_id:               "pass-developer-automerge-off",
		_expectedAudience: "",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"language": {"en"}},
		server: newProfileServer(t, ""),
	})

	// pass-valid-casual — a casual install keeps its audience
	tests = append(tests, &testCase{
		_id:               "pass-valid-casual",
		_expectedAudience: "casual",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"language": {"de"}},
		server: newProfileServer(t, casualProfile),
	})

	// pass-developer-devmode
	tests = append(tests, &testCase{
		_id:               "pass-developer-devmode",
		_expectedAudience: "",
		_expectedDevMode:  true,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"language": {"en"}, "dev_mode": {"on"}, "auto_merge": {"on"}},
		server: newProfileServer(t, ""),
	})

	// pass-casual-devmode-forced-off
	tests = append(tests, &testCase{
		_id:               "pass-casual-devmode-forced-off",
		_expectedAudience: "casual",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"language": {"de"}, "dev_mode": {"on"}},
		server: newProfileServer(t, casualProfile),
	})

	// pass-posted-audience-ignored — the audience is fixed at install (D2)
	tests = append(tests, &testCase{
		_id:               "pass-posted-audience-ignored",
		_expectedAudience: "",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusOK,
		_shouldPersist:    true,

		form:   url.Values{"audience": {"casual"}, "language": {"en"}, "auto_merge": {"on"}},
		server: newProfileServer(t, ""),
	})

	// fail-unsupported-language
	tests = append(tests, &testCase{
		_id:               "fail-unsupported-language",
		_expectedAudience: "",
		_expectedDevMode:  false,
		_expectedStatus:   http.StatusBadRequest,
		_shouldPersist:    false,

		form:   url.Values{"language": {"fr"}},
		server: newProfileServer(t, ""),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/profile", test.form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			_, statErr := os.Stat(test.server.presentation.path)
			assert.Equal(t, test._shouldPersist, statErr == nil)
			assert.Equal(t, test._expectedDevMode, test.server.presentation.isDevMode())
			assert.Equal(t, test._expectedAudience, test.server.presentation.audience())
		})
	}
}

func TestServer_HandleProfileStyleSave(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedSaved  bool
		_expectedStatus int

		content string
		server  *Server
	}

	tests := make([]*testCase, 0)

	// saves-content
	dir := t.TempDir()
	server := newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
	tests = append(tests, &testCase{
		_id:             "saves-content",
		_expectedSaved:  true,
		_expectedStatus: http.StatusOK,

		content: "- Answer tersely.\n",
		server:  server,
	})

	// rejects-oversized
	dir = t.TempDir()
	server = newTestServer(t, &Options{
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
	tests = append(tests, &testCase{
		_id:             "rejects-oversized",
		_expectedSaved:  false,
		_expectedStatus: http.StatusBadRequest,

		content: strings.Repeat("x", maxStyleProfileBytes+1),
		server:  server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			form := url.Values{"style_content": {test.content}}
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/profile/style", form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			saved, err := test.server.presentation.styleContent()
			require.NoError(t, err)
			assert.Equal(t, test._expectedSaved, saved == test.content)
			// the answer is the form itself with its notice beside Save, never a redirect
			assert.Equal(t, test._expectedSaved, strings.Contains(recorder.Body.String(), `id="profile-style-form"`))
			assert.Equal(t, test._expectedSaved, strings.Contains(recorder.Body.String(), `<span class="badge badge-ok">Ok</span> <span class="label">Saved</span>`))
		})
	}
}

func TestServer_HandleUsagePlan(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedLocation string
		_expectedPlan     budget.Plan
		_expectedStatus   int

		form   url.Values
		server *Server
	}

	tests := make([]*testCase, 0)

	// pass-max-20x
	tests = append(tests, &testCase{
		_id:               "pass-max-20x",
		_expectedLocation: "/usage",
		_expectedPlan:     budget.PlanMax20x,
		_expectedStatus:   http.StatusSeeOther,

		form:   url.Values{"plan": {"max-20x"}},
		server: newProfileServer(t, ""),
	})

	// fail-unknown-plan
	tests = append(tests, &testCase{
		_id:               "fail-unknown-plan",
		_expectedLocation: "",
		_expectedPlan:     budget.DefaultPlan,
		_expectedStatus:   http.StatusBadRequest,

		form:   url.Values{"plan": {"enterprise"}},
		server: newProfileServer(t, ""),
	})

	// fail-empty-keeps-saved
	server := newProfileServer(t, "")
	saved := &budget.Settings{Plan: budget.PlanMax20x}
	require.NoError(t, server.usage.SaveSettings(saved))
	tests = append(tests, &testCase{
		_id:               "fail-empty-keeps-saved",
		_expectedLocation: "",
		_expectedPlan:     budget.PlanMax20x,
		_expectedStatus:   http.StatusBadRequest,

		form:   url.Values{"plan": {""}},
		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/usage/plan", test.form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Equal(t, test._expectedLocation, recorder.Header().Get("Location"))
			settings, err := budget.NewStore(test.server.stateDir).Settings()
			require.NoError(t, err)
			assert.Equal(t, test._expectedPlan, settings.Plan)
		})
	}
}

func TestProfileTokenAdd(t *testing.T) {
	expectedFileMode := os.FileMode(0o600)
	if runtime.GOOS == "windows" {
		expectedFileMode = 0o666
	}
	server, _, tokenDir := routinesTestEnvWithTokens(t, nil)

	// Labeled add via the standalone widget: file 0600 with whitespace-stripped
	// content; the op result names the label and never the value (D8).
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{
		"token_label": {"work"},
		"token_value": {" secret\ntoken "},
	}))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "add token work")
	assert.NotContains(t, body, "secret")

	info, err := os.Stat(filepath.Join(tokenDir, "work"))
	require.NoError(t, err)
	assert.Equal(t, expectedFileMode, info.Mode().Perm())
	content, err := os.ReadFile(filepath.Join(tokenDir, "work"))
	require.NoError(t, err)
	assert.Equal(t, "secrettoken", string(content))

	// Empty label → dated default; a second same-day default gets -2 (D5).
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{"token_value": {"t1"}}))
	require.Equal(t, http.StatusOK, response.Code)
	dated := time.Now().Format("token-2006-01-02")
	assert.Contains(t, response.Body.String(), "add token "+dated)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{"token_value": {"t2"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), dated+"-2")

	// Duplicate explicit label → 400, original intact (D4).
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{
		"token_label": {"work"},
		"token_value": {"replacement"},
	}))
	assert.Equal(t, http.StatusBadRequest, response.Code)
	content, err = os.ReadFile(filepath.Join(tokenDir, "work"))
	require.NoError(t, err)
	assert.Equal(t, "secrettoken", string(content))

	// Path-traversal label → 400, nothing written outside the store.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{
		"token_label": {"../evil"},
		"token_value": {"t3"},
	}))
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.NoFileExists(t, filepath.Join(filepath.Dir(tokenDir), "evil"))

	// Missing or whitespace value → 400 (a forgotten paste must not save silently).
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/profile/tokens", url.Values{
		"token_label": {"blank"},
		"token_value": {" \n "},
	}))
	assert.Equal(t, http.StatusBadRequest, response.Code)

	// Token fields on a params save are ignored — adding moved off Configure.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/demo/params", url.Values{
		"key":         {"ROUTINE_CADENCE_DAYS"},
		"value":       {"2"},
		"token_label": {"rider"},
		"token_value": {"t4"},
	}))
	require.Equal(t, http.StatusOK, response.Code)
	labels, err := routines.ListTokenLabels(tokenDir)
	require.NoError(t, err)
	assert.Len(t, labels, 3)

	// The old route is gone and the Routines page no longer carries the card.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/routines/tokens", url.Values{"token_value": {"t5"}}))
	assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/routines", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "token-add-form")
}
