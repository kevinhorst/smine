package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	fragmentBase          = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "say done", "timeout": 5}]}]}}`
	fragmentCommandBumped = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "1.1.0", "hooks": [{"type": "command", "command": "say finished", "timeout": 5}]}]}}`
	fragmentCommandSame   = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "say finished", "timeout": 5}]}]}}`
	fragmentNewGroup      = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "say done", "timeout": 5}]}, {"smineId": "demo-stop-ring", "smineName": "Ring", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "ring"}]}]}}`
	fragmentTimeoutSame   = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "say done", "timeout": 9, "async": true}]}]}}`
	fragmentVersionDown   = `{"hooks": {"Stop": [{"smineId": "demo-stop-say", "smineName": "Say", "smineVersion": "0.9.0", "hooks": [{"type": "command", "command": "say finished", "timeout": 5}]}]}}`
	fragmentRelative      = "settings/claude_code/components/demo.json"
	fragmentTestSettings  = "settings/claude_code"
)

// provideFragmentRepo creates a repo whose one commit holds the demo fragment as committed, then writes working as its working copy.
func provideFragmentRepo(t *testing.T, committed, working string) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	ctx := context.Background()
	_, err = shell.Run(ctx, repo, "git", "init", "-q", "-b", "main")
	require.NoError(t, err)

	writeFragmentFile(t, repo, committed)
	_, err = shell.Run(ctx, repo, "git", "add", fragmentRelative)
	require.NoError(t, err)
	_, err = shell.Run(ctx, repo, "git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "base")
	require.NoError(t, err)

	writeFragmentFile(t, repo, working)
	return repo
}

func writeFragmentFile(t *testing.T, repo, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(fragmentRelative))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCheckFragments(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedIds []string
		_shouldPass  bool

		repo string
	}

	tests := make([]*testCase, 0)

	// pass-unchanged
	tests = append(tests, &testCase{
		_id:          "pass-unchanged",
		_expectedIds: nil,
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentBase),
	})

	// pass-command-changed-with-bump
	tests = append(tests, &testCase{
		_id:          "pass-command-changed-with-bump",
		_expectedIds: nil,
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentCommandBumped),
	})

	// fail-command-changed-same-version
	tests = append(tests, &testCase{
		_id:          "fail-command-changed-same-version",
		_expectedIds: []string{"demo-stop-say"},
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentCommandSame),
	})

	// fail-command-changed-version-down
	tests = append(tests, &testCase{
		_id:          "fail-command-changed-version-down",
		_expectedIds: []string{"demo-stop-say"},
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentVersionDown),
	})

	// pass-render-owned-fields-changed
	tests = append(tests, &testCase{
		_id:          "pass-render-owned-fields-changed",
		_expectedIds: nil,
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentTimeoutSame),
	})

	// pass-new-group
	tests = append(tests, &testCase{
		_id:          "pass-new-group",
		_expectedIds: nil,
		_shouldPass:  true,

		repo: provideFragmentRepo(t, fragmentBase, fragmentNewGroup),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			drifts, err := checkFragments(context.Background(), "HEAD", test.repo, fragmentTestSettings)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedIds, driftIds(drifts))
		})
	}
}

func TestCheckFragments_NewFragmentPasses(t *testing.T) {
	repo := provideFragmentRepo(t, fragmentBase, fragmentBase)
	newPath := filepath.Join(repo, "settings", "claude_code", "components", "other.json")
	require.NoError(t, os.WriteFile(newPath, []byte(`{"hooks": {"Stop": [{"smineId": "other-stop-x", "smineName": "X", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "x"}]}]}}`), 0o644))

	drifts, err := checkFragments(context.Background(), "HEAD", repo, fragmentTestSettings)

	require.NoError(t, err)
	assert.Empty(t, drifts)
}

func TestFragmentDrift_String(t *testing.T) {
	drift := &fragmentDrift{BaseVersion: "1.0.0", Id: "demo-stop-say", Path: fragmentRelative, Version: "1.0.0"}

	assert.Equal(t, "settings/claude_code/components/demo.json: demo-stop-say: hooks changed against the base but smineVersion stays 1.0.0 (base 1.0.0) — bump it, the apply keeps an equal-version live group's command", drift.String())
}

// driftIds is the ordered ids the drifts name.
func driftIds(drifts []*fragmentDrift) []string {
	var ids []string
	for _, drift := range drifts {
		ids = append(ids, drift.Id)
	}
	return ids
}
