//! agentwt runner edge: executes `ScanRepo` and `ParsePlan` off the engine thread. A scan waits the settle window
//! (`scan_settle`: the ref writes and the `PostToolUse` of one git command land inside it, so one command costs one scan),
//! stamps `started_at`, then runs `agentwt::status::build`, the work-item discovery and the reflog creation lookups on
//! `spawn_blocking` (each blocking call opens its own gix handles) and reports `RepoScanned`. The
//! core keeps at most one scan per repo in flight (D3); the per-repo gauge here measures it, and the reading travels
//! in `RepoScanned.in_flight` for the load bench.

use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::time::Duration;

use jiff::Timestamp;
use tokio::sync::mpsc::{self, Sender};

use crate::engine::Input;
use crate::event::Event;
use crate::plan::{self, PlannedWork};
use crate::repo::{RepoId, WorkItemKey};
use crate::scan::{FoundWork, LandingGit, RepoScan, ScanError};

#[derive(Debug)]
enum Job {
    Scan {
        repo: RepoId,
        train_bases: Vec<String>,
        candidates: Vec<String>,
    },
    Plan {
        repo: RepoId,
        item: WorkItemKey,
        default_branch: String,
        train_bases: Vec<String>,
    },
}

/// The core's handle on the runner; sending never blocks the core.
#[derive(Debug, Clone)]
pub struct Runner {
    jobs: mpsc::UnboundedSender<Job>,
}

impl Runner {
    /// Queues one scan.
    pub fn scan(&self, repo: RepoId, train_bases: Vec<String>, candidates: Vec<String>) {
        let _ = self.jobs.send(Job::Scan {
            repo,
            train_bases,
            candidates,
        });
    }

    /// Queues one plan re-read.
    pub fn parse(
        &self,
        repo: RepoId,
        item: WorkItemKey,
        default_branch: String,
        train_bases: Vec<String>,
    ) {
        let _ = self.jobs.send(Job::Plan {
            repo,
            item,
            default_branch,
            train_bases,
        });
    }
}

/// Scans in flight per repo; a repo enters on its first scan, so the set follows the registry.
type Gauges = Arc<Mutex<BTreeMap<RepoId, u64>>>;

/// Starts the runner task on the current runtime (call from `serve::run`); `staging` names the staging target whose
/// release facts each scan reads (none: all-false facts); `settle` is the quiet time before each scan.
#[must_use]
pub fn start(
    inbox: Sender<Input>,
    claude_home: PathBuf,
    staging: Option<String>,
    settle: Duration,
) -> Runner {
    let (jobs, mut queue) = mpsc::unbounded_channel::<Job>();
    let gauges: Gauges = Arc::new(Mutex::new(BTreeMap::new()));
    let claude_home = Arc::new(claude_home);
    let staging = Arc::new(staging);
    tokio::spawn(async move {
        while let Some(job) = queue.recv().await {
            tokio::spawn(run(
                job,
                inbox.clone(),
                Arc::clone(&claude_home),
                Arc::clone(&staging),
                settle,
                Arc::clone(&gauges),
            ));
        }
    });
    Runner { jobs }
}

/// Moves `repo`'s in-flight gauge by `delta` and reads it; a poisoned lock keeps serving its data.
fn adjust_gauge(gauges: &Gauges, repo: &RepoId, delta: i64) -> u64 {
    let mut gauges = gauges.lock().unwrap_or_else(PoisonError::into_inner);
    let gauge = gauges.entry(repo.clone()).or_insert(0);
    *gauge = gauge.saturating_add_signed(delta);
    *gauge
}

