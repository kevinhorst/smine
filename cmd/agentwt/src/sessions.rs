//! Session roster from the Claude pid registry and Codex rollouts, joined to worktrees by cwd.
//!
//! A Claude home contributes two directories of the same file shape: `<claude home>/sessions`, the harness
//! registry, and `<claude home>/smine/container-sessions`, written by the routine runner for container cells,
//! whose pid is the docker client's host pid. The second directory is optional; its absence is not a warning.
//!
//! Liveness is a process lookup; staleness is otherwise invisible in the file, so a dead pid's file is
//! reported with `alive: false` and loses to any live session on the same worktree.

use std::collections::HashMap;
use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use serde::Deserialize;
use sysinfo::{Pid, ProcessRefreshKind, ProcessesToUpdate, System};

use crate::error::Error;
use crate::gitstate::rfc3339_utc;
use crate::wire::{Session, SessionAgent};

/// Rollouts older than this are not scanned.
const CODEX_WINDOW: Duration = Duration::from_secs(7 * 24 * 3600);

/// Registrations the routine runner writes for container cells, under the Claude home; the pid is the docker
/// client's, a host process.
const CONTAINER_SESSIONS_DIR: &str = "smine/container-sessions";

/// One Claude session file, the fields this crate uses.
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ClaudeFile {
    pid: u32,
    session_id: String,
    cwd: PathBuf,
    #[serde(default)]
    entrypoint: Option<String>,
    #[serde(default)]
    host_session_id: Option<String>,
    #[serde(default)]
    name: Option<String>,
    #[serde(default)]
    status: Option<String>,
    #[serde(default)]
    waiting_for: Option<String>,
    #[serde(default)]
    updated_at: Option<i64>,
}

/// One rollout's first line.
#[derive(Debug, Clone, Deserialize)]
struct RolloutLine {
    #[serde(rename = "type")]
    kind: String,
    payload: RolloutMeta,
}

#[derive(Debug, Clone, Deserialize)]
struct RolloutMeta {
    id: String,
    cwd: PathBuf,
    #[serde(default)]
    originator: Option<String>,
    #[serde(default)]
    timestamp: Option<String>,
}

#[derive(Debug, Clone, Deserialize)]
struct AccountFile {
    #[serde(rename = "oauthAccount")]
    oauth_account: Option<OauthAccount>,
}

#[derive(Debug, Clone, Deserialize)]
struct OauthAccount {
    #[serde(rename = "accountUuid")]
    account_uuid: Option<String>,
}

/// A session with its cwd, before the join.
#[derive(Debug, Clone)]
pub struct Candidate {
    /// The session's working directory.
    pub cwd: PathBuf,
    /// The wire session.
    pub session: Session,
    /// Sort key: alive sessions first, then newest.
    sort_key: (bool, i64),
}

/// Reads every session of every Claude home and the recent Codex rollouts.
pub fn gather(
    claude_homes: &[PathBuf],
    codex_home: Option<&Path>,
    warnings: &mut Vec<String>,
) -> Vec<Candidate> {
    let mut candidates = Vec::new();
    for home in claude_homes {
        candidates.extend(read_claude_home(home, warnings));
    }
    if let Some(codex_home) = codex_home {
        candidates.extend(read_codex_home(codex_home, warnings));
    }
    candidates
}

fn read_claude_home(home: &Path, warnings: &mut Vec<String>) -> Vec<Candidate> {
    let account_uuid = read_account_uuid(home);
    let mut files = read_session_dir(&home.join("sessions"), warnings);
    let container_dir = home.join(CONTAINER_SESSIONS_DIR);
    if container_dir.is_dir() {
        files.extend(read_session_dir(&container_dir, warnings));
    }
    let alive = alive_pids(files.iter().map(|file| file.pid));
    files
        .into_iter()
        .map(|file| {
            let is_alive = alive.get(&file.pid).copied().unwrap_or(false);
            let updated_ms = file.updated_at.unwrap_or(0);
            let session = Session {
                agent: SessionAgent::Claude,
                id: file.session_id,
                pid: Some(file.pid),
                alive: Some(is_alive),
                status: file.status,
                waiting_for: file.waiting_for,
                entrypoint: file.entrypoint.unwrap_or_else(|| "unknown".to_owned()),
                name: file.name,
                host_session_id: file.host_session_id,
                updated_at: rfc3339_utc(updated_ms / 1000),
                account_uuid: account_uuid.clone(),
            };
            Candidate {
                cwd: file.cwd,
                session,
                sort_key: (is_alive, updated_ms),
            }
        })
        .collect()
}

