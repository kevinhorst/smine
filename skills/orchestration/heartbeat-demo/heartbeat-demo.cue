changelog: [{
	date:    "2026-10-14"
	version: "2.5.0"
	text:    "Interfaces: one variable per capability — DEMO_CLOSE, DEMO_VALUE"
	tokens:  2159
}, {
	date:    "2026-10-13"
	version: "2.4.0"
	text:    "The rules tool is the required env variable $SKILL_HEARTBEATDEMO_RULES in the producer and closer steps, the precondition, the grants and the closed and produced anchor patterns; check reasons and recoveries name the subcommands only."
	tokens:  2140
}, {
	date:    "2026-10-09"
	version: "2.3.0"
	text:    "Runtime placement declared: root-only (Workflow stages); no mode owned — the deny-workflow-in-plan-mode scenario needs plan mode."
	tokens:  2094
}, {
	date:    "2026-10-09"
	version: "2.2.1"
	text:    "Stage spawn names scriptPath `<base dir>/workflows/demo.js`; a bare relative path failed the Workflow call."
}, {
	date:    "2026-10-03"
	version: "2.2.0"
	text:    "Requires gate, heartbeat: deploy skips the skill without them."
	tokens:  2094
}, {
	date:    "2026-10-02"
	version: "2.1.3"
	text:    "kill-repeated-launder launches the laundered stage eight times: heartbeat kills on a check's eighth failure, answers that launch with the kill reason shown to the user, and the next prompt resets the run; in an interactive session a stage launch after a typed prompt follows the relay primer and a turn end."
	tokens:  2094
}, {
	date:    "2026-09-29"
	version: "2.1.2"
	text:    "value_provenance reason names the unsalted producer call; a salted call never counts."
	tokens:  1879
}, {
	date:    "2026-09-29"
	version: "2.1.1"
	text:    "recover-laundered-workflow ends its turn after the deny; the recovery arrives as a Stop block or with the user's continue."
}, {
	date:    "2026-09-29"
	version: "2.1.0"
	text:    "report-value-judgment and recover-laundered-workflow first print the session id and the /runtime command for another session, then wait until the user says continue."
}, {
	date:    "2026-09-29"
	version: "2.0.0"
	text:    "scenario names state the layer's answer and the violation: pass-clean-run, deny-laundered-workflow, deny-producer-skipped, deny-edit-after-stage, block-stop-before-close, deny-root-artifact-write, deny-worker-hand-spawn, deny-workflow-in-plan-mode, report-value-judgment, recover-laundered-workflow, kill-repeated-launder."
}, {
	date:    "2026-09-29"
	version: "1.0.0"
	text:    "initial version: a minimal real protocol — producer, one Workflow stage with one worker, closer — whose enforce block uses every gate rule, check kind and a judgment point; the scenario arg commits one deliberate violation so each enforcement layer is observable in one short run."
}]
enableDelegation: "never"
env: {
	DEMO_CLOSE: {
		interface: "demo-close@1"
		required:  true
	}
	DEMO_VALUE: {
		interface: "demo-value@1"
		required:  true
	}
}
frontmatter: [{
	key:   "name"
	value: " heartbeat-demo"
}, {
	key:   "description"
	value: " Show heartbeat's enforcement layers acting — one short run per scenario, clean or with a deliberate violation. Trigger on /heartbeat-demo. Args — scenario: pass-clean-run|deny-laundered-workflow|deny-producer-skipped|deny-edit-after-stage|block-stop-before-close|deny-root-artifact-write|deny-worker-hand-spawn|deny-workflow-in-plan-mode|report-value-judgment|recover-laundered-workflow|kill-repeated-launder (default pass-clean-run)."
}, {
	key:   "author"
	value: " Kevin Horst"
}, {
	key:   "version"
	value: " 2.5.0"
}, {
	key:   "argument-hint"
	value: " scenario:deny-laundered-workflow"
}, {
	key:   "format"
	value: " 2"
}, {
	key:   "title-code"
	value: " HD"
}, {
	key:   "acdsl-context"
	value: " RULE-SKILL-012, RULE-SKILL-013, RULE-SKILL-014"
}, {
	key:   "allowed-tools"
	value: " Read, Workflow, Bash($SKILL_HEARTBEATDEMO_DEMO_CLOSE demo-close *), Bash($SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value *), Bash(printenv CLAUDE_CODE_SESSION_ID)"
}]
preamble: """
	# Heartbeat demo

	Runs a four-call protocol under heartbeat's enforcement and the tool-policy gate: producer, stage, closer. The
	`scenario` arg names one violation to commit on purpose; the scenario table states what each layer answers.
	"""
