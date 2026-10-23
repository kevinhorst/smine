//! The engine: one synchronous loop, the sole owner of the [`Fleet`] (concept D29). Everything enters through one
//! bounded FIFO inbox — edge events, hook bodies and status queries — so a query answers from exactly the events
//! enqueued before it, and a hook body is answered with its verdict right after it and its follow-ups were applied:
//! the heart attack's ([`crate::attack::verdict`]) when it objects, else the member guard's ([`crate::member::verdict`])
//! when it objects, else the runtime's ([`crate::runtime::verdict`]) — a session that muted heartbeat
//! (`crate::switches`) gets only the heart attack's; a runtime hand-over is applied
//! (`TemplateHandedOver`) before the reply leaves. The actions `apply` returns are executed here by [`Executor`]; an
//! execution that
//! yields an event synchronously (a pid already gone) feeds it back through a local queue, never through the inbox: the engine must never block on
//! its own full inbox.

use std::collections::VecDeque;

use jiff::Timestamp;
use tokio::sync::{mpsc, oneshot};

use crate::action::{Action, ModelRead};
use crate::apply::apply;
use crate::attack::{self, Probe, Verdict};
use crate::edges::agentwt::Runner;
use crate::edges::exits::{ExitWatch, Watch};
use crate::edges::git;
use crate::edges::socket::Job;
use crate::event::{Event, ExitSighting, HookEvent};
use crate::fleet::{Fleet, Join, SensorName, SensorState};
use crate::ids::SessionId;
use crate::land::{self, LandAnswer, LandCaller};
use crate::log;
use crate::member;
use crate::runtime::{self, ActAnswer, FindingId, SubscriptionAnswer};
use crate::status::{self, Filter};
use crate::wire::{Health, RuntimeDocument, StatusDocument, SubscriptionRow};

/// One inbox message; the inbox is bounded (`inbox_capacity`): when the engine falls behind, edges wait (hook acks slow
/// down) rather than drop.
#[derive(Debug)]
pub enum Input {
    /// An event for `apply`.
    Event(Event),
    /// A hook body from `POST /hook`, answered with its verdict once applied.
    Hook {
        /// The parsed body.
        event: HookEvent,
        /// Where the verdict goes.
        reply: oneshot::Sender<Verdict>,
    },
    /// A `/status` query, answered from the fleet at this point of the queue.
    Query {
        /// Which sessions.
        filter: Filter,
        /// Where the document goes.
        reply: oneshot::Sender<StatusDocument>,
    },
    /// The reason the session's newest join was refused, for the `/join` answer.
    JoinRefusal {
        /// The session.
        session: SessionId,
        /// Where the reason goes; `None` when no refusal is recorded.
        reply: oneshot::Sender<Option<String>>,
    },
    /// The session's join record, for the `/done` answer.
    JoinOf {
        /// The session.
        session: SessionId,
        /// Where the record goes; `None` when the session never joined.
        reply: oneshot::Sender<Option<Join>>,
    },
    /// The session's subscription row, for the `/subscribe` and `/unsubscribe` answers: read directly, because an
    /// unsubscribe that dissolves the fleet leaves no fleet row to find it in.
    SubscriptionOf {
        /// The session.
        session: SessionId,
        /// Where the row goes; `None` when the session holds no subscription.
        reply: oneshot::Sender<Option<SubscriptionRow>>,
    },
    /// A `/runtime` query: the runtime document at this point of the queue.
    RuntimeQuery {
        /// Where the document goes.
        reply: oneshot::Sender<RuntimeDocument>,
    },
    /// The session's runtime subscription or its latest refusal, for the `/runtime/subscribe` and
    /// `/runtime/unsubscribe` answers.
    RuntimeSubscriptionOf {
        /// The session.
        session: SessionId,
        /// Where the answer goes.
        reply: oneshot::Sender<SubscriptionAnswer>,
    },
    /// A finding's act state or its latest refusal, for the `/runtime/act` answer.
    RuntimeActOf {
        /// The finding.
        finding: FindingId,
        /// Where the answer goes.
        reply: oneshot::Sender<ActAnswer>,
    },
    /// The land answer kept for `caller`, for the `/done`, `/land`, `/land/end` and `/land/release` answers.
    LandAnswerOf {
        /// The session or operator that asked.
        caller: LandCaller,
        /// Where the answer goes; `None` when the kept answer is another caller's.
        reply: oneshot::Sender<Option<LandAnswer>>,
    },
}

/// The fleet owner.
pub struct Engine {
    /// The state.
    pub fleet: Fleet,
    /// Header of every status document.
    pub health: Health,
    /// Runs the actions `apply` returns.
    pub executor: Executor,
}

