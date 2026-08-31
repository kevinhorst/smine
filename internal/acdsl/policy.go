package acdsl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// PolicyRuleId labels policy diagnostics in check output and the verdict log.
const PolicyRuleId = "ACDSL-POLICY"

// Policy mode values — the self-management stance of a repo's rule system.
const (
	PolicyModeFree   PolicyMode = "free"
	PolicyModeGated  PolicyMode = "gated"
	PolicyModeStrict PolicyMode = "strict"
)

// policyIdRe mirrors the id-grammar shape (capture 2 = SCOPE segment). The
// verifier keeps its own copy — it must stay standalone.
var policyIdRe = regexp.MustCompile(`^(ACDSL|RULE|FACT|ACTION)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-(\d{3})$`)

// policySurface is the frozen self-management surface: in strict and gated
// mode none of these files may differ from the policy base.
var policySurface = []string{
	"acdsl/policy.json",
	"acdsl/policy.schema.json",
	"acdsl/registry.json",
	"acdsl/evalgen.json",
}

// Policy is acdsl/policy.json: who may change the rule system, and how far.
type Policy struct {
	AllowTaskContracts bool       `json:"allow_task_contracts"`
	BaseRef            string     `json:"base_ref,omitempty"`
	DistMode           PolicyMode `json:"dist_mode,omitempty"`
	EditableScopes     []string   `json:"editable_scopes,omitempty"`
	LocalOverrides     []string   `json:"local_overrides,omitempty"`
	Mode               PolicyMode `json:"mode"`
	VerifierAllowlist  []string   `json:"verifier_allowlist,omitempty"`
}

// PolicyMode is the self-management stance of a repo's rule system.
type PolicyMode string

// policyRuleDelta is the rule-level difference between the working tree and
// the policy base, keyed by declaration marker line identity.
type policyRuleDelta struct {
	added   []Rule
	changed []Rule
	removed []Rule
}

// basePolicy reads the authoritative policy from the base ref; a base
// without one falls back to the working policy (the bootstrap branch that
// introduces it). An unparseable base policy is a hard error.
func basePolicy(ctx context.Context, root, base string, working Policy) (Policy, error) {
	content, exists, err := GitShowFile(ctx, root, base, "acdsl/policy.json")
	if err != nil {
		return Policy{}, fmt.Errorf("basePolicy: %w", err)
	}
	if !exists {
		return working, nil
	}
	policy, err := parsePolicy([]byte(content))
	if err != nil {
		return Policy{}, fmt.Errorf("basePolicy: acdsl/policy.json at %s: %w", base, err)
	}
	return policy, nil
}

// classViolations judges one delta class against the mode: strict flags
// every entry; gated flags entries outside the editable scopes and — for
// entries that (re)bind a verifier — outside the sanctioned verifier set.
// Task-lifetime entries are exempt while AllowTaskContracts holds.
func classViolations(policy Policy, allowlist map[string]bool, rules []Rule, action string, bindsVerifier bool) []Diagnostic {
	editable := map[string]bool{}
	for _, scope := range policy.EditableScopes {
		editable[scope] = true
	}
	var diagnostics []Diagnostic
	for _, rule := range rules {
		if policy.AllowTaskContracts && rule.Lifetime == LifetimeTask {
			continue
		}
		if policy.Mode == PolicyModeStrict {
			diagnostics = append(diagnostics, policyDiagnostic(rule.File, rule.Line, fmt.Sprintf("policy(strict): rule %s %s — no rule is editable in this repo", rule.Id, action)))
			continue
		}
		scope := ""
		if match := policyIdRe.FindStringSubmatch(rule.Id); match != nil {
			scope = match[2]
		}
		if !editable[scope] {
			diagnostics = append(diagnostics, policyDiagnostic(rule.File, rule.Line, fmt.Sprintf("policy(gated): rule %s %s outside the editable scopes (scope %q)", rule.Id, action, scope)))
			continue
		}
		if bindsVerifier && !allowlist[rule.Verifier] {
			diagnostics = append(diagnostics, policyDiagnostic(rule.File, rule.Line, fmt.Sprintf("policy(gated): rule %s binds verifier %q outside the sanctioned set", rule.Id, rule.Verifier)))
		}
	}
	return diagnostics
}

// effectiveAllowlist resolves the gated verifier set: the explicit
// verifier_allowlist when given, else every name in the base-ref registry —
// "the shipped set" without listing it twice.
func effectiveAllowlist(ctx context.Context, root, base string, policy Policy) (map[string]bool, error) {
	allowlist := map[string]bool{}
	if len(policy.VerifierAllowlist) > 0 {
		for _, name := range policy.VerifierAllowlist {
			allowlist[name] = true
		}
		return allowlist, nil
	}
	content, exists, err := GitShowFile(ctx, root, base, "acdsl/registry.json")
	if err != nil {
		return nil, fmt.Errorf("effectiveAllowlist: %w", err)
	}
	if !exists {
		return allowlist, nil
	}
	var registry map[string]RegistryEntry
	if err := json.Unmarshal([]byte(content), &registry); err != nil {
		return nil, fmt.Errorf("effectiveAllowlist: acdsl/registry.json at %s: %w", base, err)
	}
	for name := range registry {
		allowlist[name] = true
	}
	return allowlist, nil
}

