package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

func TestRun(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedExit     int
		_expectedHead     string
		_expectedReflog   []string
		_expectedStdout   string
		_expectedUpstream string

		name  string
		repo  string
		stdin string
	}

	ctx := context.Background()
	home := t.TempDir()
	hookConfig := defaultHookConfig(t)
	writeHookConfig(t, home, hookConfig)
	homeArgs := []string{"-claude-home", home}
	branchPrefix, worktreesDir := hookConfig.Worktreecreate.BranchPrefix, hookConfig.Worktreecreate.WorktreesDir
	root := newFixtureRepo(t, "main")
	rootHead := gitOutput(t, root, "rev-parse", "HEAD")
	linked := filepath.Join(t.TempDir(), "linked")
	gitOutput(t, root, "worktree", "add", "-b", "claude/linked", linked)
	gitOutput(t, linked, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-m", "linked")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	gitOutput(t, root, "branch", "claude/taken")
	notRepo := t.TempDir()
	trunkRoot := newFixtureRepo(t, "trunk")
	originDefaultRoot := newFixtureRepo(t, "main")
	originDefaultHead := gitOutput(t, originDefaultRoot, "rev-parse", "HEAD")
	originDefault := filepath.Join(t.TempDir(), "origin.git")
	gitOutput(t, originDefaultRoot, "init", "-q", "--bare", originDefault)
	gitOutput(t, originDefaultRoot, "remote", "add", "origin", originDefault)
	gitOutput(t, originDefaultRoot, "push", "-q", "origin", "main")
	gitOutput(t, originDefaultRoot, "remote", "set-head", "origin", "main")
	gitOutput(t, originDefaultRoot, "switch", "-q", "-c", "work")
	gitOutput(t, originDefaultRoot, "branch", "-q", "-D", "main")

	envelope := func(cwd string, name string) string {
		input := &HookInput{Cwd: cwd, Name: name}
		data, err := json.Marshal(input)
		require.NoError(t, err)
		return string(data)
	}
	createdFromMain := []string{"branch: Created from main"}

	tests := make([]*testCase, 0)

	// pass-main-checkout
	tests = append(tests, &testCase{
		_id:               "pass-main-checkout",
		_expectedExit:     0,
		_expectedHead:     rootHead,
		_expectedReflog:   createdFromMain,
		_expectedStdout:   filepath.Join(root, worktreesDir, "feature-a1b2c3") + "\n",
		_expectedUpstream: "",

		name:  "feature-a1b2c3",
		repo:  root,
		stdin: envelope(root, "feature-a1b2c3"),
	})

	// pass-from-linked-worktree
	tests = append(tests, &testCase{
		_id:               "pass-from-linked-worktree",
		_expectedExit:     0,
		_expectedHead:     rootHead,
		_expectedReflog:   createdFromMain,
		_expectedStdout:   filepath.Join(root, worktreesDir, "from-linked-d4e5f6") + "\n",
		_expectedUpstream: "",

		name:  "from-linked-d4e5f6",
		repo:  root,
		stdin: envelope(linked, "from-linked-d4e5f6"),
	})

	// pass-from-subdir
	tests = append(tests, &testCase{
		_id:               "pass-from-subdir",
		_expectedExit:     0,
		_expectedHead:     rootHead,
		_expectedReflog:   createdFromMain,
		_expectedStdout:   filepath.Join(root, worktreesDir, "from-subdir-0a0b0c") + "\n",
		_expectedUpstream: "",

		name:  "from-subdir-0a0b0c",
		repo:  root,
		stdin: envelope(filepath.Join(root, "sub"), "from-subdir-0a0b0c"),
	})

	// pass-default-only-on-origin
	tests = append(tests, &testCase{
		_id:               "pass-default-only-on-origin",
		_expectedExit:     0,
		_expectedHead:     originDefaultHead,
		_expectedReflog:   []string{"branch: Created from origin/main"},
		_expectedStdout:   filepath.Join(originDefaultRoot, worktreesDir, "origin-default") + "\n",
		_expectedUpstream: "",

		name:  "origin-default",
		repo:  originDefaultRoot,
		stdin: envelope(originDefaultRoot, "origin-default"),
	})

	// fail-bad-json
	tests = append(tests, &testCase{
		_id:               "fail-bad-json",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "bad-json",
		repo:  root,
		stdin: "{",
	})

	// fail-empty-name
	tests = append(tests, &testCase{
		_id:               "fail-empty-name",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "",
		repo:  root,
		stdin: envelope(root, ""),
	})

	// fail-traversal-name
	tests = append(tests, &testCase{
		_id:               "fail-traversal-name",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "../x",
		repo:  root,
		stdin: envelope(root, "../x"),
	})

	// fail-slash-name
	tests = append(tests, &testCase{
		_id:               "fail-slash-name",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "a/b",
		repo:  root,
		stdin: envelope(root, "a/b"),
	})

	// fail-relative-cwd
	tests = append(tests, &testCase{
		_id:               "fail-relative-cwd",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "relative-cwd",
		repo:  root,
		stdin: envelope("repo", "relative-cwd"),
	})

	// fail-existing-branch
	tests = append(tests, &testCase{
		_id:               "fail-existing-branch",
		_expectedExit:     1,
		_expectedHead:     rootHead,
		_expectedReflog:   []string{"branch: Created from main"},
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "taken",
		repo:  root,
		stdin: envelope(root, "taken"),
	})

	// fail-not-a-repo
	tests = append(tests, &testCase{
		_id:               "fail-not-a-repo",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "not-a-repo",
		repo:  root,
		stdin: envelope(notRepo, "not-a-repo"),
	})

	// fail-no-default-branch
	tests = append(tests, &testCase{
		_id:               "fail-no-default-branch",
		_expectedExit:     1,
		_expectedHead:     "",
		_expectedReflog:   nil,
		_expectedStdout:   "",
		_expectedUpstream: "",

		name:  "no-default",
		repo:  trunkRoot,
		stdin: envelope(trunkRoot, "no-default"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			exit := run(ctx, homeArgs, &bytes.Buffer{}, strings.NewReader(test.stdin), stdout)
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedStdout, stdout.String())
			assert.Equal(t, test._expectedHead, branchHead(test.repo, branchPrefix+test.name))
			assert.Equal(t, test._expectedReflog, branchReflog(test.repo, branchPrefix+test.name))
			assert.Equal(t, test._expectedUpstream, branchUpstream(test.repo, branchPrefix+test.name))
		})
	}
}

