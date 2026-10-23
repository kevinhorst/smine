//! Shared fixture builders: a throwaway claude home, the daemon subprocess, hook bodies from the replay fixture,
//! registry files, ledger lines.
#![allow(dead_code)]

use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::SocketAddr;
#[cfg(unix)]
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::process::{Child, ChildStderr, Command, Output, Stdio};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use axum::extract::State;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::{Value, json};
use tempfile::TempDir;

pub mod git;

const POLL: Duration = Duration::from_millis(50);
const DEADLINE: Duration = Duration::from_secs(5);
const LISTENING: &str = "heartbeat: listening on ";
const REPLAY: &str = include_str!("../fixtures/session.jsonl");
/// A peek address nothing listens on: the peek sensor reads degraded.
pub const NO_PEEK: &str = "127.0.0.1:9";
/// The staging target of a daemon with a registered repo unless the scenario's config names one.
pub const STAGING: &str = "develop";
/// The messaging token every subscribing test session hands over.
pub const TOKEN: &str = "feedc0defeedc0defeedc0defeedc0de";
/// The title fixture sessions carry unless a scenario chooses one.
const FIXTURE_TITLE: &str = "CC,FD - fixture - [WIP]";
/// The cwd fixture sessions carry unless a scenario chooses one.
const FIXTURE_CWD: &str = "/tmp/heartbeat-fixture";
/// Idle connections the shared agent keeps: above the load bench's 32 workers. ureq's default of 3 per host
/// closes the rest after every request, and at 2,000 requests/s their `TIME_WAIT` exhausts the ephemeral ports.
const IDLE_CONNECTIONS: usize = 64;

/// A throwaway claude home with `sessions/` and `tool-policy/`.
pub struct Home {
    dir: TempDir,
}

/// The daemon subprocess; killed and reaped on drop.
pub struct Daemon {
    child: Child,
    home: PathBuf,
    pub addr: SocketAddr,
    log: Option<JoinHandle<String>>,
    agent: ureq::Agent,
}

impl Home {
    /// Writes `repos.json` registering `repo` and the heartbeat config `config` (staging target [`STAGING`] unless it
    /// names one); returns the `serve` args that load both.
    pub fn git_args(&self, repo: &Path, config: &Value) -> Vec<String> {
        let registry = self.path().join("repos.json");
        let body = json!({ "repos": [{ "name": "fixture", "path": repo }] });
        fs::write(&registry, body.to_string()).expect("registry");
        let mut config = config.clone();
        if config.get("staging_target").is_none() {
            config["staging_target"] = Value::from(STAGING);
        }
        let mut args = vec![
            "--repos".to_owned(),
            registry.to_str().expect("utf-8 temp path").to_owned(),
        ];
        args.extend(self.config_args(&config));
        args
    }

    /// Writes the heartbeat config `config` (peek at [`NO_PEEK`] unless it names one); returns the `serve` args that
    /// load it.
    pub fn config_args(&self, config: &Value) -> Vec<String> {
        let mut config = config.clone();
        if config.get("peek_addr").is_none() {
            config["peek_addr"] = Value::from(NO_PEEK);
        }
        self.write_config(&config);
        let path = self.config_path();
        vec![
            "--config".to_owned(),
            path.to_str().expect("utf-8 temp path").to_owned(),
        ]
    }

    /// The heartbeat config file the `serve` args name.
    pub fn config_path(&self) -> PathBuf {
        self.path().join("heartbeat-config.json")
    }

    /// Writes `config` as the heartbeat config the way the install does (temp + rename), so a running daemon's
    /// config edge reads one complete file.
    pub fn write_config(&self, config: &Value) {
        self.write_config_raw(&config.to_string());
    }

    /// Writes `body` as the heartbeat config (temp + rename), parseable or not: the malformed-file scenarios.
    pub fn write_config_raw(&self, body: &str) {
        let path = self.config_path();
        let tmp = path.with_extension("json.tmp");
        fs::write(&tmp, body).expect("heartbeat config tmp");
        fs::rename(&tmp, &path).expect("heartbeat config");
    }

