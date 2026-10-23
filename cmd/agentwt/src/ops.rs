//! The three mutating commands, all plan-then-execute through git children ([`Git`]); nothing else in the crate
//! changes a repository.
//!
//! `remove`: [`RemovePlan::steps`] decides every step before anything runs; [`Git::remove`] executes them
//! (`git worktree unlock` best effort, `git worktree remove --force`, optionally `git branch -D`) and
//! [`RemovePlan::preview`] renders them for `--dry-run`. Every git child runs in the main checkout
//! (`Document.repo.path`), never in the worktree being removed. A refused step is a `skipped` line in `remove`
//! (exit 0, a batch continues) and a `Refusal` under `--dry-run` (exit 3); every unexpected git failure a
//! `GitCommand` (its status forwarded).
//!
//! `pick`: [`Git::plan_pick`] gathers the facts (commit and target tip resolved to full shas, parent count,
//! containment through [`Context::holds`], the target's checkout and its cleanliness) and [`PickPlan::step`] decides
//! the one step; [`Git::pick`] executes it on the captured shas. A checked-out target is picked in its own worktree
//! (`git cherry-pick -x`, aborted on failure); any other target gets `git merge-tree --write-tree` against the
//! commit's parent, `git commit-tree` with the original author and a `(cherry picked from commit <sha>)` trailer, and
//! a compare-and-swap `git update-ref` from the planned tip. Refusals are a `skipped` outcome, conflicts a
//! `conflict` outcome — both exit 3 in the binary; an unknown commit or branch is an error (exit 3).
//!
//! `probe`: [`Git::probe`] answers for many commits at once what `pick` would do with each — the same refusals
//! ([`pick_refusal`]) over one gix [`Context`], then the merge-tree probe, which writes objects and nothing else —
//! and exits 0 whatever the outcomes; an unknown commit or branch is an error (exit 3).
//!
//! `forward`: [`Git::plan_forward`] resolves the commit in the caller's checkout (so `HEAD` is the caller's) and the
//! target's tip, asks whether the commit holds that tip, and looks up the target's checkout; [`ForwardPlan::step`]
//! decides the one step; [`Git::forward`] executes it. A checked-out target moves in its own clean worktree with
//! `git merge --ff-only`, so ref, index and files move together; any other target moves by a compare-and-swap
//! `git update-ref` from the planned tip. A commit that does not hold the tip, a dirty checkout and a move git
//! refuses in the checkout are a `skipped` outcome (exit 3).
//! There is deliberately no merge commit or sync here — `pick` places one commit, `forward` only fast-forwards; the
//! binary is no git client.

use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};

use gix::ObjectId;

use crate::error::{Error, Refusal, Result};
use crate::gitstate::{Candidate, Context};
use crate::wire::{
    FORWARD_SCHEMA, Forward, ForwardOutcome, OpLine, OpOutcome, OpResult, PICK_SCHEMA,
    PROBE_SCHEMA, Pick, PickOutcome, Probe, ProbeEntry, ProbeOutcome,
};

/// Runs git children under an interrupt flag.
#[derive(Debug, Clone)]
pub struct Git {
    interrupted: Arc<AtomicBool>,
}

/// Captured child result.
#[derive(Debug)]
struct Child {
    success: bool,
    status: i32,
    stdout: String,
    stderr: String,
}

impl Git {
    /// `interrupted` is set by the binary's signal handler.
    #[must_use]
    pub fn new(interrupted: Arc<AtomicBool>) -> Self {
        Git { interrupted }
    }

    fn run(&self, dir: &Path, args: &[&str]) -> Result<Child> {
        self.run_env(dir, args, &[])
    }

    /// `run` with extra environment variables for the child.
    fn run_env(&self, dir: &Path, args: &[&str], env: &[(&str, &str)]) -> Result<Child> {
        if self.interrupted.load(Ordering::SeqCst) {
            return Err(Error::Interrupted { signal: 2 });
        }
        let output = Command::new("git")
            .current_dir(dir)
            .arg("-C")
            .arg(dir)
            .args(args)
            .envs(env.iter().copied())
            .output()
            .map_err(Error::io(PathBuf::from("git")))?;
        Ok(Child {
            success: output.status.success(),
            status: output.status.code().unwrap_or(-1),
            stdout: String::from_utf8_lossy(&output.stdout).trim().to_owned(),
            stderr: String::from_utf8_lossy(&output.stderr).trim().to_owned(),
        })
    }

    /// A step that must succeed.
    fn must(&self, dir: &Path, args: &[&str]) -> Result<String> {
        self.must_env(dir, args, &[])
    }

    /// `must` with extra environment variables for the child.
    fn must_env(&self, dir: &Path, args: &[&str], env: &[(&str, &str)]) -> Result<String> {
        let child = self.run_env(dir, args, env)?;
        if child.success {
            return Ok(child.stdout);
        }
        Err(failed(args, child))
    }

