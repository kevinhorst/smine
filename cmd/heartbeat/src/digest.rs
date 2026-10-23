//! Digests (concept D6): what a fleet's orchestrator hears, derived without a clock. A [`View`] is what one
//! subscription can observe of its fleet at one instant; [`diff`] of the views before and after one event yields the
//! [`Item`]s. A member's first appearance is never news — a join, a restart seed or a worker registration announces
//! nothing — and neither is the bound repo's first scan (git state rebuilt after a restart); fleet-level facts
//! already announced stay muted until they are seen to go away. Members are labelled by their token (`T1-MA`); a
//! joined member's `done` is the `delivered` item. Pure.
//!
//! Git facts (P2) come from the fleet's bound repository only: a member's branch is [`member_branch`]; its work items
//! are the plan dirs of that branch's worktree; a collision counts when the fleet's [`CollisionScope`] keeps it — a
//! worker branch on one side and, unless the run asked for outsiders, a fleet branch on the other — and is announced
//! once per pair and level; a pending fleet-level item flushes only while the view still states it; the fleet is
//! integrated when [`Fleet::target_state`] says so.

use std::collections::{BTreeMap, BTreeSet};
use std::fmt::Write as _;

use crate::branch::BranchState;
use crate::collide::Level;
use crate::config::Caps;
pub use crate::fleet::MemberKey;
use crate::fleet::{
    CollisionScope, Fleet, Member, Session, SessionState, Subscription, TargetState, member_branch,
};
use crate::ids::{DigestId, FleetId, SessionId};
use crate::repo::{BranchKey, Repo};

/// What a subscription observes of one member.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MemberView {
    /// Human label: the member token, then the title (else the id); a worker's branch.
    pub label: String,
    /// Joined and reported done, or a worker its subscriber marked delivered.
    pub delivered: bool,
    /// Digest-relevant state.
    pub state: MemberState,
    /// Why, for waiting, blocked and died.
    pub detail: Option<String>,
    /// The plan dirs of the member branch's worktree, comma-joined (P2 work items).
    pub work_item: Option<String>,
    /// The member branch, while its P2 state is `merge_ready`.
    pub merge_ready: Option<String>,
}

/// Member state as digests name it; `Done` covers an ended session and a finished subagent.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MemberState {
    /// Working.
    Running,
    /// Waiting on its human.
    Waiting,
    /// Stopped on an unresolved denial.
    Blocked,
    /// Turn finished.
    Idle,
    /// Ended cleanly.
    Done,
    /// Died.
    Died,
}

/// A P2 collision as digests show it.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct CollisionFact {
    /// One side: branch name, or worktree path when detached.
    pub a: String,
    /// The other side.
    pub b: String,
    /// P2's wire spelling: `planned_file`, `planned_symbol`, `touched` or `confirmed`.
    pub level: String,
    /// Colliding paths.
    pub paths: Vec<String>,
    /// Colliding symbols (planned-symbol level).
    pub symbols: Vec<String>,
}

impl CollisionFact {
    /// The announced-set key: pair and level. The paths are shown, never keyed, so a shifting path set is no news.
    #[must_use]
    pub fn key(&self) -> String {
        format!("collision:{}|{}|{}", self.a, self.b, self.level)
    }
}

/// One subscription's view of its fleet.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct View {
    /// Members other than the subscriber itself.
    pub members: BTreeMap<MemberKey, MemberView>,
    /// Collisions of the bound repo with a worker branch (a member other than the subscriber) on either side.
    pub collisions: BTreeSet<CollisionFact>,
    /// The fleet's target, while its target state is `integrated`.
    pub integrated: Option<String>,
    /// The bound repo has a scan: its git facts are known, so a change in them is news.
    pub scanned: bool,
}

/// Where a pending item sits; a newer item under the same key replaces the older one.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub enum ItemKey {
    /// A member's latest state.
    State(MemberKey),
    /// A member's work item.
    WorkItem(MemberKey),
    /// A member became merge-ready.
    MergeReady(MemberKey),
    /// A joined member reported done.
    Delivered(MemberKey),
    /// A fleet-level fact, by its announced-set key.
    Fleet(String),
}

/// One digest line (concept D6 kinds).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Item {
    /// `waiting(member)`.
    Waiting {
        /// Member label.
        member: String,
        /// What it waits on.
        detail: Option<String>,
    },
    /// `idle(member)`.
    Idle {
        /// Member label.
        member: String,
    },
    /// `blocked(member, candidate rule)`.
    Blocked {
        /// Member label.
        member: String,
        /// The candidate rule.
        detail: Option<String>,
    },
    /// `died(member, reason, last skill)`.
    Died {
        /// Member label.
        member: String,
        /// Reason and last skill.
        detail: Option<String>,
    },
    /// `done(member)`.
    Done {
        /// Member label.
        member: String,
    },
    /// `collision(A, B, level)` with paths and symbols.
    Collision(CollisionFact),
    /// `work_item_changed(member, plans)`.
    WorkItemChanged {
        /// Member label.
        member: String,
        /// Plan dirs, comma-joined.
        plan: String,
    },
    /// `merge_ready(member, branch)`.
    MergeReady {
        /// Member label.
        member: String,
        /// The member branch.
        branch: String,
    },
    /// `fleet_integrated(target)`.
    FleetIntegrated {
        /// The integration branch.
        target: String,
    },
    /// `delivered(member)`.
    Delivered {
        /// Member label.
        member: String,
    },
}

