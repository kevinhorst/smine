package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	storeBacklogInvalid      = `{"active": 1, "archive": {"done": [], "rejected": []}, "changelog": [], "sections": [], "updated": "2026-10-02"}`
	storeBacklogMinedInvalid = `{"archive": {"done": [], "rejected": []}, "changelog": [], "sections": [{"name": "Icebox", "items": [{"id": "B-001", "added": "2026-10-02", "deps": [], "description": "x", "evidence": [], "link": "", "mined": {"date": "2026-10-03"}, "pri": "low", "route": "", "size": "S", "status": "raw", "theme": "t", "title": "x"}]}], "updated": "2026-10-02"}`
	storeBacklogMinedValid   = `{"archive": {"done": [], "rejected": []}, "changelog": [], "sections": [{"name": "Icebox", "items": [{"id": "B-001", "added": "2026-10-02", "deps": [], "description": "x", "evidence": [], "link": "", "mined": {"date": "2026-10-03", "sessions": ["abc"]}, "pri": "low", "route": "", "size": "S", "status": "raw", "theme": "t", "title": "x"}]}], "updated": "2026-10-02"}`
	storeBacklogValid        = `{"archive": {"done": [], "rejected": []}, "changelog": [{"date": "2026-10-02", "delta": "Initial"}], "sections": [], "updated": "2026-10-02"}`
)

// storeFile is one file of a test store root, its path relative to the root.
type storeFile struct {
	content string
	path    string
}

// provideLinkedWorktree creates a repo named name with one commit and a
// linked worktree beside it, and returns the worktree.
func provideLinkedWorktree(t *testing.T, name string) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	repo := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(repo, 0o755))
	ctx := context.Background()
	_, err = shell.Run(ctx, repo, "git", "init", "-q", "-b", "main")
	require.NoError(t, err)

	_, err = shell.Run(ctx, repo, "git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-q", "-m", "init")
	require.NoError(t, err)

	worktree := filepath.Join(parent, "linked")
	_, err = shell.Run(ctx, repo, "git", "worktree", "add", "-q", "--detach", worktree)
	require.NoError(t, err)
	return worktree
}

// provideStoreRoot writes files into a fresh store root and returns it.
func provideStoreRoot(t *testing.T, files ...*storeFile) string {
	t.Helper()
	root := t.TempDir()
	for _, file := range files {
		path := filepath.Join(root, file.path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(file.content), 0o644))
	}
	return root
}

// violationFiles is the ordered set of file:line locations the violation
// lines name.
func violationFiles(violations []string) []string {
	files := make([]string, 0)
	for _, violation := range violations {
		location, _, _ := strings.Cut(violation, ": ")
		if !slices.Contains(files, location) {
			files = append(files, location)
		}
	}
	return files
}

func TestMainStoreRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		override string
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(provideLinkedWorktree(t, "storerepo"))
	tests := make([]*testCase, 0)

	// named-after-main-checkout
	tests = append(tests, &testCase{
		_id:       "named-after-main-checkout",
		_expected: filepath.Join(home, "GolandProjects", "agent-plans", "storerepo"),

		override: "",
	})

	// confined-session-root
	tests = append(tests, &testCase{
		_id:       "confined-session-root",
		_expected: "/arm/worktree/store",

		override: "/arm/worktree/store",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(config.StoreRootEnv, test.override)
			root, err := mainStoreRoot(context.Background())
			require.NoError(t, err)
			assert.Equal(t, test._expected, root)
		})
	}
}

func TestRunStoreRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		args []string
		dir  string
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.StoreRootEnv, "")
	worktree := provideLinkedWorktree(t, "storerepo")
	tests := make([]*testCase, 0)

	// linked-worktree
	tests = append(tests, &testCase{
		_id:       "linked-worktree",
		_expected: exitClean,

		args: []string{},
		dir:  worktree,
	})

	// extra-argument
	tests = append(tests, &testCase{
		_id:       "extra-argument",
		_expected: exitError,

		args: []string{"extra"},
		dir:  worktree,
	})

	// outside-a-repo
	tests = append(tests, &testCase{
		_id:       "outside-a-repo",
		_expected: exitError,

		args: []string{},
		dir:  t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Chdir(test.dir)
			assert.Equal(t, test._expected, runStoreRoot(test.args))
		})
	}
}

