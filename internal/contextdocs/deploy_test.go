package contextdocs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

// provideSourceCheckout writes a minimal smine checkout: the context tree, its generated index and the aspects.
func provideSourceCheckout(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, SourceContextDir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "rules"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "facts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agentsTemplate), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "implementing.md"),
		[]byte("**ACTION-IMPL-001** `[prose]` — Keep.\n\n* Applies: always.\n\n**ACTION-IMPL-002** `[prose]` — Home only.\n\n* Applies: always.\n* Reach: smine\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules", "go.md"),
		[]byte("# Go\n\n**Files:** `*.go`\n\n**RULE-GOLANG-ERR-001** `[prose]` — Wrap errors.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules", "python.md"),
		[]byte("# Python\n\n**Files:** `*.py`\n\n**RULE-PYTHON-ERR-001** `[prose]` — Raise.\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rules", "plan.md"), []byte("# Plan\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts", "smine.md"),
		[]byte("**FACT-REPO-ARCH-001** — Home fact.\n\n* Location: here\n* Reach: smine\n"), 0o644))
	aspects := append(provideFilterAspects(),
		RuleAspect{Name: "REPO", Scope: "Repository", Class: "scope"},
		RuleAspect{Name: "ARCH", Scope: "Architecture", Class: "topic"},
		RuleAspect{Name: "ERR", Scope: "Errors", Class: "topic"},
	)
	require.NoError(t, SaveAspects(dir, aspects))
	require.NoError(t, WriteContextFile(dir))
	return root
}

func provideTargetRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	_, err := shell.Run(context.Background(), repo, "git", "init", "-q")
	require.NoError(t, err)
	return repo
}

func provideDeployOptions(t *testing.T, repo string) *DeployOptions {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	options := &DeployOptions{
		ContextDir: "docs",
		Langs:      []string{"go"},
		Name:       "work",
		RepoPath:   repo,
		SourceRoot: provideSourceCheckout(t),
		StateDir:   filepath.Join(home, ".claude", "smine"),
	}
	return options
}

func TestDeployBuildsRecordWithoutWriting(t *testing.T) {
	repo := provideTargetRepo(t)
	options := provideDeployOptions(t, repo)

	report, err := Deploy(context.Background(), options)
	require.NoError(t, err)

	record, err := LoadContextFile(DeployRecordPath(options.StateDir, "work"))
	require.NoError(t, err)
	var ids []string
	for _, entry := range record.Entries {
		ids = append(ids, entry.Id)
	}
	assert.Equal(t, []string{"ACTION-IMPL-001", "RULE-GOLANG-ERR-001"}, ids)
	assert.Equal(t, "docs/actions/implementing.md", record.Entries[0].Source)
	assert.Empty(t, record.Self)
	assert.Equal(t, &ContextDeploy{ContextDir: "docs", Langs: []string{"go"}, Role: DefaultRole}, record.Deploy)
	assert.FileExists(t, filepath.Join(options.StateDir, DeployRecordDir, "work", AspectsFileName))
	assert.NoFileExists(t, filepath.Join(repo, "docs", ContextFileName))
	assert.Contains(t, report, "not written (write to repo off)")

	patch := filepath.Join(options.StateDir, DeployRecordDir, "work", OverlayPatchName)
	_, err = shell.Run(context.Background(), repo, "git", "apply", patch)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(repo, "docs", ContextFileName))
	assert.FileExists(t, filepath.Join(repo, "docs", AspectsFileName))
}

func TestDeployWritesRepoWithoutMarkdown(t *testing.T) {
	repo := provideTargetRepo(t)
	options := provideDeployOptions(t, repo)
	options.WriteRepo = true

	_, err := Deploy(context.Background(), options)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(repo, "docs", ContextFileName))
	assert.FileExists(t, filepath.Join(repo, "docs", AspectsFileName))
	assert.NoFileExists(t, filepath.Join(repo, "AGENTS.md"))
	assert.NoDirExists(t, filepath.Join(repo, "docs", "rules"))
	assert.True(t, IsDeployedIndex(filepath.Join(repo, "docs", ContextFileName)))
}

