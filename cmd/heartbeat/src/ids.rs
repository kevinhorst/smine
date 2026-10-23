//! Domain identifiers; all are transparent on the wire.

use std::fmt;

use jiff::Timestamp;
use serde::{Deserialize, Serialize};

use crate::protocol::{LEGACY_NAME_BYTES, MEMBER_LETTERS};

/// A Claude Code session id: `session_id` in hooks and the ledger, `sessionId` in the registry.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct SessionId(pub String);

/// A subagent id: `agent_id` in hooks.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct AgentId(pub String);

/// A fleet worker's branch (`claude/<name>`): the key of a member the fleet's subscriber registered.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct WorkerBranch(pub String);

/// An operating-system process id: `pid` in the registry.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Pid(pub u32);

/// A fleet id, `[A-Za-z0-9-]+`: the fleet a session joins or subscribes to. Typed by the user, never assigned.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct FleetId(pub String);

impl FleetId {
    /// `text` as a fleet id: non-empty, at most `max_len` bytes (`max_id_len`: titles cap at that length, so a longer
    /// id cannot come from a title) of ASCII letters, digits and `-`.
    #[must_use]
    pub fn parse(text: &str, max_len: usize) -> Option<FleetId> {
        let valid = !text.is_empty()
            && text.len() <= max_len
            && text
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-');
        valid.then(|| FleetId(text.to_owned()))
    }
}

/// One fleet run: a UUID v7, minted at the edge when the run's first join or subscribe is admitted.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct InstanceId(pub uuid::Uuid);

impl InstanceId {
    /// The id of a run that predates `fleet_started` lines: a v7 from its first line's time and the name's first
    /// [`LEGACY_NAME_BYTES`] bytes, so every replay derives the same id.
    #[must_use]
    pub fn legacy(fleet: &FleetId, at: Timestamp) -> InstanceId {
        let mut name = [0u8; LEGACY_NAME_BYTES];
        for (slot, byte) in name.iter_mut().zip(fleet.0.bytes()) {
            *slot = byte;
        }
        let millis = u64::try_from(at.as_millisecond()).unwrap_or(0);
        InstanceId(uuid::Builder::from_unix_timestamp_millis(millis, &name).into_uuid())
    }
}

impl fmt::Display for InstanceId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// A fleet member's letter, assigned in first-seen order within one fleet run: `A` … `Z`, `AA`, ….
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(transparent)]
pub struct MemberId(pub String);

impl MemberId {
    /// The `index`-th letter (0-based), spreadsheet-column style.
    #[must_use]
    pub fn nth(index: usize) -> MemberId {
        let base = MEMBER_LETTERS.len();
        let mut rest = index + 1;
        let mut letters = Vec::new();
        while rest > 0 {
            letters.push(char::from(MEMBER_LETTERS[(rest - 1) % base]));
            rest = (rest - 1) / base;
        }
        MemberId(letters.into_iter().rev().collect())
    }

    /// The title and digest token: `<fleet>-M<letter>`.
    #[must_use]
    pub fn token(&self, fleet: &FleetId) -> String {
        format!("{}-M{}", fleet.0, self.0)
    }
}

/// A digest's sequence number; it continues across restarts from the action ledger.
#[derive(
    Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default, Serialize, Deserialize,
)]
#[serde(transparent)]
pub struct DigestId(pub u64);

impl DigestId {
    /// The display form, `d<n>`.
    #[must_use]
    pub fn label(self) -> String {
        format!("d{}", self.0)
    }
}

/// A messaging-socket token handed over by `heartbeat subscribe`. `Debug` never prints it.
#[derive(Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(transparent)]
pub struct MessagingToken(pub String);

impl fmt::Debug for MessagingToken {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("MessagingToken(<redacted>)")
    }
}

impl fmt::Display for SessionId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl fmt::Display for AgentId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl fmt::Display for Pid {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    #[test]
    fn fleet_id_parse_cases() {
        let max_len = crate::config::Caps::default().max_id_len();
        let long = "T".repeat(max_len + 1);
        let cases = [
            ("plain", "T1", true),
            ("dashed", "T1-MA", true),
            ("empty", "", false),
            ("space", "T 1", false),
            ("underscore", "T1_A", false),
            ("too-long", long.as_str(), false),
        ];
        for (label, text, valid) in cases {
            assert_eq!(FleetId::parse(text, max_len).is_some(), valid, "{label}");
        }
    }

    #[test]
    fn member_id_nth() {
        let cases = [
            (0, "A"),
            (25, "Z"),
            (26, "AA"),
            (51, "AZ"),
            (701, "ZZ"),
            (702, "AAA"),
        ];
        for (index, want) in cases {
            assert_eq!(MemberId::nth(index).0, want, "{index}");
        }
        assert_eq!(MemberId::nth(0).token(&FleetId("T1".to_owned())), "T1-MA");
    }

    #[test]
    fn instance_id_legacy() {
        let t1 = FleetId("T1".to_owned());
        let at = Timestamp::from_millisecond(1_790_000_000_000).expect("time");
        let id = InstanceId::legacy(&t1, at);
        assert_eq!(id, InstanceId::legacy(&t1, at), "same name and time");
        assert_eq!(id.0.get_version_num(), 7);
        assert_ne!(
            id,
            InstanceId::legacy(&FleetId("T2".to_owned()), at),
            "names differ"
        );
        let wire = serde_json::to_string(&id).expect("encode");
        assert_eq!(wire, format!("\"{id}\""), "bare string");
        let back: InstanceId = serde_json::from_str(&wire).expect("decode");
        assert_eq!(back, id);
    }

    #[test]
    fn token_debug_is_redacted() {
        let shown = format!("{:?}", MessagingToken("abc".to_owned()));
        assert!(!shown.contains("abc"), "{shown}");
    }
}