func TestRunStoreCheck(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		args []string
	}

	validBacklog := &storeFile{content: storeBacklogValid, path: "backlog/backlog.json"}
	invalidBacklog := &storeFile{content: storeBacklogInvalid, path: "backlog/backlog.json"}
	minedBacklog := &storeFile{content: storeBacklogMinedValid, path: "backlog/backlog.json"}
	minedInvalidBacklog := &storeFile{content: storeBacklogMinedInvalid, path: "backlog/backlog.json"}
	brokenProposal := &storeFile{content: "{not json", path: "proposals/skills.json"}
	rootFile := filepath.Join(t.TempDir(), "root.json")
	require.NoError(t, os.WriteFile(rootFile, []byte("{}"), 0o644))
	tests := make([]*testCase, 0)

	// valid-store
	tests = append(tests, &testCase{
		_id:       "valid-store",
		_expected: exitClean,

		args: []string{"-root", provideStoreRoot(t, validBacklog)},
	})

	// empty-root
	tests = append(tests, &testCase{
		_id:       "empty-root",
		_expected: exitClean,

		args: []string{"-root", provideStoreRoot(t)},
	})

	// proposal-files-ignored
	tests = append(tests, &testCase{
		_id:       "proposal-files-ignored",
		_expected: exitClean,

		args: []string{"-root", provideStoreRoot(t, validBacklog, brokenProposal)},
	})

	// invalid-backlog
	tests = append(tests, &testCase{
		_id:       "invalid-backlog",
		_expected: exitViolations,

		args: []string{"-root", provideStoreRoot(t, invalidBacklog)},
	})

	// mined-marker-valid
	tests = append(tests, &testCase{
		_id:       "mined-marker-valid",
		_expected: exitClean,

		args: []string{"-root", provideStoreRoot(t, minedBacklog)},
	})

	// mined-marker-missing-sessions
	tests = append(tests, &testCase{
		_id:       "mined-marker-missing-sessions",
		_expected: exitViolations,

		args: []string{"-root", provideStoreRoot(t, minedInvalidBacklog)},
	})

	// missing-root
	tests = append(tests, &testCase{
		_id:       "missing-root",
		_expected: exitError,

		args: []string{"-root", filepath.Join(t.TempDir(), "absent")},
	})

	// root-is-file
	tests = append(tests, &testCase{
		_id:       "root-is-file",
		_expected: exitError,

		args: []string{"-root", rootFile},
	})

	// extra-argument
	tests = append(tests, &testCase{
		_id:       "extra-argument",
		_expected: exitError,

		args: []string{"-root", provideStoreRoot(t), "extra"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runStoreCheck(test.args))
		})
	}
}

func TestStoreTarget_Check(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFiles []string
		_shouldPass    bool

		target *storeTarget
	}

	schemaPath := filepath.Join("..", "..", backlogSchemaPath)
	validFile := &storeFile{content: storeBacklogValid, path: "backlog/backlog.json"}
	invalidFile := &storeFile{content: storeBacklogInvalid, path: "backlog/backlog.json"}
	validRoot := provideStoreRoot(t, validFile)
	invalidRoot := provideStoreRoot(t, invalidFile)
	validPath := filepath.Join(validRoot, validFile.path)
	invalidPath := filepath.Join(invalidRoot, invalidFile.path)
	tests := make([]*testCase, 0)

	// valid-file
	tests = append(tests, &testCase{
		_id:            "valid-file",
		_expectedFiles: []string{},
		_shouldPass:    true,

		target: &storeTarget{files: []string{validPath}, schemaPath: schemaPath},
	})

	// invalid-file-named
	tests = append(tests, &testCase{
		_id:            "invalid-file-named",
		_expectedFiles: []string{invalidPath + ":1"},
		_shouldPass:    true,

		target: &storeTarget{files: []string{invalidPath}, schemaPath: schemaPath},
	})

	// no-files
	tests = append(tests, &testCase{
		_id:            "no-files",
		_expectedFiles: []string{},
		_shouldPass:    true,

		target: &storeTarget{files: []string{}, schemaPath: schemaPath},
	})

	// missing-schema
	tests = append(tests, &testCase{
		_id:            "missing-schema",
		_expectedFiles: []string{},
		_shouldPass:    false,

		target: &storeTarget{files: []string{validPath}, schemaPath: filepath.Join(t.TempDir(), "absent.json")},
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:            "missing-file",
		_expectedFiles: []string{},
		_shouldPass:    false,

		target: &storeTarget{files: []string{filepath.Join(validRoot, "backlog", "absent.json")}, schemaPath: schemaPath},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations, err := test.target.check()
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFiles, violationFiles(violations))
		})
	}
}