    /// Executes a removal plan's steps; refusals are `skipped` lines so a batch continues. `main_checkout` is the
    /// status document's `repo.path`.
    pub fn remove(&self, main_checkout: &Path, plan: &RemovePlan) -> Result<OpResult> {
        let branch = &plan.branch;
        let mut lines = Vec::new();
        for step in plan.steps() {
            match step {
                RemoveStep::RemoveWorktree(path) => {
                    let path = path.display().to_string();
                    let _ = self.run(main_checkout, &["worktree", "unlock", &path]);
                    self.must(main_checkout, &["worktree", "remove", "--force", &path])?;
                    lines.push(line(
                        branch,
                        OpOutcome::Removed,
                        format!("removed {path} ({branch})"),
                    ));
                }
                RemoveStep::DeleteBranch => {
                    self.must(main_checkout, &["branch", "-D", branch])?;
                    lines.push(line(
                        branch,
                        OpOutcome::BranchDeleted,
                        format!("deleted branch {branch}"),
                    ));
                }
                RemoveStep::Skip(refusal) => {
                    lines.push(line(branch, OpOutcome::Skipped, refusal.to_string()));
                }
            }
        }
        Ok(OpResult {
            operation: "remove".to_owned(),
            lines,
        })
    }
}

/// One branch to remove, with the safety verdict already computed.
#[derive(Debug, Clone)]
pub struct RemovePlan {
    /// The branch.
    pub branch: String,
    /// Its worktree, if checked out.
    pub worktree: Option<PathBuf>,
    /// The status document's predicate.
    pub safe: bool,
    /// Its reasons when unsafe.
    pub unsafe_reasons: Vec<String>,
    /// `--force`.
    pub force: bool,
    /// `--delete-branch`.
    pub delete_branch: bool,
}

impl RemovePlan {
    /// Every step `remove` takes for this plan, in order — the one decision `remove` and `--dry-run` share.
    #[must_use]
    pub fn steps(&self) -> Vec<RemoveStep> {
        let allowed = self.safe || self.force;
        let reasons = self.unsafe_reasons.join(", ");
        let mut steps = Vec::new();
        match &self.worktree {
            Some(path) if allowed => steps.push(RemoveStep::RemoveWorktree(path.clone())),
            Some(path) => steps.push(RemoveStep::Skip(Refusal::Unsafe {
                branch: self.branch.clone(),
                path: path.display().to_string(),
                reasons: reasons.clone(),
            })),
            None if !self.delete_branch => {
                steps.push(RemoveStep::Skip(Refusal::NoWorktree(self.branch.clone())));
            }
            None => {}
        }
        if self.delete_branch {
            if allowed {
                steps.push(RemoveStep::DeleteBranch);
            } else {
                steps.push(RemoveStep::Skip(Refusal::UnsafeBranch {
                    branch: self.branch.clone(),
                    reasons,
                }));
            }
        }
        steps
    }

    /// `--dry-run`: the lines `remove` would print, touching nothing; the first step that would be skipped is the
    /// refusal instead.
    pub fn preview(&self) -> Result<OpResult> {
        let branch = &self.branch;
        let mut lines = Vec::new();
        for step in self.steps() {
            match step {
                RemoveStep::RemoveWorktree(path) => lines.push(line(
                    branch,
                    OpOutcome::WouldRemove,
                    format!("would remove {} ({branch})", path.display()),
                )),
                RemoveStep::DeleteBranch => lines.push(line(
                    branch,
                    OpOutcome::WouldDeleteBranch,
                    format!("would delete branch {branch}"),
                )),
                RemoveStep::Skip(refusal) => return Err(refusal.into()),
            }
        }
        Ok(OpResult {
            operation: "remove".to_owned(),
            lines,
        })
    }
}

/// One step of a removal, decided before anything runs.
#[derive(Debug, PartialEq, Eq)]
pub enum RemoveStep {
    /// Unlock (best effort) and remove this worktree.
    RemoveWorktree(PathBuf),
    /// Delete the branch.
    DeleteBranch,
    /// A refused step; its `Display` is the skip line.
    Skip(Refusal),
}

impl Git {
    /// Gathers every fact `pick` decides from, changing nothing; `main_checkout` is the repository's main checkout.
    /// An unknown commit or branch is a refusal error.
    pub fn plan_pick(&self, main_checkout: &Path, commit: &str, onto: &str) -> Result<PickPlan> {
        let sha = self
            .resolve(main_checkout, &format!("{commit}^{{commit}}"))?
            .ok_or_else(|| Refusal::UnknownCommit(commit.to_owned()))?;
        let onto_tip = self
            .resolve(main_checkout, &format!("refs/heads/{onto}"))?
            .ok_or_else(|| Refusal::UnknownBranch(onto.to_owned()))?;
        let parents = self.parent_count(main_checkout, &sha)?;
        let mut context = Context::open(main_checkout, onto, &[onto.to_owned()])?;
        let target = candidate(&context, onto)?;
        let contained = parents == 1 && context.holds(object_id(&sha)?, &target)?;
        Ok(PickPlan {
            commit: sha,
            onto: onto.to_owned(),
            onto_tip,
            parents,
            contained,
            checkout: self.target_checkout(main_checkout, onto)?,
        })
    }

