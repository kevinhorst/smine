export const meta = {
  name: 'railroad-review',
  description: 'Fan a code review out to n lanes per railroad-review direction in isolated worktrees, dedup same-defect claims per direction, refute per candidate, then consolidate everything in one station review with JSON+MD handoff artifacts',
  whenToUse: 'Invoked by the /railroad-review skill in railroad mode, once per station-protocol round, with the confirmed diff base, head, directions, and lane count as args',
  phases: [
    { title: 'Fan-out', detail: 'n lanes per direction (× chunks when chunked), isolated worktrees at the review head tip, review-only' },
    { title: 'Dedup', detail: 'lightweight grouper per direction — text-only, no worktree: same-defect claims collapse to the best-argued survivor at max group severity' },
    { title: 'Refutation', detail: 'deduped claims at or above the refute threshold each get a fresh refuter — the claim\'s second confirmation' },
    { title: 'Station merge', detail: 'single barrier agent, the only semantic consolidator: union, dedup, verdict intake, dispositions, DoD, JSON+MD handoff' },
    { title: 'Cleanup', detail: 'remove this round\'s claude/railroad-* worktrees and branches via the agent-toolset remove_agent_worktrees.sh — safety-gated, never forced' },
  ],
}

// One station-protocol ROUND. The human gate, the rejection ledger file, the fix
// handoff, and the approved-twice loop live in the dispatcher (the session fronting this workflow) — a workflow cannot
// prompt or await a merge-back. The dispatcher passes the ledger CONTENT via
// args.rejectedFindings (lane worktrees fork from committed state; an uncommitted
// ledger file would be invisible to them).
if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object: {base, head, baseCommit, headCommit, commitsAhead, scope, riskMap, chunks, refute, artifactsDir, diffFiles, round, directions, lanes, rejectedFindings}')
}
for (const key of ['base', 'head', 'baseCommit', 'headCommit', 'artifactsDir']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — resolved by the dispatcher, never assumed`)
  }
}
if (!Number.isInteger(args.commitsAhead) || args.commitsAhead <= 0) {
  throw new Error('args.commitsAhead must be a positive integer — empty diff means stop and report nothing to review, not fan out')
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
if (directions.includes('special-focus') && (typeof args.focus !== 'string' || !args.focus)) {
  throw new Error('the special-focus direction requires args.focus — the explicit focus argument')
}
const lanes = Number.isInteger(args.lanes) && args.lanes > 0 ? args.lanes : 2

// refute: loglevel-style severity threshold (SKILL ROUND-007) — candidates at that
// severity AND ABOVE get a fresh refuter each; 'none' disables the stage (the station
// is the sole second confirmation). Default 'major'.
const SEVERITY_RANK = { BLOCKER: 5, MAJOR: 4, MINOR: 3, NIT: 2, INFO: 1 }
const REFUTE_THRESHOLDS = { blocker: 5, major: 4, minor: 3, nit: 2, info: 1 }
const refuteMode = typeof args.refute === 'string' && args.refute ? args.refute : 'major'
if (refuteMode !== 'none' && !(refuteMode in REFUTE_THRESHOLDS)) {
  throw new Error(`unknown refute threshold "${refuteMode}" — blocker | major | minor | nit | info | none ` +
    `(old values map: blk-maj -> major, all -> info, station -> none)`)
}
const refuteRank = refuteMode === 'none' ? Infinity : REFUTE_THRESHOLDS[refuteMode]

// chunks: experimental chunked fan-out (SKILL MODES-005) — [{name, files[]}] computed by
// the dispatcher; absent/empty = unchunked, lanes read the whole diff. contracts is chunk-exempt.
const chunks = Array.isArray(args.chunks) && args.chunks.length ? args.chunks : null
if (chunks) {
  for (const c of chunks) {
    if (!c || typeof c.name !== 'string' || !c.name || !Array.isArray(c.files) || !c.files.length) {
      throw new Error('each chunk must be {name, files[]} with a non-empty file list — partitioned by the dispatcher')
    }
  }
}
const chunkSlug = name => name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
// chunk list for one direction: contracts always reviews the full diff (cross-boundary)
const chunkListFor = d => (chunks && d !== 'contracts') ? chunks : [null]
const laneBranchName = (d, chunk, i) =>
  chunk ? `claude/railroad-${d}-${chunkSlug(chunk.name)}-l${i}-r${r}-${hash6}`
        : `claude/railroad-${d}-l${i}-r${r}-${hash6}`

const rejected = Array.isArray(args.rejectedFindings) ? args.rejectedFindings : []
const contextRefs = typeof args.contextRefs === 'string' ? args.contextRefs : ''
// scope: 'branch' (committed range) | 'wip' | 'all' (snapshot scopes — headCommit is a
// snapshot commit descending from the checkout HEAD; lanes check it out, SCOPE-002/ROUND-002)
const scope = typeof args.scope === 'string' && args.scope ? args.scope : 'branch'
const riskMap = args.riskMap && typeof args.riskMap === 'object' ? args.riskMap : null
const riskBlock = riskMap
  ? `\nRISK MAP — the dispatcher tiered every changed file (high/medium/low). Spend your attention on high and medium; ` +
    `read low-tier files only for accidental scope creep:\n${JSON.stringify(riskMap, null, 2)}\n`
  : ''

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

const refuterBranches = [] // filled at refuter spawn time; runCleanup reads it afterwards

async function runCleanup() {
  phase('Cleanup')
  const expectedBranches = [
    ...directions.flatMap(d => chunkListFor(d).flatMap(c => Array.from({ length: lanes }, (_, i) => laneBranchName(d, c, i + 1)))),
    ...refuterBranches,
    `claude/railroad-station-r${r}-${hash6}`,
  ].flatMap(b => [b, `${b}-2`])
  return agent(
    `You are the cleanup agent of a railroad-review round on ${args.base}...${args.head}, running in the dispatcher's checkout — ` +
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
    `Never touch branches outside the expected list, the dispatcher's checkout, or its branch. Return the structured output only.`,
    { label: 'cleanup', phase: 'Cleanup', agentType: 'general-purpose', effort: 'low', schema: CLEANUP_SCHEMA },
  )
}

