export const meta = {
  name: 'railroad-dedup',
  description: 'Dedup a railroad-review round\'s once-confirmed claims — severity recomputed from each claim\'s cells at entry, a text-only grouper per direction, then one grouper over the union of all directions; deterministic assembly (best-argued survivor, max group severity)',
  whenToUse: 'Invoked by the /railroad-review dispatcher at the Groupers spawn (SKILL ROUND-025), after the lanes stage returns',
  phases: [
    { title: 'Dedup', detail: 'severity recompute (ROUND-033), then per-direction groupers — text-only, no worktree: same-defect claims collapse to the best-argued survivor at max group severity' },
    { title: 'Cross-direction dedup', detail: 'one grouper over the union of all directions, before any refuter is spent' },
  ],
}

// GEN:schemas grouper,severity-table
const GROUPER_SCHEMA = {
  "type": "object",
  "required": [
    "groups"
  ],
  "properties": {
    "groups": {
      "type": "array",
      "description": "every input claim appears in exactly one group; singletons included",
      "items": {
        "type": "object",
        "required": [
          "survivor_id",
          "member_ids"
        ],
        "properties": {
          "survivor_id": {
            "type": "string",
            "description": "the best-argued claim of the group — most concrete trigger, strongest evidence; never picked by text length"
          },
          "member_ids": {
            "type": "array",
            "items": {
              "type": "string"
            },
            "description": "all finding_ids in the group, survivor included"
          }
        }
      }
    }
  }
}
const SEVERITY_TABLE = {
  "schemaVersion": "2.0",
  "axes": {
    "effect": [
      "destructive",
      "incorrect",
      "degraded",
      "debt",
      "cosmetic"
    ],
    "exposure": [
      "always",
      "often",
      "rare",
      "eventually"
    ]
  },
  "rows": [
    {
      "effect": [
        "destructive"
      ],
      "exposure": [
        "always",
        "often",
        "rare"
      ],
      "severity": "BLOCKER"
    },
    {
      "effect": [
        "destructive"
      ],
      "exposure": [
        "eventually"
      ],
      "severity": "MAJOR"
    },
    {
      "effect": [
        "incorrect"
      ],
      "exposure": [
        "always",
        "often"
      ],
      "severity": "BLOCKER"
    },
    {
      "effect": [
        "incorrect"
      ],
      "exposure": [
        "rare"
      ],
      "severity": "MAJOR"
    },
    {
      "effect": [
        "incorrect"
      ],
      "exposure": [
        "eventually"
      ],
      "severity": "MINOR"
    },
    {
      "effect": [
        "degraded"
      ],
      "exposure": [
        "always",
        "often"
      ],
      "severity": "MAJOR"
    },
    {
      "effect": [
        "degraded"
      ],
      "exposure": [
        "rare"
      ],
      "severity": "MINOR"
    },
    {
      "effect": [
        "degraded"
      ],
      "exposure": [
        "eventually"
      ],
      "severity": "NIT"
    },
    {
      "effect": [
        "debt"
      ],
      "exposure": [
        "always",
        "often"
      ],
      "severity": "MINOR"
    },
    {
      "effect": [
        "debt"
      ],
      "exposure": [
        "rare",
        "eventually"
      ],
      "severity": "NIT"
    },
    {
      "effect": [
        "cosmetic"
      ],
      "severity": "NIT"
    }
  ]
}
// GEN:end

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

