//! Wire types of the status document (schema 1) — the only thing the config server parses.
//!
//! Optional fields: absent (`Option<T>` + `skip_serializing_if`) = not applicable, `null` = asked and unknown.
//!
//! Field-by-field: `Row::worktree` absent on branch-only rows; `Row::from.name` `null` when unknown;
//! `Row::counts.vs_from` absent when the base is unknown; `Row::containment` `null` when no candidate holds the
//! work; `Row::session` absent when no session sits on the worktree; `Row::order` absent when the branch has
//! nothing to merge; `Document::trains` absent when no train base was requested.
//!
//! Every enum-valued field is a dedicated wire enum with `snake_case` renames; the tests pin each variant's string.
//!
//! [`Unplaced`] is the second document — the answer of the `unplaced` query, with its own schema number; every
//! field of it is always present.
//!
//! [`Pick`] is the third — the answer of `pick`, own schema number; `new_commit` is absent unless `picked`,
//! `reason` absent unless `conflict` or `skipped` (absent = not applicable, never `null`).
//!
//! [`Forward`] is the fourth — the answer of `forward`, own schema number; `reason` absent unless `skipped`.
//!
//! [`Probe`] is the fifth — the answer of `probe`, own schema number: one entry per asked commit in argument order,
//! `reason` absent unless `conflict` or `skipped`.

use std::collections::BTreeMap;

use schemars::JsonSchema;
use serde::Serialize;

/// Schema version of this document.
pub const SCHEMA: u32 = 1;
/// Schema version of the [`Unplaced`] document.
pub const UNPLACED_SCHEMA: u32 = 1;
/// Schema version of the [`Pick`] document.
pub const PICK_SCHEMA: u32 = 1;
/// Schema version of the [`Forward`] document.
pub const FORWARD_SCHEMA: u32 = 1;
/// Schema version of the [`Probe`] document.
pub const PROBE_SCHEMA: u32 = 1;

/// Root envelope.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Document {
    /// Always [`SCHEMA`].
    pub schema: u32,
    /// Repo-wide facts.
    pub repo: RepoInfo,
    /// The main checkout's branch and dirty count.
    pub checkout: Checkout,
    /// Non-agent local branches every containment question was asked against, default branch first.
    pub candidates: Vec<String>,
    /// One row per agent branch (name order) or detached worktree.
    pub rows: Vec<Row>,
    /// Every kind the inventory holds (sorted, distinct): agent namespaces and `codex`, whatever `--kind` selected.
    pub kinds: Vec<String>,
    /// Merge-order result over the rows with own commits.
    pub order: OrderSummary,
    /// One train per requested `--train-base`, in request order (duplicates dropped); absent when none was requested.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub trains: Option<Vec<BaseTrain>>,
    /// Registry files that could not be read (path plus reason); the document is still complete.
    pub warnings: Vec<String>,
    /// Wall time per phase — the load evidence the UI timing line and the load test read.
    pub timings: Timings,
}

/// Milliseconds per phase of one run.
#[derive(Debug, Clone, Default, Serialize, JsonSchema)]
pub struct Timings {
    /// Refs and worktrees.
    pub inventory_ms: u64,
    /// Wall of the row phase: history analysis and worktree status run concurrently.
    pub gitstate_ms: u64,
    /// Base, ahead/behind, containment over every row (main thread, serial).
    pub analysis_ms: u64,
    /// The slowest worktree's status, content hashes of racily-clean entries included.
    pub worktrees_ms: u64,
    /// The main checkout's branch and dirty count.
    pub checkout_ms: u64,
    /// Registry reads and the join.
    pub sessions_ms: u64,
    /// Matrix and train.
    pub order_ms: u64,
    /// Everything.
    pub total_ms: u64,
}

/// Repo-wide facts.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct RepoInfo {
    /// Canonical path of the main checkout.
    pub path: String,
    /// The default branch (rank `default` of a containment).
    pub default_branch: String,
}

/// The main checkout.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Checkout {
    /// Checked-out branch, `HEAD` when detached.
    pub branch: String,
    /// Tracked changes (staged or unstaged).
    pub dirty: usize,
}

