//! Base branch, ahead/behind, containment, working-tree state — all with gix.
//!
//! Decisions: one `gix::Repository` per `Context`, opened with object memory so every simulated merge stays in RAM;
//! containment is searched lazily — candidates in preference order (base, default, others), the first that holds
//! the work wins; a candidate holds it by ancestry of the tip, by every own commit being placed, or by a merge of the
//! tip adding nothing; a commit is placed by ancestry, a no-op three-way merge, a replay (pick trailer, or the same
//! author, author time and subject committed no earlier) or a patch-id twin, searched among the candidate's commits
//! after the branch's fork points; the base is the creation line unless a rebase, reset or `branch -f` moved the
//! branch onto another candidate, and is never guessed.
//! Invariant: nothing here writes refs, the working tree or an index, and nothing takes `index.lock` — a status
//! that locked the index would fail an agent's concurrent `git add` or `git commit`. Racily-clean entries (mtime not
//! older than the index) therefore cost a content hash on every scan until a git command of the worktree's own
//! session rewrites the index.

use std::collections::{BTreeSet, HashMap, HashSet};
use std::path::Path;

use gix::ObjectId;
use gix::bstr::{BStr, BString, ByteSlice};
use gix::merge::tree::TreatAsUnresolved;
use gix::objs::tree::EntryKind;
use gix::revision::walk::Sorting;
use gix::traverse::commit::simple::CommitTimeOrder;

use crate::error::{Error, GitExt, Result};
use crate::inventory::{agent_kind, is_candidate};
use crate::patchid::{self, PatchId};
use crate::wire::{Containment, ContainmentRank, ContainmentReason, FromResolution};

/// Twin-walk cache key: candidate tip plus the row's fork points.
type TwinWalkKey = (ObjectId, Vec<ObjectId>);

/// Cap on the `behind` commit list in the document.
pub const BEHIND_CAP: usize = 200;
/// Maximum reflog hops when following agent-branch origins.
const CHAIN_HOPS: usize = 20;

/// Untracked entries the remove gate and the untracked count ignore (worktree infrastructure and tool droppings).
const UNTRACKED_EXCLUDED_PREFIXES: [&str; 2] = [".idea/", ".claude/"];
const UNTRACKED_EXCLUDED_NAMES: [&str; 2] = [".claude-worktree", ".cell-pids"];

/// A non-agent local branch every question is asked against.
#[derive(Debug, Clone)]
pub struct Candidate {
    /// Branch name.
    pub name: String,
    /// Tip commit.
    pub tip: ObjectId,
    /// Tip tree.
    pub tree: ObjectId,
}

/// One commit as the document needs it.
#[derive(Debug, Clone)]
pub struct CommitInfo {
    /// The commit.
    pub id: ObjectId,
    /// Its parents.
    pub parents: Vec<ObjectId>,
    /// Subject line.
    pub subject: String,
    /// Committer time, RFC 3339 UTC.
    pub committed_at: String,
}

impl CommitInfo {
    /// More than one parent.
    #[must_use]
    pub fn is_merge(&self) -> bool {
        self.parents.len() > 1
    }
}

/// Resolved base of a branch.
#[derive(Debug, Clone)]
pub struct Base {
    /// Ref name as the reflog spelled it (`main`, `origin/main`), `None` when unknown.
    pub name: Option<String>,
    /// Its tip.
    pub tip: Option<ObjectId>,
    /// How it was resolved.
    pub resolution: FromResolution,
    /// The raw reflog origin.
    pub origin: Option<String>,
}

/// Everything gitstate computes for one row.
#[derive(Debug, Clone)]
pub struct BranchAnalysis {
    /// The base.
    pub base: Base,
    /// Commits reachable from the tip but not from the reference (base, or default when the base is unknown), newest first.
    pub ahead: Vec<CommitInfo>,
    /// Reference commits missing here, capped.
    pub behind: Vec<CommitInfo>,
    /// Full behind count (the list above is capped).
    pub behind_total: usize,
    /// The list was cut.
    pub behind_truncated: bool,
    /// The first candidate, in preference order base → default → others, holding the work; `None` when none does.
    pub containment: Option<Containment>,
    /// Own non-merge commits no candidate holds, newest first; empty when contained.
    pub unpicked: Vec<ObjectId>,
}

/// What a cherry-pick or rebase replays unchanged: author email, author time and subject.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
struct ReplayKey {
    author_email: BString,
    author_seconds: i64,
    subject: BString,
}

/// A candidate's non-merge commits after the fork points, indexed for the per-commit layers.
#[derive(Debug, Clone, Default)]
struct TwinWalk {
    /// (commit, first parent), newest first — the patch-id twin scan.
    commits: Vec<(ObjectId, ObjectId)>,
    /// Replay identity → the newest committer time among candidate commits carrying it.
    replays: HashMap<ReplayKey, i64>,
    /// Commits named by a `(cherry picked from commit <sha>)` trailer.
    picked_from: HashSet<ObjectId>,
}

/// Working-tree state of one checkout.
#[derive(Debug, Clone, Default)]
pub struct TreeState {
    /// Tracked changes, staged or unstaged (distinct paths).
    pub dirty: usize,
    /// Untracked paths after the infrastructure filter.
    pub untracked: Vec<String>,
}

/// One opened repository plus its candidates and the per-run caches of the twin search.
pub struct Context {
    repo: gix::Repository,
    candidates: Vec<Candidate>,
    default_branch: String,
    /// Commit → paths its first-parent diff touches (shared by every row and candidate).
    paths_cache: HashMap<ObjectId, BTreeSet<BString>>,
    /// Commit → patch-id of its first-parent diff.
    patch_id_cache: HashMap<ObjectId, PatchId>,
    /// (candidate tip, fork points) → the candidate's non-merge commits newer than the forks, indexed.
    twin_walks: HashMap<TwinWalkKey, TwinWalk>,
}