/// `subscription`'s view of its fleet; the subscriber session itself is no member line, and its branch is no
/// collision side.
#[must_use]
pub fn view(fleet: &Fleet, subscriber: &SessionId, subscription: &Subscription) -> View {
    let repo = subscription
        .repo
        .as_ref()
        .and_then(|id| Some((id, fleet.repos.get(id)?)));
    let mut members = BTreeMap::new();
    for member in fleet.members(&subscription.fleet) {
        let branch = repo.and_then(|(id, _)| member_branch(fleet, id, &member));
        if member.session == *subscriber && member.worker.is_none() {
            continue;
        }
        let git = repo.map(|(_, state)| state).zip(branch.as_ref());
        let key = (member.session.clone(), member.worker.clone());
        let token = fleet
            .member_id(&subscription.fleet, &key)
            .map(|letter| letter.token(&subscription.fleet));
        let Some(member_view) = member_view(fleet, &member, token.as_deref(), git) else {
            continue;
        };
        members.insert((member.session, member.worker), member_view);
    }
    let integrated = fleet.target_state(&subscription.fleet) == Some(TargetState::Integrated);
    let scope = fleet
        .collision_scope(&subscription.fleet)
        .unwrap_or_default();
    View {
        collisions: repo
            .map(|(_, state)| collisions(state, &scope))
            .unwrap_or_default(),
        integrated: integrated.then(|| subscription.target.clone()),
        scanned: repo.is_some_and(|(_, state)| state.snapshot.is_some()),
        members,
    }
}

fn label(session: &Session) -> String {
    session
        .title
        .clone()
        .unwrap_or_else(|| session.id.0.clone())
}

/// One member's view; `None` for a session member heartbeat has no session for. `token` is the member's
/// `<fleet>-M<letter>` once assigned.
fn member_view(
    fleet: &Fleet,
    member: &Member,
    token: Option<&str>,
    git: Option<(&Repo, &BranchKey)>,
) -> Option<MemberView> {
    let (label, state, detail) = match &member.worker {
        None => {
            let session = fleet.sessions.get(&member.session)?;
            let (state, detail) = session_state(session);
            let label = match token {
                Some(token) => format!("{token} {}", label(session)),
                None => label(session),
            };
            (label, state, detail)
        }
        Some(branch) => {
            let label = match token {
                Some(token) => format!("{token} worker {}", branch.0),
                None => format!("worker {}", branch.0),
            };
            (label, MemberState::Running, None)
        }
    };
    let (work_item, merge_ready) = git
        .map(|(repo, key)| (work_items(repo, key), merge_ready(repo, key)))
        .unwrap_or_default();
    Some(MemberView {
        label,
        delivered: fleet.is_delivered(member),
        state,
        detail,
        work_item,
        merge_ready,
    })
}

fn session_state(session: &Session) -> (MemberState, Option<String>) {
    match &session.state {
        SessionState::Running => (MemberState::Running, None),
        SessionState::Waiting { on } => (MemberState::Waiting, Some(on.clone())),
        SessionState::Blocked { denial } => {
            (MemberState::Blocked, Some(denial.candidate_rule.clone()))
        }
        SessionState::Idle => (MemberState::Idle, None),
        SessionState::Ended { .. } => (MemberState::Done, None),
        SessionState::Died { cause } => {
            let skill = session
                .active_skill
                .as_deref()
                .map(|skill| format!(", last skill {skill}"))
                .unwrap_or_default();
            (MemberState::Died, Some(format!("{cause}{skill}")))
        }
    }
}

/// The plan dirs whose worktree root is the row of `key` (P2's `WorkItemKey.root`), comma-joined.
fn work_items(repo: &Repo, key: &BranchKey) -> Option<String> {
    let row = repo
        .snapshot
        .as_ref()?
        .rows
        .iter()
        .find(|row| row.key == *key)?;
    let root = row.worktree.as_ref()?;
    let dirs: Vec<&str> = repo
        .work_items
        .keys()
        .filter(|item| item.root == *root)
        .map(|item| item.plan_dir.as_str())
        .collect();
    (!dirs.is_empty()).then(|| dirs.join(", "))
}

/// The branch's name while P2 derives it `merge_ready`.
fn merge_ready(repo: &Repo, key: &BranchKey) -> Option<String> {
    match repo.branches.get(key)?.state {
        BranchState::MergeReady => Some(key_text(key)),
        BranchState::NoWork
        | BranchState::Uncommitted
        | BranchState::Unmerged
        | BranchState::Conflicting
        | BranchState::Merged
        | BranchState::Removable => None,
    }
}

