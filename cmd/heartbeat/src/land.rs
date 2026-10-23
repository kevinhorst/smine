//! Landing turns (merge strategy D5, targets D7): every landing target of a registered repository has one landing —
//! at most one session holds the turn, the others wait in request order. `heartbeat done` from a session without a
//! join requests the turn for the landing target of the session's own branch, `heartbeat land --target` the turn for
//! a named branch: the staging target always, else a local branch of the last scan that is neither agent-owned nor
//! the default branch. An entry ends only on explicit events: the session's own `land --end`, the operator's
//! `land --release`, or its session ending, dying or being unknown once the registry is seeded ([`settle`]). A root
//! prompt never touches an entry and no clock ends a turn.
//!
//! A session has at most one entry over all repositories and targets. Its first request stores its messaging token on
//! the entry; every later call must present it. A turn that frees goes to the first waiter in the same step.
//!
//! Replay feeds `land_requested` and `land_ended` records through [`on_requested`] and [`on_ended`] in ledger order
//! and re-derives every grant from them, with no record and no message; a `land_granted` record is the claim of the
//! grant message and never an input. A request record enters under its recorded target, an end record ends the
//! session's one entry. The answer of the last request, end or release is kept in one slot for the HTTP edge's
//! read-back ([`answer_of`]). Pure.

use std::collections::{BTreeMap, VecDeque};
use std::path::{Path, PathBuf};

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use crate::action::{Action, GrantNotice};
use crate::action_ledger::Record;
use crate::apply::terminal_word;
use crate::branch::Branch;
use crate::fleet::Fleet;
use crate::ids::{MessagingToken, SessionId};
use crate::protocol::NOT_HOLDER;
use crate::repo::{self, BranchKey, RepoId};
use crate::scan::ScannedRow;

/// One session's entry in a landing: the holder or a waiter.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Turn {
    /// The session.
    pub session: SessionId,
    /// Its messaging token, from its first request.
    pub token: MessagingToken,
    /// When it entered its current state: waiting since the request, holding since the grant.
    pub since: Timestamp,
}

/// One landing's turn: its holder and waiters.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Landing {
    /// The turn holder; `None` while the turn is free. A free turn has no waiter.
    pub holder: Option<Turn>,
    /// The waiters, oldest first.
    pub queue: VecDeque<Turn>,
}

/// One landing: a repository and the branch that lands there.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct LandingKey {
    /// The repository.
    pub repo: RepoId,
    /// The landing target.
    pub target: String,
}

/// Every landing, plus the last answer.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Landings {
    /// Landings by repository and target; a key without one has a free turn.
    pub turns: BTreeMap<LandingKey, Landing>,
    /// The answer of the last request, end or release and who asked, for the HTTP edge's read-back.
    pub answer: Option<(LandCaller, LandAnswer)>,
}

/// Who a land answer is kept for.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandCaller {
    /// A session's request or end.
    Session(SessionId),
    /// The operator's release, by the working directory it was typed in.
    Operator(PathBuf),
}

/// The branch a turn is asked for.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandTarget {
    /// `land --target <branch>`, or a replayed request.
    Named(String),
    /// `done` without a join: the landing target of the branch the session works on.
    OfSession,
}

/// What the core decided about a land event.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandAnswer {
    /// The event took effect, or a repeated request found its entry.
    Reply {
        /// The landing target of the entry.
        target: String,
        /// The state the caller is left in.
        reply: LandReply,
    },
    /// `done` from a session whose branch has no landing target: nothing was requested.
    NoTarget {
        /// The branch's base; `None` when agentwt resolved none.
        base: Option<String>,
    },
    /// The refusal reason.
    Refused(String),
}

/// The state a land event left its caller in.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandReply {
    /// The session holds the turn.
    Granted,
    /// The session waits.
    Queued {
        /// Its place in the queue, from 1.
        place: u32,
    },
    /// The session's entry ended.
    Ended {
        /// How.
        outcome: TypedEnd,
    },
    /// The operator released the holder's turn.
    Released {
        /// The holder that lost the turn.
        session: SessionId,
    },
}

/// How an entry ended, as the `land_ended` record spells it.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LandOutcome {
    /// The holder landed its branch.
    Landed,
    /// The holder or a waiter gave its entry up.
    Abandoned,
    /// The operator released the holder's turn.
    Released,
    /// The session ended, died or was unknown once the registry was seeded.
    SessionGone,
}

/// The two ends a session types.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TypedEnd {
    /// `land --end landed`: the holder only.
    Landed,
    /// `land --end abandoned`: the holder or a waiter.
    Abandoned,
}

impl From<TypedEnd> for LandOutcome {
    fn from(end: TypedEnd) -> LandOutcome {
        match end {
            TypedEnd::Landed => LandOutcome::Landed,
            TypedEnd::Abandoned => LandOutcome::Abandoned,
        }
    }
}

/// Where a `LandRequested` event comes from.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandSource {
    /// `POST /done` without a join, or `POST /land`: admitted, recorded, answered.
    Request,
    /// The action ledger: trusted, with the recorded repository.
    Replay {
        /// The repository the request was admitted for.
        repo: RepoId,
    },
}

/// What a `LandEnded` event ends.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LandEnd {
    /// `POST /land/end`: the session's own entry.
    Typed {
        /// The session.
        session: SessionId,
        /// Its messaging token; must match its entry.
        token: MessagingToken,
        /// Landed or abandoned.
        outcome: TypedEnd,
    },
    /// `POST /land/release`: the holder's turn for `target` in the repository of `cwd`, an operator act.
    Released {
        /// The working directory the release was typed in.
        cwd: PathBuf,
        /// The landing target whose holder it ends.
        target: String,
    },
    /// The action ledger: the recorded end of the session's one entry, whatever ended it.
    Replayed {
        /// The session.
        session: SessionId,
    },
}

/// What admission found for a request.
enum Admission {
    /// The turn it may enter.
    Turn(LandingKey),
    /// The session's branch has no landing target; `base` is its resolved base.
    NoTarget { base: Option<String> },
}

/// Where a session's entry sits.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Place {
    /// It holds the turn.
    Holding,
    /// It waits at this queue index.
    Waiting(usize),
}

/// Whether a derived grant is recorded and announced (live) or neither (replay).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Grant {
    /// Live: the granted record is claimed, then the message is sent.
    Announced,
    /// Replay: state only.
    Silent,
}

