//! Runtime enforcement: enforced skill runs of observed sessions, the findings they raise, the one runtime subscriber
//! and the moves it may make. The subscriber's own session is never enforced. A finding unlocks one move, once, while
//! its run is open.
//!
//! Runs (D3, D5): a hook that opens a run carries the skill's deployed declaration ([`crate::enforce::EnforceDoc`],
//! attached by the hook edge). A root `UserPromptSubmit` slash command ends every open run of its session and opens
//! one when its skill enforces; a root `Skill` call opens one nested inside the innermost open run; a non-compact
//! `SessionStart` and a terminal session end every open run. Runs exist whether or not anyone subscribes.
//!
//! Checks (D5, D6): every applied hook gets a `seq`; a successful `PostToolUse` of an anchored call joins the run's
//! anchor log with its captures. A root or subagent `PreToolUse` runs the provenance, requires and phase checks, the
//! root's `Stop` the stop checks. The verdict is decided while the hook is applied and kept in [`Runtime::decision`]
//! for [`verdict`], which answers the same hook. Every open run judges every guarded check and counts the ones that
//! held in [`Run::passes`]; the first failure, outermost run first, decides and alone climbs the ladder.
//!
//! Ladder (D8, D9): every failure of a check in a run below `kill_at` (config) denies the call (or blocks the Stop) with
//! its reason and raises a `check_failed` finding whose move is its recovery template — the runtime agent's recovery
//! step. The `kill_at`th failure kills the run and answers its own call with the kill's stop; every agent of the session then
//! answers `continue: false` until the human's next root prompt. That prompt also resets the session's open runs
//! (`killed` false, no failures); a prompt that lifts no kill resets nothing.
//!
//! In-flight Stops: a root `Stop` whose body lists `background_tasks` — a background Workflow, agent or shell the root
//! is waiting on — is not judged; the log stays, and the next root `Stop` with nothing in flight is.
//!
//! Blocked Stops: a root `Stop` the runtime blocks keeps the turn open — [`is_blocked_root_stop`] tells `apply` to leave
//! the session running.
//!
//! Judgment points (D10): a successful anchored call raises a `judgment` finding with its evidence while a subscriber
//! is active.
//!
//! Moves: `on_act` validates a request and asks for a durable claim ([`Action::Claim`]); only its `ClaimRecorded` makes
//! the move live — a send waits in `pending_sends` until [`verdict`] hands it over as the target agent's next
//! successful `PostToolUse` context, a stop arms (`armed`) until the root's next prompt (root) or the subagent's
//! `SubagentStop`. A failed claim voids the move, and a run's end voids its sends not yet handed over.
//!
//! Digests: findings become digest lines (at most `runtime_digest_lines` shown); one digest is flushed when the
//! subscriber is idle or blocked and none is in flight — the fleet rule applied to the one subscriber.
//!
//! Pure, like `apply`: no clock, no I/O; every effect leaves as an [`Action`].

use std::collections::{BTreeMap, VecDeque};
use std::fmt;
use std::fmt::Write as _;
use std::path::PathBuf;
use std::sync::Arc;

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use crate::action::{Action, RuntimeDigest};
use crate::action_ledger::Record;
use crate::apply::slash_skill;
use crate::attack::{ContextEvent, Probe, ProbedHook, Verdict};
use crate::config::{Caps, Texts};
use crate::enforce::{self, AnchorLog, EnforceDoc, Failure};
use crate::event::{HookEvent, HookKind, ToolCall};
use crate::fleet::{Fleet, LastDigest, Outcome, SessionState};
use crate::ids::{AgentId, DigestId, MessagingToken, SessionId};
use crate::protocol::{COMPACT_SOURCE, RUNTIME_SCHEMA, SKILL_TOOL};
use crate::wire::{
    ActedWire, AnchorHitRow, CheckFailureRow, CheckPassRow, DigestOutcomeWire, FindingKindWire,
    FindingRow, Health, ModeWire, MoveWire, RunRow, RunStateWire, RuntimeDigestRow,
    RuntimeDocument, RuntimeSubscriptionRow,
};

/// A session and, for a subagent, its agent: whom a move targets.
pub type AgentKey = (SessionId, Option<AgentId>);

/// A run id, unique for this daemon's lifetime.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct RunId(pub u64);

impl fmt::Display for RunId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "r{}", self.0)
    }
}

/// A finding id: `<run>-<check or judgment>-<n>`, minted in the core (D7).
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct FindingId(pub String);

/// Report or act, fixed at subscribe.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Mode {
    /// Findings are reported; no move is allowed.
    Report,
    /// Findings may be answered with their unlocked move.
    Act,
}

/// A move the act agent may ask for.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Move {
    /// Hand the finding's template, rendered by heartbeat, to the agent that raised it.
    Send,
    /// Stop the agent that raised the finding.
    Stop,
}

/// What raised a finding.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FindingKind {
    /// A check failed without killing its run; the move is the recovery step.
    CheckFailed,
    /// A judgment point's anchor succeeded.
    Judgment,
}

impl FindingKind {
    /// The wire spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            FindingKind::CheckFailed => "check_failed",
            FindingKind::Judgment => "judgment",
        }
    }
}

/// What happened to a finding's unlock.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Acted {
    /// Nothing asked yet.
    Open,
    /// A move was accepted; its claim is not on disk yet.
    InFlight(Move),
    /// The claim is on disk: a send waits for its hand-over, or a stop is armed.
    Live(Move),
    /// The send was handed over, or the armed stop was cleared.
    Done(Move),
    /// The move never took effect: its claim failed, or its run ended before the hand-over.
    Void(String),
}

/// Why a run ended.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EndReason {
    /// A new root slash command.
    Superseded,
    /// The session ended, died or restarted.
    SessionEnded,
}

impl EndReason {
    /// The document's spelling.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            EndReason::Superseded => "superseded",
            EndReason::SessionEnded => "session_ended",
        }
    }
}

/// One enforced skill run of an observed session.
#[derive(Debug, Clone, PartialEq)]
pub struct Run {
    /// The id.
    pub id: RunId,
    /// The observed session.
    pub session: SessionId,
    /// The skill leaf.
    pub skill: String,
    /// The deployed version the run started under.
    pub version: String,
    /// The run this one is nested in.
    pub parent: Option<RunId>,
    /// Why it ended; `None` while open.
    pub ended: Option<EndReason>,
    /// A check failed `kill_at` times: the session's agents are stopped until the next root prompt, which resets the run.
    pub killed: bool,
    /// The declaration.
    pub doc: Arc<EnforceDoc>,
    /// Successful anchored calls and their captures.
    pub log: AnchorLog,
    /// Failures per check.
    pub failures: BTreeMap<String, u8>,
    /// Passes per check: hooks on which the check's guard matched and it held.
    pub passes: BTreeMap<String, u32>,
    /// Findings raised.
    pub findings: u32,
}

impl Run {
    /// No end yet.
    #[must_use]
    pub fn is_open(&self) -> bool {
        self.ended.is_none()
    }

    /// Counts one pass of every check in `passed`.
    fn count_passes(&mut self, passed: Vec<String>) {
        for check in passed {
            let count = self.passes.entry(check).or_insert(0);
            *count = count.saturating_add(1);
        }
    }
}

/// One finding with the move it unlocks and what became of it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Finding {
    /// The id.
    pub id: FindingId,
    /// The kind.
    pub kind: FindingKind,
    /// The run it was raised on.
    pub run: RunId,
    /// The observed session.
    pub session: SessionId,
    /// The subagent whose call raised it; `None` for the root.
    pub agent: Option<AgentId>,
    /// The skill leaf.
    pub skill: String,
    /// The deployed version.
    pub version: String,
    /// The check or judgment id.
    pub entry: String,
    /// The check's reason or the judgment's question.
    pub reason: String,
    /// What was compared or observed.
    pub evidence: Vec<String>,
    /// The template a send renders: the check's recovery or the judgment's send.
    pub template: String,
    /// What became of its unlock.
    pub acted: Acted,
}

impl Finding {
    /// The agent a move on this finding targets.
    #[must_use]
    pub fn key(&self) -> AgentKey {
        (self.session.clone(), self.agent.clone())
    }
}

/// A claimed send waiting for its hand-over: the target's next successful tool call, or a waiting root's Stop or next
/// prompt.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PendingSend {
    /// The finding it answers.
    pub finding: FindingId,
    /// The rendered template.
    pub text: String,
}

/// A validated `POST /runtime/subscribe` body (the socket's file type is checked by the HTTP edge).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuntimeSubscribeRequest {
    /// The subscriber session.
    pub session: SessionId,
    /// The one observed session: its id or exact title as asked, its id once admitted (and in the ledger).
    pub target: SessionId,
    /// Report or act.
    pub mode: Mode,
    /// Its messaging socket: where digests go.
    pub socket: PathBuf,
    /// Its messaging token: digests' auth, and the proof on unsubscribe and act.
    pub token: MessagingToken,
}

impl RuntimeSubscribeRequest {
    /// Shape checks: printable-ASCII session id and token of at most `max_field` bytes (`max_field`), a target of at
    /// most that length, absolute socket path.
    pub fn parse(
        session: &str,
        target: &str,
        mode: Mode,
        socket: PathBuf,
        token: MessagingToken,
        max_field: usize,
    ) -> Result<RuntimeSubscribeRequest, String> {
        if !is_printable(session, max_field) || !is_printable(&token.0, max_field) {
            return Err(format!(
                "session id and token must be 1-{max_field} printable ASCII characters"
            ));
        }
        let is_target_valid = !target.trim().is_empty()
            && target.len() <= max_field
            && !target.chars().any(char::is_control);
        if !is_target_valid {
            return Err(
                "target must name the one observed session: its id or exact title".to_owned(),
            );
        }
        if !socket.is_absolute() {
            return Err(format!("socket path {} is not absolute", socket.display()));
        }
        Ok(RuntimeSubscribeRequest {
            session: SessionId(session.to_owned()),
            target: SessionId(target.to_owned()),
            mode,
            socket,
            token,
        })
    }
}

/// 1-`max_field` printable ASCII characters.
#[must_use]
pub fn is_printable(text: &str, max_field: usize) -> bool {
    !text.is_empty() && text.len() <= max_field && text.bytes().all(|byte| byte.is_ascii_graphic())
}

