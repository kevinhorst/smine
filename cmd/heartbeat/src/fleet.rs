//! The state the engine owns. P1 holds sessions and sensor health. P2 adds the registered repositories, the
//! exemptions and the scan counters. P3 adds the subscriptions (one per subscriber session, each binding its fleet to
//! a repository and an integration branch) and each session's socket. Fleet members add the join records, the member
//! letters, the delivered set and the orchestrators' journals, all replayed from the action ledger; fleets, members,
//! bindings, pins and target states are derived from those by [`Fleet`] methods, never stored. Letters, journal and
//! roster belong to a fleet run ([`FleetInstance`], one live run per name, a UUID v7 each); a run with no joined
//! member and no active subscription dissolves once the registry has seeded, and its archive file is its only home.
//! The heart attack ([`crate::attack`]) adds the forbidden subagent models and each subagent's model. Runtime
//! enforcement ([`crate::runtime`]) adds its singleton state beside the fleets, never coupled to them.
//!
//! Retention: terminal sessions (ended, died) stay listed until `max_terminal` newer terminal sessions push them out.
//! Eviction runs on the terminal transition itself, never on a clock.

use std::collections::{BTreeMap, BTreeSet, HashMap, VecDeque};
use std::path::PathBuf;

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use crate::branch;
use crate::config::{Caps, Exemptions, ForbiddenModels, LanePrefixes};
use crate::digest::{Item, ItemKey};
use crate::ids::{
    AgentId, DigestId, FleetId, InstanceId, MemberId, MessagingToken, Pid, SessionId, WorkerBranch,
};
use crate::land::Landings;
use crate::protocol::{BRANCH_CHARSET, FLEET_ID_CHARSET};
use crate::repo::{self, BranchKey, Repo, RepoId};
use crate::runtime::Runtime;

/// A member: its session, plus the branch for a registered worker (the session is then the subscriber that
/// registered it).
pub type MemberKey = (SessionId, Option<WorkerBranch>);

/// Everything the engine knows.
#[derive(Debug, Clone)]
pub struct Fleet {
    /// When this daemon started; a denial older than this never creates a session.
    pub started_at: Timestamp,
    /// Every known session, in id order.
    pub sessions: BTreeMap<SessionId, Session>,
    /// Registry pid → session, for the process-exit watch.
    pub pids: HashMap<Pid, SessionId>,
    /// Health per edge.
    pub sensors: BTreeMap<SensorName, Sensor>,
    /// Events applied since start, follow-ups included.
    pub events_applied: u64,
    /// Registered repositories with their git state; empty when `serve` runs without `--repos`.
    pub repos: BTreeMap<RepoId, Repo>,
    /// What never collides, from the git config.
    pub exemptions: Exemptions,
    /// Which branches are collision lanes, from the heartbeat config.
    pub lane_prefixes: LanePrefixes,
    /// Subagent models that cause a heart attack, from the heartbeat config.
    pub forbidden_models: ForbiddenModels,
    /// The engine's caps and lists, from the heartbeat config; swapped whole on a live reload.
    pub caps: Caps,
    /// agentwt scans reported back.
    pub scans_completed: u64,
    /// Highest per-repo number of scans in flight the runner observed at a scan start (the load bound is 1).
    pub scan_in_flight_high_water: u64,
    /// Subscriptions by subscriber session.
    pub subscriptions: BTreeMap<SessionId, Subscription>,
    /// Join records by session: the one source of title-free fleet membership.
    pub joins: BTreeMap<SessionId, Join>,
    /// The newest refused join per session and when, for the `/join` answer; unknown sessions keep the newest 32.
    pub join_refusals: BTreeMap<SessionId, (Timestamp, String)>,
    /// Joined sessions that reported done, and when; cleared on their next root prompt.
    pub delivered: BTreeMap<SessionId, Timestamp>,
    /// Landing turns per registered repo and target, replayed from the action ledger ([`crate::land`]).
    pub landings: Landings,
    /// The staging target; set at construction, never mutated; `None` when the install sets none — then no landing
    /// turn, no landing row, and the done skill refuses `branch:` (D11).
    pub staging: Option<StagingTarget>,
    /// Live fleet runs, one per name; a dissolved run lives only in its archive file.
    pub instances: BTreeMap<FleetId, FleetInstance>,
    /// The registry edge finished its startup scan: leave sweeps and dissolves may run.
    pub seeded: bool,
    /// The id the next digest gets; the first is `d1`.
    pub next_digest: DigestId,
    /// Runtime enforcement's state: enforced runs, findings, the one runtime subscriber, its moves
    /// ([`crate::runtime`]).
    pub runtime: Runtime,
    /// Terminal sessions, oldest terminal transition first.
    terminal: VecDeque<SessionId>,
}

/// The staging target (`heartbeat.staging_target`): the one landing target the done skill creates and catches up.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StagingTarget(pub String);

/// One Claude Code session.
#[derive(Debug, Clone, PartialEq)]
pub struct Session {
    /// The session id.
    pub id: SessionId,
    /// The lifecycle state.
    pub state: SessionState,
    /// When `state` last changed.
    pub state_since: Timestamp,
    /// This daemon did not observe the session's start (first seen via the registry, a later hook or a denial).
    pub since_restart: bool,
    /// The process, from the registry.
    pub pid: Option<Pid>,
    /// The latest `cwd` from a hook, else the registry's.
    pub cwd: Option<PathBuf>,
    /// The registry `name`: the session title.
    pub title: Option<String>,
    /// The registry's `statusUpdatedAt` at seed time: the replay guard of ledger history (D7).
    pub registry_status_at: Option<Timestamp>,
    /// The last slash command or `Skill` call of the root agent.
    pub active_skill: Option<String>,
    /// The current turn's `UserPromptSubmit`.
    pub turn_started: Option<Timestamp>,
    /// The last root `Stop`.
    pub stopped_at: Option<Timestamp>,
    /// The last completed tool call (`PostToolUse` / `PostToolUseFailure`, root or subagent).
    pub last_tool_at: Option<Timestamp>,
    /// The last turn denial no completed call has followed yet.
    pub pending_denial: Option<PendingDenial>,
    /// The last hook event applied.
    pub last_event: Option<LastEvent>,
    /// Subagents a `SubagentStart` registered.
    pub subagents: BTreeMap<AgentId, Subagent>,
    /// The session's messaging socket, from the registry.
    pub socket: Option<PathBuf>,
}

/// The session lifecycle of concept F6; each state carries its own detail.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SessionState {
    /// Working on a turn.
    Running,
    /// Waiting on a human: a tool name, a notification type or the registry's `waitingFor`.
    Waiting {
        /// What it waits on.
        on: String,
    },
    /// The turn ended while a denial was pending.
    Blocked {
        /// The unresolved denial.
        denial: PendingDenial,
    },
    /// No turn in progress.
    Idle,
    /// `SessionEnd` arrived.
    Ended {
        /// `SessionEnd.reason`.
        reason: String,
    },
    /// The process exited without `SessionEnd`, or a `StopFailure` arrived.
    Died {
        /// Which of the two, with detail.
        cause: String,
    },
}

/// A denial no completed tool call has followed yet.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PendingDenial {
    /// What was denied.
    pub program: Program,
    /// The ledger's candidate rule.
    pub candidate_rule: String,
    /// The ledger's `ts`.
    pub at: Timestamp,
}

/// What a denial denied: `Bash:<first word>` for Bash, else the tool name.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Program(String);

/// The last hook event of a session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LastEvent {
    /// `hook_event_name`.
    pub name: String,
    /// When it arrived.
    pub at: Timestamp,
}

/// One subagent of a session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Subagent {
    /// `agent_type`.
    pub agent_type: String,
    /// No `SubagentStop` yet.
    pub running: bool,
    /// When `SubagentStart` arrived.
    pub started_at: Timestamp,
    /// The model its transcript shows (the first assistant line's `message.model`); `None` until read. A forbidden
    /// model gets this subagent stopped ([`crate::attack`]).
    pub model: Option<String>,
    /// What its spawn requested, read together with `model`; an inherited or substituted model gets this subagent
    /// stopped ([`crate::attack`]).
    pub requested: Requested,
}

