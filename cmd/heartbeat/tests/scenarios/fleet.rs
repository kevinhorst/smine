//! The orchestrator loop end to end (P3): a registered fixture repo, the daemon, a fake central peek and a fake
//! messaging inbox. Sessions are `sleep` children registered with a chosen title and driven by replay hook bodies
//! whose cwd is the session's own. Workers work in the `claude/live` worktree, which holds an unmerged commit, so the
//! fleet never turns `integrated` on its own and every digest a scenario counts is a member change. No test ever
//! touches a real session's socket.

use std::collections::BTreeMap;
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Output, Stdio};

use serde_json::{Value, json};

use crate::common::git::GitRepo;
use crate::common::{Daemon, FakeInbox, FakePeek, Home, TOKEN, row, sensor};

const ORCHESTRATOR: &str = "CC,FD - Orchestrator - [WIP]";
const WORKER: &str = "CC,FImpl - Worker - T1[WIP]";
const SECOND: &str = "CC,FImpl - Second - T1[WIP]";
/// The orchestrator's merge branch before its run uuid.
const MERGE_PREFIX: &str = "claude-orch/T1-";
/// The local branch the rig creates at `main`: the landing target of the sessions based on it.
const FEATURE: &str = "feature/x";
const PEEK_FIXTURE: &str = include_str!("../fixtures/peek-session-events.json");

/// One scenario's world: repo, home, fake peek, inbox, daemon and the session processes.
struct Rig {
    repo: GitRepo,
    /// The workers' worktree (`claude/live`, one unmerged commit).
    live: PathBuf,
    home: Home,
    peek: FakePeek,
    inbox: FakeInbox,
    daemon: Option<Daemon>,
    children: Vec<Child>,
    /// Each registered session's cwd; hooks carry it, because a hook's cwd moves the session.
    cwds: BTreeMap<String, PathBuf>,
    /// Extra heartbeat config keys, kept across restarts.
    config: Value,
}

impl Rig {
    fn new() -> Rig {
        Rig::with(GitRepo::new(), Home::new())
    }

    fn with(repo: GitRepo, home: Home) -> Rig {
        Rig::with_peek(repo, home, None)
    }

    /// `peek_home`: the Claude home the fake peek claims to serve; default the rig's own.
    fn with_peek(repo: GitRepo, home: Home, peek_home: Option<&Path>) -> Rig {
        Rig::build(repo, home, peek_home, json!({}))
    }

    /// A rig whose daemon's config names `staging` as the staging target.
    fn staged(staging: &str) -> Rig {
        let config = json!({ "staging_target": staging });
        Rig::build(GitRepo::new(), Home::new(), None, config)
    }

    /// The rig's repo also holds the local branch [`FEATURE`] at `main`, a landing target for sessions based on it.
    fn build(repo: GitRepo, home: Home, peek_home: Option<&Path>, config: Value) -> Rig {
        repo.git(&repo.path, &["branch", FEATURE, "main"]);
        let live = repo.worktree("live", "main");
        repo.commit(&live, "live.txt", "live\n", "live work");
        let peek = FakePeek::start(peek_home.unwrap_or(home.path()));
        let mut rig = Rig {
            repo,
            live,
            home,
            peek,
            inbox: FakeInbox::start(),
            daemon: None,
            children: Vec::new(),
            cwds: BTreeMap::new(),
            config,
        };
        rig.start();
        rig
    }

    /// Starts the daemon with the repo registered and the fake peek, and waits for its first scan.
    fn start(&mut self) {
        let mut config = self.config.clone();
        config["peek_addr"] = Value::from(self.peek.addr.clone());
        let args = self.home.git_args(&self.repo.path, &config);
        let refs: Vec<&str> = args.iter().map(String::as_str).collect();
        let daemon = Daemon::start_with(self.home.path(), &refs);
        daemon.wait_for("the first scan", |doc| {
            doc["heartbeat"]["scans_completed"].as_u64() >= Some(1)
        });
        self.daemon = Some(daemon);
    }

    /// P1's restart pattern: the old daemon is gone before the new one starts on the same home.
    fn restart(&mut self) {
        self.daemon = None;
        self.start();
    }

    fn daemon(&self) -> &Daemon {
        self.daemon.as_ref().expect("daemon running")
    }

    /// A live session process registered as `session` with `title`, its cwd in `cwd`.
    fn session_in(&mut self, session: &str, title: &str, cwd: &Path, socket: Option<&Path>) {
        let child = Command::new("sleep")
            .arg("60")
            .stdout(Stdio::null())
            .spawn()
            .expect("sleep");
        self.home
            .registry_titled(child.id(), session, "idle", title, cwd, socket);
        self.children.push(child);
        self.cwds.insert(session.to_owned(), cwd.to_path_buf());
    }

    /// A worker session in the `claude/live` worktree.
    fn session(&mut self, session: &str, title: &str) {
        let cwd = self.live.clone();
        self.session_in(session, title, &cwd, None);
    }

    /// The orchestrator `o` in its own worktree, registered with the inbox socket and started (idle); it switches to
    /// its merge branch once subscribed ([`Rig::subscribed`]), the way `/fleet` starts.
    fn orchestrator(&mut self) {
        let cwd = self.repo.worktree("orch", "main");
        let socket = self.inbox.path.clone();
        self.session_in("o", ORCHESTRATOR, &cwd, Some(&socket));
        self.hook("SessionStart", "o");
        self.daemon().wait_for_state("o", "idle");
    }

    /// Posts the replay body of `event` as `session`, with the session's cwd (the main checkout if unregistered).
    fn hook(&self, event: &str, session: &str) {
        let cwd = self.cwds.get(session).unwrap_or(&self.repo.path);
        let body = self.home.hook_in(event, session, cwd);
        assert_eq!(self.daemon().post(&body), 200, "{event} {session}");
    }

    /// `session` joins `fleet` once the daemon knows it; returns the member token the CLI printed.
    fn join(&self, session: &str, fleet: &str) -> String {
        self.daemon().wait_for(&format!("{session} known"), |doc| {
            row(doc, session).is_some()
        });
        let token = self.daemon().join(session, fleet);
        self.daemon()
            .wait_for(&format!("{session} joins {fleet}"), |doc| {
                has_member(doc, fleet, session)
            });
        token.trim_end().to_owned()
    }

    /// A worker session in the `claude/live` worktree, joined to T1; returns its member token.
    fn member(&mut self, session: &str, title: &str) -> String {
        self.session(session, title);
        self.join(session, "T1")
    }

    fn subscribe(&self, target: &str) -> Output {
        self.daemon().subscribe("o", "T1", target, &self.inbox)
    }

    /// `o` subscribes T1 `main` and switches to the merge branch; asserts success, returns the merge branch.
    fn subscribed(&self) -> String {
        let out = self.subscribe("main");
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        self.switch_to_merge_branch()
    }

    /// `o` reads T1's `merge_branch` (`claude-orch/T1-<run uuid>`, known once subscribed) and creates it in its
    /// worktree, the way `/fleet` starts.
    fn switch_to_merge_branch(&self) -> String {
        let doc = self.status_json(&["--fleet", "T1"]);
        let merge = fleet_row(&doc, "T1").expect("T1")["merge_branch"]
            .as_str()
            .expect("merge_branch")
            .to_owned();
        assert!(merge.starts_with(MERGE_PREFIX), "{merge}");
        self.repo
            .git(&self.cwds["o"], &["switch", "-q", "-c", &merge]);
        merge
    }

    fn ledger(&self) -> Vec<Value> {
        let path = self.home.path().join("heartbeat/actions.jsonl");
        std::fs::read_to_string(path)
            .unwrap_or_default()
            .lines()
            .map(|line| serde_json::from_str(line).expect("ledger line"))
            .collect()
    }

    fn status_json(&self, args: &[&str]) -> Value {
        let mut all = vec!["status", "--format", "json"];
        all.extend_from_slice(args);
        let out = self.daemon().cli(&all, &[]);
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        serde_json::from_slice(&out.stdout).expect("status json")
    }
}

impl Drop for Rig {
    fn drop(&mut self) {
        self.daemon = None;
        for child in &mut self.children {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

fn fleet_row<'a>(doc: &'a Value, id: &str) -> Option<&'a Value> {
    doc["fleets"]
        .as_array()?
        .iter()
        .find(|fleet| fleet["id"] == id)
}

fn subscription<'a>(doc: &'a Value, session: &str) -> Option<&'a Value> {
    doc["fleets"].as_array()?.iter().find_map(|fleet| {
        fleet["subscriptions"]
            .as_array()?
            .iter()
            .find(|row| row["session_id"] == session)
    })
}

fn pending(doc: &Value, session: &str) -> Option<u64> {
    subscription(doc, session)?["pending"].as_u64()
}

/// `(session, worker branch, via)` of every member row of `fleet`; the branch only for a worker row.
fn members(doc: &Value, fleet: &str) -> Vec<(String, Option<String>, String)> {
    doc["members"]
        .as_array()
        .map(|rows| {
            rows.iter()
                .filter(|row| row["fleet"] == fleet)
                .map(|row| {
                    let via = row["via"].as_str().unwrap_or_default().to_owned();
                    let worker = (via == "worker")
                        .then(|| row["branch"].as_str().map(str::to_owned))
                        .flatten();
                    (
                        row["session_id"].as_str().unwrap_or_default().to_owned(),
                        worker,
                        via,
                    )
                })
                .collect()
        })
        .unwrap_or_default()
}

