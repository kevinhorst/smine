//! heartbeat — the fleet control plane: the lifecycle of every Claude Code session on this machine, derived from
//! hook events, the session registry, the toolpolicy denial ledger and process exits, served as one status document.
//!
//! Module map, in data-flow order:
//! * [`ids`] identifiers; [`event`] the engine's inputs, hook-body parser included; [`action`] its outputs.
//! * [`fleet`] the state; [`apply`] the pure transition function, with [`repo`] folding git state (P2), [`attack`]
//!   the heart attack's sightings and `/hook` verdicts, [`member`] the fleet-member guard's `/hook` verdicts, and
//!   [`runtime`] enforced skill runs, findings, the runtime subscriber, its moves and the third `/hook` verdict step,
//!   over [`enforce`]'s compiled declarations and check kinds.
//! * [`config`], [`gitcmd`], [`plan`] parse the git config, git-changing commands and plan files; [`scan`] reduces
//!   one agentwt document; [`branch`] and [`collide`] derive branch states and collisions (all pure); [`protocol`]
//!   holds the protocol facts, every other behavioural value is a [`config`] key.
//! * [`peek`] peek's results; [`digest`] fleet changes as digests; [`action_ledger`] the persisted subscription,
//!   claim, membership, journal and landing record (P3, all pure); [`land`] the landing turns per repository.
//! * [`wire`] + [`status`] the status document built from the fleet; [`interfaces`] the `interfaces` handshake.
//! * `identity`, `edges`, `engine`, `serve`, `client`, `log`, `switches` (feature `cli`): the I/O around the core.
//!
//! Invariants: `apply` never reads a clock and never does I/O; the engine thread is the sole owner of
//! [`fleet::Fleet`]; edges hold no fleet state and forward in arrival order through one FIFO inbox; the core builds
//! without tokio (`cargo check --lib --no-default-features`).
#![forbid(unsafe_code)]

pub mod action;
pub mod action_ledger;
pub mod apply;
pub mod attack;
pub mod branch;
#[cfg(feature = "cli")]
pub mod client;
pub mod collide;
pub mod config;
pub mod digest;
#[cfg(feature = "cli")]
pub mod edges;
pub mod enforce;
#[cfg(feature = "cli")]
pub mod engine;
pub mod error;
pub mod event;
pub mod fleet;
pub mod gitcmd;
#[cfg(test)]
mod guards;
#[cfg(feature = "cli")]
pub mod identity;
pub mod ids;
pub mod interfaces;
pub mod land;
#[cfg(feature = "cli")]
pub mod log;
pub mod member;
pub mod peek;
pub mod plan;
pub mod protocol;
pub mod repo;
pub mod runtime;
pub mod scan;
#[cfg(feature = "cli")]
pub mod serve;
pub mod status;
#[cfg(feature = "cli")]
pub mod switches;
pub mod wire;

pub use error::{Error, Result};
