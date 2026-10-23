//! Assembly of the status document: inventory → gitstate → sessions → order → wire.
//!
//! Invariant: every inventory row (agent branch or detached worktree) appears here; an unborn
//! HEAD never enters the inventory, and a row whose analysis fails fails the whole document —
//! nothing is dropped silently.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use gix::ObjectId;

use crate::error::{Error, GitExt, Result};
use crate::gitstate::{self, BranchAnalysis, Context};
use crate::inventory::{self, Inventory, Item, RowKey};
use crate::order;
use crate::patchid;
use crate::sessions;
use crate::wire::{
    AheadBehind, BaseTrain, Checkout, CommitEntry, Commits, Counts, Document, From, Head,
    OrderSummary, RepoInfo, Row, SCHEMA, Timings, Worktree,
};

/// Inputs of one status run.
#[derive(Debug, Clone)]
pub struct Options {
    /// Any worktree of the repository; resolved to the main checkout.
    pub repo: PathBuf,
    /// Claude homes to read session registries from.
    pub claude_homes: Vec<PathBuf>,
    /// Codex home, if any.
    pub codex_home: Option<PathBuf>,
    /// Skip the order module (cheaper runs for the remove planner).
    pub skip_order: bool,
    /// Only these branches' rows (the remove planner reads nothing else); `None` for every row.
    pub branches: Option<Vec<String>>,
    /// Extra train seeds: one [`BaseTrain`] per local non-agent branch, in this order; empty for the default train only.
    pub train_bases: Vec<String>,
    /// Explicitly tracked extra candidates (`--candidate`): local branches added to the candidate set even when
    /// agent-owned; empty keeps the default set.
    pub candidates: Vec<String>,
}

/// Builds the document over every row — [`build_kind`] without a kind; heartbeat and `remove` call this.
pub fn build(options: &Options) -> Result<Document> {
    build_kind(options, None)
}

/// Builds the document, measuring every phase (`Timings`) so load behaviour is visible without a profiler. A `kind`
/// restricts the analysed rows to that kind's ([`key_kind`]); `Document::kinds` still lists every kind of the inventory.
pub fn build_kind(options: &Options, kind: Option<&str>) -> Result<Document> {
    let started = std::time::Instant::now();
    let mut timings = Timings::default();
    let repo_path = inventory::main_checkout(&options.repo)?;
    let mut warnings = Vec::new();

    let phase = std::time::Instant::now();
    let inventory = Inventory::collect(&repo_path, &candidate_names(&repo_path)?)?;
    timings.inventory_ms = elapsed_ms(phase);
    let default_branch = inventory
        .default_branch
        .clone()
        .ok_or_else(|| Error::Invariant("no default branch could be determined".to_owned()))?;
    let mut context = Context::open(&repo_path, &default_branch, &options.candidates)?;
    warn_missing_candidates(&context, &options.candidates, &mut warnings);
    let phase = std::time::Instant::now();
    let checkout = checkout_state(&repo_path)?;
    timings.checkout_ms = elapsed_ms(phase);

    let codex_home = options.codex_home.as_deref();
    let kinds = inventory_kinds(&inventory.items, codex_home);
    let items = selected(
        &inventory.items,
        options.branches.as_deref(),
        kind,
        codex_home,
    );
    let worktree_paths: Vec<PathBuf> = items
        .iter()
        .filter_map(|item| item.worktree.as_ref().map(|worktree| worktree.path.clone()))
        .collect();
    let phase = std::time::Instant::now();
    let roster = sessions::gather(
        &options.claude_homes,
        options.codex_home.as_deref(),
        &mut warnings,
    );
    let mut attached = sessions::attach(roster, &worktree_paths);
    timings.sessions_ms = elapsed_ms(phase);

    let phase = std::time::Instant::now();
    let (row_analyses, worktrees) = std::thread::scope(|scope| {
        let workers: Vec<_> = items
            .iter()
            .map(|item| scope.spawn(move || timed_worktree_row(item)))
            .collect();
        let analysis_started = std::time::Instant::now();
        let analyses: Result<Vec<_>> = items
            .iter()
            .map(|item| analyze_item(&mut context, item))
            .collect();
        timings.analysis_ms = elapsed_ms(analysis_started);
        let worktrees: Vec<_> = workers.into_iter().map(join_worker).collect();
        (analyses, worktrees)
    });
    timings.worktrees_ms = worktrees.iter().map(|(_, ms)| *ms).max().unwrap_or(0);
    let mut rows = Vec::with_capacity(items.len());
    let mut analyzed_rows = Vec::with_capacity(items.len());
    for ((item, analysis), (worktree, _)) in items.iter().zip(row_analyses?).zip(worktrees) {
        let Some((key, tip, analysis)) = analysis else {
            continue;
        };
        let worktree = retry_failed(item, worktree)?;
        let session = worktree
            .as_ref()
            .and_then(|wt| attached.remove(Path::new(&wt.path)));
        rows.push(row(item, &key, &analysis, worktree, session, codex_home));
        analyzed_rows.push((key, tip, analysis));
    }
    timings.gitstate_ms = elapsed_ms(phase);

    let phase = std::time::Instant::now();
    let (order, trains) = order_phase(
        options,
        &context,
        &default_branch,
        &mut rows,
        &analyzed_rows,
        &mut warnings,
    )?;
    timings.order_ms = elapsed_ms(phase);
    timings.total_ms = elapsed_ms(started);

    Ok(Document {
        schema: SCHEMA,
        repo: RepoInfo {
            path: path_string(&repo_path)?,
            default_branch,
        },
        checkout,
        candidates: context
            .candidates()
            .iter()
            .map(|candidate| candidate.name.clone())
            .collect(),
        rows,
        kinds,
        order,
        trains,
        warnings,
        timings,
    })
}

