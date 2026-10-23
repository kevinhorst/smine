//! Scenarios against the built binary — every plan verification item that a throwaway repo can exercise.

use std::path::Path;
use std::process::Command;
use std::time::Instant;

use serde_json::Value;

use crate::common::{Fixture, containment, row, write_at};

mod forward;
mod history;
mod pick;
mod probe;
mod unplaced;

/// Shas the fixture-repo assertions refer to.
struct FixtureShas {
    base: String,
    session: String,
}

/// The scenario repo: every base-resolution shape, the verdict classes, a merged branch, worktrees with sessions.
fn fixture_repo() -> (Fixture, FixtureShas) {
    let mut f = Fixture::new();
    // A stale release branch: cut before main moved on, so main is the closest candidate for a later sha.
    f.branch("release/a", "main");
    f.commit_on("main", "base.txt", "base 1\n", "main advances");
    let base_sha = f.git(&["rev-parse", "main"]);

    // ref origin, one own commit that exists nowhere else → unpicked, unsafe.
    f.branch("claude/ref-base", "main");
    f.commit_on("claude/ref-base", "new.txt", "new\n", "new file");

    // bare-sha origin (the Desktop pool shape) → resolved to main.
    f.branch("claude/sha-base", &base_sha);

    // routine chain: chain-2 created from chain-1 created from main.
    f.branch("claude-routines/chain-1", "main");
    f.branch("claude-routines/chain-2", "claude-routines/chain-1");

    // applied: commit A on a branch, cherry-picked to main, then a rename on main moves the file on.
    f.branch("claude/applied", "main");
    let a_sha = f.commit_on("claude/applied", "a.txt", "a\n", "add a");
    f.cherry_pick("main", &a_sha);

    // picked: commit B cherry-picked to main, then main edits the same line → the probe conflicts, the twin scan places it.
    f.branch("claude/picked", "main");
    let b_sha = f.commit_on(
        "claude/picked",
        "shared.txt",
        "line1\nline2 changed\nline3\n",
        "edit shared",
    );
    f.cherry_pick("main", &b_sha);
    f.commit_on(
        "main",
        "shared.txt",
        "line1\nline2 changed again\nline3\n",
        "edit shared again on main",
    );

    // merge commit on a branch: never counted as unpicked.
    f.branch("claude/with-merge", "main");
    f.commit_on("claude/with-merge", "m.txt", "m\n", "own work");
    f.commit_on("main", "base.txt", "base 2\n", "main moves on");
    f.merge_no_ff("claude/with-merge", "main");

    // merged --no-ff into main → contained (ancestor) with rank default.
    f.branch("claude/merged", "main");
    f.commit_on("claude/merged", "merged.txt", "x\n", "merged work");
    f.merge_no_ff("main", "claude/merged");

    // base rank: created from `develop`, merged there only → the base wins over main.
    f.branch("develop", "main");
    f.branch("claude/on-develop", "develop");
    f.commit_on("claude/on-develop", "dev.txt", "d\n", "develop work");
    f.merge_no_ff("develop", "claude/on-develop");

    // other rank: created from main, picked to release/a only → neither base nor default, but release/a.
    f.branch("claude/to-release", "main");
    let r_sha = f.commit_on("claude/to-release", "rel.txt", "r\n", "release fix");
    f.cherry_pick("release/a", &r_sha);

    // worktrees: one clean with a live session, one detached (codex pool) with a rollout.
    f.checkout("main");
    let wt = f.worktree("claude/sha-base", "sha-base");
    f.lock(&wt);
    let session_id = f.claude_session(&wt, "fixture session");
    let detached = f.detached_worktree(&base_sha, "abcd");
    f.codex_session(&detached, "rollout-1");
    let shas = FixtureShas {
        base: base_sha,
        session: session_id,
    };
    (f, shas)
}

