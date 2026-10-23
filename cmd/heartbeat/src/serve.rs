//! `heartbeat serve`: binds the listener, then starts in dependency order — exit watch, agentwt runner and transcript
//! reader (before the engine, D21), engine, registry (watch + scan), config watch, ledger (watch + window), git state
//! (after the ledger: the repos watch, then the git watch armed with the repos read at start), recorder, HTTP.
//! Registry upserts are enqueued before startup denials. A registry, config, ledger, repos or git watch that cannot
//! start degrades its sensor; the daemon keeps serving. The config `main` read at start seeds the inbox capacity, the
//! edges' start-time values and the engine's live keys; the config edge reloads the live keys, the repos edge the
//! registered repos. Runs until SIGTERM/SIGINT, until the listener fails, or until the engine stops; launchd
//! `KeepAlive` restarts the daemon.
//!
//! Edges: HTTP (tokio, axum), registry and git watches (notify callback threads), ledger tail (one thread),
//! process-exit watch (one waiter thread), recorder (one thread), transcript reader (one thread), agentwt runner —
//! tokio tasks, each scan on `spawn_blocking`.
//!
//! P3: the engine's executor feeds the peek client and the socket sender; the action ledger is replayed into the
//! inbox before the registry scan, so subscriptions (with their repo binding) survive a restart; the peek client
//! starts after the recorder, before the HTTP edge.

use std::future::IntoFuture as _;
use std::net::SocketAddr;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use jiff::Timestamp;
use notify::RecommendedWatcher;
use tokio::net::TcpListener;
use tokio::sync::mpsc;

use crate::action_ledger;
use crate::config::{FileConfig, Live};
use crate::edges::exits::ExitWatch;
use crate::edges::hook::{self, HookState};
use crate::edges::{
    self, agentwt, git, ledger, peek, recorder, registry, repos, sensor, socket, transcript,
};
use crate::engine::{Engine, Executor, Input};
use crate::event::Event;
use crate::fleet::{Fleet, SensorName, SensorState, StagingTarget};
use crate::identity::Identity;
use crate::protocol::{ACTIONS_FILE, LEDGER_PATH, STATE_DIR};
use crate::repo::{Repo, RepoId};
use crate::runtime::Runtime;
use crate::{Error, Result, log};

/// What `serve` needs.
#[derive(Debug, Clone)]
pub struct Config {
    /// Listen address (loopback).
    pub addr: SocketAddr,
    /// The claude home this daemon serves.
    pub claude_home: PathBuf,
    /// `--record`: append every hook body here.
    pub record: Option<PathBuf>,
    /// `--repos`: the config server's repo registry; `None` turns git state off (D5).
    pub repos: Option<PathBuf>,
    /// The rendered config's path, watched by the config edge.
    pub config_path: PathBuf,
    /// The config read at start: `main` refused to start without it.
    pub file_config: FileConfig,
}

/// The registry at start: the repos the fleet is seeded with (warnings logged), or why it could not be read — a
/// `git` sensor detail once the edge is up; empty without `--repos`.
fn read_repos(registry: Option<&Path>) -> (Vec<(RepoId, Repo)>, Option<Error>) {
    let Some(registry) = registry else {
        return (Vec::new(), None);
    };
    match repos::read(registry) {
        Ok(registered) => {
            for warning in &registered.warnings {
                log::line(format_args!("{warning}"));
            }
            let repos = registered
                .repos
                .into_iter()
                .map(|(id, name)| (id, Repo::new(name)))
                .collect();
            (repos, None)
        }
        Err(err) => (Vec::new(), Some(err)),
    }
}

