export const meta = {
  name: 'session-mine',
  description: 'Full smine pipeline: mine transcripts into batch reports (smine-batch), then fan each batch out to the six dimension skills',
  whenToUse: 'Invoked by the /smine skill with {nightly, noBatch, skip[], batches[]} as args',
  phases: [
    { title: 'Mine', detail: 'one agent runs smine-batch (skipped with --no-batch)' },
    { title: 'Route', detail: 'per batch: six dimension agents in one parallel barrier; batches sequential (shared ledgers)' },
    { title: 'Trim', detail: 'when --max-proposals-mined is set: one agent trims overflow deterministically' },
  ],
}

// args contract (built by the fronting skill from the /smine flags):
// { nightly?: bool, noBatch?: bool, skip?: string[], batches?: string[],
//   maxMinedPerDimension?: int, maxMinedTotal?: int }
if (!args || typeof args !== 'object' || Array.isArray(args)) {
  throw new Error('args must be an object: {nightly, noBatch, skip, batches, maxMinedPerDimension, maxMinedTotal}')
}
const nightly = args.nightly === true
const noBatch = args.noBatch === true
const skip = new Set(args.skip || [])
const preResolved = Array.isArray(args.batches) ? args.batches : []
const maxMinedPerDimension = Number.isInteger(args.maxMinedPerDimension) ? args.maxMinedPerDimension : 0
const maxMinedTotal = Number.isInteger(args.maxMinedTotal) ? args.maxMinedTotal : 0
if (noBatch && preResolved.length === 0) {
  throw new Error('--no-batch requires args.batches: the fronting skill resolves the unrouted batches')
}

const DIMENSIONS = [
  'smine-memory', 'smine-skills', 'smine-workflows',
  'smine-routines', 'smine-style', 'smine-summary',
].filter(name => !skip.has(name))
if (DIMENSIONS.length === 0) throw new Error('every dimension skipped — nothing to do')

const PROPOSAL_DIMENSIONS = new Set(['smine-style', 'smine-routines', 'smine-skills', 'smine-workflows'])

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

// ---- Stage 1: Mine ----
let mined = { batchesWritten: [], sessionsMined: 0, notes: 'skipped (--no-batch)' }
if (!noBatch) {
  phase('Mine')
  const mineArgs = nightly ? '--nightly' : ''
  mined = await agent(
    `Invoke the Skill tool with skill="smine-batch" and args="${mineArgs}", then follow the loaded skill exactly. ` +
    `Work from the repo root (the directory containing sessions/). ` +
    `The skill's STOP-for-review step means: finish the batch report and the ledger append, then return` +
    (nightly ? ' after ALL pending batches are written.' : ' after one batch.') +
    ` Return every batch report path you wrote in batchesWritten (repo-relative), the mined session count, and notes.`,
    { label: 'smine-batch', phase: 'Mine', agentType: 'general-purpose', schema: MINE_SCHEMA },
  )
  if (!mined) throw new Error('smine-batch agent failed — nothing to route')
}

// ---- Stage 2: Route (sequential over batches; parallel over dimensions) ----
const batches = [...new Set([...preResolved, ...(mined.batchesWritten || [])])]
const routed = []
phase('Route')
for (const batch of batches) {
  const results = await parallel(DIMENSIONS.map(name => () =>
    agent(
      `Invoke the Skill tool with skill="${name}" and args="${batch}", then follow the loaded skill exactly, ` +
      `scoped to that single batch file. Work from the repo root (the directory containing sessions/). ` +
      (maxMinedPerDimension > 0 && PROPOSAL_DIMENSIONS.has(name)
        ? `Production cap: add at most ${maxMinedPerDimension} new proposals this run — keep the best-ranked, list every dropped candidate in notes. `
        : '') +
      `The skill's STOP-for-review step means: finish your outputs and return — the orchestrator reports to the user. ` +
      `Return the counts, every file you wrote (including the ledger), and any skips or reroutes in notes.`,
      { label: `${name} ← ${batch}`, phase: 'Route', agentType: 'general-purpose', schema: DIM_SCHEMA },
    )
  ))
  routed.push({
    batch,
    dimensions: results.filter(Boolean),
    failed: DIMENSIONS.filter((_, i) => !results[i]),
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
    `Across sessions/proposals/{routines,style,skills,workflows}.json, count proposals with status "proposed" and proposed = today. ` +
    `If the count exceeds ${maxMinedTotal}, remove the overflow: take from the kind with the most new proposals first; ` +
    `within a kind remove the last entries in JSON array order (lowest-ranked) first. ` +
    `Keep every file conformant to sessions/proposals/schema.json (jq edits). Never touch entries from earlier dates or with any other status. ` +
    `Return the pre-trim count and every removed id.`,
    { label: 'mined-cap trim', phase: 'Trim', agentType: 'general-purpose', schema: TRIM_SCHEMA },
  )
}

return {
  mined: { batchesWritten: mined.batchesWritten, sessionsMined: mined.sessionsMined, notes: mined.notes },
  skippedDimensions: [...skip],
  routed,
  trimmed,
}
