// Package acdsl implements the agentic context DSL: rules declared as
// one-line //acdsl: markers (centrally in acdsl/rules.acdsl, task contracts
// in *.acdsl files, inline in Go source as the single-file exception),
// anchors as path regexps over the repo's files, and checks delegated to
// registered verifiers (exit 0 = pass).
package acdsl

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// Marker is the rule-declaration prefix in the //-comment file classes — Go
// source and standalone .acdsl files. Other file classes declare with their
// own comment prefix (declarationMarker).
const Marker = "//acdsl:"

// Rule is one declared doctrine rule — data only, never code.
type Rule struct {
	Id         string            // stable, unique per module
	Verifier   string            // registry name, must exist
	Anchor     string            // RE2 over repo-relative paths, required
	AnchorNot  string            // optional exclude RE2
	AllowEmpty bool              // empty="ok": zero matches pass (transient artifacts); default false — tool breakage
	Needs      []string          // needs="p1,p2": repo-relative artifacts the verifier reads; any absent skips the rule in check and fixtures (private artifacts on a public tree)
	Why        string            // doctrine sentence, rendered + cited on failure
	Lifetime   string            // "doctrine" (default) or "task" — contract entries
	Reach      string            // internal/reach grammar: "global" | repo-name list; default "smine" (this repo)
	Projected  bool              // delivery flag: false = gate-only, prose leaves every prompt-side channel
	Params     map[string]string // verifier-specific key=value
	File       string            // declaration site (repo-relative)
	Line       int
}

// Lifetime values. A task entry lives for one job and is archived with it,
// never synced as pack content. Lifetime is plain data — the tool attaches
// no meaning to where an entry is declared.
const (
	LifetimeDoctrine = "doctrine"
	LifetimeTask     = "task"
)

var fieldRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*)=("(?:[^"\\]|\\.)*")`)

// ParseRules scans fileLines (repo-relative path → lines) for markers.
// Authoring problems are returned as violations, not errors — mirrors
// contextdocs.ValidateRules so the CLI prints one line per problem.
func ParseRules(fileLines map[string][]string) ([]Rule, []string) {
	var rules []Rule
	var violations []string
	seen := map[string]string{}

	paths := make([]string, 0, len(fileLines))
	for path := range fileLines {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		marker, terminator, ok := declarationMarker(path)
		if !ok {
			continue
		}
		for i, line := range fileLines[path] {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, marker) {
				continue
			}
			rest := strings.TrimPrefix(trimmed, marker)
			if terminator != "" {
				rest = strings.TrimSuffix(strings.TrimSpace(rest), terminator)
			}
			rule, problems := parseMarker(rest, path, i+1)
			if len(problems) > 0 {
				violations = append(violations, problems...)
				continue
			}
			if prev, dup := seen[rule.Id]; dup {
				violations = append(violations, fmt.Sprintf("%s:%d: duplicate rule id %s (first at %s)", path, i+1, rule.Id, prev))
				continue
			}
			seen[rule.Id] = fmt.Sprintf("%s:%d", path, i+1)
			rules = append(rules, rule)
		}
	}
	return rules, violations
}

// declarationMarker resolves a path's rule-marker form: the file class's
// trimmed comment prefix + "acdsl:", plus the class's trimmed terminator for
// block-comment syntaxes. .acdsl files carry the language's own //acdsl: form.
func declarationMarker(path string) (string, string, bool) {
	if strings.EqualFold(filepath.Ext(path), ".acdsl") {
		return Marker, "", true
	}
	syntax, ok := syntaxForPath(path)
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(syntax.linePrefix) + "acdsl:", strings.TrimSpace(syntax.lineSuffix), true
}

