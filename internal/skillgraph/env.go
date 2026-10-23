package skillgraph

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// variablePrefix opens every rendered variable name.
const variablePrefix = "SKILL_"

// Kinds of a text reading a variable.
const (
	ReadKindAnchor = "anchor"
	ReadKindCheck  = "check"
	ReadKindEntry  = "entry"
	ReadKindFlags  = "flags"
	ReadKindGrant  = "grant"
	ReadKindParam  = "param"
)

var (
	// commandTerminators are what may follow a command reference: a line end, a closing quote or fence, a parenthesis, a redirect, a pipe.
	commandTerminators = []string{"\n", "`", "'", "\"", ")", ">", "2>", "|"}
	// headTerminators end a program reference's head word: a space before its args, a closing fence or quote, a grant's closing parenthesis, an enforce pattern's group, a line end.
	headTerminators = []string{" ", "`", "'", "\"", "(", ")", "\n"}
	// valueTerminators close a bare program reference that passes the program itself as a value (`--acdsl '$SKILL_X_ACDSL_CHECK'`).
	valueTerminators = []string{"`", "'", "\"", ")"}
	// variableRefPattern matches a reference in any skill text: $SKILL_<SKILL>_<NAME>.
	variableRefPattern = regexp.MustCompile(`\$SKILL_[A-Z0-9]+_[A-Z][A-Z0-9_]*`)
)

// patternTexts returns every enforce anchor tool, input and exclude pattern; the model holds no title pattern yet.
func (s *Skill) patternTexts() []string {
	if s.Enforce == nil {
		return nil
	}

	var patterns []string
	anchorIds := slices.Sorted(maps.Keys(s.Enforce.Anchors))
	for _, id := range anchorIds {
		anchor := s.Enforce.Anchors[id]
		patterns = append(patterns, anchor.Tool)
		matches := slices.Concat(anchor.Input, anchor.Exclude)
		for _, match := range matches {
			patterns = append(patterns, match.Pattern)
		}
	}
	return patterns
}

// droppableTexts are the texts the render trims when a variable they read is unset: entries, allowed-tools rules and the enforce declaration.
func (s *Skill) droppableTexts() []string {
	var texts []string
	entries := s.Entries()
	for _, ref := range entries {
		texts = append(texts, ref.Entry.Statement)
		for _, item := range ref.Entry.Items {
			texts = append(texts, item.Text)
			if item.Fence != nil {
				texts = append(texts, item.Fence.Body)
			}
		}
		if ref.Entry.Payload != nil {
			texts = append(texts, ref.Entry.Payload.Body)
		}
		if ref.Entry.Fields != nil {
			texts = append(texts, ref.Entry.Fields.Flags)
		}
	}
	for _, field := range s.Frontmatter {
		if field.Key == fieldAllowedTools {
			texts = append(texts, field.Value)
		}
	}
	texts = append(texts, s.patternTexts()...)
	return append(texts, s.enforceTexts()...)
}

// enforceTexts are the reason and recovery of every check and the question and send of every judgment.
func (s *Skill) enforceTexts() []string {
	if s.Enforce == nil {
		return nil
	}

	var texts []string
	checkIds := slices.Sorted(maps.Keys(s.Enforce.Checks))
	for _, id := range checkIds {
		texts = append(texts, s.Enforce.Checks[id].Reason, s.Enforce.Checks[id].Recovery)
	}
	judgmentIds := slices.Sorted(maps.Keys(s.Enforce.Judgment))
	for _, id := range judgmentIds {
		texts = append(texts, s.Enforce.Judgment[id].Question, s.Enforce.Judgment[id].Send)
	}
	return texts
}

// fixedTexts render whatever the values are: an unset reference there can be neither filled nor dropped.
func (s *Skill) fixedTexts() []string {
	texts := []string{s.Preamble}
	for _, field := range s.Frontmatter {
		if field.Key != fieldAllowedTools {
			texts = append(texts, field.Value)
		}
	}
	for _, section := range s.Sections {
		for _, block := range section.Blocks {
			texts = append(texts, block.Prose)
		}
	}

	args := s.Args()
	for _, arg := range args {
		texts = append(texts, arg.Default, arg.Effect, arg.Form)
	}
	if routing := s.Routing(); routing != nil {
		texts = append(texts, routing.Purpose, routing.Pipeline.Note)
		texts = append(texts, routing.Preconditions...)
		for _, route := range routing.Routes {
			texts = append(texts, route.For, route.Use)
		}
	}
	if model := s.Model(); model != nil {
		texts = append(texts, model.Reason)
		texts = append(texts, model.Notes...)
		texts = append(texts, model.Unviable...)
		for _, route := range model.Routes {
			texts = append(texts, route.Route)
		}
	}
	return texts
}

// texts is every string the render emits.
func (s *Skill) texts() []string {
	return slices.Concat(s.droppableTexts(), s.fixedTexts())
}

