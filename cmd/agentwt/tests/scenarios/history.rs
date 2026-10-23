//! History-shape catalog: one throwaway repo per case, each stating what the status document must say about one agent
//! branch — containment, unpicked commits, base, worktree operation, unsafe reasons. One `#[test]` per family, so
//! families run in parallel; a family reports every failing case at once. A `limitation:` comment marks a pinned shape
//! the binary cannot prove held: the case asserts today's answer so a later change to it is deliberate.

use serde_json::Value;

use crate::common::{Fixture, containment};

const WORK: &str = "claude/work";
const SHARED: &str = "shared.txt";

/// What the document must say about `branch`; `None` fields are not checked.
struct Expect {
    branch: &'static str,
    /// `(candidate, reason, rank)`; `None` inside means no candidate holds the work.
    containment: Option<(&'static str, &'static str, &'static str)>,
    /// Subjects of the unpicked commits, newest first.
    unpicked: Vec<&'static str>,
    /// `(base name, resolution)`.
    base: Option<(Option<&'static str>, &'static str)>,
    /// The row's worktree operation.
    operation: Option<&'static str>,
    /// Exact unsafe reasons.
    unsafe_reasons: Option<Vec<&'static str>>,
    /// One reason the unsafe reasons must include.
    unsafe_contains: Option<&'static str>,
    /// Number of detached rows in the document.
    detached_rows: Option<usize>,
}

impl Expect {
    /// Containment and unpicked only — the common case.
    fn held(
        branch: &'static str,
        containment: Option<(&'static str, &'static str, &'static str)>,
        unpicked: Vec<&'static str>,
    ) -> Expect {
        Expect {
            branch,
            containment,
            unpicked,
            base: None,
            operation: None,
            unsafe_reasons: None,
            unsafe_contains: None,
            detached_rows: None,
        }
    }
}

/// One history shape.
struct Case {
    name: &'static str,
    build: fn(&mut Fixture) -> Expect,
}

/// Builds every case in its own repo, collects every mismatch, fails once with the full list.
fn run(cases: &[Case]) {
    let mut failures = Vec::new();
    'cases: for case in cases {
        let mut f = Fixture::new();
        let expect = (case.build)(&mut f);
        f.checkout("main");
        let out = f.run(&["--format", "json", "status"]);
        if !out.status.success() {
            failures.push(format!(
                "{}: status failed: {}",
                case.name,
                String::from_utf8_lossy(&out.stderr)
                    .lines()
                    .take(3)
                    .collect::<Vec<_>>()
                    .join(" | ")
            ));
            continue 'cases;
        }
        let doc: Value = serde_json::from_slice(&out.stdout).expect("status json");
        let Some(actual) = doc["rows"].as_array().and_then(|rows| {
            rows.iter()
                .find(|r| r["branch"].as_str() == Some(expect.branch))
        }) else {
            failures.push(format!("{}: no row for {}", case.name, expect.branch));
            continue 'cases;
        };
        failures.extend(
            check(&doc, actual, &expect)
                .into_iter()
                .map(|mismatch| format!("{}: {mismatch}", case.name)),
        );
    }
    assert!(
        failures.is_empty(),
        "{} mismatch(es):\n{}",
        failures.len(),
        failures.join("\n")
    );
}

/// Every field of `expect` that differs from `actual`.
fn check(doc: &Value, actual: &Value, expect: &Expect) -> Vec<String> {
    let mut mismatches = Vec::new();
    let want = expect
        .containment
        .map(|(branch, reason, rank)| (branch.to_owned(), reason.to_owned(), rank.to_owned()));
    if containment(actual) != want {
        mismatches.push(format!(
            "containment {:?}, want {want:?}",
            containment(actual)
        ));
    }
    let unpicked = unpicked_subjects(actual);
    if unpicked != expect.unpicked {
        mismatches.push(format!("unpicked {unpicked:?}, want {:?}", expect.unpicked));
    }
    if let Some((name, resolution)) = expect.base
        && (actual["from"]["name"].as_str() != name || actual["from"]["resolution"] != resolution)
    {
        mismatches.push(format!(
            "base {}, want {name:?}/{resolution}",
            actual["from"]
        ));
    }
    if let Some(operation) = expect.operation
        && actual["worktree"]["operation"] != operation
    {
        mismatches.push(format!(
            "operation {}, want {operation}",
            actual["worktree"]["operation"]
        ));
    }
    if let Some(reasons) = &expect.unsafe_reasons
        && actual["unsafe_reasons"] != serde_json::json!(reasons)
    {
        mismatches.push(format!(
            "unsafe_reasons {}, want {reasons:?}",
            actual["unsafe_reasons"]
        ));
    }
    if let Some(reason) = expect.unsafe_contains {
        let present = actual["unsafe_reasons"]
            .as_array()
            .is_some_and(|reasons| reasons.iter().any(|r| r == reason));
        if !present {
            mismatches.push(format!(
                "unsafe_reasons {} lack {reason}",
                actual["unsafe_reasons"]
            ));
        }
    }
    if let Some(count) = expect.detached_rows {
        let detached = doc["rows"].as_array().map_or(0, |rows| {
            rows.iter().filter(|r| r["branch"].is_null()).count()
        });
        if detached != count {
            mismatches.push(format!("detached rows {detached}, want {count}"));
        }
    }
    mismatches
}

/// Subjects of `unpicked_commits`, in the `commits.ahead` order (newest first).
fn unpicked_subjects(actual: &Value) -> Vec<String> {
    let shas: Vec<&str> = actual["unpicked_commits"]
        .as_array()
        .map(|list| list.iter().filter_map(Value::as_str).collect())
        .unwrap_or_default();
    actual["commits"]["ahead"]
        .as_array()
        .into_iter()
        .flatten()
        .filter(|entry| entry["sha"].as_str().is_some_and(|sha| shas.contains(&sha)))
        .filter_map(|entry| entry["subject"].as_str().map(str::to_owned))
        .collect()
}

/// `shared.txt` with `line2` replaced.
fn shared_with_line2(line2: &str) -> String {
    format!("line1\n{line2}\nline3\n")
}

// ---------------------------------------------------------------------------------------------------------------------
// Rebase family
// ---------------------------------------------------------------------------------------------------------------------

#[test]
fn rebase_histories() {
    run(&[
        Case {
            name: "rebase_unmerged",
            build: rebase_unmerged,
        },
        Case {
            name: "rebase_ff_merged",
            build: rebase_ff_merged,
        },
        Case {
            name: "rebase_drops_picked",
            build: rebase_drops_picked,
        },
        Case {
            name: "rebase_drops_all_picked",
            build: rebase_drops_all_picked,
        },
        Case {
            name: "rebased_then_picked_then_edited",
            build: rebased_then_picked_then_edited,
        },
        Case {
            name: "reordered_after_pick",
            build: reordered_after_pick,
        },
        Case {
            name: "reordered_after_pick_then_edited",
            build: reordered_after_pick_then_edited,
        },
        Case {
            name: "squashed_after_pick",
            build: squashed_after_pick,
        },
        Case {
            name: "squashed_after_pick_then_edited",
            build: squashed_after_pick_then_edited,
        },
        Case {
            name: "rebased_onto_other_candidate",
            build: rebased_onto_other_candidate,
        },
        Case {
            name: "reset_reapplied_onto_other",
            build: reset_reapplied_onto_other,
        },
        Case {
            name: "reset_undo_own_commit",
            build: reset_undo_own_commit,
        },
        Case {
            name: "merged_back_then_rebased",
            build: merged_back_then_rebased,
        },
    ]);
}

/// X and Y rebased onto a moved main, never merged: both still exist only here.
fn rebase_unmerged(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.rebase(WORK, "main");
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held(WORK, None, vec!["add y", "add x"])
    }
}

