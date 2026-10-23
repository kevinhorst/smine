changelog: [{
	date:    "2026-10-14"
	version: "3.2.0"
	text:    "Interfaces: one variable per capability — PLANS_ARCHIVE, STORE_ROOT"
	tokens:  4081
}, {
	date:    "2026-10-13"
	version: "3.1.0"
	text:    "Required env variables: $SKILL_BACKLOG_RULES prints the store root for the dest default (granted as Bash($SKILL_BACKLOG_RULES store-root)), $SKILL_BACKLOG_ARCHIVE is the plans-archive root of the Read and Edit grants."
	tokens:  4076
}, {
	date:    "2026-10-10"
	version: "3.0.0"
	text:    "Epics replace sprints: sections are named boxes with no order between them, created by the skill and the board; titles cap at 120 runes; link and evidence lines name the runner's writes; a session pointer may name a finding as '#<n>'."
	tokens:  4067
}, {
	date:    "2026-10-03"
	version: "2.5.0"
	text:    "References reworded to name no smine program."
	tokens:  3916
}, {
	date:    "2026-10-06"
	version: "2.4.1"
	text:    "Item schema lists status rejected and the mined marker the smine nightly sets; the board's rejected status archives an item under rejected."
	tokens:  3935
}, {
	date:    "2026-10-02"
	version: "2.4.0"
	text:    "dest defaults to the plans-archive store `~/GolandProjects/agent-plans/<repo-name>/backlog/`; allowed-tools grants Read and Edit there."
	tokens:  3892
}, {
	date:    "2026-10-01"
	version: "2.3.0"
	text:    "allowed-tools grants Skill(fdesign) for the offer chain into a shaped item's route; pipeline declares to /fdesign."
	tokens:  3864
}, {
	date:    "2026-09-28"
	version: "2.2.1"
	text:    "Routing and Model are typed blocks in skill.cue; the Pipeline line renders from declared edges"
	tokens:  3852
}, {
	date:    "2026-09-26"
	version: "2.2.0"
	text:    "Replan route declares its anchor: replaces INTAKE-002 + MERGE-001..MERGE-003 — the dump intake and the Merge phase it skips; the route line stays in the deployed skill."
}, {
	date:    "2026-09-26"
	version: "2.1.0"
	text:    "Route vocabulary adds /fexplore and /skillroutine-create (with route args, forwarded to the dispatched run); Owning skill rule — skill or routine changes route /skillroutine-create, direct only for S work no route skill owns"
}, {
	date:    "2026-09-24"
	version: "2.0.0"
	text:    "Store is one JSON file backlog.json shaped by schema.json (markdown tables retired); items carry description (dump lines + Done when) and evidence; no-loss rules — description capture, evidence capture, match appends, no information loss, description gate; ids derived, counters gone"
}, {
	date:    "2026-09-24"
	version: "1.1.0"
	text:    "Status vocabulary: in-pipeline split into the stages planning, implementing, review, testing — one axis shared with the config server's Backlog board, which writes the same cells"
}, {
	date:    "2026-09-18"
	version: "1.0.0"
	text:    "initial — dump atomize, semantic merge vs store and archive, sprint partition, one correction round, durable backlog/ store"
}]
env: {
	PLANS_ARCHIVE: {
		interface: "plans-archive@1"
		required:  true
	}
	STORE_ROOT: {
		interface: "store-root@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " backlog"
}, {
	key:   "description"
	value: " Organize a raw idea brain-dump into a durable epic backlog; re-invocation merges new dumps. Trigger on /backlog or \"organize my ideas\" or \"here is a brain dump\". Args — dest: store directory; replan: re-rank and re-group the store, no new dump."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 3.2.0"
}, {
	key:   "title-code"
	value: " Backlog"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "argument-hint"
	value: " \"dest ./backlog replan\""
}, {
	key:   "allowed-tools"
	value: " Edit($SKILL_BACKLOG_PLANS_ARCHIVE/**), Read($SKILL_BACKLOG_PLANS_ARCHIVE/**), Skill(fdesign), Bash($SKILL_BACKLOG_STORE_ROOT store-root)"
}]
preamble: """
	# Backlog

	Pre-pipeline organizer: raw multi-idea brain-dump in, prioritized epic backlog out, persisted in the
	durable store. New dumps merge; items graduate into the planning chain per their route.
	"""
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				edges: [{
					kind:     "to"
					skill:    "idea"
					optional: true
				}, {
					kind:  "to"
					skill: "concept"
				}, {
					kind:     "to"
					skill:    "fdesign"
					optional: true
				}]
			}
			preconditions: ["none — pre-chain, runs before any plan dir exists."]
			purpose: "organize and prioritize a raw multi-idea brain-dump into the durable epic backlog; re-invocation merges."
			routes: [{
				for: "one idea to stress-test or map"
				use: "/idea"
			}, {
				for: "committed feature, wants what/why written down"
				use: "/concept"
			}, {
				for: "proposals mined from session transcripts"
				use: "/smine"
			}, {
				for: "one small task, no backlog value"
				use: "do it directly"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "dest"
				effect: "directory the store lives in; default: `<store>/backlog/`, `<store>` the path `$SKILL_BACKLOG_STORE_ROOT store-root` prints."
			}, {
				name:   "replan"
				effect: "re-rank and re-group the existing store; Intake and Merge skipped, no new dump."
			}]
		}
	}]
}, {
	title: "Store"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-STORE-001a"
			role:      "def"
			statement: "The store is one file `<dest>/backlog.json` — sections (epics, Icebox), archive (done, rejected), changelog — shaped by `schema.json` next to this skill."
			title:     "Store layout"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-STORE-001b"
			role:      "def"
			statement: "An absent store file means an empty store; the first write creates it."
			title:     "Empty store"
		}
	}, {
		entry: {
			id: "SKILL-BACKLOG-STORE-002"
			items: [{
				fence: {
					body: """
						id          B-NNN
						added       date the item entered the store
						deps        item ids this item waits on
						description the dump's own lines for the item, verbatim, then one `Done when:` line — never title-only
						evidence    commits, files, sessions, docs the dump or the conversation named; a session as its id or 'session <8+ hex>', '#<n>' naming its record finding
						link        plan dir, PR, or the dispatched run's dated branch the runner writes
						mined       {by, date, sessions} — set by the smine nightly when it mined the item, by = its run's session; absent on your own rows
						pri         critical | high | medium | low
						route       /idea | /concept | /fexplore | /fdesign | /skillroutine-create | direct | experiment, plus route args
						size        S | M | L | XL (multi-week)
						status      raw | shaped | planning | implementing | review | testing | done | parked | rejected
						theme       one word
						title       the user's wording, trimmed, at most 120 runes

						"""
					info:   "text"
					marker: "```"
				}
			}]
			role:      "def"
			statement: "An item is one object of the `item` definition in `schema.json`:"
			title:     "Item schema"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-STORE-003"
			role:      "def"
			statement: "An epic is a named box of related items: no order between epics, deps may cross epics, no velocity or estimation; the Icebox holds every unsequenced item, nightly and problem-miner rows included."
			title:     "Epic semantics"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-STORE-004"
			role:      "rule"
			statement: "Archive rows MUST NOT be deleted — rejected rows are permanent anti-re-proposal memory, done rows the record."
			title:     "Archive permanence"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-STORE-005"
			role:      "rule"
			statement: "Item ids MUST stay unique across sections and archive; the next id is the highest id anywhere plus one — a retired id is NEVER reused."
			title:     "ID discipline"
		}
	}, {
		entry: {
			id: "SKILL-BACKLOG-STORE-006"
			payload: {
				info:   "json"
				marker: "```"
			}
			role:      "payload"
			statement: "Canonical form — two-space indent, keys in this order, one key per line; the board writes the same form."
			title:     "backlog.json template"
		}
	}]
	kind: "context"
}, {
	title: "Intake"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-INTAKE-001"
			role:      "rule"
			statement: "Item titles MUST keep the user's own wording, trimmed — never paraphrased into agent vocabulary."
			title:     "Title wording"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-INTAKE-004"
			role:      "rule"
			statement: "Every item's description MUST hold the dump's own lines for it, verbatim and trimmed, then one `Done when:` line — never a title alone, never a paraphrase in place of the quote."
			title:     "Description capture"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-INTAKE-005"
			role:      "rule"
			statement: "Every commit, file, session id, doc or link the dump or the conversation names for an item MUST land in its evidence list, never only in the title."
			title:     "Evidence capture"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-INTAKE-002"
			role:      "stop"
			statement: "If the invocation carries no dump and no `replan` ⇒ STOP: ask for the dump; nothing is read or written."
			title:     "Missing dump"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-INTAKE-003"
			role:      "step"
			statement: "SPLIT the dump into atomic items, one actionable idea each — a line bundling a refactor with defect fixes becomes separate items."
			title:     "Atomize"
		}
	}]
	kind: "phase"
}, {
	title: "Replan"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-REPLAN-001"
			role:      "step"
			statement: "SKIP [Missing dump](#skill-backlog-intake-002) and the Merge phase; RE-RANK and re-group the existing store, then Classify through Emit run unchanged."
			title:     "Re-rank"
		}
	}]
	kind:     "route"
	pred:     "replan"
	replaces: "INTAKE-002 + MERGE-001..MERGE-003"
}, {
	title: "Merge"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-MERGE-004"
			role:      "rule"
			statement: "A matched item MUST keep its description and gain the new dump's lines appended under a `<date>:` prefix; a merge NEVER replaces or trims existing text."
			title:     "Match appends"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-MERGE-001"
			role:      "step"
			statement: "LOAD the store file."
			title:     "Load store"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-MERGE-002"
			role:      "step"
			statement: "MATCH every new item against active and archive rows by meaning — the same defect or idea in other words is a match."
			title:     "Semantic match"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-MERGE-003"
			role:      "stop"
			statement: "If a new item matches an archived rejected row ⇒ STOP: surface the rejection date and reason; revive only on explicit confirmation."
			title:     "Rejected revive"
		}
	}]
	kind: "phase"
}, {
	title: "Clarify"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-CLARIFY-001"
			role:      "rule"
			statement: "Questions MUST target only unintelligible items, one at a time; every classifiable item proceeds unasked."
			title:     "Question budget"
		}
	}]
	kind: "phase"
}, {
	title: "Classify"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-CLASSIFY-001"
			role:      "rule"
			statement: "An XL item MUST NOT route `direct`; `direct` is legal only for S items."
			title:     "Route bounds"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-CLASSIFY-002"
			role:      "rule"
			statement: "Work already WIP elsewhere MUST enter at its current stage (`planning` … `testing`) track-only with its Link set, never re-planned here."
			title:     "In-flight work"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-CLASSIFY-004"
			role:      "rule"
			statement: "A skill or routine change MUST route `/skillroutine-create skill` or `/skillroutine-create routine`; `direct` MUST only hold S work no route skill owns."
			title:     "Owning skill"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-CLASSIFY-003"
			role:      "step"
			statement: "ASSIGN each item theme, size, priority and route per the [Item schema](#skill-backlog-store-002)."
			title:     "Assign"
		}
	}]
	kind: "phase"
}, {
	title: "Epics"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-SPRINTS-001"
			role:      "step"
			statement: "DRAW dependency edges between items into the Deps column."
			title:     "Dependency edges"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-SPRINTS-002"
			role:      "step"
			statement: "GROUP items into named epics by goal or theme — names the dump gives, else yours — and MOVE items the dump re-scopes; unsequenced items land in the Icebox."
			title:     "Group"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-SPRINTS-003"
			role:      "verify"
			statement: "After grouping: VERIFY the dependency graph is acyclic; repair a cycle by splitting one item."
			title:     "Cycle check"
		}
	}]
	kind: "phase"
}, {
	title: "Propose"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-PROPOSE-001"
			role:      "step"
			statement: "PRESENT the full proposed backlog in chat — epics, priorities, routes, every merge and park decision."
			title:     "Present"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-PROPOSE-002"
			role:      "stop"
			statement: "If the user has not ruled on the proposal ⇒ STOP: wait; exactly one correction round precedes any write."
			title:     "Correction gate"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-PROPOSE-003"
			role:      "step"
			statement: "FOLD the corrections; a second full round never runs — later changes are a new invocation."
			title:     "Fold corrections"
		}
	}]
	kind: "phase"
}, {
	title: "Emit"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-EMIT-005"
			role:      "rule"
			statement: "Shortening a title or a description MUST move the removed words into the description; a write NEVER drops text the store already holds."
			title:     "No information loss"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-EMIT-001"
			role:      "verify"
			statement: "Before write: VERIFY IDs unique across store and archive; renumber the newer item on collision."
			title:     "ID uniqueness"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-EMIT-006"
			role:      "verify"
			statement: "Before write: VERIFY every item and archive row has a non-blank description and the document matches `schema.json`; repair from the dump lines, never with a placeholder."
			title:     "Description gate"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-EMIT-002"
			role:      "step"
			statement: "WRITE `<dest>/backlog.json` in the canonical form of the [backlog.json template](#skill-backlog-store-006)."
			title:     "Write store"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-EMIT-003"
			role:      "step"
			statement: "REPORT the delta: added, merged, revived, parked, done, and every item that changed epic."
			title:     "Delta summary"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-EMIT-004"
			role:      "step"
			statement: "OFFER to chain the top shaped item into its Route skill."
			title:     "Chain offer"
		}
	}]
	kind: "phase"
}, {
	title: "Graduation"
	blocks: [{
		entry: {
			id:        "SKILL-BACKLOG-GRAD-001a"
			role:      "rule"
			statement: "An item whose work starts MUST carry its stage — `planning`, `implementing`, `review`, `testing` — and a Link to its follow-up artifact (plan dir, PR, doc, or the dispatched run's branch)."
			title:     "Pipeline link"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-GRAD-001b"
			role:      "rule"
			statement: "The config server's Backlog board MUST write one item's status, priority or epic, add or delete an epic, or archive an item, one changelog row per write, canonical form; it NEVER touches descriptions."
			title:     "Board writes"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-GRAD-001c"
			role:      "rule"
			statement: "The routine runner MUST write one dispatched item's link and pipeline stage per run and the nightly one Icebox row per oversized change, each with a changelog row."
			title:     "Runner writes"
		}
	}, {
		entry: {
			id:        "SKILL-BACKLOG-GRAD-002"
			role:      "rule"
			statement: "A finished item MUST move to the archive's done list with its description; a refused item moves to the rejected list — status `rejected` on the board does it."
			title:     "Done to archive"
		}
	}]
	kind: "context"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "medium"
			reason: "semantic dedupe, sizing and priority judgment over a small dependency graph; no fan-out."
			tier:   "frontier"
		}
	}]
}]
smoke: {
	skip: "correction gate precedes every store write and has no unattended path"
}
