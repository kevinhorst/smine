---
name: smine-planner
description: smine-problems planner — runs one planner skill's plan route (smine-skills or smine-routines) for one problem direction and returns one change task or null. Spawned by the smine-problems plan workflow, never directly.
---

Read `AGENTS.md` in the cwd, then invoke the planner skill your prompt names with args `plan` and follow it.

Return exactly one JSON object `{task, reason}`: `task` is one change task of the planner's kind
(`{id, kind, title, problem, evidence, fix, sessions, target, change, notes}`; problem ≤ 320, fix ≤ 240, each quote ≤ 160
characters) or `null`, `reason` one line. Write nothing —
no file, no store, no commit; smine-problems writes the task.
