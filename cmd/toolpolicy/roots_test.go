package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/toolpolicy"
)

// rootsFixture is the on-disk layout the roots, fence and anchor tests share:
// a main checkout with one linked worktree (the anchor), a foreign repo, a
// home with the harness dirs and a session temp dir with a sibling session.
type rootsFixture struct {
	anchor          string
	artifacts       string
	base            string
	extra           string
	foreign         string
	headlessTemp    string
	home            string
	mainCheckout    string
	otherScratchpad string
	policy          *toolpolicy.Policy
	scratchpad      string
	sessionTemp     string
	sibling         string
	transcript      string
}

// fixtureBase returns a fresh, symlink-resolved directory outside the gate's
// tmp dirs: t.TempDir() sits under /tmp on Linux, where canWrite answers
// every outside write with the scratchpad steer.
func fixtureBase(t *testing.T) string {
	t.Helper()
	cacheDir, err := os.UserCacheDir()
	require.NoError(t, err)

	base, err := os.MkdirTemp(cacheDir, "toolpolicy-test-")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, os.RemoveAll(base)) })

	resolved, err := filepath.EvalSymlinks(base)
	require.NoError(t, err)

	return resolved
}

// fixtureSessionRoots builds the shared layout under a fresh temp dir.
func fixtureSessionRoots(t *testing.T) *rootsFixture {
	t.Helper()
	base := fixtureBase(t)

	policy := defaultToolpolicy(t)

	mainCheckout := filepath.Join(base, "repo")
	home := filepath.Join(base, "home")
	sessionTemp := filepath.Join(base, "tmp", "claude-1", "proj", "sid")
	fixture := &rootsFixture{
		anchor:          filepath.Join(mainCheckout, ".claude", "worktrees", "wt"),
		artifacts:       filepath.Join(home, ".cache", "claude-routine", "artifacts"),
		base:            base,
		extra:           filepath.Join(base, "extra"),
		foreign:         filepath.Join(base, "foreign"),
		headlessTemp:    filepath.Join(sessionTempBase, sessionTempPrefix+strconv.Itoa(os.Getuid()), "p", "sid"),
		home:            home,
		mainCheckout:    mainCheckout,
		otherScratchpad: filepath.Join(base, "tmp", "claude-1", "proj", "sid2", "scratchpad"),
		policy:          policy,
		scratchpad:      filepath.Join(sessionTemp, "scratchpad"),
		sessionTemp:     sessionTemp,
		sibling:         filepath.Join(mainCheckout, ".claude", "worktrees", "wt2"),
		transcript:      filepath.Join(home, ".claude", "projects", "p", "sid.jsonl"),
	}

	dirs := []string{
		filepath.Join(mainCheckout, ".git", "worktrees", "wt"),
		filepath.Join(mainCheckout, ".git", "worktrees", "wt2"),
		filepath.Join(fixture.anchor, "sub"),
		fixture.sibling,
		filepath.Join(fixture.foreign, ".git"),
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".claude", "plans"),
		filepath.Join(home, ".claude", "projects", "p", "memory"),
		fixture.artifacts,
		fixture.extra,
		fixture.scratchpad,
		filepath.Join(sessionTemp, "images"),
		filepath.Join(sessionTemp, "tasks"),
		fixture.otherScratchpad,
	}
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}

	pointers := map[string]string{
		fixture.anchor:  filepath.Join(mainCheckout, ".git", "worktrees", "wt"),
		fixture.sibling: filepath.Join(mainCheckout, ".git", "worktrees", "wt2"),
	}
	for worktree, gitDir := range pointers {
		require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	}
	require.NoError(t, os.Symlink(fixture.foreign, filepath.Join(fixture.anchor, "out")))
	require.NoError(t, os.Symlink(fixture.anchor, filepath.Join(base, "into-anchor")))
	return fixture
}

// grants returns fixture grants anchored at the worktree, with the extra dir
// and the artifacts root as additional directories, the foreign repo and
// the own main checkout registered, and the given skill rules invoked.
func (f *rootsFixture) grants(skillAllow ...string) *Grants {
	grants := &Grants{
		anchor:                sessionAnchor{path: f.anchor, source: anchorSourceTranscript},
		repoPaths:             foreignRepoPaths(f.anchor, []string{f.foreign, f.mainCheckout}),
		worktreesDir:          harnessWorktreesDir,
		AdditionalDirectories: []string{f.extra, f.artifacts},
		Approvals:             map[string]bool{},
		SkillAllow:            parseRules(skillAllow, "fixture"),
	}
	return grants
}