/// One warning per requested `--candidate` that is no local branch; the document stays complete.
fn warn_missing_candidates(context: &Context, requested: &[String], warnings: &mut Vec<String>) {
    for name in requested {
        let is_local = context
            .candidates()
            .iter()
            .any(|candidate| candidate.name == *name);
        if !is_local {
            warnings.push(format!("candidate {name} is not a local branch"));
        }
    }
}

fn elapsed_ms(since: std::time::Instant) -> u64 {
    u64::try_from(since.elapsed().as_millis()).unwrap_or(u64::MAX)
}

/// Inventory items in order, restricted to `branches` when given (detached rows then drop out) and to one `kind` when
/// given (an item without that kind then drops out).
fn selected<'a>(
    items: &'a [Item],
    branches: Option<&[String]>,
    kind: Option<&str>,
    codex_home: Option<&Path>,
) -> Vec<&'a Item> {
    items
        .iter()
        .filter(|item| {
            branches.is_none_or(|wanted| {
                item.branch
                    .as_ref()
                    .is_some_and(|branch| wanted.contains(branch))
            })
        })
        .filter(|item| {
            kind.is_none_or(|wanted| item_kind(item, codex_home).as_deref() == Some(wanted))
        })
        .collect()
}

/// Kind of an inventory item ([`key_kind`] of its row key); `None` for an item without a key.
fn item_kind(item: &Item, codex_home: Option<&Path>) -> Option<String> {
    item.key().and_then(|key| key_kind(&key, codex_home))
}

/// Every kind the inventory holds, sorted and distinct — the kind filters a reader can offer.
fn inventory_kinds(items: &[Item], codex_home: Option<&Path>) -> Vec<String> {
    let kinds: BTreeSet<String> = items
        .iter()
        .filter_map(|item| item_kind(item, codex_home))
        .collect();
    kinds.into_iter().collect()
}