// overlayViolations reads acdsl/registry.local.json directly (tracked or
// not) and flags every entry name missing from policy.LocalOverrides — the
// overlay wins by name at load time, so an unguarded entry re-points any
// verifier.
func overlayViolations(root string, policy Policy) ([]Diagnostic, error) {
	path := filepath.Join(root, "acdsl", LocalRegistryName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("overlayViolations: %w", err)
	}
	var overlay map[string]RegistryEntry
	if err := json.Unmarshal(raw, &overlay); err != nil {
		return nil, fmt.Errorf("overlayViolations: %s: %w", path, err)
	}
	sanctioned := map[string]bool{}
	for _, name := range policy.LocalOverrides {
		sanctioned[name] = true
	}
	names := make([]string, 0, len(overlay))
	for name := range overlay {
		names = append(names, name)
	}
	sort.Strings(names)
	var diagnostics []Diagnostic
	for _, name := range names {
		if sanctioned[name] {
			continue
		}
		diagnostics = append(diagnostics, policyDiagnostic("acdsl/"+LocalRegistryName, 1, fmt.Sprintf("policy(%s): overlay entry %q is not sanctioned in local_overrides", policy.Mode, name)))
	}
	return diagnostics, nil
}

// parsePolicy decodes one policy document over the defaults (task contracts
// allowed, free mode); an unknown mode is an error, never a silent free.
func parsePolicy(raw []byte) (Policy, error) {
	policy := Policy{AllowTaskContracts: true, Mode: PolicyModeFree}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("parsePolicy: %w", err)
	}
	switch policy.Mode {
	case PolicyModeFree, PolicyModeGated, PolicyModeStrict:
		return policy, nil
	}
	return Policy{}, fmt.Errorf("parsePolicy: unknown mode %q", policy.Mode)
}

// policyBase resolves merge-base(HEAD, base ref). isBoundary is false only
// when the checked-out branch IS the base branch — where humans change the
// surface; a fresh agent branch at the base tip is a boundary, its
// uncommitted edits diff against that tip. Candidates: BaseRef when set,
// else origin/HEAD, origin/main, main, master; none resolving is a hard
// error, never a silent free.
func policyBase(ctx context.Context, root string, policy Policy) (string, bool, error) {
	candidates := []string{"origin/HEAD", "origin/main", "main", "master"}
	if policy.BaseRef != "" {
		candidates = []string{policy.BaseRef}
	}
	for _, candidate := range candidates {
		base, err := GitMergeBase(ctx, root, candidate)
		if err != nil {
			continue
		}
		branch, err := GitBranch(ctx, root)
		if err != nil {
			return "", false, fmt.Errorf("policyBase: %w", err)
		}
		baseName := candidate
		if candidate == "origin/HEAD" {
			if resolved, resolveErr := shellGitAbbrevRef(ctx, root, candidate); resolveErr == nil {
				baseName = resolved
			}
		}
		baseName = strings.TrimPrefix(baseName, "origin/")
		return base, branch != baseName, nil
	}
	return "", false, fmt.Errorf("policyBase: no policy base resolves (tried %s) — set base_ref in acdsl/policy.json or fetch the base branch", strings.Join(candidates, ", "))
}

// policyDiagnostic renders one policy finding in the verifier diagnostic
// shape so the check printer, JSON mode, and verdict log treat it uniformly.
func policyDiagnostic(file string, line int, detail string) Diagnostic {
	diagnostic := Diagnostic{
		RuleId:   PolicyRuleId,
		Verifier: "policy",
		Message:  fmt.Sprintf("%s:%d: %s", file, line, detail),
		Why:      "acdsl/policy.json governs who may change the rule system — sanction the change on the policy base branch, or revert",
	}
	return diagnostic
}

