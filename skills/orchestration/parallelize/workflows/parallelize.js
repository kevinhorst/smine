export const meta = {
  name: 'parallelize',
  description: 'Fan one skill invocation across a model/effort/arg-variant/replica matrix in isolated worktrees, then consolidate',
  whenToUse: 'Invoked by the /parallelize skill with the invocation and matrix spec as args',
  phases: [
    { title: 'Fan-out', detail: 'one agent per matrix cell, isolated worktree each' },
    { title: 'Consolidate', detail: 'single synthesizer over surviving artifacts' },
  ],
}

const MAX_CELLS = 16
const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch { throw new Error('args arrived as a string and is not valid JSON') }
}
if (!args || typeof args !== 'object' || typeof args.skill !== 'string' || !args.skill) {
  throw new Error('args must be an object: {skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit}')
}
if (typeof args.baseCommit !== 'string' || !args.baseCommit) {
  throw new Error('args.baseCommit is required — every cell verifies its worktree HEAD against it')
}

const models = Array.isArray(args.models) && args.models.length ? args.models : [null]
const efforts = Array.isArray(args.efforts) && args.efforts.length ? args.efforts : [null]
const variants = Array.isArray(args.argVariants) && args.argVariants.length ? args.argVariants : [args.skillArgs ?? '']
const replicas = Number.isInteger(args.replicas) && args.replicas > 0 ? args.replicas : 1

for (const e of efforts) {
  if (e !== null && !EFFORTS.includes(e)) throw new Error(`unknown effort "${e}" — use one of ${EFFORTS.join('/')}`)
}

const cells = []
for (const model of models)
  for (const effort of efforts)
    for (const variant of variants)
      for (let replica = 0; replica < replicas; replica++)
        cells.push({ model, effort, variant, replica })
if (cells.length > MAX_CELLS) {
  throw new Error(`matrix has ${cells.length} cells, cap is ${MAX_CELLS} — shrink a dimension`)
}

const CELL_SCHEMA = {
  type: 'object',
  required: ['run_id', 'model', 'effort', 'arg_variant', 'replica_index', 'worktree_ref', 'artifact_path', 'baseline_mismatch_flag', 'contaminated'],
  properties: {
    run_id: { type: 'string', description: 'the cell label' },
    model: { type: 'string' },
    effort: { type: 'string' },
    arg_variant: { type: 'string' },
    replica_index: { type: 'integer' },
    worktree_ref: { type: 'string', description: 'worktree path and branch/commit the run left behind' },
    artifact_path: { type: 'string', description: 'path to the deliverable (plan, findings doc, diff) inside the worktree' },
    verdict: { type: 'string', description: 'for judgment/viability questions: the answer; empty otherwise' },
    deferred_decisions: { type: 'array', items: { type: 'string' } },
    explicit_deviations: { type: 'array', items: { type: 'string' }, description: 'places the run knowingly deviated from the given context/spec' },
    self_reported_gaps: { type: 'array', items: { type: 'string' } },
    baseline_mismatch_flag: { type: 'boolean', description: 'true if worktree HEAD did not match the base commit' },
    contaminated: { type: 'boolean', description: 'true if the run read a sibling worktree or another branch of this work' },
  },
}

const cellLabel = c =>
  `${c.model ?? 'session'}/${c.effort ?? 'inherit'}/${typeof c.variant === 'string' ? c.variant.slice(0, 24) || 'default' : 'default'}#${c.replica}`

const cellBranch = c =>
  `claude/parallelize-${cellLabel(c).replace(/[^a-zA-Z0-9._-]+/g, '-')}-${args.baseCommit.slice(0, 6)}`