// GEN:common
// Shared prelude of every railroad-review stage workflow — injected between the
// GEN:common markers by make workflow-gen (single source, no runtime imports).
// Holds: universal args validation, helper functions, shared prompt blocks, and the review's
// severity and id policy (SEVERITY_RANK, REFUTE_THRESHOLDS, severityOf, remintId) — the known
// exception to the workflow-layer rule, which keeps policy in the rules binary the skill's variables name.
// Schemas live in schemas/*.json; doctrine in the SKILL body.
if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — the dispatcher assembles each stage\'s args per the Stage files def')
}
const requireStr = keys => {
  for (const key of keys) {
    if (typeof args[key] !== 'string' || !args[key]) {
      throw new Error(`args.${key} is required — resolved by the dispatcher, never assumed`)
    }
  }
}
requireStr(['base', 'head', 'baseCommit', 'headCommit', 'artifactsDir', 'runNs'])
// The specs travel as the dispatcher's copies under the artifacts dir, which every worker may
// read — never as paths inside the dispatcher's checkout.
if (args.specPaths !== undefined && (!Array.isArray(args.specPaths) || args.specPaths.some(p => typeof p !== 'string' || !p))) {
  throw new Error('args.specPaths must be an array of the round-dir spec copies — empty when no spec is given')
}
const specPaths = Array.isArray(args.specPaths) ? args.specPaths : []
const outsideSpecs = specPaths.filter(p => !p.startsWith(`${args.artifactsDir}/`))
if (outsideSpecs.length) {
  throw new Error(`args.specPaths must be the dispatcher's copies under ${args.artifactsDir}/ — never paths inside the dispatcher's checkout: ${outsideSpecs.join(', ')}`)
}
// Run namespace: isolates this run's branch prefix from any concurrent
// railroad-review run sharing the same repository.
if (!/^[a-z0-9][a-z0-9-]{2,24}$/.test(args.runNs)) {
  throw new Error('args.runNs must match ^[a-z0-9][a-z0-9-]{2,24}$ — the dispatcher mints it once per run (`rr-<run-id>` when run-id is given, else `rr-<6 hex>`)')
}
if (!Number.isInteger(args.round) || args.round <= 0) {
  throw new Error('args.round is required — the station-protocol round number (1-based); it namespaces artifacts and branch names')
}

const SEVERITY_RANK = { BLOCKER: 5, MAJOR: 4, MINOR: 3, NIT: 2, INFO: 1 }
// D1: the severities that never block a round and never make it CONDITIONAL.
const NON_BLOCKING = ['NIT', 'INFO']
const REFUTE_THRESHOLDS = { blocker: 5, major: 4, minor: 3, nit: 2, info: 1 }
const levelRank = (name, fallback, zeroOn) => {
  const mode = typeof args[name] === 'string' && args[name] ? args[name] : fallback
  if (mode !== 'none' && !(mode in REFUTE_THRESHOLDS)) {
    throw new Error(`unknown ${name} "${mode}" — blocker | major | minor | nit | info | none`)
  }
  return { mode, rank: mode === 'none' ? zeroOn : REFUTE_THRESHOLDS[mode] }
}
// Severity is a function of the finding's cells (SKILL OUT-005/OUT-005b): the table row
// for effect/exposure, INFO exactly when the fix is "none". SEVERITY_TABLE is
// GEN-injected from schemas/severity-table.json in the stages that call these.
const severityOf = f => {
  if (f.fix === 'none') return 'INFO'
  const hit = SEVERITY_TABLE.rows.filter(row =>
    (!row.effect || row.effect.includes(f.effect)) &&
    (!row.exposure || row.exposure.includes(f.exposure)))
  if (hit.length !== 1) throw new Error(`severityOf: vector ${f.effect}/${f.exposure} matches ${hit.length} rows — the severity-table gate is red`)
  return hit[0].severity
}
const SEV_TOKEN = { BLOCKER: 'BLK', MAJOR: 'MAJ', MINOR: 'MIN', NIT: 'NIT', INFO: 'INF' }
const remintId = (id, severity) => id.replace(/-(BLK|MAJ|MIN|NIT|INF)-(\d+)/, `-${SEV_TOKEN[severity]}-$2`)
const renderTable = () => SEVERITY_TABLE.rows.map(row =>
  `${(row.effect || ['any']).join('|')} / ${(row.exposure || ['any']).join('|')} -> ${row.severity}`).join('; ')

const chunkSlug = name => name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
const findingSlug = id => id.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')

const hash6 = args.baseCommit.slice(0, 6)
const r = args.round
const roundDir = `${args.artifactsDir}/round-${r}`
// NIGHTLY-007: every branch this round creates carries the run namespace.
const branchNs = `claude-review/${args.runNs}-`

