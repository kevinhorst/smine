package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/shell"
)

const proposalStoreNight = repos.NightBranchPrefix + "2026-10-05" + repos.ProposalsSuffix

// proposalStoreStub stands in for agentwt: `unplaced` logs one line to calls and prints unplaced.json; `probe` answers applies for every sha.
const proposalStoreStub = `#!/bin/sh
dir=$(dirname "$0")
cmd="$5"
shift 5
if [ "$cmd" = "unplaced" ]; then
	echo "$*" >> "$dir/calls"
	cat "$dir/unplaced.json"
	exit 0
fi
if [ "$cmd" = "probe" ]; then
	shift 2
	printf '{"schema":1,"onto":"main","onto_sha":"0","commits":['
	sep=""
	for sha; do
		printf '%s{"commit":"%s","outcome":"applies"}' "$sep" "$sha"
		sep=","
	done
	printf ']}'
	exit 0
fi
echo "agentwt: unknown command $cmd" >&2
exit 1
`

// proposalStoreFixture is a repo with one proposals branch holding two change commits and a counting agentwt stub.
type proposalStoreFixture struct {
	agentwtPath string
	repo        string
	shas        map[string]string
	stubDir     string
}

func newProposalStoreFixture(t *testing.T) *proposalStoreFixture {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	stubDir := t.TempDir()
	fixture := &proposalStoreFixture{
		agentwtPath: filepath.Join(stubDir, "agentwt.sh"),
		repo:        repo,
		shas:        map[string]string{},
		stubDir:     stubDir,
	}
	fixture.git(t, "init", "-q", "-b", "main")
	fixture.git(t, "commit", "-q", "--allow-empty", "-m", "base")

	fixture.git(t, "switch", "-q", "-c", proposalStoreNight)
	fixture.commit(t, "A", "a.md", "skills.a: Tightened the a skill\n\nSmine-Change: skills/a")
	fixture.commit(t, "B", "b.md", "skills.b: Tightened the b skill\n\nSmine-Change: skills/b")
	fixture.git(t, "switch", "-q", "main")

	commits := make([]repos.UnplacedCommit, 0)
	for _, label := range []string{"A", "B"} {
		sha := fixture.shas[label]
		commit := repos.UnplacedCommit{CommittedAt: "2026-10-05T03:00:00Z", Sha: sha, ShortSha: sha[:8], Subject: label}
		commits = append(commits, commit)
	}
	document := repos.Unplaced{Branch: proposalStoreNight, Commits: commits, Onto: "main", OntoSha: "0", OntoShortSha: "0", Schema: 1}
	encoded, err := json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stubDir, "unplaced.json"), encoded, 0o644))
	require.NoError(t, os.WriteFile(fixture.agentwtPath, []byte(proposalStoreStub), 0o755))
	return fixture
}

// scans counts the stub's logged unplaced invocations; zero before the first.
func (f *proposalStoreFixture) scans(t *testing.T) int {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(f.stubDir, "calls"))
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}

	require.NoError(t, err)
	return strings.Count(string(content), "\n")
}

func (f *proposalStoreFixture) commit(t *testing.T, label, file, message string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.repo, file), []byte(label+"\n"), 0o644))
	f.git(t, "add", file)
	f.git(t, "commit", "-q", "-m", message)
	f.shas[label] = strings.TrimSpace(f.git(t, "rev-parse", "HEAD"))
}

func (f *proposalStoreFixture) git(t *testing.T, args ...string) string {
	t.Helper()
	argv := append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)
	output, err := shell.Run(context.Background(), f.repo, "git", argv...)
	require.NoError(t, err, output)
	return output
}

func TestProposalStore(t *testing.T) {
	fixture := newProposalStoreFixture(t)
	store := newProposalStore()
	ctx := context.Background()

	// first-call-scans-once
	snapshot, err := store.Snapshot(ctx, fixture.agentwtPath, fixture.repo, "main")
	assert.NoError(t, err)
	assert.Equal(t, 2, snapshot.Count())
	assert.Equal(t, 1, fixture.scans(t))
	assert.Equal(t, repos.ProbeOutcomeApplies, snapshot.Probes[fixture.shas["A"]].Outcome)
	assert.Equal(t, "skills.a: Tightened the a skill", snapshot.Commits[fixture.shas["A"]].Subject)
	firstPayload := snapshot.Commits[fixture.shas["B"]]

	// same-refs-no-rescan
	snapshot, err = store.Snapshot(ctx, fixture.agentwtPath, fixture.repo, "main")
	assert.NoError(t, err)
	assert.Equal(t, 2, snapshot.Count())
	assert.Equal(t, 1, fixture.scans(t))

	// new-rejected-ref-rescans-payloads-kept
	require.NoError(t, repos.Reject(ctx, fixture.repo, fixture.shas["A"]))
	snapshot, err = store.Snapshot(ctx, fixture.agentwtPath, fixture.repo, "main")
	assert.NoError(t, err)
	assert.Equal(t, 2, fixture.scans(t))
	assert.Equal(t, []string{fixture.shas["B"]}, openShas(snapshot.Open))
	assert.Same(t, firstPayload, snapshot.Commits[fixture.shas["B"]])
	assert.NotContains(t, snapshot.Commits, fixture.shas["A"])

	// target-moves-rescans
	fixture.commit(t, "M", "main.md", "main: Advanced")
	snapshot, err = store.Snapshot(ctx, fixture.agentwtPath, fixture.repo, "main")
	assert.NoError(t, err)
	assert.Equal(t, 1, snapshot.Count())
	assert.Equal(t, 3, fixture.scans(t))

	// git-failure-returns-last-snapshot
	snapshot, err = store.Snapshot(ctx, fixture.agentwtPath, t.TempDir(), "main")
	assert.Error(t, err)
	assert.Equal(t, 1, snapshot.Count())
	assert.Equal(t, 3, fixture.scans(t))
}

func openShas(open []repos.OpenChange) []string {
	shas := make([]string, 0, len(open))
	for _, change := range open {
		shas = append(shas, change.Commit.Sha)
	}
	return shas
}
