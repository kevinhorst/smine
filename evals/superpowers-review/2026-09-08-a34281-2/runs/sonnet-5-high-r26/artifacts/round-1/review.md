# Review: smine v1.1.4 → v1.2.1

Range reviewed: `b4981b88..a342811b` (excluding `sessions/`, `evals/`, `proposals/`,
`plans/`) — 178 files, +7314/-749. Reviewed in passes: (1) plan TLDR/Scope sections for
all ten `plans/v1-2-1/*` dirs to establish intent, (2) full diff read for every Go
source/test file, every shell script, and `internal/server/templates/*`, (3) targeted
reads of the remaining `skills/*/SKILL.md` + `changelog.json` docs. All findings below
are grounded in the actual diff and, where cited, the actual file at HEAD in this
checkout — not the plan's description of it.

## Strengths

- **`internal/repos/registry.go`** — the read-through rework (`readRegistrySnapshot`,
  mtime+size `refreshIfChanged`, read-modify-write `Add`/`Remove`) is a clean,
  correctly-sequenced fix for FACT-REPO-ARCH-016 (stale in-memory slice resurrecting
  deleted repos). Atomic save-then-restat-then-apply is race-safe against a concurrent
  external writer.
- **`internal/server/statuscache.go`** + `repos.Fingerprint`/`WorktreeCounts` — the
  cache-invalidation design is exactly what `plans/v1-2-1/2026-08-30-repo-cache-reload-.../design/change-repo-cache.md`
  specifies: a cheap git fingerprint gates reuse, `DropBranches` keeps the pre-removal
  fingerprint deliberately stale so the next plain load re-scans. Good test coverage
  (`statuscache_test.go`) including the "unknown repo is a no-op" edge cases.
- **`cmd/worktrees/_lib/verdict.sh`** squash-containment layer (`squash_contained`,
  the layer-4 loop in `evaluate_branch`) — traced this end-to-end: `squashed_n` is
  correctly subtracted out of `unpicked_n` (verdict.sh:474-484) before `eval_unpicked`
  is published, and `candidate_fully_explained`/the IN-list star rebuild both key off
  the same `is_explained` marks layer 4 sets. A branch-level squash-merge is proven
  safe without weakening the per-commit detector for genuinely unpicked work. This is
  a non-trivial piece of logic done correctly.
- **`internal/peek/peek.go`** (`SessionIndex`, `ByBranch`/`ByCwd`) and its consumer
  `internal/server/repos.go:worktreeSessions` — correctly implements the
  FACT-REPO-ARCH-017 fix (prefer branch identity, fall back to cwd) consistently on
  both sides of the API.
- **`internal/server/profilesettings.go`** — the style-test call passes the token path,
  prompt, and model as separate `bash -c … "$1" "$2" "$3"` positional arguments rather
  than interpolating them into the script string, which avoids shell injection through
  the style-test prompt.
- `internal/sessions/sessions.go` archive/unarchive/delete (`validateScopeName`,
  `ArchiveScope`, `DeleteScope`, `UnarchiveScope`) correctly blocks path traversal and
  reserves the `archived` name; `DeleteScope` (the sole destructive path) is scoped to
  under `sessions/archived/` only, never a live folder.
- The `handleReposAdd` git-init-on-add path was caught in review by the plan's own
  authors mid-flight (Changelog: "Repo add now validates the name before the op") and
  the delivered code (`internal/server/repos.go:1395-1409`) does validate before
  running `git init` — the fix is actually in the diff, not just promised.

## Issues

### Critical (Must Fix)

#### 1. The ACDSL self-management policy engine's boundary check can be trivially defeated, disabling all `strict`/`gated` enforcement

`internal/acdsl/policy.go:195-216` (`policyBase`) and `internal/acdsl/policy.go:329-346`
(`CheckPolicy`).

This is the headline mechanism of the (marked-`done`) `2026-08-26-acdsl-policy-modes`
plan, whose own doc comment claims: *"The base-ref policy is authoritative — a branch
cannot weaken the mode by editing or deleting the working policy.json; the working
policy only bootstraps base resolution"* (policy.go:325-327), and `acdsl/README.md`
states outright: *"An agent branch can never self-escalate."* Both claims are false as
implemented.

