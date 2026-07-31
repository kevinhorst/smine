package repos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeEchoStubs fills a scripts dir with stubs that echo script name + argv
// + cwd, so ops tests can assert the exact exec shape.
func writeEchoStubs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{cherryPickScript, mergeScript, pruneScript, removeScript, syncScript} {
		script := "#!/bin/sh\necho \"" + name + " $* in $(pwd)\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	}
	return dir
}

// initGitRepo builds a fixture repo with one claude/* branch and returns its
// path plus the branch tip sha.
func initGitRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "base"), []byte("base\n"), 0o644))
	run("add", "base")
	run("commit", "-qm", "base")
	run("branch", "claude/feature")

	cmd := exec.Command("git", "rev-parse", "claude/feature")
	cmd.Dir = repo
	sha, err := cmd.Output()
	require.NoError(t, err)
	return repo, strings.TrimSpace(string(sha))
}

func TestValidateBranchRejectsNonClaude(t *testing.T) {
	err := ValidateBranch(context.Background(), "main", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Not an agent branch")
}

func TestValidateBranchRejectsUnknownRef(t *testing.T) {
	repo, _ := initGitRepo(t)
	err := ValidateBranch(context.Background(), "claude/missing", repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown branch")
}

func TestValidateCommitRejectsMalformedSha(t *testing.T) {
	for _, sha := range []string{"", "xyz", "abc;rm -rf", "ABCDEF1234"} {
		err := ValidateCommit(context.Background(), sha, t.TempDir())
		require.Error(t, err, sha)
		assert.Contains(t, err.Error(), "Invalid commit sha")
	}
}

func TestValidateCommitRejectsUnknownCommit(t *testing.T) {
	repo, _ := initGitRepo(t)
	err := ValidateCommit(context.Background(), "0123456789abcdef0123456789abcdef01234567", repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown commit")
}

func TestSyncExecShape(t *testing.T) {
	repo, _ := initGitRepo(t)
	scriptsDir := writeEchoStubs(t)

	output, err := Sync(context.Background(), "claude/feature", repo, scriptsDir)
	require.NoError(t, err)
	assert.Contains(t, output, syncScript+" claude/feature in")
}

func TestMergeExecShape(t *testing.T) {
	repo, _ := initGitRepo(t)
	scriptsDir := writeEchoStubs(t)

	output, err := Merge(context.Background(), "claude/feature", repo, scriptsDir)
	require.NoError(t, err)
	assert.Contains(t, output, mergeScript+" claude/feature in")
}

func TestCherryPickExecShape(t *testing.T) {
	repo, sha := initGitRepo(t)
	scriptsDir := writeEchoStubs(t)

	output, err := CherryPick(context.Background(), sha, repo, scriptsDir)
	require.NoError(t, err)
	assert.Contains(t, output, cherryPickScript+" "+sha+" in")
}

func TestRemoveExecShape(t *testing.T) {
	repo, _ := initGitRepo(t)
	scriptsDir := writeEchoStubs(t)

	cases := []struct {
		force        bool
		deleteBranch bool
		want         string
	}{
		{force: false, deleteBranch: false, want: removeScript + " claude/feature in"},
		{force: true, deleteBranch: false, want: removeScript + " --force claude/feature in"},
		{force: false, deleteBranch: true, want: removeScript + " --delete-branch claude/feature in"},
		{force: true, deleteBranch: true, want: removeScript + " --delete-branch --force claude/feature in"},
	}
	for _, c := range cases {
		output, err := Remove(context.Background(), "claude/feature", c.force, c.deleteBranch, repo, scriptsDir)
		require.NoError(t, err)
		assert.Contains(t, output, c.want)
	}
}

func TestPruneJetbrainsFlagComposition(t *testing.T) {
	scriptsDir := writeEchoStubs(t)

	cases := []struct {
		dryRun bool
		force  bool
		want   string
	}{
		{dryRun: true, force: true, want: pruneScript + " --dry-run --force in"},
		{dryRun: true, force: false, want: pruneScript + " --dry-run in"},
		{dryRun: false, force: true, want: pruneScript + " --force in"},
		{dryRun: false, force: false, want: pruneScript + "  in"},
	}
	for _, c := range cases {
		output, err := PruneJetbrains(context.Background(), c.dryRun, c.force, scriptsDir)
		require.NoError(t, err)
		assert.Contains(t, output, c.want)
	}
}
