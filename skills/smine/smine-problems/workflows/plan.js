export const meta = {
  name: 'smine-plan',
  description: 'Turn smine-problems directions into change tasks: one planner per direction and planner kind, each returning {task, reason}',
  whenToUse: 'Invoked by /smine-problems at its planner spawn; the root keeps one task per problem and writes it to .smine-run/changes/',
  phases: [
    { title: 'Plan' },
  ],
}

// GEN:schemas planner
const PLANNER_SCHEMA = {
  "type": "object",
  "required": [
    "task",
    "reason"
  ],
  "properties": {
    "task": {
      "type": [
        "object",
        "null"
      ],
      "description": "one change task of the planner's kind, or null when this direction yields none",
      "required": [
        "id",
        "kind",
        "title",
        "problem",
        "evidence",
        "fix",
        "sessions",
        "target",
        "change",
        "notes"
      ],
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^[a-z]+/[a-z0-9-]+$",
          "description": "<kind>/<slug>"
        },
        "backlog": {
          "type": "string",
          "pattern": "^B-[0-9]{3,}$",
          "description": "the backlog item this task picks up"
        },
        "kind": {
          "enum": [
            "acdsl",
            "context",
            "memory",
            "permissions",
            "skills",
            "routines",
            "judge"
          ]
        },
        "title": {
          "type": "string"
        },
        "problem": {
          "type": "string",
          "maxLength": 160,
          "description": "one plain sentence stating what is wrong for a reader who was not in the session; task-check refuses ids, shas, paths, counts and code in it"
        },
        "evidence": {
          "type": "array",
          "maxItems": 3,
          "items": {
            "type": "object",
            "required": [
              "session",
              "quote",
              "finding"
            ],
            "additionalProperties": false,
            "properties": {
              "session": {
                "type": "string"
              },
              "quote": {
                "type": "string",
                "maxLength": 160,
                "description": "a verbatim excerpt of the record's finding quote, at most 160 characters"
              },
              "finding": {
                "type": "integer",
                "minimum": 0,
                "description": "index of the session record finding the quote is taken from"
              }
            }
          }
        },
        "fix": {
          "type": "string",
          "maxLength": 240,
          "description": "why the change fixes it, at most 240 characters"
        },
        "sessions": {
          "type": "array",
          "items": {
            "type": "string"
          }
        },
        "target": {
          "type": "string",
          "description": "repo-relative path"
        },
        "change": {
          "type": "string",
          "description": "one imperative sentence"
        },
        "notes": {
          "type": "string"
        }
      }
    },
    "reason": {
      "type": "string",
      "description": "one line: why this task, or why none"
    }
  }
}
// GEN:end

if (typeof args === 'string') {
  args = JSON.parse(args)
}
if (!args || typeof args !== 'object') {
  throw new Error('args must be an object — /smine-problems assembles {requests} at its planner spawn')
}
if (!Array.isArray(args.requests) || !args.requests.length) {
  throw new Error('args.requests is required — one entry per direction and planner')
}
const PLANNERS = new Set(['smine-skills', 'smine-routines'])
for (const request of args.requests) {
  if (!PLANNERS.has(request.planner)) {
    throw new Error(`unknown planner ${request.planner} — must be one of ${[...PLANNERS].join(', ')}`)
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

const planDirection = request => agent(
  `Invoke the Skill tool with skill="${request.planner}" and args="plan", then follow it. Problem: ` +
  `${JSON.stringify(request.candidate)}. Direction index: ${request.directionIndex}. Return {task, reason} — task one change task or null.`,
  { label: `${request.planner} ${request.candidateId}#${request.directionIndex}`, phase: 'Plan', agentType: 'smine-planner', schema: PLANNER_SCHEMA, model: DEFAULT_SUBAGENT_MODEL, effort: 'high' },
).then(result => ({
  candidateId: request.candidateId,
  directionIndex: request.directionIndex,
  planner: request.planner,
  task: (result && result.task) || null,
  reason: (result && result.reason) || '',
}))

const tasks = await parallel(args.requests.map(request => () => planDirection(request)))
return { tasks }
