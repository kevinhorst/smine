//! Builds the status document from the fleet — pure, called on the engine thread for every `/status` query.
//! Fleets are every `FleetId` a join or a subscription names; their members come from [`Fleet::members`]; a fleet
//! filter scopes the git rows — landing rows included — to the fleet's bound repository, and its collisions to the
//! pairs the fleet's [`CollisionScope`] keeps.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use crate::fleet::{CollisionScope, Fleet, Member, Session, SubscriptionState, member_branch};
use crate::ids::{FleetId, SessionId};
use crate::land::{LandingKey, Turn};
use crate::protocol::STATUS_SCHEMA;
use crate::repo::{self, BranchKey, Repo, RepoId};
use crate::scan::StepOutcome;
use crate::wire::{
    BranchAgentRow, BranchRow, CollisionRow, FleetRow, Health, HeartbeatInfo, JournalRow,
    LandingRow, LandingTurnRow, MemberRow, ReleaseRow, RepoRow, SensorRow, SessionRow,
    StatusDocument, SubscriptionRow, TargetStateWire, TrainRow, TrainStateWire, TrainStepRow,
    WorkItemRow,
};

/// Which sessions a query wants; every set field must match.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Filter {
    /// One session.
    pub session: Option<SessionId>,
    /// Sessions whose cwd lies under this path.
    pub repo: Option<PathBuf>,
    /// Members of this fleet only (sessions, fleet row, member rows); git rows of its bound repo only.
    pub fleet: Option<FleetId>,
    /// Sessions whose cwd lies in this worktree root; git rows of that worktree only (compared as given, so the
    /// caller canonicalizes).
    pub worktree: Option<PathBuf>,
}

impl Filter {
    /// The query-string pairs of the set fields.
    #[must_use]
    pub fn query_pairs(&self) -> Vec<(&'static str, String)> {
        let session = self.session.as_ref().map(|id| ("session", id.0.clone()));
        let repo = self
            .repo
            .as_deref()
            .and_then(std::path::Path::to_str)
            .map(|repo| ("repo", repo.to_owned()));
        let fleet = self.fleet.as_ref().map(|id| ("fleet", id.0.clone()));
        let worktree = self
            .worktree
            .as_deref()
            .and_then(std::path::Path::to_str)
            .map(|root| ("worktree", root.to_owned()));
        [session, repo, fleet, worktree]
            .into_iter()
            .flatten()
            .collect()
    }

    fn matches(&self, session: &Session) -> bool {
        let is_under = |path: Option<&Path>| {
            path.is_none_or(|path| {
                session
                    .cwd
                    .as_ref()
                    .is_some_and(|cwd| cwd.starts_with(path))
            })
        };
        let is_session_match = self.session.as_ref().is_none_or(|id| *id == session.id);
        is_session_match && is_under(self.repo.as_deref()) && is_under(self.worktree.as_deref())
    }
}

/// The status document for `filter`, headed by the daemon's `health`.
#[must_use]
pub fn document(fleet: &Fleet, filter: &Filter, health: &Health) -> StatusDocument {
    let repos = selected(fleet, filter);
    let fleet_ids: Vec<FleetId> = match &filter.fleet {
        Some(id) => vec![id.clone()],
        None => fleet.fleet_ids().into_iter().collect(),
    };
    let members: Vec<(&FleetId, Member)> = fleet_ids
        .iter()
        .flat_map(|id| {
            fleet
                .members(id)
                .into_iter()
                .map(move |member| (id, member))
        })
        .collect();
    let in_fleet: Option<BTreeSet<&SessionId>> = filter
        .fleet
        .as_ref()
        .map(|_| members.iter().map(|(_, member)| &member.session).collect());
    let scope = filter
        .fleet
        .as_ref()
        .and_then(|id| fleet.collision_scope(id));
    StatusDocument {
        schema: STATUS_SCHEMA,
        heartbeat: HeartbeatInfo {
            version: health.version.clone(),
            claude_home: health.claude_home.clone(),
            addr: health.addr.clone(),
            started_at: fleet.started_at.to_string(),
            events_applied: fleet.events_applied,
            scans_completed: fleet.scans_completed,
            scan_in_flight_high_water: fleet.scan_in_flight_high_water,
        },
        staging_target: fleet.staging.as_ref().map(|target| target.0.clone()),
        sensors: fleet.sensors.iter().map(SensorRow::from).collect(),
        sessions: fleet
            .sessions
            .values()
            .filter(|session| {
                filter.matches(session)
                    && in_fleet
                        .as_ref()
                        .is_none_or(|ids| ids.contains(&session.id))
            })
            .map(SessionRow::from)
            .collect(),
        repos: repo_rows(&repos),
        branches: branch_rows(fleet, &repos),
        work_items: work_item_rows(&repos),
        collisions: collision_rows(&repos, scope.as_ref()),
        trains: train_rows(&repos),
        landings: landing_rows(fleet, &repos),
        fleets: fleet_ids
            .iter()
            .map(|id| fleet_row(fleet, id, &members))
            .collect(),
        members: members
            .iter()
            .map(|(id, member)| member_row(fleet, id, member))
            .collect(),
    }
}

/// A repo a request selects, with the worktree root a `session` filter narrows it to.
struct Selected<'a> {
    id: &'a RepoId,
    repo: &'a Repo,
    only: Option<PathBuf>,
}

