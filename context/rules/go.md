<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->

# GO CODE STYLE

**Files:** `*.go`

**For reviewers / agents:** cite the stable `RULE-*` id when flagging a violation. Fix violations
rather than debating them.

Every rule carries exactly one tag:

* `[lint]` — a CI linter proves it; the rule's first bullet names the linter(s) (`Covered by …`).
  **Do not check these in review**; their absence from a review is not a gap. They are documented
  for authors. In a repo without the named tooling the rule is checked like `[review]`.
* `[review]` — no tool covers it (or only a slice of it — the bullets say which). This is where
  review effort belongs.

Where a rule is only partly automatable it is **split**: the CI-proven part is its own `[lint]`
rule and the part a human still has to check is a sibling `[review]` rule cross-referencing it.
Read the tag, never the id, to know who checks a rule.

Repos with a golangci-lint gate (e.g. the go monorepo) also run general-purpose linters that
enforce Go hygiene this guide never documents — `errcheck`, `gosec`, `staticcheck`, `intrange`,
`modernize`, `misspell` and others. Their findings are the gate's business: do not re-derive them
in review, and do not expect a `RULE-*` id for them. Such gates report only findings on lines the
branch changed (new-from-merge-base), so a green build means the code you touched conforms — not
that the whole file does.

**Scope:** all Go style. Test rules live in the **TESTS** section, concurrency rules in
**GOROUTINES** (both below); SQL style is `sql.md` next to this file — all equally mandatory.

---

## BASELINE

All code MUST follow official Go conventions:

* https://go.dev/doc/effective_go
* https://google.github.io/styleguide/go/

Where this document conflicts with Go conventions, Go conventions win — **EXCEPT** acronym /
initialism casing, which follows **RULE-GOLANG-NAME-002** (a deliberate, documented local override of the
Go "initialisms" convention).

---

## NAMING

**RULE-GOLANG-NAME-005** `[lint]` — No interior underscores or `ALL_CAPS` in identifiers; parameters are never capitalised.

* Covered by `revive/var-naming` and `gocritic/captLocal` — types, fields, consts, vars, funcs,
  methods, parameters and locals are all checked.
* Exceptions (Go / tooling conventions): Go test function names (`TestType_Method`, `Test_helper`)
  and test-case meta fields (e.g. `_id`) — see the TESTS section.
* Exception, `*_test_data.go` only: the fixture option builders name each parameter after the field
  it sets, so `func (opt *_GameVoteTestOpt) Id(Id string)` assigning `vote.Id = Id` is sanctioned
  (excluded from `captLocal` in the linter config). This is deliberate and is NOT a review item.
  Note the cost: where the field's type shares its name, the parameter shadows the type inside the
  function body — so a builder needing to *name* that type must rename its parameter.

**RULE-GOLANG-NAME-001** `[review]` — Exported identifiers use PascalCase, unexported camelCase; the start-case slice the linters cannot see is review-owned.

* Exported identifiers MUST use PascalCase (`GameVote`); unexported MUST use camelCase (`gameVote`).
  revive only rejects underscores and ALL_CAPS (RULE-GOLANG-NAME-005), so an exported identifier wrongly
  starting lower-case, or an unexported one starting upper-case, is a review check.
* revive exempts **any** identifier beginning with `_`, anywhere — not just in tests. So `_id`,
  `_secret` or `func _helper()` in production code pass CI, and only the TESTS-section uses of that
  prefix are legitimate.
* revive's ALL_CAPS check keys on the underscore, so a shouting name without one — `MAXRETRIES` —
  passes CI and is a review check.

**RULE-GOLANG-NAME-002** `[review]` — Acronyms and initialisms are cased as ordinary words (`Otp`, `HttpClient`) with one spelling per concept repo-wide; identifiers we do not own keep their upstream spelling.

* Identifier casing SUPERSEDES the conventional casing of an acronym / initialism.
  Treat every acronym (OTP, ID, TTL, SES, SQS, UUID, URL, URI, API, HTTP, HTML, JSON, DB, …) as an ordinary word:
  * PascalCase symbols (exported types, funcs, methods, fields, consts): capitalise only the first letter → `Otp`, `Id`, `Ttl`, `Uuid`, `Url`, `Json`, `Db`, `SesSender`, `HttpClient`
  * camelCase symbols (unexported vars, params, fields): a leading acronym is fully lower-case; a non-leading acronym takes the PascalCase form → `otpHash`, `userId`, `otpTtl`, `deviceUuid`, `sqsBounceQueueUrl`
  * Holds no matter how many acronyms collide → `LoginIntentOtpTtl` (NOT `LoginIntentOTPTTL`), `SesAccessKeyId` (NOT `SESAccessKeyID`)
* EXACTLY ONE spelling per concept, repo-wide (this REPLACES the old "consistency within a file")
* SOLE EXCEPTION — identifiers we do not own: interface methods, fields, and functions from the stdlib / third-party / generated code MUST match their upstream spelling exactly
  * e.g. `ServeHTTP` (implements `http.Handler`); reading `r.RequestURI` is upstream, not ours
  * struct tags / wire formats are strings, not identifiers → follow the external contract (`json:"otp_hash"`), unaffected
* This is a deliberate LOCAL OVERRIDE of the Go "initialisms" convention (Go Code Review Comments / Google Go Style Guide), chosen because it is mechanical (needs no initialism dictionary)

**RULE-GOLANG-NAME-006** `[lint]` — A 1–2 character local or parameter spanning more than ~5 lines is a violation; sanctioned short names live in the linter config.

* Covered by `varnamelen`. Sanctioned short names are configured in the repo's `.golangci.yml`:
  well-known acronyms (`id`, `db`, `ip`), `err`, `ok`, `tx`, `wg`, the conventional loop variables
  `i j k v n`, and the HTTP handler parameters `w`/`r`.

**RULE-GOLANG-NAME-003** `[review]` — Names are verbose and descriptive; abbreviations only for well-known acronyms and the sanctioned short forms.

* Names MUST be verbose and descriptive. varnamelen (RULE-GOLANG-NAME-006) only sees 1–2-character names
  in long scopes, so `ras`, `iId` and any short **struct field** pass CI and are a review check.
* Abbreviations are allowed ONLY for: well-known acronyms (cased per RULE-GOLANG-NAME-002), single-letter
  method receivers, the error variable `err`, `ok`, `tx`, `wg`, the conventional loop variables
  (`i`, `j`, `k`, `v`, `n`), and the idiomatic HTTP handler parameters
  `w http.ResponseWriter, r *http.Request`.
* Shadowing is NOT a reason to abbreviate. When a name would otherwise be shortened only to dodge
  shadowing an outer variable, keep the full descriptive name and shadow deliberately — never
  introduce `j`/`ce`/`tmp` to sidestep the shadow.

```go
// GOOD
cooldownErr
retryAfterSeconds
intentId

// BAD
ce
ras
iId
```

**RULE-GOLANG-NAME-004** `[review]` — A method name never encodes an or/and branching choice (`FindOrCreate`); split the method or name the single outcome.

* A method name MUST NOT encode an "or" / "and" branching choice (`ResolveXOrY`, `FindOrCreate`,
  `GetXAndY`). This is a code smell: the name hides two distinct behaviors behind one signature,
  forcing the caller to know which branch fires. Split into separate, single-purpose methods
  instead — the caller decides which one to call.

