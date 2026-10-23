//! Branch states (concept D16), derived from one scan — first match wins: uncommitted, no-work (still at its creation
//! commit), removable, merged, no-work (no commits, not contained), then the target train's step: blocked ⇒
//! conflicting, clean ⇒ merge-ready, none ⇒ unmerged (D6). Every branch is judged against the train of its landing
//! target when the scan has one, else the default train. The landing target is the pin of a fleet member's branch (its
//! merge branch, P3), else the row's base when the scan lists it as a local branch other than the default one, else
//! none (merge strategy targets D1, D3). Pure.

use std::collections::BTreeMap;

use jiff::Timestamp;

use crate::repo::BranchKey;
use crate::scan::{RepoScan, ScannedRow, StepOutcome, Train, TrainStep};

/// Integration state of one branch.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BranchState {
    /// No own commits, clean worktree.
    NoWork,
    /// Tracked changes or untracked files in the worktree.
    Uncommitted,
    /// Own commits, not contained, no step in the target train.
    Unmerged,
    /// The target train's step is blocked.
    Conflicting,
    /// The target train's step merges cleanly.
    MergeReady,
    /// A non-agent branch holds all of the work.
    Merged,
    /// agentwt's removal predicate holds.
    Removable,
}

impl BranchState {
    /// The branch may still carry work that is not integrated; merged and removable branches never collide.
    #[must_use]
    pub fn is_live(self) -> bool {
        match self {
            BranchState::NoWork
            | BranchState::Uncommitted
            | BranchState::Unmerged
            | BranchState::Conflicting
            | BranchState::MergeReady => true,
            BranchState::Merged | BranchState::Removable => false,
        }
    }
}

/// One branch's derived facts.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Branch {
    /// Current state.
    pub state: BranchState,
    /// When `state` was entered.
    pub since: Timestamp,
    /// The base of the train the state was judged against.
    pub judged: String,
    /// The landing target; `None` for a branch that lands nowhere.
    pub target: Option<String>,
}

/// Every row's facts; `since` carries over from `previous` while the state holds.
#[must_use]
pub fn derive(
    scan: &RepoScan,
    previous: &BTreeMap<BranchKey, Branch>,
    pins: &BTreeMap<String, String>,
    at: Timestamp,
) -> BTreeMap<BranchKey, Branch> {
    scan.rows
        .iter()
        .map(|row| {
            let target = landing_target(row, scan, pins);
            let train = judging_train(target.as_deref(), &scan.trains);
            let step = match (&row.key, train) {
                (BranchKey::Branch(name), Some(train)) => train.steps.get(name),
                (BranchKey::Branch(_), None) | (BranchKey::Detached(_), _) => None,
            };
            let state = state(row, step);
            let since = previous
                .get(&row.key)
                .filter(|old| old.state == state)
                .map_or(at, |old| old.since);
            let judged =
                train.map_or_else(|| scan.default_branch.clone(), |train| train.base.clone());
            (
                row.key.clone(),
                Branch {
                    state,
                    since,
                    judged,
                    target,
                },
            )
        })
        .collect()
}

/// The row's landing target: its pin (a fleet member's merge branch), else its base when the scan lists that base as
/// a local branch other than the default one; `None` for every other row, a detached worktree included.
#[must_use]
pub fn landing_target(
    row: &ScannedRow,
    scan: &RepoScan,
    pins: &BTreeMap<String, String>,
) -> Option<String> {
    let BranchKey::Branch(name) = &row.key else {
        return None;
    };
    pins.get(name).cloned().or_else(|| {
        row.from
            .clone()
            .filter(|base| *base != scan.default_branch && scan.locals.contains(base))
    })
}

/// The train a row is judged against: its landing target's train when the scan has it, else the default train
/// (first).
fn judging_train<'a>(target: Option<&str>, trains: &'a [Train]) -> Option<&'a Train> {
    target
        .and_then(|base| trains.iter().find(|train| train.base == base))
        .or_else(|| trains.first())
}

