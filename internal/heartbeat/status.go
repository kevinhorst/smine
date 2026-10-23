package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// The install markers (install.Values.Markers) fill the port from the heartbeat.port setting.
const AddrTemplate = "127.0.0.1:{{HEARTBEAT_PORT}}"

// documentSchema is the heartbeat status document version this decoder reads.
const documentSchema = 1

const (
	exitForeignHome = 4
	exitUnreachable = 3
)

var (
	ErrForeignHome = errors.New("Status: Heartbeat serves another Claude home")
	ErrUnreachable = errors.New("Status: Heartbeat unreachable")
)

type Blocked struct {
	CandidateRule string `json:"candidate_rule"`
	DeniedAt      string `json:"denied_at"`
	Program       string `json:"program"`
}

// Target is empty when the branch lands nowhere.
type Branch struct {
	Agents        []BranchAgent `json:"agents"`
	Branch        string        `json:"branch"`
	Repo          string        `json:"repo"`
	Sessions      []string      `json:"sessions"`
	Since         string        `json:"since"`
	State         BranchState   `json:"state"`
	Target        string        `json:"target"`
	TrainPosition int           `json:"train_position"`
	WorkItems     []string      `json:"work_items"`
	Worktree      string        `json:"worktree"`
}

// BranchAgent is one P2 BranchAgentRow: a worker subagent on the branch.
type BranchAgent struct {
	AgentId   string `json:"agent_id"`
	SessionId string `json:"session_id"`
}

type BranchState string

const (
	BranchStateConflicting BranchState = "conflicting"
	BranchStateMergeReady  BranchState = "merge_ready"
	BranchStateMerged      BranchState = "merged"
	BranchStateNoWork      BranchState = "no_work"
	BranchStateRemovable   BranchState = "removable"
	BranchStateUncommitted BranchState = "uncommitted"
	BranchStateUnmerged    BranchState = "unmerged"
)

func (s BranchState) Tone() Tone {
	switch s {
	case BranchStateConflicting:
		return ToneError
	case BranchStateMergeReady, BranchStateMerged:
		return ToneOk
	case BranchStateNoWork, BranchStateRemovable:
		return ToneDim
	case BranchStateUncommitted, BranchStateUnmerged:
		return ToneAction
	}
	return ToneDim
}

// The binary checks the daemon's /healthz identity before every read.
type Cli struct {
	Addr       string
	BinaryPath string
	ClaudeHome string
}

func (c *Cli) Status(ctx context.Context, fleet string) (*Document, error) {
	args := c.args("status", "--format", "json")
	if fleet != "" {
		args = append(args, "--fleet", fleet)
	}

	output, err := shell.Run(ctx, "", c.BinaryPath, args...)
	if err != nil {
		return nil, classifyRunError(output, err)
	}

	return decodeDocument(output)
}

func (c *Cli) args(subcommand ...string) []string {
	return append([]string{"--addr", c.Addr, "--claude-home", c.ClaudeHome}, subcommand...)
}

// A side is a branch name, or a worktree path when detached.
type Collision struct {
	A       string         `json:"a"`
	B       string         `json:"b"`
	Level   CollisionLevel `json:"level"`
	Paths   []string       `json:"paths"`
	Plans   []string       `json:"plans"`
	Repo    string         `json:"repo"`
	Since   string         `json:"since"`
	Symbols []string       `json:"symbols"`
}

type CollisionLevel string

const (
	CollisionLevelConfirmed     CollisionLevel = "confirmed"
	CollisionLevelPlannedFile   CollisionLevel = "planned_file"
	CollisionLevelPlannedSymbol CollisionLevel = "planned_symbol"
	CollisionLevelTouched       CollisionLevel = "touched"
)

// Mirrors cmd/heartbeat/src/wire.rs Document; sections later phases add decode as empty when absent.
type Document struct {
	Branches      []Branch    `json:"branches"`
	Collisions    []Collision `json:"collisions"`
	Fleets        []Fleet     `json:"fleets"`
	Heartbeat     Heartbeat   `json:"heartbeat"`
	Landings      []Landing   `json:"landings"`
	Members       []Member    `json:"members"`
	Repos         []Repo      `json:"repos"`
	Schema        int         `json:"schema"`
	Sensors       []Sensor    `json:"sensors"`
	Sessions      []Session   `json:"sessions"`
	StagingTarget string      `json:"staging_target"`
	Trains        []Train     `json:"trains"`
	WorkItems     []WorkItem  `json:"work_items"`
}

// Repo and target come from the active subscription; a dissolved run lives in its archive file.
type Fleet struct {
	Id            string         `json:"id"`
	Foreign       []string       `json:"foreign"`
	Instance      string         `json:"instance"`
	Journal       []JournalEntry `json:"journal"`
	MemberCount   int            `json:"members"`
	MergeBranch   string         `json:"merge_branch"`
	Repo          string         `json:"repo"`
	StartedAt     string         `json:"started_at"`
	Subscriptions []Subscription `json:"subscriptions"`
	Synced        bool           `json:"synced"`
	Target        string         `json:"target"`
	TargetState   TargetState    `json:"target_state"`
}

