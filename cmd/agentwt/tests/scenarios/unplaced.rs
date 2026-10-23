//! The `unplaced` query against throwaway repos: every history shape a restack reads its replay list from.

use serde::Deserialize;

use crate::common::Fixture;

const WORK: &str = "claude/work";
const MERGE_BRANCH: &str = "claude-orch/t1";
const SHARED: &str = "shared.txt";
const RESTACK: &str = "claude-restack/claude-work-0a1b2c3d";

/// The query document as the restack skill reads it; a renamed or missing field fails the parse.
#[derive(Debug, Deserialize)]
struct UnplacedDocument {
    schema: u32,
    branch: String,
    onto: String,
    onto_sha: String,
    onto_short_sha: String,
    commits: Vec<ListedCommit>,
}

/// One listed commit.
#[derive(Debug, Deserialize)]
struct ListedCommit {
    sha: String,
    short_sha: String,
    subject: String,
    is_merge: bool,
}

/// What a scenario expects of `unplaced --onto <onto> <branch>`: the listed subjects, oldest first.
struct Expect {
    onto: &'static str,
    branch: &'static str,
    subjects: Vec<&'static str>,
}

/// One named history.
struct Case {
    name: &'static str,
    build: fn(&mut Fixture) -> Expect,
}

/// `--format json unplaced --onto <onto> <branch>` parsed, panicking on a non-zero exit.
fn unplaced(fixture: &Fixture, onto: &str, branch: &str) -> UnplacedDocument {
    let output = fixture.run(&["--format", "json", "unplaced", "--onto", onto, branch]);
    assert!(
        output.status.success(),
        "unplaced --onto {onto} {branch} failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    serde_json::from_slice(&output.stdout).expect("unplaced document")
}

/// The shared file with `text` as its second line.
fn line2(text: &str) -> String {
    format!("line1\n{text}\nline3\n")
}

#[test]
fn unplaced_histories() {
    let cases = [
        Case {
            name: "all_unplaced",
            build: all_unplaced,
        },
        Case {
            name: "partial_pick",
            build: partial_pick,
        },
        Case {
            name: "resolved_pick_no_edit",
            build: resolved_pick_no_edit,
        },
        Case {
            name: "merge_commit_excluded",
            build: merge_commit_excluded,
        },
        Case {
            name: "non_agent_branch",
            build: non_agent_branch,
        },
        Case {
            name: "agent_target",
            build: agent_target,
        },
        Case {
            name: "base_commit_listed",
            build: base_commit_listed,
        },
        Case {
            name: "fully_placed",
            build: fully_placed,
        },
        Case {
            name: "same_branch",
            build: same_branch,
        },
        Case {
            name: "resume_on_work_branch",
            build: resume_on_work_branch,
        },
    ];
    let mut failures = Vec::new();
    for case in &cases {
        let mut fixture = Fixture::new();
        let expect = (case.build)(&mut fixture);
        fixture.checkout("main");
        let document = unplaced(&fixture, expect.onto, expect.branch);
        let listed: Vec<&str> = document
            .commits
            .iter()
            .map(|commit| commit.subject.as_str())
            .collect();
        if listed != expect.subjects {
            failures.push(format!(
                "{}: listed {listed:?}, want {:?}",
                case.name, expect.subjects
            ));
        }
    }
    assert!(failures.is_empty(), "{}", failures.join("\n"));
}

/// Two commits that exist nowhere else: both are listed, oldest first.
fn all_unplaced(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on(WORK, "second.txt", "second\n", "add second");
    Expect {
        onto: "main",
        branch: WORK,
        subjects: vec!["add first", "add second"],
    }
}

/// Three commits with only the second picked onto the target: the first and the third remain.
fn partial_pick(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    let picked = fixture.commit_on(WORK, "second.txt", "second\n", "add second");
    fixture.commit_on(WORK, "third.txt", "third\n", "add third");
    fixture.cherry_pick("main", &picked);
    Expect {
        onto: "main",
        branch: WORK,
        subjects: vec!["add first", "add third"],
    }
}

/// A conflicted pick finished by `git commit --no-edit` — the way the restack skill finishes one — keeps author,
/// author time and subject, so the original counts as placed.
fn resolved_pick_no_edit(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    fixture.commit_on("main", SHARED, &line2("line2 main"), "main line2");
    let conflicting = fixture.commit_on(WORK, SHARED, &line2("line2 work"), "edit line2");
    fixture.commit_on(WORK, "second.txt", "second\n", "add second");
    fixture.checkout("main");
    let repo = fixture.repo.clone();
    let pick = fixture.try_git_in(&repo, &["cherry-pick", &conflicting]);
    assert!(
        !pick.status.success(),
        "the pick of {conflicting} must conflict"
    );
    fixture.write(SHARED, &line2("line2 both"));
    fixture.git(&["add", "-A"]);
    fixture.git(&["commit", "-q", "--no-edit"]);
    Expect {
        onto: "main",
        branch: WORK,
        subjects: vec!["add second"],
    }
}

/// A merge commit on the branch is never listed; the commits around it are.
fn merge_commit_excluded(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    fixture.merge_no_ff(WORK, "main");
    fixture.commit_on(WORK, "second.txt", "second\n", "add second");
    Expect {
        onto: "main",
        branch: WORK,
        subjects: vec!["add first", "add second"],
    }
}

/// A non-agent branch is a valid subject: develop's catch-up onto main lists what main lacks.
fn non_agent_branch(fixture: &mut Fixture) -> Expect {
    fixture.branch("develop", "main");
    let picked = fixture.commit_on("develop", "first.txt", "first\n", "add first");
    fixture.commit_on("develop", "second.txt", "second\n", "add second");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    fixture.cherry_pick("main", &picked);
    Expect {
        onto: "main",
        branch: "develop",
        subjects: vec!["add second"],
    }
}

/// An agent-owned target — a fleet's merge branch — is asked like any other.
fn agent_target(fixture: &mut Fixture) -> Expect {
    fixture.branch(MERGE_BRANCH, "main");
    fixture.commit_on(MERGE_BRANCH, "landed.txt", "landed\n", "add landed");
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    Expect {
        onto: MERGE_BRANCH,
        branch: WORK,
        subjects: vec!["add first"],
    }
}

/// The list is drawn from what the target's history lacks: a base commit the target does not hold is listed too.
fn base_commit_listed(fixture: &mut Fixture) -> Expect {
    fixture.branch("develop", "main");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    fixture.branch(WORK, "main");
    fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    Expect {
        onto: "develop",
        branch: WORK,
        subjects: vec!["main advances", "add first"],
    }
}

/// The branch's one commit is picked onto the target: nothing is owed.
fn fully_placed(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    let picked = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    fixture.cherry_pick("main", &picked);
    Expect {
        onto: "main",
        branch: WORK,
        subjects: vec![],
    }
}

/// A branch asked against itself owes nothing.
fn same_branch(_fixture: &mut Fixture) -> Expect {
    Expect {
        onto: "main",
        branch: "main",
        subjects: vec![],
    }
}

/// A stopped restack's work branch already holds the first commit: asked against it, only the second is left.
fn resume_on_work_branch(fixture: &mut Fixture) -> Expect {
    fixture.branch(WORK, "main");
    let replayed = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.commit_on(WORK, "second.txt", "second\n", "add second");
    fixture.commit_on("main", "main.txt", "main\n", "main advances");
    fixture.branch(RESTACK, "main");
    fixture.cherry_pick(RESTACK, &replayed);
    Expect {
        onto: RESTACK,
        branch: WORK,
        subjects: vec!["add second"],
    }
}

#[test]
fn unplaced_refuses_unknown_names() {
    let fixture = Fixture::new();
    // (onto, branch, the name the refusal reports)
    let cases: [(&str, &str, &str); 2] = [
        ("main", "claude/nope", "claude/nope"),
        ("nope", "main", "nope"),
    ];
    for (onto, branch, named) in cases {
        let output = fixture.run(&["unplaced", "--onto", onto, branch]);
        assert_eq!(output.status.code(), Some(3), "--onto {onto} {branch}");
        let stderr = String::from_utf8_lossy(&output.stderr);
        assert!(
            stderr.contains(&format!("unknown branch {named}")),
            "{stderr}"
        );
        assert!(output.stdout.is_empty(), "--onto {onto} {branch}");
    }
}

/// A merge driver that exits non-zero on a bound file must not fail the query: the placement merges run on gix's
/// built-in drivers, so the conflicting commit is listed as unplaced instead of aborting the whole call.
#[test]
fn unplaced_ignores_external_merge_drivers() {
    let mut fixture = Fixture::new();
    fixture.write(".gitattributes", "v.txt merge=refuse\n");
    fixture.write("v.txt", "line1\nbase\nline3\n");
    fixture.git(&["add", "."]);
    fixture.commit("bind v.txt");
    fixture.git(&["config", "merge.refuse.driver", "false"]);
    fixture.branch(WORK, "main");
    let edit = fixture.commit_on(WORK, "v.txt", "line1\nwork\nline3\n", "work edit");
    fixture.commit_on("main", "v.txt", "line1\nmain\nline3\n", "main edit");
    fixture.checkout(WORK);

    let document = unplaced(&fixture, "main", WORK);
    let subjects: Vec<&str> = document
        .commits
        .iter()
        .map(|commit| commit.subject.as_str())
        .collect();
    assert_eq!(subjects, vec!["work edit"]);
    assert_eq!(document.commits[0].sha, edit);
}

#[test]
fn unplaced_document_names_the_target_tip() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    let added = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.checkout("main");
    let main_tip = fixture.git(&["rev-parse", "main"]);
    let document = unplaced(&fixture, "main", WORK);
    assert_eq!(document.schema, 1);
    assert_eq!(document.branch, WORK);
    assert_eq!(document.onto, "main");
    assert_eq!(document.onto_sha, main_tip);
    assert_eq!(document.onto_short_sha, &main_tip[..8]);
    let [commit] = document.commits.as_slice() else {
        panic!("one listed commit expected: {document:?}");
    };
    assert_eq!(commit.sha, added);
    assert_eq!(commit.short_sha, &added[..8]);
    assert_eq!(commit.subject, "add first");
    assert!(!commit.is_merge);
}

#[test]
fn unplaced_text_lists_short_sha_and_subject() {
    let mut fixture = Fixture::new();
    fixture.branch(WORK, "main");
    let added = fixture.commit_on(WORK, "first.txt", "first\n", "add first");
    fixture.checkout("main");
    let output = fixture.run(&["unplaced", "--onto", "main", WORK]);
    assert!(output.status.success());
    let text = String::from_utf8_lossy(&output.stdout);
    assert_eq!(
        text.lines().next(),
        Some(format!("{} add first", &added[..8]).as_str())
    );
}