const FINDING_PROPS = {
  finding_id: { type: 'string', description: 'stable id, unique across directions: <direction>-<SEV>-<n> with SEV in BLK/MAJ/MIN/NIT/INF, e.g. critical-BLK-1' },
  file: { type: 'string' },
  line: { type: 'integer' },
  severity: { type: 'string', description: 'BLOCKER | MAJOR | MINOR | NIT | INFO — assigned by consequence, not by how bad it looks' },
  claim: { type: 'string', description: 'the defect and why it violates the direction standard' },
  fix: { type: 'string', description: 'concrete proposed fix — one sentence or a short diff sketch' },
  evidence: { type: 'string', description: "the lane's confirmation pass — what was re-read and why the claim holds; the claim's first confirmation (in question until the station re-verifies)" },
  route: { type: 'string', description: 'station-assigned fix route on ranked_plan entries only: human (needs author judgment) | auto-fix (dispatcher inline) | runner-fix (delegate-runner batch)' },
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
      items: { type: 'object', required: ['finding_id', 'file', 'line', 'severity', 'claim', 'fix', 'evidence'], properties: FINDING_PROPS },
    },
    notes: { type: 'string', description: 'missing context files, excluded scope, anomalies' },
  },
}

function lanePrompt(direction, i, chunk) {
  const branch = laneBranchName(direction, chunk, i)
  const laneFiles = chunk ? chunk.files : args.diffFiles
  const fileTag = chunk ? `${direction}-${chunkSlug(chunk.name)}-lane${i}` : `${direction}-lane${i}`
  return (
    `You are lane ${i} of ${lanes} on the "${direction}" direction of a railroad-review fan-out on ${args.base}...${args.head} (round ${r})` +
    (chunk ? `, scoped to chunk "${chunk.name}" of a chunked review` : '') + `. ` +
    `You run in your own isolated git worktree at the review head tip. Other lanes review the same direction independently — ` +
    `never coordinate with them or read their artifacts.\n\n` +
    `FIRST ACTION: run \`git rev-parse HEAD\` in your worktree. If HEAD does not equal ${args.headCommit}: check whether ` +
    `${args.headCommit} is a DESCENDANT of HEAD (\`git merge-base --is-ancestor HEAD ${args.headCommit}\` — the snapshot-scope ` +
    `case; the snapshot commit is reachable through the shared object store) and if so run ` +
    `\`git checkout -B ${branch} ${args.headCommit}\` — the ONE sanctioned checkout, ` +
    `which also satisfies the SECOND ACTION — and continue; otherwise return aborted=true with the exact commit facts in notes ` +
    `(artifact paths empty, no artifacts written) and stop — do not review from a different premise. ` +
    `Then verify \`git merge-base --is-ancestor ${args.baseCommit} HEAD\`; a failed ancestor check also aborts. Otherwise aborted=false.\n\n` +
    `SECOND ACTION (skip if the snapshot checkout above already put you on the branch): rename your worktree branch so any ` +
    `leftover lands in the claude/* namespace: ` +
    `\`git branch -m ${branch}\` — if the name is taken, append "-2". ` +
    `Rename only the branch; never move the worktree directory, it is harness-managed.\n\n` +
    `Invoke the Skill tool with skill="railroad-review", then follow the "${direction}" direction definition in its ` +
    `SKILL.md exactly — read only the context that direction needs, walk only that direction's checklist. ` +
    `Diff with \`git diff ${args.baseCommit} HEAD\`; read each changed file in full, not just the hunks.\n` +
    (chunk
      ? `CHUNK SCOPE: you review ONLY the files of chunk "${chunk.name}" listed below — sibling lanes cover the other ` +
        `chunks. Flag any suspected cross-chunk issue (a contract or invariant you cannot verify inside your chunk) in ` +
        `your notes for the station; do not chase it into other chunks.\n`
      : '') +
    `The changed files are:\n${laneFiles.join('\n')}\n` +
    `SELF-VERIFICATION: before returning, confirm you reviewed every file above; files_reviewed in your return lists ` +
    `exactly the files you actually read in full. Never pad it — an honest gap beats a false claim.\n` +
    `FALSIFIABILITY${direction === 'code-style' ? ' (relaxed for code-style: your ground truth is the style guide)' : ''}: ` +
    `every claim must name the concrete input, state, or sequence that produces the wrong behaviour. Reject at the source: ` +
    `naming opinions, architecture preference, and unquantified robustness ("could be more robust") never become claims.\n` +
    `CONFIRMATION PASS: your findings are CLAIMS. Before returning, re-verify each claim you drafted — re-read its cited ` +
    `code in full context; drop any claim that does not survive its own re-check. Record in the claim's "evidence" field ` +
    `what you re-read and why the claim holds. This is the claim's FIRST confirmation — it stays in question until the ` +
    `refutation stage or the station merge independently re-verifies it.\n` +
    (direction === 'special-focus' ? `Focus argument: ${args.focus}\n` : '') +
    (contextRefs ? `Additional context: ${contextRefs}\n` : '') +
    riskBlock +
    ledgerBlock +
    `\nHard constraints, violation disqualifies this lane:\n` +
    `1. Review ONLY — make no edits to any file. Fixes are a separate step applied after the human gate, never during lane review.\n` +
    `2. Never read sibling worktrees or other agents' artifacts — your own artifact files are your only writes outside your worktree.\n` +
    `3. Do NOT enter plan mode and do NOT call ExitPlanMode. Write your findings as BOTH ` +
    `${roundDir}/${fileTag}.json (the structured findings) and ` +
    `${roundDir}/${fileTag}.md (human-readable, findings table per the skill's Output Format) — ` +
    `OUTSIDE your worktree — add no files of your own to it — then TERMINATE, never park on an approval prompt. ` +
    `The projection hook may modify files as you read them; that dirt is expected and harmless (the worktree is disposable) — leave it, never tidy it.\n` +
    `4. No tree-mutating git commands — checkout, reset, clean, stash, revert, cherry-pick, merge, rebase are all forbidden. ` +
    `The only git writes you make are the FIRST ACTION (HEAD/ancestor check, plus the one sanctioned snapshot checkout when ` +
    `${args.headCommit} descends from HEAD) and the branch rename (SECOND ACTION).\n\n` +
    `Return the structured output: direction, lane index, both artifact paths, files_reviewed, and one entry per finding ` +
    `(finding_id as "${direction}-<SEV>-<n>" with SEV in BLK/MAJ/MIN/NIT/INF, file, line, severity as ` +
    `BLOCKER/MAJOR/MINOR/NIT/INFO assigned by CONSEQUENCE — BLOCKER: wrong behaviour on a production path, data loss, ` +
    `security, irreversible migration defect; MAJOR: wrong behaviour on a secondary path, missing requirement, stated edge ` +
    `case untested; MINOR/NIT/INFO below — the claim, its evidence from ` +
    `the confirmation pass, and a concrete fix — one sentence or a short diff sketch, never empty), plus the aborted flag. ` +
    `Empty findings array with aborted=false ` +
    `is a valid, expected result — a clean review; aborted=true is a failed premise, never a clean review.`
  )
}

