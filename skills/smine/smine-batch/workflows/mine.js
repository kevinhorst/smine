export const meta = {
  name: 'smine-mine',
  description: 'Mine one smine-batch run: one miner per session writes and checks its session record JSON',
  whenToUse: 'Invoked by /smine-batch at its Analyze spawn with the pending list from make sessions',
  phases: [
    { title: 'Mine' },
  ],
}

// GEN:schemas miner
const MINER_SCHEMA = {
  "type": "object",
  "required": [
    "id",
    "path",
    "failed"
  ],
  "properties": {
    "id": {
      "type": "string",
      "description": "full session id"
    },
    "path": {
      "type": "string",
      "description": "the record JSON path from the pending entry"
    },
    "failed": {
      "type": "boolean",
      "description": "true when no checked record exists"
    },
    "skipped": {
      "type": "boolean",
      "description": "curated out as no-signal; the record carries skipped and its reason"
    },
    "dataGaps": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "note": {
      "type": "string"
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /smine-batch assembles {sessions, subagents, skillDir} at its Analyze spawn')
}
if (!Array.isArray(args.sessions) || !args.sessions.length) {
  throw new Error('args.sessions is required — the pending list from make sessions ARGS="pending …"')
}
if (typeof args.skillDir !== 'string' || !args.skillDir) {
  throw new Error('args.skillDir is required — pass the smine-batch base dir holding the deployed SKILL.md; miners read their route from it')
}
const subagentsRoute = args.subagents === true ? ', then its Subagent mining substeps (--subagents is on)' : ''

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

const failedRef = (session, note) => ({ id: session.id, path: session.path, failed: true, note })

const mineSession = session => agent(
  `Read ${args.skillDir}/SKILL.md and follow its 2. Analyze substeps${subagentsRoute} for this one session. ` +
  `Pending entry: ${JSON.stringify(session)}. Write the record JSON to ${session.path}, check ` +
  `it with make sessions ARGS="check ${session.path} --context <transcript>" (--context only for a Claude session whose ` +
  `transcript exists; it merges the used-context record) until it prints ok, and return its ref.`,
  { label: `mine ${session.repo}/${session.id.slice(0, 8)}`, phase: 'Mine', agentType: 'smine-miner', schema: MINER_SCHEMA, model: DEFAULT_SUBAGENT_MODEL, effort: 'medium' },
).then(ref => ref || failedRef(session, 'miner returned nothing'))

const refs = await parallel(args.sessions.map(session => () => mineSession(session)))
return { refs }