fn has_member(doc: &Value, fleet: &str, session: &str) -> bool {
    members(doc, fleet)
        .iter()
        .any(|(id, _, via)| id == session && via != "worker")
}

/// The worker row of `branch` in `fleet`.
fn worker_row<'a>(doc: &'a Value, fleet: &str, branch: &str) -> Option<&'a Value> {
    doc["members"]
        .as_array()?
        .iter()
        .find(|row| row["fleet"] == fleet && row["via"] == "worker" && row["branch"] == branch)
}

fn user_frame(frame: &[String]) -> Value {
    assert_eq!(frame.len(), 2, "auth line plus one user line: {frame:?}");
    serde_json::from_str(&frame[1]).expect("user frame json")
}

fn mode(path: &Path) -> u32 {
    std::fs::metadata(path)
        .expect("metadata")
        .permissions()
        .mode()
        & 0o777
}

fn stderr(out: &Output) -> String {
    String::from_utf8_lossy(&out.stderr).into_owned()
}

fn branch<'a>(doc: &'a Value, name: &str) -> Option<&'a Value> {
    doc["branches"]
        .as_array()?
        .iter()
        .find(|row| row["branch"] == name)
}

/// `w1` goes idle after a turn: prompt, then stop.
fn turn(rig: &Rig, session: &str) {
    rig.hook("UserPromptSubmit", session);
    rig.hook("Stop", session);
}

#[test]
fn subscribe_registers_the_calling_session() {
    let mut rig = Rig::new();
    rig.orchestrator();
    let out = rig.subscribe("main");
    assert!(out.status.success(), "{}", stderr(&out));
    let row: Value = serde_json::from_slice(&out.stdout).expect("stdout is the row");
    let repo = rig.repo.path.to_str().expect("utf-8");
    assert_eq!(row["state"], "active");
    assert_eq!(row["session_id"], "o");
    assert_eq!(row["repo"], repo);
    assert_eq!(row["target"], "main");

    let doc = rig.daemon().document();
    let fleet = fleet_row(&doc, "T1").expect("T1 row");
    assert_eq!(fleet["repo"], repo);
    assert_eq!(fleet["target"], "main");
    assert_eq!(fleet["target_state"], "open");
    assert_eq!(fleet["subscriptions"][0]["state"], "active");
    assert_eq!(
        fleet["subscriptions"][0]["socket_path"],
        rig.inbox.path.to_str().expect("utf-8")
    );
    assert!(!doc.to_string().contains(TOKEN), "token leaked into status");

    let ledger = wait_ledger(&rig, "the subscribed line", |ledger| {
        !ledger_kinds(ledger, "subscribed").is_empty()
    });
    let subscribed = ledger_kinds(&ledger, "subscribed");
    assert_eq!(subscribed.len(), 1, "{ledger:?}");
    assert_eq!(subscribed[0]["repo"], repo);
    assert_eq!(subscribed[0]["target"], "main");
    let dir = rig.home.path().join("heartbeat");
    assert_eq!(mode(&dir), 0o700);
    assert_eq!(mode(&dir.join("actions.jsonl")), 0o600);
}

#[test]
fn existing_heartbeat_dir_is_tightened() {
    let home = Home::new();
    let dir = home.path().join("heartbeat");
    std::fs::create_dir_all(&dir).expect("heartbeat dir");
    std::fs::set_permissions(&dir, std::fs::Permissions::from_mode(0o755)).expect("chmod");
    let mut rig = Rig::with(GitRepo::new(), home);
    rig.orchestrator();
    rig.subscribed();
    assert_eq!(mode(&dir), 0o700);
    assert_eq!(mode(&dir.join("actions.jsonl")), 0o600);
}

#[test]
fn digest_reaches_idle_subscriber() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    rig.hook("Stop", "o");
    let token = rig.member("w1", WORKER);
    assert_eq!(token, "T1-MA", "the operator holds no letter");
    turn(&rig, "w1");

    let frames = rig.inbox.wait_frames(1);
    assert_eq!(
        frames[0][0],
        format!(r#"{{"token":"{TOKEN}","type":"auth"}}"#)
    );
    let user = user_frame(&frames[0]);
    assert_eq!(user["type"], "user");
    assert_eq!(user["session_id"], "o");
    assert_eq!(user["priority"], "next");
    let content = user["message"]["content"].as_str().expect("content");
    assert!(
        content.starts_with("heartbeat digest d1 — fleet T1, 1 events"),
        "{content}"
    );
    assert!(
        content.contains(&format!("- idle({token} {WORKER})")),
        "{content}"
    );

    rig.daemon().wait_for("d1 sent", |doc| {
        subscription(doc, "o").is_some_and(|row| {
            row["last_digest"]["id"] == "d1" && row["last_digest"]["outcome"] == "sent"
        })
    });
    let ledger = wait_ledger(&rig, "digest 1 sent", |ledger| {
        ledger_kinds(ledger, "sent")
            .iter()
            .any(|line| line["digest"] == 1)
    });
    let kinds: Vec<(String, Value)> = ledger
        .iter()
        .filter(|line| line["digest"] == 1)
        .map(|line| {
            (
                line["kind"].as_str().unwrap_or_default().to_owned(),
                line["digest"].clone(),
            )
        })
        .collect();
    assert_eq!(
        kinds,
        vec![
            ("claimed".to_owned(), json!(1)),
            ("sent".to_owned(), json!(1))
        ]
    );
}

#[test]
fn busy_subscriber_gets_one_coalesced_digest() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    rig.member("w1", WORKER);
    rig.member("w2", SECOND);
    rig.hook("UserPromptSubmit", "o");
    turn(&rig, "w1");
    rig.hook("UserPromptSubmit", "w2");
    rig.hook("PermissionRequest", "w2");
    rig.daemon()
        .wait_for("two pending", |doc| pending(doc, "o") == Some(2));
    assert!(
        rig.inbox.frames().is_empty(),
        "a running subscriber is never interrupted"
    );

    rig.hook("Stop", "o");
    let frames = rig.inbox.wait_frames(1);
    let content = user_frame(&frames[0])["message"]["content"]
        .as_str()
        .expect("content")
        .to_owned();
    assert!(
        content.contains("idle(") && content.contains("waiting("),
        "{content}"
    );
}

#[test]
fn stale_items_are_dropped() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    rig.member("w1", WORKER);
    rig.hook("UserPromptSubmit", "o");
    turn(&rig, "w1");
    rig.hook("UserPromptSubmit", "w1");
    rig.daemon()
        .wait_for("one pending", |doc| pending(doc, "o") == Some(1));
    rig.hook("Stop", "o");
    let doc = rig.daemon().wait_for("stale dropped", |doc| {
        pending(doc, "o") == Some(0) && row(doc, "o").is_some_and(|row| row["state"] == "idle")
    });
    assert!(
        subscription(&doc, "o")
            .expect("row")
            .get("last_digest")
            .is_none()
    );
    assert!(rig.inbox.frames().is_empty());
}

#[test]
fn restart_never_resends_a_claimed_digest() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    rig.member("w1", WORKER);
    turn(&rig, "w1");
    rig.inbox.wait_frames(1);
    rig.daemon().wait_for("d1 sent", |doc| {
        subscription(doc, "o").is_some_and(|row| row["last_digest"]["outcome"] == "sent")
    });

    rig.restart();
    let replayed = rig.daemon().wait_for("replayed", |doc| {
        subscription(doc, "o").is_some_and(|row| {
            row["state"] == "active"
                && row["last_digest"]["id"] == "d1"
                && row["last_digest"]["outcome"] == "sent"
        })
    });
    assert_eq!(
        fleet_row(&replayed, "T1").expect("T1")["repo"],
        rig.repo.path.to_str().expect("utf-8")
    );
    turn(&rig, "o");
    rig.daemon().wait_for("o idle, nothing pending", |doc| {
        pending(doc, "o") == Some(0)
            && row(doc, "o").is_some_and(|row| row["state"] == "idle")
            && subscription(doc, "o").is_some_and(|row| row["last_digest"]["id"] == "d1")
    });
    assert_eq!(rig.inbox.frames().len(), 1, "d1 never re-sent");

    rig.daemon()
        .wait_for("w1 known", |doc| has_member(doc, "T1", "w1"));
    turn(&rig, "w1");
    let frames = rig.inbox.wait_frames(2);
    let content = user_frame(&frames[1])["message"]["content"]
        .as_str()
        .expect("content")
        .to_owned();
    assert!(content.starts_with("heartbeat digest d2 "), "{content}");
}

