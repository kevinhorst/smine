//! Error model: one `Error` enum for the library, converted to `anyhow` by the binary.
//! `Refusal` is a business-rule refusal (unknown branch, unsafe removal): the operation did not
//! run and the message is the operator-facing output line — a `skipped` line in `remove`, the `reason` of a
//! `skipped` pick or forward, exit 3 as an error (unknown or non-agent branch, unknown commit, any refusal under
//! `remove --dry-run`).
//!
//! gix 0.87 returns one typed error per operation (`gix::open::Error`, `gix::revision::walk::Error`, …);
//! the `GitExt::git()` adapter lifts any of them into `gix::Error` so every gix call reads `call().git()?`.

use std::path::PathBuf;

/// Library result alias.
pub type Result<T> = std::result::Result<T, Error>;

/// Every failure the library can return.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// A gix operation failed (chain carried by gix's own error).
    #[error("git: {0}")]
    Git(#[from] gix::Error),
    /// Reading or writing a file failed; `path` names it.
    #[error("{path}: {source}")]
    Io {
        /// The file involved.
        path: PathBuf,
        /// The underlying error.
        #[source]
        source: std::io::Error,
    },
    /// JSON (de)serialization failed.
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    /// A path holds bytes that are not UTF-8 and cannot enter the JSON document.
    #[error("path is not valid UTF-8: {0}")]
    NonUtf8Path(PathBuf),
    /// A business-rule refusal; the operation did not happen.
    #[error(transparent)]
    Refused(#[from] Refusal),
    /// A `git` child exited non-zero.
    #[error("git {command} exited {status}: {stderr}")]
    GitCommand {
        /// The argv after `git`, space-joined.
        command: String,
        /// The child's exit status.
        status: i32,
        /// Captured stderr, lossily decoded.
        stderr: String,
    },
    /// SIGINT/SIGTERM arrived between steps.
    #[error("interrupted by signal {signal}")]
    Interrupted {
        /// The signal number.
        signal: i32,
    },
    /// A repository state the algorithms cannot represent (reported, never guessed around).
    #[error("{0}")]
    Invariant(String),
}

/// Business-rule refusals. `Display` is the operator-facing line the remove script printed.
#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum Refusal {
    /// The branch does not exist as a local ref.
    #[error("unknown branch {0}")]
    UnknownBranch(String),
    /// The branch is outside the agent namespaces.
    #[error("not an agent branch: {0}")]
    NotAgentBranch(String),
    /// Removal refused by the safety predicate.
    #[error("skipped: {path} ({branch}): {reasons}")]
    Unsafe {
        /// The branch.
        branch: String,
        /// The worktree path, or `-` when the branch has none.
        path: String,
        /// Comma-joined unsafe reasons, e.g. `dirty(2), unpicked(1)`.
        reasons: String,
    },
    /// `remove` without `--delete-branch` on a branch that has no worktree.
    #[error("no worktree checked out for {0}")]
    NoWorktree(String),
    /// Branch deletion refused by the safety predicate.
    #[error("skipped branch: {branch}: {reasons}")]
    UnsafeBranch {
        /// The branch.
        branch: String,
        /// Comma-joined unsafe reasons.
        reasons: String,
    },
    /// `pick` or `probe` names something that does not resolve to a commit.
    #[error("unknown commit {0}")]
    UnknownCommit(String),
    /// `pick` or `probe` of a commit without a parent: there is no base to pick it against.
    #[error("root commit")]
    RootCommit,
    /// `pick` or `probe` of a merge commit: one change per pick, never a merge.
    #[error("merge commit")]
    MergeCommit,
    /// `pick` or `probe` of a commit the target already holds.
    #[error("already on {0}")]
    AlreadyOn(String),
    /// `pick` or `forward` onto a branch whose checkout has tracked changes.
    #[error("{branch} is checked out in {path} with tracked changes")]
    DirtyCheckout {
        /// The target branch.
        branch: String,
        /// Its worktree.
        path: String,
    },
    /// `forward` to a commit whose history lacks the branch's tip.
    #[error("{commit} does not hold the tip of {branch}")]
    NotFastForward {
        /// The target branch.
        branch: String,
        /// The commit, short sha.
        commit: String,
    },
}

impl Error {
    /// Exit code the binary maps this error to: refusals 3, interrupts 128+signal,
    /// a failing git child forwards its status, everything else 1. clap keeps its own 2.
    #[must_use]
    pub fn exit_code(&self) -> i32 {
        match self {
            Error::Refused(_) => 3,
            Error::Interrupted { signal } => 128 + signal,
            Error::GitCommand { status, .. } if *status > 0 => *status,
            Error::Git(_)
            | Error::Io { .. }
            | Error::Json(_)
            | Error::NonUtf8Path(_)
            | Error::GitCommand { .. }
            | Error::Invariant(_) => 1,
        }
    }

    /// Wraps an io error with the path it concerns.
    pub(crate) fn io(path: impl Into<PathBuf>) -> impl FnOnce(std::io::Error) -> Error {
        let path = path.into();
        move |source| Error::Io { path, source }
    }

    /// Lifts any typed git error into the `Git` variant; the location gix records is the caller's.
    #[track_caller]
    pub(crate) fn git(source: impl std::error::Error + Send + Sync + 'static) -> Error {
        Error::Git(gix::Error::from_error(source))
    }
}

/// `call().git()?` — the one adapter between gix's per-operation error types and [`Error`].
pub(crate) trait GitExt<T> {
    /// Converts the error into [`Error::Git`], located at the calling line.
    #[track_caller]
    fn git(self) -> Result<T>;
}

impl<T, E> GitExt<T> for std::result::Result<T, E>
where
    E: std::error::Error + Send + Sync + 'static,
{
    #[track_caller]
    fn git(self) -> Result<T> {
        // A direct call keeps the caller's location; a fn pointer passed to map_err would not.
        match self {
            Ok(value) => Ok(value),
            Err(err) => Err(Error::git(err)),
        }
    }
}

/// gix plumbing returns `Exn<E>`; gix's own `From<Exn<E>> for gix::Error` lifts it, and this impl lets `?`
/// finish the job.
impl<E> From<gix::Exn<E>> for Error
where
    E: std::error::Error + Send + Sync + 'static,
{
    fn from(exn: gix::Exn<E>) -> Self {
        Error::Git(gix::Error::from(exn))
    }
}
