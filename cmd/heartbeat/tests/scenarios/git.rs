//! Git state end to end: a real repo with agent worktrees, the daemon's git edge, agentwt in-process.

use std::path::Path;
use std::time::Duration;

use serde_json::{Value, json};

use crate::common::git::GitRepo;
use crate::common::{Daemon, Home, sensor};

/// Starts the daemon with `repo` registered and waits for its first scan.
fn start(home: &Home, repo: &GitRepo, config: &Value) -> Daemon {
    let args = home.git_args(&repo.path, config);
    let refs: Vec<&str> = args.iter().map(String::as_str).collect();
    let daemon = Daemon::start_with(home.path(), &refs);
    daemon.wait_for("the first scan", |doc| {
        doc["heartbeat"]["scans_completed"].as_u64() >= Some(1)
    });
    daemon
}

/// A `PostToolUse` body of `tool` in `cwd`; no `transcript_path`, which P1's foreign-home guard accepts.
fn tool_body(session: &str, cwd: &Path, tool: &str, input: &Value) -> String {
    json!({
        "session_id": session,
        "hook_event_name": "PostToolUse",
        "cwd": cwd,
        "tool_name": tool,
        "tool_input": input,
    })
    .to_string()
}

fn default_config() -> Value {
    json!({
        "exempt_paths": ["plans/**", "**/changelog.json", "**/CHANGELOG", "**/CHANGELOG.md"],
        "version_keys": ["version"]
    })
}

fn branch<'a>(doc: &'a Value, name: &str) -> &'a Value {
    doc["branches"]
        .as_array()
        .and_then(|rows| rows.iter().find(|row| row["branch"] == name))
        .unwrap_or_else(|| panic!("no branch {name}: {doc:#}"))
}

/// Branch states and train positions agree with the agentwt document of the same repo.
#[test]
fn branch_states_match_agentwt() {
    let repo = GitRepo::new();
    let merged = repo.worktree("merged", "main");
    repo.commit(&merged, "m.txt", "m\n", "merged work");
    repo.git(
        &repo.path,
        &[
            "merge",
            "-q",
            "--no-ff",
            "-m",
            "merge merged",
            "claude/merged",
        ],
    );
    let dirty = repo.worktree("dirty", "main");
    repo.write(&dirty, "untracked.txt", "u\n");
    let ready = repo.worktree("ready", "main");
    repo.commit(&ready, "a.txt", "a\n", "ready work");
    for name in ["conflict-1", "conflict-2"] {
        let path = repo.worktree(name, "main");
        repo.commit(
            &path,
            "shared.txt",
            &format!("line1\n{name}\nline3\n"),
            name,
        );
    }
    repo.worktree("empty", "main");

    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = daemon.wait_for("six branch rows", |doc| {
        doc["branches"]
            .as_array()
            .is_some_and(|rows| rows.len() == 6)
    });
    let cases = [
        ("claude/merged", "removable"),
        ("claude/dirty", "uncommitted"),
        ("claude/ready", "merge_ready"),
        ("claude/empty", "no_work"),
    ];
    for (name, want) in cases {
        assert_eq!(branch(&doc, name)["state"], want, "{name}");
    }
    let pair: Vec<&Value> = ["claude/conflict-1", "claude/conflict-2"]
        .iter()
        .map(|name| &branch(&doc, name)["state"])
        .collect();
    assert!(
        pair.contains(&&json!("merge_ready")) && pair.contains(&&json!("conflicting")),
        "{pair:?}"
    );

    let options = agentwt::status::Options {
        repo: repo.path.clone(),
        claude_homes: vec![home.path().to_path_buf()],
        codex_home: None,
        skip_order: false,
        branches: None,
        train_bases: Vec::new(),
        candidates: Vec::new(),
    };
    let document = agentwt::status::build(&options).expect("agentwt");
    for row in &document.rows {
        let name = row.branch.as_deref().expect("agent branch");
        let ours = branch(&doc, name);
        assert!(
            ours.get("target").is_none(),
            "{name}: a main-based branch has no landing target"
        );
        let position = row.order.as_ref().map(|order| json!(order.train_position));
        assert_eq!(ours.get("train_position").cloned(), position, "{name}");
    }
}