    /// The first replay body of `event`, sent by subagent `agent` of `session`.
    pub fn agent_hook(&self, event: &str, session: &str, agent: &str) -> String {
        let mut body: Value = serde_json::from_str(&self.hook(event, session)).expect("hook body");
        body["agent_id"] = Value::from(agent);
        if body["agent_type"].is_null() {
            body["agent_type"] = Value::from("Explore");
        }
        body.to_string()
    }

    /// The first replay body of `event`, sent by the root agent of `session`.
    pub fn root_hook(&self, event: &str, session: &str) -> String {
        let mut body: Value = serde_json::from_str(&self.hook(event, session)).expect("hook body");
        if let Some(fields) = body.as_object_mut() {
            fields.remove("agent_id");
            fields.remove("agent_type");
        }
        body.to_string()
    }

    /// Writes subagent `agent`'s transcript under `session`'s: its prompt, then one assistant line on `model`.
    pub fn subagent_transcript(&self, session: &str, agent: &str, model: &str) {
        let dir = self
            .path()
            .join(format!("projects/fixture/{session}/subagents"));
        fs::create_dir_all(&dir).expect("subagents dir");
        let lines = [
            json!({ "type": "user", "message": { "role": "user", "content": "go" } }),
            json!({ "type": "assistant", "message": { "role": "assistant", "model": model, "content": [] } }),
        ];
        let text: Vec<String> = lines.iter().map(Value::to_string).collect();
        let path = dir.join(format!("agent-{agent}.jsonl"));
        fs::write(path, text.join("\n") + "\n").expect("subagent transcript");
    }

    pub fn new() -> Home {
        let home = Home::bare();
        fs::create_dir_all(home.path().join("sessions")).expect("sessions dir");
        home
    }

    /// A home without `sessions/` (the degraded-registry scenario).
    pub fn bare() -> Home {
        let dir = tempfile::tempdir().expect("tempdir");
        fs::create_dir_all(dir.path().join("tool-policy")).expect("tool-policy dir");
        Home { dir }
    }

    pub fn path(&self) -> &Path {
        self.dir.path()
    }

    /// Every replay body, retargeted to `session` under this home.
    pub fn replay_bodies(&self, session: &str) -> Vec<String> {
        REPLAY
            .lines()
            .map(|line| self.retarget(line, session).to_string())
            .collect()
    }

    /// The first replay body of `event`, retargeted to `session` under this home.
    pub fn hook(&self, event: &str, session: &str) -> String {
        REPLAY
            .lines()
            .map(|line| self.retarget(line, session))
            .find(|body| body["hook_event_name"] == event)
            .expect("event in fixture")
            .to_string()
    }

    /// [`Home::hook`] with the body's `cwd` set to `cwd` (fleet scenarios: a cwd in the registered fixture repo).
    pub fn hook_in(&self, event: &str, session: &str, cwd: &Path) -> String {
        let mut body: Value = serde_json::from_str(&self.hook(event, session)).expect("hook body");
        body["cwd"] = Value::from(cwd.to_str().expect("utf-8 cwd"));
        body.to_string()
    }

    fn retarget(&self, line: &str, session: &str) -> Value {
        let record: Value = serde_json::from_str(line).expect("replay line");
        let mut body = record["body"].clone();
        let transcript = self
            .path()
            .join(format!("projects/fixture/{session}.jsonl"));
        body["session_id"] = Value::from(session);
        body["transcript_path"] = Value::from(transcript.to_str().expect("utf-8 temp path"));
        body
    }

    /// Writes `sessions/<pid>.json` the way Claude Code does (temp + rename, complete file).
    pub fn registry(&self, pid: u32, session: &str, status: &str) {
        let cwd = Path::new(FIXTURE_CWD);
        self.registry_titled(pid, session, status, FIXTURE_TITLE, cwd, None);
    }

