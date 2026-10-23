//! Error model: one `Error` enum for the library, converted to `anyhow` by the binary; the exit
//! code comes from [`Error::exit_code`] alone. Edge failures at runtime never surface here: they become sensor events.

use std::net::SocketAddr;
use std::path::PathBuf;

use crate::protocol::{EXIT_FOREIGN_HOME, EXIT_SIGNAL_BASE, EXIT_UNREACHABLE};

/// Library result alias.
pub type Result<T> = std::result::Result<T, Error>;

/// Every failure the library can return.
#[derive(Debug, thiserror::Error)]
pub enum Error {
    /// Reading a file or directory failed; `path` names it.
    #[error("{path}: {source}")]
    Io {
        /// The file or directory involved.
        path: PathBuf,
        /// The underlying error.
        #[source]
        source: std::io::Error,
    },
    /// A hook body, registry file or ledger line is not the JSON heartbeat expects.
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    /// The heartbeat config refused: an unknown key, a wrong type or a pattern that does not compile; names the key.
    #[error("heartbeat config: {0}")]
    Config(String),
    /// A path is not valid UTF-8 and cannot enter the status document.
    #[error("path is not valid {encoding}: {0}", encoding = crate::protocol::PATH_ENCODING)]
    NonUtf8Path(PathBuf),
    /// Arming a filesystem watch failed.
    #[error("watch {path}: {detail}")]
    Watch {
        /// The watched directory.
        path: PathBuf,
        /// The watcher's error.
        detail: String,
    },
    /// The listener could not bind (another heartbeat, another profile) or stopped serving.
    #[error("listen {addr}: {source}")]
    Listen {
        /// The address.
        addr: SocketAddr,
        /// The underlying error.
        #[source]
        source: std::io::Error,
    },
    /// The kqueue process-exit watch failed, or the platform has none.
    #[error("process-exit watch: {0}")]
    ExitWatch(String),
    /// A background thread or task could not start, or ended.
    #[error("{task}: {detail}")]
    Task {
        /// Which thread or task.
        task: &'static str,
        /// What happened.
        detail: String,
    },
    /// No daemon answers at the address.
    #[error("no heartbeat on {addr}: {detail}")]
    Unreachable {
        /// The address tried.
        addr: SocketAddr,
        /// The transport error.
        detail: String,
    },
    /// The daemon answered non-2xx (e.g. the fleet filter before P3).
    #[error("heartbeat refused: {0}")]
    Refused(String),
    /// The daemon on this address serves another Claude home (another profile).
    #[error("heartbeat on {addr} serves claude home {served}, not {expected}")]
    ForeignHome {
        /// The daemon address.
        addr: SocketAddr,
        /// The home the daemon reports.
        served: String,
        /// The home this CLI runs for.
        expected: String,
    },
    /// `--session self` outside a Claude Code session.
    #[error("--session self needs CLAUDE_CODE_SESSION_ID (run it from a Claude Code session)")]
    NoSelfSession,
    /// SIGTERM or SIGINT ended `serve`.
    #[error("interrupted by signal {signal}")]
    Interrupted {
        /// The signal number.
        signal: i32,
    },
}

impl Error {
    /// Exit code the binary maps this error to.
    #[must_use]
    pub fn exit_code(&self) -> i32 {
        match self {
            Error::Unreachable { .. } => EXIT_UNREACHABLE,
            Error::ForeignHome { .. } => EXIT_FOREIGN_HOME,
            Error::Interrupted { signal } => EXIT_SIGNAL_BASE + signal,
            Error::Io { .. }
            | Error::Json(_)
            | Error::Config(_)
            | Error::NonUtf8Path(_)
            | Error::Watch { .. }
            | Error::Listen { .. }
            | Error::ExitWatch(_)
            | Error::Task { .. }
            | Error::Refused(_)
            | Error::NoSelfSession => 1,
        }
    }

    /// Wraps an io error with the path it concerns.
    #[cfg(feature = "cli")]
    pub(crate) fn io(path: impl Into<PathBuf>) -> impl FnOnce(std::io::Error) -> Error {
        let path = path.into();
        move |source| Error::Io { path, source }
    }
}
