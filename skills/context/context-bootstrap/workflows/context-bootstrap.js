export const meta = {
  name: 'context-bootstrap',
  description: 'Mine a repo dimension-by-dimension over chunks into verified candidate context entries',
  phases: [
    { title: 'Mine', detail: 'one miner per dimension x chunk x lane' },
    { title: 'Merge', detail: 'dedup per dimension, then across dimensions' },
    { title: 'Verify', detail: 'batch-verify candidates against the repo' },
  ],
}

const VERIFY_BATCH = 4

const CANDIDATES = {
  type: 'object',
  properties: {
    candidates: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          statement: { type: 'string' },
          kind: { type: 'string' },
          band: { type: 'string' },
          applies: { type: 'string' },
          why: { type: 'string' },
          evidence: { type: 'array', items: { type: 'string' } },
          anchor: { type: 'string' },
          verifierSketch: { type: 'string' },
        },
        required: ['statement', 'kind', 'band', 'evidence'],
      },
    },
  },
  required: ['candidates'],
}

const VERDICTS = {
  type: 'object',
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          index: { type: 'number' },
          holds: { type: 'boolean' },
          note: { type: 'string' },
        },
        required: ['index', 'holds', 'note'],
      },
    },
  },
  required: ['verdicts'],
}

// Each dimension carries its own chunk list: code dimensions share the file
// chunks, the arg-activated history dimension gets commit-range chunks. A
// chunk's `spec` is the ready-made mining instruction (file list or git log
// commands) built by the dispatcher.
const dims = args.dimensions
const lanes = args.lanes > 0 ? args.lanes : 1
// GEN:models
// Subagent model vocabulary — generated from availableModels and
// forbidden_subagent_models by the workflow generator (internal/models).
const SUBAGENT_MODELS = ["opus", "sonnet"]
const DEFAULT_SUBAGENT_MODEL = "opus"
const assertModel = (name, value) => {
  if (!SUBAGENT_MODELS.includes(value)) throw new Error(`${name} "${value}" is not a subagent model (${SUBAGENT_MODELS.join(' | ')}) — unresolved tokens silently run the session model, forbidden ones are stopped by heartbeat`)
  return value
}
// GEN:end

const laneModel = assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL)
const opts = (label, phase, schema) => ({
  label, phase, schema,
  model: laneModel,
  ...(args.laneEffort ? { effort: args.laneEffort } : {}),
})

const repoNote = args.repo
  ? `The repo under review is a read-only checkout at ${args.repo}; resolve every evidence path against it, never against the working directory. `
  : ''

const minePrompt = (d, c) => `${d.prompt}\n\nRead-only, never edit.\n${c.spec}`

const mergePrompt = (d, cands) =>
  `Deduplicate and consolidate these candidate context entries from the "${d.key}" lens. ` +
  `Merge duplicates keeping the union of their evidence; drop candidates with fewer than 2 evidence points; ` +
  `keep statements in entry grammar. Candidates:\n${JSON.stringify(cands)}`

// Pipeline: each dimension mines and merges independently; only the
// cross-dimension dedup and verification need the full set.
const merged = await pipeline(
  dims,
  (d) =>
    parallel(
      d.chunks.flatMap((c) =>
        Array.from({ length: lanes }, (_, i) => () =>
          agent(minePrompt(d, c), opts(`mine:${d.key}:${c.id}:${i}`, 'Mine', CANDIDATES))
        )
      )
    ).then((rs) => rs.filter(Boolean).flatMap((r) => r.candidates)),
  (cands, d) =>
    cands.length === 0
      ? { candidates: [] }
      : agent(mergePrompt(d, cands), opts(`merge:${d.key}`, 'Merge', CANDIDATES))
)

// Barrier justified: cross-dimension dedup needs every dimension's merged set.
const all = merged.filter(Boolean).flatMap((m) => m.candidates)
log(`${all.length} candidates after per-dimension merge`)
const crossed =
  all.length === 0
    ? { candidates: [] }
    : await agent(
        `Deduplicate across lenses — one candidate per underlying rule/fact, strongest phrasing wins, ` +
          `union of evidence. Candidates:\n${JSON.stringify(all)}`,
        opts('merge:cross', 'Merge', CANDIDATES)
      )

const cands = (crossed?.candidates ?? []).map((c, i) => ({ ...c, index: i }))
const batches = []
for (let i = 0; i < cands.length; i += VERIFY_BATCH) batches.push(cands.slice(i, i + VERIFY_BATCH))
const verdictBatches = await parallel(
  batches.map((b, i) => () =>
    agent(
      `${repoNote}Verify each candidate against the actual repo (read-only): does the stated pattern/fact hold ` +
        `repo-wide, and is each evidence point real? Refute over confirm when uncertain. ` +
        `Answer per candidate by its "index". Candidates:\n${JSON.stringify(b)}`,
      opts(`verify:${i}`, 'Verify', VERDICTS)
    )
  )
)
const verdictByIndex = new Map(
  verdictBatches.filter(Boolean).flatMap((v) => v.verdicts).map((v) => [v.index, v])
)
return {
  confirmed: cands.filter((c) => verdictByIndex.get(c.index)?.holds),
  refuted: cands
    .filter((c) => verdictByIndex.get(c.index) && !verdictByIndex.get(c.index).holds)
    .map((c) => ({ ...c, refutation: verdictByIndex.get(c.index).note })),
  unverified: cands.filter((c) => !verdictByIndex.has(c.index)),
}
