# FIXTURE GUIDE (pass)

**RULE-GOLANG-NAME-005** `[lint]` — No interior underscores in identifiers.

* Covered by `revive/var-naming` and `gocritic/captLocal` — types and locals are checked.
* Exceptions: Go test function names (`TestType_Method`).

**RULE-GOLANG-ERR-005** `[lint]` — Out-of-module errors are wrapped.

* Covered by `wrapcheck` and `errorlint`.

**RULE-GOLANG-FUNC-001** `[review]` — Signatures follow the canonical shape.

* Covered by `staticcheck ST1008` and `revive/function-result-limit`. The parameter *name* `ctx` is a review check.
* Covered by `modernize/waitgroupgo` for exactly one shape. It is silent for a bulk `wg.Add(len(items))`.

**RULE-GOLANG-IMPORT-001** `[lint]` — Imports are formatted.

* Covered by `goimports` and `grouper`.

```go
// fenced example — a fake claim in a fence is ignored:
// * Covered by `nonexistent-linter`.
```