/// Base resolution, the containment ranks, unpicked counts, sessions and the detached row on one repo.
#[test]
fn status_on_fixture_repo() {
    let (f, shas) = fixture_repo();
    let FixtureShas {
        base: base_sha,
        session: session_id,
        ..
    } = shas;
    let doc = f.status();
    assert_eq!(doc["schema"], 1);
    assert_eq!(doc["repo"]["default_branch"], "main");

    let ref_base = row(&doc, "claude/ref-base");
    assert_eq!(ref_base["from"]["resolution"], "ref");
    assert_eq!(ref_base["from"]["name"], "main");
    assert_eq!(ref_base["counts"]["vs_from"]["ahead"], 1);
    assert_eq!(ref_base["unpicked"], 1);
    assert_eq!(containment(ref_base), None, "{ref_base}");
    assert_eq!(ref_base["safe_to_remove"], false);
    assert_eq!(
        ref_base["unsafe_reasons"],
        serde_json::json!(["unpicked(1)", "no worktree"])
    );

    let sha_base = row(&doc, "claude/sha-base");
    assert_eq!(sha_base["from"]["resolution"], "sha");
    assert_eq!(sha_base["from"]["name"], "main");
    assert_eq!(sha_base["from"]["origin"], base_sha);
    assert_eq!(sha_base["worktree"]["locked"], "claude session");
    assert_eq!(sha_base["session"]["id"], session_id);
    assert_eq!(sha_base["session"]["alive"], true);
    assert_eq!(
        containment(sha_base),
        Some(("main".into(), "ancestor".into(), "base".into())),
        "{sha_base}"
    );
    assert_eq!(sha_base["safe_to_remove"], true, "{sha_base}");

    let chain = row(&doc, "claude-routines/chain-2");
    assert_eq!(chain["from"]["resolution"], "chain");
    assert_eq!(chain["from"]["name"], "main");
    assert_eq!(chain["from"]["origin"], "claude-routines/chain-1");

    // Cherry-picked to main (base) → all_commits_placed on the base; rename-blind `git cherry` used to miss this.
    let applied = row(&doc, "claude/applied");
    assert_eq!(
        containment(applied),
        Some(("main".into(), "all_commits_placed".into(), "base".into())),
        "{applied}"
    );
    assert_eq!(applied["unpicked"], 0);

    // Picked then edited again on main: the merge simulation conflicts, the patch-id twin places it.
    let picked = row(&doc, "claude/picked");
    assert_eq!(
        containment(picked),
        Some(("main".into(), "all_commits_placed".into(), "base".into())),
        "{picked}"
    );

    // A merge commit is never counted as unpicked: one own commit, not two.
    let with_merge = row(&doc, "claude/with-merge");
    assert_eq!(containment(with_merge), None, "{with_merge}");
    assert_eq!(with_merge["unpicked"], 1, "{with_merge}");

    let merged = row(&doc, "claude/merged");
    assert_eq!(
        containment(merged),
        Some(("main".into(), "ancestor".into(), "base".into()))
    );

    // Base rank beats default: created from develop and merged there, main never saw it.
    let on_develop = row(&doc, "claude/on-develop");
    assert_eq!(on_develop["from"]["name"], "develop");
    assert_eq!(
        containment(on_develop),
        Some(("develop".into(), "ancestor".into(), "base".into())),
        "{on_develop}"
    );
    assert_eq!(on_develop["safe_to_remove"], true);

    // Other rank: base main does not contain it, default main neither, release/a does.
    let to_release = row(&doc, "claude/to-release");
    assert_eq!(
        containment(to_release),
        Some((
            "release/a".into(),
            "all_commits_placed".into(),
            "other".into()
        )),
        "{to_release}"
    );
    assert_eq!(to_release["unpicked"], 0);
    assert_eq!(to_release["safe_to_remove"], true);

    let detached_row = doc["rows"]
        .as_array()
        .expect("rows")
        .iter()
        .find(|r| r["branch"].is_null())
        .expect("detached row");
    assert_eq!(detached_row["kind"], "codex");
    assert_eq!(detached_row["session"]["agent"], "codex");
    assert_eq!(detached_row["session"]["id"], "rollout-1");
    assert_eq!(
        containment(detached_row),
        Some(("main".into(), "ancestor".into(), "default".into()))
    );
    assert_eq!(detached_row["safe_to_remove"], true);

    let timings = &doc["timings"];
    let phases = ["total_ms", "analysis_ms", "worktrees_ms", "checkout_ms"];
    assert!(phases.iter().all(|p| timings[p].is_u64()), "{timings}");
    assert_eq!(doc["warnings"], serde_json::json!([]));
}