impl Context {
    /// Opens `path` (any worktree of the repo; the common object store is shared) with object memory and a
    /// 64 MiB object cache, and lists candidates with the default branch first. `added`: branches tracked as
    /// candidates on request (`--candidate`), agent-owned or not.
    pub fn open(path: &Path, default_branch: &str, added: &[String]) -> Result<Context> {
        let mut repo = gix::open(path).git()?.with_object_memory();
        repo.object_cache_size_if_unset(64 * 1024 * 1024);
        drop_merge_drivers(&mut repo)?;

        let mut candidates = Vec::new();
        for reference in repo.references().git()?.local_branches().git()? {
            let mut reference =
                reference.map_err(|err| Error::Invariant(format!("reference iteration: {err}")))?;
            let name = reference.name().shorten().to_str_lossy().into_owned();
            if !is_candidate(&name) && !added.contains(&name) {
                continue;
            }
            let tip = reference.peel_to_id().git()?.detach();
            let tree = repo.find_commit(tip).git()?.tree_id().git()?.detach();
            candidates.push(Candidate { name, tip, tree });
        }
        candidates
            .sort_by_key(|candidate| (candidate.name != default_branch, candidate.name.clone()));
        Ok(Context {
            repo,
            candidates,
            default_branch: default_branch.to_owned(),
            paths_cache: HashMap::new(),
            patch_id_cache: HashMap::new(),
            twin_walks: HashMap::new(),
        })
    }

    /// The candidates, default branch first.
    #[must_use]
    pub fn candidates(&self) -> &[Candidate] {
        &self.candidates
    }
}

/// Removes every `merge.<name>` section from the opened repository's configuration, so the in-memory merges below
/// run on gix's built-in drivers only. gix spawns a configured `merge.<name>.driver` per blob and turns the driver's
/// conflict exit into a hard error, while these merges only ask whether a change is already on a candidate — an answer
/// no external driver changes. The real merges (`git merge-tree`, `cherry-pick`) keep the drivers.
fn drop_merge_drivers(repo: &mut gix::Repository) -> Result<()> {
    let mut config = repo.config_snapshot_mut();
    let driver_sections: Vec<_> = config
        .sections_and_ids_by_name("merge")
        .into_iter()
        .flatten()
        .filter(|(section, _)| section.header().subsection_name().is_some())
        .map(|(_, id)| id)
        .collect();
    for id in driver_sections {
        config.remove_section_by_id(id);
    }
    config.commit().git()?;
    Ok(())
}

/// The default branch without worktrunk: `origin/HEAD`'s target, then `init.defaultBranch`, then `main`, then
/// `master` — each only when it is a candidate — else the first candidate; `None` without candidates.
#[must_use]
pub fn default_branch_guess(repo: &gix::Repository, candidates: &[String]) -> Option<String> {
    let is_candidate = |name: &str| candidates.iter().any(|candidate| candidate == name);
    if let Ok(Some(reference)) = repo.try_find_reference("refs/remotes/origin/HEAD")
        && let gix::refs::TargetRef::Symbolic(target) = reference.target()
    {
        let short = target.shorten().to_str_lossy();
        let name = short.strip_prefix("origin/").unwrap_or(&short);
        if is_candidate(name) {
            return Some(name.to_owned());
        }
    }
    let configured = repo
        .config_snapshot()
        .string("init.defaultBranch")
        .map(|value| value.to_str_lossy().into_owned());
    configured
        .into_iter()
        .chain(["main".to_owned(), "master".to_owned()])
        .find(|name| is_candidate(name))
        .or_else(|| candidates.first().cloned())
}

impl Context {
    /// Borrow the handle (the order module merges on it).
    #[must_use]
    pub fn repo(&self) -> &gix::Repository {
        &self.repo
    }

    /// Full analysis of one row. `branch` is `None` for a detached worktree.
    pub fn analyze(&mut self, branch: Option<&str>, tip: ObjectId) -> Result<BranchAnalysis> {
        let base = match branch {
            Some(branch) => self.resolve_base(branch)?,
            None => Base {
                name: None,
                tip: None,
                resolution: FromResolution::Unknown,
                origin: None,
            },
        };
        let reference = base
            .tip
            .or_else(|| self.default_candidate().map(|candidate| candidate.tip));
        let (ahead, behind, behind_total, behind_truncated) = match reference {
            Some(reference) => {
                let ahead = self.commits_between(tip, reference, None)?;
                let mut behind = self.commits_between(reference, tip, Some(BEHIND_CAP + 1))?;
                let truncated = behind.len() > BEHIND_CAP;
                behind.truncate(BEHIND_CAP);
                // The list is capped; the count is not (a plain walk without object decoding).
                let total = self
                    .repo
                    .rev_walk([reference])
                    .with_hidden([tip])
                    .all()
                    .git()?
                    .count();
                (ahead, behind, total, truncated)
            }
            None => (Vec::new(), Vec::new(), 0, false),
        };

        let forks = fork_points(&ahead);
        let containment = self.find_containment(&base, branch, tip, &ahead, &forks)?;
        let unpicked = match containment {
            Some(_) => Vec::new(),
            None => self.placed_nowhere(branch, &ahead, &forks)?,
        };
        Ok(BranchAnalysis {
            base,
            ahead,
            behind,
            behind_total,
            behind_truncated,
            containment,
            unpicked,
        })
    }

    /// What `tip` still owes `target`: the commits reachable from `tip` and not from the target's tip that no
    /// per-commit layer places on the target — merges and roots dropped, parents before children. The target is the
    /// only candidate asked; the range hangs on the two tips, never on a reflog.
    pub fn unplaced(&mut self, tip: ObjectId, target: &Candidate) -> Result<Vec<CommitInfo>> {
        let ahead = self.commits_between(tip, target.tip, None)?;
        let forks = fork_points(&ahead);
        let mut unplaced = Vec::new();
        for commit in parents_first(&ahead) {
            if commit.is_merge() || commit.parents.is_empty() {
                continue;
            }
            if !self.placed(commit, &forks, target)? {
                unplaced.push(commit.clone());
            }
        }
        Ok(unplaced)
    }

    /// Does `target` already hold the non-merge `commit`? An ancestor of its tip, or placed by the same per-commit
    /// layers `unplaced` asks (no-op re-apply, replay, patch-id twin) — the containment `pick` refuses to repeat.
    pub fn holds(&mut self, commit: ObjectId, target: &Candidate) -> Result<bool> {
        let ahead = self.commits_between(commit, target.tip, None)?;
        let Some(own) = ahead.iter().find(|info| info.id == commit).cloned() else {
            return Ok(true);
        };
        let forks = fork_points(&ahead);
        self.placed(&own, &forks, target)
    }