/// A `POST /runtime/act` request.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ActRequest {
    /// The asking session.
    pub session: SessionId,
    /// Its token; must be the subscriber's.
    pub token: MessagingToken,
    /// The finding answered.
    pub finding: FindingId,
    /// The move asked for.
    pub action: Move,
}

/// One watcher: a subscriber session bound to exactly one observed session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuntimeSubscription {
    /// The subscriber session.
    pub session: SessionId,
    /// The one session it watches; only its findings reach this watcher and only they can be answered.
    pub target: SessionId,
    /// Report or act.
    pub mode: Mode,
    /// Its messaging socket.
    pub socket: PathBuf,
    /// Its messaging token.
    pub token: MessagingToken,
    /// When it subscribed.
    pub since: Timestamp,
    /// Findings not delivered yet.
    pub pending: Vec<FindingId>,
    /// The digest handed to the socket edge and not reported back.
    pub in_flight: Option<DigestId>,
    /// The last digest's outcome.
    pub last: Option<LastDigest>,
}

/// The runtime's hook answer; `handed_over` names the send the verdict carries, for the engine to apply.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Answer {
    /// The `/hook` answer.
    pub verdict: Verdict,
    /// The pending send this answer hands over.
    pub handed_over: Option<FindingId>,
}

/// The `/runtime/subscribe` and `/runtime/unsubscribe` read-back for one session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SubscriptionAnswer {
    /// The session holds the subscription.
    Held(Box<RuntimeSubscriptionRow>),
    /// The session's latest subscribe was refused.
    Refused(String),
    /// Neither.
    Absent,
}

/// The `/runtime/act` read-back for one finding.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ActAnswer {
    /// The latest request on the finding was refused.
    Refused(String),
    /// The move never took effect.
    Void(String),
    /// Accepted; the claim is not on disk yet.
    Claiming,
    /// The claim is on disk (or the move already finished): the finding as it stands.
    Settled(Box<FindingRow>),
}

/// The runtime state.
#[derive(Debug, Clone)]
pub struct Runtime {
    /// Active watchers by subscriber session; each watches one session, and a session has at most one watcher.
    pub subscriptions: BTreeMap<SessionId, RuntimeSubscription>,
    /// Runs by id; ended runs stay until the bounded history drops them (last `max_ended_runs`).
    pub runs: BTreeMap<RunId, Run>,
    /// Open run stack per session, outermost first.
    pub open: BTreeMap<SessionId, Vec<RunId>>,
    /// Findings by id.
    pub findings: BTreeMap<FindingId, Finding>,
    /// Armed stops: (session, agent) → the finding that armed it.
    pub armed: BTreeMap<AgentKey, FindingId>,
    /// Sessions whose run was killed: every agent is stopped until the next root prompt; the value is the reason.
    pub killed: BTreeMap<SessionId, String>,
    /// Claimed sends waiting for their hand-over (`PostToolUse`, or a waiting root's Stop or `UserPromptSubmit`).
    pub pending_sends: BTreeMap<AgentKey, PendingSend>,
    /// The verdict and prompt texts, from the heartbeat config; swapped whole on a live reload.
    pub texts: Texts,
    /// The verdict an enforce check decided while the current hook was applied, for [`verdict`].
    pub decision: Option<(AgentKey, Verdict)>,
    /// The apply order of hooks (D6).
    pub seq: u64,
    /// Next run id.
    pub next_run: RunId,
    /// Next finding number.
    pub next_finding: u64,
    /// Next runtime digest id.
    pub next_digest: DigestId,
    /// The latest refused subscribe, for the `/runtime/subscribe` answer.
    pub subscribe_refusal: Option<(SessionId, String)>,
    /// The latest refused act, for the `/runtime/act` answer.
    pub act_refusal: Option<(FindingId, String)>,
    /// Ended runs, oldest end first.
    ended: VecDeque<RunId>,
}

impl Default for Runtime {
    fn default() -> Runtime {
        Runtime::new(Texts::default())
    }
}

impl Runtime {
    /// An empty runtime.
    #[must_use]
    pub fn new(texts: Texts) -> Runtime {
        Runtime {
            subscriptions: BTreeMap::new(),
            runs: BTreeMap::new(),
            open: BTreeMap::new(),
            findings: BTreeMap::new(),
            armed: BTreeMap::new(),
            killed: BTreeMap::new(),
            pending_sends: BTreeMap::new(),
            texts,
            decision: None,
            seq: 0,
            next_run: RunId(1),
            next_finding: 1,
            next_digest: DigestId(1),
            subscribe_refusal: None,
            act_refusal: None,
            ended: VecDeque::new(),
        }
    }

    /// `session` is a watcher.
    #[must_use]
    pub fn is_subscriber(&self, session: &SessionId) -> bool {
        self.subscriptions.contains_key(session)
    }

    /// The watcher of `target`, when one is active.
    #[must_use]
    pub fn watcher_of(&self, target: &SessionId) -> Option<&RuntimeSubscription> {
        self.subscriptions
            .values()
            .find(|subscription| subscription.target == *target)
    }

    /// The `stopReason` for `key`: its session's kill, else the stop armed on it, rendered from its finding.
    #[must_use]
    pub fn armed_reason(&self, key: &AgentKey) -> Option<String> {
        if let Some(reason) = self.killed.get(&key.0) {
            return Some(reason.clone());
        }
        let finding_id = self.armed.get(key)?;
        let template = self.texts.stop_reason();
        let text = match self.findings.get(finding_id) {
            Some(finding) => fill(template, finding),
            None => template.replace("{finding}", &finding_id.0),
        };
        Some(text)
    }

    /// Records a refused act request.
    fn refuse(&mut self, finding: &FindingId, reason: &str) {
        self.act_refusal = Some((finding.clone(), reason.to_owned()));
    }

    /// Marks an accepted act request in flight.
    fn claim(&mut self, finding: &FindingId, action: Move) {
        if let Some(found) = self.findings.get_mut(finding) {
            found.acted = Acted::InFlight(action);
        }
    }

    /// Queues a finding for the next digest of its session's watcher; no watcher, no line.
    fn line(&mut self, finding: FindingId, session: &SessionId) {
        if let Some(subscription) = self
            .subscriptions
            .values_mut()
            .find(|subscription| subscription.target == *session)
        {
            subscription.pending.push(finding);
        }
    }
}

/// Runs `step` on the fleet with its runtime moved out beside it (split borrow), then moves the runtime back.
pub(crate) fn split<T>(fleet: &mut Fleet, step: impl FnOnce(&Fleet, &mut Runtime) -> T) -> T {
    let mut runtime = std::mem::take(&mut fleet.runtime);
    let out = step(fleet, &mut runtime);
    fleet.runtime = runtime;
    out
}

/// [`on_hook`] over the fleet's own runtime.
pub(crate) fn on_hook_split(fleet: &mut Fleet, hook: &HookEvent) -> Vec<Action> {
    split(fleet, |fleet, runtime| on_hook(fleet, runtime, hook))
}

/// Run boundaries, armed-stop clearing, the anchor log and the enforce checks of one hook (applied before the session
/// state machine). The decided verdict waits in [`Runtime::decision`] for [`verdict`].
pub(crate) fn on_hook(fleet: &Fleet, runtime: &mut Runtime, hook: &HookEvent) -> Vec<Action> {
    runtime.decision = None;
    runtime.seq = runtime.seq.saturating_add(1);
    clear_armed(runtime, hook);
    if runtime.is_subscriber(&hook.session_id) {
        return Vec::new();
    }
    let session = &hook.session_id;
    let host = fleet.sessions.get(session);
    let is_subagent = hook
        .agent
        .as_ref()
        .is_some_and(|agent| host.is_some_and(|host| host.subagents.contains_key(&agent.id)));
    let is_root = !is_subagent
        && (hook.agent.is_none() || host.is_some_and(crate::fleet::Session::is_turn_open));
    let agent = if is_root {
        None
    } else {
        hook.agent.as_ref().map(|agent| agent.id.clone())
    };
    let key = (session.clone(), agent);
    let caps = &fleet.caps;
    match &hook.kind {
        HookKind::SessionStart { source } if source != COMPACT_SOURCE => {
            end_session(runtime, caps, session, EndReason::SessionEnded);
        }
        HookKind::UserPromptSubmit { prompt } if is_root => {
            if slash_skill(prompt).is_some() {
                end_session(runtime, caps, session, EndReason::Superseded);
                if let Some(doc) = &hook.enforce {
                    start_run(runtime, session, Arc::clone(doc), false);
                }
            }
        }
        HookKind::PreToolUse(call) => {
            let opens = is_root && call.tool_name == SKILL_TOOL;
            if let Some(doc) = hook.enforce.as_ref().filter(|_| opens) {
                start_run(runtime, session, Arc::clone(doc), true);
            }
            pre_tool(runtime, caps, &key, is_root, call);
        }
        HookKind::PostToolUse(call) => {
            record(runtime, &key, is_root, call, hook.response.as_deref());
        }
        // A root that ends its turn with work in flight — the Stop body lists it — is waiting, not finishing: only a
        // Stop with nothing in flight is judged.
        HookKind::Stop if is_root => {
            if hook.in_flight == 0 {
                on_stop(runtime, caps, &key);
            }
        }
        HookKind::SessionStart { .. }
        | HookKind::UserPromptSubmit { .. }
        | HookKind::SessionEnd { .. }
        | HookKind::PostToolUseFailure(_)
        | HookKind::PermissionRequest(_)
        | HookKind::Notification { .. }
        | HookKind::Stop
        | HookKind::StopFailure { .. }
        | HookKind::SubagentStart
        | HookKind::SubagentStop
        | HookKind::Other { .. } => {}
    }
    Vec::new()
}

/// A root prompt clears the root's armed stop and the session's kill — a lifted kill also resets the session's open
/// runs (D2); a `SubagentStop` clears that subagent's stop.
fn clear_armed(runtime: &mut Runtime, hook: &HookEvent) {
    let key = match (&hook.kind, &hook.agent) {
        (HookKind::UserPromptSubmit { .. }, None) => {
            if runtime.killed.remove(&hook.session_id).is_some() {
                revive(runtime, &hook.session_id);
            }
            (hook.session_id.clone(), None)
        }
        (HookKind::SubagentStop, Some(agent)) => (hook.session_id.clone(), Some(agent.id.clone())),
        _ => return,
    };
    if let Some(finding) = runtime.armed.remove(&key)
        && let Some(found) = runtime.findings.get_mut(&finding)
    {
        found.acted = Acted::Done(Move::Stop);
    }
}

