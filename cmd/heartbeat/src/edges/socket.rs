//! The socket edge: the action ledger's single writer and the digest sender (concept D6, D15). One std thread takes
//! jobs in order. A subscription record is appended. A digest is claimed — appended and synced — then written to the
//! subscriber's messaging socket as an auth line plus one user frame, its outcome appended and reported back as
//! `Delivered`. A claim that cannot be written is never followed by a send; nothing is ever retried. A dissolved fleet
//! run is written to `archive/<instance>.json` (temp, fsync, rename) before its `dissolved` line is appended durably.
//! The runtime checker's digests take the same claim → send → outcome path (reported as `RuntimeDelivered`); a
//! runtime move's claim is appended synced and reported as `ClaimRecorded`, or `ClaimFailed` when the append fails —
//! no move goes live before its claim is on disk. A landing grant to a waiter appends its
//! `land_granted` record synced, then sends the grant line once; its send outcome is only logged.

use std::fs::{File, OpenOptions};
use std::io::Write as _;
use std::path::Path;
use std::sync::mpsc::Receiver;
use std::time::Duration;

use anyhow::Context as _;
use jiff::Timestamp;
use tokio::sync::mpsc;

use crate::action::{Digest, GrantNotice, RuntimeDigest};
use crate::action_ledger::Record;
use crate::edges::sensor;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{Outcome, SensorName, SensorState};
use crate::ids::{MessagingToken, SessionId};
use crate::log;
use crate::protocol::{ACTIONS_FILE, ARCHIVE_DIR};
use crate::runtime::FindingId;
use crate::wire::FleetArchive;

/// One unit of work, in core order.
#[derive(Debug)]
pub enum Job {
    /// Append one ledger record.
    Record(Record),
    /// Claim and send a digest.
    Deliver(Digest),
    /// Write a dissolved run's archive file, then append its `dissolved` record.
    Dissolve {
        /// The run, in full.
        archive: Box<FleetArchive>,
        /// The `dissolved` record.
        record: Record,
    },
    /// Claim and send a runtime digest to the runtime subscriber.
    DeliverRuntime(RuntimeDigest),
    /// Append a move's claim synced, then report `ClaimRecorded` (or `ClaimFailed`).
    Claim {
        /// The finding the move answers.
        finding: FindingId,
        /// The `runtime_claimed` record.
        record: Record,
    },
    /// Append a landing grant's record synced, then tell the granted session once.
    Grant(GrantNotice),
}

/// Starts the edge thread; `note` is a startup problem of the ledger (unreadable lines, failed read) to report; a
/// socket write may block `write_timeout` (`socket_write_timeout`) before its digest counts as unconfirmed.
pub fn start(
    core: mpsc::Sender<Input>,
    inbox: Receiver<Job>,
    state_dir: &Path,
    note: Option<String>,
    write_timeout: Duration,
) -> crate::Result<()> {
    let state_dir = state_dir.to_path_buf();
    std::thread::Builder::new()
        .name("socket".to_owned())
        .spawn(move || run(&core, &inbox, &state_dir, note, write_timeout))
        .map_err(|err| crate::Error::Task {
            task: "socket thread",
            detail: err.to_string(),
        })?;
    Ok(())
}

fn degrade(core: &mpsc::Sender<Input>, detail: String) {
    log::line(format_args!("action ledger: {detail}"));
    let _ = core.blocking_send(sensor(
        SensorName::ActionLedger,
        SensorState::Degraded,
        Some(detail),
    ));
}

