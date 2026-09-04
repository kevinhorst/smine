<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# claude-configs — Facts

**FACT-REPO-STACK-001** — Go 1.26 module `github.com/kevinhorst/claude-configs`; `make audit` (mod verify, vet, acdsl gates incl. rules validate + generate-check, tests without race) is the inner-loop gate; `make audit-full` adds race tests and the cmd/tests shell suite and is the release gate.

* Location: go.mod, Makefile
* Reach: smine

**FACT-REPO-ARCH-001** — Deployment is script-driven: `cmd/sync/sync_skills.sh` deploys skills flat to `~/.claude/skills/<leaf>`, `cmd/sync/sync_context.sh` builds per-repo context packs, `cmd/sync/sync_settings.sh` and `sync_hooks.sh` cover settings and hooks.

* Location: cmd/sync/
* Reach: smine

**FACT-REPO-ARCH-002** — The config server (`cmd/configserver`, `internal/server`) is the UI over skills, context docs, proposals, and routines; it shells out to the sync scripts rather than reimplementing them.

* Location: internal/server, internal/contextdocs/sync.go
* Reach: smine

**FACT-REPO-ARCH-003** — Sessions run in pool worktrees under `.claude/worktrees/<name>` on `claude/*` branches; commits there are invisible on the main checkout until merged or cherry-picked.

* Location: .claude/worktrees/
* Reach: smine

**FACT-REPO-ARCH-012** — `launchctl bootout` on a currently-running routine job kills the entire process tree it launched (`run.sh`, `timeout`, `caffeinate`, and the `claude -p` child), so a routine run is terminated mid-flight. `loaded` is not the same as `running`: any plist-reload path (e.g. a reload after a config change) must first check whether a run is live (darwin: `launchctl print <label>` → the `pid = ` line; windows: `taskState == Running`) and skip the bootout while a run is active.

* Location: internal/routines/launchctl_darwin.go
* Reach: smine

**FACT-REPO-ARCH-013** — Headless `claude -p` has neither the Workflow tool nor a mid-session Skill-loading tool (no frontmatter is loaded into the headless agent), so any skill hard-gated on the Workflow tool (e.g. railroad-review MODES-002) is uninvokable as a single headless dispatcher call. A routine that fans a skill out headless must decompose the fan-out itself in `run.sh` — one `claude -p "/<skill> <full brief>"` cell per lane, relying on prompt-start slash expansion, with a single-role prompt ("you ARE the lane; never attempt to fan out"). The headless agent must be given everything explicitly.

* Location: cmd/routines run.sh cell runners; skills that gate on the Workflow tool
* Reach: smine

**FACT-REPO-ARCH-014** — A tag-triggered (or re-run) GitHub Actions workflow reads its yaml from the tagged commit, never from `main`. So a fix committed to `main` does not reach a failing tag build by re-running the failed job — the tag must be moved to a commit carrying the fix (`git tag -f <tag> <fix-commit> && git push -f origin <tag>`). Applies to the smine release pipeline (`.github/workflows/*.yml`) whose build/installer/publish jobs run on tag push.

* Location: .github/workflows/
* Reach: smine

**FACT-REPO-ARCH-015** — The peek MCP tool namespace has a casing split: skills invoke tools under the capitalized `mcp__Peek_MCP__*` namespace while the synced MCP registration and allowlist use lowercase `peek-mcp` (`settings/claude_code/settings.json` allowlists both). The source of the capitalized registration is in neither the claude-configs nor the peek-mcp repo, so a tool-not-found from a peek skill is diagnosed here: verify the live `~/.claude.json` `mcpServers` key casing matches the `mcp__<name>__*` the skill invokes, not the synced lowercase fragment. There must be one canonical namespace — align the skill invocations and the live registration on a single casing so exactly one peek MCP is registered.

* Location: ~/.claude.json mcpServers; settings/claude_code/settings.json; skills invoking mcp__Peek_MCP__*
* Reach: smine, peek-mcp

**FACT-REPO-ARCH-016** — `Registry.save` writes the entire in-memory slice (`internal/repos/registry.go`), so a second running configserver holding a pre-delete slice resurrects a deleted entry via last-writer-wins. The fix is read-modify-write on save (re-read the on-disk state and merge), not IPC coordination between the two writers. Related: cache invalidation must key on a data-derived fingerprint, not a TTL — `DropBranches` leaving an entry's pre-removal fingerprint stale is what made the next load mismatch and re-scan.

* Location: internal/repos/registry.go
* Reach: smine

**FACT-REPO-ARCH-017** — A session that changes directory mid-run reports its last cwd in peek `meta.cwd`, not its home worktree, so a cwd→session map breaks when a session hops cwd into a sibling worktree — over recycled Desktop pool dirs the stale prior occupant wins the match. Attribution must prefer branch identity (parse the already-served `git_branch`) with cwd as a detached-HEAD fallback. The fix landed on the previously-ignored `git_branch` served at `internal/server/peek.go`.

* Location: internal/server/peek.go
* Reach: smine, peek-mcp

**FACT-REPO-ARCH-018** — peek v1.2.2 persists per-session telemetry to `~/.peek/state`, so telemetry survives a restart and is served even by `control-port=0` stdio instances via the persisted fallback — the earlier "telemetry exists only when controlPort>0 / in-memory only" claim is stale for v1.2.2+. Command strings require `OTEL_LOG_TOOL_DETAILS=1` to be captured.

* Location: peek-mcp ~/.peek/state
* Reach: smine, peek-mcp

**FACT-REPO-ARCH-019** — A Max 20x account admits ≈ $190 of API-equivalent `claude -p` spend per rolling 5-hour window (measured 2026-09-04: the eval run's cells summed to $190.20, then every request returned "You've hit your session limit"). A mid-run kill surfaces as an `is_error` envelope with turns > 0 — not only as an empty envelope. Any routine fanning out headless arms must keep planned spend (arms × (arm budget + judge budget) + resumes) under the runner's window budget (default $150) — `cmd/skilleval/run.sh` enforces this via `SKILLEVAL_WINDOW_BUDGET_USD`.

* Location: cmd/skilleval/run.sh; routines/skill-eval-railroad-review/run.sh
* Reach: smine