/// One agent branch, or one detached worktree (`branch` null).
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Row {
    /// Branch name; `null` for a detached worktree.
    pub branch: Option<String>,
    /// Agent namespace (`claude`, `claude-routines`, …); `codex` for detached pool rows; `null` otherwise.
    pub kind: Option<String>,
    /// Tip commit.
    pub head: Head,
    /// Worktree facts; absent when the branch has no checkout.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub worktree: Option<Worktree>,
    /// Base branch resolution.
    pub from: From,
    /// Commit counts.
    pub counts: Counts,
    /// Commit lists behind the counts.
    pub commits: Commits,
    /// Number of `unpicked_commits`.
    pub unpicked: usize,
    /// Own non-merge commits no candidate holds (ancestor, no-op re-apply, replay, patch-id twin), full shas, newest
    /// first; empty when contained.
    pub unpicked_commits: Vec<String>,
    /// The one candidate that holds this branch's work, in preference order base → default → any other;
    /// `null` when none does.
    pub containment: Option<Containment>,
    /// The removal predicate.
    pub safe_to_remove: bool,
    /// Why removal is refused; empty when safe.
    pub unsafe_reasons: Vec<String>,
    /// Session attached to the worktree; absent when none.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub session: Option<Session>,
    /// Merge-order facts; absent when the branch has no own commits.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub order: Option<RowOrder>,
}

/// Tip commit facts.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Head {
    /// Full hex sha.
    pub sha: String,
    /// Abbreviated per `core.abbrev`.
    pub short_sha: String,
    /// Subject line.
    pub subject: String,
    /// Committer time, RFC 3339 UTC.
    pub committed_at: String,
}

/// Worktree facts.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Worktree {
    /// Canonical filesystem path.
    pub path: String,
    /// `true` for the main worktree.
    pub main: bool,
    /// HEAD is detached.
    pub detached: bool,
    /// Lock reason when locked; absent otherwise.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub locked: Option<String>,
    /// Git considers it prunable (gitdir gone).
    pub prunable: bool,
    /// In-progress operation; absent when none.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub operation: Option<Operation>,
    /// Tracked changes, staged or unstaged (the count the remove gate and dirty badge use).
    pub dirty: usize,
    /// Untracked files after the infrastructure filter.
    pub untracked: usize,
    /// The untracked paths counted (first 20), for the tooltip.
    pub untracked_paths: Vec<String>,
}

/// In-progress git operation.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Operation {
    /// A merge is in progress.
    Merge,
    /// A rebase is in progress.
    Rebase,
    /// A cherry-pick is in progress.
    CherryPick,
    /// A revert is in progress.
    Revert,
    /// A bisect is in progress.
    Bisect,
}

impl Operation {
    /// The wire spelling, for reason texts.
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Operation::Merge => "merge",
            Operation::Rebase => "rebase",
            Operation::CherryPick => "cherry_pick",
            Operation::Revert => "revert",
            Operation::Bisect => "bisect",
        }
    }
}

/// Base branch resolution.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct From {
    /// The resolved base, `null` when unknown.
    pub name: Option<String>,
    /// How it was resolved.
    pub resolution: FromResolution,
    /// The raw reflog origin text, for the tooltip.
    pub origin: Option<String>,
}

/// How the base was resolved.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum FromResolution {
    /// The reflog named a ref that exists.
    Ref,
    /// The reflog carried a bare sha; the closest containing candidate was chosen.
    Sha,
    /// The reflog named another agent branch; its own origin was followed to a root.
    Chain,
    /// A rebase, reset or `branch -f` moved the branch onto another candidate after creation; that candidate is the
    /// base.
    Reanchored,
    /// Nothing resolvable — never guessed.
    Unknown,
}

/// Ahead/behind pairs.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Counts {
    /// Against the base; absent when the base is unknown.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vs_from: Option<AheadBehind>,
}

/// Commits ahead and behind.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema)]
pub struct AheadBehind {
    /// Commits only on this side.
    pub ahead: usize,
    /// Commits only on the other side.
    pub behind: usize,
}

/// Commit lists.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Commits {
    /// Own commits (not on the base), newest first, complete.
    pub ahead: Vec<CommitEntry>,
    /// Base commits missing here, newest first, capped at 200.
    pub behind: Vec<CommitEntry>,
    /// `true` when `behind` was cut at the cap.
    pub behind_truncated: bool,
}

