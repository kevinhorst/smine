export const meta = {
  name: 'session-mine',
  description: 'Full smine pipeline: mine transcripts into batch reports plus their JSON (smine-batch), then fan each batch out to the five dimension skills',
  whenToUse: 'Invoked by the /smine skill with {nightly, noBatch, skip[], batches[], subagents?, model?, effort?} as args',
  phases: [
    { title: 'Drift', detail: 'pre-run diff of the per-dimension ledgers per scope; reports diverging session ids (informational, non-gating)' },
    { title: 'Mine', detail: 'one agent runs smine-batch (skipped with --no-batch)' },
    { title: 'Route', detail: 'per batch: dimension agents in parallel, smine-memory after smine-context (shared context.json); batches sequential (shared ledgers)' },
    { title: 'Trim', detail: 'when --max-proposals-mined is set: one agent trims overflow deterministically' },
  ],
}

// args contract (built by the fronting skill from the /smine flags):
// { nightly?: bool, noBatch?: bool, skip?: string[], batches?: string[],
//   maxMinedPerDimension?: int, maxMinedTotal?: int, since?: string,
//   last?: int, dev?: bool, subagents?: bool, agents?: string, model?: string, effort?: string }
if (!args || typeof args !== 'object' || Array.isArray(args)) {
  throw new Error('args must be an object: {nightly, noBatch, skip, batches, maxMinedPerDimension, maxMinedTotal, since, last, dev, subagents, agents, model, effort}')
}
const nightly = args.nightly === true
const noBatch = args.noBatch === true
const skip = new Set(args.skip || [])
const preResolved = Array.isArray(args.batches) ? args.batches : []
const maxMinedPerDimension = Number.isInteger(args.maxMinedPerDimension) ? args.maxMinedPerDimension : 0
const maxMinedTotal = Number.isInteger(args.maxMinedTotal) ? args.maxMinedTotal : 0
const since = typeof args.since === 'string' && args.since ? args.since : ''
const last = Number.isInteger(args.last) ? args.last : 0
const dev = args.dev === true
const subagents = args.subagents === true
const agents = typeof args.agents === 'string' && args.agents ? args.agents : ''
const model = typeof args.model === 'string' && args.model ? args.model : ''
const effort = typeof args.effort === 'string' && args.effort ? args.effort : ''
// tier override spread into every agent() opts; empty when omitted, so the
// subagents inherit the session model/effort — the tier is never baked in by filename
const tier = { ...(model && { model }), ...(effort && { effort }) }
if (noBatch && preResolved.length === 0) {
  throw new Error('--no-batch requires args.batches: the fronting skill resolves the unrouted batches')
}

const DIMENSIONS = [
  'smine-memory', 'smine-skills', 'smine-routines', 'smine-context', 'smine-permissions',
].filter(name => !skip.has(name))
if (DIMENSIONS.length === 0) throw new Error('every dimension skipped — nothing to do')

const PROPOSAL_DIMENSIONS = new Set(['smine-context', 'smine-memory', 'smine-permissions', 'smine-routines', 'smine-skills'])

const DRIFT_SCHEMA = {
  type: 'object',
  required: ['diverging_ids'],
  properties: {
    scopes: { type: 'array', items: { type: 'string' }, description: 'folders checked (every dir under sessions/ except archived/)' },
    ledgers: { type: 'array', items: { type: 'string' }, description: 'per-dimension ledger filenames discovered (analyzed-*.txt)' },
    diverging_ids: {
      type: 'array',
      description: 'one entry per session id present in some but not all of a scope\'s ledgers',
      items: {
        type: 'object',
        required: ['scope', 'id', 'held_by', 'missing_from'],
        properties: {
          scope: { type: 'string' },
          id: { type: 'string' },
          held_by: { type: 'array', items: { type: 'string' }, description: 'ledgers that hold the id' },
          missing_from: { type: 'array', items: { type: 'string' }, description: 'ledgers that are missing the id' },
          suspected_cause: { type: 'string', description: 'ghost-skip convention vs partial-failure drift' },
        },
      },
    },
    notes: { type: 'string' },
  },
}

const MINE_SCHEMA = {
  type: 'object',
  required: ['batchesWritten'],
  properties: {
    batchesWritten: { type: 'array', items: { type: 'string' },
      description: 'repo-relative paths of every batch report written this run, e.g. sessions/work/sessions-batch-20.md' },
    sessionsMined: { type: 'integer' },
    notes: { type: 'string', description: 'skipped sessions, data gaps, anomalies' },
  },
}

const DIM_SCHEMA = {
  type: 'object',
  required: ['dimension', 'extracted', 'appliedOrProposed', 'outputPaths'],
  properties: {
    dimension: { type: 'string', description: 'the skill that ran' },
    extracted: { type: 'integer', description: 'candidates/items found in the batch' },
    appliedOrProposed: { type: 'integer', description: 'entries that survived evaluation and were written' },
    outputPaths: { type: 'array', items: { type: 'string' }, description: 'every file written, including the ledger' },
    notes: { type: 'string', description: 'skips, reroutes, anomalies' },
  },
}

