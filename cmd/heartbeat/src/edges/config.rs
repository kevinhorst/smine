//! edges/config.rs — the config edge: watches `<claude home>/heartbeat/config.json`, parses it on every change and
//! forwards `ConfigChanged`; a file that does not parse degrades the `config` sensor and the engine keeps the running
//! config. The install writes the file through `config.json.tmp` plus rename, so the watch filters by the final name.

use std::path::{Path, PathBuf};
use std::sync::mpsc;

use notify::{RecommendedWatcher, RecursiveMode, Watcher as _};
use tokio::sync::mpsc::Sender;

use crate::config::{self, FileConfig};
use crate::edges::sensor;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{SensorName, SensorState};
use crate::{Error, Result};

/// Reads the file; the caller decides what a failure means.
pub fn read(path: &Path) -> Result<FileConfig> {
    let bytes = std::fs::read(path).map_err(Error::io(path))?;
    config::parse(&bytes)
}

/// Arms the watch on the file's directory, starts the reader thread, returns the watcher; dropping it stops the edge.
pub fn start(path: &Path, inbox: Sender<Input>) -> Result<RecommendedWatcher> {
    let dir = path.parent().unwrap_or_else(|| Path::new("."));
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
    let file: PathBuf = path.to_path_buf();
    std::thread::Builder::new()
        .name("config".to_owned())
        .spawn(move || {
            while wakeups.recv().is_ok() {
                while wakeups.try_recv().is_ok() {}
                forward(&file, &inbox);
            }
        })
        .map_err(|err| Error::Task {
            task: "config thread",
            detail: err.to_string(),
        })?;
    Ok(watcher)
}

/// One reload: a parsed file becomes `ConfigChanged` and an `Ok` sensor; a refusal degrades the sensor and sends
/// nothing else.
fn forward(path: &Path, inbox: &Sender<Input>) {
    match read(path) {
        Ok(file_config) => {
            let _ = inbox.blocking_send(Input::Event(Event::ConfigChanged {
                live: Box::new(file_config.live),
                at: jiff::Timestamp::now(),
            }));
            let _ = inbox.blocking_send(sensor(SensorName::Config, SensorState::Ok, None));
        }
        Err(err) => {
            let _ = inbox.blocking_send(sensor(
                SensorName::Config,
                SensorState::Degraded,
                Some(format!("config kept: {err}")),
            ));
        }
    }
}