/// A turn request. A fresh one is admitted, recorded and answered; one from a session that already has an entry
/// answers that entry's state without a record. A replayed one is trusted and silent.
pub(crate) fn on_requested(
    fleet: &mut Fleet,
    session: SessionId,
    target: LandTarget,
    token: MessagingToken,
    source: LandSource,
    at: Timestamp,
) -> Vec<Action> {
    let caller = LandCaller::Session(session.clone());
    let turn = Turn {
        session,
        token,
        since: at,
    };
    let admitted = match (source, target) {
        (LandSource::Replay { repo }, LandTarget::Named(target)) => {
            if entry_of(&fleet.landings, &turn.session).is_none() {
                let _ = enter(&mut fleet.landings, LandingKey { repo, target }, turn);
            }
            return Vec::new();
        }
        (LandSource::Replay { .. }, LandTarget::OfSession) => return Vec::new(),
        (LandSource::Request, target) => admit(fleet, &turn.session, target),
    };
    let key = match admitted {
        Ok(Admission::Turn(key)) => key,
        Ok(Admission::NoTarget { base }) => {
            fleet.landings.answer = Some((caller, LandAnswer::NoTarget { base }));
            return Vec::new();
        }
        Err(reason) => return refuse(fleet, caller, reason),
    };
    if let Some((held, place)) = entry_of(&fleet.landings, &turn.session) {
        let is_other_token =
            turn_at(&fleet.landings, &held, place).is_some_and(|kept| kept.token != turn.token);
        let answer = if held != key {
            LandAnswer::Refused(format!(
                "the session already has a landing entry for {} in {}",
                held.target,
                held.repo.0.display()
            ))
        } else if is_other_token {
            LandAnswer::Refused("token does not match the landing request".to_owned())
        } else {
            LandAnswer::Reply {
                target: key.target,
                reply: reply_at(place),
            }
        };
        fleet.landings.answer = Some((caller, answer));
        return Vec::new();
    }
    let mut actions = vec![Action::Record(Record::LandRequested {
        session_id: turn.session.clone(),
        repo: key.repo.0.clone(),
        target: key.target.clone(),
        token: turn.token.clone(),
    })];
    let grant = granted(&turn.session, &key);
    let place = enter(&mut fleet.landings, key.clone(), turn);
    if place == Place::Holding {
        actions.push(Action::Record(grant));
    }
    let answer = LandAnswer::Reply {
        target: key.target,
        reply: reply_at(place),
    };
    fleet.landings.answer = Some((caller, answer));
    actions
}

/// An end: the session's own (`land --end`), the operator's release, or a replayed record.
pub(crate) fn on_ended(fleet: &mut Fleet, end: LandEnd, at: Timestamp) -> Vec<Action> {
    match end {
        LandEnd::Typed {
            session,
            token,
            outcome,
        } => typed_end(fleet, &session, &token, outcome, at),
        LandEnd::Released { cwd, target } => release(fleet, cwd, &target, at),
        LandEnd::Replayed { session } => {
            if let Some((key, _)) = entry_of(&fleet.landings, &session) {
                let _ = leave(fleet, &key, &session, at, Grant::Silent);
            }
            Vec::new()
        }
    }
}

/// The pass after every event: once the registry is seeded, the entry of every terminal or unknown session ends as
/// `session_gone` — waiters before their holder, so a turn a gone holder frees goes to a live waiter.
pub(crate) fn settle(fleet: &mut Fleet, at: Timestamp) -> Vec<Action> {
    if !fleet.seeded {
        return Vec::new();
    }
    let gone: Vec<(LandingKey, SessionId)> = fleet
        .landings
        .turns
        .iter()
        .flat_map(|(key, landing)| {
            landing
                .queue
                .iter()
                .chain(&landing.holder)
                .filter(|turn| is_gone(fleet, &turn.session))
                .map(move |turn| (key.clone(), turn.session.clone()))
        })
        .collect();
    let mut actions = Vec::new();
    for (key, session) in gone {
        actions.push(Action::Record(ended(
            &session,
            &key,
            LandOutcome::SessionGone,
        )));
        actions.extend(leave(fleet, &key, &session, at, Grant::Announced));
    }
    actions
}

/// The land read-back for `caller`: the kept answer when it is this caller's.
#[must_use]
pub fn answer_of(fleet: &Fleet, caller: &LandCaller) -> Option<LandAnswer> {
    fleet
        .landings
        .answer
        .as_ref()
        .filter(|(kept, _)| kept == caller)
        .map(|(_, answer)| answer.clone())
}

/// The grant message (merge strategy C4), one line.
#[must_use]
pub fn grant_text(target: &str, repo_name: &str) -> String {
    format!(
        "heartbeat: landing turn granted for {target} in {repo_name}. Continue the landing steps, then run: \
         smine-heartbeat land --end landed"
    )
}

/// `land --end`: the entry must be the session's, with its token; `landed` is the holder's alone.
fn typed_end(
    fleet: &mut Fleet,
    session: &SessionId,
    token: &MessagingToken,
    outcome: TypedEnd,
    at: Timestamp,
) -> Vec<Action> {
    let caller = LandCaller::Session(session.clone());
    let owned = entry_of(&fleet.landings, session).filter(|(key, place)| {
        turn_at(&fleet.landings, key, *place).is_some_and(|kept| kept.token == *token)
    });
    let may_end = |place: Place| outcome == TypedEnd::Abandoned || place == Place::Holding;
    let Some((key, _)) = owned.filter(|(_, place)| may_end(*place)) else {
        return refuse(fleet, caller, NOT_HOLDER.to_owned());
    };
    let mut actions = vec![Action::Record(ended(session, &key, outcome.into()))];
    actions.extend(leave(fleet, &key, session, at, Grant::Announced));
    let answer = LandAnswer::Reply {
        target: key.target,
        reply: LandReply::Ended { outcome },
    };
    fleet.landings.answer = Some((caller, answer));
    actions
}

/// `land --release`: ends the holder's turn for `target` in the repository `cwd` lies in.
fn release(fleet: &mut Fleet, cwd: PathBuf, target: &str, at: Timestamp) -> Vec<Action> {
    let found = repo::repo_of(fleet, &cwd).ok_or_else(|| outside(Some(&cwd)));
    let caller = LandCaller::Operator(cwd);
    let key = match found {
        Ok(repo) => LandingKey {
            repo,
            target: target.to_owned(),
        },
        Err(reason) => return refuse(fleet, caller, reason),
    };
    let holder = fleet
        .landings
        .turns
        .get(&key)
        .and_then(|landing| landing.holder.as_ref())
        .map(|turn| turn.session.clone());
    let Some(session) = holder else {
        let reason = format!("no landing turn is held for {target}");
        return refuse(fleet, caller, reason);
    };
    let mut actions = vec![Action::Record(ended(&session, &key, LandOutcome::Released))];
    actions.extend(leave(fleet, &key, &session, at, Grant::Announced));
    let answer = LandAnswer::Reply {
        target: key.target,
        reply: LandReply::Released { session },
    };
    fleet.landings.answer = Some((caller, answer));
    actions
}

