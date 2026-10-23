//! Scenarios against the built daemon. Everything but the schema and exit-code checks needs kqueue, hence darwin
//! only (D13).

use std::path::Path;
use std::process::Command;

use serde_json::Value;

/// The schema `heartbeat schema <args>` prints equals the committed file `schema/<file>`.
fn assert_schema_committed(args: &[&str], file: &str) {
    let out = Command::new(env!("CARGO_BIN_EXE_heartbeat"))
        .arg("schema")
        .args(args)
        .output()
        .expect("spawn");
    assert!(out.status.success());
    let generated: Value = serde_json::from_slice(&out.stdout).expect("schema json");
    let committed_path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("schema")
        .join(file);
    let committed: Value =
        serde_json::from_slice(&std::fs::read(&committed_path).expect("committed schema"))
            .expect("committed json");
    let command = ["heartbeat", "schema"]
        .iter()
        .chain(args)
        .copied()
        .collect::<Vec<_>>()
        .join(" ");
    assert_eq!(
        generated, committed,
        "schema drift: regenerate with `{command} > schema/{file}`"
    );
}

#[test]
fn schema_is_committed() {
    assert_schema_committed(&[], "status-v1.json");
}

#[test]
fn archive_schema_is_committed() {
    assert_schema_committed(&["--archive"], "archive-v1.json");
}

#[test]
fn runtime_schema_is_committed() {
    assert_schema_committed(&["--runtime"], "runtime-v1.json");
}

#[test]
fn interfaces_answers_claims() {
    let out = Command::new(env!("CARGO_BIN_EXE_heartbeat"))
        .arg("interfaces")
        .output()
        .expect("spawn");
    assert!(out.status.success());
    let answer: Value = serde_json::from_slice(&out.stdout).expect("interfaces json");
    assert_eq!(
        answer,
        serde_json::json!({"interfaces": [
            "heartbeat-done@1",
            "heartbeat-join@1",
            "heartbeat-land@1",
            "heartbeat-leave@1",
            "heartbeat-record@1",
            "heartbeat-remove@1",
            "heartbeat-runtime@1",
            "heartbeat-status@1",
            "heartbeat-subscribe@1",
            "heartbeat-unsubscribe@1",
            "heartbeat-worker@1",
        ]})
    );
    assert_eq!(String::from_utf8_lossy(&out.stdout).lines().count(), 1);
}

