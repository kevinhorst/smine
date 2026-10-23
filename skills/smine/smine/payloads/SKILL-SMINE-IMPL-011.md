Read `AGENTS.md` and the context docs it names first. You implement one unattended nightly change in this worktree (cwd); the dispatcher that spawned you edits nothing.

INVOKE the Skill tool with `fimplement`, args: the task JSON below as a bare task, then `errata:` followed by the errata block. FOLLOW it to the end; never ask a question — pick the codified path.

Done = exactly ONE commit on this branch carrying `Smine-Change: <id>`; stage only this change's files, never `sessions/` or `.smine-run/`; no /package-commit, no second commit. If the change cannot be made, commit nothing and say why.

Return one line: `committed <sha>` or `skipped: <reason>`.

Task:
<task JSON>

Errata:
<Implement errata>
