//! The interface versions this build implements, the answer of `agentwt interfaces`; the catalog test in
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
    "agentwt-forward@1",
    "agentwt-probe@1",
    "agentwt-remove@1",
    "agentwt-status@1",
    "agentwt-unplaced@1",
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