#[test]
fn claimed_without_outcome_stays_unconfirmed() {
    // The ledger names the rig's inbox socket, so it is written once the rig exists and replayed by a restart.
    let mut rig = Rig::new();
    let dir = rig.home.path().join("heartbeat");
    let socket = rig.inbox.path.to_str().expect("utf-8").to_owned();
    let repo_path = rig.repo.path.to_str().expect("utf-8").to_owned();
    let lines = [
        json!({"v": 1, "at": "2026-09-27T10:00:00Z", "kind": "subscribed", "session_id": "o", "fleet": "T1",
               "repo": repo_path, "target": "main", "socket_path": socket, "token": TOKEN}),
        json!({"v": 1, "at": "2026-09-27T10:00:01Z", "kind": "claimed", "digest": 5, "session_id": "o",
               "fleet": "T1", "items": 1}),
    ];
    let text: String = lines.iter().map(|line| line.to_string() + "\n").collect();
    std::fs::write(dir.join("actions.jsonl"), text).expect("ledger");
    rig.restart();

    rig.orchestrator();
    rig.hook("Stop", "o");
    let doc = rig.daemon().wait_for("d5 unconfirmed", |doc| {
        subscription(doc, "o").is_some_and(|row| {
            row["last_digest"]["id"] == "d5" && row["last_digest"]["outcome"] == "unconfirmed"
        })
    });
    assert_eq!(fleet_row(&doc, "T1").expect("T1")["repo"], repo_path);
    assert!(
        rig.inbox.frames().is_empty(),
        "a claimed digest is never sent"
    );

    rig.member("w1", WORKER);
    turn(&rig, "w1");
    let frames = rig.inbox.wait_frames(1);
    let content = user_frame(&frames[0])["message"]["content"]
        .as_str()
        .expect("content")
        .to_owned();
    assert!(content.starts_with("heartbeat digest d6 "), "{content}");
}

#[test]
fn membership_follows_the_join() {
    let mut rig = Rig::new();
    rig.session("w1", WORKER);
    rig.daemon().wait_for_state("w1", "idle");
    assert!(
        !has_member(&rig.daemon().document(), "T1", "w1"),
        "a fleet title alone is no membership"
    );
    assert_eq!(rig.join("w1", "T1"), "T1-MA");
    assert_eq!(rig.join("w1", "T2"), "T2-MA");
    let doc = rig.daemon().document();
    assert!(!has_member(&doc, "T1", "w1"), "left T1");
    let t1 = rig.status_json(&["--fleet", "T1"]);
    assert!(row(&t1, "w1").is_none(), "{t1:#}");
    assert_eq!(rig.join("w1", "T1"), "T1-MA", "the letter is kept");
}

/// A degraded peek round is repeated on its own: once peek serves the right home the sensor recovers without a subscriber.
#[test]
fn degraded_peek_recovers_by_retry() {
    let config = json!({ "peek_retry_ms": 1_000 });
    let rig = Rig::build(
        GitRepo::new(),
        Home::new(),
        Some(Path::new("/other")),
        config,
    );
    rig.daemon().wait_for("foreign peek", |doc| {
        sensor(doc, "peek").is_some_and(|peek| peek["state"] == "degraded")
    });

    rig.peek.set_home(rig.home.path().to_str().expect("utf-8"));
    rig.daemon().wait_for("peek recovered", |doc| {
        sensor(doc, "peek").is_some_and(|peek| peek["state"] == "ok")
    });
}

#[test]
fn unreachable_peek_is_degraded() {
    let home = Home::new();
    let daemon = Daemon::start(home.path());
    let doc = daemon.wait_for("peek sensor", |doc| sensor(doc, "peek").is_some());
    let peek = sensor(&doc, "peek").expect("peek");
    assert_eq!(peek["state"], "degraded");
    assert!(
        peek["detail"]
            .as_str()
            .is_some_and(|detail| detail.contains("unreachable")),
        "{peek}"
    );
    drop(daemon);

    let mut rig = Rig::with_peek(GitRepo::new(), Home::new(), Some(Path::new("/other")));
    let doc = rig.daemon().wait_for("foreign peek", |doc| {
        sensor(doc, "peek").is_some_and(|peek| {
            peek["detail"]
                .as_str()
                .is_some_and(|detail| detail.contains("serves /other"))
        })
    });
    assert_eq!(sensor(&doc, "peek").expect("peek")["state"], "degraded");

    rig.peek.set_home(rig.home.path().to_str().expect("utf-8"));
    rig.orchestrator();
    rig.subscribed();
    rig.daemon().wait_for("peek ok", |doc| {
        sensor(doc, "peek").is_some_and(|peek| peek["state"] == "ok")
    });
    assert!(rig.peek.calls().contains(&"session_events".to_owned()));
}

/// The subscriber's spawn tree, rebuilt from peek, fills its subagent list; none of its subagents is a member.
#[test]
fn spawn_tree_rebuilds_from_peek() {
    let mut rig = Rig::new();
    let fixture: Value = serde_json::from_str(PEEK_FIXTURE).expect("peek fixture");
    rig.peek.set_events("o", fixture);
    rig.orchestrator();
    rig.subscribed();
    let doc = rig.daemon().wait_for("spawn tree", |doc| {
        row(doc, "o").is_some_and(|row| {
            row["subagents"].as_array().is_some_and(|rows| {
                rows.iter()
                    .any(|sub| sub["agent_id"] == "abcf172b9e819cc00")
            })
        })
    });
    assert_eq!(
        members(&doc, "T1"),
        vec![("o".to_owned(), None, "subscriber".to_owned())],
        "peek-subagent-is-no-member"
    );
    let subagents = row(&doc, "o").expect("o")["subagents"].clone();
    let state = |id: &str| {
        subagents
            .as_array()
            .and_then(|rows| rows.iter().find(|row| row["agent_id"] == id))
            .map(|row| row["state"].clone())
    };
    assert_eq!(
        state("abcf172b9e819cc00"),
        Some(json!("stopped")),
        "background completion"
    );
    assert_eq!(
        state("ac4decf3b5ce93432"),
        Some(json!("running")),
        "launch ack only"
    );
    assert_eq!(
        state("a97dc3464fb16fa00"),
        Some(json!("stopped")),
        "foreground result"
    );
}

/// `heartbeat worker` as the fleet's subscriber registers `claude/live` as a lettered member, delivers it into an
/// integrated fleet, survives a restart through the ledger, and leaves by the operator's `remove`.
#[test]
#[allow(clippy::too_many_lines)]
fn worker_registration() {
    let mut rig = Rig::new();
    let fixture: Value = serde_json::from_str(PEEK_FIXTURE).expect("peek fixture");
    rig.peek.set_events("o", fixture);
    rig.orchestrator();
    let merge = rig.subscribed();
    let worker = |session: &str, state: &str, format: &str| {
        rig.daemon().cli_as(
            session,
            TOKEN,
            &[
                "worker",
                "--branch",
                "claude/live",
                "--state",
                state,
                "--format",
                format,
            ],
        )
    };

    let stranger = worker("x", "running", "json");
    assert_eq!(
        stranger.status.code(),
        Some(1),
        "non-subscriber-refused-409"
    );
    assert!(
        stderr(&stranger).contains("not the fleet's active subscriber"),
        "non-subscriber-refused-409: {}",
        stderr(&stranger)
    );
    let no_env = rig.daemon().cli(
        &["worker", "--branch", "claude/live", "--state", "running"],
        &[],
    );
    assert_eq!(no_env.status.code(), Some(1), "missing-env-usage-error");
    assert!(
        stderr(&no_env).contains("worker needs CLAUDE_CODE_SESSION_ID"),
        "missing-env-usage-error: {}",
        stderr(&no_env)
    );

    let registered = worker("o", "running", "json");
    assert!(registered.status.success(), "{}", stderr(&registered));
    let answer: Value = serde_json::from_slice(&registered.stdout).expect("stdout is the row");
    assert_eq!(answer["via"], "worker", "registered-row-via-worker");
    assert_eq!(answer["session_id"], "o", "registered-row-via-worker");
    assert_eq!(answer["branch"], "claude/live", "registered-row-via-worker");
    assert_eq!(answer["token"], "T1-MA", "registered-row-via-worker");
    assert_eq!(answer["delivered"], false, "registered-row-via-worker");
    assert!(answer.get("agent_id").is_none(), "{answer}");
    let again = worker("o", "running", "text");
    assert!(again.status.success(), "{}", stderr(&again));
    assert_eq!(
        String::from_utf8_lossy(&again.stdout),
        "T1-MA\n",
        "a second registration keeps the letter"
    );
    let doc = rig.daemon().wait_for("o's spawn tree", |doc| {
        row(doc, "o").is_some_and(|row| {
            row["subagents"]
                .as_array()
                .is_some_and(|rows| !rows.is_empty())
        })
    });
    assert_eq!(
        members(&doc, "T1"),
        vec![
            ("o".to_owned(), None, "subscriber".to_owned()),
            (
                "o".to_owned(),
                Some("claude/live".to_owned()),
                "worker".to_owned()
            ),
        ],
        "peek-subagent-is-no-member"
    );

    let orch = rig.cwds["o"].clone();
    rig.repo.git(
        &orch,
        &[
            "merge",
            "--no-ff",
            "-q",
            "-m",
            "Merge claude/live",
            "claude/live",
        ],
    );
    rig.daemon()
        .wait_for("merged but not delivered: still open", |doc| {
            branch(doc, "claude/live").is_some_and(|row| {
                (row["state"] == "merged" || row["state"] == "removable")
                    && row["target"] == merge.as_str()
            }) && fleet_row(doc, "T1").is_some_and(|fleet| fleet["target_state"] == "open")
        });
    let delivered = worker("o", "delivered", "json");
    assert!(delivered.status.success(), "{}", stderr(&delivered));
    let answer: Value = serde_json::from_slice(&delivered.stdout).expect("stdout is the row");
    assert_eq!(answer["delivered"], true, "delivered-then-integrated");
    rig.daemon().wait_for("delivered-then-integrated", |doc| {
        fleet_row(doc, "T1").is_some_and(|fleet| fleet["target_state"] == "integrated")
    });

    wait_ledger(&rig, "the worker lines", |ledger| {
        ledger_kinds(ledger, "worker")
            .iter()
            .any(|line| line["delivered"] == true && line["branch"] == "claude/live")
            && ledger_kinds(ledger, "member_assigned")
                .iter()
                .any(|line| line["worker"] == "claude/live" && line.get("agent_id").is_none())
    });
    rig.restart();
    let doc = rig.daemon().wait_for("restart-replays-worker", |doc| {
        worker_row(doc, "T1", "claude/live").is_some()
    });
    let replayed = worker_row(&doc, "T1", "claude/live").expect("worker row");
    assert_eq!(replayed["token"], "T1-MA", "restart-replays-worker");
    assert_eq!(replayed["delivered"], true, "restart-replays-worker");
    assert_eq!(replayed["session_id"], "o", "restart-replays-worker");
    // The restart's replay writes no letter line: wait for a later line, then count.
    let noted = rig.daemon().cli_as(
        "o",
        TOKEN,
        &["record", "--kind", "note", "--subject", "after restart"],
    );
    assert!(noted.status.success(), "{}", stderr(&noted));
    let ledger = wait_ledger(&rig, "the note after the restart", |ledger| {
        ledger_kinds(ledger, "journal")
            .iter()
            .any(|line| line["entry"]["subject"] == "after restart")
    });
    let letters = ledger_kinds(&ledger, "member_assigned")
        .iter()
        .filter(|line| line["worker"] == "claude/live")
        .count();
    assert_eq!(
        letters, 1,
        "restart-replays-worker: one letter line: {ledger:?}"
    );

    let removed = rig
        .daemon()
        .cli(&["remove", "--fleet", "T1", "--member", "A"], &[]);
    assert!(removed.status.success(), "{}", stderr(&removed));
    let doc = rig.daemon().document();
    assert!(
        worker_row(&doc, "T1", "claude/live").is_none(),
        "remove-drops-worker: {doc:#}"
    );
    assert!(has_member(&doc, "T1", "o"), "the subscriber stays: {doc:#}");
}

