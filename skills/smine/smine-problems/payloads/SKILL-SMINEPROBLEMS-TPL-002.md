1. READ <store>/backlog/backlog.json; id = highest B-NNN across sections and archive plus one; today = run date; self = the output of `echo $CLAUDE_CODE_SESSION_ID`.
2. ITEM: id, added today, deps [], description "<problem>\nDone when: <first direction's title>", evidence [<session id>#<finding> per evidence note], link "", pri "high", route "/fdesign", size "M", status "shaped", theme "smine", title "<problem title, at most 120 runes>", mined {by: self, date: today, sessions: [<evidence session ids>]}.
3. APPEND ITEM to the items of the section named Icebox.
4. MATCHED ROW instead: set its mined.date to today and add the new session ids to mined.sessions (create mined with by: self when absent, keep an existing by); touch no other field.
5. SET updated today; APPEND {date: today, delta: "<id> added from smine problem <problem id>"} (or "<id> enriched from smine problem <problem id>") to changelog.
6. WRITE the file in the canonical form of the backlog skill's template (two-space indent, keys in template order).
