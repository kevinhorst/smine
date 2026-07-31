export const meta = {
  name: 'railroad-review',
  description: 'Fan a code review out to n lanes per railroad-review direction in isolated worktrees, merge each direction, then merge all directions into one station review with JSON+MD handoff artifacts',
  whenToUse: 'Invoked by the /railroad-review skill in railroad mode, once per station-protocol round, with the confirmed diff base, head, directions, and lane count as args',
  phases: [
    { title: 'Fan-out', detail: 'n lanes per direction, isolated worktrees at the review head tip, review-only' },
    { title: 'Direction merge', detail: 'one agent per direction: union its lanes, dedup, reconcile severities' },
    { title: 'Station merge', detail: 'single barrier agent: union all directions, disposition, re-verify, ranked fix plan, JSON+MD handoff' },
    { title: 'Cleanup', detail: 'remove this round\'s claude/railroad-* worktrees and branches via the agent-toolset remove_agent_worktrees.sh — safety-gated, never forced' },
  ],
}

// One station-protocol ROUND. The human gate, the rejection ledger file, the fix
// handoff, and the approved-twice loop live in the skill front — a workflow cannot
// prompt or await a merge-back. The front passes the ledger CONTENT via
// args.rejectedFindings (lane worktrees fork from committed state; an uncommitted
// ledger file would be invisible to them).
if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object: {base, head, baseCommit, headCommit, commitsAhead, artifactsDir, diffFiles, round, directions, lanes, rejectedFindings}')
}
for (const key of ['base', 'head', 'baseCommit', 'headCommit', 'artifactsDir']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — resolved by the skill front, never assumed`)
  }
}
if (!Number.isInteger(args.commitsAhead) || args.commitsAhead <= 0) {
  throw new Error('args.commitsAhead must be a positive integer — empty diff means stop and ask for the range, not fan out')
}
if (!Array.isArray(args.diffFiles) || !args.diffFiles.length || args.diffFiles.some(f => typeof f !== 'string')) {
  throw new Error('args.diffFiles is required — `git diff --name-only <base>...<head>`, the coverage baseline every lane is checked against')
}
if (!Number.isInteger(args.round) || args.round <= 0) {
  throw new Error('args.round is required — the station-protocol round number (1-based); it namespaces artifacts and branch names')
}

const KNOWN_DIRECTIONS = [
  'code-style', 'correctness', 'critical', 'data-integrity',
  'contracts', 'tests', 'security', 'special-focus',
]
const DEFAULT_DIRECTIONS = ['code-style', 'correctness', 'critical']

const directions = Array.isArray(args.directions) && args.directions.length ? args.directions : DEFAULT_DIRECTIONS
for (const d of directions) {
  if (!KNOWN_DIRECTIONS.includes(d)) throw new Error(`unknown direction "${d}" — use one of ${KNOWN_DIRECTIONS.join('/')}`)
}
if (directions.length < 2) {
  throw new Error('railroad mode needs at least two directions — a single direction is solo mode, run the skill directly')
}
if (directions.includes('special-focus') && (typeof args.focus !== 'string' || !args.focus)) {
  throw new Error('the special-focus direction requires args.focus — the explicit focus argument')
}
const lanes = Number.isInteger(args.lanes) && args.lanes > 0 ? args.lanes : 2

const rejected = Array.isArray(args.rejectedFindings) ? args.rejectedFindings : []
const contextRefs = typeof args.contextRefs === 'string' ? args.contextRefs : ''

const laneOpts = {}
if (typeof args.laneModel === 'string' && args.laneModel) laneOpts.model = args.laneModel
if (typeof args.laneEffort === 'string' && args.laneEffort) laneOpts.effort = args.laneEffort

const hash6 = args.baseCommit.slice(0, 6)
const r = args.round
const roundDir = `${args.artifactsDir}/round-${r}`

const ledgerBlock = rejected.length
  ? `\nREJECTED-FINDINGS LEDGER — these findings were reviewed by a human and rejected with the reasons given. ` +
    `Do NOT re-report them or trivial variants of them; re-reporting a ledger entry is itself a defect:\n` +
    `${JSON.stringify(rejected, null, 2)}\n`
  : ''

// Cleanup defined before the fan-out so the all-lanes-dead early return reaches it.
// The agent only drives the deployed toolset script — remove_agent_worktrees.sh owns
// the safety rules; --force is never passed.
const CLEANUP_SCHEMA = {
  type: 'object',
  required: ['removed', 'skipped', 'unattributed'],
  properties: {
    removed: { type: 'array', items: { type: 'string' }, description: 'branches whose worktree and branch the toolset script removed' },
    skipped: { type: 'array', items: { type: 'string' }, description: 'expected names left in place — name plus the script\'s verbatim refusal reason' },
    unattributed: { type: 'array', items: { type: 'string' }, description: 'worktrees at the review head matching no expected name — reported only, never touched' },
  },
}

async function runCleanup() {
  phase('Cleanup')
  const expectedBranches = [
    ...directions.flatMap(d => Array.from({ length: lanes }, (_, i) => `claude/railroad-${d}-l${i + 1}-r${r}-${hash6}`)),
    ...directions.map(d => `claude/railroad-merge-${d}-r${r}-${hash6}`),
    `claude/railroad-station-r${r}-${hash6}`,
  ].flatMap(b => [b, `${b}-2`])
  return agent(
    `You are the cleanup agent of a railroad-review round on ${args.base}...${args.head}, running in the orchestrator checkout — ` +
    `no isolation, real repo.\n\n` +
    `The round's agents renamed their worktree branches to these expected names:\n${expectedBranches.join('\n')}\n\n` +
    `Cleanup tool: ~/.claude/agents/tools/remove_agent_worktrees.sh (deployed by the subagent sync). If it is missing, ` +
    `do NOT improvise git commands — put "toolset missing: run cmd/sync/sync_skills.sh" into skipped and return.\n\n` +
    `For EACH expected name that \`git branch --list\` shows, run:\n` +
    `\`~/.claude/agents/tools/remove_agent_worktrees.sh --delete-branch <name>\`\n` +
    `NEVER pass --force. The script removes the worktree and branch only when the tree is pristine and the commits ` +
    `live on a non-claude branch; record its "removed"/"deleted branch" lines in removed and every "skipped" line ` +
    `verbatim in skipped.\n\n` +
    `Additionally: any OTHER worktree whose HEAD is ${args.headCommit} and whose branch matches no expected name goes into ` +
    `unattributed — report it, do NOT remove or rename it (it may belong to another session).\n` +
    `Never touch branches outside the expected list, the orchestrator checkout, or its branch. Return the structured output only.`,
    { label: 'cleanup', phase: 'Cleanup', agentType: 'general-purpose', effort: 'low', schema: CLEANUP_SCHEMA },
  )
}