`CheckPolicy` short-circuits to `return nil, nil` whenever `isBoundary` is false
(policy.go:344-346), **before** `surfaceDelta` (the check that the frozen
`policy.json`/`registry.json`/`evalgen.json` files match the base) ever runs.
`isBoundary` is computed by `policyBase` purely by **string-comparing the current
branch's name** to the name of whichever candidate ref resolved (policy.go:216:
`return base, branch != baseName, nil`).

Two independent ways to make `isBoundary` false, from an ordinary agent branch, with no
elevated access:

1. **`base_ref` self-reference.** `policyBase` picks its merge-base candidate from
   `policy.BaseRef` when it is non-empty (policy.go:196-198) — and that `policy` value
   is `LoadPolicy(root)`, i.e. read from the **working tree** (`cmd/acdsl/main.go`'s
   `runCheck`: `policy, err := acdsl.LoadPolicy(*root)`), not from the base ref. If an
   agent's working-tree `acdsl/policy.json` sets `"base_ref"` to the name of the
   agent's own current branch, `GitMergeBase(ctx, root, ownBranchName)` trivially
   resolves (merge-base of HEAD with itself is HEAD), and
   `branch != baseName` becomes `false` — `isBoundary` is false, `CheckPolicy` returns
   no diagnostics at all, regardless of mode. This is precisely "editing the working
   policy.json" that the doc comment claims is defeated by base-ref authority — it
   isn't, because the boundary determination itself runs on the unvetted working value
   before the authoritative base policy is even consulted for that decision.
2. **A local branch literally named `main`/`master`.** With `base_ref` unset (the
   shipped default — `writePolicy` in `internal/acdsl/dist.go` never sets it), the
   default candidates are `origin/HEAD, origin/main, main, master`. Any local branch
   named `main` or `master` (`git checkout -b main`) makes `branch != baseName` false
   without touching any file at all — no hook, write-guard, or permission surface is
   even in play.

Neither is a test case in `internal/acdsl/policy_test.go`: `TestCheckPolicyUnresolvableBase`
tests an unresolvable `base_ref` (hard error, not a bypass), and every other test uses
a working branch literally called `work` with `base_ref` unset or pointed at a real
distinct base. The exact self-referential scenario this finding describes is untested
and unguarded.

**Impact:** every claim this plan makes about `strict`/`gated` mode — "nothing about
the rule system is editable", "structurally closed", "an agent branch can never
self-escalate" — is false for the actual shipped code. The `acdsl-policy-guard.sh`
PreToolUse hook (`cmd/hooks/acdsl-policy-guard.sh`) is explicitly documented as
"fast feedback, fail-open by design", deferring final authority to `acdsl check`
(`CheckPolicy`) — and that authoritative path is what has the bypass. The hook is also
scoped to `Write|Edit|NotebookEdit` only, so a `Bash` write to `acdsl/policy.json`
(e.g. `printf ... > acdsl/policy.json`) isn't even hook-visible, compounding path (1).
Since this feature is the one being distributed to target repos specifically to make
their rule systems tamper-resistant against agents, a broken boundary check is a
security-relevant correctness bug in the exact sense the feature exists to prevent —
not a hypothetical edge case, but the primary threat model.

**Fix direction:** `isBoundary` must not be determined by string-comparing branch
*names*. Determine it from commit identity instead — e.g. resolve the well-known
candidates strictly from remote-tracking refs (never from the local branch's own name),
and treat the working tree as "on the boundary" whenever `git rev-parse HEAD` differs
from the resolved base commit, or the tree has any uncommitted change — never based on
what the branch happens to be called. Additionally, `policy.BaseRef` from the working
tree should only ever be used to *pick a candidate string*, and the resulting boundary
decision should be re-derived from the **base-ref policy's own recorded base_ref** (or
from a value not writable by the branch being checked) rather than from the working
copy that this whole mechanism exists to distrust.