// input is a Bash envelope at cwd inside the fixture session.
func (f *rootsFixture) input(cwd string, command string) *HookInput {
	envelope := &HookInput{
		Cwd:            cwd,
		PermissionMode: permissionModeAcceptEdits,
		ScratchpadDir:  f.scratchpad,
		SessionId:      "sid",
		ToolInput:      ToolInput{Command: command},
		ToolName:       toolNameBash,
		TranscriptPath: f.transcript,
	}
	return envelope
}

// headlessInput is a Bash envelope without a scratchpad, as a routine chat cell sends it.
func (f *rootsFixture) headlessInput(cwd string, command string) *HookInput {
	envelope := f.input(cwd, command)
	envelope.ScratchpadDir = ""
	return envelope
}

// headlessRoots builds the session roots of a headless envelope at the anchor.
func (f *rootsFixture) headlessRoots(grants *Grants) *sessionRoots {
	return newSessionRoots(grants, f.home, f.headlessInput(f.anchor, ""), f.policy)
}

// fencedRoots builds the session roots of a confined session: the foreign
// repo and the plans archive under home are fenced.
func (f *rootsFixture) fencedRoots(t *testing.T, grants *Grants) *sessionRoots {
	t.Helper()
	policy := defaultToolpolicy(t)

	policy.ReadFence = []string{f.foreign, filepath.Join(f.home, "GolandProjects", "agent-plans")}
	return newSessionRoots(grants, f.home, f.input(f.anchor, ""), policy)
}

// roots builds the session roots for grants at the anchor.
func (f *rootsFixture) roots(grants *Grants) *sessionRoots {
	return newSessionRoots(grants, f.home, f.input(f.anchor, ""), f.policy)
}

func TestSessionTempRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		input *HookInput
		uid   int
	}

	tests := make([]*testCase, 0)

	// derived
	tests = append(tests, &testCase{
		_id:       "derived",
		_expected: filepath.Join("/tmp", "claude-1", "*", "sid"),

		input: &HookInput{SessionId: "sid"},
		uid:   1,
	})

	// scratchpad-ignored
	tests = append(tests, &testCase{
		_id:       "scratchpad-ignored",
		_expected: filepath.Join("/tmp", "claude-1", "*", "sid"),

		input: &HookInput{ScratchpadDir: "/elsewhere/scratchpad", SessionId: "sid"},
		uid:   1,
	})

	// no-session
	tests = append(tests, &testCase{
		_id:       "no-session",
		_expected: "",

		input: &HookInput{TranscriptPath: "/home/.claude/projects/p/sid.jsonl"},
		uid:   1,
	})

	// glob-in-session-id
	tests = append(tests, &testCase{
		_id:       "glob-in-session-id",
		_expected: "",

		input: &HookInput{SessionId: "s*d"},
		uid:   1,
	})

	// negative-uid
	tests = append(tests, &testCase{
		_id:       "negative-uid",
		_expected: "",

		input: &HookInput{SessionId: "sid"},
		uid:   -1,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionTempRoot(test.input, test.uid))
		})
	}
}