/// A lifted kill gives every open run of `session` a fresh budget: none reads killed, no check has failed (D2). Passes
/// and findings stay.
fn revive(runtime: &mut Runtime, session: &SessionId) {
    for id in open_runs(runtime, session) {
        if let Some(run) = runtime.runs.get_mut(&id) {
            run.killed = false;
            run.failures.clear();
        }
    }
}

/// The open runs of `session`, innermost last.
fn open_runs(runtime: &Runtime, session: &SessionId) -> Vec<RunId> {
    runtime.open.get(session).cloned().unwrap_or_default()
}

/// Provenance, requires and phase checks of every open run on one call: each run counts its passes; the first failure,
/// outermost run first, decides and is the only one escalated.
fn pre_tool(runtime: &mut Runtime, caps: &Caps, key: &AgentKey, is_root: bool, call: &ToolCall) {
    let mut decided: Option<(RunId, Failure)> = None;
    for id in open_runs(runtime, &key.0) {
        let Some(run) = runtime.runs.get_mut(&id) else {
            continue;
        };
        let evaluation =
            enforce::pre_tool(&run.doc, &run.log, is_root, &call.tool_name, &call.input);
        run.count_passes(evaluation.passed);
        decided = decided.or_else(|| evaluation.failure.map(|failure| (id, failure)));
    }
    if let Some((id, failure)) = decided {
        escalate(runtime, caps, id, key, failure, false);
    }
}

/// The runtime blocked the root `Stop` of `session` on the hook being applied: its turn goes on.
#[must_use]
pub(crate) fn is_blocked_root_stop(runtime: &Runtime, session: &SessionId) -> bool {
    matches!(&runtime.decision, Some(((decided, None), Verdict::Block { .. })) if decided == session)
}

/// Stop checks of every open run on the root's Stop: each run counts its passes; the first failure, outermost run
/// first, decides and is the only one escalated.
fn on_stop(runtime: &mut Runtime, caps: &Caps, key: &AgentKey) {
    let mut decided: Option<(RunId, Failure)> = None;
    for id in open_runs(runtime, &key.0) {
        let Some(run) = runtime.runs.get_mut(&id) else {
            continue;
        };
        let evaluation = enforce::on_stop(&run.doc, &run.log);
        run.count_passes(evaluation.passed);
        decided = decided.or_else(|| evaluation.failure.map(|failure| (id, failure)));
    }
    if let Some((id, failure)) = decided {
        escalate(runtime, caps, id, key, failure, true);
    }
}

/// A successful call joins every open run's anchor log; a matched judgment anchor raises a finding while the session
/// has a watcher.
fn record(
    runtime: &mut Runtime,
    key: &AgentKey,
    is_root: bool,
    call: &ToolCall,
    response: Option<&str>,
) {
    let seq = runtime.seq;
    let is_subscribed = runtime.watcher_of(&key.0).is_some();
    for id in open_runs(runtime, &key.0) {
        let Some(run) = runtime.runs.get_mut(&id) else {
            continue;
        };
        let doc = Arc::clone(&run.doc);
        let matched = run
            .log
            .record(&doc, seq, is_root, &call.tool_name, &call.input, response);
        if !is_subscribed {
            continue;
        }
        let points: Vec<(String, Vec<String>, String, String)> = doc
            .judgment
            .iter()
            .filter(|(_, point)| matched.contains(&point.anchor))
            .map(|(point_id, point)| {
                let evidence = enforce::evidence(point, &run.log, &call.input);
                (
                    point_id.clone(),
                    evidence,
                    point.question.clone(),
                    point.send.clone(),
                )
            })
            .collect();
        for (point_id, evidence, question, send) in points {
            raise(
                runtime,
                id,
                key,
                FindingKind::Judgment,
                point_id,
                question,
                evidence,
                send,
            );
        }
    }
}

/// The ladder: a check's failure below `caps.kill_at()` denies (or blocks) and raises its recovery finding; the
/// `kill_at`th kills the run and answers its own call with the kill's stop instead (D3), rendered from
/// `texts.kill_reason()`.
fn escalate(
    runtime: &mut Runtime,
    caps: &Caps,
    id: RunId,
    key: &AgentKey,
    failure: Failure,
    is_stop: bool,
) {
    let verdict = if is_stop {
        Verdict::Block {
            reason: failure.reason.clone(),
        }
    } else {
        Verdict::Deny {
            reason: failure.reason.clone(),
        }
    };
    runtime.decision = Some((key.clone(), verdict));
    let Some(run) = runtime.runs.get_mut(&id) else {
        return;
    };
    let count = run.failures.entry(failure.check.clone()).or_insert(0);
    *count = count.saturating_add(1);
    let kill_at = caps.kill_at();
    if *count >= kill_at {
        run.killed = true;
        let reason = runtime
            .texts
            .kill_reason()
            .replace("{id}", &id.to_string())
            .replace("{skill}", &run.skill)
            .replace("{check}", &failure.check)
            .replace("{kill_at}", &kill_at.to_string())
            .replace("{reason}", &failure.reason);
        runtime.killed.insert(key.0.clone(), reason.clone());
        let stop = Verdict::Stop {
            reason,
            pre_tool: !is_stop,
        };
        runtime.decision = Some((key.clone(), stop));
        return;
    }
    let recovery = run
        .doc
        .checks
        .get(&failure.check)
        .map(|check| check.recovery.clone())
        .unwrap_or_default();
    raise(
        runtime,
        id,
        key,
        FindingKind::CheckFailed,
        failure.check,
        failure.reason,
        failure.evidence,
        recovery,
    );
}

/// Stores a finding and queues it for the next digest.
#[allow(clippy::too_many_arguments)]
fn raise(
    runtime: &mut Runtime,
    run: RunId,
    key: &AgentKey,
    kind: FindingKind,
    entry: String,
    reason: String,
    evidence: Vec<String>,
    template: String,
) {
    let Some(found) = runtime.runs.get_mut(&run) else {
        return;
    };
    found.findings = found.findings.saturating_add(1);
    let id = FindingId(format!("{run}-{entry}-{}", runtime.next_finding));
    runtime.next_finding = runtime.next_finding.saturating_add(1);
    let finding = Finding {
        id: id.clone(),
        kind,
        run,
        session: key.0.clone(),
        agent: key.1.clone(),
        skill: found.skill.clone(),
        version: found.version.clone(),
        entry,
        reason,
        evidence,
        template,
        acted: Acted::Open,
    };
    runtime.findings.insert(id.clone(), finding);
    runtime.line(id, &key.0);
}

/// Opens a run under `doc`; `nested` puts it inside the innermost open run, if any.
fn start_run(runtime: &mut Runtime, session: &SessionId, doc: Arc<EnforceDoc>, nested: bool) {
    let id = runtime.next_run;
    runtime.next_run = RunId(id.0 + 1);
    let stack = runtime.open.entry(session.clone()).or_default();
    let parent = if nested { stack.last().copied() } else { None };
    stack.push(id);
    let run = Run {
        id,
        session: session.clone(),
        skill: doc.skill.clone(),
        version: doc.version.clone(),
        parent,
        ended: None,
        killed: false,
        doc,
        log: AnchorLog::default(),
        failures: BTreeMap::new(),
        passes: BTreeMap::new(),
        findings: 0,
    };
    runtime.runs.insert(id, run);
}

/// Ends every open run of `session`, innermost first.
fn end_session(runtime: &mut Runtime, caps: &Caps, session: &SessionId, reason: EndReason) {
    let Some(stack) = runtime.open.remove(session) else {
        return;
    };
    for id in stack.into_iter().rev() {
        end_run(runtime, caps.max_ended_runs(), id, reason);
    }
}

/// Ends one run: its sends not handed over are void, the history is capped at `max_ended_runs`.
fn end_run(runtime: &mut Runtime, max_ended_runs: usize, id: RunId, reason: EndReason) {
    let Some(run) = runtime.runs.get_mut(&id).filter(|run| run.is_open()) else {
        return;
    };
    run.ended = Some(reason);
    let voided: Vec<AgentKey> = runtime
        .pending_sends
        .iter()
        .filter(|(_, pending)| {
            runtime
                .findings
                .get(&pending.finding)
                .is_some_and(|finding| finding.run == id)
        })
        .map(|(key, _)| key.clone())
        .collect();
    for key in voided {
        if let Some(pending) = runtime.pending_sends.remove(&key)
            && let Some(finding) = runtime.findings.get_mut(&pending.finding)
        {
            finding.acted = Acted::Void("run ended before the next tool call".to_owned());
        }
    }
    runtime.ended.push_back(id);
    while runtime.ended.len() > max_ended_runs {
        let Some(oldest) = runtime.ended.pop_front() else {
            break;
        };
        runtime.runs.remove(&oldest);
        let armed = &runtime.armed;
        runtime.findings.retain(|finding_id, finding| {
            finding.run != oldest || armed.values().any(|arming| arming == finding_id)
        });
    }
}

/// Admits or refuses a subscribe; a replayed one is trusted.
pub(crate) fn on_subscribed(
    fleet: &Fleet,
    runtime: &mut Runtime,
    request: RuntimeSubscribeRequest,
    replay: bool,
    at: Timestamp,
) -> Vec<Action> {
    let mut request = request;
    if !replay {
        match admit(fleet, runtime, &request) {
            Ok(target) => request.target = target,
            Err(reason) => {
                runtime.subscribe_refusal = Some((request.session, reason));
                return Vec::new();
            }
        }
    }
    if runtime
        .subscribe_refusal
        .as_ref()
        .is_some_and(|(session, _)| *session == request.session)
    {
        runtime.subscribe_refusal = None;
    }
    let mut actions = Vec::new();
    if !replay {
        actions.push(Action::Record(Record::RuntimeSubscribed {
            session_id: request.session.clone(),
            target: Some(request.target.clone()),
            mode: request.mode,
            socket_path: request.socket.clone(),
            token: request.token.clone(),
        }));
    }
    let RuntimeSubscribeRequest {
        session,
        target,
        mode,
        socket,
        token,
    } = request;
    match runtime.subscriptions.get_mut(&session) {
        Some(held) if held.target == target => {
            held.mode = mode;
            held.socket = socket;
            held.token = token;
            held.since = at;
        }
        Some(_) | None => {
            runtime.subscriptions.insert(
                session.clone(),
                RuntimeSubscription {
                    session,
                    target,
                    mode,
                    socket,
                    token,
                    since: at,
                    pending: Vec::new(),
                    in_flight: None,
                    last: None,
                },
            );
        }
    }
    actions
}