    /// [`Home::registry`] with a chosen title, cwd and messaging socket (fleet scenarios).
    pub fn registry_titled(
        &self,
        pid: u32,
        session: &str,
        status: &str,
        title: &str,
        cwd: &Path,
        socket: Option<&Path>,
    ) {
        let project = self.path().join("projects/fixture");
        fs::create_dir_all(&project).expect("project dir");
        fs::write(project.join(format!("{session}.jsonl")), "").expect("transcript");
        self.write_entry(pid, session, status, title, cwd, socket);
    }

    /// Writes `sessions/<pid>.json` without a transcript, like Claude Desktop's placeholder sessions.
    pub fn placeholder(&self, pid: u32, session: &str, status: &str) {
        let cwd = Path::new(FIXTURE_CWD);
        self.write_entry(pid, session, status, FIXTURE_TITLE, cwd, None);
    }

    fn write_entry(
        &self,
        pid: u32,
        session: &str,
        status: &str,
        title: &str,
        cwd: &Path,
        socket: Option<&Path>,
    ) {
        let mut entry = json!({
            "pid": pid,
            "sessionId": session,
            "cwd": cwd,
            "kind": "interactive",
            "name": title,
            "status": status,
            "statusUpdatedAt": jiff::Timestamp::now().as_millisecond(),
        });
        if let Some(socket) = socket {
            entry["messagingSocketPath"] = Value::from(socket.to_str().expect("utf-8 socket path"));
        }
        let tmp = self.path().join(format!("sessions/{pid}.json.tmp"));
        fs::write(&tmp, entry.to_string()).expect("registry tmp");
        fs::rename(&tmp, self.path().join(format!("sessions/{pid}.json"))).expect("registry file");
    }

    /// Appends one ledger line the way toolpolicy does; `ts` in `time.RFC3339` seconds.
    pub fn deny(&self, session: &str, command: &str, candidate_rule: &str, ts: &str) {
        let line = json!({
            "candidate_rule": candidate_rule,
            "cause": "no allow rule matches",
            "cwd": "/tmp/heartbeat-fixture",
            "input": command,
            "session_id": session,
            "ts": ts,
            "tool": "Bash",
        });
        let mut ledger = OpenOptions::new()
            .create(true)
            .append(true)
            .open(self.path().join("tool-policy/denials.jsonl"))
            .expect("ledger");
        writeln!(ledger, "{line}").expect("ledger line");
    }
}

/// The current UTC second as the ledger writes it.
pub fn ledger_now() -> String {
    format!("{:.0}", jiff::Timestamp::now())
}

impl Daemon {
    /// Starts `heartbeat serve` on an ephemeral port and waits for its listening line.
    pub fn start(home: &Path) -> Daemon {
        Daemon::start_with(home, &[])
    }

    /// Same, with extra `serve` arguments (e.g. `--record <file>`); when they name no `--config`, a config holding
    /// only `{"peek_addr": NO_PEEK}` is written under `home` and passed.
    pub fn start_with(home: &Path, serve_args: &[&str]) -> Daemon {
        let mut config_args = Vec::new();
        if !serve_args.contains(&"--config") {
            let path = home.join("heartbeat-config.json");
            fs::write(&path, json!({ "peek_addr": NO_PEEK }).to_string())
                .expect("heartbeat config");
            config_args = vec![
                "--config".to_owned(),
                path.to_str().expect("utf-8 temp path").to_owned(),
            ];
        }
        let mut child = Command::new(env!("CARGO_BIN_EXE_heartbeat"))
            .args(["--addr", "127.0.0.1:0", "--claude-home"])
            .arg(home)
            .arg("serve")
            .args(&config_args)
            .args(serve_args)
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .spawn()
            .expect("spawn heartbeat serve");
        let mut stderr = BufReader::new(child.stderr.take().expect("piped stderr"));
        let addr = listening_addr(&mut stderr);
        let log = thread::spawn(move || drain(stderr));
        let config = ureq::Agent::config_builder()
            .http_status_as_error(false)
            .max_idle_connections(IDLE_CONNECTIONS)
            .max_idle_connections_per_host(IDLE_CONNECTIONS)
            .build();
        Daemon {
            child,
            home: home.to_path_buf(),
            addr,
            log: Some(log),
            agent: ureq::Agent::new_with_config(config),
        }
    }

