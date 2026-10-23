# Review: smine 1.1.4 → 1.2.1 (`b4981b88`..`a342811b`)

Scope note up front: this range touches 178 files / ~7,300 insertions across 13 separate feature
plans under `plans/v1-2-1/`. I reviewed it in risk-tiered passes rather than line-by-line across
every file: full read-plus-verification of the plan doc and implementation for the two highest-risk
areas (ACDSL self-management policy engine; repo registry/status-cache invalidation), a solid read
of routines/launchd bootstrap and peek session attribution, a targeted read of the remaining
server/config/hook code, and a scope-creep skim of the (large) Markdown-only skill/doc changes,
which I did not fact-check clause by clause. I rebuilt the binaries, ran `go vet ./...`, and ran
`go test ./...` (29 packages, all green) and `./bin/acdsl check`/`fixtures` against a disposable
clone — never against the checkout under review, which stayed untouched (`git status` confirms no
`.go`/`.sh`/`go.mod`/`go.sum` deltas from my session).

## Strengths

- **Repo registry read-through + status-cache fingerprint** (`internal/repos/registry.go`,
  `internal/server/statuscache.go`, `internal/server/repos.go`) matches its plan
  (`plans/v1-2-1/2026-08-30-repo-cache-reload-wip-46c8d9e/design/change-repo-cache.md`) closely:
  `Add`/`Remove` correctly re-read the file under lock before applying their delta (closes the
  stale-writer resurrection bug in FACT-REPO-ARCH-016), `refreshIfChanged` stats before reading so a
  concurrent write is re-detected rather than missed, and `Fingerprint` degrades to "treat as changed"
  on any git error. Well covered by tests I could trace to their planned cases.
- **Peek session attribution** (`internal/peek/peek.go`) implements FACT-REPO-ARCH-017 exactly:
  `SessionIndex` now keys by both `cwd` and `git_branch`, `SessionsByCwd`'s signature change was swept
  cleanly to all three call sites (`bootstrap.go`, `repos.go`, `routines.go`) with zero stale
  references left behind.
- **Routine default-disabled gating** (`internal/routines/bootstrap_darwin.go`, `sched_windows.go`,
  `routines.go`) is a clean, minimal extension of the existing enable/disable machinery, with struct
  field ordering and test updates (`TestOverrideLabels`) that match the repo's Go conventions.
- The team's own instinct on the ACDSL policy design is sound in one respect I want to call out
  positively before the Critical below: the plan's original pseudocode gates on `policy.Mode` from
  the **working copy** first, which would let an agent trivially escape all checks by flipping
  `acdsl/policy.json`'s `mode` to `"free"` locally. The implementation noticed this and re-derives
  the authoritative policy from the base ref before evaluating anything — the right instinct, tested
  explicitly (`TestCheckPolicyBaseModeSurvivesWorkingWeakening`). It just doesn't go far enough (see
  Critical #1).