    /// Candidates in preference order with their rank: the base (when it is a candidate), the default branch, then
    /// every other candidate by name; never the row's own branch (an added candidate is also a row). Owned clones, so
    /// the search can borrow `self` mutably for its caches.
    fn candidate_order(
        &self,
        base: &Base,
        branch: Option<&str>,
    ) -> Vec<(Candidate, ContainmentRank)> {
        let base_name = base.name.as_deref();
        let mut order = Vec::with_capacity(self.candidates.len());
        if let Some(candidate) = self
            .candidates
            .iter()
            .find(|candidate| Some(candidate.name.as_str()) == base_name)
        {
            order.push((candidate.clone(), ContainmentRank::Base));
        }
        if base_name != Some(self.default_branch.as_str())
            && let Some(candidate) = self.default_candidate()
        {
            order.push((candidate.clone(), ContainmentRank::Default));
        }
        // `candidates` is sorted default first, then by name.
        order.extend(
            self.candidates
                .iter()
                .filter(|candidate| {
                    candidate.name != self.default_branch
                        && Some(candidate.name.as_str()) != base_name
                })
                .map(|candidate| (candidate.clone(), ContainmentRank::Other)),
        );
        order.retain(|(candidate, _)| Some(candidate.name.as_str()) != branch);
        order
    }

    /// The first candidate, in preference order, that holds this branch's work.
    fn find_containment(
        &mut self,
        base: &Base,
        branch: Option<&str>,
        tip: ObjectId,
        own: &[CommitInfo],
        forks: &[ObjectId],
    ) -> Result<Option<Containment>> {
        for (candidate, rank) in self.candidate_order(base, branch) {
            if let Some(reason) = self.contains(tip, own, forks, &candidate)? {
                return Ok(Some(Containment {
                    branch: candidate.name,
                    reason,
                    rank,
                }));
            }
        }
        Ok(None)
    }

    /// Does `candidate` hold this branch's work? The tip being an ancestor settles it; otherwise every own non-merge
    /// commit (the branch's work: `base..tip`, newest first) placed there, else merging the tip adding nothing. What
    /// the candidate lacks of the base is not this branch's business.
    fn contains(
        &mut self,
        tip: ObjectId,
        own: &[CommitInfo],
        forks: &[ObjectId],
        candidate: &Candidate,
    ) -> Result<Option<ContainmentReason>> {
        if self.is_ancestor(tip, candidate.tip)? {
            return Ok(Some(ContainmentReason::Ancestor));
        }
        if self.all_placed(own, forks, candidate)? {
            return Ok(Some(ContainmentReason::AllCommitsPlaced));
        }
        if self.merge_adds_nothing(tip, candidate)? {
            return Ok(Some(ContainmentReason::MergeAddsNothing));
        }
        Ok(None)
    }

    /// Every own non-merge commit is placed on `candidate`, and there is at least one; the first unplaced commit ends
    /// the check.
    fn all_placed(
        &mut self,
        own: &[CommitInfo],
        forks: &[ObjectId],
        candidate: &Candidate,
    ) -> Result<bool> {
        let mut any = false;
        for commit in own
            .iter()
            .filter(|commit| !commit.is_merge() && !commit.parents.is_empty())
        {
            if !self.placed(commit, forks, candidate)? {
                return Ok(false);
            }
            any = true;
        }
        Ok(any)
    }

    /// Merging the tip into the candidate adds nothing: a clean in-memory merge whose tree equals the candidate's.
    /// Catches what no single commit shows — a squash merge, commits that cancel out.
    fn merge_adds_nothing(&self, tip: ObjectId, candidate: &Candidate) -> Result<bool> {
        if let [base] = self.repo.merge_bases_many(candidate.tip, &[tip]).git()?[..] {
            let changed = patchid::touched_paths(&self.repo, base.detach(), tip)?;
            let base_tree = self.repo.find_commit(base).git()?.tree_id().git()?.detach();
            if self.keeps_any(base_tree, candidate.tree, &changed)? {
                return Ok(false);
            }
        }
        let options = self
            .repo
            .tree_merge_options()
            .git()?
            .with_fail_on_conflict(Some(TreatAsUnresolved::git()));
        let labels = gix::merge::blob::builtin_driver::text::Labels::default();
        let mut outcome = self
            .repo
            .merge_commits(candidate.tip, tip, labels, options.into())
            .git()?;
        if outcome
            .tree_merge
            .has_unresolved_conflicts(TreatAsUnresolved::git())
        {
            return Ok(false);
        }
        let merged = outcome.tree_merge.tree.write().git()?.detach();
        Ok(merged == candidate.tree)
    }

    /// Own non-merge commits that no candidate (the row's own branch excluded) holds by any per-commit layer, newest
    /// first.
    fn placed_nowhere(
        &mut self,
        branch: Option<&str>,
        own: &[CommitInfo],
        forks: &[ObjectId],
    ) -> Result<Vec<ObjectId>> {
        let candidates: Vec<Candidate> = self
            .candidates
            .iter()
            .filter(|candidate| Some(candidate.name.as_str()) != branch)
            .cloned()
            .collect();
        let mut unpicked = Vec::new();
        for commit in own
            .iter()
            .filter(|commit| !commit.is_merge() && !commit.parents.is_empty())
        {
            if !self.placed_anywhere(commit, forks, &candidates)? {
                unpicked.push(commit.id);
            }
        }
        Ok(unpicked)
    }

    /// Some candidate holds `commit`.
    fn placed_anywhere(
        &mut self,
        commit: &CommitInfo,
        forks: &[ObjectId],
        candidates: &[Candidate],
    ) -> Result<bool> {
        for candidate in candidates {
            if self.placed(commit, forks, candidate)? {
                return Ok(true);
            }
        }
        Ok(false)
    }

    fn default_candidate(&self) -> Option<&Candidate> {
        self.candidates
            .iter()
            .find(|candidate| candidate.name == self.default_branch)
    }

    /// The base: the creation line followed through agent-branch chains (named, `origin/<x>`, bare sha, `HEAD`, dead
    /// hops resolved through the start commit); a revision suffix on the origin (`main^0`, `claude/a~1`) is cut and
    /// the ref before it followed; a later rebase, reset or `branch -f` onto a different candidate overrides it.
    fn resolve_base(&self, branch: &str) -> Result<Base> {
        let created = self.created_base(branch)?;
        match self.reanchored_base(branch)? {
            Some(moved) if moved.name != created.name => Ok(moved),
            Some(_) | None => Ok(created),
        }
    }