func TestDeployProjectsMarkdown(t *testing.T) {
	repo := provideTargetRepo(t)
	options := provideDeployOptions(t, repo)
	options.WriteRepo = true
	options.Markdown = true

	_, err := Deploy(context.Background(), options)
	require.NoError(t, err)

	agents, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	require.NoError(t, err)
	assert.Contains(t, string(agents), DefaultRole)
	assert.Contains(t, string(agents), "docs/rules/go.md")
	assert.NotContains(t, string(agents), "python")
	actions, err := os.ReadFile(filepath.Join(repo, "docs", "actions", "implementing.md"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(actions), baselineHeader+"\n\n"))
	assert.Contains(t, string(actions), "ACTION-IMPL-001")
	assert.NotContains(t, string(actions), "ACTION-IMPL-002")
	assert.FileExists(t, filepath.Join(repo, "docs", "rules", "go.md"))
	assert.FileExists(t, filepath.Join(repo, "docs", "rules", "plan.md"))
	assert.NoFileExists(t, filepath.Join(repo, "docs", "rules", "python.md"))
	assert.NoFileExists(t, filepath.Join(repo, "docs", "facts", "smine.md"))
}

func TestDeployKeepsRepoOwnedEntries(t *testing.T) {
	repo := provideTargetRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs", "facts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "facts", "repo.md"),
		[]byte("**FACT-REPO-ARCH-101** — Own fact.\n\n* Location: here\n* Reach: work\n"), 0o644))
	options := provideDeployOptions(t, repo)
	options.WriteRepo = true
	options.Markdown = true

	_, err := Deploy(context.Background(), options)
	require.NoError(t, err)

	record, err := LoadContextFile(filepath.Join(repo, "docs", ContextFileName))
	require.NoError(t, err)
	var ids []string
	for _, entry := range record.Entries {
		ids = append(ids, entry.Id)
	}
	assert.Contains(t, ids, "FACT-REPO-ARCH-101")
	own, err := os.ReadFile(filepath.Join(repo, "docs", "facts", "repo.md"))
	require.NoError(t, err)
	assert.Equal(t, "**FACT-REPO-ARCH-101** — Own fact.\n\n* Location: here\n* Reach: work\n", string(own))
}

func TestDeployRefusesRepoOwnedCollision(t *testing.T) {
	repo := provideTargetRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs", "actions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "actions", "implementing.md"),
		[]byte("**ACTION-IMPL-101** `[prose]` — Own.\n\n* Applies: here.\n"), 0o644))
	options := provideDeployOptions(t, repo)
	options.WriteRepo = true
	options.Markdown = true

	_, err := Deploy(context.Background(), options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo-owned but collides")
}

func TestDeployRefusesInvalidRepoOwnedEntry(t *testing.T) {
	repo := provideTargetRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs", "actions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "actions", "local.md"),
		[]byte("**ACTION-IMPL-099** `[prose]` — Overlay with a baseline number.\n\n* Applies: here.\n"), 0o644))
	options := provideDeployOptions(t, repo)

	_, err := Deploy(context.Background(), options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overlay entries use numbers 100+")
	assert.NoFileExists(t, DeployRecordPath(options.StateDir, "work"))
}

func TestDeployRejectsBadOptions(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		options *DeployOptions
	}

	tests := make([]*testCase, 0)

	// missing-name
	tests = append(tests, &testCase{
		_id:       "missing-name",
		_expected: "Missing registry name",

		options: &DeployOptions{ContextDir: "docs", RepoPath: t.TempDir(), StateDir: t.TempDir()},
	})

	// bad-context-dir
	tests = append(tests, &testCase{
		_id:       "bad-context-dir",
		_expected: "Invalid context folder",

		options: &DeployOptions{ContextDir: "../x", Name: "work", RepoPath: t.TempDir(), StateDir: t.TempDir()},
	})

	// missing-repo
	tests = append(tests, &testCase{
		_id:       "missing-repo",
		_expected: "is not a directory",

		options: &DeployOptions{ContextDir: "docs", Name: "work", RepoPath: filepath.Join(t.TempDir(), "absent"), StateDir: t.TempDir()},
	})

	// missing-state-dir
	tests = append(tests, &testCase{
		_id:       "missing-state-dir",
		_expected: "Missing state dir",

		options: &DeployOptions{ContextDir: "docs", Name: "work", RepoPath: t.TempDir()},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := Deploy(context.Background(), test.options)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test._expected)
		})
	}
}

func TestDeployMissingSourceIndex(t *testing.T) {
	repo := provideTargetRepo(t)
	options := provideDeployOptions(t, repo)
	require.NoError(t, os.Remove(filepath.Join(options.SourceRoot, SourceContextDir, ContextFileName)))

	_, err := Deploy(context.Background(), options)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Source index")
}

func TestDeployRemovesAgentsSymlink(t *testing.T) {
	repo := provideTargetRepo(t)
	require.NoError(t, os.Symlink("AGENTS.md", filepath.Join(repo, "CLAUDE.md")))
	options := provideDeployOptions(t, repo)
	options.WriteRepo = true
	options.Markdown = true

	_, err := Deploy(context.Background(), options)
	require.NoError(t, err)

	_, err = os.Lstat(filepath.Join(repo, "CLAUDE.md"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