func TestSessionRoots_CanRead(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCause  string
		_expectedReason string

		path  string
		roots *sessionRoots
	}

	fixture := fixtureSessionRoots(t)
	plain := fixture.roots(fixture.grants())
	headless := fixture.headlessRoots(fixture.grants())
	removedGrants := fixture.grants()
	removedGrants.anchor.isMissing = true

	tests := make([]*testCase, 0)

	// anchor-file
	tests = append(tests, &testCase{
		_id:             "anchor-file",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.anchor, "main.go"),
		roots: plain,
	})

	// anchor-subdir-relative
	tests = append(tests, &testCase{
		_id:             "anchor-subdir-relative",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "sub/x.go",
		roots: plain,
	})

	// dotdot-escape-denied
	tests = append(tests, &testCase{
		_id:             "dotdot-escape-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  "../../../x.go",
		roots: plain,
	})

	// symlink-escape-denied
	tests = append(tests, &testCase{
		_id:             "symlink-escape-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  "out/x.go",
		roots: plain,
	})

	// symlink-into-roots-allowed
	tests = append(tests, &testCase{
		_id:             "symlink-into-roots-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.base, "into-anchor", "x.go"),
		roots: plain,
	})

	// scratchpad
	tests = append(tests, &testCase{
		_id:             "scratchpad",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.scratchpad, "notes.md"),
		roots: plain,
	})

	// session-temp-dir-images
	tests = append(tests, &testCase{
		_id:             "session-temp-dir-images",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.sessionTemp, "images", "a.png"),
		roots: plain,
	})

	// session-temp-dir-tasks-output
	tests = append(tests, &testCase{
		_id:             "session-temp-dir-tasks-output",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.sessionTemp, "tasks", "b1.output"),
		roots: plain,
	})

	// other-session-scratchpad-denied
	tests = append(tests, &testCase{
		_id:             "other-session-scratchpad-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.otherScratchpad, "notes.md"),
		roots: plain,
	})

	// plain-derived-tasks-output
	tests = append(tests, &testCase{
		_id:             "plain-derived-tasks-output",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.headlessTemp, "tasks", "w1.output"),
		roots: plain,
	})

	// headless-tasks-output
	tests = append(tests, &testCase{
		_id:             "headless-tasks-output",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.headlessTemp, "tasks", "w1.output"),
		roots: headless,
	})

	// headless-other-project-dir
	tests = append(tests, &testCase{
		_id:             "headless-other-project-dir",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(filepath.Dir(filepath.Dir(fixture.headlessTemp)), "other", "sid", "tasks", "w1.output"),
		roots: headless,
	})

	// headless-sibling-session-denied
	tests = append(tests, &testCase{
		_id:             "headless-sibling-session-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(filepath.Dir(fixture.headlessTemp), "sid2", "tasks", "w1.output"),
		roots: headless,
	})

	// headless-project-dir-file-denied
	tests = append(tests, &testCase{
		_id:             "headless-project-dir-file-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(filepath.Dir(fixture.headlessTemp), "notes.md"),
		roots: headless,
	})

	// headless-scratchpad-absent
	tests = append(tests, &testCase{
		_id:             "headless-scratchpad-absent",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.scratchpad, "notes.md"),
		roots: headless,
	})

	// additional-dir
	tests = append(tests, &testCase{
		_id:             "additional-dir",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.extra, "x.md"),
		roots: plain,
	})

	// extra-read-claude
	tests = append(tests, &testCase{
		_id:             "extra-read-claude",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/.claude/settings.json",
		roots: plain,
	})

	// go-pkg-mod
	tests = append(tests, &testCase{
		_id:             "go-pkg-mod",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/go/pkg/mod/mvdan.cc/sh/v3@v3.14.1/go.mod",
		roots: plain,
	})

	// go-bin
	tests = append(tests, &testCase{
		_id:             "go-bin",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/go/bin/smine-agentwt",
		roots: plain,
	})

	// cargo-registry
	tests = append(tests, &testCase{
		_id:             "cargo-registry",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/.cargo/registry/src/x/lib.rs",
		roots: plain,
	})

	// cargo-root-denied
	tests = append(tests, &testCase{
		_id:             "cargo-root-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  "~/.cargo/config.toml",
		roots: plain,
	})

	// usr-bin-system
	tests = append(tests, &testCase{
		_id:             "usr-bin-system",
		_expectedCause:  "",
		_expectedReason: systemReadReason,

		path:  "/usr/bin/env",
		roots: plain,
	})

	// opt-homebrew-system
	tests = append(tests, &testCase{
		_id:             "opt-homebrew-system",
		_expectedCause:  "",
		_expectedReason: systemReadReason,

		path:  "/opt/homebrew/bin/jq",
		roots: plain,
	})

	// etc-hosts-denied
	tests = append(tests, &testCase{
		_id:             "etc-hosts-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  "/etc/hosts",
		roots: plain,
	})

	// home-documents-denied
	tests = append(tests, &testCase{
		_id:             "home-documents-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  "~/Documents/x.txt",
		roots: plain,
	})

	// main-checkout-denied
	tests = append(tests, &testCase{
		_id:             "main-checkout-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.mainCheckout, "README.md"),
		roots: plain,
	})

	// foreign-repo-denied
	tests = append(tests, &testCase{
		_id:             "foreign-repo-denied",
		_expectedCause:  causeReadOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.foreign, "session", "session.go"),
		roots: plain,
	})

	// skill-read-grant-reason
	tests = append(tests, &testCase{
		_id:             "skill-read-grant-reason",
		_expectedCause:  "",
		_expectedReason: skillGrantReason + "Read(/" + fixture.foreign + "/**)",

		path:  filepath.Join(fixture.foreign, "go.mod"),
		roots: fixture.roots(fixture.grants("Read(/" + fixture.foreign + "/**)")),
	})

	// skill-repos-grant-reason
	tests = append(tests, &testCase{
		_id:             "skill-repos-grant-reason",
		_expectedCause:  "",
		_expectedReason: skillGrantReason + "Read(@repos/**)",

		path:  filepath.Join(fixture.foreign, "go.mod"),
		roots: fixture.roots(fixture.grants("Read(@repos/**)")),
	})

	// removed-anchor-cause
	tests = append(tests, &testCase{
		_id:             "removed-anchor-cause",
		_expectedCause:  causeAnchorRemoved,
		_expectedReason: "",

		path:  filepath.Join(fixture.mainCheckout, "README.md"),
		roots: fixture.roots(removedGrants),
	})

	// fenced-root-beats-skill-read-grant
	fenced := fixture.fencedRoots(t, fixture.grants("Read(/"+fixture.foreign+"/**)", "Read(~/GolandProjects/agent-plans/**)"))
	tests = append(tests, &testCase{
		_id:             "fenced-root-beats-skill-read-grant",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  filepath.Join(fixture.foreign, "go.mod"),
		roots: fenced,
	})

	// fenced-relative-traversal
	tests = append(tests, &testCase{
		_id:             "fenced-relative-traversal",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  "../../../../foreign/go.mod",
		roots: fenced,
	})

	// fenced-through-symlink
	tests = append(tests, &testCase{
		_id:             "fenced-through-symlink",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  "out/go.mod",
		roots: fenced,
	})

	// fenced-home-root-tilde-spelling
	tests = append(tests, &testCase{
		_id:             "fenced-home-root-tilde-spelling",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  "~/GolandProjects/agent-plans/repo/proposals/problems.json",
		roots: fenced,
	})

	// fenced-session-keeps-its-own-roots
	tests = append(tests, &testCase{
		_id:             "fenced-session-keeps-its-own-roots",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "store/proposals/problems.json",
		roots: fenced,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			access := test.roots.canRead(fixture.anchor, test.path)
			assert.Equal(t, test._expectedCause, access.cause)
			assert.Equal(t, test._expectedReason, access.reason)
		})
	}
}