// subscriptionActive is the wire name of a live subscription's state.
const subscriptionActive = "active"

// Detail says why when refused or ended.
type Subscription struct {
	Detail            string `json:"detail"`
	Fleet             string `json:"fleet"`
	OutsideCollisions bool   `json:"outside_collisions"`
	Repo              string `json:"repo"`
	SessionId         string `json:"session_id"`
	Since             string `json:"since"`
	State             string `json:"state"`
	Target            string `json:"target"`
}

// Set by `subscribe --outside-collisions`.
func (f *Fleet) OutsideCollisions() bool {
	for _, subscription := range f.Subscriptions {
		if subscription.State == subscriptionActive {
			return subscription.OutsideCollisions
		}
	}
	return false
}

// JournalNewestFirst is the fleet's journal in reverse recording order.
func (f *Fleet) JournalNewestFirst() []JournalEntry {
	entries := make([]JournalEntry, 0, len(f.Journal))
	for _, entry := range slices.Backward(f.Journal) {
		entries = append(entries, entry)
	}
	return entries
}

// Heartbeat is the answering daemon (wire.rs HeartbeatInfo).
type Heartbeat struct {
	Addr          string `json:"addr"`
	ClaudeHome    string `json:"claude_home"`
	EventsApplied uint64 `json:"events_applied"`
	StartedAt     string `json:"started_at"`
	Version       string `json:"version"`
}

// JournalEntry is one P3 JournalRow: an act the fleet's orchestrator recorded.
type JournalEntry struct {
	At      string   `json:"at"`
	By      string   `json:"by"`
	Detail  string   `json:"detail"`
	Kind    string   `json:"kind"`
	Members []string `json:"members"`
	Subject string   `json:"subject"`
}

// Holder is nil while the turn is free; release facts are set only on the staging target's row.
type Landing struct {
	Branch  string        `json:"branch"`
	Holder  *LandingTurn  `json:"holder"`
	Queue   []LandingTurn `json:"queue"`
	Release *Release      `json:"release"`
	Repo    string        `json:"repo"`
}

// Title is empty until the registry named the session.
type LandingTurn struct {
	SessionId string `json:"session_id"`
	Since     string `json:"since"`
	Title     string `json:"title"`
}

// A worker (ViaWorker) is its branch and has no session row of its own.
type Member struct {
	Branch    string `json:"branch"`
	Delivered bool   `json:"delivered"`
	Fleet     string `json:"fleet"`
	MemberId  string `json:"member"`
	SessionId string `json:"session_id"`
	Token     string `json:"token"`
	Via       Via    `json:"via"`
}

// LastEvent is the session's most recent hook: its name and receipt time.
type LastEvent struct {
	At   string `json:"at"`
	Name string `json:"name"`
}

// Release is one ReleaseRow: the staging target against the default branch.
type Release struct {
	Exists     bool `json:"exists"`
	Releasable bool `json:"releasable"`
	Unreleased int  `json:"unreleased"`
}

// State folds Exists and Releasable into the one state the page shows.
func (r *Release) State() ReleaseState {
	if !r.Exists {
		return ReleaseStateAbsent
	}

	if r.Releasable {
		return ReleaseStateReleasable
	}
	return ReleaseStateDiverged
}

// Diverged when the default branch holds commits the staging target lacks.
type ReleaseState string

const (
	ReleaseStateAbsent     ReleaseState = "absent"
	ReleaseStateDiverged   ReleaseState = "diverged"
	ReleaseStateReleasable ReleaseState = "releasable"
)

func (s ReleaseState) Tone() Tone {
	switch s {
	case ReleaseStateAbsent:
		return ToneDim
	case ReleaseStateDiverged:
		return ToneAction
	case ReleaseStateReleasable:
		return ToneOk
	}
	return ToneDim
}

// Repo is one P2 RepoRow: a registered repo's scan health.
type Repo struct {
	Name      string   `json:"name"`
	Path      string   `json:"repo"`
	ScanError string   `json:"scan_error"`
	Warnings  []string `json:"warnings"`
}

type Sensor struct {
	Detail string      `json:"detail"`
	Name   string      `json:"name"`
	Since  string      `json:"since"`
	State  SensorState `json:"state"`
}

type SensorState string

const (
	SensorStateDegraded SensorState = "degraded"
	SensorStateOk       SensorState = "ok"
)

// Session is one wire.rs SessionRow, the fields the page and the queue read.
type Session struct {
	Id           string       `json:"session_id"`
	ActiveSkill  string       `json:"active_skill"`
	Blocked      *Blocked     `json:"blocked"`
	LastEvent    *LastEvent   `json:"last_event"`
	SinceRestart bool         `json:"since_restart"`
	State        SessionState `json:"state"`
	StateSince   string       `json:"state_since"`
	Subagents    []Subagent   `json:"subagents"`
	Title        string       `json:"title"`
	WaitingFor   string       `json:"waiting_for"`
}

