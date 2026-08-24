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

func TestLoadPresentationProfile(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAudience string
		_expectedLanguage string

		content string
		missing bool
	}

	tests := make([]*testCase, 0)

	// missing-file-defaults
	tests = append(tests, &testCase{
		_id:               "missing-file-defaults",
		_expectedAudience: "",
		_expectedLanguage: "en",

		content: "",
		missing: true,
	})

	// de-non-developer-parsed
	tests = append(tests, &testCase{
		_id:               "de-non-developer-parsed",
		_expectedAudience: "non-developer",
		_expectedLanguage: "de",

		content: "---\nlanguage: de\naudience: non-developer\n---\nbody text\n",
		missing: false,
	})

	// missing-front-matter-defaults
	tests = append(tests, &testCase{
		_id:               "missing-front-matter-defaults",
		_expectedAudience: "",
		_expectedLanguage: "en",

		content: "# just a heading\nlanguage: de\n",
		missing: false,
	})

	// unknown-audience-kept-verbatim
	tests = append(tests, &testCase{
		_id:               "unknown-audience-kept-verbatim",
		_expectedAudience: "manager",
		_expectedLanguage: "en",

		content: "---\naudience: manager\n---\n",
		missing: false,
	})

	// empty-language-defaults-en
	tests = append(tests, &testCase{
		_id:               "empty-language-defaults-en",
		_expectedAudience: "non-developer",
		_expectedLanguage: "en",

		content: "---\nlanguage:\naudience: non-developer\n---\n",
		missing: false,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "presentation-profile.md")
			if !test.missing {
				require.NoError(t, os.WriteFile(path, []byte(test.content), 0o644))
			}

			profile := loadPresentationProfile(path)
			assert.Equal(t, test._expectedAudience, profile.Audience)
			assert.Equal(t, test._expectedLanguage, profile.Language)
		})
	}
}

func TestPresentationProfile_IsDeveloperAudience(t *testing.T) {
	developer := loadPresentationProfile("")
	assert.True(t, developer.isDeveloperAudience())

	nonDeveloper := &presentationProfile{Audience: audienceNonDeveloper, Language: "de"}
	assert.False(t, nonDeveloper.isDeveloperAudience())
}

func writeGermanProfile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presentation-profile.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nlanguage: de\naudience: non-developer\n---\n"), 0o644))
	return path
}

func TestProposalCardNonDeveloper(t *testing.T) {
	dir := t.TempDir()
	proposalsDir := filepath.Join(dir, "proposals")
	require.NoError(t, os.MkdirAll(proposalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proposalsDir, "skills.json"), []byte(`{
		"kind": "skills",
		"groups": [{
			"title": "Group",
			"proposals": [{
				"id": "sp-001",
				"title": "Ein Vorschlag",
				"change": "Eine Änderung",
				"status": "proposed",
				"target": "skills/foo",
				"tags": ["workflow"],
				"gate": {"band": "J", "verifier": "railroad", "anchor": "x.go:1"},
				"evidence": [{
					"title": "Beleg",
					"sessions": [{"id": "abcdef1234", "note": "eine Notiz"}],
					"snippets": [{"code": "func main() {}", "kind": "code", "lang": "go", "source": "main.go"}]
				}]
			}]
		}]
	}`), 0o644))

	server := newTestServer(t, &Options{
		PresentationPath: writeGermanProfile(t),
		ProposalsDir:     proposalsDir,
	})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.NotContains(t, body, "band J")
	assert.NotContains(t, body, "→ skills/foo")
	assert.NotContains(t, body, ">workflow</span>")
	assert.NotContains(t, body, ">gate</span>")
	assert.Contains(t, body, "Technisches Detail")
	assert.Contains(t, body, "Kommentar (Grund oder Hinweis)")
	assert.Contains(t, body, ">später</button>")
}