/// Request admission: the session is known and alive, its cwd lies in a registered repo, and the target has a turn.
fn admit(fleet: &Fleet, session: &SessionId, target: LandTarget) -> Result<Admission, String> {
    let Some(known) = fleet.sessions.get(session) else {
        return Err("unknown session: no hook or registry entry seen for it".to_owned());
    };
    if let Some(end) = terminal_word(&known.state) {
        return Err(format!("the session {end}"));
    }
    let cwd = known.cwd.as_deref();
    let Some((repo, cwd)) = cwd.and_then(|cwd| Some((repo::repo_of(fleet, cwd)?, cwd))) else {
        return Err(outside(cwd));
    };
    let target = match target {
        LandTarget::Named(name) => name,
        LandTarget::OfSession => {
            let Some((row, branch)) = own_row(fleet, &repo, cwd) else {
                return Err(format!("no branch row for {}", cwd.display()));
            };
            match &branch.target {
                Some(name) => name.clone(),
                None => {
                    return Ok(Admission::NoTarget {
                        base: row.from.clone(),
                    });
                }
            }
        }
    };
    landable(fleet, &repo, &target)?;
    Ok(Admission::Turn(LandingKey { repo, target }))
}

/// The scanned row of the worktree `cwd` lies in, with its derived facts.
fn own_row<'a>(
    fleet: &'a Fleet,
    repo: &RepoId,
    cwd: &Path,
) -> Option<(&'a ScannedRow, &'a Branch)> {
    let known = fleet.repos.get(repo)?;
    let root = known.worktree_root(repo, cwd)?;
    let row = known
        .snapshot
        .as_ref()?
        .rows
        .iter()
        .find(|row| row.worktree.as_deref() == Some(root.as_path()))?;
    Some((row, known.branches.get(&row.key)?))
}

/// A turn exists for the staging target, and for every local branch of the last scan that is neither agent-owned nor
/// the default branch.
fn landable(fleet: &Fleet, repo: &RepoId, target: &str) -> Result<(), String> {
    if fleet.staging.as_ref().map(|s| s.0.as_str()) == Some(target) {
        return Ok(());
    }
    let scan = fleet
        .repos
        .get(repo)
        .and_then(|known| known.snapshot.as_ref());
    let Some(scan) = scan else {
        return Err(format!("{} is not scanned yet", repo.0.display()));
    };
    if target == scan.default_branch {
        return Err(format!("{target} is the default branch"));
    }
    if scan.locals.iter().any(|name| name == target) {
        return Ok(());
    }
    let is_agent = scan
        .rows
        .iter()
        .any(|row| matches!(&row.key, BranchKey::Branch(name) if name == target));
    if is_agent {
        return Err(format!("{target} is an agent branch"));
    }
    Err(format!("{target} is no local branch"))
}

/// Removes `session`'s entry in the landing `key`; when it held the turn, the first waiter gets it.
fn leave(
    fleet: &mut Fleet,
    key: &LandingKey,
    session: &SessionId,
    at: Timestamp,
    grant: Grant,
) -> Vec<Action> {
    let Some(landing) = fleet.landings.turns.get_mut(key) else {
        return Vec::new();
    };
    let is_holder = landing
        .holder
        .as_ref()
        .is_some_and(|turn| turn.session == *session);
    if !is_holder {
        landing.queue.retain(|turn| turn.session != *session);
        return Vec::new();
    }
    landing.holder = landing
        .queue
        .pop_front()
        .map(|next| Turn { since: at, ..next });
    let Some(next) = landing.holder.clone().filter(|_| grant == Grant::Announced) else {
        return Vec::new();
    };
    let name = fleet.repos.get(&key.repo).map_or_else(
        || key.repo.0.display().to_string(),
        |known| known.name.clone(),
    );
    let socket = fleet
        .sessions
        .get(&next.session)
        .and_then(|known| known.socket.clone());
    vec![Action::AnnounceGrant(GrantNotice {
        record: granted(&next.session, key),
        session: next.session,
        socket,
        token: next.token,
        text: grant_text(&key.target, &name),
    })]
}

/// Adds `turn` to the landing `key`: the holder when the turn is free, else the last waiter.
fn enter(landings: &mut Landings, key: LandingKey, turn: Turn) -> Place {
    let landing = landings.turns.entry(key).or_default();
    if landing.holder.is_none() {
        landing.holder = Some(turn);
        return Place::Holding;
    }
    landing.queue.push_back(turn);
    Place::Waiting(landing.queue.len() - 1)
}

/// The session's entry: its landing and place.
fn entry_of(landings: &Landings, session: &SessionId) -> Option<(LandingKey, Place)> {
    landings.turns.iter().find_map(|(key, landing)| {
        let is_holder = landing
            .holder
            .as_ref()
            .is_some_and(|turn| turn.session == *session);
        if is_holder {
            return Some((key.clone(), Place::Holding));
        }
        landing
            .queue
            .iter()
            .position(|turn| turn.session == *session)
            .map(|index| (key.clone(), Place::Waiting(index)))
    })
}

/// The turn at `place` of the landing `key`.
fn turn_at<'a>(landings: &'a Landings, key: &LandingKey, place: Place) -> Option<&'a Turn> {
    let landing = landings.turns.get(key)?;
    match place {
        Place::Holding => landing.holder.as_ref(),
        Place::Waiting(index) => landing.queue.get(index),
    }
}

/// The reply for an entry at `place`.
fn reply_at(place: Place) -> LandReply {
    match place {
        Place::Holding => LandReply::Granted,
        Place::Waiting(index) => LandReply::Queued {
            place: u32::try_from(index + 1).unwrap_or(u32::MAX),
        },
    }
}

/// The session is terminal, or unknown.
fn is_gone(fleet: &Fleet, session: &SessionId) -> bool {
    fleet
        .sessions
        .get(session)
        .is_none_or(|known| known.state.is_terminal())
}

/// Keeps `reason` as `caller`'s answer.
fn refuse(fleet: &mut Fleet, caller: LandCaller, reason: String) -> Vec<Action> {
    fleet.landings.answer = Some((caller, LandAnswer::Refused(reason)));
    Vec::new()
}

/// The refusal of a path outside every registered repo, in the words a subscribe gets.
fn outside(cwd: Option<&Path>) -> String {
    let shown = cwd.map_or_else(|| "(unknown)".to_owned(), |cwd| cwd.display().to_string());
    format!("session cwd {shown} is in no registered repo")
}

fn granted(session: &SessionId, key: &LandingKey) -> Record {
    Record::LandGranted {
        session_id: session.clone(),
        repo: key.repo.0.clone(),
        target: key.target.clone(),
    }
}