// assertModel / DEFAULT_SUBAGENT_MODEL come from the file's GEN:models block (MODES-017).
const laneOpts = { model: assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL) }
if (typeof args.laneEffort === 'string' && args.laneEffort) laneOpts.effort = args.laneEffort

// worktreeMode 'explicit' (explicit-worktree route): workers provision their own
// worktrees of args.targetRepo instead of harness session-repo forks.
const explicitWt = args.worktreeMode === 'explicit'
if (explicitWt) {
  requireStr(['targetRepo', 'worktreeRoot'])
}
const overlayPatch = explicitWt && typeof args.overlayPatch === 'string' ? args.overlayPatch : ''
// Explicit route: repo-relative paths the provisioner links from the target repo — built once
// before the run, so no worker builds.
const prebuilt = explicitWt && Array.isArray(args.prebuilt) ? args.prebuilt.filter(p => typeof p === 'string' && p) : []
// The review diff, minus the invocation's excluded pathspecs.
const excludes = Array.isArray(args.excludes) ? args.excludes.filter(p => typeof p === 'string' && p) : []
const diffCmd = `git diff ${args.baseCommit} ${args.headCommit}` +
  (excludes.length ? ` -- . ${excludes.map(p => `':(exclude)${p}'`).join(' ')}` : '')
const wtPathFor = slug => `${args.worktreeRoot}/${slug}`
const isoOpts = explicitWt ? {} : { isolation: 'worktree' }
// Explicit route: every run of a worker — first, continuation or retry — provisions its
// worktree with the same command; provision answers an existing worktree of the same slug,
// branch and head with exit 0.
const wtLine = (slug, branch) => explicitWt
  ? `\nYOUR WORKTREE: ${wtPathFor(slug)} — the pin action below opens by creating it with this provision command, exactly: ` +
    `\`$SKILL_RAILROADREVIEW_PROVISION provision --repo ${args.targetRepo} --root ${args.worktreeRoot} ` +
    `--head ${args.headCommit} --branch ${branch}${overlayPatch ? ` --overlay ${overlayPatch}` : ''}` +
    `${prebuilt.length ? ` --prebuilt ${prebuilt.join(',')}` : ''} ${slug}\` — a non-zero exit ` +
    `means stop and return its printed error (lanes: aborted with it in notes; second confirmation: verdict unverified with it ` +
    `as reason; station: pin_error). ` +
    `Do ALL repository work there: every git command as \`git -C ${wtPathFor(slug)} ...\`; never touch the checkout at ${args.targetRepo} itself.\n`
  : ''

// Worker branch step: harness forks rename into claude-review/*; explicit worktrees run the provision command.
const branchStep = name => explicitWt
  ? `run the YOUR WORKTREE provision command above, exactly — it creates your worktree on branch ${name}`
  : `rename your worktree branch so any leftover lands in the claude-review/* namespace: ` +
    `\`git branch -m ${name}\`. ` +
    `Rename only the branch; never move the worktree directory`

// A re-spawned worker on the harness route gets its own branch: the previous attempt's
// worktree still holds the first name. Explicit worktrees keep theirs — provision reuses them.
const attemptBranch = (name, attempt) => !explicitWt && attempt > 1 ? `${name}-a${attempt}` : name

// Worker pin step, shared by lanes, refuters, verifiers and the station: a harness worktree
// forks from the dispatcher branch's tip at spawn time — ahead of, behind or diverged from
// headCommit. The rename comes first, so the fork branch never outlives the worker; a
// differing HEAD then resets that same branch to the pinned head.
const pinStep = (branch, onFail) =>
  `first ${branchStep(branch)}. Then run \`git rev-parse HEAD\` in your worktree; if it is not ${args.headCommit}, run ` +
  `\`git checkout -B ${branch} ${args.headCommit}\` — it resets the branch you are on to the pinned head, whether HEAD ` +
  `is ahead of, behind or diverged from it. Then run \`git merge-base --is-ancestor ${args.baseCommit} HEAD\`. ` +
  `A failed rename, checkout or ancestor check means: ${onFail}`

