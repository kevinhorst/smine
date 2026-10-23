//! Wire types of the status document (schema 1) — the one contract the CLI, the orchestrator (P3) and the fleet page
//! (P4) read. Built from the fleet by [`crate::status::document`], never by serializing domain types.
//!
//! Optional fields: absent = not applicable (`Option<T>` + `skip_serializing_if`); no field is `null`.
//! `pid`, `cwd`, `title`, `active_skill`, `last_event` absent until known; `waiting_for` only in `waiting`; `blocked`
//! only in `blocked`; `end_reason` only in `ended` and `died`; subagent `model` absent until its transcript was
//! read; sensor `detail` only when degraded; a branch row's `target` only when the branch has a landing target; a
//! landing's `holder` only while the turn is held, its `release` only on the staging target's row, a landing turn's
//! `title` absent until the registry named the session; `staging_target` absent when the install sets none.
//! Timestamps are RFC 3339 UTC strings. Every enum field is a dedicated `snake_case` wire enum, pinned by tests.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::branch::BranchState;
use crate::collide::Level;
use crate::fleet::{
    FleetInstance, JournalEntry, LastDigest, LastEvent, Member, Outcome, PendingDenial, Sensor,
    SensorName, SensorState, Session, SessionState, Subagent, Subscription, SubscriptionState,
    TargetState, Via,
};
use crate::ids::{AgentId, FleetId, MemberId, MessagingToken, SessionId};
use crate::land::{LandReply, TypedEnd};
use crate::protocol::ARCHIVE_VERSION;
#[cfg(doc)]
use crate::protocol::{RUNTIME_SCHEMA, STATUS_SCHEMA};
use crate::runtime::{Acted, FindingKind, Mode, Move};

/// Root envelope.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct StatusDocument {
    /// Always [`STATUS_SCHEMA`].
    pub schema: u32,
    /// The daemon that answered.
    pub heartbeat: HeartbeatInfo,
    /// The staging target (`heartbeat.staging_target`): the landing target the done skill creates and catches up;
    /// absent when the install sets none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub staging_target: Option<String>,
    /// Health of every edge that reported.
    pub sensors: Vec<SensorRow>,
    /// Sessions matching the query, in session-id order.
    pub sessions: Vec<SessionRow>,
    /// Registered repos with their scan health, in path order.
    #[serde(default)]
    pub repos: Vec<RepoRow>,
    /// Rows of the registered repos with their integration state, by repo then row.
    #[serde(default)]
    pub branches: Vec<BranchRow>,
    /// Plan dirs in flight, by repo then worktree then dir.
    #[serde(default)]
    pub work_items: Vec<WorkItemRow>,
    /// Collisions, one per branch pair and level.
    #[serde(default)]
    pub collisions: Vec<CollisionRow>,
    /// Merge trains per repo: the default branch's first, then each requested base.
    #[serde(default)]
    pub trains: Vec<TrainRow>,
    /// One landing per registered repo and target with a holder or a queue, plus each repo's staging row with the
    /// staging target's release facts; by repo, then target.
    #[serde(default)]
    pub landings: Vec<LandingRow>,
    /// Live fleet runs, plus names only a refused subscription carries (only the requested one under `?fleet=`),
    /// ordered by id; a dissolved run lives in its archive file.
    #[serde(default)]
    pub fleets: Vec<FleetRow>,
    /// Members of those fleets, ordered by fleet, session, subagent; their state is in `sessions`.
    #[serde(default)]
    pub members: Vec<MemberRow>,
}

/// The daemon that answered.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct HeartbeatInfo {
    /// Crate version.
    pub version: String,
    /// The claude home this daemon serves.
    pub claude_home: String,
    /// The bound address.
    pub addr: String,
    /// Daemon start.
    pub started_at: String,
    /// Events applied since start.
    pub events_applied: u64,
    /// agentwt scans reported back.
    #[serde(default)]
    pub scans_completed: u64,
    /// Highest number of scans in flight for one repo (the load bound is 1).
    #[serde(default)]
    pub scan_in_flight_high_water: u64,
}

/// `GET /healthz`: identity only.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct Health {
    /// Crate version.
    pub version: String,
    /// The claude home this daemon serves.
    pub claude_home: String,
    /// The bound address.
    pub addr: String,
}

/// One sensor's health.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct SensorRow {
    /// The edge.
    pub name: SensorWire,
    /// Its state.
    pub state: SensorStateWire,
    /// Why, when degraded.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
    /// When the state was entered.
    pub since: String,
}

/// Sensor names.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SensorWire {
    /// HTTP edge.
    Hooks,
    /// Registry watch.
    Registry,
    /// Process-exit watch.
    ProcessExit,
    /// Denial ledger tail.
    Ledger,
    /// Git ref watch.
    Git,
    /// agentwt scans.
    Agentwt,
    /// peek client.
    Peek,
    /// Action-ledger writer.
    ActionLedger,
    /// Config file watch.
    Config,
}

/// Sensor states.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SensorStateWire {
    /// Delivering events.
    Ok,
    /// Not delivering, or suspect; see `detail`.
    Degraded,
}

/// One session.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct SessionRow {
    /// Session id.
    pub session_id: String,
    /// Lifecycle state.
    pub state: SessionStateWire,
    /// When `state` last changed.
    pub state_since: String,
    /// The daemon did not observe this session's start; earlier history is unknown.
    pub since_restart: bool,
    /// The process.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pid: Option<u32>,
    /// Working directory.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cwd: Option<String>,
    /// Session title (registry `name`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /// Last slash command or `Skill` call of the root agent.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub active_skill: Option<String>,
    /// What a `waiting` session waits on.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub waiting_for: Option<String>,
    /// The denial a `blocked` session is blocked on.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub blocked: Option<Blocked>,
    /// Why an `ended` or `died` session ended.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub end_reason: Option<String>,
    /// The last hook event.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_event: Option<LastEventRow>,
    /// Subagents, in agent-id order.
    pub subagents: Vec<SubagentRow>,
}

/// The session lifecycle states.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SessionStateWire {
    /// Working on a turn.
    Running,
    /// Waiting on a human.
    Waiting,
    /// The turn ended on an unresolved denial.
    Blocked,
    /// No turn in progress.
    Idle,
    /// `SessionEnd` arrived.
    Ended,
    /// The process exited without `SessionEnd`, or `StopFailure`.
    Died,
}

/// An unresolved denial.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct Blocked {
    /// `Bash:<first word>` or the tool name.
    pub program: String,
    /// The allow rule that would admit the call.
    pub candidate_rule: String,
    /// The ledger timestamp.
    pub denied_at: String,
}

/// The last hook event.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct LastEventRow {
    /// `hook_event_name`.
    pub name: String,
    /// When it arrived.
    pub at: String,
}

/// One subagent.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct SubagentRow {
    /// `agent_id`.
    pub agent_id: String,
    /// `agent_type`.
    pub agent_type: String,
    /// Running or stopped.
    pub state: SubagentStateWire,
    /// `SubagentStart` time.
    pub started_at: String,
    /// The model its transcript shows; absent until read.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
}

/// Subagent states.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SubagentStateWire {
    /// No `SubagentStop` yet.
    Running,
    /// `SubagentStop` arrived.
    Stopped,
}

/// One registered repo's scan health.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct RepoRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// Registry name.
    pub name: String,
    /// Why the last scan failed; absent after a good scan.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scan_error: Option<String>,
    /// agentwt and work-item discovery warnings of the last good scan.
    pub warnings: Vec<String>,
}

/// One landing of a registered repo: a repository and a landing target (merge strategy targets C7).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct LandingRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// The landing target.
    pub branch: String,
    /// The staging target's release facts; absent on every other target's row.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub release: Option<ReleaseRow>,
    /// The turn holder; absent while the turn is free.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub holder: Option<LandingTurnRow>,
    /// The waiters, oldest first.
    pub queue: Vec<LandingTurnRow>,
}

/// The staging target against the default branch.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct ReleaseRow {
    /// The staging target is a local branch.
    pub exists: bool,
    /// The default branch's tip is an ancestor of the staging target: a release is a fast-forward.
    pub releasable: bool,
    /// Commits the staging target holds and the default branch lacks.
    pub unreleased: u32,
}

/// One session's place in a landing.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct LandingTurnRow {
    /// The session.
    pub session_id: String,
    /// The session's title (registry `name`); absent until the registry named it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    /// Since when it holds the turn or waits (RFC 3339).
    pub since: String,
}

