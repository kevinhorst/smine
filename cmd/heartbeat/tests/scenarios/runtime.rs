//! Runtime enforcement end to end: the daemon over a claude home holding a committed `enforce.json` — the
//! heartbeat-demo's here, fdesign's in the sibling module — a fake messaging inbox for the subscriber `r`, and the
//! observed session `c` driven by replay hook bodies shaped into the skill's protocol. Each demo scenario is one row of
//! the demo's scenario table (skills/orchestration/heartbeat-demo); the gate rows (hand-write, hand-spawn, plan-mode)
//! belong to the tool-policy gate and are tested there. No test ever touches a real session's socket.

use std::path::Path;
use std::process::{Child, Command, Output, Stdio};

use serde_json::{Value, json};

use crate::common::{Daemon, FakeInbox, Home, TOKEN, row};

/// The runtime subscriber.
const R: &str = "r";
/// The observed session.
pub(super) const C: &str = "c";
/// The demo's committed declaration: the same data the skill deploys.
const DEMO: &str = include_str!("../fixtures/enforce/heartbeat-demo.json");
/// The demo stage's script path, as the dispatcher passes it.
const STAGE: &str = "/home/k/.claude/skills/heartbeat-demo/workflows/demo.js";

/// One scenario's world: home with one deployed declaration, subscriber inbox, daemon, session processes.
pub(super) struct Rig {
    home: Home,
    inbox: FakeInbox,
    pub(super) daemon: Daemon,
    children: Vec<Child>,
}

impl Rig {
    /// [`Rig::deploy`] with the heartbeat-demo declaration.
    fn start(config: &Value) -> Rig {
        Rig::deploy("heartbeat-demo", DEMO, config)
    }

    /// Starts the daemon over a home with `declaration` deployed as `skill`'s `enforce.json` and the heartbeat config
    /// `config`, and registers `r` (with the inbox socket) and `c`, both started.
    pub(super) fn deploy(skill: &str, declaration: &str, config: &Value) -> Rig {
        let home = Home::new();
        let deployed = home.path().join("skills").join(skill);
        std::fs::create_dir_all(&deployed).expect("skills dir");
        std::fs::write(deployed.join("enforce.json"), declaration).expect("enforce.json");
        let args = home.config_args(config);
        let refs: Vec<&str> = args.iter().map(String::as_str).collect();
        let daemon = Daemon::start_with(home.path(), &refs);
        let mut rig = Rig {
            home,
            inbox: FakeInbox::start(),
            daemon,
            children: Vec::new(),
        };
        let socket = rig.inbox.path.clone();
        rig.session(R, "CC,Rt - Runtime - [WIP]", Some(&socket));
        rig.session(C, "CC,HD - Demo - [WIP]", None);
        rig
    }

    /// A live `sleep` process registered as `session`, then its `SessionStart`.
    fn session(&mut self, session: &str, title: &str, socket: Option<&Path>) {
        let child = Command::new("sleep")
            .arg("60")
            .stdout(Stdio::null())
            .spawn()
            .expect("sleep");
        self.home.registry_titled(
            child.id(),
            session,
            "idle",
            title,
            Path::new("/tmp/heartbeat-fixture"),
            socket,
        );
        self.children.push(child);
        assert_eq!(
            self.daemon
                .post(&self.home.root_hook("SessionStart", session)),
            200
        );
        self.daemon.wait_for_state(session, "idle");
    }

    /// `r` watches `c` in `mode`; asserts success.
    fn subscribed(&self, mode: &str) {
        let out = self.subscribe_as(R, C, &self.inbox.path, mode);
        assert!(out.status.success(), "{}", stderr(&out));
    }

    /// `heartbeat runtime subscribe --target <target> --mode <mode>` as `session`, handing over `socket` and
    /// [`TOKEN`].
    fn subscribe_as(&self, session: &str, target: &str, socket: &Path, mode: &str) -> Output {
        self.daemon.cli(
            &[
                "runtime",
                "subscribe",
                "--target",
                target,
                "--mode",
                mode,
                "--format",
                "json",
            ],
            &[
                ("CLAUDE_CODE_SESSION_ID", session),
                ("CLAUDE_CODE_MESSAGING_SOCKET", &text(socket)),
                ("CLAUDE_CODE_MESSAGING_TOKEN", TOKEN),
            ],
        )
    }

    /// `heartbeat runtime <move> <finding>` as `r`.
    fn act(&self, action: &str, finding: &str) -> Output {
        self.daemon.cli_as(R, TOKEN, &["runtime", action, finding])
    }

