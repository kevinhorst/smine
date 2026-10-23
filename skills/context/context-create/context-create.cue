changelog: [{
	date:    "2026-10-21"
	version: "3.5.2"
	text:    "The band contract points at the HTML docs, docs/acdsl.html."
	tokens:  2734
}, {
	date:    "2026-10-17"
	version: "3.5.1"
	text:    "The smoke sets its own ACDSL_CHECK and ACDSL_FIXTURES (`go run ./cmd/acdsl`) — a smoke never reads the machine's values."
	tokens:  2748
}, {
	date:    "2026-10-15"
	version: "3.5.0"
	text:    "The finish check param runs the repo's check command one Bash call per line."
	tokens:  2748
}, {
	date:    "2026-10-14"
	version: "3.4.0"
	text:    "Interfaces: one variable per capability — ACDSL_CHECK, ACDSL_FIXTURES, CHECK, GENERATE"
	tokens:  2740
}, {
	date:    "2026-10-13"
	version: "3.3.0"
	text:    "Commands are env variables: $SKILL_CONTEXTCREATE_ACDSL (required) runs fixtures and the check dry-run, $SKILL_CONTEXTCREATE_RULES generate and $SKILL_CONTEXTCREATE_CHECK are optional finish params; both programs granted."
	tokens:  2726
}, {
	date:    "2026-10-06"
	version: "3.2.1"
	text:    "Taxonomy source is context/aspects.json; context/context.json is generated and untracked."
	tokens:  2647
}, {
	date:    "2026-10-03"
	version: "3.2.0"
	text:    "Requires context: deploy skips the skill without them."
	tokens:  2643
}, {
	date:    "2026-10-05"
	version: "3.1.6"
	text:    "Stale statements corrected: the tag vocabulary lists lint, hook, gate, schema and prose; used numbers come from the kind's own docs; an ACTION goes into its activity's chapter."
	tokens:  2643
}, {
	date:    "2026-09-28"
	version: "3.1.5"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  2563
}, {
	date:    "2026-09-26"
	version: "3.1.4"
	text:    "allowed-tools: make rules dropped — the global template admits Bash(make *)."
}, {
	date:    "2026-09-25"
	version: "3.1.3"
	text:    "Finish step fixes every audit violation until green, not only those in files the run wrote."
}, {
	date:    "2026-09-25"
	version: "3.1.2"
	text:    "Make-only entry points: allowed-tools and prose call make acdsl and make rules instead of go run ./cmd/acdsl and go run ./cmd/rules."
}, {
	date:    "2026-09-15"
	version: "3.1.1"
	text:    "quality entries speak MUST — the SHOULD register retired fleet-wide"
}, {
	date:    "2026-09-14"
	version: "3.1.0"
	text:    "Formal-schema reformat: section kind on Procedure, pred/post clauses on 12 flow entries plus a pred on the J-band quality entry, bullet registers to param, sibling atoms.json (16 bool, 1 int, 1 enum), and three queries under queries/."
}, {
	date:    "2026-09-14"
	version: "3.0.0"
	text:    "Format v2 migration: ontology retag incl. spawn, role wording register, section flow, Why extraction to README, format: 2."
}, {
	date:    "2026-09-08"
	version: "2.1.0"
	text:    "Caveman trim of over-cap entry statements (wording only, cap now 320 display runes)"
}, {
	date:    "2026-09-07"
	version: "2.0.0"
	text:    "Format-v2 reformat: When to use folded into Routing (Purpose/Disambiguation/Preconditions/Pipeline), Args promoted to a 4-column table, argument-hint added, allowed-tools order normalized, caveman prose pass."
}, {
	date:    "2026-09-05"
	version: "1.1"
	text:    "Entries conformed to the titled grammar — **Title** — statement headlines, 300-rune cap with detail moved to Why/Applies bullets"
}, {
	date:    "2026-09-05"
	version: "1.0"
	text:    "Initial: author one context entry from a stated rule; gate routes auto|none|<verifier> mirroring the smine-apply authoring routine."
}]
env: {
	ACDSL_CHECK: {
		interface: "acdsl-check@1"
		required:  true
	}
	ACDSL_FIXTURES: {
		interface: "acdsl-fixtures@1"
		required:  true
	}
	CHECK: {
		interface: "check@1"
	}
	GENERATE: {
		interface: "generate@1"
	}
}
frontmatter: [{
	key:   "name"
	value: " context-create"
}, {
	key:   "description"
	value: " Author one ACDSL context entry from a stated rule — taxonomy-valid ID, entry in the right context doc, optional gate with anchor, verifier, and fixtures. Trigger on /context-create or \"create a context entry/rule\" or \"turn this rule into ACDSL context\". Args — rule: the statement; kind: action|fact|rule override; gate: auto|none|<verifier>; reach: repo names."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.5.2"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " FACT-REPO-*, RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Edit, Read, Write, Bash($SKILL_CONTEXTCREATE_ACDSL_CHECK check *), Bash($SKILL_CONTEXTCREATE_ACDSL_FIXTURES fixtures *), Bash($SKILL_CONTEXTCREATE_GENERATE generate), Bash(git diff*), Bash(git status*), Bash(jq *)"
}, {
	key:   "argument-hint"
	value: " 'rule: \"every SKILL.md carries a version header\" kind: rule gate: auto reach: repo-a repo-b'"
}]
preamble: """
	# Context Create

	Authors exactly one context entry — from a user-stated rule — directly into the context surface, this session.

	Authoring contract (ID grammar, entry grammar, numbering, tombstones) lives in context/actions/README.md; gate machinery
	(bands, generation bounds, fixtures) in docs/acdsl.html. Skill applies that contract; never
	redefines it.
	"""