/// One agentwt row of a registered repo.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct BranchRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// Branch name; absent for a detached worktree.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    /// Worktree path; absent for a branch without a checkout.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub worktree: Option<String>,
    /// Integration state.
    pub state: BranchStateWire,
    /// When the state was entered (RFC 3339).
    pub since: String,
    /// The landing target; absent for a branch that lands nowhere.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    /// 1-based position in the train the state was judged against; absent when the branch has no step in it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub train_position: Option<usize>,
    /// Sessions whose cwd lies in the worktree, by id.
    pub sessions: Vec<String>,
    /// Plan dirs of the worktree.
    pub work_items: Vec<String>,
    /// Worker subagents on this branch, by construction (D20), in session then agent order.
    #[serde(default)]
    pub agents: Vec<BranchAgentRow>,
}

/// A worker subagent running on this branch (attributed by construction, `claude/agent-<agent_id>`).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct BranchAgentRow {
    /// The session that spawned the subagent.
    pub session_id: String,
    /// The subagent.
    pub agent_id: String,
}

/// Branch states on the wire (concept D16).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum BranchStateWire {
    /// No own commits, clean worktree.
    NoWork,
    /// Tracked changes or untracked files.
    Uncommitted,
    /// Own commits, no train step.
    Unmerged,
    /// Train step blocked.
    Conflicting,
    /// Train step clean.
    MergeReady,
    /// Held by a non-agent branch.
    Merged,
    /// Safe to remove.
    Removable,
}

/// One plan dir in flight.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct WorkItemRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// Plan dir inside its feature folder.
    pub plan_dir: String,
    /// Worktree root.
    pub worktree: String,
    /// The worktree's branch; absent before the worktree was scanned or when detached.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    /// Planned files and globs.
    pub planned_files: Vec<String>,
    /// Planned `path::symbol` entries.
    pub planned_symbols: Vec<String>,
}

/// One collision.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct CollisionRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// Level.
    pub level: CollisionLevelWire,
    /// One side: branch name, or worktree path when detached.
    pub a: String,
    /// The other side.
    pub b: String,
    /// Shared paths.
    pub paths: Vec<String>,
    /// Shared `path::symbol` entries (planned-symbol).
    pub symbols: Vec<String>,
    /// Plan dirs involved (planned levels).
    pub plans: Vec<String>,
    /// When the pair first collided at this level (RFC 3339).
    pub since: String,
}

/// Collision levels on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum CollisionLevelWire {
    /// Same planned file.
    PlannedFile,
    /// Same planned enclosing signature.
    PlannedSymbol,
    /// Same edited file.
    Touched,
    /// Merge conflict.
    Confirmed,
}

/// One merge train of one repo.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct TrainRow {
    /// The repo's main-checkout path.
    pub repo: String,
    /// The branch the train starts from.
    pub base: String,
    /// Steps in merge order.
    pub steps: Vec<TrainStepRow>,
}

/// One train step.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct TrainStepRow {
    /// Branch name.
    pub branch: String,
    /// 1-based position.
    pub position: usize,
    /// Outcome.
    pub state: TrainStateWire,
    /// Branches merged before a blocked step.
    pub blocked_by: Vec<String>,
    /// Unresolved paths of a blocked step.
    pub paths: Vec<String>,
}

/// Train step outcomes on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum TrainStateWire {
    /// Merged cleanly.
    Clean,
    /// Conflicted.
    Blocked,
    /// Nothing to merge.
    Nothing,
}

impl SessionStateWire {
    /// The wire string (the text table's column).
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            SessionStateWire::Running => "running",
            SessionStateWire::Waiting => "waiting",
            SessionStateWire::Blocked => "blocked",
            SessionStateWire::Idle => "idle",
            SessionStateWire::Ended => "ended",
            SessionStateWire::Died => "died",
        }
    }
}

impl SensorWire {
    /// The wire string (the text table's degraded lines).
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            SensorWire::Hooks => "hooks",
            SensorWire::Registry => "registry",
            SensorWire::ProcessExit => "process_exit",
            SensorWire::Ledger => "ledger",
            SensorWire::Git => "git",
            SensorWire::Agentwt => "agentwt",
            SensorWire::Peek => "peek",
            SensorWire::ActionLedger => "action_ledger",
            SensorWire::Config => "config",
        }
    }
}

impl From<&SessionState> for SessionStateWire {
    fn from(state: &SessionState) -> SessionStateWire {
        match state {
            SessionState::Running => SessionStateWire::Running,
            SessionState::Waiting { .. } => SessionStateWire::Waiting,
            SessionState::Blocked { .. } => SessionStateWire::Blocked,
            SessionState::Idle => SessionStateWire::Idle,
            SessionState::Ended { .. } => SessionStateWire::Ended,
            SessionState::Died { .. } => SessionStateWire::Died,
        }
    }
}

impl From<SensorName> for SensorWire {
    fn from(name: SensorName) -> SensorWire {
        match name {
            SensorName::Hooks => SensorWire::Hooks,
            SensorName::Registry => SensorWire::Registry,
            SensorName::ProcessExit => SensorWire::ProcessExit,
            SensorName::Ledger => SensorWire::Ledger,
            SensorName::Git => SensorWire::Git,
            SensorName::Agentwt => SensorWire::Agentwt,
            SensorName::Peek => SensorWire::Peek,
            SensorName::ActionLedger => SensorWire::ActionLedger,
            SensorName::Config => SensorWire::Config,
        }
    }
}

impl From<BranchState> for BranchStateWire {
    fn from(state: BranchState) -> Self {
        match state {
            BranchState::NoWork => BranchStateWire::NoWork,
            BranchState::Uncommitted => BranchStateWire::Uncommitted,
            BranchState::Unmerged => BranchStateWire::Unmerged,
            BranchState::Conflicting => BranchStateWire::Conflicting,
            BranchState::MergeReady => BranchStateWire::MergeReady,
            BranchState::Merged => BranchStateWire::Merged,
            BranchState::Removable => BranchStateWire::Removable,
        }
    }
}

impl From<Level> for CollisionLevelWire {
    fn from(level: Level) -> Self {
        match level {
            Level::PlannedFile => CollisionLevelWire::PlannedFile,
            Level::PlannedSymbol => CollisionLevelWire::PlannedSymbol,
            Level::Touched => CollisionLevelWire::Touched,
            Level::Confirmed => CollisionLevelWire::Confirmed,
        }
    }
}

impl From<SensorState> for SensorStateWire {
    fn from(state: SensorState) -> SensorStateWire {
        match state {
            SensorState::Ok => SensorStateWire::Ok,
            SensorState::Degraded => SensorStateWire::Degraded,
        }
    }
}

impl From<(&SensorName, &Sensor)> for SensorRow {
    fn from((name, sensor): (&SensorName, &Sensor)) -> SensorRow {
        SensorRow {
            name: SensorWire::from(*name),
            state: SensorStateWire::from(sensor.state),
            detail: sensor.detail.clone(),
            since: sensor.since.to_string(),
        }
    }
}

impl From<&PendingDenial> for Blocked {
    fn from(denial: &PendingDenial) -> Blocked {
        Blocked {
            program: denial.program.as_str().to_owned(),
            candidate_rule: denial.candidate_rule.clone(),
            denied_at: denial.at.to_string(),
        }
    }
}

impl From<&LastEvent> for LastEventRow {
    fn from(event: &LastEvent) -> LastEventRow {
        LastEventRow {
            name: event.name.clone(),
            at: event.at.to_string(),
        }
    }
}

impl From<(&AgentId, &Subagent)> for SubagentRow {
    fn from((id, subagent): (&AgentId, &Subagent)) -> SubagentRow {
        SubagentRow {
            agent_id: id.0.clone(),
            agent_type: subagent.agent_type.clone(),
            state: if subagent.running {
                SubagentStateWire::Running
            } else {
                SubagentStateWire::Stopped
            },
            started_at: subagent.started_at.to_string(),
            model: subagent.model.clone(),
        }
    }
}

