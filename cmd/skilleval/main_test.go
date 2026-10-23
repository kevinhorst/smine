package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skilleval"
)

// provisionRunDir writes a run dir whose run.json carries repoName and at.
func provisionRunDir(t *testing.T, repoName string, at string) string {
	t.Helper()
	runDir := filepath.Join(t.TempDir(), "case")
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	manifest := &skilleval.RunManifest{Arms: []skilleval.ManifestArm{{Label: "a"}}, At: at, RepoName: repoName}
	content, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runDir, "run.json"), content, 0o644))
	return runDir
}

func TestRunProvision(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		args []string
	}

	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		output, err := shell.Run(context.Background(), repo, "git", args...)
		require.NoError(t, err, output)
	}
	t.Chdir(repo)
	t.Setenv("SKILLEVAL_WT_ROOT", t.TempDir())

	tests := make([]*testCase, 0)

	// unregistered-67
	tests = append(tests, &testCase{
		_id:       "unregistered-67",
		_expected: exitUnregisteredRepo,

		args: []string{provisionRunDir(t, "nosuchrepo", "")},
	})

	// unresolvable-68
	tests = append(tests, &testCase{
		_id:       "unresolvable-68",
		_expected: exitUnresolvableRef,

		args: []string{provisionRunDir(t, "", "deadbeef")},
	})

	// usage-2
	tests = append(tests, &testCase{
		_id:       "usage-2",
		_expected: exitUsage,

		args: []string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runProvision(context.Background(), test.args))
		})
	}
}

func TestDispatchSmokeUsage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		args []string
	}

	tests := make([]*testCase, 0)

	// smoke-without-leaf
	tests = append(tests, &testCase{
		_id:       "smoke-without-leaf",
		_expected: exitUsage,

		args: []string{"smoke"},
	})

	// smoke-with-two-args
	tests = append(tests, &testCase{
		_id:       "smoke-with-two-args",
		_expected: exitUsage,

		args: []string{"smoke", "a", "b"},
	})

	// smoke-verdict-with-one-arg
	tests = append(tests, &testCase{
		_id:       "smoke-verdict-with-one-arg",
		_expected: exitUsage,

		args: []string{"smoke-verdict", "qq"},
	})

	// smoke-verdict-with-three-args
	tests = append(tests, &testCase{
		_id:       "smoke-verdict-with-three-args",
		_expected: exitUsage,

		args: []string{"smoke-verdict", "qq", "s", "x"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, dispatch(context.Background(), test.args))
		})
	}
}

func TestReportSmoke(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		err     error
		outcome *skilleval.SmokeOutcome
	}

	tests := make([]*testCase, 0)

	// deployed-differs-3
	tests = append(tests, &testCase{
		_id:       "deployed-differs-3",
		_expected: exitDeployedDiffers,

		err:     fmt.Errorf("checkDeployed: /home/.claude/skills/qq/SKILL.md: %w", skilleval.ErrDeployedDiffers),
		outcome: nil,
	})

	// harness-error-1
	tests = append(tests, &testCase{
		_id:       "harness-error-1",
		_expected: exitFailure,

		err:     errors.New("loadSmoke: Skill qq declares no smoke: /repo/skills/smart/qq/qq.cue has no smoke block"),
		outcome: nil,
	})

	// fail-verdict-1
	tests = append(tests, &testCase{
		_id:       "fail-verdict-1",
		_expected: exitFailure,

		err:     nil,
		outcome: &skilleval.SmokeOutcome{IsPass: false, Lines: []string{"smoke qq 1.0.0: fail — check exited 1"}},
	})

	// pass-0
	tests = append(tests, &testCase{
		_id:       "pass-0",
		_expected: 0,

		err:     nil,
		outcome: &skilleval.SmokeOutcome{IsPass: true, Lines: []string{"smoke qq 1.0.0: pass"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, reportSmoke(test.err, test.outcome))
		})
	}
}

func TestInitExitCode(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		err error
	}

	tests := make([]*testCase, 0)

	// no-definition-69
	tests = append(tests, &testCase{
		_id:       "no-definition-69",
		_expected: exitNoDefinition,

		err: skilleval.ErrNoDefinition,
	})

	// unknown-token-70
	tests = append(tests, &testCase{
		_id:       "unknown-token-70",
		_expected: exitUnknownToken,

		err: skilleval.ErrUnknownToken,
	})

	// over-window-71
	tests = append(tests, &testCase{
		_id:       "over-window-71",
		_expected: exitOverWindow,

		err: skilleval.ErrOverWindow,
	})

	// bad-variant-72
	tests = append(tests, &testCase{
		_id:       "bad-variant-72",
		_expected: exitBadVariant,

		err: skilleval.ErrBadVariant,
	})

	// not-runnable-73
	tests = append(tests, &testCase{
		_id:       "not-runnable-73",
		_expected: exitNotRunnable,

		err: skilleval.ErrNotRunnable,
	})

	// wrapped-sentinel-70
	tests = append(tests, &testCase{
		_id:       "wrapped-sentinel-70",
		_expected: exitUnknownToken,

		err: fmt.Errorf("Init: Token nosuchtoken: %w", skilleval.ErrUnknownToken),
	})

	// other-error-1
	tests = append(tests, &testCase{
		_id:       "other-error-1",
		_expected: exitFailure,

		err: errors.New("Init: Failed to write run.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, initExitCode(test.err))
		})
	}
}

func TestRunBrief(t *testing.T) {
	type testCase struct {
		_id      string
		_isUsage bool

		args []string
	}

	runDir := t.TempDir()

	tests := make([]*testCase, 0)

	// compare-accepted
	tests = append(tests, &testCase{
		_id:      "compare-accepted",
		_isUsage: false,

		args: []string{skilleval.BriefCompare, runDir},
	})

	// adherence-accepted
	tests = append(tests, &testCase{
		_id:      "adherence-accepted",
		_isUsage: false,

		args: []string{skilleval.BriefAdherence, runDir, "A"},
	})

	// adherence-without-label-usage
	tests = append(tests, &testCase{
		_id:      "adherence-without-label-usage",
		_isUsage: true,

		args: []string{skilleval.BriefAdherence, runDir},
	})

	// compare-with-label-usage
	tests = append(tests, &testCase{
		_id:      "compare-with-label-usage",
		_isUsage: true,

		args: []string{skilleval.BriefCompare, runDir, "A"},
	})

	// no-args-usage
	tests = append(tests, &testCase{
		_id:      "no-args-usage",
		_isUsage: true,

		args: []string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._isUsage, runBrief(test.args) == exitUsage)
		})
	}
}
