//! Git state per registered repository (P2): scan coalescing, work items, touched paths and the derived branch and
//! collision tables. Called only from [`crate::apply::apply`]; pure — no clock, no I/O.
//!
//! Coalescing (D3): a trigger starts a scan when none runs, else only moves `last_trigger`; a finished scan whose
//! `started_at` is not after the last trigger is followed by exactly one more — at most one in flight, one queued.
//! A scan carries the request (train bases, candidates) of its send time, so a request change while one is in
//! flight always queues one more, however late that scan started.
//!
//! Worker attribution (D20): a subagent's branch is computed from its `agent_id` ([`agent_branch`]), never observed
//! from a cwd.
//!
//! Train bases and pins (P3): a repo's train bases are its fleets' targets and merge branches (`Fleet::train_bases`),
//! the merge branches are also tracked as containment candidates (`Fleet::candidates`), and each fleet member's branch
//! is judged against its fleet's merge-branch train (`Fleet::pins`).

use std::collections::{BTreeMap, BTreeSet};
use std::path::{Component, Path, PathBuf};

use jiff::Timestamp;
use serde_json::Value;

use crate::action::Action;
use crate::apply::apply_sensor;
use crate::branch::{self, Branch};
use crate::collide::{self, Collision, CollisionKey};
use crate::config::Exemptions;
use crate::event::{HookEvent, HookKind, ToolCall};
use crate::fleet::{Fleet, SensorName, SensorState};
use crate::gitcmd;
use crate::ids::AgentId;
use crate::plan::{self, PlannedWork};
use crate::scan::{RepoScan, ScanError};

/// A registered repository, by its canonical main-checkout path.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct RepoId(pub PathBuf);

/// One agentwt row: an agent branch, or a detached worktree by its path.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum BranchKey {
    /// An agent branch.
    Branch(String),
    /// A detached worktree.
    Detached(PathBuf),
}

/// A plan dir inside one worktree (concept D9).
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct WorkItemKey {
    /// The worktree root.
    pub root: PathBuf,
    /// The plan dir inside its feature folder, `<feature folder>/<date>-<slug>-wip-<hash>`.
    pub plan_dir: String,
}

/// Scan coalescing state.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub enum ScanState {
    /// No scan runs.
    #[default]
    Idle,
    /// A scan was requested and has not reported back.
    Running,
}

/// One registered repository.
#[derive(Debug, Clone, PartialEq)]
pub struct Repo {
    /// Registry name.
    pub name: String,
    /// Extra merge-train bases: the targets and merge branches of the active subscriptions bound to this repo (P3).
    pub train_bases: Vec<String>,
    /// Branches tracked as containment candidates on request: the merge branches of the fleets bound to this repo.
    pub candidates: Vec<String>,
    /// The member-branch pins the branch table was last requested for (`Fleet::pins`): a change asks for a rescan, so a
    /// member who joins after the subscription is judged against its fleet's merge branch.
    pub pins: BTreeMap<String, String>,
    /// The request (train bases, candidates, pins) changed while a scan was in flight: that scan was sent with the old
    /// request, so another follows it whatever its `started_at`.
    pub is_request_changed: bool,
    /// Coalescing state.
    pub scan: ScanState,
    /// The latest trigger.
    pub last_trigger: Option<Timestamp>,
    /// The last scan's error; `None` after a good scan.
    pub scan_error: Option<String>,
    /// The last good scan.
    pub snapshot: Option<RepoScan>,
    /// Derived branch facts, by row.
    pub branches: BTreeMap<BranchKey, Branch>,
    /// Plan dirs in flight.
    pub work_items: BTreeMap<WorkItemKey, PlannedWork>,
    /// Repo-relative paths agent sessions edited, with the worktree roots they edited them in.
    pub touched: BTreeMap<String, BTreeSet<PathBuf>>,
    /// Current collisions.
    pub collisions: BTreeMap<CollisionKey, Collision>,
}

impl Repo {
    /// A repository before its first scan.
    #[must_use]
    pub fn new(name: String) -> Repo {
        Repo {
            name,
            train_bases: Vec::new(),
            candidates: Vec::new(),
            pins: BTreeMap::new(),
            is_request_changed: false,
            scan: ScanState::Idle,
            last_trigger: None,
            scan_error: None,
            snapshot: None,
            branches: BTreeMap::new(),
            work_items: BTreeMap::new(),
            touched: BTreeMap::new(),
            collisions: BTreeMap::new(),
        }
    }