/// One commit line.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct CommitEntry {
    /// Full hex sha.
    pub sha: String,
    /// Eight-character prefix.
    pub short_sha: String,
    /// Subject line.
    pub subject: String,
    /// Committer time, RFC 3339 UTC.
    pub committed_at: String,
    /// `true` for a merge commit (never counted as unpicked).
    pub is_merge: bool,
}

/// The one candidate holding a branch's work.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Containment {
    /// Candidate branch.
    pub branch: String,
    /// Why it counts as containing the work.
    pub reason: ContainmentReason,
    /// Where it sits in the preference order.
    pub rank: ContainmentRank,
}

/// Containment vocabulary.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ContainmentReason {
    /// The tip is an ancestor of the candidate.
    Ancestor,
    /// Every own non-merge commit is on the candidate: re-applies as a no-op, was picked (pick trailer, or the same
    /// author, author time and subject committed later — conflict resolution included) or has a patch-id twin.
    AllCommitsPlaced,
    /// Merging the tip into the candidate changes nothing: the candidate's tree already holds the branch's net change
    /// (a squash merge, commits that cancel out).
    MergeAddsNothing,
}

/// Preference order of the containing branch.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ContainmentRank {
    /// The branch this one was created from.
    Base,
    /// The repository's default branch.
    Default,
    /// Any other non-agent branch.
    Other,
}

/// Session attached to a worktree.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Session {
    /// `claude` or `codex`.
    pub agent: SessionAgent,
    /// Claude `sessionId` / Codex rollout id.
    pub id: String,
    /// Registry pid (Claude only); absent for Codex.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pid: Option<u32>,
    /// The process is alive right now (Claude only; Codex rollouts carry no pid → absent).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub alive: Option<bool>,
    /// Registry status (`busy`, `idle`, `waiting`); absent for Codex.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status: Option<String>,
    /// What the session waits for, while `waiting`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub waiting_for: Option<String>,
    /// `claude-desktop`, `cli`, … / Codex `originator`.
    pub entrypoint: String,
    /// Session name (Claude).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Desktop host session id (Claude only).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub host_session_id: Option<String>,
    /// Last registry update, RFC 3339 UTC.
    pub updated_at: String,
    /// Account uuid from the profile's `.claude.json`; absent when unreadable.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub account_uuid: Option<String>,
}

/// Which agent owns the session.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum SessionAgent {
    /// Claude Code.
    Claude,
    /// Codex.
    Codex,
}

/// Merge-order facts of one branch.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct RowOrder {
    /// Branches this one conflicts with pairwise (touched paths overlap and the merge is unresolved).
    pub conflicts_with: Vec<String>,
    /// 1-based position in the recommended train.
    pub train_position: usize,
    /// The train step outcome.
    pub train: TrainOutcome,
}

/// Outcome of one train step.
#[derive(Debug, Clone, Serialize, JsonSchema)]
#[serde(tag = "state", rename_all = "snake_case")]
pub enum TrainOutcome {
    /// Merged cleanly onto the train head before it.
    Clean,
    /// Conflicted against the train head; `blocked_by` names the branches merged before it, `paths` the conflicts.
    Blocked {
        /// Branches merged before this one.
        blocked_by: Vec<String>,
        /// Unresolved conflict paths.
        paths: Vec<String>,
    },
    /// Skipped: no own commits (already contained).
    Nothing,
}

/// Document-level order summary.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct OrderSummary {
    /// Recommended order, branch names.
    pub train: Vec<String>,
    /// Conflict pairs, each once, lexicographic.
    pub pairs: Vec<ConflictPair>,
    /// Connected components of the conflict graph (branches with no conflicts form singleton components).
    pub components: Vec<Vec<String>>,
    /// Branches whose worktree had tracked modifications; their snapshot, not their tip, was merged.
    pub dirty_snapshots: Vec<String>,
}

/// One conflicting pair.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct ConflictPair {
    /// First branch.
    pub a: String,
    /// Second branch.
    pub b: String,
    /// Unresolved conflict paths, sorted.
    pub paths: Vec<String>,
}

