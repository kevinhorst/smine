//! Fixture builder: throwaway repositories driven through the real `git` binary, plus the binary runner.
//!
//! Every scenario builds its own repo under a `tempfile::TempDir`; nothing touches the host's repos.
//! Commit times are monotonic (`GIT_COMMITTER_DATE`) so "newest first" orderings are deterministic.

#![allow(dead_code)]

use std::io::Write as _;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

use serde_json::Value;
use tempfile::TempDir;

/// A throwaway repository with helpers for the states the verdicts must recognize.
pub struct Fixture {
    _dir: TempDir,
    /// The main checkout.
    pub repo: PathBuf,
    /// Synthetic Claude home with a `sessions/` registry.
    pub claude_home: PathBuf,
    /// Synthetic Codex home with a rollout.
    pub codex_home: PathBuf,
    tick: u64,
}

impl Fixture {
    /// `git init -b main` plus a base commit holding `base.txt` and `shared.txt`.
    pub fn new() -> Fixture {
        let dir = tempfile::tempdir().expect("tempdir");
        let repo = dir.path().join("repo");
        std::fs::create_dir_all(&repo).expect("mkdir repo");
        let claude_home = dir.path().join("claude-home");
        std::fs::create_dir_all(claude_home.join("sessions")).expect("mkdir claude home");
        let codex_home = dir.path().join("codex-home");
        std::fs::create_dir_all(&codex_home).expect("mkdir codex home");
        let mut fixture = Fixture {
            _dir: dir,
            repo,
            claude_home,
            codex_home,
            tick: 1_700_000_000,
        };
        fixture.git(&["init", "-q", "-b", "main"]);
        fixture.git(&["config", "user.email", "test@example.com"]);
        fixture.git(&["config", "user.name", "test"]);
        fixture.git(&["config", "commit.gpgsign", "false"]);
        fixture.write("base.txt", "base\n");
        fixture.write("shared.txt", "line1\nline2\nline3\n");
        fixture.git(&["add", "."]);
        fixture.commit("base");
        fixture
    }

    /// Runs git in the main checkout; panics on failure (a fixture bug, not a test outcome).
    pub fn git(&mut self, args: &[&str]) -> String {
        self.git_in(&self.repo.clone(), args)
    }

    /// A git command in `dir` with the next monotonic date and a no-op editor, so `--continue` steps never block.
    fn command(&mut self, dir: &Path, args: &[&str]) -> Command {
        self.tick += 60;
        let date = format!("{} +0000", self.tick);
        let mut command = Command::new("git");
        command
            .arg("-C")
            .arg(dir)
            .args(args)
            .env("GIT_COMMITTER_DATE", &date)
            .env("GIT_AUTHOR_DATE", &date)
            .env("GIT_EDITOR", "true");
        command
    }

    /// Runs git in `dir`.
    pub fn git_in(&mut self, dir: &Path, args: &[&str]) -> String {
        let output = self.command(dir, args).output().expect("spawn git");
        assert!(
            output.status.success(),
            "git {args:?} in {}: {}",
            dir.display(),
            String::from_utf8_lossy(&output.stderr)
        );
        String::from_utf8_lossy(&output.stdout).trim().to_owned()
    }

    /// Runs git in `dir` without asserting success — for steps that must stop on a conflict.
    pub fn try_git_in(&mut self, dir: &Path, args: &[&str]) -> Output {
        self.command(dir, args).output().expect("spawn git")
    }

    /// Writes a file in the main checkout.
    pub fn write(&self, rela: &str, content: &str) {
        write_at(&self.repo, rela, content);
    }