#[test]
fn refused_socket_ends_the_subscription() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    rig.member("w1", WORKER);
    rig.inbox.close();
    rig.hook("Stop", "o");
    turn(&rig, "w1");
    let doc = rig.daemon().wait_for("ended", |doc| {
        subscription(doc, "o").is_some_and(|row| row["state"] == "ended")
    });
    let detail = subscription(&doc, "o").expect("row")["detail"]
        .as_str()
        .unwrap_or_default()
        .to_owned();
    assert!(detail.contains("socket refused"), "{detail}");
    let ledger = wait_ledger(&rig, "the unsubscribed line", |ledger| {
        !ledger_kinds(ledger, "unsubscribed").is_empty()
    });
    let kinds: Vec<String> = ledger
        .iter()
        .filter_map(|line| line["kind"].as_str().map(str::to_owned))
        .collect();
    let refused = kinds.iter().position(|kind| kind == "refused");
    let unsubscribed = kinds.iter().position(|kind| kind == "unsubscribed");
    assert!(
        refused.is_some() && refused < unsubscribed,
        "refused, then unsubscribed: {kinds:?}"
    );
}

#[test]
fn unsubscribe_ends_the_subscription() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    let unsubscribe = |rig: &Rig, env: &[(&str, &str)]| {
        rig.daemon().cli(&["unsubscribe", "--format", "json"], env)
    };
    let own = [
        ("CLAUDE_CODE_SESSION_ID", "o"),
        ("CLAUDE_CODE_MESSAGING_TOKEN", TOKEN),
    ];

    let no_env = unsubscribe(&rig, &[("CLAUDE_CODE_SESSION_ID", "o")]);
    assert_eq!(no_env.status.code(), Some(1), "{}", stderr(&no_env));
    assert!(stderr(&no_env).contains("CLAUDE_CODE_MESSAGING_TOKEN"));

    let wrong = unsubscribe(
        &rig,
        &[
            ("CLAUDE_CODE_SESSION_ID", "o"),
            ("CLAUDE_CODE_MESSAGING_TOKEN", "not-the-token"),
        ],
    );
    assert!(!wrong.status.success(), "wrong token refused");
    assert!(
        stderr(&wrong).contains("token does not match"),
        "{}",
        stderr(&wrong)
    );

    let out = unsubscribe(&rig, &own);
    assert!(out.status.success(), "{}", stderr(&out));
    let row: Value = serde_json::from_slice(&out.stdout).expect("stdout is the row");
    assert_eq!(row["state"], "ended");
    assert_eq!(row["detail"], "unsubscribed by the session");
    let ledger = wait_ledger(&rig, "the unsubscribed line", |ledger| {
        !ledger_kinds(ledger, "unsubscribed").is_empty()
    });
    let last = ledger.last().cloned().expect("ledger line");
    assert_eq!(last["kind"], "unsubscribed");
    assert_eq!(last["session_id"], "o");

    let again = unsubscribe(&rig, &own);
    assert!(again.status.success(), "idempotent: {}", stderr(&again));

    rig.hook("Stop", "o");
    rig.member("w1", WORKER);
    turn(&rig, "w1");
    rig.daemon().wait_for_state("w1", "idle");
    assert!(rig.inbox.frames().is_empty(), "no digest after unsubscribe");

    rig.restart();
    let doc = rig.daemon().document();
    assert!(subscription(&doc, "o").is_none(), "not replayed");
}

#[test]
fn subscribe_refusals() {
    let mut rig = Rig::new();
    rig.orchestrator();
    let daemon = rig.daemon();

    let no_env = daemon.cli(
        &["subscribe", "--fleet", "T1", "--target", "main"],
        &[("CLAUDE_CODE_SESSION_ID", "o")],
    );
    assert_eq!(no_env.status.code(), Some(1), "{}", stderr(&no_env));
    assert!(stderr(&no_env).contains("CLAUDE_CODE_MESSAGING_SOCKET"));

    let socket = rig.inbox.path.to_str().expect("utf-8").to_owned();
    for (label, fleet, target) in [
        ("fleet-charset", "T 1", "main"),
        ("target-dots", "T1", "..main"),
        ("target-dash", "T1", "-x"),
    ] {
        let out = daemon.cli(
            &["subscribe", "--fleet", fleet, &format!("--target={target}")],
            &[
                ("CLAUDE_CODE_SESSION_ID", "o"),
                ("CLAUDE_CODE_MESSAGING_SOCKET", &socket),
                ("CLAUDE_CODE_MESSAGING_TOKEN", TOKEN),
            ],
        );
        assert_eq!(out.status.code(), Some(2), "{label}: {}", stderr(&out));
    }

    let ghost = daemon.subscribe_via("ghost", "T1", "main", &socket);
    assert_eq!(ghost.status.code(), Some(1));
    assert!(
        stderr(&ghost).contains("unknown session"),
        "{}",
        stderr(&ghost)
    );

    let file = rig.home.path().join("not-a-socket");
    std::fs::write(&file, "").expect("regular file");
    let regular = daemon.subscribe_via("o", "T1", "main", file.to_str().expect("utf-8"));
    assert_eq!(regular.status.code(), Some(1));
    assert!(
        stderr(&regular).contains("is not a socket"),
        "{}",
        stderr(&regular)
    );

    rig.session_in(
        "o2",
        ORCHESTRATOR,
        Path::new("/tmp/heartbeat-fixture"),
        None,
    );
    rig.daemon().wait_for_state("o2", "idle");
    let outside = rig.daemon().subscribe_via("o2", "T1", "main", &socket);
    assert_eq!(outside.status.code(), Some(1));
    assert!(
        stderr(&outside).contains("is in no registered repo"),
        "{}",
        stderr(&outside)
    );

    rig.subscribed();
    rig.session("o3", ORCHESTRATOR);
    rig.daemon().wait_for_state("o3", "idle");
    let bound = rig.daemon().subscribe_via("o3", "T1", "develop", &socket);
    assert_eq!(bound.status.code(), Some(1));
    let want = format!(
        "fleet T1 is bound to {} main",
        rig.repo.path.to_str().expect("utf-8")
    );
    assert!(stderr(&bound).contains(&want), "{}", stderr(&bound));
}

#[test]
fn cli_fleet_filter_selects_members() {
    let mut rig = Rig::new();
    rig.member("w1", WORKER);
    rig.session("w2", "CC,FImpl - Other - T2[WIP]");
    rig.join("w2", "T2");
    let doc = rig.status_json(&["--fleet", "T1"]);
    let sessions: Vec<&Value> = doc["sessions"]
        .as_array()
        .expect("sessions")
        .iter()
        .map(|row| &row["session_id"])
        .collect();
    assert_eq!(sessions, [&json!("w1")]);
    let fleets: Vec<&Value> = doc["fleets"]
        .as_array()
        .expect("fleets")
        .iter()
        .map(|row| &row["id"])
        .collect();
    assert_eq!(fleets, [&json!("T1")]);
    assert_eq!(
        members(&doc, "T1"),
        vec![("w1".to_owned(), None, "joined".to_owned())]
    );
    let bad = rig.daemon().cli(&["status", "--fleet", "T 1"], &[]);
    assert_eq!(bad.status.code(), Some(2));
}

