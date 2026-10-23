package repos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeOpStub creates a stub agentwt binary that prints a JSON op result
// whose one line's text echoes the argv, exiting with `status`.
func writeOpStub(t *testing.T, status int, body string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\nexit " + strconv.Itoa(status) + "\n"
	stub := filepath.Join(dir, "agentwt")
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return stub
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
	run("branch", "claude-routines/nightly-2026-08-12")
	run("branch", "claude-review/station-r1")

	cmd := exec.Command("git", "rev-parse", "claude/feature")
	cmd.Dir = repo
	sha, err := cmd.Output()
	require.NoError(t, err)
	return repo, strings.TrimSpace(string(sha))
}

func TestValidateBranchRejectsNonClaude(t *testing.T) {
	for _, branch := range []string{"main", "routine/nightly-2026-08-12", "claude-review", "claudefoo/x"} {
		err := ValidateBranch(context.Background(), branch, t.TempDir())
		require.Error(t, err, branch)
		assert.Contains(t, err.Error(), "Not an agent branch")
	}
}

func TestValidateBranchAcceptsFamilyNamespaces(t *testing.T) {
	repo, _ := initGitRepo(t)
	for _, branch := range []string{"claude/feature", "claude-routines/nightly-2026-08-12", "claude-review/station-r1"} {
		assert.NoError(t, ValidateBranch(context.Background(), branch, repo), branch)
	}
}

func TestAgentBranchKind(t *testing.T) {
	cases := []struct {
		branch string
		want   string
	}{
		{branch: "claude/x", want: "claude"},
		{branch: "claude-review/x", want: "claude-review"},
		{branch: "claude-routines/nightly-x", want: "claude-routines"},
		{branch: "main", want: ""},
		{branch: "claude", want: ""},
		{branch: "claude-review", want: ""},
		{branch: "claudefoo/x", want: ""},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, AgentBranchKind(c.branch), "branch=%q", c.branch)
	}
}

func TestValidateBranchRejectsUnknownRef(t *testing.T) {
	repo, _ := initGitRepo(t)
	err := ValidateBranch(context.Background(), "claude/missing", repo)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown branch")
}

func TestRemoveExecShape(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 0, `printf '{"operation":"remove","lines":[{"subject":"claude/feature","outcome":"removed","text":"argv: %s"}]}' "$*"`)

	cases := []struct {
		force        bool
		deleteBranch bool
		want         string
	}{
		{force: false, deleteBranch: false, want: "remove claude/feature"},
		{force: true, deleteBranch: false, want: "remove --force claude/feature"},
		{force: false, deleteBranch: true, want: "remove --delete-branch claude/feature"},
		{force: true, deleteBranch: true, want: "remove --delete-branch --force claude/feature"},
	}
	for _, c := range cases {
		output, err := Remove(context.Background(), "claude/feature", c.force, c.deleteBranch, repo, stub)
		require.NoError(t, err)
		require.Len(t, output.Lines, 1)
		assert.Equal(t, "argv: --format json -C "+repo+" "+c.want, output.Text)
		assert.Equal(t, "removed", output.Lines[0].Outcome)
		assert.False(t, output.HasSkips())
	}
}

func TestRemoveRejectsUnknownBranchBeforeExec(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 0, `echo "must not run" >&2`)
	_, err := Remove(context.Background(), "claude/missing", false, false, repo, stub)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown branch")
}

func TestRunOpRefusalKeepsOutput(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 3, `echo "agentwt: not an agent branch: main" >&2`)
	output, err := runOp(context.Background(), repo, stub, "remove", "main")
	require.Error(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "agentwt: not an agent branch: main", output.Text)
	assert.Empty(t, output.Lines)
}

func TestRunOpMalformedResultFails(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 0, `echo "removed something"`)
	_, err := runOp(context.Background(), repo, stub, "remove", "claude/feature")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Decode")
}

