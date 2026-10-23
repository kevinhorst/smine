//! `forward` against throwaway repos: both real paths (checked-out worktree, update-ref) and every refusal.

use std::path::Path;
use std::process::Output;

use serde::Deserialize;

use crate::common::{Fixture, write_at};

const WORK: &str = "claude/work";
const TARGET: &str = "feature/x";

/// The forward document as consumers read it; a renamed field fails the parse.
#[derive(Debug, Deserialize)]
struct ForwardDocument {
    schema: u32,
    onto: String,
    from: String,
    to: String,
    outcome: String,
    reason: Option<String>,
}

/// `--format json forward <args…>` run in `cwd`: the exit code and the parsed document.
fn forward(fixture: &Fixture, cwd: &Path, args: &[&str]) -> (Option<i32>, ForwardDocument) {
    let mut all = vec!["--format", "json", "forward"];
    all.extend_from_slice(args);
    let output = fixture.run_in(cwd, &all);
    let document = serde_json::from_slice(&output.stdout).unwrap_or_else(|err| {
        panic!(
            "forward {args:?}: {err}; stdout {} stderr {}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        )
    });
    (output.status.code(), document)
}

fn stdout(output: &Output) -> String {
    String::from_utf8_lossy(&output.stdout)
        .trim_end()
        .to_owned()
}

fn stderr(output: &Output) -> String {
    String::from_utf8_lossy(&output.stderr).into_owned()
}

/// The target at main's tip and the work branch one commit (`first.txt`) ahead of it; returns `(old tip, new tip)`.
fn one_ahead(fixture: &mut Fixture) -> (String, String) {
    fixture.branch(TARGET, "main");
    fixture.branch(WORK, TARGET);
    let tip = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    let old = fixture.git(&["rev-parse", TARGET]);
    (old, tip)
}

/// The landing case: the target is checked out in the main checkout, the session sits in a linked worktree and
/// names its own `HEAD`. Ref, index and files of the main checkout move together.
#[test]
fn forward_moves_a_checked_out_target_with_its_files() {
    let mut fixture = Fixture::new();
    let (old, tip) = one_ahead(&mut fixture);
    fixture.checkout(TARGET);
    let session = fixture.worktree(WORK, "session");
    assert!(!fixture.repo.join("first.txt").exists());

    let (code, document) = forward(&fixture, &session, &["--onto", TARGET, "HEAD"]);
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.schema, 1);
    assert_eq!(document.onto, TARGET);
    assert_eq!(document.from, old);
    assert_eq!(document.to, tip, "HEAD is the caller's checkout's");
    assert_eq!(document.outcome, "forwarded");
    assert_eq!(document.reason, None);
    assert_eq!(fixture.git(&["rev-parse", TARGET]), tip);
    assert_eq!(fixture.git(&["rev-parse", "HEAD"]), tip);
    assert_eq!(
        fixture.git(&["status", "--porcelain"]),
        "",
        "the checkout's index and files followed the ref"
    );
    assert!(fixture.repo.join("first.txt").exists());
}

/// Tracked changes in the target's worktree: skipped, nothing moves, the edit survives — even though the edited
/// file is not one the forward touches.
#[test]
fn forward_onto_dirty_checkout_is_skipped() {
    let mut fixture = Fixture::new();
    let (old, tip) = one_ahead(&mut fixture);
    fixture.checkout(TARGET);
    write_at(&fixture.repo, "base.txt", "edited\n");

    let repo = fixture.repo.clone();
    let (code, document) = forward(&fixture, &repo, &["--onto", TARGET, &tip]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "skipped");
    let reason = document.reason.expect("reason");
    assert!(
        reason.starts_with("feature/x is checked out in ")
            && reason.ends_with(" with tracked changes"),
        "{reason}"
    );
    assert_eq!(fixture.git(&["rev-parse", TARGET]), old);
    assert_eq!(
        std::fs::read_to_string(fixture.repo.join("base.txt")).expect("read"),
        "edited\n"
    );
}

/// An untracked file the forward would overwrite: git refuses the move in the checkout, the ref stays, the file
/// keeps its content, nothing is in progress.
#[test]
fn forward_git_refusal_in_checkout_is_skipped() {
    let mut fixture = Fixture::new();
    let (old, tip) = one_ahead(&mut fixture);
    fixture.checkout(TARGET);
    write_at(&fixture.repo, "first.txt", "mine\n");

    let repo = fixture.repo.clone();
    let (code, document) = forward(&fixture, &repo, &["--onto", TARGET, &tip]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "skipped");
    assert!(
        document.reason.as_deref().is_some_and(|r| !r.is_empty()),
        "{document:?}"
    );
    assert_eq!(fixture.git(&["rev-parse", TARGET]), old);
    assert_eq!(
        std::fs::read_to_string(fixture.repo.join("first.txt")).expect("read"),
        "mine\n"
    );
    assert_eq!(fixture.git(&["status", "--porcelain"]), "?? first.txt");
}