impl From<&Session> for SessionRow {
    fn from(session: &Session) -> SessionRow {
        let (waiting_for, blocked, end_reason) = match &session.state {
            SessionState::Waiting { on } => (Some(on.clone()), None, None),
            SessionState::Blocked { denial } => (None, Some(Blocked::from(denial)), None),
            SessionState::Ended { reason } => (None, None, Some(reason.clone())),
            SessionState::Died { cause } => (None, None, Some(cause.clone())),
            SessionState::Running | SessionState::Idle => (None, None, None),
        };
        SessionRow {
            session_id: session.id.0.clone(),
            state: SessionStateWire::from(&session.state),
            state_since: session.state_since.to_string(),
            since_restart: session.since_restart,
            pid: session.pid.map(|pid| pid.0),
            cwd: session
                .cwd
                .as_deref()
                .and_then(Path::to_str)
                .map(str::to_owned),
            title: session.title.clone(),
            active_skill: session.active_skill.clone(),
            waiting_for,
            blocked,
            end_reason,
            last_event: session.last_event.as_ref().map(LastEventRow::from),
            subagents: session.subagents.iter().map(SubagentRow::from).collect(),
        }
    }
}

/// One fleet.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct FleetRow {
    /// The `FleetId`.
    pub id: String,
    /// The live run's UUID; absent for a name only a refused subscription carries.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instance: Option<String>,
    /// When the live run started (RFC 3339); absent with `instance`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub started_at: Option<String>,
    /// The bound repository (main-checkout path), from the active subscription.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repo: Option<String>,
    /// The integration branch: synced from the merge branch once the fleet is integrated.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    /// The branch member merges land on: `claude-orch/<fleet>-<instance>`; absent while unbound.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub merge_branch: Option<String>,
    /// The target holds the merge branch's work; absent without a merge branch.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub synced: Option<bool>,
    /// Integration state of the fleet's member work.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target_state: Option<TargetStateWire>,
    /// Member count (see `members`).
    pub members: u32,
    /// Sessions joined to this fleet from another repository — never members.
    #[serde(default)]
    pub foreign: Vec<String>,
    /// Subscriptions of the live run, any state, plus refused ones that bound no run, ordered by session.
    pub subscriptions: Vec<SubscriptionRow>,
    /// The live run's journal, oldest first.
    #[serde(default)]
    pub journal: Vec<JournalRow>,
}

/// One dissolved fleet run, the content of `heartbeat/archive/<instance>.json` (schema `archive-v1.json`).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct FleetArchive {
    /// Always 1.
    pub v: u32,
    /// The run's UUID.
    pub instance: String,
    /// The fleet name.
    pub fleet: String,
    /// Run start (RFC 3339).
    pub started_at: String,
    /// Dissolve time (RFC 3339).
    pub dissolved_at: String,
    /// The bound repository, last seen.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repo: Option<String>,
    /// The target, last seen.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<String>,
    /// The merge branch, last seen.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub merge_branch: Option<String>,
    /// Whether the target held the merge branch, last seen.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub synced: Option<bool>,
    /// Everyone who was a member, in roster order.
    pub members: Vec<ArchivedMember>,
    /// The run's subscriptions, final state.
    pub subscriptions: Vec<SubscriptionRow>,
    /// The whole journal, oldest first.
    pub journal: Vec<JournalRow>,
}

/// One member of a dissolved run.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct ArchivedMember {
    /// `T1-MA`, or `T1-O` for the operator.
    pub token: String,
    /// The session.
    pub session_id: String,
    /// `joined`, `subscriber` or `worker`.
    pub via: ViaWire,
    /// Its branch, last seen.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    /// Delivered, last seen.
    pub delivered: bool,
    /// First seen as a member (RFC 3339).
    pub joined_at: String,
    /// Left or dissolved (RFC 3339).
    pub left_at: String,
}

impl From<(&FleetInstance, Vec<SubscriptionRow>, Timestamp)> for FleetArchive {
    fn from(
        (instance, subscriptions, at): (&FleetInstance, Vec<SubscriptionRow>, Timestamp),
    ) -> FleetArchive {
        let fleet = &instance.name;
        let members = instance
            .roster
            .iter()
            .map(|((session, _), entry)| {
                // Joined members get their letter at join and workers at registration, so the roster always holds one.
                let token = match entry.via {
                    Via::Subscriber => format!("{}-O", fleet.0),
                    Via::Joined | Via::Worker => entry
                        .member
                        .as_ref()
                        .map(|letter| letter.token(fleet))
                        .unwrap_or_default(),
                };
                ArchivedMember {
                    token,
                    session_id: session.0.clone(),
                    via: ViaWire::from(entry.via),
                    branch: entry.branch.clone(),
                    delivered: entry.delivered,
                    joined_at: entry.joined_at.to_string(),
                    left_at: entry.left_at.unwrap_or(at).to_string(),
                }
            })
            .collect();
        FleetArchive {
            v: ARCHIVE_VERSION,
            instance: instance.id.to_string(),
            fleet: fleet.0.clone(),
            started_at: instance.since.to_string(),
            dissolved_at: at.to_string(),
            repo: instance
                .last
                .repo
                .as_ref()
                .map(|repo| repo.0.to_string_lossy().into_owned()),
            target: instance.last.target.clone(),
            merge_branch: instance.last.merge_branch.clone(),
            synced: instance.last.synced,
            members,
            subscriptions,
            journal: instance.journal.iter().map(JournalRow::from).collect(),
        }
    }
}

/// One journal entry an orchestrator recorded.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct JournalRow {
    /// `assigned`, `decided`, `merged`, `spawned` or `note`.
    pub kind: String,
    /// Member letters it concerns.
    pub members: Vec<String>,
    /// What.
    pub subject: String,
    /// Why, or the evidence.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
    /// The recording session.
    pub by: String,
    /// When (RFC 3339).
    pub at: String,
}

/// Fleet target states on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum TargetStateWire {
    /// Member work is still open, or the repo has no good scan yet that requested the target.
    Open,
    /// At least one worker exists (the subscriber excluded), every worker is delivered, at least one worker branch was
    /// seen in a scan, and every worker branch that still has a row (the target branch excluded) is merged or removable;
    /// a worker branch without a row counts as done.
    Integrated,
    /// The repo's last good scan requested the target as a train base and has no train for it: not a local branch.
    Missing,
}

impl TargetStateWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            TargetStateWire::Open => "open",
            TargetStateWire::Integrated => "integrated",
            TargetStateWire::Missing => "missing",
        }
    }
}

/// One subscription.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct SubscriptionRow {
    /// The subscriber session.
    pub session_id: String,
    /// The fleet.
    pub fleet: String,
    /// The bound repository; absent only on a refusal that resolved none.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub repo: Option<String>,
    /// The fleet's integration branch.
    pub target: String,
    /// Lifecycle.
    pub state: SubscriptionStateWire,
    /// Why, when refused or ended.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
    /// When the state was entered (RFC 3339).
    pub since: String,
    /// The subscriber's messaging socket.
    pub socket_path: String,
    /// Items waiting for the next digest.
    pub pending: u32,
    /// The in-flight digest, else the last one.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_digest: Option<DigestRow>,
    /// The run hears collisions with branches outside the fleet too (`subscribe --outside-collisions`); archives
    /// written before the field read `false`.
    #[serde(default)]
    pub outside_collisions: bool,
}

/// Subscription states on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SubscriptionStateWire {
    /// Receives digests.
    Active,
    /// Never admitted.
    Refused,
    /// Ended.
    Ended,
}

impl SubscriptionStateWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            SubscriptionStateWire::Active => "active",
            SubscriptionStateWire::Refused => "refused",
            SubscriptionStateWire::Ended => "ended",
        }
    }
}

/// One digest's fate.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct DigestRow {
    /// `d<n>`.
    pub id: String,
    /// What happened.
    pub outcome: DigestOutcomeWire,
    /// Why, for refused, unconfirmed and unclaimed.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
    /// When the outcome was reported (RFC 3339); absent while in flight.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub at: Option<String>,
}

/// Digest outcomes on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum DigestOutcomeWire {
    /// Handed to the socket edge, not reported yet.
    InFlight,
    /// Written in full.
    Sent,
    /// Nothing sent.
    Refused,
    /// Claimed; the write failed or its outcome was lost.
    Unconfirmed,
    /// The claim failed; nothing sent.
    Unclaimed,
}

impl DigestOutcomeWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            DigestOutcomeWire::InFlight => "in_flight",
            DigestOutcomeWire::Sent => "sent",
            DigestOutcomeWire::Refused => "refused",
            DigestOutcomeWire::Unconfirmed => "unconfirmed",
            DigestOutcomeWire::Unclaimed => "unclaimed",
        }
    }
}

