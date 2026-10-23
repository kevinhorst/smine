//! `pick` against throwaway repos: both real paths (checked-out worktree, update-ref) and every refusal.

use std::process::Output;

use serde::Deserialize;

use crate::common::{Fixture, write_at};

const WORK: &str = "claude/work";
const SHARED: &str = "shared.txt";

/// The pick document as consumers read it; a renamed field fails the parse.
#[derive(Debug, Deserialize)]
struct PickDocument {
    schema: u32,
    commit: String,
    onto: String,
    outcome: String,
    new_commit: Option<String>,
    reason: Option<String>,
}

/// The shared file with `text` as its second line.
fn line2(text: &str) -> String {
    format!("line1\n{text}\nline3\n")
}

/// `--format json pick <args…>`: the exit code and the parsed document.
fn pick(fixture: &Fixture, args: &[&str]) -> (Option<i32>, PickDocument) {
    let mut all = vec!["--format", "json", "pick"];
    all.extend_from_slice(args);
    let output = fixture.run(&all);
    let document = serde_json::from_slice(&output.stdout).unwrap_or_else(|err| {
        panic!(
            "pick {args:?}: {err}; stdout {} stderr {}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        )
    });
    (output.status.code(), document)
}

fn stderr(output: &Output) -> String {
    String::from_utf8_lossy(&output.stderr).into_owned()
}

/// A commit on the work branch adding a new file: applies onto main.
fn new_file_commit(fixture: &mut Fixture) -> String {
    fixture.branch(WORK, "main");
    let sha = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    sha
}

/// `main` is checked out in the main checkout: the pick runs there with `-x`; a short id resolves to the full sha.
#[test]
fn pick_onto_checked_out_branch() {
    let mut fixture = Fixture::new();
    let sha = new_file_commit(&mut fixture);
    fixture.checkout("main");
    let old_tip = fixture.git(&["rev-parse", "main"]);

    let (code, document) = pick(&fixture, &["--onto", "main", &sha[..8]]);
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.schema, 1);
    assert_eq!(document.commit, sha, "resolved to the full sha");
    assert_eq!(document.onto, "main");
    assert_eq!(document.outcome, "picked");
    assert_eq!(document.reason, None);
    let new_commit = document.new_commit.expect("new_commit");
    assert_eq!(fixture.git(&["rev-parse", "main"]), new_commit);
    assert_eq!(fixture.git(&["rev-parse", "main^"]), old_tip);
    let message = fixture.git(&["show", "-s", "--format=%B", &new_commit]);
    assert!(
        message.contains(&format!("(cherry picked from commit {sha})")),
        "{message}"
    );
    assert_eq!(
        fixture.git(&["status", "--porcelain"]),
        "",
        "worktree clean"
    );
    assert!(fixture.repo.join("first.txt").exists());
}

/// Tracked changes in the target's worktree: skipped, nothing moves.
#[test]
fn pick_onto_dirty_checkout_is_skipped() {
    let mut fixture = Fixture::new();
    let sha = new_file_commit(&mut fixture);
    fixture.checkout("main");
    let old_tip = fixture.git(&["rev-parse", "main"]);
    write_at(&fixture.repo, "base.txt", "edited\n");

    let (code, document) = pick(&fixture, &["--onto", "main", &sha]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "skipped");
    let reason = document.reason.expect("reason");
    assert!(
        reason.starts_with("main is checked out in ") && reason.ends_with(" with tracked changes"),
        "{reason}"
    );
    assert_eq!(fixture.git(&["rev-parse", "main"]), old_tip);
    assert_eq!(
        std::fs::read_to_string(fixture.repo.join("base.txt")).expect("read"),
        "edited\n"
    );
}