func TestPick(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *PickResult
		_shouldPass bool

		stub string
	}

	repo := t.TempDir()
	sha := "0123456789abcdef0123456789abcdef01234567"
	tests := make([]*testCase, 0)

	// pass-picked-exit-0
	tests = append(tests, &testCase{
		_id:         "pass-picked-exit-0",
		_expected:   &PickResult{Commit: sha, NewCommit: "fedcba9876543210fedcba9876543210fedcba98", Onto: "main", Outcome: PickOutcomePicked, Schema: 1},
		_shouldPass: true,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"picked","new_commit":"fedcba9876543210fedcba9876543210fedcba98"}'`),
	})

	// fail-applies-is-no-pick-outcome
	tests = append(tests, &testCase{
		_id:         "fail-applies-is-no-pick-outcome",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"applies"}'`),
	})

	// pass-conflict-exit-3-with-stderr
	tests = append(tests, &testCase{
		_id:         "pass-conflict-exit-3-with-stderr",
		_expected:   &PickResult{Commit: sha, Onto: "main", Outcome: PickOutcomeConflict, Reason: "a.md conflicts", Schema: 1},
		_shouldPass: true,

		stub: writeOpStub(t, 3, `echo "agentwt: pick refused" >&2
echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"conflict","reason":"a.md conflicts"}'`),
	})

	// pass-skipped-exit-3
	tests = append(tests, &testCase{
		_id:         "pass-skipped-exit-3",
		_expected:   &PickResult{Commit: sha, Onto: "main", Outcome: PickOutcomeSkipped, Reason: "already on main", Schema: 1},
		_shouldPass: true,

		stub: writeOpStub(t, 3, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"skipped","reason":"already on main"}'`),
	})

	// fail-conflict-on-exit-0
	tests = append(tests, &testCase{
		_id:         "fail-conflict-on-exit-0",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"conflict","reason":"x"}'`),
	})

	// fail-picked-on-exit-3
	tests = append(tests, &testCase{
		_id:         "fail-picked-on-exit-3",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 3, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"picked","new_commit":"x"}'`),
	})

	// fail-conflict-on-exit-1
	tests = append(tests, &testCase{
		_id:         "fail-conflict-on-exit-1",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 1, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"conflict","reason":"x"}'`),
	})

	// fail-no-document
	tests = append(tests, &testCase{
		_id:         "fail-no-document",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 2, `echo "agentwt: unknown command pick" >&2`),
	})

	// fail-unknown-outcome
	tests = append(tests, &testCase{
		_id:         "fail-unknown-outcome",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"commit":"`+sha+`","onto":"main","outcome":"merged"}'`),
	})

	// fail-unknown-schema
	tests = append(tests, &testCase{
		_id:         "fail-unknown-schema",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":2,"commit":"`+sha+`","onto":"main","outcome":"picked"}'`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := Pick(context.Background(), test.stub, "main", repo, sha)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, result)
		})
	}
}

func TestPickExecShape(t *testing.T) {
	repo := t.TempDir()
	stub := writeOpStub(t, 0, `printf '{"schema":1,"commit":"%s","onto":"main","outcome":"picked","new_commit":"x"}' "$*"`)

	result, err := Pick(context.Background(), stub, "main", repo, "0123456789abcdef0123456789abcdef01234567")

	require.NoError(t, err)
	assert.Equal(t, "--format json -C "+repo+" pick --onto main 0123456789abcdef0123456789abcdef01234567", result.Commit)
}

func TestProbe(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *ProbeResult
		_shouldPass bool

		shas []string
		stub string
	}

	repo := t.TempDir()
	first := "0123456789abcdef0123456789abcdef01234567"
	second := "89abcdef0123456789abcdef0123456789abcdef"
	tests := make([]*testCase, 0)

	// pass-every-outcome-in-order
	tests = append(tests, &testCase{
		_id: "pass-every-outcome-in-order",
		_expected: &ProbeResult{
			Commits: []ProbeEntry{{Commit: first, Outcome: ProbeOutcomeApplies}, {Commit: second, Outcome: ProbeOutcomeConflict, Reason: "conflicts in a.md"}},
			Onto:    "main",
			OntoSha: "fedcba9876543210fedcba9876543210fedcba98",
			Schema:  1,
		},
		_shouldPass: true,

		shas: []string{first, second},
		stub: writeOpStub(t, 0, `echo "agentwt: probing" >&2
echo '{"schema":1,"onto":"main","onto_sha":"fedcba9876543210fedcba9876543210fedcba98","commits":[{"commit":"`+first+`","outcome":"applies"},{"commit":"`+second+`","outcome":"conflict","reason":"conflicts in a.md"}]}'`),
	})

	// pass-argv-carries-every-sha
	tests = append(tests, &testCase{
		_id: "pass-argv-carries-every-sha",
		_expected: &ProbeResult{
			Commits: []ProbeEntry{{Commit: "--format json -C " + repo + " probe --onto main " + first + " " + second, Outcome: ProbeOutcomeSkipped, Reason: "merge commit"}},
			Onto:    "main",
			OntoSha: "0",
			Schema:  1,
		},
		_shouldPass: true,

		shas: []string{first, second},
		stub: writeOpStub(t, 0, `printf '{"schema":1,"onto":"main","onto_sha":"0","commits":[{"commit":"%s","outcome":"skipped","reason":"merge commit"}]}' "$*"`),
	})

	// pass-no-sha-asks-nothing
	tests = append(tests, &testCase{
		_id:         "pass-no-sha-asks-nothing",
		_expected:   &ProbeResult{Commits: []ProbeEntry{}, Onto: "main", Schema: 1},
		_shouldPass: true,

		shas: []string{},
		stub: writeOpStub(t, 1, `echo "must not run" >&2`),
	})

	// fail-refusal-exit
	tests = append(tests, &testCase{
		_id:         "fail-refusal-exit",
		_expected:   nil,
		_shouldPass: false,

		shas: []string{first},
		stub: writeOpStub(t, 3, `echo "agentwt: unknown commit deadbeef" >&2`),
	})

	// fail-unknown-outcome
	tests = append(tests, &testCase{
		_id:         "fail-unknown-outcome",
		_expected:   nil,
		_shouldPass: false,

		shas: []string{first},
		stub: writeOpStub(t, 0, `echo '{"schema":1,"onto":"main","onto_sha":"0","commits":[{"commit":"`+first+`","outcome":"picked"}]}'`),
	})

	// fail-unknown-schema
	tests = append(tests, &testCase{
		_id:         "fail-unknown-schema",
		_expected:   nil,
		_shouldPass: false,

		shas: []string{first},
		stub: writeOpStub(t, 0, `echo '{"schema":2,"onto":"main","onto_sha":"0","commits":[]}'`),
	})

	// fail-no-document
	tests = append(tests, &testCase{
		_id:         "fail-no-document",
		_expected:   nil,
		_shouldPass: false,

		shas: []string{first},
		stub: writeOpStub(t, 0, `echo "probed"`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := Probe(context.Background(), test.stub, "main", repo, test.shas)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, result)
		})
	}
}

