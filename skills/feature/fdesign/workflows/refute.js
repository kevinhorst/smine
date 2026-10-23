export const meta = {
  name: 'fdesign-refute',
  description: 'Refute an fdesign plan: two blind refuters over its Assumptions rows, a tie-break agent for the split ones',
  whenToUse: 'Invoked by /fdesign outside plan mode at its refute spawn, once per run; the planner confirms and folds the verdicts itself',
  phases: [
    { title: 'Refute' },
  ],
}

// GEN:schemas claim-verdicts
const CLAIM_VERDICTS_SCHEMA = {
  "type": "object",
  "required": [
    "verdicts",
    "verdict_path"
  ],
  "properties": {
    "verdicts": {
      "type": "array",
      "description": "one verdict per claim of the batch, every claim_id of the batch exactly once",
      "items": {
        "type": "object",
        "required": [
          "claim_id",
          "verdict",
          "reason",
          "evidence"
        ],
        "properties": {
          "claim_id": {
            "type": "string"
          },
          "verdict": {
            "type": "string",
            "enum": [
              "confirmed",
              "debunked",
              "unverified"
            ],
            "description": "confirmed (survived refutation, evidence in hand) | debunked (refuted) | unverified (not settleable from the repo)"
          },
          "reason": {
            "type": "string",
            "description": "one line: debunked — what refuted it; unverified — the concrete missing precondition; confirmed — the decisive evidence"
          },
          "evidence": {
            "type": "string",
            "description": "path:line, or the command run plus its output"
          }
        }
      }
    },
    "verdict_path": {
      "type": "string",
      "description": "absolute path of the batch verdict file written"
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /fdesign assembles {planPath, verdictDir, claims} at its refute spawn')
}
for (const key of ['planPath', 'verdictDir']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required`)
  }
}
const claims = Array.isArray(args.claims) ? args.claims : []
if (!claims.length) {
  throw new Error('nothing to refute — pass the ledger claims')
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

// One agent judges up to 50 claims: the round is two blind refuters, plus one
// tie-break agent when they split — never a second round.
const CLAIM_BATCH = 50

const VERDICTS = ['confirmed', 'debunked', 'unverified']
const preamble = 'Read AGENTS.md and the context docs it names first.\n'

const chunk = (items, size) => Array.from({ length: Math.ceil(items.length / size) }, (_, index) => items.slice(index * size, (index + 1) * size))

const claimBlock = claim => `- ${claim.claim_id} (rests on: ${claim.source}): ${claim.text}` +
  (claim.evidence ? `\n  evidence the planner cites: ${claim.evidence}` : '')

const judgeBrief = (batch, path) => preamble +
  `Feature plan: ${args.planPath}. Claims — each is an Assumptions row of the plan, its "rests on" names the decisions built on it:\n${batch.map(claimBlock).join('\n')}\n` +
  'REFUTE each claim against the repository and the plan: check the cited evidence yourself and look for what contradicts it ' +
  '— a caller, a grant, a config, a test, a doc. Could-not-refute is a valid result; never manufacture evidence. ' +
  'Judge only the claims listed; add none.\n' +
  'Verdict per claim: debunked — only when the corrected reality changes what the claim rests on; confirmed — it survived, ' +
  'or the correction is a wording slip or a missed extra occurrence that changes no decision (put the correction in reason); ' +
  'unverified — the repository cannot settle it, the reason names the concrete missing precondition.\n' +
  `Write every verdict — claim, verdict, reason, evidence, every command you ran with its output — as one JSON file to ${path}, ` +
  `then return one verdict per claim (each claim_id exactly once) with that path. Change no other file.`

// judge returns a claim_id → verdict object for one batch (a plain object: results are
// serialized for resume); a missing agent or claim is simply absent.
const judge = (batch, role, index) => {
  const path = `${args.verdictDir}/${role}-${index + 1}.json`
  return agent(judgeBrief(batch, path), {
    label: `${role} ${index + 1} (${batch.length} claims)`,
    phase: 'Refute',
    schema: CLAIM_VERDICTS_SCHEMA,
    model: DEFAULT_SUBAGENT_MODEL,
  }).then(result => Object.fromEntries((result ? result.verdicts : [])
    .filter(verdict => batch.some(claim => claim.claim_id === verdict.claim_id))
    .map(verdict => [verdict.claim_id, { ...verdict, verdict_path: result.verdict_path }])))
}

const majorityOf = (claim, votes) => {
  const winner = VERDICTS.find(verdict => votes.filter(vote => vote.verdict === verdict).length >= 2)
  if (winner) {
    return { ...votes.find(vote => vote.verdict === winner), claim_id: claim.claim_id, votes: votes.length }
  }
  return {
    claim_id: claim.claim_id,
    verdict: 'unverified',
    reason: 'no two of the agents agreed',
    evidence: votes.map(vote => `${vote.verdict}: ${vote.reason}`).join(' | '),
    verdict_path: votes.map(vote => vote.verdict_path).join(', '),
    votes: votes.length,
  }
}

const batches = chunk(claims, CLAIM_BATCH)
const blind = await parallel(batches.flatMap((batch, index) => [
  () => judge(batch, 'refuter-a', index),
  () => judge(batch, 'refuter-b', index),
]))
const votesOf = claim => blind.filter(Boolean).map(votes => votes[claim.claim_id]).filter(Boolean)
const agreed = claims.filter(claim => {
  const votes = votesOf(claim)
  return votes.length === 2 && votes[0].verdict === votes[1].verdict
})
const split = claims.filter(claim => !agreed.includes(claim))
const tiebreaks = await parallel(chunk(split, CLAIM_BATCH).map((batch, index) => () => judge(batch, 'tiebreak', index)))
const tieVotes = claim => tiebreaks.filter(Boolean).map(votes => votes[claim.claim_id]).filter(Boolean)
const verdicts = [
  ...agreed.map(claim => ({ ...votesOf(claim)[0], claim_id: claim.claim_id, votes: 2 })),
  ...split.map(claim => majorityOf(claim, [...votesOf(claim), ...tieVotes(claim)])).filter(verdict => verdict.votes > 0),
]

const settledIds = new Set(verdicts.map(verdict => verdict.claim_id))
const lost = claims.filter(claim => !settledIds.has(claim.claim_id)).map(claim => claim.claim_id)
if (lost.length) {
  log(`no verdict returned for: ${lost.join(', ')} — treated as unverified`)
}

const idsWith = verdict => verdicts.filter(entry => entry.verdict === verdict).map(entry => entry.claim_id)
return {
  verdicts,
  debunked: idsWith('debunked'),
  unverified: [...idsWith('unverified'), ...lost],
}