#[test]
fn fleet_scope_limits_git_rows() {
    let repo = GitRepo::new();
    let work = repo.worktree("w", "main");
    repo.commit(&work, "w.txt", "w\n", "w work");
    let mut rig = Rig::with(repo, Home::new());
    rig.session_in("w1", WORKER, &work, None);
    rig.daemon()
        .wait_for("claude/w row", |doc| branch(doc, "claude/w").is_some());
    rig.join("w1", "T1");

    let scoped = rig.status_json(&["--fleet", "T1"]);
    for key in ["repos", "branches", "work_items", "collisions", "trains"] {
        assert_eq!(scoped[key], json!([]), "unbound {key}");
    }
    assert!(
        branch(&rig.status_json(&[]), "claude/w").is_some(),
        "unfiltered lists it"
    );

    rig.orchestrator();
    rig.subscribed();
    let doc = rig.status_json(&["--fleet", "T1"]);
    assert!(branch(&doc, "claude/w").is_some(), "{doc:#}");
    assert_eq!(doc["trains"][0]["base"], "main");
    let w1 = doc["members"]
        .as_array()
        .and_then(|rows| rows.iter().find(|row| row["session_id"] == "w1"))
        .expect("w1 member");
    assert_eq!(w1["branch"], "claude/w");
}

#[test]
fn foreign_join_is_listed() {
    let mut rig = Rig::new();
    rig.session_in("w2", WORKER, Path::new("/tmp/heartbeat-fixture"), None);
    rig.session_in("w3", WORKER, Path::new("/tmp/heartbeat-fixture"), None);
    rig.join("w2", "T1");
    rig.orchestrator();
    rig.subscribed();
    let doc = rig.daemon().wait_for("w2 foreign", |doc| {
        fleet_row(doc, "T1").is_some_and(|fleet| fleet["foreign"] == json!(["w2"]))
    });
    assert!(!has_member(&doc, "T1", "w2"));

    rig.daemon().wait_for_state("w3", "idle");
    let refused = rig.daemon().cli_as("w3", TOKEN, &["join", "--fleet", "T1"]);
    assert_eq!(refused.status.code(), Some(1), "{}", stderr(&refused));
    let want = format!(
        "fleet T1 is bound to {}",
        rig.repo.path.to_str().expect("utf-8")
    );
    assert!(stderr(&refused).contains(&want), "{}", stderr(&refused));
}

/// The target and the orchestrator's merge branch are both train bases; members are judged against the merge branch.
#[test]
fn subscribe_target_is_a_train_base() {
    let repo = GitRepo::new();
    repo.git(&repo.path, &["branch", "develop", "main"]);
    let work = repo.worktree("w", "main");
    repo.commit(&work, "w.txt", "w\n", "w work");
    let human = repo.worktree("h", "main");
    repo.commit(&human, "h.txt", "h\n", "h work");
    let mut rig = Rig::with(repo, Home::new());
    rig.session_in("w1", WORKER, &work, None);
    rig.daemon()
        .wait_for("claude/w row", |doc| branch(doc, "claude/w").is_some());
    rig.join("w1", "T1");
    rig.orchestrator();
    let out = rig.subscribe("develop");
    assert!(out.status.success(), "{}", stderr(&out));
    let merge = rig.switch_to_merge_branch();

    let doc = rig
        .daemon()
        .wait_for("develop and merge trains, pin", |doc| {
            let bases: Vec<&Value> = doc["trains"]
                .as_array()
                .map(|trains| trains.iter().map(|train| &train["base"]).collect())
                .unwrap_or_default();
            bases == [&json!("main"), &json!(merge), &json!("develop")]
                && branch(doc, "claude/w").is_some_and(|row| row["target"] == merge.as_str())
        });
    assert_eq!(
        fleet_row(&doc, "T1").expect("T1")["merge_branch"],
        merge.as_str()
    );
    assert!(
        branch(&doc, "claude/h")
            .expect("claude/h")
            .get("target")
            .is_none(),
        "a main-based non-member branch has no landing target, develop or not: {doc:#}"
    );
    assert_eq!(fleet_row(&doc, "T1").expect("T1")["target_state"], "open");
}

/// A plan merged into the fleet's merge branch is merged work: no work item on the merge branch, none on a branch
/// cut from it; a plan only the branch adds still is one.
#[test]
fn plan_on_train_base() {
    let mut rig = Rig::new();
    rig.orchestrator();
    let merge = rig.subscribed();
    let orch = rig.cwds["o"].clone();
    let merged = "plans/2026-09-28-merged-ddddddd/2026-09-28-merged-wip-ddddddd/design/raw.md";
    rig.repo.write(&orch, merged, "# Merged\n");
    rig.repo.git(&orch, &["add", "plans"]);
    rig.repo.git(&orch, &["commit", "-q", "-m", "merged plan"]);
    let cut = rig.repo.worktree("cut", &merge);
    rig.repo.write(
        &cut,
        "plans/2026-09-28-own-eeeeeee/2026-09-28-own-wip-eeeeeee/design/raw.md",
        "# Own\n",
    );
    rig.repo.commit(&cut, "cut.txt", "cut\n", "cut work");
    let doc = rig.daemon().wait_for("the cut branch's own plan", |doc| {
        branch(doc, "claude/cut").is_some_and(|row| {
            row["work_items"] == json!(["2026-09-28-own-eeeeeee/2026-09-28-own-wip-eeeeeee"])
        })
    });
    assert_eq!(
        branch(&doc, &merge).expect("merge row")["work_items"],
        json!([]),
        "{doc:#}"
    );
}

/// A member merged into the orchestrator's `claude-orch/T1-<run uuid>` reads merged and integrates the fleet; the
/// target is synced by one ancestry-checked fast-forward (`git branch -f <target> HEAD`), which git refuses while the
/// target is checked out elsewhere.
#[test]
fn orchestrator_merge_branch() {
    let mut rig = Rig::new();
    rig.orchestrator();
    let merge = rig.subscribed();
    rig.member("w1", WORKER);
    turn(&rig, "w1");
    let orch = rig.cwds["o"].clone();
    rig.daemon()
        .wait_for("members judged against the merge branch", |doc| {
            fleet_row(doc, "T1").is_some_and(|fleet| fleet["merge_branch"] == merge.as_str())
                && branch(doc, "claude/live").is_some_and(|row| row["target"] == merge.as_str())
        });

    rig.repo.git(
        &orch,
        &[
            "merge",
            "--no-ff",
            "-q",
            "-m",
            "Merge claude/live",
            "claude/live",
        ],
    );
    rig.daemon()
        .wait_for("merged but not delivered: still open", |doc| {
            branch(doc, "claude/live")
                .is_some_and(|row| row["state"] == "merged" || row["state"] == "removable")
                && fleet_row(doc, "T1").is_some_and(|fleet| fleet["target_state"] == "open")
        });
    let done = rig.daemon().cli_as("w1", TOKEN, &["done"]);
    assert!(done.status.success(), "{}", stderr(&done));
    let doc = rig.daemon().wait_for("integrated, not synced", |doc| {
        fleet_row(doc, "T1")
            .is_some_and(|fleet| fleet["target_state"] == "integrated" && fleet["synced"] == false)
    });
    assert!(
        branch(&doc, "claude/live")
            .is_some_and(|row| row["state"] == "merged" || row["state"] == "removable"),
        "{doc:#}"
    );

    let git_in_orch = |args: &[&str]| {
        Command::new("git")
            .current_dir(&orch)
            .args(args)
            .output()
            .expect("git")
    };
    let ancestor = git_in_orch(&["merge-base", "--is-ancestor", "main", "HEAD"]);
    assert!(ancestor.status.success(), "unmoved target: a fast-forward");
    let blocked = git_in_orch(&["branch", "-f", "main", "HEAD"]);
    assert!(
        !blocked.status.success(),
        "main is checked out in the main checkout: {}",
        stderr(&blocked)
    );

    rig.repo.git(&rig.repo.path, &["switch", "-q", "--detach"]);
    let moved = git_in_orch(&["branch", "-f", "main", "HEAD"]);
    assert!(moved.status.success(), "{}", stderr(&moved));
    rig.daemon().wait_for("synced", |doc| {
        fleet_row(doc, "T1").is_some_and(|fleet| fleet["synced"] == true)
    });
}

/// The session's own member row (no worker) in `fleet`.
fn member_row<'a>(doc: &'a Value, fleet: &str, session: &str) -> Option<&'a Value> {
    doc["members"]
        .as_array()?
        .iter()
        .find(|row| row["fleet"] == fleet && row["session_id"] == session && row["via"] != "worker")
}

fn is_delivered(doc: &Value, session: &str) -> bool {
    member_row(doc, "T1", session).is_some_and(|row| row["delivered"] == true)
}