async fn run(
    job: Job,
    inbox: Sender<Input>,
    claude_home: Arc<PathBuf>,
    staging: Arc<Option<String>>,
    settle: Duration,
    gauges: Gauges,
) {
    let event = match job {
        Job::Scan {
            repo,
            train_bases,
            candidates,
        } => {
            tokio::time::sleep(settle).await;
            let started_at = Timestamp::now();
            let in_flight = adjust_gauge(&gauges, &repo, 1);
            let id = repo.clone();
            let outcome = tokio::task::spawn_blocking(move || {
                scan(
                    &id,
                    train_bases,
                    &candidates,
                    &claude_home,
                    staging.as_deref(),
                )
            })
            .await
            .unwrap_or_else(|err| Err(ScanError::Task(err.to_string())));
            adjust_gauge(&gauges, &repo, -1);
            Event::RepoScanned {
                repo,
                started_at,
                in_flight,
                outcome: outcome.map(Box::new),
                at: Timestamp::now(),
            }
        }
        Job::Plan {
            repo,
            item,
            default_branch,
            train_bases,
        } => {
            let path = repo.0.clone();
            let key = item.clone();
            let read = tokio::task::spawn_blocking(move || {
                let view = RepoView::open(&path, &default_branch, &train_bases)?;
                read_plan_dir(&view, &key.root, &key.plan_dir)
            })
            .await;
            let Ok(Ok(plan)) = read else {
                return;
            };
            Event::PlanParsed {
                repo,
                item,
                plan,
                at: Timestamp::now(),
            }
        }
    };
    let _ = inbox.send(Input::Event(event)).await;
}

/// One scan on a blocking thread: the agentwt document, the local branches no agent owns, the staging target's
/// release facts, the work items of its worktrees, and which branches still sit at their creation commit.
fn scan(
    repo: &RepoId,
    train_bases: Vec<String>,
    candidates: &[String],
    claude_home: &Path,
    staging: Option<&str>,
) -> Result<RepoScan, ScanError> {
    let options = agentwt::status::Options {
        repo: repo.0.clone(),
        claude_homes: vec![claude_home.to_path_buf()],
        codex_home: None,
        skip_order: false,
        branches: None,
        train_bases: train_bases.clone(),
        candidates: candidates.to_vec(),
    };
    let document =
        agentwt::status::build(&options).map_err(|err| ScanError::Agentwt(chain(&err)))?;
    let view = RepoView::open(&repo.0, &document.repo.default_branch, &train_bases)?;
    let locals: Vec<String> = document
        .candidates
        .iter()
        .filter(|name| !candidates.contains(name))
        .cloned()
        .collect();
    let landing = match staging {
        Some(staging) => view.landing(staging, locals.iter().any(|name| name == staging))?,
        None => LandingGit::default(),
    };
    let mut work = Vec::new();
    let mut warnings = Vec::new();
    for worktree in document
        .rows
        .iter()
        .filter_map(|row| row.worktree.as_ref())
        .filter(|wt| !wt.main)
    {
        found_in(&view, Path::new(&worktree.path), &mut work, &mut warnings);
    }
    let mut fresh = BTreeSet::new();
    for row in &document.rows {
        let Some(branch) = row.branch.as_deref() else {
            continue;
        };
        if view.created(branch).ok().flatten().as_deref() == Some(row.head.sha.as_str()) {
            fresh.insert(branch.to_owned());
        }
    }
    Ok(RepoScan::from_document(
        &document,
        train_bases,
        locals,
        work,
        warnings,
        &fresh,
        landing,
    ))
}

/// An error and every `source()` below it, `: `-joined, so an io cause under a gix error shows; a cause whose text
/// the line already holds (a wrapper that prints its source) is not repeated.
fn chain(err: &dyn std::error::Error) -> String {
    let mut text = err.to_string();
    let mut source = err.source();
    while let Some(cause) = source {
        let cause_text = cause.to_string();
        if !text.contains(&cause_text) {
            text.push_str(": ");
            text.push_str(&cause_text);
        }
        source = cause.source();
    }
    text
}

/// One gix handle on the repository, opened on the blocking thread that uses it.
struct RepoView {
    repo: gix::Repository,
    /// The tip trees of the default branch and of every requested train base (target, merge branch) that exists: a
    /// plan file any of them holds is merged work, no branch's pending plan.
    bases: Vec<gix::ObjectId>,
    /// The default branch's tip commit, for fork-point lookups.
    tip: gix::ObjectId,
}

