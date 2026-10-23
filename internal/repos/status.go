package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/shell"
)

// statusSchema is the agentwt status document version this decoder reads.
const statusSchema = 1

// noWorktreeDirty marks a branch without a checked-out worktree.
const noWorktreeDirty = -1

// unknownCount marks AHEAD/BEHIND for an unresolved base, which is never guessed.
const unknownCount = -1

// unknownBase is the From sentinel for an unresolved base.
const unknownBase = "unknown"

type Commit struct {
	Line string
	Sha  string
}

// Containment prefers the base, then the default branch, then any other candidate.
type Containment struct {
	Branch string `json:"branch"`
	Rank   string `json:"rank"`
	Reason string `json:"reason"`
}

type OrderInfo struct {
	BlockedBy     []string
	ConflictsWith []string
	Paths         []string
	State         string // clean | blocked | nothing
	TrainPosition int
}

// SessionInfo.Alive is nil for agents without a pid (Codex).
type SessionInfo struct {
	Id            string `json:"id"`
	AccountUuid   string `json:"account_uuid"`
	Agent         string `json:"agent"`
	Alive         *bool  `json:"alive"`
	Entrypoint    string `json:"entrypoint"`
	HostSessionId string `json:"host_session_id"`
	Name          string `json:"name"`
	Pid           int    `json:"pid"`
	Status        string `json:"status"`
	UpdatedAt     string `json:"updated_at"`
	WaitingFor    string `json:"waiting_for"`
}

type WorktreeStatus struct {
	Ahead           int // unknownCount when From is "unknown"
	AheadCommits    []Commit
	Behind          int // unknownCount when From is "unknown"
	BehindCommits   []Commit
	BehindTruncated bool
	Branch          string // "" for a detached worktree row
	// Contained is nil when no non-agent branch holds the work.
	Contained       *Containment
	Detached        bool
	Dirty           int    // noWorktreeDirty when the branch has no worktree
	From            string // "unknown" when unresolved
	FromResolution  string // ref | sha | chain | reanchored | unknown
	Head            string // abbreviated tip sha (the Branch cell of a detached row)
	Kind            string // claude, claude-routines, codex, …; "" when outside every namespace
	LastCommit      string
	Order           *OrderInfo
	SafeToRemove    bool
	Session         *SessionInfo
	Unpicked        int
	UnpickedCommits []Commit // the own commits placed on no non-agent branch
	UnsafeReasons   []string // the conditions that made SafeToRemove false; empty when safe
	Untracked       int      // noWorktreeDirty when the branch has no worktree
	Worktree        string   // empty when none
}

type CheckoutStatus struct {
	Branch string
	Dirty  int
}

// statusDocument mirrors wire.rs Document.
type statusDocument struct {
	Kinds   []string      `json:"kinds"`
	Repo    statusRepo    `json:"repo"`
	Rows    []statusRow   `json:"rows"`
	Schema  int           `json:"schema"`
	Timings StatusTimings `json:"timings"`
}

type StatusScan struct {
	Kinds   []string
	Rows    []WorktreeStatus
	Timings StatusTimings
}

// StatusTimings mirrors wire.rs Timings.
type StatusTimings struct {
	AnalysisMs  int64 `json:"analysis_ms"`
	OrderMs     int64 `json:"order_ms"`
	WorktreesMs int64 `json:"worktrees_ms"`
}

type statusRepo struct {
	DefaultBranch string `json:"default_branch"`
}

type statusRow struct {
	Branch          *string         `json:"branch"`
	Commits         statusCommits   `json:"commits"`
	Containment     *Containment    `json:"containment"`
	Counts          statusCounts    `json:"counts"`
	From            statusFrom      `json:"from"`
	Head            statusHead      `json:"head"`
	Kind            *string         `json:"kind"`
	Order           *statusOrder    `json:"order"`
	SafeToRemove    bool            `json:"safe_to_remove"`
	Session         *SessionInfo    `json:"session"`
	Unpicked        int             `json:"unpicked"`
	UnpickedCommits []string        `json:"unpicked_commits"`
	UnsafeReasons   []string        `json:"unsafe_reasons"`
	Worktree        *statusWorktree `json:"worktree"`
}

type statusHead struct {
	CommittedAt string `json:"committed_at"`
	Sha         string `json:"sha"`
	ShortSha    string `json:"short_sha"`
	Subject     string `json:"subject"`
}

type statusWorktree struct {
	Detached  bool   `json:"detached"`
	Dirty     int    `json:"dirty"`
	Path      string `json:"path"`
	Untracked int    `json:"untracked"`
}

type statusFrom struct {
	Name       *string `json:"name"`
	Resolution string  `json:"resolution"`
}

type statusCounts struct {
	VsFrom *statusAheadBehind `json:"vs_from"`
}

type statusAheadBehind struct {
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
}

type statusCommits struct {
	Ahead           []statusCommitEntry `json:"ahead"`
	Behind          []statusCommitEntry `json:"behind"`
	BehindTruncated bool                `json:"behind_truncated"`
}

type statusCommitEntry struct {
	CommittedAt string `json:"committed_at"`
	IsMerge     bool   `json:"is_merge"`
	Sha         string `json:"sha"`
	ShortSha    string `json:"short_sha"`
	Subject     string `json:"subject"`
}

