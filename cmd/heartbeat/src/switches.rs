//! The session switch reader. The tool-policy gate keeps one record per session at
//! `<home>/tool-policy/state/<session>.json` (Go: `internal/sessionstate`); its `switches.off` names the features a
//! human turned off with `#disable`. Heartbeat reads one feature, `hb` (typed as `hb` or `heartbeat`): off means every
//! hook of that session is answered `{}` after the heart attack. A missing or unreadable file and an undecodable
//! record count as on — the switch fails open — and a decode failure is logged.

use std::path::Path;

use serde::Deserialize;

use crate::ids::SessionId;
use crate::log;
use crate::protocol::FEATURE;

/// The slice of the record heartbeat decodes; every other key is ignored.
#[derive(Deserialize)]
struct Record {
    #[serde(default)]
    switches: Switches,
}

#[derive(Default, Deserialize)]
struct Switches {
    #[serde(default)]
    off: Vec<String>,
}

/// Whether `session` switched heartbeat's answers off.
#[must_use]
pub fn is_off(home: &Path, session: &SessionId) -> bool {
    let path = home
        .join("tool-policy")
        .join("state")
        .join(format!("{session}.json"));
    let Ok(bytes) = std::fs::read(&path) else {
        return false;
    };
    match serde_json::from_slice::<Record>(&bytes) {
        Ok(record) => record.switches.off.iter().any(|feature| feature == FEATURE),
        Err(err) => {
            log::line(format_args!("switches: {}: {err}", path.display()));
            false
        }
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    fn write(home: &Path, session: &str, body: &str) {
        let dir = home.join("tool-policy").join("state");
        std::fs::create_dir_all(&dir).expect("state dir");
        std::fs::write(dir.join(format!("{session}.json")), body).expect("record");
    }

    #[test]
    fn reads_the_off_set() {
        let home = tempfile::tempdir().expect("home");
        let cases = [
            ("off", r#"{"switches":{"off":["hb","rgate:global"]}}"#, true),
            ("on", r#"{"switches":{"off":["rgate:global"]}}"#, false),
            (
                "long-spelling-is-not-the-feature",
                r#"{"switches":{"off":["heartbeat"]}}"#,
                false,
            ),
            (
                "no-switches-key",
                r#"{"settings_local":{"is_reverted":false,"last_appended":[]}}"#,
                false,
            ),
            ("garbage", "{", false),
        ];
        for (session, body, expected) in cases {
            write(home.path(), session, body);
            assert_eq!(
                is_off(home.path(), &SessionId(session.to_owned())),
                expected,
                "{session}"
            );
        }
        assert!(
            !is_off(home.path(), &SessionId("missing".to_owned())),
            "missing"
        );
    }
}
