//! What `apply` asks the edges to do; each action has one `Executor::execute` arm.

use std::path::PathBuf;

use crate::action_ledger::Record;
use crate::ids::{AgentId, DigestId, FleetId, MessagingToken, Pid, SessionId};
use crate::repo::{RepoId, WorkItemKey};
use crate::runtime::FindingId;
use crate::wire::FleetArchive;

/// One request from `apply` to an edge, executed by the engine's `Executor`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Action {
    /// Register a process-exit watch for a registry pid.
    WatchExit {
        /// The process.
        pid: Pid,
    },
    /// Make these the git-watched repos (git watch edge); the execution reports the `git` sensor.
    WatchRepos {
        /// Every registered repository.
        repos: Vec<RepoId>,
    },
    /// Run one agentwt scan of this repo (agentwt runner edge).
    ScanRepo {
        /// The repository.
        repo: RepoId,
        /// Extra train bases.
        train_bases: Vec<String>,
        /// Branches tracked as containment candidates on request (the fleets' merge branches).
        candidates: Vec<String>,
    },
    /// Re-read one plan dir (agentwt runner edge).
    ParsePlan {
        /// The repository.
        repo: RepoId,
        /// The plan dir.
        item: WorkItemKey,
        /// Whose tip decides which design files are new, with `train_bases`.
        default_branch: String,
        /// The requested train bases (fleet targets and merge branches): a design file one of them holds is merged work.
        train_bases: Vec<String>,
    },
    /// Fetch a subscriber's spawn tree from peek; requests arriving during a round coalesce into the next.
    FetchPeek {
        /// The subscriber session.
        session: SessionId,
    },
    /// Claim, then send one digest (socket edge).
    DeliverDigest(Digest),
    /// Append one record to the action ledger (socket edge): subscriptions, membership, journal.
    Record(Record),
    /// Write a dissolved run's archive file, then append its `dissolved` record durably (socket edge).
    Dissolve {
        /// The run, in full.
        archive: Box<FleetArchive>,
        /// The `dissolved` record.
        record: Record,
    },
    /// Read a subagent's model from the head of its transcript (transcript edge).
    ReadAgentModel(ModelRead),
    /// Claim, then send one runtime digest to the runtime subscriber (socket edge).
    DeliverRuntimeDigest(RuntimeDigest),
    /// Append a move's claim durably, then report `ClaimRecorded` or `ClaimFailed` (socket edge).
    Claim {
        /// The finding the move answers.
        finding: FindingId,
        /// The `runtime_claimed` record.
        record: Record,
    },
    /// Claim a waiter's landing grant durably, then tell the waiter once (socket edge).
    AnnounceGrant(GrantNotice),
}

/// A runtime digest ready to claim and send.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RuntimeDigest {
    /// Digest id (the runtime's own sequence).
    pub id: DigestId,
    /// The subscriber.
    pub session: SessionId,
    /// The subscriber's messaging socket.
    pub socket: PathBuf,
    /// The subscriber's messaging token.
    pub token: MessagingToken,
    /// The rendered text.
    pub text: String,
    /// Line count, for the claim record.
    pub lines: u32,
}

/// A landing grant to a waiter, ready to claim and announce.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GrantNotice {
    /// The granted session.
    pub session: SessionId,
    /// Its messaging socket from the registry; `None` when the registry named none — the grant is then recorded and
    /// not announced.
    pub socket: Option<PathBuf>,
    /// Its messaging token, from its landing request.
    pub token: MessagingToken,
    /// The grant message.
    pub text: String,
    /// The `land_granted` record: the claim, written and synced before the send.
    pub record: Record,
}

/// One transcript-head read.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ModelRead {
    /// The root session.
    pub session: SessionId,
    /// The subagent.
    pub agent: AgentId,
    /// The root session's `transcript_path`; the subagent's transcript lies under its stem.
    pub transcript: PathBuf,
}

/// A digest ready to claim and send.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Digest {
    /// Digest id.
    pub id: DigestId,
    /// The subscriber.
    pub session: SessionId,
    /// The subscriber's fleet.
    pub fleet: FleetId,
    /// The subscriber's messaging socket.
    pub socket: PathBuf,
    /// The subscriber's messaging token.
    pub token: MessagingToken,
    /// The rendered text.
    pub text: String,
    /// Item count, for the claim record.
    pub items: u32,
    /// Fleet-level item keys, for the claim record.
    pub keys: Vec<String>,
}
