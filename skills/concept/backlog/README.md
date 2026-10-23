# backlog — design notes

- **STORE-001/002** — one JSON document, schema-shaped (`schema.json`, checked in claude-configs by `make rules ARGS="store-check"`): the
  config server's Backlog board (`internal/backlog`) loads, validates and saves it; the 1.x markdown tables had no cell
  for a paragraph, so the dump text was dropped and 24 titles were cut to board width on 2026-09-24 — the description
  field and the no-loss rules (INTAKE-004/005, MERGE-004, EMIT-005/006) exist because of that loss.
- **STORE-003** — epic = box: solo operator, ordering between boxes and velocity are team ceremony with no referent here;
  the board and the skill both create boxes.
- **STORE-004** — mirrors the smine convention: rejected proposals are permanent anti-re-proposal memory; dedupe consults
  the archive so dead ideas resurface as history, not as new rows.
- **PROPOSE-002** — one correction round: item-by-item interrogation of a 20-item dump is the failure mode the skill
  exists to remove; the single gate keeps priority calls (the subjective part) with the user at fixed dialog cost.
- **CLASSIFY-001** — `direct` only for S: protects the ordering doctrine (README § Skill map) from drive-by
  implementations of L items.
