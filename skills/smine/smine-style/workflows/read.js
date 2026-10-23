export const meta = {
  name: 'smine-style-read',
  description: 'Read smine-style sources: one voice reader per batch of sampled session ids plus one taste reader, each returning a schema-typed abstract with no verbatim text',
  whenToUse: 'Invoked by /smine-style at its reader spawn; the root distills the returned abstracts into the style profile',
  phases: [
    { title: 'Read' },
  ],
}

// GEN:schemas reader
const READER_SCHEMA = {
  "$defs": {
    "voice": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "sessionsRead",
        "userTurnsRead",
        "languageMix",
        "register",
        "patterns",
        "skipped"
      ],
      "properties": {
        "sessionsRead": {
          "type": "integer",
          "minimum": 0,
          "description": "sessions of this batch whose user turns were read"
        },
        "userTurnsRead": {
          "type": "integer",
          "minimum": 0,
          "description": "user turns read across those sessions"
        },
        "languageMix": {
          "type": "string",
          "maxLength": 120,
          "description": "label, e.g. German with English technical terms"
        },
        "register": {
          "type": "object",
          "additionalProperties": false,
          "required": [
            "formality",
            "terseness",
            "directness",
            "hedging"
          ],
          "properties": {
            "formality": {
              "enum": [
                "informal",
                "neutral",
                "formal",
                "mixed"
              ]
            },
            "terseness": {
              "enum": [
                "terse",
                "medium",
                "elaborate",
                "mixed"
              ]
            },
            "directness": {
              "enum": [
                "direct",
                "neutral",
                "indirect",
                "mixed"
              ]
            },
            "hedging": {
              "enum": [
                "none",
                "low",
                "medium",
                "high"
              ]
            }
          }
        },
        "patterns": {
          "type": "array",
          "maxItems": 20,
          "description": "recurring phrasings and preferences as short neutral labels, never a quote",
          "items": {
            "type": "string",
            "maxLength": 120
          }
        },
        "skipped": {
          "type": "array",
          "description": "session ids of this batch not read: unreadable, or fewer than 2 user turns",
          "items": {
            "type": "string",
            "maxLength": 64
          }
        }
      }
    },
    "taste": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "sourcesActuallyRead",
        "chosen",
        "rejected",
        "tilts"
      ],
      "properties": {
        "sourcesActuallyRead": {
          "type": "object",
          "additionalProperties": false,
          "description": "counts of the sources opened and read, never the paths handed over",
          "required": [
            "decisionsTables",
            "archivePlans",
            "proposalRejections",
            "backlogRejections"
          ],
          "properties": {
            "decisionsTables": {
              "type": "integer",
              "minimum": 0
            },
            "archivePlans": {
              "type": "integer",
              "minimum": 0
            },
            "proposalRejections": {
              "type": "integer",
              "minimum": 0
            },
            "backlogRejections": {
              "type": "integer",
              "minimum": 0
            }
          }
        },
        "chosen": {
          "type": "array",
          "maxItems": 30,
          "description": "mechanisms the user chose, as labels",
          "items": {
            "type": "string",
            "maxLength": 120
          }
        },
        "rejected": {
          "type": "array",
          "maxItems": 30,
          "description": "shapes the user rejected and recurring rejection reasons, as labels",
          "items": {
            "type": "string",
            "maxLength": 120
          }
        },
        "tilts": {
          "type": "array",
          "maxItems": 20,
          "description": "trade-off tilts, e.g. fewest concepts over smallest diff",
          "items": {
            "type": "string",
            "maxLength": 120
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
  throw new Error('args must be an object — /smine-style assembles {sessions, tasteSources, model, batch} at its reader spawn')
}
if (!Array.isArray(args.sessions) || !args.sessions.length || !args.sessions.every(id => typeof id === 'string' && id)) {
  throw new Error('args.sessions is required — the selected session ids, one string each')
}
if (!Array.isArray(args.tasteSources) || !args.tasteSources.every(path => typeof path === 'string' && path)) {
  throw new Error('args.tasteSources is required — the listed taste-source paths, one string each, empty when none exist')
}
if (!Number.isInteger(args.batch) || args.batch < 1) {
  throw new Error(`args.batch "${args.batch}" must be a positive integer — session ids per voice reader`)
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

const readerModel = assertModel('args.model', args.model)
const VOICE_SCHEMA = READER_SCHEMA.$defs.voice
const TASTE_SCHEMA = READER_SCHEMA.$defs.taste

const preamble =
  'Read the global context file the SubagentStart hook names first, in full. Never spawn agents or workflows — no Agent ' +
  'and no Workflow call; you read and abstract alone. Return labels only: never quote a user message, plan text or ' +
  'rejection reason, never include names, addresses, credentials, file contents or other personal data, and describe ' +
  'profanity or hostility neutrally (e.g. "very direct, no praise") instead of reproducing it.'

const batches = []
for (let start = 0; start < args.sessions.length; start += args.batch) {
  batches.push(args.sessions.slice(start, start + args.batch))
}

const readVoice = (ids, index) => agent(
  `${preamble}\n\nYou are voice reader ${index + 1} of ${batches.length}. Your session ids: ${JSON.stringify(ids)}. ` +
  'Load mcp__peek-mcp__session_get through ToolSearch, then call it for your own ids only — never another session, never ' +
  'session_list. Read the USER turns of each session (follow request_id pages to the end). A session with fewer than 2 ' +
  'user turns, or one that will not load, goes to skipped. From the user turns derive the language mix, the register ' +
  '(formality, terseness, directness, hedging) and recurring phrasings and preferences as short neutral labels ' +
  '(e.g. "asks for examples", "dislikes bullet walls"). sessionsRead and userTurnsRead count what you actually read.',
  { label: `voice ${index + 1}/${batches.length}`, phase: 'Read', schema: VOICE_SCHEMA, model: readerModel, effort: 'medium' },
).then(result => result || { sessionsRead: 0, userTurnsRead: 0, languageMix: '', register: null, patterns: [], skipped: ids })

const NO_TASTE = { sourcesActuallyRead: { decisionsTables: 0, archivePlans: 0, proposalRejections: 0, backlogRejections: 0 }, chosen: [], rejected: [], tilts: [] }

const readTaste = () => agent(
  `${preamble}\n\nYou are the taste reader. Your taste sources:\n${args.tasteSources.join('\n')}\n\n` +
  'Each path is a plan file or plans-archive dir (read its Decisions tables), the proposal rejections file (read its ' +
  'reason lines) or the backlog JSON (read archive.rejected[] with jq). A missing path is skipped. Derive the user\'s ' +
  'recurring design answers as labels: mechanisms chosen, shapes rejected with the recurring rejection reasons, and ' +
  'trade-off tilts. sourcesActuallyRead counts what you opened and read per kind — a Decisions table per plan, a plans-' +
  'archive plan per plan, a rejection per reason line, a backlog rejection per row — never the number of paths given.',
  { label: 'taste', phase: 'Read', schema: TASTE_SCHEMA, model: readerModel, effort: 'medium' },
).then(result => result || NO_TASTE)

const tasks = batches.map((ids, index) => () => readVoice(ids, index))
if (args.tasteSources.length) {
  tasks.push(readTaste)
}

const results = await parallel(tasks)
const voice = results.slice(0, batches.length)
const taste = args.tasteSources.length ? results[batches.length] : NO_TASTE
return { voice, taste }