/// Worktree facts: lock reason, in-progress operation, prunable, detached, main.
#[test]
fn inventory_worktree_facts() {
    let mut f = Fixture::new();
    f.branch("claude/locked", "main");
    f.branch("claude/op", "main");
    f.commit_on(
        "claude/op",
        "shared.txt",
        "line1\nfrom op\nline3\n",
        "op edit",
    );
    f.commit_on(
        "main",
        "shared.txt",
        "line1\nfrom main\nline3\n",
        "main edit",
    );
    f.branch("claude/gone", "main");
    let base_sha = f.git(&["rev-parse", "main"]);
    f.checkout("main");
    let locked_wt = f.worktree("claude/locked", "locked");
    f.lock(&locked_wt);
    let op_wt = f.worktree("claude/op", "op");
    let merge = Command::new("git")
        .arg("-C")
        .arg(&op_wt)
        .args(["merge", "main"])
        .output()
        .expect("spawn git merge");
    assert!(!merge.status.success(), "the merge must conflict");
    let gone_wt = f.worktree("claude/gone", "gone");
    std::fs::remove_dir_all(&gone_wt).expect("remove worktree dir");
    let detached = f.detached_worktree(&base_sha, "abcd");

    let doc = f.status();
    let locked = row(&doc, "claude/locked");
    assert_eq!(locked["worktree"]["locked"], "claude session", "{locked}");
    assert_eq!(locked["worktree"]["main"], false);
    assert_eq!(locked["worktree"]["detached"], false);
    let op = row(&doc, "claude/op");
    assert_eq!(op["worktree"]["operation"], "merge", "{op}");
    let gone = row(&doc, "claude/gone");
    assert_eq!(gone["worktree"]["prunable"], true, "{gone}");
    let detached_row = doc["rows"]
        .as_array()
        .expect("rows")
        .iter()
        .find(|r| r["branch"].is_null())
        .unwrap_or_else(|| panic!("no detached row in {doc}"));
    assert_eq!(detached_row["worktree"]["detached"], true);
    let canonical = std::fs::canonicalize(&detached).expect("canonical detached path");
    assert_eq!(
        detached_row["worktree"]["path"],
        canonical.to_str().expect("utf8")
    );
    assert_eq!(detached_row["head"]["sha"], base_sha);
}

#[test]
fn order_train() {
    let mut f = Fixture::new();
    f.branch("claude/disjoint-a", "main");
    f.commit_on("claude/disjoint-a", "a.txt", "a\n", "a");
    f.branch("claude/disjoint-b", "main");
    f.commit_on("claude/disjoint-b", "b.txt", "b\n", "b");
    f.branch("claude/conflict-1", "main");
    f.commit_on(
        "claude/conflict-1",
        "shared.txt",
        "line1\nfrom one\nline3\n",
        "one",
    );
    f.branch("claude/conflict-2", "main");
    f.commit_on(
        "claude/conflict-2",
        "shared.txt",
        "line1\nfrom two\nline3\n",
        "two",
    );
    f.branch("claude/dirty", "main");
    f.commit_on("claude/dirty", "d.txt", "d\n", "d");
    f.checkout("main");
    let dirty_wt = f.worktree("claude/dirty", "dirty");
    write_at(&dirty_wt, "shared.txt", "line1\nfrom dirty\nline3\n");

    let doc = f.status();
    let pairs = doc["order"]["pairs"].as_array().expect("pairs");
    // Pair sides follow worktrunk's row order; normalize so the assertion is order-free.
    let pair_names: Vec<(String, String)> = pairs
        .iter()
        .map(|p| {
            let mut sides = [
                p["a"].as_str().unwrap_or_default().to_owned(),
                p["b"].as_str().unwrap_or_default().to_owned(),
            ];
            sides.sort();
            let [a, b] = sides;
            (a, b)
        })
        .collect();
    assert!(
        pair_names.contains(&(
            "claude/conflict-1".to_owned(),
            "claude/conflict-2".to_owned()
        )),
        "{pairs:?}"
    );
    assert!(
        pairs
            .iter()
            .all(|p| p["paths"] == serde_json::json!(["shared.txt"])),
        "{pairs:?}"
    );
    assert_eq!(
        doc["order"]["dirty_snapshots"],
        serde_json::json!(["claude/dirty"])
    );

    for name in ["claude/disjoint-a", "claude/disjoint-b"] {
        let r = row(&doc, name);
        assert_eq!(r["order"]["train"]["state"], "clean", "{r}");
        assert_eq!(r["order"]["conflicts_with"], serde_json::json!([]));
    }
    let blocked: Vec<&Value> = doc["rows"]
        .as_array()
        .expect("rows")
        .iter()
        .filter(|r| r["order"]["train"]["state"] == "blocked")
        .collect();
    assert_eq!(
        blocked.len(),
        2,
        "conflict-2 or conflict-1, and dirty: {}",
        doc["order"]
    );
    assert!(
        blocked
            .iter()
            .all(|r| r["order"]["train"]["paths"] == serde_json::json!(["shared.txt"]))
    );
}

