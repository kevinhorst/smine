# Package Commit

## Design notes

- **SKILL-PACKAGECOMMIT-STRIP-001**: the check gate refuses staged blocks, this step prevents the red instead of reacting
  to it.
- **SKILL-PACKAGECOMMIT-STRIP-003**: otherwise a block re-injected between the strip and the stage is committed — the
  initial strip runs before validation, but any audit/build after it re-projects the blocks into governed files.
- **SKILL-PACKAGECOMMIT-IDENT-004**: files staged before the run would otherwise ride along in the first group's commit;
  an empty index makes every commit hold exactly its own group.
- **SKILL-PACKAGECOMMIT-RULES-004**: rewritten SHAs break the user's continuous cherry-picking out of worktrees; one
  commit per logical change.