    /// The creation line's base.
    fn created_base(&self, branch: &str) -> Result<Base> {
        let mut current = branch.to_owned();
        let mut resolution = FromResolution::Ref;
        let mut first_origin = None;
        let mut start = None;
        for _hop in 0..CHAIN_HOPS {
            let Some((origin, created_at)) = self.creation_entry(&current)? else {
                break;
            };
            first_origin.get_or_insert_with(|| origin.clone());
            start = Some(created_at);
            let short = origin
                .strip_prefix("refs/remotes/")
                .or_else(|| origin.strip_prefix("refs/heads/"))
                .unwrap_or(&origin);
            let short = origin_ref(short);
            if short.is_empty() || short == "HEAD" {
                break;
            }
            if gix::validate::reference::name_partial(short.into()).is_err() {
                break;
            }
            if agent_kind(short).is_some() {
                short.clone_into(&mut current);
                resolution = FromResolution::Chain;
                continue;
            }
            if let Some(sha) = full_sha(short) {
                return self.base_at(sha, FromResolution::Sha, first_origin);
            }
            if let Some(base) = self.named_base(short, resolution, first_origin.clone())? {
                return Ok(base);
            }
            break;
        }
        let fallback = match resolution {
            FromResolution::Chain => FromResolution::Chain,
            FromResolution::Ref
            | FromResolution::Sha
            | FromResolution::Reanchored
            | FromResolution::Unknown => FromResolution::Sha,
        };
        match start {
            Some(sha) => self.base_at(sha, fallback, first_origin),
            None => Ok(Base {
                name: None,
                tip: None,
                resolution: FromResolution::Unknown,
                origin: first_origin,
            }),
        }
    }

    /// `branch: Created from <X>` → (`X`, the commit the branch started at), from the oldest reflog line.
    fn creation_entry(&self, branch: &str) -> Result<Option<(String, ObjectId)>> {
        let Some(reference) = self
            .repo
            .try_find_reference(&format!("refs/heads/{branch}"))
            .git()?
        else {
            return Ok(None);
        };
        let mut platform = reference.log_iter();
        let Some(mut lines) = platform.all().map_err(Error::io(self.repo.git_dir()))? else {
            return Ok(None);
        };
        let Some(first) = lines.next() else {
            return Ok(None);
        };
        let first = first.git()?;
        let message = first.message.to_str_lossy();
        let Some(origin) = message.strip_prefix("branch: Created from ") else {
            return Ok(None);
        };
        Ok(Some((origin.to_owned(), reflog_oid(first.new_oid)?)))
    }

    /// The newest rebase, reset or `branch -f` entry whose target a candidate holds; entries onto the branch's own
    /// commits (`reset --hard HEAD~1`) are skipped.
    fn reanchored_base(&self, branch: &str) -> Result<Option<Base>> {
        let Some(reference) = self
            .repo
            .try_find_reference(&format!("refs/heads/{branch}"))
            .git()?
        else {
            return Ok(None);
        };
        let mut platform = reference.log_iter();
        let Some(lines) = platform.all().map_err(Error::io(self.repo.git_dir()))? else {
            return Ok(None);
        };
        let mut entries = Vec::new();
        for line in lines {
            let line = line.git()?;
            entries.push((
                line.message.to_str_lossy().into_owned(),
                reflog_oid(line.new_oid)?,
            ));
        }
        for (message, new_oid) in entries.into_iter().rev() {
            let Some(target) = reanchor_target(&message, new_oid) else {
                continue;
            };
            if let Some(candidate) = self.closest_candidate_containing(target)? {
                return Ok(Some(Base {
                    name: Some(candidate.name.clone()),
                    tip: Some(candidate.tip),
                    resolution: FromResolution::Reanchored,
                    origin: Some(message),
                }));
            }
        }
        Ok(None)
    }

    /// A named origin as a base: `<remote>/<x>` maps to the local candidate `<x>` when that exists and the name is not
    /// itself a local candidate; else the ref as named. `None` when no such ref exists.
    fn named_base(
        &self,
        short: &str,
        resolution: FromResolution,
        origin: Option<String>,
    ) -> Result<Option<Base>> {
        let is_local = |name: &str| {
            self.candidates
                .iter()
                .any(|candidate| candidate.name == name)
        };
        let name = short
            .split_once('/')
            .map(|(_remote, local)| local)
            .filter(|local| is_local(local) && !is_local(short))
            .unwrap_or(short);
        let Some(mut reference) = self.repo.try_find_reference(name).git()? else {
            return Ok(None);
        };
        let tip = reference.peel_to_id().git()?.detach();
        Ok(Some(Base {
            name: Some(name.to_owned()),
            tip: Some(tip),
            resolution,
            origin,
        }))
    }

    /// The closest candidate holding `sha`, else the closest holding its nearest first-parent ancestor a candidate
    /// holds; unknown when the walk reaches the root.
    fn base_at(
        &self,
        sha: ObjectId,
        resolution: FromResolution,
        origin: Option<String>,
    ) -> Result<Base> {
        let mut current = Some(sha);
        while let Some(id) = current {
            if let Some(candidate) = self.closest_candidate_containing(id)? {
                return Ok(Base {
                    name: Some(candidate.name.clone()),
                    tip: Some(candidate.tip),
                    resolution,
                    origin,
                });
            }
            current = self
                .repo
                .find_commit(id)
                .git()?
                .parent_ids()
                .next()
                .map(gix::Id::detach);
        }
        Ok(Base {
            name: None,
            tip: None,
            resolution: FromResolution::Unknown,
            origin,
        })
    }

    /// Among candidates containing `sha`, the one with the fewest commits between `sha` and its tip.
    fn closest_candidate_containing(&self, sha: ObjectId) -> Result<Option<&Candidate>> {
        let mut best: Option<(usize, &Candidate)> = None;
        for candidate in &self.candidates {
            if !self.is_ancestor(sha, candidate.tip)? {
                continue;
            }
            let distance = self
                .repo
                .rev_walk([candidate.tip])
                .with_hidden([sha])
                .all()
                .git()?
                .count();
            if best.is_none_or(|(best_distance, _)| distance < best_distance) {
                best = Some((distance, candidate));
            }
        }
        Ok(best.map(|(_, candidate)| candidate))
    }