/// The engine half of subscribe admission: the subscriber is known, alive and owns the socket; the target resolves to
/// one live session (id first, else exact title), is not the subscriber, not a watcher itself, and has no other
/// watcher. `Ok` is the target's id.
fn admit(
    fleet: &Fleet,
    runtime: &Runtime,
    request: &RuntimeSubscribeRequest,
) -> Result<SessionId, String> {
    let Some(known) = fleet.sessions.get(&request.session) else {
        return Err("unknown session: no hook or registry entry seen for it".to_owned());
    };
    if let Some(end) = terminal_word(&known.state) {
        return Err(format!("the session {end}"));
    }
    if known
        .socket
        .as_deref()
        .is_some_and(|registered| registered != request.socket)
    {
        return Err(format!(
            "{} is not the session's registered socket",
            request.socket.display()
        ));
    }
    let target = resolve_target(fleet, &request.target)?;
    if target == request.session {
        return Err("a session cannot watch itself".to_owned());
    }
    if let Some(end) = fleet
        .sessions
        .get(&target)
        .and_then(|session| terminal_word(&session.state))
    {
        return Err(format!("the target session {end}"));
    }
    if runtime.is_subscriber(&target) {
        return Err(format!(
            "{target} ({}) is a watcher itself",
            title_of(fleet, &target)
        ));
    }
    match runtime.watcher_of(&target) {
        Some(held) if held.session != request.session => Err(format!(
            "{target} is already watched by {} ({})",
            held.session,
            title_of(fleet, &held.session)
        )),
        Some(_) | None => Ok(target),
    }
}

/// The target as an id when a known session has it, else the one session whose title is exactly it.
fn resolve_target(fleet: &Fleet, asked: &SessionId) -> Result<SessionId, String> {
    if fleet.sessions.contains_key(asked) {
        return Ok(asked.clone());
    }
    let titled: Vec<&SessionId> = fleet
        .sessions
        .iter()
        .filter(|(_, session)| session.title.as_deref() == Some(asked.0.as_str()))
        .map(|(id, _)| id)
        .collect();
    match titled.as_slice() {
        [one] => Ok((*one).clone()),
        [] => Err(format!("no session with id or title {:?}", asked.0)),
        [_, ..] => Err(format!(
            "{} sessions are titled {:?}; name one by id",
            titled.len(),
            asked.0
        )),
    }
}

/// Ends `session`'s watch when it holds one with `token`; anything else changes nothing.
pub(crate) fn on_unsubscribe(
    runtime: &mut Runtime,
    session: &SessionId,
    token: &MessagingToken,
) -> Vec<Action> {
    let holds = runtime
        .subscriptions
        .get(session)
        .is_some_and(|held| held.token == *token);
    if !holds {
        return Vec::new();
    }
    end_subscription(runtime, session, "unsubscribed by the session")
}

/// Drops `session`'s watch and records it; enforced runs go on without a watcher.
fn end_subscription(runtime: &mut Runtime, session: &SessionId, reason: &str) -> Vec<Action> {
    let Some(held) = runtime.subscriptions.remove(session) else {
        return Vec::new();
    };
    vec![Action::Record(Record::RuntimeUnsubscribed {
        session_id: held.session,
        reason: reason.to_owned(),
    })]
}

/// Settles the watcher's in-flight digest; `Refused` ends that watch.
pub(crate) fn on_delivered(
    runtime: &mut Runtime,
    digest: DigestId,
    outcome: Outcome,
    at: Timestamp,
) -> Vec<Action> {
    runtime.next_digest = runtime.next_digest.max(DigestId(digest.0 + 1));
    let Some(held) = runtime
        .subscriptions
        .values_mut()
        .find(|held| held.in_flight == Some(digest))
    else {
        return Vec::new();
    };
    held.in_flight = None;
    let refused = match &outcome {
        Outcome::Refused(detail) => Some(detail.clone()),
        Outcome::Sent | Outcome::Unconfirmed(_) | Outcome::Unclaimed(_) => None,
    };
    held.last = Some(LastDigest {
        id: digest,
        outcome,
        at,
    });
    let session = held.session.clone();
    match refused {
        Some(detail) => end_subscription(runtime, &session, &format!("socket refused: {detail}")),
        None => Vec::new(),
    }
}

/// Validates one act request and, when every check passes, claims the finding and returns the move's durable claim.
/// The move itself goes live only on `ClaimRecorded` ([`on_claim_recorded`]). A refusal is kept on the runtime and
/// read back by `/runtime/act`; the finding stays as it was.
pub(crate) fn on_act(fleet: &Fleet, runtime: &mut Runtime, request: ActRequest) -> Vec<Action> {
    if runtime
        .act_refusal
        .as_ref()
        .is_some_and(|(id, _)| *id == request.finding)
    {
        runtime.act_refusal = None;
    }
    let key = match admit_act(fleet, runtime, &request) {
        Ok(key) => key,
        Err(reason) => {
            runtime.refuse(&request.finding, reason);
            return Vec::new();
        }
    };
    let record = Record::RuntimeClaimed {
        finding: request.finding.clone(),
        target: key.0,
        agent: key.1,
        action: request.action,
    };
    runtime.claim(&request.finding, request.action);
    vec![Action::Claim {
        finding: request.finding,
        record,
    }]
}

/// The act checks in order; `Ok` is the targeted agent.
fn admit_act(
    fleet: &Fleet,
    runtime: &Runtime,
    request: &ActRequest,
) -> Result<AgentKey, &'static str> {
    let Some(subscription) = runtime.subscriptions.get(&request.session) else {
        return Err("no runtime subscription for this session");
    };
    if subscription.token != request.token {
        return Err("only the runtime subscriber may act");
    }
    if subscription.mode == Mode::Report {
        return Err("the subscription is in report mode");
    }
    let Some(finding) = runtime.findings.get(&request.finding) else {
        return Err("unknown finding");
    };
    if finding.session != subscription.target {
        return Err("the finding belongs to a session this watcher does not watch");
    }
    if finding.acted != Acted::Open {
        return Err("the finding was already answered");
    }
    if !allows(finding.kind, request.action) {
        return Err("this finding kind does not unlock that move");
    }
    let Some(run) = runtime.runs.get(&finding.run).filter(|run| run.is_open()) else {
        return Err("the finding's run has ended");
    };
    if run.killed {
        return Err("the finding's run was killed");
    }
    let is_live = fleet
        .sessions
        .get(&run.session)
        .is_some_and(|session| !session.state.is_terminal());
    if !is_live {
        return Err("the observed session is gone");
    }
    let key = finding.key();
    let is_claiming = runtime
        .findings
        .values()
        .any(|other| matches!(other.acted, Acted::InFlight(_)) && other.key() == key);
    let is_busy =
        is_claiming || runtime.armed.contains_key(&key) || runtime.pending_sends.contains_key(&key);
    if is_busy {
        return Err("a move on that agent is still pending");
    }
    Ok(key)
}

/// A claim is on disk: the move goes live — a send waits for its hand-over, a stop arms. A
/// claim recorded after its run ended is void.
pub(crate) fn on_claim_recorded(runtime: &mut Runtime, finding_id: &FindingId) {
    let Some(finding) = runtime.findings.get(finding_id) else {
        return;
    };
    let Acted::InFlight(action) = finding.acted else {
        return;
    };
    let is_open = runtime.runs.get(&finding.run).is_some_and(Run::is_open);
    if !is_open {
        if let Some(finding) = runtime.findings.get_mut(finding_id) {
            finding.acted = Acted::Void("run ended before the claim was recorded".to_owned());
        }
        return;
    }
    let key = finding.key();
    match action {
        Move::Send => {
            let pending = PendingSend {
                finding: finding_id.clone(),
                text: render(&finding.template, finding),
            };
            runtime.pending_sends.insert(key, pending);
        }
        Move::Stop => {
            runtime.armed.insert(key, finding_id.clone());
        }
    }
    if let Some(finding) = runtime.findings.get_mut(finding_id) {
        finding.acted = Acted::Live(action);
    }
}

/// The claim could not be written: the move is void, nothing goes live.
pub(crate) fn on_claim_failed(runtime: &mut Runtime, finding_id: &FindingId) {
    if let Some(finding) = runtime
        .findings
        .get_mut(finding_id)
        .filter(|finding| matches!(finding.acted, Acted::InFlight(_)))
    {
        finding.acted = Acted::Void("the claim could not be written; nothing was done".to_owned());
    }
}

/// A hook answer carried the send: the pending entry is cleared, the finding reads handed over, the ledger records it.
pub(crate) fn on_handed_over(runtime: &mut Runtime, finding_id: &FindingId) -> Vec<Action> {
    let key = runtime
        .pending_sends
        .iter()
        .find(|(_, pending)| pending.finding == *finding_id)
        .map(|(key, _)| key.clone());
    let Some(key) = key else {
        return Vec::new();
    };
    runtime.pending_sends.remove(&key);
    if let Some(finding) = runtime.findings.get_mut(finding_id) {
        finding.acted = Acted::Done(Move::Send);
    }
    vec![Action::Record(Record::RuntimeHandedOver {
        finding: finding_id.clone(),
    })]
}