// STATION-004b/c, MODES-016a: from round 2 both ledgers travel, so no round re-reports a carried finding.
const ledgerPath = typeof args.ledgerPath === 'string' ? args.ledgerPath : ''
const knownPath = typeof args.knownPath === 'string' ? args.knownPath : ''
if (r > 1 && (!ledgerPath || !knownPath)) {
  throw new Error('args.ledgerPath and args.knownPath are required from round 2 — the rejected and known ledgers')
}
const ledgerBlock = ledgerPath
  ? `\nREJECTED-FINDINGS LEDGER — findings a human reviewed and rejected, with reasons; read it from ${ledgerPath} (JSON) ` +
    `before finalizing claims. Do NOT re-report a ledger entry or a trivial variant of one; re-reporting is itself a defect.\n`
  : ''
// STATION-004c: every finding confirmed in a prior round — carried by the dispatcher,
// suppressed at the source so workers never spend time re-deriving them.
const knownBlock = knownPath
  ? `\nKNOWN-FINDINGS LEDGER — findings already CONFIRMED in a prior round; rows in fix_state "open" or "ignored" are still in the code; read it from ` +
    `${knownPath} (JSON) before finalizing claims. Do NOT re-report, re-derive, or re-verify a known entry or a trivial ` +
    `variant of one (match by file and claim substance, never by id) — they are carried by the dispatcher.\n`
  : ''
// Worker shell conduct: the gate denies chains and cd; the granted -C forms address the worktree.
const shellBlock =
  `\nSHELL: one command per Bash call — never cd, &&, ;, newlines, heredocs or VAR= prefixes; address your worktree ` +
  `with git -C <path>, go -C <path>, make -C <path> or absolute paths. ` +
  `Never run a script with bash or sh and never write helper scripts; jq takes files as positional arguments — never ` +
  `--slurpfile, --rawfile or -f; no shell globs (*, ?) in paths — list with ls or git ls-files, then pass literal paths; ` +
  `scratch files go under ${roundDir}/scratch/, never /tmp.\n` +
  (prebuilt.length
    ? `PREBUILT: ${prebuilt.join(', ')} are linked into your worktree, built once for this run — run binaries from there; ` +
      `never go run, go build or make build.\n`
    : '')

// Second-confirmation brief shared by the refute and verify stages (SKILL ROUND-007 / ROUND-011a).
// ROUND-035: the producing lane's context travels with every candidate — never less than first confirmation.
const manifestOf = artifact => `${artifact}.lane-manifest.json`
// One LANE-MANIFEST:<absolute path> line per distinct manifest, placed directly under the
// REQUIRE-SKILL line — the skill-invoke gate exempts those manifests' files from its context ban.
const manifestMarkers = paths => [...new Set(paths)].map(path => {
  if (!path.startsWith('/')) throw new Error(`manifest path "${path}" is not absolute — artifactsDir must be an absolute path`)
  return `LANE-MANIFEST:${path}\n`
}).join('')
const laneContextBlock = direction =>
  `LANE CONTEXT — everything the producing lane had. Diff: \`${diffCmd}\`. Each candidate's ` +
  `"manifest" is its lane's read-gate manifest: "files" is the lane's review set (a tests lane's includes each test's ` +
  `production counterparts), "walk" holds the checklist item its "checklist" field cites — read both before judging.\n` +
  (specPaths.length && (direction === 'correctness' || direction === 'tests') ? `SPECS: ${specPaths.join(', ')}\n` : '') +
  (direction === 'special-focus' ? `Focus argument: ${args.focus}\n` : '') +
  (typeof args.contextRefs === 'string' && args.contextRefs ? `Additional context: ${args.contextRefs}\n` : '') +
  ledgerBlock