#[test]
#[allow(clippy::too_many_lines)]
fn join_done_journal() {
    let mut rig = Rig::new();
    let first = rig.daemon().cli_as("w1", TOKEN, &["join", "--fleet", "T1"]);
    assert_eq!(first.status.code(), Some(1), "unknown session refused");
    assert!(
        stderr(&first).contains("unknown session"),
        "{}",
        stderr(&first)
    );

    rig.session("w1", WORKER);
    rig.session("w2", SECOND);
    rig.daemon().wait_for_state("w2", "idle");
    let joined = rig.daemon().cli_as("w1", TOKEN, &["join", "--fleet", "T1"]);
    assert!(joined.status.success(), "{}", stderr(&joined));
    assert_eq!(
        String::from_utf8_lossy(&joined.stdout),
        "T1-MA\n",
        "exact stdout"
    );
    assert_eq!(rig.join("w2", "T1"), "T1-MB");
    rig.orchestrator();
    let merge = rig.subscribed();
    rig.hook("Stop", "o");
    let doc = rig
        .daemon()
        .wait_for("o is the operator, on its merge branch", |doc| {
            member_row(doc, "T1", "o")
                .is_some_and(|row| row["token"] == "T1-O" && row["member"].is_null())
                && branch(doc, &merge).is_some()
        });
    assert_eq!(member_row(&doc, "T1", "w1").expect("w1")["via"], "joined");

    let wrong = rig.daemon().cli_as("w1", "not-the-token", &["done"]);
    assert_eq!(wrong.status.code(), Some(1));
    assert!(
        stderr(&wrong).contains("token does not match the join"),
        "{}",
        stderr(&wrong)
    );
    let not_joined = rig.daemon().cli_as("o", TOKEN, &["done"]);
    assert!(not_joined.status.success(), "{}", stderr(&not_joined));
    assert_eq!(
        String::from_utf8_lossy(&not_joined.stdout),
        "no-target main\n",
        "the subscriber of a fleet targeting main lands nowhere"
    );

    let done = rig.daemon().cli_as("w1", TOKEN, &["done"]);
    assert!(done.status.success(), "{}", stderr(&done));
    assert_eq!(String::from_utf8_lossy(&done.stdout), "delivered T1-MA\n");
    let wrong_again = rig.daemon().cli_as("w1", "not-the-token", &["done"]);
    assert!(
        stderr(&wrong_again).contains("token does not match the join"),
        "delivered is no pass: {}",
        stderr(&wrong_again)
    );
    assert!(is_delivered(&rig.daemon().document(), "w1"));
    let frames = rig.inbox.wait_frames(1);
    let content = user_frame(&frames[0])["message"]["content"]
        .as_str()
        .expect("content")
        .to_owned();
    assert!(
        content.contains(&format!("- delivered(T1-MA {WORKER})")),
        "{content}"
    );

    rig.hook("UserPromptSubmit", "w1");
    rig.daemon()
        .wait_for("w1 resumed", |doc| !is_delivered(doc, "w1"));

    let recorded = rig.daemon().cli_as(
        "o",
        TOKEN,
        &[
            "record",
            "--kind",
            "merged",
            "--subject",
            "merged claude/live",
            "--member",
            "A",
            "--detail",
            "delivered and merge-ready",
        ],
    );
    assert!(recorded.status.success(), "{}", stderr(&recorded));
    assert_eq!(
        String::from_utf8_lossy(&recorded.stdout),
        "recorded merged in fleet T1\n"
    );
    let by_member = rig.daemon().cli_as(
        "w1",
        TOKEN,
        &["record", "--kind", "note", "--subject", "mine"],
    );
    assert!(
        stderr(&by_member).contains("not the fleet's active subscriber"),
        "{}",
        stderr(&by_member)
    );
    let bad_kind = rig.daemon().cli_as(
        "o",
        TOKEN,
        &["record", "--kind", "shipped", "--subject", "x"],
    );
    assert!(
        stderr(&bad_kind).contains("kind must be"),
        "{}",
        stderr(&bad_kind)
    );

    let done2 = rig.daemon().cli_as("w2", TOKEN, &["done"]);
    assert!(done2.status.success(), "{}", stderr(&done2));
    wait_ledger(&rig, "w2 delivered", |ledger| {
        ledger
            .iter()
            .any(|line| line["kind"] == "delivered" && line["session_id"] == "w2")
    });

    rig.restart();
    let doc = rig.daemon().wait_for("members replayed", |doc| {
        member_row(doc, "T1", "w1").is_some() && member_row(doc, "T1", "w2").is_some()
    });
    let tokens: Vec<(&str, bool)> = ["w1", "w2", "o"]
        .iter()
        .map(|session| {
            let row = member_row(&doc, "T1", session).expect("member");
            (
                row["token"].as_str().unwrap_or_default(),
                row["delivered"] == true,
            )
        })
        .collect();
    assert_eq!(
        tokens,
        [("T1-MA", false), ("T1-MB", true), ("T1-O", false)],
        "letters and delivery restored"
    );
    let journal = &fleet_row(&doc, "T1").expect("T1")["journal"];
    assert_eq!(journal.as_array().map(Vec::len), Some(1), "{journal}");
    assert_eq!(journal[0]["subject"], "merged claude/live");
    assert_eq!(journal[0]["members"], json!(["A"]));
    assert_eq!(journal[0]["by"], "o");
    assert!(!doc.to_string().contains(TOKEN), "token leaked into status");
}

/// Every archive file under the rig's heartbeat dir, parsed.
fn archives(rig: &Rig) -> Vec<Value> {
    let dir = rig.home.path().join("heartbeat/archive");
    let mut files: Vec<PathBuf> = std::fs::read_dir(&dir)
        .map(|entries| entries.flatten().map(|entry| entry.path()).collect())
        .unwrap_or_default();
    files.sort();
    files
        .iter()
        .filter(|path| path.extension().is_some_and(|ext| ext == "json"))
        .map(|path| serde_json::from_slice(&std::fs::read(path).expect("archive")).expect("json"))
        .collect()
}

/// Waits until the rig's archive dir holds `count` files, then returns them.
fn wait_archives(rig: &Rig, count: usize) -> Vec<Value> {
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
    loop {
        let found = archives(rig);
        if found.len() >= count {
            return found;
        }
        assert!(
            std::time::Instant::now() < deadline,
            "{} of {count} archives",
            found.len()
        );
        std::thread::sleep(std::time::Duration::from_millis(50));
    }
}

/// Waits until the ledger satisfies `check` and returns it. Heartbeat answers every request before its records are
/// appended (the engine hands them to the ledger thread), so the answer, the status document and an archive file
/// can all be there while the line is not: a test that asserts a ledger line, or restarts and expects it to
/// survive, waits for it here — never reads the ledger right after an answer.
fn wait_ledger(rig: &Rig, what: &str, check: impl Fn(&[Value]) -> bool) -> Vec<Value> {
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(10);
    loop {
        let ledger = rig.ledger();
        if check(&ledger) {
            return ledger;
        }
        assert!(
            std::time::Instant::now() < deadline,
            "ledger: {what}: {ledger:?}"
        );
        std::thread::sleep(std::time::Duration::from_millis(50));
    }
}

fn ledger_kinds<'a>(ledger: &'a [Value], kind: &str) -> Vec<&'a Value> {
    ledger.iter().filter(|line| line["kind"] == kind).collect()
}

