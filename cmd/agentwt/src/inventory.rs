//! The inventory: which agent branches and worktrees exist, with their head and worktree facts — from gix alone.
//!
//! Rows: local agent branches (with their worktree when one is checked out, the main checkout included) and detached
//! linked worktrees. Nothing here decides containment; that is gitstate's job.

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use gix::bstr::ByteSlice;

use crate::error::{Error, GitExt, Result};

/// Branch namespaces treated as agent-owned; the first path segment must be `claude` or start with `claude-`.
#[must_use]
pub fn agent_kind(branch: &str) -> Option<&str> {
    let (kind, _rest) = branch.split_once('/')?;
    (kind == "claude" || kind.starts_with("claude-")).then_some(kind)
}

/// A candidate is any local branch that is neither agent-owned nor a Workflow-tool isolation branch.
#[must_use]
pub fn is_candidate(branch: &str) -> bool {
    agent_kind(branch).is_none() && !branch.starts_with("worktree-wf_")
}

/// One row.
#[derive(Debug, Clone)]
pub struct Item {
    /// `None` for a detached worktree.
    pub branch: Option<String>,
    /// Tip facts.
    pub head: HeadField,
    /// Present when a checkout exists.
    pub worktree: Option<WorktreeField>,
}

/// Head facts.
#[derive(Debug, Clone)]
pub struct HeadField {
    /// Full sha.
    pub sha: String,
    /// Eight-character prefix.
    pub short_sha: String,
    /// Subject line.
    pub subject: String,
    /// RFC 3339 committer time.
    pub committed_at: String,
}

/// Worktree facts.
#[derive(Debug, Clone)]
pub struct WorktreeField {
    /// Canonical filesystem path (as recorded when the directory is gone).
    pub path: PathBuf,
    /// The main worktree.
    pub main: bool,
    /// HEAD is detached.
    pub detached: bool,
    /// Lock reason when locked.
    pub locked: Option<String>,
    /// The checkout directory is gone.
    pub prunable: bool,
    /// In-progress operation.
    pub operation: Option<OperationField>,
}

/// In-progress operation vocabulary.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OperationField {
    /// Merge.
    Merge,
    /// Rebase.
    Rebase,
    /// Cherry-pick.
    CherryPick,
    /// Revert.
    Revert,
    /// Bisect.
    Bisect,
}

/// Identity of a row: branch name, or worktree path for detached rows.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub enum RowKey {
    /// A named branch.
    Branch(String),
    /// A detached worktree, keyed by path.
    Detached(PathBuf),
}

impl Item {
    /// The row's identity.
    #[must_use]
    pub fn key(&self) -> Option<RowKey> {
        match (&self.branch, &self.worktree) {
            (Some(branch), _) => Some(RowKey::Branch(branch.clone())),
            (None, Some(worktree)) => Some(RowKey::Detached(worktree.path.clone())),
            (None, None) => None,
        }
    }
}

/// The inventory of one run.
#[derive(Debug)]
pub struct Inventory {
    /// The default branch (`origin/HEAD`, `init.defaultBranch`, `main`, `master`, else the first candidate).
    pub default_branch: Option<String>,
    /// Agent-branch rows in name order, then detached worktrees.
    pub items: Vec<Item>,
}

/// Canonical path of the repository's main checkout (the common git dir for a bare repository), whichever
/// worktree `path` names. Every consumer reads the repository through this path, never through the caller's
/// worktree: `remove` may delete that worktree mid-run (`/close` runs from inside it).
pub fn main_checkout(path: &Path) -> Result<PathBuf> {
    let main = gix::open(path).git()?.main_repo().git()?;
    let dir = main.workdir().unwrap_or_else(|| main.git_dir());
    dunce::canonicalize(dir).map_err(Error::io(dir))
}

impl Inventory {
    /// Reads refs and `.git/worktrees` of the repository `repo_path` belongs to.
    pub fn collect(repo_path: &Path, candidates: &[String]) -> Result<Inventory> {
        let repo = gix::open(repo_path).git()?.main_repo().git()?;
        let default_branch = crate::gitstate::default_branch_guess(&repo, candidates);

        let mut items: BTreeMap<String, Item> = BTreeMap::new();
        for reference in repo.references().git()?.local_branches().git()? {
            let mut reference =
                reference.map_err(|err| Error::Invariant(format!("reference iteration: {err}")))?;
            let name = reference.name().shorten().to_str_lossy().into_owned();
            if agent_kind(&name).is_none() {
                continue;
            }
            let tip = reference.peel_to_id().git()?.detach();
            let item = Item {
                branch: Some(name.clone()),
                head: head_field(&repo, tip)?,
                worktree: None,
            };
            items.insert(name, item);
        }

        let mut detached = Vec::new();
        if let Some(workdir) = repo.workdir() {
            let checkout = Checkout {
                path: dunce::canonicalize(workdir).unwrap_or_else(|_| workdir.to_owned()),
                main: true,
                locked: None,
                prunable: false,
            };
            attach_worktree(&repo, checkout, &mut items, &mut detached)?;
        }
        for proxy in repo.worktrees().map_err(Error::io(repo.git_dir()))? {
            let locked = proxy
                .lock_reason()
                .map(|reason| reason.to_str_lossy().into_owned());
            let path = proxy.base().map_err(Error::io(proxy.git_dir()))?;
            // Canonical (`/private/var` vs `/var` on macOS) while the dir still exists.
            let path = dunce::canonicalize(&path).unwrap_or(path);
            let checkout = Checkout {
                prunable: !path.is_dir(),
                path,
                main: false,
                locked,
            };
            let worktree_repo = proxy
                .into_repo_with_possibly_inaccessible_worktree()
                .git()?;
            attach_worktree(&worktree_repo, checkout, &mut items, &mut detached)?;
        }

        let mut all: Vec<Item> = items.into_values().collect();
        all.extend(detached);
        Ok(Inventory {
            default_branch,
            items: all,
        })
    }
}

