package secretscan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scanRepoTree(t *testing.T, repo string) *Result {
	t.Helper()
	result := &Result{
		BaselinedFindings: make([]Finding, 0),
		NewFindings:       make([]Finding, 0),
		OversizeFiles:     make([]string, 0),
		RepoPath:          repo,
		treeFindings:      make([]Finding, 0),
	}
	require.NoError(t, scanTree(repo, mustPatterns(t), result))
	return result
}

func TestScanTree(t *testing.T) {
	secretLine := "key = \"" + fakeAwsKey + "\"\n"

	t.Run("skips-vendor-dir", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "vendor/creds.py", secretLine)
		writeFile(t, repo, "node_modules/creds.py", secretLine)

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("skips-lockfile", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "go.sum", "example.com/mod v1.0.0 h1:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e8862\n")
		writeFile(t, repo, "package-lock.json", secretLine)

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("skips-test-go-file", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "internal/api/fixtures_test.go", secretLine)

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("skips-svg-and-minified", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "assets/icon.svg", secretLine)
		writeFile(t, repo, "assets/app.min.js", secretLine)
		writeFile(t, repo, "assets/app.js.map", secretLine)

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("skips-worktree-pool", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, ".claude/worktrees/session/secret.py", secretLine)
		writeFile(t, repo, ".claude/settings.json", "{}\n")

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("skips-git-file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".git", "gitdir: /somewhere/else\n")
		writeFile(t, root, "main.go", "package main\n")

		result := scanRepoTree(t, root)
		assert.Empty(t, result.treeFindings)
	})

	t.Run("deny-file-plus-content-both-found", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, ".env", secretLine)

		result := scanRepoTree(t, repo)
		require.Len(t, result.treeFindings, 2)
		assert.Equal(t, detectorDenyFile, result.treeFindings[0].Detector)
		assert.Equal(t, 0, result.treeFindings[0].Line)
		assert.Equal(t, "aws-access-key-id", result.treeFindings[1].Detector)
	})

	t.Run("binary-counted", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "blob.bin", "prefix\x00"+secretLine)

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
		assert.Equal(t, 1, result.BinarySkipCount)
	})

	t.Run("oversize-recorded", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "huge.txt", strings.Repeat("a", maxFileSize+1))

		result := scanRepoTree(t, repo)
		assert.Empty(t, result.treeFindings)
		assert.Equal(t, []string{"huge.txt"}, result.OversizeFiles)
	})

	t.Run("symlink-name-flagged-content-unread", func(t *testing.T) {
		repo := makeRepo(t)
		target := filepath.Join(t.TempDir(), "target.txt")
		require.NoError(t, os.WriteFile(target, []byte(secretLine), 0644))
		require.NoError(t, os.Symlink(target, filepath.Join(repo, "id_rsa")))

		result := scanRepoTree(t, repo)
		require.Len(t, result.treeFindings, 1)
		assert.Equal(t, detectorDenyFile, result.treeFindings[0].Detector)
	})

	t.Run("findings-sorted", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "b.py", secretLine)
		writeFile(t, repo, "a.py", secretLine)

		result := scanRepoTree(t, repo)
		require.Len(t, result.treeFindings, 2)
		assert.Equal(t, "a.py", result.treeFindings[0].Path)
		assert.Equal(t, "b.py", result.treeFindings[1].Path)
	})

	t.Run("rel-paths-slashed", func(t *testing.T) {
		repo := makeRepo(t)
		writeFile(t, repo, "nested/dir/creds.py", secretLine)

		result := scanRepoTree(t, repo)
		require.Len(t, result.treeFindings, 1)
		assert.Equal(t, "nested/dir/creds.py", result.treeFindings[0].Path)
	})
}