/// The repo's collisions the fleet's scope keeps.
fn collisions(repo: &Repo, scope: &CollisionScope) -> BTreeSet<CollisionFact> {
    repo.collisions
        .iter()
        .filter(|(key, _)| scope.keeps(&key.a, &key.b))
        .map(|(key, collision)| CollisionFact {
            a: key_text(&key.a),
            b: key_text(&key.b),
            level: level_text(key.level).to_owned(),
            paths: collision.paths.clone(),
            symbols: collision.symbols.clone(),
        })
        .collect()
}

/// P2's wire spelling of a level (`CollisionLevelWire`).
fn level_text(level: Level) -> &'static str {
    match level {
        Level::PlannedFile => "planned_file",
        Level::PlannedSymbol => "planned_symbol",
        Level::Touched => "touched",
        Level::Confirmed => "confirmed",
    }
}

fn key_text(key: &BranchKey) -> String {
    match key {
        BranchKey::Branch(name) => name.clone(),
        BranchKey::Detached(path) => path.to_string_lossy().into_owned(),
    }
}

/// The items one event produced for one subscription; `announced` mutes fleet-level facts already delivered. Git
/// items need a scan before the event: the first scan only reveals state, it changes none.
#[must_use]
pub fn diff(before: &View, after: &View, announced: &BTreeSet<String>) -> Vec<(ItemKey, Item)> {
    let mut items = Vec::new();
    for (key, now) in &after.members {
        let Some(was) = before.members.get(key) else {
            continue;
        };
        if now.state != was.state
            && let Some(item) = state_item(now)
        {
            items.push((ItemKey::State(key.clone()), item));
        }
        let is_session = key.1.is_none();
        if is_session && now.delivered && !was.delivered {
            let item = Item::Delivered {
                member: now.label.clone(),
            };
            items.push((ItemKey::Delivered(key.clone()), item));
        }
        if !before.scanned {
            continue;
        }
        if now.work_item != was.work_item
            && let Some(plan) = &now.work_item
        {
            items.push((
                ItemKey::WorkItem(key.clone()),
                Item::WorkItemChanged {
                    member: now.label.clone(),
                    plan: plan.clone(),
                },
            ));
        }
        if let Some(branch) = &now.merge_ready
            && was.merge_ready.is_none()
        {
            let item = Item::MergeReady {
                member: now.label.clone(),
                branch: branch.clone(),
            };
            items.push((ItemKey::MergeReady(key.clone()), item));
        }
    }
    if !before.scanned {
        return items;
    }
    let known: BTreeSet<String> = before.collisions.iter().map(CollisionFact::key).collect();
    for collision in &after.collisions {
        let key = collision.key();
        if !known.contains(&key) && !announced.contains(&key) {
            items.push((ItemKey::Fleet(key), Item::Collision(collision.clone())));
        }
    }
    if let Some(target) = &after.integrated
        && after.integrated != before.integrated
    {
        let key = format!("integrated:{target}");
        if !announced.contains(&key) {
            items.push((
                ItemKey::Fleet(key),
                Item::FleetIntegrated {
                    target: target.clone(),
                },
            ));
        }
    }
    items
}

/// Fleet-level keys `before` showed and `after` no longer shows: they may be announced again.
#[must_use]
pub fn gone(before: &View, after: &View) -> Vec<String> {
    let current: BTreeSet<String> = after.collisions.iter().map(CollisionFact::key).collect();
    let collisions = before
        .collisions
        .iter()
        .map(CollisionFact::key)
        .filter(|key| !current.contains(key));
    let integrated = before
        .integrated
        .iter()
        .filter(|target| after.integrated.as_ref() != Some(*target))
        .map(|target| format!("integrated:{target}"));
    collisions.chain(integrated).collect()
}

fn state_item(member: &MemberView) -> Option<Item> {
    let name = member.label.clone();
    let detail = member.detail.clone();
    match member.state {
        MemberState::Running => None,
        MemberState::Waiting => Some(Item::Waiting {
            member: name,
            detail,
        }),
        MemberState::Blocked => Some(Item::Blocked {
            member: name,
            detail,
        }),
        MemberState::Idle => Some(Item::Idle { member: name }),
        MemberState::Done => Some(Item::Done { member: name }),
        MemberState::Died => Some(Item::Died {
            member: name,
            detail,
        }),
    }
}

/// Whether a pending item still describes `view`: a member-state item whose member moved on is stale.
#[must_use]
pub fn holds(key: &ItemKey, view: &View) -> bool {
    match key {
        ItemKey::State(member) => view
            .members
            .get(member)
            .is_some_and(|now| state_item(now).is_some()),
        ItemKey::Delivered(member) => view.members.get(member).is_some_and(|now| now.delivered),
        ItemKey::Fleet(key) => view.fleet_keys().contains(key),
        ItemKey::WorkItem(_) | ItemKey::MergeReady(_) => true,
    }
}

