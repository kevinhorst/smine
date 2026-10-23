package repos

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	settleNight03 = NightBranchPrefix + "2026-10-03" + ProposalsSuffix
	settleNight04 = NightBranchPrefix + "2026-10-04" + ProposalsSuffix
)

// commitOnBranch adds one empty commit with message on branch and returns its sha.
func commitOnBranch(t *testing.T, repo, branch, message string) string {
	t.Helper()
	gitIn(t, repo, "switch", "-q", branch)
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", message)
	sha := gitIn(t, repo, "rev-parse", "HEAD")
	gitIn(t, repo, "switch", "-q", "main")
	return sha
}

func gitIn(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
}

// initSettleRepo builds a repo with one base commit on main and the given branches cut at it.
func initSettleRepo(t *testing.T, branches ...string) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q", "-b", "main")
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "base")
	for _, branch := range branches {
		gitIn(t, repo, "branch", branch)
	}
	return repo
}

func openShas(open []OpenChange) []string {
	shas := make([]string, 0, len(open))
	for _, change := range open {
		shas = append(shas, change.Commit.Sha)
	}
	return shas
}

func unplacedCommit(sha string, isMerge bool) UnplacedCommit {
	commit := UnplacedCommit{CommittedAt: "2026-10-04T03:00:00Z", IsMerge: isMerge, Sha: sha, ShortSha: sha[:8], Subject: "change"}
	return commit
}

// writeUnplacedStub fakes agentwt unplaced: it prints the document whose branch is the last argv word.
func writeUnplacedStub(t *testing.T, documents ...*Unplaced) string {
	t.Helper()
	var body strings.Builder
	body.WriteString("case \"$8\" in\n")
	for _, document := range documents {
		encoded, err := json.Marshal(document)
		require.NoError(t, err)
		body.WriteString("'" + document.Branch + "') echo '" + string(encoded) + "' ;;\n")
	}
	body.WriteString("esac")
	return writeOpStub(t, 0, body.String())
}

func unplacedDocument(branch string, commits ...UnplacedCommit) *Unplaced {
	document := &Unplaced{
		Branch:       branch,
		Commits:      commits,
		Onto:         "main",
		OntoSha:      "89abcdef0123456789abcdef0123456789abcdef",
		OntoShortSha: "89abcdef",
		Schema:       unplacedSchema,
	}
	return document
}

func TestOpenChanges(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedShas []string

		agentwtPath string
		repoPath    string
	}

	tests := make([]*testCase, 0)

	// one-branch-two-trailer-commits
	repo := initSettleRepo(t, settleNight04)
	first := commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second := commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	tests = append(tests, &testCase{
		_id:           "one-branch-two-trailer-commits",
		_expectedShas: []string{first, second},

		agentwtPath: writeUnplacedStub(t, unplacedDocument(settleNight04, unplacedCommit(first, false), unplacedCommit(second, false))),
		repoPath:    repo,
	})

	// rejected-ref-hides-one
	repo = initSettleRepo(t, settleNight04)
	first = commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second = commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	gitIn(t, repo, "update-ref", RejectedRefPrefix+first, first)
	tests = append(tests, &testCase{
		_id:           "rejected-ref-hides-one",
		_expectedShas: []string{second},

		agentwtPath: writeUnplacedStub(t, unplacedDocument(settleNight04, unplacedCommit(first, false), unplacedCommit(second, false))),
		repoPath:    repo,
	})

	// dropped-hidden
	repo = initSettleRepo(t, settleNight04)
	first = commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second = commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	gitIn(t, repo, "update-ref", DroppedRefPrefix+first, first)
	tests = append(tests, &testCase{
		_id:           "dropped-hidden",
		_expectedShas: []string{second},

		agentwtPath: writeUnplacedStub(t, unplacedDocument(settleNight04, unplacedCommit(first, false), unplacedCommit(second, false))),
		repoPath:    repo,
	})

	// merge-commit-skipped
	repo = initSettleRepo(t, settleNight04)
	first = commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second = commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	tests = append(tests, &testCase{
		_id:           "merge-commit-skipped",
		_expectedShas: []string{second},

		agentwtPath: writeUnplacedStub(t, unplacedDocument(settleNight04, unplacedCommit(first, true), unplacedCommit(second, false))),
		repoPath:    repo,
	})

	// untrailered-commit-skipped
	repo = initSettleRepo(t, settleNight04)
	first = commitOnBranch(t, repo, settleNight04, "sessions: Recorded output")
	second = commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	tests = append(tests, &testCase{
		_id:           "untrailered-commit-skipped",
		_expectedShas: []string{second},

		agentwtPath: writeUnplacedStub(t, unplacedDocument(settleNight04, unplacedCommit(first, false), unplacedCommit(second, false))),
		repoPath:    repo,
	})

	// no-proposals-branch
	tests = append(tests, &testCase{
		_id:           "no-proposals-branch",
		_expectedShas: []string{},

		agentwtPath: writeUnplacedStub(t),
		repoPath:    initSettleRepo(t, NightBranchPrefix+"2026-10-04"+BatchSuffix),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			open, err := OpenChanges(context.Background(), test.agentwtPath, test.repoPath)
			require.NoError(t, err)
			assert.Equal(t, test._expectedShas, openShas(open))
		})
	}
}

