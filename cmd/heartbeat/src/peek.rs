//! peek's `session_events` result and `/healthz` identity, the fields heartbeat reads (peek-mcp
//! `session/event.go`, `cmd/start.go`), decoded from `structuredContent`, and their projection into the
//! [`SpawnTree`] the engine applies. Pure: the peek edge does the I/O.

use std::collections::{BTreeMap, BTreeSet};
use std::path::Path;

use jiff::Timestamp;
use serde::Deserialize;

use crate::ids::{AgentId, SessionId};
use crate::protocol::ASYNC_LAUNCH_ACK;

/// `GET /healthz`.
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct Identity {
    /// The Claude home this peek reads.
    #[serde(rename = "claudeHome")]
    pub claude_home: String,
    /// The initial session load finished.
    #[serde(default)]
    pub ready: bool,
    /// peek's version.
    #[serde(default)]
    pub version: String,
}

impl Identity {
    /// Whether heartbeat may use this peek: same (canonical) Claude home, initial load done. `Err` is the sensor detail.
    pub fn adopt(&self, canonical_home: &Path, canonical_theirs: &Path) -> Result<(), String> {
        if canonical_theirs != canonical_home {
            return Err(format!("peek {} serves {}", self.version, self.claude_home));
        }
        if !self.ready {
            return Err("peek is warming up".to_owned());
        }
        Ok(())
    }
}

/// `session_events` with `json: true` → `structuredContent`.
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct SessionEvents {
    /// The session's event buffer, oldest first.
    #[serde(default)]
    pub events: Vec<PeekEvent>,
}

/// One buffered event.
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct PeekEvent {
    /// Event kind, e.g. `subagent_spawned`.
    pub kind: String,
    /// The subagent payload of `subagent_*` kinds.
    #[serde(default)]
    pub subagent: Option<PeekSubagent>,
    /// The task payload of `task_completed` (peek ≥ 1.2.9); absent from older peeks.
    #[serde(default)]
    pub task: Option<PeekTask>,
    /// RFC 3339 with offset.
    #[serde(default)]
    pub timestamp: Option<String>,
}

/// The subagent payload.
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct PeekSubagent {
    /// Subagent id, as hooks send it in `agent_id`.
    pub agent_id: AgentId,
    /// Subagent type, e.g. `Explore`.
    #[serde(default)]
    pub agent_type: Option<String>,
    /// The Agent tool's result text (`subagent_result` only).
    #[serde(default)]
    pub content: Option<String>,
}

/// The task payload: a background task's completion notification (`queue-operation` `enqueue` in the transcript).
#[derive(Debug, Clone, PartialEq, Eq, Deserialize)]
pub struct PeekTask {
    /// The task id; for a background Agent-tool subagent, its agent id. Other background tasks carry ids no spawn
    /// record has.
    pub task_id: AgentId,
    /// Terminal status: `completed`, `failed`, `killed`, … — every one ends the task.
    #[serde(default)]
    pub status: Option<String>,
}

/// `session_events` of one subscriber, as the engine applies it: the subagents its buffer shows.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SpawnTree {
    /// The subscriber session.
    pub session: SessionId,
    /// Its subagents, by id.
    pub subagents: Vec<Spawned>,
}

/// One subagent from `subagent_spawned`, closed by a `subagent_result` that is not a background launch ack, or by a
/// `task_completed` naming it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Spawned {
    /// Subagent id.
    pub id: AgentId,
    /// Subagent type.
    pub agent_type: Option<String>,
    /// When it was spawned, when the timestamp parsed.
    pub at: Option<Timestamp>,
    /// A foreground result or a background completion closed it.
    pub finished: bool,
}

