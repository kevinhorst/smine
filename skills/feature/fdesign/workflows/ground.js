export const meta = {
  name: 'fdesign-ground',
  description: 'Ground an fdesign plan: one agent per area, a limited number of areas, reads the live tree and returns file:line facts',
  whenToUse: 'Invoked by /fdesign outside plan mode at its ground spawn; the planner writes Baseline / Current state from the returned facts',
  phases: [
    { title: 'Ground' },
  ],
}

// GEN:schemas fact
const FACT_SCHEMA = {
  "type": "object",
  "required": [
    "area",
    "facts"
  ],
  "properties": {
    "area": {
      "type": "string"
    },
    "facts": {
      "type": "array",
      "items": {
        "type": "object",
        "required": [
          "fact",
          "location",
          "needed_for"
        ],
        "properties": {
          "fact": {
            "type": "string",
            "description": "one verified fact about the current tree"
          },
          "location": {
            "type": "string",
            "description": "path:line, or the command run plus the output line proving it"
          },
          "needed_for": {
            "type": "string",
            "description": "the question the fact answers"
          }
        }
      }
    },
    "stale": {
      "type": "array",
      "description": "statements in docs, context or skill text that the tree contradicts, and facts those texts lack",
      "items": {
        "type": "object",
        "required": [
          "location",
          "statement",
          "correction"
        ],
        "properties": {
          "location": {
            "type": "string",
            "description": "path:line of the statement, or the section that lacks the fact"
          },
          "statement": {
            "type": "string",
            "description": "the statement quoted, or what is missing"
          },
          "correction": {
            "type": "string",
            "description": "the text as the tree makes it read"
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
  throw new Error('args must be an object — /fdesign assembles {base, areas, agents, model, effort} at its ground spawn')
}
if (!Array.isArray(args.areas) || !args.areas.length) {
  throw new Error('args.areas is required — one entry {area, questions[]} per area the plan touches')
}
if (typeof args.base !== 'string' || !args.base) {
  throw new Error('args.base is required — the base branch the plan builds on')
}
// The round's size: the n of the invocation's subagents arg, 3 without one.
const DEFAULT_GROUND_AGENTS = 3
const groundAgents = args.agents === undefined ? DEFAULT_GROUND_AGENTS : args.agents
if (!Number.isInteger(groundAgents) || groundAgents < 1) {
  throw new Error('args.agents must be a positive integer — the n of the invocation\'s subagents arg')
}
if (args.areas.length > groundAgents) {
  throw new Error(`args.areas holds ${args.areas.length} areas, the round is limited to ${groundAgents} agents — merge areas, the round runs once`)
}

const EFFORTS = ['low', 'medium', 'high', 'xhigh', 'max']
if (args.effort !== undefined && !EFFORTS.includes(args.effort)) {
  throw new Error(`args.effort "${args.effort}" is not an effort (${EFFORTS.join(' | ')})`)
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

const groundModel = assertModel('ground model', args.model || 'opus')
const groundOptions = { phase: 'Ground', schema: FACT_SCHEMA, model: groundModel }
if (args.effort !== undefined) {
  groundOptions.effort = args.effort
}

// GEN:payload SKILL-FDESIGN-TPL-004
const PAYLOAD_SKILL_FDESIGN_TPL_004 = "Read AGENTS.md and the context docs it names first.\nGround one area of a feature plan against the live tree on base branch <base>. Area: <area>.\nQuestions to answer:\n- <question>\nUnder facts, also list every consumer of each thing the area changes — callers, libraries reading the same config, the install or deploy reconcile that writes it live, tests pinning it — and any defect on a path the change writes through.\nAnswer every question with facts you verified yourself: each fact carries a path:line, or the exact command you ran plus the output line that proves it. A claim that something does not exist carries the search command and its empty result. Under stale, report every statement in a doc, a context entry or a skill text of this area that the tree contradicts, and every fact such a text lacks: its path:line, the statement quoted, and the corrected text. Report what the tree says, never what it should say; write nothing.\nReturn: area, facts[{fact, location, needed_for}], stale[{location, statement, correction}]."
// GEN:end

// The brief is the skill's Ground brief payload, generated above; the script fills its placeholders (function replacers keep a `$` literal).
const briefFor = area => PAYLOAD_SKILL_FDESIGN_TPL_004
  .replace('<base>', () => args.base)
  .replace('<area>', () => area.area)
  .replace('- <question>', () => area.questions.map(question => `- ${question}`).join('\n'))

const groundArea = area => agent(briefFor(area), { ...groundOptions, label: `ground ${area.area}` })
  .then(result => result || { area: area.area, facts: [] })

const grounded = (await parallel(args.areas.map(area => () => groundArea(area)))).filter(Boolean)
const empty = grounded.filter(result => !result.facts.length).map(result => result.area)
if (empty.length) {
  log(`no facts returned for: ${empty.join(', ')}`)
}
return { areas: grounded, empty }