/// `fleet` filter: the fleet's bound repo only, none without a binding; `repo` filter: repos the path lies in or
/// under; `session` filter: the repo and worktree of that session's cwd; `worktree` filter: the repo whose last scan
/// holds that worktree root, narrowed to it.
fn selected<'a>(fleet: &'a Fleet, filter: &Filter) -> Vec<Selected<'a>> {
    let bound: Option<Option<&RepoId>> = filter
        .fleet
        .as_ref()
        .map(|id| fleet.binding(id).map(|(repo, _)| repo));
    let session_cwd = filter.session.as_ref().map(|id| {
        fleet
            .sessions
            .get(id)
            .and_then(|session| session.cwd.clone())
    });
    fleet
        .repos
        .iter()
        .filter(|(id, _)| bound.is_none_or(|bound| bound == Some(*id)))
        .filter(|(id, _)| {
            filter
                .repo
                .as_ref()
                .is_none_or(|path| path.starts_with(&id.0) || id.0.starts_with(path))
        })
        .filter_map(|(id, repo)| match &session_cwd {
            None => Some(Selected {
                id,
                repo,
                only: None,
            }),
            Some(cwd) => {
                let cwd = cwd.as_deref().filter(|cwd| cwd.starts_with(&id.0))?;
                Some(Selected {
                    id,
                    repo,
                    only: Some(repo.worktree_root(id, cwd)?),
                })
            }
        })
        .filter_map(|selected| match &filter.worktree {
            None => Some(selected),
            Some(root) => {
                let is_held = selected.repo.snapshot.as_ref().is_some_and(|scan| {
                    scan.rows
                        .iter()
                        .any(|row| row.worktree.as_ref() == Some(root))
                });
                let is_agreed = selected.only.as_ref().is_none_or(|only| only == root);
                (is_held && is_agreed).then(|| Selected {
                    only: Some(root.clone()),
                    ..selected
                })
            }
        })
        .collect()
}

fn keeps(only: Option<&PathBuf>, root: Option<&Path>) -> bool {
    only.is_none_or(|only| root == Some(only.as_path()))
}

fn lane_text(key: &BranchKey) -> String {
    match key {
        BranchKey::Branch(name) => name.clone(),
        BranchKey::Detached(path) => path_text(path),
    }
}

fn path_text(path: &Path) -> String {
    path.to_string_lossy().into_owned()
}

