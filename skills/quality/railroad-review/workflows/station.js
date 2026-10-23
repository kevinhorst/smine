export const meta = {
  name: 'railroad-station',
  description: 'Consolidate a railroad-review round in one station-merge barrier agent — verdict intake over the script\'s pre-mapped drafts, ledger, reconciliation, routes, DoD, and the round\'s review.json handoff',
  whenToUse: 'Invoked by the /railroad-review dispatcher at the Station-barrier spawn (SKILL ROUND-009a), after the verify stage returns',
  phases: [
    { title: 'Station merge', detail: 'single barrier agent: verdict intake, ledger, reconciliation, routes, DoD, review.json handoff' },
  ],
}

// GEN:schemas finding,verdict,consolidation,station-return
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
const CONSOLIDATION_SCHEMA = {
  "type": "object",
  "required": [
    "dispositions",
    "ranked_plan",
    "unverified",
    "funnel",
    "permanent_checks",
    "build_test",
    "base_health",
    "debunked",
    "resolved",
    "intentional_deviations",
    "coverage_gaps",
    "escalations",
    "definition_of_done",
    "recommendation",
    "handoff_json",
    "probes",
    "probes_dir"
  ],
  "properties": {
    "dispositions": {
      "type": "array",
      "description": "every claim and every duplicate, each with exactly one disposition — nothing may lack one",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "disposition"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "disposition": {
            "type": "string",
            "enum": [
              "confirmed",
              "unverified",
              "rejected-with-reason",
              "duplicate-of",
              "debunked"
            ],
            "description": "confirmed (second confirmation passed) | unverified (survived refutation but not demonstrable here — kept, flagged) | rejected-with-reason | duplicate-of | debunked (refuted)"
          },
          "reason": {
            "type": "string",
            "description": "required for rejected-with-reason, debunked, unverified (the concrete missing precondition), and for any disposition that overrides the handed-in verdict"
          },
          "duplicate_of": {
            "type": "string",
            "description": "the surviving finding_id, required for duplicate-of"
          },
          "probe": {
            "type": "string",
            "description": "the verdict's probe field carried over: a probe rule id, \"none: <why>\", or \"manual: <kind>\""
          },
          "evidence": {
            "type": "string",
            "description": "the verdict's decisive line — probe run or first evidence read"
          }
        },
        "if": {
          "required": [
            "disposition"
          ],
          "properties": {
            "disposition": {
              "enum": [
                "confirmed",
                "debunked"
              ]
            }
          }
        },
        "then": {
          "required": [
            "evidence"
          ]
        }
      }
    },
    "ranked_plan": {
      "type": "array",
      "description": "the confirmed findings as one ranked fix plan, each with a route; severity and its effect/exposure cells carried verbatim",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "severity",
          "effect",
          "exposure",
          "file",
          "line",
          "fix",
          "route",
          "route_reason"
        ],
        "properties": {
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
      }
    },
    "unverified": {
      "type": "array",
      "description": "findings kept but not demonstrable in this environment — each with the concrete missing precondition, never a bare \"not demonstrated\"",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "reason"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "reason": {
            "type": "string"
          }
        }
      }
    },
    "funnel": {
      "type": "object",
      "description": "the claim funnel across the round — stated as one line in the gate presentation",
      "required": [
        "produced",
        "confirmed",
        "unverified",
        "duplicates",
        "rejected",
        "debunked"
      ],
      "properties": {
        "produced": {
          "type": "integer"
        },
        "confirmed": {
          "type": "integer"
        },
        "unverified": {
          "type": "integer"
        },
        "duplicates": {
          "type": "integer"
        },
        "rejected": {
          "type": "integer"
        },
        "debunked": {
          "type": "integer"
        }
      }
    },
    "permanent_checks": {
      "type": "array",
      "description": "for each confirmed finding that is a recurring class rather than a one-off: the durable check that would catch the class (ACDSL rule, lint rule, assertion, golden file, test) — the dispatcher offers to add them, never adds unasked; empty when every finding is a one-off",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "check"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "check": {
            "type": "string"
          }
        }
      }
    },
    "build_test": {
      "type": "object",
      "description": "the rules tool's buildtest result at the review head, copied through verbatim",
      "required": [
        "build",
        "tests"
      ],
      "properties": {
        "build": {
          "type": "string"
        },
        "tests": {
          "type": "string"
        }
      }
    },
    "base_health": {
      "type": "object",
      "description": "the rules tool's buildtest result at the review base, copied through verbatim",
      "required": [
        "build",
        "tests"
      ],
      "properties": {
        "build": {
          "type": "string"
        },
        "tests": {
          "type": "string"
        }
      }
    },
    "debunked": {
      "type": "array",
      "description": "findings refuted by their second confirmation",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "reason"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "reason": {
            "type": "string"
          }
        }
      }
    },
    "resolved": {
      "type": "array",
      "description": "findings already fixed out of band — finding to fixing-commit provenance",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "fixing_commit"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "fixing_commit": {
            "type": "string",
            "description": "the commit hash that fixed it"
          }
        }
      }
    },
    "intentional_deviations": {
      "type": "array",
      "description": "deviations to canonicalize rather than fix",
      "items": {
        "type": "object",
        "required": [
          "claim",
          "canonicalize_as"
        ],
        "properties": {
          "claim": {
            "type": "string"
          },
          "canonicalize_as": {
            "type": "string"
          }
        }
      }
    },
    "coverage_gaps": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "manifest paths of lanes that did not finish — their files are the uncovered assignments"
    },
    "definition_of_done": {
      "type": "array",
      "description": "the DoD walk — happens here in consolidation, it is not a direction",
      "items": {
        "type": "object",
        "required": [
          "criterion",
          "status"
        ],
        "properties": {
          "criterion": {
            "type": "string"
          },
          "status": {
            "type": "string",
            "description": "PASS | FAIL | N/A"
          },
          "notes": {
            "type": "string"
          }
        }
      }
    },
    "escalations": {
      "type": "array",
      "description": "suspected but undemonstrated defects in migrations, auth, concurrency, money, a public API contract, or persisted data — surfaced FIRST in every presentation of the round, never one unverified line among many; empty when none",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "why"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "why": {
            "type": "string"
          }
        }
      }
    },
    "recommendation": {
      "type": "string",
      "enum": [
        "APPROVE",
        "CONDITIONAL",
        "REJECT"
      ],
      "description": "APPROVE (no blocking finding — new, MINOR or worse, above ignore-level; NIT and INFO never block) | CONDITIONAL (blocking findings, mergeable once fixed) | REJECT (fundamental rework)"
    },
    "handoff_json": {
      "type": "string",
      "description": "absolute path of the round's consolidated review.json — the single handoff artifact; no review.md exists"
    },
    "probes": {
      "type": "array",
      "description": "one entry per probe the refuters and verifiers authored — THE probe→fixture→verdict index, mapped from the verdicts' probe/script/fixture fields; there is no sidecar verdicts file. Empty in non-ACDSL repos or when no probes were authored",
      "items": {
        "type": "object",
        "required": [
          "id",
          "script",
          "fixture",
          "registry",
          "verdict",
          "finding_id"
        ],
        "properties": {
          "id": {
            "type": "string",
            "description": "the probe rule id (e.g. PROBE-R4-03)"
          },
          "script": {
            "type": "string",
            "description": "the probe executable, path relative to probes_dir"
          },
          "fixture": {
            "type": "string",
            "description": "the minimal fail fixture the probe went RED on in the validity check, path relative to probes_dir"
          },
          "registry": {
            "type": "string",
            "description": "the probe's merged registry, path relative to probes_dir"
          },
          "verdict": {
            "type": "string",
            "description": "the real-tree result: \"RED on <file:line>\" | \"GREEN\""
          },
          "finding_id": {
            "type": "string",
            "description": "the finding this probe settled"
          }
        }
      }
    },
    "probes_dir": {
      "type": "string",
      "description": "absolute path of round-{r}/probes/ when any probe was authored — empty string otherwise"
    }
  }
}
const STATION_RETURN_SCHEMA = {
  "type": "object",
  "required": [
    "handoff_json",
    "recommendation",
    "funnel",
    "disposition_counts",
    "ranked"
  ],
  "properties": {
    "handoff_json": {
      "type": "string",
      "description": "absolute path of the round's review.json — the single full copy of the station's product"
    },
    "recommendation": {
      "type": "string",
      "enum": [
        "APPROVE",
        "CONDITIONAL",
        "REJECT"
      ]
    },
    "funnel": {
      "type": "object",
      "required": [
        "produced",
        "confirmed",
        "unverified",
        "duplicates",
        "rejected",
        "debunked"
      ],
      "properties": {
        "produced": {
          "type": "integer"
        },
        "confirmed": {
          "type": "integer"
        },
        "unverified": {
          "type": "integer"
        },
        "duplicates": {
          "type": "integer"
        },
        "rejected": {
          "type": "integer"
        },
        "debunked": {
          "type": "integer"
        }
      }
    },
    "disposition_counts": {
      "type": "object",
      "description": "the literal counts of review.json's dispositions array, drafts included",
      "required": [
        "confirmed",
        "unverified",
        "rejected",
        "debunked",
        "duplicate_of"
      ],
      "properties": {
        "confirmed": {
          "type": "integer"
        },
        "unverified": {
          "type": "integer"
        },
        "rejected": {
          "type": "integer"
        },
        "debunked": {
          "type": "integer"
        },
        "duplicate_of": {
          "type": "integer"
        }
      }
    },
    "ranked": {
      "type": "array",
      "description": "one ref per review.json ranked_plan entry, in order — the script derives ranked_count and the blocking count from it",
      "items": {
        "type": "object",
        "required": [
          "finding_id",
          "severity",
          "known"
        ],
        "properties": {
          "finding_id": {
            "type": "string"
          },
          "severity": {
            "type": "string",
            "enum": [
              "BLOCKER",
              "MAJOR",
              "MINOR",
              "NIT",
              "INFO"
            ]
          },
          "known": {
            "type": "boolean"
          }
        }
      }
    },
    "pin_error": {
      "type": "string",
      "description": "the git output of a failed FIRST ACTION pin (checkout or base ancestor check); absent or empty when the pin held — when set, no review.json exists and every other field is a placeholder"
    }
  }
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

// SKILL: SKILL-RAILROADREVIEW-ROUND-009a (Station barrier) and its substeps
if (!Array.isArray(args.claims)) {
  throw new Error('args.claims is required — cross-deduped claims, each carrying its verdict or null, assembled by the dispatcher via targeted jq')
}
if (!Array.isArray(args.duplicates)) {
  throw new Error('args.duplicates is required — the dedup stage\'s cross-direction duplicate pairs')
}
if (!args.buildTest || typeof args.buildTest !== 'object') {
  throw new Error('args.buildTest is required — {build, tests} from $SKILL_RAILROADREVIEW_BUILDTEST buildtest at the review head, forwarded by round.js')
}
if (!args.baseHealth || typeof args.baseHealth !== 'object') {
  throw new Error('args.baseHealth is required — {build, tests} from $SKILL_RAILROADREVIEW_BUILDTEST buildtest at the review base, forwarded by round.js')
}
if (!Array.isArray(args.perDirection)) {
  throw new Error('args.perDirection is required — [{direction, lane_artifacts, notes, claims_produced, coverage_gaps}] from the lanes stage')
}
const abortedManifests = Array.isArray(args.abortedLanes) ? args.abortedLanes.map(l => l.manifest).filter(Boolean) : []
const corrections = Array.isArray(args.corrections) ? args.corrections : []
// D1: a NEW ranked entry blocks when it is MINOR or worse and above ignore-level — NIT and INFO never do.
const { mode: ignoreMode, rank: ignoreRank } = levelRank('ignoreLevel', 'none', 0)
const isBlocking = ref => !ref.known && !NON_BLOCKING.includes(ref.severity) && (SEVERITY_RANK[ref.severity] || 0) > ignoreRank
const blockingOf = summary => (Array.isArray(summary.ranked) ? summary.ranked : []).filter(isBlocking).length

const stationOpts = { label: 'station', phase: 'Station merge', agentType: 'railroad-station', ...isoOpts, effort: 'high' }
stationOpts.model = assertModel('stationModel', args.stationModel || DEFAULT_SUBAGENT_MODEL)
if (typeof args.stationEffort === 'string' && args.stationEffort) stationOpts.effort = args.stationEffort

const claims = args.claims
const duplicates = args.duplicates
const buildTest = args.buildTest
const baseHealth = args.baseHealth
const coverageGaps = [...new Set([...args.perDirection.flatMap(fd => fd.coverage_gaps || []), ...abortedManifests])]
const claimsProduced = args.perDirection.reduce((n, fd) => n + (fd.claims_produced || 0), 0)
// Within-direction merges as dedup counted them — never the produced-minus-union
// arithmetic, where a lost direction's claims would masquerade as duplicates (ROUND-020).
if (!Number.isInteger(args.withinDuplicates)) {
  throw new Error('args.withinDuplicates is required — round.js passes the dedup count')
}
const grouperMerged = args.withinDuplicates
// Station task split (SKILL ROUND-009b): the mechanical merge is pre-mapped here —
// binding verdicts and cross-direction duplicates arrive as draft dispositions, the
// probe->fixture->verdict index as draft_probes; the station judges exceptions only.
// A draft's evidence is the verdict's decisive line: its probe run, else its first read.
const EVIDENCE_MAX = 600
const decisiveEvidence = verdict => {
  const evidence = verdict.evidence
  if (evidence && typeof evidence === 'object' && typeof evidence.probe_run === 'string' && evidence.probe_run) return evidence.probe_run.slice(0, EVIDENCE_MAX)
  if (evidence && typeof evidence === 'object' && Array.isArray(evidence.read) && evidence.read.length) return String(evidence.read[0]).slice(0, EVIDENCE_MAX)
  if (typeof evidence === 'string') return evidence.slice(0, EVIDENCE_MAX)
  return ''
}
const draftDispositions = [
  ...claims.filter(c => c.verdict).map(c => ({
    finding_id: c.finding_id,
    disposition: c.verdict.verdict,
    reason: c.verdict.reason || '',
    probe: c.verdict.probe || '',
    evidence: decisiveEvidence(c.verdict),
  })),
  ...duplicates.map(d => ({ finding_id: d.finding_id, disposition: 'duplicate-of', duplicate_of: d.duplicate_of })),
]
const draftProbes = claims
  .filter(c => c.verdict && c.verdict.probe && !/^(none:|manual:)/.test(c.verdict.probe))
  .map(c => ({
    id: c.verdict.probe,
    script: c.verdict.script || '',
    fixture: c.verdict.fixture || '',
    registry: c.verdict.registry || '',
    verdict: c.verdict.verdict === 'confirmed' ? `RED on ${c.file}:${c.line}` : 'GREEN',
    finding_id: c.finding_id,
  }))
const stationInput = {
  claims,
  cross_direction_duplicates: duplicates,
  draft_dispositions: draftDispositions,
  draft_probes: draftProbes,
  funnel_inputs: { produced: claimsProduced, grouper_merged_within_directions: grouperMerged, cross_direction_duplicates: duplicates.length, severity_corrections: corrections },
  coverage_gaps: coverageGaps,
  per_direction: args.perDirection.map(fd => ({ direction: fd.direction, lane_artifacts: fd.lane_artifacts, notes: fd.notes })),
  build_test: buildTest,
  base_health: baseHealth,
}

phase('Station merge')
const stationSlug = `station-r${r}`
const stationBranchBase = `${branchNs}station-r${r}-${hash6}`
const stationPrompt = branch =>
  `You are the station-merge barrier of a railroad-review on ${args.base}...${args.head} (round ${r}). ` +
  (explicitWt
    ? `You run in your own isolated git worktree; your FIRST ACTION pins it at ${args.headCommit} — every re-read is at that commit.\n` +
      wtLine(stationSlug, branch) + `\n`
    : `You run in your own isolated git worktree, forked from the dispatcher's checkout after the fan-out; your FIRST ACTION ` +
      `pins it at ${args.headCommit} — every re-read is at that commit.\n\n`) +
  `FIRST ACTION: ${pinStep(branch, 'stop — write no review.json; return pin_error = the git output, handoff_json "", ' +
    'recommendation "REJECT" and every count 0')}.\n\n` +
  `The round's consolidated input follows: the lanes' once-confirmed claims, already deduped within AND across directions ` +
  `by grouper agents (merged_from lists absorbed ids; cross_direction_duplicates lists the cross-direction pairs), each ` +
  `claim carrying its SECOND-CONFIRMATION verdict from a fresh refuter or verifier agent (verdict: confirmed | debunked | ` +
  `unverified with reason, probe, script, fixture, verdict_path — evidence and artifacts in that file) — or verdict null when the refuter or verifier ` +
  `was lost. Also: the funnel ` +
  `inputs, the script-unioned coverage gaps (manifest paths of lanes that did not finish — their files are the uncovered ` +
  `assignments; lane reading is read-gate-enforced), per-direction artifact paths, the build-and-test ` +
  `result at the head (build_test) and at the review base (base_health). Read the lane artifacts (lane_artifacts paths) when a claim needs its full write-up.\n\n` +
  `STATION INPUT — the consolidated input object described above, assembled by the script; work from it as given:\n` +
  `${JSON.stringify(stationInput, null, 2)}\n\n` +
  `CLAIM BODIES BY PATH: every claim in the input is a ref — read its write-up with ` +
  `\`jq -c '.findings[] | select(.finding_id == "<artifact_id>")' <artifact>\` and its full verdict (evidence, artifacts) from ` +
  `verdict_path when a disposition needs them; ranked_plan entries carry claim, fix, cells, file, line, artifact and artifact_id ` +
  `from the claim ref and its artifact — never the evidence (it stays in the artifact, one jq away).\n\n` +
  ledgerBlock +
  knownBlock +
  shellBlock +
  `\nConsolidate (the trains arrive at the station):\n` +
  `0. FINDING IDENTITY IS IMMUTABLE: every finding_id in your dispositions and ranked_plan is echoed VERBATIM from ` +
  `the input claims and duplicates — never re-mint, renumber, or change an id, and never change its direction ` +
  `prefix: a finding's direction is fixed by the lane that produced it, id prefix and direction field alike. ` +
  `An id you cannot find in the input is a defect in your own output, not a finding to rename.\n` +
  `1. Any claim matching an entry of the rejection ledger (the LEDGER block names its path; no block = no ledger) gets ` +
  `disposition "rejected-with-reason" citing the ledger — and note the lane that re-reported it. Any claim matching a ` +
  `KNOWN-FINDINGS entry (match by file and claim substance) gets disposition "confirmed" with known=true and known_id = ` +
  `that ledger row's finding_id on its ranked_plan entry — carried, never blocking, never routed for a fix this round.\n` +
  `2. Verdicts are BINDING and draft_dispositions PRE-MAP every one of them — echo those entries into your dispositions ` +
  `verbatim, evidence included. Adjust ONLY the exceptions: a verdict internally inconsistent with the code in front of you — re-read the cited ` +
  `code, decide, and state the override in the disposition reason. A claim with verdict null (lost refuter or verifier, ` +
  `absent from the drafts) you second-confirm ` +
  `yourself by re-reading its cited code adversarially — attempt to REFUTE it; "could not reproduce" is a successful result, ` +
  `never manufacture evidence; not demonstrable here -> "unverified" with the CONCRETE missing precondition, never a bare "not demonstrated". ` +
  `Every confirmed or debunked disposition carries evidence: the draft's, or your own decisive line when you settled it.\n` +
  `3. The draft duplicate-of entries cover cross_direction_duplicates — echo them. Every claim and every duplicate ` +
  `has exactly one disposition: confirmed | unverified | rejected-with-reason | duplicate-of(id) | debunked. Nothing may lack one.\n` +
  `4. SEVERITY ARRIVES FIXED: each claim's severity is the severity-table row for its effect/exposure cells ` +
  `(recomputed at dedup, raised at most once by dedup to the group maximum, lowered at most once by its refuter's cells) ` +
  `and is frozen at second confirmation — carry it and its cells verbatim, never re-grade. Resolve spec-vs-code conflicts against decisions.md — a spec-explicit ` +
  `choice is not a code bug. Separate real bugs from intentional deviations to canonicalize (intentional_deviations). A claim ` +
  `already fixed by a specific commit goes to resolved[] with the hash.\n` +
  `5. Emit one ranked fix plan over the confirmed findings, ordered BLOCKER / MAJOR / MINOR / NIT / INFO — each entry's ` +
  `severity carried verbatim per step 4. Each entry's fix carries over the lane's proposed fix — refine it against ${args.headCommit}, ` +
  `never invent one from scratch. Assign each entry a route: human (needs author judgment — surfaced at the gate) | ` +
  `auto-fix (mechanical — applied by the fix-apply stage), and a route_reason: one sentence naming the judgment a human ` +
  `must make, or why the fix is mechanical. ` +
  `A fix whose change touches Makefile, makefile, GNUmakefile or a *.mk file routes human — make sources are ` +
  `write-protected in headless runs. Every confirmed, non-known finding is part of the fix plan — the ledger marks ` +
  `entries at or below ignore-level as ignored after the gate; you rank them like any other.\n` +
  `6. Fill permanent_checks: for each confirmed finding that is a RECURRING CLASS rather than a one-off, name the durable ` +
  `check that would catch the class (ACDSL rule, lint rule, assertion, golden file, test). One-offs get no entry.\n` +
  `7. Copy build_test through verbatim. Copy base_health through verbatim into review.json's top-level base_health. Carry coverage_gaps through — the assignments of unfinished lanes; an ` +
  `uncovered diff file is a review defect worth surfacing.\n` +
  `8. Fill the funnel MECHANICALLY: produced = funnel_inputs.produced; duplicates = grouper_merged_within_directions + the ` +
  `count of ALL duplicate-of dispositions (the drafted ` +
  `cross-direction ones included); confirmed / unverified / rejected = the literal counts of those dispositions in ` +
  `your dispositions array — count the array, never estimate; debunked = the count of debunked dispositions. ` +
  `The dispatcher recounts; a mismatch is a station defect that re-runs this merge.\n` +
  `9. probes[] = draft_probes echoed verbatim (each with its registry), plus one entry per probe your own step-2 self-verification authored; ` +
  `probes_dir = ${roundDir}/probes when any entry exists, else "".\n` +
  `10. Walk the Definition of Done (per the skill: the [DoD]-marked context entries) and mark each criterion PASS/FAIL/N/A — DoD is not a direction.\n` +
  `11. Recommendation: a confirmed finding is BLOCKING when it is not known, its severity is MINOR or worse and above ` +
  `IGNORE-LEVEL "${ignoreMode}" — a NIT or INFO never blocks and never makes the round CONDITIONAL. APPROVE when no ` +
  `blocking finding exists, build_test is not red and coverage_gaps is empty; CONDITIONAL when blocking findings exist and ` +
  `are fixable as named, or when only a red build or a coverage gap ` +
  `stands in the way; REJECT for fundamental rework. A red base — base_health build or tests starting with "failed:" — ` +
  `forces REJECT; "skipped:" never forces.\n` +
  `12. Build ${roundDir}/review.json AS YOU GO — the ONLY handoff artifact, no review.md on any route: Write its skeleton ` +
  `(every top-level field, arrays empty) before step 1, Edit-append each disposition and each ranked_plan entry the moment ` +
  `you decide it, and fill funnel, recommendation and escalations in the final Edit; never one final dump. review.json ` +
  `conforms to this JSON schema: ${JSON.stringify(CONSOLIDATION_SCHEMA)}. Return ONLY the short summary schema — ` +
  `handoff_json, recommendation, funnel, disposition_counts and ranked (one {finding_id, severity, known} per ranked_plan ` +
  `entry, in order) — never the file's contents. ` +
  `Fill escalations[]: every suspected but undemonstrated defect in migrations, ` +
  `auth, concurrency, money, a public API contract, or persisted data — these lead every presentation of the round, ` +
  `never one unverified line among many; empty when none.\n` +
  `13. Any commit messages you propose carry NO rule IDs and NO Claude attribution.\n` +
  `\nReview ONLY — never edit a tracked file; your only write is the review.json handoff under ${roundDir}/. No tree-mutating ` +
  `git commands (reset/clean/stash/revert; checkout only in the FIRST ACTION pin). Return the structured output only.`

// The script checks the station's summary against its own inputs; a violation
// re-runs the station once to fix review.json in place — review.json stays the
// one full copy, the gate reads it. A failed pin left no review.json to fix: the
// round fails on it without a retry.
const expectedDispositions = claims.length + duplicates.length
const redBuild = /^failed:/.test(buildTest.build) || /^failed:/.test(buildTest.tests)
const redBase = /^failed:/.test(baseHealth.build) || /^failed:/.test(baseHealth.tests)
const stationDefects = summary => {
  if (!summary) return ['the station returned nothing']
  if (summary.pin_error) return [`station pin to ${args.headCommit} failed — ${summary.pin_error}`]
  const counts = summary.disposition_counts
  const covered = counts.confirmed + counts.unverified + counts.rejected + counts.debunked + counts.duplicate_of
  const funnel = {
    produced: claimsProduced, confirmed: counts.confirmed, unverified: counts.unverified,
    duplicates: grouperMerged + counts.duplicate_of, rejected: counts.rejected, debunked: counts.debunked,
  }
  const wrongKeys = Object.keys(funnel).filter(key => funnel[key] !== summary.funnel[key])
  return [
    ...(covered !== expectedDispositions ? [`dispositions cover ${covered} ids; the input holds ${expectedDispositions} claims and duplicates — each needs exactly one`] : []),
    ...(wrongKeys.length ? [`funnel must read ${JSON.stringify(funnel)} (wrong: ${wrongKeys.join(', ')})`] : []),
    ...(redBuild && summary.recommendation === 'APPROVE' ? ['build or tests are red — recommendation must be CONDITIONAL or REJECT'] : []),
    ...(redBase && summary.recommendation !== 'REJECT' ? ['base build or tests are red — recommendation must be REJECT'] : []),
    ...(coverageGaps.length && summary.recommendation === 'APPROVE'
      ? [`${coverageGaps.length} lane assignment(s) went unreviewed (coverage_gaps) — recommendation must be CONDITIONAL or REJECT`]
      : []),
    ...(summary.recommendation === 'APPROVE' && blockingOf(summary) > 0
      ? [`${blockingOf(summary)} blocking finding(s) stand (new, MINOR or worse, above ignore-level) — recommendation must be CONDITIONAL or REJECT`]
      : []),
  ]
}
// Every name handed out — a re-run station on the harness route takes the -a2 name.
const stationBranches = []
const runStation = (attempt, correction) => {
  const branch = attemptBranch(stationBranchBase, attempt)
  if (!stationBranches.includes(branch)) stationBranches.push(branch)
  return agent(stationPrompt(branch) + correction, { ...stationOpts, schema: STATION_RETURN_SCHEMA })
}
const isPinFailure = summary => !!(summary && summary.pin_error)
let summary = await runStation(1, '')
let defects = stationDefects(summary)
if (defects.length && !isPinFailure(summary)) {
  log(`station defects, re-running the station once: ${defects.join(' | ')}`)
  summary = await runStation(2, `\n\nCORRECTION — ${roundDir}/review.json exists from your previous run: fix it IN PLACE with Edit, never rewrite it whole, then return the corrected summary:\n- ${defects.join('\n- ')}\n`)
  defects = stationDefects(summary)
}

return {
  round: r,
  handoff: summary && !defects.length ? { json: summary.handoff_json } : null,
  defects,
  recommendation: summary && !isPinFailure(summary) ? summary.recommendation : null,
  blocking: summary ? blockingOf(summary) : 0,
  rankedCount: summary && Array.isArray(summary.ranked) ? summary.ranked.length : 0,
  funnel: summary && !isPinFailure(summary) ? summary.funnel : null,
  corrections,
  branches: stationBranches,
}