/// `--train-base` adds a train seeded with that base; the default train and the rows stay as they were, duplicates
/// are dropped and a base that is not a local non-agent branch is a warning, not a failure.
#[test]
fn order_train_bases() {
    let mut f = Fixture::new();
    f.branch("develop", "main");
    f.commit_on(
        "develop",
        "shared.txt",
        "line1\nfrom develop\nline3\n",
        "develop edit",
    );
    f.branch("claude/edit-shared", "main");
    f.commit_on(
        "claude/edit-shared",
        "shared.txt",
        "line1\nfrom agent\nline3\n",
        "agent edit",
    );
    f.branch("claude/new-file", "main");
    f.commit_on("claude/new-file", "n.txt", "n\n", "new file");
    f.checkout("main");

    let plain = f.status();
    assert!(
        plain.get("trains").is_none(),
        "absent unless requested: {plain}"
    );

    let output = f.run(&[
        "--format",
        "json",
        "status",
        "--train-base",
        "develop",
        "--train-base",
        "develop",
        "--train-base",
        "claude/new-file",
        "--train-base",
        "nope",
    ]);
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let doc: Value = serde_json::from_slice(&output.stdout).expect("status json");
    assert_eq!(doc["order"], plain["order"], "default train unchanged");
    assert_eq!(doc["rows"], plain["rows"], "rows unchanged");
    let trains = doc["trains"].as_array().expect("trains");
    assert_eq!(trains.len(), 1, "one develop train: {trains:?}");
    let develop = &trains[0];
    assert_eq!(develop["base"], "develop");
    assert_eq!(
        develop["branches"]["claude/new-file"]["train"]["state"],
        "clean"
    );
    assert_eq!(
        develop["branches"]["claude/edit-shared"]["train"]["state"],
        "blocked"
    );
    assert_eq!(
        develop["branches"]["claude/edit-shared"]["train"]["paths"],
        serde_json::json!(["shared.txt"])
    );
    assert_eq!(
        row(&doc, "claude/edit-shared")["order"]["train"]["state"],
        "clean"
    );
    let warnings = doc["warnings"].as_array().expect("warnings");
    for base in ["claude/new-file", "nope"] {
        let want = format!("train base {base} is not a local non-agent branch");
        assert!(warnings.iter().any(|w| *w == want.as_str()), "{warnings:?}");
    }
}

/// `claude-orch/t` holds `claude/member` merged: an orchestrator branch with one member merged into it.
fn orchestrated_fixture() -> Fixture {
    let mut f = Fixture::new();
    f.branch("claude-orch/t", "main");
    f.branch("claude/member", "main");
    f.commit_on("claude/member", "m.txt", "m\n", "member work");
    f.merge_no_ff("claude-orch/t", "claude/member");
    f.checkout("main");
    f
}

/// `--candidate` tracks a named branch, agent-owned or not, as a containment candidate and train base for that run
/// only; without it the output is the default one.
#[test]
fn added_candidate() {
    let f = orchestrated_fixture();
    let plain = f.status();
    assert_eq!(
        containment(row(&plain, "claude/member")),
        None,
        "default set"
    );

    let json = |args: &[&str]| -> Value {
        let mut all = vec!["--format", "json", "status"];
        all.extend_from_slice(args);
        let output = f.run(&all);
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        serde_json::from_slice(&output.stdout).expect("status json")
    };

    let base_only = json(&["--train-base", "claude-orch/t"]);
    assert_eq!(base_only["trains"], serde_json::json!([]), "no train");
    assert_eq!(base_only["rows"], plain["rows"], "rows unchanged");

    let tracked = json(&[
        "--candidate",
        "claude-orch/t",
        "--train-base",
        "claude-orch/t",
        "--candidate",
        "nope",
    ]);
    let trains = tracked["trains"].as_array().expect("trains");
    assert_eq!(trains.len(), 1, "{trains:?}");
    assert_eq!(trains[0]["base"], "claude-orch/t");
    assert_eq!(
        containment(row(&tracked, "claude/member")).map(|(branch, reason, _)| (branch, reason)),
        Some(("claude-orch/t".to_owned(), "ancestor".to_owned()))
    );
    assert!(
        containment(row(&tracked, "claude-orch/t"))
            .is_none_or(|(branch, _, _)| branch != "claude-orch/t"),
        "never contained in itself"
    );
    let warnings = tracked["warnings"].as_array().expect("warnings");
    assert!(
        warnings
            .iter()
            .any(|w| *w == "candidate nope is not a local branch"),
        "{warnings:?}"
    );
}