// Semantic within-direction dedup (SKILL ROUND-006) — a lightweight grouper agent judges
// which claims assert the same defect (text-only, no worktree, no code access); the script
// assembles the result deterministically: best-argued survivor, max group severity.
const GROUPER_SCHEMA = {
  type: 'object',
  required: ['groups'],
  properties: {
    groups: {
      type: 'array',
      description: 'every input claim appears in exactly one group; singletons included',
      items: {
        type: 'object',
        required: ['survivor_id', 'member_ids'],
        properties: {
          survivor_id: { type: 'string', description: 'the best-argued claim of the group — most concrete trigger, strongest evidence; never picked by text length' },
          member_ids: { type: 'array', items: { type: 'string' }, description: 'all finding_ids in the group, survivor included' },
        },
      },
    },
  },
}

function grouperPrompt(direction, claims) {
  return (
    `You are the dedup grouper for the "${direction}" direction of a railroad-review (round ${r}). ` +
    `Below are ${claims.length} once-confirmed claims from independent lanes that reviewed the same diff. ` +
    `Group claims that assert THE SAME defect — same root cause — even when they cite different lines or use different ` +
    `wording. Different defects that happen to sit close together stay separate. When in doubt, keep claims SEPARATE: ` +
    `a wrong merge silently drops a finding, a missed merge only costs one duplicate check downstream.\n\n` +
    `${JSON.stringify(claims, null, 2)}\n\n` +
    `Return every claim in exactly one group (singletons included). Pick each group's survivor by argument quality — ` +
    `most concrete trigger, strongest evidence — never by text length. This is a text-only judgment; you have no repo ` +
    `access and need none. Return the structured output only.`
  )
}