```go
// BAD: name bakes an "or" branch into one method
func resolveSessionOrLatest(s *session.Store, request mcp.CallToolRequest, agent session.Agent) (*session.Session, error) {
    args := request.GetArguments()
    id, _ := args["id"].(string)
    title, _ := args["title"].(string)
    if id != "" || title != "" {
        return resolveSession(s, request)
    }

    sess, ok := s.Last(agent)
    if !ok {
        return nil, errors.New("resolveSessionOrLatest: No sessions found")
    }
    return sess, nil
}

// GOOD: two single-purpose methods, caller picks
func resolveSession(s *session.Store, request mcp.CallToolRequest) (*session.Session, error) {
    ...
}

func resolveSessionLatest(s *session.Store, agent session.Agent) (*session.Session, error) {
    sess, ok := s.Last(agent)
    if !ok {
        return nil, errors.New("resolveSessionLatest: No sessions found")
    }
    return sess, nil
}
```

**RULE-GOLANG-BOOL-001** `[review]` — Booleans read as predicates: `isX`, `hasX`, `shouldX`.

* Booleans MUST read as predicates: `isX`, `hasX`, `shouldX` (e.g. `isActive`, `hasAccess`, `shouldRetry`).

**RULE-GOLANG-BOOL-002** `[review]` — Boolean configuration and feature fields on models are named `Is<X>Active`.

* Boolean configuration/feature fields on models are named `Is<X>Active` (e.g. `IsTestModeActive`),
  not bare adjectives (`TestMode`, `Enabled`).

**RULE-GOLANG-RECEIVER-001** `[review]` — Method receivers are short and consistent within a type (`s` service, `c` config).

* Method receivers MUST be short and consistent within a type: `s` (service), `c` (config),
  `r` (request), etc. The same receiver name MUST be used across all methods of a type.
* Covered by `revive/receiver-naming` for consistency **within one file** only. Receiver length and
  consistency across the type's other files are review checks — revive compares only within a file,
  so `func (configurationObject *Config)` passes CI.

**RULE-GOLANG-INTERFACE-001** `[review]` — Interface names carry no `I` prefix.

```go
type Service interface {}  // correct
type IService interface {} // forbidden
```

**RULE-GOLANG-PKG-001** `[review]` — Package names are lowercase, short, and meaningful, without underscores.

* lowercase, no underscores, short and meaningful.

---

## FILE & TYPE STRUCTURE

**RULE-GOLANG-FILE-001** `[review]` — A file orders a file-level preamble, then one self-contained block per type, then free functions.

A file is ordered as a file-level preamble, then one self-contained block per type, then free
functions:

1. Type aliases
2. Constants
3. Variables
4. Type blocks — one block per type (struct, interface, named type); order the blocks alphabetically
   by type name. Each block keeps the type together with everything defined on it, in this order:
   1. the `type` declaration
   2. its constructor(s) / factories — `New<Type>()` directly below the declaration
   3. its non-exported (private) methods, alphabetical
   4. its exported (public) methods, alphabetical
5. Package-level functions not bound to a type — non-exported first, then exported, each group alphabetical.

Do NOT collect all methods of the file into one global group; keep each type's methods in its own
block. Example: `type A`, `NewA`, A's methods, then `type B`, `NewB`, B's methods, then free functions.

* Covered by `funcorder` for one slice only. It checks that `New<Type>()` appears after the type
  declaration and before its methods; it permits other declarations between the type and its
  constructor ("**directly** below" is review), recognises only `New`-prefixed constructors (a
  `Provide…` or `Make…` factory is unchecked), and its exported-before-unexported check is
  deliberately off — it wants the opposite order to this rule.

**RULE-GOLANG-FILE-002** `[review]` — Files split by domain concern — one service/handler file per concern; a godfile accreting unrelated handlers is a defect.

* Split code across files by domain concern: one service/handler file per concern (`service_<concern>.go`), never a single file accreting many unrelated handlers.
* View/DTO structs live in their own `viewmodels.go` — they are a first-class file, not inline scaffolding.
* A file mixing unrelated concerns is a defect regardless of green tests; size (e.g. a 1000-line file) is the symptom, domain-mixing is the cause.
* No inline struct types declared inside handlers — extends RULE-GOLANG-STRUCT-004's package-level-types clause to the file-organization axis. RULE-GOLANG-FILE-001 covers ordering *within* a file; this covers how to split *across* files.

**RULE-GOLANG-STRUCT-001** `[review]` — Unexported fields first, exactly one blank line, then exported fields.

* Unexported (private) fields first, then EXACTLY one blank line, then exported (public) fields.

**RULE-GOLANG-STRUCT-002** `[review]` — Fields sort alphabetically within each group; `Id` is always the first public field.

* Sort fields alphabetically within each group. Exception: `Id` is always the first public field.

**RULE-GOLANG-STRUCT-003** `[review]` — Struct fields are concrete types — an interface-typed field is a design decision that needs approval.

* Struct fields MUST be concrete types. An interface-typed field is a design decision that needs
  explicit approval — stop and ask before introducing one.
* Interfaces belong at function boundaries (parameters), not in storage. Model reader/writer
  capabilities with the stdlib `io.Reader` / `io.Writer` / `io.ReadWriter` split at the signature
  instead of storing an abstract dependency.
* Controllers/services are created ad-hoc where needed — never stored as struct fields.

**RULE-GOLANG-STRUCT-004** `[review]` — Related structs share fields via embedding, never by copying.

* Embed instead of copying fields between related structs.
* Types are declared flat at package level — no type declarations inside functions.

**RULE-GOLANG-STRUCT-005** `[review]` — Orchestration lives on the type that owns the unexported helpers it calls — a flow is never split across types.

* Orchestration lives on the type that owns the unexported helpers it calls — never split a flow
  across a coordinator type and a helper type.
* Domain logic lives in the domain package, not in transport/CLI layers.

```go
// BAD: flow split across a coordinator and a helper type — SyncRunner owns the
// steps but ImportParser owns half the logic they call
type ImportParser struct{ /* ... */ }

func (p *ImportParser) parseRows(raw []byte) ([]Row, error) { /* ... */ }

type SyncRunner struct{ parser *ImportParser }

func (r *SyncRunner) Run(ctx context.Context) error {
	rows, err := r.parser.parseRows(r.raw)
	// ...
}

// GOOD: the orchestrating type owns the unexported helpers it calls
type SyncRunner struct{ /* ... */ }

func (r *SyncRunner) Run(ctx context.Context) error {
	rows, err := r.parseRows(r.raw)
	// ...
}

func (r *SyncRunner) parseRows(raw []byte) ([]Row, error) { /* ... */ }
```

**RULE-GOLANG-STRUCT-006** `[review]` — No anonymous struct types in production code.

* Anonymous (inline) struct types are banned in production code: no anonymous
  struct literals as function arguments or template data, and never a loop over
  a slice of anonymous structs — unroll into named-helper calls instead.
* Any proposed use of an anonymous struct is a hot item: it goes into the plan
  for explicit approval (ACTION-CONCEPT-HOT-006).

**RULE-GOLANG-STRUCT-007** `[review]` — A closed set of two or three implementations uses concrete types selected by a `switch`, not interface polymorphism.

* With exactly two or three known implementations, prefer concrete types selected by a `switch`
  over an interface abstraction. Reach for an interface only when the set is open or a real seam is
  needed (a test double, a plugin boundary). Complements RULE-GOLANG-STRUCT-003 — an interface introduced
  purely to dispatch over a fixed, closed set is unneeded indirection.

**RULE-GOLANG-TYPE-001** `[review]` — Never `json.RawMessage`, `any`, or `map[string]string` where a typed model exists or can be declared.

