# Railroad-review lane trace runbook

Answers, for one railroad-review round, per lane: what context it read, whether the
doctrine injection reached it, what it cost, and whether it followed its mandatory
steps. Pure disk-jq over the workflow transcripts — no peek-mcp dependency (peek is
at v1.2.1; the verbose mode is future work, spec'd in the plan).

All recipes were validated against a real run
(`…/railroad-review-change-plan-f0101e/09062f36…/subagents/workflows/wf_8040b323-e98`).
That run predates the current context system, so its numbers are historical; fresh
before/after rounds are the measurement.

## Routine runs (skill-eval-railroad-review)

The routine orchestrates the fan-out itself — headless `claude -p` has no
Workflow tool — so every lane/refuter/station is a top-level session. The
routine computes the trace mechanically: read `cells.jsonl` (roster + per-cell
cost/tokens/duration + `transcript` path per cell) and `trace.json` in the
run's `evals/railroad-review-eval/<date>/` dir. The recipes below apply to
those cell transcripts directly (they are plain session JSONLs, no
`subagents/workflows/` nesting) and remain the manual path for interactive
railroad rounds, which do run the Workflow tool.

## Locate the transcripts

Workflow transcripts of a session live at:

```
~/.claude/projects/<project-slug>/<session-id>/subagents/workflows/wf_<id>/
    journal.jsonl          # {type: started|result, key, agentId, result?}
    agent-<id>.jsonl       # full subagent transcript (assistant|user|attachment lines)
    agent-<id>.meta.json   # {agentType, worktreePath, spawnDepth}
```

Find the newest railroad workflow dir:

```bash
ls -dt ~/.claude/projects/*/*/subagents/workflows/wf_* | head -5
```

Set `WF=<that dir>` for every recipe below.

## Recipe 1 — lane roster

Join agent id to direction and lane index (a row with `?` is a non-lane agent —
grouper, station, cleanup — or a pre-8.x run using `track`):

```bash
jq -r 'select(.type=="result") | [.agentId, (.result.direction // .result.track // "?"), (.result.lane // 1), (.result.aborted // false)] | @tsv' "$WF/journal.jsonl"
```

## Recipe 2 — context reads per lane

As of railroad-review 8.4 a clean lane reads NO context-entry file directly — the
skill-invoke-gate hook denies it and the doctrine arrives injected instead (recipe 3).
This recipe therefore expects an empty result; any hit is a gate escape (the change's
language guide is the one allowed exception). Every context-directory file a lane read:

```bash
jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and .name=="Read") | .input.file_path' "$WF/agent-<id>.jsonl" | grep "/context/"
```

Expand a direction's requirement globs to files (repo root, current layout):

```bash
jq -r --arg glob "ACTION-REVIEW-QUALITY-" '.entries[] | select(.id | startswith($glob)) | .source' context/context.json | sort -u
```

## Recipe 3 — injection check

Did the doctrine reach this lane? As of 8.4 it MUST — the first two channels are
expected ≥1 (the injection block, then the spill file the read-gate forces the lane to
read). Hook output sits in `attachment.hook_success.stdout` (full text — `.content` is
capped, never use it):

```bash
jq -r 'select(.type=="attachment") | .attachment | select(.type=="hook_success") | .stdout' "$WF/agent-<id>.jsonl" | grep -c "SKILL CONTEXT /railroad-review"
```

```bash
grep -c "SKILL CONTEXT /railroad-review" "$WF/agent-<id>.jsonl"
```

```bash
jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and .name=="Read") | .input.file_path' "$WF/agent-<id>.jsonl" | grep -c "context-required/"
```

## Recipe 4 — cost per lane

Output tokens (summed), context size at end (cache read + creation + input of the
last assistant turn) and wall seconds:

```bash
jq -s '{output_tokens: ([.[] | select(.type=="assistant") | .message.usage.output_tokens // 0] | add), ctx_at_end: ([.[] | select(.type=="assistant") | .message.usage | select(.!=null)] | last | (.cache_read_input_tokens + .cache_creation_input_tokens + .input_tokens)), wall_s: ((([.[] | .timestamp | select(.!=null)] | last | sub("\\.[0-9]+Z$";"Z") | fromdate) - ([.[] | .timestamp | select(.!=null)] | first | sub("\\.[0-9]+Z$";"Z") | fromdate)))}' "$WF/agent-<id>.jsonl"
```

## Recipe 5 — step adherence

First tool call must be the premise check (`git rev-parse HEAD` + ancestor check);
the branch rename must appear; no Edit/Write inside the worktree:

```bash
jq -s '[.[] | select(.type=="assistant") | .message.content[]? | select(.type=="tool_use")] | first | [.name, (.input.command // "")] | @tsv' "$WF/agent-<id>.jsonl"
```

```bash
jq -r 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and .name=="Bash") | .input.command' "$WF/agent-<id>.jsonl" | grep -c "git branch -m claude-review/"
```

```bash
jq -r --arg wt "$(jq -r .worktreePath "$WF/agent-<id>.meta.json")" 'select(.type=="assistant") | .message.content[]? | select(.type=="tool_use" and (.name=="Edit" or .name=="Write")) | .input.file_path | select(startswith($wt))' "$WF/agent-<id>.jsonl"
```

An aborted lane (recipe 1 shows `aborted=true`) legitimately stops after the premise
check — score only recipes 3 and 4 for it.

## Run line — before/after measurement

```bash
# 1. at skill v8.3 (git checkout of the prior skill deployed via cmd/sync/sync_skills.sh):
#    run /railroad-review (defaults) on a real branch; note the wf_ dir → run recipes 1-5 per lane.
# 2. deploy v8.4 (this change), run the same review args on the same branch; repeat.
# 3. report per-lane: injection count (expect 0 → >=1), direct ctx reads (expect any → 0),
#    spill reads (expect 0 → >=1), ctx_at_end, output_tokens, wall_s.
```
