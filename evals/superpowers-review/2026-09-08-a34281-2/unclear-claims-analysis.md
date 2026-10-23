# Unclear uncataloged claims — adversarial pro/con analysis and rulings

The 8 groups where the two independent double-verification verifiers disagreed. For each, a fresh
prosecutor agent built the strongest case FOR the claim and a fresh refuter agent the strongest case
AGAINST, both against the pinned review head a342811b (range b4981b88..a342811b, golden v6). The
applied ruling per claim is marked — override any of them and the scores get recomputed.

Applied rulings at a glance:

| g | Topic | A / B (verifiers) | Ruling applied | Effect on score |
|---|---|---|---|---|
| 2 | ruleDelta swallows ParseRules violations | confirmed MINOR / claim-false | **claim-false** | +1 FP per instance (2 runs) |
| 4 | overlayViolations no base-diff, "hard stop" | confirmed INFO / claim-false | **claim-false** | +1 FP per instance |
| 5 | policyBase detached-HEAD string compare | confirmed INFO / matches-golden GV4-202 | **claim-false** (benign, disjoint from GV4-202) | +1 FP per instance |
| 14 | peekIdentity mismatch log overclaims | matches-golden GV3-147 / confirmed INFO | **matches-golden GV3-147** | golden credit (INFO) where unmatched |
| 19 | writePolicy aborts Dist on LoadPolicy error | confirmed MINOR / claim-false | **claim-false** (designed hard-error convention) | +1 FP per instance |
| 21 | installenv bare relative path, silent fallback | matches-golden GV6-252 / confirmed MINOR | **matches-golden GV6-252** | golden credit (MINOR) where unmatched |
| 29 | styleTestPrompt deviates from plan D8/D9, no Changelog row | confirmed MINOR / claim-false | **confirmed MINOR** | incl-new credit |
| 33 | repos Add form user-editable name vs plan C16/G17 | confirmed MAJOR / claim-false | **confirmed MAJOR** (refuter conceded) | incl-new credit |

Ruling rationale in one line each: g2 — the claims say "errors swallowed", but ParseRules has no
error return and a bad marker skips one line, not the file; the discard is documented scope
(refuter high-confidence, prosecutor conceded the wording). g4 — the flag-everything behavior is
the documented design and `local_overrides` is the sanctioned one-step path; "hard stop" refuted
(prosecutor itself downgraded to INFO). g5 — both sides agree no wrong result exists; detached
HEAD correctly yields boundary=true, and the GV4-202 exploit needs `base_ref: "HEAD"` config, a
provably disjoint path. g14 — same root cause as golden GV3-147 (unauthenticated /healthz decode);
the golden's fix removes the misleading log too; scoring both branches of one broken check
double-counts. g19 — the diff's own convention ("An unparseable base policy is a hard error") plus
the error text naming acdsl/policy.json refutes "unrelated-looking failure"; a design tradeoff, not
a defect. g21 — golden GV6-252 names internal/config/installenv.go and this mechanism verbatim;
the prosecutor conceded. g29 — the plan exists, is quoted, and the repo doctrine requires every
post-approval deviation in the plan's Changelog; the refuter's precedent covers claims that could
not locate a plan, which doesn't transfer. g33 — the refuter explicitly failed to refute: the plan
sanctions a label input only, the shipped form adds a name input wired into the identity key, and
no Changelog row sanctions it.

---

## g2 — policy.go: ruleDelta/ruleMarkers swallow ParseRules errors

**Instances:** sonnet-5-high-r21/SP-005 — internal/acdsl/policy.go; sonnet-5-high-r4/SP-005 — internal/acdsl/policy.go

**Claim (representative):** ruleDelta silently discards ParseRules errors on both the working and base sides, so an unparseable declaration file would silently under- or over-report added/removed rules, and no test exercises this path.

**Prior verdicts:** A: confirmed MINOR — ruleDelta discards ParseRules errors on both sides via `workRules, _ := ParseRules(workLines)` / `baseRules, _ := ParseRules(baseLines)`, so an unparseable declaration file silently drops added/removed/changed classification instead of surfacing an error. || B: claim-false — ParseRules (internal/acdsl/rule.go:53) returns ([]Rule, []string) — a violations list, never an error — so there is no 'ParseRules error' for ruleDelta's `workRules, _ := ParseRules(workLines)` to swallow; malformed markers are already skipped per-line inside ParseRules via parseMarker's problems slice, not as a file-level parse failure, so the claimed under/over-report mechanism misdescribes the actual code.

### FOR (prosecutor)

#### Case

`ruleDelta` (new in-range, `internal/acdsl/policy.go`) discards the second return value of `ParseRules` on both the work and base sides. That value is not an "error" as the members word it, but it is not nothing either: per `rule.go:52`, "Authoring problems are returned as violations, not errors" — a malformed `//acdsl:` marker line is dropped from the parsed rule set and recorded only in that discarded slice. `ruleDelta` never surfaces it, so a rule with a syntactically broken marker on either side silently vanishes from `workRules`/`baseRules` and gets misclassified as added/removed/changed with zero diagnostic explaining why. This is a real, in-range defect; only the members' choice of the word "error" is imprecise.

#### Key code

`internal/acdsl/policy.go:259-260`:
```go
workRules, _ := ParseRules(workLines)
baseRules, _ := ParseRules(baseLines)
```
`internal/acdsl/rule.go:51-53`:
```go
// ParseRules scans fileLines (repo-relative path → lines) for markers.
// Authoring problems are returned as violations, not errors — mirrors
// contextdocs.ValidateRules so the CLI prints one line per problem.
func ParseRules(fileLines map[string][]string) ([]Rule, []string) {
```
`rule.go:75-78`: a malformed marker's `problems` are appended to `violations` and the line is `continue`d — dropped from `rules` entirely, with no propagation path back through `ruleDelta`.

#### Against the dissent

Verifier B: "ParseRules ... returns ([]Rule, []string) — a violations list, never an error ... so there is no 'ParseRules error' ... to swallow." Conceded on the literal wording — there is no error return, and B is right to flag that mislabeling. But B stops at the vocabulary and doesn't follow through on the substance: the discarded `[]string` is exactly the per-line "malformed marker" signal B itself names, and `ruleDelta` swallows it just as completely as it would an error. The claimed effect — "an unparseable declaration... silently under- or over-report added/removed rules" — is unaffected by the rename from "error" to "violation": a broken marker still disappears from `workRules`/`baseRules` and still produces a wrong added/changed/removed classification with no diagnostic. B refutes a word choice, not the mechanism.

#### Severity if real

MINOR — matches GV3-097's calibration band (untested/unsurfaced degradation of the policy gate, not a bypass); this is a distinct mechanism (marker-parse violations in `ruleDelta`, not the base-policy-JSON hard-error path GV3-097 covers) and does not appear elsewhere in the golden catalog's `policy.go` entries.

#### Confidence

