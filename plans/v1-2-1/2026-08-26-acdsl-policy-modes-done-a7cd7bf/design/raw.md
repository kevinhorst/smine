# ACDSL agentic self-management: policy modes — Implementation Plan

## TLDR

- Add a per-repo self-management policy to ACDSL: `acdsl/policy.json` with three modes — `strict` (nothing about the rule system is editable), `gated` (rules editable only in designated scopes, only sanctioned verifiers bindable), `free` (today's behavior).
- The finite validator set is the shipped verifier registry itself: in strict/gated mode the registry, the generation bounds, and the policy file are frozen against a git base; any other validator is denied. New verifier binaries stay exclusively in the existing evalgen proposal flow (human-voted, `autoApply: false`).
- Enforcement is layered: authoritative check inside `acdsl check` (ships in the `bin/acdsl` binary — not bypassable via data files), plus a fail-open PreToolUse write-guard hook for immediate feedback.
- The privilege boundary is git topology: agent branches diff against merge-base with the base branch; humans change the self-management surface by committing to the base branch (or, for targets, via smine sync).
- Result: an agent in a gated target repo can add/tune e.g. GOLANG rules with existing verifiers, but cannot touch PLAN rules, add validators, or weaken the policy — the loosy-goosy-rules failure mode is structurally closed.

## Context

- **Problem:** ACDSL has no self-modification control — an agent can add ad-hoc rules and verifiers freely; observed failure mode is rule sprawl erroring on non-issues ([acdsl/rules.acdsl](acdsl/rules.acdsl), [acdsl/registry.json](acdsl/registry.json) are unguarded data files).
- **Existing primitives:** verifier registry with target-owned overlay ([internal/acdsl/registry.go:36](internal/acdsl/registry.go)), `reach`/`projected` rule attributes, evalgen generation bounds ([acdsl/evalgen.json](acdsl/evalgen.json)), settings `permissions.ask` on `acdsl/**` — none expresses editability modes.
- **Design:** three modes as requested (strict / gated / free), gated granularity by taxonomy SCOPE segment (e.g. GOLANG editable, PLAN not), the finite validator set anchored on the frozen registry.
- **Constraint:** the enforcement must not be neutered by the thing it guards — no registry-verifier self-reference, no policy file that can sanction its own edit.
- **Constraint:** smine's own sanctioned generation flow (smine-apply writes verifier + registry entry on routine branches) must keep working — smine itself stays `free`; targets get their mode via dist.

## Drivers

N/A — new route

## Scope

- **In:**
  - **policy file:** `acdsl/policy.json` + `acdsl/policy.schema.json`, schema-gated by a new `ACDSL-JSON-003` rule with fixtures.
  - **policy engine:** `internal/acdsl/policy.go` (`LoadPolicy`, `CheckPolicy`) wired into `runCheck`, plus git helpers in `git.go`.
  - **write-guard hook:** `cmd/hooks/acdsl-policy-guard.sh` + registration in `settings/claude_code/settings.json`.
  - **distribution:** `Dist` ships the policy (mode rewritten from `dist_mode`) so targets are gated with zero setup.
  - **docs:** `acdsl/README.md` modes section incl. a manual settings-deny snippet; `docs/acdsl-spec.md` modes paragraph.
  - **tests:** `internal/acdsl/policy_test.go`, a `dist_test.go` case, fixtures.
- **Out (explicit non-goals):**
  - **lint wrappers:** no golangci-lint / ruff wrapper verifiers — targets already run differential golangci-lint via their own Makefiles ([context/rules/go.md](context/rules/go.md) Enforcement); the finite-set requirement is met by freezing the registry.
  - **settings emission:** dist does not machine-write `permissions.deny` into a target's `.claude/settings.json` — invasive merge of a user-owned file; README snippet instead.
  - **chmod protection:** git does not persist write bits and the agent can chmod back — rejected, not deferred.
  - **config-server UI:** no policy display/edit in the ACDSL tab.
  - **hook diff inspection:** the hook does not parse Edit old/new strings for rule IDs; check-time catches out-of-scope edits.
- **Not changed:**
  - **evalgen flow:** verifier generation bounds and the voted proposal flow stay as-is.
  - **reach/projected:** semantics untouched; `SetRuleReach` remains the only in-code rule mutator.
  - **existing verifiers:** no behavior change.
- **Deferred findings:**
  - **require-fixtures flag:** optional gated-mode policy flag forcing pass/fail fixtures for agent-added rules — v1.1 candidate.
  - **per-target policy:** one shipped policy for all targets in v1; per-target overrides via the deploy section later.
  - **README overlay wording:** `DistHeader` ([internal/acdsl/dist.go:29](internal/acdsl/dist.go)) advertises `registry.local.json` as a free override — text updated in this plan's README change, header string kept (still true in free mode).

## Assumptions

| Assumption | Reality | Location |
|---|---|---|
| "golangci-lint is a good start for go" — assumed an in-ACDSL golangci integration exists to extend | None exists; `lint-tags` only cross-checks a guide's Covered-by claims against `.golangci.yml`; actual golangci gates live in target repos' Makefiles | [acdsl/registry.json:7](acdsl/registry.json) |
| Strict mode enforceable as "denied by settings, folders not writable" | Settings are user-global (synced from this repo), not per-repo; chmod is not git-durable — per-repo denial needs hook + in-binary check; settings-deny is a documented manual snippet | [settings/claude_code/settings.json](settings/claude_code/settings.json) |
| "edit the \*-GOLANG-\* rules, but not the PLAN rules" — rule groups addressable | Holds: rule IDs carry a taxonomy-registered SCOPE segment (`KIND-SCOPE[-TOPIC]-NNN`), gate-enforced by id-grammar | [acdsl/rules.acdsl:37](acdsl/rules.acdsl) |

## Current state

N/A — new route

## Target state

N/A — new route

## Behavior contract

N/A — new route

## Decisions

| ID | Problem | Facts | Decision | Why |
|---|---|---|---|---|
| <a id="d1"></a>D1 | Where the mode lives | [F4!](#f4), [F13](#f13) | New `acdsl/policy.json` + draft-07 schema, gated by new rule `ACDSL-JSON-003` (existing `jsonschema` verifier) | Single source of truth, dist-shippable like the registry, schema-checkable with zero new verifier code — debuggable (one file states the whole policy) |
| <a id="d2"></a>D2 | Enforcement mechanism | [F1!](#f1), [F3](#f3) | `CheckPolicy` built into `acdsl check` (compiled into `bin/acdsl`), **not** a registry verifier | A registry verifier is self-defeating: a `registry.local.json` entry `{"policy-check": {"argv": ["true"]}}` neuters it, and in gated mode the rule line binding it could be deleted by the very edit it guards — reliable only in the binary |
| <a id="d3"></a>D3 | What "finite validator set" means | [F1!](#f1), [F8!](#f8) | The frozen shipped registry IS the catalog — no second catalog concept; new verifier binaries only via the evalgen proposal flow (`autoApply: false`, human vote) | `Check` already fails on unknown verifier names, so freezing registry content closes every path; a parallel catalog file would be a second source of truth to desync |
| <a id="d4"></a>D4 | Privilege boundary for "who may change the policy surface" | [F14!](#f14), [F15](#f15) | Diff against `merge-base(HEAD, base_ref)`; empty diff = pass. Humans change the surface by committing to the base branch (targets: via smine sync); agent worktrees on `claude/*` branches can never self-escalate | Controllable: the boundary is the branch topology that already separates human main from agent branches — no new credential or approval concept; on the base branch the diff is vacuous by construction |
| <a id="d5"></a>D5 | Gated granularity | [F9](#f9), [F10](#f10) | Editable set = taxonomy SCOPE segments (`editable_scopes`); bindable verifiers = `verifier_allowlist`, empty meaning all names in the **base-ref** registry | Reuses the registered taxonomy (single vocabulary, id-grammar-gated) instead of inventing glob patterns; base-ref registry as default allowlist means "the shipped set" without listing it twice |
| <a id="d6"></a>D6 | `registry.local.json` bypass | [F1!](#f1) | In strict/gated every local-overlay entry name must appear in policy `local_overrides`; the check reads the overlay content directly (works even untracked) | The overlay wins by name at load time — unguarded, it re-points any verifier at `/bin/true`; content-level reading closes the untracked-file hole |
| <a id="d7"></a>D7 | Write-time feedback | [F6](#f6), [F7](#f7) | Fail-open PreToolUse hook denying writes to the policy surface (both modes) and to all rule files (strict); gated rule edits pass the hook and are judged at check time | Hooks are advisory UX (jq-missing → allow, per house hook style); the authoritative layer is the binary check — one enforcement truth, one convenience layer |
| <a id="d8"></a>D8 | Per-repo mode with one shipped file | [F4!](#f4), [F14!](#f14) | Two fields: `mode` (this repo) + `dist_mode` (what dist writes into the target's `mode`; `dist_mode` stripped from the shipped copy) | smine must stay `free` (generation flow commits registry entries on branches) while targets default `gated`; one file, deterministic rewrite at dist time |
| <a id="d9"></a>D9 | Task contracts under strict/gated | [F2](#f2) | `lifetime="task"` entries exempt from the rule-delta check by default (`allow_task_contracts: true`, flippable) | Task contracts are the plan mechanism — ephemeral, never synced, gated separately via `-lifetime task`; blocking them would break /fdesign task contracts in every gated repo |
| <a id="d10"></a>D10 | Unresolvable base ref | [F15](#f15) | In strict/gated an unresolvable base (`origin/HEAD` → `origin/main` → `main` → `master`, then `base_ref` override) is a hard tool error (exit 2), never a silent pass | Reliable: degrading to "ungated" on a shallow clone would silently disable the whole feature; loud failure names the fix (`base_ref` or fetch) |
| <a id="d11"></a>D11 | Initial policy values | [F10](#f10), [F14!](#f14) | [USER] `mode: "free"`, `dist_mode: "gated"`, `editable_scopes: ["GOLANG","PYTHON","SHELL","SQL","JSON"]`, `verifier_allowlist: []`, `local_overrides: []` — language scopes editable, PLAN/SKILL/SMINE/RUNBOOK/COMMIT protected, smine unrestricted | Decides what agents in target repos can actually do on day one; wrong-way-loose is the failure mode this feature exists for, wrong-way-tight blocks legitimate rule tuning |

## Baseline (verified)

Base branch: `main` (worktree branch `claude/acdsl-validator-modes-546cbf`).

| ID | Fact | Needed for | Location |
|---|---|---|---|
| <a id="f1"></a>F1! | `LoadRegistry` merges a sibling `registry.local.json` whose entries win by name — target-owned, never synced, no restriction on what it may override | [D2](#d2), [D3](#d3), [D6](#d6) | [internal/acdsl/registry.go:36-54](internal/acdsl/registry.go) |
| <a id="f4"></a>F4! | `Dist` ships reach-covered rule marker lines byte-verbatim, the registry subset (argv rewritten to `bin/verifiers/<name>`), and prebuilt `bin/acdsl` + verifier binaries — the target runs the shipped binary, not `go run` | [D1](#d1), [D8](#d8), [§7](#c7) | [internal/acdsl/dist.go:49-129](internal/acdsl/dist.go) |
| <a id="f8"></a>F8! | Verifier generation is already bounded and human-gated: `acdsl/evalgen.json` sets `autoApply: false`, maxLines 250, exec allowlist, stdlib-only imports, enforced by `evalgen-bounds` over `cmd/acdsl/verifiers/*/main.go` | [D3](#d3) | [acdsl/evalgen.json](acdsl/evalgen.json), [acdsl/rules.acdsl:35](acdsl/rules.acdsl) |
| <a id="f14"></a>F14! | The smine-apply generation flow writes new verifier packages + registry entries on routine worktree branches — a gated smine would red-flag its own sanctioned flow | [D4](#d4), [D8](#d8), [D11](#d11) | [acdsl/README.md](acdsl/README.md) generation section; routine worktrees per FACT-REPO-ARCH-003 |
| <a id="f2"></a>F2 | Rule attributes: `reach` (global/none/repo-list), `projected`, `lifetime` doctrine\|task, free params; `Rule` carries `File`+`Line` of its declaration; duplicate IDs are authoring violations | [D9](#d9), [§1](#c1) | [internal/acdsl/rule.go:31-53](internal/acdsl/rule.go) |
| <a id="f3"></a>F3 | `runCheck` pipeline: `DiscoverRules` → `ValidateStagedClean` → lifetime/id filter → `LoadRegistry` → `Check` → `logVerdicts` → print `Diagnostic{Message, RuleId, Verifier, Why}` records; exit 0/1/2 | [D2](#d2), [§3](#c3) | [cmd/acdsl/main.go:81-147](cmd/acdsl/main.go) |
| <a id="f5"></a>F5 | `markerLine(root, rule)` reads a rule's declaration line trimmed — the existing primitive for byte-level marker comparison | [§1](#c1) | [internal/acdsl/dist.go:182-192](internal/acdsl/dist.go) |
| <a id="f6"></a>F6 | House hook shape: `set -euo pipefail`, `command -v jq \|\| exit 0` fail-open, stdin JSON, deny via `hookSpecificOutput.permissionDecision: "deny"`, bash-3.2-safe | [§8](#c8) | [cmd/hooks/worktree-write-guard.sh](cmd/hooks/worktree-write-guard.sh) |
| <a id="f7"></a>F7 | PreToolUse `Write\|Edit\|NotebookEdit` matcher block exists with one hook entry — the new guard appends to its `hooks` array | [§9](#c9) | [settings/claude_code/settings.json:198-207](settings/claude_code/settings.json) |
| <a id="f9"></a>F9 | ID shape regex `^(ACDSL\|RULE\|FACT\|ACTION)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-(\d{3})$` — capture 2 is the SCOPE segment | [D5](#d5), [§1](#c1) | [cmd/acdsl/verifiers/idgrammar/main.go:22](cmd/acdsl/verifiers/idgrammar/main.go) |
| <a id="f10"></a>F10 | Registered class-`scope` taxonomy entries: GOLANG, PYTHON, SQL, SHELL, JSON, SKILL, SMINE, PLAN, RUNBOOK, COMMIT, CONCEPT, IMPL, NAV, REVIEW, REPO | [D5](#d5), [D11](#d11) | [context/context.json](context/context.json) `.aspects` |
| <a id="f15"></a>F15 | Git access goes through `internal/shell.Run` with context deadline (raw `exec.Command` is gate-forbidden); existing helpers: `GitBranch`, `shellGitGrepCached` | [D4](#d4), [D10](#d10), [§2](#c2) | [internal/acdsl/git.go](internal/acdsl/git.go) |
| <a id="f11"></a>F11 | `make audit` = build-audit → vet → `./bin/acdsl project -strip` → `./bin/acdsl check` → `./bin/acdsl fixtures` → tests; the gate runs the built binary | Verification | [Makefile:40-48](Makefile) |
| <a id="f13"></a>F13 | `jsonschema` verifier validates anchored JSON against a draft-07 schema named by `schema=` param; existing exemplar rules ACDSL-JSON-001/002 with `empty="ok"` + `needs=` | [D1](#d1), [§5](#c5) | [acdsl/registry.json:87-91](acdsl/registry.json), [acdsl/rules.acdsl:31-32](acdsl/rules.acdsl) |
| <a id="f12"></a>F12 | Absent-key JSON default trick: preset the Go struct before `json.Unmarshal` so a missing `allow_task_contracts` keeps `true` | [§1](#c1) | pattern; unmarshal semantics |

## Exemplar & reuse

| Existing | Used for |
|---|---|
| `internal/shell.Run` | all git invocations in the policy engine (EXEC-001 gate) |
| `jsonschema` verifier + ACDSL-JSON-001 rule shape | policy schema gate ACDSL-JSON-003 |
| `markerLine` comparison primitive ([dist.go:182](internal/acdsl/dist.go)) | rule-delta "changed" detection |
| `LoadRegistry` | base-side registry parse (via `git show` content variant) |
| evalgen proposal flow | the only path for new verifier binaries in strict/gated |
| `fsx.CopyFile` + `writeDistRegistry` pattern | shipping policy.json/schema in `Dist` |

- Without exemplar: the merge-base delta computation (`ruleDelta`) — no prior git-diff-based check exists in the repo; risk carried by the full example implementation in [Hot items](#hot-items) and the widest test table.

## Changes

### <a id="c1"></a>1. Policy engine (new)

location: `internal/acdsl/policy.go`

New file: policy type, loader, and the mode gate. Core code (complete; imports/file preamble per repo pattern):

```go
package acdsl

// PolicyRuleId labels policy diagnostics in check output and the verdict log.
const PolicyRuleId = "ACDSL-POLICY"

// PolicyMode is the self-management stance of a repo's rule system.
type PolicyMode string

const (
	PolicyModeFree   PolicyMode = "free"
	PolicyModeGated  PolicyMode = "gated"
	PolicyModeStrict PolicyMode = "strict"
)

// policySurface is the frozen self-management surface: in strict and gated
// mode none of these files may differ from the policy base.
var policySurface = []string{
	"acdsl/policy.json",
	"acdsl/policy.schema.json",
	"acdsl/registry.json",
	"acdsl/evalgen.json",
}

// policyIdRe mirrors the id-grammar shape (capture 2 = SCOPE segment). The
// verifier keeps its own copy — it must stay standalone.
var policyIdRe = regexp.MustCompile(`^(ACDSL|RULE|FACT|ACTION)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-(\d{3})$`)

// Policy is acdsl/policy.json: who may change the rule system, and how far.
type Policy struct {
	AllowTaskContracts bool       `json:"allow_task_contracts"`
	BaseRef            string     `json:"base_ref"`
	DistMode           PolicyMode `json:"dist_mode"`
	EditableScopes     []string   `json:"editable_scopes"`
	LocalOverrides     []string   `json:"local_overrides"`
	Mode               PolicyMode `json:"mode"`
	VerifierAllowlist  []string   `json:"verifier_allowlist"`
}

// LoadPolicy reads acdsl/policy.json under root. An absent file is free mode
// (today's behavior); an unknown mode is an error, never a silent free.
func LoadPolicy(root string) (Policy, error) {
	policy := Policy{AllowTaskContracts: true, Mode: PolicyModeFree}
	raw, err := os.ReadFile(filepath.Join(root, "acdsl", "policy.json"))
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("LoadPolicy: %w", err)
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("LoadPolicy: acdsl/policy.json: %w", err)
	}
	switch policy.Mode {
	case PolicyModeFree, PolicyModeGated, PolicyModeStrict:
		return policy, nil
	}
	return Policy{}, fmt.Errorf("LoadPolicy: acdsl/policy.json: unknown mode %q", policy.Mode)
}

// CheckPolicy is the mode gate: it diffs the self-management surface and the
// declared rules against the policy base and returns one diagnostic per
// violation. Free mode never diffs.
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	if policy.Mode == PolicyModeFree {
		return nil, nil
	}
	base, isBoundary, err := policyBase(ctx, root, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	if !isBoundary {
		return nil, nil // HEAD is the base tip — humans change the surface here
	}

	var diagnostics []Diagnostic
	surface, err := surfaceDelta(ctx, root, base)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, surface...)

	overlay, err := overlayViolations(root, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, overlay...)

	delta, err := ruleDelta(ctx, root, base)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, ruleViolations(ctx, root, base, policy, delta)...)
	return diagnostics, nil
}
```

Unexported helpers (complete signatures; bodies per the algorithm below):

```go
// policyBase resolves merge-base(HEAD, base ref). isBoundary is false when
// HEAD == base tip (running on the base branch itself). Candidates:
// policy.BaseRef, else origin/HEAD, origin/main, main, master — first that
// rev-parses. No candidate in strict/gated is a hard error.
func policyBase(ctx context.Context, root string, policy Policy) (base string, isBoundary bool, err error)

// surfaceDelta returns one diagnostic per policySurface file whose working
// content differs from git show <base>:<path> (absent on either side counts
// as a difference; absent on both sides is clean).
func surfaceDelta(ctx context.Context, root, base string) ([]Diagnostic, error)

// overlayViolations reads acdsl/registry.local.json directly (tracked or
// not) and flags every entry name missing from policy.LocalOverrides.
func overlayViolations(root string, policy Policy) ([]Diagnostic, error)

// ruleDelta parses declaration-capable files changed vs base (git diff
// --name-only <base> plus untracked) with ParseRules on both sides and
// returns added / removed / changed rules, "changed" = trimmed marker line
// differs (markerLine on the working side, base lines via git show).
func ruleDelta(ctx context.Context, root, base string) (policyRuleDelta, error)

// ruleViolations applies the mode to the delta: strict flags every entry;
// gated flags entries whose SCOPE segment (policyIdRe capture 2, non-match
// = protected) is outside EditableScopes, and added/changed entries binding
// a verifier outside the effective allowlist (VerifierAllowlist, else the
// base-ref registry's names via git show <base>:acdsl/registry.json).
// Task-lifetime entries are dropped first when AllowTaskContracts.
func ruleViolations(ctx context.Context, root, base string, policy Policy, delta policyRuleDelta) []Diagnostic
```

- **Diagnostic shape:** `Message` = `<file>:<line>: policy(<mode>): rule <id> added|modified|removed outside <detail>`, `RuleId` = `PolicyRuleId`, `Verifier` = `"policy"`, `Why` names the fix ("sanction the scope in acdsl/policy.json on <base ref>, or revert").
- **Normalization:** marker comparison is `strings.TrimSpace` byte equality — a field reorder counts as a change; deliberate, simple, self-inflicted false positive.
- **Base-side parse:** `git show <base>:<path>` content split into lines and fed to `ParseRules` with the same path key; base-side authoring violations are ignored (base was committed green).
- mirrors: [internal/acdsl/dist.go](internal/acdsl/dist.go) for git+registry interplay, error style `Component: %w` per [registry.go:39](internal/acdsl/registry.go).

### <a id="c2"></a>2. Git helpers (modified)

location: `internal/acdsl/git.go`

```go
// GitMergeBase returns merge-base(HEAD, ref).
func GitMergeBase(ctx context.Context, root, ref string) (string, error)

// GitRevParse resolves ref to a commit sha; the error carries git's stderr.
func GitRevParse(ctx context.Context, root, ref string) (string, error)

// GitChangedFiles lists paths differing from base: git diff --name-only
// <base> merged with untracked non-ignored files.
func GitChangedFiles(ctx context.Context, root, base string) ([]string, error)

// GitShowFile returns rev:path content; a path absent at rev returns
// ok=false, never an error.
func GitShowFile(ctx context.Context, root, rev, path string) (content string, ok bool, err error)
```

- All via `internal/shell.Run`, mirroring `GitBranch` ([git.go:30](internal/acdsl/git.go)).

### <a id="c3"></a>3. Check wiring (modified)

location: `cmd/acdsl/main.go`

```diff
 func runCheck(args []string) int {
 	// ...
 	registry, err := acdsl.LoadRegistry(*registryPath)
 	if err != nil {
 		fmt.Fprintln(os.Stderr, "acdsl:", err)
 		return exitError
 	}
+	policy, err := acdsl.LoadPolicy(*root)
+	if err != nil {
+		fmt.Fprintln(os.Stderr, "acdsl:", err)
+		return exitError
+	}
+	policyDiagnostics, err := acdsl.CheckPolicy(ctx, *root, policy)
+	if err != nil {
+		fmt.Fprintln(os.Stderr, "acdsl:", err)
+		return exitError
+	}
 	diagnostics, err := acdsl.Check(ctx, *root, gated, registry, discovery.Universe)
 	if err != nil {
 		fmt.Fprintln(os.Stderr, "acdsl:", err)
 		return exitError
 	}
+	diagnostics = append(policyDiagnostics, diagnostics...)
 	logVerdicts(ctx, *root, gated, diagnostics)
```

- Doc comment at the top of the file gains one sentence on the policy gate.

### <a id="c5"></a>4. Policy file + schema (new)

location: `acdsl/policy.json`, `acdsl/policy.schema.json`

`acdsl/policy.json` (values per [D11](#d11) recommendation, pending the OPEN answer):

```json
{
  "mode": "free",
  "dist_mode": "gated",
  "base_ref": "",
  "editable_scopes": ["GOLANG", "PYTHON", "SHELL", "SQL", "JSON"],
  "verifier_allowlist": [],
  "local_overrides": [],
  "allow_task_contracts": true
}
```

`acdsl/policy.schema.json`:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "additionalProperties": false,
  "required": ["mode"],
  "properties": {
    "mode": {"enum": ["free", "gated", "strict"]},
    "dist_mode": {"enum": ["free", "gated", "strict"]},
    "base_ref": {"type": "string"},
    "editable_scopes": {"type": "array", "items": {"type": "string", "pattern": "^[A-Z]{2,12}$"}},
    "verifier_allowlist": {"type": "array", "items": {"type": "string"}},
    "local_overrides": {"type": "array", "items": {"type": "string"}},
    "allow_task_contracts": {"type": "boolean"}
  }
}
```

### <a id="c6"></a>5. Schema gate rule + fixtures (modified/new)

location: `acdsl/rules.acdsl`, `acdsl/testdata/ACDSL-JSON-003/`

```diff
 //acdsl:ACDSL-JSON-002 jsonschema anchor="^sessions/(personal|work)/json/.*\.json$" schema="skills/smine/smine-batch/reference/schema.json" empty="ok" why="Batch summary JSON conforms to the smine-batch schema — deep links and dimension queries select on its fields; drift makes batches unqueryable"
+//acdsl:ACDSL-JSON-003 jsonschema reach="global" anchor="^acdsl/policy\.json$" schema="acdsl/policy.schema.json" empty="ok" needs="acdsl/policy.schema.json" why="acdsl/policy.json conforms to its schema — a malformed mode or scope list silently degrades the self-management gate"
```

- Fixtures mirror `acdsl/testdata/ACDSL-JSON-001/` structure: `pass/` one valid policy, `fail/` one with `"mode": "loose"` and one with an unknown key.

### <a id="c7"></a>6. Dist ships the policy (modified)

location: `internal/acdsl/dist.go`

```diff
 	if err := buildBinaries(ctx, root, dest, built); err != nil {
 		return nil, err
 	}
+	distMode, err := writePolicy(root, dest)
+	if err != nil {
+		return nil, err
+	}

 	lines := skipped
 	lines = append(lines,
 		fmt.Sprintf("  acdsl/rules.acdsl -> %d rule(s) reach %s", len(shipped), target),
 		fmt.Sprintf("  acdsl/registry.json -> %d verifier contract(s)", len(built)+countVerbatim(shipped, registry, built)),
 		"  bin/acdsl -> check/project/fixtures runner",
 	)
+	if distMode != "" {
+		lines = append(lines, fmt.Sprintf("  acdsl/policy.json -> mode: %s", distMode))
+	}
```

New unit:

```go
// writePolicy ships the policy into dest: mode rewritten from dist_mode
// (falling back to the source mode), dist_mode stripped — a target never
// re-distributes. The schema ships verbatim beside it. No source policy
// ships nothing and returns "".
func writePolicy(root, dest string) (PolicyMode, error)
```

- Shipped copy is baseline-owned and overwritten every sync, like the registry subset — a target-edited policy is reverted by the next sync (and flagged by its own gate meanwhile).

### <a id="c8"></a>7. Write-guard hook (new)

location: `cmd/hooks/acdsl-policy-guard.sh`

mirrors: [cmd/hooks/worktree-write-guard.sh](cmd/hooks/worktree-write-guard.sh)

```bash
#!/usr/bin/env bash
# PreToolUse(Write|Edit|NotebookEdit) guard: in a repo with a non-free
# acdsl/policy.json, deny writes to the self-management surface (both
# modes) and to any rule file (strict). Gated rule edits pass here and are
# judged authoritatively by acdsl check (bin/acdsl CheckPolicy) — this hook
# is fast feedback, fail-open by design.
set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
cwd=$(jq -r '.cwd // empty' <<<"$input")
file_path=$(jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' <<<"$input")
[ -n "$cwd" ] && [ -n "$file_path" ] || exit 0

case "$file_path" in
  /*) abs="$file_path" ;;
  *) abs="$cwd/$file_path" ;;
esac

root="$cwd"
while [ "$root" != "/" ] && [ ! -f "$root/acdsl/policy.json" ]; do
  root=$(dirname "$root")
done
[ -f "$root/acdsl/policy.json" ] || exit 0

mode=$(jq -r '.mode // "free"' "$root/acdsl/policy.json" 2>/dev/null) || exit 0
[ "$mode" = "strict" ] || [ "$mode" = "gated" ] || exit 0

deny() {
  jq -n --arg reason "$1" \
    '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
  exit 0
}

case "$abs" in
  "$root"/*) ;;
  *) exit 0 ;; # outside the policy repo — not this guard's business
esac
rel="${abs#"$root"/}"

case "$rel" in
  acdsl/policy.json|acdsl/policy.schema.json|acdsl/registry.json|acdsl/registry.local.json|acdsl/evalgen.json)
    deny "acdsl policy mode is $mode: the self-management surface ($rel) changes only on the base branch (see acdsl/README.md, Modes)" ;;
esac

if [ "$mode" = "strict" ]; then
  case "$rel" in
    *.acdsl|acdsl/*)
      deny "acdsl policy mode is strict: rule files are not editable in this repo ($rel)" ;;
  esac
fi

exit 0
```

### <a id="c9"></a>8. Hook registration (modified)

location: `settings/claude_code/settings.json`

```diff
       {
         "matcher": "Write|Edit|NotebookEdit",
         "hooks": [
           {
             "type": "command",
             "command": "bash ~/.claude/hooks/worktree-write-guard.sh",
             "timeout": 10
           },
+          {
+            "type": "command",
+            "command": "bash ~/.claude/hooks/acdsl-policy-guard.sh",
+            "timeout": 10
+          }
         ]
       }
```

- `sync_hooks.sh` deploys `cmd/hooks/*.sh` wholesale — no sync-script change; verified in Verification.

### <a id="c10"></a>9. Docs (modified)

location: `acdsl/README.md`, `docs/acdsl-spec.md`

- **README "Modes" section:** the three modes, the base-branch privilege boundary, `dist_mode`, `local_overrides` as the sanctioned overlay list, the task-contract exemption, and a copy-paste `permissions.deny` snippet (`Edit(acdsl/**)`, `Write(acdsl/**)`, `Edit(**/*.acdsl)`, `Write(**/*.acdsl)`) for humans who want client-level strictness in a target.
- **README overlay paragraph:** amend the `registry.local.json` description — free-mode-only unless listed in `local_overrides`.
- **Spec:** one "Self-management modes" paragraph in [docs/acdsl-spec.md](docs/acdsl-spec.md) referencing the README.

## Hot items

- **CheckPolicy guard logic (ACTION-CONCEPT-HOT-005 — new gate/validation logic):** the approved example implementation is written out in full in [Changes §1](#c1) — `LoadPolicy`, `CheckPolicy`, and the helper contracts, including the fail-loud stances (unknown mode, unresolvable base) and the free-mode early return.
- **No new interfaces, generics, concurrency, SQL, anonymous structs, or UI** — remaining hot classes not touched.

## Tests

| Location.Method | Cases | Comment |
|---|---|---|
| policy_test.go TestLoadPolicy | pass-absent-file-free<br>pass-defaults-preserved (allow_task_contracts absent → true)<br>fail-unknown-mode<br>fail-malformed-json | table-driven per house schema |
| policy_test.go TestCheckPolicy | pass-free-never-diffs<br>pass-on-base-tip-vacuous<br>fail-strict-any-rule-added<br>fail-strict-surface-edit (registry.json)<br>pass-gated-editable-scope-add<br>fail-gated-protected-scope-add (PLAN)<br>fail-gated-protected-scope-remove<br>fail-gated-changed-marker-line<br>fail-gated-verifier-outside-allowlist<br>pass-gated-verifier-in-base-registry (empty allowlist)<br>fail-overlay-unsanctioned-entry<br>pass-overlay-sanctioned-entry<br>pass-task-contract-exempt<br>fail-task-contract-when-disallowed<br>fail-unresolvable-base-hard-error<br>fail-new-untracked-policy-on-branch | fixture: `git init` repos in `t.TempDir()` with a base branch and a working branch, mirroring the repo's existing git-backed test style |
| policy_test.go TestRuleDelta | pass-added-removed-changed-classified<br>pass-untracked-declaration-file-seen<br>pass-marker-reorder-counts-as-change | delta primitive isolated |
| git_test.go TestGitShowFile | pass-existing-path<br>pass-absent-path-ok-false | new helper edge |
| dist_test.go TestDist (extended case) | pass-policy-shipped-mode-rewritten<br>pass-no-source-policy-ships-nothing | dist_mode → mode, dist_mode stripped |
| cmd/tests (shell) | not added | hook covered by manual verification below; shellcheck + bash32 gates cover static quality |

- Not tested: settings.json registration (no test harness for hook wiring — covered by Verification), README prose.

## Test runbook

Scenario index (CLI surface — no HTTP endpoints, Bruno N/A; smoke scenarios run as shell commands):

- **gated-target-blocks-plan-rule:** temp repo, `acdsl dist` from smine, append a `ACDSL-PLAN-9xx` line to target `rules.acdsl`, `bin/acdsl check` → exit 1 naming the scope.
- **gated-target-allows-golang-rule:** same repo, append a GOLANG-scope rule binding `gofmt`, `bin/acdsl check` → policy-clean.
- **strict-target-blocks-everything:** flip shipped policy mode to strict on the base, branch, touch any rule → exit 1.
- **overlay-bypass-closed:** add `registry.local.json` re-pointing `gofmt` at `true`, check → exit 1 unsanctioned overlay entry.
- **hook-denies-surface-write:** pipe a synthetic PreToolUse JSON for `acdsl/registry.json` into `acdsl-policy-guard.sh` → deny JSON on stdout.

## Contracts & sweeps

| Contract | Sides | Sweep |
|---|---|---|
| `acdsl/policy.json` shape | Go loader (`LoadPolicy`), schema file, hook (`jq .mode`), dist rewrite | grep `policy.json` across cmd/, internal/, cmd/hooks/, docs — all four read the same keys |
| Registry overlay semantics | `LoadRegistry` merge, `overlayViolations`, `DistHeader` text, README | grep `registry.local.json` to zero undocumented mentions |
| Diagnostic record shape | `CheckPolicy` producer, `runCheck` printer (text+JSON), verdict log consumer | policy diagnostics carry all four fields; `verdicts` aggregation keys on `ACDSL-POLICY` cleanly |
| Hook I/O | settings.json registration, `sync_hooks.sh` deployment, Claude Code PreToolUse JSON | deployed name matches registration; deny JSON shape identical to worktree-write-guard |

## Verification

- [ ] Run `make audit` — expect green: vet, strip, check (policy gate active, smine mode `free`), fixtures incl. ACDSL-JSON-003, tests.
- [ ] Run `go test ./internal/acdsl/ -run 'TestLoadPolicy|TestCheckPolicy|TestRuleDelta'` — expect all cases green.
- [ ] Create a scratch git repo with a `main` branch, run `./bin/acdsl dist -target scratch -dest <dir>` from smine — expect output line `acdsl/policy.json -> mode: gated` and the file present with `dist_mode` absent.
- [ ] In the scratch repo on a feature branch: append `//acdsl:ACDSL-PLAN-901 gofmt anchor="\.go$" why="x"` to `acdsl/rules.acdsl`, run `bin/acdsl check` — expect exit 1 with `policy(gated): rule ACDSL-PLAN-901 added` naming scope PLAN.
- [ ] Same branch: append a `ACDSL-GOLANG-901` rule binding `gofmt` — expect `check` policy-clean (only that rule's own verifier verdict applies).
- [ ] Same branch: add `acdsl/registry.local.json` with any entry — expect exit 1 unsanctioned overlay entry.
- [ ] Degenerate: delete `acdsl/policy.json` in the scratch repo — expect `check` to behave exactly as before this feature (free).
- [ ] Degenerate: scratch repo with no `main`/`master`/remote and mode gated — expect exit 2 with the base-ref remediation message.
- [ ] Hook: `echo '{"cwd":"<scratch>","tool_input":{"file_path":"acdsl/registry.json"}}' | bash cmd/hooks/acdsl-policy-guard.sh` — expect deny JSON; same for a non-surface path — expect silent exit 0.
- [ ] Confirm `cmd/sync/sync_hooks.sh` picks up the new hook file (dry run or inspect its copy glob) — expect `acdsl-policy-guard.sh` deployed to `~/.claude/hooks/`.

## Stop conditions

| ID | Condition | Action |
|---|---|---|
| S1 | An approved signature or contract in this plan cannot hold as written | Stop, report, no mid-edit architecture (ACTION-IMPL-001) |
| S2 | Second failed approach in a row on any unit | Stop, re-read disk state, write a plan (ACTION-IMPL-002) |
| S3 | Missing prerequisite (unbuilt bin/acdsl, absent fixture dir) | Run the producing step; if infra is down, ask (ACTION-IMPL-003) |
| S4 | Discovered work materially exceeds this plan | Ask before continuing (ACTION-IMPL-004) |
| S5 | Same bug class found twice | Fix all in-diff instances; pre-existing → report and ask (ACTION-IMPL-005) |
| S6 | Structural obstacle (import cycle) tempts a new abstraction | Stop and report; relocate, don't indirect (ACTION-IMPL-006) |
| S7 | `ParseRules` or `markerLine` behavior differs from the contracts assumed here (base-side parsing breaks) | Stop — the delta design rests on them; report before working around |
| S8 | A non-free `acdsl/policy.json` already exists on this branch during implementation (self-blocking) | Stop and report — the bootstrap assumes smine ships `mode: "free"` |

## Open questions

- None — Q1 resolved to [D11](#d11) `[USER]`.

## Changelog

| Date | Trigger | What changed |
|---|---|---|
| — | initial | plan created |
| 2026-08-26 | Q: initial policy values | D11 answered: option a (smine free, targets gated, language scopes editable) |
