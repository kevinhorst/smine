//! Git watch edge: one notify watcher, one recursive watch per registered repo's `.git` directory, forwarding
//! `RefsChanged` whenever a ref moves: `.git/HEAD`, `.git/packed-refs`, `.git/refs/**`, `.git/worktrees/*/HEAD` —
//! including worktrees created after start. Lock files and every other `.git` path are filtered out; the core
//! coalesces the bursts (D3). The watched set follows the registry: [`Watch::set_repos`] arms the added repos' `.git`
//! and drops the removed ones' watches, at start and on every `WatchRepos` action the repos edge's change causes;
//! the callback reads the current set. An event flagged `need_rescan()` (dropped events, no paths) sends `RefsChanged`
//! for every watched repo, as P1's registry edge rescans.
//!
//! The callback uses `blocking_send` and never holds the set's lock while sending; [`Watch::set_repos`] sends nothing
//! — the first scan of an added repo is the engine's trigger — so the engine thread may call it.

use std::collections::BTreeSet;
use std::path::{Component, Path};
use std::sync::{Arc, Mutex, PoisonError};

use jiff::Timestamp;
use notify::{RecommendedWatcher, RecursiveMode, Watcher as _};
use tokio::sync::mpsc::Sender;

use crate::edges::sensor;
use crate::engine::Input;
use crate::event::Event;
use crate::fleet::{SensorName, SensorState};
use crate::repo::RepoId;
use crate::{Error, Result};

/// The edge's handle: the watcher and the watched set, shared with the callback; cloned into the engine's executor.
#[derive(Clone)]
pub struct Watch {
    targets: Arc<Mutex<BTreeSet<RepoId>>>,
    watcher: Arc<Mutex<RecommendedWatcher>>,
}

impl Watch {
    /// Creates the watcher with no repo watched.
    pub fn start(inbox: Sender<Input>) -> Result<Watch> {
        let targets = Arc::new(Mutex::new(BTreeSet::new()));
        let callback_targets = Arc::clone(&targets);
        let watcher = notify::recommended_watcher(move |result: notify::Result<notify::Event>| {
            let changed = match result {
                Ok(event) => changed_repos(&lock(&callback_targets), &event),
                Err(err) => {
                    let _ = inbox.blocking_send(sensor(
                        SensorName::Git,
                        SensorState::Degraded,
                        Some(err.to_string()),
                    ));
                    return;
                }
            };
            for repo in changed {
                let _ = inbox.blocking_send(Input::Event(Event::RefsChanged {
                    repo,
                    at: Timestamp::now(),
                }));
            }
        })
        .map_err(|err| Error::Watch {
            path: ".git".into(),
            detail: err.to_string(),
        })?;
        let watch = Watch {
            targets,
            watcher: Arc::new(Mutex::new(watcher)),
        };
        Ok(watch)
    }

    /// Makes `repos` the watched set: arms the added repos' `.git` and drops the removed ones; the arming failures,
    /// for the `git` sensor.
    #[must_use]
    pub fn set_repos(&self, repos: &[RepoId]) -> Option<Error> {
        let wanted: BTreeSet<RepoId> = repos.iter().cloned().collect();
        let mut targets = lock(&self.targets);
        let mut watcher = lock(&self.watcher);
        for repo in targets.difference(&wanted) {
            let _ = watcher.unwatch(&repo.0.join(".git"));
        }
        let mut failures = Vec::new();
        for repo in wanted.difference(&targets) {
            let git = repo.0.join(".git");
            if !git.is_dir() {
                failures.push(format!("{}: no .git directory", repo.0.display()));
                continue;
            }
            if let Err(err) = watcher.watch(&git, RecursiveMode::Recursive) {
                failures.push(format!("{}: {err}", git.display()));
            }
        }
        *targets = wanted;
        (!failures.is_empty()).then(|| Error::Watch {
            path: ".git".into(),
            detail: failures.join("; "),
        })
    }
}

/// The watched repos `event` concerns: every one on a rescan, else those a changed path is a ref of.
fn changed_repos(targets: &BTreeSet<RepoId>, event: &notify::Event) -> Vec<RepoId> {
    targets
        .iter()
        .filter(|repo| {
            event.need_rescan() || event.paths.iter().any(|path| is_ref_path(&repo.0, path))
        })
        .cloned()
        .collect()
}

/// `path` is a ref of `repo`: `.git/HEAD`, `.git/packed-refs`, anything under `.git/refs/`, or
/// `.git/worktrees/<name>/HEAD`; lock files never are.
fn is_ref_path(repo: &Path, path: &Path) -> bool {
    let Ok(rest) = path.strip_prefix(repo.join(".git")) else {
        return false;
    };
    if path
        .extension()
        .is_some_and(|extension| extension == "lock")
    {
        return false;
    }
    let parts: Vec<&str> = rest
        .components()
        .filter_map(|component| match component {
            Component::Normal(part) => part.to_str(),
            Component::Prefix(_)
            | Component::RootDir
            | Component::CurDir
            | Component::ParentDir => None,
        })
        .collect();
    matches!(
        parts.as_slice(),
        ["HEAD" | "packed-refs"] | ["refs", ..] | ["worktrees", _, "HEAD"]
    )
}

/// The guarded value; a poisoned lock (a panicked holder) keeps serving its data.
fn lock<T>(mutex: &Mutex<T>) -> std::sync::MutexGuard<'_, T> {
    mutex.lock().unwrap_or_else(PoisonError::into_inner)
}
