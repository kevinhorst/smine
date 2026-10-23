package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/shell"
)

// gitOut runs git in dir and returns its trimmed output; a failure ends the test.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(t.Context(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// newRepo is a fresh repository on main with an identity, the hook source in place and the repo's own global config out of reach.
func newRepo(t *testing.T, dir string) install.Env {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	gitOut(t, dir, "init", "-q", "-b", "main")
	gitOut(t, dir, "config", "user.email", "test@example.com")
	gitOut(t, dir, "config", "user.name", "test")

	hook, err := os.ReadFile(filepath.Join("..", "..", "cmd", "sync", "hooks", "pre-commit"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "sync", "hooks"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "sync", "hooks", "pre-commit"), hook, 0o644))
	return install.Env{GOOS: "darwin", Home: dir, RepoDir: dir}
}

// isolateGitConfig keeps the machine's global and system git config out of the test.
func isolateGitConfig(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
}

func TestEnsureRepo(t *testing.T) {
	work := t.TempDir()
	isolateGitConfig(t, filepath.Join(work, "home"))
	require.NoError(t, os.MkdirAll(filepath.Join(work, "home"), 0o755))

	// a fresh dir: init on main, everything committed, no remote, no identity needed
	fresh := filepath.Join(work, "fresh")
	require.NoError(t, os.MkdirAll(fresh, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fresh, "file.txt"), []byte("hello\n"), 0o644))
	require.NoError(t, ensureRepo(t.Context(), install.Env{RepoDir: fresh}))
	assert.Equal(t, "main", gitOut(t, fresh, "symbolic-ref", "--short", "HEAD"))
	assert.Equal(t, "1", gitOut(t, fresh, "rev-list", "--count", "HEAD"))
	assert.Empty(t, gitOut(t, fresh, "status", "--porcelain"), "the tree is fully committed")
	assert.Empty(t, gitOut(t, fresh, "remote"), "no remote is configured")

	// an existing repository: untouched
	before := gitOut(t, fresh, "rev-parse", "HEAD")
	require.NoError(t, ensureRepo(t.Context(), install.Env{RepoDir: fresh}))
	assert.Equal(t, before, gitOut(t, fresh, "rev-parse", "HEAD"))

	// a worktree, .git a gitlink file: untouched, dirty changes stay uncommitted
	worktree := filepath.Join(work, "wt")
	gitOut(t, fresh, "worktree", "add", "-q", "-b", "wt-branch", worktree)
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "file.txt"), []byte("dirty\n"), 0o644))
	require.NoError(t, ensureRepo(t.Context(), install.Env{RepoDir: worktree}))
	assert.Equal(t, before, gitOut(t, worktree, "rev-parse", "HEAD"))
	assert.Equal(t, "M file.txt", gitOut(t, worktree, "status", "--porcelain"), "the dirty change stays uncommitted")
}

func TestInstallHooks(t *testing.T) {
	work := t.TempDir()
	isolateGitConfig(t, filepath.Join(work, "home"))
	marker := "[ACDSL-" + "PROJECTION]"
	env := newRepo(t, filepath.Join(work, "repo"))
	target := filepath.Join(env.RepoDir, ".git", "hooks", "pre-commit")

	// install into a fresh repo
	require.NoError(t, installHooks(t.Context(), env))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "the hook is executable")

	// a clean commit passes
	require.NoError(t, os.WriteFile(filepath.Join(env.RepoDir, "x.go"), []byte("package x\n"), 0o644))
	gitOut(t, env.RepoDir, "add", ".")
	gitOut(t, env.RepoDir, "commit", "-q", "-m", "clean")

	// a bin/acdsl built for another OS cannot run here and never blocks a clean commit
	require.NoError(t, os.MkdirAll(filepath.Join(env.RepoDir, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(env.RepoDir, "bin", "acdsl"), []byte("\x7fELFnot-for-this-os\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(env.RepoDir, "z.go"), []byte("package z\n"), 0o644))
	gitOut(t, env.RepoDir, "add", "z.go")
	gitOut(t, env.RepoDir, "commit", "-q", "-m", "foreign binary")

	// a staged projection marker is blocked; the stripped file commits
	require.NoError(t, os.WriteFile(filepath.Join(env.RepoDir, "y.go"), []byte("// "+marker+" 1 rule(s) govern this file\npackage y\n"), 0o644))
	gitOut(t, env.RepoDir, "add", "y.go")
	_, err = shell.Run(t.Context(), env.RepoDir, "git", "commit", "-q", "-m", "leak")
	assert.Error(t, err, "a commit with staged projection content is blocked")
	require.NoError(t, os.WriteFile(filepath.Join(env.RepoDir, "y.go"), []byte("package y\n"), 0o644))
	gitOut(t, env.RepoDir, "add", "y.go")
	gitOut(t, env.RepoDir, "commit", "-q", "-m", "stripped")

	// re-install over our own hook succeeds
	assert.NoError(t, installHooks(t.Context(), env))

	// a foreign pre-commit hook is refused
	foreign := newRepo(t, filepath.Join(work, "foreign"))
	require.NoError(t, os.WriteFile(filepath.Join(foreign.RepoDir, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	assert.ErrorContains(t, installHooks(t.Context(), foreign), "foreign pre-commit hook")
}

func TestMergeDriverCommand(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		env install.Env
	}

	t.Setenv("LOCALAPPDATA", filepath.Join("C:", "Users", "u", "AppData", "Local"))
	tests := make([]*testCase, 0)

	// darwin-go-bin
	tests = append(tests, &testCase{
		_id:       "darwin-go-bin",
		_expected: "/Users/u/go/bin/smine-rules merge-skillcue %O %A %B %P %X %Y",

		env: install.Env{GOOS: "darwin", Home: "/Users/u"},
	})

	// linux-go-bin
	tests = append(tests, &testCase{
		_id:       "linux-go-bin",
		_expected: "/home/u/go/bin/smine-rules merge-skillcue %O %A %B %P %X %Y",

		env: install.Env{GOOS: "linux", Home: "/home/u"},
	})

	// windows-shim-dir-exe-forward-slashes
	tests = append(tests, &testCase{
		_id:       "windows-shim-dir-exe-forward-slashes",
		_expected: filepath.ToSlash(os.Getenv("LOCALAPPDATA")) + "/smine/bin/smine-rules.exe merge-skillcue %O %A %B %P %X %Y",

		env: install.Env{GOOS: "windows", Home: `C:\Users\u`},
	})

	// darwin-home-with-space-kept-verbatim
	tests = append(tests, &testCase{
		_id:       "darwin-home-with-space-kept-verbatim",
		_expected: "/Users/u name/go/bin/smine-rules merge-skillcue %O %A %B %P %X %Y",

		env: install.Env{GOOS: "darwin", Home: "/Users/u name"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			command := mergeDriverCommand(test.env)

			assert.Equal(t, test._expected, command)
			assert.NotContains(t, command, `\`)
		})
	}
}