    /// Executes a pick plan's one step on the planned shas.
    pub fn pick(&self, main_checkout: &Path, plan: &PickPlan) -> Result<PickReport> {
        let result = match plan.step() {
            PickStep::Skip(refusal) => PickResult::Skipped(refusal),
            PickStep::CherryPick(worktree) => self.cherry_pick(&worktree, &plan.commit)?,
            PickStep::CommitTree => {
                match self.merge_tree(main_checkout, &plan.onto_tip, &plan.commit)? {
                    MergeTree::Clean(tree) => PickResult::Picked {
                        new_commit: self.commit_tree(main_checkout, plan, &tree)?,
                    },
                    MergeTree::Conflict(reason) => PickResult::Conflict { reason },
                }
            }
        };
        Ok(PickReport {
            commit: plan.commit.clone(),
            onto: plan.onto.clone(),
            result,
        })
    }

    /// Decides for every commit what a pick onto `onto`'s tip would do, in argument order: the pick refusals, then
    /// the merge-tree probe. One gix context answers every containment question; no ref, index or worktree moves.
    pub fn probe(
        &self,
        main_checkout: &Path,
        commits: &[String],
        onto: &str,
    ) -> Result<ProbeReport> {
        let onto_tip = self
            .resolve(main_checkout, &format!("refs/heads/{onto}"))?
            .ok_or_else(|| Refusal::UnknownBranch(onto.to_owned()))?;
        let mut context = Context::open(main_checkout, onto, &[onto.to_owned()])?;
        let target = candidate(&context, onto)?;
        let mut lines = Vec::with_capacity(commits.len());
        for commit in commits {
            let sha = self
                .resolve(main_checkout, &format!("{commit}^{{commit}}"))?
                .ok_or_else(|| Refusal::UnknownCommit(commit.clone()))?;
            let parents = self.parent_count(main_checkout, &sha)?;
            let contained = parents == 1 && context.holds(object_id(&sha)?, &target)?;
            let result = match pick_refusal(parents, contained, onto) {
                Some(refusal) => ProbeResult::Skipped(refusal),
                None => match self.merge_tree(main_checkout, &onto_tip, &sha)? {
                    MergeTree::Clean(_) => ProbeResult::Applies,
                    MergeTree::Conflict(reason) => ProbeResult::Conflict { reason },
                },
            };
            lines.push(ProbeLine {
                commit: sha,
                result,
            });
        }
        Ok(ProbeReport {
            onto: onto.to_owned(),
            onto_tip,
            lines,
        })
    }

    /// The commit's parent count.
    fn parent_count(&self, main_checkout: &Path, sha: &str) -> Result<usize> {
        Ok(self
            .must(main_checkout, &["rev-list", "--parents", "-n", "1", sha])?
            .split_whitespace()
            .count()
            .saturating_sub(1))
    }

    /// The worktree `branch` is checked out in and whether it has no tracked changes; `None` when checked out
    /// nowhere.
    fn target_checkout(
        &self,
        main_checkout: &Path,
        branch: &str,
    ) -> Result<Option<TargetCheckout>> {
        let list = self.must(main_checkout, &["worktree", "list", "--porcelain"])?;
        let Some(path) = checked_out_at(&list, branch) else {
            return Ok(None);
        };
        let status = self.must(&path, &["status", "--porcelain", "--untracked-files=no"])?;
        Ok(Some(TargetCheckout {
            clean: status.is_empty(),
            path,
        }))
    }

    /// Gathers every fact `forward` decides from, changing nothing. `repo` is the caller's checkout — the commit
    /// resolves there, so `HEAD` is the caller's; `main_checkout` is the repository's main checkout. An unknown
    /// commit or branch is a refusal error.
    pub fn plan_forward(
        &self,
        repo: &Path,
        main_checkout: &Path,
        commit: &str,
        onto: &str,
    ) -> Result<ForwardPlan> {
        let sha = self
            .resolve(repo, &format!("{commit}^{{commit}}"))?
            .ok_or_else(|| Refusal::UnknownCommit(commit.to_owned()))?;
        let onto_tip = self
            .resolve(main_checkout, &format!("refs/heads/{onto}"))?
            .ok_or_else(|| Refusal::UnknownBranch(onto.to_owned()))?;
        let args = ["merge-base", "--is-ancestor", &onto_tip, &sha];
        let ancestor = self.run(main_checkout, &args)?;
        let holds_tip = match ancestor.status {
            0 => true,
            1 => false,
            _ => return Err(failed(&args, ancestor)),
        };
        Ok(ForwardPlan {
            checkout: self.target_checkout(main_checkout, onto)?,
            commit: sha,
            onto: onto.to_owned(),
            onto_tip,
            holds_tip,
        })
    }

