# COMMIT MESSAGE STYLE

**For reviewers / agents:** cite the stable `RULE-COMMIT-*` id when flagging a violation. Every
rule here is a manual check (`[review]`) — there is no formatter for commit messages.

**Scope:** the commit *subject* (the first line). Keep it a single, concise line. This guide governs
how the subject is *worded*; the `package-commit` workflow in `skills/git/package-commit/SKILL.md` governs
how changes are *grouped, validated* (`go build` / `go vet` / `go test`) *and staged*. Where the
skill's examples and this guide disagree on wording, this guide wins.

---

## STRUCTURE

**RULE-COMMIT-001** — Prefix form `[review]`

* A commit subject starts with a scope prefix and a colon:

  ```
  <scope>: <Description>
  ```
* When the whole commit concerns a single symbol, name it as a second prefix:

  ```
  <scope>: <Symbol>: <Description>
  ```
* New-package imports are the sole exception — they carry no prefix (see RULE-COMMIT-010).

**RULE-COMMIT-002** — Scope path `[review]`

* The scope is the owning package in **dot notation**, including the module segment — never a
  slash path.
  * `api/models/accounts` → `api.models.accounts`
  * `api/db/postgres/apps` → `api.db.postgres.apps`
* Non-Go directories use the same dot notation: `docs.general`, `docs.plans.login_simplification`,
  `.claude`, `scripts`, `vendor`.
* An infra change spanning sibling roots may list them, comma-separated (no conjunction — RULE-COMMIT-008):

  ```
  vendor, go.mod, go.sum: Added dependencies for login simplification
  ```

**RULE-COMMIT-003** — Symbol (second) prefix `[review]`

* Use the `<Symbol>:` second prefix **only** when the entire commit concerns one existing symbol —
  adding funcs to it, or modifying it.
* **Introducing** a new symbol does NOT take a second prefix (`Config: Added struct Config` is
  redundant) — use the type-kinded body form instead (RULE-COMMIT-006).
* A commit touching several symbols, or the package as a whole, keeps only the package scope.

```
# GOOD — the whole commit is about one existing symbol
api.db.userprofile: UserProfilesController: Added funcs GetUserProfileByUserId() and GetUserProfileByUserIdForUpdate()
```

---

## WORDING

**RULE-COMMIT-004** — Capitalization `[review]`

* The first word after the final prefix is Capitalized.
* Symbol names keep their real identifier casing per `go.md` **RULE-NAME-002**: `HttpClient`,
  `Otp`, `Id` — never `HTTPClient`, `OTP`, `ID`.

**RULE-COMMIT-005** — Tense `[review]`

* State what the commit *did*, in the **past tense**: `Added`, `Moved`, `Registered`,
  `Implemented`, `Consolidated`, `Reduced`, `Switched`, `Wired`. Never imperative (`Add`) and never
  lower-case (`added`).

**RULE-COMMIT-006** — Type-kinded symbol introductions `[review]`

* When a commit introduces a named symbol, state its Go kind before the name:
  * `Added struct <Name>`
  * `Added enum <Name>` — a named scalar type (`string`, `int`, …) together with its const set
  * `Added interface <Name>`
  * `Added sentinel error <Name>`

```
# GOOD
api.models.accounts: Added enum LoginIntentState
```

**RULE-COMMIT-007** — Funcs, not methods `[review]`

* Go has no "methods" — call them **funcs**. A func named in a message carries `()`; a func on a
  type may use receiver notation (`App.Copy()`). Type kinds (`struct` / `enum` / `interface`) and
  vars (`sentinel error`) stay bare — they are not callable.

```
# GOOD
api.models.apps: Added funcs App.Copy() and ProvideCompleteApp()
```

**RULE-COMMIT-008** — No Oxford comma `[review]`

* No comma before a conjunction (`and` / `or`).

```
# GOOD
api.middleware: Added CORS origin allowlist, wildcard handling and extra headers
```

**RULE-COMMIT-009** — No trailing period `[review]`

* The subject line does not end with `.`.

---

## SPECIAL CASES

**RULE-COMMIT-010** — New packages `[review]`

* A brand-new package is introduced in a **single commit holding its final state**, with the
  subject:

  ```
  Import initial version of package <full.dot.path>
  ```

* No scope prefix — the full package path is already named in the sentence.

```
# GOOD
Import initial version of package api.email
```

**RULE-COMMIT-011** — Moves / extractions `[review]`

* When code is relocated, the scope is the **source** package it moved *from*; the body names the
  destination package. Stage the removals and additions together so the move is recorded as a
  rename.

```
# GOOD
api.worker: Moved live-TV recording tasks into api.worker.livetv
```

**RULE-COMMIT-012** — No rule identifiers `[review]`

* Never cite a `RULE-*` id in a message — describe the change in plain terms.

**RULE-COMMIT-013** — No assistant attribution `[review]`

* Never add `Co-authored-by` trailers or any AI-assistant mention.

---

## GROUPING

Grouping, validation and staging are defined by the `package-commit` skill. In short:

* One package per commit — never combine unrelated packages.
* A new package → one commit, final state (RULE-COMMIT-010).
* An existing package → one commit per new file / type.
* Skip generated files (`*.gen.go`).

---

## SUB-GUIDES

Part of the style set — see `go.md` (general Go incl. TESTS and GOROUTINES sections), `sql.md`, `plan.md` next to this file.
