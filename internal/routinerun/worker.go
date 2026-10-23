package routinerun

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	workerBranchPrefix  = "claude/"
	workerExtraTools    = ", Workflow, Skill, Write"
	workerSteerPrefix   = "worker-"
	workerStatusDone    = "done"
	workerStatusFailed  = "failed"
	workerStatusStopped = "stopped"
	workerWorktreesPath = ".claude/worktrees"
)

var (
	workerNamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	workerSessionPattern = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)
	workerSkillPattern   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

type WorkerResult struct {
	Branch     string  `json:"branch"`
	Commit     string  `json:"commit"`
	CostUsd    float64 `json:"cost_usd"`
	ExitStatus int     `json:"exit_status"`
	Name       string  `json:"name"`
	Result     string  `json:"result"`
	SessionId  string  `json:"session_id"`
	Status     string  `json:"status"`
}

// WorkerSpec omits the skill's slash: the permission gate reads an argv word starting with / as a path.
type WorkerSpec struct {
	Attended    bool
	Base        string
	BudgetUsd   string
	Effort      string
	KeepStopped bool // true = a stopped cell keeps its worktree for a resume
	Model       string
	Name        string
	Repo        string // the main checkout
	Resume      string // session id of this worker's stopped cell; "" = a fresh cell
	Skill       string
	SkillArgs   string
	Timeout     time.Duration
	TokenLabel  string // "" = the launching session's own login
}

func (s *WorkerSpec) claudeArgs() []string {
	tools := skills.AllowedTools(filepath.Join(skills.DefaultHomePath(), s.Skill))
	args := []string{"--allowedTools", strings.TrimPrefix(tools+workerExtraTools, ", ")}
	if s.Resume != "" {
		args = append(args, "--resume", s.Resume)
	}
	if s.Model != "" {
		args = append(args, "--model", s.Model)
	}
	if s.Effort != "" {
		args = append(args, "--effort", s.Effort)
	}
	if s.BudgetUsd != "" {
		args = append(args, "--max-budget-usd", s.BudgetUsd)
	}
	return args
}

// validate refuses an attended cell under the headless marker, so a headless cell never hands attendance on.
func (s *WorkerSpec) validate() error {
	// Name
	if !workerNamePattern.MatchString(s.Name) {
		return errors.New("WorkerSpec.validate: Invalid field Name")
	}

	// Base
	if s.Base == "" && s.Resume == "" {
		return errors.New("WorkerSpec.validate: Missing field Base")
	}

	// Resume
	if s.Resume != "" && !workerSessionPattern.MatchString(s.Resume) {
		return errors.New("WorkerSpec.validate: Invalid field Resume")
	}

	// Skill
	if !workerSkillPattern.MatchString(s.Skill) {
		return errors.New("WorkerSpec.validate: Invalid field Skill")
	}

	// BudgetUsd
	if s.BudgetUsd != "" {
		if _, err := strconv.ParseFloat(s.BudgetUsd, 64); err != nil {
			return errors.New("WorkerSpec.validate: Invalid field BudgetUsd")
		}
	}

	// Attended
	isMarked := os.Getenv(headlessEnvKey) != ""
	if s.Attended && isMarked {
		return errors.New("WorkerSpec.validate: Attended refused under the headless marker")
	}
	return nil
}

func (s *WorkerSpec) worktree(ctx context.Context) (*gitwt.Provisioned, error) {
	branch := workerBranchPrefix + s.Name
	root := filepath.Join(s.Repo, workerWorktreesPath)
	if s.Resume == "" {
		request := &gitwt.ProvisionRequest{Branch: branch, Head: s.Base, Repo: s.Repo, Root: root, Slug: s.Name}
		return gitwt.Provision(ctx, request)
	}

	path := filepath.Join(root, s.Name)
	head, err := git(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("WorkerSpec.worktree: No kept worktree at %s: %w", path, err)
	}

	if head != branch {
		return nil, fmt.Errorf("WorkerSpec.worktree: %s is on %s, not %s", path, head, branch)
	}
	kept := &gitwt.Provisioned{Branch: branch, Path: path}
	return kept, nil
}

// workerStatus reads a missing marker as stopped: the skill ended on one of its own stops.
func workerStatus(isFailed bool, marker string) string {
	if isFailed {
		return workerStatusFailed
	}

	if _, err := os.Stat(marker); err != nil {
		return workerStatusStopped
	}
	return workerStatusDone
}

// RunWorker keeps the branch for the orchestrator to merge.
func RunWorker(ctx context.Context, spec *WorkerSpec) (*WorkerResult, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	if spec.TokenLabel != "" {
		token, err := routines.ReadToken(spec.TokenLabel)
		if err != nil {
			return nil, fmt.Errorf("RunWorker: %w", err)
		}
		if err := os.Setenv("CLAUDE_CODE_OAUTH_TOKEN", token); err != nil {
			return nil, fmt.Errorf("RunWorker: %w", err)
		}
	}

	provisioned, err := spec.worktree(ctx)
	if err != nil {
		return nil, fmt.Errorf("RunWorker: %w", err)
	}

	steerEnv := map[string]string{"ROUTINE_STEER_ROOT": os.Getenv("ROUTINE_STEER_ROOT")}
	steerDir, err := routines.SteerDir(steerEnv, workerSteerPrefix+spec.Name)
	if err != nil {
		return nil, fmt.Errorf("RunWorker: %w", err)
	}

	prepareProcessEnv(spec.Repo)
	marker := filepath.Join(steerDir, routines.RunCompleteFileName)
	prompt := fmt.Sprintf("/%s %s", spec.Skill, spec.SkillArgs)
	if spec.Resume != "" {
		prompt = spec.SkillArgs
	}
	chat := &ChatSpec{
		Args:       spec.claudeArgs(),
		Attended:   spec.Attended,
		Complete:   CompleteMarker,
		Dir:        provisioned.Path,
		ExitGrace:  defaultExitGraceS * time.Second,
		MarkerPath: marker,
		Poll:       defaultPollS * time.Second,
		Prompt:     prompt + fmt.Sprintf(markerInstruction, marker),
		SteerDir:   steerDir,
		Timeout:    spec.Timeout,
	}
	outcome, err := RunChat(ctx, chat)
	if err != nil {
		return nil, fmt.Errorf("RunWorker: %w", err)
	}

	worktree := &groupWorktree{path: provisioned.Path, repoRoot: spec.Repo, routineName: spec.Name}
	isFailed := outcome.Status != exitOk || outcome.Envelope.isFailure()
	status := workerStatus(isFailed, marker)
	if err := worktree.land(ctx, isFailed); err != nil {
		return nil, fmt.Errorf("RunWorker: %w", err)
	}

	isKept := spec.KeepStopped && status == workerStatusStopped
	if !isKept {
		if err := worktree.release(ctx); err != nil {
			log.Printf("RunWorker: Worker %s: Worktree kept: %v", spec.Name, err)
		}
	}

	commit, err := git(ctx, spec.Repo, "rev-parse", "refs/heads/"+provisioned.Branch)
	if err != nil {
		return nil, fmt.Errorf("RunWorker: %w", err)
	}
	result := &WorkerResult{
		Branch:     provisioned.Branch,
		Commit:     commit,
		CostUsd:    outcome.Envelope.TotalCostUsd,
		ExitStatus: outcome.Status,
		Name:       spec.Name,
		Result:     outcome.Envelope.Result,
		SessionId:  outcome.Envelope.SessionId,
		Status:     status,
	}
	return result, nil
}