/// The merge train seeded with one requested base's tip instead of the default branch's; same inputs as the
/// default train, so `pairs`, `components` and `dirty_snapshots` equal it and only the train steps differ.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct BaseTrain {
    /// The base branch the train starts from.
    pub base: String,
    /// Order summary against this base.
    pub order: OrderSummary,
    /// Per-branch facts against this base, by branch name; branches with nothing to merge are absent.
    pub branches: BTreeMap<String, RowOrder>,
}

/// Outcome of `remove` (`pick` answers with its own [`Pick`] document).
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct OpResult {
    /// Always `remove` today (`--dry-run` included; its lines carry the `would_*` outcomes); the field stays so the
    /// Go decoder has one shape.
    pub operation: String,
    /// One line per acted-on subject, in order; the text mode prints these verbatim.
    pub lines: Vec<OpLine>,
}

/// One outcome line.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct OpLine {
    /// Branch, sha or path.
    pub subject: String,
    /// The outcome.
    pub outcome: OpOutcome,
    /// The human line (also what `--format text` prints).
    pub text: String,
}

/// Outcome vocabulary of `remove`.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum OpOutcome {
    /// The worktree was removed.
    Removed,
    /// The branch was deleted.
    BranchDeleted,
    /// Refused by the safety predicate.
    Skipped,
    /// `--dry-run`: the worktree would be removed.
    WouldRemove,
    /// `--dry-run`: the branch would be deleted.
    WouldDeleteBranch,
}

/// The `unplaced` document: what one branch still owes one target.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Unplaced {
    /// Always [`UNPLACED_SCHEMA`].
    pub schema: u32,
    /// The branch asked about.
    pub branch: String,
    /// The target it was judged against.
    pub onto: String,
    /// The target's tip, full hex sha.
    pub onto_sha: String,
    /// Eight-character prefix of the target's tip — the short sha a restack names its branches after.
    pub onto_short_sha: String,
    /// The branch's commits the target's history lacks and no per-commit layer places on it: no merges, parents
    /// before children.
    pub commits: Vec<CommitEntry>,
}

/// The `pick` document: what picking one commit onto one branch did.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Pick {
    /// Always [`PICK_SCHEMA`].
    pub schema: u32,
    /// The picked commit, full hex sha.
    pub commit: String,
    /// The target branch.
    pub onto: String,
    /// What happened.
    pub outcome: PickOutcome,
    /// The new commit on the target, full hex sha; present only when `outcome` is `picked`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub new_commit: Option<String>,
    /// Why; present only when `outcome` is `conflict` or `skipped`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Outcome vocabulary of `pick`.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PickOutcome {
    /// The commit was picked onto the target.
    Picked,
    /// The three-way merge conflicts; the target is unchanged.
    Conflict,
    /// Refused before any change (root or merge commit, already on the target, dirty checkout).
    Skipped,
}

/// The `probe` document: whether each asked commit applies alone onto one branch's tip; nothing moves.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Probe {
    /// Always [`PROBE_SCHEMA`].
    pub schema: u32,
    /// The target branch.
    pub onto: String,
    /// The target's tip every commit was probed against, full hex sha.
    pub onto_sha: String,
    /// One entry per asked commit, in argument order.
    pub commits: Vec<ProbeEntry>,
}

/// One probed commit.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct ProbeEntry {
    /// The probed commit, full hex sha.
    pub commit: String,
    /// What a pick would do.
    pub outcome: ProbeOutcome,
    /// Why; present only when `outcome` is `conflict` or `skipped`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Outcome vocabulary of `probe`.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ProbeOutcome {
    /// The commit applies cleanly onto the target's tip.
    Applies,
    /// The three-way merge conflicts.
    Conflict,
    /// A pick would refuse it (root or merge commit, already on the target).
    Skipped,
}