/// One commit in a worktree plus its `PostToolUse` hook cost exactly one scan.
#[test]
fn commit_triggers_one_scan() {
    let repo = GitRepo::new();
    let work = repo.worktree("work", "main");
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    std::thread::sleep(Duration::from_millis(1_000));
    let before = daemon.document()["heartbeat"]["scans_completed"]
        .as_u64()
        .expect("counter");

    repo.commit(&work, "w.txt", "w\n", "work");
    let body = tool_body(
        "s-git",
        &work,
        "Bash",
        &json!({ "command": "git commit -m work" }),
    );
    assert_eq!(daemon.post(&body), 200);
    let doc = daemon.wait_for("rescan applied", |doc| {
        branch(doc, "claude/work")["state"] == "merge_ready"
    });
    std::thread::sleep(Duration::from_millis(1_500));
    let after = daemon.document();
    assert_eq!(
        after["heartbeat"]["scans_completed"].as_u64(),
        Some(before + 1),
        "{doc:#}"
    );
    assert_eq!(after["heartbeat"]["scan_in_flight_high_water"], 1);
}

/// `status --worktree .` from a subdirectory keeps only that worktree's branches row; a worktree of an unregistered
/// repository keeps none.
#[test]
fn status_worktree_filter() {
    let repo = GitRepo::new();
    let a = repo.worktree("a", "main");
    repo.worktree("b", "main");
    repo.write(&a, "src/x.txt", "x\n");
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    daemon.wait_for("two branch rows", |doc| {
        doc["branches"]
            .as_array()
            .is_some_and(|rows| rows.len() == 2)
    });
    let branches = |cwd: &Path| -> Value {
        let output = daemon.cli_in(cwd, &["status", "--worktree", ".", "--format", "json"]);
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let doc: Value = serde_json::from_slice(&output.stdout).expect("status json");
        doc["branches"].clone()
    };
    let present = branches(&a.join("src"));
    assert_eq!(
        present.as_array().map(|rows| rows
            .iter()
            .map(|row| row["branch"].clone())
            .collect::<Vec<_>>()),
        Some(vec![json!("claude/a")]),
        "{present:#}"
    );
    let other = GitRepo::new();
    assert_eq!(
        branches(&other.path),
        json!([]),
        "unregistered repo: no row"
    );
}

/// The fixture plans collide at every level on `src/shared.rs`, and nowhere on exempt paths or version bumps.
#[test]
fn collisions_by_level() {
    let repo = GitRepo::new();
    repo.commit(
        &repo.path,
        "src/shared.rs",
        "pub fn parse(input: &str) -> Vec<String> {\n    vec![]\n}\n",
        "shared.rs",
    );
    let mut roots = Vec::new();
    for (name, plan, body) in [
        ("alpha", include_str!("../fixtures/plans/alpha.md"), "alpha"),
        ("beta", include_str!("../fixtures/plans/beta.md"), "beta"),
    ] {
        let root = repo.worktree(name, "main");
        let plan_file = format!(
            "plans/2026-09-27-{name}-{hash}/2026-09-27-{name}-wip-{hash}/design/raw.md",
            hash = "a".repeat(7)
        );
        repo.write(&root, &plan_file, plan);
        let content = format!(
            "pub fn parse(input: &str) -> Vec<String> {{\n    vec![\"{body}\".into()]\n}}\n"
        );
        repo.commit(&root, "src/shared.rs", &content, name);
        roots.push(root);
    }
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    for root in &roots {
        for (file, old, new) in [
            ("src/shared.rs", "vec![]", "vec![1]"),
            ("skills/x/SKILL.md", "version: 1.0.0", "version: 1.3.0"),
            (
                "plans/2026-09-27-alpha-aaaaaaa/2026-09-27-alpha-wip-aaaaaaa/design/raw.md",
                "a",
                "b",
            ),
        ] {
            let input =
                json!({ "file_path": root.join(file), "old_string": old, "new_string": new });
            assert_eq!(daemon.post(&tool_body("s-edit", root, "Edit", &input)), 200);
        }
    }
    let doc = daemon.wait_for("four levels", |doc| {
        doc["collisions"]
            .as_array()
            .is_some_and(|rows| rows.len() == 4)
    });
    let rows = doc["collisions"].as_array().expect("collisions");
    for level in ["planned_file", "planned_symbol", "touched", "confirmed"] {
        let row = rows
            .iter()
            .find(|row| row["level"] == level)
            .unwrap_or_else(|| panic!("{level}: {doc:#}"));
        assert_eq!(row["paths"], json!(["src/shared.rs"]), "{level}");
        assert_eq!(
            (row["a"].as_str(), row["b"].as_str()),
            (Some("claude/alpha"), Some("claude/beta")),
            "{level}"
        );
    }
    let symbol = rows
        .iter()
        .find(|row| row["level"] == "planned_symbol")
        .expect("symbol");
    assert_eq!(symbol["symbols"], json!(["src/shared.rs::parse"]));
    assert_eq!(doc["work_items"].as_array().map(Vec::len), Some(2));
}

