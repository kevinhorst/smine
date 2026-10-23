package routinerun

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settings "github.com/kevinhorst/smine/internal/config"
)

const storeSubject = "stores: Recorded code store changes after the demo run (2000-01-01)"

func TestCommitStoreDirs(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFiles string
		_shouldPass    bool

		storeRoot string
	}

	tests := make([]*testCase, 0)

	// clean-no-commit
	cleanStore := filepath.Join(t.TempDir(), "archive", "code")
	newStoreArchive(t, cleanStore, sandboxStore)
	tests = append(tests, &testCase{
		_id:            "clean-no-commit",
		_expectedFiles: "",
		_shouldPass:    true,

		storeRoot: cleanStore,
	})

	// changed-one-commit
	changedStore := filepath.Join(t.TempDir(), "archive", "code")
	newStoreArchive(t, changedStore, sandboxStore)
	require.NoError(t, os.WriteFile(filepath.Join(changedStore, settings.StoreBacklogDir, "backlog.json"), []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(changedStore, settings.StoreProposalsDir, "skills.json"), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:            "changed-one-commit",
		_expectedFiles: "code/backlog/backlog.json",
		_shouldPass:    true,

		storeRoot: changedStore,
	})

	// not-a-git-repo
	plainStore := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(plainStore, settings.StoreBacklogDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plainStore, settings.StoreBacklogDir, "backlog.json"), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:            "not-a-git-repo",
		_expectedFiles: "",
		_shouldPass:    false,

		storeRoot: plainStore,
	})

	// missing-store-root
	tests = append(tests, &testCase{
		_id:            "missing-store-root",
		_expectedFiles: "",
		_shouldPass:    false,

		storeRoot: filepath.Join(t.TempDir(), "absent"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sha, err := commitStoreDirs(context.Background(), test.storeRoot, storeSubject)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFiles, committedFiles(t, test.storeRoot, sha))
		})
	}
}

func TestCommitStoreDirs_KeepsOtherPaths(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "archive", "code")
	newStoreArchive(t, storeRoot, sandboxStore)
	archive := filepath.Dir(storeRoot)
	require.NoError(t, os.WriteFile(filepath.Join(storeRoot, settings.StoreBacklogDir, "backlog.json"), []byte("{}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(storeRoot, "plans"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(storeRoot, "plans", "x.md"), []byte("plan\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(archive, "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archive, "other", "notes.md"), []byte("notes\n"), 0o644))
	gitRun(t, archive, "add", "other/notes.md")

	sha, err := commitStoreDirs(context.Background(), storeRoot, storeSubject)

	require.NoError(t, err)
	assert.Equal(t, "code/backlog/backlog.json", committedFiles(t, storeRoot, sha))
	assert.Equal(t, storeSubject, gitRun(t, archive, "log", "-1", "--format=%s"))
	assert.Equal(t, "A  other/notes.md\n?? code/plans/", gitRun(t, archive, "status", "--porcelain"))
}

// newStoreArchive is a plans-archive repo on main at storeRoot's parent,
// holding storeRoot's backlog store and one proposal kind file, committed.
func newStoreArchive(t *testing.T, storeRoot, backlogStore string) {
	t.Helper()
	archive := filepath.Dir(storeRoot)
	backlogDir := filepath.Join(storeRoot, settings.StoreBacklogDir)
	proposalsDir := filepath.Join(storeRoot, settings.StoreProposalsDir)
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, os.MkdirAll(proposalsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "backlog.json"), []byte(backlogStore), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(proposalsDir, "problems.json"), []byte("{}\n"), 0o644))
	gitRun(t, archive, "init", "-q", "-b", "main")
	gitRun(t, archive, "config", "user.email", "test@example.com")
	gitRun(t, archive, "config", "user.name", "test")
	gitRun(t, archive, "add", "-A")
	gitRun(t, archive, "commit", "-qm", "init")
}

// committedFiles lists the paths commit sha touched in the repo at dir, ""
// when no commit was made.
func committedFiles(t *testing.T, dir, sha string) string {
	t.Helper()
	if sha == "" {
		return ""
	}
	return gitRun(t, dir, "show", "--name-only", "--format=", sha)
}

// commitFile writes rel under repo and commits everything.
func commitFile(t *testing.T, repo, rel, content string) {
	t.Helper()
	path := filepath.Join(repo, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", rel)
}

// commitFileAt commits rel with the committer date pinned to date; the pin
// stays for the rest of the test unless reset.
func commitFileAt(t *testing.T, repo, rel, content, date string) {
	t.Helper()
	t.Setenv("GIT_COMMITTER_DATE", date)
	commitFile(t, repo, rel, content)
}