    /// Executes a forward plan's one step on the planned shas.
    pub fn forward(&self, main_checkout: &Path, plan: &ForwardPlan) -> Result<ForwardReport> {
        let result = match plan.step() {
            ForwardStep::Skip(refusal) => ForwardResult::Skipped(refusal.to_string()),
            ForwardStep::Nothing => ForwardResult::Unchanged,
            ForwardStep::MergeFfOnly(worktree) => {
                let merge =
                    self.run(&worktree, &["merge", "--ff-only", "--quiet", &plan.commit])?;
                if merge.success {
                    ForwardResult::Forwarded
                } else {
                    let line = merge
                        .stderr
                        .lines()
                        .next()
                        .unwrap_or("merge --ff-only failed");
                    ForwardResult::Skipped(line.to_owned())
                }
            }
            ForwardStep::UpdateRef => {
                let reflog = format!("agentwt forward: {}", short(&plan.commit));
                self.must(
                    main_checkout,
                    &[
                        "update-ref",
                        "-m",
                        &reflog,
                        &format!("refs/heads/{}", plan.onto),
                        &plan.commit,
                        &plan.onto_tip,
                    ],
                )?;
                ForwardResult::Forwarded
            }
        };
        Ok(ForwardReport {
            onto: plan.onto.clone(),
            from: plan.onto_tip.clone(),
            to: plan.commit.clone(),
            result,
        })
    }

    /// The full sha `rev` names, `None` when it names nothing.
    fn resolve(&self, dir: &Path, rev: &str) -> Result<Option<String>> {
        let child = self.run(
            dir,
            &["rev-parse", "--verify", "--quiet", "--end-of-options", rev],
        )?;
        Ok(child.success.then_some(child.stdout))
    }

    /// Three-way merge of the commit onto the tip, base its parent: objects only, no ref, index or worktree. The
    /// attributes come from the tip's tree, so a cwd without a worktree (a container cell) still applies merge drivers.
    fn merge_tree(&self, dir: &Path, onto_tip: &str, commit: &str) -> Result<MergeTree> {
        let merge_base = format!("--merge-base={commit}^");
        let args = [
            "merge-tree",
            "--write-tree",
            "--name-only",
            "--no-messages",
            &merge_base,
            onto_tip,
            commit,
        ];
        let child = self.run_env(dir, &args, &[("GIT_ATTR_SOURCE", onto_tip)])?;
        let mut lines = child.stdout.lines();
        let tree = lines.next().unwrap_or_default().to_owned();
        match child.status {
            0 => Ok(MergeTree::Clean(tree)),
            1 => Ok(MergeTree::Conflict(conflict_reason(lines))),
            _ => Err(failed(&args, child)),
        }
    }

    /// `git cherry-pick -x` in the target's worktree; a failed pick is aborted and reported as a conflict.
    fn cherry_pick(&self, worktree: &Path, sha: &str) -> Result<PickResult> {
        let pick = self.run(worktree, &["cherry-pick", "-x", sha])?;
        if pick.success {
            let new_commit = self.must(worktree, &["rev-parse", "HEAD"])?;
            return Ok(PickResult::Picked { new_commit });
        }
        let unmerged = self.run(worktree, &["diff", "--name-only", "--diff-filter=U"])?;
        let _ = self.run(worktree, &["cherry-pick", "--abort"])?;
        let reason = if unmerged.stdout.is_empty() {
            pick.stderr
                .lines()
                .next()
                .unwrap_or("cherry-pick failed")
                .to_owned()
        } else {
            conflict_reason(unmerged.stdout.lines())
        };
        Ok(PickResult::Conflict { reason })
    }

    /// Commits `tree` on the planned tip with the original author and message plus the pick trailer, then moves the
    /// branch from the planned tip to it (compare-and-swap); returns the new commit.
    fn commit_tree(&self, dir: &Path, plan: &PickPlan, tree: &str) -> Result<String> {
        let sha = plan.commit.as_str();
        let author = self.must(
            dir,
            &[
                "show",
                "-s",
                "--date=raw",
                "--format=%an%x00%ae%x00%ad",
                sha,
            ],
        )?;
        let mut fields = author.split('\0');
        let (Some(name), Some(email), Some(date)) = (fields.next(), fields.next(), fields.next())
        else {
            return Err(Error::Invariant(format!(
                "author of {sha} unreadable: {author}"
            )));
        };
        let body = self.must(dir, &["show", "-s", "--format=%B", sha])?;
        let message = pick_message(&body, sha);
        let date = format!("@{date}");
        let env = [
            ("GIT_AUTHOR_NAME", name),
            ("GIT_AUTHOR_EMAIL", email),
            ("GIT_AUTHOR_DATE", date.as_str()),
        ];
        let new_commit = self.must_env(
            dir,
            &["commit-tree", tree, "-p", &plan.onto_tip, "-m", &message],
            &env,
        )?;
        let reflog = format!("agentwt pick: {}", short(sha));
        self.must(
            dir,
            &[
                "update-ref",
                "-m",
                &reflog,
                &format!("refs/heads/{}", plan.onto),
                &new_commit,
                &plan.onto_tip,
            ],
        )?;
        Ok(new_commit)
    }
}

/// The candidate `onto` names in an opened context; a refusal when the branch is not tracked there.
fn candidate(context: &Context, onto: &str) -> Result<Candidate> {
    context
        .candidates()
        .iter()
        .find(|candidate| candidate.name == onto)
        .cloned()
        .ok_or_else(|| Refusal::UnknownBranch(onto.to_owned()).into())
}

