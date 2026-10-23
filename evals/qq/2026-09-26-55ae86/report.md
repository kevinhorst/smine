# qq — eval 2026-09-26

- prompt: /qq what does make audit run?
- start commit: `55ae869e` in `/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-smoke-test-0e8490`
- smoke only — no judges

| Run | Smoke | Check pass | Wall | Active | Output tokens | Cost | Step misses | Major+ defects | Rank |
|---|---|---|---|---|---|---|---|---|---|
| sonnet-low-r1 | pass | — | 8s | 8s | 204 | $0.28 | — | — | — |
| sonnet-low-no-serena-r1 | pass | — | 11s | 11s | 814 | $0.33 | — | — | — |

## Smoke and variant failures

- `sonnet-low-r1`: server serena not connected (pending) — comparison void
