# GO CODE STYLE

**For reviewers / agents:** cite the stable `RULE-*` id when flagging a violation. Fix violations
rather than debating them. Prefer automatic fixes where a tool covers the rule (tagged `[gofmt]`,
`[goimports]`, `[vet]`); everything tagged `[review]` is a manual check.

**Scope:** all Go style. Test rules live in the **TESTS** section, concurrency rules in
**GOROUTINES** (both below); SQL style is `sql.md` next to this file — all equally mandatory.

---

## BASELINE

All code MUST follow official Go conventions:

* https://go.dev/doc/effective_go
* https://google.github.io/styleguide/go/

Where this document conflicts with Go conventions, Go conventions win — **EXCEPT** acronym /
initialism casing, which follows **RULE-NAME-002** (a deliberate, documented local override of the
Go "initialisms" convention).

---

## NAMING

**RULE-NAME-001** — Case `[review]`

* Exported identifiers MUST use PascalCase (`GameVote`); unexported MUST use camelCase (`gameVote`).
* Underscores are NOT allowed in identifiers. Exceptions (Go / tooling conventions): Go test
  function names (`TestType_Method`, `Test_helper`) and test-case meta fields (e.g. `_id`) — see
  `go-tests.md`.