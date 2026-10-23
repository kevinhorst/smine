//! Collision detection (concept D12): four levels, each at most once per pair of live branches under a lane prefix
//! (`collision_prefixes`, merge strategy D10), recomputed as a whole from one repository's state (D8). Exempt paths never collide; version-only plan changes are dropped here,
//! version-only edits before they are recorded (D12). Planned levels compare work items pairwise across lanes: a plan
//! dir two lanes both carry is the same work, never a collision. Pure.

use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

use jiff::Timestamp;

use crate::config::{Exemptions, LanePrefixes};
use crate::plan::{PlannedPath, PlannedWork};
use crate::repo::{BranchKey, Repo};

/// Collision level, weakest first.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Level {
    /// Two plans list the same file.
    PlannedFile,
    /// Two plans' diffs share an enclosing signature.
    PlannedSymbol,
    /// Two worktrees' sessions edited the same file.
    Touched,
    /// agentwt's pairwise merge conflicts.
    Confirmed,
}

/// One collision's identity: level plus the ordered branch pair.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct CollisionKey {
    /// Level.
    pub level: Level,
    /// Lower branch.
    pub a: BranchKey,
    /// Higher branch.
    pub b: BranchKey,
}

/// One collision's evidence.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Collision {
    /// Shared paths, or the files of the shared symbols.
    pub paths: Vec<String>,
    /// Shared `path::symbol` entries (planned-symbol only).
    pub symbols: Vec<String>,
    /// Plan dirs involved (planned levels only).
    pub plans: Vec<String>,
    /// When this pair first collided at this level.
    pub since: Timestamp,
}

#[derive(Debug, Default)]
struct Evidence {
    paths: BTreeSet<String>,
    symbols: BTreeSet<String>,
    plans: BTreeSet<String>,
}

/// Live rows: by worktree root, plus every live key (branch-only rows included).
struct Lanes {
    by_root: BTreeMap<PathBuf, BranchKey>,
    live: BTreeSet<BranchKey>,
}

/// One work item of a live lane: the lane, its plan dir, its colliding files and its work.
type PlannedItem<'a> = (
    &'a BranchKey,
    &'a str,
    Vec<&'a PlannedPath>,
    &'a PlannedWork,
);

/// All current collisions of `repo`; `since` carries over from `repo.collisions`.
#[must_use]
pub fn detect(
    repo: &Repo,
    exemptions: &Exemptions,
    prefixes: &LanePrefixes,
    at: Timestamp,
) -> BTreeMap<CollisionKey, Collision> {
    let lanes = live_lanes(repo, prefixes);
    let mut found: BTreeMap<CollisionKey, Evidence> = BTreeMap::new();
    planned(repo, &lanes, exemptions, &mut found);
    touched(repo, &lanes, exemptions, &mut found);
    confirmed(repo, &lanes, exemptions, &mut found);
    found
        .into_iter()
        .map(|(key, evidence)| {
            let since = repo.collisions.get(&key).map_or(at, |old| old.since);
            let collision = Collision {
                paths: evidence.paths.into_iter().collect(),
                symbols: evidence.symbols.into_iter().collect(),
                plans: evidence.plans.into_iter().collect(),
                since,
            };
            (key, collision)
        })
        .collect()
}

/// Live rows on a named branch under a lane prefix; a detached worktree is never a lane.
fn live_lanes(repo: &Repo, prefixes: &LanePrefixes) -> Lanes {
    let mut lanes = Lanes {
        by_root: BTreeMap::new(),
        live: BTreeSet::new(),
    };
    let Some(scan) = repo.snapshot.as_ref() else {
        return lanes;
    };
    for row in &scan.rows {
        let BranchKey::Branch(name) = &row.key else {
            continue;
        };
        if !prefixes.admits(name) {
            continue;
        }
        if !repo
            .branches
            .get(&row.key)
            .is_some_and(|branch| branch.state.is_live())
        {
            continue;
        }
        lanes.live.insert(row.key.clone());
        if let Some(root) = row.worktree.as_ref().filter(|_| !row.main) {
            lanes.by_root.insert(root.clone(), row.key.clone());
        }
    }
    lanes
}

