//! The interface versions this build implements, the answer of `heartbeat interfaces`; the catalog test in
//! `internal/skillgraph` pins the literals to `skills/interfaces.cue`.

use serde::Serialize;

/// The handshake document.
#[derive(Debug, Serialize)]
pub struct Interfaces {
    /// Sorted `<id>@<version>` entries.
    pub interfaces: Vec<&'static str>,
}

/// Every interface version this build implements, sorted.
pub const CLAIMED: &[&str] = &[
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
];

/// The document to print.
#[must_use]
pub fn answer() -> Interfaces {
    Interfaces {
        interfaces: CLAIMED.to_vec(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn claimed_is_sorted_and_versioned() {
        let mut sorted = CLAIMED.to_vec();
        sorted.sort_unstable();
        assert_eq!(sorted, CLAIMED);
        assert!(CLAIMED.iter().all(|claim| claim.contains('@')));
    }
}