func TestDrop(t *testing.T) {
	repo := initSettleRepo(t)
	sha := gitIn(t, repo, "rev-parse", "HEAD")

	require.NoError(t, Drop(context.Background(), repo, sha))

	assert.Equal(t, sha, gitIn(t, repo, "rev-parse", DroppedRefPrefix+sha))
}

func TestReject(t *testing.T) {
	repo := initSettleRepo(t)
	sha := gitIn(t, repo, "rev-parse", "HEAD")

	require.NoError(t, Reject(context.Background(), repo, sha))

	assert.Equal(t, sha, gitIn(t, repo, "rev-parse", RejectedRefPrefix+sha))
}

func TestSettledChanges(t *testing.T) {
	repo := initSettleRepo(t, settleNight04)
	rejected := commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nThe finding.\n\nSmine-Change: skills/a")
	dropped := commitOnBranch(t, repo, settleNight04, "skills.b: Fixed b\n\nSmine-Change: skills/b")
	date := gitIn(t, repo, "show", "--no-patch", "--format=%cs", rejected)

	before, err := SettledChanges(context.Background(), repo)
	require.NoError(t, err)
	assert.Equal(t, []SettledChange{}, before)

	require.NoError(t, Reject(context.Background(), repo, rejected))
	require.NoError(t, Drop(context.Background(), repo, dropped))

	after, err := SettledChanges(context.Background(), repo)
	require.NoError(t, err)

	expected := []SettledChange{
		{Date: date, Message: &CommitMessage{Body: "", ChangeIds: []string{"skills/b"}, Sessions: []string{}, Subject: "skills.b: Fixed b"}, Outcome: SettledOutcomeDropped, Sha: dropped},
		{Date: date, Message: &CommitMessage{Body: "The finding.", ChangeIds: []string{"skills/a"}, Sessions: []string{}, Subject: "skills.a: Fixed a"}, Outcome: SettledOutcomeRejected, Sha: rejected},
	}
	assert.Equal(t, expected, after)
}

func TestSettledBranches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		branches []string
		open     []OpenChange
	}

	branches := []string{settleNight04, settleNight03}
	tests := make([]*testCase, 0)

	// branch-with-open-change-kept
	tests = append(tests, &testCase{
		_id:       "branch-with-open-change-kept",
		_expected: []string{settleNight03},

		branches: branches,
		open:     []OpenChange{{Branch: settleNight04}},
	})

	// every-branch-open
	tests = append(tests, &testCase{
		_id:       "every-branch-open",
		_expected: []string{},

		branches: branches,
		open:     []OpenChange{{Branch: settleNight04}, {Branch: settleNight03}},
	})

	// empty-open-list-settles-every-branch
	tests = append(tests, &testCase{
		_id:       "empty-open-list-settles-every-branch",
		_expected: branches,

		branches: branches,
		open:     []OpenChange{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, SettledBranches(test.branches, test.open))
		})
	}
}
