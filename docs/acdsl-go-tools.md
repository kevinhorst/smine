# golangci-lint × ACDSL — integration knowledge

2026-08-23 · supersedes the 2026-08-07 investigation "Go linters as ACDSL verifiers" (its still-valid negative results are preserved in the appendix). Since that investigation, golangci-lint has been adopted in the go monorepo as a differential CI gate, and the Go style guide (`context/rules/go.md`) was rewritten around it. This document records how the two enforcement systems combine, and how to keep the combination seamless.

## Two gates, two jobs

- **golangci-lint** is the differential CI gate: `make lint` is the exact CI command, reporting only findings on lines the branch changed against its merge-base with `develop`. It carries two loads: general-purpose Go hygiene the style guide never documents (`errcheck`, `gosec`, `staticcheck`, `intrange`, `modernize`, `misspell`, …), and the `[lint]`-tagged slices of house rules where an ecosystem linter genuinely encodes the rule. A green build means the code you touched conforms — not that the whole file does.
- **ACDSL verifiers** are anchored, whole-file gates with per-rule attribution — the home of house-specific doctrine no ecosystem linter encodes: the `Component.Method:` error prefix, the `_id`/`_shouldPass` test schema, `new(expr)` pointer initialization, the exec-through-`internal/shell` rule. They run via `acdsl check` (locally through `make audit`, at agent read/write time through the hooks) and log per-rule verdicts.
- Neither replaces the other. golangci-lint cannot express the house-specific core (alphabetical struct fields was proposed upstream and declined — golangci-lint issue #1499); ACDSL should not re-implement generic hygiene a maintained linter already proves.

## The seam contract

Seamlessness is not one integration point — it is four conventions that keep the two systems from drifting apart or double-reporting:

1. **The `[lint]`/`[review]` tag doctrine** (`context/rules/go.md`, header). Every style-guide rule carries exactly one tag: `[lint]` — a CI linter proves it, the rule's first bullet names the linter (`Covered by …`), and reviewers skip it; `[review]` — no tool covers it, review effort belongs there. A partly-automatable rule is **split**: the CI-proven part becomes its own `[lint]` rule, the human remainder a sibling `[review]` rule cross-referencing it. Read the tag, never the id, to know who checks a rule.
2. **The `lint-tags` gate** — ACDSL rule `ACDSL-GOLANG-DOCS-001` (`reach="go"`, `config=".golangci.yml"`). It mechanically verifies every `Covered by` claim in the guide against the repo's golangci config: a claim naming a disabled or absent linter means the rule is checked by nobody, and the gate goes red. This replaced the go repo's `check-tags.sh`. It is the drift-killer for the seam itself — the guide cannot silently promise coverage the config no longer delivers.
3. **Config as authority.** The disable list, per-linter scoping, and sanctioned exceptions live in `.golangci.yml`, and where a linter and a documented rule disagree, the config records which one wins and why. Prose never restates config contents — `RULE-GOLANG-ERR-006` documents the lesson: an enumeration goes stale against the config and then misdirects review twice over.
4. **Suppression and versioning discipline.** `//nolint` always names the linter and carries an on-line reason (`nolintlint` enforces both); a suppression needed more than about three times for one reason is a missing config exclusion, not a suppression. The gate requires golangci-lint at the version the repo pins (`make check-lint-version`) — message texts and autofix capability drift across versions, and message texts are the rule-mapping key.

## Handing a house rule to golangci-lint

The migration recipe, one rule at a time:

1. Find the covering linter and verify by running it — coverage claims from docs alone have failed before (see appendix).
2. Check it against the fights-house-style list below; a linter that enforces the opposite of the rule is negative coverage, not coverage.
3. Enable and scope it in `.golangci.yml` — start from no preset and opt in per check; house exceptions go in config exclusions, never blanket `//nolint`.
4. Flip the guide rule to `[lint]` with a `Covered by` bullet naming the linter; state precisely which slice is covered.
5. Write the uncovered remainder as a sibling `[review]` rule — most linters check a position or shape, not the naming or judgment half (e.g. `revive/context-as-argument` proves ctx position, not the name `ctx`).
6. `lint-tags` green proves the claim; retire any ACDSL verifier the linter now fully subsumes (`noctx`'s `exec.Command` check, for instance, would merely duplicate the existing `forbid-call` gate — keep the ACDSL side there).

## Linters that fight the house style

Verified against sanctioned house code; enabling any of these (or a preset containing them) generates diagnostics — in some cases automatic rewrites — *against* the documented style:

- **staticcheck ST1003** and **revive/var-naming's initialism half** — demand `Id → ID`, `HttpClient → HTTPClient`, the exact inverse of `RULE-GOLANG-NAME-002`'s deliberate override.
- **staticcheck ST1005** — demands lowercase error strings, against the required capitalized `Component.Method: Message` grammar.
- **go vet fieldalignment** — reorders struct fields by memory layout, against alphabetical doctrine.
- **gochecknoinits** — bans all `init` functions; the house allows one per package.
- **gocritic/elseif** — collapses toward `else if`, against the switch preference (its sibling `ifElseChain` serves the house rule and is adopted — the two must not be confused).

Two entries from the original list have been resolved by scoping rather than exclusion, superseding the 2026-08-07 verdicts:

- **funcorder** is adopted for the constructor-position slice with its exported-before-unexported ordering check deliberately off (it wants the opposite order to `RULE-GOLANG-FILE-001`); the private-before-public layout stays review-owned.
- **wrapcheck** is adopted scoped to non-first-party packages (`RULE-GOLANG-ERR-005`) — it proves the boundary-wrap half while the `Component.Method:` message grammar stays with the custom `errorf-prefix` verifier. The earlier "adopt last or not at all" verdict assumed it had to stand in for the whole rule.

## Registering golangci-lint as an ACDSL verifier

For a repo with **no** CI lint gate, golangci-lint can run behind the ACDSL registry — but only in one shape, and never bare:

- **One shared wrapper verifier, one ACDSL rule per (linter, sub-check)**, selected via rule params: each rule's invocation runs `golangci-lint run --enable-only=<linter>` with JSON output and re-emits the engine's `file:line: message` contract. The engine attributes every diagnostic of a run to the single invoking rule (`internal/acdsl/run.go`), so attribution stays correct without engine changes; batch-run demultiplexing would be an engine change.
- The wrapper is **mandatory** — three verified failure modes of a bare registration: golangci-lint given a file list spanning directories logs an error to stderr and reports "0 issues" with success semantics (silent false pass — the wrapper must treat that stderr line as fatal and group anchor files by directory); `issues.uniq-by-line` defaults to true and drops co-located findings (must be false or per-rule verdict counting undercounts); warning lines interleave with JSON on stdout (JSON must go to a file).
- **The binding operational risk is the timeout ceiling:** every registry entry runs under a hard 1–60 s timeout, and per-rule attribution forces one golangci-lint process per mapped rule. Cold-cache wall time on a real module must be measured before adopting this shape; a cache-warming pre-step outside the registry timeout is the known mitigation.
- For a repo that **already runs** differential `make lint` in CI, do not double-gate: the linters' findings are the CI gate's business, and the ACDSL side gates only the *claims* — `lint-tags` verifying the guide against the config.

## Autofix notes

- A `--fix` pass is never complete by itself: verified, the perfsprint rewrite left a file without its `errors` import — uncompilable. Every fix pass chains goimports and a build check afterward.
- Fix reruns are cache-sensitive (a moved tree failed with "diff has out-of-bounds edits" until `golangci-lint cache clean`).
- Autofix capability is version-volatile (funcorder shipped fixes in v0.4.0, rolled them back in v0.5.0) — re-verify per version bump.

## Appendix — verified findings that stand (do not re-investigate)

From the 2026-08-07 investigation; each was verified by running the tools:

- Standalone go-critic or wrapcheck adoption — dominated by the golangci-lint bundle (it ships both; separate binaries add a second type-check and no shared cache).
- modernize covering `&T{...}` (`RULE-GOLANG-POINTER-003`) — verified negative twice; the custom `forbid-addr-lit` verifier stays.
- ~~wrapcheck as the RULE-GOLANG-ERR-001 gate~~ — **superseded**: adopted as `RULE-GOLANG-ERR-005`'s scoped boundary-wrap gate; the message-grammar half stays custom.
- ~~funcorder enforcing RULE-GOLANG-FILE-001~~ — **superseded**: adopted for the constructor-position slice with the ordering check disabled.
- Any ecosystem linter for alphabetical struct fields (`RULE-GOLANG-STRUCT-002`) — declined upstream (golangci-lint #1499).
- ST1012/errname as `RULE-GOLANG-ERR-003` coverage — they check *sentinel* naming (`ErrFoo`), not err-variable naming.
- Batch-run demultiplexing without engine changes — single-rule attribution forbids it.
- Bare (wrapper-less) golangci-lint registration — silent false passes on cross-directory file lists.
- `--fix` as a complete ladder step — produces uncompilable output without a goimports chase.
- Ecosystem naming/style linters as neutral free coverage — several are anti-house (negative coverage; see the fights-house-style list).
- go-critic as a coverage play — contributes roughly one partial house rule; its value is net-new generic diagnostics (`deferInLoop`, `filepathJoin`) and the embedded ruleguard DSL, an optional accelerator, not an unlock.