    /// `ancestor` reachable from `descendant` (equal counts).
    fn is_ancestor(&self, ancestor: ObjectId, descendant: ObjectId) -> Result<bool> {
        if ancestor == descendant {
            return Ok(true);
        }
        let bases = self.repo.merge_bases_many(descendant, &[ancestor]).git()?;
        Ok(bases.iter().any(|base| base.detach() == ancestor))
    }

    /// Commits reachable from `tip` but not from `hidden`, newest first, optionally capped.
    fn commits_between(
        &self,
        tip: ObjectId,
        hidden: ObjectId,
        cap: Option<usize>,
    ) -> Result<Vec<CommitInfo>> {
        let walk = self
            .repo
            .rev_walk([tip])
            .with_hidden([hidden])
            .sorting(Sorting::ByCommitTime(CommitTimeOrder::NewestFirst))
            .all()
            .git()?;
        let mut commits = Vec::new();
        for info in walk {
            let info = info.git()?;
            commits.push(self.commit_info(info.id, info.parent_ids.iter().copied().collect())?);
            if cap.is_some_and(|cap| commits.len() >= cap) {
                break;
            }
        }
        Ok(commits)
    }

    fn commit_info(&self, id: ObjectId, parents: Vec<ObjectId>) -> Result<CommitInfo> {
        let commit = self.repo.find_commit(id).git()?;
        let subject = commit
            .message()
            .git()?
            .summary()
            .to_str_lossy()
            .into_owned();
        let committed_at = rfc3339_utc(commit.time().git()?.seconds);
        Ok(CommitInfo {
            id,
            parents,
            subject,
            committed_at,
        })
    }

    /// Is the non-merge `commit` placed on `candidate`? Layers, cheapest first: reachable from the candidate tip;
    /// re-applying it onto the candidate changes nothing; a candidate commit newer than the forks replays it; one has
    /// the same patch-id.
    fn placed(
        &mut self,
        commit: &CommitInfo,
        forks: &[ObjectId],
        candidate: &Candidate,
    ) -> Result<bool> {
        if self.is_ancestor(commit.id, candidate.tip)? {
            return Ok(true);
        }
        if self.applies_as_no_op(commit, candidate)? {
            return Ok(true);
        }
        if self.replayed(commit, forks, candidate)? {
            return Ok(true);
        }
        self.has_twin(commit, forks, candidate)
    }

    /// The candidate replays `commit`: a pick trailer names it, or a candidate commit after the fork with the same
    /// author email, author time and subject was committed no earlier — a cherry-pick, conflict resolution included.
    /// A rewrite after the pick (amend, reorder, rebase) is newer than the pick and is not matched.
    fn replayed(
        &mut self,
        commit: &CommitInfo,
        forks: &[ObjectId],
        candidate: &Candidate,
    ) -> Result<bool> {
        let (key, committed, _) = self.replay_facts(commit.id)?;
        let index = self.twin_index(candidate.tip, forks)?;
        if index.picked_from.contains(&commit.id) {
            return Ok(true);
        }
        Ok(index
            .replays
            .get(&key)
            .is_some_and(|newest| *newest >= committed))
    }

    /// The replay identity, committer time and pick-trailer source of `id`.
    fn replay_facts(&self, id: ObjectId) -> Result<(ReplayKey, i64, Option<ObjectId>)> {
        let commit = self.repo.find_commit(id).git()?;
        let author = commit.author().git()?;
        let picked_from = pick_trailer(commit.message_raw().git()?);
        let key = ReplayKey {
            author_email: author.email.to_owned(),
            author_seconds: author.time().git()?.seconds,
            subject: commit.message().git()?.summary().into_owned(),
        };
        Ok((key, commit.time().git()?.seconds, picked_from))
    }

    /// The cached walk of (candidate, forks), built on first use.
    fn twin_index(&mut self, candidate_tip: ObjectId, forks: &[ObjectId]) -> Result<&TwinWalk> {
        let walk_key = (candidate_tip, forks.to_vec());
        if !self.twin_walks.contains_key(&walk_key) {
            let walk = self.twin_walk(candidate_tip, forks)?;
            self.twin_walks.insert(walk_key.clone(), walk);
        }
        self.twin_walks
            .get(&walk_key)
            .ok_or_else(|| Error::Invariant("twin walk vanished".to_owned()))
    }

    /// Three-way merge of the commit's change (base = its parent) onto the candidate tip, in memory: `true` when it
    /// merges cleanly and the result tree equals the candidate tree (the change is already there).
    fn applies_as_no_op(&mut self, commit: &CommitInfo, candidate: &Candidate) -> Result<bool> {
        let parent = commit.parents[0];
        let changed = self.paths_of(commit.id, parent)?;
        let base_tree = self
            .repo
            .find_commit(parent)
            .git()?
            .tree_id()
            .git()?
            .detach();
        let their_tree = self
            .repo
            .find_commit(commit.id)
            .git()?
            .tree_id()
            .git()?
            .detach();
        if self.keeps_any(base_tree, candidate.tree, &changed)? {
            return Ok(false);
        }
        let options = self
            .repo
            .tree_merge_options()
            .git()?
            .with_fail_on_conflict(Some(TreatAsUnresolved::git()));
        let labels = gix::merge::blob::builtin_driver::text::Labels {
            ancestor: Some(b"parent".as_bstr()),
            current: Some(candidate.name.as_bytes().as_bstr()),
            other: Some(b"commit".as_bstr()),
        };
        let mut outcome = self
            .repo
            .merge_trees(base_tree, candidate.tree, their_tree, labels, options)
            .git()?;
        if outcome.has_unresolved_conflicts(TreatAsUnresolved::git()) {
            return Ok(false);
        }
        let merged = outcome.tree.write().git()?.detach();
        Ok(merged == candidate.tree)
    }