/// The state of one row given its step in the target train.
#[must_use]
pub fn state(row: &ScannedRow, step: Option<&TrainStep>) -> BranchState {
    if row.tracked_changes > 0 || row.untracked > 0 {
        return BranchState::Uncommitted;
    }
    if row.fresh {
        return BranchState::NoWork;
    }
    if row.contained_in.is_some() {
        return if row.safe_to_remove {
            BranchState::Removable
        } else {
            BranchState::Merged
        };
    }
    if row.own_commits == 0 {
        return BranchState::NoWork;
    }
    match step.map(|step| &step.outcome) {
        Some(StepOutcome::Blocked { .. }) => BranchState::Conflicting,
        Some(StepOutcome::Clean) => BranchState::MergeReady,
        Some(StepOutcome::Nothing) | None => BranchState::Unmerged,
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    fn row(name: &str) -> ScannedRow {
        ScannedRow {
            key: BranchKey::Branch(name.to_owned()),
            worktree: None,
            main: false,
            tracked_changes: 0,
            untracked: 0,
            own_commits: 0,
            fresh: false,
            from: Some("main".to_owned()),
            contained_in: None,
            safe_to_remove: false,
        }
    }

    fn step(outcome: StepOutcome) -> TrainStep {
        TrainStep {
            position: 1,
            conflicts_with: Vec::new(),
            outcome,
        }
    }

    fn train(base: &str, steps: &[(&str, StepOutcome)]) -> Train {
        Train {
            base: base.to_owned(),
            order: steps.iter().map(|(name, _)| (*name).to_owned()).collect(),
            steps: steps
                .iter()
                .map(|(name, outcome)| ((*name).to_owned(), step(outcome.clone())))
                .collect(),
        }
    }

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    #[test]
    #[allow(clippy::too_many_lines)]
    fn state_precedence() {
        let blocked = StepOutcome::Blocked {
            blocked_by: vec!["claude/a".to_owned()],
            paths: vec!["shared.txt".to_owned()],
        };
        let cases: Vec<(&str, ScannedRow, Option<TrainStep>, BranchState)> = vec![
            (
                "tracked-changes",
                ScannedRow {
                    tracked_changes: 1,
                    ..row("x")
                },
                None,
                BranchState::Uncommitted,
            ),
            (
                "untracked-files",
                ScannedRow {
                    untracked: 2,
                    ..row("x")
                },
                None,
                BranchState::Uncommitted,
            ),
            (
                "dirty-and-contained",
                ScannedRow {
                    tracked_changes: 1,
                    contained_in: Some("main".to_owned()),
                    safe_to_remove: true,
                    ..row("x")
                },
                None,
                BranchState::Uncommitted,
            ),
            (
                "fresh-worker",
                ScannedRow {
                    fresh: true,
                    contained_in: Some("main".to_owned()),
                    safe_to_remove: true,
                    ..row("x")
                },
                None,
                BranchState::NoWork,
            ),
            (
                "merged-into-base",
                ScannedRow {
                    contained_in: Some("main".to_owned()),
                    safe_to_remove: true,
                    ..row("x")
                },
                None,
                BranchState::Removable,
            ),
            (
                "contained-unsafe",
                ScannedRow {
                    contained_in: Some("main".to_owned()),
                    ..row("x")
                },
                None,
                BranchState::Merged,
            ),
            (
                "no-commits-not-contained",
                row("x"),
                None,
                BranchState::NoWork,
            ),
            (
                "blocked-step",
                ScannedRow {
                    own_commits: 1,
                    ..row("x")
                },
                Some(step(blocked)),
                BranchState::Conflicting,
            ),
            (
                "clean-step",
                ScannedRow {
                    own_commits: 1,
                    ..row("x")
                },
                Some(step(StepOutcome::Clean)),
                BranchState::MergeReady,
            ),
            (
                "nothing-step",
                ScannedRow {
                    own_commits: 1,
                    ..row("x")
                },
                Some(step(StepOutcome::Nothing)),
                BranchState::Unmerged,
            ),
            (
                "no-step",
                ScannedRow {
                    own_commits: 1,
                    ..row("x")
                },
                None,
                BranchState::Unmerged,
            ),
        ];
        for (label, scanned, step, expected) in cases {
            assert_eq!(state(&scanned, step.as_ref()), expected, "{label}");
        }
    }

    /// A scan of `rows` over `trains`, with `main` default and `feature/x`, `release` local.
    fn scan(rows: Vec<ScannedRow>, trains: Vec<Train>) -> RepoScan {
        RepoScan {
            default_branch: "main".to_owned(),
            locals: vec![
                "main".to_owned(),
                "feature/x".to_owned(),
                "release".to_owned(),
            ],
            rows,
            pairs: Vec::new(),
            trains,
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        }
    }

    fn on(from: Option<&str>) -> ScannedRow {
        ScannedRow {
            from: from.map(str::to_owned),
            ..row("x")
        }
    }

    /// Label, row, pins (branch → merge branch), expected landing target.
    type LandingCase<'a> = (
        &'a str,
        ScannedRow,
        &'a BTreeMap<String, String>,
        Option<&'a str>,
    );

    #[test]
    fn landing_targets() {
        let none = BTreeMap::new();
        let pinned = BTreeMap::from([("x".to_owned(), "claude-orch/T1-run".to_owned())]);
        let detached = ScannedRow {
            key: BranchKey::Detached("/wt/d".into()),
            ..on(Some("feature/x"))
        };
        let cases: [LandingCase; 6] = [
            (
                "pin wins over base",
                on(Some("feature/x")),
                &pinned,
                Some("claude-orch/T1-run"),
            ),
            (
                "base local and not default",
                on(Some("feature/x")),
                &none,
                Some("feature/x"),
            ),
            ("base is the default branch", on(Some("main")), &none, None),
            (
                "base not in locals (remote ref)",
                on(Some("origin/feature/x")),
                &none,
                None,
            ),
            ("no base", on(None), &none, None),
            ("detached row", detached, &none, None),
        ];
        let scanned = scan(Vec::new(), Vec::new());
        for (label, row, pins, expected) in cases {
            let got = landing_target(&row, &scanned, pins);
            assert_eq!(got.as_deref(), expected, "{label}");
        }
    }

    #[test]
    fn judged_train() {
        let trains = vec![train("main", &[]), train("feature/x", &[])];
        let cases: [(&str, ScannedRow, Option<&str>, &str); 3] = [
            (
                "target with a train",
                on(Some("feature/x")),
                Some("feature/x"),
                "feature/x",
            ),
            (
                "target without a train yet",
                on(Some("release")),
                Some("release"),
                "main",
            ),
            ("no target", on(Some("main")), None, "main"),
        ];
        for (label, row, target, judged) in cases {
            let key = row.key.clone();
            let derived = derive(
                &scan(vec![row], trains.clone()),
                &BTreeMap::new(),
                &BTreeMap::new(),
                at(1),
            );
            assert_eq!(derived[&key].target.as_deref(), target, "{label}: target");
            assert_eq!(derived[&key].judged, judged, "{label}: judged");
        }
    }

    #[test]
    fn since_carries() {
        let scan = scan(
            vec![ScannedRow {
                own_commits: 1,
                ..row("claude/a")
            }],
            vec![train("main", &[("claude/a", StepOutcome::Clean)])],
        );
        let key = BranchKey::Branch("claude/a".to_owned());
        let previous_with = |state| {
            BTreeMap::from([(
                key.clone(),
                Branch {
                    state,
                    since: at(1),
                    judged: "main".to_owned(),
                    target: None,
                },
            )])
        };
        let pins = BTreeMap::new();
        let same = derive(&scan, &previous_with(BranchState::MergeReady), &pins, at(5));
        let changed = derive(&scan, &previous_with(BranchState::Unmerged), &pins, at(5));
        assert_eq!(same[&key].since, at(1));
        assert_eq!(changed[&key].since, at(5));
        assert_eq!(same[&key].judged, "main");
    }
}