    pub fn pid(&self) -> u32 {
        self.child.id()
    }

    /// POSTs a hook body; returns the HTTP status. The ack body is read to the end so the agent returns the
    /// connection to its pool — an unread body closes it, and the bench would exhaust ephemeral ports.
    pub fn post(&self, body: &str) -> u16 {
        let mut response = self
            .agent
            .post(format!("http://{}/hook", self.addr))
            .header("content-type", "application/json")
            .send(body.to_owned())
            .expect("POST /hook");
        let status = response.status().as_u16();
        response.body_mut().read_to_string().expect("ack body");
        status
    }

    /// POSTs a hook body; returns the HTTP status and the answer's JSON.
    pub fn post_json(&self, body: &str) -> (u16, Value) {
        let mut response = self
            .agent
            .post(format!("http://{}/hook", self.addr))
            .header("content-type", "application/json")
            .send(body.to_owned())
            .expect("POST /hook");
        let status = response.status().as_u16();
        let text = response.body_mut().read_to_string().expect("answer body");
        (status, serde_json::from_str(&text).expect("answer json"))
    }

    /// `GET /status` directly.
    pub fn document(&self) -> Value {
        let mut response = self
            .agent
            .get(format!("http://{}/status", self.addr))
            .call()
            .expect("GET /status");
        serde_json::from_str(&response.body_mut().read_to_string().expect("body"))
            .expect("status json")
    }

    /// Runs the CLI against this daemon with extra env; the caller's env never leaks in — the messaging variables
    /// least of all: a test never hands over a real session's socket.
    pub fn cli(&self, args: &[&str], env: &[(&str, &str)]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_heartbeat"))
            .args(["--addr", &self.addr.to_string(), "--claude-home"])
            .arg(&self.home)
            .args(args)
            .env_remove("HEARTBEAT_ADDR")
            .env_remove("CLAUDE_CONFIG_DIR")
            .env_remove("CLAUDE_CODE_SESSION_ID")
            .env_remove("CLAUDE_CODE_MESSAGING_SOCKET")
            .env_remove("CLAUDE_CODE_MESSAGING_TOKEN")
            .envs(env.iter().copied())
            .output()
            .expect("run heartbeat")
    }

    /// Runs the CLI against this daemon from `cwd`, with the same env hygiene as [`Daemon::cli`]; for the commands
    /// that read their working directory (`land --release`).
    pub fn cli_in(&self, cwd: &Path, args: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_heartbeat"))
            .current_dir(cwd)
            .args(["--addr", &self.addr.to_string(), "--claude-home"])
            .arg(&self.home)
            .args(args)
            .env_remove("HEARTBEAT_ADDR")
            .env_remove("CLAUDE_CONFIG_DIR")
            .env_remove("CLAUDE_CODE_SESSION_ID")
            .env_remove("CLAUDE_CODE_MESSAGING_SOCKET")
            .env_remove("CLAUDE_CODE_MESSAGING_TOKEN")
            .output()
            .expect("run heartbeat")
    }

    /// Polls until `session` is in `state`; panics with the last document after the deadline.
    pub fn wait_for_state(&self, session: &str, state: &str) -> Value {
        let started = Instant::now();
        loop {
            let document = self.document();
            if let Some(found) = row(&document, session).filter(|found| found["state"] == state) {
                return found.clone();
            }
            assert!(
                started.elapsed() < DEADLINE,
                "{session} never reached {state}: {document:#}"
            );
            thread::sleep(POLL);
        }
    }