requires: ["context"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:  "from"
					skill: "context-bootstrap"
				}]
				note:       "applies the accepted candidates"
				standalone: true
			}
			preconditions: ["a repo with the context surface (`context/aspects.json` present)", "gate routes additionally need the ACDSL toolchain (`cmd/acdsl`, `cmd/rules`, `acdsl/evalgen.json`)"]
			purpose: "turns a user-stated rule, fact, or convention into a durable ACDSL context entry — with or without a machine gate; also the apply step for accepted context-bootstrap report candidates."
			routes: [{
				for: "a finding from session mining"
				use: "the smine pipeline (proposals + votes)"
			}, {
				for: "editing or renumbering an existing entry"
				use: "edit the doc directly, per the authoring contract"
			}, {
				for: "creating a skill"
				use: "/skillroutine-create"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			rows: [{
				name:    "rule"
				default: "required"
				effect:  "the rule/fact statement — everything not parsed as another arg"
				form:    "free text, positional"
			}, {
				name:    "kind"
				default: "classified from the statement"
				effect:  "overrides KIND — [Classify](#skill-contextcreate-classify-001a)"
				form:    "`action \\| fact \\| rule`"
			}, {
				name:    "gate"
				default: "`auto`"
				effect:  "`auto` classifies band + routes; `none` forces prose-only (J-band); a name reuses that verifier — [Gate route](#skill-contextcreate-gate-001)"
				form:    "`auto \\| none \\| <verifier-name>`"
			}, {
				name:    "reach"
				default: "global"
				effect:  "scopes the entry (rule `reach=` / prose `Reach:`)"
				form:    "space-separated repo names"
			}]
		}
	}]
}, {
	title: "Procedure"
	blocks: [{
		entry: {
			id:        "SKILL-CONTEXTCREATE-AUTHOR-001a"
			role:      "def"
			statement: "Tag vocabulary is `[lint]`/`[hook]`/`[gate]`/`[schema]`/`[prose]` per the authoring contract."
			title:     "Tag vocabulary"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-AUTHOR-001b"
			custom: ["todo:unmeasured"]
			pred:      "band = j"
			role:      "quality"
			statement: "J-band entries MUST carry a worked example — measured to hold under contested attention where prose collapses."
			title:     "J-band example"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-CLASSIFY-001a"
			post: ["kind_determined"]
			role:      "step"
			statement: "DETERMINE KIND (arg wins)."
			title:     "Classify: kind"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-CLASSIFY-001b"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`gate: auto` classifies band per the band taxonomy (free tool / cheap AST / dependency-gated / judgment prose — docs/acdsl.html)."
			}, {
				label:  "param"
				marker: "*"
				text:   "checkable by an existing or generatable verifier → F/A; needs a new dependency → D; judgment call → J."
			}, {
				label:  "param"
				marker: "*"
				text:   "an ACTION goes into the chapter of its activity — concepting, implementing, reviewing, navigating."
			}]
			post: ["target_doc_mapped"]
			pred:      "kind_determined"
			role:      "step"
			statement: "MAP KIND to target doc: ACTION → `context/actions/`, FACT → `context/facts/`, RULE → `context/rules/`."
			title:     "Classify: target doc"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-CLASSIFY-001c"
			post: ["classification_stated"]
			pred:      "target_doc_mapped"
			role:      "step"
			statement: "STATE classification + target doc + band in chat before writing."
			title:     "Classify: state"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-ID-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "only ACDSL ids live in `acdsl/rules.acdsl`."
			}]
			post: ["taxonomy_read"]
			pred:      "classification_stated"
			role:      "step"
			statement: "READ the registered taxonomy (scope/topic aspects in `context/aspects.json`); used numbers are read from the kind's own docs and their `-local` siblings, tombstone tables included."
			title:     "ID selection: read"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-ID-001b"
			post: ["id_picked"]
			pred:      "taxonomy_read"
			role:      "step"
			statement: "PICK a registered scope/topic token and next free number."
			title:     "ID selection: pick"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-ID-001a"
			items: [{
				label:  "param"
				marker: "*"
				text:   "id-grammar gate rejects unregistered segments. Repo-overlay actions number from 100."
			}]
			pred:      "id_picked and (not token_registered or number_reused)"
			role:      "stop"
			statement: "If a scope/topic token is unregistered, or a number would be reused (tombstones included) ⇒ STOP: pick a different registered token/number."
			title:     "No invented tokens"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-AUTHOR-001"
			post: ["entry_authored"]
			pred:      "id_picked"
			role:      "step"
			statement: "AUTHOR the entry per entry grammar (bold ID, enforcement tag, statement, Applies/Why/Location/Reach): bold ID headline, enforcement tag (never on FACT), statement."
			title:     "Author entry: grammar"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-AUTHOR-001c"
			post: ["entry_bullets_added"]
			pred:      "entry_authored"
			role:      "step"
			statement: "INCLUDE `Applies:` bullet (required on ACTION), `Why:` where reasoning isn't self-evident, `Location:` (required on FACT), `Reach:` from reach arg."
			title:     "Author entry: bullets"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-GATE-001"
			items: [{
				depth:  1
				marker: "-"
				text:   "Existing verifier fits (named or found in `acdsl/registry.json`) → append the rule line to `acdsl/rules.acdsl` (anchor regex from rule's file class; `reach=` from arg), author `acdsl/testdata/<id>/pass|fail` fixtures from real repo snippets."
			}, {
				depth:  1
				marker: "-"
				text:   "No verifier fits → generate one exactly per the generation contract (bounds in `acdsl/evalgen.json`; A-band mirrors `bash32compat`, F-band wrappers mirror `gofmtwrap` over an execAllowlist binary): `cmd/acdsl/verifiers/<name>/main.go`, registry entry (timeout ≤ bound), rule line, fixtures. Validate: `$SKILL_CONTEXTCREATE_ACDSL_FIXTURES fixtures` green + repo-wide dry-run `$SKILL_CONTEXTCREATE_ACDSL_CHECK check -rule <id>` — every dry-run hit must be a real violation; hit on a file the rule shouldn't govern means anchor is wrong: fix anchor once, re-run. Max 2 generation attempts."
			}]
			post: ["gate_applied"]
			pred:      "band = f or band = a or gate_verifier_named"
			role:      "step"
			statement: "APPLY on band F/A, or `gate: <verifier>`:"
			title:     "Gate route"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTCREATE-GATE-002a"
			pred:      "band = d or not band_supported or not binary_allowlisted"
			role:      "stop"
			statement: "If band D (new dependency), band ∉ evalgen `bands`, or a needed binary ∉ `execAllowlist` ⇒ STOP: keep the prose entry (J-form, worked example), report `manual-verifier` with failure detail."
			title:     "Stops: band/dependency"
		}
	}, {
		entry: {
			id:        "SKILL-CONTEXTCREATE-GATE-002b"
			pred:      "generation_attempts >= 2 and not generation_succeeded"
			role:      "stop"
			statement: "If a second generation attempt fails ⇒ STOP: keep the prose entry (J-form, worked example), report `manual-verifier`; never a third attempt or weakened check."
			title:     "Stops: generation retries"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-FINISH-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "regenerate: `$SKILL_CONTEXTCREATE_GENERATE generate`."
			}, {
				label:  "param"
				marker: "*"
				text:   "check: `$SKILL_CONTEXTCREATE_CHECK`, one Bash call per line."
			}]
			post: ["context_regenerated"]
			pred:      "entry_authored"
			role:      "step"
			statement: "REGENERATE context/context.json through its sole writer — never hand-edit it — then RUN the repo's check; FIX every violation until green, whichever file or gate it sits in."
			title:     "Regenerate and validate"
		}
	}, {
		entry: {
			id: "SKILL-CONTEXTCREATE-FINISH-001b"
			post: ["reported"]
			pred:      "context_regenerated"
			role:      "step"
			statement: "REPORT entry ID, doc, band, gate artifacts created, and diff summary."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "taxonomy selection and band classification are judgment-heavy; verifier generation needs reliable Go authoring."
			tier:   "frontier"
		}
	}]
}]
smoke: {
	env: {
		ACDSL_CHECK:    "go run ./cmd/acdsl"
		ACDSL_FIXTURES: "go run ./cmd/acdsl"
	}
	expect: {}
	invoke: "rule: \"a skill's smoke fixture files live under its skill dir's smoke/ dir\" kind: rule gate: none"
}
