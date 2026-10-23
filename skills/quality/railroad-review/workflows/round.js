export const meta = {
  name: 'railroad-round',
  description: 'Run one railroad-review round in a single invocation: fan-out, dedup, second confirmation and station as child workflows, prior stage returns handed to the next in memory',
  whenToUse: 'Invoked by the /railroad-review dispatcher once per round on the interactive route (SKILL MODES-003a); cleanup is the dispatcher\'s $SKILL_RAILROADREVIEW_CLEANUP cleanup after the fix pipeline',
  phases: [
    { title: 'Fan-out' },
    { title: 'Dedup' },
    { title: 'Second confirmation' },
    { title: 'Station' },
  ],
}

// SKILL: SKILL-RAILROADREVIEW-MODES-003a (Stage files) — round orchestrator; each child
// stage validates its own args, this file only composes and forwards.
if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — the dispatcher assembles the round args per the Stage files def')
}
if (typeof args.workflowsDir !== 'string' || !args.workflowsDir) {
  throw new Error('args.workflowsDir is required — the absolute path of this skill\'s workflows/ directory, so the child stage files resolve')
}
if (typeof args.artifactsDir !== 'string' || !args.artifactsDir) {
  throw new Error('args.artifactsDir is required — resolved by the dispatcher, never assumed')
}
if (!Number.isInteger(args.round) || args.round <= 0) {
  throw new Error('args.round is required — the station-protocol round number (1-based)')
}
if (!args.buildTest || typeof args.buildTest.build !== 'string' || typeof args.buildTest.tests !== 'string') {
  throw new Error('args.buildTest is required — {build, tests} from $SKILL_RAILROADREVIEW_BUILDTEST buildtest at the review head')
}
if (!args.baseHealth || typeof args.baseHealth.build !== 'string' || typeof args.baseHealth.tests !== 'string') {
  throw new Error('args.baseHealth is required — {build, tests} from $SKILL_RAILROADREVIEW_BUILDTEST buildtest at the base')
}
const stage = name => ({ scriptPath: `${args.workflowsDir}/${name}` })

phase('Fan-out')
const lanes = await workflow(stage('lanes.js'), args)
const abortedLanes = lanes.abortedLanes || []
// CTX-003: missing context is reported and the direction runs degraded — never a finding, never a halt.
const contextGaps = (lanes.perDirection || []).flatMap(fd => (fd.notes || '').split(' | ')
  .filter(note => note.startsWith('MISSING CONTEXT:'))
  .map(note => ({ direction: fd.direction, note })))

// A round whose every planned lane aborted reviewed nothing: no dedup, no second
// confirmation, no station and no handoff — the dispatcher records it with ledger --void.
const abortedManifests = new Set(abortedLanes.map(lane => lane.manifest))
if (args.lanePlan.every(lane => abortedManifests.has(lane.manifest))) {
  log(`void round: all ${args.lanePlan.length} planned lane(s) aborted — dedup, second confirmation and station skipped`)
  return {
    round: args.round,
    void: true,
    station: null,
    clean: false,
    abortedLanes,
    contextGaps,
    deadDirections: lanes.deadDirections || [],
    branches: lanes.branches || [],
  }
}

// Each lanes.perDirection entry carries claim REFS (id, file, line, severity, cells, fix,
// artifact path) — hand them to dedup, which recomputes severity from the cells; claim
// bodies live only in the artifact files, read by agents via jq.
const claimsByDirection = (lanes.perDirection || [])
  .filter(fd => Array.isArray(fd.claims) && fd.claims.length)
  .map(fd => ({ direction: fd.direction, claims: fd.claims }))
let dedup = { claims: [], duplicates: [], withinDuplicates: 0, unionCount: 0, corrections: [] }
let conf = { claims: [], verifications: [], unsettled: [], branches: [] }
if (claimsByDirection.length) {
  phase('Dedup')
  dedup = await workflow(stage('dedup.js'), { ...args, claimsByDirection })
  phase('Second confirmation')
  conf = await workflow(stage('verify.js'), { ...args, claims: dedup.claims })
} else {
  log('no lane produced a claim — dedup and second confirmation skipped')
}

phase('Station')
const verdictById = new Map((conf.verifications || []).map(v => [v.finding_id, v]))
const verdictClaims = (conf.claims || []).map(c => ({ ...c, verdict: verdictById.get(c.finding_id) || null }))
const station = await workflow(stage('station.js'), {
  ...args,
  abortedLanes,
  claims: verdictClaims,
  duplicates: dedup.duplicates || [],
  buildTest: args.buildTest,
  perDirection: lanes.perDirection || [],
  unionCount: dedup.unionCount || 0,
  withinDuplicates: dedup.withinDuplicates || 0,
  corrections: dedup.corrections || [],
})

// ROUND-020 funnel conservation: every produced claim ends in exactly one funnel bucket.
const produced = (lanes.perDirection || []).reduce((n, fd) => n + (fd.claims_produced || 0), 0)
const funnel = station.funnel || {}
const accounted = ['confirmed', 'unverified', 'duplicates', 'rejected', 'debunked'].reduce((n, key) => n + (funnel[key] || 0), 0)
const conservation = { produced, accounted, lost: produced - accounted }
if (conservation.lost !== 0) log(`funnel conservation broken: ${JSON.stringify(conservation)}`)

return {
  round: args.round,
  void: false,
  station,
  clean: !!station.handoff && station.blocking === 0 && abortedLanes.length === 0 && conservation.lost === 0,
  conservation,
  corrections: dedup.corrections || [],
  abortedLanes,
  contextGaps,
  deadDirections: lanes.deadDirections || [],
  unsettled: conf.unsettled || [],
  branches: [...(lanes.branches || []), ...(conf.branches || []), ...(station.branches || [])],
}