const FINDING_PROPS = {
  finding_id: { type: 'string', description: 'stable id, unique across directions — prefix with the direction name' },
  file: { type: 'string' },
  line: { type: 'integer' },
  severity: { type: 'string', description: 'BLOCKER | WARNING | NIT' },
  claim: { type: 'string', description: 'the defect and why it violates the direction standard' },
  fix: { type: 'string', description: 'concrete proposed fix — one sentence or a short diff sketch' },
}

const LANE_SCHEMA = {
  type: 'object',
  required: ['direction', 'lane', 'aborted', 'artifact_json', 'artifact_md', 'files_reviewed', 'findings'],
  properties: {
    direction: { type: 'string', description: 'the railroad-review direction this lane walked' },
    lane: { type: 'integer', description: 'this lane\'s 1-based index within its direction' },
    aborted: { type: 'boolean', description: 'true when the premise check failed and the lane stopped without reviewing — an aborted lane never counts as a finished review' },
    artifact_json: { type: 'string', description: 'absolute path of the JSON findings artifact written under the round dir' },
    artifact_md: { type: 'string', description: 'absolute path of the MD findings artifact written under the round dir' },
    files_reviewed: {
      type: 'array', items: { type: 'string' },
      description: 'every changed file this lane actually read in full — the self-verification list checked against the diff',
    },
    findings: {
      type: 'array',
      items: { type: 'object', required: ['finding_id', 'file', 'line', 'severity', 'claim', 'fix'], properties: FINDING_PROPS },
    },
    notes: { type: 'string', description: 'missing context files, excluded scope, anomalies' },
  },
}

const DIRECTION_SCHEMA = {
  type: 'object',
  required: ['direction', 'artifact_json', 'artifact_md', 'findings', 'coverage_gaps'],
  properties: {
    direction: { type: 'string' },
    artifact_json: { type: 'string', description: 'absolute path of the merged direction report JSON under the round dir' },
    artifact_md: { type: 'string', description: 'absolute path of the merged direction report MD under the round dir' },
    findings: {
      type: 'array',
      description: 'the direction\'s lanes unioned and deduped; severities reconciled; ids keep the direction prefix',
      items: { type: 'object', required: ['finding_id', 'file', 'line', 'severity', 'claim', 'fix'], properties: FINDING_PROPS },
    },
    coverage_gaps: {
      type: 'array', items: { type: 'string' },
      description: 'diff files no lane of this direction reviewed — carried verbatim from the deterministic check',
    },
    notes: { type: 'string', description: 'lane disagreements worth surfacing, anomalies' },
  },
}