    /// The worktree `path` lies in: `<repo>/.claude/worktrees/<name>`, else the deepest scanned non-main worktree;
    /// `None` for the main checkout and outside every worktree.
    #[must_use]
    pub fn worktree_root(&self, id: &RepoId, path: &Path) -> Option<PathBuf> {
        let pool = id.0.join(".claude").join("worktrees");
        if let Ok(rest) = path.strip_prefix(&pool)
            && let Some(Component::Normal(name)) = rest.components().next()
        {
            return Some(pool.join(name));
        }
        self.snapshot
            .as_ref()?
            .rows
            .iter()
            .filter(|row| !row.main)
            .filter_map(|row| row.worktree.as_deref())
            .filter(|root| path.starts_with(root))
            .max_by_key(|root| root.as_os_str().len())
            .map(Path::to_path_buf)
    }
}

/// The registered repository `path` belongs to: the longest registered prefix, else the repo one of whose scanned
/// worktrees (a pool outside the repo dir) contains it.
#[must_use]
pub fn repo_of(fleet: &Fleet, path: &Path) -> Option<RepoId> {
    let registered = fleet
        .repos
        .keys()
        .filter(|id| path.starts_with(&id.0))
        .max_by_key(|id| id.0.as_os_str().len())
        .cloned();
    registered.or_else(|| {
        fleet
            .repos
            .iter()
            .find(|(_, repo)| {
                repo.snapshot.as_ref().is_some_and(|scan| {
                    scan.rows
                        .iter()
                        .filter_map(|row| row.worktree.as_deref())
                        .any(|root| path.starts_with(root))
                })
            })
            .map(|(id, _)| id.clone())
    })
}

/// The branch an Agent-tool worker with worktree isolation runs on: the `WorktreeCreate` hook names directory and branch
/// after the worktree name `agent-<agent_id>` (exploration-attribution D1).
#[must_use]
pub fn agent_branch(agent: &AgentId) -> BranchKey {
    BranchKey::Branch(format!("claude/agent-{}", agent.0))
}

/// A hook's git signals (D4): git-changing Bash and turn ends in a worktree trigger a scan; Edit/Write record touched
/// paths and re-read a plan dir they write into.
pub(crate) fn on_hook(fleet: &mut Fleet, hook: &HookEvent) -> Vec<Action> {
    let at = hook.at;
    match &hook.kind {
        HookKind::PostToolUse(call) => on_tool(fleet, at, hook.cwd.as_deref(), call),
        HookKind::PostToolUseFailure(call) if call.tool_name == "Bash" => {
            on_tool(fleet, at, hook.cwd.as_deref(), call)
        }
        HookKind::Stop | HookKind::SubagentStop => on_turn_end(fleet, at, hook.cwd.as_deref()),
        HookKind::SessionStart { .. }
        | HookKind::SessionEnd { .. }
        | HookKind::UserPromptSubmit { .. }
        | HookKind::PreToolUse(_)
        | HookKind::PostToolUseFailure(_)
        | HookKind::PermissionRequest(_)
        | HookKind::Notification { .. }
        | HookKind::StopFailure { .. }
        | HookKind::SubagentStart
        | HookKind::Other { .. } => Vec::new(),
    }
}

fn on_turn_end(fleet: &mut Fleet, at: Timestamp, cwd: Option<&Path>) -> Vec<Action> {
    let Some(cwd) = cwd else {
        return Vec::new();
    };
    let Some(id) = repo_of(fleet, cwd) else {
        return Vec::new();
    };
    let in_worktree = fleet
        .repos
        .get(&id)
        .is_some_and(|repo| repo.worktree_root(&id, cwd).is_some());
    if in_worktree {
        trigger(fleet, at, &id)
    } else {
        Vec::new()
    }
}

fn on_tool(fleet: &mut Fleet, at: Timestamp, cwd: Option<&Path>, call: &ToolCall) -> Vec<Action> {
    match call.tool_name.as_str() {
        "Bash" => {
            let changes = call.command().is_some_and(gitcmd::changes_git);
            match cwd.and_then(|cwd| repo_of(fleet, cwd)) {
                Some(id) if changes => trigger(fleet, at, &id),
                Some(_) | None => Vec::new(),
            }
        }
        "Edit" | "MultiEdit" | "Write" | "NotebookEdit" => on_edit(fleet, at, call),
        _ => Vec::new(),
    }
}