* Never `json.RawMessage`, `any`, or `map[string]string` where a typed model exists or can be declared.
* Never duplicate an existing generic model as a local struct — reuse the model.
* Before writing a helper (JSON response, HTTP client, similarity function, extraction path), grep
  for an existing one in the repo and use it. Reimplementing existing infrastructure is
  review-flaggable duplication, not a stylistic preference.

---

## FUNCTIONS, POINTERS & CONTEXT

**RULE-GOLANG-FUNC-001** `[review]` — `ctx context.Context` is the first parameter and is named `ctx`, error is the last return value, at most 3 return values.

* `ctx context.Context` MUST be the first parameter (named `ctx`).
* `error` MUST be the last return value.
* No more than 3 return values including `error`; 3 (2 values + error) is the exception, not the norm.
* Covered by `revive/context-as-argument` (position), `staticcheck ST1008` (error last) and
  `revive/function-result-limit` (≤3 returns). The parameter *name* `ctx` and "3 is the exception"
  are review checks — revive checks only the position, so `func F(c context.Context, …)` passes CI.

**RULE-GOLANG-FUNC-002** `[review]` — A function whose body is a single call to another function is inlined — a wrapper adds a name, not behavior.

* A function whose body is a single call to another function adds a name, not behavior — inline it
  at the call site. Wrappers survive only with ≥2 callers AND added logic (defaulting, adaptation).
* Exemption: transport-level response helpers (e.g. a `respond` package wrapping
  `http.Error` per status code) are sanctioned despite one-line bodies once
  they have 3+ callers — the name carries the status-code contract.

**RULE-GOLANG-FUNC-003** `[review]` — A contract change alters the signature and updates every caller — never a parallel variant function.

* When a change needs a new signature, change the signature and update every caller. Do NOT mint a
  `*V2` / `*Raw` / `*WithX` sibling to spare the call sites — that leaves two parallel functions
  where one belongs (violates single-source-of-truth). RULE-GOLANG-FUNC-002 bans one-line wrappers; this
  extends the ban to logic-carrying parallel variants.
* A signature change enumerates every call site at plan time; verify the sweep by build, not by one
  literal grep.

**RULE-GOLANG-FUNC-004** `[review]` — Create/update handlers take the complete desired state and overwrite it wholesale — no partial-diff builders.

* A create/update handler takes the complete desired state and overwrites it wholesale. Do NOT build
  partial-patch logic that nil-checks each field (`if form.X != nil { model.X = *form.X }`) —
  `*bool` / `*string` patch fields are the smell. Ties to RULE-GOLANG-POINTER-002 (express absence with the
  zero value, not a pointer).

**RULE-GOLANG-POINTER-001** `[review]` — Structs pass by pointer; by value only for small immutable value types.

* Structs MUST be passed by pointer. Passing by value is allowed only for small immutable value
  objects or when explicit copy semantics are required. If in doubt, use a pointer.

**RULE-GOLANG-POINTER-002** `[review]` — Absence is expressed with the type's zero value where unambiguous; pointer fields only where nil is meaningful.

* Express absence with the type's zero value where the zero value is unambiguous. Pointer fields
  only when nil-ness carries meaning the zero value cannot (unset vs. explicitly zero) or the
  persistence layer requires it.

**RULE-GOLANG-CTX-001** `[review]` — `context.Context` passes through the call chain and is never stored in long-lived objects.

* `context.Context` MUST be passed through the call chain and MUST NOT be stored in long-lived
  structs (services, clients, repositories).
* Exception: it MAY be stored in short-lived, request-scoped structs (forms, request models) that do
  not outlive the request lifecycle.

**RULE-GOLANG-CTX-002** `[review]` — A struct that stores `context.Context` is request-scoped and never reused across requests.

* If `context.Context` is stored, the struct MUST be request-scoped and MUST NOT be reused across
  requests.

---

## CONTROL FLOW

**RULE-GOLANG-NEST-001** `[review]` — A function body nests at most 4 levels of control flow; new code is written to 2.

* Covered by `revive/max-control-nesting` at the 4-level bound — revive counts `if`, `switch`,
  `select` and three-clause `for`. 4 is where the limit starts rather than where it stays: it
  absorbs legacy code and tightens as those files are worked on. Write new code to 2.
* The same limit counts `for … range`, which revive does not count at all — `range → range → range
  → if` reports nothing. Any nesting built from range loops passes CI and is a review check.
* Deeper logic is extracted into a named helper per level ("never-nester").
* Early returns are the primary de-nesting tool (extends RULE-GOLANG-ERR-002).

**RULE-GOLANG-COND-001** `[review]` — An `if` condition holds at most one binary boolean operator, and an `else if` chain is rewritten as a `switch`.

* An `if` condition holds at most one binary boolean operator; never two `&&`. No linter counts
  boolean operators in a condition. Compound logic is assigned to named predicate variables
  (`matchesEnabledRow := !hasEnabled || enabled`) or extracted into a helper.
* An `if x { … } else if y { … } else { … }` chain is rewritten as a `switch`.
* Covered by `gocritic/ifElseChain` and `staticcheck QF1003` for the chain shape. gocritic only
  fires from **two** branches upward (QF1003 reports the same shape where the chain compares one
  value), so a single `if … else` is unchecked; prefer early returns there.
* A chain over two data sources is restructured into a direction-resolving helper, which is a
  judgement a switch conversion does not make for you.

---

## CALLS & LITERALS

**RULE-GOLANG-CALL-001** `[review]` — A call goes multiline at 5 or more arguments or any nested composite argument.

A call MUST be multiline if ANY of the following holds: 5 or more arguments; any argument is a nested
call; single-line readability suffers.

**RULE-GOLANG-CALL-002** `[review]` — Multiline calls follow the canonical format — one argument per line with trailing commas.

```go
FunctionName(
    param1,
    param2,
    param3,
)
```

* One argument per line; trailing comma required; closing parenthesis on its own line.
* gofmt does NOT decide whether a call is multiline (that is RULE-GOLANG-CALL-001) and does NOT split
  arguments one-per-line — it only normalizes indentation and the trailing comma once the call is
  already broken across lines. Treat the layout itself as a manual check.

**RULE-GOLANG-CALL-003** `[review]` — Named fields in struct literals sort alphabetically; `Id` is always first.

* Named fields in struct literals MUST be sorted alphabetically. Exception: `Id` is always first.
* Parameters in a function signature MUST be sorted alphabetically. Exceptions: `ctx` is always
  first, and a variadic / functional-options parameter (`opts ...Option`) is always last — Go
  requires the variadic parameter last, so it overrides alphabetical order.
* Not enforced by gofmt — verify in review.

**RULE-GOLANG-CALL-004** `[review]` — Chunked slice consumption advances the cursor by `len(chunk)`, never by the nominal chunk size.

* When consuming a slice in chunks, advance the cursor by `len(chunk)` — never by the nominal chunk
  size constant, which drops the short final chunk or double-reads on partial consumption.

**RULE-GOLANG-CALL-005** `[review]` — No composite literal inline as a call argument — bind it to a named variable first.

* A composite literal (struct / slice / map literal) MUST NOT appear inline as
  a function-call argument. Assign it to a descriptively named variable first,
  then pass the variable (mirror of RULE-GOLANG-RETURN-001).

---

## RETURNS

**RULE-GOLANG-RETURN-001** `[review]` — A `return` that yields more than one value never inlines a composite literal.

* A `return` that yields more than one value MUST NOT inline a composite literal (struct / slice /
  map / array literal). Assign the literal to a descriptively named variable first, then return the
  variable, so the trailing values (`nil` / `err` / …) are not lost behind a multi-line literal.
* A single-value return of a literal (`return &Foo{...}`) is fine — there is no trailing value to obscure.

