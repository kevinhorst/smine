required: id agent repo week lastActive title findings format (always 2); optional: type signal invoked_skills skipped skipReason cost dataGaps
  frustration positive permissions context; never synthesized — make sessions ARGS="mark …" owns it
cost: turns wallSeconds activeSeconds inputTokens outputTokens prompts denials, all integers
  inputTokens = input + cache_read + cache_creation; prompts = prompted_once + prompted_always; denials = rejected
  Codex without telemetry: all 0, dataGaps gains "cost: no telemetry"
findings[].dimension: skill-candidate workflow-candidate routine-candidate skill-report-card feature-design memory rule
  doc-drift harness-friction exemplar context-use other
findings[]: headline (one plain sentence ≤ 120 runes); summary (≤ 400 runes); quotes[] {text, turn};
  snippets[] {kind violation|fix|context, lang, code, source transcript|plan|diff}; contextUse {kind, channel, ruleId}
frustration[] / positive[]: {quote, trigger}
permissions: {telemetry_present, granted[] {tool, command, decision}, denied[] {tool, command}}
context: never written by the miner — make sessions ARGS="check <path> --context <transcript>" merges the used-context record
skipped session: skipped true, skipReason one word, findings []