    /// Posts `event` for the root of `c`, shaped by `shape`; the answer.
    pub(super) fn root(&self, event: &str, shape: impl FnOnce(&mut Value)) -> Value {
        let body: Value = serde_json::from_str(&self.home.root_hook(event, C)).expect("hook body");
        self.post(body, shape)
    }

    /// Posts `event` for subagent `agent` of `c`, shaped by `shape`; the answer.
    fn agent(&self, event: &str, agent: &str, shape: impl FnOnce(&mut Value)) -> Value {
        let body: Value =
            serde_json::from_str(&self.home.agent_hook(event, C, agent)).expect("hook body");
        self.post(body, shape)
    }

    fn post(&self, mut body: Value, shape: impl FnOnce(&mut Value)) -> Value {
        shape(&mut body);
        let (status, answer) = self.daemon.post_json(&body.to_string());
        assert_eq!(status, 200, "{answer}");
        answer
    }

    /// The task notification that opens the root's next turn once a launch completed.
    fn notified(&self) -> Value {
        self.root("UserPromptSubmit", |body| {
            body["prompt"] =
                json!("<task-notification><status>completed</status></task-notification>");
        })
    }

    /// `c`'s session state in the status document.
    pub(super) fn state(&self) -> Value {
        let document = self.daemon.document();
        row(&document, C).map_or(Value::Null, |found| found["state"].clone())
    }

    /// `c` opens a heartbeat-demo run.
    fn open(&self) {
        let answer = self.root("UserPromptSubmit", |body| {
            body["prompt"] = json!("/heartbeat-demo scenario:pass-clean-run");
        });
        assert_eq!(answer, json!({}));
    }

    /// PROC-001: the producer prints `value`.
    fn produce(&self, value: &str) -> Value {
        self.root("PostToolUse", |body| {
            body["tool_name"] = json!("Bash");
            body["tool_input"] = json!({ "command": "smine-rules demo-value --seed c" });
            body["tool_response"] =
                json!({ "stdout": json!({ "value": value }).to_string(), "stderr": "", "interrupted": false });
        })
    }

    /// PROC-002 before the call: the stage with `value`.
    fn stage(&self, value: &str) -> Value {
        self.root("PreToolUse", |body| {
            body["tool_name"] = json!("Workflow");
            body["tool_input"] =
                json!({ "scriptPath": STAGE, "args": { "value": value, "artifactsDir": "/a" } });
        })
    }

    /// PROC-002 returned.
    fn staged(&self, value: &str) -> Value {
        self.root("PostToolUse", |body| {
            body["tool_name"] = json!("Workflow");
            body["tool_input"] =
                json!({ "scriptPath": STAGE, "args": { "value": value, "artifactsDir": "/a" } });
            body["tool_response"] = json!({ "artifact": "/a/artifact.json" });
        })
    }

    /// The root's turn end with `in_flight` background tasks listed in the body (`background_tasks`).
    pub(super) fn stop(&self, in_flight: usize) -> Value {
        self.root("Stop", |body| {
            let tasks: Vec<Value> = (1..=in_flight)
                .map(|n| {
                    json!({ "id": format!("t{n}"), "type": "local_workflow", "status": "running", "description": "demo stage" })
                })
                .collect();
            body["background_tasks"] = json!(tasks);
        })
    }

    /// PROC-004.
    fn close(&self) -> Value {
        self.root("PostToolUse", |body| {
            body["tool_name"] = json!("Bash");
            body["tool_input"] = json!({ "command": "smine-rules demo-close --dir /a" });
            body["tool_response"] = json!({ "stdout": "{\"closed\": true}", "stderr": "" });
        })
    }

    /// A root call of `tool` before it runs.
    fn call(&self, tool: &str) -> Value {
        self.root("PreToolUse", |body| {
            body["tool_name"] = json!(tool);
            body["tool_input"] = json!({ "file_path": "/tmp/scratch.txt" });
        })
    }

    /// The finding ids listed once `count` findings exist.
    fn findings(&self, count: usize) -> Vec<String> {
        let document = self.daemon.wait_runtime("findings", |doc| {
            doc["findings"]
                .as_array()
                .is_some_and(|rows| rows.len() >= count)
        });
        document["findings"]
            .as_array()
            .expect("findings")
            .iter()
            .map(|row| row["id"].as_str().expect("finding id").to_owned())
            .collect()
    }
}