    /// Some `changed` path holds the same entry in `ours` as in `base`. A three-way merge takes the other side's
    /// change there, so the result cannot equal `ours` — the no-op and net checks fail without merging.
    fn keeps_any(
        &self,
        base: ObjectId,
        ours: ObjectId,
        changed: &BTreeSet<BString>,
    ) -> Result<bool> {
        let base = self.repo.find_tree(base).git()?;
        let ours = self.repo.find_tree(ours).git()?;
        for rela in changed {
            let path = gix::path::from_bstr(rela.as_bstr());
            let in_base = base.lookup_entry_by_path(&path).git()?;
            let in_ours = ours.lookup_entry_by_path(&path).git()?;
            let same = match (in_base, in_ours) {
                (None, None) => true,
                (Some(b), Some(o)) => b.mode() == o.mode() && b.oid() == o.oid(),
                _ => false,
            };
            if same {
                return Ok(true);
            }
        }
        Ok(false)
    }

    /// A candidate commit newer than the forks that touches the same paths with the same patch-id — whether or not
    /// the branch also reaches it (main merged back after the pick). Paths and patch-ids are cached per commit, the
    /// walk per (candidate, forks).
    fn has_twin(
        &mut self,
        commit: &CommitInfo,
        forks: &[ObjectId],
        candidate: &Candidate,
    ) -> Result<bool> {
        let own_paths = self.paths_of(commit.id, commit.parents[0])?;
        if own_paths.is_empty() {
            return Ok(false);
        }
        let own_id = self.patch_id_of(commit.id, commit.parents[0])?;
        let commits = self.twin_index(candidate.tip, forks)?.commits.clone();
        for (id, parent) in commits {
            // The patch-id hashes every changed path, so a twin touches exactly the same set: compare the cached sets
            // before paying for a patch-id.
            if self.paths_of(id, parent)? != own_paths {
                continue;
            }
            if self.patch_id_of(id, parent)? == own_id {
                return Ok(true);
            }
        }
        Ok(false)
    }

    /// The candidate's non-merge commits not reachable from the forks, newest first, indexed by replay identity and
    /// pick trailer next to the (commit, first parent) list.
    fn twin_walk(&self, candidate_tip: ObjectId, forks: &[ObjectId]) -> Result<TwinWalk> {
        let walk = self
            .repo
            .rev_walk([candidate_tip])
            .with_hidden(forks.iter().copied())
            .sorting(Sorting::ByCommitTime(CommitTimeOrder::NewestFirst))
            .all()
            .git()?;
        let mut index = TwinWalk::default();
        for info in walk {
            let info = info.git()?;
            if info.parent_ids.len() == 1 {
                index.commits.push((info.id, info.parent_ids[0]));
                let (key, committed, picked_from) = self.replay_facts(info.id)?;
                let newest = index.replays.entry(key).or_insert(committed);
                *newest = (*newest).max(committed);
                index.picked_from.extend(picked_from);
            }
        }
        Ok(index)
    }

    /// Touched paths of `id`'s first-parent diff, cached.
    fn paths_of(&mut self, id: ObjectId, parent: ObjectId) -> Result<BTreeSet<BString>> {
        if let Some(paths) = self.paths_cache.get(&id) {
            return Ok(paths.clone());
        }
        let paths = patchid::touched_paths(&self.repo, parent, id)?;
        self.paths_cache.insert(id, paths.clone());
        Ok(paths)
    }

    /// Patch-id of `id`'s first-parent diff, cached.
    fn patch_id_of(&mut self, id: ObjectId, parent: ObjectId) -> Result<PatchId> {
        if let Some(patch_id) = self.patch_id_cache.get(&id) {
            return Ok(patch_id.clone());
        }
        let patch_id = patchid::patch_id(&self.repo, parent, id)?;
        self.patch_id_cache.insert(id, patch_id.clone());
        Ok(patch_id)
    }
}

/// Where the branch's own work starts: first parents of own non-merge commits that are not own themselves. A
/// cherry-picked twin is made after its original, so it never sits behind these — hiding them bounds the twin walk
/// without hiding twins that came back into the branch through a merge.
fn fork_points(own: &[CommitInfo]) -> Vec<ObjectId> {
    let own_ids: HashSet<ObjectId> = own.iter().map(|commit| commit.id).collect();
    let mut forks: Vec<ObjectId> = own
        .iter()
        .filter(|commit| !commit.is_merge())
        .filter_map(|commit| commit.parents.first().copied())
        .filter(|parent| !own_ids.contains(parent))
        .collect();
    forks.sort_unstable();
    forks.dedup();
    forks
}

/// `commits` reordered parents before children: a post-order walk along the parent links inside the set, started
/// from the oldest commit so unrelated lines keep their commit-time order. Commit time alone is no replay order — a
/// child committed with an older clock than its parent would be picked first.
fn parents_first(commits: &[CommitInfo]) -> Vec<&CommitInfo> {
    let by_id: HashMap<ObjectId, &CommitInfo> =
        commits.iter().map(|commit| (commit.id, commit)).collect();
    let mut ordered = Vec::with_capacity(commits.len());
    let mut done: HashSet<ObjectId> = HashSet::with_capacity(commits.len());
    for start in commits.iter().rev() {
        let mut stack = vec![(start, false)];
        while let Some((commit, expanded)) = stack.pop() {
            if done.contains(&commit.id) {
                continue;
            }
            if expanded {
                done.insert(commit.id);
                ordered.push(commit);
                continue;
            }
            stack.push((commit, true));
            for parent in commit.parents.iter().rev() {
                if let Some(parent) = by_id.get(parent)
                    && !done.contains(&parent.id)
                {
                    stack.push((*parent, false));
                }
            }
        }
    }
    ordered
}

/// The commit a reflog line re-anchored the branch on: the `onto` sha of a rebase finish, the new tip of a reset or
/// `branch -f`; `None` for every other line.
fn reanchor_target(message: &str, new_oid: ObjectId) -> Option<ObjectId> {
    let rebase = message
        .strip_prefix("rebase (finish): ")
        .or_else(|| message.strip_prefix("pull --rebase (finish): "));
    if let Some(rest) = rebase {
        return full_sha(rest.rsplit_once(" onto ")?.1.trim());
    }
    let is_reset =
        message.starts_with("reset: moving to ") || message.starts_with("branch: Reset to ");
    is_reset.then_some(new_oid)
}

/// A reflog line's hex object id.
fn reflog_oid(hex: &BStr) -> Result<ObjectId> {
    ObjectId::from_hex(hex)
        .map_err(|err| Error::Invariant(format!("reflog object id {hex}: {err}")))
}

