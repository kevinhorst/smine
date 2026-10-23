//! The action ledger, `<claude home>/heartbeat/actions.jsonl` (concept D15): append-only JSON lines, the one
//! artifact heartbeat persists. A subscription is recorded when it starts or ends; every digest is claimed — durably —
//! before its send, and its outcome appended after. Fleet membership is recorded too: joins with their letter, the
//! letters of workers and their registrations, deliveries and resumes, the orchestrators' journal entries, and each
//! fleet run's start, its members leaving and its dissolve. The runtime checker records its subscription (start and
//! end), every move's claim (synced before the move goes live), each send's hand-over, and its digests' claims and
//! outcomes; replay rebinds only the runtime subscription. Landing turns record each request, grant and end; replay
//! feeds requests and ends back in ledger order and re-derives the grants, a granted line being only the claim of its
//! grant message.
//! [`replay`] folds the file back into engine events at startup. Pure: the socket edge owns the file.
//!
//! Known limit: a fleet-level key muted by a claim stays muted after a restart until the fact is seen to go away once;
//! a resolved fact that recurs during the downtime is not re-announced.

use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use crate::event::{Event, JoinSource};
use crate::fleet::{JournalEntry, Outcome, SubscribeRequest, SubscriptionSource};
use crate::ids::{
    AgentId, DigestId, FleetId, InstanceId, MemberId, MessagingToken, SessionId, WorkerBranch,
};
use crate::land::{LandEnd, LandOutcome, LandSource, LandTarget};
use crate::protocol::ACTION_LEDGER_VERSION as VERSION;
use crate::repo::RepoId;
use crate::runtime::{FindingId, Mode, Move, RuntimeSubscribeRequest};
use crate::wire::DigestOutcomeWire;

/// One ledger line.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Line {
    /// [`crate::protocol::ACTION_LEDGER_VERSION`].
    pub v: u32,
    /// When the record was written (RFC 3339).
    pub at: String,
    /// The record.
    #[serde(flatten)]
    pub record: Record,
}