/// One fleet member.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct MemberRow {
    /// The fleet.
    pub fleet: String,
    /// The member session; for a worker, the subscriber that registered it.
    pub session_id: String,
    /// Why it is a member.
    pub via: ViaWire,
    /// The member's branch in the fleet's repo (`member_branch`), when known.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub branch: Option<String>,
    /// The member's letter; absent until assigned.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub member: Option<String>,
    /// The member's title token, `<fleet>-M<letter>`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    /// Joined and reported done, or a worker its subscriber marked delivered.
    pub delivered: bool,
}

/// Membership reasons on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ViaWire {
    /// Joined with `heartbeat join`.
    Joined,
    /// Active subscriber.
    Subscriber,
    /// Branch the subscriber registered with `heartbeat worker`.
    Worker,
}

impl ViaWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            ViaWire::Joined => "joined",
            ViaWire::Subscriber => "subscriber",
            ViaWire::Worker => "worker",
        }
    }
}

/// `POST /subscribe` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SubscribeBody {
    /// The fleet to orchestrate.
    pub fleet: String,
    /// The calling session (`CLAUDE_CODE_SESSION_ID`).
    pub session_id: String,
    /// Its messaging socket (`CLAUDE_CODE_MESSAGING_SOCKET`).
    pub socket_path: PathBuf,
    /// Its messaging token (`CLAUDE_CODE_MESSAGING_TOKEN`).
    pub token: MessagingToken,
    /// The fleet's integration branch (`--target`); the daemon resolves the repository from the session's cwd.
    pub target: String,
    /// Hear collisions with branches outside the fleet too (`--outside-collisions`).
    #[serde(default)]
    pub outside_collisions: bool,
}

/// `POST /unsubscribe` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct UnsubscribeBody {
    /// The calling session (`CLAUDE_CODE_SESSION_ID`).
    pub session_id: String,
    /// Its messaging token (`CLAUDE_CODE_MESSAGING_TOKEN`), as handed over at subscribe.
    pub token: MessagingToken,
    /// Release every joined member as well (`unsubscribe --finish`).
    #[serde(default)]
    pub finish: bool,
}

/// `POST /join` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JoinBody {
    /// The fleet to join.
    pub fleet: String,
    /// The calling session (`CLAUDE_CODE_SESSION_ID`).
    pub session_id: String,
    /// Its messaging token (`CLAUDE_CODE_MESSAGING_TOKEN`).
    pub token: MessagingToken,
}

/// `POST /done` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DoneBody {
    /// The calling session.
    pub session_id: String,
    /// Its messaging token, as handed over at join.
    pub token: MessagingToken,
}

/// `POST /land` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LandBody {
    /// The calling session (`CLAUDE_CODE_SESSION_ID`).
    pub session_id: String,
    /// Its messaging token (`CLAUDE_CODE_MESSAGING_TOKEN`).
    pub token: MessagingToken,
    /// The branch whose turn it asks for (`--target`); the daemon resolves the repository from the session's cwd.
    pub target: String,
}

/// `POST /land/end` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LandEndBody {
    /// The calling session.
    pub session_id: String,
    /// Its messaging token, as handed over at its landing request.
    pub token: MessagingToken,
    /// Landed or abandoned (`--end`).
    pub outcome: LandEndWire,
}

/// `POST /land/release` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LandReleaseBody {
    /// The caller's canonical working directory; the daemon resolves the repository from it.
    pub cwd: PathBuf,
    /// The branch whose turn is released (`--release`).
    pub target: String,
}

/// The answer of `POST /land`, `/land/end` and `/land/release`, and the accepted half of a `/done` answer.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LandAnswerRow {
    /// What the call left the caller with.
    pub state: LandStateWire,
    /// The landing target.
    pub branch: String,
    /// The place in the queue, from 1; only when `queued`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub place: Option<u32>,
    /// How the entry ended; only when `ended`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub outcome: Option<LandEndWire>,
    /// The holder that lost the turn; only when `released`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_id: Option<String>,
}

impl LandAnswerRow {
    /// The row of `reply` for the landing target `target`.
    #[must_use]
    pub fn new(target: String, reply: LandReply) -> LandAnswerRow {
        let row = |state| LandAnswerRow {
            state,
            branch: target,
            place: None,
            outcome: None,
            session_id: None,
        };
        match reply {
            LandReply::Granted => row(LandStateWire::Granted),
            LandReply::Queued { place } => LandAnswerRow {
                place: Some(place),
                ..row(LandStateWire::Queued)
            },
            LandReply::Ended { outcome } => LandAnswerRow {
                outcome: Some(outcome.into()),
                ..row(LandStateWire::Ended)
            },
            LandReply::Released { session } => LandAnswerRow {
                session_id: Some(session.0),
                ..row(LandStateWire::Released)
            },
        }
    }
}

/// Land answer states on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LandStateWire {
    /// The caller holds the turn.
    Granted,
    /// The caller waits.
    Queued,
    /// The caller's entry ended.
    Ended,
    /// The operator released the holder's turn.
    Released,
}

impl LandStateWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            LandStateWire::Granted => "granted",
            LandStateWire::Queued => "queued",
            LandStateWire::Ended => "ended",
            LandStateWire::Released => "released",
        }
    }
}

/// The ends a session types, on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LandEndWire {
    /// The holder landed its branch.
    Landed,
    /// The holder or a waiter gave its entry up.
    Abandoned,
}

impl LandEndWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            LandEndWire::Landed => "landed",
            LandEndWire::Abandoned => "abandoned",
        }
    }
}

impl From<TypedEnd> for LandEndWire {
    fn from(end: TypedEnd) -> LandEndWire {
        match end {
            TypedEnd::Landed => LandEndWire::Landed,
            TypedEnd::Abandoned => LandEndWire::Abandoned,
        }
    }
}

impl From<LandEndWire> for TypedEnd {
    fn from(end: LandEndWire) -> TypedEnd {
        match end {
            LandEndWire::Landed => TypedEnd::Landed,
            LandEndWire::Abandoned => TypedEnd::Abandoned,
        }
    }
}

/// The answer of `POST /done`: a joined member's delivery, or the acceptance of a session without a join.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "outcome", rename_all = "snake_case")]
pub enum DoneAnswer {
    /// A joined member reported done.
    Delivered {
        /// Its member row, reading delivered.
        member: MemberRow,
    },
    /// A session without a join was accepted for the landing target of its branch.
    Accepted {
        /// The landing turn it got: granted, or queued with its place.
        landing: LandAnswerRow,
    },
    /// A session without a join whose branch has no landing target: done, nothing lands.
    NoTarget {
        /// The branch's base; absent when agentwt resolved none.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        base: Option<String>,
    },
}

/// `POST /leave` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LeaveBody {
    /// The calling session.
    pub session_id: String,
    /// Its messaging token, as handed over at join.
    pub token: MessagingToken,
}

/// `POST /remove` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RemoveBody {
    /// The fleet.
    pub fleet: String,
    /// The member letter (`A` for `T1-MA`).
    pub member: String,
}

/// `POST /journal` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JournalBody {
    /// The recording session (the fleet's subscriber).
    pub session_id: String,
    /// Its messaging token.
    pub token: MessagingToken,
    /// `assigned`, `decided`, `merged`, `spawned` or `note`.
    pub kind: String,
    /// Member letters.
    #[serde(default)]
    pub members: Vec<String>,
    /// One line.
    pub subject: String,
    /// Why.
    #[serde(default)]
    pub detail: Option<String>,
}

/// `POST /journal` answer: the stored entry and the fleet it went to — not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RecordedRow {
    /// The subscriber's fleet.
    pub fleet: String,
    /// The stored entry.
    #[serde(flatten)]
    pub entry: JournalRow,
}

/// `POST /worker` body — a request, not part of the status document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WorkerBody {
    /// The calling session (the fleet's subscriber).
    pub session_id: String,
    /// Its messaging token.
    pub token: MessagingToken,
    /// The worker's branch (`claude/<name>`).
    pub branch: String,
    /// What the subscriber reports.
    pub state: WorkerStateWire,
}

/// A worker's state as its subscriber reports it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WorkerStateWire {
    /// Registered and working; also reopens a delivered worker.
    Running,
    /// Its work is finished.
    Delivered,
}

