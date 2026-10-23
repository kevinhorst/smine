//! Patch-id and touched paths of one commit, from a rename-aware tree diff.
//!
//! The id is `blake3(header ‖ content)` per file — comparable only between values produced by this function, which
//! is all the twin scan needs. Content: modified and renamed text files contribute their hunk lines without
//! whitespace (a copy onto a branch with different surroundings has other blob ids but the same lines); added and
//! deleted files contribute their blob id (the whole file is the change); binary files their blob ids. Directory
//! entries contribute nothing: their tree ids depend on untouched siblings.

use std::collections::BTreeSet;
use std::convert::Infallible;

use gix::ObjectId;
use gix::bstr::{BString, ByteSlice};
use gix::diff::blob::platform::prepare_diff::Operation;
use gix::object::blob::diff::lines::Change as LineChange;
use gix::object::tree::diff::{Action, Change};

use crate::error::{Error, GitExt, Result};

/// Opaque content hash of a commit's diff.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct PatchId([u8; 32]);

/// Paths a commit changed (rename sources included).
pub fn touched_paths(
    repo: &gix::Repository,
    old_commit: ObjectId,
    new_commit: ObjectId,
) -> Result<BTreeSet<BString>> {
    let old_tree = repo.find_commit(old_commit).git()?.tree().git()?;
    let new_tree = repo.find_commit(new_commit).git()?.tree().git()?;
    let mut paths = BTreeSet::new();
    old_tree
        .changes()
        .git()?
        .for_each_to_obtain_tree(&new_tree, |change| {
            paths.insert(change.location().to_owned());
            if let Change::Rewrite {
                source_location, ..
            } = change
            {
                paths.insert(source_location.to_owned());
            }
            Ok::<Action, Infallible>(Action::Continue(()))
        })
        .git()?;
    Ok(paths)
}

/// The patch-id of `old_commit..new_commit`.
pub fn patch_id(
    repo: &gix::Repository,
    old_commit: ObjectId,
    new_commit: ObjectId,
) -> Result<PatchId> {
    let old_tree = repo.find_commit(old_commit).git()?.tree().git()?;
    let new_tree = repo.find_commit(new_commit).git()?.tree().git()?;
    let mut blob_cache = repo.diff_resource_cache_for_tree_diff().git()?;
    let mut hasher = blake3::Hasher::new();
    let mut failure: Option<Error> = None;

    old_tree
        .changes()
        .git()?
        .for_each_to_obtain_tree(&new_tree, |change| {
            let (old_path, new_path) = match &change {
                Change::Rewrite {
                    source_location,
                    location,
                    ..
                } => (*source_location, *location),
                other => (other.location(), other.location()),
            };
            // A directory entry repeats its files' changes, and its tree id also covers every untouched sibling — the
            // twin's directory differs whenever the candidate changed a neighbour, so it never enters the id.
            if change.entry_mode().is_tree() {
                return Ok::<Action, Infallible>(Action::Continue(()));
            }
            hasher.update(old_path);
            hasher.update(b"\0");
            hasher.update(new_path);
            hasher.update(b"\0");
            // Submodule pointers have no line diff: their identity is mode plus object id.
            if !change.entry_mode().is_blob_or_symlink() {
                hasher.update(change.entry_mode().as_str().as_bytes());
                hasher.update(change.id().as_bytes());
                return Ok::<Action, Infallible>(Action::Continue(()));
            }
            // An added or deleted file's change is the whole file: identical change ⇔ identical blob id. No line
            // diff, so a bulk add/delete costs one hash per file instead of one per line.
            match &change {
                Change::Addition { .. } => {
                    hasher.update(b"A\0");
                    hasher.update(change.id().as_bytes());
                    return Ok(Action::Continue(()));
                }
                Change::Deletion { .. } => {
                    hasher.update(b"D\0");
                    hasher.update(change.id().as_bytes());
                    return Ok(Action::Continue(()));
                }
                Change::Modification { .. } | Change::Rewrite { .. } => {}
            }

            let is_binary = change.diff(&mut blob_cache).git().and_then(|mut platform| {
                let outcome = platform
                    .lines(|hunk| {
                        match hunk {
                            LineChange::Addition { lines } => hash_lines(&mut hasher, b'+', lines),
                            LineChange::Deletion { lines } => hash_lines(&mut hasher, b'-', lines),
                            LineChange::Modification {
                                lines_before,
                                lines_after,
                            } => {
                                hash_lines(&mut hasher, b'-', lines_before);
                                hash_lines(&mut hasher, b'+', lines_after);
                            }
                        }
                        Ok::<(), Infallible>(())
                    })
                    .git()?;
                Ok(matches!(
                    outcome.operation,
                    Operation::SourceOrDestinationIsBinary
                ))
            });
            match is_binary {
                Ok(true) => {
                    // Binary: the content identity is the pair of blob ids.
                    hasher.update(change.id().as_bytes());
                    if let Change::Modification { previous_id, .. } = &change {
                        hasher.update(previous_id.as_bytes());
                    }
                    Ok::<Action, Infallible>(Action::Continue(()))
                }
                Ok(false) => Ok(Action::Continue(())),
                Err(err) => {
                    // Keep walking: a `Break` surfaces as a cancellation error from gix and would mask the cause.
                    failure.get_or_insert(err);
                    Ok(Action::Continue(()))
                }
            }
        })
        .git()?;

    if let Some(err) = failure {
        return Err(err);
    }
    Ok(PatchId(*hasher.finalize().as_bytes()))
}

fn hash_lines(hasher: &mut blake3::Hasher, sign: u8, lines: &[&gix::bstr::BStr]) {
    for line in lines {
        hasher.update(&[sign]);
        let stripped: Vec<u8> = line
            .bytes()
            .filter(|byte| !byte.is_ascii_whitespace())
            .collect();
        hasher.update(&stripped);
        hasher.update(b"\n");
    }
}