/// Executes actions against the edges that own them; P2 and P3 add one field and one arm per action edge.
pub struct Executor {
    /// The process-exit edge.
    pub exits: ExitWatch,
    /// The agentwt runner edge.
    pub runner: Runner,
    /// The git watch edge; `None` without `--repos`.
    pub git: Option<git::Watch>,
    /// The peek edge: subscriber sessions whose spawn tree to fetch.
    pub peek: mpsc::UnboundedSender<SessionId>,
    /// The socket edge: ledger records and digests, in order.
    pub socket: std::sync::mpsc::Sender<Job>,
    /// The transcript edge: subagent model reads.
    pub transcript: std::sync::mpsc::Sender<ModelRead>,
}

impl Engine {
    /// Runs until every inbox sender is gone. Blocking: run it on `spawn_blocking`.
    pub fn run(mut self, mut inbox: mpsc::Receiver<Input>) {
        let mut follow_ups = VecDeque::new();
        while let Some(input) = inbox.blocking_recv() {
            match input {
                Input::Event(event) => self.drain(event, &mut follow_ups),
                Input::Hook { event, reply } => {
                    let probe = Probe::of(&event);
                    let muted = event.muted;
                    self.drain(Event::Hook(event), &mut follow_ups);
                    let answer = hook_answer(&self.fleet, &probe, muted);
                    if let Some(finding) = answer.handed_over {
                        let handed_over = Event::TemplateHandedOver {
                            finding,
                            at: Timestamp::now(),
                        };
                        self.drain(handed_over, &mut follow_ups);
                    }
                    let _ = reply.send(answer.verdict);
                }
                Input::RuntimeQuery { reply } => {
                    let _ = reply.send(runtime::document(&self.fleet, &self.health));
                }
                Input::RuntimeSubscriptionOf { session, reply } => {
                    let _ = reply.send(runtime::subscription_of(&self.fleet, &session));
                }
                Input::RuntimeActOf { finding, reply } => {
                    let _ = reply.send(runtime::act_of(&self.fleet, &finding));
                }
                Input::Query { filter, reply } => {
                    let _ = reply.send(status::document(&self.fleet, &filter, &self.health));
                }
                Input::JoinRefusal { session, reply } => {
                    let reason = self.fleet.join_refusals.get(&session);
                    let _ = reply.send(reason.map(|(_, reason)| reason.clone()));
                }
                Input::JoinOf { session, reply } => {
                    let _ = reply.send(self.fleet.joins.get(&session).cloned());
                }
                Input::SubscriptionOf { session, reply } => {
                    let row = self
                        .fleet
                        .subscriptions
                        .get_key_value(&session)
                        .map(SubscriptionRow::from);
                    let _ = reply.send(row);
                }
                Input::LandAnswerOf { caller, reply } => {
                    let _ = reply.send(land::answer_of(&self.fleet, &caller));
                }
            }
        }
    }

    /// Applies `event` and every follow-up its actions produce, before the next inbox message.
    fn drain(&mut self, event: Event, follow_ups: &mut VecDeque<Event>) {
        follow_ups.push_back(event);
        while let Some(next) = follow_ups.pop_front() {
            for action in apply(&mut self.fleet, next) {
                follow_ups.extend(self.executor.execute(action));
            }
        }
    }
}

/// The verdict chain of one hook: the heart attack always; the member guard and the runtime only while the session has
/// not muted heartbeat — a muted hook is neither judged nor counted.
fn hook_answer(fleet: &Fleet, probe: &Probe, muted: bool) -> runtime::Answer {
    let settled = |verdict: Verdict| runtime::Answer {
        verdict,
        handed_over: None,
    };
    match attack::verdict(fleet, probe) {
        Verdict::Allow if muted => settled(Verdict::Allow),
        Verdict::Allow => match member::verdict(fleet, probe) {
            Verdict::Allow => runtime::verdict(fleet, probe),
            guarded @ (Verdict::Deny { .. }
            | Verdict::Stop { .. }
            | Verdict::Context { .. }
            | Verdict::Block { .. }) => settled(guarded),
        },
        forbidden @ (Verdict::Deny { .. }
        | Verdict::Stop { .. }
        | Verdict::Context { .. }
        | Verdict::Block { .. }) => settled(forbidden),
    }
}