medium — the discard and its consequence are directly quoted and in-range; the only softening factor is that the members' own wording ("ParseRules errors") is technically wrong, which could sway a strict-wording judge toward false even though the underlying defect is real.

### AGAINST (refuter)

#### Case

Both member claims describe `ParseRules`'s discarded second return as an "error" being "swallowed," implying a file-level parse failure that could silently zero out the delta. `ParseRules` has no error return at all — its second value is a per-marker-line violations list that is deliberately not an all-or-nothing gate, and it is already surfaced through a separate, dedicated call path (`load.go`). The claimed failure mode (a malformed file yields zero delta entries) does not match what the code does.

#### Key code

`internal/acdsl/rule.go:53`
```go
func ParseRules(fileLines map[string][]string) ([]Rule, []string) {
```
`internal/acdsl/rule.go:78-89` (per-line handling — a bad marker skips only that one line, not the whole file):
```go
rule, problems := parseMarker(rest, path, i+1)
if len(problems) > 0 {
    violations = append(violations, problems...)
    continue
}
...
rules = append(rules, rule)
```
`internal/acdsl/policy.go:234-236` (ruleDelta's own doc comment, already scoping the ignore to base-side authoring):
```go
// classifies rules as added, removed, or changed ... Base-side authoring
// violations are ignored (the base was committed green).
```
`internal/acdsl/policy.go:259-260` (the disputed lines):
```go
workRules, _ := ParseRules(workLines)
baseRules, _ := ParseRules(baseLines)
```
`internal/acdsl/load.go:70` — the violations value is captured and used elsewhere in the same package, showing the discard in policy.go is a deliberate scope choice, not a swallowed error:
```go
rules, violations := ParseRules(fileLines)
```

#### Against the confirmation

VerifierA's reason: "ruleDelta discards ParseRules errors on both sides ... so an unparseable declaration file silently drops added/removed/changed classification instead of surfacing an error." This mischaracterizes the mechanism twice: (1) there is no error value — `ParseRules` returns `[]string` violations, confirmed at rule.go:53 and by rule_test.go's own `rules, violations := ParseRules(...)` usage; (2) a malformed marker line is excluded rule-by-rule (rule.go:80-82 `continue`), not file-by-file — the rest of a "syntactically broken" file's valid markers still parse and still classify normally, so "silently drop added/removed/changed classification" for the whole file is false. What actually happens on a bad line is exactly one candidate rule id doesn't exist in `workRules`/`baseRules`, which is indistinguishable from that rule simply not being declared — a correctness-neutral outcome already covered by ruleDelta's documented base-side ignore policy, extended symmetrically to the work side since violations are linted through `load.go`'s separate path, not this delta-classification path.

#### Verdict if I prevail

claim-false

#### Confidence

high — verified by direct read of rule.go:53-92 (signature and per-line continue), policy.go:234-236,259-260 (doc comment and call sites), and load.go:70 (the violations value is consumed elsewhere), all within the review range b4981b88..a342811b.

---

## g4 — policy.go: overlayViolations reads acdsl/registry.local.json off disk, not via base diff

**Instances:** sonnet-5-high-r12/SP-007 — internal/acdsl/policy.go

**Claim (representative):** overlayViolations reads acdsl/registry.local.json straight off disk rather than via the base-commit diff machinery, so a pre-existing unsanctioned overlay entry from before the policy was enabled flags on every acdsl check in that repo forever, making gated-mode adoption in a repo with a legacy local overlay a hard stop.

**Prior verdicts:** A: confirmed INFO — overlayViolations reads acdsl/registry.local.json straight off disk (os.ReadFile) rather than through GitShowFile/base diff, so a pre-existing unsanctioned name in that file flags on every check until policy.LocalOverrides is updated; real but a one-time config fix, not a permanent block, so rated low. || B: claim-false — overlayViolations is deliberately not diffed against base (doc comment: 'overlay wins by name at load time'), and policy.LocalOverrides is exactly the sanctioned escape hatch for a legacy entry — adding the entry name to local_overrides silences the flag. This is a working gate design (flag stays until explicitly sanctioned or removed), not a 'hard stop'; the claim overstates onboarding friction as a defect.

### FOR (prosecutor)

#### Case
`overlayViolations` reads `acdsl/registry.local.json` off disk with no base-diffing, and flags every entry not named in `policy.LocalOverrides` on every gated/strict check, forever, until a human edits the shared `policy.json`. This is real behavior, confirmed in-range (whole file is new in `b4981b88..a342811b`), and distinct from the golden catalog's only nearby entry (GV3-108, an untested-branches claim, not this design/onboarding-friction claim).

#### Key code
`internal/acdsl/policy.go:143-172`:
```
func overlayViolations(root string, policy Policy) ([]Diagnostic, error) {
	path := filepath.Join(root, "acdsl", LocalRegistryName)
	raw, err := os.ReadFile(path)
	...
	sanctioned := map[string]bool{}
	for _, name := range policy.LocalOverrides {
		sanctioned[name] = true
	}
	...
	for _, name := range names {
		if sanctioned[name] {
			continue
		}
		diagnostics = append(diagnostics, policyDiagnostic(...))
	}
```
`internal/acdsl/policy.go:329-355` shows this runs unconditionally in gated/strict mode on every non-base-branch check (`CheckPolicy`), with no base-ref comparison anywhere in the function.

#### Against the dissent
VerifierB: "adding the entry name to local_overrides silences the flag... a working gate design, not a 'hard stop'." True that the escape hatch works — but that only argues severity, not falsity. Nothing in the code contradicts the claim's substance: any overlay entry absent from `policy.LocalOverrides` — pre-existing or newly added by any developer — is flagged on every check, permanently, until someone edits and lands the shared `acdsl/policy.json`. Since `LocalRegistryName` is explicitly documented as "(tracked or not)" (line 139-140), a genuinely personal/untracked local override can never be silenced without promoting it into the shared, committed policy file — defeating the "local" premise. B's rebuttal shows the workaround exists, it doesn't show the claim is inaccurate about mechanism or friction. VerifierA already concedes it's real; B's "claim-false" verdict conflates "has a documented fix" with "not a defect."

#### Severity if real
INFO — real, reachable one-time fix per name, no silent security bypass or data loss; matches verifierA's calibration, not a BLOCKER/MAJOR onboarding blocker as the raw claim text implies.

#### Confidence
high — code quoted directly confirms no base-diffing and unconditional flagging; the only golden-catalog neighbor (GV3-108) is a different claim (test coverage of edge branches), so this is not a duplicate.

### AGAINST (refuter)

#### Case

`overlayViolations` not diffing against base is documented, intentional behavior, not an oversight: the function's own doc comment says the overlay "wins by name at load time," and the flag mechanism ships its own sanctioned escape hatch (`policy.LocalOverrides`) for exactly the legacy-entry case SP-007 describes. Calling this a "hard stop" for gated-mode adoption overstates a one-line config fix as a blocking defect.

#### Key code

`internal/acdsl/policy.go:143-172`:
```
// overlayViolations reads acdsl/registry.local.json directly (tracked or
// not) and flags every entry name missing from policy.LocalOverrides — the
// overlay wins by name at load time, so an unguarded entry re-points any
// verifier.
func overlayViolations(root string, policy Policy) ([]Diagnostic, error) {
	path := filepath.Join(root, "acdsl", LocalRegistryName)
	raw, err := os.ReadFile(path)
	...
	sanctioned := map[string]bool{}
	for _, name := range policy.LocalOverrides {
		sanctioned[name] = true
	}
	...
	for _, name := range names {
		if sanctioned[name] {
			continue
		}
		diagnostics = append(diagnostics, policyDiagnostic("acdsl/"+LocalRegistryName, 1, fmt.Sprintf("policy(%s): overlay entry %q is not sanctioned in local_overrides", policy.Mode, name)))
	}
	return diagnostics, nil
}
```
Confirmed new in range: `git diff b4981b88..a342811b -- internal/acdsl/policy.go` shows lines 145-172 (including this whole function and the `LocalOverrides` field) as added (`+`).

#### Against the confirmation

VerifierA's reason: "a pre-existing unsanctioned name in that file flags on every check until policy.LocalOverrides is updated; real but a one-time config fix, not a permanent block, so rated low." This already concedes the core of B's rebuttal — VerifierA does not actually dispute that `LocalOverrides` resolves it, it just still counts the flag itself as a defect worth reporting. But the claim under dispute (SP-007) goes further than A's own reason: it calls this "a hard stop" for "gated-mode adoption," not a one-time fix. That characterization is unsupported by the code — sanctioning one name in one JSON field, once, is the documented normal-adoption path, not a hard stop. The design intentionally requires an explicit acknowledgment of legacy overlay entries before gating; that's a feature of the gate (nothing silently re-points a verifier), not a bug in the diffing logic.

#### Verdict if I prevail

claim-false (the "hard stop" characterization is refuted by the sanctioned escape hatch being the designed, one-step fix; behavior is intentional per the doc comment, not a missed-diff bug)

#### Confidence

medium — the code and comment clearly support "intentional, one-step-fixable," so the claim's "hard stop" framing is refutable; but VerifierA's INFO-severity confirmation is narrow enough (acknowledging the config-fix path) that a judge could still credit a softened version of the claim as a minor finding.

---

## g5 — policy.go: policyBase isBoundary string compare, no detached-HEAD handling

**Instances:** sonnet-5-high-r32/SP-007 — internal/acdsl/policy.go

**Claim (representative):** policyBase derives isBoundary from a straight branch != baseName string compare with no explicit handling for a fully detached HEAD, relying on the incidental fact that "HEAD" != "main" evaluates the right way.

**Prior verdicts:** A: confirmed INFO — policyBase's isBoundary is `branch != baseName` with no explicit detached-HEAD branch; GitBranch returns the literal string "HEAD" for detached HEAD, so isBoundary being correct there is incidental string inequality, not a checked case — but the claim itself concedes it 'evaluates the right way', i.e. no wrong behavior is alleged, only fragility. || B: matches-golden GV4-202 — Same code region and same underlying mechanism as GV4-202: policyBase's isBoundary is a naive branch != baseName string compare (policy.go:195-218) that only 'accidentally' works because git returns the literal string "HEAD" for a detached checkout; GV4-202 already catalogs this exact comparison's weakness (via the base_ref="HEAD" case) as a BLOCKER.

### FOR (prosecutor)

#### Case

`policyBase` computes `isBoundary` as a bare `branch != baseName` string compare with no explicit branch on a detached HEAD; correctness for that state depends only on the incidental fact that `git rev-parse --abbrev-ref HEAD` prints the literal string `"HEAD"` and that string happens not to collide with any real base-branch name. This is a distinct, narrower observation than GV4-202: it is about the *default* candidate-resolution path (no attacker-controlled `base_ref`), not about a maliciously supplied `base_ref: "HEAD"`.

#### Key code

`internal/acdsl/policy.go:195-216`:
```
func policyBase(ctx context.Context, root string, policy Policy) (string, bool, error) {
	candidates := []string{"origin/HEAD", "origin/main", "main", "master"}
	...
		branch, err := GitBranch(ctx, root)
		...
		baseName = strings.TrimPrefix(baseName, "origin/")
		return base, branch != baseName, nil
```
`internal/acdsl/git.go:36-42`:
```
func GitBranch(ctx context.Context, root string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	...
	return strings.TrimSpace(output), nil
}
```
No branch of `policyBase` special-cases `branch == "HEAD"`.

#### Against the dissent

Verifier B: "Same code region and same underlying mechanism as GV4-202 ... only 'accidentally' works because git returns the literal string \"HEAD\" for a detached checkout; GV4-202 already catalogs this exact comparison's weakness."

This conflates two different triggers on the same lines. GV4-202's exploit is `policy.BaseRef` (attacker/committer-controlled config) set to the literal `"HEAD"`, which forces `candidates = []string{"HEAD"}`, so `baseName == "HEAD"` *by config*, and `merge-base(HEAD, HEAD) == HEAD` makes every downstream diagnostic vacuous — a self-management-gate bypass reachable from a normal (attached) branch, no detached checkout required. SP-007 never invokes `BaseRef`; it is about the unconfigured default path (`origin/HEAD`/`origin/main`/`main`/`master`), where `baseName` resolves to a real branch name and `branch` is the literal `"HEAD"` only because the *runner* happens to be in detached-HEAD state — a runtime/CI condition, not a config value. For SP-007's scenario to reduce to GV4-202's bypass, `baseName` would independently have to equal `"HEAD"`, which does not happen on the default candidate list. The root causes are different: one is exploitable via config, the other is an un-asserted invariant about CI checkout mode with no demonstrated bypass.

#### Severity if real

INFO — the claim itself concedes current behavior evaluates correctly; this flags an unchecked invariant/fragility, not a proven wrong result.

#### Confidence

medium — the code quote is exact and the mechanism genuinely differs from GV4-202's config-driven bypass, but I cannot construct a concrete input (absent a colliding branch/base name) where `isBoundary` is actually wrong, so the "defect" is fragility rather than a demonstrated bug.

### AGAINST (refuter)

#### Case

SP-007 and GV4-202 both cite `policy.go:195` but describe two different conditions that happen to produce the same literal string "HEAD" — one from `GitBranch` (an actual detached checkout), the other from `policy.BaseRef` (an attacker-set config value). Only the latter is exploitable; SP-007's own text concedes the former "evaluates the right way," i.e. no defect. VerifierA's INFO/no-wrong-behavior read is correct; verifierB's `matches-golden GV4-202` conflates the two mechanisms.

#### Key code

`internal/acdsl/policy.go:189-216` (new file in range b4981b88..a342811b, confirmed via `git diff`):
```
196	candidates := []string{"origin/HEAD", "origin/main", "main", "master"}
197	if policy.BaseRef != "" {
198		candidates = []string{policy.BaseRef}
199	}
...
204		branch, err := GitBranch(ctx, root)
...
208		baseName := candidate
209		if candidate == "origin/HEAD" {
210			if resolved, resolveErr := shellGitAbbrevRef(ctx, root, candidate); resolveErr == nil {
211				baseName = resolved
212			}
213		}
214		baseName = strings.TrimPrefix(baseName, "origin/")
215		return base, branch != baseName, nil
```
`internal/acdsl/git.go:36-39`: `GitBranch` runs `git rev-parse --abbrev-ref HEAD`, returning the literal `"HEAD"` only when the checkout is physically detached.

#### Against the confirmation

VerifierB: "isBoundary...only 'accidentally' works because git returns the literal string 'HEAD' for a detached checkout; GV4-202 already catalogs this exact comparison's weakness (via the base_ref='HEAD' case)."

This is wrong on the mechanism match. GV4-202's exploit needs `policy.BaseRef == "HEAD"` (line 197-198: `candidates = []string{policy.BaseRef}`), which makes `baseName == "HEAD"` (line 208, never rewritten — the `origin/HEAD` special-case at 209 doesn't fire for a bare `"HEAD"` candidate) and `base == merge-base(HEAD,HEAD) == HEAD`, so a same-commit rewrite of the frozen surface produces an empty diff — a real, attacker-triggerable bypass. SP-007's scenario is a detached physical checkout with a normal `base_ref` (or default candidates, none of which resolve to literal `"HEAD"` for `baseName` — `origin/HEAD` is rewritten to a real branch name at line 210-212). There, `branch == "HEAD"` but `baseName` is a real branch (e.g. `"main"`), so `isBoundary` is `true` — correctly treating the detached commit as a boundary, exactly as intended by the doc comment at policy.go:189-191 ("a fresh agent branch at the base tip is a boundary"). No bypass, no silent-free path exists for SP-007's condition — verifierB never shows one.