- `make audit`'s full chain (`build-audit`, `go vet`, `./bin/acdsl project -strip`, `./bin/acdsl
  check`, `./bin/acdsl fixtures`, `go test`) is green on a live rebuild of this exact range —
  confirmed independently, not taken on faith.

## Issues

### Critical (Must Fix)

**1. The ACDSL self-management policy gate is fully bypassable by an agent branch via a one-line edit — the feature's core promise does not hold.**

- File: `internal/acdsl/policy.go:195-219` (`policyBase`), wired from `cmd/acdsl/main.go:122,127`.
- What's wrong: `policyBase(ctx, root, policy)` uses `policy.BaseRef` — a field read straight out of
  the **working copy** of `acdsl/policy.json` via `LoadPolicy(*root)` in `cmd/acdsl/main.go:122`,
  i.e. attacker-controlled data on an agent's own branch — both to pick which ref counts as "the
  base" and to decide `isBoundary` (`branch != baseName`). If an agent sets `base_ref` to the literal
  name of its own current branch (which it always knows — it's a `claude/<slug>` pool branch it is
  checked out on), then:
  - `GitMergeBase(HEAD, <own-branch-name>)` resolves to `HEAD` itself (merge-base of a ref with
    itself), no error.
  - `branch == baseName` → `isBoundary = false`.
  - `CheckPolicy` then calls `basePolicy(ctx, root, base=HEAD, ...)`, which reads
    `git show HEAD:acdsl/policy.json` — i.e. the agent's **own** committed policy — as "the
    authoritative base policy," and then returns immediately because `!isBoundary`.
  - Every check (surface freeze on `acdsl/registry.json`/`policy.json`/`evalgen.json`, editable-scope
    restriction, verifier allowlist, overlay sanctioning) is skipped entirely.
  - This directly contradicts the code's own doc comment on `CheckPolicy` ("The base-ref policy is
    authoritative — a branch cannot weaken the mode by editing or deleting the working
    policy.json... agent worktrees on `claude/*` branches can never self-escalate") and the plan's
    TLDR ("an agent in a gated target repo... cannot touch PLAN rules, add validators, or weaken the
    policy — the loosy-goosy-rules failure mode is structurally closed").
- I reproduced this empirically with a standalone Go test against the actual `CheckPolicy` (not a
  hypothetical): on a `gated` repo, on a `claude/exploit-branch` checkout, adding a protected-scope
  (`PLAN`) rule and an unsanctioned entry to the frozen `acdsl/registry.json` is correctly flagged
  (2 diagnostics) — until the working copy's `acdsl/policy.json` also sets
  `"base_ref": "claude/exploit-branch"`, at which point the **same tree** produces 0 diagnostics.
  Test output:
  ```
  WITHOUT exploit: 2 diagnostic(s): acdsl/registry.json:1: policy(gated): acdsl/registry.json differs from the policy base ...
                                     acdsl/rules.acdsl:2: policy(gated): rule ACDSL-PLAN-101 added outside the editable scopes (scope "PLAN")
  WITH exploit (base_ref = own branch name): 0 diagnostic(s):
  --- PASS: TestCheckPolicy_BaseRefSelfReferenceBypass (0.29s)
  ```
- Why it matters: this is the entire point of the `acdsl-policy-modes` feature (`gated`/`strict`
  modes exist specifically to stop an agent from loosening its own rule system). As implemented, any
  agent that reads `internal/acdsl/policy.go` (or discovers this by trial) can neutralize `gated` and
  `strict` modes in every target repo this ships to, with a single-field edit that looks completely
  innocuous in a diff. None of the 16 test cases in `policy_test.go` exercise `BaseRef` pointed at the
  agent's own branch name (`TestCheckPolicyUnresolvableBase` only covers an unresolvable ref), so this
  shipped with the test suite green.
- How to fix: never let a value sourced from the untrusted working copy decide which ref is treated
  as "the base" or whether the current checkout counts as the base branch. Two viable directions:
  (a) resolve `isBoundary` and the base commit using **only** the fixed candidate chain
  (`origin/HEAD`, `origin/main`, `main`, `master`), ignoring the working copy's `base_ref` for that
  decision entirely — use `base_ref` (if at all) only as a fallback when none of those resolve, or
  (b) two-phase resolution: find the base via the fixed chain first, load the base-side policy from
  it, and only then consult *that* copy's own `base_ref` (never the working copy's) if it wants to
  point somewhere else. Either way, add the regression test for this exact scenario before closing it
  out — the fixed-chain case has one already (`TestCheckPolicyUnresolvableBase`), the override case
  does not.

### Important (Should Fix)

**2. `skill-invoke-gate.sh`'s "no direct context-file reads" enforcement is trivially bypassed, undermining the guarantee it documents.**

- File: `cmd/hooks/skill-invoke-gate.sh:55-71`.
- What's wrong: after a skill is invoked, the hook is supposed to deny direct reads of files under
  the context dir (the comment at the top: "direct reads of files under the repo context dir stay
  denied — the context arrives injected"). The detection only recognizes the `Read` tool, or a
  `Bash` command that — after word-splitting and excluding any shell metacharacters — is *exactly*
  two tokens, `cat <path>`. Any of `cat file1 file2`, `head`/`tail`/`sed -n`/`less <path>`, or the
  `Grep` tool reading the same file, produce an empty `target` and the hook exits 0 with no denial.
- Why it matters: this isn't a security boundary against an external attacker, but it *is* the
  mechanism the review-quality doctrine (FACT-REPO-ARCH-013, the railroad-review lane design) relies
  on to force a headless reviewer through the skill's curated context payload instead of an ad hoc
  file read. As written it stops exactly the laziest bypass and nothing else, which is worse than no
  enforcement in one respect: a false sense that this is closed.
- How to fix: match on the ctx-dir path appearing *anywhere* in the tokenized `Bash` command (not
  only as the second token of a 2-token `cat`), and extend the tool check beyond `Read`/`cat` to at
  least `Grep`/`head`/`tail`/`sed`/`less` the same way the existing `read-gate.sh` (governed-file
  gate) already had to solve this class of problem — worth checking whether that gate's matching
  approach can be reused here directly.

**3. The base-ref-authoritative redesign in `CheckPolicy` is an unflagged deviation from the approved plan's literal Changes-§1 code, and it's the direct cause of Critical #1.**

- File: `internal/acdsl/policy.go:329-376` vs.
  `plans/v1-2-1/2026-08-26-acdsl-policy-modes-done-a7cd7bf/design/raw.md` Changes §1.
- What's wrong: the plan's approved pseudocode is `if policy.Mode == PolicyModeFree { return nil,
  nil }` as the very first statement, using only the working-copy `policy` argument throughout. The
  shipped code instead resolves the base *first*, unconditionally, then overwrites `policy` with
  whatever `basePolicy` reads from that base, and only then applies the free-mode short-circuit. This
  is a materially different trust model from what was approved (D2/D4 in the plan explicitly reason
  about the working-copy policy being the input to every check), introduced without a Changelog row
  (RULE-PLAN-049) and without stopping to report the contract change (ACTION-IMPL-001 — "Stop and
  report when an approved signature or contract can't hold as planned; never improvise architecture
  mid-edit").
- Why it matters: the deviation itself is directionally correct — it closes a real hole in the
  plan's literal design (flip `mode` to `"free"` locally, `CheckPolicy` returns immediately without
  ever computing `surfaceDelta` on that same edit) — but doing it silently, without the wider design
  scrutiny a stop-and-report would have gotten, is exactly how the subtler bug in #1 (a value sourced
  from the same untrusted working copy still deciding `isBoundary`) slipped through unreviewed.
- How to fix: no separate code fix beyond #1 — flag this class of deviation for confirmation rather
  than resolving it unilaterally next time, per this repo's own doctrine.

### Minor (Nice to Have)

**4. `GitChangedFiles` mis-parses paths containing spaces.**
- File: `internal/acdsl/git.go:46-66`.
- `strings.Fields(diffed + "\n" + untracked)` splits on any whitespace, so a changed file whose path
  contains a space becomes two bogus, non-existent "paths," neither of which is checked by
  `ruleDelta` — a rule declared or edited in such a file is invisible to the gated/strict check.
  Low real-world likelihood in this repo's path conventions, but `strings.Split(output, "\n")`
  (trimming empty lines) is the correct, already-idiomatic fix and costs nothing.

**5. `acdsl/testdata/ACDSL-JSON-003/fail/policy.json` collapses the plan's two fail cases into one file.**
- Plan (`raw.md` Changes §5) calls for two fixtures — `"mode": "loose"` and a file with an unknown
  key — "mirroring `ACDSL-JSON-001/`'s structure." The shipped fixture combines both violations into
  a single file. The schema gate is still exercised end-to-end, so this is cosmetic, not a coverage
  gap that matters in practice.

## Recommendations

- Before this ships to any target repo in `gated`/`strict` mode, fix Critical #1 and add the
  regression test for the self-referential `base_ref` case (and, while there, a fuzz-style case
  sweeping "any string equal to the current branch name" rather than one hardcoded value).
- Given how close this came to shipping with a passing test suite, it's worth a second adversarial
  pass specifically over `internal/acdsl/policy.go` once #1 is fixed — I stopped at the first working
  bypass I found; I did not exhaustively search for a second one against the corrected design.
- The `skill-invoke-gate.sh` bypass (#2) is worth fixing before leaning further on it as the
  enforcement mechanism for "reviewers must go through the skill's context payload," since more
  review/eval infrastructure (`superpowers-review`, `railroad-review`) is being built on top of that
  assumption in this same range.

## Assessment

**Ready to merge?** With fixes.

**Reasoning:** The bulk of this range (registry/cache invalidation, peek session attribution, routine
scheduling, and the large body of skill/doc updates I skimmed) is solid, well-tested, and matches its
plans closely. But the ACDSL self-management policy feature — the one explicitly designed to stop an
agent from loosening its own guardrails — is defeated by a one-field edit an agent can make on its
own branch, verified by reproduction against the actual code, not merely inspection. That one feature
should not ship as `gated`/`strict`-enforcing until Critical #1 is fixed; everything else in the
range can merge independently of it.