/// Rebased, then fast-forwarded into main: the tip is main's history.
fn rebase_ff_merged(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.rebase(WORK, "main");
    f.checkout("main");
    f.git(&["merge", "-q", "--ff-only", WORK]);
    Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
}

/// X picked to main, then the branch rebased onto main: git drops X, only Y is left.
fn rebase_drops_picked(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.cherry_pick("main", &x);
    f.rebase(WORK, "main");
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held(WORK, None, vec!["add y"])
    }
}

/// Every commit picked, then rebased: git drops them all and the branch sits on main.
fn rebase_drops_all_picked(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    let y = f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.cherry_pick("main", &x);
    f.cherry_pick("main", &y);
    f.rebase(WORK, "main");
    Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
}

/// Rebased, the rebased commits picked, then main edits the picked line again: the twin places Y.
fn rebased_then_picked_then_edited(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on(WORK, SHARED, &shared_with_line2("line2 work"), "edit line2");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.rebase(WORK, "main");
    let x2 = f.git(&["rev-parse", &format!("{WORK}~1")]);
    let y2 = f.git(&["rev-parse", WORK]);
    f.cherry_pick("main", &x2);
    f.cherry_pick("main", &y2);
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 again"),
        "edit line2 again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// X (line 1) and Y (line 3) picked, then the branch rebuilt from the fork as Y, X.
fn reorder_after_pick(f: &mut Fixture) {
    let fork = f.git(&["rev-parse", "main"]);
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, "line1 x\nline2\nline3\n", "edit line1");
    let y = f.commit_on(WORK, SHARED, "line1 x\nline2\nline3 y\n", "edit line3");
    f.cherry_pick("main", &x);
    f.cherry_pick("main", &y);
    f.reset_hard(WORK, &fork);
    f.cherry_pick(WORK, &y);
    f.cherry_pick(WORK, &x);
}