/// The model a subagent's spawn requested — its `agent-<id>.meta.json`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Requested {
    /// The spawn named this model.
    Named(String),
    /// The spawn named none (no `model` key, empty or `inherit`): the subagent runs the session model.
    Inherited,
    /// No readable `meta.json`: nothing is judged.
    Unknown,
}

/// The edges that report health.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum SensorName {
    /// The HTTP edge; foreign-profile bodies degrade it.
    Hooks,
    /// The registry watch.
    Registry,
    /// The process-exit watch.
    ProcessExit,
    /// The denial ledger tail.
    Ledger,
    /// The git ref watch.
    Git,
    /// agentwt scans, reported by the core from each repo's last scan.
    Agentwt,
    /// The peek client.
    Peek,
    /// The action-ledger writer.
    ActionLedger,
    /// The config file watch.
    Config,
}

/// Sensor health.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SensorState {
    /// Delivering events.
    Ok,
    /// Not delivering, or delivering suspect input; `detail` says why.
    Degraded,
}

/// One sensor's health.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Sensor {
    /// Current state.
    pub state: SensorState,
    /// Why, when degraded.
    pub detail: Option<String>,
    /// When `state` was entered.
    pub since: Timestamp,
}

impl Fleet {
    /// An empty fleet for a daemon started at `started_at`.
    #[must_use]
    pub fn new(started_at: Timestamp) -> Fleet {
        Fleet {
            started_at,
            sessions: BTreeMap::new(),
            pids: HashMap::new(),
            sensors: BTreeMap::new(),
            events_applied: 0,
            repos: BTreeMap::new(),
            exemptions: Exemptions::default(),
            lane_prefixes: LanePrefixes::default(),
            forbidden_models: ForbiddenModels::default(),
            caps: Caps::default(),
            scans_completed: 0,
            scan_in_flight_high_water: 0,
            subscriptions: BTreeMap::new(),
            joins: BTreeMap::new(),
            join_refusals: BTreeMap::new(),
            delivered: BTreeMap::new(),
            landings: Landings::default(),
            staging: None,
            instances: BTreeMap::new(),
            seeded: false,
            next_digest: DigestId(1),
            runtime: Runtime::default(),
            terminal: VecDeque::new(),
        }
    }

    /// The session, created running on first sight; `since_restart` applies only to a new session.
    pub fn session_entry(
        &mut self,
        id: &SessionId,
        at: Timestamp,
        since_restart: bool,
    ) -> &mut Session {
        self.sessions
            .entry(id.clone())
            .or_insert_with(|| Session::new(id.clone(), SessionState::Running, at, since_restart))
    }

    /// Records a terminal session and evicts the oldest terminal sessions beyond `caps.max_terminal()`.
    pub fn retire(&mut self, id: &SessionId) {
        let is_terminal = self
            .sessions
            .get(id)
            .is_some_and(|session| session.state.is_terminal());
        if !is_terminal || self.terminal.contains(id) {
            return;
        }
        self.terminal.push_back(id.clone());
        let max_terminal = self.caps.max_terminal();
        while self.terminal.len() > max_terminal {
            let Some(oldest) = self.terminal.pop_front() else {
                break;
            };
            self.sessions.remove(&oldest);
            self.joins.remove(&oldest);
            self.delivered.remove(&oldest);
            self.join_refusals.remove(&oldest);
        }
    }

    /// Forgets a session's terminal record: a `SessionStart` revives it.
    pub fn unretire(&mut self, id: &SessionId) {
        self.terminal.retain(|terminal| terminal != id);
    }
}

impl Session {
    /// A new session in `state` since `at`.
    #[must_use]
    pub fn new(id: SessionId, state: SessionState, at: Timestamp, since_restart: bool) -> Session {
        Session {
            id,
            state,
            state_since: at,
            since_restart,
            pid: None,
            cwd: None,
            title: None,
            registry_status_at: None,
            active_skill: None,
            turn_started: None,
            stopped_at: None,
            last_tool_at: None,
            pending_denial: None,
            last_event: None,
            subagents: BTreeMap::new(),
            socket: None,
        }
    }

    /// Resets what belongs to one lifecycle: a new `SessionStart` observed it from its start.
    pub fn restart(&mut self) {
        self.since_restart = false;
        self.active_skill = None;
        self.turn_started = None;
        self.stopped_at = None;
        self.last_tool_at = None;
        self.pending_denial = None;
        self.subagents.clear();
    }

    /// A turn started (`UserPromptSubmit`) and no `Stop` came after it.
    #[must_use]
    pub fn is_turn_open(&self) -> bool {
        match (self.turn_started, self.stopped_at) {
            (Some(started), Some(stopped)) => started > stopped,
            (Some(_), None) => true,
            (None, _) => false,
        }
    }

    /// Moves to `state`; `state_since` changes only when the state actually changes.
    pub fn transition(&mut self, state: SessionState, at: Timestamp) {
        if self.state != state {
            self.state = state;
            self.state_since = at;
        }
    }
}

impl SessionState {
    /// Ended or died: only a `SessionStart` leaves it.
    #[must_use]
    pub fn is_terminal(&self) -> bool {
        match self {
            SessionState::Ended { .. } | SessionState::Died { .. } => true,
            SessionState::Running
            | SessionState::Waiting { .. }
            | SessionState::Blocked { .. }
            | SessionState::Idle => false,
        }
    }
}

impl Program {
    /// The program of a call: `Bash:<first word of command>` for Bash with a command, else the tool name.
    #[must_use]
    pub fn of(tool: &str, command: Option<&str>) -> Program {
        match (
            tool,
            command.and_then(|command| command.split_whitespace().next()),
        ) {
            ("Bash", Some(first)) => Program(format!("Bash:{first}")),
            _ => Program(tool.to_owned()),
        }
    }

    /// The wire spelling.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

/// A validated `POST /subscribe` body (the socket's file type is checked by the HTTP edge).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SubscribeRequest {
    /// The subscriber session.
    pub session: SessionId,
    /// The fleet it orchestrates.
    pub fleet: FleetId,
    /// The fleet's integration branch.
    pub target: String,
    /// Its messaging socket.
    pub socket: PathBuf,
    /// Its messaging token.
    pub token: MessagingToken,
    /// Hear collisions with branches outside the fleet too (`subscribe --outside-collisions`).
    pub outside_collisions: bool,
}

impl SubscribeRequest {
    /// Shape checks: `FleetId` charset of at most `caps.max_id_len()` bytes, printable-ASCII session id and token of
    /// at most `caps.max_field()` bytes, a branch-shaped target, absolute socket path.
    pub fn parse(
        fleet: &str,
        session: &str,
        target: &str,
        socket: PathBuf,
        token: MessagingToken,
        outside_collisions: bool,
        caps: &Caps,
    ) -> Result<SubscribeRequest, String> {
        let max_field = caps.max_field();
        let printable = |text: &str| {
            !text.is_empty()
                && text.len() <= max_field
                && text.bytes().all(|byte| byte.is_ascii_graphic())
        };
        let fleet = FleetId::parse(fleet, caps.max_id_len())
            .ok_or_else(|| format!("fleet {fleet:?} is not {FLEET_ID_CHARSET}"))?;
        if !printable(session) {
            return Err(format!(
                "session id must be 1-{max_field} printable ASCII characters"
            ));
        }
        if !printable(&token.0) {
            return Err(format!(
                "token must be 1-{max_field} printable ASCII characters"
            ));
        }
        Self::check_target(target, max_field)?;
        if !socket.is_absolute() {
            return Err(format!("socket path {} is not absolute", socket.display()));
        }
        Ok(SubscribeRequest {
            session: SessionId(session.to_owned()),
            fleet,
            target: target.to_owned(),
            socket,
            token,
            outside_collisions,
        })
    }

    /// A `git check-ref-format`-style branch name: 1-`max_field` bytes of [`BRANCH_CHARSET`], no `..`, no leading `-`.
    pub fn check_target(target: &str, max_field: usize) -> Result<(), String> {
        let charset = target
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'_' | b'/' | b'-'));
        let valid = !target.is_empty()
            && target.len() <= max_field
            && charset
            && !target.contains("..")
            && !target.starts_with('-');
        if valid {
            Ok(())
        } else {
            Err(format!(
                "target {target:?} is not a branch name ({BRANCH_CHARSET}, no `..`, no leading `-`)"
            ))
        }
    }
}