    /// Polls until the daemon applied at least `events` events.
    pub fn wait_for_events(&self, events: u64) -> Value {
        let started = Instant::now();
        loop {
            let document = self.document();
            if document["heartbeat"]["events_applied"]
                .as_u64()
                .unwrap_or(0)
                >= events
            {
                return document;
            }
            assert!(
                started.elapsed() < DEADLINE,
                "fewer than {events} events: {document:#}"
            );
            thread::sleep(POLL);
        }
    }

    /// Polls until `check` holds on the document; panics with the last document after the deadline.
    pub fn wait_for(&self, what: &str, check: impl Fn(&Value) -> bool) -> Value {
        let started = Instant::now();
        loop {
            let document = self.document();
            if check(&document) {
                return document;
            }
            assert!(started.elapsed() < DEADLINE, "never {what}: {document:#}");
            thread::sleep(POLL);
        }
    }

    /// `GET /runtime` directly.
    pub fn runtime_document(&self) -> Value {
        let mut response = self
            .agent
            .get(format!("http://{}/runtime", self.addr))
            .call()
            .expect("GET /runtime");
        serde_json::from_str(&response.body_mut().read_to_string().expect("body"))
            .expect("runtime json")
    }

    /// Polls until `check` holds on the runtime document; panics with the last document after the deadline.
    pub fn wait_runtime(&self, what: &str, check: impl Fn(&Value) -> bool) -> Value {
        let started = Instant::now();
        loop {
            let document = self.runtime_document();
            if check(&document) {
                return document;
            }
            assert!(started.elapsed() < DEADLINE, "never {what}: {document:#}");
            thread::sleep(POLL);
        }
    }

    /// `heartbeat subscribe --fleet <fleet> --target <target> --format json` as `session`, handing over `inbox` and
    /// [`TOKEN`].
    #[cfg(unix)]
    pub fn subscribe(&self, session: &str, fleet: &str, target: &str, inbox: &FakeInbox) -> Output {
        let socket = inbox.path.to_str().expect("utf-8 socket path");
        self.subscribe_via(session, fleet, target, socket)
    }

    /// Same, with any socket path.
    pub fn subscribe_via(&self, session: &str, fleet: &str, target: &str, socket: &str) -> Output {
        self.cli(
            &[
                "subscribe",
                "--fleet",
                fleet,
                "--target",
                target,
                "--format",
                "json",
            ],
            &[
                ("CLAUDE_CODE_SESSION_ID", session),
                ("CLAUDE_CODE_MESSAGING_SOCKET", socket),
                ("CLAUDE_CODE_MESSAGING_TOKEN", TOKEN),
            ],
        )
    }

    /// `heartbeat <args>` as `session` handing over `token` (join, done, record).
    pub fn cli_as(&self, session: &str, token: &str, args: &[&str]) -> Output {
        self.cli(
            args,
            &[
                ("CLAUDE_CODE_SESSION_ID", session),
                ("CLAUDE_CODE_MESSAGING_TOKEN", token),
            ],
        )
    }

