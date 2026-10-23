package presentation

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
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

			profile := Load(path)
			assert.Equal(t, test._expectedAudience, profile.Audience)
			assert.Equal(t, test._expectedAutoMerge, profile.AutoMerge)
			assert.Equal(t, test._expectedDevMode, profile.DevMode)
			assert.Equal(t, test._expectedLanguage, profile.Language)
		})
	}
}

func TestProfile_IsDeveloperAudience(t *testing.T) {
	developer := Load("")
	assert.True(t, developer.IsDeveloperAudience())

	casual := &Profile{Audience: AudienceCasual, Language: "de"}
	assert.False(t, casual.IsDeveloperAudience())
}