fn ended(session: &SessionId, key: &LandingKey, outcome: LandOutcome) -> Record {
    Record::LandEnded {
        session_id: session.clone(),
        repo: key.repo.0.clone(),
        target: key.target.clone(),
        outcome,
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;
    use crate::fleet::{Session, SessionState};
    use crate::repo::Repo;
    use crate::scan::RepoScan;

    const ONE: &str = "/r/one";
    const TWO: &str = "/r/two";
    /// The staging target the fixture configures.
    const STAGING: &str = "develop";
    /// A local branch of `one`'s scan.
    const FEATURE: &str = "feature/x";
    /// A second local branch of `one`'s scan.
    const OTHER: &str = "feature/y";
    /// The merge branch of a fleet bound to `one`.
    const MERGE: &str = "claude-orch/T1-run";

    fn t(second: i64) -> Timestamp {
        Timestamp::from_second(1_700_000_000 + second).expect("valid test second")
    }

    fn sid(id: &str) -> SessionId {
        SessionId(id.to_owned())
    }

    fn tok(token: &str) -> MessagingToken {
        MessagingToken(token.to_owned())
    }

    fn rid(path: &str) -> RepoId {
        RepoId(PathBuf::from(path))
    }

    fn key(repo: &str, target: &str) -> LandingKey {
        LandingKey {
            repo: rid(repo),
            target: target.to_owned(),
        }
    }

    fn named(target: &str) -> LandTarget {
        LandTarget::Named(target.to_owned())
    }

    /// An agent row of `one` with its worktree and base.
    fn row(branch: &str, worktree: &str, from: Option<&str>) -> ScannedRow {
        ScannedRow {
            key: BranchKey::Branch(branch.to_owned()),
            worktree: Some(PathBuf::from(worktree)),
            main: false,
            tracked_changes: 0,
            untracked: 0,
            own_commits: 1,
            fresh: false,
            from: from.map(str::to_owned),
            contained_in: None,
            safe_to_remove: false,
        }
    }

    /// Two registered repos and live sessions: s1–s4 in `one` (s2 and s3 with sockets), s5 in `two`, s6 in `one`'s
    /// main checkout, s7 on the fleet's merge branch, plus an ended, a died, a cwd-less and an outside session. `one`
    /// is scanned: s1 and s2 based on `feature/x`, s3 on `main`, s4 on no base, the merge branch on `feature/y`, a
    /// member branch pinned to the merge branch; `two` is not scanned.
    fn fleet() -> Fleet {
        let mut fleet = Fleet::new(t(0));
        fleet.staging = Some(crate::fleet::StagingTarget(STAGING.to_owned()));
        let mut one = Repo::new("one".to_owned());
        let scan = RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned(), FEATURE.to_owned(), OTHER.to_owned()],
            rows: vec![
                row("claude/s1", "/r/one/wt1", Some(FEATURE)),
                row("claude/s2", "/r/one/wt2", Some(FEATURE)),
                row("claude/s3", "/r/one/wt3", Some("main")),
                row("claude/s4", "/r/one/wt4", None),
                row(MERGE, "/r/one/wt7", Some(OTHER)),
                row("claude/m", "/r/one/wtm", Some(OTHER)),
            ],
            pairs: Vec::new(),
            trains: Vec::new(),
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        };
        let pins = BTreeMap::from([("claude/m".to_owned(), MERGE.to_owned())]);
        one.branches = crate::branch::derive(&scan, &BTreeMap::new(), &pins, t(0));
        one.snapshot = Some(scan);
        fleet.repos.insert(rid(ONE), one);
        fleet.repos.insert(rid(TWO), Repo::new("two".to_owned()));
        let sessions: [(&str, Option<&str>, SessionState, Option<&str>); 11] = [
            (
                "s1",
                Some("/r/one/wt1"),
                SessionState::Idle,
                Some("/sock/s1"),
            ),
            (
                "s2",
                Some("/r/one/wt2"),
                SessionState::Idle,
                Some("/sock/s2"),
            ),
            (
                "s3",
                Some("/r/one/wt3"),
                SessionState::Running,
                Some("/sock/s3"),
            ),
            ("s4", Some("/r/one/wt4"), SessionState::Idle, None),
            ("s5", Some("/r/two/wt5"), SessionState::Idle, None),
            ("s6", Some("/r/one"), SessionState::Idle, None),
            ("s7", Some("/r/one/wt7/sub"), SessionState::Idle, None),
            (
                "ended",
                Some("/r/one/x"),
                SessionState::Ended {
                    reason: "exit".to_owned(),
                },
                None,
            ),
            (
                "died",
                Some("/r/one/y"),
                SessionState::Died {
                    cause: "gone".to_owned(),
                },
                None,
            ),
            ("nocwd", None, SessionState::Idle, None),
            ("outside", Some("/elsewhere"), SessionState::Idle, None),
        ];
        for (id, cwd, state, socket) in sessions {
            let mut session = Session::new(sid(id), state, t(0), false);
            session.cwd = cwd.map(PathBuf::from);
            session.socket = socket.map(PathBuf::from);
            fleet.sessions.insert(sid(id), session);
        }
        fleet
    }

    /// Fresh requests, `(session, target, token)`, at seconds 1, 2, ….
    fn requested(fleet: &mut Fleet, prior: &[(&str, &str, &str)]) {
        for (index, (session, target, token)) in prior.iter().enumerate() {
            let at = t(i64::try_from(index).expect("small index") + 1);
            let _ = on_requested(
                fleet,
                sid(session),
                named(target),
                tok(token),
                LandSource::Request,
                at,
            );
        }
    }

    fn holder(fleet: &Fleet, repo: &str, target: &str) -> Option<String> {
        fleet
            .landings
            .turns
            .get(&key(repo, target))
            .and_then(|landing| landing.holder.as_ref())
            .map(|turn| turn.session.0.clone())
    }

    fn queue(fleet: &Fleet, repo: &str, target: &str) -> Vec<String> {
        fleet
            .landings
            .turns
            .get(&key(repo, target))
            .map(|landing| {
                landing
                    .queue
                    .iter()
                    .map(|turn| turn.session.0.clone())
                    .collect()
            })
            .unwrap_or_default()
    }

    fn requested_record(session: &str, repo: &str, target: &str, token: &str) -> Action {
        Action::Record(Record::LandRequested {
            session_id: sid(session),
            repo: PathBuf::from(repo),
            target: target.to_owned(),
            token: tok(token),
        })
    }

    fn granted_record(session: &str, repo: &str, target: &str) -> Action {
        Action::Record(granted(&sid(session), &key(repo, target)))
    }

    fn ended_record(session: &str, repo: &str, target: &str, outcome: LandOutcome) -> Action {
        Action::Record(ended(&sid(session), &key(repo, target), outcome))
    }

    fn notice(
        session: &str,
        token: &str,
        socket: Option<&str>,
        (repo, target): (&str, &str),
        name: &str,
    ) -> Action {
        Action::AnnounceGrant(GrantNotice {
            session: sid(session),
            socket: socket.map(PathBuf::from),
            token: tok(token),
            text: grant_text(target, name),
            record: granted(&sid(session), &key(repo, target)),
        })
    }

    fn reply(target: &str, reply: LandReply) -> LandAnswer {
        LandAnswer::Reply {
            target: target.to_owned(),
            reply,
        }
    }

    fn refused(reason: &str) -> LandAnswer {
        LandAnswer::Refused(reason.to_owned())
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn request_cases() {
        struct Case {
            name: &'static str,
            prior: &'static [(&'static str, &'static str, &'static str)],
            move_to: Option<&'static str>,
            session: &'static str,
            target: &'static str,
            token: &'static str,
            answer: LandAnswer,
            actions: Vec<Action>,
            /// The landing whose holder and queue are checked.
            at: (&'static str, &'static str),
            holder: Option<&'static str>,
            queue: Vec<&'static str>,
        }
        let cases = vec![
            Case {
                name: "staging target absent from the scan: granted",
                prior: &[],
                move_to: None,
                session: "s1",
                target: STAGING,
                token: "a",
                answer: reply(STAGING, LandReply::Granted),
                actions: vec![
                    requested_record("s1", ONE, STAGING, "a"),
                    granted_record("s1", ONE, STAGING),
                ],
                at: (ONE, STAGING),
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "named target granted",
                prior: &[],
                move_to: None,
                session: "s1",
                target: FEATURE,
                token: "a",
                answer: reply(FEATURE, LandReply::Granted),
                actions: vec![
                    requested_record("s1", ONE, FEATURE, "a"),
                    granted_record("s1", ONE, FEATURE),
                ],
                at: (ONE, FEATURE),
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "named target queued behind a holder",
                prior: &[("s1", FEATURE, "a")],
                move_to: None,
                session: "s2",
                target: FEATURE,
                token: "b",
                answer: reply(FEATURE, LandReply::Queued { place: 1 }),
                actions: vec![requested_record("s2", ONE, FEATURE, "b")],
                at: (ONE, FEATURE),
                holder: Some("s1"),
                queue: vec!["s2"],
            },
            Case {
                name: "held turn: place 2",
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                move_to: None,
                session: "s3",
                target: STAGING,
                token: "c",
                answer: reply(STAGING, LandReply::Queued { place: 2 }),
                actions: vec![requested_record("s3", ONE, STAGING, "c")],
                at: (ONE, STAGING),
                holder: Some("s1"),
                queue: vec!["s2", "s3"],
            },
            Case {
                name: "two targets of one repo: both granted",
                prior: &[("s1", STAGING, "a")],
                move_to: None,
                session: "s2",
                target: FEATURE,
                token: "b",
                answer: reply(FEATURE, LandReply::Granted),
                actions: vec![
                    requested_record("s2", ONE, FEATURE, "b"),
                    granted_record("s2", ONE, FEATURE),
                ],
                at: (ONE, FEATURE),
                holder: Some("s2"),
                queue: vec![],
            },
            Case {
                name: "same target in two repos: both granted",
                prior: &[("s1", STAGING, "a")],
                move_to: None,
                session: "s5",
                target: STAGING,
                token: "e",
                answer: reply(STAGING, LandReply::Granted),
                actions: vec![
                    requested_record("s5", TWO, STAGING, "e"),
                    granted_record("s5", TWO, STAGING),
                ],
                at: (TWO, STAGING),
                holder: Some("s5"),
                queue: vec![],
            },
            Case {
                name: "repeat by the holder",
                prior: &[("s1", STAGING, "a")],
                move_to: None,
                session: "s1",
                target: STAGING,
                token: "a",
                answer: reply(STAGING, LandReply::Granted),
                actions: vec![],
                at: (ONE, STAGING),
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "repeat by a waiter",
                prior: &[("s1", FEATURE, "a"), ("s2", FEATURE, "b")],
                move_to: None,
                session: "s2",
                target: FEATURE,
                token: "b",
                answer: reply(FEATURE, LandReply::Queued { place: 1 }),
                actions: vec![],
                at: (ONE, FEATURE),
                holder: Some("s1"),
                queue: vec!["s2"],
            },
            Case {
                name: "repeat with another token",
                prior: &[("s1", STAGING, "a")],
                move_to: None,
                session: "s1",
                target: STAGING,
                token: "x",
                answer: refused("token does not match the landing request"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "second request for another target",
                prior: &[("s1", STAGING, "a")],
                move_to: None,
                session: "s1",
                target: FEATURE,
                token: "a",
                answer: refused("the session already has a landing entry for develop in /r/one"),
                actions: vec![],
                at: (ONE, FEATURE),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "entry in another repo",
                prior: &[("s1", STAGING, "a")],
                move_to: Some("/r/two/wt1"),
                session: "s1",
                target: STAGING,
                token: "a",
                answer: refused("the session already has a landing entry for develop in /r/one"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "default branch refused",
                prior: &[],
                move_to: None,
                session: "s1",
                target: "main",
                token: "a",
                answer: refused("main is the default branch"),
                actions: vec![],
                at: (ONE, "main"),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "agent branch refused",
                prior: &[],
                move_to: None,
                session: "s1",
                target: "claude/s2",
                token: "a",
                answer: refused("claude/s2 is an agent branch"),
                actions: vec![],
                at: (ONE, "claude/s2"),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "merge branch refused",
                prior: &[],
                move_to: None,
                session: "s1",
                target: MERGE,
                token: "a",
                answer: refused("claude-orch/T1-run is an agent branch"),
                actions: vec![],
                at: (ONE, MERGE),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "unknown branch refused",
                prior: &[],
                move_to: None,
                session: "s1",
                target: "nope",
                token: "a",
                answer: refused("nope is no local branch"),
                actions: vec![],
                at: (ONE, "nope"),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "repo without a scan refused",
                prior: &[],
                move_to: None,
                session: "s5",
                target: FEATURE,
                token: "e",
                answer: refused("/r/two is not scanned yet"),
                actions: vec![],
                at: (TWO, FEATURE),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "repo without a scan: staging target still granted",
                prior: &[],
                move_to: None,
                session: "s5",
                target: STAGING,
                token: "e",
                answer: reply(STAGING, LandReply::Granted),
                actions: vec![
                    requested_record("s5", TWO, STAGING, "e"),
                    granted_record("s5", TWO, STAGING),
                ],
                at: (TWO, STAGING),
                holder: Some("s5"),
                queue: vec![],
            },
            Case {
                name: "unknown session",
                prior: &[],
                move_to: None,
                session: "ghost",
                target: STAGING,
                token: "a",
                answer: refused("unknown session: no hook or registry entry seen for it"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "ended session",
                prior: &[],
                move_to: None,
                session: "ended",
                target: STAGING,
                token: "a",
                answer: refused("the session ended"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "died session",
                prior: &[],
                move_to: None,
                session: "died",
                target: STAGING,
                token: "a",
                answer: refused("the session died"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "cwd unknown",
                prior: &[],
                move_to: None,
                session: "nocwd",
                target: STAGING,
                token: "a",
                answer: refused("session cwd (unknown) is in no registered repo"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: None,
                queue: vec![],
            },
            Case {
                name: "cwd outside every repo",
                prior: &[],
                move_to: None,
                session: "outside",
                target: STAGING,
                token: "a",
                answer: refused("session cwd /elsewhere is in no registered repo"),
                actions: vec![],
                at: (ONE, STAGING),
                holder: None,
                queue: vec![],
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            requested(&mut fleet, case.prior);
            if let Some(cwd) = case.move_to
                && let Some(session) = fleet.sessions.get_mut(&sid(case.session))
            {
                session.cwd = Some(PathBuf::from(cwd));
            }
            let actions = on_requested(
                &mut fleet,
                sid(case.session),
                named(case.target),
                tok(case.token),
                LandSource::Request,
                t(10),
            );
            assert_eq!(actions, case.actions, "{}: actions", case.name);
            let caller = LandCaller::Session(sid(case.session));
            assert_eq!(
                answer_of(&fleet, &caller),
                Some(case.answer),
                "{}: answer",
                case.name
            );
            let (repo, target) = case.at;
            assert_eq!(
                holder(&fleet, repo, target).as_deref(),
                case.holder,
                "{}: holder",
                case.name
            );
            assert_eq!(
                queue(&fleet, repo, target),
                case.queue,
                "{}: queue",
                case.name
            );
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn done_cases() {
        struct Case {
            name: &'static str,
            prior: &'static [&'static str],
            session: &'static str,
            answer: LandAnswer,
            actions: Vec<Action>,
            holder: Option<&'static str>,
            queue: Vec<&'static str>,
        }
        let cases = vec![
            Case {
                name: "row with target: granted for it",
                prior: &[],
                session: "s1",
                answer: reply(FEATURE, LandReply::Granted),
                actions: vec![
                    requested_record("s1", ONE, FEATURE, "s1"),
                    granted_record("s1", ONE, FEATURE),
                ],
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "row with target, turn held: queued",
                prior: &["s1"],
                session: "s2",
                answer: reply(FEATURE, LandReply::Queued { place: 1 }),
                actions: vec![requested_record("s2", ONE, FEATURE, "s2")],
                holder: Some("s1"),
                queue: vec!["s2"],
            },
            Case {
                name: "row based on the default branch",
                prior: &[],
                session: "s3",
                answer: LandAnswer::NoTarget {
                    base: Some("main".to_owned()),
                },
                actions: vec![],
                holder: None,
                queue: vec![],
            },
            Case {
                name: "row without base",
                prior: &[],
                session: "s4",
                answer: LandAnswer::NoTarget { base: None },
                actions: vec![],
                holder: None,
                queue: vec![],
            },
            Case {
                name: "session in the main checkout",
                prior: &[],
                session: "s6",
                answer: refused("no branch row for /r/one"),
                actions: vec![],
                holder: None,
                queue: vec![],
            },
            Case {
                name: "worktree not scanned yet",
                prior: &[],
                session: "s5",
                answer: refused("no branch row for /r/two/wt5"),
                actions: vec![],
                holder: None,
                queue: vec![],
            },
            Case {
                name: "unknown session",
                prior: &[],
                session: "ghost",
                answer: refused("unknown session: no hook or registry entry seen for it"),
                actions: vec![],
                holder: None,
                queue: vec![],
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            for (index, session) in case.prior.iter().enumerate() {
                let at = t(i64::try_from(index).expect("small index") + 1);
                let _ = on_requested(
                    &mut fleet,
                    sid(session),
                    LandTarget::OfSession,
                    tok(session),
                    LandSource::Request,
                    at,
                );
            }
            let actions = on_requested(
                &mut fleet,
                sid(case.session),
                LandTarget::OfSession,
                tok(case.session),
                LandSource::Request,
                t(10),
            );
            assert_eq!(actions, case.actions, "{}: actions", case.name);
            let caller = LandCaller::Session(sid(case.session));
            assert_eq!(
                answer_of(&fleet, &caller),
                Some(case.answer),
                "{}: answer",
                case.name
            );
            assert_eq!(
                holder(&fleet, ONE, FEATURE).as_deref(),
                case.holder,
                "{}: holder",
                case.name
            );
            assert_eq!(
                queue(&fleet, ONE, FEATURE),
                case.queue,
                "{}: queue",
                case.name
            );
        }

        let mut fleet = fleet();
        let actions = on_requested(
            &mut fleet,
            sid("s7"),
            LandTarget::OfSession,
            tok("g"),
            LandSource::Request,
            t(10),
        );
        assert_eq!(
            actions,
            vec![
                requested_record("s7", ONE, OTHER, "g"),
                granted_record("s7", ONE, OTHER),
            ],
            "pinned member's subscriber: granted for the fleet target"
        );
        assert_eq!(
            answer_of(&fleet, &LandCaller::Session(sid("s7"))),
            Some(reply(OTHER, LandReply::Granted)),
            "subscriber answer names the fleet target"
        );
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn end_cases() {
        struct Case {
            name: &'static str,
            prior: &'static [(&'static str, &'static str, &'static str)],
            session: &'static str,
            token: &'static str,
            outcome: TypedEnd,
            answer: LandAnswer,
            actions: Vec<Action>,
            holder: Option<&'static str>,
            queue: Vec<&'static str>,
            /// Another target's landing and its holder after the end.
            kept: Option<(&'static str, &'static str)>,
        }
        let cases = vec![
            Case {
                name: "holder landed",
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                session: "s1",
                token: "a",
                outcome: TypedEnd::Landed,
                answer: reply(
                    STAGING,
                    LandReply::Ended {
                        outcome: TypedEnd::Landed,
                    },
                ),
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::Landed),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                holder: Some("s2"),
                queue: vec![],
                kept: None,
            },
            Case {
                name: "holder abandoned",
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                session: "s1",
                token: "a",
                outcome: TypedEnd::Abandoned,
                answer: reply(
                    STAGING,
                    LandReply::Ended {
                        outcome: TypedEnd::Abandoned,
                    },
                ),
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::Abandoned),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                holder: Some("s2"),
                queue: vec![],
                kept: None,
            },
            Case {
                name: "holder ends with empty queue",
                prior: &[("s1", STAGING, "a")],
                session: "s1",
                token: "a",
                outcome: TypedEnd::Landed,
                answer: reply(
                    STAGING,
                    LandReply::Ended {
                        outcome: TypedEnd::Landed,
                    },
                ),
                actions: vec![ended_record("s1", ONE, STAGING, LandOutcome::Landed)],
                holder: None,
                queue: vec![],
                kept: None,
            },
            Case {
                name: "end frees only its own target's turn",
                prior: &[
                    ("s1", STAGING, "a"),
                    ("s2", FEATURE, "b"),
                    ("s3", STAGING, "c"),
                ],
                session: "s1",
                token: "a",
                outcome: TypedEnd::Landed,
                answer: reply(
                    STAGING,
                    LandReply::Ended {
                        outcome: TypedEnd::Landed,
                    },
                ),
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::Landed),
                    notice("s3", "c", Some("/sock/s3"), (ONE, STAGING), "one"),
                ],
                holder: Some("s3"),
                queue: vec![],
                kept: Some((FEATURE, "s2")),
            },
            Case {
                name: "waiter abandoned",
                prior: &[
                    ("s1", STAGING, "a"),
                    ("s2", STAGING, "b"),
                    ("s3", STAGING, "c"),
                ],
                session: "s2",
                token: "b",
                outcome: TypedEnd::Abandoned,
                answer: reply(
                    STAGING,
                    LandReply::Ended {
                        outcome: TypedEnd::Abandoned,
                    },
                ),
                actions: vec![ended_record("s2", ONE, STAGING, LandOutcome::Abandoned)],
                holder: Some("s1"),
                queue: vec!["s3"],
                kept: None,
            },
            Case {
                name: "waiter landed",
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                session: "s2",
                token: "b",
                outcome: TypedEnd::Landed,
                answer: refused(NOT_HOLDER),
                actions: vec![],
                holder: Some("s1"),
                queue: vec!["s2"],
                kept: None,
            },
            Case {
                name: "no entry",
                prior: &[("s1", STAGING, "a")],
                session: "s4",
                token: "d",
                outcome: TypedEnd::Abandoned,
                answer: refused(NOT_HOLDER),
                actions: vec![],
                holder: Some("s1"),
                queue: vec![],
                kept: None,
            },
            Case {
                name: "wrong token",
                prior: &[("s1", STAGING, "a")],
                session: "s1",
                token: "x",
                outcome: TypedEnd::Landed,
                answer: refused(NOT_HOLDER),
                actions: vec![],
                holder: Some("s1"),
                queue: vec![],
                kept: None,
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            requested(&mut fleet, case.prior);
            let end = LandEnd::Typed {
                session: sid(case.session),
                token: tok(case.token),
                outcome: case.outcome,
            };
            let actions = on_ended(&mut fleet, end, t(10));
            assert_eq!(actions, case.actions, "{}: actions", case.name);
            let caller = LandCaller::Session(sid(case.session));
            assert_eq!(
                answer_of(&fleet, &caller),
                Some(case.answer),
                "{}: answer",
                case.name
            );
            assert_eq!(
                holder(&fleet, ONE, STAGING).as_deref(),
                case.holder,
                "{}: holder",
                case.name
            );
            assert_eq!(
                queue(&fleet, ONE, STAGING),
                case.queue,
                "{}: queue",
                case.name
            );
            if let Some((target, kept)) = case.kept {
                assert_eq!(
                    holder(&fleet, ONE, target).as_deref(),
                    Some(kept),
                    "{}: other target kept",
                    case.name
                );
            }
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn release_cases() {
        struct Case {
            name: &'static str,
            prior: &'static [(&'static str, &'static str, &'static str)],
            cwd: &'static str,
            target: &'static str,
            answer: LandAnswer,
            actions: Vec<Action>,
            holder: Option<&'static str>,
            /// Another target's landing, free after the release.
            freed: Option<&'static str>,
        }
        let cases = vec![
            Case {
                name: "holder released",
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                cwd: ONE,
                target: STAGING,
                answer: reply(STAGING, LandReply::Released { session: sid("s1") }),
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::Released),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                holder: Some("s2"),
                freed: None,
            },
            Case {
                name: "release names the target, other targets keep their holders",
                prior: &[("s1", STAGING, "a"), ("s2", FEATURE, "b")],
                cwd: ONE,
                target: FEATURE,
                answer: reply(FEATURE, LandReply::Released { session: sid("s2") }),
                actions: vec![ended_record("s2", ONE, FEATURE, LandOutcome::Released)],
                holder: Some("s1"),
                freed: Some(FEATURE),
            },
            Case {
                name: "no holder",
                prior: &[],
                cwd: ONE,
                target: STAGING,
                answer: refused("no landing turn is held for develop"),
                actions: vec![],
                holder: None,
                freed: None,
            },
            Case {
                name: "target without holder",
                prior: &[("s1", STAGING, "a")],
                cwd: ONE,
                target: FEATURE,
                answer: refused("no landing turn is held for feature/x"),
                actions: vec![],
                holder: Some("s1"),
                freed: None,
            },
            Case {
                name: "cwd outside every repo",
                prior: &[("s1", STAGING, "a")],
                cwd: "/elsewhere",
                target: STAGING,
                answer: refused("session cwd /elsewhere is in no registered repo"),
                actions: vec![],
                holder: Some("s1"),
                freed: None,
            },
            Case {
                name: "cwd in a worktree of the repo",
                prior: &[("s1", STAGING, "a")],
                cwd: "/r/one/.claude/worktrees/x",
                target: STAGING,
                answer: reply(STAGING, LandReply::Released { session: sid("s1") }),
                actions: vec![ended_record("s1", ONE, STAGING, LandOutcome::Released)],
                holder: None,
                freed: None,
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            requested(&mut fleet, case.prior);
            let end = LandEnd::Released {
                cwd: PathBuf::from(case.cwd),
                target: case.target.to_owned(),
            };
            let actions = on_ended(&mut fleet, end, t(10));
            assert_eq!(actions, case.actions, "{}: actions", case.name);
            let caller = LandCaller::Operator(PathBuf::from(case.cwd));
            assert_eq!(
                answer_of(&fleet, &caller),
                Some(case.answer),
                "{}: answer",
                case.name
            );
            assert_eq!(
                holder(&fleet, ONE, STAGING).as_deref(),
                case.holder,
                "{}: staging holder",
                case.name
            );
            if let Some(target) = case.freed {
                assert_eq!(
                    holder(&fleet, ONE, target),
                    None,
                    "{}: released target",
                    case.name
                );
            }
        }
    }

    #[test]
    fn grant_notice() {
        struct Case {
            name: &'static str,
            prior: &'static [(&'static str, &'static str, &'static str)],
            notice: Action,
        }
        let cases = vec![
            Case {
                name: "waiter with a registry socket",
                prior: &[("s1", STAGING, "a"), ("s3", STAGING, "c")],
                notice: notice("s3", "c", Some("/sock/s3"), (ONE, STAGING), "one"),
            },
            Case {
                name: "waiter without a socket",
                prior: &[("s1", STAGING, "a"), ("s4", STAGING, "d")],
                notice: notice("s4", "d", None, (ONE, STAGING), "one"),
            },
            Case {
                name: "grant names the freed target",
                prior: &[("s1", FEATURE, "a"), ("s2", FEATURE, "b")],
                notice: notice("s2", "b", Some("/sock/s2"), (ONE, FEATURE), "one"),
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            requested(&mut fleet, case.prior);
            let end = LandEnd::Typed {
                session: sid("s1"),
                token: tok("a"),
                outcome: TypedEnd::Landed,
            };
            let actions = on_ended(&mut fleet, end, t(10));
            assert_eq!(actions.last(), Some(&case.notice), "{}", case.name);
        }
        assert_eq!(
            grant_text("feature/x", "claude-configs"),
            "heartbeat: landing turn granted for feature/x in claude-configs. Continue the landing steps, then run: \
             smine-heartbeat land --end landed",
            "text equals the contract line with the target and the registry name"
        );
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn settle_cases() {
        struct Case {
            name: &'static str,
            seeded: bool,
            prior: &'static [(&'static str, &'static str, &'static str)],
            ended: &'static [&'static str],
            died: &'static [&'static str],
            unknown: &'static [&'static str],
            actions: Vec<Action>,
            /// The landing whose holder and queue are checked.
            target: &'static str,
            holder: Option<&'static str>,
            queue: Vec<&'static str>,
        }
        let cases = vec![
            Case {
                name: "not seeded",
                seeded: false,
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                ended: &["s1"],
                died: &[],
                unknown: &[],
                actions: vec![],
                target: STAGING,
                holder: Some("s1"),
                queue: vec!["s2"],
            },
            Case {
                name: "ended holder",
                seeded: true,
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                ended: &["s1"],
                died: &[],
                unknown: &[],
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::SessionGone),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                target: STAGING,
                holder: Some("s2"),
                queue: vec![],
            },
            Case {
                name: "died holder",
                seeded: true,
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                ended: &[],
                died: &["s1"],
                unknown: &[],
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::SessionGone),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                target: STAGING,
                holder: Some("s2"),
                queue: vec![],
            },
            Case {
                name: "unknown holder after the seed",
                seeded: true,
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                ended: &[],
                died: &[],
                unknown: &["s1"],
                actions: vec![
                    ended_record("s1", ONE, STAGING, LandOutcome::SessionGone),
                    notice("s2", "b", Some("/sock/s2"), (ONE, STAGING), "one"),
                ],
                target: STAGING,
                holder: Some("s2"),
                queue: vec![],
            },
            Case {
                name: "gone session ends its one entry",
                seeded: true,
                prior: &[("s1", FEATURE, "a"), ("s2", STAGING, "b")],
                ended: &["s1"],
                died: &[],
                unknown: &[],
                actions: vec![ended_record("s1", ONE, FEATURE, LandOutcome::SessionGone)],
                target: STAGING,
                holder: Some("s2"),
                queue: vec![],
            },
            Case {
                name: "ended waiter",
                seeded: true,
                prior: &[("s1", STAGING, "a"), ("s2", STAGING, "b")],
                ended: &["s2"],
                died: &[],
                unknown: &[],
                actions: vec![ended_record("s2", ONE, STAGING, LandOutcome::SessionGone)],
                target: STAGING,
                holder: Some("s1"),
                queue: vec![],
            },
            Case {
                name: "holder and first waiter both gone",
                seeded: true,
                prior: &[
                    ("s1", STAGING, "a"),
                    ("s2", STAGING, "b"),
                    ("s3", STAGING, "c"),
                ],
                ended: &["s1", "s2"],
                died: &[],
                unknown: &[],
                actions: vec![
                    ended_record("s2", ONE, STAGING, LandOutcome::SessionGone),
                    ended_record("s1", ONE, STAGING, LandOutcome::SessionGone),
                    notice("s3", "c", Some("/sock/s3"), (ONE, STAGING), "one"),
                ],
                target: STAGING,
                holder: Some("s3"),
                queue: vec![],
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            requested(&mut fleet, case.prior);
            fleet.seeded = case.seeded;
            for id in case.ended {
                if let Some(session) = fleet.sessions.get_mut(&sid(id)) {
                    session.state = SessionState::Ended {
                        reason: "exit".to_owned(),
                    };
                }
            }
            for id in case.died {
                if let Some(session) = fleet.sessions.get_mut(&sid(id)) {
                    session.state = SessionState::Died {
                        cause: "gone".to_owned(),
                    };
                }
            }
            for id in case.unknown {
                fleet.sessions.remove(&sid(id));
            }
            let actions = settle(&mut fleet, t(10));
            assert_eq!(actions, case.actions, "{}: actions", case.name);
            assert_eq!(
                holder(&fleet, ONE, case.target).as_deref(),
                case.holder,
                "{}: holder",
                case.name
            );
            assert_eq!(
                queue(&fleet, ONE, case.target),
                case.queue,
                "{}: queue",
                case.name
            );
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn replay_rebuilds() {
        enum Step {
            Requested(&'static str, &'static str),
            Ended(&'static str),
        }
        struct Case {
            name: &'static str,
            steps: Vec<Step>,
            /// Per landing target of `one`: holder and queue.
            want: Vec<(&'static str, Option<&'static str>, Vec<&'static str>)>,
        }
        let cases = vec![
            Case {
                name: "records naming develop rebuild the develop landing",
                steps: vec![
                    Step::Requested("s1", "develop"),
                    Step::Requested("s2", "develop"),
                    Step::Requested("s3", "develop"),
                    Step::Ended("s1"),
                ],
                want: vec![("develop", Some("s2"), vec!["s3"])],
            },
            Case {
                name: "records of two targets rebuild two landings",
                steps: vec![
                    Step::Requested("s1", STAGING),
                    Step::Requested("s2", FEATURE),
                    Step::Requested("s3", FEATURE),
                    Step::Ended("s2"),
                ],
                want: vec![(STAGING, Some("s1"), vec![]), (FEATURE, Some("s3"), vec![])],
            },
            Case {
                name: "duplicate request record",
                steps: vec![
                    Step::Requested("s1", STAGING),
                    Step::Requested("s1", FEATURE),
                ],
                want: vec![(STAGING, Some("s1"), vec![]), (FEATURE, None, vec![])],
            },
            Case {
                name: "end record without a matching entry is ignored",
                steps: vec![Step::Requested("s1", STAGING), Step::Ended("s9")],
                want: vec![(STAGING, Some("s1"), vec![])],
            },
            Case {
                name: "end before request is ignored",
                steps: vec![Step::Ended("s1"), Step::Requested("s1", FEATURE)],
                want: vec![(FEATURE, Some("s1"), vec![])],
            },
            Case {
                name: "replayed request of an unknown session before the seed",
                steps: vec![Step::Requested("ghost", STAGING)],
                want: vec![(STAGING, Some("ghost"), vec![])],
            },
        ];
        for case in cases {
            let mut fleet = fleet();
            let mut actions = Vec::new();
            for step in case.steps {
                match step {
                    Step::Requested(session, target) => actions.extend(on_requested(
                        &mut fleet,
                        sid(session),
                        named(target),
                        tok(session),
                        LandSource::Replay { repo: rid(ONE) },
                        t(1),
                    )),
                    Step::Ended(session) => actions.extend(on_ended(
                        &mut fleet,
                        LandEnd::Replayed {
                            session: sid(session),
                        },
                        t(2),
                    )),
                }
            }
            assert!(actions.is_empty(), "{}: replay is silent", case.name);
            assert_eq!(fleet.landings.answer, None, "{}: no answer", case.name);
            for (target, want_holder, want_queue) in case.want {
                assert_eq!(
                    holder(&fleet, ONE, target).as_deref(),
                    want_holder,
                    "{}: {target} holder",
                    case.name
                );
                assert_eq!(
                    queue(&fleet, ONE, target),
                    want_queue,
                    "{}: {target} queue",
                    case.name
                );
            }
        }
    }
}