fn key(level: Level, x: &BranchKey, y: &BranchKey) -> CollisionKey {
    let (a, b) = if x <= y {
        (x.clone(), y.clone())
    } else {
        (y.clone(), x.clone())
    };
    CollisionKey { level, a, b }
}

/// Planned-file and planned-symbol between work items of two different live lanes. A plan dir both lanes carry never
/// collides with itself, and the evidence names only the two plan dirs that overlapped.
fn planned(
    repo: &Repo,
    lanes: &Lanes,
    exemptions: &Exemptions,
    found: &mut BTreeMap<CollisionKey, Evidence>,
) {
    let items: Vec<PlannedItem<'_>> = repo
        .work_items
        .iter()
        .filter_map(|(item, work)| {
            let lane = lanes.by_root.get(&item.root)?;
            Some((
                lane,
                item.plan_dir.as_str(),
                colliding_files(work, exemptions),
                work,
            ))
        })
        .collect();
    for (index, (a, plan_a, files_a, work_a)) in items.iter().enumerate() {
        for (b, plan_b, files_b, work_b) in items.iter().skip(index + 1) {
            let is_same_work = a == b || plan_a == plan_b;
            if is_same_work {
                continue;
            }
            let plans = [(*plan_a).to_owned(), (*plan_b).to_owned()];
            let files = shared_files(files_a, files_b);
            if !files.is_empty() {
                let evidence = found.entry(key(Level::PlannedFile, a, b)).or_default();
                evidence.paths.extend(files);
                evidence.plans.extend(plans.iter().cloned());
            }
            let symbols: Vec<&(String, String)> = work_a
                .symbols
                .intersection(&work_b.symbols)
                .filter(|(file, _)| !exemptions.path(file))
                .collect();
            if !symbols.is_empty() {
                let evidence = found.entry(key(Level::PlannedSymbol, a, b)).or_default();
                evidence
                    .paths
                    .extend(symbols.iter().map(|(file, _)| file.clone()));
                evidence
                    .symbols
                    .extend(symbols.iter().map(|(file, name)| format!("{file}::{name}")));
                evidence.plans.extend(plans);
            }
        }
    }
}

/// A plan's files minus exempt paths and files whose every planned change is a version line.
fn colliding_files<'a>(work: &'a PlannedWork, exemptions: &Exemptions) -> Vec<&'a PlannedPath> {
    work.files
        .iter()
        .filter(|file| !exemptions.path(&file.path))
        .filter(|file| {
            let lines: Vec<&str> = work
                .changed
                .get(&file.path)
                .map(|lines| lines.iter().map(String::as_str).collect())
                .unwrap_or_default();
            !exemptions.version_only(&lines)
        })
        .collect()
}

/// Paths both plans name: equal literals, equal globs, or a glob matching the other side's literal.
fn shared_files(a: &[&PlannedPath], b: &[&PlannedPath]) -> BTreeSet<String> {
    let mut shared = BTreeSet::new();
    for x in a {
        for y in b {
            let hit = match (x.glob, y.glob) {
                (false, false) | (true, true) => x.path == y.path,
                (true, false) => glob_matches(&x.path, &y.path),
                (false, true) => glob_matches(&y.path, &x.path),
            };
            if hit {
                shared.insert(if x.glob {
                    y.path.clone()
                } else {
                    x.path.clone()
                });
            }
        }
    }
    shared
}

fn glob_matches(pattern: &str, path: &str) -> bool {
    globset::GlobBuilder::new(pattern)
        .literal_separator(true)
        .build()
        .is_ok_and(|glob| glob.compile_matcher().is_match(path))
}

/// Touched: live worktrees whose sessions edited the same non-exempt path.
fn touched(
    repo: &Repo,
    lanes: &Lanes,
    exemptions: &Exemptions,
    found: &mut BTreeMap<CollisionKey, Evidence>,
) {
    for (path, roots) in &repo.touched {
        if exemptions.path(path) {
            continue;
        }
        let keys: Vec<&BranchKey> = roots
            .iter()
            .filter_map(|root| lanes.by_root.get(root))
            .collect::<BTreeSet<_>>()
            .into_iter()
            .collect();
        for (index, a) in keys.iter().enumerate() {
            for b in keys.iter().skip(index + 1) {
                found
                    .entry(key(Level::Touched, a, b))
                    .or_default()
                    .paths
                    .insert(path.clone());
            }
        }
    }
}