/// `remove` tracks no added candidate: a member whose work lives only on an orchestrator branch stays.
#[test]
fn remove_ignores_agent_bases() {
    let mut f = orchestrated_fixture();
    let wt = f.worktree("claude/member", "member");
    let out = f.run(&[
        "--format",
        "json",
        "remove",
        "--delete-branch",
        "claude/member",
    ]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "skipped", "{result}");
    assert!(wt.exists());
}

/// A worktree whose tracked file shares the index's mtime is racily clean: status reads it clean by content and
/// leaves the index alone — a rewrite would take `index.lock` and fail an agent's concurrent `git commit`.
#[test]
fn status_never_writes_the_index() {
    let mut f = Fixture::new();
    f.branch("claude/racy", "main");
    f.checkout("main");
    let wt = f.worktree("claude/racy", "racy");
    let git_dir = f.git_in(&wt, &["rev-parse", "--absolute-git-dir"]);
    let index = Path::new(&git_dir).join("index");
    let touch = Command::new("touch")
        .arg("-r")
        .arg(&index)
        .arg(wt.join("base.txt"))
        .output()
        .expect("spawn touch");
    assert!(touch.status.success());
    let mtime = |path: &Path| {
        std::fs::metadata(path)
            .and_then(|meta| meta.modified())
            .expect("index mtime")
    };
    let before = mtime(&index);
    // Racy is judged at second granularity: a rewrite would land in a later second and move the mtime.
    std::thread::sleep(std::time::Duration::from_millis(1100));

    let doc = f.status();
    assert_eq!(row(&doc, "claude/racy")["worktree"]["dirty"], 0, "{doc}");
    assert_eq!(mtime(&index), before, "status rewrote the index");
    assert!(
        !Path::new(&git_dir).join("index.lock").exists(),
        "status left an index lock"
    );
}