/// What a line records.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum Record {
    /// A subscription started (or replaced the session's earlier one), with the fleet's binding.
    Subscribed {
        /// The subscriber.
        session_id: SessionId,
        /// Its fleet.
        fleet: FleetId,
        /// The bound repository (main-checkout path).
        repo: PathBuf,
        /// The fleet's integration branch.
        target: String,
        /// The subscriber's messaging socket.
        socket_path: PathBuf,
        /// The subscriber's messaging token.
        token: MessagingToken,
        /// The run hears collisions with branches outside the fleet too.
        #[serde(default)]
        outside_collisions: bool,
    },
    /// A subscription ended.
    Unsubscribed {
        /// The subscriber.
        session_id: SessionId,
        /// Why.
        reason: String,
    },
    /// A digest is about to be sent; written and synced before the send.
    Claimed {
        /// The digest.
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
        /// Its fleet.
        fleet: FleetId,
        /// Item count.
        items: u32,
        /// Fleet-level item keys the digest announces.
        #[serde(default, skip_serializing_if = "Vec::is_empty")]
        keys: Vec<String>,
    },
    /// The digest was written in full.
    Sent {
        /// The digest.
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
    },
    /// Nothing was sent.
    Refused {
        /// The digest.
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
        /// Why.
        detail: String,
    },
    /// The write failed after the claim.
    Unconfirmed {
        /// The digest.
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
        /// Why.
        detail: String,
    },
    /// A session joined a fleet with its letter.
    Joined {
        /// The session.
        session_id: SessionId,
        /// The fleet.
        fleet: FleetId,
        /// Its letter.
        member: MemberId,
        /// Its messaging token, for `done`.
        token: MessagingToken,
    },
    /// A worker got its letter.
    MemberAssigned {
        /// The fleet.
        fleet: FleetId,
        /// The subscriber that registered the worker.
        session_id: SessionId,
        /// The worker's branch.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        worker: Option<WorkerBranch>,
        /// The subagent of a spawn-tree member, in lines written before workers were registered; read, never written.
        #[serde(default, skip_serializing)]
        agent_id: Option<AgentId>,
        /// Its letter.
        member: MemberId,
    },
    /// The fleet's subscriber registered a worker branch, or changed its delivery.
    Worker {
        /// The fleet.
        fleet: FleetId,
        /// The subscriber.
        session_id: SessionId,
        /// The worker's branch.
        branch: WorkerBranch,
        /// Whether its work is reported finished.
        delivered: bool,
    },
    /// A joined session reported done.
    Delivered {
        /// The session.
        session_id: SessionId,
    },
    /// A delivered session started a new turn.
    Resumed {
        /// The session.
        session_id: SessionId,
    },
    /// An orchestrator's journal entry.
    Journal {
        /// The fleet.
        fleet: FleetId,
        /// The entry.
        entry: JournalEntry,
    },
    /// A fleet run started: its first join or subscribe was admitted.
    FleetStarted {
        /// The fleet name.
        fleet: FleetId,
        /// The run's UUID.
        instance: InstanceId,
    },
    /// A joined session left its fleet.
    Left {
        /// The session.
        session_id: SessionId,
        /// Why: `left`, `the session ended`, ….
        reason: String,
    },
    /// The operator removed a lettered member from its fleet's live run.
    Removed {
        /// The fleet.
        fleet: FleetId,
        /// The removed letter.
        member: MemberId,
    },
    /// A fleet run dissolved; written after its archive file.
    Dissolved {
        /// The fleet name.
        fleet: FleetId,
        /// The dissolved run.
        instance: InstanceId,
    },
    /// A session took a runtime watch over one observed session.
    RuntimeSubscribed {
        /// The subscriber.
        session_id: SessionId,
        /// The watched session; absent on lines written before watches had a target, which replay drops.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        target: Option<SessionId>,
        /// Report or act.
        mode: Mode,
        /// The subscriber's messaging socket.
        socket_path: PathBuf,
        /// The subscriber's messaging token (its own, as fleet subscriptions record theirs).
        token: MessagingToken,
    },
    /// The runtime subscription ended.
    RuntimeUnsubscribed {
        /// The subscriber.
        session_id: SessionId,
        /// Why.
        reason: String,
    },
    /// A move on a finding is claimed; written and synced before the move goes live.
    RuntimeClaimed {
        /// The finding.
        finding: FindingId,
        /// The observed session.
        target: SessionId,
        /// The subagent the move targets; absent for the root.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        agent: Option<AgentId>,
        /// The move.
        #[serde(rename = "move")]
        action: Move,
    },
    /// A claimed send was handed over in a hook answer.
    RuntimeHandedOver {
        /// The finding.
        finding: FindingId,
    },
    /// A runtime digest is about to be sent; written and synced before the send.
    RuntimeDigestClaimed {
        /// The digest (the runtime's own sequence).
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
        /// Line count.
        lines: u32,
    },
    /// A claimed runtime digest's outcome.
    RuntimeDigestOutcome {
        /// The digest.
        digest: DigestId,
        /// The subscriber.
        session_id: SessionId,
        /// Sent, refused or unconfirmed.
        outcome: DigestOutcomeWire,
        /// Why, when refused or unconfirmed.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        detail: Option<String>,
    },
    /// A session asked for a landing turn and was admitted.
    LandRequested {
        /// The session.
        session_id: SessionId,
        /// The repository (main-checkout path).
        repo: PathBuf,
        /// The landing target.
        target: String,
        /// The session's messaging token, for its later calls and its grant message.
        token: MessagingToken,
    },
    /// A session got the landing turn; for a waiter, written and synced before its grant message is sent.
    LandGranted {
        /// The session.
        session_id: SessionId,
        /// The repository.
        repo: PathBuf,
        /// The landing target.
        target: String,
    },
    /// A session's landing entry ended.
    LandEnded {
        /// The session.
        session_id: SessionId,
        /// The repository.
        repo: PathBuf,
        /// The landing target.
        target: String,
        /// How.
        outcome: LandOutcome,
    },
}

impl Record {
    /// The outcome record of a claimed `digest`; `None` for `Unclaimed`, whose claim never reached the file.
    #[must_use]
    pub fn outcome(digest: DigestId, session_id: SessionId, outcome: &Outcome) -> Option<Record> {
        match outcome.clone() {
            Outcome::Sent => Some(Record::Sent { digest, session_id }),
            Outcome::Refused(detail) => Some(Record::Refused {
                digest,
                session_id,
                detail,
            }),
            Outcome::Unconfirmed(detail) => Some(Record::Unconfirmed {
                digest,
                session_id,
                detail,
            }),
            Outcome::Unclaimed(_) => None,
        }
    }

    /// The outcome record of a claimed runtime `digest`; `None` for `Unclaimed`, whose claim never reached the file.
    #[must_use]
    pub fn runtime_outcome(
        digest: DigestId,
        session_id: SessionId,
        outcome: &Outcome,
    ) -> Option<Record> {
        let detail = match outcome {
            Outcome::Sent => None,
            Outcome::Refused(detail) | Outcome::Unconfirmed(detail) => Some(detail.clone()),
            Outcome::Unclaimed(_) => return None,
        };
        Some(Record::RuntimeDigestOutcome {
            digest,
            session_id,
            outcome: DigestOutcomeWire::from(outcome),
            detail,
        })
    }

    /// The line bytes, newline-terminated.
    pub fn encode(self, at: Timestamp) -> crate::Result<Vec<u8>> {
        let mut bytes = serde_json::to_vec(&Line {
            v: VERSION,
            at: at.to_string(),
            record: self,
        })?;
        bytes.push(b'\n');
        Ok(bytes)
    }
}