/// Reordered after the pick, main unchanged since: each rewritten commit re-applies as a no-op.
fn reordered_after_pick(f: &mut Fixture) -> Expect {
    reorder_after_pick(f);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// Reordered after the pick, then main edits both lines again: each rewrite keeps its hunk, and the patch-id ignores
/// context lines, so the picks are still twins.
fn reordered_after_pick_then_edited(f: &mut Fixture) -> Expect {
    reorder_after_pick(f);
    f.commit_on(
        "main",
        SHARED,
        "line1 again\nline2\nline3 again\n",
        "edit lines again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// X and Y picked, then the branch squashes them into S.
fn squash_after_pick(f: &mut Fixture) {
    let fork = f.git(&["rev-parse", "main"]);
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, "line1 x\nline2\nline3\n", "edit line1");
    let y = f.commit_on(WORK, SHARED, "line1 x\nline2\nline3 y\n", "edit line3");
    f.cherry_pick("main", &x);
    f.cherry_pick("main", &y);
    f.squash_branch(WORK, &fork, "squash");
}

/// Squashed after the pick, main unchanged since: S re-applies as a no-op.
fn squashed_after_pick(f: &mut Fixture) -> Expect {
    squash_after_pick(f);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// limitation: squashed after the pick, then main edits the lines again — no signal proves S held.
fn squashed_after_pick_then_edited(f: &mut Fixture) -> Expect {
    squash_after_pick(f);
    f.commit_on(
        "main",
        SHARED,
        "line1 again\nline2\nline3 y\n",
        "edit line1 again",
    );
    Expect::held(WORK, None, vec!["squash"])
}

/// Created from main, moved onto develop with `rebase --onto`, merged into develop.
fn rebased_onto_other_candidate(f: &mut Fixture) -> Expect {
    f.branch("develop", "main");
    f.commit_on("develop", "d.txt", "d\n", "develop work");
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.rebase_onto(WORK, "develop", "main");
    f.merge_no_ff("develop", WORK);
    Expect {
        base: Some((Some("develop"), "reanchored")),
        ..Expect::held(WORK, Some(("develop", "ancestor", "base")), vec![])
    }
}

/// Created from develop, reset onto main and X re-applied: the base is main now.
fn reset_reapplied_onto_other(f: &mut Fixture) -> Expect {
    f.branch("develop", "main");
    f.commit_on("develop", "d.txt", "d\n", "develop work");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.branch(WORK, "develop");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.reset_hard(WORK, "main");
    f.cherry_pick(WORK, &x);
    Expect {
        base: Some((Some("main"), "reanchored")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// `reset --hard HEAD~1` onto the branch's own commit: not a re-anchor.
fn reset_undo_own_commit(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.reset_hard(WORK, "HEAD~1");
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// Main merged in, then the branch rebased onto main: linear again, X still only here.
fn merged_back_then_rebased(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.merge_no_ff(WORK, "main");
    f.rebase(WORK, "main");
    Expect::held(WORK, None, vec!["add x"])
}

// ---------------------------------------------------------------------------------------------------------------------
// Pick family
// ---------------------------------------------------------------------------------------------------------------------

#[test]
fn pick_histories() {
    run(&[
        Case {
            name: "partial_pick",
            build: partial_pick,
        },
        Case {
            name: "split_across_candidates",
            build: split_across_candidates,
        },
        Case {
            name: "pick_then_branch_edits",
            build: pick_then_branch_edits,
        },
        Case {
            name: "pick_resolved",
            build: pick_resolved,
        },
        Case {
            name: "pick_resolved_with_trailer",
            build: pick_resolved_with_trailer,
        },
        Case {
            name: "amended_after_resolved_pick",
            build: amended_after_resolved_pick,
        },
        Case {
            name: "rebased_after_resolved_pick",
            build: rebased_after_resolved_pick,
        },
        Case {
            name: "pick_reverted_on_main",
            build: pick_reverted_on_main,
        },
        Case {
            name: "pick_after_rename",
            build: pick_after_rename,
        },
        Case {
            name: "pick_after_rename_then_edited",
            build: pick_after_rename_then_edited,
        },
        Case {
            name: "pick_whitespace_twin",
            build: pick_whitespace_twin,
        },
        Case {
            name: "pick_binary",
            build: pick_binary,
        },
        Case {
            name: "empty_commit_with_pick",
            build: empty_commit_with_pick,
        },
        Case {
            name: "net_zero_branch",
            build: net_zero_branch,
        },
        Case {
            name: "amended_after_pick",
            build: amended_after_pick,
        },
        Case {
            name: "reworded_after_pick",
            build: reworded_after_pick,
        },
        Case {
            name: "pick_with_trailer",
            build: pick_with_trailer,
        },
        Case {
            name: "twin_beyond_500",
            build: twin_beyond_500,
        },
    ]);
}

/// X, Y, Z with only Y picked: X and Z are unpicked.
fn partial_pick(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    let y = f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.commit_on(WORK, "z.txt", "z\n", "add z");
    f.cherry_pick("main", &y);
    Expect::held(WORK, None, vec!["add z", "add x"])
}

/// X picked to main, Y to release/a: every commit is somewhere, no one branch holds all.
fn split_across_candidates(f: &mut Fixture) -> Expect {
    f.branch("release/a", "main");
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    let y = f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.cherry_pick("main", &x);
    f.cherry_pick("release/a", &y);
    Expect {
        unsafe_reasons: Some(vec!["work contained nowhere", "no worktree"]),
        ..Expect::held(WORK, None, vec![])
    }
}

/// X picked, then the branch edits X's line again: only the new edit is unpicked.
fn pick_then_branch_edits(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 x"), "edit line2");
    f.cherry_pick("main", &x);
    f.commit_on(
        WORK,
        SHARED,
        &shared_with_line2("line2 y"),
        "edit line2 again",
    );
    Expect::held(WORK, None, vec!["edit line2 again"])
}

/// Main and the branch both edit line 2; X is picked with a conflict resolved to "line2 both". Returns X.
fn resolved_pick(f: &mut Fixture, extra: &[&str]) -> String {
    f.branch(WORK, "main");
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 main"),
        "main line2",
    );
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 work"), "edit line2");
    f.cherry_pick_resolving("main", &x, extra, SHARED, &shared_with_line2("line2 both"));
    x
}

/// A pick with conflict resolution counts as picked (same author, author time and subject, committed later).
fn pick_resolved(f: &mut Fixture) -> Expect {
    resolved_pick(f, &[]);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// A resolved `-x` pick whose subject was reworded on main: the trailer still names X.
fn pick_resolved_with_trailer(f: &mut Fixture) -> Expect {
    let x = resolved_pick(f, &["-x"]);
    f.checkout("main");
    f.git(&[
        "commit",
        "-q",
        "--amend",
        "-m",
        &format!("picked x reworded\n\n(cherry picked from commit {x})"),
    ]);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// A resolved pick, then the branch amends X: the amended content is not on main.
fn amended_after_resolved_pick(f: &mut Fixture) -> Expect {
    resolved_pick(f, &[]);
    f.amend_on(
        WORK,
        SHARED,
        &shared_with_line2("line2 work more"),
        "edit line2",
    );
    Expect::held(WORK, None, vec!["edit line2"])
}

/// limitation: a resolved pick, then the branch rebases onto main resolving differently — the rewrite is newer.
fn rebased_after_resolved_pick(f: &mut Fixture) -> Expect {
    resolved_pick(f, &[]);
    f.checkout(WORK);
    let repo = f.repo.clone();
    let rebase = f.try_git_in(&repo, &["rebase", "main"]);
    assert!(!rebase.status.success(), "the rebase must conflict");
    f.write(SHARED, &shared_with_line2("line2 rebased"));
    f.git(&["add", "-A"]);
    f.git(&["rebase", "--continue"]);
    Expect::held(WORK, None, vec!["edit line2"])
}

/// X picked, then main reverts the pick: the change stays in main's history (plan D4).
fn pick_reverted_on_main(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    let pick = f.cherry_pick("main", &x);
    f.revert_on("main", &pick);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// Main renames the file before the pick; the pick lands on the new path.
fn rename_then_pick(f: &mut Fixture) {
    f.branch(WORK, "main");
    f.rename_on("main", SHARED, "moved.txt", "move shared");
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 x"), "edit line2");
    f.cherry_pick("main", &x);
}

/// Picked across a rename, main unchanged since.
fn pick_after_rename(f: &mut Fixture) -> Expect {
    rename_then_pick(f);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// Picked across a rename, then main edits the line again: only the replay identity places it.
fn pick_after_rename_then_edited(f: &mut Fixture) -> Expect {
    rename_then_pick(f);
    f.commit_on(
        "main",
        "moved.txt",
        &shared_with_line2("line2 again"),
        "edit moved again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// Main commits the same change with other spacing, then edits the line again: the whitespace-blind twin places X.
fn pick_whitespace_twin(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, SHARED, &shared_with_line2("line2 a b"), "edit line2");
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2  a  b"),
        "edit line2",
    );
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 again"),
        "edit line2 again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// A binary change picked, then main changes the file again: the blob-pair twin places X.
fn pick_binary(f: &mut Fixture) -> Expect {
    f.commit_on("main", "bin.dat", "\0base\u{1}", "add bin");
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "bin.dat", "\0work\u{2}", "change bin");
    f.cherry_pick("main", &x);
    f.commit_on("main", "bin.dat", "\0again\u{3}", "change bin again");
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// An empty commit plus a picked X: the empty commit carries no work.
fn empty_commit_with_pick(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.empty_commit_on(WORK, "empty");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.cherry_pick("main", &x);
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// X, then its revert: the branch's net change is nothing.
fn net_zero_branch(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.revert_on(WORK, &x);
    Expect::held(WORK, Some(("main", "merge_adds_nothing", "base")), vec![])
}

/// X picked, then the branch amends X's content: the amended content is not on main.
fn amended_after_pick(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.cherry_pick("main", &x);
    f.amend_on(WORK, "x.txt", "x more\n", "add x");
    Expect::held(WORK, None, vec!["add x"])
}

/// X picked, the branch rewords X, main edits the line again: the patch-id twin places it.
fn reworded_after_pick(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 x"), "edit line2");
    f.cherry_pick("main", &x);
    f.checkout(WORK);
    f.git(&["commit", "-q", "--amend", "-m", "edit line2 reworded"]);
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 again"),
        "edit line2 again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// A `-x` pick, then main edits the line again.
fn pick_with_trailer(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 x"), "edit line2");
    f.cherry_pick_with("main", &x, &["-x"]);
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 again"),
        "edit line2 again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// X picked right after the fork, 510 main commits since, then main edits the line again.
fn twin_beyond_500(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let x = f.commit_on(WORK, SHARED, &shared_with_line2("line2 x"), "edit line2");
    f.cherry_pick("main", &x);
    f.bulk_commits("main", 510);
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 again"),
        "edit line2 again",
    );
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

// ---------------------------------------------------------------------------------------------------------------------
// Merge family
// ---------------------------------------------------------------------------------------------------------------------

#[test]
fn merge_histories() {
    run(&[
        Case {
            name: "squash_merged_disjoint",
            build: squash_merged_disjoint,
        },
        Case {
            name: "squash_merged_overlapping",
            build: squash_merged_overlapping,
        },
        Case {
            name: "squash_merged_then_edited",
            build: squash_merged_then_edited,
        },
        Case {
            name: "conflict_merged_no_ff",
            build: conflict_merged_no_ff,
        },
        Case {
            name: "merged_via_agent_branch",
            build: merged_via_agent_branch,
        },
        Case {
            name: "stacked_on_squashed",
            build: stacked_on_squashed,
        },
        Case {
            name: "from_develop_merges_main",
            build: from_develop_merges_main,
        },
        Case {
            name: "merged_back_then_merged",
            build: merged_back_then_merged,
        },
    ]);
}

/// X and Y touch separate files, squash-merged: each re-applies as a no-op.
fn squash_merged_disjoint(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "a.txt", "a\n", "add a");
    f.commit_on(WORK, "b.txt", "b\n", "add b");
    f.squash_merge("main", WORK, "squash work");
    Expect::held(WORK, Some(("main", "all_commits_placed", "base")), vec![])
}

/// X adds c.txt, Y edits it; squash-merged into main.
fn squash_overlapping(f: &mut Fixture) {
    f.branch(WORK, "main");
    f.commit_on(WORK, "c.txt", "c1\nc2\n", "add c");
    f.commit_on(WORK, "c.txt", "c1\nc2 y\n", "edit c");
    f.squash_merge("main", WORK, "squash work");
}

/// X no longer re-applies on its own, but merging the branch adds nothing.
fn squash_merged_overlapping(f: &mut Fixture) -> Expect {
    squash_overlapping(f);
    Expect::held(WORK, Some(("main", "merge_adds_nothing", "base")), vec![])
}

/// limitation: squash-merged, then main edits the squashed lines — no signal proves the branch held.
fn squash_merged_then_edited(f: &mut Fixture) -> Expect {
    squash_overlapping(f);
    f.commit_on("main", "c.txt", "c1\nc2 again\n", "edit c again");
    Expect::held(WORK, None, vec!["edit c", "add c"])
}

/// Both sides edit line 2, main merges the branch resolving the conflict.
fn conflict_merged_no_ff(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, SHARED, &shared_with_line2("line2 work"), "edit line2");
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 main"),
        "main line2",
    );
    f.checkout("main");
    let repo = f.repo.clone();
    let merge = f.try_git_in(&repo, &["merge", "--no-ff", WORK]);
    assert!(!merge.status.success(), "the merge must conflict");
    f.write(SHARED, &shared_with_line2("line2 both"));
    f.git(&["add", "-A"]);
    f.git(&["commit", "-q", "--no-edit"]);
    Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
}

/// Merged into another agent branch, which main merged.
fn merged_via_agent_branch(f: &mut Fixture) -> Expect {
    f.branch("claude/int", "main");
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.merge_no_ff("claude/int", WORK);
    f.merge_no_ff("main", "claude/int");
    Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
}

/// claude/a squash-merged; claude/b stacked on claude/a keeps its own Y unpicked only.
fn stacked_on_squashed(f: &mut Fixture) -> Expect {
    f.branch("claude/a", "main");
    f.commit_on("claude/a", "x.txt", "x\n", "add x");
    f.squash_merge("main", "claude/a", "squash a");
    f.branch("claude/b", "claude/a");
    f.commit_on("claude/b", "y.txt", "y\n", "add y");
    Expect {
        base: Some((Some("main"), "chain")),
        ..Expect::held("claude/b", None, vec!["add y"])
    }
}

/// Created from develop, merges main: main's commits are placed on main, only X is unpicked.
fn from_develop_merges_main(f: &mut Fixture) -> Expect {
    f.branch("develop", "main");
    f.commit_on("develop", "d.txt", "d\n", "develop work");
    f.branch(WORK, "develop");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on("main", "m1.txt", "m1\n", "main one");
    f.commit_on("main", "m2.txt", "m2\n", "main two");
    f.merge_no_ff(WORK, "main");
    Expect {
        base: Some((Some("develop"), "ref")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// Main merged in, then the branch merged into main.
fn merged_back_then_merged(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on("main", "m.txt", "m\n", "main moves");
    f.merge_no_ff(WORK, "main");
    f.merge_no_ff("main", WORK);
    Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
}

// ---------------------------------------------------------------------------------------------------------------------
// Base family
// ---------------------------------------------------------------------------------------------------------------------

#[test]
fn base_histories() {
    run(&[
        Case {
            name: "created_from_head",
            build: created_from_head,
        },
        Case {
            name: "created_from_remote",
            build: created_from_remote,
        },
        Case {
            name: "created_from_name_at_equal_tips",
            build: created_from_name_at_equal_tips,
        },
        Case {
            name: "no_origin_line",
            build: no_origin_line,
        },
        Case {
            name: "renamed_branch",
            build: renamed_branch,
        },
        Case {
            name: "chain_through_deleted_parent",
            build: chain_through_deleted_parent,
        },
        Case {
            name: "branch_force_moved",
            build: branch_force_moved,
        },
        Case {
            name: "created_from_agent_revision_expression",
            build: created_from_agent_revision_expression,
        },
        Case {
            name: "created_from_revision_expression",
            build: created_from_revision_expression,
        },
        Case {
            name: "created_from_ide_suffix_renamed",
            build: created_from_ide_suffix_renamed,
        },
    ]);
}

/// `git switch -c` from main: the reflog says `Created from HEAD`.
fn created_from_head(f: &mut Fixture) -> Expect {
    f.switch_create(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    Expect {
        base: Some((Some("main"), "sha")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// Created from `origin/main`, merged into main: the base is the local main.
fn created_from_remote(f: &mut Fixture) -> Expect {
    f.remote_ref("main", "main");
    f.git(&["branch", "-q", "--no-track", WORK, "origin/main"]);
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.merge_no_ff("main", WORK);
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held(WORK, Some(("main", "ancestor", "base")), vec![])
    }
}

/// Created by name from `feature/x`, which still sits at main's tip: the creation line wins over containment, where
/// main would win the tie.
fn created_from_name_at_equal_tips(f: &mut Fixture) -> Expect {
    f.branch("feature/x", "main");
    f.git(&["branch", "-q", "--no-track", WORK, "feature/x"]);
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    Expect {
        base: Some((Some("feature/x"), "ref")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// A ref with no creation line, merged into main: base unknown, containment still found through the default.
fn no_origin_line(f: &mut Fixture) -> Expect {
    f.git(&["checkout", "-q", "--detach", "main"]);
    f.write("x.txt", "x\n");
    f.git(&["add", "-A"]);
    let x = f.commit("add x");
    f.branch_without_origin(WORK, &x);
    f.merge_no_ff("main", WORK);
    Expect {
        base: Some((None, "unknown")),
        ..Expect::held(WORK, Some(("main", "ancestor", "default")), vec![])
    }
}

/// Renamed after creation: the reflog moves with the branch.
fn renamed_branch(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.checkout("main");
    f.rename_branch(WORK, "claude/renamed");
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held("claude/renamed", None, vec!["add x"])
    }
}

/// claude/b from claude/a from main, claude/a deleted: the chain resolves through b's start commit.
fn chain_through_deleted_parent(f: &mut Fixture) -> Expect {
    f.branch("claude/a", "main");
    f.commit_on("claude/a", "x.txt", "x\n", "add x");
    f.branch("claude/b", "claude/a");
    f.commit_on("claude/b", "y.txt", "y\n", "add y");
    f.checkout("main");
    f.delete_branch("claude/a");
    Expect {
        base: Some((Some("main"), "chain")),
        ..Expect::held("claude/b", None, vec!["add y", "add x"])
    }
}

/// `branch -f` moves the branch onto develop.
fn branch_force_moved(f: &mut Fixture) -> Expect {
    f.branch("develop", "main");
    f.commit_on("develop", "d.txt", "d\n", "develop work");
    f.branch(WORK, "main");
    f.checkout("main");
    f.branch_force(WORK, "develop");
    Expect {
        base: Some((Some("develop"), "reanchored")),
        ..Expect::held(WORK, Some(("develop", "ancestor", "base")), vec![])
    }
}

/// Created from `claude/a^`: the revision suffix is cut and the chain follows `claude/a` to its base, main.
fn created_from_agent_revision_expression(f: &mut Fixture) -> Expect {
    f.branch("claude/a", "main");
    f.commit_on("claude/a", "a.txt", "a\n", "add a");
    f.checkout("main");
    f.branch(WORK, "claude/a^");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    Expect {
        base: Some((Some("main"), "chain")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// Created from `main^`: the revision suffix is cut and the base is main by name, the branch one behind it.
fn created_from_revision_expression(f: &mut Fixture) -> Expect {
    f.commit_on("main", "m.txt", "m\n", "main work");
    f.branch(WORK, "main^");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    Expect {
        base: Some((Some("main"), "ref")),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}

/// An IDE records `Created from <ref>^0`: a feature branch created that way from a session branch and renamed into
/// the agent namespace by a restack follows the chain through the session branch to main.
fn created_from_ide_suffix_renamed(f: &mut Fixture) -> Expect {
    f.branch("claude/a", "main");
    f.commit_on("claude/a", "a.txt", "a\n", "add a");
    f.checkout("main");
    f.branch("feature/x", "claude/a^0");
    f.commit_on("feature/x", "x.txt", "x\n", "add x");
    f.checkout("main");
    f.rename_branch("feature/x", "claude-restack/feature-x-12345678-orig");
    Expect {
        base: Some((Some("main"), "chain")),
        ..Expect::held(
            "claude-restack/feature-x-12345678-orig",
            None,
            vec!["add x", "add a"],
        )
    }
}

// ---------------------------------------------------------------------------------------------------------------------
// Worktree family
// ---------------------------------------------------------------------------------------------------------------------

#[test]
fn worktree_histories() {
    run(&[
        Case {
            name: "rebase_in_progress",
            build: rebase_in_progress,
        },
        Case {
            name: "bisect_in_progress",
            build: bisect_in_progress,
        },
        Case {
            name: "cherry_pick_in_progress",
            build: cherry_pick_in_progress,
        },
        Case {
            name: "plain_detached",
            build: plain_detached,
        },
    ]);
}

/// A rebase stopped on a conflict in the branch's worktree.
fn rebase_in_progress(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, SHARED, &shared_with_line2("line2 work"), "edit line2");
    f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 main"),
        "main line2",
    );
    let wt = f.worktree(WORK, "work");
    f.start_conflicting_rebase(&wt, "main");
    Expect {
        operation: Some("rebase"),
        unsafe_contains: Some("operation(rebase)"),
        detached_rows: Some(0),
        ..Expect::held(WORK, None, vec!["edit line2"])
    }
}

/// A bisect running in the branch's worktree.
fn bisect_in_progress(f: &mut Fixture) -> Expect {
    let fork = f.git(&["rev-parse", "main"]);
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.commit_on(WORK, "y.txt", "y\n", "add y");
    f.checkout("main");
    let wt = f.worktree(WORK, "work");
    f.start_bisect(&wt, WORK, &fork);
    Expect {
        operation: Some("bisect"),
        unsafe_contains: Some("operation(bisect)"),
        detached_rows: Some(0),
        ..Expect::held(WORK, None, vec!["add y", "add x"])
    }
}

/// A cherry-pick stopped on a conflict in the branch's worktree (HEAD stays on the branch).
fn cherry_pick_in_progress(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    let main_edit = f.commit_on(
        "main",
        SHARED,
        &shared_with_line2("line2 main"),
        "main line2",
    );
    f.commit_on(WORK, SHARED, &shared_with_line2("line2 work"), "edit line2");
    f.checkout("main");
    let wt = f.worktree(WORK, "work");
    f.start_conflicting_pick(&wt, &main_edit);
    Expect {
        operation: Some("cherry_pick"),
        unsafe_contains: Some("operation(cherry_pick)"),
        detached_rows: Some(0),
        ..Expect::held(WORK, None, vec!["edit line2"])
    }
}

/// A plain detached checkout of the branch tip: a detached row, the branch row without a worktree.
fn plain_detached(f: &mut Fixture) -> Expect {
    f.branch(WORK, "main");
    f.commit_on(WORK, "x.txt", "x\n", "add x");
    f.checkout("main");
    let wt = f.worktree(WORK, "work");
    f.git_in(&wt, &["checkout", "-q", "--detach"]);
    Expect {
        unsafe_reasons: Some(vec!["unpicked(1)", "no worktree"]),
        detached_rows: Some(1),
        ..Expect::held(WORK, None, vec!["add x"])
    }
}