/// Every parseable `*.json` session file of `dir`; an unreadable dir or file is a warning.
fn read_session_dir(dir: &Path, warnings: &mut Vec<String>) -> Vec<ClaudeFile> {
    let entries = match std::fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(source) => {
            warnings.push(Error::io(dir)(source).to_string());
            return Vec::new();
        }
    };
    let mut files = Vec::new();
    for entry in entries.flatten() {
        let path = entry.path();
        if path.extension().is_some_and(|ext| ext == "json") {
            let parsed = std::fs::read(&path)
                .map_err(Error::io(&path))
                .and_then(|bytes| Ok(serde_json::from_slice::<ClaudeFile>(&bytes)?));
            match parsed {
                Ok(file) => files.push(file),
                Err(err) => warnings.push(format!("{}: {err}", path.display())),
            }
        }
    }
    files
}

/// `<home>/.claude.json`, else `<home>/../.claude.json` (the default profile keeps it beside `.claude/`).
fn read_account_uuid(home: &Path) -> Option<String> {
    let mut paths = vec![home.join(".claude.json")];
    if let Some(parent) = home.parent() {
        paths.push(parent.join(".claude.json"));
    }
    paths.into_iter().find_map(|path| {
        let bytes = std::fs::read(path).ok()?;
        let file: AccountFile = serde_json::from_slice(&bytes).ok()?;
        file.oauth_account?.account_uuid
    })
}

/// One process-table refresh for all pids.
fn alive_pids(pids: impl Iterator<Item = u32>) -> HashMap<u32, bool> {
    let raw: Vec<u32> = pids.collect();
    let wanted: Vec<Pid> = raw.iter().map(|pid| Pid::from_u32(*pid)).collect();
    let mut system = System::new();
    system.refresh_processes_specifics(
        ProcessesToUpdate::Some(&wanted),
        true,
        ProcessRefreshKind::nothing(),
    );
    raw.into_iter()
        .map(|pid| (pid, system.process(Pid::from_u32(pid)).is_some()))
        .collect()
}

fn read_codex_home(home: &Path, warnings: &mut Vec<String>) -> Vec<Candidate> {
    let cutoff = SystemTime::now()
        .checked_sub(CODEX_WINDOW)
        .unwrap_or(UNIX_EPOCH);
    let mut out = Vec::new();
    let root = home.join("sessions");
    for path in rollout_files(&root, cutoff) {
        let meta = match read_first_line(&path) {
            Ok(Some(line)) if line.kind == "session_meta" => line.payload,
            Ok(_) => continue,
            Err(err) => {
                warnings.push(format!("{}: {err}", path.display()));
                continue;
            }
        };
        let started = meta.timestamp.clone().unwrap_or_default();
        let session = Session {
            agent: SessionAgent::Codex,
            id: meta.id,
            pid: None,
            alive: None,
            status: None,
            waiting_for: None,
            entrypoint: meta.originator.unwrap_or_else(|| "codex".to_owned()),
            name: None,
            host_session_id: None,
            updated_at: started,
            account_uuid: None,
        };
        let modified_secs = std::fs::metadata(&path)
            .and_then(|meta| meta.modified())
            .ok()
            .and_then(|time| time.duration_since(UNIX_EPOCH).ok())
            .map_or(0, |duration| i64::try_from(duration.as_secs()).unwrap_or(0));
        out.push(Candidate {
            cwd: meta.cwd,
            session,
            sort_key: (false, modified_secs),
        });
    }
    out
}

