//! Registry edge: watches two dirs under the claude home (`FSEvents` through notify) — `sessions/`, the harness
//! registry, and `smine/container-sessions/`, the registrations the routine runner writes for container cells under
//! the docker client's host pid — and forwards `<pid>.json` changes in either as `RegistryUpserted` /
//! `RegistryRemoved`; a file whose session has no transcript under `<claude home>/projects/` is not forwarded. One
//! watcher and one startup scan pass cover both dirs. The startup scan runs after the watch is armed, so nothing
//! written in between is missed; a duplicate upsert is harmless. A file that fails to parse (a partial write) is skipped — the
//! write that completes it fires another event. `<pid>.<hash>.key` files are ignored. Watch errors degrade the
//! `registry` sensor. A successful startup scan ends with `RegistrySeeded`: from then on a joined session heartbeat does
//! not know is gone, and fleet runs may dissolve; a failed scan of either dir never seeds.
//!
//! [`start`] and the callback use `blocking_send`: call `start` from a blocking context, never a runtime worker.

use std::io;
use std::path::Path;

use jiff::Timestamp;
use notify::{RecommendedWatcher, RecursiveMode, Watcher as _};
use tokio::sync::mpsc::Sender;

use crate::edges::sensor;
use crate::engine::Input;
use crate::event::{Event, RegistryEntry};
use crate::fleet::{SensorName, SensorState};
use crate::ids::{Pid, SessionId};
use crate::protocol::{CONTAINER_SESSIONS_DIR, PROJECTS_DIR, SESSIONS_DIR};
use crate::{Error, Result};

/// Arms the watch on both registry dirs of `home`, scans each once, and returns the watcher; dropping it stops the
/// edge. The container dir is created when missing, so the watch arms before the first container cell.
pub fn start(home: &Path, inbox: Sender<Input>) -> Result<RecommendedWatcher> {
    let dirs = [home.join(SESSIONS_DIR), home.join(CONTAINER_SESSIONS_DIR)];
    std::fs::create_dir_all(&dirs[1]).map_err(|err| Error::Watch {
        path: dirs[1].clone(),
        detail: err.to_string(),
    })?;
    let scan_inbox = inbox.clone();
    let callback_inbox = inbox;
    let callback_home = home.to_path_buf();
    let callback_dirs = dirs.clone();
    let mut watcher =
        notify::recommended_watcher(move |result: notify::Result<notify::Event>| match result {
            Ok(event) if event.need_rescan() => {
                for dir in &callback_dirs {
                    scan(&callback_home, dir, &callback_inbox);
                }
            }
            Ok(event) => {
                for path in &event.paths {
                    forward(&callback_home, path, &callback_inbox);
                }
            }
            Err(err) => degrade(&callback_inbox, err.to_string()),
        })
        .map_err(|err| Error::Watch {
            path: home.to_path_buf(),
            detail: err.to_string(),
        })?;
    for dir in &dirs {
        watcher
            .watch(dir, RecursiveMode::NonRecursive)
            .map_err(|err| Error::Watch {
                path: dir.clone(),
                detail: err.to_string(),
            })?;
    }
    let scanned: Vec<bool> = dirs
        .iter()
        .map(|dir| scan(home, dir, &scan_inbox))
        .collect();
    if scanned.iter().all(|ok| *ok) {
        let seeded = Event::RegistrySeeded {
            at: Timestamp::now(),
        };
        let _ = scan_inbox.blocking_send(Input::Event(seeded));
    }
    Ok(watcher)
}

/// Forwards every registry file in `dir`; `false` when the dir could not be read.
fn scan(home: &Path, dir: &Path, inbox: &Sender<Input>) -> bool {
    let entries = match std::fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(err) => {
            degrade(inbox, format!("{}: {err}", dir.display()));
            return false;
        }
    };
    for entry in entries.flatten() {
        forward(home, &entry.path(), inbox);
    }
    true
}

fn forward(home: &Path, path: &Path, inbox: &Sender<Input>) {
    let Some(pid) = registry_pid(path) else {
        return;
    };
    let at = Timestamp::now();
    let event = match std::fs::read(path) {
        Ok(bytes) => match serde_json::from_slice::<RegistryEntry>(&bytes) {
            Ok(entry) if has_transcript(home, &entry.session_id) => {
                Event::RegistryUpserted { entry, at }
            }
            Ok(_) | Err(_) => return,
        },
        Err(err) if err.kind() == io::ErrorKind::NotFound => Event::RegistryRemoved { pid, at },
        Err(err) => {
            degrade(inbox, format!("{}: {err}", path.display()));
            return;
        }
    };
    let _ = inbox.blocking_send(Input::Event(event));
}

fn degrade(inbox: &Sender<Input>, detail: String) {
    let _ = inbox.blocking_send(sensor(
        SensorName::Registry,
        SensorState::Degraded,
        Some(detail),
    ));
}

/// Whether `<home>/projects/*/<session>.jsonl` exists. Claude Desktop registers placeholder sessions that exit
/// without a transcript; they are not sessions and never enter the fleet.
fn has_transcript(home: &Path, session: &SessionId) -> bool {
    let transcript = format!("{session}.jsonl");
    std::fs::read_dir(home.join(PROJECTS_DIR)).is_ok_and(|projects| {
        projects
            .flatten()
            .any(|project| project.path().join(&transcript).is_file())
    })
}

/// `<digits>.json` → its pid; anything else → `None`.
fn registry_pid(path: &Path) -> Option<Pid> {
    let name = path.file_name()?.to_str()?;
    name.strip_suffix(".json")?.parse::<u32>().ok().map(Pid)
}

#[cfg(test)]
#[allow(clippy::expect_used)]
mod tests {
    use super::*;

    #[test]
    fn registry_pid_accepts_only_pid_json() {
        let cases = [
            ("pid-json", "/h/sessions/42316.json", Some(Pid(42316))),
            ("key-file", "/h/sessions/42316.ab12.key", None),
            ("non-numeric", "/h/sessions/roster.json", None),
            ("directory", "/h/sessions", None),
            (
                "container-pid-json",
                "/h/smine/container-sessions/51807.json",
                Some(Pid(51807)),
            ),
        ];
        for (label, path, expected) in cases {
            assert_eq!(registry_pid(Path::new(path)), expected, "{label}");
        }
    }

    #[test]
    fn has_transcript_reads_projects_under_home() {
        const SESSION: &str = "0d3c9a5e-1111-2222-3333-444455556666";
        let cases = [
            ("transcript-present", Some(SESSION), true),
            ("transcript-absent", Some("other-session"), false),
            ("projects-dir-missing", None, false),
        ];
        for (label, transcript, expected) in cases {
            let home = tempfile::tempdir().expect("tempdir");
            if let Some(name) = transcript {
                let project = home.path().join(PROJECTS_DIR).join("-Users-x-repo");
                std::fs::create_dir_all(&project).expect("project dir");
                std::fs::write(project.join(format!("{name}.jsonl")), b"{}\n").expect("transcript");
            }
            let session = SessionId(SESSION.to_owned());
            assert_eq!(has_transcript(home.path(), &session), expected, "{label}");
        }
    }
}