/// A resolved full sha as an object id.
fn object_id(sha: &str) -> Result<ObjectId> {
    ObjectId::from_hex(sha.as_bytes())
        .map_err(|err| Error::Invariant(format!("commit id {sha}: {err}")))
}

/// The refusal `pick` and `probe` share before any merge: root and merge commits, and a commit the target holds.
fn pick_refusal(parents: usize, contained: bool, onto: &str) -> Option<Refusal> {
    if parents == 0 {
        return Some(Refusal::RootCommit);
    }
    if parents > 1 {
        return Some(Refusal::MergeCommit);
    }
    if contained {
        return Some(Refusal::AlreadyOn(onto.to_owned()));
    }
    None
}

/// The worktree `git worktree list --porcelain` shows `branch` checked out in, if any.
fn checked_out_at(porcelain: &str, branch: &str) -> Option<PathBuf> {
    let want = format!("branch refs/heads/{branch}");
    porcelain.split("\n\n").find_map(|block| {
        let mut lines = block.lines();
        let path = lines.next()?.strip_prefix("worktree ")?;
        lines.any(|line| line == want).then(|| PathBuf::from(path))
    })
}

/// The original message, a blank line, and the trailer `git cherry-pick -x` writes.
fn pick_message(body: &str, sha: &str) -> String {
    format!("{}\n\n(cherry picked from commit {sha})\n", body.trim_end())
}

/// `conflicts in a, b` from conflicted path lines (deduplicated, order kept).
fn conflict_reason<'a>(paths: impl Iterator<Item = &'a str>) -> String {
    let mut seen: Vec<&str> = Vec::new();
    for path in paths.map(str::trim).filter(|path| !path.is_empty()) {
        if !seen.contains(&path) {
            seen.push(path);
        }
    }
    if seen.is_empty() {
        "conflicts".to_owned()
    } else {
        format!("conflicts in {}", seen.join(", "))
    }
}

/// Eight-character prefix, the short sha everywhere in this crate.
fn short(sha: &str) -> &str {
    sha.get(..8).unwrap_or(sha)
}

/// A non-zero git child as an error.
fn failed(args: &[&str], child: Child) -> Error {
    Error::GitCommand {
        command: args.join(" "),
        status: child.status,
        stderr: child.stderr,
    }
}

/// Result of the merge-tree probe.
enum MergeTree {
    /// The merged tree's id.
    Clean(String),
    /// The conflict reason.
    Conflict(String),
}

/// One commit to pick onto one branch, every fact gathered before anything runs.
#[derive(Debug, Clone)]
pub struct PickPlan {
    /// The commit, full sha.
    pub commit: String,
    /// The target branch.
    pub onto: String,
    /// The target's tip when planned, full sha — the merge side and the compare-and-swap old value.
    pub onto_tip: String,
    /// The commit's parent count.
    pub parents: usize,
    /// The target already holds the commit (asked for single-parent commits only).
    pub contained: bool,
    /// The target's worktree; `None` when not checked out.
    pub checkout: Option<TargetCheckout>,
}

/// The worktree the target is checked out in.
#[derive(Debug, Clone)]
pub struct TargetCheckout {
    /// Its path.
    pub path: PathBuf,
    /// No tracked changes, staged or unstaged.
    pub clean: bool,
}

impl PickPlan {
    /// The one step `pick` takes for this plan.
    #[must_use]
    pub fn step(&self) -> PickStep {
        if let Some(refusal) = pick_refusal(self.parents, self.contained, &self.onto) {
            return PickStep::Skip(refusal);
        }
        match &self.checkout {
            Some(checkout) if checkout.clean => PickStep::CherryPick(checkout.path.clone()),
            Some(checkout) => PickStep::Skip(Refusal::DirtyCheckout {
                branch: self.onto.clone(),
                path: checkout.path.display().to_string(),
            }),
            None => PickStep::CommitTree,
        }
    }
}

/// The one step of a pick, decided before anything runs.
#[derive(Debug, PartialEq, Eq)]
pub enum PickStep {
    /// Refused; its `Display` is the reason.
    Skip(Refusal),
    /// `git cherry-pick -x` in this worktree, where the target is checked out.
    CherryPick(PathBuf),
    /// merge-tree, commit-tree and a compare-and-swap update-ref of the target.
    CommitTree,
}

/// What one pick did.
#[derive(Debug, PartialEq, Eq)]
pub enum PickResult {
    /// Picked; the new commit's full sha.
    Picked {
        /// The new commit on the target.
        new_commit: String,
    },
    /// The merge conflicts; nothing changed.
    Conflict {
        /// Which paths conflict.
        reason: String,
    },
    /// Refused before any change.
    Skipped(Refusal),
}

impl From<&PickResult> for PickOutcome {
    fn from(result: &PickResult) -> Self {
        match result {
            PickResult::Picked { .. } => PickOutcome::Picked,
            PickResult::Conflict { .. } => PickOutcome::Conflict,
            PickResult::Skipped(_) => PickOutcome::Skipped,
        }
    }
}