// ruleDelta parses declaration-capable files changed vs base on both sides
// and classifies rules as added, removed, or changed — "changed" meaning the
// trimmed marker line differs. Base-side authoring violations are ignored
// (the base was committed green).
func ruleDelta(ctx context.Context, root, base string) (policyRuleDelta, error) {
	changedPaths, err := GitChangedFiles(ctx, root, base)
	if err != nil {
		return policyRuleDelta{}, fmt.Errorf("ruleDelta: %w", err)
	}
	workLines := map[string][]string{}
	baseLines := map[string][]string{}
	for _, path := range changedPaths {
		if _, _, declares := declarationMarker(path); !declares {
			continue
		}
		if raw, readErr := os.ReadFile(filepath.Join(root, path)); readErr == nil {
			workLines[path] = strings.Split(string(raw), "\n")
		}
		content, exists, showErr := GitShowFile(ctx, root, base, path)
		if showErr != nil {
			return policyRuleDelta{}, fmt.Errorf("ruleDelta: %w", showErr)
		}
		if exists {
			baseLines[path] = strings.Split(content, "\n")
		}
	}
	workRules, _ := ParseRules(workLines)
	baseRules, _ := ParseRules(baseLines)

	workMarkers := ruleMarkers(workLines, workRules)
	baseMarkers := ruleMarkers(baseLines, baseRules)
	var delta policyRuleDelta
	for _, rule := range workRules {
		baseMarker, existed := baseMarkers[rule.Id]
		switch {
		case !existed:
			delta.added = append(delta.added, rule)
		case baseMarker != workMarkers[rule.Id]:
			delta.changed = append(delta.changed, rule)
		}
	}
	for _, rule := range baseRules {
		if _, still := workMarkers[rule.Id]; !still {
			delta.removed = append(delta.removed, rule)
		}
	}
	return delta, nil
}

// ruleMarkers indexes each rule id to its trimmed declaration line — the
// byte-level identity the delta compares (a field reorder counts as a
// change, deliberately).
func ruleMarkers(fileLines map[string][]string, rules []Rule) map[string]string {
	markers := map[string]string{}
	for _, rule := range rules {
		lines := fileLines[rule.File]
		if rule.Line < 1 || rule.Line > len(lines) {
			continue
		}
		markers[rule.Id] = strings.TrimSpace(lines[rule.Line-1])
	}
	return markers
}

// surfaceDelta returns one diagnostic per policySurface file whose working
// content differs from the base — absent on one side counts as a
// difference, absent on both is clean.
func surfaceDelta(ctx context.Context, root, base string, policy Policy) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	for _, path := range policySurface {
		baseContent, existedAtBase, err := GitShowFile(ctx, root, base, path)
		if err != nil {
			return nil, fmt.Errorf("surfaceDelta: %w", err)
		}
		raw, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return nil, fmt.Errorf("surfaceDelta: %s: %w", path, readErr)
		}
		existsNow := readErr == nil
		if !existedAtBase && !existsNow {
			continue
		}
		if existedAtBase && existsNow && string(raw) == baseContent {
			continue
		}
		diagnostics = append(diagnostics, policyDiagnostic(path, 1, fmt.Sprintf("policy(%s): %s differs from the policy base — the self-management surface changes only on the base branch", policy.Mode, path)))
	}
	return diagnostics, nil
}

// CheckPolicy is the mode gate: it diffs the self-management surface and the
// declared rules against the policy base and returns one diagnostic per
// violation. The base-ref policy is authoritative — a branch cannot weaken
// the mode by editing or deleting the working policy.json; the working
// policy only bootstraps base resolution. Free mode never diffs; on the base
// branch the diff is vacuous.
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, root, policy)
	if baseErr != nil {
		if policy.Mode == PolicyModeFree {
			return nil, nil // no base to gate against, nothing declared — free
		}
		return nil, fmt.Errorf("CheckPolicy: %w", baseErr)
	}
	policy, err := basePolicy(ctx, root, base, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	if policy.Mode == PolicyModeFree {
		return nil, nil
	}
	if !isBoundary {
		return nil, nil // on the base branch — humans change the surface here
	}

	var diagnostics []Diagnostic
	surface, err := surfaceDelta(ctx, root, base, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, surface...)

	overlay, err := overlayViolations(root, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, overlay...)

	allowlist := map[string]bool{}
	if policy.Mode == PolicyModeGated {
		allowlist, err = effectiveAllowlist(ctx, root, base, policy)
		if err != nil {
			return nil, fmt.Errorf("CheckPolicy: %w", err)
		}
	}
	delta, err := ruleDelta(ctx, root, base)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}
	diagnostics = append(diagnostics, classViolations(policy, allowlist, delta.added, "added", true)...)
	diagnostics = append(diagnostics, classViolations(policy, allowlist, delta.changed, "modified", true)...)
	diagnostics = append(diagnostics, classViolations(policy, allowlist, delta.removed, "removed", false)...)
	return diagnostics, nil
}

// LoadPolicy reads acdsl/policy.json under root. An absent file is free mode
// (pre-policy behavior).
func LoadPolicy(root string) (Policy, error) {
	raw, err := os.ReadFile(filepath.Join(root, "acdsl", "policy.json"))
	if errors.Is(err, os.ErrNotExist) {
		policy := Policy{AllowTaskContracts: true, Mode: PolicyModeFree}
		return policy, nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("LoadPolicy: %w", err)
	}
	policy, err := parsePolicy(raw)
	if err != nil {
		return Policy{}, fmt.Errorf("LoadPolicy: acdsl/policy.json: %w", err)
	}
	return policy, nil
}
