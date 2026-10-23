//! Process-exit edge (darwin): one kqueue with an `EVFILT_PROC`/`NOTE_EXIT` knote per registry pid. A waiter thread
//! blocks in `kevent` and forwards each exit; [`ExitWatch::watch`] registers a pid from the engine thread. `EV_ONESHOT`
//! drops the knote once the exit fires; a pid already gone returns `ESRCH` → [`Watch::AlreadyGone`].
//! Elsewhere one poll thread refreshes the watched pids through sysinfo every `exits_poll` and forwards each pid gone,
//! so an exit is seen up to one interval late; the darwin `start` ignores the interval.

use tokio::sync::mpsc::Sender;

use crate::engine::Input;
use crate::ids::Pid;
use crate::{Error, Result};

/// The outcome of registering one pid.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Watch {
    /// The exit will arrive as `ProcessExited { sighting: Observed }`.
    Registered,
    /// The process was already gone.
    AlreadyGone,
}

#[cfg(target_os = "macos")]
pub use imp::ExitWatch;
#[cfg(not(target_os = "macos"))]
pub use poll::ExitWatch;

#[cfg(target_os = "macos")]
mod imp {
    use std::sync::Arc;
    use std::time::Duration;

    use jiff::Timestamp;
    use nix::errno::Errno;
    use nix::libc;
    use nix::sys::event::{EvFlags, EventFilter, FilterFlag, KEvent, Kqueue};

    use super::{Error, Input, Pid, Result, Sender, Watch};
    use crate::edges::sensor;
    use crate::event::{Event, ExitSighting};
    use crate::fleet::{SensorName, SensorState};

    /// The registration handle; the waiter thread holds the other `Arc`.
    #[derive(Debug, Clone)]
    pub struct ExitWatch {
        queue: Arc<Kqueue>,
    }

    impl ExitWatch {
        /// Creates the queue and starts the waiter thread, which reads up to `batch` events (`exits_batch`) per
        /// `kevent` call.
        pub fn start(inbox: Sender<Input>, batch: usize, _interval: Duration) -> Result<ExitWatch> {
            let queue =
                Arc::new(Kqueue::new().map_err(|err| Error::ExitWatch(format!("kqueue: {err}")))?);
            let waiter = Arc::clone(&queue);
            std::thread::Builder::new()
                .name("exit-watch".to_owned())
                .spawn(move || wait(&waiter, &inbox, batch))
                .map_err(|err| Error::Task {
                    task: "exit-watch thread",
                    detail: err.to_string(),
                })?;
            Ok(ExitWatch { queue })
        }

        /// Registers `pid`; returns immediately.
        pub fn watch(&self, pid: Pid) -> Result<Watch> {
            let ident = libc::uintptr_t::try_from(pid.0)
                .map_err(|err| Error::ExitWatch(format!("pid {pid}: {err}")))?;
            let change = KEvent::new(
                ident,
                EventFilter::EVFILT_PROC,
                EvFlags::EV_ADD | EvFlags::EV_ONESHOT,
                FilterFlag::NOTE_EXIT,
                0,
                0,
            );
            let immediately = libc::timespec {
                tv_sec: 0,
                tv_nsec: 0,
            };
            match self.queue.kevent(&[change], &mut [], Some(immediately)) {
                Ok(_) => Ok(Watch::Registered),
                Err(Errno::ESRCH) => Ok(Watch::AlreadyGone),
                Err(err) => Err(Error::ExitWatch(format!("register pid {pid}: {err}"))),
            }
        }
    }

    /// Blocks until exits arrive and forwards each; ends when the inbox is gone.
    fn wait(queue: &Kqueue, inbox: &Sender<Input>, batch: usize) {
        let empty = KEvent::new(
            0,
            EventFilter::EVFILT_PROC,
            EvFlags::empty(),
            FilterFlag::empty(),
            0,
            0,
        );
        let mut events = vec![empty; batch.max(1)];
        loop {
            let ready = match queue.kevent(&[], &mut events, None) {
                Ok(ready) => ready,
                Err(Errno::EINTR) => continue,
                Err(err) => {
                    let detail = format!("kevent wait: {err}");
                    let _ = inbox.blocking_send(sensor(
                        SensorName::ProcessExit,
                        SensorState::Degraded,
                        Some(detail),
                    ));
                    return;
                }
            };
            let at = Timestamp::now();
            for event in events.iter().take(ready) {
                if event.flags().contains(EvFlags::EV_ERROR) {
                    continue;
                }
                let Ok(pid) = u32::try_from(event.ident()) else {
                    continue;
                };
                let exited = Input::Event(Event::ProcessExited {
                    pid: Pid(pid),
                    at,
                    sighting: ExitSighting::Observed,
                });
                if inbox.blocking_send(exited).is_err() {
                    return;
                }
            }
        }
    }
}

#[cfg(not(target_os = "macos"))]
mod poll {
    use std::collections::HashSet;
    use std::sync::{Arc, Mutex, MutexGuard};
    use std::time::Duration;

    use jiff::Timestamp;
    use sysinfo::{ProcessRefreshKind, ProcessesToUpdate, System};

