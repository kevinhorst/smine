//! `make heartbeat-load`: load evidence for exploration D4. Starts the built daemon on an ephemeral port with a
//! throwaway claude home, replays the fixture bodies for 320 sessions at 2,000 events/s for 60 s from 32 paced
//! threads, then checks every target and exits 1 on a miss:
//! * sustained rate ≥ 95 %, p99 hook ack ≤ 20 ms, zero non-2xx acks,
//! * zero dropped events: the daemon's `events_applied` reaches the events sent (plus its own sensor events),
//! * daemon peak RSS ≤ 200 MB,
//! * `apply` p99 ≤ 1 ms, measured by replaying the same bodies through `apply` in process.
//!
//! A second phase closes exploration D4's scan clause: git-changing hooks at the same rate against one registered
//! repo keep at most one agentwt scan in flight and at most one scan per settle window.
#![allow(clippy::expect_used, clippy::unwrap_used)]

#[path = "../tests/common/mod.rs"]
mod common;

use std::io::Write as _;
use std::process::ExitCode;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::time::{Duration, Instant};

use heartbeat::apply::apply;
use heartbeat::event::{Event, HookEvent};
use heartbeat::fleet::Fleet;
use jiff::Timestamp;

use common::git::GitRepo;
use common::{Daemon, Home};

const RATE_PER_SECOND: u32 = 2_000;
const DURATION: Duration = Duration::from_secs(60);
const WORKERS: u32 = 32;
const SESSIONS_PER_WORKER: u32 = 10;
const ACK_P99_LIMIT: Duration = Duration::from_millis(20);
const APPLY_P99_LIMIT: Duration = Duration::from_millis(1);
const RSS_LIMIT_BYTES: u64 = 200 * 1024 * 1024;
/// The runner's settle window: every scan waits at least this long, which bounds the scan count.
const SCAN_SETTLE_MS: u128 = 250;
/// Quiet time after the git phase so the last queued scan reports.
const SCAN_QUIET: Duration = Duration::from_secs(2);

/// What the HTTP phase measured.
struct Outcome {
    sent: u64,
    failed: u64,
    acks: Vec<Duration>,
    elapsed: Duration,
    events_applied: u64,
    peak_rss: u64,
}

fn main() -> ExitCode {
    let home = Home::new();
    let daemon = Daemon::start(home.path());
    let baseline = daemon.document()["heartbeat"]["events_applied"]
        .as_u64()
        .unwrap_or(0);
    let outcome = drive(&home, &daemon);
    drop(daemon);
    let apply_p99 = apply_p99(&home, outcome.sent);
    let base_ok = report(&outcome, baseline, apply_p99);
    let scans_ok = report_scans(&git_scans());
    if base_ok && scans_ok {
        ExitCode::SUCCESS
    } else {
        ExitCode::FAILURE
    }
}

fn drive(home: &Home, daemon: &Daemon) -> Outcome {
    let done = Arc::new(AtomicBool::new(false));
    let peak_rss = Arc::new(AtomicU64::new(0));
    let monitor = {
        let done = Arc::clone(&done);
        let peak_rss = Arc::clone(&peak_rss);
        let pid = daemon.pid();
        std::thread::spawn(move || {
            while !done.load(Ordering::Relaxed) {
                peak_rss.fetch_max(common::rss_bytes(pid), Ordering::Relaxed);
                std::thread::sleep(Duration::from_millis(250));
            }
        })
    };
    let bodies: Vec<Vec<Vec<String>>> = (0..WORKERS)
        .map(|worker| {
            (0..SESSIONS_PER_WORKER)
                .map(|slot| home.replay_bodies(&format!("load-{}", worker + WORKERS * slot)))
                .collect()
        })
        .collect();
    let (results, elapsed) = pace(daemon, &bodies);
    done.store(true, Ordering::Relaxed);
    monitor.join().expect("monitor");
    let sent = u64::try_from(results.len()).expect("count");
    let acks: Vec<Duration> = results.iter().flatten().copied().collect();
    let failed = sent - u64::try_from(acks.len()).expect("count");
    let document = daemon.wait_for_events(sent);
    Outcome {
        sent,
        failed,
        acks,
        elapsed,
        events_applied: document["heartbeat"]["events_applied"]
            .as_u64()
            .unwrap_or(0),
        peak_rss: peak_rss.load(Ordering::Relaxed),
    }
}