/// Where a `Subscribed` event comes from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SubscriptionSource {
    /// `POST /subscribe`: admitted against the fleet (its repo resolved from the session's cwd), recorded in the ledger.
    Request {
        /// The id a new fleet run gets when this subscription starts one; minted by the edge.
        candidate: InstanceId,
    },
    /// The action ledger at startup: already recorded and bound; the session may not be known yet.
    Replay {
        /// The repository the recorded subscription bound.
        repo: RepoId,
        /// Fleet-level item keys this subscription's digests already claimed.
        announced: BTreeSet<String>,
    },
}

/// One subscriber's wake channel and digest state.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Subscription {
    /// The fleet it orchestrates.
    pub fleet: FleetId,
    /// The run it was bound to; `None` while refused.
    pub instance: Option<InstanceId>,
    /// The bound repository; `None` only for a refusal that happened before a repo was resolved.
    pub repo: Option<RepoId>,
    /// The fleet's integration branch.
    pub target: String,
    /// Its messaging socket.
    pub socket: PathBuf,
    /// Its messaging token.
    pub token: MessagingToken,
    /// Lifecycle.
    pub state: SubscriptionState,
    /// When `state` was entered.
    pub since: Timestamp,
    /// Items not yet delivered, latest per key.
    pub pending: BTreeMap<ItemKey, Item>,
    /// The digest handed to the socket edge and not yet reported back.
    pub in_flight: Option<DigestId>,
    /// The last digest's outcome.
    pub last: Option<LastDigest>,
    /// Fleet-level item keys already claimed in a digest; restored from the ledger on replay.
    pub announced: BTreeSet<String>,
    /// Worker branches (target excluded) seen with a scanned row since this subscription started, this daemon run;
    /// `integrated` needs one, so a fleet of branchless helpers never reads integrated.
    pub worker_branches_seen: BTreeSet<BranchKey>,
    /// The run hears collisions with branches outside the fleet too ([`CollisionScope::outside`]).
    pub outside_collisions: bool,
}

/// Subscription lifecycle.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SubscriptionState {
    /// Receives digests.
    Active,
    /// Never admitted; the reason.
    Refused(String),
    /// Was active; the reason it ended.
    Ended(String),
}

/// A digest's recorded fate.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LastDigest {
    /// Digest id.
    pub id: DigestId,
    /// What happened.
    pub outcome: Outcome,
    /// When it was reported.
    pub at: Timestamp,
}

/// A send's outcome (docs/heartbeat.html § Digests).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Outcome {
    /// Written in full and shut down; the socket gives no ack.
    Sent,
    /// Nothing sent: connect failed or the target is no socket.
    Refused(String),
    /// Claimed, then the write failed or its outcome was never recorded.
    Unconfirmed(String),
    /// The claim could not be written, so nothing was sent.
    Unclaimed(String),
}

/// One fleet member (concept D8).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Member {
    /// The member session; for a worker, the subscriber that registered it.
    pub session: SessionId,
    /// The branch, for registered workers.
    pub worker: Option<WorkerBranch>,
    /// Why it is a member.
    pub via: Via,
}

/// Why a session or worker branch is a fleet member.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Via {
    /// It joined the fleet (`heartbeat join`).
    Joined,
    /// It is an active subscriber of the fleet.
    Subscriber,
    /// The fleet's subscriber registered its branch (`heartbeat worker`).
    Worker,
}

/// One session's fleet membership.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Join {
    /// The fleet.
    pub fleet: FleetId,
    /// The session's messaging token, handed over at join; `done` must match it.
    pub token: MessagingToken,
    /// When the session joined.
    pub since: Timestamp,
}

/// One live fleet run.
#[derive(Debug, Clone, PartialEq)]
pub struct FleetInstance {
    /// The run's UUID.
    pub id: InstanceId,
    /// The fleet name (`T1`).
    pub name: FleetId,
    /// When the run started.
    pub since: Timestamp,
    /// Member letters of this run, first-seen order.
    pub member_ids: MemberIds,
    /// Every journal entry of this run, oldest first.
    pub journal: Vec<JournalEntry>,
    /// Everyone who was a member of this run, current or gone.
    pub roster: BTreeMap<MemberKey, RosterEntry>,
    /// Letters the operator removed; `members` skips them until the session joins again.
    pub removed: BTreeSet<MemberId>,
    /// Worker branches the run's subscriber registered, with their delivery.
    pub workers: BTreeMap<WorkerBranch, Worker>,
    /// The binding facts last seen while bound.
    pub last: LastFacts,
}

impl FleetInstance {
    /// An empty run `id` of `name`, started at `since`.
    #[must_use]
    pub fn new(id: InstanceId, name: FleetId, since: Timestamp) -> FleetInstance {
        FleetInstance {
            id,
            name,
            since,
            member_ids: MemberIds::default(),
            journal: Vec::new(),
            roster: BTreeMap::new(),
            removed: BTreeSet::new(),
            workers: BTreeMap::new(),
            last: LastFacts::default(),
        }
    }
}

/// One registered worker of a run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Worker {
    /// The subscriber that registered it.
    pub by: SessionId,
    /// When it was registered.
    pub since: Timestamp,
    /// The subscriber reported its work finished.
    pub delivered: bool,
}

/// One member of a run, as last seen.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RosterEntry {
    /// Its letter; `None` for the subscriber (the operator).
    pub member: Option<MemberId>,
    /// Why it was a member.
    pub via: Via,
    /// First seen as a member.
    pub joined_at: Timestamp,
    /// Left (or the run dissolved); `None` while a member.
    pub left_at: Option<Timestamp>,
    /// Its branch as last seen.
    pub branch: Option<String>,
    /// Delivered as last seen.
    pub delivered: bool,
}

/// A bound run's facts, kept after the binding ends.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct LastFacts {
    /// The bound repository.
    pub repo: Option<RepoId>,
    /// The target branch.
    pub target: Option<String>,
    /// The merge branch.
    pub merge_branch: Option<String>,
    /// Whether the target held the merge branch.
    pub synced: Option<bool>,
}

/// A fleet run's member letters.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct MemberIds {
    /// Letters by member, first-seen order.
    pub by_member: BTreeMap<MemberKey, MemberId>,
}

impl MemberIds {
    /// The member's letter, assigning the next free one on first sight; `true` when it was new.
    pub fn assign(&mut self, key: &MemberKey) -> (MemberId, bool) {
        if let Some(id) = self.by_member.get(key) {
            return (id.clone(), false);
        }
        let id = MemberId::nth(self.by_member.len());
        self.by_member.insert(key.clone(), id.clone());
        (id, true)
    }
}

/// The journal entry kinds (docs/heartbeat.html § Fleet page and orchestrator).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum JournalKind {
    /// Work handed to a member.
    Assigned,
    /// A judgement over members: combine, sequence, hold, sync.
    Decided,
    /// A member branch merged into the target.
    Merged,
    /// A worker spawned.
    Spawned,
    /// Anything else, the final report included.
    Note,
}

impl JournalKind {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            JournalKind::Assigned => "assigned",
            JournalKind::Decided => "decided",
            JournalKind::Merged => "merged",
            JournalKind::Spawned => "spawned",
            JournalKind::Note => "note",
        }
    }

    /// The kind a wire spelling names.
    #[must_use]
    pub fn parse(text: &str) -> Option<JournalKind> {
        [
            JournalKind::Assigned,
            JournalKind::Decided,
            JournalKind::Merged,
            JournalKind::Spawned,
            JournalKind::Note,
        ]
        .into_iter()
        .find(|kind| kind.as_str() == text)
    }
}

/// One journal entry an orchestrator recorded.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct JournalEntry {
    /// The kind.
    pub kind: JournalKind,
    /// The member letters it concerns.
    pub members: Vec<MemberId>,
    /// One line: what.
    pub subject: String,
    /// Why, or the evidence.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
    /// The recording orchestrator.
    pub by: SessionId,
    /// When it was recorded.
    pub at: Timestamp,
}

