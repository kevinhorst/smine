package repos

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// MaxChangeFiles and MaxChangeLines cap one change commit as a proposal; the runner backlogs a bigger one, the page shows the counts.
const (
	MaxChangeFiles = 7
	MaxChangeLines = 420
)

// RejectedRefPrefix and DroppedRefPrefix hold one ref per settled change commit, named by its sha; rejected suppresses re-mining, dropped only logs.
const (
	DroppedRefPrefix  = "refs/smine/dropped/"
	RejectedRefPrefix = "refs/smine/rejected/"

	SettledOutcomeDropped  = "dropped"
	SettledOutcomeRejected = "rejected"

	// settledFields: refname, sha, committer date and the full message, unit-separated.
	settledFields = 4
)

// OpenChange is one unsettled change commit: unplaced on main, neither rejected nor dropped.
type OpenChange struct {
	Branch  string
	Commit  UnplacedCommit
	Message *CommitMessage
}

// SettledChange is one rejected or dropped change commit, read off its ref in one pass.
type SettledChange struct {
	Date    string // committer date as YYYY-MM-DD
	Message *CommitMessage
	Outcome string
	Sha     string
}

// branchChanges is one branch's unplaced trailer commits, oldest first; merge commits and untrailered commits are skipped.
func branchChanges(ctx context.Context, agentwtPath, branch, repoPath string) ([]OpenChange, error) {
	unplaced, err := ListUnplaced(ctx, agentwtPath, branch, "main", repoPath)
	if err != nil {
		return nil, fmt.Errorf("branchChanges: %w", err)
	}

	messages, err := BranchMessages(ctx, branch, repoPath, "main")
	if err != nil {
		return nil, fmt.Errorf("branchChanges: %w", err)
	}

	changes := make([]OpenChange, 0, len(unplaced.Commits))
	for _, commit := range unplaced.Commits {
		if commit.IsMerge {
			continue
		}

		message, ok := messages[commit.Sha]
		if !ok {
			return nil, fmt.Errorf("branchChanges: Commit %s is unplaced but not in main..%s", commit.Sha, branch)
		}
		if len(message.ChangeIds) == 0 {
			continue
		}

		change := OpenChange{Branch: branch, Commit: commit, Message: message}
		changes = append(changes, change)
	}
	return changes, nil
}

// settledOutcome names the outcome a settled ref records by its prefix; empty for a ref outside both.
func settledOutcome(refname string) string {
	switch {
	case strings.HasPrefix(refname, RejectedRefPrefix):
		return SettledOutcomeRejected
	case strings.HasPrefix(refname, DroppedRefPrefix):
		return SettledOutcomeDropped
	}
	return ""
}

// Drop records a change commit as aged out or stale; the ref keeps the commit reachable after its branch dies and never suppresses re-mining.
func Drop(ctx context.Context, repoPath, sha string) error {
	output, err := shell.Run(ctx, repoPath, "git", "update-ref", DroppedRefPrefix+sha, sha)
	if err != nil {
		return fmt.Errorf("Drop: %s: %w", strings.TrimSpace(output), err)
	}
	return nil
}

// OpenChanges lists proposals branches newest first, each branch's commits oldest first.
func OpenChanges(ctx context.Context, agentwtPath, repoPath string) ([]OpenChange, error) {
	branches, err := ListNightBranches(ctx, repoPath, ProposalsSuffix)
	if err != nil {
		return nil, fmt.Errorf("OpenChanges: %w", err)
	}

	settled, err := SettledChanges(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("OpenChanges: %w", err)
	}
	settledShas := make([]string, 0, len(settled))
	for _, change := range settled {
		settledShas = append(settledShas, change.Sha)
	}

	open := make([]OpenChange, 0)
	for _, branch := range branches {
		changes, err := branchChanges(ctx, agentwtPath, branch, repoPath)
		if err != nil {
			return nil, fmt.Errorf("OpenChanges: %w", err)
		}
		for _, change := range changes {
			if slices.Contains(settledShas, change.Commit.Sha) {
				continue
			}
			open = append(open, change)
		}
	}
	return open, nil
}

// Reject records a change commit as rejected; the ref also keeps the commit reachable after its branch dies.
func Reject(ctx context.Context, repoPath, sha string) error {
	output, err := shell.Run(ctx, repoPath, "git", "update-ref", RejectedRefPrefix+sha, sha)
	if err != nil {
		return fmt.Errorf("Reject: %s: %w", strings.TrimSpace(output), err)
	}
	return nil
}

// SettledBranches names the proposals branches none of the open changes belong to.
func SettledBranches(branches []string, open []OpenChange) []string {
	settled := make([]string, 0)
	for _, branch := range branches {
		isOpen := slices.ContainsFunc(open, func(change OpenChange) bool { return change.Branch == branch })
		if !isOpen {
			settled = append(settled, branch)
		}
	}
	return settled
}

// SettledChanges reads every rejected and dropped ref with its commit's date and message in one git call, ref order.
func SettledChanges(ctx context.Context, repoPath string) ([]SettledChange, error) {
	output, err := shell.Run(
		ctx,
		repoPath,
		"git",
		"for-each-ref",
		"--format=%(refname)%1f%(objectname)%1f%(committerdate:short)%1f%(contents)%1e",
		RejectedRefPrefix,
		DroppedRefPrefix,
	)
	if err != nil {
		return nil, fmt.Errorf("SettledChanges: %s: %w", strings.TrimSpace(output), err)
	}

	settled := make([]SettledChange, 0)
	for record := range strings.SplitSeq(output, "\x1e") {
		fields := strings.SplitN(strings.TrimLeft(record, "\n"), "\x1f", settledFields)
		if len(fields) < settledFields {
			continue
		}

		change := SettledChange{
			Date:    fields[2],
			Message: ParseCommitMessage(fields[3]),
			Outcome: settledOutcome(fields[0]),
			Sha:     fields[1],
		}
		settled = append(settled, change)
	}
	return settled, nil
}