    use super::{Error, Input, Pid, Result, Sender, Watch};
    use crate::edges::sensor;
    use crate::event::{Event, ExitSighting};
    use crate::fleet::{SensorName, SensorState};

    /// The registration handle; the poll thread holds the other `Arc`.
    #[derive(Debug, Clone)]
    pub struct ExitWatch {
        watched: Arc<Mutex<HashSet<Pid>>>,
    }

    impl ExitWatch {
        /// Starts the poll thread, which refreshes the watched pids every `interval` and forwards each pid gone.
        pub fn start(inbox: Sender<Input>, _batch: usize, interval: Duration) -> Result<ExitWatch> {
            let watched = Arc::new(Mutex::new(HashSet::new()));
            let polled = Arc::clone(&watched);
            std::thread::Builder::new()
                .name("exit-watch".to_owned())
                .spawn(move || poll(&polled, &inbox, interval))
                .map_err(|err| Error::Task {
                    task: "exit-watch thread",
                    detail: err.to_string(),
                })?;
            Ok(ExitWatch { watched })
        }

        /// Registers `pid` after one liveness check; a pid the system no longer lists is already gone.
        pub fn watch(&self, pid: Pid) -> Result<Watch> {
            if !alive(&mut System::new(), &[pid]).contains(&pid) {
                return Ok(Watch::AlreadyGone);
            }
            self.lock()?.insert(pid);
            Ok(Watch::Registered)
        }

        fn lock(&self) -> Result<MutexGuard<'_, HashSet<Pid>>> {
            self.watched
                .lock()
                .map_err(|err| Error::ExitWatch(format!("watched set poisoned: {err}")))
        }
    }

    /// Sleeps `interval`, refreshes only the watched pids and forwards each one gone; ends when the inbox is gone.
    fn poll(watched: &Mutex<HashSet<Pid>>, inbox: &Sender<Input>, interval: Duration) {
        let mut system = System::new();
        loop {
            std::thread::sleep(interval);
            let pids: Vec<Pid> = match watched.lock() {
                Ok(set) => set.iter().copied().collect(),
                Err(err) => {
                    let detail = format!("watched set poisoned: {err}");
                    let _ = inbox.blocking_send(sensor(
                        SensorName::ProcessExit,
                        SensorState::Degraded,
                        Some(detail),
                    ));
                    return;
                }
            };
            if pids.is_empty() {
                continue;
            }
            let still = alive(&mut system, &pids);
            let at = Timestamp::now();
            for pid in pids.into_iter().filter(|pid| !still.contains(pid)) {
                if let Ok(mut set) = watched.lock() {
                    set.remove(&pid);
                }
                let exited = Input::Event(Event::ProcessExited {
                    pid,
                    at,
                    sighting: ExitSighting::Observed,
                });
                if inbox.blocking_send(exited).is_err() {
                    return;
                }
            }
        }
    }

    /// The subset of `pids` the system still lists after a refresh limited to them.
    fn alive(system: &mut System, pids: &[Pid]) -> HashSet<Pid> {
        let wanted: Vec<sysinfo::Pid> = pids
            .iter()
            .map(|pid| sysinfo::Pid::from_u32(pid.0))
            .collect();
        system.refresh_processes_specifics(
            ProcessesToUpdate::Some(&wanted),
            true,
            ProcessRefreshKind::nothing(),
        );
        pids.iter()
            .copied()
            .filter(|pid| system.process(sysinfo::Pid::from_u32(pid.0)).is_some())
            .collect()
    }

    #[cfg(test)]
    mod tests {
        use std::process::{Command, Stdio};

        use tokio::sync::mpsc;

        use super::*;

        /// A child that lives until its stdin closes: `cat` on unix, `more` on windows.
        fn stdin_bound_child() -> std::process::Child {
            let program = if cfg!(windows) { "more" } else { "cat" };
            Command::new(program)
                .stdin(Stdio::piped())
                .stdout(Stdio::null())
                .spawn()
                .expect("spawn")
        }

        #[test]
        fn alive_keeps_only_listed_pids() {
            let own = Pid(std::process::id());
            let gone = Pid(u32::MAX - 1);

            let still = alive(&mut System::new(), &[own, gone]);

            assert!(still.contains(&own));
            assert!(!still.contains(&gone));
        }

        #[test]
        fn exited_child_is_forwarded_once() {
            let (inbox, mut outbox) = mpsc::channel(4);
            let watch = ExitWatch::start(inbox, 1, Duration::from_millis(50)).expect("start");
            let mut child = stdin_bound_child();
            let pid = Pid(child.id());

            assert_eq!(watch.watch(pid).expect("watch"), Watch::Registered);
            drop(child.stdin.take());
            child.wait().expect("wait");
            let first = outbox.blocking_recv().expect("one event");

            assert!(
                matches!(first, Input::Event(Event::ProcessExited { pid: got, .. }) if got == pid)
            );
            assert_eq!(watch.watch(pid).expect("watch"), Watch::AlreadyGone);
        }
    }
}