function lanePrompt(direction, i) {
  return (
    `You are lane ${i} of ${lanes} on the "${direction}" direction of a railroad-review fan-out on ${args.base}...${args.head} (round ${r}). ` +
    `You run in your own isolated git worktree at the review head tip. Other lanes review the same direction independently — ` +
    `never coordinate with them or read their artifacts.\n\n` +
    `FIRST ACTION: run \`git rev-parse HEAD\` and \`git merge-base --is-ancestor ${args.baseCommit} HEAD\` in your worktree. ` +
    `If HEAD does not equal ${args.headCommit}, or the ancestor check fails, return aborted=true with the exact commit facts in notes ` +
    `(artifact paths empty, no artifacts written) and stop — do not review from a different premise. Otherwise aborted=false.\n\n` +
    `SECOND ACTION: rename your worktree branch so any leftover lands in the claude/* namespace: ` +
    `\`git branch -m claude/railroad-${direction}-l${i}-r${r}-${hash6}\` — if the name is taken, append "-2". ` +
    `Rename only the branch; never move the worktree directory, it is harness-managed.\n\n` +
    `Invoke the Skill tool with skill="railroad-review", then follow the "${direction}" direction definition in its ` +
    `SKILL.md exactly — read only the context that direction needs, walk only that direction's checklist. ` +
    `Diff with \`git diff ${args.base}...HEAD\`; read each changed file in full, not just the hunks.\n` +
    `The changed files are:\n${args.diffFiles.join('\n')}\n` +
    `SELF-VERIFICATION: before returning, confirm you reviewed every file above; files_reviewed in your return lists ` +
    `exactly the files you actually read in full. Never pad it — an honest gap beats a false claim.\n` +
    (direction === 'special-focus' ? `Focus argument: ${args.focus}\n` : '') +
    (contextRefs ? `Additional context: ${contextRefs}\n` : '') +
    ledgerBlock +
    `\nHard constraints, violation disqualifies this lane:\n` +
    `1. Review ONLY — make no edits to any file. Fixes are a separate later step owned by a different agent.\n` +
    `2. Never read sibling worktrees or other agents' artifacts — your own artifact files are your only writes outside your worktree.\n` +
    `3. Do NOT enter plan mode and do NOT call ExitPlanMode. Write your findings as BOTH ` +
    `${roundDir}/${direction}-lane${i}.json (the structured findings) and ` +
    `${roundDir}/${direction}-lane${i}.md (human-readable, findings table per the skill's Output Format) — ` +
    `OUTSIDE your worktree, which must stay pristine (zero writes, zero untracked files) — then TERMINATE, never park on an approval prompt.\n\n` +
    `Return the structured output: direction, lane index, both artifact paths, files_reviewed, and one entry per finding ` +
    `(finding_id prefixed with "${direction}-", file, line, severity as BLOCKER/WARNING/NIT, the claim, and a concrete ` +
    `fix — one sentence or a short diff sketch, never empty), plus the aborted flag. Empty findings array with aborted=false ` +
    `is a valid, expected result — a clean review; aborted=true is a failed premise, never a clean review.`
  )
}

function directionMergePrompt(direction, finished, coverage) {
  return (
    `You are the direction-merge agent for the "${direction}" direction of a railroad-review on ${args.base}...${args.head} (round ${r}). ` +
    `You run in your own isolated git worktree at the current tree.\n\n` +
    `FIRST ACTION: rename your worktree branch: \`git branch -m claude/railroad-merge-${direction}-r${r}-${hash6}\` — ` +
    `if the name is taken, append "-2". Rename only the branch; never move the worktree directory.\n\n` +
    `${finished.length} lanes of this direction finished; their structured results follow. Read each lane's artifact_json — ` +
    `actively ingest it; never assume its content reached you.\n\n${JSON.stringify(finished, null, 2)}\n\n` +
    `Deterministic coverage check (diff files no lane reviewed):\n${JSON.stringify(coverage, null, 2)}\n\n` +
    `Merge this direction:\n` +
    `1. Union the lanes' findings; collapse duplicates (same file/line/claim substance) keeping the better-argued claim and fix.\n` +
    `2. Reconcile divergent severities for identical facts — re-read the code where lanes disagree.\n` +
    `3. Keep ids with the "${direction}-" prefix; re-number collisions.\n` +
    `4. Carry the coverage gaps verbatim into coverage_gaps; note lane disagreements worth surfacing in notes.\n` +
    `5. Review ONLY — no edits; your worktree stays pristine. Write the merged direction report as BOTH ` +
    `${roundDir}/${direction}.json and ${roundDir}/${direction}.md (findings table per the skill's Output Format), then return the structured output only.`
  )
}