    /// `heartbeat join --fleet <fleet>` as `session` with [`TOKEN`]; asserts success and returns the member token.
    pub fn join(&self, session: &str, fleet: &str) -> String {
        let output = self.cli_as(session, TOKEN, &["join", "--fleet", fleet]);
        assert!(
            output.status.success(),
            "join {session} {fleet}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        String::from_utf8(output.stdout).expect("utf-8 stdout")
    }
}

/// A stand-in central peek: `/healthz`, and a JSON-RPC `/mcp` answering what rmcp's client sends (`initialize`,
/// notifications, `tools/call` for `session_events`); `GET /mcp` is 405 like the real peek.
pub struct FakePeek {
    pub addr: String,
    state: Arc<Mutex<PeekState>>,
    _runtime: tokio::runtime::Runtime,
}

struct PeekState {
    claude_home: String,
    ready: bool,
    events: BTreeMap<String, Value>,
    calls: Vec<String>,
}

impl FakePeek {
    /// Serves `claude_home` as its identity, with empty event buffers.
    pub fn start(claude_home: &Path) -> FakePeek {
        let state = Arc::new(Mutex::new(PeekState {
            claude_home: claude_home.to_str().expect("home is UTF-8").to_owned(),
            ready: true,
            events: BTreeMap::new(),
            calls: Vec::new(),
        }));
        let runtime = tokio::runtime::Builder::new_multi_thread()
            .worker_threads(1)
            .enable_all()
            .build()
            .expect("runtime");
        let listener = runtime
            .block_on(tokio::net::TcpListener::bind("127.0.0.1:0"))
            .expect("bind fake peek");
        let addr = listener.local_addr().expect("fake peek addr").to_string();
        let app = axum::Router::new()
            .route("/healthz", axum::routing::get(peek_healthz))
            .route(
                "/mcp",
                axum::routing::post(peek_mcp)
                    .get(|| async { StatusCode::METHOD_NOT_ALLOWED })
                    .delete(|| async { StatusCode::OK }),
            )
            .with_state(Arc::clone(&state));
        runtime.spawn(async move {
            let _ = axum::serve(listener, app).await;
        });
        FakePeek {
            addr,
            state,
            _runtime: runtime,
        }
    }

    /// `session_events` `structuredContent` for `session`.
    pub fn set_events(&self, session: &str, content: Value) {
        self.state
            .lock()
            .expect("fake peek state")
            .events
            .insert(session.to_owned(), content);
    }

    /// Serves another Claude home (the foreign-identity scenario).
    pub fn set_home(&self, claude_home: &str) {
        claude_home.clone_into(&mut self.state.lock().expect("fake peek state").claude_home);
    }

    /// Tool names called so far, in order.
    pub fn calls(&self) -> Vec<String> {
        self.state.lock().expect("fake peek state").calls.clone()
    }
}

async fn peek_healthz(State(state): State<Arc<Mutex<PeekState>>>) -> axum::Json<Value> {
    let state = state.lock().expect("fake peek state");
    axum::Json(json!({
        "claudeHome": state.claude_home,
        "codexHome": "/nonexistent/.codex",
        "controlPort": 0,
        "ready": state.ready,
        "version": "fake"
    }))
}

async fn peek_mcp(
    State(state): State<Arc<Mutex<PeekState>>>,
    axum::Json(request): axum::Json<Value>,
) -> Response {
    let Some(id) = request.get("id").cloned() else {
        return StatusCode::ACCEPTED.into_response();
    };
    let result = match request["method"].as_str().unwrap_or_default() {
        "initialize" => json!({
            "protocolVersion": request["params"]["protocolVersion"],
            "capabilities": { "tools": { "listChanged": true } },
            "serverInfo": { "name": "peek-mcp", "version": "fake" }
        }),
        "tools/call" => {
            let name = request["params"]["name"]
                .as_str()
                .unwrap_or_default()
                .to_owned();
            let mut state = state.lock().expect("fake peek state");
            state.calls.push(name.clone());
            if name != "session_events" {
                return StatusCode::NOT_FOUND.into_response();
            }
            let session = request["params"]["arguments"]["id"]
                .as_str()
                .unwrap_or_default();
            let content = state
                .events
                .get(session)
                .cloned()
                .unwrap_or_else(|| json!({ "events": [] }));
            json!({
                "content": [{ "type": "text", "text": "See structuredContent for the full response." }],
                "structuredContent": content
            })
        }
        _ => json!({}),
    };
    let body = json!({ "jsonrpc": "2.0", "id": id, "result": result });
    ([("mcp-session-id", "fake-session")], axum::Json(body)).into_response()
}

/// A stand-in messaging socket: records every connection's lines, never answers (like Claude Code's inbox).
#[cfg(unix)]
pub struct FakeInbox {
    pub path: PathBuf,
    dir: Option<TempDir>,
    frames: Arc<Mutex<Vec<Vec<String>>>>,
}

#[cfg(unix)]
impl FakeInbox {
    /// Binds a socket under `/tmp` (macOS caps socket paths at 104 bytes).
    pub fn start() -> FakeInbox {
        let dir = tempfile::Builder::new()
            .prefix("hb")
            .tempdir_in("/tmp")
            .expect("socket dir");
        let path = dunce::canonicalize(dir.path())
            .expect("canonical socket dir")
            .join("inbox.sock");
        let listener = UnixListener::bind(&path).expect("bind inbox");
        let frames = Arc::new(Mutex::new(Vec::new()));
        let sink = Arc::clone(&frames);
        thread::spawn(move || {
            for stream in listener.incoming() {
                let Ok(mut stream) = stream else {
                    continue;
                };
                let mut text = String::new();
                let _ = stream.read_to_string(&mut text);
                sink.lock()
                    .expect("inbox frames")
                    .push(text.lines().map(str::to_owned).collect());
            }
        });
        FakeInbox {
            path,
            dir: Some(dir),
            frames,
        }
    }