async function dedupeDirection(claims, d) {
  if (claims.length <= 1) return claims
  const res = await agent(grouperPrompt(d, claims), {
    label: `dedup-${d}`, phase: 'Dedup', agentType: 'general-purpose', effort: 'low', schema: GROUPER_SCHEMA,
  })
  if (!res || !Array.isArray(res.groups)) return claims // grouper lost: refute duplicates rather than lose claims
  const byId = new Map(claims.map(c => [c.finding_id, c]))
  const placed = new Set()
  const out = []
  for (const g of res.groups) {
    const members = (g.member_ids || []).filter(id => byId.has(id) && !placed.has(id))
    if (!members.length) continue
    const survivor = byId.get(g.survivor_id) && !placed.has(g.survivor_id) ? byId.get(g.survivor_id) : byId.get(members[0])
    members.forEach(id => placed.add(id))
    placed.add(survivor.finding_id)
    const sev = members.map(id => byId.get(id).severity)
      .reduce((a, b) => (SEVERITY_RANK[b] || 0) > (SEVERITY_RANK[a] || 0) ? b : a, survivor.severity)
    const mergedFrom = members.filter(id => id !== survivor.finding_id)
    out.push({ ...survivor, ...(sev !== survivor.severity ? { severity: sev } : {}), ...(mergedFrom.length ? { merged_from: mergedFrom } : {}) })
  }
  // any claim the grouper failed to place stays in — never silently dropped
  for (const c of claims) if (!placed.has(c.finding_id)) out.push(c)
  return out
}

// Refutation (SKILL ROUND-007): fresh agent per candidate in the refute set, briefed to
// refute — the claim's second confirmation, performed before the station.
const REFUTER_SCHEMA = {
  type: 'object',
  required: ['finding_id', 'verdict', 'reason', 'evidence', 'artifacts'],
  properties: {
    finding_id: { type: 'string' },
    verdict: { type: 'string', description: 'confirmed (survived refutation, artifact in hand) | debunked (refuted) | unverified (survives but not demonstrable here)' },
    reason: { type: 'string', description: 'debunked: what refuted it; unverified: the concrete missing precondition, never "not demonstrated"; confirmed: one line on the decisive artifact' },
    probe: { type: 'string', description: 'ACDSL repos: the probe rule id that decided the verdict, or "none: <why unprobeable after 2 attempts>"; "manual: <kind>" otherwise' },
    fixture: { type: 'string', description: 'the minimal fail fixture the probe went RED on in the validity check, path under the round dir — absent on the manual path' },
    evidence: { type: 'string', description: 'what was run/read and what it showed — the decisive lines inline' },
    artifacts: { type: 'array', items: { type: 'string' }, description: 'absolute paths of the artifacts written under the round dir, named for the finding' },
  },
}

const findingSlug = id => id.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')

function refuterPrompt(direction, finding, branch) {
  return (
    `You are a REFUTER in a railroad-review on ${args.base}...${args.head} (round ${r}). You did not produce the candidate ` +
    `below and know nothing about how the code came to be. Your brief: REFUTE it. "Could not reproduce" is a successful ` +
    `result; never manufacture evidence. You run in your own isolated git worktree.\n\n` +
    `FIRST ACTION: if \`git rev-parse HEAD\` is not ${args.headCommit} and ${args.headCommit} descends from HEAD ` +
    `(snapshot scope), run \`git checkout -B ${branch} ${args.headCommit}\`; otherwise rename your worktree branch: ` +
    `\`git branch -m ${branch}\` — if the name is taken, append "-2". Rename only the branch; never move the worktree directory.\n\n` +
    `THE CANDIDATE (direction "${direction}", once-confirmed by its lane, still in question):\n` +
    `${JSON.stringify(finding, null, 2)}\n\n` +
    `Perform the claim's SECOND CONFIRMATION per the railroad-review skill's Probe protocol (PROBE entries):\n` +
    `- ACDSL repos (acdsl/registry.json or bin/acdsl exists): author a debug probe — one rule line in the untracked file ` +
    `railroad-probes-r${r}.acdsl at your worktree root (\`//acdsl:<PROBE-ID> <script> anchor="^<defect file, regex-escaped>$" ` +
    `lifetime="task" why="<the claim>"\`), an executable under ${roundDir}/probes/ (exit 0 clean, exit 1 + "file:line: message" ` +
    `on hit), an entry in a merged registry at ${roundDir}/probes/registry-${findingSlug(finding.finding_id)}.json. ` +
    `VALIDITY FIRST: the probe must exit 1 on a minimal fail fixture under ${roundDir}/probes/fixtures/ before the real run — ` +
    `run it against a hand-built files list written under the system temp dir, NEVER under ${roundDir}/probes/ (transient ` +
    `scaffolding, not a review artifact); report the fixture's path in the fixture field. ` +
    `Two attempts before declaring the claim unprobeable. RED on the claimed file -> verdict "confirmed"; GREEN -> "debunked".\n` +
    `- Otherwise (or unprobeable): the cheapest settling artifact, in preference order — a failing test, a command with ` +
    `actual and expected output, a reproduction sequence with the observed result. A test demonstrates the claim only if it ` +
    `fails on the current tree FOR THE REASON THE CLAIM STATES and passes with the proposed fix — check both directions and ` +
    `say you did. For that both-directions check you may apply the proposed fix in your disposable worktree, run the test, ` +
    `and restore the file content exactly afterwards — the fix never persists and is never committed.\n` +
    `- Survives refutation but not demonstrable in this environment -> verdict "unverified" with the CONCRETE missing ` +
    `precondition ("needs a device with an expired subscription"), never a bare "not demonstrated".\n\n` +
    `Write every artifact under ${roundDir}/, named for the finding (${findingSlug(finding.finding_id)}-*). ` +
    `Review ONLY otherwise — no persistent edits, no tree-mutating git commands (reset/clean/stash/revert; checkout only as ` +
    `the sanctioned snapshot checkout in FIRST ACTION); projection-hook dirt is expected, leave it. Do NOT enter plan mode; ` +
    `TERMINATE after returning the structured output.`
  )
}

