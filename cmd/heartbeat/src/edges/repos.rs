//! Repo-registry edge: watches the directory of `--repos` (`<claude home>/smine/repos.json`, which the config server
//! writes through a temp file plus rename, so the watch filters by the final name), re-reads the file on every change
//! and forwards `ReposChanged` with the canonical repos; the engine's `WatchRepos` action then re-arms the git watch
//! and reports the `git` sensor. A file not written yet (no repo registered) is an empty registry; a malformed one
//! degrades the `git` sensor and keeps the running set. [`read`] serves the start too. One reader thread, like the
//! config edge.

use std::path::{Path, PathBuf};
use std::sync::mpsc;

use notify::{RecommendedWatcher, RecursiveMode, Watcher as _};
use tokio::sync::mpsc::Sender;

use crate::config;
use crate::edges::sensor;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{SensorName, SensorState};
use crate::log;
use crate::repo::RepoId;
use crate::{Error, Result};

/// The registry's repos, canonical, with their registry names, and the rows that do not canonicalize (log lines).
pub struct Registered {
    /// Canonical path and registry name per row.
    pub repos: Vec<(RepoId, String)>,
    /// Rows whose path does not canonicalize.
    pub warnings: Vec<String>,
}

/// Reads the registry; a missing file is an empty one.
pub fn read(path: &Path) -> Result<Registered> {
    let entries = match std::fs::read(path) {
        Ok(bytes) => config::parse_repos(&bytes)?,
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => Vec::new(),
        Err(err) => return Err(Error::io(path)(err)),
    };
    let mut registered = Registered {
        repos: Vec::new(),
        warnings: Vec::new(),
    };
    for entry in entries {
        match dunce::canonicalize(&entry.path) {
            Ok(path) => registered.repos.push((RepoId(path), entry.name)),
            Err(err) => registered.warnings.push(format!(
                "repo {} ({}): {err}",
                entry.name,
                entry.path.display()
            )),
        }
    }
    Ok(registered)
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
        .name("repos".to_owned())
        .spawn(move || {
            while wakeups.recv().is_ok() {
                while wakeups.try_recv().is_ok() {}
                forward(&file, &inbox);
            }
        })
        .map_err(|err| Error::Task {
            task: "repos thread",
            detail: err.to_string(),
        })?;
    Ok(watcher)
}

/// One reload: a readable file becomes `ReposChanged` (the engine's `WatchRepos` reports the sensor); a refusal
/// degrades the sensor and sends nothing else.
fn forward(path: &Path, inbox: &Sender<Input>) {
    match read(path) {
        Ok(registered) => {
            for warning in &registered.warnings {
                log::line(format_args!("{warning}"));
            }
            let _ = inbox.blocking_send(Input::Event(Event::ReposChanged {
                repos: registered.repos,
                at: jiff::Timestamp::now(),
            }));
        }
        Err(err) => {
            let _ = inbox.blocking_send(sensor(
                SensorName::Git,
                SensorState::Degraded,
                Some(format!("registry kept: {err}")),
            ));
        }
    }
}