impl Drop for Rig {
    fn drop(&mut self) {
        for child in &mut self.children {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

fn text(path: &Path) -> String {
    path.to_str().expect("utf-8 temp path").to_owned()
}

fn stderr(out: &Output) -> String {
    String::from_utf8_lossy(&out.stderr).into_owned()
}

pub(super) fn denied(answer: &Value, reason: &str) -> bool {
    answer["hookSpecificOutput"]["permissionDecision"] == "deny"
        && answer["hookSpecificOutput"]["permissionDecisionReason"]
            .as_str()
            .is_some_and(|text| text.contains(reason))
}

#[test]
fn pass_clean_run() {
    let rig = Rig::start(&json!({}));
    rig.open();
    assert_eq!(rig.produce("abc"), json!({}));
    assert_eq!(rig.stage("abc"), json!({}));
    assert_eq!(rig.staged("abc"), json!({}));
    assert_eq!(rig.stop(1), json!({}), "the launch is awaited");
    let notified = rig.root("UserPromptSubmit", |body| {
        body["prompt"] = json!("<task-notification><status>completed</status></task-notification>");
    });
    assert_eq!(notified, json!({}));
    assert_eq!(rig.close(), json!({}));
    assert_eq!(rig.root("Stop", |_| {}), json!({}));
    let document = rig.daemon.runtime_document();
    assert_eq!(document["runs"][0]["skill"], "heartbeat-demo");
    assert_eq!(document["runs"][0]["state"], "open");
    let anchors: Vec<&str> = document["runs"][0]["anchors"]
        .as_array()
        .expect("anchors")
        .iter()
        .filter_map(|hit| hit["id"].as_str())
        .collect();
    assert_eq!(anchors, ["produced", "stage", "closed"]);
    assert_eq!(document["runs"][0]["failures"], json!([]));
    assert_eq!(
        document["runs"][0]["passes"],
        json!([
            {"check": "close_before_stop", "count": 1},
            {"check": "producer_first", "count": 1},
            {"check": "value_provenance", "count": 1}
        ]),
        "every declared check but the never-guarded phase passed once"
    );
    assert_eq!(document["findings"], json!([]));
}

#[test]
fn deny_laundered_workflow_then_kill_repeated_launder() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    for launch in 1..=7 {
        let answer = rig.stage("zzz");
        assert!(
            denied(&answer, "args.value must be the value"),
            "launch {launch}: {answer}"
        );
        assert!(
            answer.get("continue").is_none(),
            "launch {launch} only denies: {answer}"
        );
    }
    let document = rig.daemon.runtime_document();
    let kinds: Vec<&str> = document["findings"]
        .as_array()
        .expect("findings")
        .iter()
        .filter_map(|row| row["kind"].as_str())
        .collect();
    assert_eq!(kinds, ["check_failed"; 7]);
    assert_eq!(document["findings"][0]["entry"], "value_provenance");

    let eighth = rig.stage("zzz");
    assert_eq!(
        eighth["continue"], false,
        "the eighth launch stops at once: {eighth}"
    );
    assert!(denied(&eighth, "killed"), "{eighth}");
    assert_eq!(eighth["stopReason"], eighth["systemMessage"], "{eighth}");
    assert!(
        eighth["systemMessage"]
            .as_str()
            .is_some_and(|text| text.contains("value_provenance failed 8 times")),
        "{eighth}"
    );
    let after = rig.call("Read");
    assert_eq!(
        after["continue"], false,
        "the kill stops the session: {after}"
    );
    assert_eq!(after["systemMessage"], eighth["systemMessage"], "{after}");
    let killed = rig.daemon.runtime_document();
    assert_eq!(killed["runs"][0]["state"], "killed");
    assert_eq!(killed["runs"][0]["reason"], "value_provenance ×8");
    assert_eq!(
        killed["runs"][0]["failures"],
        json!([{"check": "value_provenance", "count": 8}])
    );
    assert_eq!(
        killed["runs"][0]["passes"],
        json!([{"check": "producer_first", "count": 8}]),
        "the killed run keeps its passes; every denied call held producer_first"
    );
    assert_eq!(
        killed["findings"].as_array().map(Vec::len),
        Some(7),
        "the kill raises no finding"
    );

    let resumed = rig.root("UserPromptSubmit", |body| {
        body["prompt"] = json!("carry on");
    });
    assert_eq!(resumed, json!({}));
    assert_eq!(
        rig.call("Read"),
        json!({}),
        "the next prompt clears the kill"
    );
    let reset = rig.daemon.runtime_document();
    assert_eq!(reset["runs"][0]["state"], "open");
    assert_eq!(reset["runs"][0]["failures"], json!([]));
    let retry = rig.stage("zzz");
    assert!(denied(&retry, "args.value must be the value"), "{retry}");
    assert!(
        retry.get("continue").is_none(),
        "a fresh budget: the retry only denies: {retry}"
    );
}

#[test]
fn deny_producer_skipped() {
    let rig = Rig::start(&json!({}));
    rig.open();
    let answer = rig.stage("abc");
    assert!(
        denied(&answer, "run `smine-rules demo-value` before the Workflow"),
        "{answer}"
    );
    let document = rig.daemon.runtime_document();
    assert_eq!(
        document["runs"][0]["failures"],
        json!([{"check": "producer_first", "count": 1}]),
        "value_provenance broke on the same call; only the first failure in id order counts"
    );
    assert_eq!(document["runs"][0]["passes"], json!([]));
}

#[test]
fn deny_edit_after_stage() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    rig.staged("abc");
    let answer = rig.call("Write");
    assert!(
        denied(&answer, "no edits between the stage and demo-close"),
        "{answer}"
    );
    rig.close();
    assert_eq!(rig.call("Write"), json!({}));
}

#[test]
fn block_stop_before_close() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    rig.staged("abc");
    assert_eq!(
        rig.stop(1),
        json!({}),
        "the turn end that awaits the launch is not judged"
    );
    let notified = rig.root("UserPromptSubmit", |body| {
        body["prompt"] = json!("<task-notification><status>completed</status></task-notification>");
    });
    assert_eq!(notified, json!({}));
    let blocked = rig.root("Stop", |_| {});
    assert_eq!(blocked["decision"], "block", "{blocked}");
    assert!(
        blocked["reason"]
            .as_str()
            .is_some_and(|reason| reason.contains("demo-close")),
        "{blocked}"
    );
    rig.close();
    assert_eq!(rig.root("Stop", |_| {}), json!({}));
}