requires: ["gate", "heartbeat"]
sections: [{
	title: "Routing"
	blocks: [{
		routing: {
			pipeline: {
				standalone: true
			}
			preconditions: ["`$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value --seed x` prints a value", "heartbeat serving for the heartbeat rows; the gate rows need only the tool-policy hook"]
			purpose: "smoke test and showcase of heartbeat's enforcement and the tool-policy gate rules in one short run."
			routes: [{
				for: "watching or answering findings live"
				use: "/runtime"
			}, {
				for: "a real review under the same enforcement"
				use: "/railroad-review"
			}]
		}
	}]
}, {
	title: "Args"
	blocks: [{
		args: {
			bullets: true
			rows: [{
				name:   "scenario"
				effect: "`scenario:<name>`, default `pass-clean-run` — the violation to commit, per [Scenario table](#skill-heartbeatdemo-def-001)"
			}]
		}
	}]
}, {
	title: "Definitions"
	blocks: [{
		entry: {
			id: "SKILL-HEARTBEATDEMO-DEF-001"
			payload: {
				info:   "markdown"
				marker: "```"
			}
			role:  "payload"
			title: "Scenario table"
		}
	}, {
		entry: {
			id: "SKILL-HEARTBEATDEMO-DEF-002"
			payload: {
				info:   "js"
				marker: "```"
			}
			role:  "payload"
			title: "Primer"
		}
	}]
	kind: "context"
}, {
	title: "Watcher"
	blocks: [{
		entry: {
			id: "SKILL-HEARTBEATDEMO-WATCH-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "only `report-value-judgment` and `recover-laundered-workflow`; every other scenario skips this."
			}]
			role:      "step"
			statement: "RUN `printenv CLAUDE_CODE_SESSION_ID`; STATE the session id and `/runtime session: <id> mode: act` to run in another session; END turn until the user says continue."
			title:     "Await watcher"
		}
	}]
	kind: "phase"
}, {
	title: "Protocol"
	blocks: [{
		entry: {
			id: "SKILL-HEARTBEATDEMO-PROC-001"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`deny-laundered-workflow`, `recover-laundered-workflow`, `kill-repeated-launder`: run it again with `--salt other`; keep that value instead."
			}, {
				label:  "param"
				marker: "*"
				text:   "`deny-producer-skipped`: run PROC-002 first, then this step."
			}]
			role:      "step"
			statement: "RUN `$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value --seed <session id>`; KEEP `value`."
			title:     "Produce"
		}
	}, {
		entry: {
			id: "SKILL-HEARTBEATDEMO-PROC-002"
			items: [{
				label:  "param"
				marker: "*"
				text:   "args `{value, artifactsDir}` — artifactsDir is `<scratchpad>/heartbeat-demo/<session id>`."
			}, {
				label:  "param"
				marker: "*"
				text:   "interactive, typed-prompt turn: launch [Primer](#skill-heartbeatdemo-def-002) as inline `script`, END turn; the notification turn launches the stage."
			}, {
				label:  "param"
				marker: "*"
				text:   "`deny-worker-hand-spawn`: spawn the Agent tool with `subagent_type: heartbeat-demo-worker` instead."
			}, {
				label:  "param"
				marker: "*"
				text:   "`deny-workflow-in-plan-mode`: run in plan mode."
			}, {
				label:  "param"
				marker: "*"
				text:   "`recover-laundered-workflow`: END turn after the deny; follow the recovery exactly (Stop block or with continue), then invoke again."
			}, {
				label:  "param"
				marker: "*"
				text:   "`kill-repeated-launder`: invoke again with the same laundered value until eight launches ran."
			}, {
				label:  "Script"
				marker: "*"
				text:   "workflows/demo.js"
			}]
			role:      "spawn"
			statement: "SPAWN the stage through the Workflow tool, scriptPath `<base dir>/workflows/demo.js`."
			title:     "Stage"
		}
	}, {
		entry: {
			id: "SKILL-HEARTBEATDEMO-PROC-003"
			items: [{
				label:  "param"
				marker: "*"
				text:   "`deny-edit-after-stage`: write one scratch file now."
			}, {
				label:  "param"
				marker: "*"
				text:   "`deny-root-artifact-write`: write `<artifactsDir>/artifact.json` yourself."
			}, {
				label:  "param"
				marker: "*"
				text:   "`block-stop-before-close`: end the turn now; on the block, continue with PROC-004."
			}]
			role:      "step"
			statement: "READ the stage's `artifact` path from its return; STATE it."
			title:     "Read artifact"
		}
	}, {
		entry: {
			id:        "SKILL-HEARTBEATDEMO-PROC-004"
			role:      "step"
			statement: "RUN `$SKILL_HEARTBEATDEMO_DEMO_CLOSE demo-close --dir <artifactsDir>`."
			title:     "Close"
		}
	}, {
		entry: {
			id:        "SKILL-HEARTBEATDEMO-PROC-005"
			role:      "step"
			statement: "STATE the scenario's expected outcome from [Scenario table](#skill-heartbeatdemo-def-001) and every deny, block or message received."
			title:     "Report"
		}
	}]
	kind: "phase"
}, {
	title: "Model"
	blocks: [{
		model: {
			effort: "low"
			reason: "fixed four-call protocol, no judgment"
			tier:   "small"
		}
	}]
}]
enforce: {
	anchors: {
		closed: {
			input: [{
				field:   "command"
				pattern: "^$SKILL_HEARTBEATDEMO_DEMO_CLOSE demo-close --dir "
			}]
			tool: "^Bash$"
		}
		edit: {
			tool: "^(Edit|Write|NotebookEdit)$"
		}
		produced: {
			capture: [{
				name:    "value"
				pointer: "/value"
			}]
			input: [{
				field:   "command"
				pattern: "^$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value --seed \\S+$"
			}]
			tool: "^Bash$"
		}
		stage: {
			input: [{
				field:   "scriptPath"
				pattern: "/heartbeat-demo/workflows/demo\\.js$"
			}]
			tool: "^Workflow$"
		}
	}
	checks: {
		close_before_stop: {
			after:    "stage"
			kind:     "stop"
			reason:   "heartbeat-demo: run `$SKILL_HEARTBEATDEMO_DEMO_CLOSE demo-close --dir <artifactsDir>` before ending"
			recovery: "heartbeat runtime ({finding}): run demo-close, then end the turn"
			require:  "closed"
		}
		no_edit_after_stage: {
			deny:     "edit"
			from:     "stage"
			kind:     "phase"
			reason:   "heartbeat-demo: no edits between the stage and demo-close"
			recovery: "heartbeat runtime ({finding}): do not edit; run `$SKILL_HEARTBEATDEMO_DEMO_CLOSE demo-close` next"
			until:    "closed"
		}
		producer_first: {
			after:    "produced"
			kind:     "requires"
			on:       "stage"
			reason:   "heartbeat-demo: run `$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value` before the Workflow"
			recovery: "heartbeat runtime ({finding}): run the producer first, then the Workflow"
		}
		value_provenance: {
			arg:      "/args/value"
			capture:  "value"
			from:     "produced"
			kind:     "provenance"
			on:       "stage"
			reason:   "heartbeat-demo: args.value must be the value the last unsalted `$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value --seed <session>` call printed; a `--salt` call is no producer"
			recovery: "heartbeat runtime ({finding}): pass exactly the value `$SKILL_HEARTBEATDEMO_DEMO_VALUE demo-value` printed: {evidence}"
		}
	}
	gate: {
		planModeDenied: ["Workflow"]
		rootDenied: ["*/heartbeat-demo/*/closed.json"]
		workflowOnly: ["heartbeat-demo-worker"]
		writers: [{
			agentTypes: ["heartbeat-demo-worker"]
			glob: "*/heartbeat-demo/*/artifact.json"
		}]
	}
	judgment: {
		value_check: {
			anchor: "stage"
			evidence: ["value", "arg:/args/value"]
			question: "Is the value passed to the stage the producer's value?"
			send:     "heartbeat runtime ({finding}): the runtime agent asks you to re-check the value you passed: {evidence}"
		}
	}
}
smoke: {
	expect: {
		anchors: ["closed", "produced", "stage"]
		minChecks: 0.75
	}
	invoke: "scenario:pass-clean-run"
}