function refuterPrompt(findings, branch, slug) {
  const many = findings.length > 1
  const direction = findings[0].direction
  const candidates = findings.map(f => ({ ...f, manifest: manifestOf(f.artifact) }))
  return (
    `REQUIRE-SKILL:railroad-review\n` +
    manifestMarkers(candidates.map(c => c.manifest)) +
    `You are a REFUTER in a railroad-review on ${args.base}...${args.head} (round ${r}). You did not produce the ` +
    `candidate${many ? 's' : ''} below and know nothing about how the code came to be. Your brief: REFUTE ${many ? 'each one' : 'it'}. ` +
    `"Could not reproduce" is a successful result; never manufacture evidence. You run in your own isolated git worktree.\n` +
    wtLine(slug, branch) + `\n` +
    `FIRST ACTION: invoke the Skill tool with skill="railroad-review" and args="lane:${direction}"; the hook injects the ` +
    `"${direction}" direction's review context and a gate blocks every other tool call until you have invoked it and ` +
    `read the injected context file in full.\n\n` +
    `SECOND ACTION: ${pinStep(branch, 'stop — settle every candidate as verdict "unverified" with the git output as ' +
      'reason and probe "none: pin failed", write its verdict file and return')}.\n\n` +
    laneContextBlock(direction) + `\n` +
    `THE CANDIDATE${many ? 'S' : ''} (${findings.length} claim ref${many ? 's' : ''}; each once-confirmed by its lane, still in question; the direction field names the review lens). ` +
    `FIRST read each candidate's write-up from its artifact: \`jq -c '.findings[] | select(.finding_id == "<artifact_id>")' <artifact>\`:\n` +
    `${JSON.stringify(candidates, null, 2)}\n\n` +
    `Each candidate carries effect/exposure with evidence; attack the cells as claims — an exposure "always" without a ` +
    `named path, an effect "incorrect" on an absent behavior without a checklist item. When a cell falls but the defect ` +
    `stands, return confirmed with your own effect/exposure; when you agree, omit them. Cells only ever lower a severity.\n\n` +
    `Perform each claim's SECOND CONFIRMATION per the railroad-review skill's Probe protocol (PROBE entries), one claim at a time, independently:\n` +
    `- ACDSL repos (acdsl/registry.json or bin/acdsl exists): author a debug probe — one rule line in the untracked file ` +
    `railroad-probes-r${r}.acdsl at your worktree root (\`//acdsl:<PROBE-ID> <script> anchor="^<defect file, regex-escaped>$" ` +
    `lifetime="task" why="<the claim>"\`; <PROBE-ID> = PROBE- + the finding slug upper-cased, unique per finding), ` +
    `mirrored by writing the same line to ${roundDir}/probes/<finding-slug>.acdsl (your own ` +
    `file — never another finding's). <script> is a repo-registry verifier with params when one fits (forbid-phrase, ` +
    `symbol-exists, …) — then there is NO registry work; else a script under ${roundDir}/probes/ (exit 0 clean, ` +
    `exit 1 + "file:line: message" on hit) registered in ${roundDir}/probes/registry-${findingSlug(findings[0].finding_id)}.json ` +
    `holding ONLY your entries (argv ["/bin/bash", "<abs script>"], never chmod, timeout_s <= 60) — \`-registry\` lays it ` +
    `over the repo registry, so never copy repo entries. ` +
    `VALIDITY FIRST: the probe must exit 1 on a minimal fail fixture under ${roundDir}/probes/fixtures/ before the real run — ` +
    `the one direct run allowed: the verifier itself (its prebuilt binary under the linked paths, ` +
    `with \`<files list> key=value\`, or your ` +
    `script's interpreter) over a hand-built files list written under ${roundDir}/scratch/<finding-slug>/, NEVER under ` +
    `${roundDir}/probes/ and never /tmp (transient scaffolding, not a review artifact); report the fixture's path in the ` +
    `fixture field and the script in the script field. Two attempts before declaring a claim unprobeable. Never author a ` +
    `Makefile or .mk file, never run a build. VERDICT, from your worktree root, exactly: ` +
    `\`<your worktree>/bin/acdsl check -rule <PROBE-ID> [-registry <your registry>]\` — provisioning links the prebuilt ` +
    `binary; never a make target. ` +
    `RED on the claimed file -> "confirmed"; ` +
    `GREEN (after the fixture proved the probe works) -> "debunked". Unprobeable after two attempts -> manual path, probe "none: <why>".\n` +
    `- Otherwise (or unprobeable): the cheapest settling artifact, in preference order — a failing test, a command with ` +
    `actual and expected output, a reproduction sequence with the observed result. A test demonstrates the claim only if it ` +
    `fails at ${args.headCommit} FOR THE REASON THE CLAIM STATES and passes with the proposed fix — check both directions and ` +
    `say you did. For that both-directions check you may apply the proposed fix in your disposable worktree, run the test, ` +
    `and restore the file content exactly afterwards — the fix never persists and is never committed. Probe field "manual: <kind>".\n` +
    `- Survives refutation but not demonstrable in this environment -> verdict "unverified" with the CONCRETE missing ` +
    `precondition ("needs a device with an expired subscription"), never a bare "not demonstrated".\n\n` +
    `Write every artifact under ${roundDir}/, named for the finding it settles (<finding-slug>-*). Write each verdict in full — ` +
    `finding_id, verdict, reason, probe, script, fixture, registry (its path relative to ${roundDir}/probes/, absent when ` +
    `none was needed), your cells when you return them, evidence (what was run/read, ` +
    `decisive lines), artifacts (absolute paths) — never a copy of the candidate, the script holds it — ` +
    `to ${roundDir}/verdicts/<finding-slug>.json the moment it is settled, ` +
    `and return only its short fields plus verdict_path and evidence — one string, the file's decisive line: its ` +
    `evidence.probe_run, else its first evidence.read entry. ` +
    knownBlock +
    shellBlock +
    `Review ONLY otherwise — no persistent edits, no tree-mutating git commands (reset/clean/stash/revert; checkout only in ` +
    `the SECOND ACTION pin). Do NOT enter plan mode.\n` +
    `TERMINATE after returning the structured output: exactly one verdict per candidate, in input order.`
  )
}
// GEN:end