/// The runtime document (`GET /runtime`, `heartbeat runtime status --format json`, schema
/// `schema/runtime-v1.json`): its own document beside the status document (D15). Unlike the status document, every
/// field is always present: `subscriptions` is empty without a watcher, `last_digest` is `null` before the first
/// digest and `at` is `null` while that digest is in flight; unknown strings (`title`, `version`, `reason`) are empty.
/// No token field anywhere.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct RuntimeDocument {
    /// Always [`RUNTIME_SCHEMA`].
    pub schema: u32,
    /// The daemon that answered.
    pub heartbeat: Health,
    /// Active watchers, each bound to one observed session, in subscriber order.
    pub subscriptions: Vec<RuntimeSubscriptionRow>,
    /// Runs since the daemon started (ended ones capped), in id order.
    pub runs: Vec<RunRow>,
    /// Findings of those runs, in id (time) order.
    pub findings: Vec<FindingRow>,
}

/// One runtime watcher: a subscriber session bound to exactly one observed session.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct RuntimeSubscriptionRow {
    /// The in-flight digest, else the last one; `null` before the first.
    pub last_digest: Option<RuntimeDigestRow>,
    /// Report or act.
    pub mode: ModeWire,
    /// The subscriber session.
    pub session: String,
    /// When it subscribed (RFC 3339).
    pub since: String,
    /// The one session it watches.
    pub target: String,
    /// The watched session's title; empty when unknown.
    pub target_title: String,
    /// The subscriber's session title; empty when unknown.
    pub title: String,
}

/// One runtime digest's fate.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct RuntimeDigestRow {
    /// `d<n>`.
    pub id: String,
    /// What happened.
    pub outcome: DigestOutcomeWire,
    /// When the outcome was reported (RFC 3339); `null` while in flight.
    pub at: Option<String>,
}

/// One enforced skill run of an observed session.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct RunRow {
    /// The run id.
    pub id: u64,
    /// Successful anchored calls, in apply order.
    pub anchors: Vec<AnchorHitRow>,
    /// Failed checks with their failure counts.
    pub failures: Vec<CheckFailureRow>,
    /// Findings raised.
    pub findings: u32,
    /// Checks judged with their guard matched that held, with their pass counts, in check id order; empty when none.
    pub passes: Vec<CheckPassRow>,
    /// The failed checks with their counts (open, killed), or why the run ended.
    pub reason: String,
    /// The observed session.
    pub session: String,
    /// The skill leaf.
    pub skill: String,
    /// Open, ended or killed.
    pub state: RunStateWire,
    /// The session title; empty when unknown.
    pub title: String,
    /// The deployed skill version the run started under.
    pub version: String,
}

/// One successful anchored call of a run.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, JsonSchema)]
pub struct AnchorHitRow {
    /// The anchor id.
    pub id: String,
    /// The apply sequence number of the call.
    pub seq: u64,
    /// The captures the call yielded, by name; a declared capture its response lacked is absent.
    pub captures: BTreeMap<String, Value>,
}

/// One failed check of a run.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct CheckFailureRow {
    /// The check id.
    pub check: String,
    /// How often it failed (8 kills the run).
    pub count: u8,
}

/// One check of a run that held at least once.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct CheckPassRow {
    /// The check id.
    pub check: String,
    /// How often it was judged with its guard matched and held.
    pub count: u32,
}

/// One finding, its moves and what became of it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
pub struct FindingRow {
    /// The finding id (`<run>-<check or judgment>-<n>`).
    pub id: String,
    /// What became of its unlock.
    pub acted: ActedWire,
    /// The check or judgment id.
    pub entry: String,
    /// What was compared or observed.
    pub evidence: Vec<String>,
    /// The finding kind.
    pub kind: FindingKindWire,
    /// The moves it unlocks right now (empty once answered or once its run ended).
    pub moves: Vec<MoveWire>,
    /// The check's reason or the judgment's question.
    pub reason: String,
    /// The run id.
    pub run: u64,
    /// The observed session.
    pub session: String,
    /// The session title; empty when unknown.
    pub title: String,
    /// The skill leaf.
    pub skill: String,
    /// The skill model version.
    pub version: String,
}

/// Runtime subscription modes on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ModeWire {
    /// Findings are reported only.
    Report,
    /// Findings may be answered with a move.
    Act,
}

/// Moves on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum MoveWire {
    /// Heartbeat's template, handed over as hook context.
    Send,
    /// The agent is stopped.
    Stop,
}

/// Run states on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum RunStateWire {
    /// Open and enforced.
    Open,
    /// Ended; `reason` says why.
    Ended,
    /// A check failed 8 times: the session's agents are stopped until the next prompt, which reopens the run.
    Killed,
}

/// Finding kinds on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum FindingKindWire {
    /// A check failed without killing its run.
    CheckFailed,
    /// A judgment point's anchor succeeded.
    Judgment,
}

/// What became of a finding's unlock, on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ActedWire {
    /// Not answered.
    Open,
    /// A move was accepted; its claim is not on disk yet.
    Claiming,
    /// A send is claimed and waits for its hand-over: the agent's next successful tool call, or a waiting root's Stop or next prompt.
    SendPending,
    /// A stop is claimed and armed.
    StopArmed,
    /// The send was handed over.
    HandedOver,
    /// The armed stop was cleared by the next prompt or the subagent's stop.
    StopCleared,
    /// The move never took effect.
    Void,
}

impl ModeWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            ModeWire::Report => "report",
            ModeWire::Act => "act",
        }
    }
}

impl MoveWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            MoveWire::Send => "send",
            MoveWire::Stop => "stop",
        }
    }
}

impl RunStateWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            RunStateWire::Open => "open",
            RunStateWire::Ended => "ended",
            RunStateWire::Killed => "killed",
        }
    }
}

impl FindingKindWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            FindingKindWire::CheckFailed => "check_failed",
            FindingKindWire::Judgment => "judgment",
        }
    }
}

impl ActedWire {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            ActedWire::Open => "open",
            ActedWire::Claiming => "claiming",
            ActedWire::SendPending => "send_pending",
            ActedWire::StopArmed => "stop_armed",
            ActedWire::HandedOver => "handed_over",
            ActedWire::StopCleared => "stop_cleared",
            ActedWire::Void => "void",
        }
    }
}

impl From<Mode> for ModeWire {
    fn from(mode: Mode) -> ModeWire {
        match mode {
            Mode::Report => ModeWire::Report,
            Mode::Act => ModeWire::Act,
        }
    }
}

impl From<ModeWire> for Mode {
    fn from(mode: ModeWire) -> Mode {
        match mode {
            ModeWire::Report => Mode::Report,
            ModeWire::Act => Mode::Act,
        }
    }
}

impl From<Move> for MoveWire {
    fn from(action: Move) -> MoveWire {
        match action {
            Move::Send => MoveWire::Send,
            Move::Stop => MoveWire::Stop,
        }
    }
}

impl From<MoveWire> for Move {
    fn from(action: MoveWire) -> Move {
        match action {
            MoveWire::Send => Move::Send,
            MoveWire::Stop => Move::Stop,
        }
    }
}

impl From<FindingKind> for FindingKindWire {
    fn from(kind: FindingKind) -> FindingKindWire {
        match kind {
            FindingKind::CheckFailed => FindingKindWire::CheckFailed,
            FindingKind::Judgment => FindingKindWire::Judgment,
        }
    }
}

impl From<&Acted> for ActedWire {
    fn from(acted: &Acted) -> ActedWire {
        match acted {
            Acted::Open => ActedWire::Open,
            Acted::InFlight(_) => ActedWire::Claiming,
            Acted::Live(Move::Send) => ActedWire::SendPending,
            Acted::Live(Move::Stop) => ActedWire::StopArmed,
            Acted::Done(Move::Send) => ActedWire::HandedOver,
            Acted::Done(Move::Stop) => ActedWire::StopCleared,
            Acted::Void(_) => ActedWire::Void,
        }
    }
}

impl From<&Outcome> for DigestOutcomeWire {
    fn from(outcome: &Outcome) -> DigestOutcomeWire {
        match outcome {
            Outcome::Sent => DigestOutcomeWire::Sent,
            Outcome::Refused(_) => DigestOutcomeWire::Refused,
            Outcome::Unconfirmed(_) => DigestOutcomeWire::Unconfirmed,
            Outcome::Unclaimed(_) => DigestOutcomeWire::Unclaimed,
        }
    }
}