/// The ref a reflog origin names: the text before its first revision operator (`^`, `~`, `:`, `@{`), so `main^0`
/// (an IDE's start point) and `claude/a~1` resolve through `main` and `claude/a`; a plain name is unchanged.
fn origin_ref(origin: &str) -> &str {
    let operator = origin
        .find(['^', '~', ':'])
        .into_iter()
        .chain(origin.find("@{"))
        .min()
        .unwrap_or(origin.len());
    &origin[..operator]
}

/// A 40-hex string as an object id.
fn full_sha(text: &str) -> Option<ObjectId> {
    let is_hex = text.len() == 40 && text.bytes().all(|byte| byte.is_ascii_hexdigit());
    is_hex
        .then(|| ObjectId::from_hex(text.as_bytes()).ok())
        .flatten()
}

/// The commit a `(cherry picked from commit <sha>)` trailer names; `None` without a full-sha trailer.
fn pick_trailer(message: &BStr) -> Option<ObjectId> {
    message.lines().find_map(|line| {
        let line = line.to_str().ok()?.trim();
        full_sha(
            line.strip_prefix("(cherry picked from commit ")?
                .strip_suffix(')')?,
        )
    })
}

/// Dirty count and filtered untracked list of one checkout (a separate handle: status needs the worktree).
pub fn tree_state(worktree: &Path) -> Result<TreeState> {
    let repo = gix::open(worktree).git()?;
    let mut iter = repo
        .status(gix::progress::Discard)
        .git()?
        .untracked_files(gix::status::UntrackedFiles::Files)
        .into_iter(Vec::new())
        .git()?;
    let mut dirty = BTreeSet::new();
    let mut untracked = Vec::new();
    for item in iter.by_ref() {
        let item = item.git()?;
        match &item {
            gix::status::Item::TreeIndex(change) => {
                dirty.insert(change.location().to_owned());
            }
            gix::status::Item::IndexWorktree(entry) => match entry.summary() {
                Some(gix::status::index_worktree::iter::Summary::Added) => {
                    let path = entry.rela_path();
                    if !is_infrastructure(path) {
                        untracked.push(path.to_str_lossy().into_owned());
                    }
                }
                Some(_) => {
                    dirty.insert(entry.rela_path().to_owned());
                }
                None => {}
            },
        }
    }
    untracked.sort();
    Ok(TreeState {
        dirty: dirty.len(),
        untracked,
    })
}

/// The exclusion list shared by the untracked column and the remove gate.
#[must_use]
pub fn is_infrastructure(path: &BStr) -> bool {
    let text = path.to_str_lossy();
    if UNTRACKED_EXCLUDED_PREFIXES
        .iter()
        .any(|prefix| text.starts_with(prefix))
    {
        return true;
    }
    if UNTRACKED_EXCLUDED_NAMES
        .iter()
        .any(|name| text.as_ref() == *name || text.starts_with(&format!("{name}/")))
    {
        return true;
    }
    text.rsplit('/').next() == Some(".DS_Store")
}

/// In-memory commit of `worktree`'s HEAD tree plus its tracked modifications; `None` when clean.
pub fn snapshot_commit(context: &Context, worktree: &Path) -> Result<Option<ObjectId>> {
    let checkout = gix::open(worktree).git()?;
    let head = checkout.head_id().git()?.detach();
    let head_tree = context
        .repo
        .find_commit(head)
        .git()?
        .tree_id()
        .git()?
        .detach();
    let iter = checkout
        .status(gix::progress::Discard)
        .git()?
        .untracked_files(gix::status::UntrackedFiles::None)
        .into_iter(Vec::new())
        .git()?;
    let mut paths = BTreeSet::new();
    for item in iter {
        let item = item.git()?;
        match &item {
            gix::status::Item::TreeIndex(change) => {
                paths.insert(change.location().to_owned());
            }
            gix::status::Item::IndexWorktree(entry) => {
                if entry.summary().is_some() {
                    paths.insert(entry.rela_path().to_owned());
                    if let gix::status::index_worktree::Item::Rewrite { source, .. } = entry {
                        paths.insert(source.rela_path().to_owned());
                    }
                }
            }
        }
    }
    if paths.is_empty() {
        return Ok(None);
    }
    let mut editor = context.repo.edit_tree(head_tree).git()?;
    for path in &paths {
        let on_disk = worktree.join(path.to_path_lossy());
        match std::fs::metadata(&on_disk) {
            Ok(meta) if meta.is_file() => {
                let bytes = std::fs::read(&on_disk).map_err(Error::io(&on_disk))?;
                let blob = context.repo.write_blob(bytes).git()?.detach();
                editor
                    .upsert(path.as_bstr(), entry_kind(&meta), blob)
                    .git()?;
            }
            Ok(_) => {}
            Err(_) => {
                editor.remove(path.as_bstr()).git()?;
            }
        }
    }
    let tree = editor.write().git()?.detach();
    if tree == head_tree {
        return Ok(None);
    }
    let signature = gix::actor::SignatureRef {
        name: b"agentwt".as_bstr(),
        email: b"agentwt@local".as_bstr(),
        time: "0 +0000",
    };
    let commit = context
        .repo
        .new_commit_as(signature, signature, "agentwt dirty snapshot", tree, [head])
        .git()?;
    Ok(Some(commit.id))
}

#[cfg(unix)]
fn entry_kind(meta: &std::fs::Metadata) -> EntryKind {
    use std::os::unix::fs::PermissionsExt;
    if meta.permissions().mode() & 0o111 != 0 {
        EntryKind::BlobExecutable
    } else {
        EntryKind::Blob
    }
}

#[cfg(not(unix))]
fn entry_kind(_meta: &std::fs::Metadata) -> EntryKind {
    EntryKind::Blob
}

