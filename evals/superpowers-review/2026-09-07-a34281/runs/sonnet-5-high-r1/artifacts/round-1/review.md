# Review: smine v1.1.4 → v1.2.1 (b4981b8..a342811)

## Methodology note

This range is 178 files / ~7,300 inserted lines across ~10 independent feature plans
(`plans/v1-2-1/*`). I reviewed it in passes rather than line-by-line: full read of every
plan document, then deep-dive diff review + code read on the highest-risk/most-complex
units (registry read-through, status-cache fingerprinting, the squash-containment verdict
layer, the ACDSL self-management policy engine, casual-audience rename, sessions
archiving, peek multi-profile wiring, profile/style settings), with a lighter pass
(diff read, no full trace) over the remaining templates, docs, and routine/plist changes.
Build, `go vet`, and the full `go test ./...` suite were run against a **clean worktree of
the actual head commit** (see Critical #1 for why not the working copy directly).

## Important: the review checkout is not clean

The `arm-wt` checkout handed to me for this review has substantial **uncommitted, dirty
working-tree state** unrelated to the reviewed range — `git status` shows modified
`acdsl/policy.json`, `acdsl/registry.json`, `acdsl/rules.acdsl`, several `context/*.md`
files, plus untracked files. Most seriously, the working copy's `acdsl/registry.json` has
been overwritten with **dist-style rewritten argv** (`bin/verifiers/gofmt` instead of
`go run ./cmd/acdsl/verifiers/gofmtwrap`) and is missing 8 verifier entries the committed
file has (`id-grammar`, `plan-format`, `symbol-exists`, `symbol-coverage`, `golden-diff`,
`lint-tags`, `runbook-format`, `forbid-addr-lit`). Running the test suite directly in
`arm-wt` fails (`TestDistShipsReachCoveredSliceWithBinaries`) because of this.

I confirmed this is **not part of the reviewed commit**: `git diff` between the two
commits on `acdsl/registry.json` is empty, and a fresh `git worktree add` of the head
commit builds, vets, and passes `go test ./...` cleanly (including the test above). This
is an environment/operational problem with the `arm-wt` copy itself (something ran a
dist-shaped write against the repo's own tracked files, uncommitted), not a defect in this
PR. Flagging it because whoever owns that checkout should discard/investigate it before
using it for anything else — a `make audit` run there today would fail on unrelated,
uncommitted corruption.

## Strengths

- **Plan fidelity is unusually high.** For the units I traced end-to-end — `Registry`
  read-through + write-modify-write (`internal/repos/registry.go`), the status-cache
  fingerprint invalidation (`internal/server/statuscache.go`, `repos.go`), the
  `git merge-tree` squash-containment layer (`cmd/worktrees/_lib/verdict.sh`), the
  `casual` audience rename with migration (`sync_skills.sh`, `presentation.go`), and the
  sessions archive/unarchive/delete flow (`internal/sessions/sessions.go`) — the shipped
  code matches its plan's decisions and example implementations closely, including
  subtle details (e.g. the fingerprint check counting an error as "changed, never stale";
  `DropBranches` deliberately keeping the pre-removal fingerprint so the *next* load
  reconverges rather than the cache lying forever).
- **`internal/acdsl/policy.go`'s `CheckPolicy` closes a real hole the plan's own pseudocode
  didn't spell out**: it re-reads the *base-ref's* `acdsl/policy.json` as the authoritative
  mode (`basePolicy`) rather than trusting the working tree's copy, so an agent branch
  cannot escape gated/strict enforcement by editing or deleting its own local
  `policy.json` — the very self-reference risk the plan's D2 worries about is closed one
  layer further than the plan text describes. This is verified by
  `TestCheckPolicyBaseModeSurvivesWorkingWeakening`, so it's a deliberate, tested
  improvement, not an accidental deviation.
- **Reserved-name validation, casual-lockout, and squash-containment all extend safety
  gates without weakening existing ones** — in every case I checked, the new logic is
  additive (more things get classified as safe/explained) and layered so a fail-open
  convenience layer (hooks) sits in front of a fail-loud authoritative layer (the
  `bin/acdsl` binary, `CheckPolicy`), matching the stated design principle.
- **Test coverage matches the plans' test tables closely** — e.g. every case listed for
  the repo-cache-reload plan appears as a named test in `registry_test.go`
  (`TestRegistryRefreshOnExternalEdit`, `TestRegistryAddPreservesExternalEdit`,
  `TestRegistryDeleteStaysDeletedAcrossInstances`, `TestFingerprint`,
  `TestWorktreeCounts`), and the ACDSL policy plan's ~16-case `TestCheckPolicy` table is
  present nearly verbatim in `policy_test.go`.
- Full suite is green on a clean checkout: `go build ./...`, `go vet ./...`, and
  `go test ./...` all pass with no failures.
- The two-file split for casual UI work (`presentation-profile.md` declared vs
  `style-profile.md` learned) and the "reversal errata" documented directly in the
  repo-cache-reload plan's changelog (D4 flipped mid-plan by the user, cleanly recorded)
  show good discipline about keeping plans authoritative over improvisation.

## Issues

### Critical (Must Fix)

None found in the reviewed commit range itself. (See "Important" note above for the
checkout-state issue, which is environmental, not a code defect in this diff.)

### Important (Should Fix)

1. **Undocumented scope-creep in `cmd/worktrees/_lib/verdict.sh`'s `filter_untracked`**
   (was `count_untracked`). Beyond the planned squash-containment work
   (`repos-routines-improvements` plan), this diff also silently adds exclusion of
   `.serena/` and `.DS_Store` entries from the UNTRACKED column / removal-safety count.
   This isn't mentioned in *any* plan document under `plans/v1-2-1/`. It's a plausible,
   low-risk fix (tool droppings shouldn't count as "dirty"), but it changes the
   remove-safety gate's behavior (a worktree with only `.serena/`/`.DS_Store` cruft now
   reads as clean and becomes removable) without a driver, decision, or test case backing
   it. Confirm this was intentional and, if so, add it to a plan/changelog — the
   remove-safety gate is exactly the kind of surface where undocumented widening deserves
   a paper trail.
2. **`internal/acdsl/git.go`'s `GitRevParse` is dead in production code** — it's declared,
   exported, and exercised only by `git_test.go`; nothing in `policy.go` or elsewhere
   calls it (the plan's own `policyBase` pseudocode implies it'd be used to validate a
   candidate before diffing, but the shipped `policyBase` instead relies on
   `GitMergeBase` failing for an unresolvable ref). Not a bug, but it's a public API
   surface with no caller — either wire it in where the plan implied it was needed, or
   drop it.

### Minor (Nice to Have)

1. `internal/server/presentation.go`'s `saveProfileSelection` and
   `internal/config/installenv.go` both read from repo-relative paths
   (`settings/claude_code/presentation-profile.de.md`, `install.env`) rather than an
   injected root. This matches an existing repo convention (the configserver's
   `WorkingDirectory` is pinned to the repo root by the launchd plist / install script),
   so it's not a new problem, but it's a second and third instance of the same implicit
   contract — worth a one-line doc comment noting "assumes CWD == repo root" so a future
   caller (e.g., a test or a different launcher) doesn't get bitten silently.
2. `handleReposAdd`'s new `git init -b main` side effect (for a bare folder with no
   `.git`) runs unconditionally inside the repo-op closure whenever `path` exists as a
   directory without `.git` — including, e.g., a `node_modules`-only folder a user
   fat-fingers into the picker. Low risk (it's opt-in via the Add form), but there's no
   confirmation step before mutating a directory on disk with `git init`; worth a note in
   the UI copy if not already present (I didn't verify the template copy for this).
3. Several new German i18n catalog entries mix straight sentence punctuation with the
   more terse existing entries (e.g., the long confirm strings for archive/delete) — not
   a defect, just worth a proofreading pass by a native speaker before it reaches a real
   casual/German install, since i18n prose quality isn't mechanically checkable here.

## Recommendations

- Given the size of this range (10 plans in one release), consider whether future
  releases benefit from landing plan-sized PRs instead of one combined diff — nothing
  here suggests it caused a real defect, but it does make a single review pass
  materially harder to keep rigorous end-to-end, and the "reversal errata" pattern (D4 in
  the repo-cache-reload plan) suggests plans are already evolving faster than a
  single-commit review can track.
- The `arm-wt` checkout's dirty state (see above) is worth a five-minute investigation on
  its own — if something in this repo's own tooling (a script, a stray `make` target, an
  agent session) can overwrite `acdsl/registry.json` with dist output in place, that's a
  footgun worth closing even though `writeRules`'s "refuse to overwrite an unheadered
  file" guard should prevent `acdsl dist -dest .` specifically from doing it.

## Assessment

**Ready to merge?** Yes.

**Reasoning:** No correctness, security, or regression issues found in the reviewed
commit range; build/vet/full test suite pass cleanly on a clean checkout of the head
commit, and the two Important items are low-severity paper-trail/dead-code cleanups, not
blockers. The one alarming-looking signal (a failing test) traced to contamination in the
review checkout itself, not the code under review.