/// The ledger folded into startup events.
#[derive(Debug, Default, PartialEq)]
pub struct Replay {
    /// Membership and journal events in ledger order, then `Subscribed` (source replay) per live subscription, then
    /// `Delivered` per session's last digest.
    pub events: Vec<Event>,
    /// Lines that did not parse or carry another version.
    pub skipped: u32,
}

/// A replayed subscription: its record time, request, repo and announced keys.
type Replayed = (Timestamp, SubscribeRequest, RepoId, BTreeSet<String>);

/// A session's last claim: its time, id and outcome once recorded.
type LastClaim = (Timestamp, DigestId, Option<Outcome>);

/// Folds the ledger text; a torn or foreign line is skipped and counted, never fatal.
#[must_use]
pub fn replay(text: &str) -> Replay {
    let mut subscriptions: BTreeMap<SessionId, Replayed> = BTreeMap::new();
    let mut runtime: BTreeMap<SessionId, (Timestamp, RuntimeSubscribeRequest)> = BTreeMap::new();
    let mut last: BTreeMap<SessionId, LastClaim> = BTreeMap::new();
    let mut members: Vec<Event> = Vec::new();
    let mut skipped = 0;
    for raw in text.lines().filter(|raw| !raw.trim().is_empty()) {
        let Some((at, record)) = parse_line(raw) else {
            skipped += 1;
            continue;
        };
        match record {
            Record::Subscribed {
                session_id,
                fleet,
                repo,
                target,
                socket_path,
                token,
                outside_collisions,
            } => {
                let request = SubscribeRequest {
                    session: session_id.clone(),
                    fleet,
                    target,
                    socket: socket_path,
                    token,
                    outside_collisions,
                };
                subscriptions.insert(session_id, (at, request, RepoId(repo), BTreeSet::new()));
            }
            Record::Unsubscribed { session_id, .. } => {
                subscriptions.remove(&session_id);
            }
            Record::Claimed {
                digest,
                session_id,
                keys,
                ..
            } => {
                if let Some((_, _, _, announced)) = subscriptions.get_mut(&session_id) {
                    announced.extend(keys);
                }
                last.insert(session_id, (at, digest, None));
            }
            Record::Sent { digest, session_id } => {
                settle(&mut last, &session_id, digest, at, Outcome::Sent);
            }
            Record::Refused {
                digest,
                session_id,
                detail,
            } => settle(&mut last, &session_id, digest, at, Outcome::Refused(detail)),
            Record::Unconfirmed {
                digest,
                session_id,
                detail,
            } => settle(
                &mut last,
                &session_id,
                digest,
                at,
                Outcome::Unconfirmed(detail),
            ),
            subscription @ (Record::RuntimeSubscribed { .. }
            | Record::RuntimeUnsubscribed { .. }) => {
                fold_runtime(&mut runtime, subscription, at);
            }
            member => members.extend(member_event(member, at)),
        }
    }
    let mut events = members;
    events.extend(runtime.into_values().map(runtime_watch));
    events.extend(
        subscriptions
            .into_values()
            .map(|(at, request, repo, announced)| Event::Subscribed {
                request,
                source: SubscriptionSource::Replay { repo, announced },
                at,
            }),
    );
    events.extend(
        last.into_iter()
            .map(|(session, (at, digest, outcome))| Event::Delivered {
                session,
                digest,
                outcome: outcome.unwrap_or_else(|| {
                    Outcome::Unconfirmed(
                        "claimed; no outcome recorded before the restart".to_owned(),
                    )
                }),
                at,
            }),
    );
    Replay { events, skipped }
}

/// One ledger line of this [`VERSION`] with its time; `None` for a torn or foreign line.
fn parse_line(raw: &str) -> Option<(Timestamp, Record)> {
    serde_json::from_str::<Line>(raw)
        .ok()
        .filter(|line| line.v == VERSION)
        .and_then(|line| {
            line.at
                .parse::<Timestamp>()
                .ok()
                .map(|at| (at, line.record))
        })
}

/// A replayed runtime watch as its subscribe event.
fn runtime_watch((at, request): (Timestamp, RuntimeSubscribeRequest)) -> Event {
    Event::RuntimeSubscribed {
        request,
        replay: true,
        at,
    }
}