type statusOrder struct {
	ConflictsWith []string    `json:"conflicts_with"`
	Train         statusTrain `json:"train"`
	TrainPosition int         `json:"train_position"`
}

type statusTrain struct {
	BlockedBy []string `json:"blocked_by"`
	Paths     []string `json:"paths"`
	State     string   `json:"state"`
}

// Checkout excludes untracked files from the uncommitted-change count.
func Checkout(ctx context.Context, repoPath string) (*CheckoutStatus, error) {
	branch, err := shell.Run(ctx, repoPath, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(branch), err)
	}

	output, err := shell.Run(ctx, repoPath, "git", "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("Checkout: %s: %w", strings.TrimSpace(output), err)
	}

	dirty := 0
	for line := range strings.Lines(output) {
		if line != "\n" && line != "" && !strings.HasPrefix(line, "??") {
			dirty++
		}
	}
	return &CheckoutStatus{Branch: strings.TrimSpace(branch), Dirty: dirty}, nil
}

// Commits reads the decoded document, so a drill-down spawns no subprocess.
func Commits(class string, status *WorktreeStatus) []Commit {
	switch class {
	case "ahead":
		return status.AheadCommits
	case "behind":
		return status.BehindCommits
	case "unpicked":
		return status.UnpickedCommits
	}
	return nil
}

func Status(ctx context.Context, agentwtPath, kind, repoPath string) (*StatusScan, error) {
	args := agentwtArgs(repoPath, "status")
	if kind != "" {
		args = append(args, "--kind="+kind)
	}
	output, err := shell.Run(ctx, repoPath, agentwtPath, args...)
	if err != nil {
		return nil, fmt.Errorf("Status: %s: %w", strings.TrimSpace(output), err)
	}

	return decodeStatus(output)
}

// decodeStatus refuses an unknown schema rather than guess a field mapping.
func decodeStatus(output string) (*StatusScan, error) {
	var document statusDocument
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decodeStatus: Decode: %w", err)
	}
	if document.Schema != statusSchema {
		return nil, fmt.Errorf("decodeStatus: Unsupported schema %d", document.Schema)
	}

	rows := make([]WorktreeStatus, 0, len(document.Rows))
	for _, row := range document.Rows {
		rows = append(rows, row.view())
	}
	scan := &StatusScan{Kinds: document.Kinds, Rows: rows, Timings: document.Timings}
	return scan, nil
}

// view maps one wire row into the template model.
func (r *statusRow) view() WorktreeStatus {
	status := WorktreeStatus{
		Ahead:           unknownCount,
		Behind:          unknownCount,
		BehindTruncated: r.Commits.BehindTruncated,
		Branch:          deref(r.Branch),
		Contained:       r.Containment,
		Dirty:           noWorktreeDirty,
		From:            unknownBase,
		FromResolution:  r.From.Resolution,
		Head:            r.Head.ShortSha,
		Kind:            deref(r.Kind),
		LastCommit:      formatCommitTime(r.Head.CommittedAt),
		SafeToRemove:    r.SafeToRemove,
		Session:         r.Session,
		Unpicked:        r.Unpicked,
		UnsafeReasons:   r.UnsafeReasons,
		Untracked:       noWorktreeDirty,
	}
	if r.From.Name != nil {
		status.From = *r.From.Name
	}
	if r.Counts.VsFrom != nil {
		status.Ahead = r.Counts.VsFrom.Ahead
		status.Behind = r.Counts.VsFrom.Behind
	}
	if r.Worktree != nil {
		status.Detached = r.Worktree.Detached
		status.Dirty = r.Worktree.Dirty
		status.Untracked = r.Worktree.Untracked
		status.Worktree = r.Worktree.Path
	}
	if r.Order != nil {
		status.Order = &OrderInfo{
			BlockedBy:     r.Order.Train.BlockedBy,
			ConflictsWith: r.Order.ConflictsWith,
			Paths:         r.Order.Train.Paths,
			State:         r.Order.Train.State,
			TrainPosition: r.Order.TrainPosition,
		}
	}

	status.AheadCommits = commitLines(r.Commits.Ahead)
	status.BehindCommits = commitLines(r.Commits.Behind)
	status.UnpickedCommits = unpickedLines(r.Commits.Ahead, r.UnpickedCommits)
	return status
}

func commitLines(entries []statusCommitEntry) []Commit {
	var commits []Commit
	for _, entry := range entries {
		line := entry.ShortSha + "  " + formatCommitTime(entry.CommittedAt) + "  " + entry.Subject
		commits = append(commits, Commit{Line: line, Sha: entry.Sha})
	}
	return commits
}

func unpickedLines(ahead []statusCommitEntry, unpicked []string) []Commit {
	placedNowhere := make(map[string]bool, len(unpicked))
	for _, sha := range unpicked {
		placedNowhere[sha] = true
	}

	var lines []Commit
	for _, entry := range ahead {
		if !placedNowhere[entry.Sha] {
			continue
		}

		line := entry.ShortSha + "  " + formatCommitTime(entry.CommittedAt) + "  " + entry.Subject
		lines = append(lines, Commit{Line: line, Sha: entry.Sha})
	}
	return lines
}

func formatCommitTime(rfc3339 string) string {
	parsed, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return parsed.UTC().Format("2006-01-02 15:04")
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