// SKILL: SKILL-RAILROADREVIEW-ROUND-025 (Groupers) / ROUND-033 / ROUND-006 / ROUND-022a
if (!Array.isArray(args.claimsByDirection) || !args.claimsByDirection.length) {
  throw new Error('args.claimsByDirection is required — [{direction, claims[]}] assembled by the dispatcher from the lanes stage result via targeted jq')
}

// ROUND-033: severity is recomputed from the cells at stage entry over the in-memory lane
// returns; a mismatch corrects the severity, re-mints the id's SEV token and is recorded.
const corrections = []
const recompute = f => {
  let severity
  try {
    severity = severityOf(f)
  } catch (err) {
    corrections.push({ from_id: f.finding_id, to_id: f.finding_id, from: f.severity, to: f.severity, invalid: `${f.effect}/${f.exposure}` })
    log(`severity recompute skipped for ${f.finding_id}: invalid vector ${f.effect}/${f.exposure} — lane severity kept`)
    return f
  }
  if (severity === f.severity) return f
  const to = remintId(f.finding_id, severity)
  corrections.push({ from_id: f.finding_id, to_id: to, from: f.severity, to: severity })
  log(`severity recomputed ${f.finding_id} -> ${to} (${f.effect}/${f.exposure})`)
  return { ...f, severity, finding_id: to }
}
const claimsByDirection = args.claimsByDirection.map(fd => ({ direction: fd.direction, claims: fd.claims.map(recompute) }))

function grouperPrompt(scopeLabel, claims) {
  const artifacts = [...new Set(claims.map(c => c.artifact))]
  return (
    `You are the dedup grouper for ${scopeLabel} of a railroad-review (round ${r}). ` +
    `Below are ${claims.length} once-confirmed claim REFS from independent reviewers of the same diff; each claim's ` +
    `write-up (claim, evidence) lives in its artifact file — read it with ` +
    `\`jq -c '.findings[] | select(.finding_id == "<artifact_id>")' <artifact>\` (artifacts: ${artifacts.join(', ')}). ` +
    `Group claims that assert THE SAME defect — same root cause — even when they cite different lines or use different ` +
    `wording. Different defects that happen to sit close together stay separate. When in doubt, keep claims SEPARATE: ` +
    `a wrong merge silently drops a finding, a missed merge only costs one duplicate check downstream.\n\n` +
    `${JSON.stringify(claims, null, 2)}\n\n` +
    `Return every claim in exactly one group (singletons included). Pick each group's survivor by argument quality — ` +
    `most concrete trigger, strongest evidence — never by text length. This is a text-only judgment over the artifact ` +
    `files; you have no repo access and need none. Return the structured output only.`
  )
}

