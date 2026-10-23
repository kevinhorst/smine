//! The `unplaced` query: the commits one branch still owes one target, as [`Unplaced`].
//!
//! Decisions: the target is the only candidate asked, tracked by name so an agent-owned target (a fleet's merge
//! branch) works like `develop` or `main`; the list is drawn from the range between the two tips, so the answer
//! needs no reflog; a name that is no local branch is a refusal, never an empty list.
//! Invariant: read-only like `status` — object memory, no ref, index or working-tree write.

use std::path::Path;

use gix::ObjectId;

use crate::error::{GitExt, Refusal, Result};
use crate::gitstate::Context;
use crate::inventory;
use crate::status::commit_entry;
use crate::wire::{UNPLACED_SCHEMA, Unplaced};

/// Builds the document for `branch` against `onto`; `repo` is any worktree of the repository.
pub fn build(repo: &Path, branch: &str, onto: &str) -> Result<Unplaced> {
    let repo_path = inventory::main_checkout(repo)?;
    let mut context = Context::open(&repo_path, onto, &[onto.to_owned()])?;
    let target = context
        .candidates()
        .iter()
        .find(|candidate| candidate.name == onto)
        .cloned()
        .ok_or_else(|| Refusal::UnknownBranch(onto.to_owned()))?;
    let tip = branch_tip(&context, branch)?;
    let commits = context.unplaced(tip, &target)?;
    Ok(Unplaced {
        schema: UNPLACED_SCHEMA,
        branch: branch.to_owned(),
        onto: onto.to_owned(),
        onto_sha: target.tip.to_string(),
        onto_short_sha: target.tip.to_hex_with_len(8).to_string(),
        commits: commits.iter().map(commit_entry).collect(),
    })
}

/// The tip of the local branch `name`; a refusal when no such branch exists.
fn branch_tip(context: &Context, name: &str) -> Result<ObjectId> {
    let reference = context
        .repo()
        .try_find_reference(format!("refs/heads/{name}").as_str())
        .git()?;
    match reference {
        Some(mut reference) => Ok(reference.peel_to_id().git()?.detach()),
        None => Err(Refusal::UnknownBranch(name.to_owned()).into()),
    }
}
