export const meta = {
  name: 'investigation',
  description: 'Fan out N independent investigations of one open question, re-verify every load-bearing claim against primary sources, then merge into one baseline with a refuted-hypotheses register',
  whenToUse: 'Invoked by the /investigation skill with the shared question, inputs, optional prior artifact, and investigator count as args',
  phases: [
    { title: 'Fan-out', detail: 'N independent investigators build ranked hypotheses from the shared inputs' },
    { title: 'Re-verify', detail: 'adversarial barrier — every load-bearing claim checked against its actual primary artifact' },
    { title: 'Merge', detail: 'single baseline, refuted-hypotheses register, discrimination plan' },
  ],
}

const MAX_INVESTIGATORS = 8

// Some harnesses deliver args as a JSON-encoded string — normalize before validating.
if (typeof args === 'string') {
  try {
    args = JSON.parse(args)
  } catch (e) {
    throw new Error('args arrived as a string that is not valid JSON: ' + e.message)
  }
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object: {question, inputs, artifactsDir, priorArtifact?, investigators?}')
}
if (typeof args.question !== 'string' || !args.question) {
  throw new Error('args.question is required — the one shared open question the investigations attack')
}
if (typeof args.inputs !== 'string' || !args.inputs) {
  throw new Error('args.inputs is required — the shared dataset / source refs every investigator reads first')
}

const n = Number.isInteger(args.investigators) && args.investigators > 0 ? args.investigators : 3
if (n > MAX_INVESTIGATORS) {
  throw new Error(`asked for ${n} investigators, cap is ${MAX_INVESTIGATORS} — shrink the fan-out`)
}
if (typeof args.artifactsDir !== 'string' || !args.artifactsDir || !args.artifactsDir.startsWith('/')) {
  throw new Error('args.artifactsDir is required — an absolute path to the session scratchpad, outside every worktree; investigator artifacts must never land in the repo')
}
const artifactsDir = args.artifactsDir
const priorArtifact = typeof args.priorArtifact === 'string' && args.priorArtifact ? args.priorArtifact : null

const INVESTIGATOR_SCHEMA = {
  type: 'object',
  required: ['run_id', 'ranked_hypotheses', 'load_bearing_claims', 'artifact_path'],
  properties: {
    run_id: { type: 'string' },
    ranked_hypotheses: {
      type: 'array',
      items: {
        type: 'object',
        required: ['rank', 'hypothesis'],
        properties: {
          rank: { type: 'integer' },
          hypothesis: { type: 'string' },
          rationale: { type: 'string' },
        },
      },
    },
    load_bearing_claims: {
      type: 'array',
      description: 'claims the ranking rests on, each with the primary source that would confirm or refute it',
      items: {
        type: 'object',
        required: ['claim', 'source_refs'],
        properties: {
          claim: { type: 'string' },
          source_refs: { type: 'array', items: { type: 'string' }, description: 'primary artifacts a verifier can fetch — files, URLs, query targets' },
        },
      },
    },
    artifact_path: { type: 'string' },
  },
}

// Each run gets a distinct id; if a prior artifact is supplied it enters as investigation #1.
const runIds = []
if (priorArtifact) runIds.push('prior#1')
for (let i = 0; i < n; i++) runIds.push(`inv#${i + (priorArtifact ? 2 : 1)}`)

