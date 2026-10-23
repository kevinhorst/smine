export const meta = {
  name: 'heartbeat-demo-stage',
  description: 'The heartbeat-demo stage: one worker writes the artifact from the producer value and returns its path',
  whenToUse: 'Invoked by /heartbeat-demo at its Stage spawn (PROC-002)',
  phases: [
    { title: 'Stage' },
  ],
}

// GEN:schemas demo-worker
const DEMO_WORKER_SCHEMA = {
  "type": "object",
  "required": [
    "artifact"
  ],
  "properties": {
    "artifact": {
      "type": "string",
      "description": "absolute path of the artifact.json the worker wrote"
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /heartbeat-demo passes {value, artifactsDir}')
}
for (const key of ['value', 'artifactsDir']) {
  if (typeof args[key] !== 'string' || !args[key]) {
    throw new Error(`args.${key} is required — /heartbeat-demo passes {value, artifactsDir}`)
  }
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

phase('Stage')
const artifact = `${args.artifactsDir}/artifact.json`
const result = await agent(
  `WRITE ${artifact} with the JSON object {"value": "${args.value}"} using the Write tool, then return {"artifact": "${artifact}"}.`,
  { label: 'heartbeat-demo-worker', phase: 'Stage', agentType: 'heartbeat-demo-worker', schema: DEMO_WORKER_SCHEMA, model: DEFAULT_SUBAGENT_MODEL, effort: 'low' },
)
return { artifact: result ? result.artifact : null }