func TestRun_Acdsl(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBinary bool
		_expectedExit   int

		binaryPath string
		stdin      string
	}

	ctx := context.Background()
	home := t.TempDir()
	hookConfig := defaultHookConfig(t)
	writeHookConfig(t, home, hookConfig)
	homeArgs := []string{"-claude-home", home}
	worktreesDir := hookConfig.Worktreecreate.WorktreesDir
	sourceRoot := newFixtureRepo(t, "main")
	require.NoError(t, os.MkdirAll(filepath.Join(sourceRoot, acdslSourceDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceRoot, acdslSourceDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	gitOutput(t, sourceRoot, "add", ".")
	gitOutput(t, sourceRoot, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "acdsl")
	plainRoot := newFixtureRepo(t, "main")

	binary := filepath.Join("bin", "acdsl")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	envelope := func(cwd string, name string) string {
		input := &HookInput{Cwd: cwd, Name: name}
		data, err := json.Marshal(input)
		require.NoError(t, err)
		return string(data)
	}

	tests := make([]*testCase, 0)

	// pass-builds-from-source
	tests = append(tests, &testCase{
		_id:             "pass-builds-from-source",
		_expectedBinary: true,
		_expectedExit:   0,

		binaryPath: filepath.Join(sourceRoot, worktreesDir, "with-source", binary),
		stdin:      envelope(sourceRoot, "with-source"),
	})

	// pass-no-source-no-binary
	tests = append(tests, &testCase{
		_id:             "pass-no-source-no-binary",
		_expectedBinary: false,
		_expectedExit:   0,

		binaryPath: filepath.Join(plainRoot, worktreesDir, "without-source", binary),
		stdin:      envelope(plainRoot, "without-source"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exit := run(ctx, homeArgs, &bytes.Buffer{}, strings.NewReader(test.stdin), &bytes.Buffer{})
			_, statErr := os.Stat(test.binaryPath)
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedBinary, statErr == nil)
		})
	}
}

func TestRunBase(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedExit         int
		_expectedHead         string
		_expectedJournal      []string
		_expectedReflog       []string
		_expectedStderrPrefix string
		_expectedStdout       string
		_keptFile             string

		args  []string
		name  string
		stdin string
	}

	ctx := context.Background()
	home := t.TempDir()
	hookConfig := defaultHookConfig(t)
	hookConfig.Worktreecreate.ResetAttempts = 1
	hookConfig.Worktreecreate.ResetRetryDelayMs = 0
	writeHookConfig(t, home, hookConfig)
	section := &hookConfig.Worktreecreate
	namePattern := regexp.MustCompile(section.NamePattern)
	branchPrefix, shortShaLength, worktreesDir := section.BranchPrefix, section.ShortShaLength, section.WorktreesDir
	root := newFixtureRepo(t, "main")
	rootHead := gitOutput(t, root, "rev-parse", "HEAD")
	gitOutput(t, root, "switch", "-c", "feature/ahead")
	gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-m", "ahead")
	featureHead := gitOutput(t, root, "rev-parse", "HEAD")
	gitOutput(t, root, "switch", "main")
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitOutput(t, root, "init", "-q", "--bare", origin)
	gitOutput(t, root, "remote", "add", "origin", origin)
	gitOutput(t, root, "push", "-q", "origin", "main", "feature/ahead", "feature/ahead:remote-only")
	gitOutput(t, root, "remote", "set-head", "origin", "main")
	gitOutput(t, root, "branch", "feature/equal", "main")
	gitOutput(t, root, "branch", "feature/tagged", "feature/ahead")
	taggedCommit := gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit-tree", "HEAD^{tree}", "-m", "tagged")
	gitOutput(t, root, "tag", "feature/tagged", taggedCommit)
	gitOutput(t, root, "branch", "feature/gone", "feature/ahead")
	storeDir := t.TempDir()
	createdFromMain := []string{"branch: Created from main"}

	worktreeOf := func(name string) string {
		path, err := createWorktree(ctx, &HookInput{Cwd: root, Name: name}, namePattern, section)
		require.NoError(t, err)
		return path
	}
	storeFor := func(name string, path string, sourceBranch string) string {
		entry := map[string]any{
			"branch":    name,
			"hookBased": true,
			"leasedBy":  "local_0001",
			"name":      name,
			"path":      path,
		}
		if sourceBranch != "" {
			entry["sourceBranch"] = sourceBranch
		}
		store := map[string]any{"worktrees": map[string]any{name: entry}}
		data, err := json.Marshal(store)
		require.NoError(t, err)
		storePath := filepath.Join(storeDir, name+".json")
		require.NoError(t, os.WriteFile(storePath, data, 0o600))
		return storePath
	}
	envelope := func(cwd string, source string, agentId string) string {
		input := &HookInput{AgentId: agentId, Cwd: cwd, HookEventName: eventSessionStart, Source: source}
		data, err := json.Marshal(input)
		require.NoError(t, err)
		return string(data)
	}
	baseArgs := func(storePath string) []string {
		return []string{"-base", "-claude-home", home, "-store", storePath}
	}

	tests := make([]*testCase, 0)

	// pass-moved-to-feature
	moved := worktreeOf("moved-to-feature")
	tests = append(tests, &testCase{
		_id:                   "pass-moved-to-feature",
		_expectedExit:         0,
		_expectedHead:         featureHead,
		_expectedJournal:      []string{"recreated from feature/ahead; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from feature/ahead"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "worktree-base: moved claude/moved-to-feature from " + rootHead[:shortShaLength] + " to feature/ahead (" + featureHead[:shortShaLength] + "), the base picked for this session\n",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("moved-to-feature", moved, "feature/ahead")),
		name:  "moved-to-feature",
		stdin: envelope(moved, sourceStartup, ""),
	})

	// pass-already-at-base
	atBase := worktreeOf("already-at-base")
	tests = append(tests, &testCase{
		_id:                   "pass-already-at-base",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("already-at-base", atBase, "main")),
		name:  "already-at-base",
		stdin: envelope(atBase, sourceStartup, ""),
	})

	// pass-default-branch
	defaulted := worktreeOf("default-branch")
	tests = append(tests, &testCase{
		_id:                   "pass-default-branch",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("default-branch", defaulted, "")),
		name:  "default-branch",
		stdin: envelope(defaulted, sourceStartup, ""),
	})

	// pass-at-default-tip
	equalTips := worktreeOf("at-default-tip")
	tests = append(tests, &testCase{
		_id:                   "pass-at-default-tip",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{"recreated from feature/equal; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from feature/equal"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "worktree-base: moved claude/at-default-tip from " + rootHead[:shortShaLength] + " to feature/equal (" + rootHead[:shortShaLength] + "), the base picked for this session\n",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("at-default-tip", equalTips, "feature/equal")),
		name:  "at-default-tip",
		stdin: envelope(equalTips, sourceStartup, ""),
	})

	// pass-second-run
	secondRun := worktreeOf("second-run")
	secondRunArgs := baseArgs(storeFor("second-run", secondRun, "feature/ahead"))
	firstRunExit := run(ctx, secondRunArgs, &bytes.Buffer{}, strings.NewReader(envelope(secondRun, sourceStartup, "")), &bytes.Buffer{})
	require.Equal(t, 0, firstRunExit)
	tests = append(tests, &testCase{
		_id:                   "pass-second-run",
		_expectedExit:         0,
		_expectedHead:         featureHead,
		_expectedJournal:      []string{"recreated from feature/ahead; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from feature/ahead"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  secondRunArgs,
		name:  "second-run",
		stdin: envelope(secondRun, sourceStartup, ""),
	})

	// pass-origin-only-branch
	remoteOnly := worktreeOf("origin-only-branch")
	tests = append(tests, &testCase{
		_id:                   "pass-origin-only-branch",
		_expectedExit:         0,
		_expectedHead:         featureHead,
		_expectedJournal:      []string{"recreated from origin/remote-only; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from origin/remote-only"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "worktree-base: moved claude/origin-only-branch from " + rootHead[:shortShaLength] + " to origin/remote-only (" + featureHead[:shortShaLength] + "), the base picked for this session\n",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("origin-only-branch", remoteOnly, "remote-only")),
		name:  "origin-only-branch",
		stdin: envelope(remoteOnly, sourceStartup, ""),
	})

	// pass-tag-shadows-branch
	tagShadowed := worktreeOf("tag-shadows-branch")
	tests = append(tests, &testCase{
		_id:                   "pass-tag-shadows-branch",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{"recreated from feature/tagged; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from " + rootHead},
		_expectedStderrPrefix: "Aligner.Align: Failed to move claude/tag-shadows-branch to " + featureHead + ": gitRetrying: git switch: ",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("tag-shadows-branch", tagShadowed, "feature/tagged")),
		name:  "tag-shadows-branch",
		stdin: envelope(tagShadowed, sourceStartup, ""),
	})

	// pass-own-commit
	owned := worktreeOf("own-commit")
	gitOutput(t, owned, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-m", "own")
	ownHead := gitOutput(t, owned, "rev-parse", "HEAD")
	tests = append(tests, &testCase{
		_id:                   "pass-own-commit",
		_expectedExit:         0,
		_expectedHead:         ownHead,
		_expectedJournal:      []string{},
		_expectedReflog:       []string{"commit: own", "branch: Created from main"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("own-commit", owned, "feature/ahead")),
		name:  "own-commit",
		stdin: envelope(owned, sourceStartup, ""),
	})

	// pass-dirty-tree
	dirty := worktreeOf("dirty-tree")
	require.NoError(t, os.WriteFile(filepath.Join(dirty, "tracked.txt"), []byte("changed\n"), 0o644))
	tests = append(tests, &testCase{
		_id:                   "pass-dirty-tree",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "",
		_expectedStdout:       "worktree-base: claude/dirty-tree sits at " + rootHead[:shortShaLength] + ", the session's base is feature/ahead (" + featureHead[:shortShaLength] + "); not moved: the tracked tree has changes\n",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("dirty-tree", dirty, "feature/ahead")),
		name:  "dirty-tree",
		stdin: envelope(dirty, sourceStartup, ""),
	})

	// pass-untracked-only
	untracked := worktreeOf("untracked-only")
	require.NoError(t, os.WriteFile(filepath.Join(untracked, ".claude-worktree"), []byte("sentinel\n"), 0o644))
	tests = append(tests, &testCase{
		_id:                   "pass-untracked-only",
		_expectedExit:         0,
		_expectedHead:         featureHead,
		_expectedJournal:      []string{"recreated from feature/ahead; previous sha " + rootHead},
		_expectedReflog:       []string{"branch: Created from feature/ahead"},
		_expectedStderrPrefix: "",
		_expectedStdout:       "worktree-base: moved claude/untracked-only from " + rootHead[:shortShaLength] + " to feature/ahead (" + featureHead[:shortShaLength] + "), the base picked for this session\n",
		_keptFile:             filepath.Join(untracked, ".claude-worktree"),

		args:  baseArgs(storeFor("untracked-only", untracked, "feature/ahead")),
		name:  "untracked-only",
		stdin: envelope(untracked, sourceStartup, ""),
	})

	// pass-not-startup
	resumed := worktreeOf("not-startup")
	tests = append(tests, &testCase{
		_id:                   "pass-not-startup",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("not-startup", resumed, "feature/ahead")),
		name:  "not-startup",
		stdin: envelope(resumed, "resume", ""),
	})

	// pass-subagent
	agent := worktreeOf("subagent")
	tests = append(tests, &testCase{
		_id:                   "pass-subagent",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("subagent", agent, "feature/ahead")),
		name:  "subagent",
		stdin: envelope(agent, sourceStartup, "a1"),
	})

	// pass-outside-pool
	tests = append(tests, &testCase{
		_id:                   "pass-outside-pool",
		_expectedExit:         0,
		_expectedHead:         "",
		_expectedJournal:      []string{},
		_expectedReflog:       nil,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("outside-pool", root, "feature/ahead")),
		name:  "outside-pool",
		stdin: envelope(root, sourceStartup, ""),
	})

	// pass-branch-mismatch
	mismatch := filepath.Join(root, worktreesDir, "branch-mismatch")
	gitOutput(t, root, "worktree", "add", "-b", "claude/some-other-name", mismatch, "main")
	tests = append(tests, &testCase{
		_id:                   "pass-branch-mismatch",
		_expectedExit:         0,
		_expectedHead:         "",
		_expectedJournal:      []string{},
		_expectedReflog:       nil,
		_expectedStderrPrefix: "",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("branch-mismatch", mismatch, "feature/ahead")),
		name:  "branch-mismatch",
		stdin: envelope(mismatch, sourceStartup, ""),
	})

	// pass-no-entry
	noEntry := worktreeOf("no-entry")
	tests = append(tests, &testCase{
		_id:                   "pass-no-entry",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "desktop.ReadWorktree: ",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("someone-else", noEntry, "feature/ahead")),
		name:  "no-entry",
		stdin: envelope(noEntry, sourceStartup, ""),
	})

	// pass-entry-other-path
	otherPath := worktreeOf("entry-other-path")
	tests = append(tests, &testCase{
		_id:                   "pass-entry-other-path",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "Aligner.Align: Store entry \"entry-other-path\" is not this worktree",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("entry-other-path", moved, "feature/ahead")),
		name:  "entry-other-path",
		stdin: envelope(otherPath, sourceStartup, ""),
	})

	// pass-unknown-base
	unknownBase := worktreeOf("unknown-base")
	tests = append(tests, &testCase{
		_id:                   "pass-unknown-base",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "resolveBase: Source branch \"nope\" names no local or origin branch",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("unknown-base", unknownBase, "nope")),
		name:  "unknown-base",
		stdin: envelope(unknownBase, sourceStartup, ""),
	})

	// pass-picked-branch-deleted
	pickedGone := worktreeOf("picked-branch-deleted")
	pickedGoneArgs := baseArgs(storeFor("picked-branch-deleted", pickedGone, "feature/gone"))
	gitOutput(t, root, "branch", "-q", "-D", "feature/gone")
	tests = append(tests, &testCase{
		_id:                   "pass-picked-branch-deleted",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "resolveBase: Source branch \"feature/gone\" names no local or origin branch",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  pickedGoneArgs,
		name:  "picked-branch-deleted",
		stdin: envelope(pickedGone, sourceStartup, ""),
	})

	// pass-no-store
	noStore := worktreeOf("no-store")
	tests = append(tests, &testCase{
		_id:                   "pass-no-store",
		_expectedExit:         0,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "desktop.ReadWorktree: Failed to read ",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(filepath.Join(storeDir, "absent.json")),
		name:  "no-store",
		stdin: envelope(noStore, sourceStartup, ""),
	})

	// reset-attempts-from-config
	locked := worktreeOf("reset-attempts")
	lockPath := gitOutput(t, locked, "rev-parse", "--path-format=absolute", "--git-path", "index.lock")
	require.NoError(t, os.WriteFile(lockPath, nil, 0o644))
	tests = append(tests, &testCase{
		_id:                   "reset-attempts-from-config",
		_expectedExit:         0,
		_expectedHead:         "",
		_expectedJournal:      []string{"recreated from feature/ahead; previous sha " + rootHead},
		_expectedReflog:       nil,
		_expectedStderrPrefix: "Aligner.Align: Failed to move claude/reset-attempts to " + featureHead + ": recreateBranch: gitRetrying: git switch: ",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(storeFor("reset-attempts", locked, "feature/ahead")),
		name:  "reset-attempts",
		stdin: envelope(locked, sourceStartup, ""),
	})

	// fail-missing-config
	missingConfigWorktree := worktreeOf("missing-config")
	tests = append(tests, &testCase{
		_id:                   "fail-missing-config",
		_expectedExit:         1,
		_expectedHead:         rootHead,
		_expectedJournal:      []string{},
		_expectedReflog:       createdFromMain,
		_expectedStderrPrefix: "run: loadSection: Load: Failed to read ",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  []string{"-base", "-claude-home", t.TempDir(), "-store", storeFor("missing-config", missingConfigWorktree, "feature/ahead")},
		name:  "missing-config",
		stdin: envelope(missingConfigWorktree, sourceStartup, ""),
	})

	// fail-relative-cwd
	tests = append(tests, &testCase{
		_id:                   "fail-relative-cwd",
		_expectedExit:         1,
		_expectedHead:         "",
		_expectedJournal:      []string{},
		_expectedReflog:       nil,
		_expectedStderrPrefix: "HookInput.Validate: Invalid field Cwd",
		_expectedStdout:       "",
		_keptFile:             filepath.Join(root, "tracked.txt"),

		args:  baseArgs(filepath.Join(storeDir, "absent.json")),
		name:  "relative-cwd",
		stdin: envelope("repo", sourceStartup, ""),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exit := run(ctx, test.args, stderr, strings.NewReader(test.stdin), stdout)
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedStdout, stdout.String())
			assert.Truef(t, strings.HasPrefix(stderr.String(), test._expectedStderrPrefix), "stderr = %q", stderr.String())
			assert.Equal(t, test._expectedStderrPrefix == "", stderr.Len() == 0)
			assert.Equal(t, test._expectedHead, branchHead(root, branchPrefix+test.name))
			assert.Equal(t, test._expectedReflog, branchReflog(root, branchPrefix+test.name))
			assert.Equal(t, test._expectedJournal, recreatedReasons(t, home, branchPrefix+test.name))
			assert.FileExists(t, test._keptFile)
		})
	}
}