/// The target is checked out nowhere: commit-tree plus update-ref, the original author kept.
#[test]
fn pick_onto_branch_not_checked_out() {
    let mut fixture = Fixture::new();
    fixture.branch("develop", "main");
    fixture.branch(WORK, "main");
    fixture.checkout(WORK);
    fixture.write("first.txt", "first\n");
    fixture.git(&["add", "-A"]);
    fixture.git(&[
        "commit",
        "-q",
        "--author",
        "Ada Author <ada@example.com>",
        "-m",
        "add first",
        "-m",
        "body line",
    ]);
    let sha = fixture.git(&["rev-parse", "HEAD"]);
    let old_tip = fixture.git(&["rev-parse", "develop"]);

    let output = fixture.run(&["pick", "--onto", "develop", &sha]);
    assert_eq!(output.status.code(), Some(0), "{}", stderr(&output));
    let new_commit = fixture.git(&["rev-parse", "develop"]);
    assert_eq!(
        String::from_utf8_lossy(&output.stdout).trim_end(),
        format!("picked {} onto develop as {}", &sha[..8], &new_commit[..8])
    );
    assert_eq!(fixture.git(&["rev-parse", "develop^"]), old_tip);
    let mut author = |rev: &str| fixture.git(&["show", "-s", "--format=%an <%ae> %at", rev]);
    let (original, picked) = (author(&sha), author(&new_commit));
    assert_eq!(picked, original);
    assert!(
        original.starts_with("Ada Author <ada@example.com>"),
        "{original}"
    );
    let message = fixture.git(&["show", "-s", "--format=%B", &new_commit]);
    assert_eq!(
        message,
        format!("add first\n\nbody line\n\n(cherry picked from commit {sha})")
    );
    assert_eq!(
        fixture.git(&["show", "develop:first.txt"]),
        "first",
        "the picked tree"
    );
    assert_eq!(
        fixture.git(&["status", "--porcelain"]),
        "",
        "work checkout untouched"
    );

    // Placed by the pick trailer: a second pick is skipped.
    let (code, document) = pick(&fixture, &["--onto", "develop", &sha]);
    assert_eq!(code, Some(3));
    assert_eq!(document.outcome, "skipped");
    assert_eq!(document.reason.as_deref(), Some("already on develop"));
}

/// The target is checked out nowhere and the edit conflicts textually, but the file is bound to a driver that takes
/// theirs: commit-tree picks it and the target gets the work line. A keep-ours driver would leave the tip's tree
/// unchanged, which containment already answers as `already on develop`.
#[test]
fn pick_commit_tree_through_merge_driver() {
    let mut fixture = Fixture::new();
    fixture.write(".gitattributes", "v.txt merge=theirs\n");
    fixture.write("v.txt", &line2("base"));
    fixture.git(&["add", "."]);
    fixture.commit("bind v.txt");
    fixture.git(&["config", "merge.theirs.driver", "cp %B %A"]);
    fixture.branch(WORK, "main");
    let edit = fixture.commit_on(WORK, "v.txt", &line2("work"), "work edit");
    fixture.commit_on("main", "v.txt", &line2("main"), "main edit");
    fixture.branch("develop", "main");
    let old_tip = fixture.git(&["rev-parse", "develop"]);

    let (code, document) = pick(&fixture, &["--onto", "develop", &edit]);
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.outcome, "picked");
    let new_commit = document.new_commit.expect("new_commit");
    assert_eq!(fixture.git(&["rev-parse", "develop"]), new_commit);
    assert_eq!(fixture.git(&["rev-parse", "develop^"]), old_tip);
    assert_eq!(
        fixture.git(&["show", "develop:v.txt"]),
        line2("work").trim_end(),
        "the driver took theirs"
    );
}

/// A conflict on the update-ref path leaves the ref where it was.
#[test]
fn pick_conflict_leaves_ref_untouched() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, SHARED, &line2("first"), "first edit");
    let second = fixture.commit_on(WORK, SHARED, &line2("second"), "second edit");
    fixture.branch("develop", "main");
    let old_tip = fixture.git(&["rev-parse", "develop"]);

    let (code, document) = pick(&fixture, &["--onto", "develop", &second]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "conflict");
    assert_eq!(document.reason.as_deref(), Some("conflicts in shared.txt"));
    assert_eq!(fixture.git(&["rev-parse", "develop"]), old_tip);
}

