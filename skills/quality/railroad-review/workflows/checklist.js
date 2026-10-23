export const meta = {
  name: 'railroad-review-checklist',
  description: 'The railroad-review checklist stage: one checklist agent per judgment direction lacking a checklist writes round-{r}/checklist-<direction>.json',
  whenToUse: 'Invoked by /railroad-review at its Checklist agents spawn (WALK-004), before $SKILL_RAILROADREVIEW_LANES lanes — contracts every round, correctness in round 1',
  phases: [
    { title: 'Checklist' },
  ],
}

// GEN:schemas checklist
const CHECKLIST_SCHEMA = {
  "type": "object",
  "required": [
    "path",
    "groups",
    "items"
  ],
  "properties": {
    "path": {
      "type": "string",
      "description": "absolute path of the checklist-<direction>.json the agent wrote"
    },
    "groups": {
      "type": "integer",
      "description": "number of groups written"
    },
    "items": {
      "type": "integer",
      "description": "number of items written across all groups"
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /railroad-review passes {artifactsDir, round, baseCommit, headCommit, directions}')
}
for (const key of ['artifactsDir', 'baseCommit', 'headCommit']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — resolved by the dispatcher, never assumed`)
  }
}
if (!Number.isInteger(args.round) || args.round <= 0) {
  throw new Error('args.round is required — the station-protocol round number (1-based)')
}
const JUDGMENT_DIRECTIONS = ['contracts', 'correctness']
if (!Array.isArray(args.directions) || !args.directions.length || args.directions.some(d => !JUDGMENT_DIRECTIONS.includes(d))) {
  throw new Error(`args.directions is required — the selected judgment directions, a subset of ${JUDGMENT_DIRECTIONS.join(', ')}`)
}
if (args.specPaths !== undefined && (!Array.isArray(args.specPaths) || args.specPaths.some(p => typeof p !== 'string' || !p))) {
  throw new Error('args.specPaths must be an array of the round-dir spec copies — empty when no spec is given')
}
if (args.round > 1 && args.directions.includes('correctness')) {
  throw new Error('the correctness checklist is extracted in round 1 only — later rounds pass round-1/checklist-correctness.json to $SKILL_RAILROADREVIEW_LANES lanes')
}
const specPaths = Array.isArray(args.specPaths) ? args.specPaths : []
if (args.directions.includes('correctness') && !specPaths.length) {
  throw new Error('the correctness checklist requires args.specPaths — no spec means the correctness direction is dropped')
}
// The specs travel as the dispatcher's copies under the artifacts dir — never paths inside the dispatcher's checkout.
const outsideSpecs = specPaths.filter(p => !p.startsWith(`${args.artifactsDir}/`))
if (outsideSpecs.length) {
  throw new Error(`args.specPaths must be the dispatcher's copies under ${args.artifactsDir}/ — never paths inside the dispatcher's checkout: ${outsideSpecs.join(', ')}`)
}
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
const opts = { agentType: 'railroad-checklist', schema: CHECKLIST_SCHEMA }
opts.model = assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL)
const walkGroup = Number.isInteger(args.walkGroup) && args.walkGroup > 0 ? args.walkGroup : 10
const roundDir = `${args.artifactsDir}/round-${args.round}`
const excludes = Array.isArray(args.excludes) ? args.excludes.filter(p => typeof p === 'string' && p) : []
const diffCmd = `git diff ${args.baseCommit} ${args.headCommit}` +
  (excludes.length ? ` -- . ${excludes.map(p => `':(exclude)${p}'`).join(' ')}` : '')

const sourceOf = direction => direction === 'correctness'
  ? `the requirements of every spec listed here, read each in full: ${specPaths.join(', ')}. Group the items per spec ` +
    `(a group id names its spec's basename), never merge two specs' requirements into one item, and drop a requirement ` +
    `another listed spec explicitly supersedes; omit requirements naming only tests — tests lanes check those`
  : `the changed boundaries (exported signatures, schemas, wire formats, CLI flags, config keys) in \`${diffCmd}\``

const promptOf = direction =>
  `EXTRACT ${sourceOf(direction)} as numbered checklist items, grouped coherently.\n` +
  `WRITE ${roundDir}/checklist-${direction}.json with the Write tool as the JSON object ` +
  `{"shape": "group-first", "head": "${args.headCommit}", "groups": [{"id": "<group name>", "items": ["${direction}-<n>: <item text>", ...]}]} — ` +
  `head the commit given here ($SKILL_RAILROADREVIEW_LANES lanes carries the file into the next round while the head stands), ` +
  `<n> numbers items across all groups from 1; each group holds at most ${walkGroup} items.\n` +
  `RETURN {"path", "groups", "items"} — the path written and the group and item counts.`

phase('Checklist')
const results = await parallel(args.directions.map(direction => () => agent(promptOf(direction), { ...opts, label: `checklist:${direction}` })))
const checklists = args.directions.map((direction, index) => ({ direction, ...results[index] }))
for (const checklist of checklists) {
  if (!checklist.path || checklist.items <= 0) {
    throw new Error(`checklist ${checklist.direction} came back empty — ${JSON.stringify(checklist)}`)
  }
}
return { checklists }