fn run(
    core: &mpsc::Sender<Input>,
    inbox: &Receiver<Job>,
    state_dir: &Path,
    note: Option<String>,
    write_timeout: Duration,
) {
    let path = state_dir.join(ACTIONS_FILE);
    let mut ledger = match open(state_dir, &path) {
        Ok(file) => {
            let state = if note.is_some() {
                SensorState::Degraded
            } else {
                SensorState::Ok
            };
            let _ = core.blocking_send(sensor(SensorName::ActionLedger, state, note));
            Some(file)
        }
        Err(err) => {
            degrade(core, format!("{}: {err:#}", path.display()));
            None
        }
    };
    for job in inbox {
        match job {
            Job::Record(record) => {
                if let Err(err) = append(ledger.as_mut(), record, Timestamp::now(), false) {
                    degrade(core, format!("record: {err:#}"));
                }
            }
            Job::Deliver(digest) => {
                let (outcome, problem) = deliver(ledger.as_mut(), &digest, write_timeout);
                if let Some(problem) = problem {
                    degrade(core, problem);
                }
                let delivered = Event::Delivered {
                    session: digest.session,
                    digest: digest.id,
                    outcome,
                    at: Timestamp::now(),
                };
                let _ = core.blocking_send(Input::Event(delivered));
            }
            Job::Dissolve { archive, record } => {
                if let Err(err) = dissolve(ledger.as_mut(), state_dir, &archive, record) {
                    degrade(core, format!("archive {}: {err:#}", archive.instance));
                }
            }
            Job::DeliverRuntime(digest) => {
                let (outcome, problem) = deliver_runtime(ledger.as_mut(), &digest, write_timeout);
                if let Some(problem) = problem {
                    degrade(core, problem);
                }
                let delivered = Event::RuntimeDelivered {
                    digest: digest.id,
                    outcome,
                    at: Timestamp::now(),
                };
                let _ = core.blocking_send(Input::Event(delivered));
            }
            Job::Claim { finding, record } => {
                let settled = match append(ledger.as_mut(), record, Timestamp::now(), true) {
                    Ok(()) => Event::ClaimRecorded {
                        finding,
                        at: Timestamp::now(),
                    },
                    Err(err) => {
                        degrade(core, format!("claim {}: {err:#}", finding.0));
                        Event::ClaimFailed {
                            finding,
                            at: Timestamp::now(),
                        }
                    }
                };
                let _ = core.blocking_send(Input::Event(settled));
            }
            Job::Grant(notice) => {
                if let Some(problem) = grant(ledger.as_mut(), &notice, write_timeout) {
                    degrade(core, problem);
                }
            }
        }
    }
}

/// A landing grant: the `land_granted` record is the claim — appended and synced — then one message goes to the
/// granted session's socket. A failed append sends nothing and is returned as the ledger problem to report. The
/// send's outcome is logged and never reported back: the grant stands either way. Without a socket only the record
/// is written.
fn grant(ledger: Option<&mut File>, notice: &GrantNotice, timeout: Duration) -> Option<String> {
    if let Err(err) = append(ledger, notice.record.clone(), Timestamp::now(), true) {
        return Some(format!("grant {}: {err:#}", notice.session));
    }
    let Some(socket) = notice.socket.as_deref() else {
        log::line(format_args!(
            "grant {}: no messaging socket registered, not announced",
            notice.session
        ));
        return None;
    };
    match send(
        socket,
        &frame(&notice.token, &notice.session, &notice.text),
        timeout,
    ) {
        Outcome::Sent => {}
        Outcome::Refused(detail) | Outcome::Unconfirmed(detail) | Outcome::Unclaimed(detail) => {
            log::line(format_args!("grant {}: {detail}", notice.session));
        }
    }
    None
}

/// A runtime digest: claim (synced), send, record the outcome — the fleet digest's sequence. The second value is a
/// ledger problem to report.
fn deliver_runtime(
    mut ledger: Option<&mut File>,
    digest: &RuntimeDigest,
    timeout: Duration,
) -> (Outcome, Option<String>) {
    let claim = Record::RuntimeDigestClaimed {
        digest: digest.id,
        session_id: digest.session.clone(),
        lines: digest.lines,
    };
    if let Err(err) = append(ledger.as_deref_mut(), claim, Timestamp::now(), true) {
        let detail = format!("runtime claim {}: {err:#}", digest.id.label());
        return (Outcome::Unclaimed(detail.clone()), Some(detail));
    }
    let outcome = send(
        &digest.socket,
        &frame(&digest.token, &digest.session, &digest.text),
        timeout,
    );
    let problem = Record::runtime_outcome(digest.id, digest.session.clone(), &outcome)
        .and_then(|record| append(ledger, record, Timestamp::now(), false).err())
        .map(|err| format!("runtime outcome {}: {err:#}", digest.id.label()));
    (outcome, problem)
}

