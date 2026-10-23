# Context index cache — Concept

status: In Review

## TLDR
- **Fewer reads** — the config server reads `context/context.json` once instead of on every request.

## Goals
- A page that shows context coverage renders without re-reading `context/context.json` while the file is unchanged.
- A changed file is picked up on the next request.

## User flows
1. The user opens a skill page twice; the second render does not read the file.
2. The user regenerates the context index; the next render shows the new coverage.

## MVP
- In: the cache and its invalidation.
- Backlog: none.

## Open questions
N/A — none open