#### Verdict if I prevail

claim-false in the sense that it does **not** match-golden GV4-202 (different trigger, no exploit) — it is benign-by-design, correctly assessed by verifierA as INFO, not a BLOCKER duplicate.

#### Confidence

high — the code paths for `policy.BaseRef=="HEAD"` (golden's exploit) and physical detached-HEAD (`GitBranch` returning `"HEAD"`) are provably disjoint at policy.go:197-198 vs 204, and only the former reaches `baseName=="HEAD"`.

---

## g14 — configserver main.go: peekIdentity mismatch log overclaims 'another profile's peek'

**Instances:** sonnet-5-high-r31/SP-003 — cmd/configserver/main.go

**Claim (representative):** peekIdentity's mismatch branch logs "another profile's peek holds this port" even when the listener is an unrelated process that happens to return decodable JSON on /healthz, which can mislead an operator debugging a port conflict.

**Prior verdicts:** A: matches-golden — Same mechanism already golden-cataloged: peekIdentity/startPeek trusts a plain, unauthenticated JSON decode of /healthz as sufficient proof of process identity; GV3-147 covers the security consequence (a spoofable listener), and this claim's 'overclaims another profile's peek' framing is the same root defect (identity check validates JSON shape only, not actual origin) viewed from the misleading-log-message angle. || B: confirmed — peekIdentity's json.Decode succeeds for any 200-status JSON body regardless of whether it actually has claudeHome/codexHome fields (missing fields simply decode to zero-value strings), so startPeek's mismatch branch logs "another profile's peek holds this port" even when the listener is an unrelated service that merely returns some unrelated valid JSON object on /healthz — a misleading operator-facing diagnostic, not a functional defect (the fallback behavior, disabling the session column, is correct either way).

### FOR (prosecutor)

#### Case

SP-003 is a distinct defect from GV3-147: GV3-147 is about the *match* branch trusting an unauthenticated JSON body enough to reuse a spoofed listener (a security bypass). SP-003 is about the *mismatch* branch's log message asserting a specific, unverifiable cause ("another profile's peek") for any field mismatch, including cases with no other profile involved at all — a misleading-diagnostic defect, not a security-bypass defect. Root causes differ: GV3-147's fix is "authenticate before trusting," SP-003's fix is "don't assert an identity you didn't verify." This is new and not in the golden catalog under either candidate ID.

#### Key code

`cmd/configserver/main.go:222-235` (`peekIdentity`):
```go
func peekIdentity(addr string) (*peekID, error) {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	...
	var id peekID
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return nil, err
	}
	return &id, nil
}
```
`cmd/configserver/main.go:252`:
```go
log.Printf("ERROR: peek-mcp on %s serves claude-home=%s codex-home=%s, want %s / %s — another profile's peek holds this port; set PEEK_PORT/PEEK_CONTROL_PORT and reinstall (session column disabled)", addr, id.ClaudeHome, id.CodexHome, claudeHome, codexHome)
```
`Decode` succeeds on any JSON object; a listener returning e.g. `{"status":"ok"}` on `/healthz` decodes to `id.ClaudeHome==""`, `id.CodexHome==""`, which mismatches `claudeHome`/`codexHome` and falls straight into the "another profile's peek holds this port" branch — a specific, wrong causal claim about an unrelated service.

#### Against the dissent

Verifier A: "this claim's 'overclaims another profile's peek' framing is the same root defect [as GV3-147] (identity check validates JSON shape only, not actual origin) viewed from the misleading-log-message angle." This conflates the two failure directions. GV3-147's evidence (line 248, the match branch: `id.ClaudeHome == claudeHome && id.CodexHome == codexHome`) is about a *false positive* — the check accepting a hostile actor as legitimate, enabling session-data leakage to an attacker. SP-003 is about a *false negative/mislabel* on the mismatch branch (line 252) — the check correctly refuses reuse (safe outcome, `return false`, session column disabled) but attributes the refusal to a wrong cause. The security-relevant golden entry is about trusting unauthenticated data to permit an action; SP-003's defect exists even in a completely benign environment with zero adversaries — any non-peek-mcp process bound to the port produces this exact wrong log line. Same code region, non-overlapping mechanism.

#### Severity if real

INFO — misleading log/diagnostic text only; the fallback behavior (disable session column) is correct regardless of the log's claimed cause.

#### Confidence

medium — the code and golden text clearly support a distinct root cause, but this is a naming/attribution nuance (log message correctness) rather than a functional bug, so reasonable reviewers could still fold it into GV3-147 as "close enough" scope.

### AGAINST (refuter)

#### Case

SP-003's "misleading log" defect and GV3-147's "spoofable listener" defect are the two branches of the exact same `if/else` on the exact same line, both caused by the exact same root defect: `peekIdentity` trusts a plain, unauthenticated JSON decode of `/healthz` as proof of process identity. GV3-147 already catalogs that root cause at this code; SP-003 is a restatement of it from the failure-mode angle rather than the success-mode angle.

#### Key code

`cmd/configserver/main.go:247-253`:
```
if id, err := peekIdentity(addr); err == nil {
    if id.ClaudeHome == claudeHome && id.CodexHome == codexHome {          // line 248 — GV3-147's target
        log.Printf("peek-mcp %s already serving on %s for this profile, not spawning", id.Version, addr)
        return true
    }
    log.Printf("ERROR: peek-mcp on %s serves claude-home=%s codex-home=%s, want %s / %s — another profile's peek holds this port; ...", addr, id.ClaudeHome, id.CodexHome, claudeHome, codexHome)  // SP-003's target
    return false
```
Both branches execute only after `peekIdentity` (`main.go:223-235`) has decoded *any* well-formed JSON body from `/healthz` with no signature, token, or origin check — `json.NewDecoder(resp.Body).Decode(&id)` — the same unauthenticated-decode call GV3-147 cites.

#### Against the confirmation

VerifierB's reason: "`json.Decode` succeeds for any 200-status JSON body regardless of whether it actually has claudeHome/codexHome fields ... startPeek's mismatch branch logs 'another profile's peek holds this port' even when the listener is an unrelated service."

This is correct as a description of the code, but it identifies the same defect GV3-147 already catalogs at the same `if`/line region, just entered via the false branch instead of the true branch. GV3-147: "trusts a self-reported, unauthenticated JSON body (`claudeHome`/`codexHome` strings) over plain loopback HTTP to decide whether to reuse an already-listening peek-mcp." The unauthenticated-trust defect is identical; SP-003 does not identify a second, independent bug — it identifies the inevitable corollary (an unauthenticated field-compare that can wrongly confirm identity can equally wrongly deny it and mislabel the cause). Cataloging both branches of one broken identity check as separate findings double-counts a single root cause already golden.

#### Verdict if I prevail

matches-golden GV3-147 (quoted above, `cmd/configserver/main.go:248` — the branch pair SP-003 also targets)

#### Confidence

medium — the mechanism-identity argument is solid (same conditional, same unauthenticated-decode root cause), but GV3-147's own text frames the *consequence* as a security spoof on the match path, while SP-003's consequence (a misleading log on the mismatch path) is a distinct operator-facing symptom; a strict reader could reasonably keep them as separate INFO-level findings.

---

## g19 — dist.go: writePolicy aborts Dist on non-ErrNotExist LoadPolicy error

**Instances:** sonnet-5-high-r11/SP-006 — internal/acdsl/dist.go

**Claim (representative):** writePolicy aborts Dist entirely when LoadPolicy returns a non-ErrNotExist error (e.g. malformed JSON mid-edit), which may surface as an unrelated-looking dist failure.

**Prior verdicts:** A: confirmed MINOR — writePolicy calls LoadPolicy(root), and LoadPolicy returns an error (not just for ErrNotExist) whenever acdsl/policy.json exists but fails to parse (parsePolicy error on malformed JSON or unknown mode); writePolicy propagates that straight up, aborting the entire Dist() build for a policy-file typo unrelated to most shipped binaries. || B: claim-false — Aborting Dist on a non-ErrNotExist LoadPolicy error is consistent with this same diff's established convention that an unparseable policy is always a hard error (basePolicy's own doc comment: 'An unparseable base policy is a hard error'), not a silent-fallback bug; the wrapped error text ('writePolicy: parsePolicy: ...') also clearly names the policy file, contradicting the 'unrelated-looking' framing.