impl RepoView {
    fn open(
        repo: &Path,
        default_branch: &str,
        train_bases: &[String],
    ) -> Result<RepoView, ScanError> {
        let repo = gix::open(repo).map_err(|err| ScanError::Git(err.to_string()))?;
        let (tip, tree) = tip_and_tree(&repo, default_branch)?
            .ok_or_else(|| ScanError::Git(format!("default branch {default_branch} not found")))?;
        let mut bases = vec![tree];
        for base in train_bases {
            bases.extend(tip_and_tree(&repo, base)?.map(|(_, tree)| tree));
        }
        Ok(RepoView { repo, bases, tip })
    }

    /// The tree of the fork point of the worktree at `root` with the default branch: plans already there when the
    /// branch forked were not added by it, even when the default branch has since archived them. An error when the
    /// worktree's HEAD or the merge base cannot be read: the plan dir is then skipped with a warning, never counted
    /// against the base tips alone.
    fn fork_tree(&self, root: &Path) -> Result<gix::ObjectId, ScanError> {
        let git = |err: &dyn std::fmt::Display| ScanError::Git(err.to_string());
        let head = gix::open(root)
            .map_err(|err| git(&err))?
            .head_id()
            .map_err(|err| git(&err))?
            .detach();
        let base = self
            .repo
            .merge_base(head, self.tip)
            .map_err(|err| git(&err))?;
        let commit = base
            .object()
            .map_err(|err| git(&err))?
            .peel_to_commit()
            .map_err(|err| git(&err))?;
        Ok(commit.tree_id().map_err(|err| git(&err))?.detach())
    }

    /// Every stored version of `rel`: at the fork point `fork` and in each base tip tree.
    fn versions(&self, fork: gix::ObjectId, rel: &str) -> Result<Vec<String>, ScanError> {
        let mut held = Vec::new();
        for tree in std::iter::once(fork).chain(self.bases.iter().copied()) {
            held.extend(self.read_in(tree, rel)?);
        }
        Ok(held)
    }

    /// `rel`'s text in `tree`, lossily decoded; `None` when absent.
    fn read_in(&self, tree: gix::ObjectId, rel: &str) -> Result<Option<String>, ScanError> {
        let git = |err: &dyn std::fmt::Display| ScanError::Git(err.to_string());
        let tree = self.repo.find_tree(tree).map_err(|err| git(&err))?;
        let Some(entry) = tree.lookup_entry_by_path(rel).map_err(|err| git(&err))? else {
            return Ok(None);
        };
        let object = entry.object().map_err(|err| git(&err))?;
        Ok(Some(String::from_utf8_lossy(&object.data).into_owned()))
    }

    /// The commit `branch` was created at: the new oid of its oldest reflog line; `None` without a reflog.
    fn created(&self, branch: &str) -> Result<Option<String>, ScanError> {
        let git = |err: &dyn std::fmt::Display| ScanError::Git(err.to_string());
        let reference = self
            .repo
            .find_reference(format!("refs/heads/{branch}").as_str())
            .map_err(|err| git(&err))?;
        let mut log = reference.log_iter();
        let Some(mut lines) = log.all().map_err(|err| git(&err))? else {
            return Ok(None);
        };
        Ok(lines
            .find_map(Result::ok)
            .map(|line| line.new_oid.to_string()))
    }