#[test]
#[allow(clippy::too_many_lines)]
fn fleet_lifecycle_archive() {
    let mut rig = Rig::new();
    assert_eq!(rig.member("w1", WORKER), "T1-MA");
    rig.orchestrator();
    rig.subscribed();
    let first_run = rig.daemon().wait_for("T1 has a run", |doc| {
        fleet_row(doc, "T1").is_some_and(|fleet| fleet["instance"].is_string())
    });
    let first_run = fleet_row(&first_run, "T1").expect("T1")["instance"].clone();
    let noted = rig.daemon().cli_as(
        "o",
        TOKEN,
        &["record", "--kind", "note", "--subject", "start"],
    );
    assert!(noted.status.success(), "{}", stderr(&noted));

    let wrong = rig.daemon().cli_as("w1", "not-the-token", &["leave"]);
    assert!(
        stderr(&wrong).contains("token does not match the join"),
        "{}",
        stderr(&wrong)
    );
    let left = rig.daemon().cli_as("w1", TOKEN, &["leave"]);
    assert!(left.status.success(), "{}", stderr(&left));
    assert_eq!(String::from_utf8_lossy(&left.stdout), "left T1-MA\n");
    let again = rig.daemon().cli_as("w1", TOKEN, &["leave"]);
    assert!(
        stderr(&again).contains("not a joined session"),
        "{}",
        stderr(&again)
    );
    let doc = rig.daemon().document();
    assert!(!has_member(&doc, "T1", "w1"), "w1 left");
    assert!(
        fleet_row(&doc, "T1").is_some(),
        "the orchestrator keeps T1 live"
    );

    let own = [
        ("CLAUDE_CODE_SESSION_ID", "o"),
        ("CLAUDE_CODE_MESSAGING_TOKEN", TOKEN),
    ];
    let out = rig.daemon().cli(&["unsubscribe", "--format", "json"], &own);
    assert!(out.status.success(), "{}", stderr(&out));
    let row: Value = serde_json::from_slice(&out.stdout).expect("stdout is the row");
    assert_eq!(row["state"], "ended", "answered after the dissolve");
    rig.daemon()
        .wait_for("T1 dissolved", |doc| fleet_row(doc, "T1").is_none());

    let found = wait_archives(&rig, 1);
    let archive = &found[0];
    assert_eq!(archive["instance"], first_run);
    assert_eq!(archive["fleet"], "T1");
    let tokens: Vec<&Value> = archive["members"]
        .as_array()
        .expect("members")
        .iter()
        .map(|member| &member["token"])
        .collect();
    assert_eq!(tokens, [&json!("T1-O"), &json!("T1-MA")]);
    assert_eq!(archive["journal"][0]["subject"], "start");
    assert_eq!(archive["subscriptions"][0]["state"], "ended");
    assert_eq!(archive["target"], "main");
    let dir = rig.home.path().join("heartbeat/archive");
    assert_eq!(mode(&dir), 0o700);
    let file = dir.join(format!("{}.json", first_run.as_str().expect("id")));
    assert_eq!(mode(&file), 0o600);
    let ledger = wait_ledger(&rig, "the dissolved line", |ledger| {
        !ledger_kinds(ledger, "dissolved").is_empty()
    });
    assert_eq!(ledger_kinds(&ledger, "left")[0]["reason"], "left");
    assert_eq!(
        ledger_kinds(&ledger, "fleet_started")[0]["instance"],
        first_run
    );
    assert_eq!(ledger_kinds(&ledger, "dissolved")[0]["instance"], first_run);

    assert_eq!(rig.join("w1", "T1"), "T1-MA", "a new run starts at A");
    let doc = rig.daemon().document();
    let second_run = fleet_row(&doc, "T1").expect("T1")["instance"].clone();
    assert_ne!(second_run, first_run);
    wait_ledger(&rig, "the new run's join", |ledger| {
        ledger_kinds(ledger, "fleet_started")
            .iter()
            .any(|line| line["instance"] == second_run)
            && ledger_kinds(ledger, "joined").len() == 2
    });

    rig.restart();
    let doc = rig
        .daemon()
        .wait_for("w1 replayed", |doc| member_row(doc, "T1", "w1").is_some());
    assert_eq!(fleet_row(&doc, "T1").expect("T1")["instance"], second_run);
    assert_eq!(
        member_row(&doc, "T1", "w1").expect("w1")["token"],
        "T1-MA",
        "the letter of the new run"
    );
    assert_eq!(archives(&rig).len(), 1, "the old run stays dissolved");
}

#[test]
fn ended_member_leaves() {
    let mut rig = Rig::new();
    assert_eq!(rig.member("w1", WORKER), "T1-MA");
    rig.hook("SessionEnd", "w1");
    rig.daemon()
        .wait_for("T1 dissolved", |doc| fleet_row(doc, "T1").is_none());
    let found = wait_archives(&rig, 1);
    assert_eq!(found[0]["members"][0]["token"], "T1-MA");
    let ledger = wait_ledger(&rig, "the left line", |ledger| {
        !ledger_kinds(ledger, "left").is_empty()
    });
    assert_eq!(
        ledger_kinds(&ledger, "left")[0]["reason"],
        "the session ended"
    );
}

#[test]
fn default_branch_target_is_one_train() {
    let mut rig = Rig::new();
    rig.orchestrator();
    rig.subscribed();
    let doc = rig.daemon().wait_for("main requested", |doc| {
        fleet_row(doc, "T1").is_some_and(|fleet| fleet["target_state"] == "open")
            && doc["heartbeat"]["scans_completed"].as_u64() >= Some(2)
    });
    let bases: Vec<&Value> = doc["trains"]
        .as_array()
        .map(|trains| trains.iter().map(|train| &train["base"]).collect())
        .unwrap_or_default();
    assert_eq!(bases, [&json!("main")], "{}", doc["trains"]);
}

/// A session outside any fleet in its own `claude/<session>` worktree created from `base`, messaging socket `inbox`,
/// known to the daemon, its branch row scanned — with `base` as landing target unless `base` is `main`.
fn lander(rig: &mut Rig, session: &str, base: &str, inbox: &FakeInbox) {
    let cwd = rig.repo.worktree(session, base);
    let title = format!("CC,FImpl - {session} - [WIP]");
    let socket = inbox.path.clone();
    rig.session_in(session, &title, &cwd, Some(&socket));
    let name = format!("claude/{session}");
    let want = (base != "main").then_some(base);
    rig.daemon()
        .wait_for(&format!("{session} known, {name} scanned"), |doc| {
            row(doc, session).is_some()
                && branch(doc, &name).is_some_and(|row| row["target"].as_str() == want)
        });
}

/// `heartbeat <args>` as `session` with its own token `tok-<session>`.
fn land_as(rig: &Rig, session: &str, args: &[&str]) -> Output {
    rig.daemon()
        .cli_as(session, &format!("tok-{session}"), args)
}

fn stdout(out: &Output) -> String {
    String::from_utf8_lossy(&out.stdout).into_owned()
}

/// The fixture repo's landing row for `target`; `null` when the status lists none.
fn landing<'a>(doc: &'a Value, target: &str) -> &'a Value {
    doc["landings"]
        .as_array()
        .and_then(|rows| rows.iter().find(|row| row["branch"] == target))
        .unwrap_or(&Value::Null)
}

/// `(holder, queue)` session ids of the fixture repo's landing for `target`.
fn turns(doc: &Value, target: &str) -> (Option<String>, Vec<String>) {
    let landing = landing(doc, target);
    let holder = landing["holder"]["session_id"].as_str().map(str::to_owned);
    let queue = landing["queue"]
        .as_array()
        .map(|rows| {
            rows.iter()
                .filter_map(|row| row["session_id"].as_str().map(str::to_owned))
                .collect()
        })
        .unwrap_or_default();
    (holder, queue)
}

fn holds(holder: &str, queue: &[&str]) -> (Option<String>, Vec<String>) {
    (
        Some(holder.to_owned()),
        queue.iter().map(|id| (*id).to_owned()).collect(),
    )
}

/// Two sessions outside a fleet based on `feature/x`: the first done holds its turn, the second waits; the holder's
/// landed end grants the waiter with one message on its own socket.
#[test]
fn landing_turn_serialises() {
    let mut rig = Rig::new();
    let second_inbox = FakeInbox::start();
    let first_inbox = FakeInbox::start();
    lander(&mut rig, "l1", FEATURE, &first_inbox);
    lander(&mut rig, "l2", FEATURE, &second_inbox);

    let first = land_as(&rig, "l1", &["done"]);
    assert!(first.status.success(), "{}", stderr(&first));
    assert_eq!(stdout(&first), "accepted feature/x granted\n");
    let second = land_as(&rig, "l2", &["done"]);
    assert!(second.status.success(), "{}", stderr(&second));
    assert_eq!(stdout(&second), "accepted feature/x queued 1\n");
    let doc = rig.status_json(&[]);
    assert_eq!(turns(&doc, FEATURE), holds("l1", &["l2"]), "{doc:#}");
    assert!(
        landing(&doc, FEATURE).get("release").is_none(),
        "release facts only on the staging row: {doc:#}"
    );
    assert!(landing(&doc, "develop")["release"].is_object(), "{doc:#}");

    let landed = land_as(&rig, "l1", &["land", "--end", "landed"]);
    assert!(landed.status.success(), "{}", stderr(&landed));
    assert_eq!(stdout(&landed), "ended feature/x landed\n");
    let frames = second_inbox.wait_frames(1);
    let content = user_frame(&frames[0])["message"]["content"]
        .as_str()
        .expect("content")
        .to_owned();
    assert!(
        content.starts_with("heartbeat: landing turn granted for feature/x in fixture."),
        "{content}"
    );
    assert!(
        first_inbox.frames().is_empty(),
        "the holder gets no message"
    );
    let doc = rig.status_json(&[]);
    assert_eq!(turns(&doc, FEATURE), holds("l2", &[]), "{doc:#}");
    wait_ledger(&rig, "two requests, two grants, one end", |ledger| {
        ledger_kinds(ledger, "land_requested").len() == 2
            && ledger_kinds(ledger, "land_granted").len() == 2
            && ledger_kinds(ledger, "land_ended").len() == 1
    });
}