/// Folds a runtime watch record: each subscriber's latest subscribe stands until its unsubscribe; a subscribe without
/// a target (written before watches had one) watches nothing and is dropped.
fn fold_runtime(
    runtime: &mut BTreeMap<SessionId, (Timestamp, RuntimeSubscribeRequest)>,
    record: Record,
    at: Timestamp,
) {
    match record {
        Record::RuntimeSubscribed {
            session_id,
            target: Some(target),
            mode,
            socket_path,
            token,
        } => {
            let request = RuntimeSubscribeRequest {
                session: session_id.clone(),
                target,
                mode,
                socket: socket_path,
                token,
            };
            runtime.insert(session_id, (at, request));
        }
        Record::RuntimeSubscribed {
            session_id,
            target: None,
            ..
        }
        | Record::RuntimeUnsubscribed { session_id, .. } => {
            runtime.remove(&session_id);
        }
        Record::Subscribed { .. }
        | Record::Unsubscribed { .. }
        | Record::Claimed { .. }
        | Record::Sent { .. }
        | Record::Refused { .. }
        | Record::Unconfirmed { .. }
        | Record::Joined { .. }
        | Record::MemberAssigned { .. }
        | Record::Worker { .. }
        | Record::Delivered { .. }
        | Record::Resumed { .. }
        | Record::Journal { .. }
        | Record::FleetStarted { .. }
        | Record::Left { .. }
        | Record::Removed { .. }
        | Record::Dissolved { .. }
        | Record::RuntimeClaimed { .. }
        | Record::RuntimeHandedOver { .. }
        | Record::RuntimeDigestClaimed { .. }
        | Record::RuntimeDigestOutcome { .. }
        | Record::LandRequested { .. }
        | Record::LandGranted { .. }
        | Record::LandEnded { .. } => {}
    }
}

/// The replayed event of a member record (join, letter, delivery, resume, journal) or of a landing request or end;
/// `None` for subscription records and landing grants.
fn member_event(record: Record, at: Timestamp) -> Option<Event> {
    let event = match record {
        Record::Joined {
            session_id,
            fleet,
            member,
            token,
        } => Event::Joined {
            session: session_id,
            fleet,
            token,
            source: JoinSource::Replay { member },
            at,
        },
        Record::MemberAssigned {
            fleet,
            session_id,
            worker: Some(worker),
            member,
            ..
        } => Event::MemberAssigned {
            fleet,
            key: (session_id, Some(worker)),
            member,
            at,
        },
        Record::Worker {
            fleet,
            session_id,
            branch,
            delivered,
        } => Event::WorkerSet {
            fleet,
            branch,
            delivered,
            by: session_id,
            token: None,
            at,
        },
        Record::Delivered { session_id } => Event::DoneReported {
            session: session_id,
            token: None,
            at,
        },
        Record::Resumed { session_id } => Event::Resumed {
            session: session_id,
            at,
        },
        Record::Journal { fleet, entry } => Event::JournalRecorded {
            fleet,
            entry,
            token: None,
        },
        Record::FleetStarted { fleet, instance } => Event::FleetStarted {
            fleet,
            instance,
            at,
        },
        Record::Left { session_id, .. } => Event::Left {
            session: session_id,
            at,
        },
        Record::Removed { fleet, member } => Event::Removed { fleet, member, at },
        Record::Dissolved { fleet, instance } => Event::Dissolved {
            fleet,
            instance,
            at,
        },
        Record::LandRequested {
            session_id,
            repo,
            target,
            token,
        } => replayed_request(session_id, repo, target, token, at),
        Record::LandEnded { session_id, .. } => Event::LandEnded {
            end: LandEnd::Replayed {
                session: session_id,
            },
            at,
        },
        Record::MemberAssigned { worker: None, .. }
        | Record::Subscribed { .. }
        | Record::Unsubscribed { .. }
        | Record::Claimed { .. }
        | Record::Sent { .. }
        | Record::Refused { .. }
        | Record::Unconfirmed { .. }
        | Record::RuntimeSubscribed { .. }
        | Record::RuntimeUnsubscribed { .. }
        | Record::RuntimeClaimed { .. }
        | Record::RuntimeHandedOver { .. }
        | Record::RuntimeDigestClaimed { .. }
        | Record::RuntimeDigestOutcome { .. }
        | Record::LandGranted { .. } => return None,
    };
    Some(event)
}

/// The replayed turn request of a `land_requested` record: trusted, for the recorded repository and target.
fn replayed_request(
    session: SessionId,
    repo: PathBuf,
    target: String,
    token: MessagingToken,
    at: Timestamp,
) -> Event {
    Event::LandRequested {
        session,
        target: LandTarget::Named(target),
        token,
        source: LandSource::Replay { repo: RepoId(repo) },
        at,
    }
}

