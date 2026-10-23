//! A throwaway git repository with agent worktrees under `.claude/worktrees/`, driven through the git CLI with
//! fixed identities and monotonic dates, so every run builds the same history.

use std::cell::Cell;
use std::path::{Path, PathBuf};
use std::process::Command;

use tempfile::TempDir;

pub struct GitRepo {
    _dir: TempDir,
    pub path: PathBuf,
    clock: Cell<u64>,
}

impl GitRepo {
    /// `main` with `base.txt` and `shared.txt`; `.claude/` excluded so the pool never dirties the checkout.
    pub fn new() -> GitRepo {
        let dir = tempfile::tempdir().expect("tempdir");
        let path = dunce::canonicalize(dir.path()).expect("canonical");
        let repo = GitRepo {
            _dir: dir,
            path,
            clock: Cell::new(1_790_000_000),
        };
        repo.git(&repo.path, &["init", "-q", "-b", "main"]);
        std::fs::write(repo.path.join(".git/info/exclude"), ".claude/\n").expect("exclude");
        repo.commit(&repo.path, "base.txt", "base\n", "base");
        repo.commit(&repo.path, "shared.txt", "line1\nline2\nline3\n", "shared");
        repo
    }

    /// Runs git in `cwd`; panics with stderr on failure; returns stdout.
    pub fn git(&self, cwd: &Path, args: &[&str]) -> String {
        let tick = self.clock.get() + 1;
        self.clock.set(tick);
        let date = format!("{tick} +0000");
        let output = Command::new("git")
            .current_dir(cwd)
            .args([
                "-c",
                "user.name=fixture",
                "-c",
                "user.email=fixture@local",
                "-c",
                "commit.gpgsign=false",
            ])
            .args(args)
            .env("GIT_AUTHOR_DATE", &date)
            .env("GIT_COMMITTER_DATE", &date)
            .output()
            .expect("spawn git");
        assert!(
            output.status.success(),
            "git {args:?}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        String::from_utf8_lossy(&output.stdout).into_owned()
    }

    /// `claude/<name>` from `from`, checked out at `.claude/worktrees/<name>`; returns the canonical worktree path.
    pub fn worktree(&self, name: &str, from: &str) -> PathBuf {
        let path = self.path.join(".claude/worktrees").join(name);
        let branch = format!("claude/{name}");
        let target = path.to_str().expect("utf-8");
        self.git(
            &self.path,
            &["worktree", "add", "-q", "-b", &branch, target, from],
        );
        dunce::canonicalize(&path).expect("canonical worktree")
    }

    /// Writes `rela` under `cwd`, creating parents.
    #[allow(clippy::unused_self)]
    pub fn write(&self, cwd: &Path, rela: &str, content: &str) {
        let file = cwd.join(rela);
        std::fs::create_dir_all(file.parent().expect("parent")).expect("parents");
        std::fs::write(file, content).expect("write");
    }

    /// Writes, stages and commits one file in `cwd`.
    pub fn commit(&self, cwd: &Path, rela: &str, content: &str, subject: &str) {
        self.write(cwd, rela, content);
        self.git(cwd, &["add", rela]);
        self.git(cwd, &["commit", "-q", "-m", subject]);
    }
}