// catalog is never nil: a model loaded without a root carries the empty catalog.
func (s *Skill) catalog() *Catalog {
	if s.Catalog == nil {
		return &Catalog{}
	}
	return s.Catalog
}

// RequiresSessionValues reports a required repo-scoped variable: its value reaches a session only through the Skill environment hook.
func (s *Skill) RequiresSessionValues() bool {
	for _, variable := range s.Env {
		iface, isKnown := s.catalog().Interface(variable.Interface)
		if variable.Required && isKnown && iface.IsRepoScoped() {
			return true
		}
	}
	return false
}

// VariableNames maps every rendered name of the skill's env block to its declaration.
func (s *Skill) VariableNames() map[string]*Variable {
	names := make(map[string]*Variable, len(s.Env))
	for key, variable := range s.Env {
		names[VariableName(s.Name(), key)] = variable
	}
	return names
}

// VariableRead is one text reading a variable: Id is the entry, rule, anchor or check it belongs to.
type VariableRead struct {
	Id   string
	Kind string
}

// checkEnv refuses a referenced undeclared variable, a declared one no text reads, an optional one read where nothing drops it, and a repo-scoped one read where no placeholder can stand.
func checkEnv(skill *Skill) []string {
	declared := skill.VariableNames()
	referenced := referencesIn(skill.texts())

	violations := bindingViolations(skill)
	repoScoped := skill.RepoScoped()
	outsideTexts := slices.Concat(skill.fixedTexts(), skill.patternTexts(), skill.enforceTexts())
	outsideEntries := referencesIn(outsideTexts)
	outsideNames := slices.Sorted(maps.Keys(outsideEntries))
	for _, name := range outsideNames {
		if repoScoped[name] {
			violations = append(violations, fmt.Sprintf("env: %s is repo-scoped and read in the preamble, frontmatter, prose, args, routing, model or enforce block, where no placeholder can stand — move the reference into an entry", name))
		}
	}
	fixedReferences := referencesIn(skill.fixedTexts())
	fixedNames := slices.Sorted(maps.Keys(fixedReferences))
	for _, name := range fixedNames {
		variable, isDeclared := declared[name]
		if isDeclared && !variable.Required {
			violations = append(violations, fmt.Sprintf("env: %s is optional but read in the preamble, frontmatter, prose, args, routing or model, where an unset value cannot be dropped — mark it required or move the reference into an entry", name))
		}
	}
	referencedNames := slices.Sorted(maps.Keys(referenced))
	for _, name := range referencedNames {
		if _, isDeclared := declared[name]; !isDeclared {
			violations = append(violations, fmt.Sprintf("env: %s is referenced but no env entry declares it", name))
		}
	}

	declaredNames := slices.Sorted(maps.Keys(declared))
	for _, name := range declaredNames {
		if !referenced[name] {
			violations = append(violations, fmt.Sprintf("env: %s: no text reads it — delete the declaration", name))
		}
	}
	return append(violations, referenceShapeViolations(skill)...)
}

// bindingViolations: every env key names its interface id and binds a catalog record.
func bindingViolations(skill *Skill) []string {
	var violations []string
	keys := slices.Sorted(maps.Keys(skill.Env))
	for _, key := range keys {
		ref := skill.Env[key].Interface
		iface, isKnown := skill.catalog().Interface(ref)
		if !isKnown {
			violations = append(violations, fmt.Sprintf("env: %s: interface %s is not in %s", key, ref, FileCatalog))
			continue
		}
		if expected := VariableKey(iface.Id); key != expected {
			violations = append(violations, fmt.Sprintf("env: %s: key must be %s for %s", key, expected, ref))
		}
	}
	return violations
}

// referenceShapeViolations: a program reference is followed by its args head, a command reference by nothing but the end, a quote, a backtick or a redirect; a path or skill reference by anything.
func referenceShapeViolations(skill *Skill) []string {
	var violations []string
	declared := skill.VariableNames()
	for _, text := range skill.texts() {
		for _, match := range variableRefPattern.FindAllStringIndex(text, -1) {
			name := text[match[0]+1 : match[1]]
			variable, isDeclared := declared[name]
			if !isDeclared {
				continue
			}

			iface, isKnown := skill.catalog().Interface(variable.Interface)
			if !isKnown {
				continue
			}

			rest := strings.TrimLeft(text[match[1]:], " ")
			violations = append(violations, shapeViolation(iface, name, rest)...)
		}
	}
	slices.Sort(violations)
	return slices.Compact(violations)
}