/// A row's kind: the agent namespace of a branch (`claude`, `claude-routines`, …), `codex` for a detached worktree in
/// the Codex pool, `None` otherwise.
fn key_kind(key: &RowKey, codex_home: Option<&Path>) -> Option<String> {
    match key {
        RowKey::Branch(branch) => inventory::agent_kind(branch).map(str::to_owned),
        // Both sides canonical: the inventory reports resolved paths, the caller's home may be a symlinked spelling.
        RowKey::Detached(path) => codex_home
            .is_some_and(|home| {
                let pool = home.join("worktrees");
                let pool = dunce::canonicalize(&pool).unwrap_or(pool);
                dunce::canonicalize(path)
                    .unwrap_or_else(|_| path.clone())
                    .starts_with(pool)
            })
            .then(|| "codex".to_owned()),
    }
}

/// Row key, tip and history analysis of one item; `None` for an item without a key.
fn analyze_item(
    context: &mut Context,
    item: &Item,
) -> Result<Option<(RowKey, ObjectId, BranchAnalysis)>> {
    let Some(key) = item.key() else {
        return Ok(None);
    };
    let tip = ObjectId::from_hex(item.head.sha.as_bytes())
        .map_err(|err| Error::Invariant(format!("tip sha {}: {err}", item.head.sha)))?;
    let analysis = context.analyze(item.branch.as_deref(), tip)?;
    Ok(Some((key, tip, analysis)))
}

/// Worktree state of one item (`None` without a checkout) and the milliseconds it took.
fn timed_worktree_row(item: &Item) -> (Result<Option<Worktree>>, u64) {
    let started = std::time::Instant::now();
    let worktree = item.worktree.as_ref().map(worktree_row).transpose();
    (worktree, elapsed_ms(started))
}

/// A worktree status that failed on its parallel thread, run once more alone: a transient open failure (a descriptor
/// limit hit while every worktree walks at once) must not fail the whole document. A second failure is returned.
fn retry_failed(item: &Item, first: Result<Option<Worktree>>) -> Result<Option<Worktree>> {
    match first {
        Ok(worktree) => Ok(worktree),
        Err(_) => item.worktree.as_ref().map(worktree_row).transpose(),
    }
}

/// A worker's result; a panicked worker is an invariant error, never a silently missing row.
fn join_worker(
    worker: std::thread::ScopedJoinHandle<'_, (Result<Option<Worktree>>, u64)>,
) -> (Result<Option<Worktree>>, u64) {
    worker.join().unwrap_or_else(|_| {
        (
            Err(Error::Invariant(
                "worktree status thread panicked".to_owned(),
            )),
            0,
        )
    })
}

/// Local branches that are candidates — read once with a throwaway handle before the inventory runs.
fn candidate_names(repo_path: &Path) -> Result<Vec<String>> {
    let repo = gix::open(repo_path).git()?;
    let mut names = Vec::new();
    for reference in repo.references().git()?.local_branches().git()? {
        let reference =
            reference.map_err(|err| Error::Invariant(format!("reference iteration: {err}")))?;
        let name = reference.name().shorten().to_string();
        if inventory::is_candidate(&name) {
            names.push(name);
        }
    }
    Ok(names)
}

fn checkout_state(repo_path: &Path) -> Result<Checkout> {
    let repo = gix::open(repo_path).git()?;
    let branch = repo
        .head()
        .git()?
        .referent_name()
        .map_or_else(|| "HEAD".to_owned(), |name| name.shorten().to_string());
    let state = gitstate::tree_state(repo_path)?;
    Ok(Checkout {
        branch,
        dirty: state.dirty,
    })
}