func TestProbeResultByCommit(t *testing.T) {
	first := ProbeEntry{Commit: "a", Outcome: ProbeOutcomeApplies}
	second := ProbeEntry{Commit: "b", Outcome: ProbeOutcomeConflict, Reason: "x"}
	result := &ProbeResult{Commits: []ProbeEntry{first, second}}

	assert.Equal(t, map[string]ProbeEntry{"a": first, "b": second}, result.ByCommit())
	assert.True(t, first.IsApplicable())
	assert.False(t, second.IsApplicable())
}

func TestForward(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *ForwardResult
		_shouldPass bool

		stub string
	}

	repo := t.TempDir()
	from := "89abcdef0123456789abcdef0123456789abcdef"
	sha := "0123456789abcdef0123456789abcdef01234567"
	tests := make([]*testCase, 0)

	// pass-forwarded-exit-0
	tests = append(tests, &testCase{
		_id:         "pass-forwarded-exit-0",
		_expected:   &ForwardResult{From: from, Onto: "main", Outcome: ForwardOutcomeForwarded, Schema: 1, To: sha},
		_shouldPass: true,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"onto":"main","from":"`+from+`","to":"`+sha+`","outcome":"forwarded","reason":null}'`),
	})

	// pass-unchanged-exit-0
	tests = append(tests, &testCase{
		_id:         "pass-unchanged-exit-0",
		_expected:   &ForwardResult{From: sha, Onto: "main", Outcome: ForwardOutcomeUnchanged, Schema: 1, To: sha},
		_shouldPass: true,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"onto":"main","from":"`+sha+`","to":"`+sha+`","outcome":"unchanged","reason":null}'`),
	})

	// pass-skipped-exit-3
	tests = append(tests, &testCase{
		_id:         "pass-skipped-exit-3",
		_expected:   &ForwardResult{From: from, Onto: "main", Outcome: ForwardOutcomeSkipped, Reason: "main has uncommitted changes", Schema: 1, To: sha},
		_shouldPass: true,

		stub: writeOpStub(t, 3, `echo "agentwt: forward refused" >&2
echo '{"schema":1,"onto":"main","from":"`+from+`","to":"`+sha+`","outcome":"skipped","reason":"main has uncommitted changes"}'`),
	})

	// fail-skipped-on-exit-0
	tests = append(tests, &testCase{
		_id:         "fail-skipped-on-exit-0",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"onto":"main","from":"`+from+`","to":"`+sha+`","outcome":"skipped","reason":"x"}'`),
	})

	// fail-unknown-outcome
	tests = append(tests, &testCase{
		_id:         "fail-unknown-outcome",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":1,"onto":"main","from":"`+from+`","to":"`+sha+`","outcome":"merged"}'`),
	})

	// fail-unknown-schema
	tests = append(tests, &testCase{
		_id:         "fail-unknown-schema",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 0, `echo '{"schema":2,"onto":"main","from":"`+from+`","to":"`+sha+`","outcome":"forwarded"}'`),
	})

	// fail-no-document
	tests = append(tests, &testCase{
		_id:         "fail-no-document",
		_expected:   nil,
		_shouldPass: false,

		stub: writeOpStub(t, 2, `echo "agentwt: unknown command forward" >&2`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := Forward(context.Background(), test.stub, "main", repo, sha)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, result)
		})
	}
}

func TestForwardExecShape(t *testing.T) {
	repo := t.TempDir()
	stub := writeOpStub(t, 0, `printf '{"schema":1,"onto":"main","from":"%s","to":"x","outcome":"forwarded"}' "$*"`)

	result, err := Forward(context.Background(), stub, "main", repo, "0123456789abcdef0123456789abcdef01234567")

	require.NoError(t, err)
	assert.Equal(t, "--format json -C "+repo+" forward --onto main 0123456789abcdef0123456789abcdef01234567", result.From)
}

func TestOpOutputHasSkips(t *testing.T) {
	cases := []struct {
		lines []OpLine
		want  bool
	}{
		{lines: nil, want: false},
		{lines: []OpLine{{Outcome: "removed"}, {Outcome: "branch_deleted"}}, want: false},
		{lines: []OpLine{{Outcome: "removed"}, {Outcome: "skipped"}}, want: true},
	}
	for _, c := range cases {
		output := &OpOutput{Lines: c.lines}
		assert.Equalf(t, c.want, output.HasSkips(), "lines=%+v", c.lines)
	}
}