/// The target is checked out nowhere: the ref moves, no checkout is touched.
#[test]
fn forward_moves_a_target_not_checked_out() {
    let mut fixture = Fixture::new();
    let (old, tip) = one_ahead(&mut fixture);
    fixture.checkout(WORK);

    let output = fixture.run(&["forward", "--onto", TARGET, WORK]);
    assert_eq!(output.status.code(), Some(0), "{}", stderr(&output));
    assert_eq!(
        stdout(&output),
        format!("forwarded {TARGET} {}..{}", &old[..8], &tip[..8])
    );
    assert_eq!(fixture.git(&["rev-parse", TARGET]), tip);
    assert_eq!(fixture.git(&["status", "--porcelain"]), "");
    let reflog = fixture.git(&["reflog", "show", "-1", "--format=%gs", TARGET]);
    assert_eq!(reflog, format!("agentwt forward: {}", &tip[..8]));
}

/// The target moved on after the branch was cut: the commit no longer holds its tip — skipped on both paths,
/// never a merge commit, never a forced move.
#[test]
fn forward_refuses_a_commit_that_lacks_the_tip() {
    let mut fixture = Fixture::new();
    let (_, tip) = one_ahead(&mut fixture);
    let moved = fixture.commit_on(TARGET, "other.txt", "other\n", "target advances");
    let reason = format!("{} does not hold the tip of {TARGET}", &tip[..8]);

    // (label, branch the main checkout sits on)
    let cases = [("checked-out", TARGET), ("not-checked-out", "main")];
    for (label, checkout) in cases {
        fixture.checkout(checkout);
        let repo = fixture.repo.clone();
        let (code, document) = forward(&fixture, &repo, &["--onto", TARGET, &tip]);
        assert_eq!(code, Some(3), "{label}: {document:?}");
        assert_eq!(document.outcome, "skipped", "{label}");
        assert_eq!(document.reason.as_deref(), Some(reason.as_str()), "{label}");
        assert_eq!(fixture.git(&["rev-parse", TARGET]), moved, "{label}");
    }

    let output = fixture.run(&["forward", "--onto", TARGET, &tip]);
    assert_eq!(output.status.code(), Some(3));
    assert_eq!(stdout(&output), format!("skipped {TARGET}: {reason}"));

    // Backwards is no fast-forward either.
    let (code, document) = forward(&fixture, &fixture.repo.clone(), &["--onto", TARGET, "main"]);
    assert_eq!(code, Some(3), "{document:?}");
    assert_eq!(document.outcome, "skipped");
}

/// The target already points at the commit: nothing to do, even with a dirty checkout.
#[test]
fn forward_to_the_same_commit_is_unchanged() {
    let mut fixture = Fixture::new();
    let (old, _) = one_ahead(&mut fixture);
    fixture.checkout(TARGET);
    write_at(&fixture.repo, "base.txt", "edited\n");

    let repo = fixture.repo.clone();
    let (code, document) = forward(&fixture, &repo, &["--onto", TARGET, "main"]);
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.outcome, "unchanged");
    assert_eq!(document.reason, None);
    assert_eq!(fixture.git(&["rev-parse", TARGET]), old);

    let output = fixture.run(&["forward", "--onto", TARGET, "main"]);
    assert_eq!(
        stdout(&output),
        format!("unchanged {TARGET} at {}", &old[..8])
    );
}

#[test]
fn forward_refuses_unknown_names() {
    let mut fixture = Fixture::new();
    let (_, tip) = one_ahead(&mut fixture);
    // (onto, commit, the refusal)
    let cases: [(&str, &str, &str); 2] = [
        ("nope", &tip, "unknown branch nope"),
        (TARGET, "deadbeef", "unknown commit deadbeef"),
    ];
    for (onto, commit, refusal) in cases {
        let output = fixture.run(&["forward", "--onto", onto, commit]);
        assert_eq!(output.status.code(), Some(3), "--onto {onto} {commit}");
        assert!(stderr(&output).contains(refusal), "{}", stderr(&output));
        assert!(output.stdout.is_empty(), "--onto {onto} {commit}");
    }
    let output = fixture.run(&["forward", "--onto=", &tip]);
    assert_eq!(output.status.code(), Some(2));
}