impl SessionEvents {
    /// The spawn tree `session`'s buffer shows. Closing events may precede the spawn record in the buffer (a
    /// background agent's launch ack is stamped before the agent's own first line; a completion may come from another
    /// transcript file), so closings are matched after the scan. A `task_completed` whose id no spawn record carries
    /// (a background Bash command, a monitor) closes nothing.
    #[must_use]
    pub fn spawn_tree(self, session: SessionId) -> SpawnTree {
        let mut subagents: BTreeMap<AgentId, Spawned> = BTreeMap::new();
        let mut finished: BTreeSet<AgentId> = BTreeSet::new();
        for event in self.events {
            match (event.kind.as_str(), event.subagent, event.task) {
                ("subagent_spawned", Some(sub), _) => {
                    let at = event
                        .timestamp
                        .as_deref()
                        .and_then(|text| text.parse::<Timestamp>().ok());
                    subagents.entry(sub.agent_id.clone()).or_insert(Spawned {
                        id: sub.agent_id,
                        agent_type: sub.agent_type,
                        at,
                        finished: false,
                    });
                }
                ("subagent_result", Some(sub), _) => {
                    let launch_ack = sub
                        .content
                        .as_deref()
                        .is_some_and(|text| text.starts_with(ASYNC_LAUNCH_ACK));
                    if !launch_ack {
                        finished.insert(sub.agent_id);
                    }
                }
                ("task_completed", _, Some(task)) => {
                    finished.insert(task.task_id);
                }
                // peek's other event kinds (skills, plans, permissions) and payload-less events carry no spawn-tree
                // fact.
                _ => {}
            }
        }
        for (id, spawned) in &mut subagents {
            spawned.finished = finished.contains(id);
        }
        SpawnTree {
            session,
            subagents: subagents.into_values().collect(),
        }
    }
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    const FIXTURE: &str = include_str!("../tests/fixtures/peek-session-events.json");

    #[test]
    fn session_events_fixture_spawn_tree() {
        let events: SessionEvents = serde_json::from_str(FIXTURE).expect("fixture");
        let tree = events.spawn_tree(SessionId("o".to_owned()));
        assert_eq!(tree.session, SessionId("o".to_owned()));
        let by_id = |id: &str| {
            tree.subagents
                .iter()
                .find(|spawned| spawned.id.0 == id)
                .unwrap_or_else(|| panic!("{id} spawned"))
        };
        assert_eq!(tree.subagents.len(), 3, "a non-agent task adds no member");
        assert!(
            by_id("abcf172b9e819cc00").finished,
            "task_completed closes it, though listed before its spawn record"
        );
        assert!(
            !by_id("ac4decf3b5ce93432").finished,
            "a launch ack alone is no completion"
        );
        let foreground = by_id("a97dc3464fb16fa00");
        assert!(foreground.finished);
        assert_eq!(foreground.agent_type.as_deref(), Some("Explore"));
        let at = foreground.at.expect("offset + nanosecond timestamp parses");
        assert_eq!(at.to_string(), "2026-09-27T16:11:17.64620027Z");
    }

    #[test]
    fn task_completed_cases() {
        let event = |kind: &str, body: serde_json::Value| {
            let mut event =
                serde_json::json!({ "kind": kind, "timestamp": "2026-09-27T16:00:00Z" });
            if let (Some(object), Some(extra)) = (event.as_object_mut(), body.as_object()) {
                object.extend(extra.clone());
            }
            event
        };
        let spawned = event(
            "subagent_spawned",
            serde_json::json!({ "subagent": { "agent_id": "a1", "agent_type": "Explore" } }),
        );
        let completed = |status: &str| {
            event(
                "task_completed",
                serde_json::json!({ "task": { "task_id": "a1", "status": status } }),
            )
        };
        let cases = [
            (
                "completion-after-spawn",
                vec![spawned.clone(), completed("completed")],
                true,
            ),
            (
                "completion-before-spawn",
                vec![completed("completed"), spawned.clone()],
                true,
            ),
            (
                "failed-is-terminal",
                vec![spawned.clone(), completed("failed")],
                true,
            ),
            (
                "killed-is-terminal",
                vec![spawned.clone(), completed("killed")],
                true,
            ),
            (
                "payload-less-task-event-ignored",
                vec![
                    spawned.clone(),
                    event("task_completed", serde_json::json!({})),
                ],
                false,
            ),
            ("no-completion", vec![spawned], false),
        ];
        for (label, events, want) in cases {
            let parsed: SessionEvents =
                serde_json::from_value(serde_json::json!({ "events": events })).expect(label);
            let tree = parsed.spawn_tree(SessionId("o".to_owned()));
            assert_eq!(tree.subagents.len(), 1, "{label}");
            assert_eq!(tree.subagents[0].finished, want, "{label}");
        }
    }

    #[test]
    fn adopt_cases() {
        let identity = |ready| Identity {
            claude_home: "/x".to_owned(),
            ready,
            version: "1.2.8".to_owned(),
        };
        let home = Path::new("/h");
        let cases = [
            ("same-home", identity(true), Path::new("/h"), Ok(())),
            (
                "other-home",
                identity(true),
                Path::new("/x"),
                Err("peek 1.2.8 serves /x".to_owned()),
            ),
            (
                "not-ready",
                identity(false),
                Path::new("/h"),
                Err("peek is warming up".to_owned()),
            ),
        ];
        for (label, identity, theirs, want) in cases {
            assert_eq!(identity.adopt(home, theirs), want, "{label}");
        }
    }
}
