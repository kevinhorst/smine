package skillgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
)

// Check kinds of an enforce declaration.
const (
	CheckPhase      = "phase"
	CheckProvenance = "provenance"
	CheckRequires   = "requires"
	CheckStop       = "stop"
)

// Anchor agents: whose calls an anchor matches; an empty Agent means root.
const (
	AnchorAgentAny      = "any"
	AnchorAgentRoot     = "root"
	AnchorAgentSubagent = "subagent"
)

// Roles an anchor plays in a check: the check field naming it.
const (
	anchorRoleAfter   = "after"
	anchorRoleDeny    = "deny"
	anchorRoleFrom    = "from"
	anchorRoleOn      = "on"
	anchorRoleRequire = "require"
	anchorRoleSince   = "since"
	anchorRoleUntil   = "until"
)

// maxAnchors keeps the whole declaration reviewable.
const maxAnchors = 16

// enforceSchema is the schema number of a rendered enforce.json.
const enforceSchema = 1

type Enforce struct {
	Anchors  map[string]*Anchor       `json:"anchors,omitempty"`
	Checks   map[string]*EnforceCheck `json:"checks,omitempty"`
	Gate     *GateRules               `json:"gate,omitempty"`
	Judgment map[string]*Judgment     `json:"judgment,omitempty"`
}

// GateRules is the stateless part the tool-policy gate enforces.
type GateRules struct {
	PlanModeDenied []string `json:"planModeDenied,omitempty"`
	RootDenied     []string `json:"rootDenied,omitempty"`
	WorkflowOnly   []string `json:"workflowOnly,omitempty"`
	Writers        []Writer `json:"writers,omitempty"`
}

// Writer names the agent types allowed to write files matching Glob.
type Writer struct {
	AgentTypes []string `json:"agentTypes"`
	Glob       string   `json:"glob"`
}

// InputMatch tests a non-string field value against its compact JSON encoding.
type InputMatch struct {
	Field   string `json:"field"`
	Pattern string `json:"pattern"`
}

type Anchor struct {
	Agent   string       `json:"agent,omitempty"`
	Capture []Capture    `json:"capture,omitempty"`
	Exclude []InputMatch `json:"exclude,omitempty"`
	Input   []InputMatch `json:"input,omitempty"`
	Tool    string       `json:"tool"`
}

// Capture's Pointer is an RFC 6901 pointer into a successful call's response.
type Capture struct {
	Name    string `json:"name"`
	Pointer string `json:"pointer"`
}

// AnchorRef is one anchor a check names and the role it plays there.
type AnchorRef struct {
	Anchor string
	Role   string
}

