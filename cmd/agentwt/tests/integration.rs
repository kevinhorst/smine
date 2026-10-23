//! Single integration test binary: fixtures in `common`, scenarios in `scenarios`
//! (both directories with `mod.rs`, so cargo builds one test target, not one per file).
//!
//! `expect`/`unwrap` are denied in production code only; a fixture builder panicking on a broken
//! fixture is the intended behaviour of a test.
#![allow(clippy::expect_used, clippy::unwrap_used)]

mod common;
mod scenarios;
