//! The engine's input vocabulary. Every event carries its own timestamp, stamped by the edge that received it
//! (the ledger's own second-precision `ts` travels inside [`Denial`]); `apply` never reads a clock.
//!
//! Hook bodies parse leniently: only `session_id` and `hook_event_name` are required, an event heartbeat does not
//! model becomes [`HookKind::Other`] and unknown fields are ignored. `tool_input` stays raw JSON so later phases read
//! `file_path` / `command` without a second parse; `tool_response` is kept as unparsed text, parsed by the enforcement
//! core only for a call an enforced run anchors.

use std::path::PathBuf;
use std::sync::Arc;

use jiff::Timestamp;
use serde::Deserialize;
use serde_json::Value;
use serde_json::value::RawValue;

use crate::enforce::EnforceDoc;

use crate::fleet::{
    JournalEntry, MemberKey, Outcome, Requested, SensorName, SensorState, SubscribeRequest,
    SubscriptionSource,
};
use crate::ids::{
    AgentId, DigestId, FleetId, InstanceId, MemberId, MessagingToken, Pid, SessionId, WorkerBranch,
};
use crate::land::{LandEnd, LandSource, LandTarget};
use crate::peek::SpawnTree;
use crate::plan::PlannedWork;
use crate::repo::{RepoId, WorkItemKey};
use crate::runtime::{ActRequest, FindingId, RuntimeSubscribeRequest};
use crate::scan::{RepoScan, ScanError};