/// Confirmed: agentwt's pairwise merge conflicts between live branches, exempt paths dropped.
fn confirmed(
    repo: &Repo,
    lanes: &Lanes,
    exemptions: &Exemptions,
    found: &mut BTreeMap<CollisionKey, Evidence>,
) {
    let Some(scan) = repo.snapshot.as_ref() else {
        return;
    };
    for pair in &scan.pairs {
        let (a, b) = (
            BranchKey::Branch(pair.a.clone()),
            BranchKey::Branch(pair.b.clone()),
        );
        if !lanes.live.contains(&a) || !lanes.live.contains(&b) {
            continue;
        }
        let paths: Vec<String> = pair
            .paths
            .iter()
            .filter(|path| !exemptions.path(path))
            .cloned()
            .collect();
        if !paths.is_empty() {
            found
                .entry(key(Level::Confirmed, &a, &b))
                .or_default()
                .paths
                .extend(paths);
        }
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::branch::{Branch, BranchState};
    use crate::plan::PlannedPath;
    use crate::repo::WorkItemKey;
    use crate::scan::{RepoScan, ScannedPair, ScannedRow};

    const POOL: &str = "/r/.claude/worktrees";

    fn at(second: i64) -> Timestamp {
        Timestamp::from_second(second).expect("timestamp")
    }

    fn root(name: &str) -> PathBuf {
        Path::new(POOL).join(name)
    }

    fn lane(name: &str) -> BranchKey {
        BranchKey::Branch(format!("claude/{name}"))
    }

    fn scanned(name: &str) -> ScannedRow {
        ScannedRow {
            key: lane(name),
            worktree: Some(root(name)),
            main: false,
            tracked_changes: 0,
            untracked: 0,
            own_commits: 1,
            fresh: false,
            from: Some("main".to_owned()),
            contained_in: None,
            safe_to_remove: false,
        }
    }

    fn literal(path: &str) -> PlannedPath {
        PlannedPath {
            path: path.to_owned(),
            glob: path.contains('*'),
        }
    }

    fn work(files: &[&str], symbols: &[(&str, &str)], changed: &[(&str, &[&str])]) -> PlannedWork {
        PlannedWork {
            files: files.iter().map(|path| literal(path)).collect(),
            symbols: symbols
                .iter()
                .map(|(file, name)| ((*file).to_owned(), (*name).to_owned()))
                .collect(),
            changed: changed
                .iter()
                .map(|(file, lines)| {
                    let lines = lines.iter().map(|line| (*line).to_owned()).collect();
                    ((*file).to_owned(), lines)
                })
                .collect(),
        }
    }

    /// Lanes `a`, `b` live and `m` removable, each a worktree under the pool.
    fn repo(
        items: Vec<(&str, PlannedWork)>,
        touched: &[(&str, &[&str])],
        pairs: &[(&str, &str, &[&str])],
    ) -> Repo {
        let mut repo = Repo::new("fixture".to_owned());
        let names = ["a", "b", "m"];
        repo.snapshot = Some(RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows: names.iter().map(|name| scanned(name)).collect(),
            pairs: pairs
                .iter()
                .map(|(a, b, paths)| ScannedPair {
                    a: format!("claude/{a}"),
                    b: format!("claude/{b}"),
                    paths: paths.iter().map(|path| (*path).to_owned()).collect(),
                })
                .collect(),
            trains: Vec::new(),
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        });
        for name in names {
            let state = if name == "m" {
                BranchState::Removable
            } else {
                BranchState::MergeReady
            };
            let branch = Branch {
                state,
                since: at(0),
                judged: "main".to_owned(),
                target: None,
            };
            repo.branches.insert(lane(name), branch);
        }
        for (name, planned) in items {
            let item = WorkItemKey {
                root: root(name),
                plan_dir: format!("2026-09-27-{name}-wip-aaaaaaa"),
            };
            repo.work_items.insert(item, planned);
        }
        for (path, names) in touched {
            let roots = names.iter().map(|name| root(name)).collect();
            repo.touched.insert((*path).to_owned(), roots);
        }
        repo
    }

    fn exemptions() -> Exemptions {
        Exemptions::new(&["plans/**".to_owned()], vec!["version".to_owned()])
            .expect("patterns compile")
    }

    fn shared_work() -> PlannedWork {
        work(
            &[
                "src/shared.rs",
                "plans/p/design/raw.md",
                "skills/x/SKILL.md",
            ],
            &[("src/shared.rs", "parse"), ("plans/p/design/raw.md", "x")],
            &[("skills/x/SKILL.md", &["version: 1.0.0", "version: 1.1.0"])],
        )
    }

    #[test]
    fn levels() {
        let repo = repo(
            vec![
                ("a", shared_work()),
                ("b", shared_work()),
                ("m", shared_work()),
            ],
            &[
                ("src/shared.rs", &["a", "b", "m"]),
                ("plans/p/design/raw.md", &["a", "b"]),
            ],
            &[
                ("a", "b", &["src/shared.rs", "plans/p/design/raw.md"]),
                ("a", "m", &["src/shared.rs"]),
            ],
        );
        let found = detect(&repo, &exemptions(), &LanePrefixes::default(), at(9));
        let levels: Vec<Level> = found.keys().map(|key| key.level).collect();
        assert_eq!(
            levels,
            [
                Level::PlannedFile,
                Level::PlannedSymbol,
                Level::Touched,
                Level::Confirmed
            ],
            "each level once per pair, the removable lane excluded"
        );
        for (key, collision) in &found {
            assert_eq!((&key.a, &key.b), (&lane("a"), &lane("b")), "{key:?}");
            assert_eq!(
                collision.paths,
                ["src/shared.rs"],
                "exempt and version-only paths dropped: {key:?}"
            );
            assert_eq!(collision.since, at(9));
        }
        let symbol = &found[&key(Level::PlannedSymbol, &lane("a"), &lane("b"))];
        assert_eq!(symbol.symbols, ["src/shared.rs::parse"]);
        assert_eq!(
            symbol.plans,
            ["2026-09-27-a-wip-aaaaaaa", "2026-09-27-b-wip-aaaaaaa"]
        );
    }

    #[test]
    fn glob_matches_literal() {
        let repo = repo(
            vec![
                ("a", work(&["src/*.rs"], &[], &[])),
                ("b", work(&["src/other.rs", "docs/x.md"], &[], &[])),
            ],
            &[],
            &[],
        );
        let found = detect(&repo, &exemptions(), &LanePrefixes::default(), at(1));
        let file = &found[&key(Level::PlannedFile, &lane("a"), &lane("b"))];
        assert_eq!(file.paths, ["src/other.rs"]);
    }

    #[test]
    fn planned_pairs() {
        let item = |name: &str, plan: &str| WorkItemKey {
            root: root(name),
            plan_dir: plan.to_owned(),
        };
        let shared = || work(&["src/shared.rs"], &[("src/shared.rs", "parse")], &[]);
        let mut two_lanes = repo(Vec::new(), &[], &[]);
        two_lanes
            .work_items
            .insert(item("a", "merged-plan"), shared());
        two_lanes
            .work_items
            .insert(item("b", "merged-plan"), shared());
        assert!(
            detect(&two_lanes, &exemptions(), &LanePrefixes::default(), at(1)).is_empty(),
            "the same plan dir on two lanes is the same work"
        );

        two_lanes.work_items.insert(item("b", "own-plan"), shared());
        let found = detect(&two_lanes, &exemptions(), &LanePrefixes::default(), at(2));
        let file = &found[&key(Level::PlannedFile, &lane("a"), &lane("b"))];
        assert_eq!(
            file.plans,
            ["merged-plan", "own-plan"],
            "evidence names the two overlapping dirs only"
        );

        let mut one_lane = repo(Vec::new(), &[], &[]);
        one_lane.work_items.insert(item("a", "first"), shared());
        one_lane.work_items.insert(item("a", "second"), shared());
        assert!(
            detect(&one_lane, &exemptions(), &LanePrefixes::default(), at(3)).is_empty(),
            "one lane never collides with itself"
        );
    }

    #[test]
    fn since_kept_across_recomputes() {
        let mut repo = repo(Vec::new(), &[("src/shared.rs", &["a", "b"])], &[]);
        let first = detect(&repo, &exemptions(), &LanePrefixes::default(), at(3));
        repo.collisions = first;
        let again = detect(&repo, &exemptions(), &LanePrefixes::default(), at(8));
        let touched = &again[&key(Level::Touched, &lane("a"), &lane("b"))];
        assert_eq!(touched.since, at(3));
    }

    /// A session lane `claude/a`, a routine row, a merge-branch row and a detached row, all live, all planning,
    /// touching and conflicting on the same file with the session lane.
    fn mixed() -> Repo {
        let rows = [
            (BranchKey::Branch("claude/a".to_owned()), root("a")),
            (BranchKey::Branch("claude-routines/r".to_owned()), root("r")),
            (BranchKey::Branch("claude-orch/T1-x".to_owned()), root("o")),
            (BranchKey::Detached(root("d")), root("d")),
        ];
        let mut repo = Repo::new("fixture".to_owned());
        repo.snapshot = Some(RepoScan {
            default_branch: "main".to_owned(),
            locals: vec!["main".to_owned()],
            rows: rows
                .iter()
                .map(|(key, worktree)| ScannedRow {
                    key: key.clone(),
                    worktree: Some(worktree.clone()),
                    ..scanned("a")
                })
                .collect(),
            pairs: ["claude-routines/r", "claude-orch/T1-x"]
                .iter()
                .map(|other| ScannedPair {
                    a: "claude/a".to_owned(),
                    b: (*other).to_owned(),
                    paths: vec!["src/shared.rs".to_owned()],
                })
                .collect(),
            trains: Vec::new(),
            requested: Vec::new(),
            warnings: Vec::new(),
            work: Vec::new(),
            landing: crate::scan::LandingGit::default(),
        });
        for (key, worktree) in rows {
            let branch = Branch {
                state: BranchState::MergeReady,
                since: at(0),
                judged: "main".to_owned(),
                target: None,
            };
            repo.branches.insert(key, branch);
            let item = WorkItemKey {
                root: worktree.clone(),
                plan_dir: format!("2026-09-27-{}-wip-aaaaaaa", worktree.display()),
            };
            repo.work_items.insert(item, shared_work());
            repo.touched
                .entry("src/shared.rs".to_owned())
                .or_default()
                .insert(worktree);
        }
        repo
    }

    /// Label, prefixes, expected (level, other branch) of every collision with the session lane.
    type PrefixCase = (&'static str, LanePrefixes, Vec<(Level, BranchKey)>);

    #[test]
    fn lane_prefixes() {
        let merge = BranchKey::Branch("claude-orch/T1-x".to_owned());
        let all_levels = [
            Level::PlannedFile,
            Level::PlannedSymbol,
            Level::Touched,
            Level::Confirmed,
        ];
        let cases: [PrefixCase; 3] = [
            ("default", LanePrefixes::default(), Vec::new()),
            (
                "list with the merge prefix",
                LanePrefixes {
                    prefixes: Some(vec!["claude/".to_owned(), "claude-orch/".to_owned()]),
                },
                all_levels
                    .iter()
                    .map(|level| (*level, merge.clone()))
                    .collect(),
            ),
            (
                "empty list",
                LanePrefixes {
                    prefixes: Some(Vec::new()),
                },
                Vec::new(),
            ),
        ];
        let repo = mixed();
        for (label, prefixes, want) in cases {
            let found = detect(&repo, &exemptions(), &prefixes, at(1));
            let got: Vec<(Level, BranchKey)> = found
                .keys()
                .map(|key| {
                    let other = if key.a == lane("a") {
                        key.b.clone()
                    } else {
                        key.a.clone()
                    };
                    (key.level, other)
                })
                .collect();
            assert_eq!(got, want, "{label}");
        }
    }
}