func TestSessionRoots_CanWrite(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCause  string
		_expectedReason string

		path  string
		roots *sessionRoots
	}

	fixture := fixtureSessionRoots(t)
	plain := fixture.roots(fixture.grants())
	headless := fixture.headlessRoots(fixture.grants())
	unlockedGrants := fixture.grants()
	unlockedGrants.Approvals = map[string]bool{unlockMakefile: true}
	acdslGrants := fixture.grants()
	acdslGrants.AdditionalDirectories = append(acdslGrants.AdditionalDirectories, filepath.Join(fixture.home, ".claude", "acdsl"))
	foreignEdit := "Edit(/" + fixture.foreign + "/**)"
	foreignRead := "Read(/" + fixture.foreign + "/**)"

	tests := make([]*testCase, 0)

	// anchor-file
	tests = append(tests, &testCase{
		_id:             "anchor-file",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "sub/x.go",
		roots: plain,
	})

	// scratchpad
	tests = append(tests, &testCase{
		_id:             "scratchpad",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.scratchpad, "out.md"),
		roots: plain,
	})

	// session-temp-dir-denied
	tests = append(tests, &testCase{
		_id:             "session-temp-dir-denied",
		_expectedCause:  causeWriteOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.sessionTemp, "images", "a.png"),
		roots: plain,
	})

	// tmp-steer-cause
	tests = append(tests, &testCase{
		_id:             "tmp-steer-cause",
		_expectedCause:  causeTmpWrite,
		_expectedReason: "",

		path:  "/tmp/x",
		roots: plain,
	})

	// private-tmp-steer-cause
	tests = append(tests, &testCase{
		_id:             "private-tmp-steer-cause",
		_expectedCause:  causeTmpWrite,
		_expectedReason: "",

		path:  "/private/tmp/x",
		roots: plain,
	})

	// headless-tasks-output-steer
	tests = append(tests, &testCase{
		_id:             "headless-tasks-output-steer",
		_expectedCause:  causeTmpWrite,
		_expectedReason: "",

		path:  filepath.Join(fixture.headlessTemp, "tasks", "w1.output"),
		roots: headless,
	})

	// dev-null
	tests = append(tests, &testCase{
		_id:             "dev-null",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "/dev/null",
		roots: plain,
	})

	// dev-fd
	tests = append(tests, &testCase{
		_id:             "dev-fd",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "/dev/fd/2",
		roots: plain,
	})

	// system-usr-denied
	tests = append(tests, &testCase{
		_id:             "system-usr-denied",
		_expectedCause:  causeWriteOutside,
		_expectedReason: "",

		path:  "/usr/local/x",
		roots: plain,
	})

	// extra-read-dir-denied
	tests = append(tests, &testCase{
		_id:             "extra-read-dir-denied",
		_expectedCause:  causeWriteOutside,
		_expectedReason: "",

		path:  "~/go/pkg/mod/x",
		roots: plain,
	})

	// plans-harness-allowed
	tests = append(tests, &testCase{
		_id:             "plans-harness-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/.claude/plans/x.md",
		roots: plain,
	})

	// memory-glob-harness-allowed
	tests = append(tests, &testCase{
		_id:             "memory-glob-harness-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "~/.claude/projects/p/memory/m.md",
		roots: plain,
	})

	// settings-local-gate-protected
	tests = append(tests, &testCase{
		_id:             "settings-local-gate-protected",
		_expectedCause:  causeSettingsViaCommand,
		_expectedReason: "",

		path:  ".claude/settings.local.json",
		roots: plain,
	})

	// claude-hooks-gate-protected
	tests = append(tests, &testCase{
		_id:             "claude-hooks-gate-protected",
		_expectedCause:  causeProtectedPath,
		_expectedReason: "",

		path:  "~/.claude/hooks/bin/toolpolicy",
		roots: plain,
	})

	// transcript-gate-protected
	tests = append(tests, &testCase{
		_id:             "transcript-gate-protected",
		_expectedCause:  causeProtectedPath,
		_expectedReason: "",

		path:  fixture.transcript,
		roots: plain,
	})

	// makefile-locked
	tests = append(tests, &testCase{
		_id:             "makefile-locked",
		_expectedCause:  approvableCause(unlockMakefile),
		_expectedReason: "",

		path:  "Makefile",
		roots: plain,
	})

	// makefile-unlocked-allowed
	tests = append(tests, &testCase{
		_id:             "makefile-unlocked-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "Makefile",
		roots: fixture.roots(unlockedGrants),
	})

	// git-dir-gate-protected
	tests = append(tests, &testCase{
		_id:             "git-dir-gate-protected",
		_expectedCause:  causeGitDir,
		_expectedReason: "",

		path:  "sub/.git/config",
		roots: plain,
	})

	// git-pointer-file-gate-protected
	tests = append(tests, &testCase{
		_id:             "git-pointer-file-gate-protected",
		_expectedCause:  causeGitDir,
		_expectedReason: "",

		path:  ".git",
		roots: plain,
	})

	// gitignore-allowed
	tests = append(tests, &testCase{
		_id:             "gitignore-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  ".gitignore",
		roots: plain,
	})

	// claude-worktrees-exempt
	tests = append(tests, &testCase{
		_id:             "claude-worktrees-exempt",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  filepath.Join(fixture.anchor, "sub", "x.go"),
		roots: plain,
	})

	// worktree-own-claude-dir-protected
	tests = append(tests, &testCase{
		_id:             "worktree-own-claude-dir-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".claude/commands/x.md",
		roots: plain,
	})

	// config-git-protected
	tests = append(tests, &testCase{
		_id:             "config-git-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".config/git/ignore",
		roots: plain,
	})

	// config-other-allowed
	tests = append(tests, &testCase{
		_id:             "config-other-allowed",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  ".config/tool/x.toml",
		roots: plain,
	})

	// zshrc-protected
	tests = append(tests, &testCase{
		_id:             "zshrc-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".zshrc",
		roots: plain,
	})

	// yarnrc-pattern-protected
	tests = append(tests, &testCase{
		_id:             "yarnrc-pattern-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".yarnrc.yml",
		roots: plain,
	})

	// lefthook-pattern-protected
	tests = append(tests, &testCase{
		_id:             "lefthook-pattern-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  "lefthook-local.yml",
		roots: plain,
	})

	// mcp-json-protected
	tests = append(tests, &testCase{
		_id:             "mcp-json-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".mcp.json",
		roots: plain,
	})

	// vscode-protected
	tests = append(tests, &testCase{
		_id:             "vscode-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  ".vscode/settings.json",
		roots: plain,
	})

	// skill-edit-grant-allowed
	tests = append(tests, &testCase{
		_id:             "skill-edit-grant-allowed",
		_expectedCause:  "",
		_expectedReason: skillGrantReason + foreignEdit,

		path:  filepath.Join(fixture.foreign, "x.md"),
		roots: fixture.roots(fixture.grants(foreignEdit)),
	})

	// skill-read-grant-write-denied
	tests = append(tests, &testCase{
		_id:             "skill-read-grant-write-denied",
		_expectedCause:  causeWriteOutside,
		_expectedReason: "",

		path:  filepath.Join(fixture.foreign, "x.md"),
		roots: fixture.roots(fixture.grants(foreignRead)),
	})

	// skill-grant-on-protected-denied
	tests = append(tests, &testCase{
		_id:             "skill-grant-on-protected-denied",
		_expectedCause:  causeProtectedPath,
		_expectedReason: "",

		path:  "~/.claude/skills/jq/SKILL.md",
		roots: fixture.roots(fixture.grants("Edit(~/.claude/skills/**)")),
	})

	// acdsl-additional-dir-engine-protected
	tests = append(tests, &testCase{
		_id:             "acdsl-additional-dir-engine-protected",
		_expectedCause:  causeEngineProtected,
		_expectedReason: "",

		path:  "~/.claude/acdsl/rules.acdsl",
		roots: fixture.roots(acdslGrants),
	})

	// fenced-root-beats-skill-edit-grant
	fenced := fixture.fencedRoots(t, fixture.grants(foreignEdit, "Edit(~/GolandProjects/agent-plans/**)"))
	tests = append(tests, &testCase{
		_id:             "fenced-root-beats-skill-edit-grant",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  filepath.Join(fixture.foreign, "x.md"),
		roots: fenced,
	})

	// fenced-home-root-tilde-spelling
	tests = append(tests, &testCase{
		_id:             "fenced-home-root-tilde-spelling",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  "~/GolandProjects/agent-plans/repo/proposals/context.json",
		roots: fenced,
	})

	// fenced-relative-traversal
	tests = append(tests, &testCase{
		_id:             "fenced-relative-traversal",
		_expectedCause:  causeFenced,
		_expectedReason: "",

		path:  "../../../../foreign/x.md",
		roots: fenced,
	})

	// fenced-session-keeps-its-own-store
	tests = append(tests, &testCase{
		_id:             "fenced-session-keeps-its-own-store",
		_expectedCause:  "",
		_expectedReason: rootsAllowReason,

		path:  "store/proposals/context.json",
		roots: fenced,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			access := test.roots.canWrite(fixture.anchor, test.path)
			assert.Equal(t, test._expectedCause, access.cause)
			assert.Equal(t, test._expectedReason, access.reason)
		})
	}
}

