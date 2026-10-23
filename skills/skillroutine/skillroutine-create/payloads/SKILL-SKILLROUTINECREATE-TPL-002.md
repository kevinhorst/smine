You audit the ROLE tag of every entry in the attached draft skill body. Apply per entry, in order:
1. MUST test — insert MUST / MUST NOT / NEVER after the actor. Meaning unchanged? rule. A command dressed as an
   invariant fails: "The plan MUST be copied" still orders an action, so step.
2. Action test — names one action in a transcript? the spawning agent's own: step; inside a spawned agent: substep; the
   spawn event itself: spawn.
3. Guard tests — condition met, then halt: stop. Test a condition then repair, no halt: verify. Conduct measured
   statistically, not gate-checked: quality.
4. Vocabulary test — no firing moment, defines a term or data shape: def; template or example block only: payload.
5. Placement tests — a flow entry inside a context section; a pred/post clause on a def or payload; a post on a rule or
   quality; a register mismatch (rule without MUST, step without a leading UPPERCASE verb, stop not "If ... => STOP:").
Worked examples:
- "No deliverable is written before approval" tagged [step] — MUST-insertable with meaning intact: a rule (or a stop
  when a halt is meant) wearing step clothes.
- "COPY the approved <leaf>.cue out of the plan" tagged [rule] — leading imperative, one transcript action: step.
- "Directory skills/<group>/<name>/ — the leaf dir is the name" tagged [step] — no action, vocabulary: def.
Return one JSON line per entry: {"id", "verdict": "agree|dispute", "role_claimed", "role_probed", "probe", "example"}.
Dispute only with a concrete counter-reading; never re-litigate content — classification and placement only.