phase('Fan-out')
const investigators = await parallel(runIds.map(runId => () => {
  const isPrior = runId === 'prior#1'
  const prompt = isPrior
    ? `You are investigation ${runId} in an adversarial-merge study. An existing analysis artifact enters here as ` +
      `investigation #1: ${priorArtifact}. Read it in full plus the shared inputs, then extract — do not soften — its ` +
      `ranked hypotheses and every load-bearing claim it rests on, each paired with the PRIMARY artifact that would ` +
      `confirm or refute it (a file path, a live/archived URL, a prod-query target), never a secondhand summary.\n\n` +
      `Shared question: ${args.question}\nShared inputs: ${args.inputs}\n\n` +
      `Write your extraction to ${artifactsDir}/${runId.replace('#', '-')}.md and return its path as artifact_path.`
    : `You are investigation ${runId}, one of ${n} INDEPENDENT investigators attacking one shared question. ` +
      `Hard constraints: (1) read the actual inputs / primary artifacts FIRST — do not theorize from descriptions; ` +
      `matrix variation does not protect against a shared blind spot. (2) You run concurrently with sibling ` +
      `investigators — never read their output or anchor on any prior conclusion; build your own ranking from the inputs. ` +
      `(3) Do not use plan mode / ExitPlanMode — your artifact is the deliverable.\n\n` +
      `Shared question: ${args.question}\nShared inputs: ${args.inputs}\n\n` +
      `Build ranked hypotheses. For every load-bearing claim, name the PRIMARY artifact that would confirm or refute it ` +
      `(file, live/archived URL, prod-query target) so it can be re-verified against the source, not against your prose. ` +
      `You have web access — use it for external sources. Write your analysis to ${artifactsDir}/${runId.replace('#', '-')}.md ` +
      `and return its path as artifact_path.`
  return agent(prompt, {
    label: runId,
    phase: 'Fan-out',
    agentType: 'general-purpose',
    schema: INVESTIGATOR_SCHEMA,
  })
}))

const surviving = investigators.filter(Boolean)
const dead = runIds.filter((_, i) => !investigators[i])
if (dead.length) log(`dead investigators: ${dead.join(', ')}`)
if (!surviving.length) {
  return { investigators: [], failed: dead, verification: null, baseline: null }
}

const VERIFICATION_SCHEMA = {
  type: 'object',
  required: ['claim_verdicts', 'needs_measurement_queries', 'funnel_note'],
  properties: {
    claim_verdicts: {
      type: 'array',
      items: {
        type: 'object',
        required: ['run_id', 'claim', 'verdict'],
        properties: {
          run_id: { type: 'string' },
          claim: { type: 'string' },
          verdict: { type: 'string', enum: ['holds', 'refuted', 'needs-measurement'], description: 'unreachable primary artifact stays needs-measurement, never holds' },
          evidence: { type: 'string' },
          primary_source: { type: 'string', description: 'the actual artifact fetched — never a prior run summary' },
        },
      },
    },
    needs_measurement_queries: {
      type: 'array',
      description: 'concrete queries the user can run in prod; their results return as binding facts',
      items: {
        type: 'object',
        required: ['claim', 'query'],
        properties: {
          claim: { type: 'string' },
          query: { type: 'string' },
          where_to_run: { type: 'string' },
        },
      },
    },
    funnel_note: { type: 'string', description: 'observation across the FULL funnel, not just the metric the question started with' },
  },
}

phase('Re-verify')
const allClaims = surviving.flatMap(r => (r.load_bearing_claims || []).map(c => ({ run_id: r.run_id, ...c })))
// Dedup: siblings restate the same claim — verify each unique claim once, crediting every run that raised it.
const byKey = new Map()
for (const c of allClaims) {
  const key = c.claim.toLowerCase().replace(/\s+/g, ' ').trim()
  const hit = byKey.get(key)
  if (hit) {
    hit.run_id = `${hit.run_id}+${c.run_id}`
    for (const s of c.source_refs || []) if (!hit.source_refs.includes(s)) hit.source_refs.push(s)
  } else {
    byKey.set(key, { run_id: c.run_id, claim: c.claim, source_refs: [...(c.source_refs || [])] })
  }
}
const uniqueClaims = [...byKey.values()]
log(`re-verify: ${allClaims.length} claims → ${uniqueClaims.length} unique`)

const BATCH = 8
const batches = []
for (let i = 0; i < uniqueClaims.length; i += BATCH) batches.push(uniqueClaims.slice(i, i + BATCH))