phase('Fan-out')
const abortedLanes = []
const directionResults = await pipeline(
  directions,
  d => parallel(Array.from({ length: lanes }, (_, i) => () =>
    agent(lanePrompt(d, i + 1), {
      label: `${d}-l${i + 1}`, phase: 'Fan-out', agentType: 'general-purpose',
      isolation: 'worktree', schema: LANE_SCHEMA, ...laneOpts,
    })
  )),
  (laneResults, d) => {
    const returned = (laneResults || []).filter(Boolean)
    for (const l of returned.filter(l => l.aborted)) {
      abortedLanes.push({ direction: d, lane: l.lane, notes: l.notes || '' })
    }
    const finished = returned.filter(l => !l.aborted)
    if (returned.length > finished.length) {
      log(`direction ${d}: ${returned.length - finished.length}/${returned.length} lanes aborted on premise`)
    }
    if (!finished.length) return null
    const coverage = args.diffFiles.filter(f => !finished.some(l => l.files_reviewed.includes(f)))
    const perLane = finished.map(l => ({ lane: l.lane, missing: args.diffFiles.filter(f => !l.files_reviewed.includes(f)) }))
    if (finished.length === 1) {
      // one surviving lane: it IS the direction report — no merge agent for one input
      log(`direction ${d}: single surviving lane, skipping direction merge`)
      return Promise.resolve({
        direction: d, artifact_json: finished[0].artifact_json, artifact_md: finished[0].artifact_md,
        findings: finished[0].findings, coverage_gaps: coverage,
        notes: `single surviving lane (lane ${finished[0].lane}); ${finished[0].notes || ''}`,
      })
    }
    return agent(directionMergePrompt(d, finished, perLane), {
      label: `merge-${d}`, phase: 'Direction merge', agentType: 'general-purpose',
      isolation: 'worktree', schema: DIRECTION_SCHEMA,
    })
  },
)

const deadDirections = directions.filter((_, i) => !directionResults[i])
const finishedDirections = directionResults.filter(Boolean)
if (deadDirections.length) log(`dead directions (all lanes skipped, errored, or aborted): ${deadDirections.join(', ')}`)
if (!finishedDirections.length) {
  return { base: args.base, head: args.head, round: r, directions, lanes, failed: deadDirections, abortedLanes, consolidation: null, handoff: null, cleanup: await runCleanup() }
}

// Station merge (barrier): union all direction reports, disposition everything,
// re-verify against the CURRENT tree (fixes land out of band mid-review), and write
// the round's JSON+MD handoff artifacts. Like every other agent, it renames its
// worktree branch into the claude/* namespace so any leftover is visible to tooling.
const CONSOLIDATION_SCHEMA = {
  type: 'object',
  required: ['dispositions', 'ranked_plan', 'debunked', 'resolved', 'intentional_deviations', 'coverage_gaps', 'definition_of_done', 'recommendation', 'handoff_json', 'handoff_md'],
  properties: {
    dispositions: {
      type: 'array',
      description: 'every union\'d finding, each with exactly one disposition — nothing may lack one',
      items: {
        type: 'object',
        required: ['finding_id', 'disposition'],
        properties: {
          finding_id: { type: 'string' },
          disposition: { type: 'string', description: 'confirmed | rejected-with-reason | duplicate-of | debunked' },
          reason: { type: 'string', description: 'required for rejected-with-reason and debunked' },
          duplicate_of: { type: 'string', description: 'the surviving finding_id, required for duplicate-of' },
        },
      },
    },
    ranked_plan: {
      type: 'array',
      description: 'the confirmed findings as one ranked fix plan',
      items: { type: 'object', required: ['finding_id', 'severity', 'file', 'line', 'fix'], properties: FINDING_PROPS },
    },
    debunked: {
      type: 'array',
      description: 'findings not reproducible against the current tree',
      items: { type: 'object', required: ['finding_id', 'reason'], properties: { finding_id: { type: 'string' }, reason: { type: 'string' } } },
    },
    resolved: {
      type: 'array',
      description: 'findings already fixed out of band — finding to fixing-commit provenance',
      items: { type: 'object', required: ['finding_id', 'fixing_commit'], properties: { finding_id: { type: 'string' }, fixing_commit: { type: 'string', description: 'the commit hash that fixed it' } } },
    },
    intentional_deviations: {
      type: 'array',
      description: 'deviations to canonicalize rather than fix',
      items: { type: 'object', required: ['claim', 'canonicalize_as'], properties: { claim: { type: 'string' }, canonicalize_as: { type: 'string' } } },
    },
    coverage_gaps: {
      type: 'array', items: { type: 'string' },
      description: 'diff files not covered by any lane of any direction — from the direction reports',
    },
    definition_of_done: {
      type: 'array',
      description: 'the DoD walk — happens here in consolidation, it is not a direction',
      items: {
        type: 'object',
        required: ['criterion', 'status'],
        properties: { criterion: { type: 'string' }, status: { type: 'string', description: 'PASS | FAIL | N/A' }, notes: { type: 'string' } },
      },
    },
    recommendation: { type: 'string', description: 'APPROVE | CONDITIONAL | REJECT' },
    handoff_json: { type: 'string', description: 'absolute path of the round\'s consolidated review.json' },
    handoff_md: { type: 'string', description: 'absolute path of the round\'s consolidated review.md' },
  },
}