/// The third verdict step, asked only when the heart attack and the member guard allow. Pure, like `attack`: the
/// enforce decision was taken while the hook was applied.
#[must_use]
pub fn verdict(fleet: &Fleet, probe: &Probe) -> Answer {
    let runtime = &fleet.runtime;
    let allow = Answer {
        verdict: Verdict::Allow,
        handed_over: None,
    };
    let answer = |verdict: Verdict| Answer {
        verdict,
        handed_over: None,
    };
    let pre_tool = match probe.hook() {
        ProbedHook::PreTool { tool_name, .. } => {
            let is_guarded = runtime.is_subscriber(probe.session())
                && fleet.caps.guarded_tools().contains(tool_name);
            if is_guarded {
                let reason = runtime
                    .texts
                    .subscriber_deny_reason()
                    .replace("{tool}", tool_name);
                return answer(Verdict::Deny { reason });
            }
            true
        }
        ProbedHook::PostTool { .. } => false,
        ProbedHook::Prompt => return waiting_hand_over(runtime, probe, false),
        ProbedHook::Stop => {
            return match &runtime.decision {
                Some((key, verdict)) if key.0 == *probe.session() => answer(verdict.clone()),
                Some(_) | None => waiting_hand_over(runtime, probe, true),
            };
        }
        ProbedHook::Other => {
            return match &runtime.decision {
                Some((key, verdict)) if key.0 == *probe.session() => answer(verdict.clone()),
                Some(_) | None => allow,
            };
        }
    };
    let key = (probe.session().clone(), probe.agent().cloned());
    if let Some((decided, verdict)) = &runtime.decision
        && decided.0 == key.0
    {
        return answer(verdict.clone());
    }
    if let Some(reason) = runtime.armed_reason(&key) {
        return answer(Verdict::Stop { reason, pre_tool });
    }
    let is_success = matches!(probe.hook(), ProbedHook::PostTool { is_failure: false });
    match runtime.pending_sends.get(&key).filter(|_| is_success) {
        Some(pending) => Answer {
            verdict: Verdict::Context {
                event: ContextEvent::PostToolUse,
                text: pending.text.clone(),
            },
            handed_over: Some(pending.finding.clone()),
        },
        None => allow,
    }
}

/// Hands a root's pending send over where the root waits instead of calling a tool: its `Stop` is blocked with the
/// rendered template as the reason (the agent reads it and goes on), its next prompt carries it as context. A waiting
/// agent makes no tool call, so the `PostToolUse` hand-over alone would deadlock it.
fn waiting_hand_over(runtime: &Runtime, probe: &Probe, is_stop: bool) -> Answer {
    let key = (probe.session().clone(), None);
    let pending = runtime
        .pending_sends
        .get(&key)
        .filter(|_| probe.agent().is_none());
    let Some(pending) = pending else {
        return Answer {
            verdict: Verdict::Allow,
            handed_over: None,
        };
    };
    let verdict = if is_stop {
        Verdict::Block {
            reason: pending.text.clone(),
        }
    } else {
        Verdict::Context {
            event: ContextEvent::UserPromptSubmit,
            text: pending.text.clone(),
        }
    };
    Answer {
        verdict,
        handed_over: Some(pending.finding.clone()),
    }
}

/// The per-event pass: runs of terminal sessions end; a watch ends when its subscriber or its target ended, the
/// subscriber moved sockets, or (once the registry seeded) either is unknown; each watcher that is idle or blocked with
/// nothing in flight and findings pending gets one digest.
pub(crate) fn flush(fleet: &Fleet, runtime: &mut Runtime) -> Vec<Action> {
    let mut actions = Vec::new();
    let gone: Vec<SessionId> = runtime
        .open
        .keys()
        .filter(|id| {
            fleet
                .sessions
                .get(*id)
                .is_none_or(|session| session.state.is_terminal())
        })
        .cloned()
        .collect();
    for session in gone {
        end_session(runtime, &fleet.caps, &session, EndReason::SessionEnded);
    }
    let watchers: Vec<SessionId> = runtime.subscriptions.keys().cloned().collect();
    for watcher in watchers {
        actions.extend(flush_watch(fleet, runtime, &watcher));
    }
    actions
}

/// [`flush`] for one watch.
fn flush_watch(fleet: &Fleet, runtime: &mut Runtime, watcher: &SessionId) -> Vec<Action> {
    let Some(held) = runtime.subscriptions.get(watcher) else {
        return Vec::new();
    };
    let Some(host) = fleet.sessions.get(&held.session) else {
        if fleet.seeded {
            return end_subscription(runtime, watcher, "session gone while heartbeat was down");
        }
        return Vec::new();
    };
    if let Some(end) = terminal_word(&host.state) {
        return end_subscription(runtime, watcher, &format!("the session {end}"));
    }
    if host
        .socket
        .as_ref()
        .is_some_and(|registered| *registered != held.socket)
    {
        return end_subscription(
            runtime,
            watcher,
            "the session's socket changed; subscribe again",
        );
    }
    let target = fleet.sessions.get(&held.target);
    if target.is_none() && fleet.seeded {
        return farewell(
            fleet,
            runtime,
            watcher,
            "the watched session is gone while heartbeat was down",
        );
    }
    if let Some(end) = target.and_then(|target| terminal_word(&target.state)) {
        return farewell(
            fleet,
            runtime,
            watcher,
            &format!("the watched session {end}"),
        );
    }
    let is_ready = matches!(
        host.state,
        SessionState::Idle | SessionState::Blocked { .. }
    ) && held.in_flight.is_none()
        && !held.pending.is_empty();
    if !is_ready {
        return Vec::new();
    }
    let id = runtime.next_digest;
    let text = render_digest(fleet, runtime, held, id);
    let Some(held) = runtime.subscriptions.get_mut(watcher) else {
        return Vec::new();
    };
    let lines = u32::try_from(held.pending.len()).unwrap_or(u32::MAX);
    held.pending.clear();
    held.in_flight = Some(id);
    let action = Action::DeliverRuntimeDigest(RuntimeDigest {
        id,
        session: held.session.clone(),
        socket: held.socket.clone(),
        token: held.token.clone(),
        text,
        lines,
    });
    runtime.next_digest = DigestId(id.0 + 1);
    vec![action]
}

/// Ends a watch whose watched session is gone, telling the still-live watcher: one last digest carries any undelivered
/// findings and the end line, so the watcher stops instead of waiting for a digest that never comes.
fn farewell(
    fleet: &Fleet,
    runtime: &mut Runtime,
    watcher: &SessionId,
    reason: &str,
) -> Vec<Action> {
    let Some(held) = runtime.subscriptions.get(watcher) else {
        return Vec::new();
    };
    let id = runtime.next_digest;
    let mut text = String::new();
    if !held.pending.is_empty() {
        text = render_digest(fleet, runtime, held, id);
        text.push('\n');
    }
    let _ = write!(
        text,
        "heartbeat runtime watch ended — {reason} ({}). Nothing more arrives here: run your Stop step.",
        title_of(fleet, &held.target)
    );
    let action = Action::DeliverRuntimeDigest(RuntimeDigest {
        id,
        session: held.session.clone(),
        socket: held.socket.clone(),
        token: held.token.clone(),
        text,
        lines: u32::try_from(held.pending.len()).unwrap_or(u32::MAX),
    });
    runtime.next_digest = DigestId(id.0 + 1);
    let mut actions = vec![action];
    actions.extend(end_subscription(runtime, watcher, reason));
    actions
}

/// The digest text: a header naming the watched session, one line per pending finding (at most
/// `caps.runtime_digest_lines()`), the pointer to the status.
fn render_digest(
    fleet: &Fleet,
    runtime: &Runtime,
    held: &RuntimeSubscription,
    id: DigestId,
) -> String {
    let max_lines = fleet.caps.runtime_digest_lines();
    let mut text = format!(
        "heartbeat runtime digest {} — {} findings in {}\n",
        id.label(),
        held.pending.len(),
        title_of(fleet, &held.target)
    );
    for finding in held.pending.iter().take(max_lines) {
        let _ = writeln!(text, "- {}", digest_line(fleet, runtime, finding));
    }
    if held.pending.len() > max_lines {
        let _ = writeln!(text, "- … and {} more", held.pending.len() - max_lines);
    }
    text.push_str("Full state: smine-heartbeat runtime status --format json");
    text
}

fn digest_line(fleet: &Fleet, runtime: &Runtime, id: &FindingId) -> String {
    match runtime.findings.get(id) {
        Some(finding) => format!(
            "finding {} {} {} {} in {}: {} — {} (moves: send, stop)",
            id.0,
            finding.kind.as_str(),
            finding.skill,
            finding.entry,
            title_of(fleet, &finding.session),
            finding.reason,
            finding.evidence.join("; ")
        ),
        None => format!("finding {}", id.0),
    }
}

/// Fills `{finding}`, `{skill}`, `{version}`, `{check}` (alias `{entry}`), `{reason}` and `{evidence}` from the
/// finding; any other `{…}` stays verbatim (D9).
#[must_use]
pub fn render(template: &str, finding: &Finding) -> String {
    fill(template, finding)
}

fn fill(template: &str, finding: &Finding) -> String {
    let evidence = finding.evidence.join("; ");
    let mut out = String::with_capacity(template.len());
    let mut rest = template;
    while let Some(open) = rest.find('{') {
        out.push_str(&rest[..open]);
        let tail = &rest[open..];
        let Some(close) = tail.find('}') else {
            out.push_str(tail);
            return out;
        };
        let value = match &tail[1..close] {
            "finding" => Some(finding.id.0.as_str()),
            "skill" => Some(finding.skill.as_str()),
            "version" => Some(finding.version.as_str()),
            "check" | "entry" => Some(finding.entry.as_str()),
            "reason" => Some(finding.reason.as_str()),
            "evidence" => Some(evidence.as_str()),
            _ => None,
        };
        match value {
            Some(value) => out.push_str(value),
            None => out.push_str(&tail[..=close]),
        }
        rest = &tail[close + 1..];
    }
    out.push_str(rest);
    out
}

/// The moves each finding kind unlocks (D7): both kinds allow a send and a stop.
#[must_use]
pub fn allows(kind: FindingKind, action: Move) -> bool {
    match (kind, action) {
        (FindingKind::CheckFailed | FindingKind::Judgment, Move::Send | Move::Stop) => true,
    }
}

