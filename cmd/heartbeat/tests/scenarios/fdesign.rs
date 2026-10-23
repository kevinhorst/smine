//! fdesign's deployed declaration end to end: the daemon over a claude home holding the committed fdesign
//! `enforce.json` (the skill's render with the rules program filled), and the session `c` driven by replay hook bodies
//! shaped into the calls the skill prescribes — plan edits, the section check, the title set calls, the claim ledger
//! and the refute launch.

use serde_json::{Value, json};

use super::runtime::{Rig, denied};

/// fdesign's committed declaration: the same data the skill deploys.
const FDESIGN: &str = include_str!("../fixtures/enforce/fdesign.json");
/// The session plan file a plan-mode run writes.
const DRAFT: &str = "/home/k/.claude/plans/shout.md";
/// The persisted plan, relative to the repository root as the section check takes it.
const PERSISTED: &str =
    "plans/2026-10-14-shout-abc1234/2026-10-14-shout-wip-abc1234/design/shout.md";
/// The refute stage's script path, as the dispatcher passes it.
const REFUTE: &str = "/home/k/.claude/skills/fdesign/workflows/refute.js";
const DESIGN_READY: &str = "smine-rules title set DESIGN-READY";
const CODE_READY: &str = "smine-rules title set CODE-READY";

/// A rig with fdesign deployed and a run opened by the typed `/fdesign`.
fn opened() -> Rig {
    let rig = Rig::deploy("fdesign", FDESIGN, &json!({}));
    let answer = rig.root("UserPromptSubmit", |body| {
        body["prompt"] = json!("/fdesign plan the shout package");
    });
    assert_eq!(answer, json!({}));
    rig
}

/// The root wrote `path`.
fn wrote(rig: &Rig, path: &str) -> Value {
    rig.root("PostToolUse", |body| {
        body["tool_name"] = json!("Write");
        body["tool_input"] = json!({ "file_path": path, "content": "# Plan" });
        body["tool_response"] = json!({ "type": "create", "filePath": path });
    })
}

/// The root is about to run `command`.
fn runs(rig: &Rig, command: &str) -> Value {
    rig.root("PreToolUse", |body| {
        body["tool_name"] = json!("Bash");
        body["tool_input"] = json!({ "command": command });
    })
}

/// The root ran `command`, which printed `stdout`.
fn ran(rig: &Rig, command: &str, stdout: &str) -> Value {
    rig.root("PostToolUse", |body| {
        body["tool_name"] = json!("Bash");
        body["tool_input"] = json!({ "command": command });
        body["tool_response"] = json!({ "stdout": stdout, "stderr": "", "interrupted": false });
    })
}

/// The root is about to launch the refute round with `claims`.
fn refute(rig: &Rig, claims: &Value) -> Value {
    rig.root("PreToolUse", |body| {
        body["tool_name"] = json!("Workflow");
        body["tool_input"] =
            json!({ "scriptPath": REFUTE, "args": { "claims": claims, "artifactsDir": "/a" } });
    })
}

#[test]
fn design_ready_waits_for_the_section_check() {
    let rig = opened();
    assert_eq!(
        runs(&rig, DESIGN_READY),
        json!({}),
        "no plan edit yet: nothing owed"
    );
    wrote(&rig, DRAFT);
    let early = runs(&rig, DESIGN_READY);
    assert!(
        denied(
            &early,
            "after the last plan edit before setting DESIGN-READY"
        ),
        "{early}"
    );
    ran(&rig, &format!("smine-rules plan-sections {DRAFT}"), "");
    assert_eq!(
        runs(&rig, DESIGN_READY),
        json!({}),
        "the section check ran after the edit"
    );
    wrote(&rig, DRAFT);
    let again = runs(&rig, DESIGN_READY);
    assert!(
        denied(&again, "before setting DESIGN-READY"),
        "a later edit owes a new check: {again}"
    );
}

#[test]
fn code_ready_waits_for_the_persisted_check() {
    let rig = opened();
    wrote(&rig, &format!("/repo/{PERSISTED}"));
    let early = runs(&rig, CODE_READY);
    assert!(denied(&early, "before setting CODE-READY"), "{early}");
    ran(&rig, &format!("smine-rules plan-sections {DRAFT}"), "");
    let draft_checked = runs(&rig, CODE_READY);
    assert!(
        denied(&draft_checked, "before setting CODE-READY"),
        "the session plan's check does not count: {draft_checked}"
    );
    ran(&rig, &format!("smine-rules plan-sections {PERSISTED}"), "");
    assert_eq!(runs(&rig, CODE_READY), json!({}));
}

#[test]
fn refute_takes_the_printed_ledger() {
    let rig = opened();
    let ledger = json!([{
        "claim_id": "A1",
        "text": "Nothing imports a package named smokeshout.",
        "evidence": "go.mod:1",
        "source": "\"a new package nothing imports\" — decisions: D1"
    }]);
    let unprinted = refute(&rig, &ledger);
    assert!(denied(&unprinted, "before the refute round"), "{unprinted}");
    let printed = json!({ "claims": ledger, "schema": 1 });
    ran(
        &rig,
        &format!("smine-rules plan-claims {PERSISTED}"),
        &format!("{printed:#}\n"),
    );
    let edited = refute(&rig, &json!([]));
    assert!(
        denied(&edited, "args.claims must be the `claims` array"),
        "a dropped claim: {edited}"
    );
    assert_eq!(
        refute(&rig, &ledger),
        json!({}),
        "the printed ledger, verbatim"
    );
    let document = rig.daemon.runtime_document();
    let captured: Vec<&Value> = document["runs"][0]["anchors"]
        .as_array()
        .expect("anchors")
        .iter()
        .filter(|hit| hit["id"] == "claims")
        .map(|hit| &hit["captures"]["claims"])
        .collect();
    assert_eq!(captured, [&ledger], "the runtime document shows the ledger");
}

#[test]
fn in_flight_stop_is_deferred() {
    let rig = opened();
    wrote(&rig, DRAFT);
    assert_eq!(
        rig.stop(1),
        json!({}),
        "a refute round in flight: the turn has not ended"
    );
    let judged = rig.stop(0);
    assert_eq!(
        judged["decision"], "block",
        "nothing in flight: judged: {judged}"
    );
}

#[test]
fn blocked_stop_keeps_the_session_running() {
    let rig = opened();
    wrote(&rig, DRAFT);
    let blocked = rig.stop(0);
    assert!(
        blocked["reason"]
            .as_str()
            .is_some_and(|reason| reason.contains("plan-sections")),
        "{blocked}"
    );
    assert_eq!(rig.state(), "running", "the blocked Stop ended no turn");
    ran(&rig, &format!("smine-rules plan-sections {DRAFT}"), "");
    assert_eq!(rig.stop(0), json!({}));
    rig.daemon.wait_for_state(super::runtime::C, "idle");
    let document = rig.daemon.runtime_document();
    assert_eq!(
        document["runs"][0]["failures"],
        json!([{"check": "sections_before_stop", "count": 1}])
    );
}
