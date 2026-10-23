export const meta = {
  name: 'railroad-review-fix-check',
  description: 'The railroad-review fix-check stage: one fresh agent judges per applied finding whether its defect is gone at the fix commit',
  whenToUse: 'Invoked by /railroad-review at its Fix-diff check spawn (STATION-011c), before the fix-state fixed ledger call',
  phases: [
    { title: 'Fix check' },
  ],
}

// GEN:schemas fix-check
const FIX_CHECK_SCHEMA = {
  "type": "object",
  "required": [
    "fixed",
    "hits"
  ],
  "properties": {
    "fixed": {
      "type": "array",
      "description": "finding ids whose defect no longer exists at the post-fix head, by a change that adds nothing the claim does not need",
      "items": {
        "type": "string"
      }
    },
    "hits": {
      "type": "array",
      "description": "what is not fixed: a defect still present, a new mechanism or fallback, a weakened guard or test, or a changed file no entry accounts for",
      "items": {
        "type": "object",
        "required": [
          "file",
          "why"
        ],
        "properties": {
          "finding_id": {
            "type": "string",
            "description": "the finding the hit belongs to; absent for a change no entry accounts for"
          },
          "file": {
            "type": "string"
          },
          "why": {
            "type": "string"
          }
        }
      }
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /railroad-review passes {reviewPath, repo, preFixHead, postFixHead, ids}')
}
for (const key of ['reviewPath', 'repo', 'preFixHead', 'postFixHead']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — resolved by the dispatcher, never assumed`)
  }
}
// The fix range bounds the check: postFixHead is fix-apply's fixHead, preFixHead its preFixHead —
// never the round's head, whose range pulls in commits that landed since the pin.
for (const key of ['preFixHead', 'postFixHead']) {
  if (!/^[0-9a-f]{40}$/.test(args[key])) {
    throw new Error(`args.${key} must be a full commit SHA — postFixHead is fix-apply's fixHead, preFixHead its preFixHead`)
  }
}
if (args.preFixHead === args.postFixHead) {
  throw new Error('args.preFixHead equals args.postFixHead — there are no fix commits to check')
}
if (!Array.isArray(args.ids) || !args.ids.length) {
  throw new Error('args.ids is required — the finding ids the fix-apply stage applied')
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
const opts = { agentType: 'railroad-fix-checker', schema: FIX_CHECK_SCHEMA, label: 'fix-check' }
opts.model = assertModel('laneModel', args.laneModel || DEFAULT_SUBAGENT_MODEL)

const prompt =
  `CHECK the fix commits \`git -C ${args.repo} diff ${args.preFixHead} ${args.postFixHead}\` against the ranked_plan entries ` +
  `${args.ids.join(', ')} of ${args.reviewPath} (jq by finding_id).\n` +
  `JUDGE each entry at ${args.postFixHead}: read its claim, then the code the claim cites as it stands after the fix ` +
  `(\`git -C ${args.repo} show ${args.postFixHead}:<file>\`). The entry is fixed only when the defect the claim names no ` +
  `longer exists there. A hit is a defect still present, a change the claim does not need — a new mechanism or fallback, ` +
  `a weakened guard or test — or a changed file no entry accounts for.\n` +
  `RETURN {"fixed", "hits"} — the fixed ids, and each hit with its file, reason and finding_id when it belongs to one; ` +
  `every entry is in exactly one of the two.`

phase('Fix check')
const result = await agent(prompt, opts)
const isHit = id => result.hits.some(hit => hit.finding_id === id)
const unjudged = args.ids.filter(id => !result.fixed.includes(id) && !isHit(id))
if (unjudged.length) {
  throw new Error(`fix checker left finding ids unjudged: ${unjudged.join(', ')}`)
}
const both = result.fixed.filter(isHit)
if (both.length) {
  throw new Error(`fix checker returned finding ids as fixed and as hit: ${both.join(', ')}`)
}
return result