/// A plan the default branch archived after a worktree forked is not that worktree's work; a plan it adds is.
#[test]
fn archived_plan_is_no_work_item() {
    let repo = GitRepo::new();
    let archived = "plans/2026-09-01-old-bbbbbbb/2026-09-01-old-wip-bbbbbbb/design/raw.md";
    repo.commit(&repo.path, archived, "# Old\n", "old plan");
    let old = repo.worktree("old", "main");
    repo.git(&repo.path, &["rm", "-q", "-r", "plans"]);
    repo.git(&repo.path, &["commit", "-q", "-m", "archive old plan"]);
    repo.write(
        &old,
        CONTROL_PLAN,
        include_str!("../fixtures/plans/alpha.md"),
    );
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = wait_for_work(&daemon);
    assert_eq!(plan_dirs(&doc), [CONTROL_DIR], "{doc:#}");
}

/// A plan file the worktree added; its plan dir is the one work item the negative scenarios expect.
const CONTROL_PLAN: &str = "plans/2026-09-27-new-ccccccc/2026-09-27-new-wip-ccccccc/design/raw.md";

/// The work-item `plan_dir` of [`CONTROL_PLAN`].
const CONTROL_DIR: &str = "2026-09-27-new-ccccccc/2026-09-27-new-wip-ccccccc";

/// A held plan file at the new depth, committed on the default branch.
const HELD_PLAN: &str = "plans/2026-09-01-held-ddddddd/2026-09-01-held-wip-ddddddd/design/held.md";

/// The Changes of [`HELD_PLAN`] as the default branch holds it.
const HELD_TEXT: &str = "# Held\n\n## Changes\n\n| File | Kind | Entry |\n| --- | --- | --- |\n| `src/old.rs` | modified | §1 |\n";

/// Waits for at least one work item.
fn wait_for_work(daemon: &Daemon) -> Value {
    daemon.wait_for("a work item", |doc| {
        doc["work_items"]
            .as_array()
            .is_some_and(|rows| !rows.is_empty())
    })
}

/// The `plan_dir` of every work item.
fn plan_dirs(doc: &Value) -> Vec<&str> {
    doc["work_items"]
        .as_array()
        .expect("work items")
        .iter()
        .filter_map(|row| row["plan_dir"].as_str())
        .collect()
}

/// A held plan the worktree changed is that worktree's work, counted whole: its planned files are the whole Changes
/// table, the held rows included.
#[test]
fn changed_held_plan_counts_whole() {
    let repo = GitRepo::new();
    repo.commit(&repo.path, HELD_PLAN, HELD_TEXT, "held plan");
    let root = repo.worktree("held", "main");
    repo.write(
        &root,
        HELD_PLAN,
        &format!("{HELD_TEXT}| `src/new.rs` | modified | §2 |\n"),
    );
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = wait_for_work(&daemon);
    let rows = doc["work_items"].as_array().expect("work items");
    assert_eq!(rows.len(), 1, "{doc:#}");
    assert_eq!(
        rows[0]["plan_dir"], "2026-09-01-held-ddddddd/2026-09-01-held-wip-ddddddd",
        "{doc:#}"
    );
    assert_eq!(
        rows[0]["planned_files"],
        json!(["src/new.rs", "src/old.rs"]),
        "{doc:#}"
    );
}

/// A held plan the worktree left unchanged is no work item; a plan the same worktree added still is.
#[test]
fn unchanged_held_plan_counts_nothing() {
    let repo = GitRepo::new();
    repo.commit(&repo.path, HELD_PLAN, HELD_TEXT, "held plan");
    let root = repo.worktree("held", "main");
    repo.write(
        &root,
        CONTROL_PLAN,
        include_str!("../fixtures/plans/alpha.md"),
    );
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = wait_for_work(&daemon);
    assert_eq!(plan_dirs(&doc), [CONTROL_DIR], "{doc:#}");
}

