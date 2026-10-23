//! One test binary: `common` builds fixtures and runs the built daemon as a subprocess,
//! `scenarios` holds every plan verification item a throwaway claude home can exercise.
#![allow(clippy::expect_used, clippy::unwrap_used)]

mod common;
mod scenarios;
