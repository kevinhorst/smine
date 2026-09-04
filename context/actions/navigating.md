<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# Navigation & Scoping

Ambient doctrine for orienting in and extending a repo — every session, every task. Cite the entry
ID when flagging a violation.

**ACTION-NAV-001** `[review]` — Never scan the entire repository.

* Why: full scans burn context on irrelevant files; the context docs and targeted search answer faster.
* Applies: any repo exploration.

**ACTION-NAV-002** `[review]` — Always orient via the context docs (rules, facts, style guides) before exploring the code.

* Why: context facts are verified ground truth; re-deriving them wastes context and drifts.
* Applies: session start and any unfamiliar area.

**ACTION-NAV-003** `[review]` — Always read the nearest sibling (service, model, test file) before writing new code — new code mirrors its architecture.

* Why: the sibling is the golden path; deviating creates a second pattern reviewers must reconcile.
* Applies: any new file, type, or endpoint.

**ACTION-NAV-004** `[review]` — Always prefer existing implementations over new ones.

* Why: duplicate mechanisms double the maintenance surface and diverge.
* Applies: any new helper, utility, or abstraction.

**ACTION-NAV-005** `[review]` — Always use `jq` for ad-hoc JSON exploration (webhook dumps, DLQ files, API responses) — never throwaway Python scripts.

* Why: jq is inspectable in one line and leaves no script litter.
* Applies: any ad-hoc structured-data extraction.

**ACTION-NAV-006** `[review]` — Treat generated, gitignored artifacts as part of the codebase: consult the repo's `FACT-REPO-GEN-*` facts for what is generated and how to produce it before concluding a file or package is missing.

* Why: generated models are invisible to git-based grounding; an agent that misses them re-implements them or misreads the build.
* Applies: session start and any missing-file or missing-package conclusion.

**ACTION-NAV-007** `[review]` — Prune `vendor/`, `bin/`, `node_modules/`, `*.gen.go`, and other generated trees from sweeps; never grep the repo's own high-frequency domain keyword.

* Why: unpruned sweeps and a domain-keyword grep match everything and overflow the output; large-branch reviews drown in vendored/generated diffs. Operationalizes ACTION-NAV-001.
* How: pre-scope the vendored and generated trees before reading; grep for the specific symbol, never the repo's name or its central noun.
* Applies: any repo-wide sweep or large-branch review.

**ACTION-NAV-008** `[review]` — A rename or group-move sweep drives to zero every human-readable path reference in docs and READMEs, not just code and slash-command names.

* Why: a doc path pointing at a pre-refactor location (`.claude/commit/SKILL.md` after a rename; `skills/util/peek` after a move) is drift; a slug-only sweep leaves it standing.
* How: enumerate path-shaped references as part of the grep-to-zero close, alongside code references and command names.
* Applies: any skill/asset rename or move between directories.

**ACTION-NAV-009** `[review]` — An active (non-archived) plan left referencing a renamed or deleted asset is drift — record the staleness in that plan's Changelog.

* Why: an unflagged stale reference makes a stale-by-decision plan indistinguishable from an unfinished sweep.
* How: when a rename or deletion sweep consciously leaves an active plan pointing at the old name, add a Changelog note rather than leaving it silently wrong.
* Applies: any rename or deletion sweep that leaves an active plan untouched by decision.

