Feature name: Routine runs panel in the config server.

Problem: the nightly routines in this repo (routines/skill-eval, routines/smine-nightly, routines/coverage-increaser) each append one JSON line per stage to routines/<name>/results.jsonl, but the only way to see whether last night's runs succeeded is to open those files by hand. We want a read-only panel in the config server (cmd/configserver, internal/server) that shows the latest state of every routine at a glance.

Actors: the repo owner checking the morning after, the config server (renders the panel), the routines (producers of results.jsonl — out of scope to change).

Flows to cover:
1. Overview table: one row per directory under routines/ that contains a results.jsonl — routine name, timestamp of the last line, its stage, exit_status, and total_cost_usd summed over the lines of the most recent calendar day. Failed runs (exit_status != 0) are visually distinct.
2. Drill-down: clicking a routine shows its most recent 50 lines, newest first, with stage, exit status, session id, and the result excerpt; malformed lines are skipped and counted, never fail the page.
3. Missing or empty results.jsonl: the routine still appears in the overview with an explicit "no runs recorded" state.

Detailed design block wanted for flow 1: the line shape as written by the routines' append_result helper (timestamp, stage, exit_status, session_id, num_turns, total_cost_usd, subtype, is_error, result), the Go loading model, and how the per-day cost sum treats lines with a missing total_cost_usd.

Constraints: read-only over existing files — no new store, no schema change to results.jsonl; follow the existing config-server patterns (Go stdlib mux, html/template, one handler file per concern); the panel must render even while a routine is mid-run and appending.

Out of scope: triggering or scheduling routines from the UI, editing launchd plists, log tailing, Windows.