/// Remove plans only the named rows: a dirty sibling neither blocks nor changes.
#[test]
fn remove_builds_named_rows_only() {
    let mut f = Fixture::new();
    f.branch("claude/clean", "main");
    f.branch("claude/dirty", "main");
    f.checkout("main");
    let clean_wt = f.worktree("claude/clean", "clean");
    let dirty_wt = f.worktree("claude/dirty", "dirty");
    write_at(&dirty_wt, "base.txt", "edited\n");

    let out = f.run(&["--format", "json", "remove", "claude/clean"]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(
        result["lines"].as_array().map(Vec::len),
        Some(1),
        "{result}"
    );
    assert_eq!(result["lines"][0]["outcome"], "removed");
    assert!(!clean_wt.exists());
    assert!(dirty_wt.exists());

    let out = f.run(&["remove", "claude/nope"]);
    assert_eq!(out.status.code(), Some(3));
}

/// `--kind` analyses only that kind's rows; `kinds` lists every kind either way; an empty kind is a usage error.
#[test]
fn status_kind_filter() {
    let mut f = Fixture::new();
    f.branch("claude/a", "main");
    f.branch("claude-routines/nightly", "main");
    f.checkout("main");
    let branches = |document: &Value| -> Vec<String> {
        document["rows"]
            .as_array()
            .expect("rows")
            .iter()
            .filter_map(|row| row["branch"].as_str().map(str::to_owned))
            .collect()
    };

    let all = f.status();
    assert_eq!(branches(&all), ["claude-routines/nightly", "claude/a"]);
    assert_eq!(
        all["kinds"],
        serde_json::json!(["claude", "claude-routines"])
    );

    let out = f.run(&["--format", "json", "status", "--kind", "claude"]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let claude: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(branches(&claude), ["claude/a"]);
    assert_eq!(claude["kinds"], all["kinds"]);

    let out = f.run(&["--format", "json", "status", "--kind", "claude-review"]);
    assert!(out.status.success());
    let absent: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(absent["rows"], serde_json::json!([]));
    assert_eq!(absent["kinds"], all["kinds"]);

    let out = f.run(&["status", "--kind="]);
    assert_eq!(out.status.code(), Some(2));
}

#[test]
fn ops_remove() {
    let mut f = Fixture::new();
    f.branch("claude/clean", "main");
    f.branch("claude/dirty", "main");
    f.branch("claude/unpicked", "main");
    f.commit_on("claude/unpicked", "u.txt", "u\n", "unpicked work");
    f.checkout("main");
    let clean_wt = f.worktree("claude/clean", "clean");
    f.lock(&clean_wt);
    let dirty_wt = f.worktree("claude/dirty", "dirty");
    write_at(&dirty_wt, "base.txt", "edited\n");

    // safe + locked → unlocked, removed; branch kept without --delete-branch.
    let out = f.run(&["--format", "json", "remove", "claude/clean"]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "removed");
    assert!(!clean_wt.exists());
    assert!(
        f.git(&["branch", "--list", "claude/clean"])
            .contains("claude/clean")
    );

    // dirty → skipped line, worktree kept, exit 0.
    let out = f.run(&["--format", "json", "remove", "claude/dirty"]);
    assert!(out.status.success());
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "skipped");
    assert!(
        result["lines"][0]["text"]
            .as_str()
            .unwrap_or_default()
            .contains("dirty(1)")
    );
    assert!(dirty_wt.exists());

    // --force → removed.
    let out = f.run(&["--format", "text", "remove", "--force", "claude/dirty"]);
    assert!(out.status.success());
    assert!(String::from_utf8_lossy(&out.stdout).contains("removed"));
    assert!(!dirty_wt.exists());

    // --delete-branch unsafe → skipped branch line; branch kept.
    let out = f.run(&[
        "--format",
        "text",
        "remove",
        "--delete-branch",
        "claude/unpicked",
    ]);
    assert!(out.status.success());
    let text = String::from_utf8_lossy(&out.stdout).into_owned();
    assert!(
        text.contains("no worktree checked out") || text.contains("skipped branch:"),
        "{text}"
    );
    assert!(
        text.contains("skipped branch: claude/unpicked: unpicked(1)"),
        "{text}"
    );
    assert!(
        f.git(&["branch", "--list", "claude/unpicked"])
            .contains("claude/unpicked")
    );

    // --delete-branch safe (worktree-less, contained) → branch gone.
    let out = f.run(&[
        "--format",
        "text",
        "remove",
        "--delete-branch",
        "claude/clean",
    ]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(String::from_utf8_lossy(&out.stdout).contains("deleted branch claude/clean"));
    assert_eq!(f.git(&["branch", "--list", "claude/clean"]), "");

    // refusals: non-agent branch and unknown branch → exit 3.
    let out = f.run(&["remove", "main"]);
    assert_eq!(out.status.code(), Some(3));
    let out = f.run(&["remove", "claude/nope"]);
    assert_eq!(out.status.code(), Some(3));
}

#[test]
fn from_inside_worktree() {
    let mut f = Fixture::new();
    f.branch("claude/session", "main");
    let session_wt = f.worktree("claude/session", "session");
    f.lock(&session_wt);
    write_at(&f.repo, "base.txt", "main dirty\n");
    let main_path = dunce::canonicalize(&f.repo).expect("canonical main");

    // status from inside the worktree describes the main checkout.
    let out = f.run_in(&session_wt, &["--format", "json", "status", "--no-order"]);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let document: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(document["repo"]["path"], main_path.to_str().expect("utf8"));
    assert_eq!(document["checkout"]["branch"], "main");
    assert_eq!(document["checkout"]["dirty"], 1);
    f.git(&["checkout", "--", "base.txt"]);

    // /close shape: cwd inside the worktree being removed, default --repo ., --delete-branch.
    let out = f.run_in(
        &session_wt,
        &[
            "--format",
            "json",
            "remove",
            "--delete-branch",
            "claude/session",
        ],
    );
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "removed");
    assert_eq!(result["lines"][1]["outcome"], "branch_deleted");
    assert!(!session_wt.exists());
    assert_eq!(f.git(&["branch", "--list", "claude/session"]), "");
}

#[test]
fn ops_remove_dry_run() {
    let mut f = Fixture::new();
    f.branch("claude/session", "main");
    f.branch("claude/dirty", "main");
    let session_wt = f.worktree("claude/session", "session");
    f.lock(&session_wt);
    let dirty_wt = f.worktree("claude/dirty", "dirty");
    write_at(&dirty_wt, "base.txt", "edited\n");
    let dry_run = ["remove", "--dry-run", "--delete-branch", "claude/session"];

    // /close shape: passing check from inside the worktree, nothing touched.
    let mut text_args = vec!["--format", "text"];
    text_args.extend(dry_run);
    let out = f.run_in(&session_wt, &text_args);
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let text = String::from_utf8_lossy(&out.stdout).into_owned();
    assert!(text.contains("would remove"), "{text}");
    assert!(
        text.contains("would delete branch claude/session"),
        "{text}"
    );
    assert!(session_wt.exists());
    assert!(
        f.git(&["branch", "--list", "claude/session"])
            .contains("claude/session")
    );

    // same check in json: the would_* outcomes.
    let mut json_args = vec!["--format", "json"];
    json_args.extend(dry_run);
    let out = f.run_in(&session_wt, &json_args);
    assert!(out.status.success());
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "would_remove");
    assert_eq!(result["lines"][1]["outcome"], "would_delete_branch");

    // dirty → refusal: exit 3, skip line on stderr, nothing on stdout, worktree kept.
    let out = f.run(&[
        "--format",
        "text",
        "remove",
        "--dry-run",
        "--delete-branch",
        "claude/dirty",
    ]);
    assert_eq!(out.status.code(), Some(3));
    let stderr = String::from_utf8_lossy(&out.stderr).into_owned();
    assert!(stderr.contains("skipped:"), "{stderr}");
    assert_eq!(stderr.matches("dirty(1)").count(), 1, "{stderr}");
    assert_eq!(String::from_utf8_lossy(&out.stdout), "");
    assert!(dirty_wt.exists());

    // the real remove afterwards still runs: the dry run changed nothing.
    let out = f.run_in(
        &session_wt,
        &[
            "--format",
            "json",
            "remove",
            "--delete-branch",
            "claude/session",
        ],
    );
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let result: Value = serde_json::from_slice(&out.stdout).expect("json");
    assert_eq!(result["lines"][0]["outcome"], "removed");
    assert!(!session_wt.exists());
}