```go
// BAD: the `, nil` hides behind the literal
func NewSesSender(...) (*SesSender, error) {
    return &SesSender{
        client:      sesv2.New(awsSession),
        fromAddress: fromAddress,
    }, nil
}

// GOOD: literal named, return is scannable
func NewSesSender(...) (*SesSender, error) {
    sender := &SesSender{
        client:      sesv2.New(awsSession),
        fromAddress: fromAddress,
    }
    return sender, nil
}
```

---

## ERROR HANDLING

**RULE-GOLANG-ERR-005** `[lint]` — Out-of-module errors are wrapped before returning; `fmt.Errorf` wraps with `%w`; errors are compared with `errors.Is` / `errors.As`.

* Covered by `wrapcheck` and `errorlint`.
* An error from a package **outside this module** MUST be wrapped before it is returned —
  `wrapcheck`, scoped in the linter config to non-first-party packages precisely because the
  pass-through allowance in RULE-GOLANG-ERR-001 is not something it can detect.
* A wrapping `fmt.Errorf` MUST carry the error with `%w`, not `%v` or `%s`, and errors are compared
  with `errors.Is` / `errors.As` rather than `==` or a type assertion — `errorlint`.

**RULE-GOLANG-ERR-001** `[review]` — An error returned to a caller is wrapped with context added.

* An error returned to a caller MUST be wrapped with `errors.Wrap` / `errors.Wrapf`, adding a
  `Component.Method: context` message (see RULE-GOLANG-LOG-001 for the prefix shape). New errors use
  `errors.New` / `errors.Errorf` with the same prefix. **No linter expresses this prefix**, which
  makes it the loudest genuinely review-owned rule in the guide.
* An error from a **first-party** package returned unwrapped is invisible to CI (RULE-GOLANG-ERR-005's
  scoping) — a review check.
* Bare `return err` is allowed ONLY as a direct pass-through where the callee's error is already
  fully contextualized and this function has nothing to add — do NOT double-wrap.
* Error message strings MUST NOT end with a period.

**RULE-GOLANG-ERR-002** `[review]` — Errors are handled with early returns, never unnecessary nesting after a check.

* Handle errors with early returns; do NOT nest the success path in an `else`.
* Covered by `revive/early-return` and `revive/indent-error-flow` for the else-nesting shapes.
* Insert a blank line after an `if err != nil { … }` block when code follows. No blank line is
  required before a guard-clause `return`.
* The blank-line requirement applies after EVERY early-return guard block
  (`http.Error` + `return`, `continue` guards), not only `if err != nil`. `wsl_v5` proves some of
  these shapes where it runs; the rest stays a review check.

```go
if err != nil {
    return errors.Wrap(err, "Service.Method: Failed to load user")
}
```

  from the operation that statement exists to serve. **The disable list in `.golangci.yml` is the
  authority on what CI cannot see here**; this rule deliberately does not restate it, because such
  an enumeration goes stale against the config and then misdirects review twice over.
* Blank-line placement in a shape one of those checks owns is therefore a review check.

**RULE-GOLANG-ERR-003** `[review]` — The error variable is named `err`.

* The error variable MUST be named `err` (not `e`, `error`, `err1`).
* In transaction closures, shadow the outer `err` — do NOT introduce `txErr` or similar.

**RULE-GOLANG-ERR-004** `[review]` — Constant messages use the non-formatting constructor; formatted messages the `f` variant.

* A constant message (no format directives) MUST use the non-formatting constructor: `errors.New`
  (new error) or `errors.Wrap` (wrapping). The formatting variants `errors.Errorf` / `errors.Wrapf`
  MUST be used ONLY when the message interpolates a value (i.e. the format string contains at least
  one verb such as `%s` / `%d`).
* Covered by `perfsprint`, for one half only. perfsprint matches `fmt.Errorf` with a constant
  message; **CI cannot see the `pkg/errors` half** — it reports nothing for `errors.Errorf` or
  `errors.Wrapf`, so that half is a live review check, not a formality.
* `errors.Wrap` / `errors.Wrapf` on a `nil` error return `nil` — never use them to *synthesize* a
  new error from a `nil` input. Creating a new error uses `errors.New` / `errors.Errorf`.

```go
// BAD: formatting variant with a constant string
return errors.Errorf("Model.Validate: Missing field Foo")
return errors.Wrapf(err, "Service.Method: Failed to load user")

// GOOD
return errors.New("Model.Validate: Missing field Foo")
return errors.Wrap(err, "Service.Method: Failed to load user")

// GOOD: formatting variant justified — the message interpolates a value
return errors.Errorf("Model.Validate: Invalid field Foo: %s", c.Foo)
return errors.Wrapf(err, "Service.Method: Failed to load user %s", userId)
```

---

## LOGGING

**RULE-GOLANG-LOG-001** `[review]` — Log messages follow `<Receiver>.<Method>: <Context>: <Message>`.

Log messages MUST follow: `<Receiver>.<Method>: <Context>: <Message>`. For a package-level function
(no receiver) the prefix is just the function name: `<Function>: <Context>: <Message>`.

The `<Message>` MUST begin with a capital letter (`Failed to load user`, `Missing field Foo`,
`Unsupported hash format`). This applies equally to error strings (RULE-GOLANG-ERR-001).

```go
log.Infof("RecordingService.GetLiveTvRecordingById: Recording %s: Fetching recording", recording.Id)
log.Errorf("IngestLiveTvRequestEventPubSubTask.Run: Event %s: Failed to save", event.Id)
// package-level function — function name only, no receiver
return errors.New("VerifyPassword: Unsupported hash format")
```

**RULE-GOLANG-LOG-002** `[review]` — Every log line names the component (`Receiver.Method`) and the relevant identifiers.

* Always include the component (`Receiver.Method`) and relevant identifiers (ids, request, …).
* Messages MUST be concise and descriptive.

---

## VALIDATION

**RULE-GOLANG-VALIDATE-001** `[review]` — Models implement the validation contract (`Validate`).

Models SHOULD implement:

```go
type Model interface {
    HasAutoIncrementId() bool
    Validate(skipId bool) error
}
```

**RULE-GOLANG-VALIDATE-002** `[review]` — Every `Validate()` groups its checks under per-field comments.

* Applies to EVERY `Validate()` func — model `Validate(skipId bool)`, API form
  `Validate(ctx, r, serverContext)`, and worker task validation alike.
* Each validated field MUST have a comment directly above its check, and the comment MUST name the
  value being validated (this is the sanctioned exception to RULE-GOLANG-COMMENT-001):
  * struct / payload fields → the Go field name (`// Email`, `// Otp`, `// DeviceName`)
  * request parameters → their source name: the header (`// CF-Device-ID`, `// X-Forwarded-For`,
    `// Origin`) or the path parameter (`// id`)

```go
// GameId
if c.GameId == "" {
    return errors.New("Game.Validate: Missing field GameId")
}

// CF-Device-ID
f.deviceUuid = r.Header.Get("CF-Device-ID")
if f.deviceUuid == "" {
    return errors.New("CreateBigScreenLoginIntentForm.Validate: Missing header CF-Device-ID")
}
```

**RULE-GOLANG-VALIDATE-003** `[review]` — Missing optional values never fail validation, but present values still meet their constraints.

* Missing optional values MUST NOT cause validation errors, BUT value constraints MUST still be
  checked when a value is present.

```go
// OptionalField
if c.OptionalField != "" && len(c.OptionalField) < 3 {
    return errors.New("Model.Validate: Invalid field OptionalField")
}
```

**RULE-GOLANG-VALIDATE-004** `[review]` — API form `Validate()` methods follow the canonical processing steps and order.