type SessionState string

const (
	SessionStateBlocked SessionState = "blocked"
	SessionStateDied    SessionState = "died"
	SessionStateEnded   SessionState = "ended"
	SessionStateIdle    SessionState = "idle"
	SessionStateRunning SessionState = "running"
	SessionStateWaiting SessionState = "waiting"
)

func (s SessionState) Tone() Tone {
	switch s {
	case SessionStateBlocked, SessionStateDied, SessionStateWaiting:
		return ToneError
	case SessionStateEnded, SessionStateIdle:
		return ToneDim
	case SessionStateRunning:
		return ToneAction
	}
	return ToneDim
}

type Subagent struct {
	Id        string        `json:"agent_id"`
	AgentType string        `json:"agent_type"`
	StartedAt string        `json:"started_at"`
	State     SubagentState `json:"state"`
}

type SubagentState string

const (
	SubagentStateRunning SubagentState = "running"
	SubagentStateStopped SubagentState = "stopped"
)

func (s SubagentState) Tone() Tone {
	switch s {
	case SubagentStateRunning:
		return ToneAction
	case SubagentStateStopped:
		return ToneDim
	}
	return ToneDim
}

type TargetState string

const (
	TargetStateIntegrated TargetState = "integrated"
	TargetStateMissing    TargetState = "missing"
	TargetStateOpen       TargetState = "open"
)

func (s TargetState) Tone() Tone {
	switch s {
	case TargetStateIntegrated:
		return ToneOk
	case TargetStateMissing:
		return ToneError
	case TargetStateOpen:
		return ToneAction
	}
	return ToneDim
}

// The page maps it to a badge-<tone> class.
type Tone string

const (
	ToneAction Tone = "action"
	ToneDim    Tone = "dim"
	ToneError  Tone = "error"
	ToneOk     Tone = "ok"
)

// Train is one P2 TrainRow: a merge train of one repo, steps in merge order.
type Train struct {
	Base  string      `json:"base"`
	Repo  string      `json:"repo"`
	Steps []TrainStep `json:"steps"`
}

type TrainStep struct {
	BlockedBy []string       `json:"blocked_by"`
	Branch    string         `json:"branch"`
	Paths     []string       `json:"paths"`
	Position  int            `json:"position"`
	State     TrainStepState `json:"state"`
}

// TrainStepState is a step outcome: clean, blocked or nothing.
type TrainStepState string

const (
	TrainStepStateBlocked TrainStepState = "blocked"
	TrainStepStateClean   TrainStepState = "clean"
	TrainStepStateNothing TrainStepState = "nothing"
)

func (s TrainStepState) Tone() Tone {
	switch s {
	case TrainStepStateBlocked:
		return ToneError
	case TrainStepStateClean:
		return ToneOk
	case TrainStepStateNothing:
		return ToneDim
	}
	return ToneDim
}

// Via is why a member belongs to its fleet: joined, subscriber or worker.
type Via string

const (
	ViaJoined     Via = "joined"
	ViaSubscriber Via = "subscriber"
	ViaWorker     Via = "worker"
)

// Role is the member's part in the fleet as the page names it.
func (v Via) Role() string {
	switch v {
	case ViaJoined:
		return "member"
	case ViaSubscriber:
		return "operator"
	case ViaWorker:
		return "worker"
	}
	return string(v)
}

// WorkItem is one P2 WorkItemRow: a plan dir in flight.
type WorkItem struct {
	Branch         string   `json:"branch"`
	PlanDir        string   `json:"plan_dir"`
	PlannedFiles   []string `json:"planned_files"`
	PlannedSymbols []string `json:"planned_symbols"`
	Repo           string   `json:"repo"`
	Worktree       string   `json:"worktree"`
}

// Every failure other than exit codes 3 and 4 keeps the CLI output.
func classifyRunError(output string, err error) error {
	detail := strings.TrimSpace(output)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("Status: %s: %w", detail, err)
	}

	switch exitErr.ExitCode() {
	case exitForeignHome:
		return fmt.Errorf("%w: %s", ErrForeignHome, detail)
	case exitUnreachable:
		return fmt.Errorf("%w: %s", ErrUnreachable, detail)
	}
	return fmt.Errorf("Status: %s: %w", detail, err)
}

// A schema other than documentSchema is a hard error, never a guessed field mapping.
func decodeDocument(output string) (*Document, error) {
	var document Document
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decodeDocument: Decode: %w", err)
	}

	if document.Schema != documentSchema {
		return nil, fmt.Errorf("decodeDocument: Unsupported schema %d", document.Schema)
	}
	return &document, nil
}