/// What a pick of one probed commit would do.
#[derive(Debug, PartialEq, Eq)]
pub enum ProbeResult {
    /// Applies cleanly.
    Applies,
    /// The merge conflicts.
    Conflict {
        /// Which paths conflict.
        reason: String,
    },
    /// A pick would refuse it.
    Skipped(Refusal),
}

impl From<&ProbeResult> for ProbeOutcome {
    fn from(result: &ProbeResult) -> Self {
        match result {
            ProbeResult::Applies => ProbeOutcome::Applies,
            ProbeResult::Conflict { .. } => ProbeOutcome::Conflict,
            ProbeResult::Skipped(_) => ProbeOutcome::Skipped,
        }
    }
}

/// One probed commit and its result.
#[derive(Debug)]
pub struct ProbeLine {
    /// The commit, full sha.
    pub commit: String,
    /// What a pick would do.
    pub result: ProbeResult,
}

/// One probe's target and every commit's result, in argument order.
#[derive(Debug)]
pub struct ProbeReport {
    /// The target branch.
    pub onto: String,
    /// Its tip when probed, full sha.
    pub onto_tip: String,
    /// One line per asked commit.
    pub lines: Vec<ProbeLine>,
}

impl From<ProbeLine> for ProbeEntry {
    fn from(line: ProbeLine) -> Self {
        let outcome = ProbeOutcome::from(&line.result);
        let reason = match line.result {
            ProbeResult::Applies => None,
            ProbeResult::Conflict { reason } => Some(reason),
            ProbeResult::Skipped(refusal) => Some(refusal.to_string()),
        };
        ProbeEntry {
            commit: line.commit,
            outcome,
            reason,
        }
    }
}

impl From<ProbeReport> for Probe {
    fn from(report: ProbeReport) -> Self {
        Probe {
            schema: PROBE_SCHEMA,
            onto: report.onto,
            onto_sha: report.onto_tip,
            commits: report.lines.into_iter().map(ProbeEntry::from).collect(),
        }
    }
}

/// One pick's subject and result.
#[derive(Debug)]
pub struct PickReport {
    /// The commit, full sha.
    pub commit: String,
    /// The target branch.
    pub onto: String,
    /// What happened.
    pub result: PickResult,
}

impl From<PickReport> for Pick {
    fn from(report: PickReport) -> Self {
        let outcome = PickOutcome::from(&report.result);
        let (new_commit, reason) = match report.result {
            PickResult::Picked { new_commit } => (Some(new_commit), None),
            PickResult::Conflict { reason } => (None, Some(reason)),
            PickResult::Skipped(refusal) => (None, Some(refusal.to_string())),
        };
        Pick {
            schema: PICK_SCHEMA,
            commit: report.commit,
            onto: report.onto,
            outcome,
            new_commit,
            reason,
        }
    }
}

/// One branch to fast-forward to one commit, every fact gathered before anything runs.
#[derive(Debug, Clone)]
pub struct ForwardPlan {
    /// The commit, full sha.
    pub commit: String,
    /// The target branch.
    pub onto: String,
    /// The target's tip when planned, full sha — the compare-and-swap old value.
    pub onto_tip: String,
    /// The commit's history holds the target's tip.
    pub holds_tip: bool,
    /// The target's worktree; `None` when not checked out.
    pub checkout: Option<TargetCheckout>,
}

impl ForwardPlan {
    /// The one step `forward` takes for this plan.
    #[must_use]
    pub fn step(&self) -> ForwardStep {
        if self.commit == self.onto_tip {
            return ForwardStep::Nothing;
        }
        if !self.holds_tip {
            return ForwardStep::Skip(Refusal::NotFastForward {
                branch: self.onto.clone(),
                commit: short(&self.commit).to_owned(),
            });
        }
        match &self.checkout {
            Some(checkout) if checkout.clean => ForwardStep::MergeFfOnly(checkout.path.clone()),
            Some(checkout) => ForwardStep::Skip(Refusal::DirtyCheckout {
                branch: self.onto.clone(),
                path: checkout.path.display().to_string(),
            }),
            None => ForwardStep::UpdateRef,
        }
    }
}

/// The one step of a forward, decided before anything runs.
#[derive(Debug, PartialEq, Eq)]
pub enum ForwardStep {
    /// Refused; its `Display` is the reason.
    Skip(Refusal),
    /// The target already points at the commit.
    Nothing,
    /// `git merge --ff-only` in this worktree, where the target is checked out.
    MergeFfOnly(PathBuf),
    /// A compare-and-swap update-ref of the target.
    UpdateRef,
}

/// What one forward did.
#[derive(Debug, PartialEq, Eq)]
pub enum ForwardResult {
    /// The target points at the commit now.
    Forwarded,
    /// The target already pointed at the commit.
    Unchanged,
    /// Nothing moved; the reason.
    Skipped(String),
}

impl From<&ForwardResult> for ForwardOutcome {
    fn from(result: &ForwardResult) -> Self {
        match result {
            ForwardResult::Forwarded => ForwardOutcome::Forwarded,
            ForwardResult::Unchanged => ForwardOutcome::Unchanged,
            ForwardResult::Skipped(_) => ForwardOutcome::Skipped,
        }
    }
}

