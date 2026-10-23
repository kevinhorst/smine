package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

func TestPresentationStore_Load(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedAudience  string
		_expectedAutoMerge bool
		_expectedDevMode   bool
		_expectedLanguage  string

		content string
		missing bool
	}

	tests := make([]*testCase, 0)

	// missing-file-defaults
	tests = append(tests, &testCase{
		_id:                "missing-file-defaults",
		_expectedAudience:  "",
		_expectedAutoMerge: true,
		_expectedLanguage:  "en",

		content: "",
		missing: true,
	})

	// legacy-proposals-key-ignored
	tests = append(tests, &testCase{
		_id:                "legacy-proposals-key-ignored",
		_expectedAudience:  "casual",
		_expectedAutoMerge: true,
		_expectedLanguage:  "en",

		content: "---\naudience: casual\nproposals: auto\n---\n",
		missing: false,
	})

	// auto-merge-false-parsed
	tests = append(tests, &testCase{
		_id:                "auto-merge-false-parsed",
		_expectedAudience:  "",
		_expectedAutoMerge: false,
		_expectedLanguage:  "en",

		content: "---\nauto-merge: false\n---\n",
		missing: false,
	})

	// de-casual-parsed
	tests = append(tests, &testCase{
		_id:                "de-casual-parsed",
		_expectedAudience:  "casual",
		_expectedAutoMerge: true,
		_expectedLanguage:  "de",

		content: "---\nlanguage: de\naudience: casual\n---\nbody text\n",
		missing: false,
	})

	// missing-front-matter-defaults
	tests = append(tests, &testCase{
		_id:                "missing-front-matter-defaults",
		_expectedAudience:  "",
		_expectedAutoMerge: true,
		_expectedLanguage:  "en",

		content: "# just a heading\nlanguage: de\n",
		missing: false,
	})

	// unknown-audience-kept-verbatim
	tests = append(tests, &testCase{
		_id:                "unknown-audience-kept-verbatim",
		_expectedAudience:  "manager",
		_expectedAutoMerge: true,
		_expectedLanguage:  "en",

		content: "---\naudience: manager\n---\n",
		missing: false,
	})

	// empty-language-defaults-en
	tests = append(tests, &testCase{
		_id:                "empty-language-defaults-en",
		_expectedAudience:  "casual",
		_expectedAutoMerge: true,
		_expectedLanguage:  "en",

		content: "---\nlanguage:\naudience: casual\n---\n",
		missing: false,
	})

	// dev-mode-true-parsed
	tests = append(tests, &testCase{
		_id:                "dev-mode-true-parsed",
		_expectedAudience:  "",
		_expectedAutoMerge: true,
		_expectedDevMode:   true,
		_expectedLanguage:  "en",

		content: "---\nlanguage: en\naudience: \ndev-mode: true\n---\n",
		missing: false,
	})

	// dev-mode-garbage-off
	tests = append(tests, &testCase{
		_id:                "dev-mode-garbage-off",
		_expectedAudience:  "",
		_expectedAutoMerge: true,
		_expectedDevMode:   false,
		_expectedLanguage:  "en",

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

			store := newPresentationStore(path, filepath.Join(filepath.Dir(path), "style-profile.md"), filepath.Join(filepath.Dir(path), "template.md"), nil)
			assert.Equal(t, test._expectedAudience, store.audience())
			assert.Equal(t, test._expectedAutoMerge, store.autoMerge())
			assert.Equal(t, test._expectedDevMode, store.isDevMode())
			assert.Equal(t, test._expectedLanguage, store.language())
		})
	}
}

func writeGermanProfile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "presentation-profile.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nlanguage: de\naudience: casual\n---\n"), 0o644))
	return path
}