/// A fleet's integration state (docs/heartbeat.html § Fleets).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TargetState {
    /// Member work is still open, or the repo has no good scan yet that requested the target.
    Open,
    /// At least one worker exists (the subscriber excluded), every worker is delivered, at least one worker branch was seen
    /// in a scan, and every worker branch that still has a row (the target branch excluded) is merged or removable; a
    /// worker branch without a row counts as done.
    Integrated,
    /// The repo's last good scan requested the target as a train base and has no train for it: not a local branch.
    Missing,
}

impl Fleet {
    /// The repository and integration branch `fleet` is bound to: those of its active subscription.
    #[must_use]
    pub fn binding(&self, fleet: &FleetId) -> Option<(&RepoId, &str)> {
        let subscription = self.bound_subscription(fleet)?;
        Some((subscription.repo.as_ref()?, subscription.target.as_str()))
    }

    /// The active subscription binding `fleet`, if any.
    #[must_use]
    pub fn bound_subscription(&self, fleet: &FleetId) -> Option<&Subscription> {
        self.subscriptions.values().find(|subscription| {
            subscription.fleet == *fleet
                && subscription.state == SubscriptionState::Active
                && subscription.repo.is_some()
        })
    }

    /// The members of `fleet`, ordered by session then worker branch: joined sessions, the active subscriber (a
    /// subscriber that also joined stays `Subscriber`) and the run's registered workers. With a binding, a joined
    /// session of another repository is no member (see [`Fleet::foreign`]).
    #[must_use]
    pub fn members(&self, fleet: &FleetId) -> Vec<Member> {
        let bound = self.binding(fleet).map(|(repo, _)| repo);
        let mut members: BTreeMap<MemberKey, Via> = BTreeMap::new();
        for (id, join) in &self.joins {
            let joined_here = join.fleet == *fleet
                && self
                    .sessions
                    .get(id)
                    .is_some_and(|session| self.in_repo(session, bound));
            if joined_here {
                members.insert((id.clone(), None), Via::Joined);
            }
        }
        for (id, subscription) in &self.subscriptions {
            if subscription.fleet != *fleet || subscription.state != SubscriptionState::Active {
                continue;
            }
            members.insert((id.clone(), None), Via::Subscriber);
        }
        let workers = self
            .instances
            .get(fleet)
            .into_iter()
            .flat_map(|instance| &instance.workers);
        for (branch, worker) in workers {
            members.insert((worker.by.clone(), Some(branch.clone())), Via::Worker);
        }
        let removed = self.instances.get(fleet).map(|instance| &instance.removed);
        let is_removed = |key: &MemberKey| {
            removed
                .zip(self.member_id(fleet, key))
                .is_some_and(|(removed, letter)| removed.contains(letter))
        };
        members
            .into_iter()
            .filter(|(key, _)| !is_removed(key))
            .map(|((session, worker), via)| Member {
                session,
                worker,
                via,
            })
            .collect()
    }

    /// Sessions joined to `fleet` whose cwd lies in another repository than the bound one; empty without a binding.
    /// An active subscriber of the fleet is never foreign.
    #[must_use]
    pub fn foreign(&self, fleet: &FleetId) -> Vec<SessionId> {
        let Some((bound, _)) = self.binding(fleet) else {
            return Vec::new();
        };
        let subscriber = |id: &SessionId| {
            self.subscriptions.get(id).is_some_and(|subscription| {
                subscription.fleet == *fleet && subscription.state == SubscriptionState::Active
            })
        };
        self.joins
            .iter()
            .filter(|(_, join)| join.fleet == *fleet)
            .filter_map(|(id, _)| self.sessions.get(id))
            .filter(|session| !subscriber(&session.id) && !self.in_repo(session, Some(bound)))
            .map(|session| session.id.clone())
            .collect()
    }

    /// Whether `session`'s cwd resolves to `bound` (`repo::repo_of`); always true without a binding.
    fn in_repo(&self, session: &Session, bound: Option<&RepoId>) -> bool {
        bound.is_none_or(|bound| {
            session
                .cwd
                .as_deref()
                .and_then(|cwd| repo::repo_of(self, cwd))
                .as_ref()
                == Some(bound)
        })
    }

    /// Every live fleet run's name, plus the names of refused, unbound subscriptions.
    #[must_use]
    pub fn fleet_ids(&self) -> BTreeSet<FleetId> {
        let live = self.instances.keys().cloned();
        let refused = self
            .subscriptions
            .values()
            .filter(|subscription| subscription.instance.is_none())
            .filter(|subscription| matches!(subscription.state, SubscriptionState::Refused(_)))
            .map(|subscription| subscription.fleet.clone());
        live.chain(refused).collect()
    }

    /// The member's letter in `fleet`'s live run, once assigned.
    #[must_use]
    pub fn member_id(&self, fleet: &FleetId, key: &MemberKey) -> Option<&MemberId> {
        self.instances.get(fleet)?.member_ids.by_member.get(key)
    }

    /// Whether the member finished: a joined session that reported done, a worker its subscriber marked delivered.
    #[must_use]
    pub fn is_delivered(&self, member: &Member) -> bool {
        match &member.worker {
            Some(branch) => self
                .instances
                .values()
                .filter_map(|instance| instance.workers.get(branch))
                .any(|worker| worker.by == member.session && worker.delivered),
            None => self.delivered.contains_key(&member.session),
        }
    }

    /// The branch the fleet's merges land on: `claude-orch/<fleet>-<run uuid>` (the fleet skill's own branch,
    /// unique per run, so a reused fleet name never meets an old run's branch) while the fleet is bound; `None`
    /// unbound or without a live run.
    #[must_use]
    pub fn merge_branch(&self, fleet: &FleetId) -> Option<String> {
        self.binding(fleet)?;
        let instance = self.instances.get(fleet)?;
        Some(format!(
            "{}{}-{}",
            self.caps.merge_branch_prefix(),
            fleet.0,
            instance.id.0
        ))
    }

    /// The collision scope of `fleet`; `None` without a binding.
    #[must_use]
    pub fn collision_scope(&self, fleet: &FleetId) -> Option<CollisionScope> {
        let (repo, _) = self.binding(fleet)?;
        let subscription = self.bound_subscription(fleet)?;
        let members = self
            .members(fleet)
            .iter()
            .filter(|member| member.via != Via::Subscriber)
            .filter_map(|member| member_branch(self, repo, member))
            .collect();
        let scope = CollisionScope {
            members,
            merge: self.merge_branch(fleet).map(BranchKey::Branch),
            outside: subscription.outside_collisions,
        };
        Some(scope)
    }

    /// Whether the target holds the merge branch's work: agentwt's containment of the merge-branch row names the
    /// target; `None` without a binding, a merge branch or its row.
    #[must_use]
    pub fn synced(&self, fleet: &FleetId) -> Option<bool> {
        let (repo, target) = self.binding(fleet)?;
        let merge = BranchKey::Branch(self.merge_branch(fleet)?);
        let scan = self.repos.get(repo)?.snapshot.as_ref()?;
        let row = scan.rows.iter().find(|row| row.key == merge)?;
        Some(row.contained_in.as_deref() == Some(target))
    }

    /// The fleets bound to `repo`, each with its target and merge branch (when known).
    fn bound_to(&self, repo: &RepoId) -> Vec<(String, Option<String>)> {
        self.fleet_ids()
            .into_iter()
            .filter_map(|id| {
                let (bound, target) = self.binding(&id)?;
                (bound == repo).then(|| (target.to_owned(), self.merge_branch(&id)))
            })
            .collect()
    }

