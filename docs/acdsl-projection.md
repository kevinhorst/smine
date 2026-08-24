# ACDSL Projection Alternatives — Idea Findings

> **Mode:** open
> **Date:** 2026-08-24
> **Verdict:** survives-with-conditions — the "universal file header" claim is dead; the goal survives as a choice between three shapes, pending classification of the confusion evidence.
> **Reference:** [acdsl-spec.md](acdsl-spec.md) — this document is delta-only against it.

## Claim

The projection channel (spec §6.2) has two defects: it covers only formats with a comment syntax, and it mutates the working-copy byte stream, which regularly confuses the agent. The additionalContext fallback is rejected as harness-coupled and not self-contained. Hypothesis under test: a universal "file header" — metadata attachable to any file on any platform, writable without touching content, readable by any agent — could replace projection as the delivery channel.

## Flags

- "Not self-contained / harness-dependent" was established as the disqualifier for additionalContext before examination. The incumbent projection is equally harness-coupled — it exists only because a PreToolUse Read hook fires (spec §6.5), gated by `ACDSL_PROJECT_ENABLED`. The criterion, taken literally, indicts the incumbent too; the real objection is probably **binding strength** (additionalContext is a side annotation outside the artifact the agent treats as ground truth, inconsistent with view-equals-disk), not harness coupling per se. Unresolved — see Open point below.
- "If that even exists ... can be read by any agent as well, reliably" — honest uncertainty, stated in the dump. Resolved: it does not exist (Shape A).

## The trilemma

The goal decomposes into: in the agent's **primary read path**, **content-clean** (no byte mutation), **harness-independent**. No channel satisfies all three, because the only thing every agent on every platform reliably reads is the byte stream itself. Everything outside the bytes is either harness-mediated (hooks, tool results, MCP) or OS-mediated (mounts, filters, metadata). Every viable shape relaxes exactly one constraint; the design decision is which one.

## Shapes

### Shape A — out-of-band file metadata ("file header"). Dead.

What the file-header idea resolves to. There is no universal file header: a file, to every OS, is a byte stream plus limited metadata; format-specific headers (PNG chunks, EXIF, PE) *are* content — writing them alters the byte signature the shape is supposed to protect. The out-of-band candidates — xattrs (macOS/Linux), NTFS alternate data streams, sidecar files — fail harder:

- Not tracked by git: the world model (spec §4.2.1) is `git ls-files`; the metadata would not survive a clone.
- Dropped by zip/rsync/FAT-family filesystems; ADS is NTFS-only.
- Decisive: **no agent's Read tool surfaces them.** An agent must be separately instructed to run `xattr -p`, which is strictly weaker binding than additionalContext — the channel this shape was invented to beat.

Refuted by construction, not merely untested. No cheapest test required.

### Shape B — filesystem-level projection (virtual FS overlay)

**Mechanism.** The agent's working tree is a mount; the backing store holds clean bytes; the filesystem layer injects the projection block into read results. View-equals-disk holds from the agent's perspective; git and the byte signature never see a block; strip becomes structural (nothing to strip); format coverage is total for the agent.

**Platform status (verified 2026-08).** Linux: FUSE. Windows: ProjFS (Microsoft's "Projected File System", built for VFS-for-Git) or WinFsp. macOS: macFUSE 5.3.3 (2026-07) runs kext-free on the FSKit backend, macOS 15.4+ ([macFUSE](https://macfuse.github.io/), [FSKit](https://developer.apple.com/documentation/fskit), [Fuse-T](https://www.fuse-t.org/)).

**Assumes.**
1. Per-process view discrimination is solvable. If the agent's Bash children (`go build`, `jq`) read through the mount, injected blocks break every non-commentable format for compilers and parsers — today's format ceiling recreated one layer down. Per-PID views are possible in FUSE but hairy; getting the boundary wrong creates a new confusion class (Read sees the block, `cat` does not).
2. Git operates on the backing store, not the mount.
3. The infrastructure weight (a filesystem driver per platform, mount lifecycle per worktree) is acceptable for a rule-delivery mechanism.

**Cheapest test.** A passthrough FUSE mount over one repo that appends one line to reads of `*.go`; run an agent session on top; observe read consistency and toolchain behavior. About a day.

### Shape C — MCP projected-read server

**Mechanism.** A small MCP filesystem server serves projected reads; the harness's native Read is deny-listed for governed paths. Disk untouched; all formats coverable (the block arrives in the tool result, not in the file); MCP is the one interface every relevant harness speaks (Claude Code, Codex, Cursor, Copilot). This is additionalContext with the binding problem fixed: the block arrives *inside* the content the agent treats as the file, not beside it.

**Assumes.**
1. Agents actually route reads through the server — enforcement is per-harness configuration, so the harness coupling returns through the back door, thinner.
2. In-tool-result delivery binds as strongly as on-disk delivery. Plausible, **unmeasured** — no §8.1 round ever tested this arm.

**Cheapest test.** One A/B round on the existing harness: MCP-read arm vs. on-disk-projection arm, on the anti-prior rule that produced the 0/6 → 6/6 differential (spec §8.1).

### Shape D — in-band projection domesticated by git clean/smudge filters

**Mechanism.** Reframe: the byte-signature problem may not be "bytes changed" but "bytes changed *visibly to git*" — diffs show the block, staging leaks it, the agent tries to clean it up (leak-copy, spec §10.7). Git clean/smudge filters are projection as a git-native mechanism: smudge injects the block on checkout, clean strips it on stage, `git diff` runs through clean so diffs never show a block, and staged leaks become structurally impossible. `ValidateStagedClean`, `project -strip`, and the make-audit strip discipline get deleted rather than maintained.

**Properties.** Platform-universal (git is the one runtime already required). As self-contained as ACDSL gets: `.gitattributes` is in-repo; one `git config filter.acdsl` line per clone is the whole install.

**Assumes.**
1. The agent confusion is mostly git-layer (diff/stage/status noise), not read-layer.
2. `clean(smudge(x)) == x` holds exactly — projection is already idempotent (spec §6.2), so it does.
3. The format ceiling is acceptable: this shape deliberately does **not** solve JSON/binaries, which stay gate-only.

**Cheapest test.** Enable the filter for `*.go` in one worktree, run normal sessions, count confusion incidents against baseline.

## Load-bearing assumption

The dumped idea rests on the assumption that a channel exists satisfying all three trilemma constraints. That assumption is **false** (refuted, not untested); Shape A dies with it.

What actually carries weight across the surviving shapes is an empirical unknown for which evidence already exists but is unclassified: **what "confuses the agent regularly" concretely consists of.** Git-layer incidents (diffs, staging, hash/`wc` mismatches, leak-copy) → Shape D solves them at near-zero cost and the format ceiling remains a separate, smaller problem. Read-layer incidents (the agent reasoning wrongly about content it sees) → only B or C help. Choosing between a one-line git config and a virtual filesystem without this classification is guessing.

## Next step

Pull the actual confusion incidents from session transcripts (peek-mcp / smine batch reports — "regularly" implies the evidence exists) and classify each as git-layer or read-layer. No prototype, no design doc. The tally decides which trilemma constraint is worth relaxing, and therefore which single shape earns its cheapest test.

## Open point

Whether "self-contained" really means *binding strength* (in-channel vs. side annotation) rather than *harness independence*. If binding strength is the true criterion, Shape C is acceptable and pure-annotation channels are not — which changes the ranking. Needs an explicit decision at concept time.