#[test]
fn report_value_judgment() {
    let rig = Rig::start(&json!({}));
    rig.subscribed("report");
    rig.open();
    rig.produce("abc");
    rig.staged("abc");
    let frames = rig.inbox.wait_frames(1);
    let user: Value = serde_json::from_str(&frames[0][1]).expect("user frame");
    let content = user["message"]["content"].as_str().expect("content");
    assert!(
        content.contains("judgment heartbeat-demo value_check"),
        "{content}"
    );
    assert!(content.contains("value = \"abc\""), "{content}");
}

#[test]
fn recover_laundered_workflow() {
    let rig = Rig::start(&json!({}));
    rig.subscribed("act");
    rig.open();
    rig.produce("abc");
    let first = rig.stage("zzz");
    assert!(denied(&first, "args.value"), "{first}");
    let ids = rig.findings(1);
    let out = rig.act("send", &ids[0]);
    assert!(out.status.success(), "{}", stderr(&out));
    let handed = rig.root("PostToolUse", |body| {
        body["tool_name"] = json!("Read");
        body["tool_input"] = json!({ "file_path": "/tmp/x" });
    });
    assert!(
        handed["hookSpecificOutput"]["additionalContext"]
            .as_str()
            .is_some_and(|text| text.contains("pass exactly the value")),
        "{handed}"
    );
    assert_eq!(rig.stage("abc"), json!({}), "the recovered call passes");
    let again = rig.act("send", &ids[0]);
    assert_eq!(again.status.code(), Some(1));
    assert!(
        stderr(&again).contains("already answered"),
        "{}",
        stderr(&again)
    );
}

#[test]
fn a_session_has_one_watcher() {
    let mut rig = Rig::start(&json!({}));
    rig.subscribed("report");
    let other = FakeInbox::start();
    rig.session("q", "CC,Rt - Second runtime - [WIP]", Some(&other.path));
    let out = rig.subscribe_as("q", C, &other.path, "act");
    assert_eq!(out.status.code(), Some(1), "{}", stderr(&out));
    assert!(
        stderr(&out).contains("c is already watched by r"),
        "{}",
        stderr(&out)
    );
    let document = rig.daemon.runtime_document();
    assert_eq!(document["schema"], 1);
    let watches = document["subscriptions"].as_array().expect("watches");
    assert_eq!(watches.len(), 1);
    assert_eq!(watches[0]["session"], R);
    assert_eq!(watches[0]["target"], C);
}

