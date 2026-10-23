package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
)

func TestSelectPending(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		candidates []peek.Session
		request    *PendingRequest
	}

	base := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	roster := []repos.Repo{
		{Name: "go", Path: "/src/go"},
		{Name: "go-tools", Path: "/src/go/tools"},
	}
	newRequest := func() *PendingRequest {
		request := &PendingRequest{Known: map[string]string{}, RepoRoot: "/src/configs", Root: "sessions", Roster: roster}
		return request
	}
	candidate := func(id, cwd string, hoursAfterBase int) peek.Session {
		session := peek.Session{Id: id, Agent: "claude", Cwd: cwd, LastActive: base.Add(time.Duration(hoursAfterBase) * time.Hour)}
		return session
	}

	tests := make([]*testCase, 0)

	// mined-dropped
	minedRequest := newRequest()
	minedRequest.Known["a"] = "sessions/go/2026-W39/a.json"
	tests = append(tests, &testCase{
		_id:       "mined-dropped",
		_expected: []string{"sessions/go/2026-W39/b.json"},

		candidates: []peek.Session{candidate("a", "/src/go", 1), candidate("b", "/src/go", 2)},
		request:    minedRequest,
	})

	// live-dropped
	liveSession := candidate("a", "/src/go", 1)
	liveSession.Live = true
	tests = append(tests, &testCase{
		_id:       "live-dropped",
		_expected: []string{},

		candidates: []peek.Session{liveSession},
		request:    newRequest(),
	})

	// zero-last-active-dropped
	stubSession := candidate("stub", "", 0)
	stubSession.LastActive = time.Time{}
	tests = append(tests, &testCase{
		_id:       "zero-last-active-dropped",
		_expected: []string{"sessions/go/2026-W39/a.json"},

		candidates: []peek.Session{stubSession, candidate("a", "/src/go", 1)},
		request:    newRequest(),
	})

	// outside-window-dropped
	windowRequest := newRequest()
	windowRequest.Since = base
	tests = append(tests, &testCase{
		_id:       "outside-window-dropped",
		_expected: []string{"sessions/go/2026-W39/in.json"},

		candidates: []peek.Session{candidate("out", "/src/go", -48), candidate("in", "/src/go", 1)},
		request:    windowRequest,
	})

	// self-mining-dropped
	tests = append(tests, &testCase{
		_id:       "self-mining-dropped",
		_expected: []string{},

		candidates: []peek.Session{candidate("a", "/src/configs/.claude/worktrees/x", 1)},
		request:    newRequest(),
	})

	// dev-keeps-self
	devRequest := newRequest()
	devRequest.IsDev = true
	tests = append(tests, &testCase{
		_id:       "dev-keeps-self",
		_expected: []string{"sessions/unknown/2026-W39/a.json"},

		candidates: []peek.Session{candidate("a", "/src/configs", 1)},
		request:    devRequest,
	})

	// routine-branch-dropped
	routineSession := candidate("a", "/src/go", 1)
	routineSession.GitBranch = "claude-routines/smine-nightly-2026-09-26"
	tests = append(tests, &testCase{
		_id:       "routine-branch-dropped",
		_expected: []string{},

		candidates: []peek.Session{routineSession},
		request:    newRequest(),
	})

	// longest-prefix-wins
	tests = append(tests, &testCase{
		_id:       "longest-prefix-wins",
		_expected: []string{"sessions/go-tools/2026-W39/a.json"},

		candidates: []peek.Session{candidate("a", "/src/go/tools/cmd", 1)},
		request:    newRequest(),
	})

	// no-match-unknown
	tests = append(tests, &testCase{
		_id:       "no-match-unknown",
		_expected: []string{"sessions/unknown/2026-W39/a.json"},

		candidates: []peek.Session{candidate("a", "/src/gopher", 1)},
		request:    newRequest(),
	})

	// empty-cwd-unknown
	tests = append(tests, &testCase{
		_id:       "empty-cwd-unknown",
		_expected: []string{"sessions/unknown/2026-W39/a.json"},

		candidates: []peek.Session{candidate("a", "", 1)},
		request:    newRequest(),
	})

	// external-pool-worktree-gets-repo
	poolRequest := newRequest()
	mainCheckout, poolWorktree := provideWorktreePair(t)
	poolRequest.Roster = []repos.Repo{{Name: "pooled", Path: mainCheckout}}
	tests = append(tests, &testCase{
		_id:       "external-pool-worktree-gets-repo",
		_expected: []string{"sessions/pooled/2026-W39/a.json"},

		candidates: []peek.Session{candidate("a", poolWorktree, 1)},
		request:    poolRequest,
	})

	// newest-first-across-weeks
	tests = append(tests, &testCase{
		_id:       "newest-first-across-weeks",
		_expected: []string{"sessions/go/2026-W39/new.json", "sessions/go/2026-W39/mid.json", "sessions/go/2026-W38/old.json"},

		candidates: []peek.Session{candidate("new", "/src/go", 5), candidate("old", "/src/go", -48), candidate("mid", "/src/go", 1)},
		request:    newRequest(),
	})

	// limit-keeps-newest
	limitRequest := newRequest()
	limitRequest.Limit = 1
	tests = append(tests, &testCase{
		_id:       "limit-keeps-newest",
		_expected: []string{"sessions/go/2026-W39/new.json"},

		candidates: []peek.Session{candidate("old", "/src/go", -48), candidate("new", "/src/go", 5)},
		request:    limitRequest,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			paths := make([]string, 0)
			for _, pending := range SelectPending(test.candidates, test.request) {
				paths = append(paths, pending.Path)
			}
			assert.Equal(t, test._expected, paths)
		})
	}
}

// provideWorktreePair builds a main checkout and a linked worktree outside
// it, joined the way git joins them: the worktree's .git file points into
// <main>/.git/worktrees/<name>.
func provideWorktreePair(t *testing.T) (string, string) {
	t.Helper()
	mainCheckout := filepath.Join(t.TempDir(), "main")
	poolWorktree := filepath.Join(t.TempDir(), "pool", "wt")
	require.NoError(t, os.MkdirAll(filepath.Join(mainCheckout, ".git", "worktrees", "wt"), 0o755))
	require.NoError(t, os.MkdirAll(poolWorktree, 0o755))
	pointer := "gitdir: " + filepath.Join(mainCheckout, ".git", "worktrees", "wt") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(poolWorktree, ".git"), []byte(pointer), 0o644))
	return mainCheckout, poolWorktree
}
