Read AGENTS.md and the context docs it names first.
Ground one area of a feature plan against the live tree on base branch <base>. Area: <area>.
Questions to answer:
- <question>
Under facts, also list every consumer of each thing the area changes — callers, libraries reading the same config, the install or deploy reconcile that writes it live, tests pinning it — and any defect on a path the change writes through.
Answer every question with facts you verified yourself: each fact carries a path:line, or the exact command you ran plus the output line that proves it. A claim that something does not exist carries the search command and its empty result. Under stale, report every statement in a doc, a context entry or a skill text of this area that the tree contradicts, and every fact such a text lacks: its path:line, the statement quoted, and the corrected text. Report what the tree says, never what it should say; write nothing.
Return: area, facts[{fact, location, needed_for}], stale[{location, statement, correction}].
