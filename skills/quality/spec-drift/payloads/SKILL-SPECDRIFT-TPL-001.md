# Spec drift — <scope> (HEAD <short-sha>)

## Summary
<n claims: p pass / d drift / r dead / u unverifiable / c unchecked; caps and scope stated>

## Drift
| # | Claim | Doc (file:line) | Code (file:line or MISSING) | Verdict | Classification | Evidence |
| - | ----- | --------------- | --------------------------- | ------- | -------------- | -------- |
| 1 | <the claim, one line> | spec.md:42 | server/routes.go:88 | drift | code-wrong | <verbatim doc excerpt vs code excerpt> |

## Dead references
<claims whose code was not found at all, with the anchor searched>

## Unverifiable
<prose claims with no code anchor — listed, not judged>

## Unchecked
<claims past the per-run cap>

## Hand-offs
<drift findings that are contract changes → new contract-mode run; doc-stale items → doc-fix session>