/// Runs the daemon.
#[allow(clippy::too_many_lines)]
pub async fn run(config: Config) -> Result<()> {
    let listener = TcpListener::bind(config.addr)
        .await
        .map_err(|source| Error::Listen {
            addr: config.addr,
            source,
        })?;
    let addr = listener.local_addr().map_err(|source| Error::Listen {
        addr: config.addr,
        source,
    })?;
    let identity = Arc::new(Identity::new(config.claude_home.clone(), addr)?);
    let (inbox, receiver) = mpsc::channel(config.file_config.startup.inbox_capacity());
    let (peek_requests, peek_inbox) = mpsc::unbounded_channel();
    let (jobs, job_inbox) = std::sync::mpsc::channel();

    let startup = config.file_config.startup.clone();
    let exits = ExitWatch::start(inbox.clone(), startup.exits_batch(), startup.exits_poll())?;
    let (repos, registry_error) = read_repos(config.repos.as_deref());
    let repo_ids: Vec<RepoId> = repos.iter().map(|(id, _)| id.clone()).collect();
    let git_watch = config
        .repos
        .as_ref()
        .map(|_| git::Watch::start(inbox.clone()))
        .transpose()?;
    let runner = agentwt::start(
        inbox.clone(),
        config.claude_home.clone(),
        startup.staging_target().map(str::to_owned),
        startup.scan_settle(),
    );
    let (reads, read_inbox) = std::sync::mpsc::channel();
    transcript::start(inbox.clone(), read_inbox, startup.transcript_max_lines())?;
    let engine = Engine {
        fleet: seeded_fleet(
            repos,
            config.file_config.live.clone(),
            startup.staging_target(),
        ),
        health: identity.health(),
        executor: Executor {
            exits,
            runner,
            git: git_watch.clone(),
            peek: peek_requests,
            socket: jobs,
            transcript: reads,
        },
    };
    let mut engine_task = tokio::task::spawn_blocking(move || engine.run(receiver));
    report(&inbox, SensorName::ProcessExit, None).await;

    let state_dir = config.claude_home.join(STATE_DIR);
    let (replay, note) = read_ledger(&state_dir.join(ACTIONS_FILE));
    for event in replay.events {
        let _ = inbox.send(Input::Event(event)).await;
    }
    socket::start(
        inbox.clone(),
        job_inbox,
        &state_dir,
        note,
        startup.socket_write_timeout(),
    )?;

    let registry_home = config.claude_home.clone();
    let registry_inbox = inbox.clone();
    let _registry = start_watch(&inbox, SensorName::Registry, "registry edge", move || {
        registry::start(&registry_home, registry_inbox)
    })
    .await?;

    let config_inbox = inbox.clone();
    let config_path = config.config_path.clone();
    let _config_watch = start_watch(&inbox, SensorName::Config, "config edge", move || {
        edges::config::start(&config_path, config_inbox)
    })
    .await?;

    let ledger_path = config.claude_home.join(LEDGER_PATH);
    let ledger_inbox = inbox.clone();
    let tail_window = startup.ledger_tail_window();
    let _ledger = start_watch(&inbox, SensorName::Ledger, "ledger edge", move || {
        ledger::start(&ledger_path, ledger_inbox, tail_window)
    })
    .await?;

    let _repos_watch = start_git(
        &inbox,
        config.repos.as_deref(),
        git_watch.as_ref(),
        registry_error,
        &repo_ids,
    )
    .await?;

    let recorder = config.record.as_deref().map(recorder::start).transpose()?;
    let peek_options = peek::Options {
        addr: startup.peek_addr(),
        claude_home: config.claude_home.clone(),
        retry: startup.peek_retry(),
        healthz_timeout: startup.peek_healthz_timeout(),
        call_timeout: startup.peek_call_timeout(),
    };
    peek::start(inbox.clone(), peek_inbox, peek_options);
    report(&inbox, SensorName::Hooks, None).await;
    let app = hook::router(HookState {
        inbox,
        identity,
        recorder,
        startup,
        caps: config.file_config.live.caps.clone(),
    });
    let shutdown = shutdown_signal()?;
    #[cfg(not(unix))]
    log::line(format_args!(
        "messaging sockets are not supported on windows: fleet subscriptions are refused"
    ));
    log::line(format_args!("listening on {addr}"));
    tokio::select! {
        served = axum::serve(listener, app).into_future() => served.map_err(|source| Error::Listen { addr, source }),
        joined = &mut engine_task => Err(Error::Task {
            task: "engine",
            detail: joined.map_or_else(|err| err.to_string(), |()| "inbox closed".to_owned()),
        }),
        signal = shutdown => Err(Error::Interrupted { signal }),
    }
}

/// Installs the termination handlers and yields the raw number of the first signal: SIGTERM or SIGINT.
#[cfg(unix)]
fn shutdown_signal() -> Result<impl std::future::Future<Output = i32>> {
    use tokio::signal::unix::{SignalKind, signal};

    let mut terminate = signal(SignalKind::terminate()).map_err(|err| Error::Task {
        task: "SIGTERM handler",
        detail: err.to_string(),
    })?;
    let mut interrupt = signal(SignalKind::interrupt()).map_err(|err| Error::Task {
        task: "SIGINT handler",
        detail: err.to_string(),
    })?;
    Ok(async move {
        tokio::select! {
            _ = terminate.recv() => SignalKind::terminate().as_raw_value(),
            _ = interrupt.recv() => SignalKind::interrupt().as_raw_value(),
        }
    })
}

/// Installs the console handlers and yields the unix number the event stands for: Ctrl-C and Ctrl-Break as SIGINT, close as SIGTERM.
#[cfg(windows)]
fn shutdown_signal() -> Result<impl std::future::Future<Output = i32>> {
    use tokio::signal::windows::{ctrl_break, ctrl_c, ctrl_close};

    use crate::protocol::{INTERRUPT_SIGNAL, TERMINATE_SIGNAL};

    let mut interrupt = ctrl_c().map_err(|err| Error::Task {
        task: "Ctrl-C handler",
        detail: err.to_string(),
    })?;
    let mut brk = ctrl_break().map_err(|err| Error::Task {
        task: "Ctrl-Break handler",
        detail: err.to_string(),
    })?;
    let mut close = ctrl_close().map_err(|err| Error::Task {
        task: "close handler",
        detail: err.to_string(),
    })?;
    Ok(async move {
        tokio::select! {
            _ = interrupt.recv() => INTERRUPT_SIGNAL,
            _ = brk.recv() => INTERRUPT_SIGNAL,
            _ = close.recv() => TERMINATE_SIGNAL,
        }
    })
}