#[test]
fn engine_chain_keeps_precedence() {
    let rig = Rig::start(&json!({ "forbidden_subagent_models": ["*fable*"] }));
    rig.subscribed("act");
    rig.open();
    rig.produce("abc");
    rig.stage("zzz");
    let ids = rig.findings(1);
    let out = rig.act("stop", &ids[0]);
    assert!(out.status.success(), "{}", stderr(&out));
    let spawn = rig.root("PreToolUse", |body| {
        body["tool_name"] = json!("Agent");
        body["tool_input"] =
            json!({ "subagent_type": "Plan", "description": "d", "prompt": "p", "model": "fable" });
    });
    assert_eq!(spawn["hookSpecificOutput"]["permissionDecision"], "deny");
    assert!(
        spawn.get("continue").is_none(),
        "the heart attack's deny wins over the armed stop: {spawn}"
    );
    let armed = rig.call("Read");
    assert_eq!(armed["continue"], false, "{armed}");
    let status = rig.daemon.document();
    assert!(row(&status, C).is_some());
}

#[test]
fn capture_after_log_lines() {
    let rig = Rig::start(&json!({}));
    rig.open();
    let produced = rig.root("PostToolUse", |body| {
        body["tool_name"] = json!("Bash");
        body["tool_input"] = json!({ "command": "smine-rules demo-value --seed c" });
        body["tool_response"] = json!({
            "stdout": "2026/10/02 01:41:07 shell: git dur=58ms err=false\n{\n  \"value\": \"abc\"\n}\n",
            "stderr": "",
            "interrupted": false
        });
    });
    assert_eq!(produced, json!({}));
    assert_eq!(
        rig.stage("abc"),
        json!({}),
        "the JSON after the log line is the capture"
    );
}

#[test]
fn in_flight_turn_end_is_not_judged() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    rig.staged("abc");
    assert_eq!(
        rig.stop(1),
        json!({}),
        "the launch turn's Stop with one task in flight"
    );
    let judged = rig.stop(0);
    assert_eq!(
        judged["decision"], "block",
        "nothing in flight: judged: {judged}"
    );
    rig.close();
    assert_eq!(rig.stop(0), json!({}), "the Stop after the close");
    let document = rig.daemon.runtime_document();
    assert_eq!(
        document["runs"][0]["failures"],
        json!([{"check": "close_before_stop", "count": 1}]),
        "the in-flight Stop counted nothing"
    );
}

#[test]
fn lost_subagent_stop_exempts_no_stop() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    rig.staged("abc");
    rig.stop(1);
    rig.notified();
    assert_eq!(rig.agent("SubagentStart", "a1", |_| {}), json!({}));
    let judged = rig.stop(0);
    assert_eq!(
        judged["decision"], "block",
        "a subagent whose SubagentStop never came exempts no Stop with nothing in flight: {judged}"
    );
}

#[test]
fn subagent_dialog_keeps_the_root_running() {
    let rig = Rig::start(&json!({}));
    rig.root("UserPromptSubmit", |body| {
        body["prompt"] = json!("go");
    });
    rig.agent("SubagentStart", "a1", |_| {});
    rig.agent("PermissionRequest", "a1", |_| {});
    assert_eq!(
        rig.state(),
        "running",
        "a subagent's permission dialog is its own"
    );
}

#[test]
fn stale_capture_is_never_compared() {
    let rig = Rig::start(&json!({}));
    rig.open();
    rig.produce("abc");
    let voided = rig.root("PostToolUse", |body| {
        body["tool_name"] = json!("Bash");
        body["tool_input"] = json!({ "command": "smine-rules demo-value --seed c" });
        body["tool_response"] =
            json!({ "stdout": "demo-value: seed cache cold\n", "stderr": "", "interrupted": false });
    });
    assert_eq!(voided, json!({}));
    let answer = rig.stage("abc");
    assert!(
        denied(&answer, "args.value must be the value"),
        "the last producer printed no value, the older one is not compared: {answer}"
    );
    let document = rig.daemon.runtime_document();
    let captures: Vec<&Value> = document["runs"][0]["anchors"]
        .as_array()
        .expect("anchors")
        .iter()
        .filter(|hit| hit["id"] == "produced")
        .map(|hit| &hit["captures"])
        .collect();
    assert_eq!(
        captures,
        [&json!({ "value": "abc" }), &json!({})],
        "each producing call keeps its own capture"
    );
}
