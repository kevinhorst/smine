export const meta = {
  name: 'delegate-run',
  description: 'Run one unattended-safe skill on a runner agent with a schema-validated result',
  whenToUse: 'Invoked by the /delegate skill for unattended-safe targets; gated targets stay on the Agent tool',
  phases: [
    { title: 'Run', detail: 'one runner agent executing the target skill unattended' },
  ],
}

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch { throw new Error('args arrived as a string and is not valid JSON') }
}
if (!args || typeof args !== 'object' || typeof args.skill !== 'string' || !args.skill) {
  throw new Error('args must be an object: {skill, prompt, runner, model}')
}
if (typeof args.prompt !== 'string' || !args.prompt) {
  throw new Error('args.prompt is required — the assembled spawn prompt from the delegate skill')
}

const RESULT_SCHEMA = {
  type: 'object',
  required: ['status', 'commits', 'notes'],
  properties: {
    status: { enum: ['done', 'failed'] },
    commits: { type: 'array', items: { type: 'string' }, description: 'hash — subject per commit made, empty on failed' },
    failure: { type: 'string', description: 'failing command plus trimmed output, only on failed' },
    notes: { type: 'string', description: 'deviations or skipped groups, empty when none' },
  },
}

phase('Run')
const opts = { label: `delegate:${args.skill}`, phase: 'Run', schema: RESULT_SCHEMA }
if (args.runner) opts.agentType = args.runner
if (args.model) opts.model = args.model
return await agent(args.prompt, opts)
