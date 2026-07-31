package secretscan

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitCmd(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-q")
	gitCmd(t, repo, "config", "user.email", "test@example.com")
	gitCmd(t, repo, "config", "user.name", "test")
	return repo
}

func TestScanHistory(t *testing.T) {
	t.Run("removed-secret-found", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, repo, "secret.py", "key = \""+fakeAwsKey+"\"\n")
		gitCmd(t, repo, "add", "secret.py")
		gitCmd(t, repo, "commit", "-qm", "add secret")
		gitCmd(t, repo, "rm", "-q", "secret.py")
		gitCmd(t, repo, "commit", "-qm", "remove secret")

		findings, err := scanHistory(repo, mustPatterns(t))
		require.NoError(t, err)

		require.Len(t, findings, 1)
		assert.Equal(t, "aws-access-key-id", findings[0].Detector)
		assert.Equal(t, "secret.py", findings[0].Path)
		assert.Len(t, findings[0].Commits, 2, "add and remove commits, newest first")
	})

	t.Run("skip-glob-applies-to-blobs", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, repo, "api_test.go", "key = \""+fakeAwsKey+"\"\n")
		gitCmd(t, repo, "add", "api_test.go")
		gitCmd(t, repo, "commit", "-qm", "add test fixture")

		findings, err := scanHistory(repo, mustPatterns(t))
		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("binary-blob-skipped", func(t *testing.T) {
		repo := initGitRepo(t)
		writeFile(t, repo, "blob.bin", "prefix\x00key = \""+fakeAwsKey+"\"\n")
		gitCmd(t, repo, "add", "blob.bin")
		gitCmd(t, repo, "commit", "-qm", "add binary")

		findings, err := scanHistory(repo, mustPatterns(t))
		require.NoError(t, err)
		assert.Empty(t, findings)
	})

	t.Run("empty-repo-no-findings", func(t *testing.T) {
		repo := initGitRepo(t)

		findings, err := scanHistory(repo, mustPatterns(t))
		require.NoError(t, err)
		assert.Empty(t, findings)
	})
}

func TestListObjects(t *testing.T) {
	repo := initGitRepo(t)
	writeFile(t, repo, "data.txt", "hello\n")
	gitCmd(t, repo, "add", "data.txt")
	gitCmd(t, repo, "commit", "-qm", "add data")

	blobOid, err := gitOutput(repo, "rev-parse", "HEAD:data.txt")
	require.NoError(t, err)
	commitOid, err := gitOutput(repo, "rev-parse", "HEAD")
	require.NoError(t, err)

	objectPaths, err := listObjects(repo)
	require.NoError(t, err)

	t.Run("blob-paths-mapped", func(t *testing.T) {
		assert.Equal(t, "data.txt", objectPaths[strings.TrimSpace(string(blobOid))])
	})

	t.Run("commit-lines-empty-path", func(t *testing.T) {
		path, found := objectPaths[strings.TrimSpace(string(commitOid))]
		assert.True(t, found)
		assert.Empty(t, path)
	})
}