/// One forward's subject and result.
#[derive(Debug)]
pub struct ForwardReport {
    /// The target branch.
    pub onto: String,
    /// Its tip when planned, full sha.
    pub from: String,
    /// The commit, full sha.
    pub to: String,
    /// What happened.
    pub result: ForwardResult,
}

impl From<ForwardReport> for Forward {
    fn from(report: ForwardReport) -> Self {
        let outcome = ForwardOutcome::from(&report.result);
        let reason = match report.result {
            ForwardResult::Forwarded | ForwardResult::Unchanged => None,
            ForwardResult::Skipped(reason) => Some(reason),
        };
        Forward {
            schema: FORWARD_SCHEMA,
            onto: report.onto,
            from: report.from,
            to: report.to,
            outcome,
            reason,
        }
    }
}

fn line(subject: &str, outcome: OpOutcome, text: String) -> OpLine {
    OpLine {
        subject: subject.to_owned(),
        outcome,
        text,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn plan(worktree: Option<&str>, safe: bool, force: bool, delete_branch: bool) -> RemovePlan {
        RemovePlan {
            branch: "claude/x".to_owned(),
            worktree: worktree.map(PathBuf::from),
            safe,
            unsafe_reasons: if safe {
                vec![]
            } else {
                vec!["dirty(1)".to_owned()]
            },
            force,
            delete_branch,
        }
    }

    #[test]
    fn steps_follow_the_plan() {
        let cases = [
            (
                "safe-worktree-delete",
                plan(Some("/wt"), true, false, true),
                vec![
                    RemoveStep::RemoveWorktree(PathBuf::from("/wt")),
                    RemoveStep::DeleteBranch,
                ],
            ),
            (
                "unsafe-worktree-delete",
                plan(Some("/wt"), false, false, true),
                vec![
                    RemoveStep::Skip(Refusal::Unsafe {
                        branch: "claude/x".to_owned(),
                        path: "/wt".to_owned(),
                        reasons: "dirty(1)".to_owned(),
                    }),
                    RemoveStep::Skip(Refusal::UnsafeBranch {
                        branch: "claude/x".to_owned(),
                        reasons: "dirty(1)".to_owned(),
                    }),
                ],
            ),
            (
                "forced-unsafe",
                plan(Some("/wt"), false, true, true),
                vec![
                    RemoveStep::RemoveWorktree(PathBuf::from("/wt")),
                    RemoveStep::DeleteBranch,
                ],
            ),
            (
                "no-worktree-no-delete",
                plan(None, true, false, false),
                vec![RemoveStep::Skip(Refusal::NoWorktree("claude/x".to_owned()))],
            ),
            (
                "no-worktree-delete-safe",
                plan(None, true, false, true),
                vec![RemoveStep::DeleteBranch],
            ),
        ];
        for (label, plan, want) in cases {
            assert_eq!(plan.steps(), want, "{label}");
        }
    }

    fn pick_plan(parents: usize, contained: bool, checkout: Option<bool>) -> PickPlan {
        PickPlan {
            commit: "c".repeat(40),
            onto: "main".to_owned(),
            onto_tip: "a".repeat(40),
            parents,
            contained,
            checkout: checkout.map(|clean| TargetCheckout {
                path: PathBuf::from("/wt"),
                clean,
            }),
        }
    }

    fn forward_plan(commit: &str, holds_tip: bool, checkout: Option<bool>) -> ForwardPlan {
        ForwardPlan {
            commit: commit.repeat(40),
            onto: "feature/x".to_owned(),
            onto_tip: "a".repeat(40),
            holds_tip,
            checkout: checkout.map(|clean| TargetCheckout {
                path: PathBuf::from("/wt"),
                clean,
            }),
        }
    }

    #[test]
    fn forward_step_follows_the_plan() {
        let cases = [
            (
                "same-commit",
                forward_plan("a", true, Some(false)),
                ForwardStep::Nothing,
            ),
            (
                "not-a-fast-forward",
                forward_plan("c", false, Some(true)),
                ForwardStep::Skip(Refusal::NotFastForward {
                    branch: "feature/x".to_owned(),
                    commit: "c".repeat(8),
                }),
            ),
            (
                "clean-checkout",
                forward_plan("c", true, Some(true)),
                ForwardStep::MergeFfOnly(PathBuf::from("/wt")),
            ),
            (
                "dirty-checkout",
                forward_plan("c", true, Some(false)),
                ForwardStep::Skip(Refusal::DirtyCheckout {
                    branch: "feature/x".to_owned(),
                    path: "/wt".to_owned(),
                }),
            ),
            (
                "not-checked-out",
                forward_plan("c", true, None),
                ForwardStep::UpdateRef,
            ),
        ];
        for (label, plan, want) in cases {
            assert_eq!(plan.step(), want, "{label}");
        }
    }

    #[test]
    fn forward_result_maps_to_wire() {
        let cases = [
            (ForwardResult::Forwarded, ForwardOutcome::Forwarded, None),
            (ForwardResult::Unchanged, ForwardOutcome::Unchanged, None),
            (
                ForwardResult::Skipped("why".to_owned()),
                ForwardOutcome::Skipped,
                Some("why"),
            ),
        ];
        for (result, outcome, reason) in cases {
            let label = format!("{result:?}");
            let forward = Forward::from(ForwardReport {
                onto: "feature/x".to_owned(),
                from: "a".to_owned(),
                to: "c".to_owned(),
                result,
            });
            assert_eq!(forward.outcome, outcome, "{label}");
            assert_eq!(forward.reason.as_deref(), reason, "{label}");
        }
    }

    #[test]
    fn pick_step_follows_the_plan() {
        let cases = [
            (
                "root",
                pick_plan(0, false, None),
                PickStep::Skip(Refusal::RootCommit),
            ),
            (
                "merge",
                pick_plan(2, false, None),
                PickStep::Skip(Refusal::MergeCommit),
            ),
            (
                "contained",
                pick_plan(1, true, Some(true)),
                PickStep::Skip(Refusal::AlreadyOn("main".to_owned())),
            ),
            (
                "clean-checkout",
                pick_plan(1, false, Some(true)),
                PickStep::CherryPick(PathBuf::from("/wt")),
            ),
            (
                "dirty-checkout",
                pick_plan(1, false, Some(false)),
                PickStep::Skip(Refusal::DirtyCheckout {
                    branch: "main".to_owned(),
                    path: "/wt".to_owned(),
                }),
            ),
            (
                "not-checked-out",
                pick_plan(1, false, None),
                PickStep::CommitTree,
            ),
        ];
        for (label, plan, want) in cases {
            assert_eq!(plan.step(), want, "{label}");
        }
    }

    #[test]
    fn probe_result_maps_to_wire() {
        let cases = [
            (ProbeResult::Applies, ProbeOutcome::Applies, None),
            (
                ProbeResult::Conflict {
                    reason: "conflicts in a".to_owned(),
                },
                ProbeOutcome::Conflict,
                Some("conflicts in a"),
            ),
            (
                ProbeResult::Skipped(Refusal::MergeCommit),
                ProbeOutcome::Skipped,
                Some("merge commit"),
            ),
        ];
        for (result, outcome, reason) in cases {
            let label = format!("{result:?}");
            let probe = Probe::from(ProbeReport {
                onto: "main".to_owned(),
                onto_tip: "a".repeat(40),
                lines: vec![ProbeLine {
                    commit: "c".to_owned(),
                    result,
                }],
            });
            assert_eq!(probe.schema, PROBE_SCHEMA, "{label}");
            assert_eq!(probe.onto_sha, "a".repeat(40), "{label}");
            assert_eq!(probe.commits.len(), 1, "{label}");
            assert_eq!(probe.commits[0].commit, "c", "{label}");
            assert_eq!(probe.commits[0].outcome, outcome, "{label}");
            assert_eq!(probe.commits[0].reason.as_deref(), reason, "{label}");
        }
    }

    #[test]
    fn pick_result_maps_to_wire() {
        let cases = [
            (
                PickResult::Picked {
                    new_commit: "n".to_owned(),
                },
                PickOutcome::Picked,
                Some("n"),
                None,
            ),
            (
                PickResult::Conflict {
                    reason: "conflicts in a".to_owned(),
                },
                PickOutcome::Conflict,
                None,
                Some("conflicts in a"),
            ),
            (
                PickResult::Skipped(Refusal::AlreadyOn("main".to_owned())),
                PickOutcome::Skipped,
                None,
                Some("already on main"),
            ),
        ];
        for (result, outcome, new_commit, reason) in cases {
            let label = format!("{result:?}");
            let pick = Pick::from(PickReport {
                commit: "c".to_owned(),
                onto: "main".to_owned(),
                result,
            });
            assert_eq!(pick.outcome, outcome, "{label}");
            assert_eq!(pick.new_commit.as_deref(), new_commit, "{label}");
            assert_eq!(pick.reason.as_deref(), reason, "{label}");
        }
    }

    #[test]
    fn checked_out_at_reads_porcelain() {
        let porcelain = "worktree /repo\nHEAD 1111\nbranch refs/heads/main\n\n\
                         worktree /wt/a\nHEAD 2222\nbranch refs/heads/claude/a\n\n\
                         worktree /wt/d\nHEAD 3333\ndetached";
        let cases = [
            ("main", Some("/repo")),
            ("claude/a", Some("/wt/a")),
            ("claude", None),
            ("develop", None),
        ];
        for (branch, want) in cases {
            assert_eq!(
                checked_out_at(porcelain, branch),
                want.map(PathBuf::from),
                "{branch}"
            );
        }
    }

    #[test]
    fn pick_message_and_reason() {
        assert_eq!(
            pick_message("subject\n\nbody\n", "abc"),
            "subject\n\nbody\n\n(cherry picked from commit abc)\n"
        );
        assert_eq!(
            conflict_reason(["a.txt", "b.txt", "a.txt", ""].into_iter()),
            "conflicts in a.txt, b.txt"
        );
        assert_eq!(conflict_reason(std::iter::empty()), "conflicts");
    }
}
