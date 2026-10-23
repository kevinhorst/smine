//! Git-changing Bash commands — a trigger of the git edge (D7). A command changes git state unless every `git`
//! invocation in it runs a read-only subcommand: the inverse of tool-policy's read-only list, so a verb missing
//! from the list costs one coalesced scan, never a stale state. No shell parser: quotes may split a segment, which
//! only ever adds a trigger.

/// Read-only subcommands with their required leading arguments; mirrors the `gate.read_only_commands` default
/// (`internal/components/settings_rows.go`).
const READ_ONLY: &[&[&str]] = &[
    &["blame"],
    &["cherry"],
    &["config", "--get"],
    &["describe"],
    &["diff"],
    &["grep"],
    &["log"],
    &["ls-files"],
    &["ls-tree"],
    &["merge-base"],
    &["remote", "-v"],
    &["rev-list"],
    &["rev-parse"],
    &["shortlog"],
    &["show"],
    &["stash", "list"],
    &["status"],
    &["tag", "-l"],
    &["worktree", "list"],
];

/// `branch` flags that create, move, copy or delete (every positional name does too).
const BRANCH_CHANGING: &[&str] = &[
    "-d",
    "-D",
    "-m",
    "-M",
    "-c",
    "-C",
    "-f",
    "-u",
    "--delete",
    "--move",
    "--copy",
    "--force",
    "--set-upstream-to",
    "--unset-upstream",
    "--edit-description",
];

/// Global flags that take the next word as their value.
const VALUE_FLAGS: &[&str] = &["-C", "-c", "--git-dir", "--work-tree", "--namespace"];

/// `true` when `command` runs at least one git subcommand that can move a ref, the index or the worktree.
#[must_use]
pub fn changes_git(command: &str) -> bool {
    command
        .split(['\n', ';', '|', '&'])
        .filter_map(|segment| git_args(&segment.split_whitespace().collect::<Vec<_>>()))
        .any(|args| !is_read_only(&args))
}

/// The words after `git` and its global flags, when this segment runs git.
fn git_args<'a>(words: &[&'a str]) -> Option<Vec<&'a str>> {
    let mut words = words.iter().copied().skip_while(|word| is_assignment(word));
    let program = words.next()?;
    if program != "git" && !program.ends_with("/git") {
        return None;
    }
    while let Some(word) = words.next() {
        if VALUE_FLAGS.contains(&word) {
            words.next();
            continue;
        }
        if word.starts_with("--git-dir=")
            || word.starts_with("--work-tree=")
            || word.starts_with("--namespace=")
        {
            continue;
        }
        if matches!(
            word,
            "--no-pager" | "-P" | "-p" | "--paginate" | "--no-replace-objects"
        ) {
            continue;
        }
        return Some(std::iter::once(word).chain(words).collect());
    }
    Some(Vec::new())
}

fn is_read_only(args: &[&str]) -> bool {
    match args.split_first() {
        None => true,
        Some((&"branch", rest)) => rest
            .iter()
            .all(|arg| arg.starts_with('-') && !BRANCH_CHANGING.contains(arg)),
        Some(_) => READ_ONLY.iter().any(|prefix| args.starts_with(prefix)),
    }
}

/// `NAME=value` with a shell-variable name.
fn is_assignment(word: &str) -> bool {
    word.split_once('=').is_some_and(|(name, _)| {
        !name.is_empty() && name.chars().all(|c| c == '_' || c.is_ascii_alphanumeric())
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn changes_git_cases() {
        let cases = [
            ("commit", "git commit -m x", true),
            ("status-with-dir", "git -C /r status", false),
            ("chain-with-pull", "git status && git pull", true),
            ("env-prefix-log", "FOO=1 git log", false),
            ("stash", "git stash", true),
            ("stash-list", "git stash list", false),
            ("branch-list", "git branch -a", false),
            ("branch-create", "git branch new", true),
            ("worktree-add", "git worktree add p", true),
            ("absolute-reset", "/usr/bin/git reset --hard", true),
            ("not-git", "make audit", false),
            ("bare-git", "git", false),
        ];
        for (label, command, expected) in cases {
            assert_eq!(changes_git(command), expected, "{label}");
        }
    }
}