/// One input to [`crate::apply::apply`].
#[derive(Debug, Clone, PartialEq)]
pub enum Event {
    /// A hook body (HTTP hook, or the `SessionStart` command forwarder).
    Hook(HookEvent),
    /// A registry file `<claude home>/sessions/<pid>.json` was created or changed.
    RegistryUpserted {
        /// The parsed file.
        entry: RegistryEntry,
        /// When the edge read it.
        at: Timestamp,
    },
    /// A registry file disappeared. Informational: the process-exit watch decides the lifecycle.
    RegistryRemoved {
        /// The pid from the file name.
        pid: Pid,
        /// When the edge saw it gone.
        at: Timestamp,
    },
    /// A watched process exited.
    ProcessExited {
        /// The process.
        pid: Pid,
        /// When the exit was seen.
        at: Timestamp,
        /// How it was seen.
        sighting: ExitSighting,
    },
    /// One toolpolicy denial ledger record.
    Denied(Denial),
    /// An edge's health changed.
    SensorChanged {
        /// The edge.
        sensor: SensorName,
        /// Its new state.
        state: SensorState,
        /// Why, when degraded.
        detail: Option<String>,
        /// When it was observed.
        at: Timestamp,
    },
    /// The config edge re-read `<claude home>/heartbeat/config.json`; the engine takes the live keys.
    ConfigChanged {
        /// The parsed live keys (boxed: the largest payload by far).
        live: Box<crate::config::Live>,
        /// When the edge read the file.
        at: Timestamp,
    },
    /// The repos edge re-read the registry: every registered repo, canonical, with its registry name.
    ReposChanged {
        /// The registered set.
        repos: Vec<(RepoId, String)>,
        /// When the edge read the file.
        at: Timestamp,
    },
    /// A registered repo's refs or worktree HEADs changed (git watch edge; once per repo at startup).
    RefsChanged {
        /// The repository.
        repo: RepoId,
        /// When the edge saw it.
        at: Timestamp,
    },
    /// An agentwt scan finished.
    RepoScanned {
        /// The repository.
        repo: RepoId,
        /// When the scan started reading git, after the settle window — the coalescing reference (D3).
        started_at: Timestamp,
        /// Scans of this repo in flight at that start, this one included, as the runner's gauge read it.
        in_flight: u64,
        /// The scan, or why it failed.
        outcome: Result<Box<RepoScan>, ScanError>,
        /// When the result arrived.
        at: Timestamp,
    },
    /// One work item's plan files were re-read ("plan changed").
    PlanParsed {
        /// The repository.
        repo: RepoId,
        /// The plan dir.
        item: WorkItemKey,
        /// Its planned work; `None` when it holds no design file the default branch lacks.
        plan: Option<PlannedWork>,
        /// When the read finished.
        at: Timestamp,
    },
    /// A peek `session_events` call answered with a subscriber's spawn tree.
    PeekObserved {
        /// The subagents the buffer shows.
        tree: SpawnTree,
        /// Receipt time.
        at: Timestamp,
    },
    /// A session asked for digests, or the action ledger replayed its subscription.
    Subscribed {
        /// The shape-checked request.
        request: SubscribeRequest,
        /// Request or replay.
        source: SubscriptionSource,
        /// Receipt time, or the ledger record's time on replay.
        at: Timestamp,
    },
    /// The socket edge reported a digest's outcome, or the ledger replayed it.
    Delivered {
        /// The subscriber.
        session: SessionId,
        /// The digest.
        digest: DigestId,
        /// What happened.
        outcome: Outcome,
        /// Report time.
        at: Timestamp,
    },
    /// The session asked to end its subscription (`POST /unsubscribe`).
    UnsubscribeRequested {
        /// The subscriber.
        session: SessionId,
        /// The token it handed over at subscribe; must match.
        token: MessagingToken,
        /// Every joined member of the fleet leaves too (`unsubscribe --finish`).
        finish: bool,
        /// Receipt time.
        at: Timestamp,
    },
    /// A session joined a fleet (`POST /join`), or the ledger replayed its join.
    Joined {
        /// The session.
        session: SessionId,
        /// The fleet.
        fleet: FleetId,
        /// Its messaging token.
        token: MessagingToken,
        /// Request (admitted by the engine) or replay (trusted, with the recorded letter).
        source: JoinSource,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
    /// The ledger replayed a letter assigned to a worker.
    MemberAssigned {
        /// The fleet.
        fleet: FleetId,
        /// The member.
        key: MemberKey,
        /// Its letter.
        member: MemberId,
        /// The record's time.
        at: Timestamp,
    },
    /// A joined session reported done (`POST /done`), or the ledger replayed it.
    DoneReported {
        /// The session.
        session: SessionId,
        /// Its messaging token; `None` on replay.
        token: Option<MessagingToken>,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
    /// The ledger replayed a delivered member's resume.
    Resumed {
        /// The session.
        session: SessionId,
        /// The record's time.
        at: Timestamp,
    },
    /// An orchestrator recorded a journal entry (`POST /journal`), or the ledger replayed it.
    JournalRecorded {
        /// The fleet.
        fleet: FleetId,
        /// The entry; `by` is the recording session.
        entry: JournalEntry,
        /// The subscriber's token; `None` on replay.
        token: Option<MessagingToken>,
    },
    /// The fleet's subscriber registered a worker branch or changed its delivery (`POST /worker`), or the ledger
    /// replayed it.
    WorkerSet {
        /// The fleet.
        fleet: FleetId,
        /// The worker's branch.
        branch: WorkerBranch,
        /// Whether its work is reported finished.
        delivered: bool,
        /// The registering subscriber.
        by: SessionId,
        /// The subscriber's token; `None` on replay.
        token: Option<MessagingToken>,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
    /// The ledger replayed the start of a fleet run.
    FleetStarted {
        /// The fleet name.
        fleet: FleetId,
        /// The run's recorded id.
        instance: InstanceId,
        /// The record's time.
        at: Timestamp,
    },
    /// A joined session asked to leave its fleet (`POST /leave`).
    LeaveRequested {
        /// The session.
        session: SessionId,
        /// Its messaging token; must match the join.
        token: MessagingToken,
        /// Receipt time.
        at: Timestamp,
    },
    /// The ledger replayed a session leaving its fleet.
    Left {
        /// The session.
        session: SessionId,
        /// The record's time.
        at: Timestamp,
    },
    /// The operator asked to remove a lettered member (`POST /remove`).
    RemoveRequested {
        /// The fleet.
        fleet: FleetId,
        /// The member letter.
        member: MemberId,
        /// Receipt time.
        at: Timestamp,
    },
    /// The ledger replayed an operator removal.
    Removed {
        /// The fleet.
        fleet: FleetId,
        /// The member letter.
        member: MemberId,
        /// The record's time.
        at: Timestamp,
    },
    /// The ledger replayed a fleet run's dissolve; its archive file holds the rest.
    Dissolved {
        /// The fleet name.
        fleet: FleetId,
        /// The dissolved run.
        instance: InstanceId,
        /// The record's time.
        at: Timestamp,
    },
    /// The registry edge finished its startup scan: every live session is known.
    RegistrySeeded {
        /// When the scan finished.
        at: Timestamp,
    },
    /// The transcript edge read a subagent's transcript head.
    AgentModelRead {
        /// The root session.
        session: SessionId,
        /// The subagent.
        agent: AgentId,
        /// The first assistant line's `message.model`; `None` while the transcript holds none or cannot be read.
        model: Option<String>,
        /// What the spawn requested; `Unknown` without a readable `meta.json`.
        requested: Requested,
        /// When the read finished.
        at: Timestamp,
    },
    /// A session asked for the runtime subscription (`POST /runtime/subscribe`), or the ledger replayed it.
    RuntimeSubscribed {
        /// The shape-checked request.
        request: RuntimeSubscribeRequest,
        /// The action ledger replayed it: trusted, not recorded again.
        replay: bool,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
    /// The subscriber asked to end the runtime subscription (`POST /runtime/unsubscribe`).
    RuntimeUnsubscribeRequested {
        /// The subscriber.
        session: SessionId,
        /// The token it handed over at subscribe; must match.
        token: MessagingToken,
        /// Receipt time.
        at: Timestamp,
    },
    /// The subscriber asked for a move on a finding (`POST /runtime/act`).
    RuntimeActRequested {
        /// The request.
        request: ActRequest,
        /// Receipt time.
        at: Timestamp,
    },
    /// The socket edge reported a runtime digest's outcome.
    RuntimeDelivered {
        /// The digest.
        digest: DigestId,
        /// What happened.
        outcome: Outcome,
        /// Report time.
        at: Timestamp,
    },
    /// A move's claim is on disk: the move goes live.
    ClaimRecorded {
        /// The finding.
        finding: FindingId,
        /// Report time.
        at: Timestamp,
    },
    /// A move's claim could not be written: the move is void.
    ClaimFailed {
        /// The finding.
        finding: FindingId,
        /// Report time.
        at: Timestamp,
    },
    /// A hook answer carried a pending send (drained by the engine right after the runtime `Context` verdict).
    TemplateHandedOver {
        /// The finding.
        finding: FindingId,
        /// Answer time.
        at: Timestamp,
    },
    /// A session asked for a landing turn (`POST /done` without a join, `POST /land`), or the ledger replayed its
    /// request.
    LandRequested {
        /// The session.
        session: SessionId,
        /// The branch it asked for: a named one, or the landing target of the session's own branch.
        target: LandTarget,
        /// Its messaging token: stored on a new entry, checked against an existing one.
        token: MessagingToken,
        /// Request (admitted by the engine) or replay (trusted, with the recorded repo).
        source: LandSource,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
    /// A landing entry ended: the session's own end (`POST /land/end`), the operator's release
    /// (`POST /land/release`), or the ledger replayed an end.
    LandEnded {
        /// Whose entry, and how the end was asked.
        end: LandEnd,
        /// Receipt time, or the record's time.
        at: Timestamp,
    },
}

/// Where a `Joined` event comes from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum JoinSource {
    /// `POST /join`: admitted, assigned a letter, recorded.
    Request {
        /// The id a new fleet run gets when this join starts one; minted by the edge.
        candidate: InstanceId,
    },
    /// The action ledger: the recorded letter.
    Replay {
        /// The letter the join was assigned.
        member: MemberId,
    },
}

/// How a process exit was seen.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ExitSighting {
    /// kqueue reported `NOTE_EXIT` for a registered pid.
    Observed,
    /// Registration found the pid already gone (`ESRCH`).
    AlreadyGone,
}

/// A parsed hook body.
#[derive(Debug, Clone, PartialEq)]
pub struct HookEvent {
    /// `session_id`.
    pub session_id: SessionId,
    /// `agent_id` / `agent_type`: inside subagents and on every event of an `--agent` session.
    pub agent: Option<Agent>,
    /// `cwd` at the time of the event.
    pub cwd: Option<PathBuf>,
    /// `transcript_path`: lies under the claude home the session belongs to.
    pub transcript_path: Option<PathBuf>,
    /// When the edge received the body.
    pub at: Timestamp,
    /// The event and its event-specific fields.
    pub kind: HookKind,
    /// `tool_response` as raw JSON text, unparsed: the enforcement core parses it only for an anchored call.
    pub response: Option<Arc<str>>,
    /// The deployed enforce declaration of the skill this hook opens a run of; attached by the hook edge.
    pub enforce: Option<Arc<EnforceDoc>>,
    /// The session switched heartbeat off (`#disable heartbeat`); attached by the hook edge, read by the engine.
    pub muted: bool,
    /// The `background_tasks` entries of a `Stop` or `SubagentStop` body — work still in flight when the agent ended
    /// its turn; 0 on every other event and on a body without the key.
    pub in_flight: usize,
}

/// The agent an event came from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Agent {
    /// `agent_id`.
    pub id: AgentId,
    /// `agent_type`; empty for internal agents.
    pub agent_type: String,
}

/// The hook events heartbeat models; everything else is [`HookKind::Other`].
#[derive(Debug, Clone, PartialEq)]
pub enum HookKind {
    /// `SessionStart` with `source` (`startup`, `resume`, `clear`, `compact`, `fork`).
    SessionStart {
        /// `source`.
        source: String,
    },
    /// `SessionEnd` with `reason`.
    SessionEnd {
        /// `reason`.
        reason: String,
    },
    /// `UserPromptSubmit` with `prompt`.
    UserPromptSubmit {
        /// `prompt`.
        prompt: String,
    },
    /// `PreToolUse`.
    PreToolUse(ToolCall),
    /// `PostToolUse`: the call ran.
    PostToolUse(ToolCall),
    /// `PostToolUseFailure`: the call ran and failed.
    PostToolUseFailure(ToolCall),
    /// `PermissionRequest`: a permission dialog is about to show.
    PermissionRequest(ToolCall),
    /// `Notification` with `notification_type`.
    Notification {
        /// `notification_type`.
        notification_type: String,
    },
    /// `Stop`: the root agent finished its turn.
    Stop,
    /// `StopFailure` with `error`: the turn ended on an API error.
    StopFailure {
        /// `error`.
        error: String,
    },
    /// `SubagentStart` (agent on [`HookEvent::agent`]).
    SubagentStart,
    /// `SubagentStop` (agent on [`HookEvent::agent`]).
    SubagentStop,
    /// Any other `hook_event_name`.
    Other {
        /// `hook_event_name`.
        name: String,
    },
}

/// A tool call of a tool event.
#[derive(Debug, Clone, PartialEq)]
pub struct ToolCall {
    /// `tool_name`.
    pub tool_name: String,
    /// `tool_input`, raw; `Null` when absent.
    pub input: Value,
}

/// Registry file fields heartbeat reads (camelCase on disk).
#[derive(Debug, Clone, PartialEq, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegistryEntry {
    /// `pid`.
    pub pid: Pid,
    /// `sessionId`.
    pub session_id: SessionId,
    /// `cwd` at session start.
    pub cwd: PathBuf,
    /// `name`: the session title.
    #[serde(default)]
    pub name: Option<String>,
    /// `status`: `busy`, `idle` or `waiting`.
    #[serde(default)]
    pub status: Option<String>,
    /// `waitingFor`, e.g. `permission prompt`; present only while waiting.
    #[serde(default)]
    pub waiting_for: Option<String>,
    /// `statusUpdatedAt`: epoch milliseconds of the last `status` change.
    #[serde(default)]
    pub status_updated_at: Option<i64>,
    /// `messagingSocketPath`: the session's messaging socket (`/tmp/cc-socks-<uid>/<pid>.sock`).
    #[serde(default)]
    pub messaging_socket_path: Option<PathBuf>,
}

/// One denial ledger record (cmd/toolpolicy/ledger.go:27-37).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Denial {
    /// `session_id`.
    pub session_id: SessionId,
    /// `tool`.
    pub tool: String,
    /// `input`: the full Bash command, or the host, skill, subagent type, MCP tool or path.
    pub input: String,
    /// `candidate_rule`.
    pub candidate_rule: String,
    /// `ts`: the start of the second the denial happened in.
    pub at: Timestamp,
}