Applies to API form `Validate()` methods, which read whole requests rather than a single struct.

* Every non-trivial processing step — loading a related entity, trimming / normalizing input,
  hashing, or deriving a value — MUST have a short comment above it describing what it achieves
  (`// Resolve the calling app from its client id`, `// Load the referenced login intent`,
  `// Reject blacklisted email addresses`). One line only (this is covered by RULE-GOLANG-COMMENT-002).
* Steps SHOULD appear in this order so every form reads top-to-bottom like the same story:
  1. `nil` receiver guard
  2. authenticate / resolve the calling app
  3. request-context headers (device id, client ip, origin)
  4. path parameters
  5. read the JSON body (`ReadJSONForm`)
  6. validate the payload fields
  7. fetch / derive dependent models

```go
func (f *CreateLoginIntentForm) Validate(ctx context.Context, r *http.Request, serverContext *cfapi.Context) error {
    if f == nil {
        return errors.New("CreateLoginIntentForm.Validate: Called on nil")
    }

    // Resolve the calling app from its client id
    appId, err := validateAppByClientId(r, serverContext)
    if err != nil {
        return errors.Wrap(err, "CreateLoginIntentForm.Validate")
    }

    f.appId = appId

    // X-Forwarded-For
    f.clientIp = cfapiutil.ClientIpForRequest(r, "X-Forwarded-For")
    if f.clientIp == "" {
        return errors.New("CreateLoginIntentForm.Validate: Missing header X-Forwarded-For")
    }

    // Read the request payload
    if err := cfrequest.ReadJSONForm(r, f); err != nil {
        return errors.Wrap(err, "CreateLoginIntentForm.Validate: Failed to read JSON")
    }

    // Email
    f.Email = strings.ToLower(strings.TrimSpace(f.Email))
    if f.Email == "" {
        return errors.New("CreateLoginIntentForm.Validate: Missing field Email")
    }

    // Reject blacklisted email addresses
    isBlacklisted, err := serverContext.Db.Controller().Accounts.IsEmailBlacklisted(f.Email)
    if err != nil {
        return errors.Wrap(err, "CreateLoginIntentForm.Validate: Failed to check blacklist")
    }

    if isBlacklisted {
        return errors.New("CreateLoginIntentForm.Validate: Email is blacklisted")
    }

    return nil
}
```

**RULE-GOLANG-VALIDATE-005** `[review]` — `Validate() error` belongs to models and forms; predicate checks use predicate names, never `Validate`.

* `Validate() error` belongs to models and forms implementing the validation contract. Predicate
  checks anywhere else are named `IsValid...() bool` — do not mint `Validate()` methods on
  non-model types.

---

## COMMENTS

**RULE-GOLANG-COMMENT-001** `[review]` — Comments only where intent is not obvious from names and code, or to record a decision.

* Avoid comments. Add one ONLY when intent is not obvious from names and code, or to record a
  non-obvious assumption the code cannot express.

```go
// BAD: restates what the code already says
// OtpSendClaim claims the send identified by sendCount by flipping the state to requires_action.
func (i *LoginIntent) OtpSendClaim(sendCount int) bool {

// GOOD: no comment — name and body carry the intent
func (i *LoginIntent) OtpSendClaim(sendCount int) bool {

// GOOD: documents a non-obvious assumption the code cannot express
// Django marks users without a usable password with the hash '!'.
func VerifyPassword(password, encoded string) bool {
```

**RULE-GOLANG-COMMENT-002** `[review]` — Grouping comments are allowed only in tests and `Validate` methods.

* Grouping / readability comments are allowed in tests and in `Validate` methods.

```go
// GOOD: grouping comments in a test
// first-send
test := &testCase{...}
tests = append(tests, test)

// resend
test = &testCase{...}
tests = append(tests, test)
```

---

## CONSTANTS

**RULE-GOLANG-CONST-001** `[review]` — Magic values are extracted into named constants (`const MaxRetries = 3`).

* Extract magic values into named constants: `const MaxRetries = 3`.
* Covered by `mnd` for numbers in **arguments, case clauses and conditions** and by `goconst` for a
  string repeated **5 or more times**. Both are scoped in the linter config, and the exemptions
  there are deliberate rather than review items (conventional numeric literals, fixture literals,
  strings in tests, identifier-shaped strings).
* The review remainder: numbers in an assignment, a return, or an operation are outside `mnd`'s
  enabled checks, and a string repeated two to four times is below `goconst`'s threshold — both are
  still violations when the value carries domain meaning.
* A named constant is also the wrong fix in one direction CI cannot judge: where the stdlib or a
  client library already names the value (`http.MethodGet`, `http.StatusInternalServerError`,
  `time.December`), use theirs rather than minting a local one.

---

## IMPORTS

**RULE-GOLANG-IMPORT-001** `[lint]` — Imports are formatted with `goimports`, internal packages in their own last group, one `import` declaration per file.

* Covered by `goimports` and `grouper`.
* Format imports with `goimports`; separate internal packages using `goimports -local` (the repo's
  module path as `local-prefixes` in the linter config where golangci runs).
* One `import` declaration per file — `grouper`. gofmt does not merge two separate import
  statements, so nothing else catches that.

---

## CLI (cobra)

**RULE-GOLANG-CLI-001** `[review]` — Subcommands are wired with `AddCommand` in the root/main setup, never registered via `init()`.

* Subcommands are wired with `AddCommand` in the root/main setup — never registered via `init()`
  side effects. At most one `init()` per package, and none for command registration.
* No package-level mutable flag variables — bind flags to fields on the command's own options struct.

---

## SECTIONS AND SIBLINGS (equally mandatory)

* Tests → **TESTS** section below (`RULE-GOLANG-TEST-*`, `RULE-GOLANG-ASSERT-*`)
* Goroutines → **GOROUTINES** section below (`RULE-GOLANG-GR-*`)
* SQL → `sql.md` next to this file (`RULE-SQL-*`, `RULE-SQL-DBMCP-*`)

---

## ENFORCEMENT

**Scope — what a rule applies to.** All rules are mandatory. Rules are introduced against a codebase
that already exists, so existing code will not satisfy a new rule. That is the normal state, not a
backlog.

* The enforcement unit is THE CODE YOU ADD OR MODIFY — the declaration or function you touch. Not the
  whole file, not the package, not the repository. A one-line fix inside a long legacy function does
  not pull the rest of it into conformance.
* NO drive-by conformance churn. Converting untouched code to a rule in a feature change hides the
  actual change and cuts against per-package logical commits. Conversion is its own deliberate commit.
* Pre-existing violations outside the change are NOT review findings. Before reporting one, check
  whether the pattern is pre-existing: if the rule has no other instances in the repository, treat the
  hit as a regression and flag it; if the codebase is full of them, it is legacy and out of scope.
* A differential CI gate (where one runs) reports only findings on lines the branch changed, which
  implements exactly this scope.

**Tooling.**

* Auto-format first: `gofmt` (indentation, trailing commas) and `goimports` (RULE-GOLANG-IMPORT-001);
  `go vet` MUST pass.
* In repos with a golangci-lint gate: run `make lint` before committing and fix what it reports — it
  is the same command CI runs. `make lint-autofix` applies every fix the linters can apply and
  re-reports what is left; `make fmt` formats the files your branch changed. The gate requires
  golangci-lint at the version the repo pins — another version disagrees with CI.
* Every `[review]` rule is a manual check; cite the `RULE-*` id when flagging a violation.
* Fix violations — do not merely note them.
* `go mod vendor` regenerates `vendor/` from the module cache and wipes manual edits to
  vendored dependencies — never hand-edit `vendor/`; fix upstream or vendor a fork.

