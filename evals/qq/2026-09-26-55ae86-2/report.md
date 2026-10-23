# qq — eval 2026-09-26

- prompt: /qq what does make audit run?
- start commit: `55ae869e` in `/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-smoke-test-0e8490`
- smoke only — no judges

| Run | Smoke | Check pass | Wall | Active | Output tokens | Cost | Step misses | Major+ defects | Rank |
|---|---|---|---|---|---|---|---|---|---|
| nosuchmodel-low-r1 | pass | — | 10s | 10s | 513 | $0.28 | — | — | — |

## Smoke and variant failures

- `nosuchmodel-low-r1`: requested model nosuchmodel but the cell ran claude-opus-5-5[1m]
