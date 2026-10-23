//! `serve --record <file>`: appends every received hook body with its receipt time as one JSON line
//! `{"at": <ms>, "body": <hook input>}` — the replay format of the engine tests, the integration bodies and the bench.
//! Bodies that are not JSON are skipped; a write error ends the recorder, never the daemon.

use std::fs::OpenOptions;
use std::io::Write as _;
use std::path::Path;
use std::sync::mpsc::{Sender, channel};

use jiff::Timestamp;

use crate::{Error, Result};

/// One received hook body.
#[derive(Debug)]
pub struct Recorded {
    /// Receipt time.
    pub at: Timestamp,
    /// The raw body.
    pub body: Vec<u8>,
}

/// Opens `path` for appending and starts the writer thread.
pub fn start(path: &Path) -> Result<Sender<Recorded>> {
    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
        .map_err(Error::io(path))?;
    let (sender, records) = channel::<Recorded>();
    std::thread::Builder::new()
        .name("recorder".to_owned())
        .spawn(move || {
            for record in records {
                let Ok(body) = serde_json::from_slice::<serde_json::Value>(&record.body) else {
                    continue;
                };
                let line = serde_json::json!({ "at": record.at.as_millisecond(), "body": body });
                if writeln!(file, "{line}").is_err() {
                    return;
                }
            }
        })
        .map_err(|err| Error::Task {
            task: "recorder thread",
            detail: err.to_string(),
        })?;
    Ok(sender)
}
