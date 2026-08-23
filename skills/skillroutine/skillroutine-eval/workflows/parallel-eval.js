export const meta = {
  name: 'skillroutine-parallel-eval',
  description: 'Pipe: render skill variants, nest the parallelize workflow over a fresh matrix per skill name, then score every surviving run with skillroutine-eval (v2, three axes)',
  whenToUse: 'Invoked by skillroutine-eval matrix mode with the matrix spec, eval context, skill variants, and resolved script/skillMd paths as args',
  phases: [
    { title: 'Variants', detail: 'render each skill variant into ~/.claude/skills/<leaf>--<name>/ and copy the rendered SKILL.md files into resultsDir' },
    { title: 'Fan-out', detail: 'nested parallelize workflow per skill name — one cell per matrix entry, isolated worktrees' },
    { title: 'Manifest', detail: 'copy artifacts out of cell worktrees, write the v2 eval manifest' },
    { title: 'Eval', detail: 'one agent runs skillroutine-eval on the manifest, unattended' },
    { title: 'Cleanup', detail: 'remove the rendered variant skill dirs' },
  ],
}

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch { throw new Error('args arrived as a string and is not valid JSON') }
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object: {parallelizeScript, syncSkillsScript, skill, skillArgs, contextDoc, models, efforts, argVariants, replicas, baseCommit, skillMd, contextFiles, qualityContext, skillVariants, resultsDir}')
}
for (const key of ['parallelizeScript', 'syncSkillsScript', 'skill', 'baseCommit', 'skillMd']) {
  if (typeof args[key] !== 'string' || !args[key]) throw new Error(`args.${key} is required (string)`)
}
const skillVariants = Array.isArray(args.skillVariants) ? args.skillVariants : []
for (const variant of skillVariants) {
  const wellFormed = variant && typeof variant.name === 'string' && Array.isArray(variant.disable) && variant.disable.length
  if (!wellFormed) throw new Error('skillVariants[] entries are {name: string, disable: [ids or globs, non-empty]}')
}
const cellsPerSkill = (args.models?.length || 1) * (args.efforts?.length || 1) * (args.argVariants?.length || 1) * (args.replicas || 1)
if (cellsPerSkill * (1 + skillVariants.length) > 16) {
  throw new Error(`matrix ${cellsPerSkill} cells × ${1 + skillVariants.length} skill names exceeds the 16-cell cap`)
}

const resultsDir = typeof args.resultsDir === 'string' && args.resultsDir
  ? args.resultsDir.replace(/\/+$/, '')
  : `evals/${args.skill.replace(/[^a-zA-Z0-9._-]+/g, '-')}-${args.baseCommit.slice(0, 6)}`
const contextFiles = Array.isArray(args.contextFiles) ? args.contextFiles : []
const qualityContext = Array.isArray(args.qualityContext) ? args.qualityContext : []
const argVariants = Array.isArray(args.argVariants) && args.argVariants.length ? args.argVariants : [args.skillArgs ?? '']
const inputs = [...new Set([...argVariants.filter(v => typeof v === 'string' && v), ...(args.contextDoc ? [args.contextDoc] : [])])]
if (!inputs.length) inputs.push('(no skill args — bare invocation)')

// Skill names to run: the full skill plus one rendered variant per entry.
const skillsToRun = [
  { name: 'default', skill: args.skill, disable: [] },
  ...skillVariants.map(v => ({ name: v.name, skill: `${args.skill}--${v.name}`, disable: v.disable })),
]

const RENDERED_SCHEMA = {
  type: 'object',
  required: ['variants'],
  properties: {
    variants: {
      type: 'array',
      items: {
        type: 'object',
        required: ['name', 'dir', 'skillMd'],
        properties: {
          name: { type: 'string' },
          dir: { type: 'string', description: 'installed variant dir (~/.claude/skills/<leaf>--<name>)' },
          skillMd: { type: 'string', description: 'copy of the rendered SKILL.md under resultsDir/skills/' },
        },
      },
    },
  },
}

