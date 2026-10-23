//! `probe` against throwaway repos: one call over many commits, every outcome, nothing moved, and the refusals.

use serde::Deserialize;

use crate::common::Fixture;

const WORK: &str = "claude/work";
const SHARED: &str = "shared.txt";

/// The probe document as consumers read it; a renamed field fails the parse.
#[derive(Debug, Deserialize)]
struct ProbeDocument {
    schema: u32,
    onto: String,
    onto_sha: String,
    commits: Vec<ProbeEntry>,
}

#[derive(Debug, Deserialize)]
struct ProbeEntry {
    commit: String,
    outcome: String,
    reason: Option<String>,
}

/// The shared file with `text` as its second line.
fn line2(text: &str) -> String {
    format!("line1\n{text}\nline3\n")
}

/// `--format json probe <args…>`: the exit code and the parsed document.
fn probe(fixture: &Fixture, args: &[&str]) -> (Option<i32>, ProbeDocument) {
    let mut all = vec!["--format", "json", "probe"];
    all.extend_from_slice(args);
    let output = fixture.run(&all);
    let document = serde_json::from_slice(&output.stdout).unwrap_or_else(|err| {
        panic!(
            "probe {args:?}: {err}; stdout {} stderr {}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        )
    });
    (output.status.code(), document)
}

/// Three work commits: a new file (applies), two edits of one line main also edited (the second conflicts), plus a
/// merge commit and a commit main already holds (skipped).
#[test]
fn probe_answers_every_commit_in_order() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    let first = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on(WORK, SHARED, &line2("first"), "first edit");
    let second = fixture.commit_on(WORK, SHARED, &line2("second"), "second edit");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    let merge = fixture.merge_no_ff(WORK, "main");
    let held = fixture.git(&["rev-parse", "main"]);
    fixture.checkout(WORK);
    let main_tip = fixture.git(&["rev-parse", "main"]);

    let (code, document) = probe(
        &fixture,
        &["--onto", "main", &first[..8], &second, &merge, &held],
    );
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.schema, 1);
    assert_eq!(document.onto, "main");
    assert_eq!(document.onto_sha, main_tip);
    let entries: Vec<(&str, &str, Option<&str>)> = document
        .commits
        .iter()
        .map(|entry| {
            (
                entry.commit.as_str(),
                entry.outcome.as_str(),
                entry.reason.as_deref(),
            )
        })
        .collect();
    assert_eq!(
        entries,
        vec![
            (first.as_str(), "applies", None),
            (second.as_str(), "conflict", Some("conflicts in shared.txt")),
            (merge.as_str(), "skipped", Some("merge commit")),
            (held.as_str(), "skipped", Some("already on main")),
        ]
    );
    assert_eq!(
        fixture.git(&["rev-parse", "main"]),
        main_tip,
        "probe moved main"
    );
    assert_eq!(
        fixture.git(&["status", "--porcelain"]),
        "",
        "worktree clean"
    );

    let output = fixture.run(&["probe", "--onto", "main", &first, &second, &merge]);
    assert_eq!(output.status.code(), Some(0));
    assert_eq!(
        String::from_utf8_lossy(&output.stdout).trim_end(),
        format!(
            "applies {} onto main\nconflict {} onto main: conflicts in shared.txt\nskipped {}: merge commit",
            &first[..8],
            &second[..8],
            &merge[..8]
        )
    );
}

/// A conflicting edit of a file bound to a merge driver applies: merge-tree reads the binding from the tip's tree even
/// when the checkout holds no `.gitattributes`. The driver takes theirs; one that keeps ours would leave the tip's
/// tree unchanged, which containment already answers as `already on main`.
#[test]
fn probe_applies_through_merge_driver() {
    let mut fixture = Fixture::new();
    fixture.write(".gitattributes", "v.txt merge=theirs\n");
    fixture.write("v.txt", &line2("base"));
    fixture.git(&["add", "."]);
    fixture.commit("bind v.txt");
    fixture.git(&["config", "merge.theirs.driver", "cp %B %A"]);
    fixture.branch(WORK, "main");
    let edit = fixture.commit_on(WORK, "v.txt", &line2("work"), "work edit");
    fixture.commit_on("main", "v.txt", &line2("main"), "main edit");
    std::fs::remove_file(fixture.repo.join(".gitattributes")).expect("remove worktree attributes");
    let main_tip = fixture.git(&["rev-parse", "main"]);

    let merge_base = format!("--merge-base={edit}^");
    let bare = fixture.try_git_in(
        &fixture.repo.clone(),
        &[
            "merge-tree",
            "--write-tree",
            "--name-only",
            "--no-messages",
            &merge_base,
            &main_tip,
            &edit,
        ],
    );
    assert_eq!(
        bare.status.code(),
        Some(1),
        "without an attribute source the edit conflicts"
    );

    let (code, document) = probe(&fixture, &["--onto", "main", &edit]);
    assert_eq!(code, Some(0), "{document:?}");
    assert_eq!(document.commits[0].outcome, "applies");
}

#[test]
fn probe_refuses_unknown_names() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    let sha = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    // (onto, commits, the refusal)
    let cases: [(&str, Vec<&str>, &str); 2] = [
        ("nope", vec![&sha], "unknown branch nope"),
        ("main", vec![&sha, "deadbeef"], "unknown commit deadbeef"),
    ];
    for (onto, commits, refusal) in cases {
        let mut args = vec!["probe", "--onto", onto];
        args.extend(commits);
        let output = fixture.run(&args);
        assert_eq!(output.status.code(), Some(3), "{args:?}");
        let stderr = String::from_utf8_lossy(&output.stderr);
        assert!(stderr.contains(refusal), "{stderr}");
        assert!(output.stdout.is_empty(), "{args:?}");
    }
    let output = fixture.run(&["probe", "--onto", "main"]);
    assert_eq!(output.status.code(), Some(2), "no commit is a usage error");
}
