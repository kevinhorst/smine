package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/trash"
)

func TestInstallPresentationProfile(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedOutput  string
		_isRecorded      bool

		env    install.Env
		output *strings.Builder
		target string
	}

	repoDir := t.TempDir()
	templateDir := filepath.Join(repoDir, "settings", "claude_code")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "presentation-profile.test.md"), []byte("template profile\n"), 0o644))
	provideEnv := func(claudeHome string, output *strings.Builder) install.Env {
		env := install.Env{
			ClaudeHome: claudeHome,
			RepoDir:    repoDir,
			Stdout:     output,
			Trash:      trash.New(filepath.Join(claudeHome, "smine"), 0),
		}
		return env
	}
	profilePath := func(claudeHome string) string {
		return filepath.Join(claudeHome, "context", "global", "presentation-profile.md")
	}
	tests := make([]*testCase, 0)

	// absent-written-and-recorded
	claudeHome := t.TempDir()
	output := &strings.Builder{}
	tests = append(tests, &testCase{
		_id:              "absent-written-and-recorded",
		_expectedContent: "template profile\n",
		_expectedOutput:  "-> Presentation profile installed: " + profilePath(claudeHome) + " (test)\n",
		_isRecorded:      true,

		env:    provideEnv(claudeHome, output),
		output: output,
		target: profilePath(claudeHome),
	})

	// present-kept
	claudeHome = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(profilePath(claudeHome)), 0o755))
	require.NoError(t, os.WriteFile(profilePath(claudeHome), []byte("my own profile\n"), 0o644))
	output = &strings.Builder{}
	tests = append(tests, &testCase{
		_id:              "present-kept",
		_expectedContent: "my own profile\n",
		_expectedOutput:  "kept: " + profilePath(claudeHome) + " (exists)\n",
		_isRecorded:      false,

		env:    provideEnv(claudeHome, output),
		output: output,
		target: profilePath(claudeHome),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := installPresentationProfile(test.env, "test")
			content, readErr := os.ReadFile(test.target)
			isRecorded, recordErr := test.env.Trash.IsUnchanged(test.target)

			assert.NoError(t, err)
			assert.NoError(t, readErr)
			assert.NoError(t, recordErr)
			assert.Equal(t, test._expectedContent, string(content))
			assert.Equal(t, test._expectedOutput, test.output.String())
			assert.Equal(t, test._isRecorded, isRecorded)
		})
	}
}