// Group assembly shared by both dedup call sites: best-argued survivor, max group
// severity, merged_from provenance, unplaced claims kept, absorbed ids reported as
// duplicates so the station can disposition them duplicate-of.
function assembleGroups(claims, groups) {
  const byId = new Map(claims.map(c => [c.finding_id, c]))
  const placed = new Set()
  const out = []
  const duplicates = []
  for (const g of groups) {
    const members = (g.member_ids || []).filter(id => byId.has(id) && !placed.has(id))
    if (!members.length) continue
    const survivor = byId.get(g.survivor_id) && !placed.has(g.survivor_id) ? byId.get(g.survivor_id) : byId.get(members[0])
    members.forEach(id => placed.add(id))
    placed.add(survivor.finding_id)
    const top = members.map(id => byId.get(id))
      .reduce((a, b) => (SEVERITY_RANK[b.severity] || 0) > (SEVERITY_RANK[a.severity] || 0) ? b : a, survivor)
    const mergedFrom = members.filter(id => id !== survivor.finding_id)
    mergedFrom.forEach(id => duplicates.push({ finding_id: id, duplicate_of: survivor.finding_id }))
    out.push({
      ...survivor,
      ...(top.severity !== survivor.severity
        ? { severity: top.severity, effect: top.effect, exposure: top.exposure, finding_id: remintId(survivor.finding_id, top.severity) }
        : {}),
      // absorbed members hand their own merged_from on, so no earlier merge drops out of the funnel
      ...(mergedFrom.length ? { merged_from: [...(survivor.merged_from || []), ...mergedFrom, ...mergedFrom.flatMap(id => byId.get(id).merged_from || [])] } : {}),
    })
  }
  // any claim the grouper failed to place stays in — never silently dropped
  for (const c of claims) if (!placed.has(c.finding_id)) out.push(c)
  return { claims: out, duplicates }
}

async function dedupe(claims, scopeLabel, label) {
  if (claims.length <= 1) return { claims, duplicates: [] }
  const res = await agent(grouperPrompt(scopeLabel, claims), {
    label, phase: 'Dedup', agentType: 'railroad-grouper', schema: GROUPER_SCHEMA, ...laneOpts, effort: 'low',
  })
  if (!res || !Array.isArray(res.groups)) return { claims, duplicates: [] } // grouper lost: verify duplicates rather than lose claims
  return assembleGroups(claims, res.groups)
}

phase('Dedup')
const dedupedByDirection = await parallel(claimsByDirection.map(fd => async () => {
  const { claims } = await dedupe(fd.claims, `the "${fd.direction}" direction`, `dedup-${fd.direction}`)
  if (fd.claims.length > claims.length) log(`direction ${fd.direction}: semantic dedup ${fd.claims.length} -> ${claims.length} claims`)
  return { direction: fd.direction, claims }
}))

// Cross-direction dedup (SKILL ROUND-022a): union every direction's claims, one grouper
// over the union, deterministic assembly — BEFORE refutation, so no refuter is ever
// spent on a cross-direction duplicate.
phase('Cross-direction dedup')
const finished = dedupedByDirection.filter(Boolean)
const merged = finished.flatMap(fd => fd.claims.map(c => ({ ...c, direction: fd.direction })))
// D2: every severity continues — the ledger marks the rows at or below ignore-level after the gate.
const unionClaims = merged
const crossDedup = finished.length > 1
  ? await dedupe(unionClaims, 'the union of all directions', 'dedup-cross')
  : { claims: unionClaims, duplicates: [] }
if (crossDedup.duplicates.length) log(`cross-direction dedup: ${unionClaims.length} -> ${crossDedup.claims.length} claims`)

return {
  round: r,
  claims: crossDedup.claims,
  duplicates: crossDedup.duplicates,
  // ROUND-020 conservation: within-direction merges counted here
  withinDuplicates: claimsByDirection.reduce((n, fd) => n + fd.claims.length, 0) - merged.length,
  unionCount: unionClaims.length,
  corrections,
}