/// A checkout found by gix, before its HEAD is read.
struct Checkout {
    path: PathBuf,
    main: bool,
    locked: Option<String>,
    prunable: bool,
}

/// Reads the checkout's HEAD: an agent branch gets the worktree attached to its item, a detached HEAD becomes a
/// row of its own, any other branch (or an unborn HEAD) is not reported.
fn attach_worktree(
    worktree_repo: &gix::Repository,
    checkout: Checkout,
    items: &mut BTreeMap<String, Item>,
    detached: &mut Vec<Item>,
) -> Result<()> {
    let head = worktree_repo.head().git()?;
    if head.is_unborn() {
        return Ok(());
    }
    let branch = head
        .referent_name()
        .map(|name| name.shorten().to_str_lossy().into_owned())
        .or_else(|| operating_branch(worktree_repo.git_dir()));
    let field = WorktreeField {
        path: checkout.path,
        main: checkout.main,
        detached: head.is_detached(),
        locked: checkout.locked,
        prunable: checkout.prunable,
        operation: operation_field(worktree_repo.state()),
    };
    if let Some(name) = branch {
        if let Some(item) = items.get_mut(&name) {
            item.worktree = Some(field);
        }
        return Ok(());
    }
    let tip = worktree_repo.head_id().git()?.detach();
    detached.push(Item {
        branch: None,
        head: head_field(worktree_repo, tip)?,
        worktree: Some(field),
    });
    Ok(())
}

/// The branch a detached checkout is operating on: `rebase-merge/head-name` or `rebase-apply/head-name` during a
/// rebase, `BISECT_START` during a bisect; `None` for a plain detached HEAD.
fn operating_branch(git_dir: &Path) -> Option<String> {
    [
        "rebase-merge/head-name",
        "rebase-apply/head-name",
        "BISECT_START",
    ]
    .into_iter()
    .filter_map(|rela| std::fs::read_to_string(git_dir.join(rela)).ok())
    .map(|text| {
        let name = text.trim();
        name.strip_prefix("refs/heads/").unwrap_or(name).to_owned()
    })
    .find(|name| !name.is_empty() && name != "detached HEAD")
}

/// Maps gix's in-progress state onto the operation vocabulary.
fn operation_field(state: Option<gix::state::InProgress>) -> Option<OperationField> {
    use gix::state::InProgress;
    match state? {
        InProgress::Merge => Some(OperationField::Merge),
        InProgress::Rebase | InProgress::RebaseInteractive | InProgress::ApplyMailboxRebase => {
            Some(OperationField::Rebase)
        }
        InProgress::CherryPick | InProgress::CherryPickSequence => Some(OperationField::CherryPick),
        InProgress::Revert | InProgress::RevertSequence => Some(OperationField::Revert),
        InProgress::Bisect => Some(OperationField::Bisect),
        InProgress::ApplyMailbox => None,
    }
}

/// Head facts of `tip`.
fn head_field(repo: &gix::Repository, tip: gix::ObjectId) -> Result<HeadField> {
    let commit = repo.find_commit(tip).git()?;
    let subject = commit
        .message()
        .git()?
        .summary()
        .to_str_lossy()
        .into_owned();
    let committed_at = crate::gitstate::rfc3339_utc(commit.time().git()?.seconds);
    Ok(HeadField {
        sha: tip.to_string(),
        short_sha: tip.to_hex_with_len(8).to_string(),
        subject,
        committed_at,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn agent_kind_and_candidates() {
        let cases: [(&str, Option<&str>, bool); 5] = [
            ("claude/x", Some("claude"), false),
            ("claude-routines/nightly", Some("claude-routines"), false),
            ("main", None, true),
            ("worktree-wf_abc", None, false),
            ("release/v1.2.0", None, true),
        ];
        for (branch, kind, candidate) in cases {
            assert_eq!(agent_kind(branch), kind, "{branch}");
            assert_eq!(is_candidate(branch), candidate, "{branch}");
        }
    }

    #[test]
    fn operating_branch_files() -> std::io::Result<()> {
        // (label, file written under the git dir with its content, expected branch)
        type Case = (
            &'static str,
            Option<(&'static str, &'static str)>,
            Option<&'static str>,
        );
        let cases: [Case; 5] = [
            (
                "rebase-merge",
                Some(("rebase-merge/head-name", "refs/heads/claude/x\n")),
                Some("claude/x"),
            ),
            (
                "rebase-apply",
                Some(("rebase-apply/head-name", "refs/heads/claude/y\n")),
                Some("claude/y"),
            ),
            (
                "bisect",
                Some(("BISECT_START", "claude/z\n")),
                Some("claude/z"),
            ),
            ("none", None, None),
            (
                "detached-rebase",
                Some(("rebase-merge/head-name", "detached HEAD\n")),
                None,
            ),
        ];
        'cases: for (label, file, expected) in cases {
            let dir = tempfile::tempdir()?;
            let Some((rela, content)) = file else {
                assert_eq!(operating_branch(dir.path()), None, "{label}");
                continue 'cases;
            };
            let path = dir.path().join(rela);
            if let Some(parent) = path.parent() {
                std::fs::create_dir_all(parent)?;
            }
            std::fs::write(&path, content)?;
            assert_eq!(operating_branch(dir.path()).as_deref(), expected, "{label}");
        }
        Ok(())
    }
}