phase('Station merge')
const consolidation = await agent(
  `You are the station-merge barrier of a railroad-review on ${args.base}...${args.head} (round ${r}). ` +
  `You run in your own isolated git worktree, forked from the orchestrator's checkout after the fan-out — ` +
  `for re-verification purposes it IS the current tree.\n\n` +
  `FIRST ACTION: rename your worktree branch: \`git branch -m claude/railroad-station-r${r}-${hash6}\` — ` +
  `if the name is taken, append "-2". Rename only the branch; never move the worktree directory.\n\n` +
  `${finishedDirections.length} direction reports follow. Read each artifact_json — actively ingest it; ` +
  `never assume one direction's content reached another.\n\n${JSON.stringify(finishedDirections, null, 2)}\n\n` +
  ledgerBlock +
  `\nMerge (the trains arrive at the station):\n` +
  `1. Union ALL findings from ALL direction reports — the full universe, no pick-one-report aggregation.\n` +
  `2. Give every finding exactly one disposition: confirmed | rejected-with-reason | duplicate-of(id) | debunked. Nothing may lack one.\n` +
  `3. Any finding matching a ledger entry above gets disposition "rejected-with-reason" citing the ledger — and note the lane that re-reported it.\n` +
  `4. Re-verify every BLOCKER/WARNING finding against PRIMARY SOURCES — re-read the actual code in the current tree ` +
  `(fixes land out of band mid-review). Not reproducible -> "debunked" with a reason; already fixed by a commit -> resolved[] with the hash.\n` +
  `5. Reconcile divergent severities across directions. Resolve spec-vs-code conflicts against decisions.md — a spec-explicit choice is not a code bug.\n` +
  `6. Separate real bugs from intentional deviations to canonicalize.\n` +
  `7. Union the directions' coverage_gaps into coverage_gaps — an uncovered diff file is a review defect worth surfacing.\n\n` +
  `Consolidation output:\n` +
  `8. Emit one ranked fix plan over the confirmed findings: BLOCKER / WARNING / NIT. Each entry's fix carries over ` +
  `the lane's proposed fix — refine it against the current tree, never invent one from scratch.\n` +
  `9. Walk the Definition of Done (per the skill) and mark each criterion PASS/FAIL/N/A — DoD is not a direction.\n` +
  `10. Write the consolidated review as BOTH ${roundDir}/review.json (this structured output verbatim) and ` +
  `${roundDir}/review.md (findings table with a Direction column, coverage statement listing all reviewed files vs the diff, ` +
  `DoD table, recommendation — per the skill's Output Format). These are the station handoff artifacts.\n` +
  `11. Any commit messages you propose carry NO rule IDs and NO Claude attribution.\n\n` +
  `Review ONLY — no edits; your worktree stays pristine. Return the structured output only.`,
  { label: 'station', phase: 'Station merge', agentType: 'general-purpose', isolation: 'worktree', effort: 'high', schema: CONSOLIDATION_SCHEMA },
)

return {
  base: args.base, head: args.head, round: r, directions, lanes,
  failed: deadDirections,
  abortedLanes,
  consolidation,
  handoff: consolidation ? { json: consolidation.handoff_json, md: consolidation.handoff_md } : null,
  cleanup: await runCleanup(),
}