/// `sessions/YYYY/MM/DD/rollout-*.jsonl` newer than `cutoff`, three fixed directory levels — no recursion.
fn rollout_files(root: &Path, cutoff: SystemTime) -> Vec<PathBuf> {
    let mut files = Vec::new();
    let Ok(years) = std::fs::read_dir(root) else {
        return files;
    };
    for year in years.flatten() {
        let Ok(months) = std::fs::read_dir(year.path()) else {
            continue;
        };
        for month in months.flatten() {
            let Ok(days) = std::fs::read_dir(month.path()) else {
                continue;
            };
            for day in days.flatten() {
                let Ok(rollouts) = std::fs::read_dir(day.path()) else {
                    continue;
                };
                for rollout in rollouts.flatten() {
                    let path = rollout.path();
                    let recent = rollout
                        .metadata()
                        .and_then(|meta| meta.modified())
                        .is_ok_and(|time| time >= cutoff);
                    if recent && path.extension().is_some_and(|ext| ext == "jsonl") {
                        files.push(path);
                    }
                }
            }
        }
    }
    files
}

fn read_first_line(path: &Path) -> crate::Result<Option<RolloutLine>> {
    let file = std::fs::File::open(path).map_err(Error::io(path))?;
    let mut reader = BufReader::new(file);
    let mut line = Vec::new();
    reader
        .read_until(b'\n', &mut line)
        .map_err(Error::io(path))?;
    if line.is_empty() {
        return Ok(None);
    }
    Ok(Some(serde_json::from_slice(&line)?))
}