async function refuteDirection(report, d) {
  if (!report) return null
  if (refuteMode === 'none') return { ...report, refuter_verdicts: [] }
  const targets = (report.claims || []).filter(f => (SEVERITY_RANK[f.severity] || 0) >= refuteRank)
  if (!targets.length) return { ...report, refuter_verdicts: [] }
  log(`direction ${d}: refuting ${targets.length} candidate(s) at threshold "${refuteMode}"`)
  const verdicts = (await parallel(targets.map(f => {
    const branch = `claude/railroad-refute-${findingSlug(f.finding_id)}-r${r}-${hash6}`
    refuterBranches.push(branch)
    return () => agent(refuterPrompt(d, f, branch), {
      label: `refute-${f.finding_id}`, phase: 'Refutation', agentType: 'general-purpose',
      isolation: 'worktree', schema: REFUTER_SCHEMA,
    })
  }))).filter(Boolean)
  return { ...report, refuter_verdicts: verdicts }
}

phase('Fan-out')
const abortedLanes = []
const directionResults = await pipeline(
  directions,
  d => {
    const laneDefs = chunkListFor(d).flatMap(c => Array.from({ length: lanes }, (_, i) => ({ chunk: c, i: i + 1 })))
    return parallel(laneDefs.map(def => () =>
      agent(lanePrompt(d, def.i, def.chunk), {
        label: def.chunk ? `${d}-${chunkSlug(def.chunk.name)}-l${def.i}` : `${d}-l${def.i}`,
        phase: 'Fan-out', agentType: 'general-purpose',
        isolation: 'worktree', schema: LANE_SCHEMA, ...laneOpts,
      })
    )).then(results => ({ laneDefs, results }))
  },
  async (fanout, d) => {
    const { laneDefs, results } = fanout
    const zipped = results.map((l, idx) => l && { ...l, assigned: laneDefs[idx].chunk ? laneDefs[idx].chunk.files : args.diffFiles, chunkName: laneDefs[idx].chunk ? laneDefs[idx].chunk.name : null }).filter(Boolean)
    for (const l of zipped.filter(l => l.aborted)) {
      abortedLanes.push({ direction: d, lane: l.lane, chunk: l.chunkName, notes: l.notes || '' })
    }
    const finished = zipped.filter(l => !l.aborted)
    if (zipped.length > finished.length) {
      log(`direction ${d}: ${zipped.length - finished.length}/${zipped.length} lanes aborted on premise`)
    }
    if (!finished.length) return null
    const coverage = args.diffFiles.filter(f => !finished.some(l => l.files_reviewed.includes(f)))
    const perLane = finished.map(l => ({ lane: l.lane, chunk: l.chunkName, missing: l.assigned.filter(f => !l.files_reviewed.includes(f)) }))
    const produced = finished.flatMap(l => l.findings || [])
    const claims = await dedupeDirection(produced, d)
    if (produced.length > claims.length) log(`direction ${d}: semantic dedup ${produced.length} -> ${claims.length} claims`)
    // no heavyweight merge agent (SKILL ROUND-006/009): grouping is text-only here,
    // cross-direction dedup belongs to the station
    return {
      direction: d, claims, claims_produced: produced.length, coverage_gaps: coverage, per_lane: perLane,
      lane_artifacts: finished.map(l => l.artifact_json),
      notes: finished.map(l => l.notes).filter(Boolean).join(' | '),
    }
  },
  (report, d) => refuteDirection(report, d),
)

const deadDirections = directions.filter((_, i) => !directionResults[i])
const finishedDirections = directionResults.filter(Boolean)
if (deadDirections.length) log(`dead directions (all lanes skipped, errored, or aborted): ${deadDirections.join(', ')}`)
if (!finishedDirections.length) {
  return { base: args.base, head: args.head, scope, round: r, directions, lanes, refute: refuteMode, chunked: !!chunks, failed: deadDirections, abortedLanes, refutations: [], consolidation: null, handoff: null, cleanup: await runCleanup() }
}