    /// Sorted, deduplicated targets and merge branches of the fleets bound to `repo`, plus the staging target while
    /// the repo's last scan says it exists, plus every landing target a row of that scan resolved: its train bases.
    #[must_use]
    pub fn train_bases(&self, repo: &RepoId) -> Vec<String> {
        let scan = self
            .repos
            .get(repo)
            .and_then(|known| known.snapshot.as_ref());
        let staging = scan
            .filter(|scan| scan.landing.exists)
            .and(self.staging.as_ref().map(|s| s.0.clone()));
        let pins = self.pins(repo);
        let row_bases = scan.into_iter().flat_map(|scan| {
            scan.rows
                .iter()
                .filter_map(|row| branch::landing_target(row, scan, &pins))
        });
        let bases: BTreeSet<String> = self
            .bound_to(repo)
            .into_iter()
            .flat_map(|(target, merge)| std::iter::once(target).chain(merge))
            .chain(staging)
            .chain(row_bases)
            .collect();
        bases.into_iter().collect()
    }

    /// Sorted, deduplicated merge branches of the fleets bound to `repo`: tracked as containment candidates, so a
    /// member merged into an agent-owned merge branch reads merged.
    #[must_use]
    pub fn candidates(&self, repo: &RepoId) -> Vec<String> {
        let merges: BTreeSet<String> = self
            .bound_to(repo)
            .into_iter()
            .filter_map(|(_, merge)| merge)
            .collect();
        merges.into_iter().collect()
    }

    /// Branch name → merge branch, for every member branch (the subscriber's own excluded) of every fleet bound to
    /// `repo` (P2's train pins); none before the merge branch is known.
    #[must_use]
    pub fn pins(&self, repo: &RepoId) -> BTreeMap<String, String> {
        let mut pins = BTreeMap::new();
        for id in self.fleet_ids() {
            let Some((bound, _)) = self.binding(&id).filter(|(bound, _)| *bound == repo) else {
                continue;
            };
            let Some(merge) = self.merge_branch(&id) else {
                continue;
            };
            for member in self.members(&id) {
                if member.via == Via::Subscriber {
                    continue;
                }
                match member_branch(self, bound, &member) {
                    Some(BranchKey::Branch(name)) => {
                        pins.insert(name, merge.clone());
                    }
                    Some(BranchKey::Detached(_)) | None => {}
                }
            }
        }
        pins
    }

    /// The integration state of `fleet`; `None` without a binding. Judged only on the repo's last good scan that
    /// requested the target as a train base — before one, `Open`; a requested target without a train is `Missing`.
    /// The worker rule is judged against the merge branch ([`Fleet::merge_branch`]): `Open` until it is known and its
    /// train was scanned. Only workers count: members other than the active subscriber, whose session runs whenever it
    /// reads status and whose own branch is the merge branch; every worker must be delivered ([`Fleet::is_delivered`]:
    /// a joined member's `/done` not cleared by a newer prompt, a worker marked delivered). A worker branch without a row (removed after the merge,
    /// or never created) counts as done; a row keyed by the merge branch or the target is ignored. `Integrated` also needs a worker
    /// branch seen in a scan since the subscription started ([`Subscription::worker_branches_seen`]), so branchless
    /// helpers alone never make the fleet integrated.
    #[must_use]
    pub fn target_state(&self, fleet: &FleetId) -> Option<TargetState> {
        let (bound, target) = self.binding(fleet)?;
        let judged = self
            .repos
            .get(bound)
            .and_then(|repo| Some((repo, repo.snapshot.as_ref()?)));
        let Some((repo, scan)) =
            judged.filter(|(_, scan)| scan.requested.iter().any(|base| base == target))
        else {
            return Some(TargetState::Open);
        };
        if !scan.trains.iter().any(|train| train.base == target) {
            return Some(TargetState::Missing);
        }
        let Some(merge) = self.merge_branch(fleet) else {
            return Some(TargetState::Open);
        };
        if !scan.trains.iter().any(|train| train.base == merge) {
            return Some(TargetState::Open);
        }
        let workers: Vec<Member> = self
            .members(fleet)
            .into_iter()
            .filter(|member| member.via != Via::Subscriber)
            .collect();
        let seen_any = self
            .bound_subscription(fleet)
            .is_some_and(|subscription| !subscription.worker_branches_seen.is_empty());
        let is_all_delivered = workers.iter().all(|member| self.is_delivered(member));
        if workers.is_empty() || !seen_any || !is_all_delivered {
            return Some(TargetState::Open);
        }
        let skipped = [
            BranchKey::Branch(merge),
            BranchKey::Branch(target.to_owned()),
        ];
        let done = workers
            .iter()
            .filter_map(|member| member_branch(self, bound, member))
            .filter(|key| !skipped.contains(key))
            .filter_map(|key| repo.branches.get(&key))
            .all(|branch| !branch.state.is_live());
        Some(if done {
            TargetState::Integrated
        } else {
            TargetState::Open
        })
    }
}

/// Which collision pairs a fleet's orchestrator hears (docs/heartbeat.html § Digests).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct CollisionScope {
    /// Member branches, the subscriber's own excluded.
    pub members: BTreeSet<BranchKey>,
    /// The fleet's merge branch.
    pub merge: Option<BranchKey>,
    /// The run asked for pairs with non-fleet branches too (`subscribe --outside-collisions`).
    pub outside: bool,
}

impl CollisionScope {
    /// A pair with a member on one side; unless `outside`, the other side is a member or the merge branch too.
    #[must_use]
    pub fn keeps(&self, a: &BranchKey, b: &BranchKey) -> bool {
        let is_member = |key: &BranchKey| self.members.contains(key);
        let is_fleet = |key: &BranchKey| is_member(key) || self.merge.as_ref() == Some(key);
        let touches_member = is_member(a) || is_member(b);
        if self.outside {
            return touches_member;
        }
        touches_member && is_fleet(a) && is_fleet(b)
    }
}

