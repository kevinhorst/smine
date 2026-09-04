<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# COMMIT MESSAGE STYLE

**For reviewers / agents:** cite the stable `RULE-COMMIT-*` id when flagging a violation. Every
rule here is a manual check (`[review]`) — there is no formatter for commit messages.

**Scope:** the commit *subject* (the first line). Keep it a single, concise line. This guide governs
how the subject is *worded*; the `package-commit` workflow in `skills/git/package-commit/SKILL.md` governs
how changes are *grouped, validated* (project build and tests) *and staged*. Where the
skill's examples and this guide disagree on wording, this guide wins.

---

## STRUCTURE

**RULE-COMMIT-001** `[review]` — A commit subject starts with a scope prefix and a colon: `<scope>: <Description>`.

* A commit subject starts with a scope prefix and a colon:

  ```
  <scope>: <Description>
  ```
* When the whole commit concerns a single symbol, name it as a second prefix:

  ```
  <scope>: <Symbol>: <Description>
  ```
* New-package imports are the sole exception — they carry no prefix (see RULE-COMMIT-010).

**RULE-COMMIT-002** `[review]` — The scope is the owning package in dot notation including the module segment — never a file path.

* The scope is the owning package in **dot notation**, including the module segment — never a
  slash path.
  * `cfapi/models/accounts` → `cfapi.models.accounts`
  * `cfapi/db/postgres/apps` → `cfapi.db.postgres.apps`
* Directories without package semantics use the same dot notation: `docs.general`, `docs.plans.login_simplification`,
  `.claude`, `scripts`, `vendor`.
* An infra change spanning sibling roots may list them, comma-separated (no conjunction — RULE-COMMIT-008):

  ```
  vendor, go.mod, go.sum: Added dependencies for login simplification
  ```

**RULE-COMMIT-003** `[review]` — The `<Symbol>:` second prefix is used only when the entire commit concerns one existing symbol.

* Use the `<Symbol>:` second prefix **only** when the entire commit concerns one existing symbol —
  adding funcs to it, or modifying it.
* **Introducing** a new symbol does NOT take a second prefix (`Config: Added struct Config` is
  redundant) — use the type-kinded body form instead (RULE-COMMIT-006).
* A commit touching several symbols, or the package as a whole, keeps only the package scope.

```
# GOOD — the whole commit is about one existing symbol
cfapi.db.userprofile: UserProfilesController: Added funcs GetUserProfileByUserId() and GetUserProfileByUserIdForUpdate()
cfapi: Config: Added login-simplification config fields, made AWS required, unified AllowedOrigins

# GOOD — introduces a new symbol (type-kinded body, no second prefix)
cfapi.db.apps: Added interface ConfigsController

# GOOD — several symbols / package-wide
cfapi.api: Registered accounts services (LoginIntents, BigScreenLoginIntents, Passwords)

# BAD — second prefix on a multi-symbol change
cfapi.errors: Responder: Added code LoginFailed and func WithTooManyRequests()
```

---

## WORDING

**RULE-COMMIT-004** `[review]` — The first word after the final prefix is Capitalized.

* The first word after the final prefix is Capitalized.
* Symbol names keep their real identifier casing per `go.md` **RULE-GOLANG-NAME-002**: `CfApiClient`,
  `Otp`, `Id` — never `CFAPIClient`, `OTP`, `ID`.

**RULE-COMMIT-005** `[review]` — The subject states what the commit did, in the past tense (`Added`, `Moved`, `Registered`).

* State what the commit *did*, in the **past tense**: `Added`, `Moved`, `Registered`,
  `Implemented`, `Consolidated`, `Reduced`, `Switched`, `Wired`. Never imperative (`Add`) and never
  lower-case (`added`).

**RULE-COMMIT-006** `[review]` — A commit introducing a named symbol states its declaration kind before the name.

* When a commit introduces a named symbol, state its declaration kind before the name — the
  language's own kinds; for Go:
  * `Added struct <Name>`
  * `Added enum <Name>` — a named scalar type (`string`, `int`, …) together with its const set
  * `Added interface <Name>`
  * `Added sentinel error <Name>`

```
# GOOD
cfapi.models.accounts: Added enum LoginIntentState
cfapi.models.accounts: Added struct LoginIntent
cfapi: Added interface CfApiClient
cfapi.models: Added sentinel error ErrTooManyRequests
```

**RULE-COMMIT-007** `[review]` — Funcs, never "methods" — a func named in a message carries `()`.

* In Go there are no "methods" — call them **funcs**. A func named in a message carries `()`; a func on a
  type may use receiver notation (`App.Copy()`). Type kinds (`struct` / `enum` / `interface`) and
  vars (`sentinel error`) stay bare — they are not callable.

```
# GOOD
cfapi.models.apps: Added funcs App.Copy() and ProvideCompleteApp()

# BAD
cfapi.models.apps: Added method Copy to App
```

**RULE-COMMIT-008** `[review]` — No comma before a conjunction (`and` / `or`).

* No comma before a conjunction (`and` / `or`).

```
# GOOD
cfapi.api.middleware: Added CORS origin allowlist, wildcard handling and extra headers

# BAD
cfapi.api.middleware: Added CORS origin allowlist, wildcard handling, and extra headers
```

**RULE-COMMIT-009** `[review]` — The subject line does not end with a period.

* The subject line does not end with `.`.

---

## SPECIAL CASES

**RULE-COMMIT-010** `[review]` — A brand-new package is introduced in a single commit holding its final state.

* A brand-new package is introduced in a **single commit holding its final state**, with the
  subject:

  ```
  Import initial version of package <full.dot.path>
  ```

* No scope prefix — the full package path is already named in the sentence.

```
# GOOD
Import initial version of package cfapi.email
Import initial version of package cfapi.api.accounts
```

**RULE-COMMIT-011** `[review]` — Relocated code takes the source package it moved from as scope; the body names the destination.

* When code is relocated, the scope is the **source** package it moved *from*; the body names the
  destination package. Stage the removals and additions together so the move is recorded as a
  rename.

```
# GOOD
cfapi.worker: Moved live-TV recording and Zattoo tasks into cfapi.worker.livetv
```

**RULE-COMMIT-012** `[review]` — Never cite a `RULE-*` id in a message — describe the change in plain terms.

* Never cite a `RULE-*` id in a message — describe the change in plain terms.

**RULE-COMMIT-013** `[review]` — Never add `Co-authored-by` trailers or any AI-assistant mention.

* Never add `Co-authored-by` trailers or any AI-assistant mention.

---

## GROUPING

Grouping, validation and staging are defined by the `package-commit` skill. In short:

* One package per commit — never combine unrelated packages.
* A new package → one commit, final state (RULE-COMMIT-010).
* An existing package → one commit per new file / type.
* Skip generated files (per repo convention, e.g. `*.gen.go`).

---

## SUB-GUIDES

Part of the style set — see `go.md` (general Go incl. TESTS and GOROUTINES sections), `sql.md`, `plan.md` next to this file.