/// The hook body fields heartbeat reads.
#[derive(Deserialize)]
struct HookInput {
    session_id: SessionId,
    hook_event_name: String,
    #[serde(default)]
    transcript_path: Option<PathBuf>,
    #[serde(default)]
    cwd: Option<PathBuf>,
    #[serde(default)]
    agent_id: Option<AgentId>,
    #[serde(default)]
    agent_type: Option<String>,
    #[serde(default)]
    tool_name: Option<String>,
    #[serde(default)]
    tool_input: Option<Value>,
    #[serde(default)]
    tool_response: Option<Box<RawValue>>,
    #[serde(default)]
    source: Option<String>,
    #[serde(default)]
    reason: Option<String>,
    #[serde(default)]
    notification_type: Option<String>,
    #[serde(default)]
    prompt: Option<String>,
    #[serde(default)]
    error: Option<String>,
    /// `background_tasks` of a `Stop` or `SubagentStop` body: one object per in-flight task; only its length is read.
    #[serde(default)]
    background_tasks: Vec<Value>,
}

/// The ledger line fields heartbeat reads.
#[derive(Deserialize)]
struct DenialLine {
    session_id: SessionId,
    tool: String,
    input: String,
    candidate_rule: String,
    ts: Timestamp,
}

impl HookEvent {
    /// Parses one hook body received at `at`.
    pub fn parse(body: &[u8], at: Timestamp) -> crate::Result<HookEvent> {
        let input: HookInput = serde_json::from_slice(body)?;
        let agent = input.agent_id.map(|id| Agent {
            id,
            agent_type: input.agent_type.unwrap_or_default(),
        });
        let call = ToolCall {
            tool_name: input.tool_name.unwrap_or_default(),
            input: input.tool_input.unwrap_or(Value::Null),
        };
        let kind = match input.hook_event_name.as_str() {
            "SessionStart" => HookKind::SessionStart {
                source: input.source.unwrap_or_default(),
            },
            "SessionEnd" => HookKind::SessionEnd {
                reason: input.reason.unwrap_or_default(),
            },
            "UserPromptSubmit" => HookKind::UserPromptSubmit {
                prompt: input.prompt.unwrap_or_default(),
            },
            "PreToolUse" => HookKind::PreToolUse(call),
            "PostToolUse" => HookKind::PostToolUse(call),
            "PostToolUseFailure" => HookKind::PostToolUseFailure(call),
            "PermissionRequest" => HookKind::PermissionRequest(call),
            "Notification" => HookKind::Notification {
                notification_type: input.notification_type.unwrap_or_default(),
            },
            "Stop" => HookKind::Stop,
            "StopFailure" => HookKind::StopFailure {
                error: input.error.unwrap_or_default(),
            },
            "SubagentStart" => HookKind::SubagentStart,
            "SubagentStop" => HookKind::SubagentStop,
            other => HookKind::Other {
                name: other.to_owned(),
            },
        };
        Ok(HookEvent {
            session_id: input.session_id,
            agent,
            cwd: input.cwd,
            transcript_path: input.transcript_path,
            at,
            kind,
            response: input.tool_response.map(|raw| Arc::from(raw.get())),
            enforce: None,
            muted: false,
            in_flight: input.background_tasks.len(),
        })
    }
}