/// Paces `RATE_PER_SECOND` POSTs for `DURATION` over one thread per entry of `bodies`; a worker cycles its session
/// slots, each slot through its bodies in order. Returns each ack latency (`None` for a non-200) and the wall time.
fn pace(daemon: &Daemon, bodies: &[Vec<Vec<String>>]) -> (Vec<Option<Duration>>, Duration) {
    let per_worker = RATE_PER_SECOND * u32::try_from(DURATION.as_secs()).expect("secs") / WORKERS;
    let interval = Duration::from_secs(u64::from(WORKERS)) / RATE_PER_SECOND;
    let started = Instant::now();
    let results = std::thread::scope(|scope| {
        let workers: Vec<_> = bodies
            .iter()
            .map(|sessions| {
                scope.spawn(move || {
                    let mut results = Vec::with_capacity(usize::try_from(per_worker).unwrap_or(0));
                    for step in 0..per_worker {
                        if let Some(wait) =
                            (started + interval * step).checked_duration_since(Instant::now())
                        {
                            std::thread::sleep(wait);
                        }
                        let slot = usize::try_from(step % SESSIONS_PER_WORKER).unwrap_or(0)
                            % sessions.len();
                        let session = &sessions[slot];
                        let turn = usize::try_from(step / SESSIONS_PER_WORKER).unwrap_or(0)
                            % session.len();
                        let sent = Instant::now();
                        let is_ok = daemon.post(&session[turn]) == 200;
                        results.push(is_ok.then(|| sent.elapsed()));
                    }
                    results
                })
            })
            .collect();
        workers
            .into_iter()
            .flat_map(|worker| worker.join().expect("worker"))
            .collect()
    });
    (results, started.elapsed())
}

/// What the git phase measured.
struct ScanOutcome {
    sent: u64,
    failed: u64,
    elapsed: Duration,
    scans: u64,
    in_flight_high_water: u64,
}

/// Exploration D4's scan clause: git-changing hooks at the full rate against one registered repo.
fn git_scans() -> ScanOutcome {
    let home = Home::new();
    let repo = GitRepo::new();
    let work = repo.worktree("load", "main");
    let config = serde_json::json!({ "exempt_paths": ["plans/**"], "version_keys": ["version"] });
    let args = home.git_args(&repo.path, &config);
    let refs: Vec<&str> = args.iter().map(String::as_str).collect();
    let daemon = Daemon::start_with(home.path(), &refs);
    daemon.wait_for("the first scan", |doc| {
        doc["heartbeat"]["scans_completed"].as_u64() >= Some(1)
    });
    let body = serde_json::json!({
        "session_id": "load-git",
        "hook_event_name": "PostToolUse",
        "cwd": work,
        "tool_name": "Bash",
        "tool_input": { "command": "git commit -m load" },
    })
    .to_string();
    let bodies: Vec<Vec<Vec<String>>> = (0..WORKERS).map(|_| vec![vec![body.clone()]]).collect();
    let (results, elapsed) = pace(&daemon, &bodies);
    std::thread::sleep(SCAN_QUIET);
    let document = daemon.document();
    let sent = u64::try_from(results.len()).expect("count");
    ScanOutcome {
        sent,
        failed: sent - u64::try_from(results.iter().flatten().count()).expect("count"),
        elapsed,
        scans: document["heartbeat"]["scans_completed"]
            .as_u64()
            .unwrap_or(0),
        in_flight_high_water: document["heartbeat"]["scan_in_flight_high_water"]
            .as_u64()
            .unwrap_or(0),
    }
}