/// Every land refusal exits 1 with its reason on stderr.
#[test]
fn landing_refusals() {
    let mut rig = Rig::new();
    let inbox = FakeInbox::start();
    let repo_path = rig.repo.path.clone();
    let none_held = rig
        .daemon()
        .cli_in(&repo_path, &["land", "--release", "develop"]);
    assert_eq!(none_held.status.code(), Some(1));
    assert!(
        stderr(&none_held).contains("no landing turn is held for develop"),
        "{}",
        stderr(&none_held)
    );
    lander(&mut rig, "l1", FEATURE, &inbox);
    lander(&mut rig, "l2", FEATURE, &inbox);

    let refusals = [
        ("main", "main is the default branch"),
        ("claude/l2", "claude/l2 is an agent branch"),
        ("nope", "nope is no local branch"),
    ];
    for (target, reason) in refusals {
        let refused = land_as(&rig, "l1", &["land", "--target", target]);
        assert_eq!(refused.status.code(), Some(1), "{target}");
        assert!(
            stderr(&refused).contains(reason),
            "{target}: {}",
            stderr(&refused)
        );
    }
    let unknown = land_as(&rig, "ghost", &["done"]);
    assert_eq!(unknown.status.code(), Some(1), "{}", stderr(&unknown));
    assert!(
        stderr(&unknown).contains("unknown session"),
        "{}",
        stderr(&unknown)
    );

    let requested = land_as(&rig, "l1", &["land", "--target", FEATURE]);
    assert_eq!(
        stdout(&requested),
        "granted feature/x\n",
        "{}",
        stderr(&requested)
    );
    let held = land_as(&rig, "l1", &["land", "--target", "develop"]);
    assert_eq!(held.status.code(), Some(1));
    let want = format!(
        "the session already has a landing entry for feature/x in {}",
        rig.repo.path.display()
    );
    assert!(stderr(&held).contains(&want), "{}", stderr(&held));
    let queued = land_as(&rig, "l2", &["done"]);
    assert_eq!(stdout(&queued), "accepted feature/x queued 1\n");
    let waiter = land_as(&rig, "l2", &["land", "--end", "landed"]);
    assert_eq!(waiter.status.code(), Some(1));
    assert!(
        stderr(&waiter).contains("not the turn holder"),
        "{}",
        stderr(&waiter)
    );
    let wrong = rig
        .daemon()
        .cli_as("l1", "not-the-token", &["land", "--end", "landed"]);
    assert_eq!(wrong.status.code(), Some(1));
    assert!(
        stderr(&wrong).contains("not the turn holder"),
        "{}",
        stderr(&wrong)
    );
    let elsewhere = tempfile::tempdir().expect("outside dir");
    let outside = rig
        .daemon()
        .cli_in(elsewhere.path(), &["land", "--release", "develop"]);
    assert_eq!(outside.status.code(), Some(1));
    assert!(
        stderr(&outside).contains("is in no registered repo"),
        "{}",
        stderr(&outside)
    );
    let doc = rig.status_json(&[]);
    assert_eq!(
        turns(&doc, FEATURE),
        holds("l1", &["l2"]),
        "refusals change nothing"
    );
}

/// A waiter abandons, a prompt keeps a waiter queued, and the operator's release hands the turn on.
#[test]
fn landing_queue_exits() {
    let mut rig = Rig::new();
    let inbox = FakeInbox::start();
    lander(&mut rig, "l1", FEATURE, &inbox);
    lander(&mut rig, "l2", FEATURE, &inbox);
    assert!(land_as(&rig, "l1", &["done"]).status.success());
    assert!(land_as(&rig, "l2", &["done"]).status.success());

    let abandoned = land_as(&rig, "l2", &["land", "--end", "abandoned"]);
    assert!(abandoned.status.success(), "{}", stderr(&abandoned));
    assert_eq!(stdout(&abandoned), "ended feature/x abandoned\n");
    assert_eq!(turns(&rig.status_json(&[]), FEATURE), holds("l1", &[]));

    let again = land_as(&rig, "l2", &["done"]);
    assert_eq!(stdout(&again), "accepted feature/x queued 1\n");
    rig.hook("UserPromptSubmit", "l2");
    rig.daemon().wait_for_state("l2", "running");
    assert_eq!(
        turns(&rig.status_json(&[]), FEATURE),
        holds("l1", &["l2"]),
        "a prompt keeps the waiter queued"
    );

    let repo_path = rig.repo.path.clone();
    let released = rig
        .daemon()
        .cli_in(&repo_path, &["land", "--release", FEATURE]);
    assert!(released.status.success(), "{}", stderr(&released));
    assert_eq!(stdout(&released), "released feature/x l1\n");
    assert_eq!(turns(&rig.status_json(&[]), FEATURE), holds("l2", &[]));
}

/// Holder and waiter survive a restart without a second grant message; the holder's session end then grants once.
#[test]
fn landing_survives_restart() {
    let mut rig = Rig::new();
    let first_inbox = FakeInbox::start();
    let second_inbox = FakeInbox::start();
    lander(&mut rig, "l1", FEATURE, &first_inbox);
    lander(&mut rig, "l2", FEATURE, &second_inbox);
    assert!(land_as(&rig, "l1", &["done"]).status.success());
    assert!(land_as(&rig, "l2", &["done"]).status.success());
    let before = turns(&rig.status_json(&[]), FEATURE);
    wait_ledger(&rig, "both requests and the grant", |ledger| {
        ledger_kinds(ledger, "land_requested").len() == 2
            && ledger_kinds(ledger, "land_granted").len() == 1
    });

    rig.restart();
    let doc = rig
        .daemon()
        .wait_for("turns replayed", |doc| turns(doc, FEATURE) == before);
    assert_eq!(turns(&doc, FEATURE), holds("l1", &["l2"]));
    assert!(second_inbox.frames().is_empty(), "no grant on replay");

    rig.hook("SessionEnd", "l1");
    rig.daemon().wait_for("the waiter holds", |doc| {
        turns(doc, FEATURE) == holds("l2", &[])
    });
    let frames = second_inbox.wait_frames(1);
    assert!(
        user_frame(&frames[0])["message"]["content"]
            .as_str()
            .is_some_and(|text| text.contains("landing turn granted for feature/x")),
        "{frames:?}"
    );
    assert!(first_inbox.frames().is_empty());
}

/// Two sessions on two bases both hold a turn; a third on the first base queues behind the first.
#[test]
fn landing_targets_are_separate_turns() {
    let mut rig = Rig::new();
    rig.repo
        .git(&rig.repo.path, &["branch", "feature/y", "main"]);
    let inbox = FakeInbox::start();
    lander(&mut rig, "l1", FEATURE, &inbox);
    lander(&mut rig, "l2", "feature/y", &inbox);
    lander(&mut rig, "l3", FEATURE, &inbox);

    let answers = [
        ("l1", "accepted feature/x granted\n"),
        ("l2", "accepted feature/y granted\n"),
        ("l3", "accepted feature/x queued 1\n"),
    ];
    for (session, want) in answers {
        let out = land_as(&rig, session, &["done"]);
        assert!(out.status.success(), "{session}: {}", stderr(&out));
        assert_eq!(stdout(&out), want, "{session}");
    }
    let doc = rig.status_json(&[]);
    assert_eq!(turns(&doc, FEATURE), holds("l1", &["l3"]), "{doc:#}");
    assert_eq!(turns(&doc, "feature/y"), holds("l2", &[]), "{doc:#}");
}

/// A session based on `main` is done without a landing; a session in the main checkout has no branch row.
#[test]
fn done_without_target() {
    let mut rig = Rig::new();
    let inbox = FakeInbox::start();
    lander(&mut rig, "m1", "main", &inbox);
    let exempt = land_as(&rig, "m1", &["done"]);
    assert!(exempt.status.success(), "{}", stderr(&exempt));
    assert_eq!(stdout(&exempt), "no-target main\n");
    assert!(
        ledger_kinds(&rig.ledger(), "land_requested").is_empty(),
        "no request recorded"
    );
    let doc = rig.status_json(&[]);
    assert!(turns(&doc, "main").0.is_none(), "{doc:#}");

    let checkout = rig.repo.path.clone();
    rig.session_in("mc", "CC,FImpl - mc - [WIP]", &checkout, None);
    rig.daemon()
        .wait_for("mc known", |doc| row(doc, "mc").is_some());
    let main_checkout = land_as(&rig, "mc", &["done"]);
    assert_eq!(
        main_checkout.status.code(),
        Some(1),
        "{}",
        stdout(&main_checkout)
    );
    assert!(
        stderr(&main_checkout).contains("no branch row for"),
        "{}",
        stderr(&main_checkout)
    );
}

/// A daemon whose config names `release` as the staging target names it in the status and grants its turn before it
/// exists.
#[test]
fn custom_staging_target() {
    let mut rig = Rig::staged("release");
    let inbox = FakeInbox::start();
    lander(&mut rig, "l1", "main", &inbox);
    let doc = rig.status_json(&[]);
    assert_eq!(doc["staging_target"], "release", "{doc:#}");
    assert_eq!(
        landing(&doc, "release")["release"]["exists"],
        false,
        "{doc:#}"
    );
    let granted = land_as(&rig, "l1", &["land", "--target", "release"]);
    assert!(granted.status.success(), "{}", stderr(&granted));
    assert_eq!(stdout(&granted), "granted release\n");
    let doc = rig.status_json(&[]);
    assert_eq!(turns(&doc, "release"), holds("l1", &[]), "{doc:#}");
}

/// A daemon whose config names no staging target (D11's inert default) omits `staging_target` from the status, lists
/// no staging landing row and refuses a turn on the former default `develop`.
#[test]
fn no_staging_target_no_landing_row() {
    let mut rig = Rig::staged("");
    let inbox = FakeInbox::start();
    lander(&mut rig, "l1", "main", &inbox);
    let doc = rig.status_json(&[]);
    assert!(doc.get("staging_target").is_none(), "{doc:#}");
    assert_eq!(doc["landings"], json!([]), "{doc:#}");
    let refused = land_as(&rig, "l1", &["land", "--target", "develop"]);
    assert!(!refused.status.success(), "{}", stdout(&refused));
}
