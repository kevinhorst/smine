package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
)

const (
	testProtectedPaths = "/target:/harness"
	testWorktree       = "/repo/.claude/worktrees/session-a"
)

func TestDenyReason(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedReason string

		cwd            string
		path           string
		protectedPaths string
	}

	section := &defaultHookConfig(t).Worktreewriteguard

	tests := make([]*testCase, 0)

	// own-worktree-allowed
	tests = append(tests, &testCase{
		_id:             "own-worktree-allowed",
		_expectedReason: "",

		cwd:            testWorktree,
		path:           testWorktree + "/internal/x.go",
		protectedPaths: "",
	})

	// relative-path-allowed
	tests = append(tests, &testCase{
		_id:             "relative-path-allowed",
		_expectedReason: "",

		cwd:            testWorktree,
		path:           "internal/x.go",
		protectedPaths: "",
	})

	// main-checkout-denied
	tests = append(tests, &testCase{
		_id:             "main-checkout-denied",
		_expectedReason: "session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/internal/x.go",

		cwd:            testWorktree,
		path:           "/repo/internal/x.go",
		protectedPaths: "",
	})

	// sibling-worktree-denied
	tests = append(tests, &testCase{
		_id:             "sibling-worktree-denied",
		_expectedReason: "session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/.claude/worktrees/session-b/internal/x.go",

		cwd:            testWorktree,
		path:           "/repo/.claude/worktrees/session-b/internal/x.go",
		protectedPaths: "",
	})

	// parent-segment-escape-denied
	tests = append(tests, &testCase{
		_id:             "parent-segment-escape-denied",
		_expectedReason: "path contains ../ segments — write with a normalized absolute path (session worktree: /repo/.claude/worktrees/session-a)",

		cwd:            testWorktree,
		path:           testWorktree + "/../../../internal/x.go",
		protectedPaths: "",
	})

	// tmp-allowed
	tests = append(tests, &testCase{
		_id:             "tmp-allowed",
		_expectedReason: "",

		cwd:            testWorktree,
		path:           "/tmp/scratch.txt",
		protectedPaths: "",
	})

	// home-plans-dir-allowed
	tests = append(tests, &testCase{
		_id:             "home-plans-dir-allowed",
		_expectedReason: "",

		cwd:            testWorktree,
		path:           "/Users/someone/.claude/plans/plan.md",
		protectedPaths: "",
	})

	// non-pool-cwd-allowed
	tests = append(tests, &testCase{
		_id:             "non-pool-cwd-allowed",
		_expectedReason: "",

		cwd:            "/somewhere/else",
		path:           "/repo/internal/x.go",
		protectedPaths: "",
	})

	// subdir-cwd-main-checkout-denied
	tests = append(tests, &testCase{
		_id:             "subdir-cwd-main-checkout-denied",
		_expectedReason: "session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/internal/x.go",

		cwd:            testWorktree + "/internal/deep",
		path:           "/repo/internal/x.go",
		protectedPaths: "",
	})

	// worktree-root-itself-denied
	tests = append(tests, &testCase{
		_id:             "worktree-root-itself-denied",
		_expectedReason: "session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/.claude/worktrees/session-a",

		cwd:            testWorktree,
		path:           testWorktree,
		protectedPaths: "",
	})

	// protected-deep-path-denied
	tests = append(tests, &testCase{
		_id:             "protected-deep-path-denied",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/sandbox/arm",
		path:           "/target/internal/x.go",
		protectedPaths: testProtectedPaths,
	})

	// protected-root-file-denied
	tests = append(tests, &testCase{
		_id:             "protected-root-file-denied",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/sandbox/arm",
		path:           "/target/Makefile",
		protectedPaths: testProtectedPaths,
	})

	// protected-root-itself-denied
	tests = append(tests, &testCase{
		_id:             "protected-root-itself-denied",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/sandbox/arm",
		path:           "/target",
		protectedPaths: testProtectedPaths,
	})

	// protected-relative-path-at-root-denied
	tests = append(tests, &testCase{
		_id:             "protected-relative-path-at-root-denied",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/target",
		path:           "internal/x.go",
		protectedPaths: testProtectedPaths,
	})

	// protected-second-root-denied
	tests = append(tests, &testCase{
		_id:             "protected-second-root-denied",
		_expectedReason: "path is inside a protected checkout (/harness) — routine cells write only inside their own worktree",

		cwd:            "/sandbox/arm",
		path:           "/harness/settings/settings.json",
		protectedPaths: testProtectedPaths,
	})

	// protected-sandbox-allowed
	tests = append(tests, &testCase{
		_id:             "protected-sandbox-allowed",
		_expectedReason: "",

		cwd:            "/sandbox/arm",
		path:           "/sandbox/arm/internal/x.go",
		protectedPaths: testProtectedPaths,
	})

	// protected-tmp-allowed
	tests = append(tests, &testCase{
		_id:             "protected-tmp-allowed",
		_expectedReason: "",

		cwd:            "/sandbox/arm",
		path:           "/tmp/scratch.txt",
		protectedPaths: testProtectedPaths,
	})

	// protected-sibling-named-dir-allowed
	tests = append(tests, &testCase{
		_id:             "protected-sibling-named-dir-allowed",
		_expectedReason: "",

		cwd:            "/sandbox/arm",
		path:           "/targetfoo/x.go",
		protectedPaths: testProtectedPaths,
	})

	// protected-pool-worktree-under-root-allowed
	tests = append(tests, &testCase{
		_id:             "protected-pool-worktree-under-root-allowed",
		_expectedReason: "",

		cwd:            "/sandbox/arm",
		path:           "/target/.claude/worktrees/s1/x.go",
		protectedPaths: testProtectedPaths,
	})

	// protected-pool-worktree-to-main-checkout-denied
	tests = append(tests, &testCase{
		_id:             "protected-pool-worktree-to-main-checkout-denied",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/target/.claude/worktrees/s1",
		path:           "/target/internal/x.go",
		protectedPaths: testProtectedPaths,
	})

	// pool-deny-holds-under-env
	tests = append(tests, &testCase{
		_id:             "pool-deny-holds-under-env",
		_expectedReason: "session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/internal/x.go",

		cwd:            testWorktree,
		path:           "/repo/internal/x.go",
		protectedPaths: testProtectedPaths,
	})

	// empty-list-entries-skipped
	tests = append(tests, &testCase{
		_id:             "empty-list-entries-skipped",
		_expectedReason: "path is inside a protected checkout (/target) — routine cells write only inside their own worktree",

		cwd:            "/sandbox/arm",
		path:           "/target/x.go",
		protectedPaths: "::/target:",
	})

	// only-empty-list-entries-allowed
	tests = append(tests, &testCase{
		_id:             "only-empty-list-entries-allowed",
		_expectedReason: "",

		cwd:            "/sandbox/arm",
		path:           "/target/x.go",
		protectedPaths: "::",
	})

	// empty-cwd-allowed
	tests = append(tests, &testCase{
		_id:             "empty-cwd-allowed",
		_expectedReason: "",

		cwd:            "",
		path:           "/target/x.go",
		protectedPaths: testProtectedPaths,
	})

	// empty-path-allowed
	tests = append(tests, &testCase{
		_id:             "empty-path-allowed",
		_expectedReason: "",

		cwd:            "/target",
		path:           "",
		protectedPaths: testProtectedPaths,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			reason := denyReason(test.cwd, test.path, test.protectedPaths, section)
			assert.Equal(t, test._expectedReason, reason)
		})
	}
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCode   int
		_expectedStdout string

		args   []string
		stdin  *strings.Reader
		stdout *bytes.Buffer
	}

	t.Setenv(config.ProtectedPathsKey, "")
	home := t.TempDir()
	writeHookConfig(t, home, defaultHookConfig(t))
	homeArgs := []string{"-claude-home", home}

	customSegmentHome := t.TempDir()
	customSegmentConfig := defaultHookConfig(t)
	customSegmentConfig.Worktreewriteguard.PoolSegment = "/.agents/pool/"
	writeHookConfig(t, customSegmentHome, customSegmentConfig)

	customTextsHome := t.TempDir()
	customTextsConfig := defaultHookConfig(t)
	customTextsConfig.Worktreewriteguard.DenyTexts = []string{"stay in %s, not %s", "no parent segments under %s", "protected: %s"}
	writeHookConfig(t, customTextsHome, customTextsConfig)

	shortTextsHome := t.TempDir()
	shortTextsConfig := defaultHookConfig(t)
	shortTextsConfig.Worktreewriteguard.DenyTexts = []string{"stay in %s, not %s"}
	writeHookConfig(t, shortTextsHome, shortTextsConfig)

	mainCheckoutWrite := `{"cwd":"/repo/.claude/worktrees/session-a","tool_input":{"file_path":"/repo/internal/x.go"}}`

	tests := make([]*testCase, 0)

	// deny-prints-verdict
	tests = append(tests, &testCase{
		_id:             "deny-prints-verdict",
		_expectedCode:   0,
		_expectedStdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/internal/x.go"}}` + "\n",

		args:   homeArgs,
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// allow-prints-nothing
	tests = append(tests, &testCase{
		_id:             "allow-prints-nothing",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   homeArgs,
		stdin:  strings.NewReader(`{"cwd":"/repo/.claude/worktrees/session-a","tool_input":{"file_path":"/repo/.claude/worktrees/session-a/x.go"}}`),
		stdout: &bytes.Buffer{},
	})

	// notebook-path-judged
	tests = append(tests, &testCase{
		_id:             "notebook-path-judged",
		_expectedCode:   0,
		_expectedStdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"session is worktree-anchored: writes go under /repo/.claude/worktrees/session-a — refusing write into the main checkout or a sibling worktree: /repo/notes.ipynb"}}` + "\n",

		args:   homeArgs,
		stdin:  strings.NewReader(`{"cwd":"/repo/.claude/worktrees/session-a","tool_input":{"notebook_path":"/repo/notes.ipynb"}}`),
		stdout: &bytes.Buffer{},
	})

	// pool-segment-from-config
	tests = append(tests, &testCase{
		_id:             "pool-segment-from-config",
		_expectedCode:   0,
		_expectedStdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"session is worktree-anchored: writes go under /repo/.agents/pool/session-a — refusing write into the main checkout or a sibling worktree: /repo/internal/x.go"}}` + "\n",

		args:   []string{"-claude-home", customSegmentHome},
		stdin:  strings.NewReader(`{"cwd":"/repo/.agents/pool/session-a","tool_input":{"file_path":"/repo/internal/x.go"}}`),
		stdout: &bytes.Buffer{},
	})

	// default-segment-not-a-pool-under-custom-config
	tests = append(tests, &testCase{
		_id:             "default-segment-not-a-pool-under-custom-config",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   []string{"-claude-home", customSegmentHome},
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// deny-texts-from-config
	tests = append(tests, &testCase{
		_id:             "deny-texts-from-config",
		_expectedCode:   0,
		_expectedStdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"stay in /repo/.claude/worktrees/session-a, not /repo/internal/x.go"}}` + "\n",

		args:   []string{"-claude-home", customTextsHome},
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// deny-texts-short-list-exits-1
	tests = append(tests, &testCase{
		_id:             "deny-texts-short-list-exits-1",
		_expectedCode:   1,
		_expectedStdout: "",

		args:   []string{"-claude-home", shortTextsHome},
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// missing-config-errors
	tests = append(tests, &testCase{
		_id:             "missing-config-errors",
		_expectedCode:   1,
		_expectedStdout: "",

		args:   []string{"-claude-home", t.TempDir()},
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// malformed-input-exits-1
	tests = append(tests, &testCase{
		_id:             "malformed-input-exits-1",
		_expectedCode:   1,
		_expectedStdout: "",

		args:   homeArgs,
		stdin:  strings.NewReader("{not json"),
		stdout: &bytes.Buffer{},
	})

	// unknown-flag-exits-2
	tests = append(tests, &testCase{
		_id:             "unknown-flag-exits-2",
		_expectedCode:   2,
		_expectedStdout: "",

		args:   []string{"-bogus"},
		stdin:  strings.NewReader(mainCheckoutWrite),
		stdout: &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exitCode := run(test.args, io.Discard, test.stdin, test.stdout)
			assert.Equal(t, test._expectedCode, exitCode)
			assert.Equal(t, test._expectedStdout, test.stdout.String())
		})
	}
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

// writeHookConfig writes the config where hookconfig.Load reads it.
func writeHookConfig(t *testing.T, home string, hookConfig *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(hookconfig.Path(home)), 0o755))
	require.NoError(t, os.WriteFile(hookconfig.Path(home), encoded, 0o644))
}
