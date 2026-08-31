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
		_expectedDevMode  bool
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

	// de-casual-parsed
	tests = append(tests, &testCase{
		_id:               "de-casual-parsed",
		_expectedAudience: "casual",
		_expectedLanguage: "de",

		content: "---\nlanguage: de\naudience: casual\n---\nbody text\n",
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
		_expectedAudience: "casual",
		_expectedLanguage: "en",

		content: "---\nlanguage:\naudience: casual\n---\n",
		missing: false,
	})

	// dev-mode-true-parsed
	tests = append(tests, &testCase{
		_id:               "dev-mode-true-parsed",
		_expectedAudience: "",
		_expectedDevMode:  true,
		_expectedLanguage: "en",

		content: "---\nlanguage: en\naudience: \ndev-mode: true\n---\n",
		missing: false,
	})

	// dev-mode-garbage-off
	tests = append(tests, &testCase{
		_id:               "dev-mode-garbage-off",
		_expectedAudience: "",
		_expectedDevMode:  false,
		_expectedLanguage: "en",

		content: "---\ndev-mode: yes please\n---\n",
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
			assert.Equal(t, test._expectedDevMode, profile.DevMode)
			assert.Equal(t, test._expectedLanguage, profile.Language)
		})
	}
}

func TestPresentationProfile_IsDeveloperAudience(t *testing.T) {
	developer := loadPresentationProfile("")
	assert.True(t, developer.isDeveloperAudience())

	casual := &presentationProfile{Audience: audienceCasual, Language: "de"}
	assert.False(t, casual.isDeveloperAudience())
}

func writeGermanProfile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presentation-profile.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nlanguage: de\naudience: casual\n---\n"), 0o644))
	return path
}

func TestProposalCardCasual(t *testing.T) {
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

// groupTitlesFixture carries the canonical English group titles plus one
// free-form title and one accepted proposal for the tooltip pills.
const groupTitlesFixture = `{
	"kind": "skills",
	"groups": [
		{
			"title": "New skills",
			"proposals": [{
				"id": "gt-001",
				"title": "foo — do a thing",
				"change": "A change.",
				"status": "accepted",
				"target": "skills/foo"
			}]
		},
		{
			"title": "Workflows (skill-bundled scripts)",
			"proposals": [{
				"id": "gt-002",
				"title": "bar — a workflow",
				"change": "Another change.",
				"status": "proposed",
				"target": "skills/bar"
			}]
		},
		{
			"title": "Freeform group heading",
			"proposals": [{
				"id": "gt-003",
				"title": "baz — third thing",
				"change": "Third change.",
				"status": "proposed",
				"target": "skills/baz"
			}]
		}
	]
}`

func newGroupTitlesServer(t *testing.T, presentationPath string) *Server {
	t.Helper()
	proposalsDir := filepath.Join(t.TempDir(), "proposals")
	require.NoError(t, os.MkdirAll(proposalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(proposalsDir, "skills.json"), []byte(groupTitlesFixture), 0o644))
	options := &Options{
		PresentationPath: presentationPath,
		ProposalsDir:     proposalsDir,
	}
	return newTestServer(t, options)
}

func TestProposalsGroupTitlesNonDeveloper(t *testing.T) {
	server := newGroupTitlesServer(t, writeGermanProfile(t))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Neue Funktionen")
	assert.Contains(t, body, "Abläufe (gebündelte Skripte)")
	assert.NotContains(t, body, ">New skills <")
	assert.NotContains(t, body, ">Workflows (skill-bundled scripts) <")
	// The kind heading translates; the free-form title falls back to identity.
	assert.Contains(t, body, "<h2>Funktionen</h2>")
	assert.Contains(t, body, "Freeform group heading")
}

func TestProposalsTooltipsGerman(t *testing.T) {
	server := newGroupTitlesServer(t, writeGermanProfile(t))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "1 von 1 angenommen")
	assert.NotContains(t, body, "1 of 1 accepted")
}

func TestProposalsDefaultEnglish(t *testing.T) {
	server := newGroupTitlesServer(t, filepath.Join(t.TempDir(), "missing-profile.md"))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, ">New skills <")
	assert.Contains(t, body, ">Workflows (skill-bundled scripts) <")
	assert.Contains(t, body, "<h2>skills</h2>")
	assert.Contains(t, body, "1 of 1 accepted")
	assert.NotContains(t, body, "Neue Funktionen")
}

func TestPresentationStore_SaveProfileSelection(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedContent  string
		_expectedLanguage string
		_expectedMissing  bool

		audience string
		devMode  bool
		existing string
		language string
	}

	tests := make([]*testCase, 0)

	// developer-en-deletes
	tests = append(tests, &testCase{
		_id:               "developer-en-deletes",
		_expectedContent:  "",
		_expectedLanguage: "en",
		_expectedMissing:  true,

		audience: "",
		devMode:  false,
		existing: "---\nlanguage: de\naudience: casual\n---\nbody text\n",
		language: "en",
	})

	// developer-en-missing-noop
	tests = append(tests, &testCase{
		_id:               "developer-en-missing-noop",
		_expectedContent:  "",
		_expectedLanguage: "en",
		_expectedMissing:  true,

		audience: "",
		devMode:  false,
		existing: "",
		language: "en",
	})

	// developer-en-devmode-persists
	tests = append(tests, &testCase{
		_id:               "developer-en-devmode-persists",
		_expectedContent:  "---\nlanguage: en\naudience: \ndev-mode: true\n---\n",
		_expectedLanguage: "en",
		_expectedMissing:  false,

		audience: "",
		devMode:  true,
		existing: "",
		language: "en",
	})

	// de-casual-creates
	// The repo template is not readable from the test cwd, so the body
	// falls back to empty — the seeding path is frontmatter-only here.
	tests = append(tests, &testCase{
		_id:               "de-casual-creates",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\n---\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience: "casual",
		devMode:  false,
		existing: "",
		language: "de",
	})

	// frontmatter-rewrite-keeps-body
	tests = append(tests, &testCase{
		_id:               "frontmatter-rewrite-keeps-body",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\n---\nbody text\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience: "casual",
		devMode:  false,
		existing: "---\nlanguage: en\naudience: casual\n---\nbody text\n",
		language: "de",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "presentation-profile.md")
			if test.existing != "" {
				require.NoError(t, os.WriteFile(path, []byte(test.existing), 0o644))
			}
			store := newPresentationStore(path, filepath.Join(filepath.Dir(path), "style-profile.md"))

			err := store.saveProfileSelection(test.audience, test.language, test.devMode)

			require.NoError(t, err)
			content, readErr := os.ReadFile(path)
			assert.Equal(t, test._expectedMissing, os.IsNotExist(readErr))
			assert.Equal(t, test._expectedContent, string(content))
			assert.Equal(t, test._expectedLanguage, store.language())
		})
	}
}

func TestPresentationStore_SaveStyle(t *testing.T) {
	dir := t.TempDir()
	store := newPresentationStore(
		filepath.Join(dir, "presentation-profile.md"),
		filepath.Join(dir, "global", "style-profile.md"),
	)

	missing, err := store.styleContent()
	require.NoError(t, err)
	assert.Equal(t, "", missing)

	require.NoError(t, store.saveStyle("- Answer tersely.\n"))
	content, err := store.styleContent()
	require.NoError(t, err)
	assert.Equal(t, "- Answer tersely.\n", content)
}