/// A pick of a file inside a directory main also changed elsewhere: the directory's tree differs on both sides, the
/// twin must still match on the file change alone.
#[test]
fn picked_in_subdirectory() {
    let mut f = Fixture::new();
    f.commit_on("main", "dir/a.txt", "a1\na2\na3\n", "add dir");
    f.branch("claude/sub", "main");
    let pick = f.commit_on("claude/sub", "dir/a.txt", "a1\na2 changed\na3\n", "edit a");
    f.commit_on("main", "dir/b.txt", "b\n", "sibling on main");
    f.cherry_pick("main", &pick);
    // main edits the picked line again, so re-applying the pick conflicts and only the twin can place it.
    f.commit_on(
        "main",
        "dir/a.txt",
        "a1\na2 changed again\na3\n",
        "edit a again",
    );
    f.checkout("main");

    let doc = f.status();
    let sub = row(&doc, "claude/sub");
    assert_eq!(
        containment(sub),
        Some(("main".into(), "all_commits_placed".into(), "base".into())),
        "{sub}"
    );
    assert_eq!(sub["unpicked"], 0, "{sub}");
}

/// Picked to main, then main merged back into the branch: the twin is shared history now, and still places the
/// original once main moves the picked line on.
#[test]
fn picked_then_merged_back() {
    let mut f = Fixture::new();
    f.branch("claude/merged-back", "main");
    let pick = f.commit_on(
        "claude/merged-back",
        "shared.txt",
        "line1\nline2 changed\nline3\n",
        "edit shared",
    );
    f.cherry_pick("main", &pick);
    f.merge_no_ff("claude/merged-back", "main");
    // main edits the picked line again, so re-applying the pick conflicts and only the twin can place it.
    f.commit_on(
        "main",
        "shared.txt",
        "line1\nline2 changed again\nline3\n",
        "edit shared again on main",
    );
    f.checkout("main");

    let doc = f.status();
    let merged_back = row(&doc, "claude/merged-back");
    assert_eq!(
        containment(merged_back),
        Some(("main".into(), "all_commits_placed".into(), "base".into())),
        "{merged_back}"
    );
    assert_eq!(merged_back["unpicked"], 0, "{merged_back}");
}

#[test]
fn schema_is_committed() {
    // (document argument, committed file)
    let documents: [(&str, &str); 5] = [
        ("status", "schema/status-v1.json"),
        ("unplaced", "schema/unplaced-v1.json"),
        ("pick", "schema/pick-v1.json"),
        ("forward", "schema/forward-v1.json"),
        ("probe", "schema/probe-v1.json"),
    ];
    for (document, file) in documents {
        let out = Command::new(env!("CARGO_BIN_EXE_agentwt"))
            .args(["schema", document])
            .output()
            .expect("spawn");
        assert!(out.status.success(), "{document}");
        let generated: Value = serde_json::from_slice(&out.stdout).expect("schema json");
        let committed_path = Path::new(env!("CARGO_MANIFEST_DIR")).join(file);
        let committed: Value =
            serde_json::from_slice(&std::fs::read(&committed_path).expect("committed schema"))
                .expect("committed json");
        assert_eq!(
            generated, committed,
            "schema drift: regenerate with `agentwt schema {document} > {file}`"
        );
    }
}