/// Starts a watch edge on the blocking pool (its `start` uses `blocking_send`) and reports its sensor; the watcher,
/// held by the caller until `run` returns, or `None` when it could not start.
async fn start_watch<W: Send + 'static>(
    inbox: &mpsc::Sender<Input>,
    name: SensorName,
    task: &'static str,
    start: impl FnOnce() -> Result<W> + Send + 'static,
) -> Result<Option<W>> {
    let started = tokio::task::spawn_blocking(start)
        .await
        .map_err(|err| Error::Task {
            task,
            detail: err.to_string(),
        })?;
    match started {
        Ok(watcher) => {
            report(inbox, name, None).await;
            Ok(Some(watcher))
        }
        Err(err) => {
            report(inbox, name, Some(err)).await;
            Ok(None)
        }
    }
}

/// Git state's edges, off without `--repos`: the repos watch (its watcher, held by the caller until `run` returns),
/// then the git watch armed with the start-time repos; one `git` sensor report carries every failure — a registry
/// that did not read, a watch that did not arm — and each start-time repo gets its first `RefsChanged`.
async fn start_git(
    inbox: &mpsc::Sender<Input>,
    registry: Option<&Path>,
    watch: Option<&git::Watch>,
    registry_error: Option<Error>,
    repo_ids: &[RepoId],
) -> Result<Option<RecommendedWatcher>> {
    let (Some(registry), Some(watch)) = (registry, watch) else {
        return Ok(None);
    };
    let mut failures: Vec<String> = registry_error.iter().map(ToString::to_string).collect();
    let repos_inbox = inbox.clone();
    let registry = registry.to_path_buf();
    let watcher = tokio::task::spawn_blocking(move || repos::start(&registry, repos_inbox))
        .await
        .map_err(|err| Error::Task {
            task: "repos edge",
            detail: err.to_string(),
        })?;
    let watcher = match watcher {
        Ok(watcher) => Some(watcher),
        Err(err) => {
            failures.push(err.to_string());
            None
        }
    };
    let armed = watch.clone();
    let targets = repo_ids.to_vec();
    let arm_failure = tokio::task::spawn_blocking(move || armed.set_repos(&targets))
        .await
        .map_err(|err| Error::Task {
            task: "git edge",
            detail: err.to_string(),
        })?;
    failures.extend(arm_failure.iter().map(ToString::to_string));
    let failure = (!failures.is_empty()).then(|| Error::Task {
        task: "git state",
        detail: failures.join("; "),
    });
    report(inbox, SensorName::Git, failure).await;
    for repo in repo_ids {
        let changed = Event::RefsChanged {
            repo: repo.clone(),
            at: Timestamp::now(),
        };
        let _ = inbox.send(Input::Event(changed)).await;
    }
    Ok(watcher)
}

/// The engine's starting fleet: the registered repos, the live config keys and the staging target are seeded at
/// construction; the live keys change only through `apply` (`ConfigChanged`).
fn seeded_fleet(repos: Vec<(RepoId, Repo)>, live: Live, staging: Option<&str>) -> Fleet {
    let mut fleet = Fleet::new(Timestamp::now());
    fleet.staging = staging.map(|name| StagingTarget(name.to_owned()));
    fleet.repos = repos.into_iter().collect();
    fleet.exemptions = live.exemptions;
    fleet.lane_prefixes = live.lane_prefixes;
    fleet.forbidden_models = live.forbidden_models;
    fleet.caps = live.caps;
    fleet.runtime = Runtime::new(live.texts);
    fleet
}

/// Enqueues `sensor` ok, or degraded with the start failure; logs a failure too.
async fn report(inbox: &mpsc::Sender<Input>, name: SensorName, failure: Option<Error>) {
    let health = match failure {
        None => sensor(name, SensorState::Ok, None),
        Some(err) => {
            log::line(format_args!("{err}"));
            sensor(name, SensorState::Degraded, Some(err.to_string()))
        }
    };
    let _ = inbox.send(health).await;
}

/// The ledger's replay and a startup note for its sensor; a missing file is an empty ledger.
fn read_ledger(path: &Path) -> (action_ledger::Replay, Option<String>) {
    match std::fs::read_to_string(path) {
        Ok(text) => {
            let replay = action_ledger::replay(&text);
            let note = (replay.skipped > 0)
                .then(|| format!("{} unreadable ledger lines skipped", replay.skipped));
            (replay, note)
        }
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
            (action_ledger::Replay::default(), None)
        }
        Err(err) => (
            action_ledger::Replay::default(),
            Some(format!("read {}: {err}", path.display())),
        ),
    }
}