/// The `/runtime/subscribe` and `/runtime/unsubscribe` read-back for `session`.
#[must_use]
pub fn subscription_of(fleet: &Fleet, session: &SessionId) -> SubscriptionAnswer {
    let runtime = &fleet.runtime;
    if let Some(held) = runtime.subscriptions.get(session) {
        return SubscriptionAnswer::Held(Box::new(subscription_row(fleet, held)));
    }
    match runtime
        .subscribe_refusal
        .as_ref()
        .filter(|(refused, _)| refused == session)
    {
        Some((_, reason)) => SubscriptionAnswer::Refused(reason.clone()),
        None => SubscriptionAnswer::Absent,
    }
}

/// The `/runtime/act` read-back for `finding`.
#[must_use]
pub fn act_of(fleet: &Fleet, finding: &FindingId) -> ActAnswer {
    let runtime = &fleet.runtime;
    if let Some((_, reason)) = runtime
        .act_refusal
        .as_ref()
        .filter(|(refused, _)| refused == finding)
    {
        return ActAnswer::Refused(reason.clone());
    }
    let Some(found) = runtime.findings.get(finding) else {
        return ActAnswer::Refused("unknown finding".to_owned());
    };
    match &found.acted {
        Acted::InFlight(_) => ActAnswer::Claiming,
        Acted::Void(reason) => ActAnswer::Void(reason.clone()),
        Acted::Open | Acted::Live(_) | Acted::Done(_) => {
            ActAnswer::Settled(Box::new(finding_row(fleet, found)))
        }
    }
}

/// The runtime document (`GET /runtime`, `heartbeat runtime status --format json`).
#[must_use]
pub fn document(fleet: &Fleet, health: &Health) -> RuntimeDocument {
    let runtime = &fleet.runtime;
    RuntimeDocument {
        schema: RUNTIME_SCHEMA,
        heartbeat: health.clone(),
        subscriptions: runtime
            .subscriptions
            .values()
            .map(|held| subscription_row(fleet, held))
            .collect(),
        runs: runtime
            .runs
            .values()
            .map(|run| run_row(fleet, run))
            .collect(),
        findings: runtime
            .findings
            .values()
            .map(|finding| finding_row(fleet, finding))
            .collect(),
    }
}

fn subscription_row(fleet: &Fleet, held: &RuntimeSubscription) -> RuntimeSubscriptionRow {
    let last_digest = match held.in_flight {
        Some(id) => Some(RuntimeDigestRow {
            id: id.label(),
            outcome: DigestOutcomeWire::InFlight,
            at: None,
        }),
        None => held.last.as_ref().map(|last| RuntimeDigestRow {
            id: last.id.label(),
            outcome: DigestOutcomeWire::from(&last.outcome),
            at: Some(last.at.to_string()),
        }),
    };
    RuntimeSubscriptionRow {
        last_digest,
        mode: ModeWire::from(held.mode),
        session: held.session.0.clone(),
        since: held.since.to_string(),
        target: held.target.0.clone(),
        target_title: title_of(fleet, &held.target),
        title: title_of(fleet, &held.session),
    }
}

fn run_row(fleet: &Fleet, run: &Run) -> RunRow {
    let failed: Vec<String> = run
        .failures
        .iter()
        .map(|(check, count)| format!("{check} ×{count}"))
        .collect();
    let (state, reason) = match (run.killed, run.ended) {
        (true, _) => (RunStateWire::Killed, failed.join(", ")),
        (false, Some(end)) => (RunStateWire::Ended, end.as_str().to_owned()),
        (false, None) => (RunStateWire::Open, failed.join(", ")),
    };
    RunRow {
        id: run.id.0,
        anchors: run
            .log
            .entries
            .iter()
            .map(|entry| AnchorHitRow {
                id: entry.anchor.clone(),
                seq: entry.seq,
                captures: entry.captures.clone(),
            })
            .collect(),
        failures: run
            .failures
            .iter()
            .map(|(check, count)| CheckFailureRow {
                check: check.clone(),
                count: *count,
            })
            .collect(),
        findings: run.findings,
        passes: run
            .passes
            .iter()
            .map(|(check, count)| CheckPassRow {
                check: check.clone(),
                count: *count,
            })
            .collect(),
        reason,
        session: run.session.0.clone(),
        skill: run.skill.clone(),
        state,
        title: title_of(fleet, &run.session),
        version: run.version.clone(),
    }
}

fn finding_row(fleet: &Fleet, finding: &Finding) -> FindingRow {
    let runtime = &fleet.runtime;
    let is_actionable = finding.acted == Acted::Open
        && runtime
            .runs
            .get(&finding.run)
            .is_some_and(|run| run.is_open() && !run.killed);
    let moves = if is_actionable {
        [Move::Send, Move::Stop]
            .into_iter()
            .filter(|action| allows(finding.kind, *action))
            .map(MoveWire::from)
            .collect()
    } else {
        Vec::new()
    };
    FindingRow {
        id: finding.id.0.clone(),
        acted: ActedWire::from(&finding.acted),
        entry: finding.entry.clone(),
        evidence: finding.evidence.clone(),
        kind: FindingKindWire::from(finding.kind),
        moves,
        reason: finding.reason.clone(),
        run: finding.run.0,
        session: finding.session.0.clone(),
        title: title_of(fleet, &finding.session),
        skill: finding.skill.clone(),
        version: finding.version.clone(),
    }
}

/// The session's registry title; empty when unknown.
fn title_of(fleet: &Fleet, session: &SessionId) -> String {
    fleet
        .sessions
        .get(session)
        .and_then(|found| found.title.clone())
        .unwrap_or_default()
}