func TestSessionRoots_IsCriticalRemoval(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		text string
	}

	fixture := fixtureSessionRoots(t)
	roots := fixture.roots(fixture.grants())

	tests := make([]*testCase, 0)

	// root
	tests = append(tests, &testCase{
		_id:       "root",
		_expected: true,

		text: "/",
	})

	// top-level-users
	tests = append(tests, &testCase{
		_id:       "top-level-users",
		_expected: true,

		text: "/Users",
	})

	// top-level-tmp-symlink
	tests = append(tests, &testCase{
		_id:       "top-level-tmp-symlink",
		_expected: true,

		text: "/tmp",
	})

	// home
	tests = append(tests, &testCase{
		_id:       "home",
		_expected: true,

		text: fixture.home,
	})

	// home-tilde
	tests = append(tests, &testCase{
		_id:       "home-tilde",
		_expected: true,

		text: "~",
	})

	// anchor
	tests = append(tests, &testCase{
		_id:       "anchor",
		_expected: true,

		text: fixture.anchor,
	})

	// anchor-parent
	tests = append(tests, &testCase{
		_id:       "anchor-parent",
		_expected: true,

		text: filepath.Dir(fixture.anchor),
	})

	// cwd-dot
	tests = append(tests, &testCase{
		_id:       "cwd-dot",
		_expected: true,

		text: ".",
	})

	// cwd-dotdot
	tests = append(tests, &testCase{
		_id:       "cwd-dotdot",
		_expected: true,

		text: "..",
	})

	// worktree-file-not-critical
	tests = append(tests, &testCase{
		_id:       "worktree-file-not-critical",
		_expected: false,

		text: "sub/x.go",
	})

	// glob-root
	tests = append(tests, &testCase{
		_id:       "glob-root",
		_expected: true,

		text: "/*",
	})

	// glob-home
	tests = append(tests, &testCase{
		_id:       "glob-home",
		_expected: true,

		text: "~/*",
	})

	// glob-in-anchor-not-critical
	tests = append(tests, &testCase{
		_id:       "glob-in-anchor-not-critical",
		_expected: false,

		text: "./*",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, roots.isCriticalRemoval(fixture.anchor, test.text))
		})
	}
}

