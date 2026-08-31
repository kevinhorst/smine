# golangci-lint × ACDSL — integration knowledge

2026-08-30 · supersedes the 2026-08-23 edition (which superseded the 2026-08-07 investigation "Go linters as ACDSL verifiers"). This edition folds in an adversarial 4-run re-verification: every load-bearing claim was checked against the actual primary source — the in-worktree ACDSL engine code, the official golangci-lint v2 docs, the issue tracker, and the locally installed binary (golangci-lint **2.12.2**, go1.26.2, 114 linters). 43 of 48 claims held; 4 were refuted and are corrected below (see the refuted register); 1 stays open as a measurement. Behavioral claims are version-bound to 2.12.2 unless stated.

## Two gates, two jobs

- **golangci-lint** is the differential CI gate: `make lint` is the exact CI command, reporting only findings on lines the branch changed against its merge-base with `develop`. It carries two loads: general-purpose Go hygiene the style guide never documents (`errcheck`, `gosec`, `staticcheck`, `intrange`, `modernize`, `misspell`, …), and the `[lint]`-tagged slices of house rules where an ecosystem linter genuinely encodes the rule. A green build means the code you touched conforms — not that the whole file does.
- **ACDSL verifiers** are anchored, whole-file gates with per-rule attribution — the home of house-specific doctrine no ecosystem linter encodes: the `Component.Method:` error prefix, the `_id`/`_shouldPass` test schema, `new(expr)` pointer initialization, the exec-through-`internal/shell` rule. They run via `acdsl check` (locally through `make audit`, at agent read/write time through the hooks) and log per-rule verdicts.
- Neither replaces the other. golangci-lint cannot express the house-specific core (alphabetical struct fields was proposed upstream and declined — golangci-lint issue #1499, closed with labels `declined`/`linter: idea`, requester withdrew); ACDSL should not re-implement generic hygiene a maintained linter already proves.

## What golangci-lint is — capability and config model (v2)

golangci-lint is a metalinter: it runs many linters over one shared load/parse/type-check pass and one shared cache. Facts that shape every integration decision, all verified against the v2 docs or the 2.12.2 binary:

- **Type-checking is the cost and cannot be disabled.** Every run type-checks the module; code that does not compile cannot be analyzed. The first run caches type information; subsequent runs are faster (cold vs warm is the dominant wall-time variable). Linters needing type info are internally marked slow (`IsSlow` via `WithLoadForGoAnalysis`), and `unused` is excluded from the metalinter merge because of its high memory usage.
- **Config file** (`.golangci.yml`, YAML/TOML/JSON) requires `version: "2"`. Top-level sections: `run`, `linters`, `formatters`, `issues`, `output`, `severity`.
- **Linter selection**: `linters.default` ∈ `standard` (default) | `all` | `none` | `fast`, then `enable`/`disable` deltas plus per-linter `settings.<linter>`. v1's presets, `enable-all`, `disable-all`, and the `fast` run flag were **removed** in v2.
- **Formatters split out**: `gci`, `gofmt`, `gofumpt`, `goimports` moved to a dedicated `formatters` section, driven by the separate `golangci-lint fmt` subcommand. Consequence: `run --enable-only=<formatter>` does **not** exercise a formatter — formatter-backed `[lint]` coverage (e.g. goimports/grouper for `RULE-GOLANG-IMPORT-001`) needs `golangci-lint fmt --diff`, not `run`.
- **Exclusions** are the sanctioned home for house exceptions: `linters.exclusions.{generated (default strict), warn-unused (default false), presets (comments / std-error-handling / common-false-positives / legacy), rules (path / path-except / linters / text / source), paths, paths-except}`.
- **Issue-limiting defaults that bite any counting consumer**: `issues.uniq-by-line: true` (drops co-located findings), `max-issues-per-linter: 50`, `max-same-issues: 3` — all silent-undercount traps unless overridden.
- **Differential mode**: `--new-from-merge-base=<branch>` (preferred), `--new-from-rev`, `--new-from-patch`, `--new` (unsafe); all skip any issue not on a changed line unless `--whole-files`. This is the mechanism behind the `make lint` differential gate.
- **`run.timeout` is disabled by default in v2** (v1 defaulted to 1m). `issues-exit-code` defaults to 1.
- **Exit-code taxonomy** (exitcodes.go): 0 Success, 1 IssuesFound, 2 WarningInTest, 3 Failure, 4 Timeout, 5 NoGoFiles, 6 NoConfigFileDetected, 7 ErrorWasLogged.
- **Custom linters**: module plugins (`.custom-gcl.yml` + `golangci-lint custom` producing a forked binary, enabled via `linters.settings.custom.<name>.type: module`) are the sanctioned path; the older `.so` Go-plugin system coexists (module is the recommended/portable one — the docs do not mark `.so` deprecated). Module plugins are the wrong home for house doctrine: every rule change forces a rebuilt pinned binary and inverts the ACDSL/golangci division of labor. House rules stay in ACDSL's cheap syntax-level verifiers.

### What it cannot do

- Express house-specific doctrine: alphabetical struct fields (#1499 declined), the `Component.Method:` error-message grammar, the `_id`/`_shouldPass` test schema, `new(expr)` initialization. No configuration reaches these.
- Attribute findings to *house rules* — its identity unit is the linter (`FromLinter` in JSON), not a style-guide rule id.
- Run without a compilable module, or per-file across package boundaries: a file list spanning directories is rejected ("named files must all be in one directory").
- Guarantee complete coverage out of the box even where a linter nominally applies: `errcheck` by default ignores blank-identifier assignments (`f, _ := ...`; `check-blank` is off) — "golangci covers unchecked errors" is only partially true until configured.

## Limitations

### Performance

- **Cold vs warm is the binding variable.** The whole-module type-check dominates the first run; the cache makes later runs cheap. Any consumer with a hard wall-time budget must either pre-warm the cache outside that budget or accept cold-run risk.
- **Memory/OOM is a first-class, recurring, documented failure mode**, not an edge case: issues #337, #731, #3565, #3582 (80 GB), #5449 (~34 GB, OOM-killed at CircleCI's 16 GB). `staticcheck`/`gosimple` are the hogs. Mitigations — lowering `GOGC`, limiting `--concurrency`, raising the CI memory limit — live in issue threads and the repo README's memory-usage section, not the FAQ. Anything that multiplies concurrent golangci-lint processes multiplies peak memory.
- The docs' dedicated performance page is gone (404); performance guidance is scattered across the FAQ, CLI help, and issue tracker.

### Overfitting

- **`linters.default: all` imports anti-house rewrites.** Several linters enforce the exact opposite of documented house style (list below); a maximal config is negative coverage, not thoroughness. Correct posture: start from `none`, opt in per check, encode sanctioned exceptions in config exclusions (as `varnamelen` already does).
- **Coverage claims from docs alone have repeatedly failed** (modernize/`&T{...}`, ST1012/errname — see appendix). A `Covered by` claim is only real after running the linter against sanctioned house code.
- **Partial coverage masquerades as full coverage**: linter defaults (errcheck's `check-blank`, issue-limiting caps) silently narrow what "enabled" means. Every adoption must state precisely which slice is covered.

### Overmanagement

- **Suppression sprawl**: `//nolint` always names the linter and carries an on-line reason (`nolintlint` enforces both); a suppression needed more than about three times for one reason is a missing config exclusion, not a suppression.
- **Config as authority.** The disable list, per-linter scoping, and sanctioned exceptions live in `.golangci.yml`, and where a linter and a documented rule disagree, the config records which one wins and why. Prose never restates config contents — `RULE-GOLANG-ERR-006` documents the lesson: an enumeration goes stale against the config and then misdirects review twice over.
- **Version pinning is load-bearing.** Message texts are the rule-mapping key and drift across versions; autofix capability is version-volatile (funcorder shipped fixes in v0.4.0, rolled them back in v0.5.0). The gate requires golangci-lint at the version the repo pins (`make check-lint-version`).
- **The v1→v2 migration is a breaking cliff**: `golangci-lint migrate` exists (backs up `.bck`) but does not migrate comments; removed linters (golint, scopelint, maligned, structcheck/varcheck/deadcode, exhaustivestruct, interfacer, execinquery, tenv, …); renames (gomnd→mnd, gas→gosec, vet→govet, goerr113→err113, logrlint→loggercheck); stylecheck+gosimple merged into staticcheck; typecheck non-configurable; the default-selection model flipped. Budget a real migration, not a config rename.

## The seam contract

Seamlessness is not one integration point — it is four conventions that keep the two systems from drifting apart or double-reporting:

1. **The `[lint]`/`[review]` tag doctrine** (`context/rules/go.md`, header). Every style-guide rule carries exactly one tag: `[lint]` — a CI linter proves it, the rule's first bullet names the linter (`Covered by …`), and reviewers skip it; `[review]` — no tool covers it, review effort belongs there. A partly-automatable rule is **split**: the CI-proven part becomes its own `[lint]` rule, the human remainder a sibling `[review]` rule cross-referencing it. Read the tag, never the id, to know who checks a rule.
2. **The `lint-tags` gate** — ACDSL rule `ACDSL-GOLANG-DOCS-001` (`reach="go"`, `config=".golangci.yml"`, registered verifier `go run ./cmd/acdsl/verifiers/linttags`, timeout 30 s). It verifies every `Covered by` claim in the guide against the repo's golangci config: a claim naming a disabled or absent linter means the rule is checked by nobody, and the gate goes red. This replaced the go repo's `check-tags.sh`. **Known blind spots (verified in `linttags/main.go`)**: it parses only explicit `linters.enable:` lists and settings-level `disable:` blocks; recognizes staticcheck sub-checks only via `^(ST|QF)[0-9]+$`; does not model `linters.default`, top-level `linters.disable`, or gocritic `enabled-checks`/`disabled-checks`; and a guide with no config above it passes silently. Load-bearing constraint: the `.golangci.yml` must use an explicit enable list, or the gate mis-reads coverage. It never runs golangci-lint itself — it reads the config as text.
3. **Config as authority** (see Overmanagement above — the same convention, stated once).
4. **Suppression and versioning discipline** (see Overmanagement above).

## Handing a house rule to golangci-lint

The migration recipe, one rule at a time:

1. Find the covering linter and verify by running it — coverage claims from docs alone have failed before (see appendix).
2. Check it against the fights-house-style list below; a linter that enforces the opposite of the rule is negative coverage, not coverage.
3. Check which slice of the rule it actually covers at its *default* settings (errcheck/`check-blank` class of trap) and whether the check lives under `linters` or `formatters` — formatter-backed coverage needs `golangci-lint fmt --diff`, not `run`.
4. Enable and scope it in `.golangci.yml` — start from `default: none` and opt in per check; house exceptions go in config exclusions, never blanket `//nolint`.
5. Flip the guide rule to `[lint]` with a `Covered by` bullet naming the linter; state precisely which slice is covered.
6. Write the uncovered remainder as a sibling `[review]` rule — most linters check a position or shape, not the naming or judgment half (e.g. `revive/context-as-argument` proves ctx position, not the name `ctx`).
7. `lint-tags` green proves the claim; retire any ACDSL verifier the linter now fully subsumes (`noctx`'s `exec.Command` check, for instance, would merely duplicate the existing `forbid-call` gate — keep the ACDSL side there).

## Linters that fight the house style

Verified against sanctioned house code; enabling any of these (or `linters.default: all`, which contains them) generates diagnostics — in some cases automatic rewrites — *against* the documented style:

- **staticcheck ST1003** and **revive/var-naming's initialism half** — demand `Id → ID`, `HttpClient → HTTPClient`, the exact inverse of `RULE-GOLANG-NAME-002`'s deliberate override.
- **staticcheck ST1005** — demands lowercase error strings, against the required capitalized `Component.Method: Message` grammar.
- **go vet fieldalignment** — reorders struct fields by memory layout, against alphabetical doctrine.
- **gochecknoinits** — bans all `init` functions; the house allows one per package.
- **gocritic/elseif** — collapses toward `else if`, against the switch preference (its sibling `ifElseChain` serves the house rule and is adopted — the two must not be confused).

Two entries from the original list have been resolved by scoping rather than exclusion, superseding the 2026-08-07 verdicts:

- **funcorder** is adopted for the constructor-position slice with its exported-before-unexported ordering check deliberately off (it wants the opposite order to `RULE-GOLANG-FILE-001`); it recognizes only `New`-prefixed constructors; the private-before-public layout stays review-owned.
- **wrapcheck** is adopted scoped to non-first-party packages (`RULE-GOLANG-ERR-005`) — it proves the boundary-wrap half while the `Component.Method:` message grammar stays with the custom `errorf-prefix` verifier. The earlier "adopt last or not at all" verdict assumed it had to stand in for the whole rule.

## Registering golangci-lint as an ACDSL verifier

Current state, verified in the worktree: **no golangci-lint-as-verifier wrapper exists** — `registry.json` has 23 entries, all ACDSL's own `go run ./cmd/acdsl/verifiers/<name>` binaries. Everything below is design, not implementation.

The engine facts that constrain any design (all confirmed in source):

- Every registry entry runs under a hard **1–60 s timeout**, validated at load (`registry.go:59-61`), applied per run via `context.WithTimeout` (`run.go:122`). The ceiling is a hard wall, not a tuning knob; `exhaustive-switch` already pins the 60 s max.
- One verifier process per rule; every parsed diagnostic is stamped with the single invoking `rule.Id` (`run.go:137,141`). Batch demultiplexing to different rule ids would be an engine change.
- Exit contract: 0 = pass, 1 = violations (only stdout lines matching `^\S+:\d+: ` kept), any other nonzero = tool breakage that fails the check loudly.
- The verifier receives one flat newline file list **spanning directories** (`writeFilesList`), drawn from the whole-repo file universe filtered by anchor — cross-directory input is the normal shape, not an edge case.
- `resolveVerifierBinary` removes only the `go run` link/toolchain tax for ACDSL's own binaries; it does nothing for a golangci-lint child's type-check cost.

### For a repo that already runs differential `make lint` in CI

Do not double-gate. The linters' findings are the CI gate's business; the ACDSL side gates only the *claims* — `lint-tags` verifying the guide against the config. Re-running linters in the registry duplicates CI and pays the timeout/type-check/memory cost for zero new signal. This is the implemented state.

### For a repo with no CI lint gate

Two candidate shapes; the second is the correction this edition adds:

- **Per-linter rules** (the 2026-08-23 design): one shared wrapper verifier, one ACDSL rule per (linter, sub-check) via `run --enable-only=<linter>`, each re-emitting the `file:line: message` contract. Attribution is exact, but per-rule attribution forces one golangci-lint process per mapped rule — multiplying type-check wall time against the 60 s wall *and* peak memory (see the OOM record). The earlier claim that this shape is *mandatory* conflated rule-level verdicts with diagnostic attribution.
- **Single coarse rule** (cheaper, and correct): one `golangci-lint run` with JSON output behind **one** ACDSL rule, re-emitting `file:line: [<FromLinter>] message`. golangci JSON carries per-issue linter identity in `FromLinter` (verified live on 2.12.2), so per-linter attribution survives inside the message with no engine change. N processes collapse to 1 and the timeout risk mostly dissolves. The rule-level verdict is coarse (one rule goes red for any linter's finding) — but that matches how `make lint` itself reports. Prefer this shape unless per-rule red/green is a hard requirement.

**Wrapper duties** (mandatory either way, each verified on 2.12.2):

- **Group anchor files by directory** (or run per-package/module): a cross-directory file list makes golangci-lint run no linter at all — stderr `named files must all be in one directory`, stdout `0 issues.`, **exit 7**. Correction to the previous edition: this is *not* a silent false pass — exit 7 is not 1, so ACDSL fails closed, loudly. The grouping requirement stands because a cross-directory invocation lints nothing; the "treat the stderr line as fatal" instruction is redundant on 2.12.2.
- **JSON to its own file** (`--output.json.path=<file>`), because the default text formatter co-writes its `N issues:` summary to stdout even when JSON is requested. Correction: warning/error log lines go to *stderr* on 2.12.2, never stdout — the contamination is the formatter's, not the logger's.
- **Force `issues.uniq-by-line: false`** (and lift `max-issues-per-linter`/`max-same-issues`), or per-rule verdict counting undercounts.
- **Synthesize `file:line:` from JSON `Pos`** rather than piping text output. Nuance (corrects a wrong premise, keeps the conclusion): ACDSL's `^\S+:\d+: ` regex *does* loosely match golangci's `file:line:col:` text — the greedy `\S+` absorbs `file:line` and the column is read as the line. A naive text pipe is therefore accepted-but-mis-attributed, not rejected; JSON is required for correctness, not because the regex refuses text.
- **Drop or distinctly fail on `(typecheck)` findings** — the real hazard this re-verification surfaced. A genuine compile error inside a valid run exits 1 and prints `file:line:col: undefined: X (typecheck)` plus a `file:1: : # pkg` banner; both match the violation regex, so a broken build would be mis-parsed as a style violation stamped with the invoking rule id. (Correction: the old #529 behavior — typecheck issues with exit 0 — does not exist on 2.12.2; compile failures exit 1 and *surface*, just mis-attributed without this filter.)
- **Normalize the exit-code taxonomy**: map golangci 1 → violations, 0 → pass, and 3/4/5/7 → distinct tool-breakage (ACDSL already fails closed on any non-{0,1}, but the wrapper's own error text should say which mode it hit).
- **Cache-warming pre-step outside the registry timeout** is a hard precondition: type-check cost cannot be disabled, cold runs on a real module can breach 60 s, and the FAQ-documented warm cache is the only lever. Mandatory under the per-linter shape; still required (once) under the single-rule shape.
- **Formatter-backed rules go through `golangci-lint fmt --diff`**, not `run` — the v2 split means `run --enable-only=goimports` exercises nothing.

## Autofix notes

- A `--fix` pass is never complete by itself: reproduced on 2.12.2, the perfsprint rewrite (`fmt.Errorf` → `errors.New`) neither adds the `errors` import nor drops the now-unused `fmt` import — `go build` fails both ways. Every fix pass chains goimports and a build check afterward.
- Correction: the previously reported rerun symptom "diff has out-of-bounds edits" did **not** reproduce on 2.12.2 (a rerun on the broken file reports a plain typecheck error; multi-hit fixes after `cache clean` apply cleanly). Document the hazard as "autofix can leave an uncompilable file," not as a promised error string.
- Autofix capability is version-volatile (funcorder shipped fixes in v0.4.0, rolled them back in v0.5.0) — re-verify per version bump.

## Refuted register — corrected claims (do not re-assert)

Each refuted against the pinned local binary (2.12.2); the corrections are folded into the sections above.

- **R1** — "cross-directory file list → stderr error + `0 issues` with exit-0 success (silent false pass)." Refuted: exit is **7**; ACDSL fails closed. Directory grouping still required (nothing gets linted).
- **R2** — "warning lines interleave with JSON on stdout." Refuted: warnings go to stderr. The real stdout contamination is the default text formatter co-writing its summary; JSON-to-file stands, for that reason.
- **R3** — "exit code on load/typecheck errors is inconsistent between config-present and no-config (exit 0 vs 7)." Refuted: both exit 7; a dir without go.mod exits 5; no exit-0 path found for load errors.
- **R4** — "typecheck failures print as issues but exit 0 (issue #529 live)." Refuted: uncompilable packages exit **1**. The surviving hazard is mis-attribution of `(typecheck)` findings as style violations, handled by the wrapper filter above.
- **R5** — "golangci text output cannot match ACDSL's violation regex, so text piping is rejected." Partially refuted: the regex loosely matches (column read as line) — text piping is accepted-but-wrong, which is worse. JSON stands, for correctness.

## Open measurements

Still unresolved; results return as binding facts:

1. Cold-cache `golangci-lint run` wall time on the real go monorepo with its pinned version — the go/no-go number for any registry shape (and whether ACDSL's flat cross-package file list, grouped by the wrapper, stays under budget).
2. Whether *any* 2.12.2 invocation can log `level=error` yet exit 0 with `0 issues` — the exit-code enumeration closed the cross-dir and uncompilable paths but was not exhaustive (linter panic, invalid config, unknown linter).
3. The go monorepo specifics this doc repeats from prose — merge-base branch literally `develop`, a target literally `make check-lint-version` — are unverifiable from this worktree; confirm against the monorepo's Makefile/CI before relying on the exact names.
4. If CI pins a golangci-lint version other than 2.12.2, re-confirm the exit-code behavior on that exact toolchain — the refuted claims above were version-bound findings once, in the other direction.

## Known documentation gap

`RULE-GOLANG-POINTER-003` (the modernize-negative / `forbid-addr-lit` target) is absent from the current `context/rules/go.md` (only POINTER-001/002 exist). The custom addr-literal verifier maps to a rule id not in the rules file — reconcile before shipping any coverage claim that names it.

## Appendix — verified findings that stand (do not re-investigate)

From the 2026-08-07 investigation, re-confirmed 2026-08-30 where marked; each was verified by running the tools:

- Standalone go-critic or wrapcheck adoption — dominated by the golangci-lint bundle (it ships both; separate binaries add a second type-check and no shared cache).
- modernize covering `&T{...}` (`RULE-GOLANG-POINTER-003`) — verified negative again on 2.12.2; the custom `forbid-addr-lit` verifier stays.
- ~~wrapcheck as the RULE-GOLANG-ERR-001 gate~~ — **superseded**: adopted as `RULE-GOLANG-ERR-005`'s scoped boundary-wrap gate; the message-grammar half stays custom.
- ~~funcorder enforcing RULE-GOLANG-FILE-001~~ — **superseded**: adopted for the constructor-position slice with the ordering check disabled.
- Any ecosystem linter for alphabetical struct fields (`RULE-GOLANG-STRUCT-002`) — proposed upstream and declined/withdrawn (golangci-lint #1499, closed; verified via `gh`).
- ST1012/errname as `RULE-GOLANG-ERR-003` coverage — they check *sentinel* naming (`ErrFoo`), not err-variable naming.
- Batch-run demultiplexing without engine changes — single-rule attribution forbids it (but see the single-coarse-rule shape above, which sidesteps rather than solves it).
- ~~Bare (wrapper-less) golangci-lint registration → silent false passes on cross-directory file lists~~ — **corrected** (R1): fails closed with exit 7, not silently; the wrapper stays mandatory for the other duties listed.
- `--fix` as a complete ladder step — produces uncompilable output without a goimports chase (re-reproduced on 2.12.2).
- Ecosystem naming/style linters as neutral free coverage — several are anti-house (negative coverage; see the fights-house-style list).
- go-critic as a coverage play — contributes roughly one partial house rule; its value is net-new generic diagnostics (`deferInLoop`, `filepathJoin`) and the embedded ruleguard DSL, an optional accelerator, not an unlock.
