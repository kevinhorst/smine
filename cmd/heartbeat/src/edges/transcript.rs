//! The transcript edge: reads a subagent's model from the head of its transcript for the heart attack
//! ([`crate::attack`]), plus what the spawn requested (the `agent-<agent_id>.meta.json` beside the transcript). One std
//! thread takes reads in order and reports each as `AgentModelRead`: the first
//! `assistant` line's `message.model`, or `None` while the transcript holds none or cannot be read — the core asks
//! again on the subagent's next tool hook. The transcript lies under the root transcript's stem:
//! `<stem>/subagents/agent-<agent_id>.jsonl`, or `<stem>/subagents/workflows/<run>/agent-<agent_id>.jsonl` for a
//! Workflow agent. Lines are read byte-wise; at most `transcript_max_lines` are scanned; synthetic
//! assistant lines (`<synthetic>`) are skipped.

use std::fs::File;
use std::io::{BufRead as _, BufReader};
use std::path::{Path, PathBuf};
use std::sync::mpsc::Receiver;

use jiff::Timestamp;
use serde::Deserialize;
use tokio::sync::mpsc;

use crate::action::ModelRead;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::Requested;
use crate::ids::AgentId;
use crate::log;

/// The fields of a transcript line the read needs.
#[derive(Deserialize)]
struct Line {
    #[serde(default, rename = "type")]
    kind: Option<String>,
    #[serde(default)]
    message: Option<Message>,
}

/// The `message` of a transcript line.
#[derive(Deserialize)]
struct Message {
    #[serde(default)]
    model: Option<String>,
}

/// The fields of a subagent's `agent-<id>.meta.json` the read needs.
#[derive(Deserialize)]
struct Meta {
    #[serde(default)]
    model: Option<String>,
}

/// A subagent's model as its transcript shows it, with what its spawn requested.
#[derive(Debug, PartialEq, Eq)]
struct Sighting {
    /// The first assistant line's `message.model`.
    model: String,
    /// What the spawn requested.
    requested: Requested,
}

/// Starts the edge thread; each read scans at most `max_lines` transcript lines for the first assistant line.
pub fn start(
    core: mpsc::Sender<Input>,
    inbox: Receiver<ModelRead>,
    max_lines: usize,
) -> crate::Result<()> {
    std::thread::Builder::new()
        .name("transcript".to_owned())
        .spawn(move || {
            for read in inbox {
                let (model, requested) = match model_of(&read.transcript, &read.agent, max_lines) {
                    Some(Sighting { model, requested }) => (Some(model), requested),
                    None => (None, Requested::Unknown),
                };
                let event = Event::AgentModelRead {
                    session: read.session,
                    agent: read.agent,
                    model,
                    requested,
                    at: Timestamp::now(),
                };
                let _ = core.blocking_send(Input::Event(event));
            }
        })
        .map_err(|err| crate::Error::Task {
            task: "transcript thread",
            detail: err.to_string(),
        })?;
    Ok(())
}

/// The subagent's model from the first candidate transcript that shows one, with the request beside that transcript;
/// I/O failures other than a missing file are logged.
fn model_of(root: &Path, agent: &AgentId, max_lines: usize) -> Option<Sighting> {
    candidates(root, agent).iter().find_map(|path| {
        let model = match first_model(path, max_lines) {
            Ok(model) => model,
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => None,
            Err(err) => {
                log::line(format_args!("transcript {}: {err}", path.display()));
                None
            }
        }?;
        Some(Sighting {
            model,
            requested: requested_of(path),
        })
    })
}

/// What the spawn requested, from the `agent-<id>.meta.json` beside `transcript`; read and parse failures other than a
/// missing file are logged.
fn requested_of(transcript: &Path) -> Requested {
    let path = transcript.with_extension("meta.json");
    let bytes = match std::fs::read(&path) {
        Ok(bytes) => bytes,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => return Requested::Unknown,
        Err(err) => {
            log::line(format_args!("meta {}: {err}", path.display()));
            return Requested::Unknown;
        }
    };
    match serde_json::from_slice::<Meta>(&bytes) {
        Ok(meta) => match meta.model.map(|model| model.trim().to_owned()) {
            Some(model) if !model.is_empty() && !model.eq_ignore_ascii_case("inherit") => {
                Requested::Named(model)
            }
            Some(_) | None => Requested::Inherited,
        },
        Err(err) => {
            log::line(format_args!("meta {}: {err}", path.display()));
            Requested::Unknown
        }
    }
}

/// Where the subagent's transcript may lie: directly under `subagents/`, else under each Workflow run dir.
fn candidates(root: &Path, agent: &AgentId) -> Vec<PathBuf> {
    let file = format!("agent-{}.jsonl", agent.0);
    let subagents = root.with_extension("").join("subagents");
    let mut paths = vec![subagents.join(&file)];
    if let Ok(runs) = std::fs::read_dir(subagents.join("workflows")) {
        paths.extend(
            runs.filter_map(Result::ok)
                .map(|run| run.path().join(&file)),
        );
    }
    paths
}

