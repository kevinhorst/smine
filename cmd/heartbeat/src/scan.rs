//! One agentwt scan, reduced to the facts the engine folds, plus the work items found in the scanned worktrees.
//! agentwt's wire types stop here. Pure.

use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use agentwt::wire::{Document, Row, RowOrder, TrainOutcome};

use crate::plan::PlannedWork;
use crate::repo::{BranchKey, WorkItemKey};

/// Why a scan or a plan read failed; carried in `Event::RepoScanned` and shown on the `agentwt` sensor.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ScanError {
    /// `agentwt::status::build` failed.
    #[error("agentwt: {0}")]
    Agentwt(String),
    /// Opening the repository or reading a tree failed.
    #[error("git: {0}")]
    Git(String),
    /// Reading a plan file failed.
    #[error("{path}: {detail}")]
    Read {
        /// The file.
        path: String,
        /// The io error.
        detail: String,
    },
    /// The blocking task panicked or was cancelled.
    #[error("scan task: {0}")]
    Task(String),
}

/// One scan of one repository.
#[derive(Debug, Clone, PartialEq)]
pub struct RepoScan {
    /// The repository's default branch.
    pub default_branch: String,
    /// The local branches no agent owns, the default one included: agentwt's candidates minus the merge branches the
    /// scan was asked to add.
    pub locals: Vec<String>,
    /// Every agentwt row.
    pub rows: Vec<ScannedRow>,
    /// Pairwise merge conflicts between rows with own commits.
    pub pairs: Vec<ScannedPair>,
    /// The default branch's train first, then one per requested base.
    pub trains: Vec<Train>,
    /// The train bases this scan was requested with (`ScanRepo.train_bases`), as sent.
    pub requested: Vec<String>,
    /// agentwt warnings plus work-item discovery warnings.
    pub warnings: Vec<String>,
    /// Plan dirs in flight in the scanned worktrees.
    pub work: Vec<FoundWork>,
    /// The staging target's release facts.
    pub landing: LandingGit,
}

/// The staging target against the default branch, read at scan time; all-false without the branch.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct LandingGit {
    /// The staging target is a local branch: agentwt lists it as a candidate.
    pub exists: bool,
    /// The default branch's tip is an ancestor of the staging target's tip: a release is a fast-forward.
    pub releasable: bool,
    /// Commits reachable from the staging target's tip and not from the default branch's tip.
    pub unreleased: u32,
}

/// One agentwt row.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScannedRow {
    /// Branch, or detached worktree path.
    pub key: BranchKey,
    /// Canonical worktree path; `None` for a branch without a checkout.
    pub worktree: Option<PathBuf>,
    /// The worktree is the main checkout.
    pub main: bool,
    /// Tracked changes, staged or unstaged.
    pub tracked_changes: usize,
    /// Untracked files after agentwt's infrastructure filter.
    pub untracked: usize,
    /// Commits not on the base.
    pub own_commits: usize,
    /// The tip is still the commit the branch was created at (oldest reflog line): a worker that has not started.
    pub fresh: bool,
    /// The resolved base branch.
    pub from: Option<String>,
    /// The one candidate holding all of the work.
    pub contained_in: Option<String>,
    /// agentwt's removal predicate.
    pub safe_to_remove: bool,
}

/// One conflicting pair.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ScannedPair {
    /// First branch.
    pub a: String,
    /// Second branch.
    pub b: String,
    /// Unresolved conflict paths.
    pub paths: Vec<String>,
}

/// One merge train.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Train {
    /// The branch the train starts from.
    pub base: String,
    /// Branch names in merge order.
    pub order: Vec<String>,
    /// Per-branch step, by branch name.
    pub steps: BTreeMap<String, TrainStep>,
}

/// One branch's step in a train.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TrainStep {
    /// 1-based position.
    pub position: usize,
    /// Branches it conflicts with pairwise.
    pub conflicts_with: Vec<String>,
    /// How the step went.
    pub outcome: StepOutcome,
}