func TestIsEngineProtected(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	tests := make([]*testCase, 0)

	// dir-cargo
	tests = append(tests, &testCase{
		_id:       "dir-cargo",
		_expected: true,

		path: "/w/.cargo/config.toml",
	})

	// dir-devcontainer
	tests = append(tests, &testCase{
		_id:       "dir-devcontainer",
		_expected: true,

		path: "/w/.devcontainer/devcontainer.json",
	})

	// dir-git
	tests = append(tests, &testCase{
		_id:       "dir-git",
		_expected: true,

		path: "/w/.git/config",
	})

	// dir-husky
	tests = append(tests, &testCase{
		_id:       "dir-husky",
		_expected: true,

		path: "/w/.husky/pre-commit",
	})

	// dir-idea
	tests = append(tests, &testCase{
		_id:       "dir-idea",
		_expected: true,

		path: "/w/.idea/workspace.xml",
	})

	// dir-mvn
	tests = append(tests, &testCase{
		_id:       "dir-mvn",
		_expected: true,

		path: "/w/.mvn/jvm.config",
	})

	// dir-vscode
	tests = append(tests, &testCase{
		_id:       "dir-vscode",
		_expected: true,

		path: "/w/.vscode/settings.json",
	})

	// dir-yarn
	tests = append(tests, &testCase{
		_id:       "dir-yarn",
		_expected: true,

		path: "/w/.yarn/releases/x.cjs",
	})

	// dir-claude
	tests = append(tests, &testCase{
		_id:       "dir-claude",
		_expected: true,

		path: "/w/.claude/commands/x.md",
	})

	// dir-claude-worktrees-exempt
	tests = append(tests, &testCase{
		_id:       "dir-claude-worktrees-exempt",
		_expected: false,

		path: "/w/.claude/worktrees/wt/x.go",
	})

	// dir-config-git
	tests = append(tests, &testCase{
		_id:       "dir-config-git",
		_expected: true,

		path: "/w/.config/git/config",
	})

	// dir-config-other
	tests = append(tests, &testCase{
		_id:       "dir-config-other",
		_expected: false,

		path: "/w/.config/tool/config",
	})

	// file-bash-aliases
	tests = append(tests, &testCase{
		_id:       "file-bash-aliases",
		_expected: true,

		path: "/w/.bash_aliases",
	})

	// file-bash-login
	tests = append(tests, &testCase{
		_id:       "file-bash-login",
		_expected: true,

		path: "/w/.bash_login",
	})

	// file-bash-logout
	tests = append(tests, &testCase{
		_id:       "file-bash-logout",
		_expected: true,

		path: "/w/.bash_logout",
	})

	// file-bash-profile
	tests = append(tests, &testCase{
		_id:       "file-bash-profile",
		_expected: true,

		path: "/w/.bash_profile",
	})

	// file-bashrc
	tests = append(tests, &testCase{
		_id:       "file-bashrc",
		_expected: true,

		path: "/w/.bashrc",
	})

	// file-claude-json
	tests = append(tests, &testCase{
		_id:       "file-claude-json",
		_expected: true,

		path: "/w/.claude.json",
	})

	// file-devcontainer-json
	tests = append(tests, &testCase{
		_id:       "file-devcontainer-json",
		_expected: true,

		path: "/w/.devcontainer.json",
	})

	// file-envrc
	tests = append(tests, &testCase{
		_id:       "file-envrc",
		_expected: true,

		path: "/w/.envrc",
	})

	// file-gitconfig
	tests = append(tests, &testCase{
		_id:       "file-gitconfig",
		_expected: true,

		path: "/w/.gitconfig",
	})

	// file-gitmodules
	tests = append(tests, &testCase{
		_id:       "file-gitmodules",
		_expected: true,

		path: "/w/.gitmodules",
	})

	// file-mcp-json
	tests = append(tests, &testCase{
		_id:       "file-mcp-json",
		_expected: true,

		path: "/w/.mcp.json",
	})

	// file-npmrc
	tests = append(tests, &testCase{
		_id:       "file-npmrc",
		_expected: true,

		path: "/w/.npmrc",
	})

	// file-pnpmfile
	tests = append(tests, &testCase{
		_id:       "file-pnpmfile",
		_expected: true,

		path: "/w/.pnpmfile.cjs",
	})

	// file-pre-commit-config
	tests = append(tests, &testCase{
		_id:       "file-pre-commit-config",
		_expected: true,

		path: "/w/.pre-commit-config.yaml",
	})

	// file-profile
	tests = append(tests, &testCase{
		_id:       "file-profile",
		_expected: true,

		path: "/w/.profile",
	})

	// file-ripgreprc
	tests = append(tests, &testCase{
		_id:       "file-ripgreprc",
		_expected: true,

		path: "/w/.ripgreprc",
	})

	// file-zlogin
	tests = append(tests, &testCase{
		_id:       "file-zlogin",
		_expected: true,

		path: "/w/.zlogin",
	})

	// file-zlogout
	tests = append(tests, &testCase{
		_id:       "file-zlogout",
		_expected: true,

		path: "/w/.zlogout",
	})

	// file-zprofile
	tests = append(tests, &testCase{
		_id:       "file-zprofile",
		_expected: true,

		path: "/w/.zprofile",
	})

	// file-zshenv
	tests = append(tests, &testCase{
		_id:       "file-zshenv",
		_expected: true,

		path: "/w/.zshenv",
	})

	// file-zshrc
	tests = append(tests, &testCase{
		_id:       "file-zshrc",
		_expected: true,

		path: "/w/.zshrc",
	})

	// file-bunfig
	tests = append(tests, &testCase{
		_id:       "file-bunfig",
		_expected: true,

		path: "/w/bunfig.toml",
	})

	// file-gradle-wrapper
	tests = append(tests, &testCase{
		_id:       "file-gradle-wrapper",
		_expected: true,

		path: "/w/gradle/wrapper/gradle-wrapper.properties",
	})

	// file-maven-wrapper
	tests = append(tests, &testCase{
		_id:       "file-maven-wrapper",
		_expected: true,

		path: "/w/maven-wrapper.properties",
	})

	// file-pyrightconfig
	tests = append(tests, &testCase{
		_id:       "file-pyrightconfig",
		_expected: true,

		path: "/w/pyrightconfig.json",
	})

	// pattern-bazelrc
	tests = append(tests, &testCase{
		_id:       "pattern-bazelrc",
		_expected: true,

		path: "/w/.bazelrc.user",
	})

	// pattern-bazeliskrc
	tests = append(tests, &testCase{
		_id:       "pattern-bazeliskrc",
		_expected: true,

		path: "/w/.bazeliskrc",
	})

	// pattern-bazelversion
	tests = append(tests, &testCase{
		_id:       "pattern-bazelversion",
		_expected: true,

		path: "/w/.bazelversion",
	})

	// pattern-pnp
	tests = append(tests, &testCase{
		_id:       "pattern-pnp",
		_expected: true,

		path: "/w/.pnp.cjs",
	})

	// pattern-yarnrc
	tests = append(tests, &testCase{
		_id:       "pattern-yarnrc",
		_expected: true,

		path: "/w/.yarnrc.yml",
	})

	// pattern-lefthook-yaml
	tests = append(tests, &testCase{
		_id:       "pattern-lefthook-yaml",
		_expected: true,

		path: "/w/lefthook.yaml",
	})

	// pattern-lefthook-yml
	tests = append(tests, &testCase{
		_id:       "pattern-lefthook-yml",
		_expected: true,

		path: "/w/lefthook-local.yml",
	})

	// plain-source-file
	tests = append(tests, &testCase{
		_id:       "plain-source-file",
		_expected: false,

		path: "/w/internal/x.go",
	})

	policy := defaultToolpolicy(t)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isEngineProtected(test.path, policy))
		})
	}
}