/// A conflict in the checked-out worktree is aborted: nothing in progress, the branch unmoved.
#[test]
fn pick_conflict_in_worktree_is_aborted() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, SHARED, &line2("first"), "first edit");
    let second = fixture.commit_on(WORK, SHARED, &line2("second"), "second edit");
    fixture.checkout("main");
    let old_tip = fixture.git(&["rev-parse", "main"]);

    let (code, document) = pick(&fixture, &["--onto", "main", &second]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "conflict");
    assert_eq!(document.reason.as_deref(), Some("conflicts in shared.txt"));
    assert_eq!(fixture.git(&["rev-parse", "main"]), old_tip);
    assert_eq!(fixture.git(&["status", "--porcelain"]), "");
    let git_dir = fixture.git(&["rev-parse", "--absolute-git-dir"]);
    assert!(
        !std::path::Path::new(&git_dir)
            .join("CHERRY_PICK_HEAD")
            .exists()
    );
}

#[test]
fn pick_skips_contained_and_merge_commits() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    let first = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    let merge = fixture.merge_no_ff(WORK, "main");
    let base = fixture.git(&["rev-list", "--max-parents=0", "main"]);
    fixture.merge_no_ff("main", WORK);
    fixture.branch("develop", "main~1");
    let ancestor = fixture.git(&["rev-parse", "main~1"]);

    // (commit, onto, reason)
    let cases: [(&str, &str, &str); 4] = [
        (&first, "main", "already on main"),
        (&ancestor, "develop", "already on develop"),
        (&merge, "develop", "merge commit"),
        (&base, "develop", "root commit"),
    ];
    for (commit, onto, reason) in cases {
        let (code, document) = pick(&fixture, &["--onto", onto, commit]);
        assert_eq!(code, Some(3), "{commit} onto {onto}: {document:?}");
        assert_eq!(document.outcome, "skipped", "{commit} onto {onto}");
        assert_eq!(
            document.reason.as_deref(),
            Some(reason),
            "{commit} onto {onto}"
        );
    }

    let output = fixture.run(&["pick", "--onto", "develop", &merge]);
    assert_eq!(output.status.code(), Some(3));
    assert_eq!(
        String::from_utf8_lossy(&output.stdout).trim_end(),
        format!("skipped {}: merge commit", &merge[..8])
    );
}

/// A root commit the target does not hold: there is no parent to pick against.
#[test]
fn pick_skips_root_commit() {
    let mut fixture = Fixture::new();
    fixture.git(&["checkout", "-q", "--orphan", "claude/orphan"]);
    fixture.git(&["rm", "-q", "-r", "--cached", "."]);
    fixture.write("orphan.txt", "orphan\n");
    fixture.git(&["add", "orphan.txt"]);
    let root = fixture.commit("orphan root");
    let (code, document) = pick(&fixture, &["--onto", "main", &root]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "skipped");
    assert_eq!(document.reason.as_deref(), Some("root commit"));
}

#[test]
fn pick_refuses_unknown_names() {
    let mut fixture = Fixture::new();
    let sha = new_file_commit(&mut fixture);
    // (onto, commit, the refusal)
    let cases: [(&str, &str, String); 2] = [
        ("nope", &sha, "unknown branch nope".to_owned()),
        ("main", "deadbeef", "unknown commit deadbeef".to_owned()),
    ];
    for (onto, commit, refusal) in cases {
        let output = fixture.run(&["pick", "--onto", onto, commit]);
        assert_eq!(output.status.code(), Some(3), "--onto {onto} {commit}");
        assert!(stderr(&output).contains(&refusal), "{}", stderr(&output));
        assert!(output.stdout.is_empty(), "--onto {onto} {commit}");
    }
    let output = fixture.run(&["pick", "--onto=", &sha]);
    assert_eq!(output.status.code(), Some(2));
}