/// Writes the archive file (temp, fsync, rename, 0600), then appends the dissolved line durably; a failed archive
/// leaves no ledger line, so the next start replays the run and dissolves it again.
fn dissolve(
    ledger: Option<&mut File>,
    state_dir: &Path,
    archive: &FleetArchive,
    record: Record,
) -> anyhow::Result<()> {
    let dir = state_dir.join(ARCHIVE_DIR);
    create_private_dir(&dir).context("archive dir")?;
    let path = dir.join(format!("{}.json", archive.instance));
    let temp = dir.join(format!(".{}.json.tmp", archive.instance));
    let bytes = serde_json::to_vec_pretty(archive).context("encode archive")?;
    // A temp file left by a crash between write and rename is stale: the run replays and is written again.
    match std::fs::remove_file(&temp) {
        Ok(()) => {}
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {}
        Err(err) => return Err(err).context("remove stale temp"),
    }
    write_private(&temp, &bytes).context("write archive")?;
    std::fs::rename(&temp, &path).context("rename archive")?;
    append(ledger, record, Timestamp::now(), true)
}

/// Claim (synced), send, record the outcome. The second value is a ledger problem to report.
fn deliver(
    mut ledger: Option<&mut File>,
    digest: &Digest,
    timeout: Duration,
) -> (Outcome, Option<String>) {
    let claim = Record::Claimed {
        digest: digest.id,
        session_id: digest.session.clone(),
        fleet: digest.fleet.clone(),
        items: digest.items,
        keys: digest.keys.clone(),
    };
    if let Err(err) = append(ledger.as_deref_mut(), claim, Timestamp::now(), true) {
        let detail = format!("claim {}: {err:#}", digest.id.label());
        return (Outcome::Unclaimed(detail.clone()), Some(detail));
    }
    let outcome = send(
        &digest.socket,
        &frame(&digest.token, &digest.session, &digest.text),
        timeout,
    );
    let problem = Record::outcome(digest.id, digest.session.clone(), &outcome)
        .and_then(|record| append(ledger, record, Timestamp::now(), false).err())
        .map(|err| format!("outcome {}: {err:#}", digest.id.label()));
    (outcome, problem)
}

fn append(
    ledger: Option<&mut File>,
    record: Record,
    at: Timestamp,
    durable: bool,
) -> anyhow::Result<()> {
    let file = ledger.context("the action ledger is not open")?;
    file.write_all(&record.encode(at)?).context("append")?;
    if durable {
        file.sync_data().context("sync")?;
    }
    Ok(())
}

/// The frame bytes: auth line, user line (concept D6; protocol of Claude Code's messaging socket).
pub(crate) fn frame(token: &MessagingToken, session: &SessionId, text: &str) -> Vec<u8> {
    let auth = serde_json::json!({ "type": "auth", "token": token.0 });
    let user = serde_json::json!({
        "type": "user",
        "session_id": session.0,
        "priority": "next",
        "message": { "role": "user", "content": text },
    });
    let mut bytes = auth.to_string().into_bytes();
    bytes.push(b'\n');
    bytes.extend(user.to_string().into_bytes());
    bytes.push(b'\n');
    bytes
}