fn terminal_word(state: &SessionState) -> Option<&'static str> {
    match state {
        SessionState::Ended { .. } => Some("ended"),
        SessionState::Died { .. } => Some("died"),
        SessionState::Running
        | SessionState::Waiting { .. }
        | SessionState::Blocked { .. }
        | SessionState::Idle => None,
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use serde_json::{Value, json};

    use super::*;
    use crate::apply::apply;
    use crate::event::{Agent, Event};

    /// The observed session.
    const C: &str = "c";
    /// The runtime subscriber.
    const R: &str = "r";
    const TOKEN: &str = "feedc0de";
    const DEMO: &str = include_str!("../tests/fixtures/enforce/heartbeat-demo.json");

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    /// The default kill threshold.
    fn kill_at() -> u8 {
        Caps::default().kill_at()
    }

    fn sid(id: &str) -> SessionId {
        SessionId(id.to_owned())
    }

    fn demo() -> Arc<EnforceDoc> {
        Arc::new(EnforceDoc::parse(DEMO.as_bytes()).expect("demo enforce.json"))
    }

    fn hook(session: &str, agent: Option<&str>, kind: HookKind) -> HookEvent {
        HookEvent {
            session_id: sid(session),
            agent: agent.map(|id| Agent {
                id: AgentId(id.to_owned()),
                agent_type: "Explore".to_owned(),
            }),
            cwd: None,
            transcript_path: None,
            at: at(10),
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        }
    }

    /// The root's `Stop` of `session` with `in_flight` background tasks listed in its body.
    fn stop_with(session: &str, in_flight: usize) -> HookEvent {
        HookEvent {
            in_flight,
            ..hook(session, None, HookKind::Stop)
        }
    }

    fn call(tool: &str, input: Value) -> ToolCall {
        ToolCall {
            tool_name: tool.to_owned(),
            input,
        }
    }

    fn send_hook(fleet: &mut Fleet, hook: HookEvent) -> Verdict {
        let probe = Probe::of(&hook);
        let _ = apply(fleet, Event::Hook(hook));
        verdict(fleet, &probe).verdict
    }

    /// Sessions `c` (running) and `r` (idle), with an open heartbeat-demo run on `c`.
    fn world() -> Fleet {
        let mut fleet = Fleet::new(at(0));
        let start = || HookKind::SessionStart {
            source: "startup".to_owned(),
        };
        let _ = send_hook(&mut fleet, hook(C, None, start()));
        let _ = send_hook(&mut fleet, hook(R, None, start()));
        let mut open = hook(
            C,
            None,
            HookKind::UserPromptSubmit {
                prompt: "/heartbeat-demo scenario:deny-laundered-workflow".to_owned(),
            },
        );
        open.enforce = Some(demo());
        let _ = send_hook(&mut fleet, open);
        fleet
    }

    /// `r` watches `c`.
    fn subscribe(fleet: &mut Fleet, mode: Mode) {
        subscribe_to(fleet, R, C, mode);
    }

    fn subscribe_to(fleet: &mut Fleet, watcher: &str, target: &str, mode: Mode) {
        let request = RuntimeSubscribeRequest {
            session: sid(watcher),
            target: sid(target),
            mode,
            socket: PathBuf::from("/tmp/r.sock"),
            token: MessagingToken(TOKEN.to_owned()),
        };
        let _ = apply(
            fleet,
            Event::RuntimeSubscribed {
                request,
                replay: false,
                at: at(11),
            },
        );
    }

    fn produce(fleet: &mut Fleet, value: &str) {
        let mut post = hook(
            C,
            None,
            HookKind::PostToolUse(call(
                "Bash",
                json!({"command": "smine-rules demo-value --seed c"}),
            )),
        );
        let stdout = json!({"value": value}).to_string();
        post.response = Some(Arc::from(
            json!({"stdout": stdout, "stderr": ""}).to_string(),
        ));
        let _ = send_hook(fleet, post);
    }

    fn stage(value: &str) -> HookKind {
        HookKind::PreToolUse(call(
            "Workflow",
            json!({"scriptPath": "/h/skills/heartbeat-demo/workflows/demo.js", "args": {"value": value}}),
        ))
    }

    /// The relay primer (K1), byte for byte as the gate admits it.
    const PRIMER: &str = "export const meta = { name: 'relay-primer', description: 'SMINE-PRIMER — opens a notification turn' }\nawait new Promise(r => setTimeout(r, 10000))\nreturn 'primed'";

    /// PROC-002 returned: the stage anchor is logged, so `close_before_stop` is due at the root's next judged Stop.
    fn launch_stage(fleet: &mut Fleet) {
        let _ = send_hook(
            fleet,
            hook(
                C,
                None,
                HookKind::PostToolUse(call(
                    "Workflow",
                    json!({"scriptPath": "/h/skills/heartbeat-demo/workflows/demo.js", "args": {"value": "abc"}}),
                )),
            ),
        );
    }

    /// A root prompt that is no slash command.
    fn go_on() -> HookKind {
        HookKind::UserPromptSubmit {
            prompt: "go on".to_owned(),
        }
    }

    /// The primer's launch returned.
    fn primer() -> HookKind {
        HookKind::PostToolUse(call("Workflow", json!({ "script": PRIMER })))
    }

    #[test]
    fn provenance_ladder() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        assert_eq!(
            send_hook(&mut fleet, hook(C, None, stage("abc"))),
            Verdict::Allow
        );

        for failure in 1..kill_at() {
            let denied = send_hook(&mut fleet, hook(C, None, stage("zzz")));
            assert!(
                matches!(denied, Verdict::Deny { .. }),
                "failure {failure}: {denied:?}"
            );
        }
        assert_eq!(
            fleet.runtime.findings.len(),
            usize::from(kill_at() - 1),
            "one finding per failure below the kill"
        );
        assert!(
            fleet
                .runtime
                .findings
                .values()
                .all(|finding| finding.kind == FindingKind::CheckFailed
                    && finding.entry == "value_provenance"),
            "{:?}",
            fleet.runtime.findings
        );

        let killing = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        assert!(
            matches!(&killing, Verdict::Stop { reason, pre_tool: true }
                if reason.contains("value_provenance failed 8 times")),
            "the eighth failure stops its own call: {killing:?}"
        );
        assert_eq!(
            fleet.runtime.findings.len(),
            usize::from(kill_at() - 1),
            "the kill raises no finding"
        );
        let run = fleet.runtime.runs.values().next().expect("run");
        assert!(run.killed, "the eighth failure kills");
        assert_eq!(
            run.failures,
            BTreeMap::from([("value_provenance".to_owned(), kill_at())])
        );
        let passed: BTreeMap<String, u32> = BTreeMap::from([
            ("producer_first".to_owned(), 9),
            ("value_provenance".to_owned(), 1),
        ]);
        assert_eq!(run.passes, passed);
        let ninth = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        assert!(
            matches!(ninth, Verdict::Stop { pre_tool: true, .. }),
            "a failure after the kill stops too, never a plain deny: {ninth:?}"
        );
        let after = send_hook(
            &mut fleet,
            hook(C, None, HookKind::PreToolUse(call("Read", json!({})))),
        );
        assert!(
            matches!(after, Verdict::Stop { pre_tool: true, .. }),
            "{after:?}"
        );
        let subagent = send_hook(
            &mut fleet,
            hook(
                C,
                Some("a1"),
                HookKind::PostToolUse(call("Read", json!({}))),
            ),
        );
        assert!(
            matches!(subagent, Verdict::Stop { .. }),
            "the kill stops every agent: {subagent:?}"
        );

        let _ = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::UserPromptSubmit {
                    prompt: "go on".to_owned(),
                },
            ),
        );
        let cleared = send_hook(
            &mut fleet,
            hook(C, None, HookKind::PreToolUse(call("Read", json!({})))),
        );
        assert_eq!(cleared, Verdict::Allow);
    }

    #[test]
    fn lifted_kill_resets_runs() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        for _ in 0..kill_at() {
            let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        }
        let demo_run = |fleet: &Fleet| fleet.runtime.runs.values().next().cloned().expect("run");
        assert!(demo_run(&fleet).killed, "the eighth failure kills");

        let _ = send_hook(&mut fleet, hook(C, None, go_on()));
        assert!(fleet.runtime.killed.is_empty(), "the prompt lifts the kill");
        let revived = demo_run(&fleet);
        assert!(revived.is_open(), "a plain prompt keeps the run");
        assert!(!revived.killed, "the lifting prompt resets killed");
        assert!(revived.failures.is_empty(), "and empties the failures");
        assert_eq!(
            revived.passes.get("producer_first"),
            Some(&8),
            "passes stay"
        );

        let again = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        assert!(
            matches!(again, Verdict::Deny { .. }),
            "a fresh budget: the next failure only denies: {again:?}"
        );
        assert_eq!(demo_run(&fleet).failures.get("value_provenance"), Some(&1));
    }

    #[test]
    fn prompt_without_kill_resets_nothing() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        let _ = send_hook(&mut fleet, hook(C, None, go_on()));
        let run = fleet.runtime.runs.values().next().expect("run");
        assert!(!run.killed);
        assert_eq!(
            run.failures.get("value_provenance"),
            Some(&2),
            "a prompt that lifts no kill resets nothing"
        );
    }

    #[test]
    fn stop_kill_answers_stop() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        launch_stage(&mut fleet);
        let _ = send_hook(&mut fleet, hook(C, None, go_on()));
        for failure in 1..kill_at() {
            let blocked = send_hook(&mut fleet, hook(C, None, HookKind::Stop));
            assert!(
                matches!(blocked, Verdict::Block { .. }),
                "failure {failure}: {blocked:?}"
            );
        }
        let killing = send_hook(&mut fleet, hook(C, None, HookKind::Stop));
        assert!(
            matches!(&killing, Verdict::Stop { reason, pre_tool: false }
                if reason.contains("close_before_stop failed 8 times")),
            "the eighth blocked Stop stops: {killing:?}"
        );
    }

    #[test]
    fn slash_prompt_after_kill_supersedes_reset_run() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        for _ in 0..kill_at() {
            let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        }
        let mut slash = hook(
            C,
            None,
            HookKind::UserPromptSubmit {
                prompt: "/heartbeat-demo scenario:pass-clean-run".to_owned(),
            },
        );
        slash.enforce = Some(demo());
        let _ = send_hook(&mut fleet, slash);
        let runs: Vec<&Run> = fleet.runtime.runs.values().collect();
        assert_eq!(runs.len(), 2);
        assert_eq!(runs[0].ended, Some(EndReason::Superseded));
        assert!(!runs[0].killed, "the lifted kill reset it before it ended");
        assert!(runs[1].is_open() && runs[1].failures.is_empty());
        assert!(fleet.runtime.killed.is_empty());
    }

    /// Every case runs behind a stage launch: `close_before_stop` is due, and only the Stop body's `background_tasks`
    /// decide whether the Stop under test is judged.
    #[test]
    fn in_flight_stop_cases() {
        let close = || {
            hook(
                C,
                None,
                HookKind::PostToolUse(call(
                    "Bash",
                    json!({"command": "smine-rules demo-close --dir /a"}),
                )),
            )
        };
        // (label, events before the Stop, its in-flight count, blocked, failures, close_before_stop passes)
        let cases = [
            ("in-flight-defers", vec![], 1, false, 0, 0),
            (
                "next-empty-stop-judges",
                vec![stop_with(C, 1)],
                0,
                true,
                1,
                0,
            ),
            (
                "empty-stop-after-a-launch-is-judged",
                vec![hook(C, None, primer())],
                0,
                true,
                1,
                0,
            ),
            (
                "running-subagent-exempts-nothing",
                vec![hook(C, Some("a1"), HookKind::SubagentStart)],
                0,
                true,
                1,
                0,
            ),
            ("in-flight-after-the-close", vec![close()], 1, false, 0, 0),
            (
                "empty-after-the-close-passes",
                vec![close()],
                0,
                false,
                0,
                1,
            ),
        ];
        for (label, before, in_flight, is_blocked, failures, passes) in cases {
            let mut fleet = world();
            produce(&mut fleet, "abc");
            launch_stage(&mut fleet);
            for event in before {
                let _ = send_hook(&mut fleet, event);
            }
            let answer = send_hook(&mut fleet, stop_with(C, in_flight));
            assert_eq!(
                matches!(answer, Verdict::Block { .. }),
                is_blocked,
                "{label}: {answer:?}"
            );
            if !is_blocked {
                assert_eq!(answer, Verdict::Allow, "{label}");
            }
            let run = fleet.runtime.runs.values().next().expect("run");
            assert_eq!(run.failures.len(), failures, "{label}");
            assert_eq!(
                run.passes.get("close_before_stop").copied().unwrap_or(0),
                passes,
                "{label}"
            );
        }
    }

    #[test]
    fn stop_block() {
        let mut fleet = world();
        produce(&mut fleet, "abc");
        let _ = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::PostToolUse(call(
                    "Workflow",
                    json!({"scriptPath": "/h/skills/heartbeat-demo/workflows/demo.js", "args": {"value": "abc"}}),
                )),
            ),
        );
        assert_eq!(
            send_hook(&mut fleet, stop_with(C, 1)),
            Verdict::Allow,
            "a root waiting on its in-flight stage is not finishing"
        );
        let observed = |fleet: &Fleet| {
            let session = &fleet.sessions[&sid(C)];
            (session.state.clone(), session.stopped_at)
        };
        let _ = send_hook(&mut fleet, hook(C, None, go_on()));
        let before = observed(&fleet);
        assert_eq!(before.0, SessionState::Running);
        let blocked = send_hook(&mut fleet, stop_with(C, 0));
        assert!(matches!(blocked, Verdict::Block { .. }), "{blocked:?}");
        assert_eq!(
            observed(&fleet),
            before,
            "a blocked Stop keeps the turn open: still running, no stopped_at move"
        );
        let failures = fleet
            .runtime
            .runs
            .values()
            .map(|run| run.failures.len())
            .sum::<usize>();
        assert_eq!(failures, 1, "the waiting Stop counted no failure");

        let _ = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::PostToolUse(call(
                    "Bash",
                    json!({"command": "smine-rules demo-close --dir /a"}),
                )),
            ),
        );
        assert_eq!(send_hook(&mut fleet, stop_with(C, 0)), Verdict::Allow);
        assert_eq!(
            fleet.sessions[&sid(C)].state,
            SessionState::Idle,
            "the allowed Stop ends the turn"
        );
        let passes: Vec<BTreeMap<String, u32>> = fleet
            .runtime
            .runs
            .values()
            .map(|run| run.passes.clone())
            .collect();
        assert_eq!(
            passes,
            vec![BTreeMap::from([("close_before_stop".to_owned(), 1)])],
            "only the judged Stop after demo-close passed"
        );
    }

    #[test]
    fn every_open_run_counts_its_passes() {
        let mut fleet = world();
        let mut nested = hook(
            C,
            None,
            HookKind::PreToolUse(call("Skill", json!({"skill": "heartbeat-demo"}))),
        );
        nested.enforce = Some(demo());
        let _ = send_hook(&mut fleet, nested);
        produce(&mut fleet, "abc");
        assert_eq!(
            send_hook(&mut fleet, hook(C, None, stage("abc"))),
            Verdict::Allow
        );
        let denied = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        assert!(matches!(denied, Verdict::Deny { .. }), "{denied:?}");

        let outer = fleet
            .runtime
            .runs
            .values()
            .find(|run| run.parent.is_none())
            .expect("outer run");
        let inner = fleet
            .runtime
            .runs
            .values()
            .find(|run| run.parent.is_some())
            .expect("nested run");
        let passed: BTreeMap<String, u32> = BTreeMap::from([
            ("producer_first".to_owned(), 2),
            ("value_provenance".to_owned(), 1),
        ]);
        assert_eq!(outer.passes, passed);
        assert_eq!(
            inner.passes, passed,
            "the nested run is judged although the outer run decided"
        );
        assert_eq!(
            outer.failures,
            BTreeMap::from([("value_provenance".to_owned(), 1)])
        );
        assert!(
            inner.failures.is_empty(),
            "only the deciding failure is counted, as before"
        );
        assert_eq!(
            run_row(&fleet, outer).passes,
            vec![
                CheckPassRow {
                    check: "producer_first".to_owned(),
                    count: 2,
                },
                CheckPassRow {
                    check: "value_provenance".to_owned(),
                    count: 1,
                },
            ]
        );
    }

    #[test]
    fn recovery_send_hands_over_once() {
        let mut fleet = world();
        subscribe(&mut fleet, Mode::Act);
        produce(&mut fleet, "abc");
        let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        let finding = fleet
            .runtime
            .findings
            .values()
            .find(|finding| finding.kind == FindingKind::CheckFailed)
            .expect("finding")
            .id
            .clone();
        let request = ActRequest {
            session: sid(R),
            token: MessagingToken(TOKEN.to_owned()),
            finding: finding.clone(),
            action: Move::Send,
        };
        let _ = apply(
            &mut fleet,
            Event::RuntimeActRequested {
                request,
                at: at(12),
            },
        );
        let _ = apply(
            &mut fleet,
            Event::ClaimRecorded {
                finding: finding.clone(),
                at: at(13),
            },
        );
        let post = send_hook(
            &mut fleet,
            hook(C, None, HookKind::PostToolUse(call("Read", json!({})))),
        );
        match post {
            Verdict::Context { text, .. } => {
                assert!(text.contains("pass exactly the value"), "{text}");
            }
            other => panic!("expected the recovery hand-over, got {other:?}"),
        }
    }

    #[test]
    fn judgment_raised_while_subscribed() {
        let mut fleet = world();
        subscribe(&mut fleet, Mode::Report);
        produce(&mut fleet, "abc");
        let _ = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::PostToolUse(call(
                    "Workflow",
                    json!({"scriptPath": "/h/skills/heartbeat-demo/workflows/demo.js", "args": {"value": "abc"}}),
                )),
            ),
        );
        let judgment = fleet
            .runtime
            .findings
            .values()
            .find(|finding| finding.kind == FindingKind::Judgment)
            .expect("judgment finding");
        assert_eq!(judgment.entry, "value_check");
        assert_eq!(
            judgment.evidence,
            vec![
                "value = \"abc\"".to_owned(),
                "arg:/args/value = \"abc\"".to_owned()
            ]
        );
    }

    #[test]
    fn subscriber_guard() {
        let mut fleet = world();
        subscribe(&mut fleet, Mode::Act);
        let guarded = send_hook(
            &mut fleet,
            hook(
                R,
                None,
                HookKind::PreToolUse(call("SendMessage", json!({}))),
            ),
        );
        assert!(matches!(guarded, Verdict::Deny { .. }), "{guarded:?}");
        let other = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::PreToolUse(call("SendMessage", json!({}))),
            ),
        );
        assert_eq!(other, Verdict::Allow);
    }

    #[test]
    fn a_watch_is_one_to_one() {
        let mut fleet = world();
        let start = HookKind::SessionStart {
            source: "startup".to_owned(),
        };
        for session in ["d", "q"] {
            let _ = send_hook(&mut fleet, hook(session, None, start.clone()));
        }
        let mut open = hook(
            "d",
            None,
            HookKind::UserPromptSubmit {
                prompt: "/heartbeat-demo scenario:deny-laundered-workflow".to_owned(),
            },
        );
        open.enforce = Some(demo());
        let _ = send_hook(&mut fleet, open);
        subscribe(&mut fleet, Mode::Act);
        assert_eq!(fleet.runtime.subscriptions.len(), 1);

        let _ = send_hook(&mut fleet, hook("d", None, stage("zzz")));
        let other = fleet
            .runtime
            .findings
            .values()
            .find(|finding| finding.session == sid("d"))
            .expect("d's finding")
            .id
            .clone();
        let watch = &fleet.runtime.subscriptions[&sid(R)];
        assert!(
            watch.pending.is_empty() && watch.in_flight.is_none(),
            "d's finding never reaches c's watcher: {watch:?}"
        );

        let request = ActRequest {
            session: sid(R),
            token: MessagingToken(TOKEN.to_owned()),
            finding: other.clone(),
            action: Move::Stop,
        };
        let _ = apply(
            &mut fleet,
            Event::RuntimeActRequested {
                request,
                at: at(12),
            },
        );
        assert_eq!(
            fleet
                .runtime
                .act_refusal
                .as_ref()
                .map(|(_, reason)| reason.as_str()),
            Some("the finding belongs to a session this watcher does not watch")
        );

        let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        assert!(
            fleet.runtime.subscriptions[&sid(R)].in_flight.is_some(),
            "c's finding is digested to its watcher"
        );

        subscribe_to(&mut fleet, "q", C, Mode::Report);
        let refusal = fleet.runtime.subscribe_refusal.clone().expect("refused");
        assert!(refusal.1.contains("already watched"), "{refusal:?}");
        subscribe_to(&mut fleet, "q", "q", Mode::Report);
        let refusal = fleet.runtime.subscribe_refusal.clone().expect("refused");
        assert!(refusal.1.contains("cannot watch itself"), "{refusal:?}");
        subscribe_to(&mut fleet, "q", R, Mode::Report);
        let refusal = fleet.runtime.subscribe_refusal.clone().expect("refused");
        assert!(refusal.1.contains("watcher itself"), "{refusal:?}");

        fleet.sessions.get_mut(&sid("d")).expect("d").title = Some("CC,HD - Demo d".to_owned());
        subscribe_to(&mut fleet, "q", "CC,HD - Demo d", Mode::Report);
        assert_eq!(
            fleet.runtime.subscriptions.len(),
            2,
            "d gets its own watcher"
        );
        assert_eq!(
            fleet.runtime.subscriptions[&sid("q")].target,
            sid("d"),
            "the exact title resolved to d's id"
        );
    }

    #[test]
    fn waiting_root_gets_the_send_on_stop() {
        let mut fleet = world();
        subscribe(&mut fleet, Mode::Act);
        produce(&mut fleet, "abc");
        let _ = send_hook(&mut fleet, hook(C, None, stage("zzz")));
        let finding = fleet
            .runtime
            .findings
            .values()
            .find(|finding| finding.kind == FindingKind::CheckFailed)
            .expect("finding")
            .id
            .clone();
        let request = ActRequest {
            session: sid(R),
            token: MessagingToken(TOKEN.to_owned()),
            finding: finding.clone(),
            action: Move::Send,
        };
        let _ = apply(
            &mut fleet,
            Event::RuntimeActRequested {
                request,
                at: at(12),
            },
        );
        let _ = apply(
            &mut fleet,
            Event::ClaimRecorded {
                finding: finding.clone(),
                at: at(13),
            },
        );

        let stop = hook(C, None, HookKind::Stop);
        let probe = Probe::of(&stop);
        let _ = apply(&mut fleet, Event::Hook(stop));
        let answer = verdict(&fleet, &probe);
        match &answer.verdict {
            Verdict::Block { reason } => {
                assert!(reason.contains("pass exactly the value"), "{reason}");
            }
            other => panic!("the waiting root's Stop carries the recovery: {other:?}"),
        }
        assert_eq!(answer.handed_over, Some(finding));
    }

    #[test]
    fn watcher_is_told_when_its_session_ends() {
        let mut fleet = world();
        subscribe(&mut fleet, Mode::Act);
        let actions = apply(
            &mut fleet,
            Event::Hook(hook(
                C,
                None,
                HookKind::SessionEnd {
                    reason: "prompt_input_exit".to_owned(),
                },
            )),
        );
        let farewell = actions
            .iter()
            .find_map(|action| match action {
                Action::DeliverRuntimeDigest(digest) => Some(digest),
                _ => None,
            })
            .expect("a last digest to the watcher");
        assert_eq!(farewell.session, sid(R));
        assert!(
            farewell
                .text
                .contains("watch ended — the watched session ended"),
            "{}",
            farewell.text
        );
        assert!(fleet.runtime.subscriptions.is_empty(), "the watch is gone");
    }

    #[test]
    fn unenforced_skill_opens_no_run() {
        let mut fleet = world();
        let _ = send_hook(
            &mut fleet,
            hook(
                C,
                None,
                HookKind::UserPromptSubmit {
                    prompt: "/qq hi".to_owned(),
                },
            ),
        );
        assert!(fleet.runtime.open.get(&sid(C)).is_none_or(Vec::is_empty));
        assert!(fleet.runtime.runs.values().all(|run| !run.is_open()));
    }

    #[test]
    fn fill_placeholders() {
        let finding = Finding {
            id: FindingId("r1-x-1".to_owned()),
            kind: FindingKind::CheckFailed,
            run: RunId(1),
            session: sid(C),
            agent: None,
            skill: "demo".to_owned(),
            version: "1.0.0".to_owned(),
            entry: "x".to_owned(),
            reason: "why".to_owned(),
            evidence: vec!["a".to_owned(), "b".to_owned()],
            template: String::new(),
            acted: Acted::Open,
        };
        assert_eq!(
            render(
                "{finding} {skill} {version} {check} {entry} {reason} {evidence} {other}",
                &finding
            ),
            "r1-x-1 demo 1.0.0 x x why a; b {other}"
        );
    }
}