/// The `forward` document: what fast-forwarding one branch to one commit did.
#[derive(Debug, Clone, Serialize, JsonSchema)]
pub struct Forward {
    /// Always [`FORWARD_SCHEMA`].
    pub schema: u32,
    /// The branch that was to move.
    pub onto: String,
    /// Its tip before the command, full hex sha.
    pub from: String,
    /// The commit it was to move to, full hex sha.
    pub to: String,
    /// What happened.
    pub outcome: ForwardOutcome,
    /// Why; present only when `outcome` is `skipped`.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Outcome vocabulary of `forward`.
#[derive(Debug, Clone, Copy, Serialize, JsonSchema, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ForwardOutcome {
    /// The branch now points at the commit; a checkout of it moved along.
    Forwarded,
    /// The branch already pointed at the commit.
    Unchanged,
    /// Refused before any change (not a fast-forward, dirty checkout, git refused the checkout's move).
    Skipped,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn forward_outcome_wire_strings_are_pinned() {
        let cases = [
            (ForwardOutcome::Forwarded, "\"forwarded\""),
            (ForwardOutcome::Unchanged, "\"unchanged\""),
            (ForwardOutcome::Skipped, "\"skipped\""),
        ];
        for (outcome, want) in cases {
            let wire = serde_json::to_string(&outcome).unwrap_or_default();
            assert_eq!(wire, want, "{outcome:?}");
        }
    }

    #[test]
    fn pick_outcome_wire_strings_are_pinned() {
        let cases = [
            (PickOutcome::Picked, "\"picked\""),
            (PickOutcome::Conflict, "\"conflict\""),
            (PickOutcome::Skipped, "\"skipped\""),
        ];
        for (outcome, want) in cases {
            let wire = serde_json::to_string(&outcome).unwrap_or_default();
            assert_eq!(wire, want, "{outcome:?}");
        }
    }

    #[test]
    fn probe_outcome_wire_strings_are_pinned() {
        let cases = [
            (ProbeOutcome::Applies, "\"applies\""),
            (ProbeOutcome::Conflict, "\"conflict\""),
            (ProbeOutcome::Skipped, "\"skipped\""),
        ];
        for (outcome, want) in cases {
            let wire = serde_json::to_string(&outcome).unwrap_or_default();
            assert_eq!(wire, want, "{outcome:?}");
        }
    }

    #[test]
    fn enum_wire_strings_are_pinned() {
        let cases: [(&str, String); 10] = [
            (
                "rank",
                serde_json::to_string(&ContainmentRank::Other).unwrap_or_default(),
            ),
            (
                "resolution",
                serde_json::to_string(&FromResolution::Sha).unwrap_or_default(),
            ),
            (
                "containment",
                serde_json::to_string(&ContainmentReason::AllCommitsPlaced).unwrap_or_default(),
            ),
            (
                "reanchored",
                serde_json::to_string(&FromResolution::Reanchored).unwrap_or_default(),
            ),
            (
                "net",
                serde_json::to_string(&ContainmentReason::MergeAddsNothing).unwrap_or_default(),
            ),
            (
                "agent",
                serde_json::to_string(&SessionAgent::Codex).unwrap_or_default(),
            ),
            (
                "outcome",
                serde_json::to_string(&OpOutcome::BranchDeleted).unwrap_or_default(),
            ),
            (
                "would_remove",
                serde_json::to_string(&OpOutcome::WouldRemove).unwrap_or_default(),
            ),
            (
                "would_delete_branch",
                serde_json::to_string(&OpOutcome::WouldDeleteBranch).unwrap_or_default(),
            ),
            (
                "train",
                serde_json::to_string(&TrainOutcome::Blocked {
                    blocked_by: vec![],
                    paths: vec![],
                })
                .unwrap_or_default(),
            ),
        ];
        let expected = [
            "\"other\"",
            "\"sha\"",
            "\"all_commits_placed\"",
            "\"reanchored\"",
            "\"merge_adds_nothing\"",
            "\"codex\"",
            "\"branch_deleted\"",
            "\"would_remove\"",
            "\"would_delete_branch\"",
            "{\"state\":\"blocked\",\"blocked_by\":[],\"paths\":[]}",
        ];
        for ((label, actual), want) in cases.iter().zip(expected) {
            assert_eq!(actual, want, "{label}");
        }
    }

    #[test]
    fn operation_reason_spelling_matches_wire() {
        let operations = [
            Operation::Merge,
            Operation::Rebase,
            Operation::CherryPick,
            Operation::Revert,
            Operation::Bisect,
        ];
        for operation in operations {
            let wire = serde_json::to_string(&operation).unwrap_or_default();
            assert_eq!(wire, format!("\"{}\"", operation.as_str()), "{operation:?}");
        }
    }
}