**Suppressing a finding — `//nolint`.**

A `//nolint` says *the code is right and the linter is wrong here*. It is never a way to defer work.

```go
//nolint:gosec // G404: the jitter is not security-relevant, only spreads retries
delay := baseDelay + time.Duration(rand.Intn(500))*time.Millisecond
```

* Always name the linter (`//nolint:gosec`, never a bare `//nolint`) and always give the reason.
  `nolintlint` enforces both, so an unexplained or over-broad directive fails CI.
* State *why the code is correct*, not what the linter said. "G404: jitter is not
  security-relevant" is a reason; "false positive" and "linter is wrong" are not.
* The explanation MUST be on the directive line. A prose comment in the block *above* it is invisible
  to `nolintlint`, however good it is.
* Narrowest scope that works: the line, or the declaration — never a whole file, and never a whole
  package.
* If the same suppression is needed more than about three times for one reason, it is not a
  suppression, it is a **missing exclusion or a wrong rule**. Take it to the linter config with a
  comment, or amend the rule here.
* Never suppress to avoid a refactor the rule is asking for. Nesting, error wrapping and magic
  values are the code's problem, not the linter's; if the fix is out of scope for the change at
  hand, leave the finding for a deliberate commit rather than silencing it.

---

## QUICK CHECKLIST (review pass)

Only `[review]` rules (and the review remainders the bullets call out) belong here. Casing
underscores, receiver consistency within a file, short names in long scopes, constructor position,
`ctx`-first / `error`-last / ≤3 returns, `else if` chains, early-return else-nesting, unwrapped
external errors, `fmt.Errorf` with a constant message, magic numbers and repeated strings, and
import formatting are **CI's** in gated repos — do not check them there.

* [ ] Names: Pascal/camel correctness, acronyms per RULE-GOLANG-NAME-002 (`Otp`/`Id`/`Uuid`/`Db`/`Json`…), verbose and descriptive, no `_` prefix outside tests, no boolean operator in method names (RULE-GOLANG-NAME-004), booleans as predicates
* [ ] Receivers short, and consistent across the type's other files (RULE-GOLANG-RECEIVER-001)
* [ ] File/type order: per-type blocks (type → its `New` *directly* below → its methods), types alphabetical, private before public, free funcs last; struct fields private-then-public, alphabetical, `Id` first
* [ ] Signatures: context parameter *named* `ctx`; params alphabetical (RULE-GOLANG-CALL-003)
* [ ] Calls: multiline when required; struct-literal fields alphabetical (`Id` first)
* [ ] Returns: no composite literal in a multi-value return — name it, then return (RULE-GOLANG-RETURN-001)
* [ ] Errors: `Component.Method:` prefix present, no trailing period, named `err`, early return, tx shadowing, first-party errors wrapped; `errors.Errorf`/`Wrapf` only with a verb (RULE-GOLANG-ERR-004)
* [ ] Nesting: `range` loops count toward the limit too (RULE-GOLANG-NEST-001); one boolean operator per condition (RULE-GOLANG-COND-001)
* [ ] Logging: `Receiver.Method: Context: Message`, message starts capitalized
* [ ] Validation: field comments present; optional-field handling correct
* [ ] Structure: no interface-typed struct fields (RULE-GOLANG-STRUCT-003); concrete+switch over interface for a closed set (RULE-GOLANG-STRUCT-007); typed models over RawMessage/any/maps; embedding over field-copying; no one-line wrappers, alter signatures over `*V2`/`*Raw` siblings (RULE-GOLANG-FUNC-003); complete-object CRUD writes over partial-patch nil-checks (RULE-GOLANG-FUNC-004); zero values over pointers
* [ ] Comments minimal; a value the stdlib already names uses the stdlib constant (RULE-GOLANG-CONST-001)
* [ ] Sub-guides satisfied (tests, goroutines, SQL)

---

## TESTS

### SCOPE

This document defines required test patterns for Go tests in this repository.
It is the single source of truth for test case shape and execution style.

---

### BASELINE

All tests MUST use:

* `testing`
* `github.com/stretchr/testify/assert` for assertions
* `github.com/stretchr/testify/require` ONLY for test setup steps that MUST abort the test on failure (e.g. building requests, fixtures)

Keep tests short. Use table-driven tests for complex behavior or many combinations.

---

### TEST DATA

**RULE-GOLANG-TEST-011** `[review]` — Test fixtures come from `ProvideComplete<Model>` builders taking variadic options; variants are produced via options, never construct-then-mutate.

`ProvideComplete<Model>` MUST accept variadic options:

```go
func ProvideCompleteLoginIntent(opts ...func(*LoginIntent) *LoginIntent) *LoginIntent
```

Options are produced by a private `_<Model>Opt` builder returned from `<Model>Opts()`:

```go
type _LoginIntentOpt struct{}

func LoginIntentOpts() *_LoginIntentOpt { return &_LoginIntentOpt{} }

func (opt *_LoginIntentOpt) State(v LoginIntentState) func(*LoginIntent) *LoginIntent {
    return func(i *LoginIntent) *LoginIntent { i.State = v; return i }
}
```

* Declare `opt := <Model>Opts()` once per test function and reuse it across all test cases.
* Use `ProvideComplete<Model>(opt.Field(value))` to produce variants — do NOT construct and then mutate.
* Validation-style tables (one object in, pass/fail out) build each failing case by breaking exactly
  one field via an option (`ProvideComplete<Model>(opt.Name(""))`) — never hand-assemble a broken
  object. Every meta field is set explicitly, including zero values; the `_id` encodes the outcome
  with a `fail-`/`pass-` prefix (`"fail-empty-name"`, `"pass-minimal"`); assert with the single
  `assert.Equalf(t, test._shouldPass, err == nil, …)` form (RULE-GOLANG-ASSERT-002).

**RULE-GOLANG-TEST-012** `[review]` — Every field of a `ProvideComplete<Model>` fixture is explicitly set and deterministic.

* Every field of the model MUST be explicitly set (no zero-value omissions).
* Covered by `exhaustruct` where it runs. The linter scope is `*_test_data.go` — a fixture living
  outside it is outside that scope entirely, so its field completeness is a review check.
* All fields MUST also be deterministic — a value exhaustruct cannot judge, since it only proves the
  field is present:
  * No `RandomId()` or `uuid.New()` — use a fixed string such as `"id"`.
  * No `time.Now()` — use the canonical test epoch `time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)`.

  them; trust their defaults as a valid interpretation of the business rules.
* Use opts where they are generated; where opts don't exist yet, construct-then-mutate in the case
  block is the interim fallback.

---

### TABLE-DRIVEN TESTS

**RULE-GOLANG-TEST-001** `[review]` — Table-driven tests use a struct named `testCase`.

* Table-driven tests MUST use a struct named `testCase`
* Meta fields MUST be prefixed with `_`
* Input fields MUST NOT be prefixed with `_`

**RULE-GOLANG-TEST-002** `[review]` — Meta fields come first (`_id` first, rest alphabetical), one blank line, then input fields alphabetical — in the declaration and every literal.

Meta fields in `testCase` MUST come first: `_id` first, then the remaining meta fields alphabetically.
Then EXACTLY one blank line, then input fields, also sorted alphabetically. The same order applies in
the `type` declaration and in every case literal.

Example:

```go
type testCase struct {
	_id                string
	_expectedQuery     string
	_expectedQueryArgs []interface{}
	_shouldPass        bool

	filter *GameLinksFilter
	query  string
}
```

