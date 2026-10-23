//! agentwt — integration state of agent worktrees, merge order and the safety-gated removal
//! over one repository.
//!
//! Module map, in data-flow order:
//! * [`inventory`] lists agent branches and worktrees from refs and `.git/worktrees`.
//! * [`gitstate`] computes base branch, ahead/behind vs base, per-commit verdicts,
//!   the containing candidates, filtered dirty/untracked counts.
//! * [`patchid`] hashes a commit's rename-aware diff for the twin scan.
//! * [`sessions`] reads the Claude and Codex session registries and joins them to worktrees.
//! * [`order`] computes the pairwise conflict matrix and the simulated merge train.
//! * [`status`] assembles everything into [`wire::Document`].
//! * [`unplaced`] answers what one branch still owes one target — the replay list of a restack.
//! * [`interfaces`] lists the interface versions this build implements, the `interfaces` handshake.
//! * [`ops`] performs the state-mutating commands `remove`, `pick` and `forward`, plan-then-execute through git
//!   children, and the read-only `probe` that asks what `pick` would do with many commits at once.
//!
//! Invariants: the library never prints; the gix analysis never writes to `.git/objects` (object memory) — only
//! [`ops`]' git children change a repository; the library never guesses a base branch (`unknown` is a valid answer).
#![forbid(unsafe_code)]

pub mod error;
pub mod gitstate;
pub mod interfaces;
pub mod inventory;
pub mod ops;
pub mod order;
pub mod patchid;
pub mod sessions;
pub mod status;
pub mod unplaced;
pub mod wire;

pub use error::{Error, Refusal, Result};