/// Outcome of one train step.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StepOutcome {
    /// Merged cleanly onto the train head.
    Clean,
    /// Conflicted against the train head.
    Blocked {
        /// Branches merged before it.
        blocked_by: Vec<String>,
        /// Unresolved paths.
        paths: Vec<String>,
    },
    /// Nothing to merge.
    Nothing,
}

/// A plan dir found in a worktree.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FoundWork {
    /// Worktree root and plan dir.
    pub key: WorkItemKey,
    /// What its new design files plan.
    pub plan: PlannedWork,
}

impl RepoScan {
    /// Reduces an agentwt document requested with `requested` train bases; `locals`, `work`, `warnings`, the `fresh`
    /// branch names and the `landing` facts come from the lookups that ran beside it. A requested base equal to the
    /// default branch adds no second train: the default train is the same computation.
    #[must_use]
    pub fn from_document(
        document: &Document,
        requested: Vec<String>,
        locals: Vec<String>,
        work: Vec<FoundWork>,
        mut warnings: Vec<String>,
        fresh: &BTreeSet<String>,
        landing: LandingGit,
    ) -> RepoScan {
        let default = Train {
            base: document.repo.default_branch.clone(),
            order: document.order.train.clone(),
            steps: document
                .rows
                .iter()
                .filter_map(|row| Some((row.branch.clone()?, step(row.order.as_ref()?))))
                .collect(),
        };
        let requested_trains = document
            .trains
            .iter()
            .flatten()
            .filter(|train| train.base != document.repo.default_branch)
            .map(|train| Train {
                base: train.base.clone(),
                order: train.order.train.clone(),
                steps: train
                    .branches
                    .iter()
                    .map(|(name, order)| (name.clone(), step(order)))
                    .collect(),
            });
        warnings.splice(0..0, document.warnings.iter().cloned());
        RepoScan {
            default_branch: document.repo.default_branch.clone(),
            locals,
            rows: document
                .rows
                .iter()
                .map(|row| scanned_row(row, fresh))
                .collect(),
            pairs: document
                .order
                .pairs
                .iter()
                .map(|pair| ScannedPair {
                    a: pair.a.clone(),
                    b: pair.b.clone(),
                    paths: pair.paths.clone(),
                })
                .collect(),
            trains: std::iter::once(default).chain(requested_trains).collect(),
            requested,
            warnings,
            work,
            landing,
        }
    }
}

fn scanned_row(row: &Row, fresh: &BTreeSet<String>) -> ScannedRow {
    let worktree = row.worktree.as_ref();
    let path = worktree.map(|wt| PathBuf::from(&wt.path));
    ScannedRow {
        key: match &row.branch {
            Some(name) => BranchKey::Branch(name.clone()),
            None => BranchKey::Detached(path.clone().unwrap_or_default()),
        },
        worktree: path,
        main: worktree.is_some_and(|wt| wt.main),
        tracked_changes: worktree.map_or(0, |wt| wt.dirty),
        untracked: worktree.map_or(0, |wt| wt.untracked),
        own_commits: row.commits.ahead.len(),
        fresh: row.branch.as_ref().is_some_and(|name| fresh.contains(name)),
        from: row.from.name.clone(),
        contained_in: row
            .containment
            .as_ref()
            .map(|containment| containment.branch.clone()),
        safe_to_remove: row.safe_to_remove,
    }
}

fn step(order: &RowOrder) -> TrainStep {
    TrainStep {
        position: order.train_position,
        conflicts_with: order.conflicts_with.clone(),
        outcome: match &order.train {
            TrainOutcome::Clean => StepOutcome::Clean,
            TrainOutcome::Blocked { blocked_by, paths } => StepOutcome::Blocked {
                blocked_by: blocked_by.clone(),
                paths: paths.clone(),
            },
            TrainOutcome::Nothing => StepOutcome::Nothing,
        },
    }
}