func TestPresentationStore_SaveProfileSelection(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedContent  string
		_expectedLanguage string
		_expectedMissing  bool

		audience  string
		autoMerge bool
		devMode   bool
		existing  string
		language  string
		template  string
	}

	tests := make([]*testCase, 0)

	// developer-en-writes-defaults
	tests = append(tests, &testCase{
		_id:               "developer-en-writes-defaults",
		_expectedContent:  "---\nlanguage: en\naudience: \ndev-mode: false\nauto-merge: true\n---\nbody text\n",
		_expectedLanguage: "en",
		_expectedMissing:  false,

		audience:  "",
		autoMerge: true,
		devMode:   false,
		existing:  "---\nlanguage: de\naudience: casual\n---\nbody text\n",
		language:  "en",
	})

	// developer-en-missing-writes-defaults
	tests = append(tests, &testCase{
		_id:               "developer-en-missing-writes-defaults",
		_expectedContent:  "---\nlanguage: en\naudience: \ndev-mode: false\nauto-merge: true\n---\n",
		_expectedLanguage: "en",
		_expectedMissing:  false,

		audience:  "",
		autoMerge: true,
		devMode:   false,
		existing:  "",
		language:  "en",
	})

	// developer-en-devmode-persists
	tests = append(tests, &testCase{
		_id:               "developer-en-devmode-persists",
		_expectedContent:  "---\nlanguage: en\naudience: \ndev-mode: true\nauto-merge: true\n---\n",
		_expectedLanguage: "en",
		_expectedMissing:  false,

		audience:  "",
		autoMerge: true,
		devMode:   true,
		existing:  "",
		language:  "en",
	})

	// auto-merge-off-persists
	tests = append(tests, &testCase{
		_id:               "auto-merge-off-persists",
		_expectedContent:  "---\nlanguage: en\naudience: \ndev-mode: false\nauto-merge: false\n---\n",
		_expectedLanguage: "en",
		_expectedMissing:  false,

		audience:  "",
		autoMerge: false,
		devMode:   false,
		existing:  "",
		language:  "en",
	})

	// legacy-proposals-line-dropped
	tests = append(tests, &testCase{
		_id:               "legacy-proposals-line-dropped",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\nauto-merge: true\n---\nbody text\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience:  "casual",
		autoMerge: true,
		devMode:   false,
		existing:  "---\nlanguage: en\naudience: casual\nproposals: auto\n---\nbody text\n",
		language:  "de",
	})

	// de-casual-missing-template-empty-body
	tests = append(tests, &testCase{
		_id:               "de-casual-missing-template-empty-body",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\nauto-merge: true\n---\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience:  "casual",
		autoMerge: true,
		devMode:   false,
		existing:  "",
		language:  "de",
		template:  "",
	})

	// de-casual-seeds-template-body
	tests = append(tests, &testCase{
		_id:               "de-casual-seeds-template-body",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\nauto-merge: true\n---\nAntworte kurz.\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience:  "casual",
		autoMerge: true,
		devMode:   false,
		existing:  "",
		language:  "de",
		template:  "---\nlanguage: de\naudience: casual\n---\nAntworte kurz.\n",
	})

	// frontmatter-rewrite-keeps-body
	tests = append(tests, &testCase{
		_id:               "frontmatter-rewrite-keeps-body",
		_expectedContent:  "---\nlanguage: de\naudience: casual\ndev-mode: false\nauto-merge: true\n---\nbody text\n",
		_expectedLanguage: "de",
		_expectedMissing:  false,

		audience:  "casual",
		autoMerge: true,
		devMode:   false,
		existing:  "---\nlanguage: en\naudience: casual\n---\nbody text\n",
		language:  "de",
		template:  "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "presentation-profile.md")
			if test.existing != "" {
				require.NoError(t, os.WriteFile(path, []byte(test.existing), 0o644))
			}
			templatePath := filepath.Join(filepath.Dir(path), "template.md")
			if test.template != "" {
				require.NoError(t, os.WriteFile(templatePath, []byte(test.template), 0o644))
			}
			store := newPresentationStore(path, filepath.Join(filepath.Dir(path), "style-profile.md"), templatePath, nil)

			err := store.saveProfileSelection(test.audience, test.language, test.devMode, test.autoMerge)

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
	stateDir := t.TempDir()
	store := newPresentationStore(
		filepath.Join(dir, "presentation-profile.md"),
		filepath.Join(dir, "global", "style-profile.md"),
		filepath.Join(dir, "template.md"),
		trash.New(stateDir, 0),
	)

	missing, err := store.styleContent()
	require.NoError(t, err)
	assert.Equal(t, "", missing)

	require.NoError(t, store.saveStyle("- Answer tersely.\n"))
	require.NoError(t, store.saveStyle("- Answer at length.\n"))
	content, err := store.styleContent()
	require.NoError(t, err)
	assert.Equal(t, "- Answer at length.\n", content)

	stashes, err := trash.New(stateDir, 0).List()
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	require.Len(t, stashes[0].Entries, 1)
	stashed, err := os.ReadFile(filepath.Join(stateDir, trash.TrashDir, stashes[0].Stamp, filepath.FromSlash(stashes[0].Entries[0].Rel)))
	require.NoError(t, err)
	assert.Equal(t, "- Answer tersely.\n", string(stashed))
}