/// An edit inside an agent worktree: queue a plan re-read for plan files, record the touch unless exempt (D12, D13).
fn on_edit(fleet: &mut Fleet, at: Timestamp, call: &ToolCall) -> Vec<Action> {
    let Some(path) = ["file_path", "notebook_path"]
        .iter()
        .find_map(|key| call.input.get(*key)?.as_str())
        .map(Path::new)
    else {
        return Vec::new();
    };
    let Some(id) = repo_of(fleet, path) else {
        return Vec::new();
    };
    let Fleet {
        repos,
        exemptions,
        lane_prefixes,
        ..
    } = fleet;
    let Some(repo) = repos.get_mut(&id) else {
        return Vec::new();
    };
    let Some(root) = repo.worktree_root(&id, path) else {
        return Vec::new();
    };
    let Some(rel) = relative(&root, path) else {
        return Vec::new();
    };
    let mut actions = Vec::new();
    if let (Some(plan_dir), Some(scan)) = (plan::plan_dir_of(&rel), repo.snapshot.as_ref()) {
        actions.push(Action::ParsePlan {
            repo: id.clone(),
            item: WorkItemKey {
                root: root.clone(),
                plan_dir,
            },
            default_branch: scan.default_branch.clone(),
            train_bases: scan.requested.clone(),
        });
    }
    if !exemptions.path(&rel) && !version_only_edit(exemptions, call) {
        repo.touched.entry(rel).or_default().insert(root);
        repo.collisions = collide::detect(repo, exemptions, lane_prefixes, at);
    }
    actions
}

/// `path` below `root`, `/`-joined; `None` outside it or for a non-UTF-8 component.
fn relative(root: &Path, path: &Path) -> Option<String> {
    let parts: Option<Vec<&str>> = path
        .strip_prefix(root)
        .ok()?
        .components()
        .map(|component| match component {
            Component::Normal(part) => part.to_str(),
            Component::Prefix(_)
            | Component::RootDir
            | Component::CurDir
            | Component::ParentDir => None,
        })
        .collect();
    parts
        .filter(|parts| !parts.is_empty())
        .map(|parts| parts.join("/"))
}

/// An `Edit`/`MultiEdit` whose every changed line assigns a version key; a `Write` never is.
fn version_only_edit(exemptions: &Exemptions, call: &ToolCall) -> bool {
    let edits: Vec<(&str, &str)> = match call.tool_name.as_str() {
        "Edit" => edit_pair(&call.input).into_iter().collect(),
        "MultiEdit" => call
            .input
            .get("edits")
            .and_then(Value::as_array)
            .map(|edits| edits.iter().filter_map(edit_pair).collect())
            .unwrap_or_default(),
        _ => Vec::new(),
    };
    !edits.is_empty()
        && edits
            .iter()
            .all(|(old, new)| exemptions.version_only(&one_sided_lines(old, new)))
}

fn edit_pair(input: &Value) -> Option<(&str, &str)> {
    Some((
        input.get("old_string")?.as_str()?,
        input.get("new_string")?.as_str()?,
    ))
}

/// Lines present on only one side of an edit.
fn one_sided_lines<'a>(old: &'a str, new: &'a str) -> Vec<&'a str> {
    let before: BTreeSet<&str> = old.lines().collect();
    let after: BTreeSet<&str> = new.lines().collect();
    before.symmetric_difference(&after).copied().collect()
}

/// The registry re-read (repos edge): a repo not known yet enters before its first scan and is triggered, a known one
/// keeps its state under its current name, one no longer registered leaves with its git state; the git watch
/// follows the new set.
pub(crate) fn on_registry(
    fleet: &mut Fleet,
    at: Timestamp,
    repos: Vec<(RepoId, String)>,
) -> Vec<Action> {
    let wanted: Vec<RepoId> = repos.iter().map(|(id, _)| id.clone()).collect();
    fleet.repos.retain(|id, _| wanted.contains(id));
    let mut actions = vec![Action::WatchRepos { repos: wanted }];
    for (id, name) in repos {
        if let Some(repo) = fleet.repos.get_mut(&id) {
            repo.name = name;
            continue;
        }
        fleet.repos.insert(id.clone(), Repo::new(name));
        actions.extend(trigger(fleet, at, &id));
    }
    actions
}