    /// Every connection's lines so far.
    pub fn frames(&self) -> Vec<Vec<String>> {
        self.frames.lock().expect("inbox frames").clone()
    }

    /// Waits up to 5 s for `count` connections.
    pub fn wait_frames(&self, count: usize) -> Vec<Vec<String>> {
        let deadline = Instant::now() + DEADLINE;
        loop {
            let frames = self.frames();
            if frames.len() >= count || Instant::now() > deadline {
                assert_eq!(frames.len(), count, "inbox connections");
                return frames;
            }
            thread::sleep(POLL);
        }
    }

    /// Removes the socket: the inbox is gone, a send is refused.
    pub fn close(&mut self) {
        let _ = fs::remove_file(&self.path);
        self.dir = None;
    }
}

impl Drop for Daemon {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        if let Some(log) = self.log.take() {
            let _ = log.join();
        }
    }
}

/// The session row of `document`, if listed.
pub fn row<'a>(document: &'a Value, session: &str) -> Option<&'a Value> {
    document["sessions"]
        .as_array()
        .and_then(|rows| rows.iter().find(|row| row["session_id"] == session))
}

/// The sensor row named `name`.
pub fn sensor<'a>(document: &'a Value, name: &str) -> Option<&'a Value> {
    document["sensors"]
        .as_array()
        .and_then(|rows| rows.iter().find(|row| row["name"] == name))
}

/// Resident set of `pid`, for the load bench.
pub fn rss_bytes(pid: u32) -> u64 {
    let pid = sysinfo::Pid::from_u32(pid);
    let mut system = sysinfo::System::new();
    system.refresh_processes(sysinfo::ProcessesToUpdate::Some(&[pid]), true);
    system.process(pid).map_or(0, sysinfo::Process::memory)
}

/// Reads stderr byte-wise until the listening line.
fn listening_addr(stderr: &mut BufReader<ChildStderr>) -> SocketAddr {
    let mut line = Vec::new();
    loop {
        line.clear();
        let read = stderr.read_until(b'\n', &mut line).expect("read stderr");
        assert!(read > 0, "heartbeat exited before listening");
        let text = String::from_utf8_lossy(&line);
        if let Some(addr) = text.trim().strip_prefix(LISTENING) {
            return addr.parse().expect("listen address");
        }
    }
}

/// Keeps the pipe drained for the daemon's lifetime; returns everything it logged.
fn drain(mut stderr: BufReader<ChildStderr>) -> String {
    let mut all = Vec::new();
    let mut line = Vec::new();
    while stderr
        .read_until(b'\n', &mut line)
        .is_ok_and(|read| read > 0)
    {
        all.extend_from_slice(&line);
        line.clear();
    }
    String::from_utf8_lossy(&all).into_owned()
}
