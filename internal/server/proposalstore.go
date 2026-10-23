package server

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/shell"
)

// proposalSnapshot is the open set at one fingerprint: every open change, its probe outcome and its commit payload by sha.
type proposalSnapshot struct {
	Commits map[string]*repos.ChangeCommit
	Open    []repos.OpenChange
	Probes  map[string]repos.ProbeEntry
}

func newProposalSnapshot() *proposalSnapshot {
	snapshot := &proposalSnapshot{
		Commits: map[string]*repos.ChangeCommit{},
		Open:    []repos.OpenChange{},
		Probes:  map[string]repos.ProbeEntry{},
	}
	return snapshot
}

func (s *proposalSnapshot) Count() int {
	return len(s.Open)
}

// proposalStore refreshes only when a proposals tip, the target, a rejected or a dropped ref changed; commit payloads survive refreshes.
type proposalStore struct {
	// mu serializes refreshes and guards fingerprint and snapshot (declared directly above the fields it guards).
	mu          sync.Mutex
	fingerprint string
	snapshot    *proposalSnapshot
}

func newProposalStore() *proposalStore {
	return &proposalStore{snapshot: newProposalSnapshot()}
}

// refresh lists the open changes, probes them in one agentwt call and reads the payload of every sha not held yet.
func (c *proposalStore) refresh(ctx context.Context, agentwtPath, repoPath, target string) (*proposalSnapshot, error) {
	open, err := repos.OpenChanges(ctx, agentwtPath, repoPath)
	if err != nil {
		return nil, err
	}

	shas := make([]string, 0, len(open))
	for _, change := range open {
		shas = append(shas, change.Commit.Sha)
	}
	probe, err := repos.Probe(ctx, agentwtPath, target, repoPath, shas)
	if err != nil {
		return nil, err
	}

	snapshot := &proposalSnapshot{Commits: make(map[string]*repos.ChangeCommit, len(open)), Open: open, Probes: probe.ByCommit()}
	for index := range open {
		change := &open[index]
		if _, ok := snapshot.Probes[change.Commit.Sha]; !ok {
			return nil, fmt.Errorf("proposalStore.refresh: Probe answered nothing for %s", change.Commit.Sha)
		}

		commit, ok := c.snapshot.Commits[change.Commit.Sha]
		if !ok {
			commit, err = repos.ReadChangeCommit(ctx, maxFileDiffLines, change.Message, repoPath, change.Commit.Sha)
			if err != nil {
				return nil, err
			}
		}
		snapshot.Commits[change.Commit.Sha] = commit
	}
	return snapshot, nil
}

// Snapshot answers the last snapshot with the error when git or agentwt fails, so the page and the nav still render.
func (c *proposalStore) Snapshot(ctx context.Context, agentwtPath, repoPath, target string) (*proposalSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fingerprint, err := proposalFingerprint(ctx, repoPath, target)
	if err != nil {
		return c.snapshot, fmt.Errorf("proposalStore.Snapshot: %w", err)
	}
	isCurrent := fingerprint != "" && fingerprint == c.fingerprint
	if isCurrent {
		return c.snapshot, nil
	}

	snapshot, err := c.refresh(ctx, agentwtPath, repoPath, target)
	if err != nil {
		return c.snapshot, fmt.Errorf("proposalStore.Snapshot: %w", err)
	}
	c.snapshot, c.fingerprint = snapshot, fingerprint
	return snapshot, nil
}

// proposalFingerprint lists the target tip, every proposals tip and every rejected and dropped ref with its object id.
func proposalFingerprint(ctx context.Context, repoPath, target string) (string, error) {
	output, err := shell.Run(
		ctx,
		repoPath,
		"git",
		"for-each-ref",
		"--format=%(refname) %(objectname)",
		"refs/heads/"+target,
		"refs/heads/"+repos.NightBranchPrefix+"*"+repos.ProposalsSuffix,
		repos.RejectedRefPrefix,
		repos.DroppedRefPrefix,
	)
	if err != nil {
		return "", fmt.Errorf("proposalFingerprint: %s: %w", strings.TrimSpace(output), err)
	}
	return output, nil
}