// ---- Stage 0: Drift (pre-run ledger diff, informational; never gates) ----
phase('Drift')
const drift = await agent(
  `Pre-run ledger drift check for the smine pipeline. Work from the repo root (the directory containing sessions/). ` +
  `Enumerate the folders: every directory directly under sessions/ EXCEPT archived/. For each folder, find its per-dimension ` +
  `ledgers — the sessions/<scope>/analyzed-*.txt files (each holds one session id per line). ` +
  `Within a scope, compute the symmetric difference of the id sets across those ledgers: report every id present in some ledgers but ` +
  `not all, with held_by[] (ledgers holding it), missing_from[] (ledgers missing it), and a suspected_cause classifying it as ` +
  `"ghost-skip convention" (a dimension deliberately skips ghost/no-op sessions) vs "partial-failure drift" (a dimension fell behind ` +
  `because a run failed or a batch was passed by explicit arg, bypassing the missing-from-a-ledger auto-resolve). ` +
  `This is a diagnostic only — never mine, append, or modify any file; return the findings so the orchestrator can surface silent coverage drift.`,
  { label: 'ledger drift', phase: 'Drift', agentType: 'general-purpose', schema: DRIFT_SCHEMA, ...tier },
) || { scopes: [], ledgers: [], diverging_ids: [], notes: 'drift agent returned nothing' }
if (drift.diverging_ids && drift.diverging_ids.length) {
  log(`ledger drift: ${drift.diverging_ids.length} diverging id(s) across ${(drift.scopes || []).length} scope(s)`)
}

// ---- Stage 1: Mine ----
let mined = { batchesWritten: [], sessionsMined: 0, notes: 'skipped (--no-batch)' }
if (!noBatch) {
  phase('Mine')
  const mineArgs = [nightly ? '--nightly' : '', since ? `--since ${since}` : '', last ? `--last ${last}` : '', dev ? '--dev' : '', subagents ? '--subagents' : '', agents ? `--agents ${agents}` : ''].filter(Boolean).join(' ')
  mined = await agent(
    `Invoke the Skill tool with skill="smine-batch" and args="${mineArgs}", then follow the loaded skill exactly. ` +
    `Work from the repo root (the directory containing sessions/). ` +
    `The skill's STOP-for-review step means: finish the batch report and the ledger append, then return` +
    (nightly ? ' after ALL pending batches are written.' : ' after one batch.') +
    ` Return every batch report path you wrote in batchesWritten (repo-relative), the mined session count, and notes.`,
    { label: 'smine-batch', phase: 'Mine', agentType: 'general-purpose', schema: MINE_SCHEMA, ...tier },
  )
  if (!mined) throw new Error('smine-batch agent failed — nothing to route')
}

// ---- Stage 2: Route (sequential over batches; parallel over dimensions,
// except smine-memory runs AFTER smine-context — both write proposals/context.json) ----
const batches = [...new Set([...preResolved, ...(mined.batchesWritten || [])])]
const routed = []
phase('Route')
const dimAgent = (name, batch) =>
  agent(
    `Invoke the Skill tool with skill="${name}" and args="${batch}", then follow the loaded skill exactly, ` +
    `scoped to that single batch file. Work from the repo root (the directory containing sessions/). ` +
    (maxMinedPerDimension > 0 && PROPOSAL_DIMENSIONS.has(name)
      ? `Production cap: add at most ${maxMinedPerDimension} new proposals this run — keep the best-ranked, list every dropped candidate in notes. `
      : '') +
    `The skill's STOP-for-review step means: finish your outputs and return — the orchestrator reports to the user. ` +
    `Return the counts, every file you wrote (including the ledger), and any skips or reroutes in notes.`,
    { label: `${name} ← ${batch}`, phase: 'Route', agentType: 'general-purpose', schema: DIM_SCHEMA, ...tier },
  )
const PARALLEL_DIMS = DIMENSIONS.filter(name => name !== 'smine-memory')
const runMemory = DIMENSIONS.includes('smine-memory')
for (const batch of batches) {
  const results = await parallel(PARALLEL_DIMS.map(name => () => dimAgent(name, batch)))
  const ordered = [...PARALLEL_DIMS]
  if (runMemory) {
    results.push(await dimAgent('smine-memory', batch))
    ordered.push('smine-memory')
  }
  routed.push({
    batch,
    dimensions: results.filter(Boolean),
    failed: ordered.filter((_, i) => !results[i]),
  })
}

// ---- Stage 3: Trim (only with a global mined cap) ----
const TRIM_SCHEMA = {
  type: 'object',
  required: ['newProposals', 'removedIds'],
  properties: {
    newProposals: { type: 'integer', description: 'proposals with proposed = run date found before trimming' },
    removedIds: { type: 'array', items: { type: 'string' }, description: 'ids removed by the trim, empty when under the cap' },
    notes: { type: 'string' },
  },
}

let trimmed = null
if (maxMinedTotal > 0 && batches.length > 0) {
  phase('Trim')
  trimmed = await agent(
    `Enforce the nightly mined-proposals cap of ${maxMinedTotal}. Determine today's date from the system clock. ` +
    `Across proposals/{routines,context,skills}.json, count proposals with status "proposed" and proposed = today. ` +
    `If the count exceeds ${maxMinedTotal}, remove the overflow: take from the kind with the most new proposals first; ` +
    `within a kind remove the last entries in JSON array order (lowest-ranked) first. ` +
    `Keep every file conformant to proposals/schema.json (jq edits). Never touch entries from earlier dates or with any other status. ` +
    `Return the pre-trim count and every removed id.`,
    { label: 'mined-cap trim', phase: 'Trim', agentType: 'general-purpose', schema: TRIM_SCHEMA, ...tier },
  )
}

return {
  drift,
  mined: { batchesWritten: mined.batchesWritten, sessionsMined: mined.sessionsMined, notes: mined.notes },
  skippedDimensions: [...skip],
  routed,
  trimmed,
}