/// A plan dir directly under `plans`, outside any feature folder, is no work item.
#[test]
fn old_depth_plan_is_no_work_item() {
    let repo = GitRepo::new();
    let root = repo.worktree("old-depth", "main");
    repo.write(
        &root,
        "plans/2026-09-27-flat-wip-eeeeeee/design/raw.md",
        include_str!("../fixtures/plans/beta.md"),
    );
    repo.write(
        &root,
        CONTROL_PLAN,
        include_str!("../fixtures/plans/alpha.md"),
    );
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = wait_for_work(&daemon);
    assert_eq!(plan_dirs(&doc), [CONTROL_DIR], "{doc:#}");
}

/// A registry not written yet (no repo registered) is an empty one: the git sensor is ok and serves no git rows.
#[test]
fn missing_registry_is_empty() {
    let home = Home::new();
    let missing = home.path().join("missing.json");
    let daemon = Daemon::start_with(home.path(), &["--repos", missing.to_str().expect("utf-8")]);
    let doc = daemon.wait_for("git sensor", |doc| sensor(doc, "git").is_some());
    let git = sensor(&doc, "git").expect("git sensor");
    assert_eq!(git["state"], "ok", "{git}");
    assert_eq!(doc["branches"], json!([]));
    assert_eq!(doc["repos"], json!([]));
}

/// A repo registered after start is scanned and served without a restart; one deregistered leaves the rows.
#[test]
fn registry_change_is_followed_live() {
    let repo = GitRepo::new();
    let home = Home::new();
    let registry = home.path().join("repos.json");
    let daemon = Daemon::start_with(home.path(), &["--repos", registry.to_str().expect("utf-8")]);
    let doc = daemon.wait_for("git sensor", |doc| sensor(doc, "git").is_some());
    assert_eq!(doc["repos"], json!([]));
    let body = json!({ "repos": [{ "name": "late", "path": repo.path }] });
    std::fs::write(&registry, body.to_string()).expect("write registry");
    let doc = daemon.wait_for("the late repo's scan", |doc| {
        doc["heartbeat"]["scans_completed"].as_u64() >= Some(1)
    });
    assert_eq!(doc["repos"][0]["name"], "late", "{doc:#}");
    assert_eq!(sensor(&doc, "git").expect("git sensor")["state"], "ok");
    std::fs::write(&registry, json!({ "repos": [] }).to_string()).expect("write registry");
    daemon.wait_for("the repo to leave", |doc| doc["repos"] == json!([]));
}

/// An unreadable registry degrades the git sensor and serves no git rows.
#[test]
fn malformed_registry_degrades_git() {
    let home = Home::new();
    let malformed = home.path().join("malformed.json");
    std::fs::write(&malformed, "{").expect("write registry");
    let daemon = Daemon::start_with(
        home.path(),
        &["--repos", malformed.to_str().expect("utf-8")],
    );
    let doc = daemon.wait_for("git sensor", |doc| sensor(doc, "git").is_some());
    let git = sensor(&doc, "git").expect("git sensor");
    assert_eq!(git["state"], "degraded");
    assert!(
        git["detail"]
            .as_str()
            .is_some_and(|detail| detail.contains("json:")),
        "{git}"
    );
    assert_eq!(doc["branches"], json!([]));
    assert_eq!(doc["repos"], json!([]));
}

/// The bases of every train in `doc`, in order.
fn train_bases(doc: &Value) -> Vec<String> {
    doc["trains"]
        .as_array()
        .map(|trains| {
            trains
                .iter()
                .filter_map(|train| train["base"].as_str().map(str::to_owned))
                .collect()
        })
        .unwrap_or_default()
}

/// `(exists, releasable, unreleased)` of the fixture repo's staging row.
fn landing_facts(doc: &Value) -> (Value, Value, Value) {
    let release = &doc["landings"][0]["release"];
    (
        release["exists"].clone(),
        release["releasable"].clone(),
        release["unreleased"].clone(),
    )
}