### Important (Should Fix)

#### 2. New stage runners fail open (no `--allowedTools`) instead of failing closed when a skill manifest is missing

`cmd/bootstrap/run.sh:120-128` (`run_stage`, new file) and
`routines/smine-nightly/run.sh:308-317` (new Stage 3 `orchestrate`, added by this diff).

```sh
tools="$(routine_allowed_tools "$skill_name")"
if [ -n "$tools" ]; then
  flags=(--allowedTools "$tools")
else
  echo "no allowed-tools manifest for $skill_name; running without --allowedTools" >&2
fi
```

The entire point of both wrappers (per `plans/v1-2-1/2026-08-27-bootstrap-profile-improvements-.../design/change-headless-bootstrap.md`
DR1/DR2) is that headless `claude -p` ignores frontmatter `allowed-tools`, so the
wrapper must pass `--allowedTools` explicitly from the installed skill's manifest —
that's the fix for the exact failure mode that motivated this redesign ("headless
ignores frontmatter allowed-tools... sub-stages hit Bash/Read denials"). When
`routine_allowed_tools` comes back empty (missing/corrupt/not-yet-synced manifest), both
scripts silently proceed with **no tool restriction at all** — which is *more*
permissive than the broken behavior being fixed, not less. This is exercised on:
- `cmd/bootstrap/run.sh`'s `smine-orchestrate` stage, which commits to `main` on the
  user's main checkout, unattended, for up to an hour;
- `routines/smine-nightly/run.sh`'s new orchestrate stage, same commit authority,
  unattended, nightly.

`cmd/tests/test_bootstrap_wrapper.sh` never exercises or asserts on this branch (its
"installed manifest" test case only checks that the tools string is echoed when a
manifest *is* present — every earlier test case in the file runs with no manifest
installed at all and never asserts the fallback warning or restricted-tool behavior).

This is the identical pattern in both new pieces of code (not a pre-existing issue —
`smine`/`smine-consolidate`/`smine-apply`'s pre-existing occurrences of this same string
in `smine-nightly/run.sh` predate this diff and are legacy, out of scope per the
enforcement rule for unmodified code; the new `orchestrate` stage and the entirely new
`cmd/bootstrap/run.sh` reproduce the same weak pattern instead of hardening it).

**Fix:** a missing manifest for a stage with real write/commit authority should abort
the run (non-zero exit, clear message) rather than degrade to unrestricted tool access.

#### 3. Casual-audience "never propose edits to skills the user doesn't own" is enforced only in prose, not by a compiled check

