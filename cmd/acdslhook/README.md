# acdslhook

The Read hook of ACDSL repos, wired once in the Claude Code settings template on PostToolUse(Read): it delivers the governing rules of the file just read as additional context. The file on disk is never touched; the on-disk projection (`acdsl project -file`) is a CLI capability for consumers outside Claude.

## Usage

```
Usage of acdslhook:
  -claude-home string
    	Claude Code home directory (default ~/.claude)
```

- Runs `acdsl project -context <rel>` and returns its lines as `additionalContext` under the `acdslhook.context_text` header (default `ACDSL rules governing <rel>:`); nothing when the file is ungoverned or every rule on it is gate-only.

## How it works

**Which acdsl runs**
- The session repo's own build: the first executable `bin/<name>` of `acdslhook.binaries` (default `acdsl`, `acdsl.exe`), else `go run ./cmd/acdsl` while `acdslhook.go_run_fallback` holds (default) — a repo is judged by the version it vendors.
- Each call runs in the session cwd under `acdslhook.timeout_ms` (default 14 seconds); a failed or timed-out call yields no output.

**When it does nothing**
- The hook never blocks a read. It exits silently when the envelope has no file path or cwd, the cwd has no `acdsl/registry.json`, the file lies outside the cwd, or the acdsl call fails.

**Configuration**
- Reads `~/.claude/hooks/config.json` (`internal/hookconfig`): the `acdslhook` section (`binaries`, `go_run_fallback`, `timeout_ms`, `context_text`) — the Context system's `acdsl_*` settings in `docs/settings.html`. A missing or malformed file is a hook error naming the path.
- Disable: park the hook group on the Hooks tab for every session, or `#disable inject:acdsl` for one session (`internal/sessionstate`, read before acting; an unreadable record counts as on).

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | done or nothing to do; the context JSON is on stdout when there is one |
| 1 | failure (undecodable envelope, home dir unresolvable, hooks config unreadable, encode error) |
| 2 | usage error |