// parseMarker parses one declaration's rest — the marker line with its
// class's marker prefix and terminator already stripped.
func parseMarker(rest, path string, lineNo int) (Rule, []string) {
	at := func(msg string) string { return fmt.Sprintf("%s:%d: %s", path, lineNo, msg) }

	head := strings.Fields(rest)
	if len(head) < 2 {
		return Rule{}, []string{at("marker needs <ID> <verifier> [fields]")}
	}
	rule := Rule{Id: head[0], Verifier: head[1], Projected: true, Params: map[string]string{}, File: path, Line: lineNo}

	var violations []string
	for _, match := range fieldRe.FindAllStringSubmatch(rest, -1) {
		value := unquoteField(match[2])
		switch match[1] {
		case "anchor":
			rule.Anchor = value
		case "anchor-not":
			rule.AnchorNot = value
		case "empty":
			switch value {
			case "ok":
				rule.AllowEmpty = true
			default:
				violations = append(violations, at(rule.Id+`: empty must be "ok" when given`))
			}
		case "needs":
			for _, path := range strings.Split(value, ",") {
				if trimmedPath := strings.TrimSpace(path); trimmedPath != "" {
					rule.Needs = append(rule.Needs, trimmedPath)
				}
			}
			if len(rule.Needs) == 0 {
				violations = append(violations, at(rule.Id+": needs must name at least one path when given"))
			}
		case "why":
			rule.Why = value
		case "lifetime":
			rule.Lifetime = value
		case "reach":
			rule.Reach = value
		case "projected":
			switch value {
			case "true":
				rule.Projected = true
			case "false":
				rule.Projected = false
			default:
				violations = append(violations, at(rule.Id+`: projected must be "true" or "false"`))
			}
		default:
			rule.Params[match[1]] = value
		}
	}

	if rule.Anchor == "" {
		violations = append(violations, at(rule.Id+": anchor is required"))
	}
	if rule.Why == "" {
		violations = append(violations, at(rule.Id+": why is required"))
	}
	switch rule.Lifetime {
	case "":
		rule.Lifetime = LifetimeDoctrine
	case LifetimeDoctrine, LifetimeTask:
	default:
		violations = append(violations, at(rule.Id+": lifetime must be doctrine or task"))
	}
	if rule.Reach == "" {
		rule.Reach = reach.ThisRepo
	} else if !reach.Valid(rule.Reach) {
		violations = append(violations, at(rule.Id+": reach must be global, none, or a comma-separated repo-name list"))
	}
	return rule, violations
}

// FilterLifetime narrows rules to the selected lifetime: "doctrine", "task",
// or "all". The audit gates doctrine by default; the task gate is explicit.
func FilterLifetime(rules []Rule, selector string) ([]Rule, error) {
	if selector == "all" {
		return rules, nil
	}
	if selector != LifetimeDoctrine && selector != LifetimeTask {
		return nil, fmt.Errorf("FilterLifetime: unknown selector %q (doctrine|task|all)", selector)
	}
	var filtered []Rule
	for _, rule := range rules {
		if rule.Lifetime == selector {
			filtered = append(filtered, rule)
		}
	}
	return filtered, nil
}

// FilterId narrows rules to one id across all lifetimes — the generation
// dry-run gates a freshly authored rule repo-wide before it is accepted.
func FilterId(rules []Rule, id string) ([]Rule, error) {
	for _, rule := range rules {
		if rule.Id == id {
			return []Rule{rule}, nil
		}
	}
	return nil, fmt.Errorf("FilterId: no rule with id %q", id)
}

// unquoteField strips the surrounding quotes and resolves only \" and \\ —
// every other backslash stays literal, so regexp values like \.go$ survive.
// strconv.Unquote would reject them as invalid Go escapes.
func unquoteField(quoted string) string {
	inner := quoted[1 : len(quoted)-1]
	var builder strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) && (inner[i+1] == '"' || inner[i+1] == '\\') {
			builder.WriteByte(inner[i+1])
			i++
			continue
		}
		builder.WriteByte(inner[i])
	}
	return builder.String()
}
