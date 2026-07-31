# smine — Facts

**FACT-STACK-001** — Go 1.26 module `github.com/kevinhorst/smine`; `make audit` (mod verify, vet, rules validate + generate-check, race tests, cmd/tests shell suite) is the only quality gate.

* Location: go.mod, Makefile

**FACT-ARCH-001** — Deployment is script-driven: `cmd/sync/sync_skills.sh` deploys skills flat to `~/.claude/skills/<leaf>`, `cmd/sync/sync_context.sh` builds per-repo context packs, `cmd/sync/sync_settings.sh` and `sync_hooks.sh` cover settings and hooks.

* Location: cmd/sync/

**FACT-ARCH-002** — The config server (`cmd/configserver`, `internal/server`) is the UI over skills, context docs, proposals, and routines; it shells out to the sync scripts rather than reimplementing them.

* Location: internal/server, internal/contextdocs/sync.go

**FACT-ARCH-003** — Sessions run in pool worktrees under `.claude/worktrees/<name>` on `claude/*` branches; commits there are invisible on the main checkout until merged or cherry-picked.

* Location: .claude/worktrees/