**RULE-GOLANG-TEST-003** `[review]` — In each test case literal, `_id` is the first field.

In each test case literal, `_id` MUST be the first field.

**RULE-GOLANG-TEST-004** `[review]` — The canonical setup line shown below precedes the loop over test cases.

Before the loop over test cases, add:

```go
// Run tests
```

**RULE-GOLANG-TEST-005** `[review]` — Test execution uses `t.Run(test._id, func(t *testing.T) { ... })`.

Test execution MUST use `t.Run(test._id, func(t *testing.T) { ... })`.

**RULE-GOLANG-TEST-006** `[review]` — Test case IDs use kebab-case (`"nil-error"`).

Test case IDs MUST use kebab-case (e.g. `"nil-error"`, `"pq-error-with-different-code"`).

**RULE-GOLANG-TEST-007** `[review]` — A table-driven test follows the canonical shape below; for one or two simple cases plain asserts are preferred.

This rule defines what "table-driven test" means concretely. It applies only when a table-driven test is warranted (complex behavior or many combinations — see baseline). For one or two simple cases, plain `assert.Equal` calls are preferred over the boilerplate below.

Test cases MUST be collected using `make([]*testCase, 0)` and appended individually with `append`. Do NOT use a slice literal.

Each append block MUST be preceded by a comment that repeats the `_id` value verbatim:

```go
tests := make([]*testCase, 0)

// nil-error
tests = append(tests, &testCase{
	_id:       "nil-error",
	_expected: false,

	err: nil,
})

// unrelated-error
tests = append(tests, &testCase{
	_id:       "unrelated-error",
	_expected: false,

	err: errors.New("something went wrong"),
})
```

**RULE-GOLANG-TEST-014** `[review]` — No table-driven structure when the method has no branching and only one meaningful scenario — write a direct test.

A test MUST NOT use table-driven structure when the method under test has no branching AND there is
only one meaningful scenario. Write a direct test instead:

```go
func TestLoginIntent_UpdateExpired(t *testing.T) {
    intent := ProvideCompleteLoginIntent()
    intent.UpdateExpired()

    assert.Equal(t, LoginIntentStateExpired, intent.State)
    assert.False(t, intent.Active)
}
```

When a method branches (e.g. on a threshold or state), use table-driven tests to cover each branch.

**RULE-GOLANG-TEST-015** `[review]` — `type testCase` is the first declaration in the test function; shared fixtures come after it.

* `type testCase` MUST be the first declaration in the test function. Shared fixtures
  (`opt := <Model>Opts()`, time anchors, hashes) come after it.

**RULE-GOLANG-TEST-008** `[review]` — Test-case structs carry fully-constructed dependencies as fields; the `t.Run` body holds only the call under test and assertions.

Test-case structs MUST carry fully-constructed dependencies (db, service, clients) as fields, initialized inside each case's append block. The `t.Run` loop body contains ONLY the call under test and assertions — no dependency construction, no field wiring from indirection fields.

The body MUST be completely flat: value / assert / assert — zero declarations, no `if`, no early `return`, no nil-guards. All per-case setup (including path variables, request mutation, form aliasing) happens in the case block above the loop. Assertions MUST NOT be conditional (no `if test._shouldPass { … }`, no `switch` on expected outcomes) — expected outcomes are flat `_expected*` meta fields asserted on every case (zero values for failure cases); errors are checked via the `_shouldPass` assertion (RULE-GOLANG-ASSERT-002).

```go
// fallback-match
db := models.NewMockDB()
db.TagRequests = provideTagRequests()
tests = append(tests, &testCase{
	_id:     "fallback-match",
	service: NewPrebidWebEventsServiceV1(&ServerContext{db: db}),
	event:   provideWebEvent(),
})
```

Not: `db.TagRequests = test.tagRequests` inside the run loop.

**RULE-GOLANG-TEST-016** `[lint]` — No mocking-framework imports in tests.

* Covered by `depguard` where it runs. Importing a mocking framework (`golang/mock`, `uber-go/mock`,
  `testify/mock`, `mockery`) in a test is forbidden — a guard against adding one rather than a live
  finding.

  (`stub.ProvideCompleteServerContext()` returns a ready `*cfapi.Context` with an in-memory DB
  pre-seeded with relation-consistent fixtures, plus a `*stub.Seed` exposing the seeded models).
* Creating any OTHER fake, mock, or test double of DB controllers or infrastructure interfaces
  remains FORBIDDEN — extend `cfapi/db/stub` in place (same package, same patterns) when a method or
  domain is missing; never build a parallel abstraction.
* Add extra test data through the stub's public `Create*` methods, built with
  `ProvideComplete<Model>` + options (RULE-GOLANG-TEST-011).
* The stub covers `serverContext.CfApiClient` via `CfApiClientStub` — seeded with a session
  (accessible via `stub.SeedSessionKey`) and a feature matrix. Use `stub.SeedSessionKey` in
  `Authorization: Bearer` headers to pass `auth.Do`.

**RULE-GOLANG-TEST-010** `[review]` — A test asserting cross-system compatibility builds its fixture from the counterpart system's real output, never from the code under test.