func TestRun_BranchPrefixFromConfig(t *testing.T) {
	home := t.TempDir()
	hookConfig := defaultHookConfig(t)
	hookConfig.Worktreecreate.BranchPrefix = "bot/"
	writeHookConfig(t, home, hookConfig)
	root := newFixtureRepo(t, "main")
	rootHead := gitOutput(t, root, "rev-parse", "HEAD")
	stdin := `{"cwd":` + encodeJson(t, root) + `,"name":"prefixed"}`
	stdout := &bytes.Buffer{}

	exit := run(context.Background(), []string{"-claude-home", home}, &bytes.Buffer{}, strings.NewReader(stdin), stdout)

	assert.Equal(t, 0, exit)
	assert.Equal(t, filepath.Join(root, hookConfig.Worktreecreate.WorktreesDir, "prefixed")+"\n", stdout.String())
	assert.Equal(t, rootHead, branchHead(root, "bot/prefixed"))
	assert.Empty(t, branchHead(root, "claude/prefixed"))
}

// branchHead returns the commit the branch points at, empty when absent.
func branchHead(root string, branch string) string {
	output, err := shell.Run(context.Background(), root, "git", "rev-parse", "--verify", "--quiet", branch)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// branchReflog returns the subjects of the branch's reflog, newest first,
// nil when the branch is absent.
func branchReflog(root string, branch string) []string {
	output, err := shell.Run(context.Background(), root, "git", "log", "-g", "--format=%gs", branch)
	if err != nil {
		return nil
	}

	subjects := strings.TrimSpace(output)
	if subjects == "" {
		return nil
	}
	return strings.Split(subjects, "\n")
}

// branchUpstream returns the upstream of the branch, empty when it has
// none or is absent.
func branchUpstream(root string, branch string) string {
	output, err := shell.Run(context.Background(), root, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", branch+"@{upstream}")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(output)
}

// defaultHookConfig is the hooks config the install renders from every catalog default.
func defaultHookConfig(t *testing.T) *hookconfig.Config {
	t.Helper()
	hookConfig := &hookconfig.Config{}
	for _, component := range components.Catalog() {
		rendered, err := install.RenderDefaults(component.ID, components.SinkHooksConfig)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(rendered, hookConfig))
	}
	return hookConfig
}

func encodeJson(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

// recreatedReasons lists the reasons of the branch-recreated journal lines for branch under claudeHome's state dir, oldest first.
func recreatedReasons(t *testing.T, claudeHome string, branch string) []string {
	t.Helper()
	stateDir, err := config.StateDir(claudeHome)
	require.NoError(t, err)
	journal, err := trash.New(stateDir, 0).Tail(1000)
	require.NoError(t, err)
	reasons := make([]string, 0)
	for _, entry := range slices.Backward(journal) {
		isRecreated := entry.Kind == trash.KindBranchRecreated
		if isRecreated && strings.HasSuffix(entry.Path, "#"+branch) {
			reasons = append(reasons, entry.Reason)
		}
	}
	return reasons
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// newFixtureRepo returns the symlink-resolved root of a repo with one commit
// holding tracked.txt on the given branch, matching the absolute paths git
// reports.
func newFixtureRepo(t *testing.T, branch string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitOutput(t, root, "init", "-q", "-b", branch)
	require.NoError(t, os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644))
	gitOutput(t, root, "add", "tracked.txt")
	gitOutput(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init")
	return root
}

// writeHookConfig writes the config where hookconfig.Load reads it.
func writeHookConfig(t *testing.T, home string, hookConfig *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(hookconfig.Path(home)), 0o755))
	require.NoError(t, os.WriteFile(hookconfig.Path(home), encoded, 0o644))
}
