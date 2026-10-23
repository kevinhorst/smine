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

// The id-grammar verifier keeps its own copy of this shape: it must stay standalone.
var policyIdRe = regexp.MustCompile(`^(ACDSL|RULE|FACT|ACTION)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-(\d{3})$`)

// The working policy's base_ref is never a candidate: a branch owns it.
var DefaultBaseCandidates = []string{"origin/HEAD", "origin/main", "main", "master"}

// In strict and gated mode none of these files may differ from the policy base.
var policySurface = []string{
	"acdsl/policy.json",
	"acdsl/policy.schema.json",
	"acdsl/registry.json",
	"acdsl/evalgen.json",
	"bin/acdsl",
	"bin/acdsl.exe",
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

// changedTask holds modified rules that are task-lifetime on both sides.
type policyRuleDelta struct {
	added       []Rule
	changed     []Rule
	changedTask []Rule
	removed     []Rule
}

func argvPaths(argv []string) []string {
	var paths []string
	for _, arg := range argv {
		isRelativePath := strings.Contains(arg, "/") && !filepath.IsAbs(arg)
		if isRelativePath {
			paths = append(paths, strings.TrimPrefix(arg, "./"))
		}
	}
	return paths
}

// A base without a policy falls back to the working one minus base_ref, honored from the base only.
func basePolicy(ctx context.Context, root, base string, working Policy) (Policy, error) {
	content, exists, err := GitShowFile(ctx, root, base, "acdsl/policy.json")
	if err != nil {
		return Policy{}, fmt.Errorf("basePolicy: %w", err)
	}
	if !exists {
		working.BaseRef = ""
		return working, nil
	}
	policy, err := parsePolicy([]byte(content))
	if err != nil {
		return Policy{}, fmt.Errorf("basePolicy: acdsl/policy.json at %s: %w", base, err)
	}
	return policy, nil
}

func baseRegistry(ctx context.Context, base, root string) (map[string]RegistryEntry, error) {
	registry := map[string]RegistryEntry{}
	content, exists, err := GitShowFile(ctx, root, base, "acdsl/registry.json")
	if err != nil {
		return nil, fmt.Errorf("baseRegistry: %w", err)
	}
	if !exists {
		return registry, nil
	}
	if err := json.Unmarshal([]byte(content), &registry); err != nil {
		return nil, fmt.Errorf("baseRegistry: acdsl/registry.json at %s: %w", base, err)
	}
	return registry, nil
}

// Task-contract exemption happens before, in CheckPolicy.
func classViolations(policy Policy, allowlist map[string]bool, rules []Rule, action string, bindsVerifier bool) []Diagnostic {
	editable := map[string]bool{}
	for _, scope := range policy.EditableScopes {
		editable[scope] = true
	}
	var diagnostics []Diagnostic
	for _, rule := range rules {
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

func effectiveAllowlist(policy Policy, registry map[string]RegistryEntry) map[string]bool {
	allowlist := map[string]bool{}
	if len(policy.VerifierAllowlist) > 0 {
		for _, name := range policy.VerifierAllowlist {
			allowlist[name] = true
		}
		return allowlist
	}
	for name := range registry {
		allowlist[name] = true
	}
	return allowlist
}

// The overlay wins by name at load time, so an unguarded entry re-points any verifier.
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

// An unknown mode is an error, never a silent free.
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

// isBoundary is false only on the base branch itself; a fresh agent branch at the base tip is a boundary.
func policyBase(ctx context.Context, candidates []string, root string) (string, bool, error) {
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
	return "", false, fmt.Errorf(
		"policyBase: No policy base resolves (tried %s) — fetch the base branch, or set base_ref in acdsl/policy.json on it",
		strings.Join(candidates, ", "),
	)
}

func policyDiagnostic(file string, line int, detail string) Diagnostic {
	diagnostic := Diagnostic{
		RuleId:   PolicyRuleId,
		Verifier: "policy",
		Message:  fmt.Sprintf("%s:%d: %s", file, line, detail),
		Why:      "acdsl/policy.json governs who may change the rule system — sanction the change on the policy base branch, or revert",
	}
	return diagnostic
}

// Base-side authoring violations are ignored: the base was committed green.
func ruleDelta(ctx context.Context, base string, changedPaths []string, root string) (policyRuleDelta, error) {
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
	baseById := map[string]Rule{}
	for _, rule := range baseRules {
		baseById[rule.Id] = rule
	}
	var delta policyRuleDelta
	for _, rule := range workRules {
		baseMarker, existed := baseMarkers[rule.Id]
		isTaskBothSides := rule.Lifetime == LifetimeTask && baseById[rule.Id].Lifetime == LifetimeTask
		switch {
		case !existed:
			delta.added = append(delta.added, rule)
		case baseMarker == workMarkers[rule.Id]:
			// unchanged
		case isTaskBothSides:
			delta.changedTask = append(delta.changedTask, rule)
		default:
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

// Byte-level identity: a field reorder counts as a change, deliberately.
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

// Membership comes from git's change list, so autocrlf line-ending differences never count.
func surfaceDelta(changedPaths []string, policy Policy, verifierRoots []string) []Diagnostic {
	frozen := map[string]bool{}
	for _, path := range policySurface {
		frozen[path] = true
	}
	var diagnostics []Diagnostic
	for _, path := range changedPaths {
		if frozen[path] {
			detail := fmt.Sprintf(
				"policy(%s): %s differs from the policy base — the self-management surface changes only on the base branch",
				policy.Mode,
				path,
			)
			diagnostics = append(diagnostics, policyDiagnostic(path, 1, detail))
			continue
		}
		root, isVerifierSource := verifierRootOf(path, verifierRoots)
		if isVerifierSource {
			detail := fmt.Sprintf(
				"policy(%s): %s is verifier source the base registry runs (%s) — it changes only on the base branch",
				policy.Mode,
				path,
				root,
			)
			diagnostics = append(diagnostics, policyDiagnostic(path, 1, detail))
		}
	}
	return diagnostics
}

// verifierRootOf reports the verifier root path lies under, if any.
func verifierRootOf(path string, roots []string) (string, bool) {
	for _, root := range roots {
		isUnderRoot := path == root || strings.HasPrefix(path, root+"/")
		if isUnderRoot {
			return root, true
		}
	}
	return "", false
}

func verifierRoots(registry map[string]RegistryEntry) []string {
	seen := map[string]bool{}
	for _, entry := range registry {
		for _, path := range argvPaths(entry.Argv) {
			seen[path] = true
		}
	}
	roots := make([]string, 0, len(seen))
	for root := range seen {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

// Base, policy and registry are read from the base; the working policy decides only when no base resolves.
func CheckPolicy(ctx context.Context, root string, policy Policy) ([]Diagnostic, error) {
	base, isBoundary, baseErr := policyBase(ctx, DefaultBaseCandidates, root)
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
	if policy.BaseRef != "" {
		baseRefCandidates := []string{policy.BaseRef}
		base, isBoundary, err = policyBase(ctx, baseRefCandidates, root)
		if err != nil {
			return nil, fmt.Errorf("CheckPolicy: %w", err)
		}

		policy, err = basePolicy(ctx, root, base, policy)
		if err != nil {
			return nil, fmt.Errorf("CheckPolicy: %w", err)
		}
	}
	if policy.Mode == PolicyModeFree {
		return nil, nil
	}
	if !isBoundary {
		return nil, nil // on the base branch — humans change the surface here
	}

	changedPaths, err := GitChangedFiles(ctx, root, base)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}

	registry, err := baseRegistry(ctx, base, root)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}

	diagnostics := surfaceDelta(changedPaths, policy, verifierRoots(registry))

	overlay, err := overlayViolations(root, policy)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}

	diagnostics = append(diagnostics, overlay...)

	allowlist := map[string]bool{}
	if policy.Mode == PolicyModeGated {
		allowlist = effectiveAllowlist(policy, registry)
	}
	delta, err := ruleDelta(ctx, base, changedPaths, root)
	if err != nil {
		return nil, fmt.Errorf("CheckPolicy: %w", err)
	}

	added, changed, removed := delta.added, delta.changed, delta.removed
	if policy.AllowTaskContracts {
		added, err = FilterLifetime(added, LifetimeDoctrine)
		if err != nil {
			return nil, fmt.Errorf("CheckPolicy: %w", err)
		}

		removed, err = FilterLifetime(removed, LifetimeDoctrine)
		if err != nil {
			return nil, fmt.Errorf("CheckPolicy: %w", err)
		}
	} else {
		changed = append(changed, delta.changedTask...)
	}
	diagnostics = append(diagnostics, classViolations(policy, allowlist, added, "added", true)...)
	diagnostics = append(diagnostics, classViolations(policy, allowlist, changed, "modified", true)...)
	diagnostics = append(diagnostics, classViolations(policy, allowlist, removed, "removed", false)...)
	return diagnostics, nil
}

func GuardedSurface() []string {
	guarded := append([]string{}, policySurface...)
	return append(guarded, "acdsl/"+LocalRegistryName)
}

// An absent file is free mode, the pre-policy behavior.
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
