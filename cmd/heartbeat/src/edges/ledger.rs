//! Denial-ledger edge: tails `<claude home>/tool-policy/denials.jsonl`, which toolpolicy appends one JSON line per
//! denial to (cmd/toolpolicy/ledger.go:56). A notify watch on the ledger's directory only signals "changed"; one
//! reader thread owns the cursor and forwards each complete new line as `Denied`, so no state is shared.
//! Startup reads the last `ledger_tail_window` bytes (dropping the first, possibly cut, line); `apply`'s replay guard
//! (D7) keeps that history from re-blocking restarted sessions. A file shorter than the cursor restarts at 0.
//! Read errors and unparseable lines degrade the `ledger` sensor.

use std::fs::File;
use std::io::{self, Read as _, Seek as _, SeekFrom};
use std::path::{Path, PathBuf};
use std::sync::mpsc;

use notify::{RecommendedWatcher, RecursiveMode, Watcher as _};
use tokio::sync::mpsc::Sender;

use crate::edges::sensor;
use crate::engine::Input;
use crate::event::{Denial, Event};
use crate::fleet::{SensorName, SensorState};
use crate::{Error, Result};

/// The reader's cursor over the ledger file.
struct Tail {
    path: PathBuf,
    offset: u64,
    partial: Vec<u8>,
    is_first_read: bool,
    /// How much ledger history the startup read covers.
    window: u64,
}

/// Arms the watch, starts the reader thread, returns the watcher; dropping it stops the edge. The startup read covers
/// the last `tail_window` bytes (`ledger_tail_window`).
pub fn start(path: &Path, inbox: Sender<Input>, tail_window: u64) -> Result<RecommendedWatcher> {
    let dir = path.parent().unwrap_or_else(|| Path::new("."));
    std::fs::create_dir_all(dir).map_err(Error::io(dir))?;
    let (wake, wakeups) = mpsc::channel::<()>();
    let file_name = path.file_name().map(std::ffi::OsStr::to_owned);
    let mut watcher = notify::recommended_watcher(move |result: notify::Result<notify::Event>| {
        if let Ok(event) = result
            && (event.need_rescan()
                || event
                    .paths
                    .iter()
                    .any(|p| p.file_name() == file_name.as_deref()))
        {
            let _ = wake.send(());
        }
    })
    .map_err(|err| Error::Watch {
        path: dir.to_path_buf(),
        detail: err.to_string(),
    })?;
    watcher
        .watch(dir, RecursiveMode::NonRecursive)
        .map_err(|err| Error::Watch {
            path: dir.to_path_buf(),
            detail: err.to_string(),
        })?;
    let mut tail = Tail {
        path: path.to_path_buf(),
        offset: 0,
        partial: Vec::new(),
        is_first_read: true,
        window: tail_window,
    };
    std::thread::Builder::new()
        .name("ledger".to_owned())
        .spawn(move || {
            tail.forward_new(&inbox);
            while wakeups.recv().is_ok() {
                while wakeups.try_recv().is_ok() {}
                tail.forward_new(&inbox);
            }
        })
        .map_err(|err| Error::Task {
            task: "ledger thread",
            detail: err.to_string(),
        })?;
    Ok(watcher)
}

impl Tail {
    fn forward_new(&mut self, inbox: &Sender<Input>) {
        let lines = match self.read_new() {
            Ok(lines) => lines,
            Err(err) => {
                degrade(inbox, err.to_string());
                return;
            }
        };
        for line in lines {
            match Denial::parse(&line) {
                Ok(denial) => {
                    if inbox
                        .blocking_send(Input::Event(Event::Denied(denial)))
                        .is_err()
                    {
                        return;
                    }
                }
                Err(err) => degrade(inbox, format!("ledger line skipped: {err}")),
            }
        }
    }

    /// Complete lines appended since the cursor.
    fn read_new(&mut self) -> Result<Vec<Vec<u8>>> {
        let mut file = match File::open(&self.path) {
            Ok(file) => file,
            Err(err) if err.kind() == io::ErrorKind::NotFound => return Ok(Vec::new()),
            Err(err) => return Err(Error::io(&self.path)(err)),
        };
        let len = file.metadata().map_err(Error::io(&self.path))?.len();
        let mut skip_first = false;
        if self.is_first_read {
            self.is_first_read = false;
            if len > self.window {
                self.offset = len - self.window;
                skip_first = true;
            }
        }
        if len < self.offset {
            self.offset = 0;
            self.partial.clear();
        }
        file.seek(SeekFrom::Start(self.offset))
            .map_err(Error::io(&self.path))?;
        let mut fresh = Vec::new();
        file.read_to_end(&mut fresh)
            .map_err(Error::io(&self.path))?;
        self.offset += fresh.len() as u64;
        self.partial.extend_from_slice(&fresh);
        let mut lines = split_lines(&mut self.partial);
        if skip_first && !lines.is_empty() {
            lines.remove(0);
        }
        Ok(lines)
    }
}

fn degrade(inbox: &Sender<Input>, detail: String) {
    let _ = inbox.blocking_send(sensor(
        SensorName::Ledger,
        SensorState::Degraded,
        Some(detail),
    ));
}

/// Drains every `\n`-terminated, non-empty line from `buffer`; an unterminated tail stays for the next read.
fn split_lines(buffer: &mut Vec<u8>) -> Vec<Vec<u8>> {
    let Some(last_newline) = buffer.iter().rposition(|byte| *byte == b'\n') else {
        return Vec::new();
    };
    let rest = buffer.split_off(last_newline + 1);
    let complete = std::mem::replace(buffer, rest);
    complete
        .split(|byte| *byte == b'\n')
        .filter(|line| !line.is_empty())
        .map(<[u8]>::to_vec)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Label, buffer in, lines out, buffer left.
    type Case = (
        &'static str,
        &'static [u8],
        &'static [&'static [u8]],
        &'static [u8],
    );

    #[test]
    fn split_lines_keeps_the_unterminated_tail() {
        let cases: [Case; 4] = [
            ("two-complete", b"a\nb\n", &[b"a", b"b"], b""),
            ("tail-kept", b"a\nb", &[b"a"], b"b"),
            ("no-newline", b"ab", &[], b"ab"),
            ("blank-lines-dropped", b"\n\na\n", &[b"a"], b""),
        ];
        for (label, input, lines, rest) in cases {
            let mut buffer = input.to_vec();
            let got = split_lines(&mut buffer);
            let expected: Vec<Vec<u8>> = lines.iter().map(|line| line.to_vec()).collect();
            assert_eq!(got, expected, "{label}");
            assert_eq!(buffer, rest, "{label}");
        }
    }
}