impl View {
    /// The fleet-level keys this view states: its collisions and, while integrated, `integrated:<target>`.
    #[must_use]
    pub fn fleet_keys(&self) -> BTreeSet<String> {
        let integrated = self
            .integrated
            .iter()
            .map(|target| format!("integrated:{target}"));
        self.collisions
            .iter()
            .map(CollisionFact::key)
            .chain(integrated)
            .collect()
    }
}

/// The first `max` entries comma-joined, then `+N more`.
fn capped(entries: &[String], max: usize) -> String {
    let shown = entries
        .iter()
        .take(max)
        .cloned()
        .collect::<Vec<_>>()
        .join(", ");
    match entries.len().checked_sub(max).filter(|rest| *rest > 0) {
        Some(rest) => format!("{shown} +{rest} more"),
        None => shown,
    }
}

/// The digest text: header, one line per item (at most `caps.digest_lines()`, each collision line showing at most
/// `caps.digest_line_paths()` paths and symbols — `status` has the rest), a pointer to the full status.
#[must_use]
pub fn render(id: DigestId, fleet: &FleetId, items: &[&Item], caps: &Caps) -> String {
    let max_lines = caps.digest_lines();
    let mut text = format!(
        "heartbeat digest {} — fleet {}, {} events\n",
        id.label(),
        fleet.0,
        items.len()
    );
    for item in items.iter().take(max_lines) {
        let _ = writeln!(text, "- {}", line(item, caps.digest_line_paths()));
    }
    if items.len() > max_lines {
        let _ = writeln!(text, "- … and {} more", items.len() - max_lines);
    }
    let _ = write!(
        text,
        "Full state: smine-heartbeat status --fleet {} --format json",
        fleet.0
    );
    text
}