func shapeViolation(iface *Interface, name, rest string) []string {
	switch iface.Kind {
	case KindProgram:
		head := iface.ArgsHead()
		isHeaded := slices.ContainsFunc(headTerminators, func(terminator string) bool { return strings.HasPrefix(rest, head+terminator) })
		isBareValue := slices.ContainsFunc(valueTerminators, func(terminator string) bool { return strings.HasPrefix(rest, terminator) })
		isShaped := isHeaded || isBareValue
		if !isShaped && rest != head {
			return []string{fmt.Sprintf("env: $%s: followed by %q — %s expects %q", name, firstWord(rest), iface.Ref(), head)}
		}
	case KindCommand:
		isTerminated := rest == "" || slices.ContainsFunc(commandTerminators, func(terminator string) bool { return strings.HasPrefix(rest, terminator) })
		if !isTerminated {
			return []string{fmt.Sprintf("env: $%s: followed by %q — a command value takes no arguments", name, firstWord(rest))}
		}
	case KindPath, KindSkill:
	}
	return nil
}

// referencesIn collects the variable names texts reference, without the leading $.
func referencesIn(texts []string) map[string]bool {
	referenced := map[string]bool{}
	for _, text := range texts {
		for _, ref := range variableRefPattern.FindAllString(text, -1) {
			referenced[strings.TrimPrefix(ref, "$")] = true
		}
	}
	return referenced
}

// UnconfiguredVariables lists the required names values lacks, sorted, leaving out the deferred ones filled at invocation; the deploy skips a skill with any.
func UnconfiguredVariables(deferred map[string]bool, skill *Skill, values map[string]string) []string {
	var unconfigured []string
	for name, variable := range skill.VariableNames() {
		isUnset := values[name] == "" && !deferred[name]
		if isUnset && variable.Required {
			unconfigured = append(unconfigured, name)
		}
	}
	slices.Sort(unconfigured)
	return unconfigured
}

// VariableName renders a declared key as the install-facing name: SKILL_ + the leaf name uppercased without hyphens + _ + key.
func VariableName(skillName, key string) string {
	leaf := strings.ToUpper(strings.ReplaceAll(skillName, "-", ""))
	return variablePrefix + leaf + "_" + key
}

// VariableReaders lists what an unset name drops: "entry <id>", "param of <id>", "field flags of <id>", "grant <rule>", "anchor <id>", "check <id>".
func VariableReaders(skill *Skill, name string) []string {
	var readers []string
	for _, read := range VariableReads(skill, name) {
		switch read.Kind {
		case ReadKindAnchor, ReadKindCheck, ReadKindEntry, ReadKindGrant:
			readers = append(readers, read.Kind+" "+read.Id)
		case ReadKindFlags:
			readers = append(readers, "field flags of "+read.Id)
		case ReadKindParam:
			readers = append(readers, "param of "+read.Id)
		}
	}
	return readers
}

// VariableReads lists what reads a name, in model order: entry statements and payloads, their items and field flags, grants, enforce anchors, then checks.
func VariableReads(skill *Skill, name string) []VariableRead {
	reads := func(text string) bool { return referencesIn([]string{text})[name] }
	var readers []VariableRead
	for _, ref := range skill.Entries() {
		body := ref.Entry.Statement
		if ref.Entry.Payload != nil {
			body += "\n" + ref.Entry.Payload.Body
		}
		if reads(body) {
			readers = append(readers, VariableRead{Id: ref.Entry.Id, Kind: ReadKindEntry})
		}
		for _, item := range ref.Entry.Items {
			text := item.Text
			if item.Fence != nil {
				text += "\n" + item.Fence.Body
			}
			if reads(text) {
				readers = append(readers, VariableRead{Id: ref.Entry.Id, Kind: ReadKindParam})
			}
		}
		if ref.Entry.Fields != nil && reads(ref.Entry.Fields.Flags) {
			readers = append(readers, VariableRead{Id: ref.Entry.Id, Kind: ReadKindFlags})
		}
	}
	for rule := range strings.SplitSeq(skill.Field(fieldAllowedTools), ",") {
		rule = strings.TrimSpace(rule)
		if reads(rule) {
			readers = append(readers, VariableRead{Id: rule, Kind: ReadKindGrant})
		}
	}
	if skill.Enforce == nil {
		return readers
	}

	anchorIds := slices.Sorted(maps.Keys(skill.Enforce.Anchors))
	for _, id := range anchorIds {
		anchor := skill.Enforce.Anchors[id]
		patterns := []string{anchor.Tool}
		matches := slices.Concat(anchor.Input, anchor.Exclude)
		for _, match := range matches {
			patterns = append(patterns, match.Pattern)
		}
		if reads(strings.Join(patterns, "\n")) {
			readers = append(readers, VariableRead{Id: id, Kind: ReadKindAnchor})
		}
	}
	checkIds := slices.Sorted(maps.Keys(skill.Enforce.Checks))
	for _, id := range checkIds {
		check := skill.Enforce.Checks[id]
		if reads(check.Reason + "\n" + check.Recovery) {
			readers = append(readers, VariableRead{Id: id, Kind: ReadKindCheck})
		}
	}
	return readers
}
