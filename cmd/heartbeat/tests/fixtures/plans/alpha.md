# Alpha — Implementation Plan

## Changes
| File | Kind | Entry |
| :--- | :--- | :--- |
| `src/shared.rs` | modified | [§1](#changes) |
| `skills/x/SKILL.md` + `skills/x/changelog.json` | modified | [§2](#changes) |
| `plans/2026-09-27-alpha-aaaaaaa/2026-09-27-alpha-wip-aaaaaaa/design/raw.md` | new | this plan |

### Shared parser (modified)
location: `src/shared.rs`

```diff
 pub fn parse(input: &str) -> Vec<String> {
     // ...
+    let input = input.trim();
     input.lines().map(str::to_owned).collect()
```

### Skill version (modified)
location: `skills/x/SKILL.md`

```diff
-version: 1.0.0
+version: 1.1.0
```
