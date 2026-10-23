//! The daemon's one log channel: `heartbeat: …` lines on stderr through anstream, or appended to the file
//! `set_file` installed (the Windows logon task captures nothing, so it passes `--log-file`); launchd routes
//! stderr to `~/Library/Logs/smine-heartbeat.err.log`. stdout stays payload-only.

use std::fs::{File, OpenOptions};
use std::io::Write as _;
use std::path::Path;
use std::sync::{Mutex, OnceLock};

static FILE: OnceLock<Mutex<File>> = OnceLock::new();

/// Routes every later line into `path`, appended; a second call is refused.
pub fn set_file(path: &Path) -> std::io::Result<()> {
    let file = OpenOptions::new().create(true).append(true).open(path)?;
    FILE.set(Mutex::new(file))
        .map_err(|_| std::io::Error::other("log file already set"))
}

/// Writes one log line; a failed write is dropped, because nothing else could report it.
pub fn line(message: std::fmt::Arguments<'_>) {
    if let Some(file) = FILE.get() {
        if let Ok(mut file) = file.lock() {
            let _ = writeln!(file, "heartbeat: {message}");
        }
        return;
    }
    let _ = writeln!(anstream::stderr(), "heartbeat: {message}");
}