/// Creates the heartbeat dir `0700` when absent and always tightens it to `0700` (P2's sync creates it, but an existing
/// dir keeps its mode there); opens the ledger for append, `0600` at creation and tightened to `0600` (D14).
#[cfg(unix)]
fn open(state_dir: &Path, path: &Path) -> anyhow::Result<File> {
    use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
    create_private_dir(state_dir).context("heartbeat dir")?;
    let file = OpenOptions::new()
        .create(true)
        .append(true)
        .mode(0o600)
        .open(path)
        .context("open")?;
    file.set_permissions(std::fs::Permissions::from_mode(0o600))
        .context("ledger mode")?;
    Ok(file)
}

/// Creates `dir` `0700` when absent and always tightens it to `0700`.
#[cfg(unix)]
fn create_private_dir(dir: &Path) -> anyhow::Result<()> {
    use std::os::unix::fs::{DirBuilderExt as _, PermissionsExt as _};
    std::fs::DirBuilder::new()
        .recursive(true)
        .mode(0o700)
        .create(dir)
        .context("create")?;
    std::fs::set_permissions(dir, std::fs::Permissions::from_mode(0o700)).context("mode")
}

#[cfg(not(unix))]
fn create_private_dir(dir: &Path) -> anyhow::Result<()> {
    std::fs::create_dir_all(dir).context("create")
}

/// Creates `path` (`0600`, never over an existing file), writes `bytes` and syncs them.
fn write_private(path: &Path, bytes: &[u8]) -> anyhow::Result<()> {
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt as _;
        options.mode(0o600);
    }
    let mut file = options.open(path).context("create")?;
    file.write_all(bytes).context("write")?;
    file.sync_all().context("sync")
}

#[cfg(not(unix))]
fn open(state_dir: &Path, path: &Path) -> anyhow::Result<File> {
    std::fs::create_dir_all(state_dir).context("heartbeat dir")?;
    OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .context("open")
}

/// One connection per digest; the target must be a socket, never a symlink; nothing is read back; a write blocking
/// past `timeout` is unconfirmed.
#[cfg(unix)]
fn send(socket: &Path, bytes: &[u8], timeout: Duration) -> Outcome {
    use std::net::Shutdown;
    use std::os::unix::fs::FileTypeExt as _;
    use std::os::unix::net::UnixStream;

    match std::fs::symlink_metadata(socket) {
        Ok(meta) if meta.file_type().is_socket() => {}
        Ok(_) => return Outcome::Refused(format!("{} is not a socket", socket.display())),
        Err(err) => return Outcome::Refused(format!("{}: {err}", socket.display())),
    }
    let mut stream = match UnixStream::connect(socket) {
        Ok(stream) => stream,
        Err(err) => return Outcome::Refused(format!("connect {}: {err}", socket.display())),
    };
    if let Err(err) = stream.set_write_timeout(Some(timeout)) {
        return Outcome::Refused(format!("write timeout: {err}"));
    }
    if let Err(err) = stream.write_all(bytes) {
        return Outcome::Unconfirmed(format!("write: {err}"));
    }
    match stream.shutdown(Shutdown::Write) {
        Ok(()) => Outcome::Sent,
        Err(err) => Outcome::Unconfirmed(format!("shutdown: {err}")),
    }
}

#[cfg(not(unix))]
fn send(_socket: &Path, _bytes: &[u8], _timeout: Duration) -> Outcome {
    Outcome::Refused("messaging sockets are not supported on windows".to_owned())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn frame_is_two_json_lines() {
        let bytes = frame(
            &MessagingToken("t0k".to_owned()),
            &SessionId("s1".to_owned()),
            "line one\nline two",
        );
        let text = String::from_utf8(bytes).unwrap_or_default();
        let lines: Vec<&str> = text.lines().collect();
        assert_eq!(
            lines.len(),
            2,
            "a newline in the text never splits the frame"
        );
        assert_eq!(lines[0], r#"{"token":"t0k","type":"auth"}"#);
        let user: serde_json::Value = serde_json::from_str(lines[1]).unwrap_or_default();
        assert_eq!(user["session_id"], "s1");
        assert_eq!(user["priority"], "next");
        assert_eq!(user["message"]["content"], "line one\nline two");
    }
}