/// `POST /runtime/subscribe` body — a request, not part of the runtime document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RuntimeSubscribeBody {
    /// The calling session (`CLAUDE_CODE_SESSION_ID`).
    pub session_id: String,
    /// Its messaging socket (`CLAUDE_CODE_MESSAGING_SOCKET`): where runtime digests go.
    pub socket_path: PathBuf,
    /// Its messaging token (`CLAUDE_CODE_MESSAGING_TOKEN`).
    pub token: MessagingToken,
    /// Report or act (`--mode`).
    pub mode: ModeWire,
    /// The one session to watch, by id or exact title (`--target`).
    pub target: String,
}

/// `POST /runtime/unsubscribe` body — a request, not part of the runtime document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RuntimeUnsubscribeBody {
    /// The calling session.
    pub session_id: String,
    /// Its messaging token, as handed over at subscribe.
    pub token: MessagingToken,
}

/// `POST /runtime/act` body — a request, not part of the runtime document or its schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RuntimeActBody {
    /// The calling session (the runtime subscriber).
    pub session_id: String,
    /// Its messaging token, as handed over at subscribe.
    pub token: MessagingToken,
    /// The finding answered.
    pub finding: String,
    /// The move asked for.
    #[serde(rename = "move")]
    pub action: MoveWire,
}

impl From<(&SessionId, &Subscription)> for SubscriptionRow {
    fn from((session, subscription): (&SessionId, &Subscription)) -> SubscriptionRow {
        let (state, detail) = match &subscription.state {
            SubscriptionState::Active => (SubscriptionStateWire::Active, None),
            SubscriptionState::Refused(reason) => {
                (SubscriptionStateWire::Refused, Some(reason.clone()))
            }
            SubscriptionState::Ended(reason) => {
                (SubscriptionStateWire::Ended, Some(reason.clone()))
            }
        };
        let last_digest = match subscription.in_flight {
            Some(id) => Some(DigestRow {
                id: id.label(),
                outcome: DigestOutcomeWire::InFlight,
                detail: None,
                at: None,
            }),
            None => subscription.last.as_ref().map(DigestRow::from),
        };
        SubscriptionRow {
            session_id: session.0.clone(),
            fleet: subscription.fleet.0.clone(),
            repo: subscription
                .repo
                .as_ref()
                .map(|repo| repo.0.to_string_lossy().into_owned()),
            target: subscription.target.clone(),
            state,
            detail,
            since: subscription.since.to_string(),
            socket_path: subscription.socket.to_string_lossy().into_owned(),
            pending: u32::try_from(subscription.pending.len()).unwrap_or(u32::MAX),
            last_digest,
            outside_collisions: subscription.outside_collisions,
        }
    }
}

impl From<&LastDigest> for DigestRow {
    fn from(last: &LastDigest) -> DigestRow {
        let (outcome, detail) = match &last.outcome {
            Outcome::Sent => (DigestOutcomeWire::Sent, None),
            Outcome::Refused(detail) => (DigestOutcomeWire::Refused, Some(detail.clone())),
            Outcome::Unconfirmed(detail) => (DigestOutcomeWire::Unconfirmed, Some(detail.clone())),
            Outcome::Unclaimed(detail) => (DigestOutcomeWire::Unclaimed, Some(detail.clone())),
        };
        DigestRow {
            id: last.id.label(),
            outcome,
            detail,
            at: Some(last.at.to_string()),
        }
    }
}

/// What a member row carries beyond the member itself: its branch, letter and delivery.
pub type MemberFacts<'a> = (Option<String>, Option<&'a MemberId>, bool);