func TestOperatorRoots(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		anchor string
		grants *Grants
	}

	fixture := fixtureSessionRoots(t)

	tests := make([]*testCase, 0)

	// gate-repo-worktree
	tests = append(tests, &testCase{
		_id:       "gate-repo-worktree",
		_expected: []string{filepath.Join(fixture.anchor, "install.sh"), filepath.Join(fixture.anchor, "cmd", "sync"), filepath.Join(fixture.anchor, "bin", "configserver")},

		anchor: fixture.anchor,
		grants: &Grants{gateRepo: fixture.mainCheckout},
	})

	// gate-repo-main-checkout
	tests = append(tests, &testCase{
		_id:       "gate-repo-main-checkout",
		_expected: nil,

		anchor: fixture.mainCheckout,
		grants: &Grants{gateRepo: fixture.mainCheckout},
	})

	// other-repo-worktree
	tests = append(tests, &testCase{
		_id:       "other-repo-worktree",
		_expected: nil,

		anchor: fixture.anchor,
		grants: &Grants{gateRepo: fixture.foreign},
	})

	// no-gate-repo-flag
	tests = append(tests, &testCase{
		_id:       "no-gate-repo-flag",
		_expected: nil,

		anchor: fixture.anchor,
		grants: &Grants{gateRepo: ""},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, operatorRoots(test.grants, fixture.policy, test.anchor))
		})
	}
}