// Station merge (barrier): the only semantic consolidator — union all direction claim
// sets, disposition everything, take refuter verdicts in, verify the rest against the
// CURRENT tree (fixes land out of band mid-review), and write the round's JSON+MD
// handoff artifacts. Like every other agent, it renames its
// worktree branch into the claude/* namespace so any leftover is visible to tooling.
const CONSOLIDATION_SCHEMA = {
  type: 'object',
  required: ['dispositions', 'ranked_plan', 'unverified', 'funnel', 'permanent_checks', 'build_test', 'debunked', 'resolved', 'intentional_deviations', 'coverage_gaps', 'definition_of_done', 'recommendation', 'handoff_json', 'handoff_md', 'probes', 'probes_dir'],
  properties: {
    dispositions: {
      type: 'array',
      description: 'every union\'d finding, each with exactly one disposition — nothing may lack one',
      items: {
        type: 'object',
        required: ['finding_id', 'disposition'],
        properties: {
          finding_id: { type: 'string' },
          disposition: { type: 'string', description: 'confirmed (second confirmation passed) | unverified (survived refutation but not demonstrable here — kept, flagged) | rejected-with-reason | duplicate-of | debunked (refuted on re-verification)' },
          reason: { type: 'string', description: 'required for rejected-with-reason, debunked, and unverified (the concrete missing precondition)' },
          duplicate_of: { type: 'string', description: 'the surviving finding_id, required for duplicate-of' },
          probe: { type: 'string', description: 'ACDSL repos: the probe rule id that decided this disposition, or "none: <why unprobeable after 2 attempts>" / "manual: <why>" — absent only in non-ACDSL repos' },
        },
      },
    },
    ranked_plan: {
      type: 'array',
      description: 'the confirmed findings as one ranked fix plan, each with a route',
      items: { type: 'object', required: ['finding_id', 'severity', 'file', 'line', 'fix', 'route'], properties: FINDING_PROPS },
    },
    unverified: {
      type: 'array',
      description: 'findings kept but not demonstrable in this environment — each with the concrete missing precondition, never a bare "not demonstrated"',
      items: { type: 'object', required: ['finding_id', 'reason'], properties: { finding_id: { type: 'string' }, reason: { type: 'string' } } },
    },
    funnel: {
      type: 'object',
      description: 'the claim funnel across the round — rendered as one line at the end of review.md',
      required: ['produced', 'confirmed', 'unverified', 'duplicates', 'rejected', 'debunked'],
      properties: {
        produced: { type: 'integer' }, confirmed: { type: 'integer' }, unverified: { type: 'integer' },
        duplicates: { type: 'integer' }, rejected: { type: 'integer' }, debunked: { type: 'integer' },
      },
    },
    permanent_checks: {
      type: 'array',
      description: 'for each confirmed finding that is a recurring class rather than a one-off: the durable check that would catch the class (ACDSL rule, lint rule, assertion, golden file, test) — the front offers to add them, never adds unasked; empty when every finding is a one-off',
      items: { type: 'object', required: ['finding_id', 'check'], properties: { finding_id: { type: 'string' }, check: { type: 'string' } } },
    },
    build_test: {
      type: 'object',
      description: 'the project build and test suite, each run once in the station worktree — result or "skipped: <why>", never unmentioned',
      required: ['build', 'tests'],
      properties: { build: { type: 'string' }, tests: { type: 'string' } },
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
    probes: {
      type: 'array',
      description: 'one entry per authored probe (station and refuter probes alike) — THE probe→fixture→verdict index; there is no sidecar verdicts file. Empty in non-ACDSL repos or when no probes were authored',
      items: {
        type: 'object',
        required: ['id', 'script', 'fixture', 'verdict', 'finding_id'],
        properties: {
          id: { type: 'string', description: 'the probe rule id (e.g. PROBE-R4-03)' },
          script: { type: 'string', description: 'the probe executable, path relative to probes_dir' },
          fixture: { type: 'string', description: 'the minimal fail fixture the probe went RED on in the validity check, path relative to probes_dir' },
          verdict: { type: 'string', description: 'the real-tree result: "RED on <file:line>" | "GREEN"' },
          finding_id: { type: 'string', description: 'the finding this probe settled' },
        },
      },
    },
    probes_dir: { type: 'string', description: 'absolute path of round-{r}/probes/ with the probe rules (probes.acdsl), scripts, fixtures, and registry — empty string when the repo has no ACDSL or no probes were authored' },
  },
}

phase('Station merge')
const consolidation = await agent(
  `You are the station-merge barrier of a railroad-review on ${args.base}...${args.head} (round ${r}). ` +
  `You run in your own isolated git worktree, forked from the dispatcher's checkout after the fan-out — ` +
  `for re-verification purposes it IS the current tree.\n\n` +
  `FIRST ACTION: if \`git rev-parse HEAD\` is not ${args.headCommit} and ${args.headCommit} descends from HEAD ` +
  `(snapshot scope), run \`git checkout -B claude/railroad-station-r${r}-${hash6} ${args.headCommit}\` — ` +
  `re-verification must run against the reviewed snapshot tree. Otherwise rename your worktree branch: ` +
  `\`git branch -m claude/railroad-station-r${r}-${hash6}\` — ` +
  `if the name is taken, append "-2". Rename only the branch; never move the worktree directory.\n\n` +
  `${finishedDirections.length} direction claim sets follow — the lanes' once-confirmed claims, already deduped WITHIN ` +
  `each direction by a grouper (same-defect groups collapsed to the best-argued survivor at max group severity, ` +
  `merged_from listing the absorbed ids), with claims_produced (pre-dedup count), coverage gaps, per-lane misses, lane ` +
  `artifact paths, and refuter verdicts where the refutation stage ran. Read the lane artifacts (lane_artifacts paths) ` +
  `when a claim needs its full write-up.\n\n${JSON.stringify(finishedDirections, null, 2)}\n\n` +
  ledgerBlock +
  `\nMerge (the trains arrive at the station) — cross-direction consolidation is yours alone:\n` +
  `1. Union ALL claims from ALL directions, then dedup ACROSS directions (within-direction dedup already happened): ` +
  `collapse cross-direction duplicates to the better-argued claim via duplicate-of, reconciling refuter-verdict pairs ` +
  `onto the survivor.\n` +
  `2. Give every finding exactly one disposition: confirmed | unverified | rejected-with-reason | duplicate-of(id) | debunked. Nothing may lack one.\n` +
  `3. Any finding matching a ledger entry above gets disposition "rejected-with-reason" citing the ledger — and note the lane that re-reported it.\n` +
  `4. Complete the SECOND confirmation for every claim — all severities, NITs included. Direction reports may carry ` +
  `refuter_verdicts: fresh per-candidate refuters already performed the second confirmation for those claims. Their ` +
  `verdicts are BINDING — map confirmed/debunked/unverified straight into the disposition, carrying the refuter's reason, ` +
  `probe, fixture, and artifacts — unless a verdict is internally inconsistent with the code in front of you; then re-verify ` +
  `yourself and say so in the disposition reason. Every claim WITHOUT a refuter verdict you re-verify yourself: each ` +
  `arrives once-confirmed by its lane (its evidence field) and is still in question. Be ADVERSARIAL: attempt ` +
  `to REFUTE each claim — "could not reproduce" is a successful result, and you never manufacture evidence. A claim that ` +
  `survives refutation but cannot be demonstrated in this environment is "unverified": keep it, flag it, and record the ` +
  `CONCRETE missing precondition ("needs a device with an expired subscription"), never a bare "not demonstrated". ` +
  `Re-verification must be deterministic where the repo supports it:\n` +
  `   ACDSL PROBE PROTOCOL (when acdsl/registry.json or bin/acdsl exists): per claim, author a debug probe — make at ` +
  `least TWO attempts before declaring a claim unprobeable. A probe is (a) one rule line in the untracked file ` +
  `railroad-probes-r${r}.acdsl at your worktree root: \`//acdsl:<PROBE-ID> <script-name> anchor="^<defect file, regex-escaped>$" ` +
  `lifetime="task" why="<the claim>"\` (file-scoped anchors are the norm here); (b) an executable under ` +
  `${roundDir}/probes/ that detects THIS defect — invoked as <argv> <files-list path> [key=value...], exit 0 = clean, ` +
  `exit 1 with one "file:line: message" line per hit = defect present; reuse a registry verifier with params when one ` +
  `fits instead of writing a script; (c) an entry in ${roundDir}/probes/registry.json — start from the repo's ` +
  `acdsl/registry.json, overlay its acdsl/registry.local.json if present, add your probe entries with ABSOLUTE argv ` +
  `paths and timeout_s <= 60.\n` +
  `   VALIDITY FIRST: write a minimal fail fixture under ${roundDir}/probes/fixtures/ reproducing the defect pattern and ` +
  `run the probe executable directly against a hand-built files list containing it — the probe MUST exit 1 there. The ` +
  `files list is transient scaffolding: write it under the system temp dir, NEVER under ${roundDir}/probes/ — it is not a ` +
  `review artifact. A probe that cannot detect its own fixture is a failed attempt; fix it or write a new one (that is ` +
  `the second attempt).\n` +
  `   VERDICT: run \`<acdsl binary> check -rule <PROBE-ID> -registry ${roundDir}/probes/registry.json\` from your worktree ` +
  `root (go run ./cmd/acdsl in this repo, bin/acdsl when vendored). RED on the claimed file = deterministically reproduced ` +
  `-> "confirmed", record the probe id in the disposition's probe field. GREEN (after the fixture proved the probe works) ` +
  `= refuted -> "debunked" with the probe id and reason. Already fixed by a specific commit -> resolved[] with the hash. ` +
  `Unprobeable after two attempts -> fall back to manual re-read of the current tree; set probe to "none: <why>".\n` +
  `   Persist under ${roundDir}/probes/ EXACTLY: the rule lines mirrored as probes.acdsl, the scripts, fixtures/, and ` +
  `registry.json — nothing else: no verdicts.md, no files-list sidecars. Each probe's fixture and tree verdict are ` +
  `recorded in the structured output's probes[] instead (refuter probes included — map their probe/fixture pairs in). ` +
  `Set probes_dir accordingly. NON-ACDSL repos: manual ` +
  `re-read of PRIMARY SOURCES for every claim, probe field "manual: no acdsl", probes_dir empty — settle each claim with ` +
  `the cheapest artifact that does it, in preference order: a failing test, a command with actual and expected output, a ` +
  `reproduction sequence with the observed result; a test demonstrates a claim only if it fails on the current tree FOR ` +
  `THE REASON THE CLAIM STATES and passes with the proposed fix — check both directions and say you did. Artifacts land ` +
  `under ${roundDir}/, named for the finding they settle.\n` +
  `5. Reconcile divergent severities across directions. Resolve spec-vs-code conflicts against decisions.md — a spec-explicit choice is not a code bug.\n` +
  `6. Separate real bugs from intentional deviations to canonicalize.\n` +
  `7. Union the directions' coverage_gaps into coverage_gaps — an uncovered diff file is a review defect worth surfacing.\n\n` +
  `Consolidation output:\n` +
  `8. Emit one ranked fix plan over the confirmed findings: BLOCKER / MAJOR / MINOR / NIT / INFO, severity by CONSEQUENCE ` +
  `(BLOCKER: production path, data loss, security, irreversible migration; MAJOR: secondary path, missing requirement, ` +
  `stated edge case untested). Each entry's fix carries over the lane's proposed fix — refine it against the current tree, ` +
  `never invent one from scratch. Assign each entry a route: human (needs author judgment) | auto-fix (mechanical, ` +
  `dispatcher inline) | runner-fix (well-specified but large — delegate-runner batch).\n` +
  `9. Fill permanent_checks: for each confirmed finding that is a RECURRING CLASS rather than a one-off, name the durable ` +
  `check that would catch the class (ACDSL rule, lint rule, assertion, golden file, test). One-offs get no entry.\n` +
  `10. Run the project's build and its test suite ONCE each in your worktree and record both results in build_test; ` +
  `too slow to run -> "skipped: <why>", never unmentioned.\n` +
  `11. Fill the funnel: produced = the directions' claims_produced summed; duplicates = grouper-merged (produced minus ` +
  `the claims you received) plus your own duplicate-of dispositions; confirmed / unverified / rejected / debunked from ` +
  `the dispositions.\n` +
  `12. Walk the Definition of Done (per the skill) and mark each criterion PASS/FAIL/N/A — DoD is not a direction.\n` +
  `13. Write the consolidated review as BOTH ${roundDir}/review.json (this structured output verbatim) and ` +
  `${roundDir}/review.md (findings table with Direction and Route columns, coverage statement listing all reviewed files ` +
  `vs the diff, DoD table, build/test results, a Probes section — one table row per probes[] entry: id, finding, script, ` +
  `fixture, tree verdict, so the reviewer navigates from the review doc to each probe and its fixture — recommendation, ` +
  `and the funnel as the LAST LINE — per the skill's Output ` +
  `Format). The findings table lists confirmed (twice-confirmed) claims, each citing its probe, plus unverified claims ` +
  `flagged "unverified" with their missing precondition; debunked and resolved claims appear in their own sections, never ` +
  `in the findings table. ESCALATE, DON'T BURY: a suspected defect in migrations, auth, concurrency, money, a public API ` +
  `contract, or persisted data that you could not demonstrate goes at the TOP of review.md, never as one unverified line ` +
  `among many. These are the station handoff artifacts.\n` +
  `14. Any commit messages you propose carry NO rule IDs and NO Claude attribution.\n\n` +
  `Review ONLY on tracked files — never edit an existing file, with ONE exception: for a both-directions test check you ` +
  `may apply the proposed fix in your disposable worktree, run the test, and restore the file content exactly afterwards — ` +
  `the fix never persists and is never committed. Your only other in-worktree write is the untracked ` +
  `railroad-probes-r${r}.acdsl (all other probe artifacts live under ${roundDir}/probes/). No tree-mutating git commands ` +
  `(reset/clean/stash/revert; checkout only as the sanctioned snapshot checkout in FIRST ACTION); projection-hook dirt is ` +
  `expected, leave it. Return the structured output only.`,
  { label: 'station', phase: 'Station merge', agentType: 'general-purpose', isolation: 'worktree', effort: 'high', schema: CONSOLIDATION_SCHEMA },
)

return {
  base: args.base, head: args.head, scope, round: r, directions, lanes,
  refute: refuteMode,
  chunked: !!chunks,
  failed: deadDirections,
  abortedLanes,
  refutations: finishedDirections.flatMap(fd => fd.refuter_verdicts || []),
  consolidation,
  handoff: consolidation ? { json: consolidation.handoff_json, md: consolidation.handoff_md } : null,
  cleanup: await runCleanup(),
}
