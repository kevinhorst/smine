//! Edges: the daemon's inputs and action executors. Each owns one input or one action kind, parses and forwards into
//! the engine inbox, and holds no fleet state (read cursors and watch handles only). Runtime failures become
//! `SensorChanged` events, never daemon exits.
//!
//! * [`hook`] — HTTP (tokio, axum): `POST /hook`, `GET /healthz`, `GET /status`, `GET /runtime`, and the fleet,
//!   landing, worker and runtime-watch routes (`/subscribe`, `/join`, `/done`, `/land*`, `/worker`, `/runtime/*`).
//! * [`registry`] — `<claude home>/sessions/` watch (notify's callback thread).
//! * [`ledger`] — `<claude home>/tool-policy/denials.jsonl` tail (one reader thread).
//! * [`exits`] — kqueue process-exit watch (one waiter thread; registration from the engine thread).
//! * [`recorder`] — `serve --record` (one writer thread).
//! * [`repos`] — `--repos` registry watch, `ReposChanged` on every change (one reader thread).
//! * [`git`] — ref watch over the registered repos (notify's callback thread), re-armed by `WatchRepos`.
//! * [`agentwt`] — scan and plan-read runner (tokio tasks, `spawn_blocking`).
//! * [`peek`] — spawn trees from the central peek over MCP (one tokio task).
//! * [`socket`] — the action-ledger writer and digest sender (one thread).
//! * [`transcript`] — subagent models from transcript heads, for the heart attack (one thread).
//! * [`config`] — `<claude home>/heartbeat/config.json` watch, live keys on every change (one reader thread).

use jiff::Timestamp;

use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{SensorName, SensorState};

pub mod agentwt;
pub mod config;
pub mod exits;
pub mod git;
pub mod hook;
pub mod ledger;
pub mod peek;
pub mod recorder;
pub mod registry;
pub mod repos;
pub mod socket;
pub mod transcript;

/// A sensor-health input, stamped now.
#[must_use]
pub fn sensor(sensor: SensorName, state: SensorState, detail: Option<String>) -> Input {
    Input::Event(Event::SensorChanged {
        sensor,
        state,
        detail,
        at: Timestamp::now(),
    })
}