phase('Variants')
if (skillVariants.length) {
  const rendered = await agent(
    `Mechanical task, no judgment. Render the skill variants for a bake-off of "${args.skill}".\n\n` +
    `1. mkdir -p ${resultsDir}/skills.\n` +
    `2. For each variant run the command, in order:\n` +
    skillVariants.map(v => `   bash ${args.syncSkillsScript} --variant ${args.skill}=${v.name}:${v.disable.join(',')}`).join('\n') + `\n` +
    `   Each prints "rendered variant <leaf>--<name> (...) -> <dir>". A non-zero exit (unknown id, bad name) is fatal — report it and stop.\n` +
    `3. Copy each <dir>/SKILL.md to ${resultsDir}/skills/<name>.md.\n` +
    `Return the structured output: one row per variant with name, dir, and the copied skillMd path.`,
    { label: 'render-variants', phase: 'Variants', agentType: 'general-purpose', effort: 'low', schema: RENDERED_SCHEMA },
  )
  if (!rendered) throw new Error('variant render agent died — no cells were started')
} else {
  log('no skill variants — full skill only')
}

phase('Fan-out')
const runs = await parallel(skillsToRun.map(s => () =>
  workflow({ scriptPath: args.parallelizeScript }, {
    skill: s.skill,
    skillArgs: args.skillArgs,
    contextDoc: args.contextDoc,
    models: args.models,
    efforts: args.efforts,
    argVariants: args.argVariants,
    replicas: args.replicas,
    baseCommit: args.baseCommit,
  }).then(par => ({ variant: s, par })),
))
const finished = runs.filter(Boolean)