// EnforceCheck's Kind selects the anchor fields that apply.
type EnforceCheck struct {
	After    string `json:"after,omitempty"`
	Arg      string `json:"arg,omitempty"`
	Capture  string `json:"capture,omitempty"`
	Deny     string `json:"deny,omitempty"`
	From     string `json:"from,omitempty"`
	Kind     string `json:"kind"`
	On       string `json:"on,omitempty"`
	Reason   string `json:"reason"`
	Recovery string `json:"recovery"`
	Require  string `json:"require,omitempty"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
}

type Judgment struct {
	Anchor   string   `json:"anchor"`
	Evidence []string `json:"evidence"`
	Question string   `json:"question"`
	Send     string   `json:"send"`
}

type EnforceDocument struct {
	*Enforce

	Schema  int    `json:"schema"`
	Skill   string `json:"skill"`
	Version string `json:"version"`
}

func (c *EnforceCheck) AnchorRefs() []AnchorRef {
	switch c.Kind {
	case CheckPhase:
		return []AnchorRef{{Anchor: c.Deny, Role: anchorRoleDeny}, {Anchor: c.From, Role: anchorRoleFrom}, {Anchor: c.Until, Role: anchorRoleUntil}}
	case CheckProvenance:
		return []AnchorRef{{Anchor: c.On, Role: anchorRoleOn}, {Anchor: c.From, Role: anchorRoleFrom}}
	case CheckRequires:
		refs := []AnchorRef{{Anchor: c.On, Role: anchorRoleOn}, {Anchor: c.After, Role: anchorRoleAfter}}
		if c.Since != "" {
			since := AnchorRef{Anchor: c.Since, Role: anchorRoleSince}
			refs = append(refs, since)
		}
		return refs
	case CheckStop:
		return []AnchorRef{{Anchor: c.Require, Role: anchorRoleRequire}, {Anchor: c.After, Role: anchorRoleAfter}}
	}
	return nil
}

// captures reports whether the anchor records a capture named name.
func (a *Anchor) captures(name string) bool {
	return slices.ContainsFunc(a.Capture, func(capture Capture) bool {
		return capture.Name == name
	})
}

// filled is a copy of the anchor with the tool, input and exclude patterns filled.
func (a *Anchor) filled(values map[string]string) *Anchor {
	fill := func(matches []InputMatch) []InputMatch {
		if matches == nil {
			return nil
		}

		filledMatches := make([]InputMatch, 0, len(matches))
		for _, match := range matches {
			filledMatches = append(filledMatches, InputMatch{Field: match.Field, Pattern: FillPattern(match.Pattern, values)})
		}
		return filledMatches
	}

	view := *a
	view.Tool = FillPattern(a.Tool, values)
	view.Input = fill(a.Input)
	view.Exclude = fill(a.Exclude)
	return &view
}

// hasUnsetReference reports whether the tool, an input or an exclude pattern reads an unset variable.
func (a *Anchor) hasUnsetReference(values map[string]string) bool {
	patterns := slices.Concat(a.Input, a.Exclude)
	isUnset := func(match InputMatch) bool { return hasUnsetReference(match.Pattern, values) }
	return hasUnsetReference(a.Tool, values) || slices.ContainsFunc(patterns, isUnset)
}

// patternViolations compiles the tool pattern and every input pattern.
func (a *Anchor) patternViolations(subject string) []string {
	patterns := []InputMatch{{Field: "tool", Pattern: a.Tool}}
	for _, match := range slices.Concat(a.Input, a.Exclude) {
		patterns = append(patterns, InputMatch{Field: "input." + match.Field, Pattern: match.Pattern})
	}

	var violations []string
	for _, pattern := range patterns {
		if _, err := regexp.Compile(variableRefPattern.ReplaceAllString(pattern.Pattern, "x")); err != nil {
			violations = append(violations, fmt.Sprintf("%s: %s %q does not compile: %v", subject, pattern.Field, pattern.Pattern, err))
		}
	}
	return violations
}

// checkEnforce validates only what the schema cannot; a capture nobody reads is the vocabulary growing back.
func checkEnforce(skill *Skill) []string {
	enforce := skill.Enforce
	if enforce == nil {
		return nil
	}

	var violations []string
	if len(enforce.Anchors) > maxAnchors {
		violations = append(violations, fmt.Sprintf("enforce: %d anchors, at most %d", len(enforce.Anchors), maxAnchors))
	}

	referenced := map[string]bool{}
	resolve := func(subject, anchorId string) {
		referenced[anchorId] = true
		if _, isKnown := enforce.Anchors[anchorId]; !isKnown {
			violations = append(violations, fmt.Sprintf("enforce: %s: unknown anchor %s", subject, anchorId))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(enforce.Checks)) {
		check := enforce.Checks[id]
		subject := "check " + id
		for _, ref := range check.AnchorRefs() {
			resolve(subject, ref.Anchor)
		}
		from, isKnown := enforce.Anchors[check.From]
		isMissingCapture := check.Kind == CheckProvenance && isKnown && !from.captures(check.Capture)
		if isMissingCapture {
			violations = append(violations, fmt.Sprintf("enforce: %s: anchor %s captures no %s", subject, check.From, check.Capture))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(enforce.Judgment)) {
		resolve("judgment "+id, enforce.Judgment[id].Anchor)
	}

	readCaptures := map[string]bool{}
	for _, check := range enforce.Checks {
		if check.Kind == CheckProvenance {
			readCaptures[check.From+"."+check.Capture] = true
		}
	}
	for _, judgment := range enforce.Judgment {
		for _, evidence := range judgment.Evidence {
			readCaptures[judgment.Anchor+"."+evidence] = true
		}
	}
	for key := range RuntimeReaders(skill) {
		readCaptures[key] = true
	}
	for _, id := range slices.Sorted(maps.Keys(enforce.Anchors)) {
		if !referenced[id] {
			violations = append(violations, fmt.Sprintf("enforce: anchor %s: referenced by no check or judgment", id))
		}
		for _, capture := range enforce.Anchors[id].Capture {
			if !readCaptures[id+"."+capture.Name] {
				violations = append(violations, fmt.Sprintf("enforce: anchor %s capture %s: read by no check, judgment or atom", id, capture.Name))
			}
		}
		violations = append(violations, enforce.Anchors[id].patternViolations("enforce: anchor "+id)...)
	}
	return violations
}

// enforceView drops the anchors reading an unset variable with the checks and judgments naming them, and fills every kept pattern.
func enforceView(enforce *Enforce, values map[string]string) *Enforce {
	if enforce == nil {
		return nil
	}

	view := *enforce
	view.Anchors = make(map[string]*Anchor, len(enforce.Anchors))
	for id, anchor := range enforce.Anchors {
		if !anchor.hasUnsetReference(values) {
			view.Anchors[id] = anchor.filled(values)
		}
	}

	isKept := func(anchorId string) bool {
		_, isKnown := view.Anchors[anchorId]
		return isKnown || enforce.Anchors[anchorId] == nil
	}
	view.Checks = make(map[string]*EnforceCheck, len(enforce.Checks))
	for id, check := range enforce.Checks {
		refs := check.AnchorRefs()
		isAnchorDropped := slices.ContainsFunc(refs, func(ref AnchorRef) bool { return ref.Anchor != "" && !isKept(ref.Anchor) })
		isTextUnset := hasUnsetReference(check.Reason+"\n"+check.Recovery, values)
		if !isAnchorDropped && !isTextUnset {
			view.Checks[id] = check
		}
	}
	view.Judgment = make(map[string]*Judgment, len(enforce.Judgment))
	for id, judgment := range enforce.Judgment {
		isTextUnset := hasUnsetReference(judgment.Question+"\n"+judgment.Send, values)
		if isKept(judgment.Anchor) && !isTextUnset {
			view.Judgment[id] = judgment
		}
	}
	return &view
}

// renderEnforce encodes the skill's enforce.json.
func renderEnforce(skill *Skill) ([]byte, error) {
	document := &EnforceDocument{Enforce: skill.Enforce, Schema: enforceSchema, Skill: skill.Name(), Version: skill.Version()}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("renderEnforce: %w", err)
	}
	return append(encoded, '\n'), nil
}
