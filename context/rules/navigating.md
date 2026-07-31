# Navigation & Scoping

Ambient doctrine for orienting in and extending a repo — every session, every task. Cite the entry
ID when flagging a violation.

**NEVER-NAV-001** `[review]` — Never scan the entire repository.

* Why: full scans burn context on irrelevant files; the pack and targeted search answer faster.
* Applies: any repo exploration.

**ALWAYS-NAV-001** `[review]` — Always orient via the context pack (rules, facts, style guides) before exploring the code.

* Why: pack facts are verified ground truth; re-deriving them wastes context and drifts.
* Applies: session start and any unfamiliar area.

**ALWAYS-NAV-002** `[review]` — Always read the nearest sibling (service, model, test file) before writing new code — new code mirrors its architecture.

* Why: the sibling is the golden path; deviating creates a second pattern reviewers must reconcile.
* Applies: any new file, type, or endpoint.

**ALWAYS-NAV-003** `[review]` — Always prefer existing implementations over new ones.

* Why: duplicate mechanisms double the maintenance surface and diverge.
* Applies: any new helper, utility, or abstraction.

**ALWAYS-NAV-004** `[review]` — Always use `jq` for ad-hoc JSON exploration (webhook dumps, DLQ files, API responses) — never throwaway Python scripts.

* Why: jq is inspectable in one line and leaves no script litter.
* Applies: any ad-hoc structured-data extraction.