/// Attaches one session per worktree: a session belongs to the worktree that is the longest prefix of its cwd;
/// among several, alive first, then the newest. Pure: takes the gathered roster and the worktree paths.
#[must_use]
pub fn attach(candidates: Vec<Candidate>, worktrees: &[PathBuf]) -> HashMap<PathBuf, Session> {
    let canonical: Vec<(PathBuf, PathBuf)> = worktrees
        .iter()
        .map(|path| {
            (
                path.clone(),
                dunce::canonicalize(path).unwrap_or_else(|_| path.clone()),
            )
        })
        .collect();
    let mut best: HashMap<PathBuf, (Candidate, usize)> = HashMap::new();
    for candidate in candidates {
        let cwd = dunce::canonicalize(&candidate.cwd).unwrap_or_else(|_| candidate.cwd.clone());
        let Some((original, depth)) = canonical
            .iter()
            .filter(|(_, canon)| cwd.starts_with(canon))
            .map(|(original, canon)| (original.clone(), canon.components().count()))
            .max_by_key(|(_, depth)| *depth)
        else {
            continue;
        };
        let replace = best
            .get(&original)
            .is_none_or(|(current, _)| candidate.sort_key > current.sort_key);
        if replace {
            best.insert(original, (candidate, depth));
        }
    }
    best.into_iter()
        .map(|(path, (candidate, _))| (path, candidate.session))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn candidate(cwd: &str, alive: bool, updated: i64) -> Candidate {
        let session = Session {
            agent: SessionAgent::Claude,
            id: format!("{cwd}-{updated}"),
            pid: Some(1),
            alive: Some(alive),
            status: None,
            waiting_for: None,
            entrypoint: "test".to_owned(),
            name: None,
            host_session_id: None,
            updated_at: String::new(),
            account_uuid: None,
        };
        Candidate {
            cwd: PathBuf::from(cwd),
            session,
            sort_key: (alive, updated),
        }
    }

    #[test]
    fn longest_prefix_alive_newest_wins() {
        let worktrees = vec![PathBuf::from("/r"), PathBuf::from("/r/.claude/worktrees/a")];
        let roster = vec![
            candidate("/r/.claude/worktrees/a/sub", false, 5),
            candidate("/r/.claude/worktrees/a", true, 1),
            candidate("/r/docs", true, 9),
            candidate("/elsewhere", true, 9),
        ];
        let attached = attach(roster, &worktrees);
        assert_eq!(
            attached[&PathBuf::from("/r/.claude/worktrees/a")].id,
            "/r/.claude/worktrees/a-1"
        );
        assert_eq!(attached[&PathBuf::from("/r")].id, "/r/docs-9");
        assert_eq!(attached.len(), 2);
    }

    /// One roster case: session files under a fresh Claude home and the roster `gather` returns.
    struct RosterCase {
        label: &'static str,
        /// `(dir under the Claude home, file name, body)`.
        files: Vec<(&'static str, &'static str, String)>,
        container_dir: bool,
        /// `(session id, alive)`, sorted.
        want: Vec<(&'static str, bool)>,
        want_warnings: usize,
    }

    fn session_json(pid: u32, id: &str) -> String {
        format!(r#"{{"pid":{pid},"sessionId":"{id}","cwd":"/r"}}"#)
    }

    #[test]
    fn container_sessions_join_the_roster() -> std::io::Result<()> {
        // No process runs under this pid: macOS caps pids at 99998, Linux at 2^22.
        const DEAD_PID: u32 = 999_999_999;
        let live_pid = std::process::id();
        let cases = vec![
            RosterCase {
                label: "container-dir-missing",
                files: vec![("sessions", "a.json", session_json(live_pid, "a"))],
                container_dir: false,
                want: vec![("a", true)],
                want_warnings: 0,
            },
            RosterCase {
                label: "container-file-live-pid",
                files: vec![(
                    CONTAINER_SESSIONS_DIR,
                    "c.json",
                    session_json(live_pid, "c"),
                )],
                container_dir: true,
                want: vec![("c", true)],
                want_warnings: 0,
            },
            RosterCase {
                label: "container-file-dead-pid",
                files: vec![(
                    CONTAINER_SESSIONS_DIR,
                    "c.json",
                    session_json(DEAD_PID, "c"),
                )],
                container_dir: true,
                want: vec![("c", false)],
                want_warnings: 0,
            },
            RosterCase {
                label: "malformed-container-file",
                files: vec![(CONTAINER_SESSIONS_DIR, "c.json", "not json".to_owned())],
                container_dir: true,
                want: vec![],
                want_warnings: 1,
            },
            RosterCase {
                label: "both-dirs",
                files: vec![
                    ("sessions", "a.json", session_json(live_pid, "a")),
                    (
                        CONTAINER_SESSIONS_DIR,
                        "c.json",
                        session_json(DEAD_PID, "c"),
                    ),
                ],
                container_dir: true,
                want: vec![("a", true), ("c", false)],
                want_warnings: 0,
            },
        ];
        for RosterCase {
            label,
            files,
            container_dir,
            want,
            want_warnings,
        } in cases
        {
            let home = tempfile::tempdir()?;
            std::fs::create_dir_all(home.path().join("sessions"))?;
            if container_dir {
                std::fs::create_dir_all(home.path().join(CONTAINER_SESSIONS_DIR))?;
            }
            for (dir, name, body) in &files {
                std::fs::write(home.path().join(dir).join(name), body)?;
            }
            let mut warnings = Vec::new();
            let roster = gather(&[home.path().to_path_buf()], None, &mut warnings);
            let mut got: Vec<(String, bool)> = roster
                .into_iter()
                .map(|candidate| {
                    (
                        candidate.session.id,
                        candidate.session.alive.unwrap_or_default(),
                    )
                })
                .collect();
            got.sort();
            let want: Vec<(String, bool)> = want
                .into_iter()
                .map(|(id, alive)| (id.to_owned(), alive))
                .collect();
            assert_eq!(got, want, "{label}: roster");
            assert_eq!(
                warnings.len(),
                want_warnings,
                "{label}: warnings {warnings:?}"
            );
        }
        Ok(())
    }
}
