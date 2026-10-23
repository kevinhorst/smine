package sessions

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
)

const routineBranchPrefix = "claude-routines/"

// PendingRequest treats RepoRoot's sessions as self-mining unless IsDev.
type PendingRequest struct {
	IsDev    bool
	Known    map[string]string
	Limit    int
	RepoRoot string
	Root     string
	Roster   []repos.Repo
	Since    time.Time
}

// NewPendingRequest treats repoRoot's sessions as self-mining unless isDev; known records come from root.
func NewPendingRequest(isDev bool, limit int, registryPath, repoRoot, root string) (*PendingRequest, error) {
	known, err := RecordFiles(root)
	if err != nil {
		return nil, fmt.Errorf("NewPendingRequest: %w", err)
	}

	request := &PendingRequest{
		IsDev:    isDev,
		Known:    known,
		Limit:    limit,
		RepoRoot: repoRoot,
		Root:     root,
		Roster:   repos.NewRegistry(registryPath).Repos(),
	}
	return request, nil
}

// isEligible also drops index stubs peek lists without any activity time.
func (r *PendingRequest) isEligible(candidate *peek.Session) bool {
	if _, isMined := r.Known[candidate.Id]; isMined {
		return false
	}
	isOutsideWindow := candidate.LastActive.IsZero() || candidate.LastActive.Before(r.Since)
	if isOutsideWindow || candidate.Live {
		return false
	}
	return r.IsDev || !r.isSelfMining(candidate)
}

func (r *PendingRequest) isSelfMining(candidate *peek.Session) bool {
	if strings.HasPrefix(candidate.GitBranch, routineBranchPrefix) {
		return true
	}
	return repos.IsUnder(candidate.Cwd, r.RepoRoot)
}

// repoFor attributes a cwd to its roster repo, UnknownScope when none owns it.
func (r *PendingRequest) repoFor(cwd string) string {
	name := repos.OwnerOf(cwd, r.Roster)
	if name == "" {
		return UnknownScope
	}
	return name
}

// PendingSession is one session to mine, with the record path it gets.
type PendingSession struct {
	Id string `json:"id"`

	Agent      string `json:"agent"`
	Branch     string `json:"branch"`
	Cwd        string `json:"cwd"`
	LastActive string `json:"lastActive"`
	Path       string `json:"path"`
	Repo       string `json:"repo"`
	Title      string `json:"title"`
	Week       string `json:"week"`
}

// SelectPending returns newest first, so a capped run mines the freshest sessions; Limit 0 means all.
func SelectPending(candidates []peek.Session, request *PendingRequest) []PendingSession {
	eligible := make([]peek.Session, 0, len(candidates))
	for index := range candidates {
		if request.isEligible(&candidates[index]) {
			eligible = append(eligible, candidates[index])
		}
	}
	slices.SortFunc(eligible, func(a, b peek.Session) int { return b.LastActive.Compare(a.LastActive) })
	if request.Limit > 0 && len(eligible) > request.Limit {
		eligible = eligible[:request.Limit]
	}

	pending := make([]PendingSession, 0, len(eligible))
	for index := range eligible {
		candidate := &eligible[index]
		repo, week := request.repoFor(candidate.Cwd), WeekKey(candidate.LastActive)
		session := PendingSession{
			Id:         candidate.Id,
			Agent:      candidate.Agent,
			Branch:     candidate.GitBranch,
			Cwd:        candidate.Cwd,
			LastActive: candidate.LastActive.UTC().Format(time.RFC3339),
			Path:       RecordPath(candidate.Id, request.Root, repo, week),
			Repo:       repo,
			Title:      candidate.Title,
			Week:       week,
		}
		pending = append(pending, session)
	}
	return pending
}