* A test that claims compatibility with an external or counterpart system (a Django session blob, a real vendor response, another service's persisted artifact) MUST build its fixture from that system's captured real output.
* A self-derived fixture — one generated by the implementation under test — proves only internal self-consistency, not compatibility. Overlaps the design-side data-integrity doctrine (ACTION-IMPL-INTEG-011).

---

### ASSERTION STYLE

**RULE-GOLANG-ASSERT-001** `[review]` — Assertions use the testify helpers (`assert.Equal`, `assert.NotNil`, `assert.Same`).

* Use `assert.Equal`, `assert.NotNil`, `assert.Same`, etc.
* Prefer direct value assertions over custom `if`/`t.Fatalf` checks, unless assertion helpers are
  insufficient.
* Covered by `testifylint` for the fitting-assertion slice. It rejects
  `assert.Equal(t, len(x), 3)` for `assert.Len`, `assert.Equal(t, true, x)` for `assert.True`,
  reversed expected/actual, and so on — but it only inspects calls that already go through testify,
  so custom `if`/`t.Fatalf` checks are a review check. Its `require-error` check is deliberately
  disabled: `require` is for setup steps that must abort, not for error assertions (see BASELINE and
  RULE-GOLANG-ASSERT-002).

**RULE-GOLANG-ASSERT-002** `[review]` — `_shouldPass` patterns assert `err == nil` against `_shouldPass`.

For `_shouldPass` patterns, assert `err == nil` against `_shouldPass`.

```go
assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
```

**RULE-GOLANG-ASSERT-003** `[review]` — A method mutating 3 or more fields is asserted against a single `_expected *Model` built with `ProvideComplete` + options.

When a method under test mutates 3 or more fields, use a single `_expected *Model` meta field instead
of multiple `_expectedFoo`, `_expectedBar`, … fields. Build `_expected` with `ProvideComplete<Model>`
+ options, overriding only the fields the method is expected to change. All unchanged fields inherit
their deterministic defaults, making unexpected mutations visible.

```go
type testCase struct {
    _id       string
    _expected *LoginIntent

    intent *LoginIntent
}

// below-max-attempts
expected := ProvideCompleteLoginIntent(
    opt.VerifyCount(1),
    opt.UpdatedAt(syncNow),
    opt.VerifyAvailableAt(sql.NullTime{Time: syncNow.Add(LoginIntentVerifyCooldown), Valid: true}),
)
// ...

// Run tests
for _, test := range tests {
    synctest.Test(t, func(t *testing.T) {
        test.intent.UpdateFailedVerification()
        assert.Equal(t, test._expected, test.intent)
    })
}
```

---

### TIME-DEPENDENT TESTS

**RULE-GOLANG-TEST-018** `[review]` — A method calling `time.Now()` internally is tested inside `synctest.Test` against the canonical epoch anchor.

Any method under test that calls `time.Now()` internally MUST be tested inside `synctest.Test`:

```go
synctest.Test(t, func(t *testing.T) {
    test.intent.UpdateFailedVerification()
    assert.Equal(t, test._expected, test.intent)
})
```

* Declare `syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)` as the time anchor — this matches
  the `ProvideComplete` epoch (RULE-GOLANG-TEST-012) so base fields need no override.
* Build `_expected` using time options anchored to `syncNow`, e.g. `opt.UpdatedAt(syncNow)`,
  `opt.OtpExpiresAt(sql.NullTime{Time: syncNow.Add(LoginIntentOtpTtl), Valid: true})`.
* `synctest.Test` replaces `t.Run` for that iteration; `_id`-based subtest naming inside
  `synctest.Test` is optional.

---

### AGENT WORKFLOW

When creating or editing tests, AI agents MUST:

1. Reuse an existing test file in the same package as template first.
2. Keep naming and field order consistent with this document.
3. Run targeted tests for changed tests first (`go test ./path -run TestName`).
4. Run `gofmt` (or `goimports` if imports changed) before finishing.

---

## GOROUTINES

### BASELINE

Concurrent code MUST be readable top-to-bottom by a human who has never seen it before.

---

### WORKER POOL

**RULE-GOLANG-GR-001** `[review]` — Concurrent batch processing uses a fixed worker pool consuming from a work channel.

* Concurrent batch processing MUST use a fixed worker pool: N goroutines consuming from a work channel
* MUST NOT spawn one goroutine per work item
* MUST NOT use channel-based semaphores (`make(chan struct{}, N)`) to limit concurrency

**RULE-GOLANG-GR-002** `[review]` — Use `wg.Go` (Go 1.26) instead of manual `wg.Add` / `wg.Done`.

* Use `wg.Go` (Go 1.26) instead of manual `wg.Add` / `wg.Done`.
* Covered by `modernize/waitgroupgo` for exactly one shape. That shape is a `wg.Add(1)` immediately
  followed by a goroutine that calls `wg.Done`. It is silent whenever the closure takes a parameter
  (`go func(value string){ defer wg.Done() }(item)`), for a bulk `wg.Add(len(items))` before a
  loop, for an `Add` separated from its goroutine by another statement, and where the goroutine body
  is a named function that calls `wg.Done` itself — all of those are review checks.
* The parameterized-closure case compounds with RULE-GOLANG-GR-008: passing the loop variable as a
  closure parameter is itself a violation, and it is exactly what hides the `wg.Add`/`wg.Done` from
  CI. That shape trips neither linter — check it by eye.

---

### READING ORDER

**RULE-GOLANG-GR-003** `[review]` — Concurrent code reads in data-flow order: input → processing → output.

* Code MUST read in data-flow order: input → processing → output
* The work source (what is being processed) MUST appear before the consumer (goroutines that process it)
* A reader scanning top-to-bottom MUST NOT encounter a channel consumer before understanding what the channel carries

BAD — consumer before producer:

```go
workChan := make(chan work)

for range workerCount {
    wg.Go(func() {
        for w := range workChan { // what is in workChan? unknown at this point
            process(w)
        }
    })
}

for _, item := range items { // NOW I see the data — too late
    workChan <- item
}
```

GOOD — data flow reads top-to-bottom:

```go
workChan := make(chan work, len(items))

for _, item := range items {
    workChan <- item
}

close(workChan)

resultChan := make(chan result, len(items))

for range workerCount {
    wg.Go(func() {
        for w := range workChan {
            resultChan <- process(w)
        }
    })
}

wg.Wait()
close(resultChan)

var results []result
for r := range resultChan {
    results = append(results, r)
}
```

---

### RESULT COLLECTION

**RULE-GOLANG-GR-004** `[review]` — Worker results are sent to a result channel.

* Worker results MUST be sent to a result channel
* MUST NOT write into a shared slice by index (`results[idx] = ...`)
* After all workers complete, close the result channel and collect into a slice

WHY: Index-based collection forces bookkeeping (passing indices through the pipeline, `batchWork.idx`, `results[work.idx]`). A result channel eliminates this: workers send, collector receives, done.

---

### WORKER BODY

**RULE-GOLANG-GR-005** `[review]` — The `wg.Go` closure contains only the range loop and a single function call per item.

* The `wg.Go` closure MUST contain only the range loop and a single function call per item
* The per-item work logic MUST be a named function

BAD — fat inline closure:

```go
wg.Go(func() {
    for w := range workChan {
        details, _, err := hub.GetNotificationDetails(w.id)
        if err != nil {
            log.Printf(...)
            resultChan <- errorResult(w.id, err)
        } else {
            log.Printf(...)
            resultChan <- successResult(w.id, details)
        }
    }
})
```

GOOD — named function:

```go
wg.Go(func() {
    for w := range workChan {
        resultChan <- fetchTelemetry(hub, w)
    }
})
```

---

### CHANNEL SIZING

**RULE-GOLANG-GR-006** `[review]` — Work channels for pre-known input buffer to `len(items)`, filled and closed before workers start.

* Work channels for pre-known input: buffer to `len(items)`, fill and close before starting workers
* Result channels: buffer to expected result count to prevent worker blocking
* Unbuffered channels: only when synchronization between sender and receiver is the intent

---

### SHARED STATE

**RULE-GOLANG-GR-007** `[review]` — Never `sync.Map` — a `sync.Mutex` (or `RWMutex`) guarding a typed map.

* MUST NOT use `sync.Map` — use a `sync.Mutex` (or `RWMutex`) guarding a typed map.
* Covered by `forbidigo` where it runs.
* Declare the mutex directly above the map it guards on the owning struct
* A `sync.Mutex` guarding a shared `sendFailed`/`ok` bool across goroutines is a code smell — send failures on an error channel (buffered to the goroutine count) and collect them after `wg.Wait()`. RULE-GOLANG-GR-004 already mandates a result channel; this extends it to failure signalling.

---

### LOOP VARIABLES

**RULE-GOLANG-GR-008** `[review]` — Go 1.22+ loop variables are per-iteration — never re-declared or passed as an extra closure parameter.

* Go 1.22+ loop variables are per-iteration — MUST NOT re-declare the loop variable (`w := w`) to
  "fix" capture; that idiom is dead weight.
* Covered by `copyloopvar` where it runs.
* The same applies to passing the loop variable as an extra closure parameter
  (`go func(item string){ … }(item)`), which copyloopvar does not detect — a review check.

---

### DEDUPLICATION

**RULE-GOLANG-GR-009** `[review]` — Eliminate a mode-switch branch by normalizing inputs into one collection, not by extracting the duplicated body into a helper.

* When two code paths differ only by a mode flag, remove the branch by normalizing the inputs into one collection and running a single loop over it — one logic, one path.
* Do NOT extract the duplicated body into a shared helper called from both branches; that preserves the fork instead of removing it. Complements RULE-GOLANG-GR-005 (worker body is a single named call) but is about de-duplication strategy, not closure shape.

---

### ENFORCEMENT

* These rules MUST be applied strictly
* Fix violations instead of discussing them

---

## Tombstones

| Retired | Replacement | Date |
|---|---|---|
| RULE-GOLANG-TEST-009 | RULE-GOLANG-TEST-011, RULE-GOLANG-TEST-012 | 2026-08-19 |