The plan's headline security property for casual installs (DR3 in
`2026-08-27-bootstrap-profile-improvements-.../design/change-bootstrap-profile.md`):
*"the pipeline must never propose (or apply) edits to skills the user does not own... enforced at extraction (smine-skills), apply (smine-apply, hard gate), and the orchestrate judge."*
Checking the delivered diff, this entire gate lives in **skill markdown prose**
(`skills/smine/smine-apply/SKILL.md`'s "Casual lockout (hard gate)" paragraph,
`skills/smine/smine-skills/SKILL.md`, `skills/smine/smine-orchestrate/SKILL.md`) — there
is no Go/shell code anywhere in this diff that reads `presentation-profile.md`'s
`audience` field and refuses a skill-file write. Every other hard boundary added in
this release (the ACDSL policy engine, `acdsl-policy-guard.sh`, `skill-invoke-gate.sh`)
got a real enforcement point in code or a hook; this one is an instruction to the agent
that is doing the editing, which is exactly the trust model the ACDSL policy feature
in this same release exists to replace. Not necessarily wrong for a first cut (the
`smine` pipeline's own dimension skills are themselves agent-run, so "the agent reads
its own gate" is the existing pattern elsewhere in the pipeline too), but given the
stated intent ("hard gate") and the emphasis the plan places on it, this is worth
confirming was a deliberate scope decision rather than an oversight — a determined or
confused agent session has no compiled backstop if it ignores or misreads the prose
gate.

### Minor (Nice to Have)

#### 4. `Registry` struct field order breaks the repo's own alphabetical-field rule

`internal/repos/registry.go:61-67`:

```go
type Registry struct {
	path string

	mu            sync.RWMutex
	loadedModTime time.Time
	loadedSize    int64
	repos         []Repo
}
```

RULE-GOLANG-STRUCT-002 requires fields sorted alphabetically within their group; `mu`
sorts after `loadedModTime`/`loadedSize` alphabetically (`l` < `m`), so the new fields
should precede `mu`. Pre-existing `mu`/`repos` ordering was fine before this diff added
`loadedModTime`/`loadedSize` above `repos`; this is newly-introduced code, not legacy.

#### 5. `settings.json`'s blanket `ask` gate on `acdsl/**` edits was removed in favor of a narrower, tool-scoped hook

`settings/claude_code/settings.json` diff removes
`Edit(**/*.acdsl)`/`Edit(acdsl/**)`/`Write(**/*.acdsl)`/`Write(acdsl/**)` from
`permissions.ask`, relying instead on `cmd/hooks/acdsl-policy-guard.sh`. For the smine
repo itself (which runs `policy.json` mode `free`) this is a no-op change either way.
For distributed `gated` targets, the new hook is narrower than the removed
confirmation prompt in two ways worth being deliberate about: it only fires on
`Write|Edit|NotebookEdit` (not `Bash`), and in `gated` mode it only blocks the four
frozen surface files, not general rule-file edits (which is by design — gated mode
allows rule edits within scope — but means there's no interactive confirmation left
anywhere in the tool-permission layer for a `gated` repo's rule changes; the
`acdsl check` gate is the only backstop, which is the same code path Critical finding
#1 shows can be bypassed). Given finding #1, this permission relaxation compounds the
exposure for `gated` targets rather than being purely neutral.

## Recommendations

- Add an explicit regression test for `internal/acdsl/policy.go` that sets
  `working-tree policy.json`'s `base_ref` to the checked-out branch's own name (or
  checks out a branch literally named `main`) under `strict`/`gated` mode and asserts
  `CheckPolicy` still reports the surface/rule violations. This is the test that would
  have caught finding #1.
- Consider computing `isBoundary` from a git-verifiable signal (commit-reachability,
  divergence from a remote-tracking ref) rather than string identity of branch names —
  the current approach conflates "what the branch happens to be called" with "is this
  where humans are trusted to change the surface", which are unrelated facts.
- For the casual-lockout gate (finding #3), consider at minimum a fixture-based
  `acdsl` or smoke test that proves a casual-profile `smine-apply` run refuses to touch
  a skill lacking `origin: user`, even if the enforcement stays prose-driven for now —
  today nothing in CI would catch a future prose edit that silently weakens the gate.
- This diff is large enough (178 files) that a second focused pass on the
  `skills/smine/smine-orchestrate/SKILL.md` and `smine-permissions/SKILL.md` prose (both
  entirely new, both carrying real write authority over `settings.json`'s
  `permissions.allow` and the merge/reconcile logic) would be worth a dedicated review
  session — this pass treated them as lower-risk prose per the doc/test tier and did
  not fully verify every instruction they give an agent is safe under adversarial
  proposal content.

## Assessment

**Ready to merge?** No — with fixes.

**Reasoning:** The bulk of this release (repo registry, status cache, session
archiving, peek multi-profile, worktree squash detection) is well-executed and matches
its plans closely. But the ACDSL self-management policy engine — the release's most
security-relevant new feature, whose entire purpose is to make a repo's rule system
agent-tamper-resistant — has a boundary-detection bug that lets any agent branch fully
disable `strict`/`gated` enforcement, directly contradicting the mechanism's own
documented invariant ("an agent branch can never self-escalate"). That must be fixed
(and covered by a test that would have caught it) before this ships to `gated`/`strict`
target repos; the Important findings (fail-open stage manifests, prose-only casual
lockout) should be resolved or explicitly accepted before the next bootstrap/nightly
run gets unattended commit authority on `main`.