### FOR (prosecutor)

#### Case

`writePolicy` (new in this diff) calls `LoadPolicy`, and any non-`ErrNotExist` error — most plausibly a malformed `acdsl/policy.json` — aborts the entire `Dist()` build via error propagation, not just policy shipping. This is a real, in-range behavior with a genuine severity question (blocking a full binary/rule distribution because of a policy-file parse error), and it is not in the golden catalog (nearest golden dist.go findings are at lines 74, 206, 243, 251 — none touch writePolicy/LoadPolicy).

#### Key code

`internal/acdsl/dist.go:104-107` (Dist call site):
```go
distMode, err := writePolicy(root, dest)
if err != nil {
    return nil, err
}
```

`internal/acdsl/dist.go:233-236` (writePolicy):
```go
policy, err := LoadPolicy(root)
if err != nil {
    return "", fmt.Errorf("writePolicy: %w", err)
}
```

`internal/acdsl/policy.go:386-392` (LoadPolicy's non-ErrNotExist path):
```go
if err != nil {
    return Policy{}, fmt.Errorf("LoadPolicy: %w", err)
}
policy, err := parsePolicy(raw)
if err != nil {
    return Policy{}, fmt.Errorf("LoadPolicy: acdsl/policy.json: %w", err)
}
```

#### Against the dissent

Verifier B's reason: "consistent with this same diff's established convention that an unparseable policy is always a hard error (basePolicy's own doc comment: 'An unparseable base policy is a hard error')."

That doc comment (`internal/acdsl/policy.go:62`) is scoped to `basePolicy`, which reads the policy at a *base ref* for drift-checking — a narrow, policy-focused operation where a hard stop is the obvious contract. `writePolicy` is not that function; it is one step inside `Dist()`, which also ran `writeRules`, `writeDistRegistry`, and `buildBinaries` successfully before reaching it (dist.go:95-105). The doc-comment convention establishes that *policy loading itself* should hard-fail — it says nothing about whether a full, otherwise-successful distribution build should be aborted because of it. Verifier B's "unrelated-looking" rebuttal is stronger on the error-text point (the wrapped message does say `writePolicy: LoadPolicy: acdsl/policy.json: ...`, naming the file), but the message being named doesn't change the code-level fact: a build that has nothing else wrong with it fails entirely for a policy typo, when the more targeted alternative — failing to ship/update `policy.json` while still emitting rules/registry/binaries — was available and is exactly the "no source policy ships nothing and returns ''" pattern already used for the ErrNotExist-adjacent stat check two lines below (dist.go:237-239). So the design intent elsewhere in this same function is graceful degradation on missing policy; only a malformed one is escalated to the harshest possible outcome (whole-build abort), which is the actual inconsistency, not a "convention".

#### Severity if real

MINOR — degraded blast radius (whole dist aborts) rather than a correctness/security defect; policy files are self-authored so triggering it isn't user-facing/attacker-facing.

#### Confidence

medium — the code fact (propagation aborting Dist) is undisputed by both verifiers; the dispute is purely about whether this is "intended hard-error convention" vs "an avoidable added-blast-radius bug," and reasonable reviewers can land either way since no explicit spec text says whether writePolicy itself must be all-or-nothing with the rest of Dist.

### AGAINST (refuter)

#### Case

The behavior claimed is real but not a defect: it is the same "malformed policy is a hard error" convention this diff already establishes for `basePolicy`, and the failure is not "unrelated-looking" — the propagated error always names `acdsl/policy.json` by construction. There is no silent fallback anywhere in this path; every failure mode surfaces with the offending file in the message.

#### Key code

`internal/acdsl/dist.go:233-236` (new in this diff):
```
policy, err := LoadPolicy(root)
if err != nil {
    return "", fmt.Errorf("writePolicy: %w", err)
}
```
`internal/acdsl/policy.go:378-394` (`LoadPolicy`):
```
raw, err := os.ReadFile(filepath.Join(root, "acdsl", "policy.json"))
if errors.Is(err, os.ErrNotExist) { ... return policy, nil }
if err != nil {
    return Policy{}, fmt.Errorf("LoadPolicy: %w", err)
}
policy, err := parsePolicy(raw)
if err != nil {
    return Policy{}, fmt.Errorf("LoadPolicy: acdsl/policy.json: %w", err)
}
```
`internal/acdsl/policy.go:62`: "An unparseable base policy is a hard error."

#### Against the confirmation

Verifier A's evidence is accurate on mechanics but the "may surface as an unrelated-looking dist failure" framing doesn't hold up. Two error sources feed the non-ErrNotExist branch: (1) a parse failure, wrapped as `"LoadPolicy: acdsl/policy.json: %w"` — explicitly names the file; (2) a non-ENOENT `os.ReadFile` error (permission denied, is-a-directory, etc.) — `os.ReadFile` returns a `*fs.PathError` whose `.Error()` already embeds the full path, so even the terser `"LoadPolicy: %w"` wrap surfaces as `writePolicy: LoadPolicy: open .../acdsl/policy.json: permission denied`. Either way the final message names the exact file. "Unrelated-looking" is not accurate.

Verifier B's point stands unrebutted: this diff's own doc comment at policy.go:62 states unparseable policy is deliberately a hard error elsewhere in the same file, and `writePolicy` (Dist's ship-the-policy step) applying the same "don't ship silently-degraded config" stance is consistent design, not an oversight — Dist genuinely cannot produce a coherent target policy.json from an unreadable source one.

#### Verdict if I prevail

claim-false

#### Confidence

medium — the mechanics A cites are correct and this is a legitimate design-tradeoff observation (aborting the whole build for a scoped file), but the specific rationale offered ("unrelated-looking," implying poor diagnosability) is directly contradicted by the wrapped error text, and the same-file precedent for treating unparseable policy as a hard error undercuts "bug" framing.

---

## g21 — installenv.go: bare relative install.env path, silent port fallback

**Instances:** sonnet-5-high-r13/SP-005 — internal/config/installenv.go

**Claim (representative):** installEnvPath is read via the bare relative path "install.env", so ExpandInstallMarkers silently falls back to DefaultPeekControlPort with no logging whenever the process cwd is not the repo root.

**Prior verdicts:** A: matches-golden — GV6-252 explicitly names this exact defect: 'saveProfileSelection and internal/config/installenv.go both read from repo-relative paths rather than an injected root, adding second and third instances of the same implicit CWD==repo-root contract' — the same bare-relative-path/silent-fallback issue this claim raises for installEnvPath. || B: confirmed — installEnvPath is the bare relative literal "install.env"; os.ReadFile resolves it against the process's current working directory, so ExpandInstallMarkers silently falls back to the map's preloaded DefaultPeekControlPort with zero logging whenever the caller's cwd isn't the repo root at call time.

### FOR (prosecutor)

#### Case

SP-005 is a real in-range defect: `installEnvPath` is a bare relative literal, `os.ReadFile` resolves it against process cwd, and any resolution failure is swallowed into a preloaded default with zero diagnostic. The honest question is not whether the bug is real (it is) — it's whether it earns credit beyond GV6-252, and the honest answer is mostly no.

#### Key code

`internal/config/installenv.go:8-12`
```
const (
	DefaultPeekControlPort = "42442"

	installEnvPath = "install.env"
)
```
`internal/config/installenv.go:24-29`
```
func installEnvValues(path string) map[string]string {
	values := map[string]string{"PEEK_CONTROL_PORT": DefaultPeekControlPort}
	data, err := os.ReadFile(path)
	if err != nil {
		return values
	}
```
Confirmed in-range: `git -C verify-wt diff b4981b88..a342811b -- internal/config/installenv.go` shows the whole file as a new addition in this range (`new file mode 100644`).

#### Against the dissent

VerifierA: "GV6-252 explicitly names this exact defect: '...internal/config/installenv.go both read from repo-relative paths rather than an injected root, adding second and third instances of the same implicit CWD==repo-root contract'". This is correct and I can't honestly refute it — GV6-252 names this file by path and describes the identical root cause (bare repo-relative path standing in for an injected root, breaking whenever cwd != repo root). SP-005's core claim ("bare relative path so cwd-dependent resolution fails") is the same defect already catalogued.

The only distinguishable thread is SP-005's extra emphasis on *silent* fallback with *no logging* — a consequence of the read failing, not a separate root cause. GV6-252's fix (inject the root) also fixes the failure mode SP-005 describes; they are not independent bugs, just two write-ups of one CWD-assumption defect at the same file. I can't build a credible case that root causes diverge.

#### Severity if real

MINOR — matches VerifierB's calibration; silent fallback to a hardcoded port default is a real but low-blast-radius correctness gap, not a data-loss or security issue.

#### Confidence

Low — this is a golden match, not a distinct defect. Verifier A's dissent stands; I cannot honestly argue divergent root causes.

### AGAINST (refuter)

#### Case

SP-005's claim — `installEnvPath` is a bare relative path so `ExpandInstallMarkers` silently falls back to `DefaultPeekControlPort` when cwd isn't the repo root — is not a new defect. The golden catalog already names this exact file and mechanism inside GV6-252. Verifier A caught this; Verifier B's "confirmed" (distinct, MINOR) verdict rests on a calibration reference (GV3-069) that is a different file and a different bug entirely.

#### Key code

`verify-wt/internal/config/installenv.go:11`:
```
installEnvPath = "install.env"
```
`verify-wt/internal/config/installenv.go:24-27`:
```
func installEnvValues(path string) map[string]string {
	values := map[string]string{"PEEK_CONTROL_PORT": DefaultPeekControlPort}
	data, err := os.ReadFile(path)
	if err != nil {
		return values
	}
```
`git -C verify-wt diff b4981b88..a342811b -- internal/config/installenv.go` shows the whole file as `new file mode 0000000..c7f75fe` — in range, not disputed on that front.

Golden catalog, `golden-v6.json` `.findings[]`, `golden_id: "GV6-252"`:
```
"file": "internal/server/presentation.go", "line": 117,
"claim": "saveProfileSelection and internal/config/installenv.go both read from
repo-relative paths rather than an injected root, adding second and third
instances of the same implicit CWD==repo-root contract."
```

#### Against the confirmation

Verifier B's reason: "installEnvPath is the bare relative literal... ExpandInstallMarkers silently falls back... with zero logging" — this is a correct read of the code, but it doesn't engage with the golden catalog at all. B calibrates severity against **GV3-069** ("install.env home values written unquoted → spaced home aborts install... in install.sh:30") — a different file, a different bug (unquoted shell var vs. Go path resolution), not even about `installenv.go`. That calibration pick is simply wrong; the applicable golden entry is GV6-252, which B never checked and which literally spells out "internal/config/installenv.go" reading from a repo-relative path as one of three instances of the same CWD-assumption pattern. B's "distinct new finding" framing only holds if GV6-252 is ignored.

#### Verdict if I prevail

matches-golden GV6-252

#### Confidence

high — GV6-252's claim text names `internal/config/installenv.go` verbatim and describes the identical repo-relative-path mechanism; Verifier B's alternate calibration (GV3-069) is off-target (different file, different bug) and doesn't rebut the golden match.

---

## g29 — profilesettings.go: styleTestPrompt/handleProfileTest deviate from plan D8/D9 without Changelog

**Instances:** sonnet-5-high-r18/SP-004 — internal/server/profilesettings.go; sonnet-5-high-r9/SP-002 — internal/server/profilesettings.go

**Claim (representative):** handleProfileTest diverged from the smine-profile-settings plan's D8/D9 (rewritten prompt and a styleTestTarget model override from ROUTINE_MODEL) without any Changelog entry recording the change.

**Prior verdicts:** A: confirmed MINOR — The plan's D9 sample prompt text ('Summarize for me in 3-4 sentences: last night 12 of my work sessions...') was rewritten in the shipped styleTestPrompt into a longer 'this is a style check, not a task...' variant (justified in-code by an observed error_max_turns failure), and a styleTestTarget ROUTINE_MODEL override was added — neither appears in D8/D9 nor in the plan's Changelog, which records only unrelated adjustments. || B: claim-false — This is a plan-traceability/changelog-recording observation about an already-working, deliberately-chosen mechanism (the code comment documents the technical reason for the rewritten prompt: an earlier task-phrased prompt caused the model to investigate with tools and hit --max-turns 1), not a functional code defect — matching this review's own established pattern (falsePositives entries correctness-INF-1/-2/-1-l2) that plan-traceability requests for working code are process observations, not defects.

### FOR (prosecutor)

#### Case

The shipped `styleTestPrompt` text and the `styleTestTarget` ROUTINE_MODEL override both diverge from what D8/D9 specify, and neither appears in the plan's Changelog — which RULE-PLAN-049 makes mandatory for exactly this situation. This isn't a "can't locate the plan" complaint (the golden false-positive pattern); the plan is found, it is quoted verbatim, and it contradicts the code with a repo rule on point.

#### Key code

`verify-wt/internal/server/profilesettings.go:25-30`:
```
const styleTestPrompt = "This is a style check, not a task: do not use any tools or read any files — reply directly with prose only. Write me 3-4 sentences conveying exactly this: last night 12 of my work sessions were reviewed and 3 improvement suggestions came out."
```
vs. plan D9 (`plans/.../design/raw.md:77`): `"Summarize for me in 3–4 sentences: last night 12 of my work sessions were reviewed and 3 improvement suggestions came out."` — different prompt, plus a whole no-tools/no-files preamble D9 never mentions.

`profilesettings.go:176-183` adds a `styleTestTarget()` model-override mechanism (`ROUTINE_MODEL` env lookup) that D8 (`raw.md:76`, "mirrors `handleWelcomeVerifyToken`... via bash + routine token") never describes.

RULE-PLAN-049 (`context/rules/plan.md:107`): "Any post-approval plan edit appends a row. The plan body is updated in place; history lives only here." The plan's Changelog (`raw.md:505-511`) has exactly 3 rows (format fix, POST-redirect-GET/lock/CSS adjustments) — none reference the prompt rewrite or the model override.

#### Against the dissent

Verifier B: "not a functional code defect... matching this review's own established pattern (falsePositives entries correctness-INF-1/-2/-1-l2) that plan-traceability requests for working code are process observations."

Checked those three golden entries directly (`golden-v6.json`): all three were rejected because the claim could not locate *any* originating plan or rule ("no confirmed originating plan", "neither live plan... documents [it]"). That is not this claim. Here the plan exists, is cited by decision ID, is quoted, and a specific repo rule (RULE-PLAN-049) names the exact required remediation — a Changelog row — which is verifiably absent. The in-code comment explaining *why* the prompt changed is a justification for the deviation, not a substitute for the row RULE-PLAN-049 requires; the rule has no "documented in a comment" exception. Verifier B's cited precedent doesn't transfer.

#### Severity if real

MINOR — process/traceability gap on working code, not a runtime defect; same class as GV3-153 per Verifier A.

#### Confidence

high — the rule text, the quoted plan decision, and the quoted Changelog are all directly on-record and unambiguous; the only real question (is RULE-PLAN-049 in scope) is settled by its being an active repo rule, not a stylistic preference.

### AGAINST (refuter)

#### Case

Both the plan doc and profilesettings.go are new files introduced in the same reviewed range (b4981b88..a342811b) — there is no prior merged plan being violated after the fact. The shipped prompt text and model-override are working, deliberately-chosen mechanisms with in-code rationale, not silent contradictions. RULE-PLAN-049 governs edits to the plan *document* body, not implementation details diverging from plan prose — so the cited rule doesn't even reach this claim. This is a plan-traceability observation about functioning code, matching this review's own established false-positive pattern.

#### Key code

`internal/server/profilesettings.go:25-30` (new in range, confirmed via `git diff b4981b88..a342811b`):
```
// styleTestPrompt is the fixed sample the test button sends: ... It must read as a writing sample, never a task:
// phrased as a task, the model started investigating the repo with tools and
// died on --max-turns 1 (observed error_max_turns).
const styleTestPrompt = "This is a style check, not a task: do not use any tools or read any files — reply directly with prose only. Write me 3-4 sentences conveying exactly this: last night 12 of my work sessions were reviewed and 3 improvement suggestions came out."
```
`internal/server/profilesettings.go:176-179`:
```
// styleTestTarget resolves what the test call runs with: the smine-nightly
// routine's ROUTINE_MODEL / ROUTINE_TOKEN when configured, else the routine
// default model and the token-verify pick order. Without the explicit model,
// headless claude falls back to the settings.json model, which the routine
// token's account may have no credits for (observed 429).
```
Plan D9 (`plans/.../design/raw.md:76`) gives only the sample-content intent ("Summarize for me..."); D8 (`raw.md:75`) says nothing about model selection either way.

#### Against the confirmation

VerifierA's reason: "neither appears in D8/D9 nor in the plan's Changelog, which records only unrelated adjustments." True as a documentation-completeness fact, but it doesn't establish a code defect — the code itself documents *why* each divergence exists (max-turns tool-investigation failure; 429 credit failure), i.e. these are bug-driven implementation refinements of an approved feature, not undocumented architectural contradictions. RULE-PLAN-049 ("Any post-approval plan edit appends a row") applies when the *plan document* is edited post-approval — it says nothing about implementation prose deviating from planned sample text. Unlike GV3-153 (cited as calibration), which was a hook file *directly contradicting* an explicit "Not changed: stays as-is" plan line, here the plan makes no "stays exactly as this text" commitment — D9 is illustrative sample content, and D8 doesn't enumerate every resolution detail.

#### Verdict if I prevail

claim-false (matches the golden false-positive pattern of correctness-INF-1/-2/-1-l2: plan-traceability request against admittedly-working, in-code-rationalized code, not a functional defect)

#### Confidence

medium — the "no changelog entry" fact is accurate and RULE-PLAN-049's literal text is debatable at the margins, but the golden precedent for this exact claim shape (traceability nit on working code) is strong and directly on point.

---

## g33 — repos.go: Add form user-editable name overrides Repo.Name vs plan C16/G17

**Instances:** sonnet-5-high-r18/SP-003 — internal/server/repos.go

**Claim (representative):** The repo Add form adds a user-editable name input that overrides Repo.Name, deviating from the language-style-setting plan's C16/G17 design that identity is derived from the folder and only Label is editable, with no plan changelog row.

**Prior verdicts:** A: confirmed MAJOR — Plan decisions C16 ('Repo gains optional Label ... Name stays the key' — a display field must not touch identity) and G17 ('add handler builds Repo{Name: filepath.Base(path), Path: path}') explicitly forbid a user-editable Name, but the shipped form has a real `name` input (repos_index.html, placeholder 'name (default: folder name)') and handleReposAdd uses `name := r.FormValue("name"); if name == "" { name = filepath.Base(path) }` — Name is user-overridable, contradicting the binding identity invariant the plan states. || B: claim-false — This is a plan-traceability observation about a working, form-accepted name field (still gated by repo.Validate()'s namePattern/reserved-name checks) — matches this review's own established pattern that 'deviates from plan without a changelog row' claims against otherwise-functioning code are provenance/process observations, not code defects.

### FOR (prosecutor)

#### Case

The shipped add form and handler let a user override `Repo.Name` directly, but the plan's own literal spec for this exact form change (C16) shows only a `label` input being added — no `name` input appears anywhere in the plan text. This is an unapproved, unauthorized addition to the identity-bearing field the plan explicitly locked down, with no changelog row anywhere in the plan documenting it.

#### Key code

`internal/server/repos.go:561-568`:
```
path = filepath.Clean(path)
name := strings.TrimSpace(r.FormValue("name"))
if name == "" {
	name = filepath.Base(path)
}
...
repo := repos.Repo{Label: label, Name: name, Path: path}
```
`internal/server/templates/repos_index.html:8-9`:
```
<input type="text" name="name" maxlength="80" placeholder="{{t "name (default: folder name)"}}"
       title="registry name — also the mining folder under sessions/; edit to resolve a name collision">
```
Plan spec, `plans/v1-2-1/2026-08-24-language-style-setting-wip-ef09567/design/change-nondev-automation.md:263`, the literal add-form diff C16 authorizes:
```
- add form: `<input type="text" name="label" placeholder="{{t "label (optional)"}}">` next to the path input (both audiences).
```
No `name` input is in that diff. Line 128 (C16) states the invariant: "Name stays the key ... a display field must not touch identity."

#### Against the dissent

Verifier B says: "still gated by repo.Validate()'s namePattern/reserved-name checks... matches this review's own established pattern that 'deviates from plan without a changelog row' claims against otherwise-functioning code are provenance/process observations, not defects." This misses that the claim isn't about missing input validation — `Validate()` checking pattern/reserved-name says nothing about whether Name may be user-chosen at all. The plan's own spec (line 263) is the concrete artifact showing what was approved for this form: a label field only. The shipped diff (repos_index.html:8) adds a second, unspecified input that is wired directly into the identity field the plan calls load-bearing (G17: "the URL segment and dedup key"). This is not a vague "no changelog row" gripe over working code; it is a literal mismatch between the approved diff text and the shipped diff for the same line-item (C16), on the same field the plan explicitly protects.

#### Severity if real

MAJOR — same class as GV3-194 (unauthorized deviation from an approved spec on the same handler), touching an identity-bearing field rather than a cosmetic one.

#### Confidence

medium — the plan-text mismatch is textually clean (label-only in the diff vs. name+label shipped), but "Name stays the key" in C16 is stated as a rationale for the Label decision, not as an explicit standalone prohibition on ever making Name form-editable, so a defender could argue the invariant's scope is ambiguous.

### AGAINST (refuter)

#### Case

I could not build an honest refutation. Both `internal/server/repos.go` and `internal/server/templates/repos_index.html` are touched by this review's diff (b4981b88..a342811b), the shipped code adds a genuinely user-editable `name` field alongside `label`, and the plan explicitly scopes the add-form change to label only while keeping Name folder-derived.

#### Key code

`internal/server/repos.go:562-568`:
```
name := strings.TrimSpace(r.FormValue("name"))
if name == "" {
	name = filepath.Base(path)
}
...
repo := repos.Repo{Label: label, Name: name, Path: path}
```
`internal/server/templates/repos_index.html` (new in diff, was not present before a342811b):
```
<input type="text" name="name" maxlength="80" placeholder="{{t "name (default: folder name)"}}"
       title="registry name — also the mining folder under sessions/; edit to resolve a name collision">
```
Plan `plans/v1-2-1/2026-08-24-language-style-setting-wip-ef09567/design/change-nondev-automation.md:128` (C16): "`Repo` gains optional `Label string` (display-only ... never the URL segment — Name stays the key); **add form gains a label input** (both audiences)" — only a label input is sanctioned. `:155` (G17 baseline): "add handler builds `Repo{Name: filepath.Base(path), Path: path}`" — no changelog row (`:427-438`) revises this to add a name input.

#### Against the confirmation

VerifierA's evidence quotes match the code exactly; I confirm both citations verbatim. VerifierB's rebuttal — "'deviates from plan without a changelog row' claims ... are provenance/process observations, not code defects" — is a generic policy assertion with no repo-specific backing found; `repo.Validate()` gating the input only bounds *malformed* names, not whether Name should be user-settable at all, so it does not neutralize the identity-invariant argument C16 states ("a display field must not touch identity"). I checked for a later sanctioning decision (collision-resolution rationale in the shipped tooltip) — none exists in the plan's Changelog (`:427-438`), so the tooltip's justification is undocumented, not approved.

#### Verdict if I prevail

Cannot honestly claim claim-false / out-of-range / matches-golden. No golden entry in golden-v6.json addresses this file/line's name-field deviation (closest repos.go entries are about git-init ordering, RULE-GOLANG nesting/naming, and struct-field ordering — none about the Name/Label identity split). This claim should stand as confirmed.

#### Confidence

High — the code diff and plan text are unambiguous and I found no changelog entry sanctioning the name input.

---