#[test]
fn unreachable_daemon_exits_3() {
    let out = Command::new(env!("CARGO_BIN_EXE_heartbeat"))
        .args(["--addr", "127.0.0.1:1", "--claude-home", "/tmp", "status"])
        .output()
        .expect("run");
    assert_eq!(
        out.status.code(),
        Some(3),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
}

#[cfg(target_os = "macos")]
mod git;

#[cfg(target_os = "macos")]
mod fleet;

#[cfg(target_os = "macos")]
mod attack;

#[cfg(target_os = "macos")]
mod runtime;

#[cfg(target_os = "macos")]
mod fdesign;

#[cfg(target_os = "macos")]
mod config;

#[cfg(target_os = "macos")]
mod daemon {
    use std::process::{Command, Stdio};

    use crate::common::{Daemon, Home, ledger_now, row, sensor};

    #[test]
    fn hook_lifecycle() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        assert_eq!(daemon.post(&home.hook("SessionStart", "s1")), 200);
        let started = daemon.wait_for_state("s1", "idle");
        assert_eq!(started["since_restart"], false);
        assert_eq!(daemon.post(&home.hook("UserPromptSubmit", "s1")), 200);
        let prompted = daemon.wait_for_state("s1", "running");
        assert_eq!(prompted["active_skill"], "fdesign");
        assert_eq!(daemon.post(&home.hook("Stop", "s1")), 200);
        daemon.wait_for_state("s1", "idle");
        assert_eq!(daemon.post(&home.hook("SessionEnd", "s1")), 200);
        let ended = daemon.wait_for_state("s1", "ended");
        assert_eq!(ended["end_reason"], "prompt_input_exit");
    }

    #[test]
    fn sensors_report_ok() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        let document = daemon.document();
        for name in ["hooks", "registry", "process_exit", "ledger"] {
            assert_eq!(
                sensor(&document, name).map(|row| row["state"].clone()),
                Some("ok".into()),
                "{name}"
            );
        }
    }

    #[test]
    fn registry_dir_missing_degrades() {
        let home = Home::bare();
        let daemon = Daemon::start(home.path());
        let document = daemon.document();
        let registry = sensor(&document, "registry").expect("registry sensor");
        assert_eq!(registry["state"], "degraded");
        assert!(
            registry["detail"]
                .as_str()
                .is_some_and(|detail| detail.contains("sessions"))
        );
    }

    #[test]
    fn foreign_home_is_refused() {
        let home = Home::new();
        let other = Home::new();
        let daemon = Daemon::start(home.path());
        assert_eq!(daemon.post(&other.hook("SessionStart", "foreign")), 409);
        let document = daemon.document();
        assert!(row(&document, "foreign").is_none());
        assert_eq!(
            sensor(&document, "hooks").map(|row| row["state"].clone()),
            Some("degraded".into())
        );
        assert_eq!(daemon.post(&home.hook("SessionStart", "own")), 200);
        daemon.wait_for_state("own", "idle");
        assert_eq!(
            sensor(&daemon.document(), "hooks").map(|row| row["state"].clone()),
            Some("ok".into())
        );
    }

    #[test]
    fn denial_blocks_until_prompt() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        daemon.post(&home.hook("SessionStart", "s1"));
        daemon.post(&home.hook("UserPromptSubmit", "s1"));
        home.deny("s1", "git -C /x status", "Bash(git *)", &ledger_now());
        daemon.post(&home.hook("Stop", "s1"));
        let blocked = daemon.wait_for_state("s1", "blocked");
        assert_eq!(blocked["blocked"]["candidate_rule"], "Bash(git *)");
        assert_eq!(blocked["blocked"]["program"], "Bash:git");
        daemon.post(&home.hook("UserPromptSubmit", "s1"));
        daemon.wait_for_state("s1", "running");
    }

    #[test]
    fn registry_seed_then_exit_dies() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        let mut worker = Command::new("sleep")
            .arg("60")
            .stdout(Stdio::null())
            .spawn()
            .expect("sleep");
        home.registry(worker.id(), "s1", "busy");
        let seeded = daemon.wait_for_state("s1", "running");
        assert_eq!(seeded["since_restart"], true);
        assert_eq!(seeded["title"], "CC,FD - fixture - [WIP]");
        worker.kill().expect("kill worker");
        worker.wait().expect("reap worker");
        let died = daemon.wait_for_state("s1", "died");
        assert_eq!(died["end_reason"], "process exited without SessionEnd");
    }

    #[test]
    fn stale_registry_file_is_dropped() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        let baseline = daemon.document()["heartbeat"]["events_applied"]
            .as_u64()
            .unwrap_or(0);
        let mut gone = Command::new("true").spawn().expect("true");
        gone.wait().expect("reap");
        home.registry(gone.id(), "stale", "idle");
        let document = daemon.wait_for_events(baseline + 2);
        assert!(row(&document, "stale").is_none(), "{document:#}");
    }

    #[test]
    fn placeholder_without_transcript_is_never_listed() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        let spawn = || {
            Command::new("sleep")
                .arg("60")
                .stdout(Stdio::null())
                .spawn()
                .expect("sleep")
        };
        let mut placeholder = spawn();
        let mut real = spawn();
        home.placeholder(placeholder.id(), "placeholder", "idle");
        home.registry(real.id(), "real", "busy");
        daemon.wait_for_state("real", "running");
        placeholder.kill().expect("kill placeholder");
        placeholder.wait().expect("reap placeholder");
        real.kill().expect("kill real");
        real.wait().expect("reap real");
        daemon.wait_for_state("real", "died");
        let document = daemon.document();
        assert!(row(&document, "placeholder").is_none(), "{document:#}");
    }

    #[test]
    fn restart_rebuilds_from_registry() {
        let home = Home::new();
        let mut worker = Command::new("sleep")
            .arg("60")
            .stdout(Stdio::null())
            .spawn()
            .expect("sleep");
        home.registry(worker.id(), "s1", "idle");
        let hour_ago = format!(
            "{:.0}",
            jiff::Timestamp::now() - jiff::SignedDuration::from_hours(1)
        );
        home.deny("s1", "ps -o pid", "Bash(ps *)", &hour_ago);
        let first = Daemon::start(home.path());
        first.wait_for_state("s1", "idle");
        drop(first);
        let second = Daemon::start(home.path());
        let rebuilt = second.wait_for_state("s1", "idle");
        assert_eq!(rebuilt["since_restart"], true);
        assert!(
            rebuilt.get("blocked").is_none(),
            "old denial re-blocked: {rebuilt}"
        );
        worker.kill().expect("kill worker");
        worker.wait().expect("reap worker");
    }

    #[test]
    fn cli_session_self_and_foreign_home() {
        let home = Home::new();
        let daemon = Daemon::start(home.path());
        daemon.post(&home.hook("SessionStart", "s1"));
        daemon.post(&home.hook("SessionStart", "s2"));
        daemon.wait_for_state("s2", "idle");
        let out = daemon.cli(
            &["status", "--format", "json", "--session", "self"],
            &[("CLAUDE_CODE_SESSION_ID", "s1")],
        );
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        let document: serde_json::Value =
            serde_json::from_slice(&out.stdout).expect("stdout is pure JSON");
        assert_eq!(document["sessions"].as_array().map(Vec::len), Some(1));
        let other = Home::new();
        let foreign = Command::new(env!("CARGO_BIN_EXE_heartbeat"))
            .args(["--addr", &daemon.addr.to_string(), "--claude-home"])
            .arg(other.path())
            .args(["status", "--format", "json"])
            .output()
            .expect("run");
        assert_eq!(foreign.status.code(), Some(4));
        assert!(String::from_utf8_lossy(&foreign.stderr).contains("serves claude home"));
    }

    #[test]
    fn record_writes_replay_lines() {
        let home = Home::new();
        let record = home.path().join("hooks.jsonl");
        let daemon =
            Daemon::start_with(home.path(), &["--record", record.to_str().expect("utf-8")]);
        daemon.post(&home.hook("SessionStart", "s1"));
        daemon.wait_for_state("s1", "idle");
        let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
        let line = loop {
            if let Some(line) = std::fs::read_to_string(&record)
                .ok()
                .and_then(|text| text.split_once('\n').map(|(line, _)| line.to_owned()))
            {
                break line;
            }
            assert!(
                std::time::Instant::now() < deadline,
                "recorder wrote nothing"
            );
            std::thread::sleep(std::time::Duration::from_millis(50));
        };
        let recorded: serde_json::Value = serde_json::from_str(&line).expect("replay line");
        assert!(recorded["at"].as_i64().is_some());
        assert_eq!(recorded["body"]["hook_event_name"], "SessionStart");
    }
}