impl From<(&FleetId, &Member, MemberFacts<'_>)> for MemberRow {
    fn from((fleet, member, facts): (&FleetId, &Member, MemberFacts<'_>)) -> MemberRow {
        let (branch, letter, delivered) = facts;
        let token = match member.via {
            Via::Subscriber => Some(format!("{}-O", fleet.0)),
            Via::Joined | Via::Worker => letter.map(|letter| letter.token(fleet)),
        };
        MemberRow {
            fleet: fleet.0.clone(),
            session_id: member.session.0.clone(),
            via: ViaWire::from(member.via),
            branch,
            member: letter.map(|letter| letter.0.clone()),
            token,
            delivered,
        }
    }
}

impl From<&JournalEntry> for JournalRow {
    fn from(entry: &JournalEntry) -> JournalRow {
        JournalRow {
            kind: entry.kind.as_str().to_owned(),
            members: entry
                .members
                .iter()
                .map(|member| member.0.clone())
                .collect(),
            subject: entry.subject.clone(),
            detail: entry.detail.clone(),
            by: entry.by.0.clone(),
            at: entry.at.to_string(),
        }
    }
}

impl From<TargetState> for TargetStateWire {
    fn from(state: TargetState) -> TargetStateWire {
        match state {
            TargetState::Open => TargetStateWire::Open,
            TargetState::Integrated => TargetStateWire::Integrated,
            TargetState::Missing => TargetStateWire::Missing,
        }
    }
}

impl From<Via> for ViaWire {
    fn from(via: Via) -> ViaWire {
        match via {
            Via::Joined => ViaWire::Joined,
            Via::Subscriber => ViaWire::Subscriber,
            Via::Worker => ViaWire::Worker,
        }
    }
}

/// The document's JSON Schema under the serialize contract, pretty-printed.
pub fn schema() -> crate::Result<String> {
    let schema = schemars::generate::SchemaSettings::default()
        .for_serialize()
        .into_generator()
        .into_root_schema_for::<StatusDocument>();
    Ok(serde_json::to_string_pretty(&schema)?)
}

/// The runtime document's JSON Schema under the serialize contract, pretty-printed.
pub fn runtime_schema() -> crate::Result<String> {
    let schema = schemars::generate::SchemaSettings::default()
        .for_serialize()
        .into_generator()
        .into_root_schema_for::<RuntimeDocument>();
    Ok(serde_json::to_string_pretty(&schema)?)
}

/// The archive file's JSON Schema under the serialize contract, pretty-printed.
pub fn archive_schema() -> crate::Result<String> {
    let schema = schemars::generate::SchemaSettings::default()
        .for_serialize()
        .into_generator()
        .into_root_schema_for::<FleetArchive>();
    Ok(serde_json::to_string_pretty(&schema)?)
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;
    use crate::ids::WorkerBranch;
    use crate::protocol::RUNTIME_SCHEMA;

    #[test]
    #[allow(clippy::too_many_lines)]
    fn enum_strings_are_pinned() {
        let states = [
            SessionStateWire::Running,
            SessionStateWire::Waiting,
            SessionStateWire::Blocked,
            SessionStateWire::Idle,
            SessionStateWire::Ended,
            SessionStateWire::Died,
        ];
        for state in states {
            assert_eq!(
                serde_json::to_value(state).expect("serialize"),
                state.as_str()
            );
        }
        let sensors = [
            SensorWire::Hooks,
            SensorWire::Registry,
            SensorWire::ProcessExit,
            SensorWire::Ledger,
            SensorWire::Git,
            SensorWire::Agentwt,
            SensorWire::Peek,
            SensorWire::ActionLedger,
            SensorWire::Config,
        ];
        for sensor in sensors {
            assert_eq!(
                serde_json::to_value(sensor).expect("serialize"),
                sensor.as_str()
            );
        }
        for state in [
            TargetStateWire::Open,
            TargetStateWire::Integrated,
            TargetStateWire::Missing,
        ] {
            assert_eq!(
                serde_json::to_value(state).expect("serialize"),
                state.as_str()
            );
        }
        for state in [
            SubscriptionStateWire::Active,
            SubscriptionStateWire::Refused,
            SubscriptionStateWire::Ended,
        ] {
            assert_eq!(
                serde_json::to_value(state).expect("serialize"),
                state.as_str()
            );
        }
        for via in [ViaWire::Joined, ViaWire::Subscriber, ViaWire::Worker] {
            assert_eq!(serde_json::to_value(via).expect("serialize"), via.as_str());
        }
        assert_eq!(
            serde_json::to_value(SensorStateWire::Degraded).expect("serialize"),
            "degraded"
        );
        assert_eq!(
            serde_json::to_value(SubagentStateWire::Stopped).expect("serialize"),
            "stopped"
        );
        let git = [
            (
                "no-work",
                serde_json::to_value(BranchStateWire::NoWork),
                "no_work",
            ),
            (
                "uncommitted",
                serde_json::to_value(BranchStateWire::Uncommitted),
                "uncommitted",
            ),
            (
                "unmerged",
                serde_json::to_value(BranchStateWire::Unmerged),
                "unmerged",
            ),
            (
                "conflicting",
                serde_json::to_value(BranchStateWire::Conflicting),
                "conflicting",
            ),
            (
                "merge-ready",
                serde_json::to_value(BranchStateWire::MergeReady),
                "merge_ready",
            ),
            (
                "merged",
                serde_json::to_value(BranchStateWire::Merged),
                "merged",
            ),
            (
                "removable",
                serde_json::to_value(BranchStateWire::Removable),
                "removable",
            ),
            (
                "planned-file",
                serde_json::to_value(CollisionLevelWire::PlannedFile),
                "planned_file",
            ),
            (
                "planned-symbol",
                serde_json::to_value(CollisionLevelWire::PlannedSymbol),
                "planned_symbol",
            ),
            (
                "touched",
                serde_json::to_value(CollisionLevelWire::Touched),
                "touched",
            ),
            (
                "confirmed",
                serde_json::to_value(CollisionLevelWire::Confirmed),
                "confirmed",
            ),
            (
                "clean",
                serde_json::to_value(TrainStateWire::Clean),
                "clean",
            ),
            (
                "blocked",
                serde_json::to_value(TrainStateWire::Blocked),
                "blocked",
            ),
            (
                "nothing",
                serde_json::to_value(TrainStateWire::Nothing),
                "nothing",
            ),
        ];
        for (label, wire, want) in git {
            assert_eq!(wire.expect("serialize"), want, "{label}");
        }
        let fleets = [
            (
                "active",
                serde_json::to_value(SubscriptionStateWire::Active),
                "active",
            ),
            (
                "refused",
                serde_json::to_value(SubscriptionStateWire::Refused),
                "refused",
            ),
            (
                "ended",
                serde_json::to_value(SubscriptionStateWire::Ended),
                "ended",
            ),
            (
                "in-flight",
                serde_json::to_value(DigestOutcomeWire::InFlight),
                "in_flight",
            ),
            (
                "sent",
                serde_json::to_value(DigestOutcomeWire::Sent),
                "sent",
            ),
            (
                "outcome-refused",
                serde_json::to_value(DigestOutcomeWire::Refused),
                "refused",
            ),
            (
                "unconfirmed",
                serde_json::to_value(DigestOutcomeWire::Unconfirmed),
                "unconfirmed",
            ),
            (
                "unclaimed",
                serde_json::to_value(DigestOutcomeWire::Unclaimed),
                "unclaimed",
            ),
            ("joined", serde_json::to_value(ViaWire::Joined), "joined"),
            (
                "subscriber",
                serde_json::to_value(ViaWire::Subscriber),
                "subscriber",
            ),
            (
                "via-worker-wire-string",
                serde_json::to_value(ViaWire::Worker),
                "worker",
            ),
            (
                "worker-state-wire-strings-running",
                serde_json::to_value(WorkerStateWire::Running),
                "running",
            ),
            (
                "worker-state-wire-strings-delivered",
                serde_json::to_value(WorkerStateWire::Delivered),
                "delivered",
            ),
            ("open", serde_json::to_value(TargetStateWire::Open), "open"),
            (
                "integrated",
                serde_json::to_value(TargetStateWire::Integrated),
                "integrated",
            ),
            (
                "missing",
                serde_json::to_value(TargetStateWire::Missing),
                "missing",
            ),
        ];
        for (label, wire, want) in fleets {
            assert_eq!(wire.expect("serialize"), want, "{label}");
        }
        for state in [
            LandStateWire::Granted,
            LandStateWire::Queued,
            LandStateWire::Ended,
            LandStateWire::Released,
        ] {
            assert_eq!(
                serde_json::to_value(state).expect("serialize"),
                state.as_str()
            );
        }
        let land_states = [
            (LandStateWire::Granted, "granted"),
            (LandStateWire::Queued, "queued"),
            (LandStateWire::Ended, "ended"),
            (LandStateWire::Released, "released"),
        ];
        for (state, want) in land_states {
            assert_eq!(state.as_str(), want);
        }
        for (end, want) in [
            (LandEndWire::Landed, "landed"),
            (LandEndWire::Abandoned, "abandoned"),
        ] {
            assert_eq!(serde_json::to_value(end).expect("serialize"), want);
            assert_eq!(end.as_str(), want);
        }
    }

    const TOKEN: &str = "feedc0defeedc0defeedc0defeedc0de";

    fn subscription(state: SubscriptionState) -> Subscription {
        Subscription {
            fleet: FleetId("T1".to_owned()),
            instance: None,
            repo: None,
            target: "main".to_owned(),
            socket: PathBuf::from("/tmp/o.sock"),
            token: MessagingToken(TOKEN.to_owned()),
            state,
            since: jiff::Timestamp::UNIX_EPOCH,
            pending: std::collections::BTreeMap::new(),
            in_flight: None,
            last: None,
            announced: std::collections::BTreeSet::new(),
            worker_branches_seen: std::collections::BTreeSet::new(),
            outside_collisions: false,
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn absent_not_null() {
        let session = SessionId("o".to_owned());
        let row = SubscriptionRow::from((&session, &subscription(SubscriptionState::Active)));
        let json = serde_json::to_value(&row).expect("serialize");
        for key in ["last_digest", "repo", "detail"] {
            assert!(json.get(key).is_none(), "subscription {key}: {json}");
        }
        let member = Member {
            session,
            worker: None,
            via: Via::Joined,
        };
        let facts = (None, None, false);
        let json =
            serde_json::to_value(MemberRow::from((&FleetId("T1".to_owned()), &member, facts)))
                .expect("serialize");
        for key in ["branch", "member", "token"] {
            assert!(json.get(key).is_none(), "member {key}: {json}");
        }
        let unbound = FleetRow {
            id: "T1".to_owned(),
            instance: None,
            started_at: None,
            repo: None,
            target: None,
            merge_branch: None,
            synced: None,
            target_state: None,
            members: 0,
            foreign: Vec::new(),
            subscriptions: Vec::new(),
            journal: Vec::new(),
        };
        let json = serde_json::to_value(&unbound).expect("serialize");
        for key in [
            "instance",
            "started_at",
            "repo",
            "target",
            "merge_branch",
            "synced",
            "target_state",
        ] {
            assert!(json.get(key).is_none(), "fleet {key}: {json}");
        }
        let free = LandingRow {
            repo: "/r".to_owned(),
            branch: "feature/x".to_owned(),
            release: None,
            holder: None,
            queue: vec![LandingTurnRow {
                session_id: "s".to_owned(),
                title: None,
                since: "2026-10-02T00:00:00Z".to_owned(),
            }],
        };
        let json = serde_json::to_value(&free).expect("serialize");
        assert!(json.get("holder").is_none(), "landing holder: {json}");
        assert!(
            json.get("release").is_none(),
            "non-staging landing release: {json}"
        );
        assert!(
            json["queue"][0].get("title").is_none(),
            "landing turn title: {json}"
        );
        let staging = LandingRow {
            branch: "develop".to_owned(),
            release: Some(ReleaseRow {
                exists: true,
                releasable: false,
                unreleased: 3,
            }),
            ..free
        };
        let json = serde_json::to_value(&staging).expect("serialize");
        assert_eq!(
            json["release"],
            serde_json::json!({"exists": true, "releasable": false, "unreleased": 3}),
            "staging landing release: {json}"
        );
        let json = serde_json::to_value(LandAnswerRow::new(
            "feature/x".to_owned(),
            LandReply::Granted,
        ))
        .expect("serialize");
        for key in ["place", "outcome", "session_id"] {
            assert!(json.get(key).is_none(), "land answer {key}: {json}");
        }
        assert_eq!(json["branch"], "feature/x", "land answer branch: {json}");
        let branch = BranchRow {
            repo: "/r".to_owned(),
            branch: Some("claude/a".to_owned()),
            worktree: None,
            state: BranchStateWire::NoWork,
            since: "2026-10-02T00:00:00Z".to_owned(),
            target: None,
            train_position: None,
            sessions: Vec::new(),
            work_items: Vec::new(),
            agents: Vec::new(),
        };
        let json = serde_json::to_value(&branch).expect("serialize");
        for key in ["target", "train_position", "worktree"] {
            assert!(json.get(key).is_none(), "branch {key}: {json}");
        }
    }

    #[test]
    fn done_answer_shape() {
        let member = Member {
            session: SessionId("w".to_owned()),
            worker: None,
            via: Via::Joined,
        };
        let row = MemberRow::from((&FleetId("T1".to_owned()), &member, (None, None, true)));
        let delivered = serde_json::to_value(DoneAnswer::Delivered {
            member: row.clone(),
        })
        .expect("serialize");
        assert_eq!(delivered["outcome"], "delivered", "{delivered}");
        assert_eq!(
            delivered["member"],
            serde_json::to_value(&row).expect("serialize"),
            "{delivered}"
        );
        let landing = LandAnswerRow::new("feature/x".to_owned(), LandReply::Queued { place: 2 });
        let accepted = serde_json::to_value(DoneAnswer::Accepted {
            landing: landing.clone(),
        })
        .expect("serialize");
        assert_eq!(accepted["outcome"], "accepted", "{accepted}");
        assert_eq!(accepted["landing"]["state"], "queued", "{accepted}");
        assert_eq!(accepted["landing"]["place"], 2, "{accepted}");
        assert_eq!(accepted["landing"]["branch"], "feature/x", "{accepted}");
        let back: DoneAnswer = serde_json::from_value(accepted).expect("decode");
        assert_eq!(back, DoneAnswer::Accepted { landing });
        let cases = [
            (
                Some("main"),
                serde_json::json!({"outcome": "no_target", "base": "main"}),
            ),
            (None, serde_json::json!({"outcome": "no_target"})),
        ];
        for (base, want) in cases {
            let answer = DoneAnswer::NoTarget {
                base: base.map(str::to_owned),
            };
            let json = serde_json::to_value(&answer).expect("serialize");
            assert_eq!(json, want, "no_target {base:?}");
            let back: DoneAnswer = serde_json::from_value(json).expect("decode");
            assert_eq!(back, answer, "no_target {base:?} decodes");
        }
    }

    #[test]
    fn worker_rows_carry_no_agent_id() {
        let t1 = FleetId("T1".to_owned());
        let branch = WorkerBranch("claude/t1-a".to_owned());
        let member = Member {
            session: SessionId("o".to_owned()),
            worker: Some(branch.clone()),
            via: Via::Worker,
        };
        let letter = MemberId("A".to_owned());
        let facts = (Some(branch.0.clone()), Some(&letter), true);
        let row = serde_json::to_value(MemberRow::from((&t1, &member, facts))).expect("serialize");
        let mut instance = FleetInstance::new(
            crate::ids::InstanceId::legacy(&t1, jiff::Timestamp::UNIX_EPOCH),
            t1.clone(),
            jiff::Timestamp::UNIX_EPOCH,
        );
        instance.roster.insert(
            (SessionId("o".to_owned()), Some(branch.clone())),
            crate::fleet::RosterEntry {
                member: Some(letter.clone()),
                via: Via::Worker,
                joined_at: jiff::Timestamp::UNIX_EPOCH,
                left_at: None,
                branch: Some(branch.0.clone()),
                delivered: true,
            },
        );
        let archive = FleetArchive::from((&instance, Vec::new(), jiff::Timestamp::UNIX_EPOCH));
        let archived = serde_json::to_value(&archive.members[0]).expect("serialize");
        let cases = [
            ("member-row-without-agent-id", row),
            ("archived-member-without-agent-id", archived),
        ];
        for (label, json) in cases {
            assert!(json.get("agent_id").is_none(), "{label}: {json}");
            assert_eq!(json["via"], "worker", "{label}: {json}");
            assert_eq!(json["token"], "T1-MA", "{label}: {json}");
            assert_eq!(json["branch"], "claude/t1-a", "{label}: {json}");
            assert_eq!(json["session_id"], "o", "{label}: {json}");
        }
    }

    #[test]
    fn worker_body_cases() {
        let cases = [
            (
                "worker-body-decodes",
                r#"{"session_id":"o","token":"t","branch":"claude/t1-a","state":"delivered"}"#,
                Some(WorkerStateWire::Delivered),
            ),
            (
                "worker-body-running",
                r#"{"session_id":"o","token":"t","branch":"claude/t1-a","state":"running"}"#,
                Some(WorkerStateWire::Running),
            ),
            (
                "worker-body-unknown-state-refused",
                r#"{"session_id":"o","token":"t","branch":"claude/t1-a","state":"failed"}"#,
                None,
            ),
        ];
        for (label, text, want) in cases {
            let decoded = serde_json::from_str::<WorkerBody>(text).ok();
            assert_eq!(decoded.as_ref().map(|body| body.state), want, "{label}");
            if let Some(body) = decoded {
                assert_eq!(body.branch, "claude/t1-a", "{label}");
                assert_eq!(body.session_id, "o", "{label}");
            }
        }
    }

    #[test]
    fn p1_document_still_decodes() {
        let document = serde_json::json!({
            "schema": 1,
            "heartbeat": {
                "version": "0", "claude_home": "/h", "addr": "127.0.0.1:1",
                "started_at": "2026-09-27T00:00:00Z", "events_applied": 0
            },
            "sensors": [],
            "sessions": []
        });
        let decoded: StatusDocument = serde_json::from_value(document).expect("P1 document");
        assert!(decoded.fleets.is_empty() && decoded.members.is_empty());
        assert!(decoded.landings.is_empty(), "a document without landings");
    }

    #[test]
    fn token_never_serialized() {
        let row = SubscriptionRow::from((
            &SessionId("o".to_owned()),
            &subscription(SubscriptionState::Active),
        ));
        let text = serde_json::to_string(&row).expect("serialize");
        assert!(!text.contains(TOKEN), "{text}");
    }

    #[test]
    fn runtime_enum_strings_are_pinned() {
        let modes = [ModeWire::Report, ModeWire::Act];
        let actions = [MoveWire::Send, MoveWire::Stop];
        let states = [
            RunStateWire::Open,
            RunStateWire::Ended,
            RunStateWire::Killed,
        ];
        let kinds = [FindingKindWire::CheckFailed, FindingKindWire::Judgment];
        let acted = [
            ActedWire::Open,
            ActedWire::Claiming,
            ActedWire::SendPending,
            ActedWire::StopArmed,
            ActedWire::HandedOver,
            ActedWire::StopCleared,
            ActedWire::Void,
        ];
        let pairs = modes
            .iter()
            .map(|wire| (serde_json::to_value(wire), wire.as_str()))
            .chain(
                actions
                    .iter()
                    .map(|wire| (serde_json::to_value(wire), wire.as_str())),
            )
            .chain(
                states
                    .iter()
                    .map(|wire| (serde_json::to_value(wire), wire.as_str())),
            )
            .chain(
                kinds
                    .iter()
                    .map(|wire| (serde_json::to_value(wire), wire.as_str())),
            )
            .chain(
                acted
                    .iter()
                    .map(|wire| (serde_json::to_value(wire), wire.as_str())),
            );
        for (wire, want) in pairs {
            assert_eq!(wire.expect("serialize"), want);
        }
        for outcome in [
            DigestOutcomeWire::InFlight,
            DigestOutcomeWire::Sent,
            DigestOutcomeWire::Refused,
            DigestOutcomeWire::Unconfirmed,
            DigestOutcomeWire::Unclaimed,
        ] {
            assert_eq!(
                serde_json::to_value(outcome).expect("serialize"),
                outcome.as_str()
            );
        }
        for kind in [FindingKind::CheckFailed, FindingKind::Judgment] {
            assert_eq!(FindingKindWire::from(kind).as_str(), kind.as_str());
        }
    }

    #[test]
    fn runtime_document_nulls_and_no_token() {
        let document = RuntimeDocument {
            schema: RUNTIME_SCHEMA,
            heartbeat: Health {
                version: "0".to_owned(),
                claude_home: "/h".to_owned(),
                addr: "127.0.0.1:1".to_owned(),
            },
            subscriptions: Vec::new(),
            runs: Vec::new(),
            findings: Vec::new(),
        };
        let json = serde_json::to_value(&document).expect("serialize");
        assert_eq!(json["subscription"], serde_json::Value::Null, "{json}");
        let body = RuntimeActBody {
            session_id: "o".to_owned(),
            token: MessagingToken(TOKEN.to_owned()),
            finding: "f".to_owned(),
            action: MoveWire::Stop,
        };
        let text = serde_json::to_string(&body).expect("serialize");
        assert!(text.contains(r#""move":"stop""#), "{text}");
    }
}