/// The branch a fleet member works on in the fleet's repo: a worker's registered branch, else the scanned row of
/// the worktree the member session's cwd lies in; `None` when the repo has no such row.
#[must_use]
pub fn member_branch(fleet: &Fleet, repo: &RepoId, member: &Member) -> Option<BranchKey> {
    if let Some(branch) = &member.worker {
        return Some(BranchKey::Branch(branch.0.clone()));
    }
    let state = fleet.repos.get(repo)?;
    let cwd = fleet.sessions.get(&member.session)?.cwd.as_deref()?;
    let root = state.worktree_root(repo, cwd)?;
    state
        .snapshot
        .as_ref()?
        .rows
        .iter()
        .find(|row| row.worktree.as_deref() == Some(root.as_path()))
        .map(|row| row.key.clone())
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;
    use crate::branch::{Branch, BranchState};
    use crate::scan::{RepoScan, ScannedRow, Train};

    const R1: &str = "/r1";
    const R2: &str = "/r2";
    const WT_W: &str = "/r1/.claude/worktrees/w";
    const WT_A: &str = "/r1/.claude/worktrees/t1-a";
    /// The registered worker branch of the fixture run.
    const WORKER: &str = "claude/t1-a";
    const WT_X: &str = "/r1/.claude/worktrees/x";
    const WT_O: &str = "/r1/.claude/worktrees/o";
    /// The merge branches of the fixture runs (`claude-orch/<fleet>-<run uuid>`; test run ids carry the name).
    const MERGE: &str = "claude-orch/T1-00000000-0000-7431-8000-000000000000";
    const MERGE_T2: &str = "claude-orch/T2-00000000-0000-7432-8000-000000000000";
    const MERGE_T3: &str = "claude-orch/T3-00000000-0000-7433-8000-000000000000";

    fn id(path: &str) -> RepoId {
        RepoId(PathBuf::from(path))
    }

    fn sid(name: &str) -> SessionId {
        SessionId(name.to_owned())
    }

    fn t1_id() -> FleetId {
        FleetId("T1".to_owned())
    }

    fn row(name: &str, worktree: &str, main: bool) -> ScannedRow {
        ScannedRow {
            key: BranchKey::Branch(name.to_owned()),
            worktree: Some(PathBuf::from(worktree)),
            main,
            tracked_changes: 0,
            untracked: 0,
            own_commits: 1,
            fresh: false,
            from: Some("main".to_owned()),
            contained_in: None,
            safe_to_remove: false,
        }
    }

    fn train(base: &str) -> Train {
        Train {
            base: base.to_owned(),
            order: Vec::new(),
            steps: BTreeMap::new(),
        }
    }

    /// A scanned repo: `rows`, one branch fact per `(name, state)`, trains per base, requested bases.
    fn scanned_repo(
        rows: Vec<ScannedRow>,
        states: &[(&str, BranchState)],
        trains: &[&str],
        requested: &[&str],
    ) -> Repo {
        let mut repo = Repo::new("r".to_owned());
        repo.branches = states
            .iter()
            .map(|(name, state)| {
                let branch = Branch {
                    state: *state,
                    since: Timestamp::UNIX_EPOCH,
                    judged: "main".to_owned(),
                    target: None,
                };
                (BranchKey::Branch((*name).to_owned()), branch)
            })
            .collect();
        repo.snapshot = Some(RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows,
            pairs: Vec::new(),
            trains: trains.iter().map(|base| train(base)).collect(),
            requested: requested.iter().map(|base| (*base).to_owned()).collect(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        });
        repo
    }

    fn session(name: &str, cwd: &str, state: SessionState) -> Session {
        let mut session = Session::new(sid(name), state, Timestamp::UNIX_EPOCH, false);
        session.cwd = Some(PathBuf::from(cwd));
        session
    }

    /// Starts `name`'s live run when none is live.
    fn live(fleet: &mut Fleet, name: &str) {
        let name = FleetId(name.to_owned());
        let id = InstanceId::legacy(&name, Timestamp::UNIX_EPOCH);
        fleet
            .instances
            .entry(name.clone())
            .or_insert_with(|| FleetInstance::new(id, name, Timestamp::UNIX_EPOCH));
    }

    /// Inserts `session` joined to `fleet_id`.
    fn insert_joined(fleet: &mut Fleet, session: Session, fleet_id: &str) {
        live(fleet, fleet_id);
        let join = Join {
            fleet: FleetId(fleet_id.to_owned()),
            token: MessagingToken("t".to_owned()),
            since: Timestamp::UNIX_EPOCH,
        };
        fleet.joins.insert(session.id.clone(), join);
        insert(fleet, session);
    }

    /// Inserts `subscription` of `session`, its fleet's run started.
    fn subscribe(fleet: &mut Fleet, session: &str, mut subscription: Subscription) {
        live(fleet, &subscription.fleet.0);
        subscription.instance = fleet
            .instances
            .get(&subscription.fleet)
            .map(|instance| instance.id);
        fleet.subscriptions.insert(sid(session), subscription);
    }

    fn subagent(running: bool) -> Subagent {
        Subagent {
            agent_type: "general-purpose".to_owned(),
            running,
            started_at: Timestamp::UNIX_EPOCH,
            model: None,
            requested: Requested::Unknown,
        }
    }

    fn active(fleet: &str, repo: &str, target: &str) -> Subscription {
        Subscription {
            fleet: FleetId(fleet.to_owned()),
            instance: None,
            repo: Some(id(repo)),
            target: target.to_owned(),
            socket: PathBuf::from("/tmp/o.sock"),
            token: MessagingToken("t".to_owned()),
            state: SubscriptionState::Active,
            since: Timestamp::UNIX_EPOCH,
            pending: BTreeMap::new(),
            in_flight: None,
            last: None,
            announced: BTreeSet::new(),
            worker_branches_seen: BTreeSet::new(),
            outside_collisions: false,
        }
    }

    /// Registers worker `branch` in `fleet_id`'s live run on behalf of subscriber `by`.
    fn register(fleet: &mut Fleet, fleet_id: &str, by: &str, branch: &str, delivered: bool) {
        live(fleet, fleet_id);
        let instance = fleet
            .instances
            .get_mut(&FleetId(fleet_id.to_owned()))
            .expect("run");
        instance.workers.insert(
            WorkerBranch(branch.to_owned()),
            Worker {
                by: sid(by),
                since: Timestamp::UNIX_EPOCH,
                delivered,
            },
        );
    }

    fn worker_member(session: &str, branch: &str) -> Member {
        Member {
            session: sid(session),
            worker: Some(WorkerBranch(branch.to_owned())),
            via: Via::Worker,
        }
    }

    fn clear_seen(fleet: &mut Fleet) {
        let subscription = fleet.subscriptions.get_mut(&sid("o")).expect("o");
        subscription.worker_branches_seen.clear();
    }

    fn insert(fleet: &mut Fleet, session: Session) {
        fleet.sessions.insert(session.id.clone(), session);
    }

    /// T1 bound to `/r1` `main` by `o` (idle, own worktree on the merge branch `claude-orch/T1`, stopped subagent
    /// `a1`, delivered worker `claude/t1-a`); `w` joined T1, delivered, and works in `claude/w` (merged, seen by the
    /// subscription); `claude/t1-a` is removable; the last scan requested `main` and `claude-orch/T1` and has a train
    /// for both.
    fn bound() -> Fleet {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let rows = vec![
            row("main", R1, true),
            row(MERGE, WT_O, false),
            row("claude/w", WT_W, false),
            row(WORKER, WT_A, false),
        ];
        let states = [
            ("claude/w", BranchState::Merged),
            (WORKER, BranchState::Removable),
        ];
        let bases = ["main", MERGE];
        fleet
            .repos
            .insert(id(R1), scanned_repo(rows, &states, &bases, &bases));
        let mut o = session("o", WT_O, SessionState::Idle);
        o.subagents
            .insert(AgentId("a1".to_owned()), subagent(false));
        insert(&mut fleet, o);
        insert_joined(&mut fleet, session("w", WT_W, SessionState::Idle), "T1");
        fleet.delivered.insert(sid("w"), Timestamp::UNIX_EPOCH);
        let mut subscription = active("T1", R1, "main");
        subscription
            .worker_branches_seen
            .insert(BranchKey::Branch("claude/w".to_owned()));
        subscribe(&mut fleet, "o", subscription);
        register(&mut fleet, "T1", "o", WORKER, true);
        fleet
    }

    fn set_branch(fleet: &mut Fleet, name: &str, state: BranchState) {
        let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
        repo.branches.insert(
            BranchKey::Branch(name.to_owned()),
            Branch {
                state,
                since: Timestamp::UNIX_EPOCH,
                judged: "main".to_owned(),
                target: None,
            },
        );
    }

    /// Removes the worker rows; the main checkout and the orchestrator's merge-branch row stay.
    fn drop_rows(fleet: &mut Fleet) {
        let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
        repo.branches.clear();
        if let Some(scan) = repo.snapshot.as_mut() {
            scan.rows
                .retain(|row| row.main || row.key == BranchKey::Branch(MERGE.to_owned()));
        }
    }

    #[test]
    fn check_target_cases() {
        let max_field = Caps::default().max_field();
        let long = "b".repeat(max_field + 1);
        let cases = [
            ("main", "main", true),
            ("slashed", "feature/heartbeat", true),
            ("dotted", "release-1.2", true),
            ("empty", "", false),
            ("leading-dots", "..x", false),
            ("inner-dots", "a..b", false),
            ("leading-dash", "-x", false),
            ("space", "a b", false),
            ("too-long", long.as_str(), false),
        ];
        for (label, target, valid) in cases {
            assert_eq!(
                SubscribeRequest::check_target(target, max_field).is_ok(),
                valid,
                "{label}"
            );
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn target_state_cases() {
        type Setup = Box<dyn Fn(&mut Fleet)>;
        let t1 = FleetId("T1".to_owned());
        let cases: Vec<(&str, Setup, Option<TargetState>)> = vec![
            (
                "no-binding",
                Box::new(|fleet: &mut Fleet| fleet.subscriptions.clear()),
                None,
            ),
            (
                "no-scan",
                Box::new(|fleet: &mut Fleet| {
                    fleet.repos.get_mut(&id(R1)).expect("r1").snapshot = None;
                }),
                Some(TargetState::Open),
            ),
            (
                "rescan-pending",
                Box::new(|fleet: &mut Fleet| {
                    let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
                    repo.snapshot.as_mut().expect("scan").requested.clear();
                }),
                Some(TargetState::Open),
            ),
            (
                "target-missing",
                Box::new(|fleet: &mut Fleet| {
                    let subscription = fleet.subscriptions.get_mut(&sid("o")).expect("o");
                    subscription.target = "nope".to_owned();
                    let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
                    repo.snapshot.as_mut().expect("scan").requested = vec!["nope".to_owned()];
                }),
                Some(TargetState::Missing),
            ),
            (
                "merge-train-not-scanned",
                Box::new(|fleet: &mut Fleet| {
                    let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
                    let scan = repo.snapshot.as_mut().expect("scan");
                    scan.trains.retain(|train| train.base != MERGE);
                }),
                Some(TargetState::Open),
            ),
            (
                "joined-worker-running",
                Box::new(|fleet: &mut Fleet| {
                    fleet.sessions.get_mut(&sid("w")).expect("w").state = SessionState::Running;
                    fleet.delivered.remove(&sid("w"));
                }),
                Some(TargetState::Open),
            ),
            (
                "joined-worker-idle-not-delivered",
                Box::new(|fleet: &mut Fleet| {
                    fleet.delivered.remove(&sid("w"));
                }),
                Some(TargetState::Open),
            ),
            (
                "stuck-worker-removed",
                Box::new(|fleet: &mut Fleet| {
                    fleet.delivered.remove(&sid("w"));
                    let instance = fleet.instances.get_mut(&t1_id()).expect("run");
                    instance
                        .member_ids
                        .by_member
                        .insert((sid("w"), None), MemberId::nth(0));
                    instance.removed.insert(MemberId::nth(0));
                }),
                Some(TargetState::Integrated),
            ),
            (
                "integrated-needs-delivered-worker",
                Box::new(|fleet: &mut Fleet| register(fleet, "T1", "o", WORKER, false)),
                Some(TargetState::Open),
            ),
            (
                "integrated-with-merged-worker",
                Box::new(|fleet: &mut Fleet| {
                    fleet.joins.remove(&sid("w"));
                    set_branch(fleet, WORKER, BranchState::Merged);
                }),
                Some(TargetState::Integrated),
            ),
            (
                "subagent-of-subscriber-is-no-member",
                Box::new(|fleet: &mut Fleet| {
                    let o = fleet.sessions.get_mut(&sid("o")).expect("o");
                    o.subagents.insert(AgentId("a2".to_owned()), subagent(true));
                }),
                Some(TargetState::Integrated),
            ),
            (
                "only-the-subscriber",
                Box::new(|fleet: &mut Fleet| {
                    fleet.sessions.remove(&sid("w"));
                    fleet
                        .instances
                        .get_mut(&t1_id())
                        .expect("run")
                        .workers
                        .clear();
                }),
                Some(TargetState::Open),
            ),
            (
                "all-merged-or-removable",
                Box::new(|_: &mut Fleet| {}),
                Some(TargetState::Integrated),
            ),
            (
                "subscriber-running",
                Box::new(|fleet: &mut Fleet| {
                    fleet.sessions.get_mut(&sid("o")).expect("o").state = SessionState::Running;
                }),
                Some(TargetState::Integrated),
            ),
            (
                "row-keyed-by-target-ignored",
                Box::new(|fleet: &mut Fleet| {
                    let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
                    let scan = repo.snapshot.as_mut().expect("scan");
                    scan.rows.push(row("main", WT_X, false));
                    insert_joined(fleet, session("x", WT_X, SessionState::Idle), "T1");
                    fleet.delivered.insert(sid("x"), Timestamp::UNIX_EPOCH);
                    set_branch(fleet, "main", BranchState::Unmerged);
                }),
                Some(TargetState::Integrated),
            ),
            (
                "worker-rows-removed",
                Box::new(drop_rows),
                Some(TargetState::Integrated),
            ),
            (
                "worker-branch-never-created",
                Box::new(|fleet: &mut Fleet| register(fleet, "T1", "o", "claude/t1-z", true)),
                Some(TargetState::Integrated),
            ),
            (
                "no-worker-branch-seen",
                Box::new(clear_seen),
                Some(TargetState::Open),
            ),
            (
                "only-branchless-workers-finished",
                Box::new(|fleet: &mut Fleet| {
                    drop_rows(fleet);
                    clear_seen(fleet);
                }),
                Some(TargetState::Open),
            ),
            (
                "one-worker-branch-live",
                Box::new(|fleet: &mut Fleet| {
                    set_branch(fleet, "claude/w", BranchState::MergeReady);
                }),
                Some(TargetState::Open),
            ),
        ];
        for (label, setup, want) in cases {
            let mut fleet = bound();
            setup(&mut fleet);
            assert_eq!(fleet.target_state(&t1), want, "{label}");
        }
    }

    #[test]
    fn merge_branch_fixed() {
        let mut fleet = bound();
        assert_eq!(
            fleet.merge_branch(&t1_id()).as_deref(),
            Some(MERGE),
            "bound"
        );
        fleet.repos.get_mut(&id(R1)).expect("r1").snapshot = None;
        assert_eq!(
            fleet.merge_branch(&t1_id()).as_deref(),
            Some(MERGE),
            "bound, before any scan"
        );
        fleet.subscriptions.clear();
        assert_eq!(fleet.merge_branch(&t1_id()), None, "unbound");
    }

    #[test]
    fn collision_scope_keeps() {
        let key = |name: &str| BranchKey::Branch(name.to_owned());
        let scope = |outside: bool| CollisionScope {
            members: BTreeSet::from([key("claude/w"), key("claude/agent-a1")]),
            merge: Some(key(MERGE)),
            outside,
        };
        let cases = [
            (
                "member-vs-member",
                "claude/w",
                "claude/agent-a1",
                true,
                true,
            ),
            ("member-vs-merge-branch", "claude/w", MERGE, true, true),
            ("member-vs-outsider", "claude/w", "claude/h", false, true),
            ("merge-vs-outsider", MERGE, "claude/h", false, false),
            (
                "outsider-vs-outsider",
                "claude/h",
                "claude/h2",
                false,
                false,
            ),
        ];
        for (label, a, b, fleet_only, with_outside) in cases {
            assert_eq!(scope(false).keeps(&key(a), &key(b)), fleet_only, "{label}");
            assert_eq!(
                scope(true).keeps(&key(a), &key(b)),
                with_outside,
                "{label} outside"
            );
        }
    }

    #[test]
    fn member_branch_cases() {
        let mut fleet = bound();
        insert_joined(&mut fleet, session("m", R1, SessionState::Idle), "T1");
        let member = |session: &str| Member {
            session: sid(session),
            worker: None,
            via: Via::Joined,
        };
        let cases = [
            (
                "worker-branch-is-member-branch",
                worker_member("o", "claude/t1-z"),
                Some(BranchKey::Branch("claude/t1-z".to_owned())),
            ),
            (
                "joined-member-in-worktree",
                member("w"),
                Some(BranchKey::Branch("claude/w".to_owned())),
            ),
            ("joined-member-in-main-checkout", member("m"), None),
        ];
        for (label, member, want) in cases {
            assert_eq!(member_branch(&fleet, &id(R1), &member), want, "{label}");
        }
    }

    #[test]
    fn members_cases() {
        type Setup = Box<dyn Fn(&mut Fleet)>;
        let subscriber = Member {
            session: sid("o"),
            worker: None,
            via: Via::Subscriber,
        };
        let joined = Member {
            session: sid("w"),
            worker: None,
            via: Via::Joined,
        };
        let cases: Vec<(&str, Setup, Vec<Member>)> = vec![
            (
                "worker-member-listed",
                Box::new(|_: &mut Fleet| {}),
                vec![
                    subscriber.clone(),
                    worker_member("o", WORKER),
                    joined.clone(),
                ],
            ),
            (
                "worker-of-other-fleet-not-listed",
                Box::new(|fleet: &mut Fleet| register(fleet, "T2", "o2", "claude/t2-b", false)),
                vec![
                    subscriber.clone(),
                    worker_member("o", WORKER),
                    joined.clone(),
                ],
            ),
            (
                "removed-worker-skipped",
                Box::new(|fleet: &mut Fleet| {
                    let instance = fleet.instances.get_mut(&t1_id()).expect("run");
                    let key = (sid("o"), Some(WorkerBranch(WORKER.to_owned())));
                    instance.member_ids.by_member.insert(key, MemberId::nth(1));
                    instance.removed.insert(MemberId::nth(1));
                }),
                vec![subscriber.clone(), joined.clone()],
            ),
            (
                "subagent-of-subscriber-is-no-member",
                Box::new(|fleet: &mut Fleet| {
                    let o = fleet.sessions.get_mut(&sid("o")).expect("o");
                    o.subagents.insert(AgentId("a2".to_owned()), subagent(true));
                }),
                vec![
                    subscriber.clone(),
                    worker_member("o", WORKER),
                    joined.clone(),
                ],
            ),
        ];
        for (label, setup, want) in cases {
            let mut fleet = bound();
            setup(&mut fleet);
            assert_eq!(fleet.members(&t1_id()), want, "{label}");
        }
    }

    #[test]
    fn is_delivered_cases() {
        let cases = [
            ("worker-delivered", true, true),
            ("worker-running-not-delivered", false, false),
        ];
        for (label, delivered, want) in cases {
            let mut fleet = bound();
            register(&mut fleet, "T1", "o", WORKER, delivered);
            assert_eq!(
                fleet.is_delivered(&worker_member("o", WORKER)),
                want,
                "{label}"
            );
        }
    }

    #[test]
    fn pins_cover_bound_members() {
        let mut fleet = bound();
        let wt_o2 = "/r1/.claude/worktrees/o2";
        let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
        let rows = &mut repo.snapshot.as_mut().expect("scan").rows;
        rows.push(row("claude/x", WT_X, false));
        rows.push(row(MERGE_T2, wt_o2, false));
        insert(&mut fleet, session("o2", wt_o2, SessionState::Idle));
        insert_joined(&mut fleet, session("x", WT_X, SessionState::Idle), "T2");
        subscribe(&mut fleet, "o2", active("T2", R1, "develop"));
        let wt_y = "/r2/.claude/worktrees/y";
        fleet.repos.insert(
            id(R2),
            scanned_repo(vec![row("claude/y", wt_y, false)], &[], &["main"], &[]),
        );
        insert(&mut fleet, session("o3", R2, SessionState::Idle));
        insert_joined(&mut fleet, session("y", wt_y, SessionState::Idle), "T3");
        subscribe(&mut fleet, "o3", active("T3", R2, "main"));

        let pin = |branch: &str, target: &str| (branch.to_owned(), target.to_owned());
        let want = BTreeMap::from([
            pin("claude/t1-a", MERGE),
            pin("claude/w", MERGE),
            pin("claude/x", MERGE_T2),
        ]);
        assert_eq!(
            fleet.pins(&id(R1)),
            want,
            "members pinned to their merge branch"
        );
        assert_eq!(
            fleet.pins(&id(R2)),
            BTreeMap::from([pin("claude/y", MERGE_T3)]),
            "the merge branch is known from the binding, before any scan shows it"
        );
        assert_eq!(
            fleet.train_bases(&id(R1)),
            vec![MERGE, MERGE_T2, "develop", "main"]
        );
        assert_eq!(fleet.candidates(&id(R1)), vec![MERGE, MERGE_T2]);
        assert_eq!(fleet.train_bases(&id(R2)), vec![MERGE_T3, "main"]);
        assert_eq!(fleet.candidates(&id(R2)), vec![MERGE_T3]);
    }

    #[test]
    fn train_bases_follow_row_bases() {
        struct Case {
            name: &'static str,
            rows: Vec<ScannedRow>,
            staging: Option<&'static str>,
            exists: bool,
            want: Vec<&'static str>,
        }
        let based = |name: &str, from: &str| ScannedRow {
            from: Some(from.to_owned()),
            ..row(name, &format!("/r1/.claude/worktrees/{name}"), false)
        };
        let cases = [
            Case {
                name: "row base becomes a train base",
                rows: vec![based("claude/a", "feature/x")],
                staging: None,
                exists: false,
                want: vec!["feature/x"],
            },
            Case {
                name: "default-based row adds none",
                rows: vec![based("claude/a", "main")],
                staging: None,
                exists: false,
                want: vec![],
            },
            Case {
                name: "staging target only once a scan saw it",
                rows: vec![],
                staging: Some("develop"),
                exists: true,
                want: vec!["develop"],
            },
            Case {
                name: "staging target absent from the scan",
                rows: vec![],
                staging: Some("develop"),
                exists: false,
                want: vec![],
            },
            Case {
                name: "no staging target configured",
                rows: vec![],
                staging: None,
                exists: true,
                want: vec![],
            },
            Case {
                name: "custom staging name",
                rows: vec![],
                staging: Some("release"),
                exists: true,
                want: vec!["release"],
            },
            Case {
                name: "duplicates collapse",
                rows: vec![
                    based("claude/a", "feature/x"),
                    based("claude/b", "feature/x"),
                    based("claude/c", "develop"),
                ],
                staging: None,
                exists: true,
                want: vec!["develop", "feature/x"],
            },
        ];
        for case in cases {
            let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
            if let Some(name) = case.staging {
                fleet.staging = Some(StagingTarget(name.to_owned()));
            }
            let mut repo = scanned_repo(case.rows, &[], &["main"], &["main"]);
            if let Some(scan) = repo.snapshot.as_mut() {
                scan.locals = ["main", "develop", "feature/x", "release"]
                    .map(str::to_owned)
                    .to_vec();
                scan.landing.exists = case.exists;
            }
            fleet.repos.insert(id(R1), repo);
            assert_eq!(fleet.train_bases(&id(R1)), case.want, "{}", case.name);
        }
    }

    #[test]
    fn synced_follows_merge_branch_containment() {
        let t1 = FleetId("T1".to_owned());
        let mut fleet = bound();
        assert_eq!(fleet.merge_branch(&t1).as_deref(), Some(MERGE));
        assert_eq!(fleet.synced(&t1), Some(false), "not contained");
        let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
        let scan = repo.snapshot.as_mut().expect("scan");
        let merge_row = scan
            .rows
            .iter_mut()
            .find(|row| row.key == BranchKey::Branch(MERGE.to_owned()))
            .expect("merge row");
        merge_row.contained_in = Some("develop".to_owned());
        assert_eq!(fleet.synced(&t1), Some(false), "contained elsewhere");
        let repo = fleet.repos.get_mut(&id(R1)).expect("r1");
        let scan = repo.snapshot.as_mut().expect("scan");
        let merge_row = scan
            .rows
            .iter_mut()
            .find(|row| row.key == BranchKey::Branch(MERGE.to_owned()))
            .expect("merge row");
        merge_row.contained_in = Some("main".to_owned());
        assert_eq!(fleet.synced(&t1), Some(true), "target holds it");
        fleet.subscriptions.clear();
        assert_eq!(fleet.synced(&t1), None, "no binding");
    }

    #[test]
    fn program_of() {
        let cases = [
            (
                "bash-first-word",
                "Bash",
                Some("git -C /x status"),
                "Bash:git",
            ),
            (
                "bash-leading-space",
                "Bash",
                Some("  make audit"),
                "Bash:make",
            ),
            ("bash-empty", "Bash", Some(""), "Bash"),
            ("read", "Read", None, "Read"),
            (
                "mcp",
                "mcp__peek-mcp__session_get",
                Some("ignored"),
                "mcp__peek-mcp__session_get",
            ),
        ];
        for (label, tool, command, expected) in cases {
            assert_eq!(Program::of(tool, command).as_str(), expected, "{label}");
        }
    }

    #[test]
    fn terminal_sessions_beyond_the_cap_are_evicted_oldest_first() {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let max_terminal = fleet.caps.max_terminal();
        for n in 0..=max_terminal {
            let id = SessionId(format!("s{n:04}"));
            fleet
                .session_entry(&id, Timestamp::UNIX_EPOCH, false)
                .transition(
                    SessionState::Ended {
                        reason: "other".to_owned(),
                    },
                    Timestamp::UNIX_EPOCH,
                );
            fleet.retire(&id);
        }
        assert_eq!(fleet.sessions.len(), max_terminal);
        assert!(!fleet.sessions.contains_key(&SessionId("s0000".to_owned())));
    }
}