/// The first non-synthetic assistant line's `message.model` among the first `max_lines` lines; `None` when there
/// is none yet.
fn first_model(path: &Path, max_lines: usize) -> std::io::Result<Option<String>> {
    let mut reader = BufReader::new(File::open(path)?);
    let mut line = Vec::new();
    for _ in 0..max_lines {
        line.clear();
        if reader.read_until(b'\n', &mut line)? == 0 {
            break;
        }
        let Ok(parsed) = serde_json::from_slice::<Line>(&line) else {
            continue;
        };
        if parsed.kind.as_deref() != Some("assistant") {
            continue;
        }
        let model = parsed.message.and_then(|message| message.model);
        if let Some(model) = model.filter(|model| !model.starts_with('<')) {
            return Ok(Some(model));
        }
    }
    Ok(None)
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    /// The default `transcript_max_lines`.
    const MAX_LINES: usize = 64;
    const USER: &str = r#"{"type":"user","message":{"role":"user","content":"go"}}"#;
    const SYNTHETIC: &str =
        r#"{"type":"assistant","message":{"model":"<synthetic>","content":[]}}"#;
    const FABLE: &str =
        r#"{"type":"assistant","message":{"model":"claude-fable-5-1","content":[]}}"#;

    #[test]
    fn first_model_reads_the_first_real_assistant_line() {
        let dir = tempfile::tempdir().expect("tempdir");
        let cases: [(&str, &[&str], Option<&str>); 4] = [
            ("after-the-prompt", &[USER, FABLE], Some("claude-fable-5-1")),
            ("prompt-only", &[USER], None),
            (
                "synthetic-first",
                &[USER, SYNTHETIC, FABLE],
                Some("claude-fable-5-1"),
            ),
            ("partial-last-line", &[USER, r#"{"type":"assist"#], None),
        ];
        for (label, lines, want) in cases {
            let path = dir.path().join(format!("{label}.jsonl"));
            std::fs::write(&path, lines.join("\n")).expect("write");
            assert_eq!(
                first_model(&path, MAX_LINES).expect("read").as_deref(),
                want,
                "{label}"
            );
        }
    }

    #[test]
    fn model_of_finds_workflow_agents() {
        let dir = tempfile::tempdir().expect("tempdir");
        let root = dir.path().join("s.jsonl");
        let run = dir.path().join("s/subagents/workflows/wf_1");
        std::fs::create_dir_all(&run).expect("run dir");
        std::fs::write(run.join("agent-a1.jsonl"), [USER, FABLE].join("\n")).expect("write");
        let found = model_of(&root, &AgentId("a1".to_owned()), MAX_LINES);
        assert_eq!(
            found.map(|sighting| sighting.model).as_deref(),
            Some("claude-fable-5-1")
        );
        assert_eq!(
            model_of(&root, &AgentId("a2".to_owned()), MAX_LINES),
            None,
            "missing transcript"
        );
    }

    #[test]
    fn requested_is_read_beside_the_transcript() {
        let dir = tempfile::tempdir().expect("tempdir");
        let cases: [(&str, Option<&str>, Requested); 6] = [
            (
                "named",
                Some(r#"{"agentType":"Plan","model":"opus"}"#),
                Requested::Named("opus".to_owned()),
            ),
            (
                "no-model-key",
                Some(r#"{"agentType":"Plan"}"#),
                Requested::Inherited,
            ),
            (
                "inherit",
                Some(r#"{"model":"inherit"}"#),
                Requested::Inherited,
            ),
            ("empty", Some(r#"{"model":""}"#), Requested::Inherited),
            ("no-meta", None, Requested::Unknown),
            ("not-json", Some("{not json"), Requested::Unknown),
        ];
        for (label, meta, want) in cases {
            let transcript = dir.path().join(format!("agent-{label}.jsonl"));
            std::fs::write(&transcript, [USER, FABLE].join("\n")).expect("write");
            if let Some(meta) = meta {
                std::fs::write(dir.path().join(format!("agent-{label}.meta.json")), meta)
                    .expect("write meta");
            }
            assert_eq!(requested_of(&transcript), want, "{label}");
        }

        let root = dir.path().join("s.jsonl");
        let run = dir.path().join("s/subagents/workflows/wf_1");
        std::fs::create_dir_all(&run).expect("run dir");
        std::fs::write(run.join("agent-w1.jsonl"), [USER, FABLE].join("\n")).expect("write");
        std::fs::write(
            run.join("agent-w1.meta.json"),
            r#"{"agentType":"workflow","model":"haiku"}"#,
        )
        .expect("write meta");
        let found = model_of(&root, &AgentId("w1".to_owned()), MAX_LINES);
        assert_eq!(
            found.map(|sighting| sighting.requested),
            Some(Requested::Named("haiku".to_owned())),
            "workflow run dir"
        );
    }
}
