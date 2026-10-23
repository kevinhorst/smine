//! Config edge scenarios against the built daemon: a rewritten config's live keys apply without a restart, a
//! malformed rewrite keeps the running config and degrades the `config` sensor, and a malformed or missing config at
//! start ends the daemon with exit 1.

use std::process::{Command, Output};
use std::thread;
use std::time::{Duration, Instant};

use serde_json::{Value, json};

use crate::common::{Daemon, Home, NO_PEEK, sensor};

const POLL: Duration = Duration::from_millis(50);
const DEADLINE: Duration = Duration::from_secs(5);
/// A config body whose `kill_at` has the wrong type.
const MALFORMED: &str = r#"{"kill_at": "x"}"#;

/// Starts the daemon on a config forbidding `forbidden`, and waits for the config sensor's first `ok`.
fn start(home: &Home, forbidden: &[&str]) -> Daemon {
    let args = home.config_args(&json!({ "forbidden_subagent_models": forbidden }));
    let refs: Vec<&str> = args.iter().map(String::as_str).collect();
    let daemon = Daemon::start_with(home.path(), &refs);
    daemon.wait_for("config sensor ok", |doc| {
        sensor(doc, "config").is_some_and(|config| config["state"] == "ok")
    });
    daemon
}

/// The permission decision of a root `Agent` call on `fable` by `s1`; `None` when the call is allowed.
fn fable_decision(daemon: &Daemon, home: &Home) -> Option<String> {
    let mut spawn: Value =
        serde_json::from_str(&home.root_hook("PreToolUse", "s1")).expect("hook body");
    spawn["tool_name"] = json!("Agent");
    spawn["tool_input"] =
        json!({ "subagent_type": "Plan", "description": "d", "prompt": "p", "model": "fable" });
    let (status, answer) = daemon.post_json(&spawn.to_string());
    assert_eq!(status, 200);
    answer["hookSpecificOutput"]["permissionDecision"]
        .as_str()
        .map(str::to_owned)
}

/// Runs `heartbeat serve --config <config>` on `home` until it exits.
fn serve_once(home: &Home, config: &str) -> Output {
    Command::new(env!("CARGO_BIN_EXE_heartbeat"))
        .args(["--addr", "127.0.0.1:0", "--claude-home"])
        .arg(home.path())
        .args(["serve", "--config", config])
        .env_remove("HEARTBEAT_ADDR")
        .env_remove("CLAUDE_CONFIG_DIR")
        .output()
        .expect("run heartbeat serve")
}

#[test]
fn reload_applies_live_keys() {
    let home = Home::new();
    let daemon = start(&home, &[]);
    assert_eq!(daemon.post(&home.root_hook("SessionStart", "s1")), 200);
    assert_eq!(
        fable_decision(&daemon, &home),
        None,
        "nothing forbidden yet"
    );

    home.write_config(&json!({ "forbidden_subagent_models": ["*fable*"], "peek_addr": NO_PEEK }));
    let started = Instant::now();
    while fable_decision(&daemon, &home).as_deref() != Some("deny") {
        assert!(
            started.elapsed() < DEADLINE,
            "the reloaded forbidden list never applied: {:#}",
            daemon.document()
        );
        thread::sleep(POLL);
    }
    let doc = daemon.document();
    let config = sensor(&doc, "config").expect("config sensor");
    assert_eq!(config["state"], "ok", "{doc:#}");
}

#[test]
fn malformed_reload_keeps_config() {
    let home = Home::new();
    let daemon = start(&home, &["*fable*"]);
    assert_eq!(daemon.post(&home.root_hook("SessionStart", "s1")), 200);
    assert_eq!(fable_decision(&daemon, &home).as_deref(), Some("deny"));

    home.write_config_raw(MALFORMED);
    let doc = daemon.wait_for("config sensor degraded", |doc| {
        sensor(doc, "config").is_some_and(|config| config["state"] == "degraded")
    });
    let config = sensor(&doc, "config").expect("config sensor");
    assert!(
        config["detail"]
            .as_str()
            .is_some_and(|detail| detail.starts_with("config kept:")),
        "{config}"
    );
    assert_eq!(
        fable_decision(&daemon, &home).as_deref(),
        Some("deny"),
        "the running forbidden list stays"
    );
}

#[test]
fn malformed_at_start_exits() {
    let home = Home::new();
    let cases: [(&str, &str, &str); 2] = [
        ("unknown key", r#"{"kill_after": 3}"#, "kill_after"),
        ("wrong type", MALFORMED, "invalid type"),
    ];
    for (label, body, want) in cases {
        home.write_config_raw(body);
        let path = home.config_path();
        let out = serve_once(&home, path.to_str().expect("utf-8 temp path"));
        let stderr = String::from_utf8_lossy(&out.stderr);
        assert_eq!(out.status.code(), Some(1), "{label}: {stderr}");
        assert!(stderr.contains("heartbeat config"), "{label}: {stderr}");
        assert!(stderr.contains(want), "{label}: {stderr}");
    }
}

#[test]
fn missing_config_at_start_exits() {
    let home = Home::new();
    let missing = home.path().join("absent.json");
    let missing = missing.to_str().expect("utf-8 temp path");
    let out = serve_once(&home, missing);
    let stderr = String::from_utf8_lossy(&out.stderr);
    assert_eq!(out.status.code(), Some(1), "{stderr}");
    assert!(stderr.contains(missing), "{stderr}");
}