fn report_scans(outcome: &ScanOutcome) -> bool {
    let bound = u64::try_from(outcome.elapsed.as_millis() / SCAN_SETTLE_MS).unwrap_or(u64::MAX) + 2;
    let checks = [
        (
            "scans in flight ≤ 1",
            outcome.in_flight_high_water <= 1,
            outcome.in_flight_high_water.to_string(),
        ),
        (
            "scans ≤ settle bound",
            outcome.scans <= bound,
            format!("{} of ≤ {bound}", outcome.scans),
        ),
        (
            "zero failed git acks",
            outcome.failed == 0,
            outcome.failed.to_string(),
        ),
    ];
    let mut out = anstream::stdout();
    let _ = writeln!(
        out,
        "heartbeat-load git: {} git-changing events in {:?}",
        outcome.sent, outcome.elapsed
    );
    for (target, is_met, value) in &checks {
        let verdict = if *is_met { "PASS" } else { "FAIL" };
        let _ = writeln!(out, "{verdict}  {target:<22} {value}");
    }
    checks.iter().all(|(_, is_met, _)| *is_met)
}

fn apply_p99(home: &Home, sent: u64) -> Duration {
    let bodies = home.replay_bodies("apply");
    let mut fleet = Fleet::new(Timestamp::UNIX_EPOCH);
    let mut durations = Vec::with_capacity(usize::try_from(sent).unwrap_or(0));
    for n in 0..usize::try_from(sent).unwrap_or(0) {
        let hook = HookEvent::parse(bodies[n % bodies.len()].as_bytes(), Timestamp::UNIX_EPOCH)
            .expect("fixture parses");
        let started = Instant::now();
        let actions = apply(&mut fleet, Event::Hook(hook));
        durations.push(started.elapsed());
        drop(actions);
    }
    p99(&mut durations)
}

fn p99(values: &mut [Duration]) -> Duration {
    values.sort_unstable();
    let index = (values.len() * 99).div_ceil(100).saturating_sub(1);
    values.get(index).copied().unwrap_or_default()
}

fn report(outcome: &Outcome, baseline: u64, apply_p99: Duration) -> bool {
    let mut acks = outcome.acks.clone();
    let ack_p99 = p99(&mut acks);
    let achieved = u128::from(outcome.sent) * 1_000 / outcome.elapsed.as_millis().max(1);
    let applied = outcome.events_applied.saturating_sub(baseline);
    let checks = [
        (
            "rate ≥ 95 %",
            achieved * 100 >= u128::from(RATE_PER_SECOND) * 95,
            format!("{achieved}/s"),
        ),
        (
            "ack p99 ≤ 20 ms",
            ack_p99 <= ACK_P99_LIMIT,
            format!("{ack_p99:?}"),
        ),
        (
            "zero failed acks",
            outcome.failed == 0,
            outcome.failed.to_string(),
        ),
        (
            "zero dropped events",
            applied >= outcome.sent,
            format!("{applied} applied of {} sent", outcome.sent),
        ),
        (
            "RSS ≤ 200 MB",
            outcome.peak_rss <= RSS_LIMIT_BYTES,
            format!("{} MB", outcome.peak_rss / 1024 / 1024),
        ),
        (
            "apply p99 ≤ 1 ms",
            apply_p99 <= APPLY_P99_LIMIT,
            format!("{apply_p99:?}"),
        ),
    ];
    let mut out = anstream::stdout();
    let _ = writeln!(
        out,
        "heartbeat-load: {} events in {:?}",
        outcome.sent, outcome.elapsed
    );
    for (target, is_met, value) in &checks {
        let verdict = if *is_met { "PASS" } else { "FAIL" };
        let _ = writeln!(out, "{verdict}  {target:<22} {value}");
    }
    checks.iter().all(|(_, is_met, _)| *is_met)
}