impl Executor {
    /// Executes one action; returns the event it produced synchronously, if any.
    #[must_use]
    pub fn execute(&self, action: Action) -> Option<Event> {
        match action {
            Action::WatchExit { pid } => match self.exits.watch(pid) {
                Ok(Watch::Registered) => None,
                Ok(Watch::AlreadyGone) => Some(Event::ProcessExited {
                    pid,
                    at: Timestamp::now(),
                    sighting: ExitSighting::AlreadyGone,
                }),
                Err(err) => {
                    log::line(format_args!("watch pid {pid}: {err}"));
                    None
                }
            },
            Action::WatchRepos { repos } => self.git.as_ref().map(|watch| {
                let failure = watch.set_repos(&repos);
                if let Some(err) = &failure {
                    log::line(format_args!("{err}"));
                }
                Event::SensorChanged {
                    sensor: SensorName::Git,
                    state: failure
                        .as_ref()
                        .map_or(SensorState::Ok, |_| SensorState::Degraded),
                    detail: failure.map(|err| err.to_string()),
                    at: Timestamp::now(),
                }
            }),
            Action::ScanRepo {
                repo,
                train_bases,
                candidates,
            } => {
                self.runner.scan(repo, train_bases, candidates);
                None
            }
            Action::ParsePlan {
                repo,
                item,
                default_branch,
                train_bases,
            } => {
                self.runner.parse(repo, item, default_branch, train_bases);
                None
            }
            Action::FetchPeek { session } => {
                let _ = self.peek.send(session);
                None
            }
            Action::DeliverDigest(digest) => {
                let _ = self.socket.send(Job::Deliver(digest));
                None
            }
            Action::Record(record) => {
                let _ = self.socket.send(Job::Record(record));
                None
            }
            Action::Dissolve { archive, record } => {
                let _ = self.socket.send(Job::Dissolve { archive, record });
                None
            }
            Action::ReadAgentModel(read) => {
                let _ = self.transcript.send(read);
                None
            }
            Action::DeliverRuntimeDigest(digest) => {
                let _ = self.socket.send(Job::DeliverRuntime(digest));
                None
            }
            Action::Claim { finding, record } => {
                let _ = self.socket.send(Job::Claim { finding, record });
                None
            }
            Action::AnnounceGrant(notice) => {
                let _ = self.socket.send(Job::Grant(notice));
                None
            }
        }
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use serde_json::{Value, json};

    use super::*;
    use crate::config::ForbiddenModels;
    use crate::event::{HookKind, ToolCall};
    use crate::fleet::{FleetInstance, Session, SessionState};
    use crate::ids::{FleetId, InstanceId, MemberId, MessagingToken};

    const MEMBER: &str = "m";

    /// `MEMBER` joined to fleet T1 as member C; `*fable*` is a forbidden subagent model.
    fn fleet() -> Fleet {
        let at = Timestamp::UNIX_EPOCH;
        let mut fleet = Fleet::new(at);
        let name = FleetId("T1".to_owned());
        let mut instance = FleetInstance::new(InstanceId(uuid::Uuid::nil()), name.clone(), at);
        instance
            .member_ids
            .by_member
            .insert((SessionId(MEMBER.to_owned()), None), MemberId::nth(2));
        fleet.instances.insert(name.clone(), instance);
        let join = Join {
            fleet: name,
            token: MessagingToken("t".to_owned()),
            since: at,
        };
        fleet.joins.insert(SessionId(MEMBER.to_owned()), join);
        let session = Session::new(SessionId(MEMBER.to_owned()), SessionState::Idle, at, false);
        fleet.sessions.insert(SessionId(MEMBER.to_owned()), session);
        fleet.forbidden_models =
            ForbiddenModels::new(&["*fable*".to_owned()]).expect("patterns compile");
        fleet
    }

    /// The member root's `PreToolUse` probe of `tool` with `input`.
    fn pre_tool(tool: &str, input: Value) -> Probe {
        let hook = HookEvent {
            session_id: SessionId(MEMBER.to_owned()),
            agent: None,
            cwd: None,
            transcript_path: None,
            at: Timestamp::UNIX_EPOCH,
            kind: HookKind::PreToolUse(ToolCall {
                tool_name: tool.to_owned(),
                input,
            }),
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        };
        Probe::of(&hook)
    }

    #[test]
    fn hook_answer_mutes() {
        let fleet = fleet();
        let send = || pre_tool("SendMessage", json!({}));
        let spawn = || {
            pre_tool(
                "Agent",
                json!({ "subagent_type": "Plan", "prompt": "p", "model": "fable" }),
            )
        };
        let is_member_deny = |verdict: &Verdict| matches!(verdict, Verdict::Deny { reason } if reason.contains("T1-MC is a fleet member"));
        let is_model_deny = |verdict: &Verdict| matches!(verdict, Verdict::Deny { reason } if reason.contains("forbidden subagent model"));
        let cases = [
            ("member-send-denied-unmuted", send(), false, 'd'),
            ("member-send-allowed-muted", send(), true, 'a'),
            ("attack-still-denies-muted", spawn(), true, 'f'),
        ];
        for (label, probe, muted, want) in cases {
            let answer = hook_answer(&fleet, &probe, muted);
            let is_match = match want {
                'd' => is_member_deny(&answer.verdict),
                'f' => is_model_deny(&answer.verdict),
                _ => answer.verdict == Verdict::Allow,
            };
            assert!(is_match, "{label}: {:?}", answer.verdict);
            assert_eq!(answer.handed_over, None, "{label}");
        }
    }
}
