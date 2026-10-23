export const meta = {
  name: 'railroad-lanes',
  description: 'Fan a railroad-review round out to n review lanes per direction (× chunks) in isolated worktrees at the review head — spawn control and lane prompts for the Fan-out stage',
  whenToUse: 'Invoked by the /railroad-review dispatcher at the Fan-out spawn (SKILL ROUND-001), once per round',
  phases: [
    { title: 'Fan-out', detail: 'n lanes per direction (× chunks when chunked), isolated worktrees pinned at the review head, review-only' },
  ],
}

// GEN:schemas finding,lane,severity-table,severity-fixtures
const FINDING_PROPS = {
  "finding_id": {
    "type": "string",
    "description": "stable id, unique across directions: <direction>-<SEV>-<n> with SEV in BLK/MAJ/MIN/NIT/INF, e.g. critical-BLK-1"
  },
  "file": {
    "type": "string"
  },
  "line": {
    "type": "integer"
  },
  "severity": {
    "type": "string",
    "enum": [
      "BLOCKER",
      "MAJOR",
      "MINOR",
      "NIT",
      "INFO"
    ],
    "description": "the severity-table row for effect/exposure; INFO exactly when fix is \"none\""
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
    "description": "what goes wrong if this ships as-is — destructive: data lost or corrupted, a security boundary crossed, an irreversible migration; incorrect: a wrong result, a crash, a broken promise, or a required behavior that is absent (checklist cites the item); degraded: works but slower, noisier, leakier, harder to operate — users or operators feel it; debt: only developers feel it — maintainability, tests, tooling; cosmetic: no functional effect"
  },
  "exposure": {
    "type": "string",
    "enum": [
      "always",
      "often",
      "rare",
      "eventually"
    ],
    "description": "how much of normal operation hits it — always: every use; often: a common configuration, input class or state, named in evidence; rare: an unusual but real input, state or timing, named; eventually: nobody today — needs code that does not exist or an unreachable state"
  },
  "claim": {
    "type": "string",
    "description": "the defect and why it violates the direction standard"
  },
  "fix": {
    "type": "string",
    "description": "concrete proposed fix — one sentence or a short diff sketch"
  },
  "evidence": {
    "type": "string",
    "description": "the lane's confirmation pass — what was re-read and why the claim holds; the claim's first confirmation (in question until the station re-verifies); lane artifacts only — ranked_plan entries omit it"
  },
  "checklist": {
    "type": "string",
    "description": "the checklist item this finding answers — a context entry id on rule directions, <direction>-<n> on correctness and contracts; a walk-item FAIL always cites its item here, never a prose quote"
  },
  "route": {
    "type": "string",
    "enum": [
      "human",
      "auto-fix"
    ],
    "description": "station-assigned fix route on ranked_plan entries only: human (needs author judgment, surfaced at the gate) | auto-fix (applied by the fix-apply stage)"
  },
  "route_reason": {
    "type": "string",
    "description": "ranked_plan entries only: one sentence why the route — the judgment a human must make, or why the fix is mechanical"
  },
  "known": {
    "type": "boolean",
    "description": "ranked_plan entries only: true when the claim matches a KNOWN-FINDINGS ledger entry — carried from a prior round, never blocking, never routed for a fix this round"
  },
  "known_id": {
    "type": "string",
    "description": "ranked_plan entries only, with known: the finding_id of the ledger row the claim matched"
  },
  "artifact": {
    "type": "string",
    "description": "ranked_plan entries only: the lane artifact path"
  },
  "artifact_id": {
    "type": "string",
    "description": "ranked_plan entries only: the finding's id inside that artifact"
  }
}
const LANE_SCHEMA = {
  "type": "object",
  "required": [
    "direction",
    "lane",
    "aborted",
    "artifact_json",
    "checkout_sha",
    "walk_complete",
    "walk_open",
    "claim_refs"
  ],
  "properties": {
    "direction": {
      "type": "string",
      "description": "the railroad-review direction this lane walked"
    },
    "lane": {
      "type": "integer",
      "description": "this lane's 1-based index within its direction"
    },
    "aborted": {
      "type": "boolean",
      "description": "true when the premise check failed and the lane stopped without reviewing — an aborted lane never counts as a finished review"
    },
    "artifact_json": {
      "type": "string",
      "description": "absolute path of the JSON findings artifact written under the round dir"
    },
    "checkout_sha": {
      "type": "string",
      "description": "full HEAD SHA of the lane worktree, from git rev-parse after the SECOND ACTION pin — recorded in the artifact JSON too"
    },
    "overlay_ok": {
      "type": "boolean",
      "description": "overlay runs only: true exactly when git apply -R --check of the review-context overlay exits 0 in the lane worktree — recorded in the artifact JSON too"
    },
    "walk_complete": {
      "type": "boolean",
      "description": "true exactly when the artifact's walk_complete flag was accepted by the read-gate — every checklist pair answered"
    },
    "walk_open": {
      "type": "integer",
      "description": "number of checklist pairs (file × group, or group) still without a walk entry; 0 when complete"
    },
    "claim_refs": {
      "type": "array",
      "description": "one ref per finding in the artifact — everything the pipeline computes on; the write-up (claim, evidence) stays in the artifact file",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "file",
          "line",
          "severity",
          "effect",
          "exposure",
          "fix"
        ],
        "properties": {
          "finding_id": {
            "type": "string",
            "description": "the artifact's finding id: <direction>-<SEV>-<n>"
          },
          "file": {
            "type": "string"
          },
          "line": {
            "type": "integer"
          },
          "severity": {
            "type": "string",
            "enum": [
              "BLOCKER",
              "MAJOR",
              "MINOR",
              "NIT",
              "INFO"
            ],
            "description": "the severity-table row for effect/exposure; INFO exactly when fix is \"none\" — recomputed at dedup entry"
          },
          "effect": {
            "type": "string",
            "enum": [
              "destructive",
              "incorrect",
              "degraded",
              "debt",
              "cosmetic"
            ]
          },
          "exposure": {
            "type": "string",
            "enum": [
              "always",
              "often",
              "rare",
              "eventually"
            ]
          },
          "fix": {
            "type": "string",
            "description": "the concrete fix, one sentence or a short diff sketch; \"none\" for INFO"
          },
          "checklist": {
            "type": "string",
            "description": "the checklist item this finding answers — a context entry id on rule directions, <direction>-<n> on correctness and contracts"
          }
        }
      }
    },
    "notes": {
      "type": "string",
      "description": "missing context files, excluded scope, anomalies"
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
const SEVERITY_EXAMPLES = [
  {
    "effect": "incorrect",
    "exposure": "often",
    "severity": "BLOCKER",
    "discriminator": "exposure=often: a normal input class hits it (any retry after a timeout)"
  },
  {
    "effect": "incorrect",
    "exposure": "rare",
    "severity": "MAJOR",
    "discriminator": "exposure=rare: an unusual but real input hits it (a home directory containing a space)"
  },
  {
    "effect": "incorrect",
    "exposure": "rare",
    "severity": "MAJOR",
    "discriminator": "effect=incorrect: the plan requires this edge case handled and it is not — the required outcome is absent; checklist cites the item"
  },
  {
    "effect": "destructive",
    "exposure": "rare",
    "severity": "BLOCKER",
    "discriminator": "effect=destructive: data deleted, even on a rare path"
  },
  {
    "effect": "destructive",
    "exposure": "eventually",
    "severity": "MAJOR",
    "discriminator": "exposure=eventually: the delete path exists but no caller reaches it today — never below MAJOR"
  },
  {
    "effect": "degraded",
    "exposure": "always",
    "severity": "MAJOR",
    "discriminator": "effect=degraded: every request re-reads the config file — works, slower, users feel it"
  },
  {
    "effect": "debt",
    "exposure": "often",
    "severity": "MINOR",
    "discriminator": "effect=debt: a helper duplicated in two packages every change touches — only developers feel it"
  },
  {
    "effect": "cosmetic",
    "exposure": "often",
    "severity": "NIT",
    "discriminator": "effect=cosmetic: an untested branch no spec mentions"
  }
]
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

const idFloor = args.idFloor && typeof args.idFloor === 'object' ? args.idFloor : {}

// SKILL: SKILL-RAILROADREVIEW-ROUND-001 (Fan-out) / ROUND-032 (Lane review)
if (!Array.isArray(args.lanePlan) || !args.lanePlan.length) {
  throw new Error('args.lanePlan is required — the lanes array $SKILL_RAILROADREVIEW_LANES lanes printed; an empty plan means no lane to fan out')
}
for (const lane of args.lanePlan) {
  const isComplete = lane && ['artifact', 'direction', 'manifest', 'slug'].every(key => typeof lane[key] === 'string' && lane[key]) && Number.isInteger(lane.lane)
  if (!isComplete) {
    throw new Error('each lanePlan entry must be {artifact, chunk, direction, files, lane, manifest, slug} as $SKILL_RAILROADREVIEW_LANES lanes printed it')
  }
}
if (args.lanePlan.some(lane => lane.direction === 'special-focus') && (typeof args.focus !== 'string' || !args.focus)) {
  throw new Error('the special-focus direction requires args.focus — the explicit focus argument')
}
const contextRefs = typeof args.contextRefs === 'string' ? args.contextRefs : ''
const directions = [...new Set(args.lanePlan.map(lane => lane.direction))]
const laneBranchName = lane => lane.chunk
  ? `${branchNs}${lane.direction}-${chunkSlug(lane.chunk)}-l${lane.lane}-r${r}-${hash6}`
  : `${branchNs}${lane.direction}-l${lane.lane}-r${r}-${hash6}`
const laneLabel = lane => lane.chunk ? `${lane.direction}-${chunkSlug(lane.chunk)}-l${lane.lane}` : `${lane.direction}-l${lane.lane}`
const laneStub = lane => ({ direction: lane.direction, lane: lane.lane, aborted: true, artifact_json: '', checkout_sha: '', claim_refs: [], walk_complete: false, walk_open: -1 })

function lanePrompt(lane, continuation, branch) {
  const direction = lane.direction
  const chunk = lane.chunk ? { name: lane.chunk } : null
  const artifact = lane.artifact
  const fileFirst = !['correctness', 'contracts'].includes(direction)
  const laneTotal = args.lanePlan.filter(other => other.direction === direction && (other.chunk || '') === (lane.chunk || '')).length
  return (
    `REQUIRE-SKILL:railroad-review\n` +
    manifestMarkers([lane.manifest]) +
    `You are lane ${lane.lane} of ${laneTotal} on the "${direction}" direction of a railroad-review fan-out on ${args.base}...${args.head} (round ${r})` +
    (chunk ? `, scoped to chunk "${chunk.name}" of a chunked review` : '') + `. ` +
    `You run in your own isolated git worktree; your SECOND ACTION pins it at the review head ${args.headCommit}. ` +
    `Other lanes review the same direction independently — ` +
    `never coordinate with them or read their artifacts.\n` +
    wtLine(lane.slug, branch) +
    `\nFIRST ACTION: invoke the Skill tool with skill="railroad-review" and args="lane:${direction}"; the hook injects the "${direction}" ` +
    `direction's review context (its context-map.json slice — an unmapped direction receives the full set) and ` +
    `a gate blocks every other tool call until you have invoked it and read the injected context file in full. Then follow the ` +
    `"${direction}" direction definition in its SKILL.md exactly — your context is the injection; do not read files from ` +
    `the repo's context directory (denied; language guides excepted), walk only that direction's checklist.\n` +
    `MISSING CONTEXT: when the injection carries no context entry, or lacks one your direction requires, start your ` +
    `notes with "MISSING CONTEXT: <what is missing>" and review degraded from the direction definition — never a ` +
    `finding, never an abort.\n\n` +
    `SECOND ACTION: ${pinStep(branch, 'return aborted=true with the exact commit facts and the git output in notes ' +
      '(artifact paths empty, no artifacts written) and stop — do not review from a different premise')}. Otherwise aborted=false. ` +
    `Record \`git rev-parse HEAD\` after the pin as the top-level field \`checkout_sha\` in your JSON artifact and in your return value.` +
    (overlayPatch
      ? ` Also record the top-level field \`overlay_ok\` in your JSON artifact and return value: true exactly when ` +
        `\`git -C <your worktree> apply -R --check ${overlayPatch}\` exits 0 (the review-context overlay is applied), else false.`
      : '') + `\n\n` +
    `Diff with \`${diffCmd}\`; read each changed file in full, not just the hunks.\n` +
    `YOUR ASSIGNMENT is your manifest ${lane.manifest} — read it first with \`jq . ${lane.manifest}\`: "files" minus ` +
    `"partners" are the changed files you review; "partners" are counterpart files — read each in full as context, judge ` +
    `coverage of your files against them, never review them; "walk" is your checklist.\n` +
    (chunk
      ? `CHUNK SCOPE: your files are chunk "${chunk.name}" — sibling lanes cover the other chunks. Flag any suspected ` +
        `cross-chunk issue (a contract or invariant you cannot verify inside your chunk) in your notes for the station; ` +
        `do not chase it into other chunks.\n`
      : '') +
    (direction === 'correctness'
      ? `Test files are out of your scope — the tests direction reviews them with their production files; never read ` +
        `or flag a test file.\n`
      : '') +
    (specPaths.length && (direction === 'correctness' || direction === 'tests')
      ? `SPECS: ${specPaths.join(', ')}${direction === 'tests' ? ' — every test they name for your files must exist and cover its ' +
        'stated cases; a missing one is a finding citing the spec and its section' : ''}.\n`
      : '') +
    `COVERAGE IS ENFORCED: your findings-artifact write is denied by the read-gate until every "files" entry of your ` +
    `manifest is fully read — the deny message lists exact Read offsets to close each gap.\n` +
    `WALK: the manifest's "walk" object is your checklist — groups of ${fileFirst ? 'rules' : direction === 'correctness' ? 'spec requirements' : 'changed contract boundaries'}. ` +
    `Read it after the files. ` +
    (fileFirst
      ? `Walk FILE BY FILE: for each assigned file, look at its changed lines, then take one group at a time and answer it ` +
        `in one step — append one entry {"file", "group", "hits": [finding ids]} to the artifact's "walk" array, hits empty ` +
        `when the group has nothing for that file. Every file × every group needs an entry.\n`
      : `Walk GROUP BY GROUP: for each group, recall — or re-read when unsure — the places in the diff that concern it and ` +
        `answer the group in one step — append one entry {"group", "hits": [finding ids]} to the artifact's "walk" array. ` +
        `Every group needs an entry. A FAIL on an item is a finding: effect "incorrect", its "checklist" field the item id, ` +
        `its evidence naming the item.\n`) +
    `A finding born from a group answer names its item in "checklist" — the context entry id, or the <direction>-<n> item id.\n` +
    `ARTIFACT AS YOU GO: right after the reads, Write the artifact skeleton at ${artifact} — every top-level field, ` +
    `"walk": [], "findings": [], "walk_complete": false. Then append each finding to "findings" the moment it exists and ` +
    `each answer to "walk" the moment the group is answered, with Edit — never one final dump. LAST, in its own Edit, set ` +
    `"walk_complete": true — the read-gate denies it while any pair lacks a walk entry and lists them; answer them and retry.\n` +
    (continuation
      ? `CONTINUATION: ${artifact} already exists — a previous run of this lane stopped before its walk was complete. ` +
        `Read it in full before anything else, keep every existing finding and walk entry, skip answered pairs, continue at ` +
        `the first unanswered one.\n`
      : '') +
    `FALSIFIABILITY${direction === 'code-style' ? ' (relaxed for code-style: your ground truth is the style guide)' : ''}: ` +
    `every claim must name the concrete input, state, or sequence that produces the wrong behaviour. Reject at the source: ` +
    `naming opinions, architecture preference, and unquantified robustness ("could be more robust") never become claims.\n` +
    `CONFIRMATION PASS: your findings are CLAIMS. Before returning, re-verify each claim you drafted — re-read its cited ` +
    `code in full context; drop any claim that does not survive its own re-check. Record in the claim's "evidence" field ` +
    `what you re-read and why the claim holds. This is the claim's FIRST confirmation — it stays in question until the ` +
    `refutation stage or the station merge independently re-verifies it.\n` +
    (direction === 'special-focus' ? `Focus argument: ${args.focus}\n` : '') +
    (contextRefs ? `Additional context: ${contextRefs}\n` : '') +
    ledgerBlock +
    knownBlock +
    shellBlock +
    `\nHard constraints, violation disqualifies this lane:\n` +
    `1. Review ONLY — make no edits to any file. Fixes are a separate step applied after the human gate, never during lane review.\n` +
    `2. Never read sibling worktrees or other agents' artifacts — your own artifact files are your only writes outside your worktree.\n` +
    `3. Do NOT enter plan mode and do NOT call ExitPlanMode. Write your findings as ` +
    `${artifact} (the structured findings, JSON only — no .md sibling) — OUTSIDE your worktree — then TERMINATE, ` +
    `never park on an approval prompt. Leave files you created in place; cleanup salvages them. ` +
    `Every line you cite is the line's number in \`git show ${args.headCommit}:<file>\`, the committed file.\n` +
    `4. No tree-mutating git commands — reset, clean, stash, revert, cherry-pick, merge, rebase are all forbidden, and so is ` +
    `any checkout outside the SECOND ACTION. Your only git writes are the SECOND ACTION pin: the branch rename and, when HEAD ` +
    `differs from ${args.headCommit}, the checkout.\n\n` +
    `Return the structured output: direction, lane index, the artifact path, walk_complete, walk_open (unanswered pairs), ` +
    `and one CLAIM REF per finding — the write-up (claim, evidence) stays in the artifact finding, never in the return — ` +
    `(finding_id as "${direction}-<SEV>-<n>" with SEV in BLK/MAJ/MIN/NIT/INF, file, line, checklist, then TWO CELLS you answer ` +
    `from evidence, never from how bad it looks — effect (what goes wrong if this ships as-is: destructive = data lost or ` +
    `corrupted, a security boundary crossed, an irreversible migration; incorrect = a wrong result, a crash, a broken ` +
    `promise the code makes, or a behavior the spec, plan or DoD requires that is absent — cite the checklist item; ` +
    `degraded = works but slower, noisier, leakier, harder to operate — users or operators feel it; debt = only ` +
    `developers feel it — maintainability, tests, tooling; cosmetic = no functional effect), exposure (how much of ` +
    `normal operation hits it: always = every use, every time; often = a common configuration, input class or state — ` +
    `name it; rare = an unusual but real input, state or timing — name it; eventually = nobody today, needs code that ` +
    `does not exist or an unreachable state). A cell you cannot prove takes the value that yields the LOWER severity. severity is the table row for your ` +
    `cells — the table: ${renderTable()}; a finding proposing no change has fix "none" and severity INFO. Calibration ` +
    `pairs, each naming the cell that decides: ${JSON.stringify(SEVERITY_EXAMPLES)}. The ref closes with the fix — one ` +
    `sentence or a short diff sketch, never empty, "none" when no change is asked; the artifact finding carries the same ` +
    `fields plus the claim and the evidence from the confirmation pass), plus the aborted flag. ` +
    `Empty claim_refs with aborted=false ` +
    `is a valid, expected result — a clean review; aborted=true is a failed premise, never a clean review.`
  )
}

phase('Fan-out')
const abortedLanes = []
// Every name handed out — a re-spawned lane on the harness route takes the next -a<n> name.
const laneBranches = []
// DR4: a lane that returns nothing (lost) or returns with an open walk is run
// again over the same manifest; it continues from its artifact. Stops as an
// aborted lane after two consecutive lost runs or a run that closed nothing.
const runLane = async lane => {
  let prev = null
  let lost = 0
  for (let attempt = 1; ; attempt++) {
    const branch = attemptBranch(laneBranchName(lane), attempt)
    if (!laneBranches.includes(branch)) laneBranches.push(branch)
    const res = await agent(lanePrompt(lane, prev !== null || lost > 0, branch), {
      label: laneLabel(lane), phase: 'Fan-out', agentType: `railroad-lane-${lane.direction}`,
      ...isoOpts, schema: LANE_SCHEMA, ...laneOpts,
    })
    if (!res) {
      if (++lost >= 2) return { ...laneStub(lane), notes: 'lane lost twice in a row without returning' }
      continue
    }
    lost = 0
    if (res.aborted || res.walk_complete) return res
    if (prev && res.walk_open >= prev.walk_open) return { ...res, aborted: true, notes: `walk stalled at ${res.walk_open} open pair(s); ${res.notes || ''}` }
    log(`lane ${laneLabel(lane)}: walk open ${res.walk_open}, continuing`)
    prev = res
  }
}
const directionResults = await pipeline(
  directions,
  d => {
    const lanes = args.lanePlan.filter(lane => lane.direction === d)
    return parallel(lanes.map(lane => () => runLane(lane))).then(results => ({ lanes, results }))
  },
  (fanout, d) => {
    const zipped = fanout.results.map((result, idx) => ({ ...(result || laneStub(fanout.lanes[idx])), plan: fanout.lanes[idx] }))
    for (const l of zipped.filter(l => l.aborted)) {
      abortedLanes.push({ direction: d, lane: l.lane, chunk: l.plan.chunk || null, manifest: l.plan.manifest, notes: l.notes || '' })
    }
    const finished = zipped.filter(l => !l.aborted)
    if (zipped.length > finished.length) log(`direction ${d}: ${zipped.length - finished.length}/${zipped.length} lanes aborted`)
    if (!finished.length) return null
    // OUT-002: lanes number their ids from 1 independently — the direction re-mints one
    // sequence across its lanes and severities, so a later severity re-mint never collides;
    // artifact_id keeps the lane's own id
    let k = Number.isInteger(idFloor[d]) ? idFloor[d] : 0
    const produced = finished.flatMap(l => (l.claim_refs || []).map(f => {
      const sev = (String(f.finding_id).match(/-(BLK|MAJ|MIN|NIT|INF)-\d+$/) || [])[1] || SEV_TOKEN[f.severity] || 'INF'
      k += 1
      return { ...f, finding_id: `${d}-${sev}-${k}-r${r}`, artifact_id: f.finding_id, direction: d, artifact: l.artifact_json }
    }))
    return {
      direction: d, claims: produced, claims_produced: produced.length,
      coverage_gaps: zipped.filter(l => l.aborted).map(l => l.plan.manifest),
      lane_artifacts: finished.map(l => l.artifact_json),
      notes: finished.map(l => l.notes).filter(Boolean).join(' | '),
    }
  },
)

const deadDirections = directions.filter((_, i) => !directionResults[i])
const finishedDirections = directionResults.filter(Boolean)
if (deadDirections.length) log(`dead directions (all lanes skipped, errored, or aborted): ${deadDirections.join(', ')}`)

return {
  round: r, directions,
  perDirection: finishedDirections,
  deadDirections, abortedLanes,
  branches: laneBranches,
}
