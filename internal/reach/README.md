# reach

The deployment-reach grammar shared by ACDSL rules and context entries: which repos an entry or rule binds in and ships to. Used by the rules pack, the context sync and the ACDSL dist.

## How it works

**Values**

| Value | Meaning |
|---|---|
| `global` | binds in any repo; ships to every target |
| `always` | ships into no repo pack, only into the generated machine-global always pack under `~/.claude/context/global`, which the global-context hook injects into every session |
| `none` | disables the entry or rule: it governs no files locally and deploys nowhere |
| `a,b,c` | a comma-separated list of repo names deploys to exactly those targets; a name is the target dir's basename, the roster convention |

**Grammar**
- A list member matches `[A-Za-z0-9._-]+`; members are trimmed, and the list must not be empty.
- `global`, `always` and `none` are reserved standalone tokens and never appear as list members.
- An empty value deploys nowhere.

**This repo**
- There is no self-keyword. This repo is referenced by its own roster name, `smine`, an ordinary name: it never matches a sync target, so an entry reaching only `smine` binds here and never deploys.