/// A scan trigger for `id` (D3).
pub(crate) fn trigger(fleet: &mut Fleet, at: Timestamp, id: &RepoId) -> Vec<Action> {
    let Some(repo) = fleet.repos.get_mut(id) else {
        return Vec::new();
    };
    repo.last_trigger = repo.last_trigger.max(Some(at));
    match repo.scan {
        ScanState::Running => Vec::new(),
        ScanState::Idle => {
            repo.scan = ScanState::Running;
            vec![scan_action(id, repo)]
        }
    }
}

fn scan_action(id: &RepoId, repo: &Repo) -> Action {
    Action::ScanRepo {
        repo: id.clone(),
        train_bases: repo.train_bases.clone(),
        candidates: repo.candidates.clone(),
    }
}

/// A finished scan: fold it (or keep the last good one and record the error), re-derive branches, work items,
/// touched paths and collisions, then start the queued scan when a trigger arrived at or after this scan started.
pub(crate) fn on_scanned(
    fleet: &mut Fleet,
    at: Timestamp,
    id: &RepoId,
    started_at: Timestamp,
    in_flight: u64,
    outcome: Result<Box<RepoScan>, ScanError>,
) -> Vec<Action> {
    fleet.scans_completed += 1;
    fleet.scan_in_flight_high_water = fleet.scan_in_flight_high_water.max(in_flight);
    let pins = fleet.pins(id);
    let Fleet {
        repos,
        exemptions,
        lane_prefixes,
        ..
    } = &mut *fleet;
    let Some(repo) = repos.get_mut(id) else {
        return Vec::new();
    };
    match outcome {
        Ok(scan) => {
            repo.scan_error = None;
            repo.work_items = scan
                .work
                .iter()
                .map(|found| (found.key.clone(), found.plan.clone()))
                .collect();
            repo.branches = branch::derive(&scan, &repo.branches, &pins, at);
            let live: BTreeSet<&Path> = scan
                .rows
                .iter()
                .filter(|row| {
                    repo.branches
                        .get(&row.key)
                        .is_some_and(|branch| branch.state.is_live())
                })
                .filter_map(|row| row.worktree.as_deref())
                .collect();
            repo.touched.retain(|_, roots| {
                roots.retain(|root| live.contains(root.as_path()));
                !roots.is_empty()
            });
            repo.snapshot = Some(*scan);
        }
        Err(error) => repo.scan_error = Some(error.to_string()),
    }
    repo.collisions = collide::detect(repo, exemptions, lane_prefixes, at);
    let is_rescan =
        repo.is_request_changed || repo.last_trigger.is_some_and(|last| last >= started_at);
    let actions = if is_rescan {
        repo.is_request_changed = false;
        vec![scan_action(id, repo)]
    } else {
        repo.scan = ScanState::Idle;
        Vec::new()
    };
    report_scans(fleet, at);
    actions
}

/// A re-read plan dir: replace its planned work, or drop it when it holds no new design file.
pub(crate) fn on_plan(
    fleet: &mut Fleet,
    at: Timestamp,
    id: &RepoId,
    item: WorkItemKey,
    plan: Option<PlannedWork>,
) {
    let Fleet {
        repos,
        exemptions,
        lane_prefixes,
        ..
    } = fleet;
    let Some(repo) = repos.get_mut(id) else {
        return;
    };
    match plan {
        Some(plan) => {
            repo.work_items.insert(item, plan);
        }
        None => {
            repo.work_items.remove(&item);
        }
    }
    repo.collisions = collide::detect(repo, exemptions, lane_prefixes, at);
}

