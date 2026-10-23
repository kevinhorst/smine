//! Who this daemon is — version, claude home, bound address — and whose hook bodies it accepts. A body belongs to this
//! daemon when its `transcript_path` lies under the claude home, compared as configured and canonicalized (macOS
//! `/var` vs `/private/var`). Anything else is another profile's session: refused, never applied.

use std::net::SocketAddr;
use std::path::{Path, PathBuf};

use crate::wire::Health;
use crate::{Error, Result};

/// This daemon's identity.
#[derive(Debug, Clone)]
pub struct Identity {
    /// Crate version.
    pub version: &'static str,
    /// The claude home as configured.
    pub claude_home: PathBuf,
    /// The claude home canonicalized.
    pub canonical_home: PathBuf,
    /// The bound address.
    pub addr: SocketAddr,
    claude_home_text: String,
}

impl Identity {
    /// The identity of a daemon serving `claude_home` on `addr`; the home must exist and be UTF-8.
    pub fn new(claude_home: PathBuf, addr: SocketAddr) -> Result<Identity> {
        let claude_home_text = claude_home
            .to_str()
            .ok_or_else(|| Error::NonUtf8Path(claude_home.clone()))?
            .to_owned();
        let canonical_home = dunce::canonicalize(&claude_home).map_err(Error::io(&claude_home))?;
        Ok(Identity {
            version: env!("CARGO_PKG_VERSION"),
            claude_home,
            canonical_home,
            addr,
            claude_home_text,
        })
    }

    /// Whether a hook body with this `transcript_path` belongs to this daemon.
    #[must_use]
    pub fn owns(&self, transcript_path: Option<&Path>) -> bool {
        transcript_path.is_none_or(|path| {
            path.starts_with(&self.claude_home) || path.starts_with(&self.canonical_home)
        })
    }

    /// The `/healthz` answer and the status document header.
    #[must_use]
    pub fn health(&self) -> Health {
        Health {
            version: self.version.to_owned(),
            claude_home: self.claude_home_text.clone(),
            addr: self.addr.to_string(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn owns_only_transcripts_under_the_home() {
        let identity = Identity {
            version: "0.0.0",
            claude_home: PathBuf::from("/Users/k/.claude"),
            canonical_home: PathBuf::from("/private/Users/k/.claude"),
            addr: SocketAddr::from(([127, 0, 0, 1], 6101)),
            claude_home_text: "/Users/k/.claude".to_owned(),
        };
        let cases = [
            (
                "configured-home",
                Some("/Users/k/.claude/projects/p/s.jsonl"),
                true,
            ),
            (
                "canonical-home",
                Some("/private/Users/k/.claude/projects/p/s.jsonl"),
                true,
            ),
            (
                "other-profile",
                Some("/Users/k/.claude-b/projects/p/s.jsonl"),
                false,
            ),
            ("elsewhere", Some("/tmp/s.jsonl"), false),
            ("absent", None, true),
        ];
        for (label, path, expected) in cases {
            assert_eq!(identity.owns(path.map(Path::new)), expected, "{label}");
        }
    }
}