fn worktree_row(field: &inventory::WorktreeField) -> Result<Worktree> {
    // The checkout can vanish between the inventory and this read (`git worktree remove` under a running scan).
    let prunable = field.prunable || !field.path.join(".git").exists();
    let state = if prunable {
        gitstate::TreeState::default()
    } else {
        gitstate::tree_state(&field.path)?
    };
    let untracked_paths: Vec<String> = state.untracked.iter().take(20).cloned().collect();
    Ok(Worktree {
        path: path_string(&field.path)?,
        main: field.main,
        detached: field.detached,
        locked: field.locked.clone(),
        prunable,
        operation: field.operation.map(|op| match op {
            inventory::OperationField::Merge => crate::wire::Operation::Merge,
            inventory::OperationField::Rebase => crate::wire::Operation::Rebase,
            inventory::OperationField::CherryPick => crate::wire::Operation::CherryPick,
            inventory::OperationField::Revert => crate::wire::Operation::Revert,
            inventory::OperationField::Bisect => crate::wire::Operation::Bisect,
        }),
        dirty: state.dirty,
        untracked: state.untracked.len(),
        untracked_paths,
    })
}

fn row(
    item: &Item,
    key: &RowKey,
    analysis: &BranchAnalysis,
    worktree: Option<Worktree>,
    session: Option<crate::wire::Session>,
    codex_home: Option<&Path>,
) -> Row {
    let kind = key_kind(key, codex_home);
    let head = Head {
        sha: item.head.sha.clone(),
        short_sha: item.head.short_sha.clone(),
        subject: item.head.subject.clone(),
        committed_at: item.head.committed_at.clone(),
    };
    let containment = analysis.containment.clone();
    let unpicked_commits: Vec<String> = analysis.unpicked.iter().map(ToString::to_string).collect();
    let unpicked = unpicked_commits.len();
    let (safe_to_remove, unsafe_reasons) =
        safety(worktree.as_ref(), containment.is_some(), unpicked);

    Row {
        branch: item.branch.clone(),
        kind,
        head,
        worktree,
        from: From {
            name: analysis.base.name.clone(),
            resolution: analysis.base.resolution,
            origin: analysis.base.origin.clone(),
        },
        counts: Counts {
            vs_from: analysis.base.tip.map(|_| AheadBehind {
                ahead: analysis.ahead.len(),
                behind: analysis.behind_total,
            }),
        },
        commits: Commits {
            ahead: analysis.ahead.iter().map(commit_entry).collect(),
            behind: analysis.behind.iter().map(commit_entry).collect(),
            behind_truncated: analysis.behind_truncated,
        },
        unpicked,
        unpicked_commits,
        containment,
        safe_to_remove,
        unsafe_reasons,
        session,
        order: None,
    }
}

/// The removal predicate and its reasons, in the order the Go code lists them today.
#[must_use]
pub fn safety(
    worktree: Option<&Worktree>,
    contained: bool,
    unpicked: usize,
) -> (bool, Vec<String>) {
    let safe = match worktree {
        None => contained,
        Some(wt) => wt.dirty == 0 && wt.untracked == 0 && wt.operation.is_none() && contained,
    };
    if safe {
        return (true, Vec::new());
    }
    let mut reasons = Vec::new();
    if let Some(wt) = worktree {
        if wt.dirty > 0 {
            reasons.push(format!("dirty({})", wt.dirty));
        }
        if wt.untracked > 0 {
            reasons.push(format!("untracked({})", wt.untracked));
        }
        if let Some(operation) = wt.operation {
            reasons.push(format!("operation({})", operation.as_str()));
        }
    }
    if !contained {
        reasons.push(if unpicked > 0 {
            format!("unpicked({unpicked})")
        } else {
            "work contained nowhere".to_owned()
        });
        if worktree.is_none() {
            reasons.push("no worktree".to_owned());
        }
    }
    (false, reasons)
}

/// One commit as its wire line.
pub(crate) fn commit_entry(commit: &gitstate::CommitInfo) -> CommitEntry {
    CommitEntry {
        sha: commit.id.to_string(),
        short_sha: commit.id.to_hex_with_len(8).to_string(),
        subject: commit.subject.clone(),
        committed_at: commit.committed_at.clone(),
        is_merge: commit.is_merge(),
    }
}