/// Sensor `agentwt`: degraded while any repo's last scan failed, one `<repo>: <error>` per failing repo (D15).
fn report_scans(fleet: &mut Fleet, at: Timestamp) {
    let failures: Vec<String> = fleet
        .repos
        .iter()
        .filter_map(|(id, repo)| Some(format!("{}: {}", id.0.display(), repo.scan_error.as_ref()?)))
        .collect();
    let (state, detail) = if failures.is_empty() {
        (SensorState::Ok, None)
    } else {
        (SensorState::Degraded, Some(failures.join("; ")))
    };
    apply_sensor(fleet, SensorName::Agentwt, state, detail, at);
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use serde_json::json;

    use super::*;
    use crate::apply::apply;
    use crate::event::Event;
    use crate::ids::SessionId;
    use crate::scan::ScannedRow;

    const REPO: &str = "/r";
    const WORKTREE: &str = "/r/.claude/worktrees/a";
    const OUTSIDE_POOL: &str = "/pool/x";

    /// Label, whether the repo has a snapshot, the edit, the actions it yields and the touched paths after it.
    type EditCase = (&'static str, bool, ToolCall, Vec<Action>, Vec<&'static str>);

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    fn id() -> RepoId {
        RepoId(PathBuf::from(REPO))
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

    fn scan() -> RepoScan {
        RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows: vec![
                scanned("main", REPO, true),
                scanned("claude/a", WORKTREE, false),
                scanned("claude/x", OUTSIDE_POOL, false),
            ],
            pairs: Vec::new(),
            trains: Vec::new(),
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        }
    }

    /// A fleet with repo `/r` registered; `snapshot` seeds its last scan.
    fn fleet(snapshot: bool) -> Fleet {
        let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
        let mut repo = Repo::new("r".to_owned());
        repo.snapshot = snapshot.then(scan);
        fleet.repos.insert(id(), repo);
        fleet.exemptions = Exemptions::new(&["plans/**".to_owned()], vec!["version".to_owned()])
            .expect("patterns compile");
        fleet
    }

    fn hook(cwd: &str, kind: HookKind) -> Event {
        Event::Hook(HookEvent {
            session_id: SessionId("s".to_owned()),
            agent: None,
            cwd: Some(PathBuf::from(cwd)),
            transcript_path: None,
            at: at(1),
            kind,
            response: None,
            enforce: None,
            muted: false,
            in_flight: 0,
        })
    }

    fn call(tool: &str, input: Value) -> ToolCall {
        ToolCall {
            tool_name: tool.to_owned(),
            input,
        }
    }

    fn bash(command: &str) -> ToolCall {
        call("Bash", json!({ "command": command }))
    }

    fn scan_repo() -> Action {
        Action::ScanRepo {
            repo: id(),
            train_bases: Vec::new(),
            candidates: Vec::new(),
        }
    }

    fn scanned_event(
        started: i64,
        reported: i64,
        outcome: Result<Box<RepoScan>, ScanError>,
    ) -> Event {
        Event::RepoScanned {
            repo: id(),
            started_at: at(started),
            in_flight: 1,
            outcome,
            at: at(reported),
        }
    }

    fn refs(second: i64) -> Event {
        Event::RefsChanged {
            repo: id(),
            at: at(second),
        }
    }

    #[test]
    fn coalescing() {
        let good = || Ok(Box::new(scan()));
        let cases: Vec<(&str, Vec<Event>, Vec<Action>, ScanState)> = vec![
            (
                "idle-trigger-scans",
                vec![refs(1)],
                vec![scan_repo()],
                ScanState::Running,
            ),
            (
                "trigger-while-running",
                vec![refs(1), refs(2)],
                vec![],
                ScanState::Running,
            ),
            (
                "report-without-later-trigger",
                vec![refs(1), refs(2), scanned_event(3, 4, good())],
                vec![],
                ScanState::Idle,
            ),
            (
                "trigger-during-scan",
                vec![refs(1), refs(6), scanned_event(5, 7, good())],
                vec![scan_repo()],
                ScanState::Running,
            ),
            (
                "trigger-at-started-at",
                vec![refs(1), refs(5), scanned_event(5, 7, good())],
                vec![scan_repo()],
                ScanState::Running,
            ),
        ];
        for (label, events, last_actions, state) in cases {
            let mut fleet = fleet(false);
            let mut actions = Vec::new();
            for event in events {
                actions = apply(&mut fleet, event);
            }
            assert_eq!(actions, last_actions, "{label}");
            assert_eq!(fleet.repos[&id()].scan, state, "{label}");
        }
    }

    #[test]
    fn hook_triggers() {
        let cases: Vec<(&str, Event, Vec<Action>)> = vec![
            (
                "commit-in-worktree",
                hook(WORKTREE, HookKind::PostToolUse(bash("git commit -m x"))),
                vec![scan_repo()],
            ),
            (
                "failed-merge",
                hook(
                    WORKTREE,
                    HookKind::PostToolUseFailure(bash("git merge claude/b")),
                ),
                vec![scan_repo()],
            ),
            (
                "read-only-git",
                hook(WORKTREE, HookKind::PostToolUse(bash("git status"))),
                vec![],
            ),
            (
                "bash-outside-every-repo",
                hook("/elsewhere", HookKind::PostToolUse(bash("git commit -m x"))),
                vec![],
            ),
            (
                "worktree-outside-repo-dir",
                hook(OUTSIDE_POOL, HookKind::PostToolUse(bash("git commit -m x"))),
                vec![scan_repo()],
            ),
            (
                "stop-in-worktree",
                hook(WORKTREE, HookKind::Stop),
                vec![scan_repo()],
            ),
            ("stop-in-main-checkout", hook(REPO, HookKind::Stop), vec![]),
        ];
        for (label, event, expected) in cases {
            let mut fleet = fleet(true);
            assert_eq!(apply(&mut fleet, event), expected, "{label}");
        }

        let mut failed_edit = fleet(true);
        let input = json!({ "file_path": format!("{WORKTREE}/src/a.rs"), "old_string": "a", "new_string": "b" });
        let _ = apply(
            &mut failed_edit,
            hook(WORKTREE, HookKind::PostToolUseFailure(call("Edit", input))),
        );
        assert!(failed_edit.repos[&id()].touched.is_empty(), "failed edit");
    }

    #[test]
    fn edits() {
        let edit = |file: &str, old: &str, new: &str| {
            call(
                "Edit",
                json!({ "file_path": file, "old_string": old, "new_string": new }),
            )
        };
        let plan_file =
            format!("{WORKTREE}/plans/2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234/design/raw.md");
        let parse_plan = Action::ParsePlan {
            repo: id(),
            item: WorkItemKey {
                root: PathBuf::from(WORKTREE),
                plan_dir: "2026-09-27-x-abc1234/2026-09-27-x-wip-abc1234".to_owned(),
            },
            default_branch: "main".to_owned(),
            train_bases: Vec::new(),
        };
        let cases: Vec<EditCase> = vec![
            (
                "edit-in-worktree",
                true,
                edit(&format!("{WORKTREE}/src/a.rs"), "a", "b"),
                vec![],
                vec!["src/a.rs"],
            ),
            (
                "edit-in-main-checkout",
                true,
                edit(&format!("{REPO}/src/a.rs"), "a", "b"),
                vec![],
                vec![],
            ),
            (
                "version-only-edit",
                true,
                edit(
                    &format!("{WORKTREE}/skills/x/SKILL.md"),
                    "version: 1.0.0",
                    "version: 1.1.0",
                ),
                vec![],
                vec![],
            ),
            (
                "exempt-path",
                true,
                edit(&format!("{WORKTREE}/plans/notes.md"), "a", "b"),
                vec![],
                vec![],
            ),
            (
                "plan-file-with-snapshot",
                true,
                edit(&plan_file, "a", "b"),
                vec![parse_plan],
                vec![],
            ),
            (
                "plan-file-without-snapshot",
                false,
                edit(&plan_file, "a", "b"),
                vec![],
                vec![],
            ),
        ];
        for (label, snapshot, tool, expected, touched) in cases {
            let mut fleet = fleet(snapshot);
            let actions = apply(&mut fleet, hook(WORKTREE, HookKind::PostToolUse(tool)));
            assert_eq!(actions, expected, "{label}");
            let got: Vec<&str> = fleet.repos[&id()]
                .touched
                .keys()
                .map(String::as_str)
                .collect();
            assert_eq!(got, touched, "{label}");
        }
    }

    #[test]
    fn scan_failure() {
        let mut fleet = fleet(true);
        let _ = apply(&mut fleet, refs(1));
        let _ = apply(
            &mut fleet,
            scanned_event(2, 3, Err(ScanError::Agentwt("boom".to_owned()))),
        );
        let repo = &fleet.repos[&id()];
        assert_eq!(repo.snapshot, Some(scan()), "error keeps the snapshot");
        assert_eq!(repo.scan_error.as_deref(), Some("agentwt: boom"));
        let sensor = &fleet.sensors[&SensorName::Agentwt];
        assert_eq!(sensor.state, SensorState::Degraded);
        assert_eq!(sensor.detail.as_deref(), Some("/r: agentwt: boom"));

        let _ = apply(&mut fleet, refs(4));
        let _ = apply(&mut fleet, scanned_event(5, 6, Ok(Box::new(scan()))));
        assert_eq!(fleet.sensors[&SensorName::Agentwt].state, SensorState::Ok);
        assert_eq!(fleet.repos[&id()].scan_error, None);
    }

    #[test]
    fn agent_branch_naming() {
        assert_eq!(
            agent_branch(&AgentId("a3916720275283eba".to_owned())),
            BranchKey::Branch("claude/agent-a3916720275283eba".to_owned())
        );
    }
}