    /// The staging target's release facts against the default tip; all-false when agentwt does not list `staging` as
    /// a local branch (`is_listed`) or it is no local branch.
    fn landing(&self, staging: &str, is_listed: bool) -> Result<LandingGit, ScanError> {
        let git = |err: &dyn std::fmt::Display| ScanError::Git(err.to_string());
        if !is_listed {
            return Ok(LandingGit::default());
        }
        let Some((tip, _)) = tip_and_tree(&self.repo, staging)? else {
            return Ok(LandingGit::default());
        };
        let releasable = tip == self.tip
            || self
                .repo
                .merge_bases_many(tip, &[self.tip])
                .map_err(|err| git(&err))?
                .iter()
                .any(|base| base.detach() == self.tip);
        let unreleased = self
            .repo
            .rev_walk([tip])
            .with_hidden([self.tip])
            .all()
            .map_err(|err| git(&err))?
            .count();
        Ok(LandingGit {
            exists: true,
            releasable,
            unreleased: u32::try_from(unreleased).unwrap_or(u32::MAX),
        })
    }
}

/// `branch`'s tip commit and tree; `None` when it is no local branch.
fn tip_and_tree(
    repo: &gix::Repository,
    branch: &str,
) -> Result<Option<(gix::ObjectId, gix::ObjectId)>, ScanError> {
    let git = |err: &dyn std::fmt::Display| ScanError::Git(err.to_string());
    let Some(mut reference) = repo
        .try_find_reference(format!("refs/heads/{branch}").as_str())
        .map_err(|err| git(&err))?
    else {
        return Ok(None);
    };
    let commit = reference.peel_to_commit().map_err(|err| git(&err))?;
    let tree = commit.tree_id().map_err(|err| git(&err))?.detach();
    Ok(Some((commit.id, tree)))
}

/// Work items of one worktree: the wip plan dirs of its feature folders holding a design file that is new or changed
/// against every base tip (default branch, fleet targets, merge branches) and the fork point (D9).
fn found_in(view: &RepoView, root: &Path, work: &mut Vec<FoundWork>, warnings: &mut Vec<String>) {
    for plan_dir in wip_plan_dirs(&root.join("plans")) {
        match read_plan_dir(view, root, &plan_dir) {
            Ok(Some(plan)) => work.push(FoundWork {
                key: WorkItemKey {
                    root: root.to_path_buf(),
                    plan_dir,
                },
                plan,
            }),
            Ok(None) => {}
            Err(err) => warnings.push(format!("{}/plans/{plan_dir}: {err}", root.display())),
        }
    }
}

/// The entry names of `dir`; an unreadable dir has none.
fn dir_names(dir: &Path) -> Vec<String> {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return Vec::new();
    };
    entries
        .flatten()
        .filter_map(|entry| entry.file_name().into_string().ok())
        .collect()
}

/// The wip plan dirs under `plans`, each as `<feature folder>/<plan dir>`.
fn wip_plan_dirs(plans: &Path) -> Vec<String> {
    let mut found = Vec::new();
    for feature in dir_names(plans) {
        for dir in dir_names(&plans.join(&feature)) {
            if dir.contains("-wip-") {
                found.push(format!("{feature}/{dir}"));
            }
        }
    }
    found
}

/// The planned work of `<root>/plans/<plan_dir>/design/` the branch added: a design file counts whole unless its text
/// equals a stored version (fork point, base tips). `None` when nothing counts.
fn read_plan_dir(
    view: &RepoView,
    root: &Path,
    plan_dir: &str,
) -> Result<Option<PlannedWork>, ScanError> {
    let design = root.join("plans").join(plan_dir).join("design");
    let Ok(entries) = std::fs::read_dir(&design) else {
        return Ok(None);
    };
    let fork = view.fork_tree(root)?;
    let mut work = PlannedWork::default();
    let mut counts = false;
    for name in entries
        .flatten()
        .filter_map(|entry| entry.file_name().into_string().ok())
        .filter(|name| plan::is_markdown(name))
    {
        let rel = format!("plans/{plan_dir}/design/{name}");
        let held = view.versions(fork, &rel)?;
        let path = design.join(&name);
        let text = std::fs::read_to_string(&path).map_err(|err| ScanError::Read {
            path: path.display().to_string(),
            detail: err.to_string(),
        })?;
        if !held.contains(&text) {
            work.merge(plan::parse(&text));
            counts = true;
        }
    }
    Ok(counts.then_some(work))
}
