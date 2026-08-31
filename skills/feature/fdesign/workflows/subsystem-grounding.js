export const meta = {
  name: 'subsystem-grounding',
  description: 'Ground a multi-subsystem fchange/fdesign plan against the live tree: fan one Explore agent per named subsystem, collect each agent\'s file:line report at a barrier, then fold every report verbatim into the plan skeleton',
  whenToUse: 'Invoked by /fdesign (or /fdesign change) when the drivers span two or more subsystems and per-subsystem grounding is worth parallelizing',
  phases: [
    { title: 'Ground', detail: 'one Explore agent per subsystem returns a file:line report of that subsystem in the live tree (barrier: all reports collected)' },
    { title: 'Fold', detail: 'one agent folds each report verbatim into the diffed plan, returning the grounded sections' },
  ],
}

// args contract (built by the fronting fdesign skill from the drivers):
// { subsystems: string[], drivers?: string, planRef?: string }
// subsystems — the named subsystems the drivers touch (UI, pipeline/schema, sync, …); one Explore agent per entry.
// drivers    — optional free-text of the change/feature drivers, passed to every agent for scoping context.
// planRef    — optional repo-relative path of the plan skeleton drafted concurrently by the front; when set the Fold
//              agent folds the reports into that file in place, otherwise it returns grounded_sections for the front to fold.
if (!args || typeof args !== 'object' || Array.isArray(args)) {
  throw new Error('args must be an object: {subsystems, drivers, planRef}')
}
const subsystems = Array.isArray(args.subsystems) ? args.subsystems.filter(s => typeof s === 'string' && s) : []
if (subsystems.length === 0) {
  throw new Error('args.subsystems is required — the named subsystems the drivers touch, one Explore agent per entry')
}
const drivers = typeof args.drivers === 'string' ? args.drivers : ''
const planRef = typeof args.planRef === 'string' && args.planRef ? args.planRef : ''

const GROUND_SCHEMA = {
  type: 'object',
  required: ['subsystem', 'file_line_report'],
  properties: {
    subsystem: { type: 'string', description: 'the subsystem this agent grounded' },
    file_line_report: {
      type: 'array',
      items: { type: 'string' },
      description: 'verbatim file:line findings, one per entry, e.g. "internal/server/handlers.go:212 — handleToggleHook moves hooks to settings.disabled.json"; the plan consumes these verbatim',
    },
    worktree_ref: { type: 'string', description: 'the tree the agent grounded against (branch or commit it read)' },
    notes: { type: 'string', description: 'scope boundaries, gaps, anything the report could not resolve' },
  },
}

const FOLD_SCHEMA = {
  type: 'object',
  required: ['grounded_sections'],
  properties: {
    plan_ref: { type: 'string', description: 'the plan file the reports were folded into, empty when no planRef was given' },
    grounded_sections: {
      type: 'array',
      items: { type: 'string' },
      description: 'one entry per subsystem — the plan section grounded from that subsystem\'s file:line report',
    },
    notes: { type: 'string' },
  },
}

// ---- Stage A/B: Ground (one Explore agent per subsystem; parallel() is the barrier) ----
phase('Ground')
const reports = await parallel(subsystems.map(subsystem => () =>
  agent(
    `Ground the "${subsystem}" subsystem against the LIVE tree for an fchange/fdesign plan. Work from the repo root. ` +
    (drivers ? `The plan drivers are: ${drivers}. ` : '') +
    `Scope yourself to this one subsystem's files only. Return a file:line report — every finding as a "path:line — what is there" ` +
    `string the plan will consume verbatim: the concrete functions, types, handlers, templates, schema fields, and call sites the drivers ` +
    `would touch in this subsystem. Read the real files; never ground from memory. Also return the worktree_ref you read (branch or commit) ` +
    `and any scope gaps in notes. This is read-only grounding — never edit a file.`,
    { label: `ground ← ${subsystem}`, phase: 'Ground', agentType: 'Explore', schema: GROUND_SCHEMA },
  )
))

const grounded = reports.map((r, i) => r || {
  subsystem: subsystems[i],
  file_line_report: [],
  worktree_ref: '',
  notes: `ground agent returned nothing for ${subsystems[i]}`,
})
const failed = subsystems.filter((_, i) => !reports[i])

// ---- Stage C: Fold every report verbatim into the plan ----
phase('Fold')
const reportBlock = grounded
  .map(r => `## ${r.subsystem}${r.worktree_ref ? ` (grounded against ${r.worktree_ref})` : ''}\n` +
    (r.file_line_report.length ? r.file_line_report.map(l => `- ${l}`).join('\n') : '- (no findings)') +
    (r.notes ? `\n_notes: ${r.notes}_` : ''))
  .join('\n\n')

const fold = await agent(
  `Fold these per-subsystem file:line grounding reports into the fchange/fdesign plan. Work from the repo root. ` +
  (planRef
    ? `The plan skeleton is at "${planRef}" — edit it in place, folding each subsystem's report into its matching plan section so every ` +
      `plan claim is anchored to a real path:line, and return plan_ref="${planRef}". `
    : `No plan file was given — return one grounded_sections[] entry per subsystem, each the plan section text grounded from that report, ` +
      `for the front to fold into the plan. `) +
  `Preserve every file:line reference verbatim; never invent or paraphrase a path or line. Reports:\n\n${reportBlock}`,
  { label: 'fold reports → plan', phase: 'Fold', agentType: 'general-purpose', schema: FOLD_SCHEMA },
) || { plan_ref: '', grounded_sections: [], notes: 'fold agent returned nothing' }

return {
  subsystems,
  reports: grounded,
  failed,
  fold,
}
