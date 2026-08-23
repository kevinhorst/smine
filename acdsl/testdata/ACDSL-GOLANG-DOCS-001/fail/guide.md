# FIXTURE GUIDE (fail)

**RULE-GOLANG-NAME-005** `[lint]` — No interior underscores in identifiers.

* Covered by `varnamelen` — not enabled in the fixture config.

**RULE-GOLANG-ERR-005** `[lint]` — Out-of-module errors are wrapped.

* The lint claim above has no Covered-by bullet, which is its own violation.

**RULE-GOLANG-FUNC-001** `[review]` — Signatures follow the canonical shape.

* Covered by `revive/max-control-nesting`. That revive rule is not in the fixture's rule list.
* Covered by `modernize/stringscutprefix`. That check is disabled in the fixture config.
* Covered by `staticcheck ST1000`. That check is excluded by the fixture's checks pin.