/// The staging row follows develop: absent, at main, ahead of main, behind a moved main; while develop exists its
/// train is requested, and a session based on main still has no landing target.
#[test]
fn landing_facts_follow_develop() {
    let repo = GitRepo::new();
    let work = repo.worktree("s", "main");
    repo.commit(&work, "s.txt", "s\n", "session work");
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = daemon.wait_for("the landing row", |doc| doc["landings"][0].is_object());
    assert_eq!(
        landing_facts(&doc),
        (json!(false), json!(false), json!(0)),
        "{doc:#}"
    );
    assert_eq!(doc["staging_target"], "develop");
    assert_eq!(doc["landings"].as_array().map(Vec::len), Some(1), "{doc:#}");
    assert_eq!(doc["landings"][0]["branch"], "develop");
    assert_eq!(doc["landings"][0]["queue"], json!([]));
    assert!(doc["landings"][0].get("holder").is_none(), "{doc:#}");
    assert_eq!(train_bases(&doc), ["main"]);
    assert_eq!(doc["repos"][0]["warnings"], json!([]), "no develop warning");

    repo.git(&repo.path, &["branch", "develop", "main"]);
    let doc = daemon.wait_for("develop at main", |doc| {
        landing_facts(doc) == (json!(true), json!(true), json!(0))
            && train_bases(doc) == ["main", "develop"]
    });
    assert!(
        branch(&doc, "claude/s").get("target").is_none(),
        "a main-based session has no landing target: {doc:#}"
    );
    assert_eq!(doc["repos"][0]["warnings"], json!([]), "{doc:#}");

    let develop = repo.path.join(".claude/develop");
    let target = develop.to_str().expect("utf-8");
    repo.git(&repo.path, &["worktree", "add", "-q", target, "develop"]);
    repo.commit(&develop, "d.txt", "d\n", "develop work");
    daemon.wait_for("one commit on develop", |doc| {
        landing_facts(doc) == (json!(true), json!(true), json!(1))
    });

    repo.commit(&repo.path, "m.txt", "m\n", "main moves");
    daemon.wait_for("main moved past the fork", |doc| {
        landing_facts(doc) == (json!(true), json!(false), json!(1))
    });
}

/// Only branches under a lane prefix collide: a routine branch conflicting with a session branch does not under the
/// default, and an empty list silences two conflicting session branches.
#[test]
fn collision_prefixes_scope_lanes() {
    let conflicting = |repo: &GitRepo, worktrees: &[(&str, &str)]| {
        for (branch, dir) in worktrees {
            let path = repo.path.join(".claude/worktrees").join(dir);
            let target = path.to_str().expect("utf-8");
            repo.git(
                &repo.path,
                &["worktree", "add", "-q", "-b", branch, target, "main"],
            );
            repo.commit(&path, "shared.txt", &format!("line1\n{dir}\nline3\n"), dir);
        }
    };
    let rows_of = |doc: &Value| doc["collisions"].as_array().map_or(0, Vec::len);

    let repo = GitRepo::new();
    conflicting(&repo, &[("claude/a", "a"), ("claude-routines/r", "r")]);
    let home = Home::new();
    let daemon = start(&home, &repo, &default_config());
    let doc = daemon.wait_for("both rows scanned", |doc| {
        doc["branches"]
            .as_array()
            .is_some_and(|rows| rows.len() == 2)
    });
    assert_eq!(
        rows_of(&doc),
        0,
        "default: the routine branch is no lane: {doc:#}"
    );
    drop(daemon);
    let mut widened = default_config();
    widened["collision_prefixes"] = json!(["claude/", "claude-routines/"]);
    let home = Home::new();
    let daemon = start(&home, &repo, &widened);
    daemon.wait_for("the routine prefix listed: confirmed", |doc| {
        rows_of(doc) == 1
    });
    drop(daemon);

    let repo = GitRepo::new();
    conflicting(&repo, &[("claude/a", "a"), ("claude/b", "b")]);
    let mut silenced = default_config();
    silenced["collision_prefixes"] = json!([]);
    let home = Home::new();
    let daemon = start(&home, &repo, &silenced);
    let doc = daemon.wait_for("both rows scanned", |doc| {
        doc["branches"]
            .as_array()
            .is_some_and(|rows| rows.len() == 2)
    });
    std::thread::sleep(Duration::from_millis(500));
    assert_eq!(
        rows_of(&daemon.document()),
        0,
        "an empty list: no lanes: {doc:#}"
    );
}