fn repo_rows(repos: &[Selected<'_>]) -> Vec<RepoRow> {
    repos
        .iter()
        .map(|Selected { id, repo, .. }| RepoRow {
            repo: path_text(&id.0),
            name: repo.name.clone(),
            scan_error: repo.scan_error.clone(),
            warnings: repo
                .snapshot
                .as_ref()
                .map(|scan| scan.warnings.clone())
                .unwrap_or_default(),
        })
        .collect()
}

/// One row per landing with a holder or a queue, plus each selected repo's staging row with the last scan's release
/// facts (all-false before the first good scan); sorted by repo, then target. A free turn has no waiter, so a holder
/// selects every landing that has a holder or a queue.
fn landing_rows(fleet: &Fleet, repos: &[Selected<'_>]) -> Vec<LandingRow> {
    let turn_row = |turn: &Turn| LandingTurnRow {
        session_id: turn.session.0.clone(),
        title: fleet
            .sessions
            .get(&turn.session)
            .and_then(|session| session.title.clone()),
        since: turn.since.to_string(),
    };
    let mut rows = Vec::new();
    for Selected { id, repo, .. } in repos {
        let staging = fleet.staging.as_ref().map(|s| LandingKey {
            repo: (**id).clone(),
            target: s.0.clone(),
        });
        let held = fleet
            .landings
            .turns
            .iter()
            .filter(|(key, landing)| key.repo == **id && landing.holder.is_some())
            .map(|(key, _)| key.clone());
        let keys: BTreeSet<LandingKey> = held.chain(staging.clone()).collect();
        let git = repo
            .snapshot
            .as_ref()
            .map(|scan| scan.landing)
            .unwrap_or_default();
        for key in keys {
            let landing = fleet.landings.turns.get(&key);
            rows.push(LandingRow {
                repo: path_text(&id.0),
                release: (staging.as_ref() == Some(&key)).then_some(ReleaseRow {
                    exists: git.exists,
                    releasable: git.releasable,
                    unreleased: git.unreleased,
                }),
                branch: key.target,
                holder: landing
                    .and_then(|landing| landing.holder.as_ref())
                    .map(turn_row),
                queue: landing
                    .map(|landing| landing.queue.iter().map(&turn_row).collect())
                    .unwrap_or_default(),
            });
        }
    }
    rows
}

fn branch_rows(fleet: &Fleet, repos: &[Selected<'_>]) -> Vec<BranchRow> {
    let mut rows = Vec::new();
    for Selected { id, repo, only } in repos {
        let Some(scan) = &repo.snapshot else {
            continue;
        };
        for row in scan
            .rows
            .iter()
            .filter(|row| keeps(only.as_ref(), row.worktree.as_deref()))
        {
            let Some(branch) = repo.branches.get(&row.key) else {
                continue;
            };
            let name = match &row.key {
                BranchKey::Branch(name) => Some(name.clone()),
                BranchKey::Detached(_) => None,
            };
            let train = scan.trains.iter().find(|train| train.base == branch.judged);
            rows.push(BranchRow {
                repo: path_text(&id.0),
                train_position: name
                    .as_ref()
                    .and_then(|name| train?.steps.get(name))
                    .map(|step| step.position),
                branch: name,
                worktree: row.worktree.as_deref().map(path_text),
                state: branch.state.into(),
                since: branch.since.to_string(),
                target: branch.target.clone(),
                sessions: row
                    .worktree
                    .as_deref()
                    .map(|root| sessions_in(fleet, root))
                    .unwrap_or_default(),
                work_items: repo
                    .work_items
                    .keys()
                    .filter(|item| row.worktree.as_ref() == Some(&item.root))
                    .map(|item| item.plan_dir.clone())
                    .collect(),
                agents: agents_on(fleet, id, &row.key),
            });
        }
    }
    rows
}

/// Root sessions whose cwd lies in the worktree; subagents never move a root cwd (P1 D9).
fn sessions_in(fleet: &Fleet, root: &Path) -> Vec<String> {
    fleet
        .sessions
        .values()
        .filter(|session| {
            session
                .cwd
                .as_ref()
                .is_some_and(|cwd| cwd.starts_with(root))
        })
        .map(|session| session.id.0.clone())
        .collect()
}

/// Subagents whose constructed branch ([`repo::agent_branch`]) is `key`, spawned by a session whose cwd resolves to
/// repo `id` (D20).
fn agents_on(fleet: &Fleet, id: &RepoId, key: &BranchKey) -> Vec<BranchAgentRow> {
    fleet
        .sessions
        .values()
        .filter(|session| {
            session
                .cwd
                .as_deref()
                .and_then(|cwd| repo::repo_of(fleet, cwd))
                .as_ref()
                == Some(id)
        })
        .flat_map(|session| {
            session
                .subagents
                .keys()
                .filter(|agent| repo::agent_branch(agent) == *key)
                .map(|agent| BranchAgentRow {
                    session_id: session.id.0.clone(),
                    agent_id: agent.0.clone(),
                })
        })
        .collect()
}

fn work_item_rows(repos: &[Selected<'_>]) -> Vec<WorkItemRow> {
    let mut rows = Vec::new();
    for Selected { id, repo, only } in repos {
        for (item, work) in repo
            .work_items
            .iter()
            .filter(|(item, _)| keeps(only.as_ref(), Some(item.root.as_path())))
        {
            let row = repo.snapshot.as_ref().and_then(|scan| {
                scan.rows
                    .iter()
                    .find(|row| row.worktree.as_ref() == Some(&item.root))
            });
            let is_live = row.is_none_or(|row| {
                repo.branches
                    .get(&row.key)
                    .is_none_or(|branch| branch.state.is_live())
            });
            if !is_live {
                continue;
            }
            let branch = row.and_then(|row| match &row.key {
                BranchKey::Branch(name) => Some(name.clone()),
                BranchKey::Detached(_) => None,
            });
            rows.push(WorkItemRow {
                repo: path_text(&id.0),
                plan_dir: item.plan_dir.clone(),
                worktree: path_text(&item.root),
                branch,
                planned_files: work.files.iter().map(|file| file.path.clone()).collect(),
                planned_symbols: work
                    .symbols
                    .iter()
                    .map(|(file, name)| format!("{file}::{name}"))
                    .collect(),
            });
        }
    }
    rows
}

/// Every collision of the selected repos; under `--fleet` only the pairs the fleet's scope keeps.
fn collision_rows(repos: &[Selected<'_>], scope: Option<&CollisionScope>) -> Vec<CollisionRow> {
    let mut rows = Vec::new();
    for Selected { id, repo, only } in repos {
        let lane = only.as_ref().and_then(|root| {
            repo.snapshot
                .as_ref()?
                .rows
                .iter()
                .find(|row| row.worktree.as_ref() == Some(root))
                .map(|row| &row.key)
        });
        for (key, collision) in &repo.collisions {
            if only.is_some() && lane.is_none_or(|lane| *lane != key.a && *lane != key.b) {
                continue;
            }
            if scope.is_some_and(|scope| !scope.keeps(&key.a, &key.b)) {
                continue;
            }
            rows.push(CollisionRow {
                repo: path_text(&id.0),
                level: key.level.into(),
                a: lane_text(&key.a),
                b: lane_text(&key.b),
                paths: collision.paths.clone(),
                symbols: collision.symbols.clone(),
                plans: collision.plans.clone(),
                since: collision.since.to_string(),
            });
        }
    }
    rows
}

fn train_rows(repos: &[Selected<'_>]) -> Vec<TrainRow> {
    let mut rows = Vec::new();
    for Selected { id, repo, only } in repos {
        let Some(scan) = repo.snapshot.as_ref().filter(|_| only.is_none()) else {
            continue;
        };
        for train in &scan.trains {
            let mut steps: Vec<TrainStepRow> = train
                .steps
                .iter()
                .map(|(branch, step)| {
                    let (state, blocked_by, paths) = match &step.outcome {
                        StepOutcome::Clean => (TrainStateWire::Clean, Vec::new(), Vec::new()),
                        StepOutcome::Blocked { blocked_by, paths } => {
                            (TrainStateWire::Blocked, blocked_by.clone(), paths.clone())
                        }
                        StepOutcome::Nothing => (TrainStateWire::Nothing, Vec::new(), Vec::new()),
                    };
                    TrainStepRow {
                        branch: branch.clone(),
                        position: step.position,
                        state,
                        blocked_by,
                        paths,
                    }
                })
                .collect();
            steps.sort_by_key(|step| step.position);
            rows.push(TrainRow {
                repo: path_text(&id.0),
                base: train.base.clone(),
                steps,
            });
        }
    }
    rows
}

fn fleet_row(fleet: &Fleet, id: &FleetId, members: &[(&FleetId, Member)]) -> FleetRow {
    let binding = fleet.binding(id);
    let instance = fleet.instances.get(id);
    let live = instance.map(|instance| instance.id);
    FleetRow {
        id: id.0.clone(),
        instance: live.map(|live| live.to_string()),
        started_at: instance.map(|instance| instance.since.to_string()),
        repo: binding.map(|(repo, _)| path_text(&repo.0)),
        target: binding.map(|(_, target)| target.to_owned()),
        merge_branch: fleet.merge_branch(id),
        synced: fleet.synced(id),
        target_state: fleet.target_state(id).map(TargetStateWire::from),
        members: u32::try_from(members.iter().filter(|(of, _)| *of == id).count())
            .unwrap_or(u32::MAX),
        foreign: fleet
            .foreign(id)
            .into_iter()
            .map(|session| session.0)
            .collect(),
        subscriptions: fleet
            .subscriptions
            .iter()
            .filter(|(_, subscription)| {
                subscription.fleet == *id
                    && (subscription.instance == live
                        || (subscription.instance.is_none()
                            && matches!(subscription.state, SubscriptionState::Refused(_))))
            })
            .map(SubscriptionRow::from)
            .collect(),
        journal: instance
            .into_iter()
            .flat_map(|instance| &instance.journal)
            .map(JournalRow::from)
            .collect(),
    }
}

/// A member row; `branch` is the member's branch in the fleet's bound repo.
fn member_row(fleet: &Fleet, id: &FleetId, member: &Member) -> MemberRow {
    let branch = fleet
        .binding(id)
        .and_then(|(repo, _)| member_branch(fleet, repo, member))
        .map(|key| lane_text(&key));
    let key = (member.session.clone(), member.worker.clone());
    let facts = (
        branch,
        fleet.member_id(id, &key),
        fleet.is_delivered(member),
    );
    MemberRow::from((id, member, facts))
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use std::collections::BTreeMap;

    use jiff::Timestamp;

    use super::*;
    use crate::collide::{Collision, CollisionKey, Level};
    use crate::fleet::{
        FleetInstance, Join, JournalEntry, JournalKind, Requested, SessionState, Subagent,
        Subscription, Worker,
    };
    use crate::ids::{AgentId, InstanceId, MemberId, MessagingToken, WorkerBranch};
    use crate::plan::PlannedWork;
    use crate::repo::WorkItemKey;
    use crate::scan::{RepoScan, ScannedRow, Train, TrainStep};
    use crate::wire::ViaWire;

    #[test]
    fn filter_matches() {
        let mut a = Session::new(
            SessionId("a".to_owned()),
            SessionState::Idle,
            Timestamp::UNIX_EPOCH,
            false,
        );
        a.cwd = Some("/repo/.claude/worktrees/x".into());
        let cases = [
            ("empty-matches-all", Filter::default(), true),
            (
                "session-hit",
                Filter {
                    session: Some(SessionId("a".to_owned())),
                    ..Filter::default()
                },
                true,
            ),
            (
                "session-miss",
                Filter {
                    session: Some(SessionId("b".to_owned())),
                    ..Filter::default()
                },
                false,
            ),
            (
                "repo-prefix",
                Filter {
                    repo: Some("/repo".into()),
                    ..Filter::default()
                },
                true,
            ),
            (
                "repo-component-not-string-prefix",
                Filter {
                    repo: Some("/rep".into()),
                    ..Filter::default()
                },
                false,
            ),
        ];
        for (label, filter, expected) in cases {
            assert_eq!(filter.matches(&a), expected, "{label}");
        }
    }

    #[test]
    fn query_pairs_carry_only_set_fields() {
        let filter = Filter {
            session: Some(SessionId("a".to_owned())),
            ..Filter::default()
        };
        assert_eq!(filter.query_pairs(), vec![("session", "a".to_owned())]);
        let fleet = Filter {
            fleet: Some(FleetId("T1".to_owned())),
            ..Filter::default()
        };
        assert_eq!(fleet.query_pairs(), vec![("fleet", "T1".to_owned())]);
    }

    /// `fleet_id`'s live run, started when none is live.
    fn run<'a>(fleet: &'a mut Fleet, fleet_id: &FleetId) -> &'a mut FleetInstance {
        let id = InstanceId::legacy(fleet_id, Timestamp::UNIX_EPOCH);
        fleet
            .instances
            .entry(fleet_id.clone())
            .or_insert_with(|| FleetInstance::new(id, fleet_id.clone(), Timestamp::UNIX_EPOCH))
    }

    /// Records `id`'s join to `fleet_id` with the next free letter.
    fn join(fleet: &mut Fleet, id: &str, fleet_id: &str) {
        let (session, fleet_id) = (SessionId(id.to_owned()), FleetId(fleet_id.to_owned()));
        let _ = run(fleet, &fleet_id)
            .member_ids
            .assign(&(session.clone(), None));
        let join = Join {
            fleet: fleet_id,
            token: MessagingToken("t".to_owned()),
            since: Timestamp::UNIX_EPOCH,
        };
        fleet.joins.insert(session, join);
    }

    /// Inserts `o`'s `subscription`, bound to its fleet's live run.
    fn subscribe(fleet: &mut Fleet, mut subscription: Subscription) {
        subscription.instance = Some(run(fleet, &subscription.fleet.clone()).id);
        fleet
            .subscriptions
            .insert(SessionId("o".to_owned()), subscription);
    }

    fn subscription(fleet: &str, repo: &str, target: &str) -> Subscription {
        Subscription {
            fleet: FleetId(fleet.to_owned()),
            instance: None,
            repo: Some(RepoId(PathBuf::from(repo))),
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

    fn with_sessions(sessions: Vec<Session>) -> Fleet {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        for session in sessions {
            fleet.sessions.insert(session.id.clone(), session);
        }
        fleet
    }

    fn fleet_filter(id: &str) -> Filter {
        Filter {
            fleet: Some(FleetId(id.to_owned())),
            ..Filter::default()
        }
    }

    #[test]
    fn fleet_filter_selects_members() {
        let mut fleet = with_sessions(vec![
            session("a", "/x"),
            session("b", "/x"),
            session("c", "/x"),
        ]);
        join(&mut fleet, "a", "T1");
        join(&mut fleet, "b", "T2");
        let doc = document(&fleet, &fleet_filter("T1"), &health());
        let sessions: Vec<&str> = doc
            .sessions
            .iter()
            .map(|row| row.session_id.as_str())
            .collect();
        assert_eq!(sessions, ["a"]);
        let fleets: Vec<&str> = doc.fleets.iter().map(|row| row.id.as_str()).collect();
        assert_eq!(fleets, ["T1"]);
        let members: Vec<&str> = doc
            .members
            .iter()
            .map(|row| row.session_id.as_str())
            .collect();
        assert_eq!(members, ["a"]);
    }

    #[test]
    fn document_lists_every_named_fleet() {
        let mut fleet = with_sessions(vec![session("a", "/x"), session("b", "/x")]);
        join(&mut fleet, "a", "T2");
        join(&mut fleet, "b", "T1");
        subscribe(&mut fleet, subscription("T3", "/r1", "main"));
        let doc = document(&fleet, &Filter::default(), &health());
        let fleets: Vec<&str> = doc.fleets.iter().map(|row| row.id.as_str()).collect();
        assert_eq!(fleets, ["T1", "T2", "T3"]);
    }

    #[test]
    fn fleet_row_shows_only_the_live_run() {
        let mut fleet = with_sessions(vec![session("a", "/x")]);
        let mut old = subscription("T1", "/r1", "main");
        old.state = SubscriptionState::Ended("unsubscribed by the session".to_owned());
        old.instance = Some(InstanceId::legacy(
            &FleetId("T1".to_owned()),
            Timestamp::from_second(1).expect("time"),
        ));
        fleet.subscriptions.insert(SessionId("o".to_owned()), old);
        join(&mut fleet, "a", "T1");
        let doc = document(&fleet, &fleet_filter("T1"), &health());
        let row = &doc.fleets[0];
        let live = &fleet.instances[&FleetId("T1".to_owned())];
        assert_eq!(row.instance, Some(live.id.to_string()));
        assert_eq!(row.started_at, Some(live.since.to_string()));
        assert!(
            row.subscriptions.is_empty(),
            "the ended subscription of the dissolved run: {:?}",
            row.subscriptions
        );
    }

    /// Repos `/r1` and `/r2`, each with one scanned worktree row `claude/a`, a work item, a collision and trains;
    /// `o` idle in `/r1`.
    fn two_repos() -> Fleet {
        let mut fleet = with_sessions(vec![session("o", "/r1")]);
        for root in ["/r1", "/r2"] {
            let worktree = format!("{root}/.claude/worktrees/a");
            let mut repo = repo_with(vec![
                scanned("claude/a", &worktree, false),
                scanned("claude/b", &format!("{root}/.claude/worktrees/b"), false),
            ]);
            repo.work_items.insert(
                WorkItemKey {
                    root: PathBuf::from(&worktree),
                    plan_dir: "2026-09-27-x-wip-abc1234".to_owned(),
                },
                PlannedWork::default(),
            );
            repo.collisions.insert(
                CollisionKey {
                    level: Level::Touched,
                    a: BranchKey::Branch("claude/a".to_owned()),
                    b: BranchKey::Branch("claude/b".to_owned()),
                },
                Collision {
                    paths: vec!["x.rs".to_owned()],
                    symbols: Vec::new(),
                    plans: Vec::new(),
                    since: Timestamp::UNIX_EPOCH,
                },
            );
            fleet.repos.insert(RepoId(PathBuf::from(root)), repo);
        }
        fleet
    }

    #[test]
    fn fleet_filter_scopes_git_rows() {
        let mut fleet = two_repos();
        fleet.staging = Some(crate::fleet::StagingTarget("develop".to_owned()));
        subscribe(&mut fleet, subscription("T1", "/r1", "main"));
        let repos_of = |doc: &StatusDocument| -> [Vec<String>; 6] {
            let unique = |rows: Vec<String>| {
                let mut rows = rows;
                rows.dedup();
                rows
            };
            [
                unique(doc.repos.iter().map(|row| row.repo.clone()).collect()),
                unique(doc.branches.iter().map(|row| row.repo.clone()).collect()),
                unique(doc.work_items.iter().map(|row| row.repo.clone()).collect()),
                unique(doc.collisions.iter().map(|row| row.repo.clone()).collect()),
                unique(doc.trains.iter().map(|row| row.repo.clone()).collect()),
                unique(doc.landings.iter().map(|row| row.repo.clone()).collect()),
            ]
        };
        let only_r1 = vec!["/r1".to_owned()];
        let bound = document(&fleet, &fleet_filter("T1"), &health());
        // claude/a vs claude/b has no member side: the fleet's collision scope drops it (fleet_collision_scope).
        let mut want: [Vec<String>; 6] = std::array::from_fn(|_| only_r1.clone());
        want[3] = Vec::new();
        assert_eq!(repos_of(&bound), want, "bound");
        let unbound = document(&fleet, &fleet_filter("T9"), &health());
        assert_eq!(
            repos_of(&unbound),
            std::array::from_fn(|_| Vec::<String>::new()),
            "unbound"
        );
        let both = vec!["/r1".to_owned(), "/r2".to_owned()];
        let all = document(&fleet, &Filter::default(), &health());
        assert_eq!(
            repos_of(&all),
            std::array::from_fn(|_| both.clone()),
            "no filter"
        );
    }

    #[test]
    fn fleet_collision_scope() {
        let mut fleet = two_repos();
        subscribe(&mut fleet, subscription("T1", "/r1", "main"));
        fleet.sessions.insert(
            SessionId("a".to_owned()),
            session("a", "/r1/.claude/worktrees/a"),
        );
        join(&mut fleet, "a", "T1");
        let merge = fleet
            .merge_branch(&FleetId("T1".to_owned()))
            .expect("bound");
        let repo = fleet
            .repos
            .get_mut(&RepoId(PathBuf::from("/r1")))
            .expect("r1");
        repo.collisions.insert(
            CollisionKey {
                level: Level::Touched,
                a: BranchKey::Branch(merge.clone()),
                b: BranchKey::Branch("claude/a".to_owned()),
            },
            Collision {
                paths: vec!["y.rs".to_owned()],
                symbols: Vec::new(),
                plans: Vec::new(),
                since: Timestamp::UNIX_EPOCH,
            },
        );
        let pairs = |doc: &StatusDocument| {
            doc.collisions
                .iter()
                .filter(|row| row.repo == "/r1")
                .map(|row| (row.a.clone(), row.b.clone()))
                .collect::<Vec<_>>()
        };
        let pair = |a: &str, b: &str| (a.to_owned(), b.to_owned());
        let scoped = document(&fleet, &fleet_filter("T1"), &health());
        assert_eq!(
            pairs(&scoped),
            vec![pair(&merge, "claude/a")],
            "--fleet: member vs merge branch only"
        );
        fleet
            .subscriptions
            .get_mut(&SessionId("o".to_owned()))
            .expect("o")
            .outside_collisions = true;
        let outside = document(&fleet, &fleet_filter("T1"), &health());
        assert_eq!(
            pairs(&outside),
            vec![pair(&merge, "claude/a"), pair("claude/a", "claude/b")],
            "--outside-collisions: the member's outsider pair too"
        );
        let all = document(&fleet, &Filter::default(), &health());
        assert_eq!(pairs(&all).len(), 2, "unfiltered keeps every pair");
    }

    #[test]
    fn fleet_row_binding() {
        let mut fleet = two_repos();
        let mut o = session("o", "/r1");
        o.subagents.insert(
            AgentId("a1".to_owned()),
            Subagent {
                agent_type: "general-purpose".to_owned(),
                running: false,
                started_at: Timestamp::UNIX_EPOCH,
                model: None,
                requested: Requested::Unknown,
            },
        );
        fleet.sessions.insert(o.id.clone(), o);
        let w = session("w", "/r1/.claude/worktrees/a/src");
        fleet.sessions.insert(w.id.clone(), w);
        join(&mut fleet, "w", "T1");
        let x = session("x", "/elsewhere");
        fleet.sessions.insert(x.id.clone(), x);
        join(&mut fleet, "x", "T1");

        let unbound = document(&fleet, &fleet_filter("T1"), &health());
        let row = &unbound.fleets[0];
        assert_eq!(
            (row.repo.as_ref(), row.target.as_ref(), row.target_state),
            (None, None, None),
            "unbound"
        );

        subscribe(&mut fleet, subscription("T1", "/r1", "main"));
        run(&mut fleet, &FleetId("T1".to_owned())).workers.insert(
            WorkerBranch("claude/t1-a".to_owned()),
            Worker {
                by: SessionId("o".to_owned()),
                since: Timestamp::UNIX_EPOCH,
                delivered: false,
            },
        );
        let doc = document(&fleet, &fleet_filter("T1"), &health());
        let row = &doc.fleets[0];
        assert_eq!(row.repo.as_deref(), Some("/r1"));
        assert_eq!(row.target.as_deref(), Some("main"));
        assert_eq!(row.target_state, Some(TargetStateWire::Open));
        assert_eq!(row.foreign, ["x"]);
        let branch = |session: &str, via: ViaWire| {
            doc.members
                .iter()
                .find(|member| member.session_id == session && member.via == via)
                .and_then(|member| member.branch.clone())
        };
        assert_eq!(branch("o", ViaWire::Worker).as_deref(), Some("claude/t1-a"));
        assert_eq!(branch("w", ViaWire::Joined).as_deref(), Some("claude/a"));
        assert!(
            doc.members
                .iter()
                .all(|member| member.branch.as_deref() != Some("claude/agent-a1")),
            "the subscriber's subagent is no member"
        );
    }

    #[test]
    fn member_rows_carry_letter_delivery_and_journal() {
        let mut fleet = with_sessions(vec![session("o", "/r1"), session("w", "/r1")]);
        fleet
            .repos
            .insert(RepoId(PathBuf::from("/r1")), Repo::new("r".to_owned()));
        subscribe(&mut fleet, subscription("T1", "/r1", "main"));
        join(&mut fleet, "w", "T1");
        fleet
            .delivered
            .insert(SessionId("w".to_owned()), Timestamp::UNIX_EPOCH);
        run(&mut fleet, &FleetId("T1".to_owned()))
            .journal
            .push(JournalEntry {
                kind: JournalKind::Merged,
                members: vec![MemberId("A".to_owned())],
                subject: "merged claude/w".to_owned(),
                detail: Some("delivered and merge-ready".to_owned()),
                by: SessionId("o".to_owned()),
                at: Timestamp::UNIX_EPOCH,
            });
        let doc = document(&fleet, &fleet_filter("T1"), &health());
        let facts: Vec<(&str, Option<&str>, Option<&str>, bool)> = doc
            .members
            .iter()
            .map(|row| {
                (
                    row.session_id.as_str(),
                    row.member.as_deref(),
                    row.token.as_deref(),
                    row.delivered,
                )
            })
            .collect();
        assert_eq!(
            facts,
            [
                ("o", None, Some("T1-O"), false),
                ("w", Some("A"), Some("T1-MA"), true)
            ],
            "the operator is T1-O, never lettered"
        );
        let journal = &doc.fleets[0].journal;
        assert_eq!(journal.len(), 1);
        assert_eq!(
            (
                journal[0].kind.as_str(),
                journal[0].members.as_slice(),
                journal[0].by.as_str()
            ),
            ("merged", ["A".to_owned()].as_slice(), "o")
        );
    }

    fn scanned(name: &str, worktree: &str, main: bool) -> ScannedRow {
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

    /// A repo at `path` whose scan holds `rows`, every row derived against a default train stepping them all.
    fn repo_with(rows: Vec<ScannedRow>) -> Repo {
        let steps = rows
            .iter()
            .filter_map(|row| match &row.key {
                BranchKey::Branch(name) => Some(name.clone()),
                BranchKey::Detached(_) => None,
            })
            .enumerate()
            .map(|(index, name)| {
                let step = TrainStep {
                    position: index + 1,
                    conflicts_with: Vec::new(),
                    outcome: StepOutcome::Clean,
                };
                (name, step)
            })
            .collect();
        let scan = RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows,
            pairs: Vec::new(),
            trains: vec![Train {
                base: "main".to_owned(),
                order: Vec::new(),
                steps,
            }],
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        };
        let mut repo = Repo::new("r".to_owned());
        repo.branches = crate::branch::derive(
            &scan,
            &BTreeMap::new(),
            &BTreeMap::new(),
            Timestamp::UNIX_EPOCH,
        );
        repo.snapshot = Some(scan);
        repo
    }

    fn session(id: &str, cwd: &str) -> Session {
        let mut session = Session::new(
            SessionId(id.to_owned()),
            SessionState::Idle,
            Timestamp::UNIX_EPOCH,
            false,
        );
        session.cwd = Some(cwd.into());
        session
    }

    fn health() -> Health {
        Health {
            version: "0".to_owned(),
            claude_home: "/h".to_owned(),
            addr: "127.0.0.1:0".to_owned(),
        }
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn landing_rows() {
        use crate::land::{Landing, Turn};
        use crate::scan::LandingGit;
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        fleet.staging = Some(crate::fleet::StagingTarget("develop".to_owned()));
        fleet
            .repos
            .insert(RepoId(PathBuf::from("/r1")), Repo::new("r1".to_owned()));
        let mut scanned_repo = repo_with(vec![scanned("main", "/r2", true)]);
        if let Some(scan) = scanned_repo.snapshot.as_mut() {
            scan.landing = LandingGit {
                exists: true,
                releasable: true,
                unreleased: 3,
            };
        }
        fleet
            .repos
            .insert(RepoId(PathBuf::from("/r2")), scanned_repo);
        let mut holder = session("h", "/r2/.claude/worktrees/h");
        holder.title = Some("CC - h - [WIP]".to_owned());
        fleet.sessions.insert(holder.id.clone(), holder);
        let waiter = session("w", "/r2/.claude/worktrees/w");
        fleet.sessions.insert(waiter.id.clone(), waiter);
        let turn = |id: &str| Turn {
            session: SessionId(id.to_owned()),
            token: MessagingToken("t".to_owned()),
            since: Timestamp::UNIX_EPOCH,
        };
        let key = |repo: &str, target: &str| LandingKey {
            repo: RepoId(PathBuf::from(repo)),
            target: target.to_owned(),
        };
        fleet.landings.turns.insert(
            key("/r2", "feature/x"),
            Landing {
                holder: Some(turn("h")),
                queue: [turn("w")].into(),
            },
        );
        fleet
            .landings
            .turns
            .insert(key("/r2", "feature/y"), Landing::default());

        let doc = document(&fleet, &Filter::default(), &health());
        assert_eq!(
            doc.staging_target.as_deref(),
            Some("develop"),
            "staging target named"
        );
        let keys: Vec<(&str, &str)> = doc
            .landings
            .iter()
            .map(|row| (row.repo.as_str(), row.branch.as_str()))
            .collect();
        assert_eq!(
            keys,
            [("/r1", "develop"), ("/r2", "develop"), ("/r2", "feature/x")],
            "staging row per repo, held target's own row, ended landing gone; by repo, then target"
        );
        let fresh = &doc.landings[0];
        assert_eq!(
            fresh.release,
            Some(ReleaseRow {
                exists: false,
                releasable: false,
                unreleased: 0
            }),
            "before the first scan: staging row all-false"
        );
        assert!(
            fresh.holder.is_none() && fresh.queue.is_empty(),
            "free turn"
        );
        assert_eq!(
            doc.landings[1].release,
            Some(ReleaseRow {
                exists: true,
                releasable: true,
                unreleased: 3
            }),
            "scan facts copied onto the staging row"
        );
        let held = &doc.landings[2];
        assert_eq!(held.release, None, "no release facts off the staging row");
        let holder = held.holder.as_ref().expect("holder");
        assert_eq!(
            (holder.session_id.as_str(), holder.title.as_deref()),
            ("h", Some("CC - h - [WIP]"))
        );
        assert_eq!(held.queue.len(), 1);
        assert_eq!(
            (
                held.queue[0].session_id.as_str(),
                held.queue[0].title.as_deref()
            ),
            ("w", None),
            "title absent on the unnamed waiter"
        );
        assert_eq!(held.queue[0].since, Timestamp::UNIX_EPOCH.to_string());

        let by_repo = Filter {
            repo: Some("/r2".into()),
            ..Filter::default()
        };
        let scoped = document(&fleet, &by_repo, &health());
        let repos: Vec<&str> = scoped
            .landings
            .iter()
            .map(|row| row.repo.as_str())
            .collect();
        assert_eq!(repos, ["/r2", "/r2"], "repo filter scopes the rows");

        subscribe(&mut fleet, subscription("T1", "/r1", "main"));
        let bound = document(&fleet, &fleet_filter("T1"), &health());
        let repos: Vec<&str> = bound.landings.iter().map(|row| row.repo.as_str()).collect();
        assert_eq!(repos, ["/r1"], "fleet filter: bound repo only");

        fleet.staging = None;
        let unstaged = document(&fleet, &Filter::default(), &health());
        assert_eq!(unstaged.staging_target, None, "no staging target");
        let keys: Vec<(&str, &str)> = unstaged
            .landings
            .iter()
            .map(|row| (row.repo.as_str(), row.branch.as_str()))
            .collect();
        assert_eq!(
            keys,
            [("/r2", "feature/x")],
            "no staging row without a staging target"
        );
        let json = serde_json::to_value(&unstaged).expect("encode");
        assert!(json.get("staging_target").is_none(), "key omitted");
    }

    #[test]
    fn git_rows_filter() {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let repo = repo_with(vec![
            scanned("main", "/r", true),
            scanned("claude/a", "/r/.claude/worktrees/a", false),
            scanned("claude/b", "/r/.claude/worktrees/b", false),
        ]);
        fleet.repos.insert(RepoId(PathBuf::from("/r")), repo);
        let s = session("s", "/r/.claude/worktrees/a/src");
        fleet.sessions.insert(s.id.clone(), s);

        let by_worktree = Filter {
            repo: Some("/r/.claude/worktrees/a".into()),
            ..Filter::default()
        };
        let admitted = document(&fleet, &by_worktree, &health());
        assert_eq!(admitted.repos.len(), 1, "a worktree path admits its repo");
        assert_eq!(admitted.branches.len(), 3);

        let elsewhere = Filter {
            repo: Some("/other".into()),
            ..Filter::default()
        };
        assert_eq!(
            document(&fleet, &elsewhere, &health()).repos,
            Vec::<crate::wire::RepoRow>::new()
        );

        let by_session = Filter {
            session: Some(SessionId("s".to_owned())),
            ..Filter::default()
        };
        let narrowed = document(&fleet, &by_session, &health());
        let branches: Vec<Option<&str>> = narrowed
            .branches
            .iter()
            .map(|row| row.branch.as_deref())
            .collect();
        assert_eq!(branches, [Some("claude/a")]);
        assert!(
            narrowed.trains.is_empty(),
            "no trains under a session filter"
        );

        let worktree = |root: &str| Filter {
            worktree: Some(root.into()),
            ..Filter::default()
        };
        let cases = [
            (
                "worktree-row",
                "/r/.claude/worktrees/b",
                vec![Some("claude/b")],
            ),
            ("worktree-main", "/r", vec![Some("main")]),
            ("worktree-unscanned", "/r/.claude/worktrees/c", Vec::new()),
            (
                "worktree-subdir-is-no-root",
                "/r/.claude/worktrees/a/src",
                Vec::new(),
            ),
        ];
        for (label, root, want) in cases {
            let doc = document(&fleet, &worktree(root), &health());
            let branches: Vec<Option<&str>> = doc
                .branches
                .iter()
                .map(|row| row.branch.as_deref())
                .collect();
            assert_eq!(branches, want, "{label}");
        }
    }

    #[test]
    fn branch_agents_by_construction() {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        for root in ["/r1", "/r2"] {
            let worktree = format!("{root}/.claude/worktrees/agent-a1");
            let repo = repo_with(vec![scanned("claude/agent-a1", &worktree, false)]);
            fleet.repos.insert(RepoId(PathBuf::from(root)), repo);
        }
        let mut root = session("s", "/r1");
        root.subagents.insert(
            AgentId("a1".to_owned()),
            Subagent {
                agent_type: "general-purpose".to_owned(),
                running: true,
                started_at: Timestamp::UNIX_EPOCH,
                model: None,
                requested: Requested::Unknown,
            },
        );
        fleet.sessions.insert(root.id.clone(), root);

        let rows = branch_rows(&fleet, &selected(&fleet, &Filter::default()));
        let by_repo = |repo: &str| {
            rows.iter()
                .find(|row| row.repo == repo)
                .expect("row per repo")
        };
        let want = vec![BranchAgentRow {
            session_id: "s".to_owned(),
            agent_id: "a1".to_owned(),
        }];
        assert_eq!(by_repo("/r1").agents, want);
        assert!(by_repo("/r2").agents.is_empty(), "other repo never matches");
        assert!(
            by_repo("/r1").sessions.is_empty(),
            "root cwd outside the worktree"
        );
    }
}