fn line(item: &Item, max_paths: usize) -> String {
    let with = |detail: &Option<String>| {
        detail
            .as_deref()
            .map(|detail| format!(" — {detail}"))
            .unwrap_or_default()
    };
    match item {
        Item::Waiting { member, detail } => format!("waiting({member}){}", with(detail)),
        Item::Idle { member } => format!("idle({member})"),
        Item::Blocked { member, detail } => format!("blocked({member}){}", with(detail)),
        Item::Died { member, detail } => format!("died({member}){}", with(detail)),
        Item::Done { member } => format!("done({member})"),
        Item::Collision(fact) => {
            let symbols = if fact.symbols.is_empty() {
                String::new()
            } else {
                format!("; symbols: {}", capped(&fact.symbols, max_paths))
            };
            format!(
                "collision({}, {}, {}) — paths: {}{symbols}",
                fact.a,
                fact.b,
                fact.level,
                capped(&fact.paths, max_paths)
            )
        }
        Item::WorkItemChanged { member, plan } => format!("work_item_changed({member}, {plan})"),
        Item::MergeReady { member, branch } => format!("merge_ready({member}, {branch})"),
        Item::FleetIntegrated { target } => format!("fleet_integrated({target})"),
        Item::Delivered { member } => format!("delivered({member})"),
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use std::path::PathBuf;

    use jiff::Timestamp;

    use super::*;
    use crate::branch::Branch;
    use crate::collide::{Collision, CollisionKey};
    use crate::fleet::{FleetInstance, Join, Requested, Subagent, SubscriptionState, Worker};
    use crate::ids::{AgentId, InstanceId, MessagingToken, WorkerBranch};
    use crate::plan::PlannedWork;
    use crate::repo::{RepoId, WorkItemKey};
    use crate::scan::{RepoScan, ScannedRow, Train};

    fn member(state: MemberState, detail: Option<&str>) -> MemberView {
        MemberView {
            label: "w".to_owned(),
            delivered: false,
            state,
            detail: detail.map(str::to_owned),
            work_item: None,
            merge_ready: None,
        }
    }

    fn key() -> MemberKey {
        (SessionId("w".to_owned()), None)
    }

    fn with_member(view: MemberView) -> View {
        View {
            members: BTreeMap::from([(key(), view)]),
            ..View::default()
        }
    }

    fn fact(a: &str, b: &str) -> CollisionFact {
        CollisionFact {
            a: a.to_owned(),
            b: b.to_owned(),
            level: "confirmed".to_owned(),
            paths: vec!["shared.txt".to_owned()],
            symbols: Vec::new(),
        }
    }

    fn with_collision(fact: CollisionFact) -> View {
        View {
            collisions: BTreeSet::from([fact]),
            scanned: true,
            ..View::default()
        }
    }

    fn scanned_empty() -> View {
        View {
            scanned: true,
            ..View::default()
        }
    }

    #[test]
    fn diff_first_appearance_is_silent() {
        let after = with_member(member(MemberState::Idle, None));
        assert_eq!(
            diff(&View::default(), &after, &BTreeSet::new()),
            Vec::<(ItemKey, Item)>::new()
        );
    }

    #[test]
    fn diff_state_changes() {
        let cases = [
            (
                "running-to-idle",
                MemberState::Running,
                member(MemberState::Idle, None),
                Some(Item::Idle {
                    member: "w".to_owned(),
                }),
            ),
            (
                "idle-to-waiting",
                MemberState::Idle,
                member(MemberState::Waiting, Some("Bash")),
                Some(Item::Waiting {
                    member: "w".to_owned(),
                    detail: Some("Bash".to_owned()),
                }),
            ),
            (
                "running-stays",
                MemberState::Running,
                member(MemberState::Running, None),
                None,
            ),
            (
                "idle-to-running",
                MemberState::Idle,
                member(MemberState::Running, None),
                None,
            ),
            (
                "running-to-died",
                MemberState::Running,
                member(
                    MemberState::Died,
                    Some("StopFailure: rate_limit, last skill fimplement"),
                ),
                Some(Item::Died {
                    member: "w".to_owned(),
                    detail: Some("StopFailure: rate_limit, last skill fimplement".to_owned()),
                }),
            ),
        ];
        for (label, was, now, want) in cases {
            let items = diff(
                &with_member(member(was, None)),
                &with_member(now),
                &BTreeSet::new(),
            );
            let got: Vec<Item> = items.into_iter().map(|(_, item)| item).collect();
            assert_eq!(got, want.into_iter().collect::<Vec<_>>(), "{label}");
        }
    }

    #[test]
    fn diff_fleet_items_respect_announced() {
        let collision = fact("claude/a", "claude/b");
        let after = with_collision(collision.clone());
        let fresh = diff(&scanned_empty(), &after, &BTreeSet::new());
        assert_eq!(
            fresh,
            vec![(
                ItemKey::Fleet(collision.key()),
                Item::Collision(collision.clone())
            )]
        );
        let muted = diff(&scanned_empty(), &after, &BTreeSet::from([collision.key()]));
        assert_eq!(muted, Vec::<(ItemKey, Item)>::new());
        let first_scan = diff(&View::default(), &after, &BTreeSet::new());
        assert!(
            first_scan.is_empty(),
            "the first scan reveals, it announces nothing"
        );
    }

    fn with_paths(fact: CollisionFact, paths: &[&str]) -> CollisionFact {
        CollisionFact {
            paths: paths.iter().map(|path| (*path).to_owned()).collect(),
            ..fact
        }
    }

    #[test]
    fn diff_path_shift_is_no_news() {
        let before = with_collision(fact("claude/a", "claude/b"));
        let shifted = with_collision(with_paths(fact("claude/a", "claude/b"), &["other.txt"]));
        assert!(
            diff(&before, &shifted, &BTreeSet::new()).is_empty(),
            "same pair and level, other paths"
        );
        let mut added = shifted.clone();
        let pair = fact("claude/a", "claude/c");
        added.collisions.insert(pair.clone());
        assert_eq!(
            diff(&before, &added, &BTreeSet::new()),
            vec![(ItemKey::Fleet(pair.key()), Item::Collision(pair))],
            "new pair"
        );
    }

    #[test]
    fn gone_lists_resolved_collisions() {
        let collision = fact("claude/a", "claude/b");
        assert_eq!(
            gone(&with_collision(collision.clone()), &View::default()),
            vec![collision.key()],
            "pair absent"
        );
        let shifted = with_collision(with_paths(collision.clone(), &["other.txt"]));
        assert!(
            gone(&with_collision(collision), &shifted).is_empty(),
            "pair still present with other paths"
        );
    }

    #[test]
    fn holds_drops_stale_state() {
        let running = with_member(member(MemberState::Running, None));
        assert!(!holds(&ItemKey::State(key()), &running));
        let idle = with_member(member(MemberState::Idle, None));
        assert!(holds(&ItemKey::State(key()), &idle));
    }

    #[test]
    fn delivered_item_carries_the_token() {
        let mut fleet = bound(true);
        let h = SessionId("h".to_owned());
        let mut session = Session::new(h.clone(), SessionState::Idle, Timestamp::UNIX_EPOCH, false);
        session.cwd = Some(PathBuf::from(WT_H));
        fleet.sessions.insert(h.clone(), session);
        let t1 = FleetId("T1".to_owned());
        let _ = fleet
            .instances
            .get_mut(&t1)
            .expect("T1 run")
            .member_ids
            .assign(&(h.clone(), None));
        let join = Join {
            fleet: t1,
            token: MessagingToken("t".to_owned()),
            since: Timestamp::UNIX_EPOCH,
        };
        fleet.joins.insert(h.clone(), join);

        let was = o_view(&fleet);
        fleet.delivered.insert(h.clone(), Timestamp::UNIX_EPOCH);
        let now = o_view(&fleet);
        let items = diff(&was, &now, &BTreeSet::new());
        let want = Item::Delivered {
            member: "T1-MA h".to_owned(),
        };
        assert_eq!(
            items,
            vec![(ItemKey::Delivered((h.clone(), None)), want.clone())]
        );
        assert_eq!(line(&want, 8), "delivered(T1-MA h)");
        assert!(holds(&ItemKey::Delivered((h.clone(), None)), &now));
        fleet.delivered.remove(&h);
        assert!(
            !holds(&ItemKey::Delivered((h, None)), &o_view(&fleet)),
            "resumed: stale"
        );
    }

    #[test]
    fn render_caps_lines() {
        let item = Item::Idle {
            member: "w".to_owned(),
        };
        let items: Vec<&Item> = std::iter::repeat_n(&item, 60).collect();
        let text = render(
            DigestId(3),
            &FleetId("T1".to_owned()),
            &items,
            &Caps::default(),
        );
        assert!(text.starts_with("heartbeat digest d3 — fleet T1, 60 events\n"));
        assert_eq!(text.lines().filter(|line| *line == "- idle(w)").count(), 50);
        assert!(text.contains("- … and 10 more\n"));
    }

    #[test]
    fn render_pins_text() {
        let item = Item::Idle {
            member: "CC,FD - work - T1[WIP]".to_owned(),
        };
        let text = render(
            DigestId(1),
            &FleetId("T1".to_owned()),
            &[&item],
            &Caps::default(),
        );
        assert_eq!(
            text,
            "heartbeat digest d1 — fleet T1, 1 events\n\
             - idle(CC,FD - work - T1[WIP])\n\
             Full state: smine-heartbeat status --fleet T1 --format json"
        );
    }

    const R1: &str = "/r1";
    const WT_A: &str = "/r1/.claude/worktrees/t1-a";
    /// The fixture run's registered worker branch.
    const WORKER_BRANCH: &str = "claude/t1-a";
    const WT_H: &str = "/r1/.claude/worktrees/human";
    const WT_H2: &str = "/r1/.claude/worktrees/h2";
    const WT_O: &str = "/r1/.claude/worktrees/o";
    const MERGE: &str = "claude-orch/T1-00000000-0000-7431-8000-000000000000";

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

    fn lane(name: &str) -> BranchKey {
        BranchKey::Branch(name.to_owned())
    }

    fn set_state(fleet: &mut Fleet, name: &str, state: BranchState) {
        let repo = fleet.repos.get_mut(&RepoId(PathBuf::from(R1))).expect("r1");
        repo.branches.insert(
            lane(name),
            Branch {
                state,
                since: Timestamp::UNIX_EPOCH,
                judged: "main".to_owned(),
                target: None,
            },
        );
    }

    fn collide(fleet: &mut Fleet, a: &str, b: &str) {
        let repo = fleet.repos.get_mut(&RepoId(PathBuf::from(R1))).expect("r1");
        repo.collisions.insert(
            CollisionKey {
                level: Level::Confirmed,
                a: lane(a),
                b: lane(b),
            },
            Collision {
                paths: vec!["shared.txt".to_owned()],
                symbols: Vec::new(),
                plans: Vec::new(),
                since: Timestamp::UNIX_EPOCH,
            },
        );
    }

    /// T1 bound to `/r1` `main` by idle `o` on its merge branch `claude-orch/T1` (with a running subagent `a1`), whose
    /// registered, unlettered worker `claude/t1-a` (unmerged) is running.
    fn bound(snapshot: bool) -> Fleet {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let mut repo = Repo::new("r".to_owned());
        let train = |base: &str| Train {
            base: base.to_owned(),
            order: Vec::new(),
            steps: BTreeMap::new(),
        };
        repo.snapshot = snapshot.then(|| RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows: vec![
                row("main", R1, true),
                row(MERGE, WT_O, false),
                row(WORKER_BRANCH, WT_A, false),
                row("claude/human", WT_H, false),
                row("claude/h2", WT_H2, false),
            ],
            pairs: Vec::new(),
            trains: vec![train("main"), train(MERGE)],
            requested: vec!["main".to_owned(), MERGE.to_owned()],
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        });
        fleet.repos.insert(RepoId(PathBuf::from(R1)), repo);
        let mut o = Session::new(
            SessionId("o".to_owned()),
            SessionState::Idle,
            Timestamp::UNIX_EPOCH,
            false,
        );
        o.cwd = Some(PathBuf::from(WT_O));
        o.subagents.insert(
            AgentId("a1".to_owned()),
            Subagent {
                agent_type: "general-purpose".to_owned(),
                running: true,
                started_at: Timestamp::UNIX_EPOCH,
                model: None,
                requested: Requested::Unknown,
            },
        );
        fleet.sessions.insert(o.id.clone(), o);
        let t1 = FleetId("T1".to_owned());
        let run = InstanceId::legacy(&t1, Timestamp::UNIX_EPOCH);
        let mut instance = FleetInstance::new(run, t1.clone(), Timestamp::UNIX_EPOCH);
        instance.workers.insert(
            WorkerBranch(WORKER_BRANCH.to_owned()),
            Worker {
                by: SessionId("o".to_owned()),
                since: Timestamp::UNIX_EPOCH,
                delivered: false,
            },
        );
        fleet.instances.insert(t1, instance);
        fleet.subscriptions.insert(
            SessionId("o".to_owned()),
            Subscription {
                fleet: FleetId("T1".to_owned()),
                instance: Some(run),
                repo: Some(RepoId(PathBuf::from(R1))),
                target: "main".to_owned(),
                socket: PathBuf::from("/tmp/o.sock"),
                token: MessagingToken("t".to_owned()),
                state: SubscriptionState::Active,
                since: Timestamp::UNIX_EPOCH,
                pending: BTreeMap::new(),
                in_flight: None,
                last: None,
                announced: BTreeSet::new(),
                worker_branches_seen: BTreeSet::from([lane(WORKER_BRANCH)]),
                outside_collisions: false,
            },
        );
        set_state(&mut fleet, WORKER_BRANCH, BranchState::Unmerged);
        set_state(&mut fleet, "claude/human", BranchState::Unmerged);
        set_state(&mut fleet, "claude/h2", BranchState::Unmerged);
        fleet
    }

    fn o_view(fleet: &Fleet) -> View {
        let o = SessionId("o".to_owned());
        view(fleet, &o, &fleet.subscriptions[&o])
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn p2_items() {
        type Change = Box<dyn Fn(&mut Fleet)>;
        let worker = "worker claude/t1-a".to_owned();
        let cases: Vec<(&str, bool, Change, Vec<Item>)> = vec![
            (
                "worker-merge-ready-item",
                true,
                Box::new(|fleet: &mut Fleet| {
                    set_state(fleet, WORKER_BRANCH, BranchState::MergeReady);
                }),
                vec![Item::MergeReady {
                    member: worker.clone(),
                    branch: WORKER_BRANCH.to_owned(),
                }],
            ),
            (
                "collision-member-vs-merge-branch",
                true,
                Box::new(|fleet: &mut Fleet| collide(fleet, WORKER_BRANCH, MERGE)),
                vec![Item::Collision(fact(WORKER_BRANCH, MERGE))],
            ),
            (
                "collision-member-vs-outsider-dropped",
                true,
                Box::new(|fleet: &mut Fleet| collide(fleet, WORKER_BRANCH, "claude/human")),
                vec![],
            ),
            (
                "collision-member-vs-outsider-with-outside",
                true,
                Box::new(|fleet: &mut Fleet| {
                    fleet
                        .subscriptions
                        .get_mut(&SessionId("o".to_owned()))
                        .expect("o")
                        .outside_collisions = true;
                    collide(fleet, WORKER_BRANCH, "claude/human");
                }),
                vec![Item::Collision(fact(WORKER_BRANCH, "claude/human"))],
            ),
            (
                "collision-member-vs-target-dropped",
                true,
                Box::new(|fleet: &mut Fleet| collide(fleet, WORKER_BRANCH, "main")),
                vec![],
            ),
            (
                "collision-merge-branch-vs-outsider-dropped",
                true,
                Box::new(|fleet: &mut Fleet| collide(fleet, MERGE, "main")),
                vec![],
            ),
            (
                "collision-between-non-members",
                true,
                Box::new(|fleet: &mut Fleet| collide(fleet, "claude/h2", "claude/human")),
                vec![],
            ),
            (
                "work-item-changed",
                true,
                Box::new(|fleet: &mut Fleet| {
                    let repo = fleet.repos.get_mut(&RepoId(PathBuf::from(R1))).expect("r1");
                    repo.work_items.insert(
                        WorkItemKey {
                            root: PathBuf::from(WT_A),
                            plan_dir: "2026-09-27-x-wip-abc1234".to_owned(),
                        },
                        PlannedWork::default(),
                    );
                }),
                vec![Item::WorkItemChanged {
                    member: worker.clone(),
                    plan: "2026-09-27-x-wip-abc1234".to_owned(),
                }],
            ),
            (
                "fleet-integrated",
                true,
                Box::new(|fleet: &mut Fleet| {
                    set_state(fleet, WORKER_BRANCH, BranchState::Merged);
                    fleet
                        .instances
                        .get_mut(&FleetId("T1".to_owned()))
                        .expect("T1 run")
                        .workers
                        .get_mut(&WorkerBranch(WORKER_BRANCH.to_owned()))
                        .expect("worker")
                        .delivered = true;
                }),
                vec![Item::FleetIntegrated {
                    target: "main".to_owned(),
                }],
            ),
            (
                "worker-delivered-is-no-item",
                true,
                Box::new(|fleet: &mut Fleet| {
                    fleet
                        .instances
                        .get_mut(&FleetId("T1".to_owned()))
                        .expect("T1 run")
                        .workers
                        .get_mut(&WorkerBranch(WORKER_BRANCH.to_owned()))
                        .expect("worker")
                        .delivered = true;
                }),
                vec![],
            ),
            (
                "subagent-stop-is-no-item",
                true,
                Box::new(|fleet: &mut Fleet| {
                    let o = fleet
                        .sessions
                        .get_mut(&SessionId("o".to_owned()))
                        .expect("o");
                    o.subagents
                        .get_mut(&AgentId("a1".to_owned()))
                        .expect("a1")
                        .running = false;
                }),
                vec![],
            ),
            (
                "first-scan-is-no-news",
                false,
                Box::new(|fleet: &mut Fleet| {
                    let scanned = bound(true);
                    let id = RepoId(PathBuf::from(R1));
                    let snapshot = scanned.repos[&id].snapshot.clone();
                    fleet.repos.get_mut(&id).expect("r1").snapshot = snapshot;
                    set_state(fleet, WORKER_BRANCH, BranchState::MergeReady);
                    collide(fleet, WORKER_BRANCH, "claude/human");
                }),
                vec![],
            ),
            (
                "no-snapshot-no-git-items",
                false,
                Box::new(|fleet: &mut Fleet| {
                    set_state(fleet, WORKER_BRANCH, BranchState::Merged);
                    let repo = fleet.repos.get_mut(&RepoId(PathBuf::from(R1))).expect("r1");
                    repo.work_items.insert(
                        WorkItemKey {
                            root: PathBuf::from(WT_A),
                            plan_dir: "2026-09-27-x-wip-abc1234".to_owned(),
                        },
                        PlannedWork::default(),
                    );
                }),
                vec![],
            ),
        ];
        for (label, snapshot, change, want) in cases {
            let mut fleet = bound(snapshot);
            let before = o_view(&fleet);
            change(&mut fleet);
            let after = o_view(&fleet);
            let got: Vec<Item> = diff(&before, &after, &BTreeSet::new())
                .into_iter()
                .map(|(_, item)| item)
                .collect();
            assert_eq!(got, want, "{label}");
        }
    }

    #[test]
    fn worker_views() {
        type Setup = Box<dyn Fn(&mut Fleet)>;
        let worker_key = (
            SessionId("o".to_owned()),
            Some(WorkerBranch(WORKER_BRANCH.to_owned())),
        );
        let lettered = |fleet: &mut Fleet| {
            let _ = fleet
                .instances
                .get_mut(&FleetId("T1".to_owned()))
                .expect("T1 run")
                .member_ids
                .assign(&(
                    SessionId("o".to_owned()),
                    Some(WorkerBranch(WORKER_BRANCH.to_owned())),
                ));
        };
        let cases: Vec<(&str, Setup, &str)> = vec![
            (
                "worker-view-labelled-by-branch",
                Box::new(lettered),
                "T1-MA worker claude/t1-a",
            ),
            (
                "worker-without-letter-label",
                Box::new(|_: &mut Fleet| {}),
                "worker claude/t1-a",
            ),
        ];
        for (label, setup, want) in cases {
            let mut fleet = bound(true);
            setup(&mut fleet);
            let view = o_view(&fleet);
            let keys: Vec<&MemberKey> = view.members.keys().collect();
            assert_eq!(
                keys,
                vec![&worker_key],
                "{label}: subscriber-not-in-own-view, its subagent no member"
            );
            let member = &view.members[&worker_key];
            assert_eq!(member.label, want, "{label}");
            assert_eq!(member.state, MemberState::Running, "{label}");
            assert_eq!(member.detail, None, "{label}");
        }
    }

    #[test]
    fn holds_fleet_items() {
        let collision = fact(WORKER_BRANCH, MERGE);
        let integrated = ItemKey::Fleet("integrated:main".to_owned());
        let collided = ItemKey::Fleet(collision.key());
        let open = View::default();
        let stating = View {
            collisions: BTreeSet::from([collision]),
            integrated: Some("main".to_owned()),
            ..View::default()
        };
        let cases = [
            ("integrated-while-open-dropped", &integrated, &open, false),
            ("integrated-still-stated-held", &integrated, &stating, true),
            ("collision-gone-dropped", &collided, &open, false),
            ("collision-present-held", &collided, &stating, true),
        ];
        for (label, key, view, want) in cases {
            assert_eq!(holds(key, view), want, "{label}");
        }
    }

    #[test]
    fn capped_paths() {
        let paths = |count: usize| {
            (0..count)
                .map(|index| format!("p{index}"))
                .collect::<Vec<_>>()
        };
        let max = Caps::default().digest_line_paths();
        assert_eq!(capped(&paths(3), max), "p0, p1, p2", "under the cap");
        assert_eq!(
            capped(&paths(8), max),
            "p0, p1, p2, p3, p4, p5, p6, p7",
            "at the cap"
        );
        assert_eq!(
            capped(&paths(87), max),
            "p0, p1, p2, p3, p4, p5, p6, p7 +79 more",
            "the T1 planned_file line"
        );
    }
}