const surviving = finished.flatMap(({ variant, par }) => {
  const disqualifiedIds = new Set((par.disqualified ?? []).map(r => r.run_id))
  return (par.cells ?? []).filter(r => !disqualifiedIds.has(r.run_id)).map(r => ({ ...r, variant }))
})
const disqualifiedCount = finished.reduce((n, { par }) => n + (par.disqualified ?? []).length, 0)
const deadCount = finished.reduce((n, { par }) => n + (par.failed ?? []).length, 0)
if (!surviving.length) {
  log('no surviving cells — nothing to evaluate')
  return { parallelize: finished, manifest: null, eval: null }
}
log(`${surviving.length} surviving run(s) across ${finished.length} skill name(s), ${disqualifiedCount} disqualified, ${deadCount} dead`)

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
  `Mechanical task, no judgment. Build a skillroutine-eval v2 manifest for a finished matrix bake-off of the skill "${args.skill}".\n\n` +
  `Surviving cells (structured output of the parallelize workflow, each tagged with its skill variant):\n${JSON.stringify(surviving, null, 2)}\n\n` +
  `1. Create the directory ${resultsDir}/runs/.\n` +
  `2. For each cell, copy the file at artifact_path (inside the cell's worktree — see worktree_ref) to ` +
  `${resultsDir}/runs/<run_id sanitized to [a-zA-Z0-9._-]>.<original extension>. Copy FIRST, before anything else — ` +
  `cell worktrees are ephemeral. If a worktree is already gone, try \`git show <cell branch>:<artifact path relative to repo root>\` ` +
  `(branches are named claude/parallelize-*); if that also fails, the run goes in missing[] and is left out of the manifest.\n` +
  `3. Write ${resultsDir}/manifest.json with exactly this shape (the skillroutine-eval manifest contract, schemaVersion 2.0):\n` +
  `   {\n` +
  `     "schemaVersion": "2.0",\n` +
  `     "skill": ${JSON.stringify(args.skill)},\n` +
  `     "skillMd": ${JSON.stringify(args.skillMd)},\n` +
  `     "contextFiles": ${JSON.stringify(contextFiles)},\n` +
  `     "qualityContext": ${JSON.stringify(qualityContext)},\n` +
  `     "inputs": ${JSON.stringify(inputs)},\n` +
  `     "runs": [ per surviving cell: { "id": <run_id>, "output": <copied path>, ` +
  `"skillMd": "${resultsDir}/skills/<variant.name>.md" (${JSON.stringify(args.skillMd)} for the full skill), ` +
  `"variant": { "name": <variant.name>, "disable": <variant.disable> } (omit the key for the default), ` +
  `"transcript": <the cell's session JSONL path when the cell reported one, else omit>, ` +
  `"worktree": <worktree_ref when the directory still exists, else omit>, ` +
  `"model": { "id": <cell model, or "session" if null/absent>, "effort": <cell effort, omit the key if null/absent> } } ],\n` +
  `     "output": "${resultsDir}/eval.json",\n` +
  `     "md": true\n` +
  `   }\n` +
  `4. Validate with jq: manifest parses, every runs[].output and runs[].skillMd file exists, required fields present.\n\n` +
  `Return the structured output: manifest_path and missing[].`,
  { label: 'manifest', phase: 'Manifest', agentType: 'general-purpose', effort: 'low', schema: MANIFEST_SCHEMA },
)
if (!manifest) {
  return { parallelize: finished, manifest: null, eval: null, error: 'manifest agent died — artifacts may still be in cell worktrees' }
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
      description: 'per run × axis totals as reported by skillroutine-eval',
      items: {
        type: 'object',
        required: ['run_id', 'axis', 'pct'],
        properties: {
          run_id: { type: 'string' },
          axis: { type: 'string', enum: ['self', 'context', 'output'] },
          pct: { type: 'number' },
        },
      },
    },
    sharedTotals: {
      type: 'array',
      description: 'per run × axis over the rows every variant shares — only when variants differ',
      items: {
        type: 'object',
        required: ['run_id', 'axis', 'pct'],
        properties: {
          run_id: { type: 'string' },
          axis: { type: 'string', enum: ['self', 'context', 'output'] },
          pct: { type: 'number' },
        },
      },
    },
    ranking: {
      type: 'array',
      description: 'per axis: run_ids best-first',
      items: {
        type: 'object',
        required: ['axis', 'run_ids'],
        properties: {
          axis: { type: 'string', enum: ['self', 'context', 'output'] },
          run_ids: { type: 'array', items: { type: 'string' } },
        },
      },
    },
    deltas: {
      type: 'array',
      description: 'per axis and variant: variant minus default over the shared rows, in pct points',
      items: {
        type: 'object',
        required: ['axis', 'variant', 'delta_pct'],
        properties: {
          axis: { type: 'string', enum: ['self', 'context', 'output'] },
          variant: { type: 'string' },
          delta_pct: { type: 'number' },
        },
      },
    },
  },
}

phase('Eval')
const evaluation = await agent(
  `Invoke the Skill tool with skill="skillroutine-eval" and args="${manifest.manifest_path}", then follow the loaded skill ` +
  `exactly and unattended — never wait for user approval; where the skill says to stop for user review, instead finish ` +
  `and return. The manifest is already written and validated; do not rebuild it.\n\n` +
  `Return the structured output: the eval JSON path (and md path if rendered), per run × axis totals (and sharedTotals when variants differ), ` +
  `the per-axis ranking best-first, and per-axis variant deltas versus the default when variants were run.`,
  { label: 'skillroutine-eval', phase: 'Eval', agentType: 'general-purpose', schema: EVAL_SCHEMA },
)

phase('Cleanup')
if (skillVariants.length) {
  await agent(
    `Mechanical task. Remove the rendered variant skill dirs — they are eval-only leftovers, copies live in ${resultsDir}/skills/:\n` +
    skillVariants.map(v => `   rm -rf ~/.claude/skills/${args.skill}--${v.name}`).join('\n') + `\n` +
    `Return the word done.`,
    { label: 'cleanup-variants', phase: 'Cleanup', agentType: 'general-purpose', effort: 'low' },
  )
}

return {
  parallelize: finished.map(({ variant, par }) => ({ variant: variant.name, consolidation: par.consolidation, disqualified: par.disqualified, failed: par.failed })),
  manifest,
  eval: evaluation,
}
