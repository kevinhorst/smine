package routinerun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/repos"
)

// fakeAgentwtName is the basename the test binary answers as agentwt, the
// one Config.AgentwtPath names under $HOME/go/bin.
const fakeAgentwtName = "smine-agentwt"

// fakeAgentwtRefusalExit is agentwt's exit status for a skipped or conflicting pick or forward.
const fakeAgentwtRefusalExit = 3

// fakeAgentwt plays agentwt's unplaced, pick, probe and forward over real git:
// placement is `git cherry`, a pick is merge-tree plus commit-tree, a probe
// merge-tree alone per commit, and a dirty checked-out target refuses a pick
// with skipped. argv is `--format json -C <repo> <command> --onto <onto> <target>…`.
func fakeAgentwt(args []string) int {
	if len(args) < 8 {
		fmt.Fprintln(os.Stderr, "fake agentwt: short argv")
		return 2
	}

	repo, command, rest := args[3], args[4], args[5:]
	onto, target := rest[1], rest[len(rest)-1]
	switch command {
	case "unplaced":
		return fakeUnplaced(repo, onto, target)
	case "pick":
		return fakePick(repo, onto, target)
	case "probe":
		return fakeProbe(repo, onto, rest[2:])
	case "forward":
		return fakeForward(repo, onto, target)
	}
	fmt.Fprintln(os.Stderr, "fake agentwt: unknown command "+command)
	return 2
}

func fakeForward(repo, onto, sha string) int {
	ctx := context.Background()
	from, _ := git(ctx, repo, "rev-parse", onto)
	result := &repos.ForwardResult{From: from, Onto: onto, Schema: 1, To: sha}
	if isFakeDirtyCheckout(repo, onto) {
		result.Outcome = repos.ForwardOutcomeSkipped
		result.Reason = onto + " is checked out with uncommitted changes"
		return printFakeDocument(result, fakeAgentwtRefusalExit)
	}

	if err := moveFakeTarget(repo, onto, from, sha); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result.Outcome = repos.ForwardOutcomeForwarded
	return printFakeDocument(result, 0)
}

func fakePick(repo, onto, sha string) int {
	ctx := context.Background()
	result := &repos.PickResult{Commit: sha, Onto: onto, Schema: 1}
	if isFakeDirtyCheckout(repo, onto) {
		result.Outcome = repos.PickOutcomeSkipped
		result.Reason = onto + " is checked out with uncommitted changes"
		return printFakeDocument(result, fakeAgentwtRefusalExit)
	}

	tree, err := git(ctx, repo, "merge-tree", "--write-tree", "--merge-base="+sha+"^", onto, sha)
	if err != nil {
		result.Outcome = repos.PickOutcomeConflict
		result.Reason = "merge-tree reports conflicts"
		return printFakeDocument(result, fakeAgentwtRefusalExit)
	}

	message, _ := git(ctx, repo, "log", "-1", "--format=%B", sha)
	ontoSha, _ := git(ctx, repo, "rev-parse", onto)
	newSha, err := git(ctx, repo, "commit-tree", firstLine(tree), "-p", ontoSha, "-m", message)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := moveFakeTarget(repo, onto, ontoSha, newSha); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result.NewCommit = newSha
	result.Outcome = repos.PickOutcomePicked
	return printFakeDocument(result, 0)
}

// fakeProbe answers every sha in order: merge-tree clean is applies, else conflict; exit 0 whatever the outcomes.
func fakeProbe(repo, onto string, shas []string) int {
	ctx := context.Background()
	ontoSha, _ := git(ctx, repo, "rev-parse", onto)
	result := &repos.ProbeResult{Commits: make([]repos.ProbeEntry, 0, len(shas)), Onto: onto, OntoSha: ontoSha, Schema: 1}
	for _, sha := range shas {
		entry := repos.ProbeEntry{Commit: sha, Outcome: repos.ProbeOutcomeApplies}
		if _, err := git(ctx, repo, "merge-tree", "--write-tree", "--merge-base="+sha+"^", onto, sha); err != nil {
			entry.Outcome = repos.ProbeOutcomeConflict
			entry.Reason = "merge-tree reports conflicts"
		}
		result.Commits = append(result.Commits, entry)
	}
	return printFakeDocument(result, 0)
}

func fakeUnplaced(repo, onto, branch string) int {
	ctx := context.Background()
	output, err := git(ctx, repo, "cherry", onto, branch)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	commits := make([]repos.UnplacedCommit, 0)
	for _, line := range nonEmptyLines(output) {
		mark, sha, _ := strings.Cut(line, " ")
		if mark != "+" {
			continue
		}
		subject, _ := git(ctx, repo, "log", "-1", "--format=%s", sha)
		commits = append(commits, repos.UnplacedCommit{Sha: sha, ShortSha: sha[:8], Subject: subject})
	}
	document := &repos.Unplaced{Branch: branch, Commits: commits, Onto: onto, Schema: 1}
	return printFakeDocument(document, 0)
}

// installFakeAgentwt links the test binary as agentwt into dir/go/bin and returns its path.
func installFakeAgentwt(t *testing.T, dir string) string {
	t.Helper()
	binDir := filepath.Join(dir, "go", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	executable, err := os.Executable()
	require.NoError(t, err)
	path := filepath.Join(binDir, fakeAgentwtName)
	require.NoError(t, os.Symlink(executable, path))
	return path
}

func isFakeDirtyCheckout(repo, branch string) bool {
	ctx := context.Background()
	head, _ := git(ctx, repo, "symbolic-ref", "--short", "HEAD")
	if head != branch {
		return false
	}
	status, _ := git(ctx, repo, "status", "--porcelain")
	return status != ""
}

// moveFakeTarget fast-forwards a checked-out target through its worktree, any other by update-ref.
func moveFakeTarget(repo, branch, fromSha, toSha string) error {
	ctx := context.Background()
	head, _ := git(ctx, repo, "symbolic-ref", "--short", "HEAD")
	if head == branch {
		_, err := git(ctx, repo, "merge", "--ff-only", "--quiet", toSha)
		return err
	}
	_, err := git(ctx, repo, "update-ref", "refs/heads/"+branch, toSha, fromSha)
	return err
}

func printFakeDocument(document any, status int) int {
	encoded, err := json.Marshal(document)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintln(os.Stdout, string(encoded))
	return status
}
