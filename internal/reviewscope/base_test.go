package reviewscope

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitFixture is a throwaway repository whose main branch holds one commit.
type gitFixture struct {
	repo string
	t    *testing.T
}

// newGitFixture initializes the repository on main with one base commit.
func newGitFixture(t *testing.T) *gitFixture {
	t.Helper()
	fixture := &gitFixture{repo: t.TempDir(), t: t}
	fixture.run("init", "-q", "-b", "main")
	fixture.run("config", "user.email", "test@example.com")
	fixture.run("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(fixture.repo, "base"), []byte("base\n"), 0o644))
	fixture.run("add", "base")
	fixture.run("commit", "-qm", "base")
	return fixture
}

// branch creates name at from without checking it out.
func (f *gitFixture) branch(name, from string) {
	f.run("branch", name, from)
}

// commit adds one file commit on branch and returns the new tip sha.
func (f *gitFixture) commit(branch, file string) string {
	f.t.Helper()
	f.run("checkout", "-q", branch)
	require.NoError(f.t, os.WriteFile(filepath.Join(f.repo, file), []byte(file+"\n"), 0o644))
	f.run("add", file)
	f.run("commit", "-qm", file)
	return f.run("rev-parse", "HEAD")
}

// run executes one git command in the fixture and returns its trimmed output.
func (f *gitFixture) run(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.repo
	output, err := cmd.CombinedOutput()
	require.NoError(f.t, err, string(output))
	return strings.TrimSpace(string(output))
}

func TestDeriveBase(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *DerivedBase
		_shouldPass bool

		bracket string
		head    string
		repo    string
	}

	tests := make([]*testCase, 0)

	plain := newGitFixture(t)
	plainMain := plain.run("rev-parse", "main")
	plain.branch("claude/only", "main")
	plain.commit("claude/only", "only")

	// bracket
	tests = append(tests, &testCase{
		_id:         "bracket",
		_expected:   &DerivedBase{Commit: plainMain, Source: BaseSourceBracket},
		_shouldPass: true,

		bracket: "main",
		head:    "claude/only",
		repo:    plain.repo,
	})

	// bracket-unresolvable
	tests = append(tests, &testCase{
		_id:         "bracket-unresolvable",
		_expected:   nil,
		_shouldPass: false,

		bracket: "no-such-branch",
		head:    "claude/only",
		repo:    plain.repo,
	})

	// mainline
	tests = append(tests, &testCase{
		_id:         "mainline",
		_expected:   &DerivedBase{Commit: plainMain, Source: BaseSourceMainline},
		_shouldPass: true,

		bracket: "",
		head:    "claude/only",
		repo:    plain.repo,
	})

	// upstream
	tracked := newGitFixture(t)
	tracked.branch("claude/parent", "main")
	parentTip := tracked.commit("claude/parent", "parent")
	tracked.branch("claude/child", "claude/parent")
	tracked.commit("claude/child", "child")
	tracked.run("branch", "--set-upstream-to=claude/parent", "claude/child")
	tests = append(tests, &testCase{
		_id:         "upstream",
		_expected:   &DerivedBase{Commit: parentTip, Source: BaseSourceUpstream},
		_shouldPass: true,

		bracket: "",
		head:    "claude/child",
		repo:    tracked.repo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			derived, err := DeriveBase(context.Background(), test.repo, test.head, "main", test.bracket)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, derived)
		})
	}
}