/// Runs the order module over rows with own commits and writes the per-row facts back; then one train per requested
/// base over the same inputs, seeded with that base's tip (exploration D3).
fn apply_order(
    context: &Context,
    default_branch: &str,
    train_bases: &[String],
    rows: &mut [Row],
    analyzed_rows: &[(RowKey, ObjectId, BranchAnalysis)],
    warnings: &mut Vec<String>,
) -> Result<(OrderSummary, Option<Vec<BaseTrain>>)> {
    let default_tip = candidate_tip(context, default_branch).ok_or_else(|| {
        Error::Invariant(format!(
            "default branch {default_branch} is not a local branch"
        ))
    })?;

    let mut inputs = Vec::new();
    for (row, (key, tip, analysis)) in rows.iter().zip(analyzed_rows) {
        let RowKey::Branch(name) = key else { continue };
        let own: Vec<&gitstate::CommitInfo> = analysis
            .ahead
            .iter()
            .filter(|commit| !commit.is_merge())
            .collect();
        // Work a non-agent branch already holds has nothing to merge here; moving it onward is that branch's business.
        if own.is_empty() || analysis.containment.is_some() {
            continue;
        }
        let mut touched = BTreeSet::new();
        for commit in own {
            touched.extend(patchid::touched_paths(
                context.repo(),
                commit.parents[0],
                commit.id,
            )?);
        }
        let snapshot = match row.worktree.as_ref().filter(|wt| wt.dirty > 0) {
            Some(wt) => gitstate::snapshot_commit(context, Path::new(&wt.path))?,
            None => None,
        };
        inputs.push(order::Input {
            name: name.clone(),
            merge_head: snapshot.unwrap_or(*tip),
            snapshot: snapshot.is_some(),
            touched,
        });
    }
    let computed = order::compute(context, default_tip, &inputs)?;
    for row in rows.iter_mut() {
        if let Some(name) = &row.branch {
            row.order = computed.rows.get(name).cloned();
        }
    }
    let trains = requested_trains(context, train_bases, &inputs, warnings)?;
    Ok((computed.summary, trains))
}

/// The order phase: the default train plus the requested ones, or empty when the run skips ordering.
fn order_phase(
    options: &Options,
    context: &Context,
    default_branch: &str,
    rows: &mut [Row],
    analyzed_rows: &[(RowKey, ObjectId, BranchAnalysis)],
    warnings: &mut Vec<String>,
) -> Result<(OrderSummary, Option<Vec<BaseTrain>>)> {
    if options.skip_order {
        let empty = OrderSummary {
            train: Vec::new(),
            pairs: Vec::new(),
            components: Vec::new(),
            dirty_snapshots: Vec::new(),
        };
        return Ok((empty, None));
    }
    apply_order(
        context,
        default_branch,
        &options.train_bases,
        rows,
        analyzed_rows,
        warnings,
    )
}

/// The requested bases' trains; `None` when none was requested. A base that is not a candidate (a local non-agent
/// branch) gets no train and one warning — the document stays complete. Duplicates are dropped.
fn requested_trains(
    context: &Context,
    train_bases: &[String],
    inputs: &[order::Input],
    warnings: &mut Vec<String>,
) -> Result<Option<Vec<BaseTrain>>> {
    if train_bases.is_empty() {
        return Ok(None);
    }
    let mut seen = BTreeSet::new();
    let mut trains = Vec::new();
    for base in train_bases {
        if !seen.insert(base.as_str()) {
            continue;
        }
        let Some(tip) = candidate_tip(context, base) else {
            warnings.push(format!("train base {base} is not a local non-agent branch"));
            continue;
        };
        let computed = order::compute(context, tip, inputs)?;
        trains.push(BaseTrain {
            base: base.clone(),
            order: computed.summary,
            branches: computed.rows,
        });
    }
    Ok(Some(trains))
}

