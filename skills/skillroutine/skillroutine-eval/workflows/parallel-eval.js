export const meta = {
  name: 'skillroutine-parallel-eval',
  description: 'Pipe: nest the parallelize workflow over a fresh matrix, then score every surviving run with skillroutine-eval',
  whenToUse: 'Invoked by skillroutine-eval matrix mode with the matrix spec, eval context, and resolved script/skillMd paths as args',
  phases: [
    { title: 'Fan-out', detail: 'nested parallelize workflow — one cell per matrix entry, isolated worktrees' },
    { title: 'Manifest', detail: 'copy artifacts out of cell worktrees, write the eval manifest' },
    { title: 'Eval', detail: 'one agent runs skillroutine-eval on the manifest, unattended' },
  ],
}

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch { throw new Error('args arrived as a string and is not valid JSON') }
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object: {parallelizeScript, skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit, skillMd, contextFiles, qualityContext, resultsDir}')
}
for (const key of ['parallelizeScript', 'skill', 'baseCommit', 'skillMd']) {
  if (typeof args[key] !== 'string' || !args[key]) throw new Error(`args.${key} is required (string)`)
}

const resultsDir = typeof args.resultsDir === 'string' && args.resultsDir
  ? args.resultsDir.replace(/\/+$/, '')
  : `sessions/skillroutine-eval/${args.skill.replace(/[^a-zA-Z0-9._-]+/g, '-')}-${args.baseCommit.slice(0, 6)}`
const contextFiles = Array.isArray(args.contextFiles) ? args.contextFiles : []
const qualityContext = Array.isArray(args.qualityContext) ? args.qualityContext : []
const variants = Array.isArray(args.argVariants) && args.argVariants.length ? args.argVariants : [args.skillArgs ?? '']
const inputs = [...new Set([...variants.filter(v => typeof v === 'string' && v), ...(args.contextDoc ? [args.contextDoc] : [])])]
if (!inputs.length) inputs.push('(no skill args — bare invocation)')

phase('Fan-out')
const par = await workflow({ scriptPath: args.parallelizeScript }, {
  skill: args.skill,
  skillArgs: args.skillArgs,
  contextDoc: args.contextDoc,
  models: args.models,
  efforts: args.efforts,
  argVariants: args.argVariants,
  replicas: args.replicas,
  baseCommit: args.baseCommit,
})

const disqualifiedIds = new Set((par.disqualified ?? []).map(r => r.run_id))
const surviving = (par.cells ?? []).filter(r => !disqualifiedIds.has(r.run_id))
if (!surviving.length) {
  log('no surviving cells — nothing to evaluate')
  return { parallelize: par, manifest: null, eval: null }
}
log(`${surviving.length} surviving run(s), ${disqualifiedIds.size} disqualified, ${(par.failed ?? []).length} dead`)

const MANIFEST_SCHEMA = {
  type: 'object',
  required: ['manifest_path', 'missing'],
  properties: {
    manifest_path: { type: 'string', description: 'path to the written manifest.json' },
    missing: { type: 'array', items: { type: 'string' }, description: 'run_ids whose artifact could not be recovered' },
  },
}

phase('Manifest')
const manifest = await agent(
  `Mechanical task, no judgment. Build a skillroutine-eval manifest for a finished matrix bake-off of the skill "${args.skill}".\n\n` +
  `Surviving cells (structured output of the parallelize workflow):\n${JSON.stringify(surviving, null, 2)}\n\n` +
  `1. Create the directory ${resultsDir}/runs/.\n` +
  `2. For each cell, copy the file at artifact_path (inside the cell's worktree — see worktree_ref) to ` +
  `${resultsDir}/runs/<run_id sanitized to [a-zA-Z0-9._-]>.<original extension>. Copy FIRST, before anything else — ` +
  `cell worktrees are ephemeral. If a worktree is already gone, try \`git show <cell branch>:<artifact path relative to repo root>\` ` +
  `(branches are named claude/parallelize-*); if that also fails, the run goes in missing[] and is left out of the manifest.\n` +
  `3. Write ${resultsDir}/manifest.json with exactly this shape (the skillroutine-eval manifest contract):\n` +
  `   {\n` +
  `     "skill": ${JSON.stringify(args.skill)},\n` +
  `     "skillMd": ${JSON.stringify(args.skillMd)},\n` +
  `     "contextFiles": ${JSON.stringify(contextFiles)},\n` +
  `     "qualityContext": ${JSON.stringify(qualityContext)},\n` +
  `     "inputs": ${JSON.stringify(inputs)},\n` +
  `     "runs": [ per surviving cell: { "id": <run_id>, "output": <copied path>, ` +
  `"model": { "id": <cell model, or "session" if null/absent>, "effort": <cell effort, omit the key if null/absent> } } ],\n` +
  `     "output": "${resultsDir}/eval.json",\n` +
  `     "md": true\n` +
  `   }\n` +
  `   Omit empty optional arrays is fine; required keys are skill, skillMd, inputs, runs.\n` +
  `4. Validate with jq: manifest parses, every runs[].output file exists, required fields present.\n\n` +
  `Return the structured output: manifest_path and missing[].`,
  { label: 'manifest', phase: 'Manifest', agentType: 'general-purpose', effort: 'low', schema: MANIFEST_SCHEMA },
)
if (!manifest) {
  return { parallelize: par, manifest: null, eval: null, error: 'manifest agent died — artifacts may still be in cell worktrees' }
}
if (manifest.missing.length) log(`artifacts unrecoverable for: ${manifest.missing.join(', ')}`)

const EVAL_SCHEMA = {
  type: 'object',
  required: ['eval_json', 'totals', 'ranking'],
  properties: {
    eval_json: { type: 'string', description: 'path to the validated eval JSON output' },
    eval_md: { type: 'string', description: 'path to the rendered markdown, if md was set' },
    totals: {
      type: 'array',
      description: 'per run: per-axis totals as reported by skillroutine-eval',
      items: {
        type: 'object',
        required: ['run_id', 'compliance_pct'],
        properties: {
          run_id: { type: 'string' },
          compliance_pct: { type: 'number' },
          quality_pct: { type: 'number', description: 'omit if the quality axis was N/A' },
        },
      },
    },
    ranking: { type: 'array', items: { type: 'string' }, description: 'run_ids best-first' },
  },
}

phase('Eval')
const evaluation = await agent(
  `Invoke the Skill tool with skill="skillroutine-eval" and args="${manifest.manifest_path}", then follow the loaded skill ` +
  `exactly and unattended — never wait for user approval; where the skill says to stop for user review, instead finish ` +
  `and return. The manifest is already written and validated; do not rebuild it.\n\n` +
  `Return the structured output: the eval JSON path (and md path if rendered), per-run per-axis totals, and the ranking best-first.`,
  { label: 'skillroutine-eval', phase: 'Eval', agentType: 'general-purpose', schema: EVAL_SCHEMA },
)

return { parallelize: { consolidation: par.consolidation, disqualified: par.disqualified, failed: par.failed }, manifest, eval: evaluation }