/// Unix seconds → `YYYY-MM-DDTHH:MM:SSZ` (Howard Hinnant's civil-from-days; no date crate needed).
#[must_use]
pub fn rfc3339_utc(seconds: i64) -> String {
    let days = seconds.div_euclid(86_400);
    let rem = seconds.rem_euclid(86_400);
    let z = days + 719_468;
    let era = z.div_euclid(146_097);
    let doe = z.rem_euclid(146_097);
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let day = doy - (153 * mp + 2) / 5 + 1;
    let month = if mp < 10 { mp + 3 } else { mp - 9 };
    let year = yoe + era * 400 + i64::from(month <= 2);
    format!(
        "{year:04}-{month:02}-{day:02}T{:02}:{:02}:{:02}Z",
        rem / 3600,
        (rem % 3600) / 60,
        rem % 60
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn infrastructure_filter() {
        let cases: [(&str, bool); 7] = [
            (".idea/workspace.xml", true),
            (".claude/settings.local.json", true),
            (".claude-worktree", true),
            ("docs/.DS_Store", true),
            (".cell-pids", true),
            ("src/main.rs", false),
            (".claude-worktree.bak", false),
        ];
        for (path, expected) in cases {
            assert_eq!(is_infrastructure(path.into()), expected, "{path}");
        }
    }

    #[test]
    fn rfc3339_epoch_and_leap() {
        let cases: [(i64, &str); 3] = [
            (0, "1970-01-01T00:00:00Z"),
            (951_782_400, "2000-02-29T00:00:00Z"),
            (1_790_289_427, "2026-09-24T22:37:07Z"),
        ];
        for (seconds, expected) in cases {
            assert_eq!(rfc3339_utc(seconds), expected, "{seconds}");
        }
    }

    #[test]
    fn origin_ref_cuts_revision_operators() {
        let cases: [(&str, &str); 7] = [
            ("main", "main"),
            ("main^0", "main"),
            ("claude/a^", "claude/a"),
            ("claude/a~1", "claude/a"),
            ("develop@{1}", "develop"),
            ("main:path", "main"),
            ("^0", ""),
        ];
        for (origin, expected) in cases {
            assert_eq!(origin_ref(origin), expected, "{origin}");
        }
    }

    const SHA_A: &str = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
    const SHA_B: &str = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";

    fn oid(hex: &str) -> ObjectId {
        ObjectId::from_hex(hex.as_bytes()).unwrap_or_else(|_| ObjectId::null(gix::hash::Kind::Sha1))
    }

    #[test]
    fn reanchor_target_lines() {
        let new_oid = oid(SHA_B);
        let rebase = format!("rebase (finish): refs/heads/claude/x onto {SHA_A}");
        let pull = format!("pull --rebase (finish): refs/heads/claude/x onto {SHA_A}");
        let cases: [(&str, &str, Option<ObjectId>); 7] = [
            ("rebase finish", &rebase, Some(oid(SHA_A))),
            ("pull rebase finish", &pull, Some(oid(SHA_A))),
            ("reset", "reset: moving to main", Some(new_oid)),
            ("branch -f", "branch: Reset to develop", Some(new_oid)),
            ("amend", "commit (amend): x", None),
            ("fast-forward", "merge refs/heads/main: Fast-forward", None),
            ("creation", "branch: Created from main", None),
        ];
        for (label, message, expected) in cases {
            assert_eq!(reanchor_target(message, new_oid), expected, "{label}");
        }
    }

    fn info(id: &str, parents: &[&str]) -> CommitInfo {
        CommitInfo {
            id: oid(id),
            parents: parents.iter().map(|parent| oid(parent)).collect(),
            subject: String::new(),
            committed_at: String::new(),
        }
    }

    #[test]
    fn fork_points_shapes() {
        let fork = "1111111111111111111111111111111111111111";
        let other_fork = "2222222222222222222222222222222222222222";
        let x = "3333333333333333333333333333333333333333";
        let y = "4444444444444444444444444444444444444444";
        let merge = "5555555555555555555555555555555555555555";
        // (label, own commits newest first, expected forks sorted)
        let cases: [(&str, Vec<CommitInfo>, Vec<ObjectId>); 4] = [
            (
                "linear",
                vec![info(y, &[x]), info(x, &[fork])],
                vec![oid(fork)],
            ),
            (
                "own merge skipped",
                vec![
                    info(merge, &[y, other_fork]),
                    info(y, &[x]),
                    info(x, &[fork]),
                ],
                vec![oid(fork)],
            ),
            (
                "two lines",
                vec![info(y, &[other_fork]), info(x, &[fork])],
                vec![oid(fork), oid(other_fork)],
            ),
            (
                "duplicates collapse",
                vec![info(y, &[fork]), info(x, &[fork])],
                vec![oid(fork)],
            ),
        ];
        for (label, own, expected) in cases {
            assert_eq!(fork_points(&own), expected, "{label}");
        }
    }

    #[test]
    fn parents_first_shapes() {
        let fork = "1111111111111111111111111111111111111111";
        let first = "3333333333333333333333333333333333333333";
        let second = "4444444444444444444444444444444444444444";
        let merge = "5555555555555555555555555555555555555555";
        let side = "6666666666666666666666666666666666666666";
        // (label, commits in walk order, expected ids parents first)
        let cases: [(&str, Vec<CommitInfo>, Vec<ObjectId>); 3] = [
            (
                "linear",
                vec![info(second, &[first]), info(first, &[fork])],
                vec![oid(first), oid(second)],
            ),
            (
                "skewed clock: the walk yields a parent before its children",
                vec![
                    info(merge, &[second, side]),
                    info(first, &[fork]),
                    info(second, &[first]),
                    info(side, &[first]),
                ],
                vec![oid(first), oid(side), oid(second), oid(merge)],
            ),
            ("empty", vec![], vec![]),
        ];
        for (label, commits, expected) in cases {
            let ordered: Vec<ObjectId> = parents_first(&commits)
                .iter()
                .map(|commit| commit.id)
                .collect();
            assert_eq!(ordered, expected, "{label}");
        }
    }

    #[test]
    fn pick_trailer_lines() {
        let full = format!("subject\n\nbody\n\n(cherry picked from commit {SHA_A})\n");
        let among = format!(
            "subject\n\nSigned-off-by: a\n(cherry picked from commit {SHA_A})\nX-Other: y\n"
        );
        let cases: [(&str, String, Option<ObjectId>); 4] = [
            ("full sha", full, Some(oid(SHA_A))),
            (
                "short sha",
                "subject\n\n(cherry picked from commit aaaaaaa)\n".to_owned(),
                None,
            ),
            ("no trailer", "subject\n\nbody\n".to_owned(), None),
            ("among trailers", among, Some(oid(SHA_A))),
        ];
        for (label, message, expected) in cases {
            assert_eq!(
                pick_trailer(message.as_bytes().as_bstr()),
                expected,
                "{label}"
            );
        }
    }
}