phase('Fan-out')
const results = await parallel(cells.map(c => () => {
  const opts = {
    label: cellLabel(c),
    phase: 'Fan-out',
    agentType: 'general-purpose',
    isolation: 'worktree',
    schema: CELL_SCHEMA,
  }
  if (c.model) opts.model = c.model
  if (c.effort) opts.effort = c.effort
  return agent(
    `You are one cell of a matrix bake-off: run_id "${cellLabel(c)}", replica index ${c.replica}. ` +
    `You run in your own isolated git worktree.\n\n` +
    `FIRST ACTION: run \`git rev-parse HEAD\` in your worktree. If it does not equal ${args.baseCommit}, ` +
    `set baseline_mismatch_flag=true in your output and say so — do not silently continue from a different premise.\n\n` +
    `SECOND ACTION: rename your worktree branch so any leftover lands in the claude/* namespace: ` +
    `\`git branch -m ${cellBranch(c)}\` — if the name is taken, append "-2". ` +
    `Rename only the branch; never move the worktree directory, it is harness-managed.\n\n` +
    `Hard constraints, violation disqualifies the cell:\n` +
    `1. Never read outside your own worktree — no sibling worktrees, no other branches of this task, no other cells' output. ` +
    `If you did anyway, report contaminated=true honestly.\n` +
    `2. Inputs-first: read the actual input files/payloads referenced by the task before theorizing. ` +
    `Matrix variation does not protect against a shared blind spot.\n` +
    `3. Do not use plan mode or ExitPlanMode. Your artifact/findings ARE the deliverable — write them to a file in your worktree ` +
    `and return its path as artifact_path.\n\n` +
    `TASK: invoke the Skill tool with skill="${args.skill}" and args="${typeof c.variant === 'string' ? c.variant : (args.skillArgs ?? '')}", ` +
    `then follow the loaded skill exactly, unattended — never wait for user approval.\n` +
    (args.contextDoc ? `Shared context for the task: ${args.contextDoc}\n` : '') +
    `\nReturn the structured output. For judgment/viability questions put the answer in verdict; ` +
    `list every decision you deferred, every knowing deviation from the given context, and every gap you know you left.`,
    opts,
  )
}))

const dead = cells.filter((_, i) => !results[i]).map(cellLabel)
const finished = results.filter(Boolean)
const disqualified = finished.filter(r => r.contaminated || r.baseline_mismatch_flag)
const surviving = finished.filter(r => !r.contaminated && !r.baseline_mismatch_flag)
if (dead.length) log(`dead cells (skipped or errored): ${dead.join(', ')}`)
if (disqualified.length) log(`disqualified: ${disqualified.map(r => `${r.run_id} (${r.contaminated ? 'contaminated' : 'baseline mismatch'})`).join(', ')}`)
if (!surviving.length) {
  return { cells: finished, disqualified, failed: dead, consolidation: null }
}

const CONSOLIDATION_SCHEMA = {
  type: 'object',
  required: ['decision_matrix', 'divergences', 'shared_blind_spots', 'grounded_base', 'why', 'cross_graft_plan'],
  properties: {
    decision_matrix: {
      type: 'array',
      description: 'rows = decision points, columns = per-run choices',
      items: {
        type: 'object',
        required: ['decision_point', 'choices'],
        properties: {
          decision_point: { type: 'string' },
          choices: { type: 'array', items: { type: 'object', required: ['run_id', 'choice'], properties: { run_id: { type: 'string' }, choice: { type: 'string' } } } },
        },
      },
    },
    divergences: { type: 'array', items: { type: 'string' }, description: 'material disagreements and correctness gaps' },
    shared_blind_spots: { type: 'array', items: { type: 'string' }, description: 'claims all runs make that none verified' },
    grounded_base: { type: 'string', description: 'run_id of the winning cell' },
    why: { type: 'string', description: 'why it won — grounding in actual code/inputs, not fanciness' },
    cross_graft_plan: { type: 'string', description: 'base + the losers’ genuinely-good deltas as a small patch plan' },
  },
}

phase('Consolidate')
const consolidation = await agent(
  `Consolidate a matrix bake-off of the skill "${args.skill}". ${surviving.length} surviving runs; ` +
  `structured outputs follow. Read every artifact_path listed — actively ingest each one; never assume ` +
  `one run's content reached another.\n\n${JSON.stringify(surviving, null, 2)}\n\n` +
  `Produce:\n` +
  `1. A decision-point comparison matrix: rows = decision points, columns = each run's choice.\n` +
  `2. Divergences and correctness gaps; shared blind spots — claims all runs make that none verified against the actual inputs.\n` +
  `3. The grounded base: the run that read the actual code/inputs and falsified premises wins, not the fanciest. ` +
  `Do not debate architectures.\n` +
  `4. A cross-graft plan: the base plus the losers' genuinely-good deltas as a small patch.\n` +
  `Return the structured output only.`,
  { label: 'consolidate', phase: 'Consolidate', agentType: 'general-purpose', effort: 'high', schema: CONSOLIDATION_SCHEMA },
)

return { cells: finished, disqualified, failed: dead, consolidation }