    /// Commits everything staged in the main checkout, returning the sha.
    pub fn commit(&mut self, subject: &str) -> String {
        self.git(&["commit", "-q", "-m", subject]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// Switches the main checkout to `branch`.
    pub fn checkout(&mut self, branch: &str) {
        self.git(&["checkout", "-q", branch]);
    }

    /// Creates `name` at `from` (a ref name → reflog `Created from <name>`; a sha → `Created from <sha>`).
    pub fn branch(&mut self, name: &str, from: &str) {
        self.git(&["branch", name, from]);
    }

    /// Adds a commit on `branch` (checked out in the main checkout) writing `rela`.
    pub fn commit_on(&mut self, branch: &str, rela: &str, content: &str, subject: &str) -> String {
        self.checkout(branch);
        self.write(rela, content);
        self.git(&["add", "-A"]);
        self.commit(subject)
    }

    /// Renames `from` to `to` on `branch` and commits.
    pub fn rename_on(&mut self, branch: &str, from: &str, to: &str, subject: &str) -> String {
        self.checkout(branch);
        self.git(&["mv", from, to]);
        self.commit(subject)
    }

    /// Cherry-picks `sha` onto `onto` (fixture-only: it builds the applied/picked states the verdicts recognize).
    pub fn cherry_pick(&mut self, onto: &str, sha: &str) -> String {
        self.checkout(onto);
        self.git(&["cherry-pick", sha]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// `git merge --no-ff branch` into `into`.
    pub fn merge_no_ff(&mut self, into: &str, branch: &str) -> String {
        self.checkout(into);
        self.git(&[
            "merge",
            "--no-ff",
            "-q",
            "-m",
            &format!("Merge branch '{branch}'"),
            branch,
        ]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// Adds a linked worktree for `branch` under `<tmp>/wt/<dir>`; returns its path.
    pub fn worktree(&mut self, branch: &str, dir: &str) -> PathBuf {
        let path = self.repo.parent().expect("parent").join("wt").join(dir);
        std::fs::create_dir_all(path.parent().expect("wt dir")).expect("mkdir wt");
        let path_str = path.to_str().expect("utf8").to_owned();
        self.git(&["worktree", "add", "-q", &path_str, branch]);
        path
    }

    /// Adds a detached worktree at `sha`.
    pub fn detached_worktree(&mut self, sha: &str, dir: &str) -> PathBuf {
        let path = self.codex_home.join("worktrees").join(dir).join("repo");
        std::fs::create_dir_all(path.parent().expect("parent")).expect("mkdir codex pool");
        let path_str = path.to_str().expect("utf8").to_owned();
        self.git(&["worktree", "add", "-q", "--detach", &path_str, sha]);
        path
    }

    /// Locks a worktree the way older sessions did; nothing locks session
    /// worktrees any more, but a kept lock must still come off on removal.
    pub fn lock(&mut self, worktree: &Path) {
        let path = worktree.to_str().expect("utf8").to_owned();
        self.git(&["worktree", "lock", "--reason", "claude session", &path]);
    }

    /// `git rebase <onto>` of `branch` in the main checkout; returns the new tip.
    pub fn rebase(&mut self, branch: &str, onto: &str) -> String {
        self.checkout(branch);
        self.git(&["rebase", "-q", onto]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// `git rebase --onto <new_base> <upstream> <branch>`; returns the new tip.
    pub fn rebase_onto(&mut self, branch: &str, new_base: &str, upstream: &str) -> String {
        self.git(&["rebase", "-q", "--onto", new_base, upstream, branch]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// `git reset --hard <to>` on `branch`.
    pub fn reset_hard(&mut self, branch: &str, to: &str) {
        self.checkout(branch);
        self.git(&["reset", "-q", "--hard", to]);
    }

    /// `git branch -f <name> <to>` — the reflog records `branch: Reset to <to>`.
    pub fn branch_force(&mut self, name: &str, to: &str) {
        self.git(&["branch", "-f", name, to]);
    }

    /// Squash-merges `branch` into `into` as one commit; returns its sha.
    pub fn squash_merge(&mut self, into: &str, branch: &str, subject: &str) -> String {
        self.checkout(into);
        self.git(&["merge", "-q", "--squash", branch]);
        self.commit(subject)
    }

    /// Rewrites `branch`'s commits since `base` into one commit; returns its sha.
    pub fn squash_branch(&mut self, branch: &str, base: &str, subject: &str) -> String {
        self.checkout(branch);
        self.git(&["reset", "-q", "--soft", base]);
        self.commit(subject)
    }

    /// `git revert` of `sha` on `branch`; returns the revert.
    pub fn revert_on(&mut self, branch: &str, sha: &str) -> String {
        self.checkout(branch);
        self.git(&["revert", "--no-edit", sha]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// Replaces `branch`'s tip with `rela` = `content` under `subject`; returns the amended commit.
    pub fn amend_on(&mut self, branch: &str, rela: &str, content: &str, subject: &str) -> String {
        self.checkout(branch);
        self.write(rela, content);
        self.git(&["add", "-A"]);
        self.git(&["commit", "-q", "--amend", "-m", subject]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// An empty commit on `branch`; returns its sha.
    pub fn empty_commit_on(&mut self, branch: &str, subject: &str) -> String {
        self.checkout(branch);
        self.git(&["commit", "-q", "--allow-empty", "-m", subject]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// Cherry-picks `sha` onto `onto` with `extra` flags (`-x`); returns the pick.
    pub fn cherry_pick_with(&mut self, onto: &str, sha: &str, extra: &[&str]) -> String {
        self.checkout(onto);
        let mut args = vec!["cherry-pick"];
        args.extend_from_slice(extra);
        args.push(sha);
        self.git(&args);
        self.git(&["rev-parse", "HEAD"])
    }

    /// Cherry-picks `sha` onto `onto` with `extra` flags (`-x`); it must conflict, and is resolved by writing
    /// `rela` = `content`.
    pub fn cherry_pick_resolving(
        &mut self,
        onto: &str,
        sha: &str,
        extra: &[&str],
        rela: &str,
        content: &str,
    ) -> String {
        self.checkout(onto);
        let repo = self.repo.clone();
        let mut args = vec!["cherry-pick"];
        args.extend_from_slice(extra);
        args.push(sha);
        let pick = self.try_git_in(&repo, &args);
        assert!(
            !pick.status.success(),
            "cherry-pick of {sha} onto {onto} must conflict"
        );
        self.write(rela, content);
        self.git(&["add", "-A"]);
        self.git(&["cherry-pick", "--continue"]);
        self.git(&["rev-parse", "HEAD"])
    }

    /// `git switch -c <name>` from `from` — the reflog records `branch: Created from HEAD`.
    pub fn switch_create(&mut self, name: &str, from: &str) {
        self.checkout(from);
        self.git(&["switch", "-q", "-c", name]);
    }

    /// Points `refs/heads/<name>` at `sha` without a creation line in its reflog.
    pub fn branch_without_origin(&mut self, name: &str, sha: &str) {
        self.git(&["update-ref", &format!("refs/heads/{name}"), sha]);
    }

    /// Points `refs/remotes/origin/<name>` at `at`.
    pub fn remote_ref(&mut self, name: &str, at: &str) {
        self.git(&["update-ref", &format!("refs/remotes/origin/{name}"), at]);
    }

    /// `git branch -m <old> <new>`.
    pub fn rename_branch(&mut self, old: &str, new: &str) {
        self.git(&["branch", "-m", old, new]);
    }

    /// `git branch -D <name>`.
    pub fn delete_branch(&mut self, name: &str) {
        self.git(&["branch", "-q", "-D", name]);
    }

    /// Appends `count` commits to `branch` in one `git fast-import`, each adding `bulk/<n>.txt`. Detaches the main
    /// checkout first so the moved ref never leaves a stale index behind.
    pub fn bulk_commits(&mut self, branch: &str, count: usize) {
        self.git(&["checkout", "-q", "--detach"]);
        let mut chunks = Vec::with_capacity(count);
        for n in 0..count {
            self.tick += 60;
            let message = format!("bulk {n}");
            let content = format!("{n}\n");
            let from = if n == 0 {
                format!("from refs/heads/{branch}^0\n")
            } else {
                String::new()
            };
            chunks.push(format!(
                "commit refs/heads/{branch}\ncommitter test <test@example.com> {tick} +0000\ndata {message_len}\n\
                 {message}\n{from}M 100644 inline bulk/{n}.txt\ndata {content_len}\n{content}\n",
                tick = self.tick,
                message_len = message.len(),
                content_len = content.len(),
            ));
        }
        let stream = chunks.concat();
        let mut child = Command::new("git")
            .arg("-C")
            .arg(&self.repo)
            .args(["fast-import", "--quiet"])
            .stdin(std::process::Stdio::piped())
            .spawn()
            .expect("spawn fast-import");
        child
            .stdin
            .take()
            .expect("fast-import stdin")
            .write_all(stream.as_bytes())
            .expect("write fast-import stream");
        let status = child.wait().expect("wait fast-import");
        assert!(
            status.success(),
            "fast-import of {count} commits on {branch}"
        );
    }

    /// Starts a rebase of `worktree`'s branch onto `onto` that must stop on a conflict.
    pub fn start_conflicting_rebase(&mut self, worktree: &Path, onto: &str) {
        let rebase = self.try_git_in(worktree, &["rebase", onto]);
        assert!(
            !rebase.status.success(),
            "rebase onto {onto} in {} must stop on a conflict",
            worktree.display()
        );
    }

    /// Starts a cherry-pick of `sha` in `worktree` that must stop on a conflict.
    pub fn start_conflicting_pick(&mut self, worktree: &Path, sha: &str) {
        let pick = self.try_git_in(worktree, &["cherry-pick", sha]);
        assert!(
            !pick.status.success(),
            "cherry-pick of {sha} in {} must conflict",
            worktree.display()
        );
    }

    /// `git bisect start <bad> <good>` in `worktree` (detaches its HEAD).
    pub fn start_bisect(&mut self, worktree: &Path, bad: &str, good: &str) {
        self.git_in(worktree, &["bisect", "start", bad, good]);
    }

    /// Writes a registry file for this test's own pid (alive) with `cwd` = the worktree.
    pub fn claude_session(&self, worktree: &Path, name: &str) -> String {
        let id = format!("session-{}", name.replace(' ', "-"));
        let pid = std::process::id();
        let json = serde_json::json!({
            "pid": pid,
            "sessionId": id,
            "cwd": worktree,
            "kind": "interactive",
            "entrypoint": "claude-desktop",
            "hostSessionId": "local_test",
            "name": name,
            "status": "busy",
            "updatedAt": 1_790_289_427_789_i64,
        });
        let path = self
            .claude_home
            .join("sessions")
            .join(format!("{pid}.json"));
        std::fs::write(path, serde_json::to_vec(&json).expect("json")).expect("write session");
        id
    }

    /// Writes a Codex rollout whose `session_meta.cwd` is `worktree`.
    pub fn codex_session(&self, worktree: &Path, id: &str) {
        let dir = self
            .codex_home
            .join("sessions")
            .join("2026")
            .join("09")
            .join("25");
        std::fs::create_dir_all(&dir).expect("mkdir rollouts");
        let line = serde_json::json!({
            "timestamp": "2026-09-25T00:00:00.000Z",
            "type": "session_meta",
            "payload": {
                "id": id,
                "cwd": worktree,
                "originator": "Codex Desktop",
                "cli_version": "0.153.4",
                "timestamp": "2026-09-25T00:00:00.000Z",
                "git": { "commit_hash": "0000" }
            }
        });
        let path = dir.join(format!("rollout-2026-09-25T00-00-00-{id}.jsonl"));
        std::fs::write(path, format!("{line}\n")).expect("write rollout");
    }

    /// Runs the built binary from the main checkout with `args`, homes pointed at the fixture.
    pub fn run(&self, args: &[&str]) -> Output {
        self.run_in(&self.repo, args)
    }

    /// Runs the built binary with its working directory at `cwd` and the default `--repo .` — the way
    /// `/close` invokes it from inside the session worktree.
    pub fn run_in(&self, cwd: &Path, args: &[&str]) -> Output {
        let claude_home = self.claude_home.to_str().expect("utf8");
        let codex_home = self.codex_home.to_str().expect("utf8");
        Command::new(env!("CARGO_BIN_EXE_agentwt"))
            .current_dir(cwd)
            .args(["--claude-home", claude_home, "--codex-home", codex_home])
            .args(args)
            .output()
            .expect("spawn agentwt")
    }

    /// `status --format json` parsed, panicking on a non-zero exit.
    pub fn status(&self) -> Value {
        let output = self.run(&["--format", "json", "status"]);
        assert!(
            output.status.success(),
            "status failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        serde_json::from_slice(&output.stdout).expect("status json")
    }
}

/// Writes `content` at `root/rela`, creating parents.
pub fn write_at(root: &Path, rela: &str, content: &str) {
    let path = root.join(rela);
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).expect("mkdir");
    }
    std::fs::write(path, content).expect("write");
}

/// The row of `branch` in a status document.
pub fn row<'a>(document: &'a Value, branch: &str) -> &'a Value {
    document["rows"]
        .as_array()
        .expect("rows")
        .iter()
        .find(|row| row["branch"].as_str() == Some(branch))
        .unwrap_or_else(|| panic!("no row for {branch} in {document}"))
}

/// The row's containment as `(branch, reason, rank)`, `None` when the work lives nowhere.
pub fn containment(row: &Value) -> Option<(String, String, String)> {
    let c = &row["containment"];
    if c.is_null() {
        return None;
    }
    let field = |key: &str| c[key].as_str().expect(key).to_owned();
    Some((field("branch"), field("reason"), field("rank")))
}
