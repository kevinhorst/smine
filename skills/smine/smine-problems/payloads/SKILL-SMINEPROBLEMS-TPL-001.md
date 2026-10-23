{
  "id": "skills/fimplement-smoke-before-deploy",
  "kind": "skills",
  "title": "Run a changed skill's smoke before it is deployed",
  "problem": "Skill edits are deployed without a single run, so a broken check surfaces on the next real run.",
  "backlog": "B-022",
  "evidence": [
    {
      "session": "00000000-0000-0000-0000-000000000000",
      "quote": "the enforce check failed on the first real run after the edit",
      "finding": 2
    }
  ],
  "fix": "A smoke run before deploy catches the broken check at edit time.",
  "sessions": ["00000000-0000-0000-0000-000000000000"],
  "target": "skills/implement/fimplement/fimplement.cue",
  "change": "Add a verify step that runs make skill-smoke for every skill whose <leaf>.cue the run changed before it commits.",
  "notes": "make skill-render SKILL=fimplement; bump version and the newest changelog row together; the smoke block is unchanged."
}
