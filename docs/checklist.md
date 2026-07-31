# Claude Desktop (Chat/Code) — Workflow Changes

> **Scope** — Claude Desktop (Chat and Code modes of the macOS app).
> **Out of scope** — Claude Code CLI, Claude Web, Codex, ChatGPT.

Each entry follows a fixed shape:

| Field | Meaning |
| :--- | :--- |
| **Workflow** | The situation I'm in when the problem shows up |
| **Problem** | What goes wrong, or what I lose |
| **Solution** | The change I made (or intend to make) |

**Status tags:** `[Done]` in place · `[Ongoing]` working pattern, no single switch · `[Manual]` works but not automated · `[Untested]` speculative, unverified.

---

## Overview

| # | Change | Theme | Status |
| :--- | :--- | :--- | :--- |
| 1 | Audible cue on permission prompts | Reduce idle time | `[Done]` |
| 2 | Goland MCP in a worktree setup | Correctness | `[Done]` |
| 3 | Parallel work via git worktrees | Reduce idle time | `[Done]` |
| 4 | Session rot / context management | Session durability | `[Done]` |
| 5 | Lightweight model for review & triage | Reduce idle time | `[Done]` |
| 6 | Cross-vendor handoff (Claude → Codex) | Capacity & strengths | `[Done]` |
| 7 | Settings allowlist hygiene | Housekeeping | `[Done]` |
| 8 | Worktree awareness degradation | Correctness | `[Done]` |
| 9 | Subagent context loss (AGENTS.md / CLAUDE.md) | Correctness | `[Untested]` |
| 10 | Hook management (no matching on UserPromptSubmit) | Ergonomics | `[Open]` |
| 11 | Nightly routine management | Automation | `[Done]` |
| 12 | Open-source tool exploration | Tooling | `[Open]` |
| 13 | Skill extraction | Automation | `[Done]` |
| 14 | Worktrees pollute GoLand recent-projects list | Housekeeping | `[Done]` |
| 15 | Desktop worktree pool corrupts repos (main-checkout hijacks) | Correctness | `[Done]` |
| 16 | Custom system prompt — harness contradicts skill contracts | Correctness | `[Done]` |
| 17 | Codex as subagent in Claude Code (push direction) | Capacity & strengths | `[Open]` |
| 18 | Claude telemetry → config server (tool/skill efficacy stats) | Tooling | `[Open]` |
| 19 | ccusage for overview spend stats | Tooling | `[Open]` |
| 20 | Test dynamic workflows (output-gated / iterative shapes) | Tooling | `[Open]` |

---

## 1. Audible cue on permission prompts &nbsp; `[Done]`

**Workflow**
Dispatch a task to Claude Code and context-switch to other work for a round (20–30 min).

**Problem**
Claude hits a tool that needs permission. The prompt sits unanswered. The task stalls silently until I happen to glance back at the window.

**Solution**
Hook on `Notification` and `PermissionRequest` in `~/.claude/settings.json`, running `afplay /System/Library/Sounds/Funk.aiff` async.

- Fires in Claude Desktop **Code** mode.
- Claude Desktop **Chat** mode ignores hooks entirely — different subsystem.

---

## 2. Goland MCP in a worktree setup &nbsp; `[Done]`

**Workflow**
Work in a git worktree. Main repo and worktree are both open as separate projects in GoLand. Ask Claude to use the goland MCP for file reads, symbol lookups, or DB schema.

**Problem**
The goland MCP plugin routes tool calls by TCP port, first-bind-wins. The `projectPath` argument is:

- **dropped entirely** on the SSE transport — [IJPL-242972](https://youtrack.jetbrains.com/issue/IJPL-242972),
- **cosmetic even when present** — [IJPL-214287](https://youtrack.jetbrains.com/issue/IJPL-214287).

Consequence: project-tree reads silently return content from the wrong GoLand window. Editorial calls (`rename_refactoring`, `replace_text_in_file`) land in the wrong tree without warning.

Upstream feature request for configurable per-project port: [IJPL-207839](https://youtrack.jetbrains.com/issue/IJPL-207839) — still broken as of 2026-06-29.

**Solution**
Narrow the allowlist to **eight** database-read tools where failure is loud ("connection not found") rather than silent ("here's stale data from the wrong project"):

- `list_database_connections`, `list_database_schemas`, `list_schema_object_kinds`, `list_schema_objects`
- `get_database_object_description`, `preview_table_data`, `test_database_connection`, `cancel_sql_query`

`execute_sql_query` → `ask` list, never blanket-allow.
Every project-tree read and every editorial tool → denied.
Rule codified in `feedback_no_goland_mcp.md`.

**Update 2026-06-29.** Could theoretically be used for refactors against develop/feature branches, but untested in a real production scenario. Blocked on JetBrains fixing the port routing.

---

## 3. Parallel work via git worktrees &nbsp; `[Done]`

**Workflow**
Several tasks queued up — a bug, a refactor, a docs pass. Want Claude to work on them in parallel instead of sequentially.

**Problem**
Two Claude sessions in the **same** checkout trip over each other:

- Uncommitted edits collide.
- Mid-session branch switches corrupt in-progress state.
- Review is impossible when files mutate under me.

Serial dispatch wastes 20–30 min of idle time per task, waiting for one to finish before starting the next.

**Solution**
One `git worktree` per task. Each Claude session lives in its own working directory on its own branch — zero file-level collision.

- Dispatch 3–4 tasks concurrently.
- Keep reviewing and merging in the main checkout, unaffected.
- Idle time drops to near zero.
- The `Agent` tool's `isolation: worktree` option automates setup and auto-cleans worktrees whose agents made no changes.

**Resolved.** Worktree status tracking and cleanup scripts (`sync_worktrees.sh`) are in place. See item 8 for remaining worktree-awareness issues and item 15 for the Desktop pool corrupting the worktrees themselves.

---

## 4. Session rot / context management &nbsp; `[Done]`

**Workflow**
Long complex tasks that span many rounds, or a single very dense session — design + implementation + verification across hours of work in one Claude Code session.

**Problem**
Sessions "rot". As the context window fills toward auto-compaction, quality degrades — output gets vaguer, prior decisions get misremembered, files edited earlier in the session are re-read with a partial picture. Compaction itself summarises lossily: information that felt pinned just disappears. The remaining-context indicator in the Code UI tells me *when* compaction is coming; it doesn't preserve anything across it.

**Solution**

- **Watch the UI's context indicator.** It's there — use it to time resets deliberately, not to coast into auto-compaction.
- **peek-mcp** — the key tool. Snapshots session state so key details can be rehydrated in a fresh session after a reset.
- **Periodic manual reset.** On long / complex tasks, end and restart the session before rot sets in, rather than riding it to auto-compaction. Carry forward only what matters.
- **`/context`** — on-demand breakdown of which categories are eating the budget (tool definitions, memory, transcript). Useful diagnostic when the indicator starts dropping fast.
- **Disable auto compaction** for maximum control over when resets happen.

**Resolved.** peek-mcp tested and working. Key finding: reset at **150–180k tokens** of a 200k context window — don't wait for auto-compaction. Disable auto compaction entirely for maximum control.

---

## 5. Lightweight model for review & triage &nbsp; `[Done]`

**Workflow**
A Claude Code session on Opus produces a large artefact — a big diff, a long analysis, a refactoring plan. I want to ask clarifying questions, spot-check logic, or debug a small issue in the output.

**Problem**

- Opus roundtrips are slow (10–30s each) for what is effectively trivial Q&A.
- Follow-ups burn tokens and eat the heavy session's context budget.
- They also interrupt the main task's focus — the agent has to context-switch between "do the work" and "explain the work".

**Solution**
Hand the output to a lighter model in a separate window.

- Claude Desktop **Chat** on Sonnet/Haiku, or a second Claude Code session pinned to a cheaper model.
- Heavy session keeps grinding uninterrupted; reviewer gives near-instant feedback.

Good fits for the light reviewer:

- Sanity-checking a diff.
- Restating a plan in my own words.
- Debugging a single failing test.
- Triaging between three options.

**Resolved.** Opus → GPT 5.5 Low handoff works great — better than Claude → Claude for any model combination (usually too slow). Codex MCP integration is fantastic: essentially no token limit, making it ideal for the heavy-output review role. See item 6 for the full cross-vendor pattern.

---

## 6. Cross-vendor handoff — Claude plans, Codex executes &nbsp; `[Done]`

**Workflow**
Use Claude Desktop Code for the plan (heavy reasoning, design, trade-off analysis), then hand the plan to Codex to execute the implementation.

**Problem**

- Single-vendor lock-in forgoes playing each vendor's strengths to the task.
- Capacity and rate limits are per-vendor — a second agent on a second account doubles the ceiling.
- Parallel work across vendors isn't yet wired up in my workflow.

**Solution**
Claude writes a self-contained plan (file paths, line numbers, intent, done-criteria) that Codex can pick up cold. `AGENTS.md` is the shared contract — Codex reads it too, so repo conventions carry across.

**Resolved.** Works as described. Codex MCP integration makes the handoff seamless — essentially no token limit on the Codex side. The Opus → GPT 5.5 Low pattern is the validated combo (see item 5).

*Clarification (2026-07-19, item 17):* "Codex MCP integration" here means peek-mcp configured **inside** Codex — a pull channel where Codex fetches heavy Claude output. There is no Codex server on the Claude side; the push direction is item 17.

---

## 7. Settings allowlist hygiene &nbsp; `[Done]`

**Workflow**
Over the course of a session, approving tool calls accumulates entries in `.claude/settings.local.json` — dozens of narrow, one-off rules like `Bash(some-specific-command)` or MCP tool accepts.

**Problem**

- File bloats with redundant and stale entries.
- Reusable permissions end up trapped in a gitignored worktree file instead of `~/.claude/settings.json`.
- Memory-rule violations (banned MCP tools) sneak back in.

**Solution**
Periodic cleanup pass:

1. Drop one-offs (`find …`, `awk …`, specific scripts that ran once).
2. Promote genuinely reusable entries to the global config.
3. Cross-check every entry against standing memory rules.
4. Keep the worktree file minimal — it's scratch space, not configuration.

**Resolved.** Managed via this repo (`smine`). Settings are version-controlled and synced.

---

## 8. Worktree awareness degradation &nbsp; `[Done]`

**Workflow**
Dispatch Claude Code into a git worktree. Expect it to know which worktree it's in and operate on the correct files.

**Problem**
Claude frequently loses track of which worktree it's operating in. Despite the worktree path being present in the system context, tool calls (especially `Read`, `Bash`) drift to the main checkout or use wrong paths. This requires a manual "on your worktree" follow-up to re-anchor the session — unreliable and tedious.

The degradation is not well understood. The worktree path is injected into the system prompt, but it doesn't stick across rounds, especially after compaction or when subagents are involved.

**Solution**
Open. Needs investigation into why the worktree context degrades and how to make it durable.

**Update 2026-07-15.** One slice of this is now explained and fixed: sessions placed in a *hollow* pool directory (no `.git` entry) had every git command silently resolve to the main checkout — the "drift" was real, not just model confusion. See item 15; the guard hook now blocks such sessions at the first prompt.

---

## 9. Subagent context loss (AGENTS.md / CLAUDE.md) &nbsp; `[Untested]`

**Workflow**
Main agent spawns subagents (via `Agent` tool or workflows) to parallelize work. Repo has `AGENTS.md` and `CLAUDE.md` with project conventions, Go version constraints, and breaking-change notes.

**Problem**
The main agent is properly rooted — it reads `CLAUDE.md` and `AGENTS.md` and knows the project conventions. But spawned subagents start fresh and frequently ignore these files. Concrete example: Go 1.26.0 introduced a `new(expr)` breaking change documented in the repo's context files. Subagents don't read it, assume the codebase doesn't compile, and trigger unnecessary verification runs or further subagent spawns. The main agent then wastes tokens re-checking work that was fine.

Adding Go release notes to the repo context helps the main agent but doesn't propagate to subagents, because they don't read it.

**Solution**
The preamble convention is now implemented in the `AGENTS.md` template (`context/AGENTS.md`, "Subagents" section): any subagent prompt must instruct the subagent to read `AGENTS.md` and the context dir first, or inline the constraints that matter for its task. Manual — it relies on the main agent honoring the rule; there is no automated enforcement.

Remaining ideas if the manual rule proves unreliable:

- Investigate whether Claude Code's subagent spawning can be configured to auto-inject `CLAUDE.md` content.

---

## 10. Hook management — no matching on UserPromptSubmit &nbsp; `[Open]`

**Workflow**
Use `UserPromptSubmit` hooks in `settings.json` to inject context (e.g., `go.mod` review context) on every prompt.

**Problem**
`UserPromptSubmit` hooks fire on **every** prompt with no matching or filtering. There is no way to conditionally enable/disable a hook without manually editing `settings.json` and restarting Claude. This makes experimentation painful and clutters the workflow when a hook is temporarily unwanted.

The repo exposes a server to make toggling easier, but the UX is still poor compared to a proper matching/toggle mechanism.

**Solution**
Open. Waiting on Anthropic to add event matching to hooks. Workaround: manual `settings.json` editing + Claude restart (assumed required, needs testing).

---

## 11. Nightly routine management &nbsp; `[Done]`

**Workflow**
Want automated nightly routines — e.g., use peek-mcp to analyze the day's sessions, generate summaries, flag patterns, clean up stale worktrees.

**Problem**
Anthropic's scheduled tasks (cloud routines) run in a cloud environment, not locally. They cannot access local MCP servers (like peek-mcp), local files, or local git state. This makes them unusable for workflows that depend on local tooling.

**Solution**
Open. Leading idea: `claude code -p` with the `/loop` command to create a locally-running daemon that executes on a schedule. Needs management infrastructure (process supervision, logging, error handling).

---

## 12. Open-source tool exploration &nbsp; `[Open]`

**Workflow**
Evaluate new open-source tools (MCP servers, CLI utilities, analysis tools) for potential workflow improvements.

**Problem**
Recurring concerns when evaluating new tools:

- **MCP tool context bloat** — each MCP server adds tool definitions to the context window, eating token budget even when unused.
- **Dependency heaviness** — many tools pull in Python/Node ecosystems with large dependency trees.
- **Bloatware** — tools that do too much, poorly, rather than one thing well.
- **Security** — running third-party MCP servers means trusting them with file system access and potentially credentials.

**Interesting leads (2026-06-29):**

- Claude usage analyzers — tools for understanding token spend and session patterns.
  - GitHub: https://github.com/ryoppippi/ccusage
  - Seems good!
- peek-mcp alternatives — other approaches to cross-session context sharing.

**Interesting leads (2026-07-05):**

- [serena](https://github.com/oraios/serena) — semantic code agent tools / MCP server for code navigation and editing.

---

## 13. Skill extraction &nbsp; `[Done]`

**Workflow**
Recurring patterns emerge across sessions — things Claude is asked to do repeatedly, in the same way, with the same structure.

**Problem**
Without extraction, these patterns live only in conversation history. Each session rediscovers the same approach from scratch, burning context and producing inconsistent results. Common failure modes: slightly different prompting, missing steps, degraded output quality.

**Solution**
Extract recurring patterns into skills (`.claude/skills/`). A skill is a self-contained, reusable prompt fragment that can be invoked with a slash command, keeping the session context lean and the pattern consistent across uses.

**Status:** Ongoing. Extracted skills are tracked in `skills/`.

---

## 14. Worktrees pollute GoLand recent-projects list &nbsp; `[Done]`

**Workflow**
Every Claude Code session runs in its own worktree at `<repo>/.claude/worktrees/<name>`. GoLand is open on the parent repos.

**Problem**
GoLand 2026.1's native git-worktree integration ([IJPL-204771](https://youtrack.jetbrains.com/issue/IJPL-204771)) auto-registers each worktree of an open repo as a separate project. The Welcome-screen Recent Projects list fills with dozens of `.claude/worktrees/*` entries across every repo (44 of 50 entries on my machine). Worktrees also sit *inside* the project dir — the layout JetBrains warns against, misidentified as multi-root. I remove them one by one; it recurs every session.

No native fix: JetBrains has no pattern/glob exclusion for Recent Projects. `Settings | Directories` and `File Types | Ignore files and folders` don't touch the welcome-screen list. Relocating worktrees isn't an option — the path is a Claude Code harness convention.

**Solution**
`cmd/worktrees/prune_jetbrains_recent_projects.sh` strips `<entry key="...worktrees...">` blocks from every `~/Library/Application Support/JetBrains/GoLand*/options/recentProjects.xml`, backing up each file first. Run it with **GoLand closed** — GoLand rewrites the file on exit, so a live edit gets clobbered; the script refuses to run while GoLand is up unless `--force` is passed, and `-n/--dry-run` previews. Not yet automated: candidate to fold into `remove_agent_worktrees.sh` or a periodic cleanup, but the GoLand-must-be-closed constraint rules out a plain Claude Code hook.

---

## 15. Desktop worktree pool corrupts repos — main-checkout hijacks &nbsp; `[Done]`

**Workflow**
Every Desktop session gets a pooled worktree under `<repo>/.claude/worktrees/`. I keep working in the main checkout alongside — reviewing, merging, switching branches.

**Problem**
Claude Desktop pools worktree directories and reuses "free" ones for new sessions without validating they are real git worktrees. Diagnosed 2026-07-15 in peek-mcp (reflog-proven): the pool handed two *hollow* directories (no `.git` entry, leftovers since June) to new sessions and ran the session-branch checkout in them — git's upward discovery resolved to the **main checkout** and switched its HEAD to `claude/*` branches under my feet, twice within minutes. Agents in hollow dirs silently operate on the main repo; every branch switch of mine yanked their branch, every checkout of theirs trashed my state.

Upstream landscape, statuses verified via `gh issue view` on 2026-07-15 (never trust WebFetch/agent claims about issue state):

- [#27044](https://github.com/anthropics/claude-code/issues/27044) `--worktree` creates no git worktree — **closed** (completed 2026-02), yet the hollow dirs it left behind stayed live ammunition for the pool.
- [#39924](https://github.com/anthropics/claude-code/issues/39924) Desktop switches branch on new session — **closed as not planned**.
- [#45737](https://github.com/anthropics/claude-code/issues/45737) cleanup leaves `.claude/worktrees/` dirs behind — **closed** (completed 2026-04).
- [#29716](https://github.com/anthropics/claude-code/issues/29716) Desktop bypasses `WorktreeCreate`/`WorktreeRemove` hooks — **open**. Owning worktree creation via hook is therefore impossible; the CLI honors these hooks, Desktop does not.
- [#77268](https://github.com/anthropics/claude-code/issues/77268) recycling destroys live sibling sessions' worktrees, **including locked ones and uncommitted work** — **open**. Means `git worktree lock` and dirty state are *not* reliable protection against the pool.
- [#77506](https://github.com/anthropics/claude-code/issues/77506) pool maintenance sweep **detached the MAIN repository's HEAD**; worktree removal is non-atomic — **open**. A main-checkout mutation vector that never touches a hollow dir's cwd.
- [#76144](https://github.com/anthropics/claude-code/issues/76144) pool writes `.git/worktrees/<name>/gitdir` as literal `.git`, flagging healthy worktrees prunable; dormant ones get reclaimed/deleted — **open**. Plausible husk factory: this is how real worktrees degrade into the hollow dirs the pool later reuses.
- [#63317](https://github.com/anthropics/claude-code/issues/63317) docs don't clarify `worktree.baseRef: "head"` semantics inside linked worktrees — **open**. Relevant since `baseRef: "head"` is set globally here.

**Solution**
Defense stack in `smine` (2026-07-15), designed so nothing depends on the pool behaving:

- **Pool decoy** — `install_pool_guard.sh` writes a `.git` file with a dead `gitdir:` pointer at each pool root. Git discovery from any hollow dir stops there and fails loudly (exit 128) instead of reaching the main checkout. Structural kill for every git *subprocess* running with cwd in a broken pool dir.
- **Sweep** — `worktree-sessionstart.sh` (SessionStart hook, works in Desktop) deletes verified husks at every session start, so the pool has nothing dangerous to reuse; also self-installs the decoy per repo.
- **Hardening** — same hook: `.idea/vcs.xml` per worktree (GoLand), untracked `.claude-worktree` sentinel + `git worktree lock` to make worktrees look non-free. Best-effort only given #77268.
- **Tripwire** — `worktree-guard.sh` (UserPromptSubmit) blocks the first prompt of any session sitting in a hollow/broken pool dir.
- **`worktree.baseRef: "head"`** — session branches fork from my checked-out state, not `origin/HEAD`.

Known gap: #77506-style operations that target the main `.git` directly (not via cwd in the pool) bypass the decoy — bounded to loud, recoverable damage, not silently preventable from outside the app. Candidate hardening for #76144: have the SessionStart hook verify/repair `.git/worktrees/<name>/gitdir` admin files. Live desktop verification of the full stack still pending (hooks activate on merge + sync).

---

## 16. Custom system prompt — harness contradicts skill contracts &nbsp; `[Done]`

**Workflow**
Invoke a planning skill (`/fdesign`, `/fchange`) in a Desktop plan-mode session. The skill's contract is explicit: the output is a plan, not code. Skills are the trust anchor for increasingly autonomous workflows — their output contract must hold unconditionally.

**Problem**
The harness injects instructions that contradict the skill contract, and none of them are adjustable:

- The plan-mode system-prompt block prescribes its own Phase 1–5 workflow ending in `ExitPlanMode`.
- The `ExitPlanMode` tool result is hardcoded: *"User has approved your plan. You can now start coding."* — a canned template, not the user's words.
- The generic preset pushes act-don't-ask autonomy ("when you have enough information to act, act").
- The preset mandates a commit trailer — *"End git commit messages with: Co-Authored-By: Claude …"* — directly contradicting `package-commit`'s "No Co-authored by Claude" rule. Observed 2026-07-15 in the same session: the skill happened to win, but the resolution is probabilistic, same class as the plan-mode case.

Incident 2026-07-15: a `/fdesign` run for secretscan implemented the entire feature (package, CLI, tests, wiring) immediately after plan approval — the model resolved the contradiction silently in favor of the harness template. Root cause traced in-session: not a subagent, not context loss; the SKILL.md was in context the whole time.

Verified via docs (2026-07-15): the ExitPlanMode text and the plan-mode injection are hardcoded — no surface in `settings.json`, hooks, env vars, or output styles. Hooks and CLAUDE.md/SKILL.md lines can only *add* counter-instructions, which keeps contradicting instructions in context — unacceptable when the failure rate merely drops rather than disappears; autonomous chains cannot tolerate probabilistic skill contracts. Scope note: plan mode / `ExitPlanMode` do not exist in headless (`claude -p`) or the Agent SDK (`permissionMode` has no `"plan"`), so this specific contradiction is interactive-only — but the over-generalized preset itself remains in all modes.

**Solution**
Replace, don't counterbalance: write a custom system prompt for this repo and drop the generalized preset (full replacement via custom output style, or SDK `systemPrompt` as string). The preset is generalized too heavily for this workflow anyway; a purpose-built prompt has ripple effects across all sessions — intended. If Desktop cannot do a full system-prompt replacement, switch to the CLI, where it can. Open: author the prompt, decide the Desktop-vs-CLI split, wire it into `smine` sync.

---

## 17. Codex as subagent in Claude Code — push direction &nbsp; `[Open]`

**Workflow**
Mid-session, Claude Code should delegate arbitrary tasks (implement, review, research) to Codex and get results back — including the validated Opus → GPT 5.5 Low heavy-output review handoff, without switching apps.

**Problem**
The push direction (Claude invoking Codex) has never existed. The "Codex MCP integration" in items 5–6 is peek-mcp configured *inside* Codex (`settings/codex/config.toml`, global `tool_output_token_limit=125000`) — the validated handoff is **pull-based**: Codex, as reviewer, pulls heavy Claude output via its own peek-mcp. No Codex server exists on the Claude side, repo or live.

**Findings** (investigation 2026-07-19, run `wf_1f743aad-5bd`; full baseline + refuted-hypotheses register in `sessions/investigations/merged-baseline.md`, per-run artifacts `inv-{1,2,3}.md`):

- **Codex surfaces (codex-cli 0.144.4, live-probed):** `codex mcp-server` (stdio) exposes exactly two tools — `codex` (prompt, model, cwd, sandbox, approval-policy, config) and `codex-reply` (threadId continuity, working since rust-v0.81.0; `conversationId` is a deprecated alias). `codex mcp` (no hyphen) manages Codex's *own* external MCP servers — there is no `codex mcp serve`. `codex exec` supports `--json`, `-o <file>`, `resume --last`, `-C`, `-c` overrides. Auth is a non-issue everywhere: all surfaces load `~/.codex/auth.json` + `config.toml`.
- **Official plugin exists:** `openai/codex-plugin-cc` (29k stars, current). Ships `/codex:{review,adversarial-review,rescue,transfer,status,result,cancel,setup}` and a `codex-rescue` delegation subagent — one Bash call to a `codex app-server` JSON-RPC broker, **no MCP in the delegation path**. Background jobs with on-disk results, `--model/--effort` routing, `/codex:transfer` imports the Claude transcript into a resumable Codex thread, workspace root = git toplevel of cwd (worktree-correct).
- **MCP-route choke points (verified):** Claude caps MCP results at 25k tokens by default; a 30-min stdio idle timeout aborts calls whose server sends no standard progress notifications — and `codex mcp-server` emits only custom `codex/event` notifications, zero standard ones (live probe); subagent MCP calls are never auto-backgrounded. The `codex` tool blocks for the whole turn; no status/cancel tools.
- **Plugin constraints (verified):** plugin subagents ignore `mcpServers`/`hooks`/`permissionMode` frontmatter — a plugin cannot bundle an MCP-bound agent (why OpenAI chose Bash forwarding). Desktop lacks `/plugin` ([#42142](https://github.com/anthropics/claude-code/issues/42142) open) and the plugin is not in the official marketplace — install goes via CLI.
- **Dead ends (refuted, don't re-investigate):** third-party Codex-bridge MCP servers (existed to patch the `conversationId` bug, fixed upstream; strictly dominated by first-party surfaces); the per-server `env` placement of `MAX_MCP_OUTPUT_TOKENS` as proven-working (the peek-mcp entry has never been exercised by an oversize output; docs only document it as a client-process variable).
- **Recommendation (pending the checks below):** plugin-first hybrid — `codex-plugin-cc` as the delegation vehicle (results ride Bash stdout + files, sidestepping all MCP choke points); review handoff = `/codex:review --effort low` with Codex still pulling context via its own peek-mcp (pull channel preserved). `codex mcp-server` stays as the scriptable side-channel; promote to primary if the Desktop plugin test fails.

**Needs manual verification**

1. **Decisive:** does a CLI-installed `codex-plugin-cc` load in a Desktop worktree session? Install via terminal claude, then check `/codex:setup` + the `codex:codex-rescue` agent in a fresh Desktop session (control: same check in a CLI session). Fails → MCP route becomes primary.
2. Does per-server `env` placement of `MAX_MCP_OUTPUT_TOKENS` raise the cap at all, vs client-process export? A/B with a ~40k-token MCP result (also settles whether the existing peek-mcp entry ever did anything).
3. Does a >30-min `codex` MCP turn survive, or does `codex/event` traffic fail to reset the idle window? (Mitigation arms: per-server `timeout`, `CLAUDE_CODE_MCP_TOOL_IDLE_TIMEOUT=0`.)
4. Does `approval_mode="approve"` on the 7 peek-mcp tools in `config.toml` stall headless Codex runs that pull session context (`codex exec` → `session_latest`)? Determines whether the review flow needs a per-call config override.
5. Do the plugin's SessionStart hook (transcript capture for `/codex:transfer`) and Stop review gate work under Desktop? (Stop gate stays off regardless — usage drain, conflicts with the max-2-attempts rule; cf. Desktop hook gaps, item 15 / [#29716](https://github.com/anthropics/claude-code/issues/29716).)

---

## 18. Claude telemetry → config server — tool/skill efficacy stats &nbsp; `[Open]`

**Workflow**
Compare skills and tools with hard data — is serena worth its context cost vs Grep/Read, which skills burn the most, where does permission friction sit — instead of gut feeling.

**Problem**
No existing tool provides per-tool or per-skill stats. Transcript JSONL carries no success/duration fields, and usage lines aren't tagged with the skill or tool that caused them — transcript-based attribution is heuristic slicing. ccusage's finest grain is the session (see item 19); it never reads message content.

**Solution**
Claude Code's native OTel **events** carry exactly this: `claude_code.tool_result` (`tool_name`, `success`, `duration_ms`, input/result sizes, `mcp_server_scope`), `claude_code.tool_decision` (accept/reject + decision source = permission friction), `claude_code.user_prompt` (`command_name` = skill invocations), `claude_code.api_request` (tokens, duration) — all joinable via `session.id`. Exporters are OTLP/console only, forward-only, so: enable telemetry (`CLAUDE_CODE_ENABLE_TELEMETRY=1`, `OTEL_LOGS_EXPORTER=otlp`, endpoint → localhost) and integrate a minimal OTLP HTTP log-receiver endpoint into the config server (standing local process), flattening events to SQLite/JSONL; stats pages on top. Full findings and open verifications (does `command_name` fire for mid-session Skill invocations? do subagent events carry the parent `session.id`?) in [telemetry.md](telemetry.md).

**Source:** https://code.claude.com/docs/en/monitoring-usage

---

## 19. ccusage for overview spend stats &nbsp; `[Open]`

**Workflow**
Overview usage/spend reporting — daily/weekly/monthly totals, per-session, per-model, 5h billing blocks.

**Problem**
Rolling our own means solving dedup (transcripts repeat identical usage lines up to 5× from streaming rewrites), cache-tier pricing (5m/1h), tiered >200k pricing, and a permanent pricing-table maintenance treadmill — all for a notional number (no `costUSD` in transcripts; subscription billing makes every dollar figure API-equivalent, not billed).

**Solution**
ccusage v20 is a native Rust binary (bottled homebrew-core formula, zero deps — the old "npx-only" objection is obsolete) and already solves all of the above; `--json --offline` on every report. **Installed.** Remaining: concept + design for the config-server integration — a `/usage` page shelling out to `ccusage daily --json --offline` via the existing exec tool-action pattern, ccusage staying the single source of truth. Needs `/concept` → `/fdesign`. Investigation baseline and rejected alternatives in [telemetry.md](telemetry.md).

---

## 20. Test dynamic workflows — output-gated / iterative shapes &nbsp; `[Open]`

**Workflow**
Repo workflows (session-mine, investigation, railroad-review, parallelize, parallel-eval) fan out with a shape fixed before the first agent runs. The Workflow tool contract supports far more: loops and conditionals over agent outputs, threshold-gated spawning, `budget`-scaled fleets, loop-until-dry/convergence, adversarial multi-verifier voting, one-level `workflow()` nesting.

**Problem**
The headroom is largely unexercised (investigation 2026-07-24, `sessions/investigations/baseline-dynamic-workflows.md`): no repo script uses `budget` or any `while` loop — grep-verified; railroad-review v3 is the first to use `pipeline()` (per-direction lane→merge chain). The strongest occupied tier is investigation v1.2's output-dependent verifier shard count, and that code has **never executed**: the deployed `~/.claude` copy is still v1.1 (serial single verifier), proven in-band when the 2026-07-24 study's own verification ran as the v1.1 serial agent. railroad-review's convergence loop now exists as the front-driven station protocol (one workflow invocation per round — human gates cannot live in a workflow), so the workflow itself stays non-T3. Whether the dynamic shapes behave — shard fan-out sizing, resume semantics across a data-dependent prefix, budget-gated loops terminating correctly — is untested.

**Solution**
Open. Steps: (1) sync so the v1.2 sharded Stage B actually deploys, then run a real `/investigation` and compare shard count/wall-time/merge quality against the v1.1 serial baseline; (2) build one deliberately T3 workflow (candidate: loop-until-dry finding rounds) and verify loop termination, dedup-vs-seen convergence, and `resumeFromRunId` behavior when the agent-call prefix is data-dependent; (3) test `budget`-gated scaling with a "+Nk" directive, including the `budget.total === null` guard.

See [workflows.md](workflows.md) "Dynamic shapes" for the tier taxonomy and authoritative sources.