impl HookKind {
    /// The `hook_event_name` this kind was parsed from.
    #[must_use]
    pub fn name(&self) -> &str {
        match self {
            HookKind::SessionStart { .. } => "SessionStart",
            HookKind::SessionEnd { .. } => "SessionEnd",
            HookKind::UserPromptSubmit { .. } => "UserPromptSubmit",
            HookKind::PreToolUse(_) => "PreToolUse",
            HookKind::PostToolUse(_) => "PostToolUse",
            HookKind::PostToolUseFailure(_) => "PostToolUseFailure",
            HookKind::PermissionRequest(_) => "PermissionRequest",
            HookKind::Notification { .. } => "Notification",
            HookKind::Stop => "Stop",
            HookKind::StopFailure { .. } => "StopFailure",
            HookKind::SubagentStart => "SubagentStart",
            HookKind::SubagentStop => "SubagentStop",
            HookKind::Other { name } => name,
        }
    }
}

impl ToolCall {
    /// The Bash `command` of the call; `None` for other tools.
    #[must_use]
    pub fn command(&self) -> Option<&str> {
        self.input.get("command").and_then(Value::as_str)
    }
}

impl Denial {
    /// Parses one ledger line.
    pub fn parse(line: &[u8]) -> crate::Result<Denial> {
        let raw: DenialLine = serde_json::from_slice(line)?;
        Ok(Denial {
            session_id: raw.session_id,
            tool: raw.tool,
            input: raw.input,
            candidate_rule: raw.candidate_rule,
            at: raw.ts,
        })
    }
}