/// Records `outcome` when it answers the session's last claim.
fn settle(
    last: &mut BTreeMap<SessionId, LastClaim>,
    session: &SessionId,
    digest: DigestId,
    at: Timestamp,
    outcome: Outcome,
) {
    if let Some(entry) = last
        .get_mut(session)
        .filter(|(_, claimed, _)| *claimed == digest)
    {
        *entry = (at, digest, Some(outcome));
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    const TOKEN: &str = "feedc0defeedc0de";

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    fn sid() -> SessionId {
        SessionId("o".to_owned())
    }

    fn subscribed(target: &str) -> Record {
        Record::Subscribed {
            session_id: sid(),
            fleet: FleetId("T1".to_owned()),
            repo: PathBuf::from("/r1"),
            target: target.to_owned(),
            socket_path: PathBuf::from("/tmp/o.sock"),
            token: MessagingToken(TOKEN.to_owned()),
            outside_collisions: false,
        }
    }

    fn claimed(n: u64, keys: &[&str]) -> Record {
        Record::Claimed {
            digest: DigestId(n),
            session_id: sid(),
            fleet: FleetId("T1".to_owned()),
            items: 1,
            keys: keys.iter().map(|key| (*key).to_owned()).collect(),
        }
    }

    fn joined(session: &str, member: &str) -> Record {
        Record::Joined {
            session_id: SessionId(session.to_owned()),
            fleet: FleetId("T1".to_owned()),
            member: MemberId(member.to_owned()),
            token: MessagingToken(TOKEN.to_owned()),
        }
    }

    fn member_assigned(worker: Option<&str>, member: &str) -> Record {
        Record::MemberAssigned {
            fleet: FleetId("T1".to_owned()),
            session_id: sid(),
            worker: worker.map(|branch| WorkerBranch(branch.to_owned())),
            agent_id: None,
            member: MemberId(member.to_owned()),
        }
    }

    fn worker(branch: &str, delivered: bool) -> Record {
        Record::Worker {
            fleet: FleetId("T1".to_owned()),
            session_id: sid(),
            branch: WorkerBranch(branch.to_owned()),
            delivered,
        }
    }

    /// A `member_assigned` line as the ledger wrote it for a spawn-tree member before workers were registered.
    const LEGACY_MEMBER_ASSIGNED: &str = r#"{"v":1,"at":"2026-09-30T10:00:00Z","kind":"member_assigned","fleet":"T1","session_id":"o","agent_id":"abcf172b9e819cc00","member":"A"}"#;

    #[test]
    fn worker_records_round_trip() {
        let cases = [
            (
                "worker-record-round-trip",
                worker("claude/t1-a", true),
                r#""kind":"worker","fleet":"T1","session_id":"o","branch":"claude/t1-a","delivered":true"#,
            ),
            (
                "member-assigned-worker-round-trip",
                member_assigned(Some("claude/t1-a"), "A"),
                r#""kind":"member_assigned","fleet":"T1","session_id":"o","worker":"claude/t1-a","member":"A""#,
            ),
        ];
        for (label, record, wire) in cases {
            let bytes = record.clone().encode(at(1)).expect("encode");
            let line_text = String::from_utf8(bytes.clone()).expect("utf-8");
            assert!(line_text.contains(wire), "{label}: {line_text}");
            assert!(!line_text.contains("agent_id"), "{label}: {line_text}");
            let line: Line = serde_json::from_slice(&bytes).expect("decode");
            assert_eq!(line.record, record, "{label}");
        }
    }

    #[test]
    fn legacy_agent_id_line() {
        let line: Line =
            serde_json::from_str(LEGACY_MEMBER_ASSIGNED).expect("legacy-agent-id-line-parses");
        assert_eq!(
            line.record,
            Record::MemberAssigned {
                fleet: FleetId("T1".to_owned()),
                session_id: sid(),
                worker: None,
                agent_id: Some(AgentId("abcf172b9e819cc00".to_owned())),
                member: MemberId("A".to_owned()),
            },
            "legacy-agent-id-line-parses"
        );
        let replayed = replay(&format!("{LEGACY_MEMBER_ASSIGNED}\n"));
        assert_eq!(
            replayed,
            Replay::default(),
            "legacy-agent-id-line-replays-nothing"
        );
    }

    #[test]
    fn worker_record_folds_to_no_runtime_change() {
        let replayed = replay(&text(vec![
            runtime_subscribed("r"),
            worker("claude/t1-a", false),
        ]));
        let got: Vec<String> = replayed
            .events
            .iter()
            .map(|event| match event {
                Event::WorkerSet {
                    branch,
                    delivered,
                    by,
                    token: None,
                    ..
                } => format!("worker {} {delivered} by {}", branch.0, by.0),
                Event::RuntimeSubscribed { request, .. } => {
                    format!("runtime {}", request.session.0)
                }
                other => panic!("unexpected replay event {other:?}"),
            })
            .collect();
        assert_eq!(got, ["worker claude/t1-a false by o", "runtime r"]);
    }

    fn journal(subject: &str) -> Record {
        Record::Journal {
            fleet: FleetId("T1".to_owned()),
            entry: crate::fleet::JournalEntry {
                kind: crate::fleet::JournalKind::Merged,
                members: vec![MemberId("A".to_owned())],
                subject: subject.to_owned(),
                detail: None,
                by: sid(),
                at: at(9),
            },
        }
    }

    #[test]
    fn replay_member_events_before_subscriptions() {
        let replayed = replay(&text(vec![
            subscribed("main"),
            joined("w", "A"),
            member_assigned(Some("claude/t1-b"), "B"),
            Record::Delivered {
                session_id: SessionId("w".to_owned()),
            },
            Record::Resumed {
                session_id: SessionId("w".to_owned()),
            },
            journal("merged w"),
        ]));
        let kinds: Vec<&str> = replayed
            .events
            .iter()
            .map(|event| match event {
                Event::Joined { .. } => "joined",
                Event::MemberAssigned { .. } => "assigned",
                Event::DoneReported { .. } => "delivered",
                Event::Resumed { .. } => "resumed",
                Event::JournalRecorded { .. } => "journal",
                Event::Subscribed { .. } => "subscribed",
                _ => "other",
            })
            .collect();
        assert_eq!(
            kinds,
            [
                "joined",
                "assigned",
                "delivered",
                "resumed",
                "journal",
                "subscribed"
            ]
        );
        assert_eq!(replayed.skipped, 0);
    }

    fn text(records: Vec<Record>) -> String {
        records
            .into_iter()
            .enumerate()
            .map(|(index, record)| {
                let second = i64::try_from(index).expect("small index") + 1;
                String::from_utf8(record.encode(at(second)).expect("encode")).expect("utf-8")
            })
            .collect()
    }

    #[test]
    fn encode_round_trips() {
        let records = vec![
            subscribed("main"),
            Record::Unsubscribed {
                session_id: sid(),
                reason: "the session ended".to_owned(),
            },
            claimed(1, &["integrated:main"]),
            Record::Sent {
                digest: DigestId(1),
                session_id: sid(),
            },
            Record::Refused {
                digest: DigestId(2),
                session_id: sid(),
                detail: "gone".to_owned(),
            },
            Record::Unconfirmed {
                digest: DigestId(3),
                session_id: sid(),
                detail: "broken pipe".to_owned(),
            },
            joined("w", "A"),
            member_assigned(Some("claude/t1-b"), "B"),
            worker("claude/t1-b", true),
            Record::Delivered {
                session_id: SessionId("w".to_owned()),
            },
            Record::Resumed {
                session_id: SessionId("w".to_owned()),
            },
            journal("merged w"),
            Record::FleetStarted {
                fleet: FleetId("T1".to_owned()),
                instance: InstanceId::legacy(&FleetId("T1".to_owned()), at(1)),
            },
            Record::Left {
                session_id: SessionId("w".to_owned()),
                reason: "left".to_owned(),
            },
            Record::Dissolved {
                fleet: FleetId("T1".to_owned()),
                instance: InstanceId::legacy(&FleetId("T1".to_owned()), at(1)),
            },
        ];
        for record in records {
            let bytes = record.clone().encode(at(1)).expect("encode");
            assert_eq!(bytes.split(|byte| *byte == b'\n').count(), 2, "one line");
            assert_eq!(bytes.last(), Some(&b'\n'));
            let line: Line = serde_json::from_slice(&bytes).expect("decode");
            assert_eq!(line.v, VERSION);
            assert_eq!(line.record, record);
        }
    }

    /// The replay's subscription (target, repo, announced) and last delivery (digest, outcome).
    type Folded = (
        Option<(String, RepoId, BTreeSet<String>)>,
        Option<(DigestId, Outcome)>,
    );

    fn fold(replay: &Replay) -> Folded {
        let mut subscription = None;
        let mut delivered = None;
        for event in &replay.events {
            match event {
                Event::Subscribed {
                    request,
                    source: SubscriptionSource::Replay { repo, announced },
                    ..
                } => subscription = Some((request.target.clone(), repo.clone(), announced.clone())),
                Event::Delivered {
                    digest, outcome, ..
                } => delivered = Some((*digest, outcome.clone())),
                other => panic!("unexpected replay event {other:?}"),
            }
        }
        (subscription, delivered)
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn replay_cases() {
        let repo = RepoId(PathBuf::from("/r1"));
        let keys = |list: &[&str]| -> BTreeSet<String> {
            list.iter().map(|key| (*key).to_owned()).collect()
        };
        let unconfirmed =
            Outcome::Unconfirmed("claimed; no outcome recorded before the restart".to_owned());
        let torn = format!("{}{{\"v\":1,\"at\":\"2026-", text(vec![subscribed("main")]));
        let foreign = format!(
            "{}{}",
            text(vec![subscribed("main")]),
            "{\"v\":2,\"at\":\"2026-09-27T00:00:00Z\",\"kind\":\"sent\",\"digest\":1,\"session_id\":\"o\"}\n"
        );
        let cases: Vec<(&str, String, Folded, u32)> = vec![
            ("empty", String::new(), (None, None), 0),
            (
                "subscribe-restores-repo-and-target",
                text(vec![subscribed("develop")]),
                (Some(("develop".to_owned(), repo.clone(), keys(&[]))), None),
                0,
            ),
            (
                "subscribe-then-unsubscribe",
                text(vec![
                    subscribed("main"),
                    Record::Unsubscribed {
                        session_id: sid(),
                        reason: "ended".to_owned(),
                    },
                ]),
                (None, None),
                0,
            ),
            (
                "claim-without-outcome",
                text(vec![subscribed("main"), claimed(1, &[])]),
                (
                    Some(("main".to_owned(), repo.clone(), keys(&[]))),
                    Some((DigestId(1), unconfirmed.clone())),
                ),
                0,
            ),
            (
                "claim-then-sent",
                text(vec![
                    subscribed("main"),
                    claimed(1, &[]),
                    Record::Sent {
                        digest: DigestId(1),
                        session_id: sid(),
                    },
                ]),
                (
                    Some(("main".to_owned(), repo.clone(), keys(&[]))),
                    Some((DigestId(1), Outcome::Sent)),
                ),
                0,
            ),
            (
                "resubscribe-clears-announced",
                text(vec![
                    subscribed("main"),
                    claimed(1, &["integrated:main"]),
                    subscribed("develop"),
                ]),
                (
                    Some(("develop".to_owned(), repo.clone(), keys(&[]))),
                    Some((DigestId(1), unconfirmed.clone())),
                ),
                0,
            ),
            (
                "claim-keys-restored",
                text(vec![subscribed("main"), claimed(1, &["integrated:main"])]),
                (
                    Some(("main".to_owned(), repo.clone(), keys(&["integrated:main"]))),
                    Some((DigestId(1), unconfirmed)),
                ),
                0,
            ),
            (
                "torn-last-line",
                torn,
                (Some(("main".to_owned(), repo.clone(), keys(&[]))), None),
                1,
            ),
            (
                "other-version",
                foreign,
                (Some(("main".to_owned(), repo, keys(&[]))), None),
                1,
            ),
        ];
        for (label, input, want, skipped) in cases {
            let replayed = replay(&input);
            assert_eq!(fold(&replayed), want, "{label}");
            assert_eq!(replayed.skipped, skipped, "{label}");
        }
    }

    fn runtime_subscribed(session: &str) -> Record {
        Record::RuntimeSubscribed {
            session_id: SessionId(session.to_owned()),
            target: Some(SessionId(format!("{session}-target"))),
            mode: Mode::Act,
            socket_path: PathBuf::from("/tmp/r.sock"),
            token: MessagingToken(TOKEN.to_owned()),
        }
    }

    #[test]
    fn runtime_records_round_trip_and_replay_only_the_subscription() {
        let records = vec![
            runtime_subscribed("r"),
            Record::RuntimeClaimed {
                finding: FindingId("f1".to_owned()),
                target: SessionId("c".to_owned()),
                agent: Some(AgentId("a1".to_owned())),
                action: Move::Stop,
            },
            Record::RuntimeHandedOver {
                finding: FindingId("f1".to_owned()),
            },
            Record::RuntimeDigestClaimed {
                digest: DigestId(1),
                session_id: SessionId("r".to_owned()),
                lines: 2,
            },
            Record::RuntimeDigestOutcome {
                digest: DigestId(1),
                session_id: SessionId("r".to_owned()),
                outcome: DigestOutcomeWire::Sent,
                detail: None,
            },
            Record::RuntimeUnsubscribed {
                session_id: SessionId("r".to_owned()),
                reason: "unsubscribed by the session".to_owned(),
            },
        ];
        for record in &records {
            let bytes = record.clone().encode(at(1)).expect("encode");
            let line: Line = serde_json::from_slice(&bytes).expect("decode");
            assert_eq!(line.record, *record);
        }
        let claimed =
            String::from_utf8(records[1].clone().encode(at(1)).expect("encode")).expect("utf-8");
        assert!(claimed.contains(r#""kind":"runtime_claimed""#), "{claimed}");
        assert!(claimed.contains(r#""move":"stop""#), "{claimed}");
        let legacy = Record::RuntimeSubscribed {
            session_id: SessionId("old".to_owned()),
            target: None,
            mode: Mode::Act,
            socket_path: PathBuf::from("/tmp/o.sock"),
            token: MessagingToken(TOKEN.to_owned()),
        };
        let cases: Vec<(&str, Vec<Record>, Vec<&str>)> = vec![
            ("subscribed", vec![runtime_subscribed("r")], vec!["r"]),
            ("ended", records.clone(), vec![]),
            (
                "resubscribed",
                vec![
                    runtime_subscribed("r"),
                    Record::RuntimeUnsubscribed {
                        session_id: SessionId("r".to_owned()),
                        reason: "x".to_owned(),
                    },
                    runtime_subscribed("q"),
                ],
                vec!["q"],
            ),
            (
                "two watchers",
                vec![runtime_subscribed("r"), runtime_subscribed("q")],
                vec!["q", "r"],
            ),
            ("untargeted legacy line", vec![legacy], vec![]),
        ];
        for (label, input, want) in cases {
            let replayed = replay(&text(input));
            let got: Vec<&str> = replayed
                .events
                .iter()
                .map(|event| match event {
                    Event::RuntimeSubscribed {
                        request,
                        replay: true,
                        ..
                    } => request.session.0.as_str(),
                    other => panic!("{label}: unexpected replay event {other:?}"),
                })
                .collect();
            assert_eq!(got, want, "{label}");
        }
    }

    #[test]
    fn token_not_in_debug() {
        let replayed = replay(&text(vec![subscribed("main")]));
        assert_eq!(replayed.events.len(), 1);
        let shown = format!("{replayed:?}");
        assert!(!shown.contains(TOKEN), "{shown}");
    }

    fn land_requested(session: &str) -> Record {
        land_requested_for(session, "develop")
    }

    fn land_requested_for(session: &str, target: &str) -> Record {
        Record::LandRequested {
            session_id: SessionId(session.to_owned()),
            repo: PathBuf::from("/r1"),
            target: target.to_owned(),
            token: MessagingToken(TOKEN.to_owned()),
        }
    }

    fn land_granted(session: &str) -> Record {
        Record::LandGranted {
            session_id: SessionId(session.to_owned()),
            repo: PathBuf::from("/r1"),
            target: "develop".to_owned(),
        }
    }

    fn land_ended(session: &str, outcome: LandOutcome) -> Record {
        Record::LandEnded {
            session_id: SessionId(session.to_owned()),
            repo: PathBuf::from("/r1"),
            target: "develop".to_owned(),
            outcome,
        }
    }

    #[test]
    fn replay_land_records() {
        let cases: [(&str, Vec<Record>, Vec<Event>); 5] = [
            (
                "requested record",
                vec![land_requested("a")],
                vec![Event::LandRequested {
                    session: SessionId("a".to_owned()),
                    target: LandTarget::Named("develop".to_owned()),
                    token: MessagingToken(TOKEN.to_owned()),
                    source: LandSource::Replay {
                        repo: RepoId(PathBuf::from("/r1")),
                    },
                    at: at(1),
                }],
            ),
            (
                "requested record of another target",
                vec![land_requested_for("a", "feature/x")],
                vec![Event::LandRequested {
                    session: SessionId("a".to_owned()),
                    target: LandTarget::Named("feature/x".to_owned()),
                    token: MessagingToken(TOKEN.to_owned()),
                    source: LandSource::Replay {
                        repo: RepoId(PathBuf::from("/r1")),
                    },
                    at: at(1),
                }],
            ),
            (
                "ended record",
                vec![land_ended("a", LandOutcome::Landed)],
                vec![Event::LandEnded {
                    end: LandEnd::Replayed {
                        session: SessionId("a".to_owned()),
                    },
                    at: at(1),
                }],
            ),
            ("granted record", vec![land_granted("a")], vec![]),
            (
                "land and member records interleaved",
                vec![
                    land_requested("a"),
                    joined("w", "A"),
                    land_granted("a"),
                    land_ended("a", LandOutcome::Released),
                ],
                vec![
                    Event::LandRequested {
                        session: SessionId("a".to_owned()),
                        target: LandTarget::Named("develop".to_owned()),
                        token: MessagingToken(TOKEN.to_owned()),
                        source: LandSource::Replay {
                            repo: RepoId(PathBuf::from("/r1")),
                        },
                        at: at(1),
                    },
                    Event::Joined {
                        session: SessionId("w".to_owned()),
                        fleet: FleetId("T1".to_owned()),
                        token: MessagingToken(TOKEN.to_owned()),
                        source: JoinSource::Replay {
                            member: MemberId("A".to_owned()),
                        },
                        at: at(2),
                    },
                    Event::LandEnded {
                        end: LandEnd::Replayed {
                            session: SessionId("a".to_owned()),
                        },
                        at: at(4),
                    },
                ],
            ),
        ];
        for (label, records, want) in cases {
            let replayed = replay(&text(records));
            assert_eq!(replayed.events, want, "{label}");
            assert_eq!(replayed.skipped, 0, "{label}");
        }
    }

    #[test]
    fn land_records_round_trip() {
        let cases = [
            (land_requested("a"), "land_requested"),
            (land_granted("a"), "land_granted"),
            (land_ended("a", LandOutcome::SessionGone), "land_ended"),
        ];
        for (record, kind) in cases {
            let bytes = record.clone().encode(at(1)).expect("encode");
            let line: serde_json::Value = serde_json::from_slice(&bytes).expect("json");
            assert_eq!(line["kind"], kind, "{kind}");
            let back: Line = serde_json::from_slice(&bytes).expect("decode");
            assert_eq!(back.record, record, "{kind}");
        }
        let words = [
            (LandOutcome::Landed, "landed"),
            (LandOutcome::Abandoned, "abandoned"),
            (LandOutcome::Released, "released"),
            (LandOutcome::SessionGone, "session_gone"),
        ];
        for (outcome, word) in words {
            assert_eq!(
                serde_json::to_value(outcome).expect("outcome"),
                serde_json::Value::String(word.to_owned()),
                "{word}"
            );
        }
    }
}
