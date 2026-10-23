# Beta — Implementation Plan

## Changes
| File | Kind | Entry |
| :--- | :--- | :--- |
| `src/shared.rs` | modified | [§1](#changes) |
| `src/other.rs` | new | [§2](#changes) |
| `skills/x/SKILL.md` + `changelog.json` | modified | [§3](#changes) |
| `plans/2026-09-27-beta-aaaaaaa/2026-09-27-beta-wip-aaaaaaa/design/raw.md` | new | this plan |

### Shared parser (modified)
location: `src/shared.rs`

```diff
 pub fn parse(input: &str) -> Vec<String> {
     // ...
+    let input = input.to_lowercase();
     input.lines().map(str::to_owned).collect()
```

### Other (new)
location: `src/other.rs`

```rust
pub fn other() -> u8 {
    0
}
```

### Skill version (modified)
location: `skills/x/SKILL.md`

```diff
-version: 1.0.0
+version: 1.2.0
```
