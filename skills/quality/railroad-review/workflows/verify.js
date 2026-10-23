export const meta = {
  name: 'railroad-verify',
  description: 'Second confirmation of a railroad-review round: fresh refuters over claims at or above the refute threshold and verifier batches over the rest — all concurrent, a lost refuter\'s claim re-entering the pool the moment the refuter is lost',
  whenToUse: 'Invoked by the /railroad-review round orchestrator at the second-confirmation spawns (SKILL ROUND-007 + ROUND-011a), after the dedup stage returns',
  phases: [
    { title: 'Second confirmation', detail: 'refuters (one per candidate at or above the threshold) and verifier batches over the rest, concurrent; a lost refuter\'s claim retried at once' },
  ],
}

// GEN:schemas verdict,refuter,severity-table
const VERDICT_PROPS = {
  "finding_id": {
    "type": "string"
  },
  "verdict": {
    "type": "string",
    "enum": [
      "confirmed",
      "debunked",
      "unverified"
    ],
    "description": "confirmed (survived refutation, artifact in hand) | debunked (refuted) | unverified (survives but not demonstrable here)"
  },
  "reason": {
    "type": "string",
    "description": "one line: debunked — what refuted it; unverified — the concrete missing precondition; confirmed — the decisive artifact"
  },
  "probe": {
    "type": "string",
    "description": "ACDSL repos: the probe rule id that decided the verdict, or \"none: <why unprobeable after 2 attempts>\"; \"manual: <kind>\" otherwise"
  },
  "script": {
    "type": "string",
    "description": "the probe executable, path relative to the round's probes/ dir — absent on the manual path"
  },
  "fixture": {
    "type": "string",
    "description": "the minimal fail fixture the probe went RED on in the validity check, path under the round dir — absent on the manual path"
  },
  "registry": {
    "type": "string",
    "description": "the probe's merged registry, path relative to the round's probes/ dir — absent on the manual path"
  },
  "effect": {
    "type": "string",
    "enum": [
      "destructive",
      "incorrect",
      "degraded",
      "debt",
      "cosmetic"
    ],
    "description": "confirmed only, optional: the refuter's own answer when it differs from the candidate's"
  },
  "exposure": {
    "type": "string",
    "enum": [
      "always",
      "often",
      "rare",
      "eventually"
    ],
    "description": "confirmed only, optional: as effect"
  },
  "verdict_path": {
    "type": "string",
    "description": "absolute path of round-{r}/verdicts/<finding-slug>.json — the full verdict with evidence (what was run/read, decisive lines) and artifacts (absolute paths written under the round dir)"
  },
  "evidence": {
    "type": "string",
    "description": "returned verdict only, one string: the verdict file's decisive line — its evidence.probe_run, else its first evidence.read entry"
  }
}
const REFUTER_SCHEMA = {
  "type": "object",
  "required": [
    "verdicts"
  ],
  "properties": {
    "verdicts": {
      "type": "array",
      "description": "exactly one verdict per candidate, in input order, finding_id echoed",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "verdict",
          "reason",
          "probe",
          "verdict_path"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "verdict": {
            "type": "string",
            "enum": [
              "confirmed",
              "debunked",
              "unverified"
            ],
            "description": "confirmed (survived refutation, artifact in hand) | debunked (refuted) | unverified (survives but not demonstrable here)"
          },
          "reason": {
            "type": "string",
            "description": "one line: debunked — what refuted it; unverified — the concrete missing precondition; confirmed — the decisive artifact"
          },
          "probe": {
            "type": "string",
            "description": "ACDSL repos: the probe rule id that decided the verdict, or \"none: <why unprobeable after 2 attempts>\"; \"manual: <kind>\" otherwise"
          },
          "script": {
            "type": "string",
            "description": "the probe executable, path relative to the round's probes/ dir — absent on the manual path"
          },
          "fixture": {
            "type": "string",
            "description": "the minimal fail fixture the probe went RED on in the validity check, path under the round dir — absent on the manual path"
          },
          "registry": {
            "type": "string",
            "description": "the probe's merged registry, path relative to the round's probes/ dir — absent on the manual path"
          },
          "effect": {
            "type": "string",
            "enum": [
              "destructive",
              "incorrect",
              "degraded",
              "debt",
              "cosmetic"
            ],
            "description": "confirmed only, optional: the refuter's own answer when it differs from the candidate's"
          },
          "exposure": {
            "type": "string",
            "enum": [
              "always",
              "often",
              "rare",
              "eventually"
            ],
            "description": "confirmed only, optional: as effect"
          },
          "verdict_path": {
            "type": "string",
            "description": "absolute path of round-{r}/verdicts/<finding-slug>.json — the full verdict with evidence (what was run/read, decisive lines) and artifacts (absolute paths written under the round dir)"
          },
          "evidence": {
            "type": "string",
            "description": "returned verdict only, one string: the verdict file's decisive line — its evidence.probe_run, else its first evidence.read entry"
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

// SKILL: SKILL-RAILROADREVIEW-ROUND-007 (Refutation) / ROUND-026, ROUND-027,
// ROUND-011a (Verification pool) / ROUND-028
if (!Array.isArray(args.claims)) {
  throw new Error('args.claims is required — the dedup stage\'s cross-deduped claims (all severities); this stage splits them at the refute threshold')
}
const { mode: refuteMode, rank: refuteRank } = levelRank('refuteLevel', 'major', Infinity)
const VERIFY_BATCH = 4 // below-threshold claims per verifier agent
// ROUND-011a: single-direction batches — one lane token per verifier, severity-descending inside.
const batchesOf = claims => {
  const byDirection = new Map()
  for (const c of claims) byDirection.set(c.direction, [...(byDirection.get(c.direction) || []), c])
  return [...byDirection.values()].flatMap(group =>
    Array.from({ length: Math.ceil(group.length / VERIFY_BATCH) }, (_, i) => group.slice(i * VERIFY_BATCH, (i + 1) * VERIFY_BATCH)))
}

phase('Second confirmation')
// ROUND-007: claims at or above the threshold each get a fresh refuter.
// ROUND-011a: the rest go to the verification pool CONCURRENTLY — pool membership is
// known at dedup output; only lost-refuter retries depend on refutation results.
const refuteTargets = refuteMode === 'none' ? []
  : args.claims.filter(f => (SEVERITY_RANK[f.severity] || 0) >= refuteRank)
const refuteIds = new Set(refuteTargets.map(f => f.finding_id))
const pending = args.claims.filter(f => !refuteIds.has(f.finding_id))
  .sort((a, b) => (SEVERITY_RANK[b.severity] || 0) - (SEVERITY_RANK[a.severity] || 0))
const batches = batchesOf(pending)
log(`second confirmation: ${refuteTargets.length} refuter(s) at threshold "${refuteMode}", ${pending.length} pool claim(s) in ${batches.length} batch(es) of <= ${VERIFY_BATCH}`)
const refuterSlug = f => `refute-${findingSlug(f.finding_id)}-r${r}`
const verifierSlug = k => `verify-b${k + 1}-r${r}`
const workerBranches = []
// ROUND-022b: a lost refuter's claim re-enters verification the moment the refuter
// returns nothing — never a second refuter, never a wait for the whole wave.
let retryIndex = 0
const retryLost = f => {
  log(`second confirmation: lost refuter for ${f.finding_id} — the claim re-enters the pool`)
  return verifierThunkFor([f], batches.length + retryIndex++)()
}
const refuterThunks = refuteTargets.map(f => {
  const branch = `${branchNs}refute-${findingSlug(f.finding_id)}-r${r}-${hash6}`
  workerBranches.push(branch)
  return () => agent(refuterPrompt([f], branch, refuterSlug(f)), {
    label: `refute-${f.finding_id}`, phase: 'Second confirmation', agentType: 'railroad-refuter',
    ...isoOpts, schema: REFUTER_SCHEMA, ...laneOpts,
  }).then(verdict => verdict || retryLost(f))
})
const verifierThunkFor = (batch, k) => {
  const branch = `${branchNs}verify-b${k + 1}-r${r}-${hash6}`
  workerBranches.push(branch)
  return () => agent(refuterPrompt(batch, branch, verifierSlug(k)), {
    label: `verify-b${k + 1}`, phase: 'Second confirmation', agentType: 'railroad-verifier',
    ...isoOpts, schema: REFUTER_SCHEMA, ...laneOpts,
  })
}
const verifierThunks = batches.map(verifierThunkFor)
const firstWave = await parallel([...refuterThunks, ...verifierThunks])
const verifications = firstWave.filter(Boolean).flatMap(v => v.verdicts || [])
const settled = new Set(verifications.map(v => v.finding_id))
const unsettled = args.claims.map(c => c.finding_id).filter(id => !settled.has(id))
if (unsettled.length) log(`second confirmation: ${unsettled.length} claim(s) without a verdict (worker lost) — the station re-verifies them: ${unsettled.join(', ')}`)

// Refuter re-vectoring (SKILL ROUND-007 / OUT-005b): a confirmed verdict may carry its own
// cells; severity becomes the lower of the lane's row and the refuter's row, the id's SEV
// token follows, and the verdict is re-keyed to the re-minted id. Refuters never raise.
const verdictById = new Map(verifications.map(v => [v.finding_id, v]))
const applyVerdictVector = claim => {
  const verdict = verdictById.get(claim.finding_id)
  if (!verdict || verdict.verdict !== 'confirmed') return claim
  const hasCells = verdict.effect || verdict.exposure
  if (!hasCells) return claim
  const candidate = { ...claim, effect: verdict.effect || claim.effect, exposure: verdict.exposure || claim.exposure }
  let refuted
  try {
    refuted = severityOf(candidate)
  } catch (err) {
    log(`refuter vector skipped for ${claim.finding_id}: invalid vector ${candidate.effect}/${candidate.exposure} — severity kept`)
    return claim
  }
  if ((SEVERITY_RANK[refuted] || 0) >= (SEVERITY_RANK[claim.severity] || 0)) return claim
  const to = remintId(claim.finding_id, refuted)
  log(`refuter lowered ${claim.finding_id} ${claim.severity} -> ${refuted} (${to})`)
  verdict.finding_id = to
  return { ...candidate, severity: refuted, finding_id: to }
}
const claims = args.claims.map(applyVerdictVector)

return {
  round: r,
  refuteLevel: refuteMode,
  claims,
  verifications,
  unsettled,
  branches: workerBranches,
}