const shards = await parallel(batches.map((batch, i) => () => agent(
  `Adversarial re-verification shard ${i + 1}/${batches.length} for an investigation study. Below is your batch of ` +
  `load-bearing claims (deduped across ${surviving.length} investigations; run_id names every investigation that ` +
  `raised the claim). For EACH claim: fetch and read the ACTUAL primary artifact it names (public files, spreadsheets, ` +
  `live AND archived URLs, prod query results) — never a prior run's summary of it; the classic failure is a refuted ` +
  `hypothesis that rested on an under-sampled summary of a file curl could have fetched whole. Then rule: holds | ` +
  `refuted | needs-measurement, each with the evidence and the primary source you actually read. A claim whose primary ` +
  `artifact is unreachable stays needs-measurement — never silently holds. For needs-measurement claims, emit the ` +
  `concrete query the user can run in prod. Note anything you see across the FULL funnel, not just the metric the ` +
  `question started with. You have web access.\n\n` +
  `Investigation question: ${args.question}\nShared inputs: ${args.inputs}\n\n` +
  `Investigator artifacts:\n${JSON.stringify(surviving.map(r => ({ run_id: r.run_id, artifact_path: r.artifact_path })), null, 2)}\n\n` +
  `Load-bearing claims to verify (your batch):\n${JSON.stringify(batch, null, 2)}\n\n` +
  `Return the structured output only.`,
  { label: `re-verify#${i + 1}`, phase: 'Re-verify', agentType: 'general-purpose', effort: 'medium', schema: VERIFICATION_SCHEMA },
)))

const liveShards = shards.filter(Boolean)
if (shards.length !== liveShards.length) log(`re-verify: ${shards.length - liveShards.length} shard(s) died — their claims stay unverified`)
const verification = {
  claim_verdicts: liveShards.flatMap(s => s.claim_verdicts || []),
  needs_measurement_queries: liveShards.flatMap(s => s.needs_measurement_queries || []),
  funnel_note: liveShards.map((s, i) => `[shard ${i + 1}] ${s.funnel_note}`).join('\n'),
}

const BASELINE_SCHEMA = {
  type: 'object',
  required: ['baseline', 'refuted_register', 'net_new_findings', 'experiment_plan'],
  properties: {
    baseline: { type: 'string', description: 'one merged baseline; every statement labeled fact (verified) vs hypothesis (unverified)' },
    refuted_register: {
      type: 'array',
      description: 'R1..Rn — dead ends, each with its refuting evidence, so nobody re-investigates them',
      items: {
        type: 'object',
        required: ['id', 'hypothesis', 'refuting_evidence'],
        properties: {
          id: { type: 'string' },
          hypothesis: { type: 'string' },
          refuting_evidence: { type: 'string' },
        },
      },
    },
    net_new_findings: {
      type: 'array',
      description: 'genuinely-new findings credited to the run that produced them',
      items: {
        type: 'object',
        required: ['run_id', 'finding'],
        properties: { run_id: { type: 'string' }, finding: { type: 'string' } },
      },
    },
    experiment_plan: {
      type: 'array',
      description: 'discrimination plan — one variable per test cell, a control arm, pre-registered failure criteria',
      items: {
        type: 'object',
        required: ['variable', 'failure_criteria'],
        properties: {
          variable: { type: 'string', description: 'the single variable this cell changes' },
          control_arm: { type: 'string' },
          failure_criteria: { type: 'string', description: 'pre-registered — what result would falsify the hypothesis' },
        },
      },
    },
    open_measurements: {
      type: 'array',
      description: 'needs-measurement claims still awaiting the user\'s prod results before they become binding facts',
      items: { type: 'string' },
    },
  },
}

phase('Merge')
const baseline = await agent(
  `Merge an adversarial investigation study into one baseline. You have ${surviving.length} investigations and a ` +
  `per-claim verification register. Read every investigator artifact_path listed — actively ingest each; never assume ` +
  `one run's content reached another.\n\n` +
  `Question: ${args.question}\n\n` +
  `Investigations:\n${JSON.stringify(surviving, null, 2)}\n\n` +
  `Verification register:\n${JSON.stringify(verification, null, 2)}\n\n` +
  `Produce:\n` +
  `1. One baseline with fact-vs-hypothesis labeling on EVERY statement — verified claims are facts, the rest hypotheses.\n` +
  `2. A refuted-hypotheses register R1..Rn, each with the refuting evidence, so no one re-investigates a dead end.\n` +
  `3. Net-new findings credited per run.\n` +
  `4. A discrimination/experiment plan: one variable per test cell, a control arm, pre-registered failure criteria.\n` +
  `5. open_measurements: the needs-measurement claims still awaiting the user's prod results.\n` +
  `Return the structured output only.`,
  { label: 'merge', phase: 'Merge', agentType: 'general-purpose', effort: 'high', schema: BASELINE_SCHEMA },
)

return { investigators: surviving, failed: dead, verification, baseline }
