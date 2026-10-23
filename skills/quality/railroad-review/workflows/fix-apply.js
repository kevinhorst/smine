export const meta = {
  name: 'railroad-review-fix-apply',
  description: 'The railroad-review fix-apply stage: one fixer agent applies the auto-fix set of a round and commits it per package through package-commit',
  whenToUse: 'Invoked by /railroad-review at its Fix application spawn (STATION-005), after the ledger split',
  phases: [
    { title: 'Fix' },
  ],
}

// GEN:schemas fix-return
const FIX_RETURN_SCHEMA = {
  "type": "object",
  "required": [
    "applied",
    "failed",
    "base",
    "commits"
  ],
  "properties": {
    "applied": {
      "type": "array",
      "description": "finding ids for which a change landed in a commit",
      "items": {
        "type": "string"
      }
    },
    "failed": {
      "type": "array",
      "description": "finding ids for which no change could be made — a denied write, code that no longer exists — each with the reason",
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
    "base": {
      "type": "string",
      "description": "the full 40-character SHA of HEAD before the first fix commit (git rev-parse HEAD after applying, before committing); empty when nothing was applied"
    },
    "commits": {
      "type": "array",
      "description": "the full 40-character SHAs of the fix commits package-commit made, oldest first; empty when nothing was applied",
      "items": {
        "type": "string"
      }
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /railroad-review passes {reviewPath, round, ids, repo}')
}
for (const key of ['reviewPath', 'repo']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — resolved by the dispatcher, never assumed`)
  }
}
if (!Number.isInteger(args.round) || args.round <= 0) {
  throw new Error('args.round is required — the station-protocol round number (1-based)')
}
if (!Array.isArray(args.ids) || !args.ids.length || args.ids.some(id => typeof id !== 'string' || !id)) {
  throw new Error('args.ids is required — the autofix set $SKILL_RAILROADREVIEW_LEDGER ledger printed, whole')
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
const opts = { agentType: 'railroad-fixer', schema: FIX_RETURN_SCHEMA, label: `fix:r${args.round}` }
opts.model = assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL)

const prompt =
  `APPLY the fixes of these findings in the repository at ${args.repo}: ${args.ids.join(', ')}.\n` +
  `READ each entry from the ranked_plan of ${args.reviewPath} (jq by finding_id): its file, line, claim and fix. ` +
  `ATTEMPT every entry. Implement its fix as proposed; where the proposed fix does not fit the checked-out branch as it ` +
  `is now, make the smallest change that removes the defect the claim names. Nothing beyond that: no new mechanism, ` +
  `no fallback, no weakened guard or test. An entry is failed only when no change can be made — a denied write, code ` +
  `that no longer exists — reported with the reason.\n` +
  `Then RECORD \`git -C ${args.repo} rev-parse HEAD\` as base, and COMMIT the applied fixes by invoking the Skill tool with ` +
  `skill="package-commit" — it groups the changed files per package and writes one rule-conforming commit per group; ` +
  `nothing applied means no commit and an empty base.\n` +
  `RETURN {"applied", "failed", "base", "commits"} — the applied ids, the failed ids with reasons, base as recorded, and ` +
  `commits as the full 40-character SHAs of every commit package-commit made, oldest first ` +
  `(\`git -C ${args.repo} rev-list --reverse <base>..HEAD\` right after it finishes), or [] when nothing was applied.`

phase('Fix')
const result = await agent(prompt, opts)
const accounted = new Set([...result.applied, ...result.failed.map(f => f.finding_id)])
const missing = args.ids.filter(id => !accounted.has(id))
if (missing.length) {
  throw new Error(`fixer left finding ids unaccounted: ${missing.join(', ')}`)
}
// The fix range bounds every later fix step: base is the pre-fix head, the last commit the post-fix head.
const isSha = value => /^[0-9a-f]{40}$/.test(value)
const commits = Array.isArray(result.commits) ? result.commits : []
if (result.applied.length && (!isSha(result.base) || !commits.length || commits.some(c => !isSha(c)))) {
  throw new Error(`fixer applied fixes without a full-SHA base and commit list (got base "${result.base}", commits ${JSON.stringify(commits)})`)
}
if (!result.applied.length && commits.length) {
  throw new Error(`fixer reported commits ${commits.join(', ')} without an applied fix`)
}
const isApplied = result.applied.length > 0
return {
  applied: result.applied,
  failed: result.failed,
  preFixHead: isApplied ? result.base : null,
  fixHead: isApplied ? commits[commits.length - 1] : null,
  fixCommits: isApplied ? commits : [],
}