impl Event {
    /// When the edge received or observed the event.
    #[must_use]
    pub fn at(&self) -> Timestamp {
        match self {
            Event::Hook(hook) => hook.at,
            Event::Denied(denial) => denial.at,
            Event::RegistryUpserted { at, .. }
            | Event::RegistryRemoved { at, .. }
            | Event::ProcessExited { at, .. }
            | Event::SensorChanged { at, .. }
            | Event::RefsChanged { at, .. }
            | Event::RepoScanned { at, .. }
            | Event::PlanParsed { at, .. }
            | Event::PeekObserved { at, .. }
            | Event::Subscribed { at, .. }
            | Event::Delivered { at, .. }
            | Event::UnsubscribeRequested { at, .. }
            | Event::Joined { at, .. }
            | Event::MemberAssigned { at, .. }
            | Event::DoneReported { at, .. }
            | Event::Resumed { at, .. }
            | Event::FleetStarted { at, .. }
            | Event::LeaveRequested { at, .. }
            | Event::Left { at, .. }
            | Event::RemoveRequested { at, .. }
            | Event::Removed { at, .. }
            | Event::Dissolved { at, .. }
            | Event::RegistrySeeded { at }
            | Event::AgentModelRead { at, .. }
            | Event::RuntimeSubscribed { at, .. }
            | Event::RuntimeUnsubscribeRequested { at, .. }
            | Event::RuntimeActRequested { at, .. }
            | Event::RuntimeDelivered { at, .. }
            | Event::ClaimRecorded { at, .. }
            | Event::ClaimFailed { at, .. }
            | Event::TemplateHandedOver { at, .. }
            | Event::LandRequested { at, .. }
            | Event::LandEnded { at, .. }
            | Event::ConfigChanged { at, .. }
            | Event::ReposChanged { at, .. }
            | Event::WorkerSet { at, .. } => *at,
            Event::JournalRecorded { entry, .. } => entry.at,
        }
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    const REPLAY: &str = include_str!("../tests/fixtures/session.jsonl");

    #[test]
    fn every_replay_line_parses_to_its_kind() {
        for line in REPLAY.lines() {
            let record: Value = serde_json::from_str(line).expect("replay line");
            let body = serde_json::to_vec(&record["body"]).expect("body");
            let event = HookEvent::parse(&body, Timestamp::UNIX_EPOCH).expect("hook");
            assert_eq!(
                event.kind.name(),
                record["body"]["hook_event_name"],
                "{line}"
            );
        }
    }

    #[test]
    fn unknown_event_is_other() {
        let body = br#"{"session_id":"s","hook_event_name":"PreCompact","trigger":"auto"}"#;
        let event = HookEvent::parse(body, Timestamp::UNIX_EPOCH).expect("parse");
        assert_eq!(
            event.kind,
            HookKind::Other {
                name: "PreCompact".to_owned()
            }
        );
    }

    #[test]
    fn missing_session_id_is_an_error() {
        assert!(HookEvent::parse(br#"{"hook_event_name":"Stop"}"#, Timestamp::UNIX_EPOCH).is_err());
    }

    #[test]
    fn stop_failure_reads_error() {
        let body = br#"{"session_id":"s","hook_event_name":"StopFailure","error":"rate_limit"}"#;
        let event = HookEvent::parse(body, Timestamp::UNIX_EPOCH).expect("parse");
        assert_eq!(
            event.kind,
            HookKind::StopFailure {
                error: "rate_limit".to_owned()
            }
        );
    }

    #[test]
    fn parses_background_tasks() {
        let cases: [(&str, &[u8], usize); 3] = [
            (
                "stop-with-two-tasks",
                br#"{"session_id":"s","hook_event_name":"Stop","stop_hook_active":false,"background_tasks":[{"id":"t1","type":"local_workflow","status":"running","description":"refute"},{"id":"t2","type":"local_bash","status":"running","description":"sleep"}]}"#,
                2,
            ),
            (
                "stop-without-the-key",
                br#"{"session_id":"s","hook_event_name":"Stop"}"#,
                0,
            ),
            (
                "subagent-stop-with-one-task",
                br#"{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"a1","background_tasks":[{"id":"t1","type":"local_agent","status":"running","description":"explore"}]}"#,
                1,
            ),
        ];
        for (label, body, want) in cases {
            let event = HookEvent::parse(body, Timestamp::UNIX_EPOCH).expect(label);
            assert_eq!(event.in_flight, want, "{label}");
        }
    }

    #[test]
    fn live_registry_shape_parses() {
        let bytes = br#"{"pid":46441,"sessionId":"abc","cwd":"/r","startedAt":1,"kind":"interactive","name":"CC,FD - T1","status":"waiting","waitingFor":"permission prompt","statusUpdatedAt":1790518804638,"formerNames":[],"messagingSocketPath":"/tmp/cc-socks-502/46441.sock"}"#;
        let entry: RegistryEntry = serde_json::from_slice(bytes).expect("parse");
        assert_eq!(entry.waiting_for.as_deref(), Some("permission prompt"));
        assert_eq!(entry.status_updated_at, Some(1_790_518_804_638));
        assert_eq!(
            entry.messaging_socket_path,
            Some(PathBuf::from("/tmp/cc-socks-502/46441.sock"))
        );
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn at_reads_every_variant() {
        use crate::fleet::SubscribeRequest;
        use crate::ids::{FleetId, MessagingToken};
        use crate::repo::RepoId;

        let at = |second: i64| Timestamp::from_second(second).expect("timestamp");
        let session = SessionId("s".to_owned());
        let repo = RepoId(PathBuf::from("/r"));
        let candidate = InstanceId::legacy(&FleetId("T1".to_owned()), at(0));
        let request = SubscribeRequest {
            session: session.clone(),
            fleet: FleetId("T1".to_owned()),
            target: "main".to_owned(),
            socket: PathBuf::from("/tmp/s.sock"),
            token: MessagingToken("t".to_owned()),
            outside_collisions: false,
        };
        let events = vec![
            Event::Hook(HookEvent {
                session_id: session.clone(),
                agent: None,
                cwd: None,
                transcript_path: None,
                at: at(1),
                kind: HookKind::Stop,
                response: None,
                enforce: None,
                muted: false,
                in_flight: 0,
            }),
            Event::RegistryUpserted {
                entry: RegistryEntry {
                    pid: Pid(1),
                    session_id: session.clone(),
                    cwd: PathBuf::from("/r"),
                    name: None,
                    status: None,
                    waiting_for: None,
                    status_updated_at: None,
                    messaging_socket_path: None,
                },
                at: at(2),
            },
            Event::RegistryRemoved {
                pid: Pid(1),
                at: at(3),
            },
            Event::ProcessExited {
                pid: Pid(1),
                at: at(4),
                sighting: ExitSighting::Observed,
            },
            Event::Denied(Denial {
                session_id: session.clone(),
                tool: "Bash".to_owned(),
                input: "ls".to_owned(),
                candidate_rule: "Bash(ls *)".to_owned(),
                at: at(5),
            }),
            Event::SensorChanged {
                sensor: SensorName::Peek,
                state: SensorState::Ok,
                detail: None,
                at: at(6),
            },
            Event::RefsChanged {
                repo: repo.clone(),
                at: at(7),
            },
            Event::RepoScanned {
                repo: repo.clone(),
                started_at: at(0),
                in_flight: 1,
                outcome: Err(ScanError::Task("cancelled".to_owned())),
                at: at(8),
            },
            Event::PlanParsed {
                repo,
                item: WorkItemKey {
                    root: PathBuf::from("/r/.claude/worktrees/a"),
                    plan_dir: "2026-09-27-x-wip-abc1234".to_owned(),
                },
                plan: None,
                at: at(9),
            },
            Event::PeekObserved {
                tree: SpawnTree {
                    session: session.clone(),
                    subagents: Vec::new(),
                },
                at: at(10),
            },
            Event::Subscribed {
                request,
                source: SubscriptionSource::Request { candidate },
                at: at(11),
            },
            Event::Delivered {
                session: session.clone(),
                digest: DigestId(1),
                outcome: Outcome::Sent,
                at: at(12),
            },
            Event::UnsubscribeRequested {
                session: session.clone(),
                token: MessagingToken("t".to_owned()),
                finish: false,
                at: at(13),
            },
            Event::Joined {
                session: session.clone(),
                fleet: FleetId("T1".to_owned()),
                token: MessagingToken("t".to_owned()),
                source: JoinSource::Request { candidate },
                at: at(14),
            },
            Event::MemberAssigned {
                fleet: FleetId("T1".to_owned()),
                key: (session.clone(), None),
                member: crate::ids::MemberId::nth(0),
                at: at(15),
            },
            Event::DoneReported {
                session: session.clone(),
                token: None,
                at: at(16),
            },
            Event::Resumed {
                session: session.clone(),
                at: at(17),
            },
            Event::JournalRecorded {
                fleet: FleetId("T1".to_owned()),
                entry: crate::fleet::JournalEntry {
                    kind: crate::fleet::JournalKind::Note,
                    members: Vec::new(),
                    subject: "s".to_owned(),
                    detail: None,
                    by: session.clone(),
                    at: at(18),
                },
                token: None,
            },
            Event::FleetStarted {
                fleet: FleetId("T1".to_owned()),
                instance: candidate,
                at: at(19),
            },
            Event::LeaveRequested {
                session: session.clone(),
                token: MessagingToken("t".to_owned()),
                at: at(20),
            },
            Event::Left {
                session: session.clone(),
                at: at(21),
            },
            Event::Dissolved {
                fleet: FleetId("T1".to_owned()),
                instance: candidate,
                at: at(22),
            },
            Event::RegistrySeeded { at: at(23) },
            Event::RemoveRequested {
                fleet: FleetId("T1".to_owned()),
                member: crate::ids::MemberId::nth(0),
                at: at(24),
            },
            Event::Removed {
                fleet: FleetId("T1".to_owned()),
                member: crate::ids::MemberId::nth(0),
                at: at(25),
            },
            Event::AgentModelRead {
                session: session.clone(),
                agent: AgentId("a1".to_owned()),
                model: None,
                requested: Requested::Unknown,
                at: at(26),
            },
            Event::RuntimeSubscribed {
                request: crate::runtime::RuntimeSubscribeRequest {
                    session: session.clone(),
                    target: SessionId("watched".to_owned()),
                    mode: crate::runtime::Mode::Act,
                    socket: PathBuf::from("/tmp/s.sock"),
                    token: MessagingToken("t".to_owned()),
                },
                replay: false,
                at: at(27),
            },
            Event::RuntimeUnsubscribeRequested {
                session: session.clone(),
                token: MessagingToken("t".to_owned()),
                at: at(28),
            },
            Event::RuntimeActRequested {
                request: ActRequest {
                    session,
                    token: MessagingToken("t".to_owned()),
                    finding: FindingId("f".to_owned()),
                    action: crate::runtime::Move::Send,
                },
                at: at(29),
            },
            Event::RuntimeDelivered {
                digest: DigestId(1),
                outcome: Outcome::Sent,
                at: at(30),
            },
            Event::ClaimRecorded {
                finding: FindingId("f".to_owned()),
                at: at(31),
            },
            Event::ClaimFailed {
                finding: FindingId("f".to_owned()),
                at: at(32),
            },
            Event::TemplateHandedOver {
                finding: FindingId("f".to_owned()),
                at: at(33),
            },
            Event::LandRequested {
                session: SessionId("s".to_owned()),
                target: crate::land::LandTarget::OfSession,
                token: MessagingToken("t".to_owned()),
                source: crate::land::LandSource::Request,
                at: at(34),
            },
            Event::LandEnded {
                end: crate::land::LandEnd::Typed {
                    session: SessionId("s".to_owned()),
                    token: MessagingToken("t".to_owned()),
                    outcome: crate::land::TypedEnd::Landed,
                },
                at: at(35),
            },
            Event::WorkerSet {
                fleet: FleetId("T1".to_owned()),
                branch: WorkerBranch("claude/t1-a".to_owned()),
                delivered: false,
                by: SessionId("o".to_owned()),
                token: None,
                at: at(36),
            },
        ];
        for (second, event) in (1..).zip(&events) {
            assert_eq!(event.at(), at(second), "{event:?}");
        }
    }

    #[test]
    fn real_ledger_line_parses() {
        let line = br#"{"agent_id":"a306e32f1a51e091c","agent_type":"general-purpose","candidate_rule":"Bash(git *)","cause":"no allow rule matches","cwd":"/w","input":"git -C /w check-ignore -v target","session_id":"b96ac8a3-f0fd-449c-a743-8af8415c9dd8","ts":"2026-09-27T14:58:11Z","tool":"Bash"}"#;
        let denial = Denial::parse(line).expect("parse");
        assert_eq!(denial.candidate_rule, "Bash(git *)");
        assert_eq!(denial.at.to_string(), "2026-09-27T14:58:11Z");
    }
}
