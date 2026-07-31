package secretscan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAwsKey is AWS's documented example access key id — deterministic fake,
// never live (data-integrity: dev/test credentials are deterministic).
const fakeAwsKey = "AKIAIOSFODNN7EXAMPLE"

func makeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0755))
	return root
}

func mustPatterns(t *testing.T) []compiledPattern {
	t.Helper()
	patterns, err := compilePatterns()
	require.NoError(t, err)
	return patterns
}

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestGuardGitRepo(t *testing.T) {
	t.Run("git-dir-accepted", func(t *testing.T) {
		repo := makeRepo(t)
		assert.NoError(t, guardGitRepo(repo))
	})

	t.Run("git-file-accepted", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".git", "gitdir: /somewhere/else\n")
		assert.NoError(t, guardGitRepo(root))
	})

	t.Run("missing-git-errors", func(t *testing.T) {
		err := guardGitRepo(t.TempDir())
		assert.ErrorContains(t, err, "Not a git repository")
	})
}

func TestScan(t *testing.T) {
	t.Run("end-to-end-tree-fixture", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "config.py", "key = \""+fakeAwsKey+"\"\n")

		result, err := Scan(repo, Options{})
		require.NoError(t, err)

		assert.Equal(t, repo, result.RepoPath)
		require.Len(t, result.NewFindings, 1)
		assert.Equal(t, "aws-access-key-id", result.NewFindings[0].Detector)
		assert.Equal(t, "config.py", result.NewFindings[0].Path)
		assert.Empty(t, result.BaselinedFindings)
		assert.Empty(t, result.HistoryFindings)
	})

	t.Run("clean-repo-empty-result", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "main.go", "package main\n")

		result, err := Scan(repo, Options{})
		require.NoError(t, err)

		assert.Empty(t, result.NewFindings)
		assert.Empty(t, result.BaselinedFindings)
		assert.Zero(t, result.BinarySkipCount)
	})
}