#[test]
fn interfaces_answers_claims() {
    let out = Command::new(env!("CARGO_BIN_EXE_agentwt"))
        .arg("interfaces")
        .output()
        .expect("spawn");
    assert!(out.status.success());
    assert_eq!(
        String::from_utf8_lossy(&out.stdout),
        "{\"interfaces\":[\"agentwt-forward@1\",\"agentwt-probe@1\",\"agentwt-remove@1\",\"agentwt-status@1\",\"agentwt-unplaced@1\"]}\n"
    );
}

#[test]
fn no_stdout_narration() {
    let f = Fixture::new();
    let out = f.run(&["--format", "json", "status"]);
    assert!(out.status.success());
    let parsed: Result<Value, _> = serde_json::from_slice(&out.stdout);
    assert!(
        parsed.is_ok(),
        "stdout must be pure JSON: {}",
        String::from_utf8_lossy(&out.stdout)
    );
}

#[test]
fn repo_without_agent_branches() {
    let f = Fixture::new();
    let doc = f.status();
    assert_eq!(doc["rows"], serde_json::json!([]));
    assert_eq!(doc["order"]["train"], serde_json::json!([]));
}

/// Load evidence: 40 agent branches × 5 commits with shared-file overlap, 12 worktrees (3 dirty), 4 candidates.
/// Runs under `make rust-load`; `AGENTWT_LOAD_REPO` repeats the run on a real repo and `AGENTWT_LOAD_OLD` times the
/// retired shell script there for the comparison.
#[test]
#[ignore = "load evidence, run via make rust-load"]
fn load_forty_branches() {
    let mut f = Fixture::new();
    for i in 0..200 {
        f.commit_on(
            "main",
            &format!("hist/{i}.txt"),
            &format!("{i}\n"),
            &format!("history {i}"),
        );
    }
    for candidate in ["release/a", "release/b", "improvement/c"] {
        f.branch(candidate, "main");
    }
    for b in 0..40 {
        let name = format!("claude/load-{b:02}");
        f.branch(&name, "main");
        for c in 0..5 {
            let (path, content) = if c % 3 == 0 {
                (
                    format!("shared/{}.txt", b % 5),
                    format!("branch {b} commit {c}\n"),
                )
            } else {
                (format!("own/{b}/{c}.txt"), format!("{b}-{c}\n"))
            };
            f.commit_on(&name, &path, &content, &format!("load {b} {c}"));
        }
    }
    f.checkout("main");
    for b in 0..12 {
        let wt = f.worktree(&format!("claude/load-{b:02}"), &format!("load-{b:02}"));
        if b < 3 {
            write_at(&wt, "base.txt", &format!("dirty {b}\n"));
        }
    }

    let started = Instant::now();
    let doc = f.status();
    let wall = started.elapsed();
    eprintln!(
        "synthetic 40 branches: wall {:?} timings {}",
        wall, doc["timings"]
    );
    assert_eq!(doc["rows"].as_array().map(Vec::len), Some(40));
    let total = doc["timings"]["total_ms"].as_u64().expect("total_ms");
    assert!(total < 10_000, "load threshold: total_ms {total} >= 10000");

    // `make rust-load` exports both variables even when unset (empty); empty means "skip".
    let real_repo = std::env::var("AGENTWT_LOAD_REPO")
        .ok()
        .filter(|v| !v.is_empty());
    if let Some(real) = real_repo {
        let started = Instant::now();
        let out = Command::new(env!("CARGO_BIN_EXE_agentwt"))
            .args(["--repo", &real, "--format", "json", "status"])
            .output()
            .expect("spawn");
        let new_ms = started.elapsed().as_millis();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        let doc: Value = serde_json::from_slice(&out.stdout).expect("json");
        eprintln!(
            "real repo {real}: new_ms {new_ms} timings {}",
            doc["timings"]
        );
        let old_checkout = std::env::var("AGENTWT_LOAD_OLD")
            .ok()
            .filter(|v| !v.is_empty());
        if let Some(old) = old_checkout {
            let script = Path::new(&old).join("cmd/worktrees/print_agent_worktrees_status.sh");
            let started = Instant::now();
            let out = Command::new("bash")
                .arg(&script)
                .current_dir(&real)
                .env("WORKTREE_STATUS_NO_CACHE", "1")
                .output()
                .expect("spawn old script");
            let old_ms = started.elapsed().as_millis();
            eprintln!(
                "old script exit {:?}: old_ms {old_ms} new_ms {new_ms} ratio_percent {}",
                out.status.code(),
                old_ms * 100 / new_ms.max(1)
            );
        }
    }
}