/// The tip of the candidate named `name`.
fn candidate_tip(context: &Context, name: &str) -> Option<ObjectId> {
    context
        .candidates()
        .iter()
        .find(|candidate| candidate.name == name)
        .map(|candidate| candidate.tip)
}

fn path_string(path: &Path) -> Result<String> {
    path.to_str()
        .map(str::to_owned)
        .ok_or_else(|| Error::NonUtf8Path(path.to_owned()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn worktree(dirty: usize, untracked: usize) -> Worktree {
        Worktree {
            path: "/w".to_owned(),
            main: false,
            detached: false,
            locked: None,
            prunable: false,
            operation: None,
            dirty,
            untracked,
            untracked_paths: Vec::new(),
        }
    }

    fn item(branch: Option<&str>) -> Item {
        Item {
            branch: branch.map(str::to_owned),
            head: inventory::HeadField {
                sha: String::new(),
                short_sha: String::new(),
                subject: String::new(),
                committed_at: String::new(),
            },
            worktree: None,
        }
    }

    #[test]
    fn selected_rows() {
        // (label, branch filter, kind, expected branches in order)
        type Case<'a> = (
            &'a str,
            Option<&'a [String]>,
            Option<&'a str>,
            Vec<Option<&'a str>>,
        );
        let items = [
            item(Some("claude/a")),
            item(None),
            item(Some("claude/b")),
            item(Some("claude-routines/c")),
        ];
        let only_b = ["claude/b".to_owned()];
        let cases: [Case; 7] = [
            (
                "all, no filters",
                None,
                None,
                vec![
                    Some("claude/a"),
                    None,
                    Some("claude/b"),
                    Some("claude-routines/c"),
                ],
            ),
            (
                "named only, detached dropped",
                Some(&only_b),
                None,
                vec![Some("claude/b")],
            ),
            ("empty branch filter", Some(&[]), None, vec![]),
            (
                "kind claude: detached and other kinds dropped",
                None,
                Some("claude"),
                vec![Some("claude/a"), Some("claude/b")],
            ),
            (
                "kind claude-routines",
                None,
                Some("claude-routines"),
                vec![Some("claude-routines/c")],
            ),
            (
                "kind absent from the inventory",
                None,
                Some("claude-review"),
                vec![],
            ),
            (
                "branch filter and kind combine",
                Some(&only_b),
                Some("claude-routines"),
                vec![],
            ),
        ];
        for (label, branches, kind, expected) in cases {
            let branches: Vec<Option<&str>> = selected(&items, branches, kind, None)
                .iter()
                .map(|item| item.branch.as_deref())
                .collect();
            assert_eq!(branches, expected, "{label}");
        }
    }

    #[test]
    fn inventory_kinds_sorted_distinct() {
        let codex_home = Path::new("/nonexistent/codex");
        let detached = |path: &str| Item {
            worktree: Some(inventory::WorktreeField {
                path: PathBuf::from(path),
                main: false,
                detached: true,
                locked: None,
                prunable: false,
                operation: None,
            }),
            ..item(None)
        };
        let items = [
            item(Some("claude-routines/c")),
            item(Some("claude/a")),
            detached("/nonexistent/codex/worktrees/ffc8/repo"),
            detached("/elsewhere/pool/repo"),
            item(Some("claude/b")),
        ];
        assert_eq!(
            inventory_kinds(&items, Some(codex_home)),
            ["claude", "claude-routines", "codex"]
        );
        assert_eq!(inventory_kinds(&items, None), ["claude", "claude-routines"]);
        let codex: Vec<Option<PathBuf>> = selected(&items, None, Some("codex"), Some(codex_home))
            .iter()
            .map(|item| item.worktree.as_ref().map(|worktree| worktree.path.clone()))
            .collect();
        assert_eq!(
            codex,
            [Some(PathBuf::from(
                "/nonexistent/codex/worktrees/ffc8/repo"
            ))]
        );
    }

    fn item_at(path: PathBuf, prunable: bool) -> Item {
        Item {
            worktree: Some(inventory::WorktreeField {
                path,
                main: false,
                detached: false,
                locked: None,
                prunable,
                operation: None,
            }),
            ..item(Some("claude/a"))
        }
    }

    #[test]
    fn retry_failed_worktree() -> std::io::Result<()> {
        let transient = || Err(Error::Invariant("too many open files".to_owned()));
        let healed = retry_failed(&item_at(PathBuf::from("/gone"), true), transient());
        assert!(
            healed.is_ok_and(|worktree| worktree.is_some()),
            "a first failure is retried alone"
        );
        let kept = retry_failed(&item_at(PathBuf::from("/gone"), true), Ok(None));
        assert!(
            kept.is_ok_and(|worktree| worktree.is_none()),
            "a success is kept"
        );
        let dir = tempfile::tempdir()?;
        std::fs::write(dir.path().join(".git"), "not a gitdir pointer")?;
        let broken = retry_failed(&item_at(dir.path().to_owned(), false), transient());
        let text = match broken {
            Err(err) => err.to_string(),
            Ok(_) => String::from("unexpectedly healed"),
        };
        assert!(
            text.contains("src/gitstate.rs:") && !text.contains("error.rs"),
            "a second failure fails the row, naming its real call site: {text}"
        );
        Ok(())
    }

    #[test]
    fn worktree_row_vanished_checkout() -> std::io::Result<()> {
        // (label, creates the dir, expected prunable)
        let cases = [
            ("dir removed after inventory", false, true),
            ("dir left without .git", true, true),
        ];
        for (label, create, expected) in cases {
            let parent = tempfile::tempdir()?;
            let path = parent.path().join("wt");
            if create {
                std::fs::create_dir(&path)?;
            }
            let field = inventory::WorktreeField {
                path,
                main: false,
                detached: false,
                locked: None,
                prunable: false,
                operation: None,
            };
            let row = worktree_row(&field).unwrap_or_else(|err| panic!("{label}: {err}"));
            assert_eq!(row.prunable, expected, "{label}");
            assert_eq!((row.dirty, row.untracked), (0, 0), "{label}");
        }
        Ok(())
    }

    #[test]
    fn safety_matrix() {
        // (label, worktree, contained, unpicked, expected safe, expected reasons)
        type Case = (
            &'static str,
            Option<Worktree>,
            bool,
            usize,
            bool,
            &'static [&'static str],
        );
        let cases: [Case; 7] = [
            ("clean-contained", Some(worktree(0, 0)), true, 0, true, &[]),
            (
                "rebase-contained",
                Some(Worktree {
                    operation: Some(crate::wire::Operation::Rebase),
                    ..worktree(0, 0)
                }),
                true,
                0,
                false,
                &["operation(rebase)"],
            ),
            (
                "dirty-cherry-pick",
                Some(Worktree {
                    operation: Some(crate::wire::Operation::CherryPick),
                    ..worktree(1, 0)
                }),
                true,
                0,
                false,
                &["dirty(1)", "operation(cherry_pick)"],
            ),
            ("dirty", Some(worktree(2, 0)), true, 0, false, &["dirty(2)"]),
            (
                "untracked-unpicked",
                Some(worktree(0, 3)),
                false,
                1,
                false,
                &["untracked(3)", "unpicked(1)"],
            ),
            ("no-worktree-contained", None, true, 0, true, &[]),
            (
                "no-worktree-nowhere",
                None,
                false,
                0,
                false,
                &["work contained nowhere", "no worktree"],
            ),
        ];
        for (label, wt, contained, unpicked, expected_safe, expected_reasons) in cases {
            let (safe, reasons) = safety(wt.as_ref(), contained, unpicked);
            assert_eq!(safe, expected_safe, "{label}");
            assert_eq!(reasons, expected_reasons, "{label}");
        }
    }
}
