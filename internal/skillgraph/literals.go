package skillgraph

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// literalDirs are the deployed sibling dirs the literal check scans beside the model.
var literalDirs = []string{"agents", "workflows"}

// LiteralNeedles are the smine programs no skill source may name, as regex alternatives; the word boundary keeps the smine-routines skill name clean. The bundle scan reuses them.
var LiteralNeedles = []string{"smine-rules", "smine-agentwt", "smine-heartbeat", `smine-routine\b`, "toolpolicy"}

var literalPattern = regexp.MustCompile(strings.Join(LiteralNeedles, "|"))

// checkLiterals refuses a smine program name in any model text or in a file under agents/ or workflows/ of dir.
func checkLiterals(dir string, skill *Skill) []string {
	var violations []string
	isFormatFields := entryFormat(skill) >= entryFormatFields
	for _, text := range skill.texts() {
		for _, needle := range literalPattern.FindAllString(text, -1) {
			violations = append(violations, fmt.Sprintf("literal %s: call it through a $SKILL_%s_<NAME> variable", needle, strings.ToUpper(strings.ReplaceAll(skill.Name(), "-", ""))))
		}
		if isFormatFields && refPattern.MatchString(text) {
			violations = append(violations, "markdown link to an entry: a format-3 skill writes [[SECTION-NNN]]")
		}
	}
	declared := skill.VariableNames()
	repoScoped := skill.RepoScoped()
	for _, sub := range literalDirs {
		for _, file := range siblingFiles(filepath.Join(dir, sub)) {
			violations = append(violations, fileLiteralViolations(file)...)
			violations = append(violations, fileReferenceViolations(declared, repoScoped, file)...)
		}
	}
	return slices.Compact(violations)
}

// fileReferenceViolations: a copied sibling file is filled at deploy, never trimmed, so it may reference install-scoped required variables only.
func fileReferenceViolations(declared map[string]*Variable, repoScoped map[string]bool, path string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var violations []string
	for _, ref := range variableRefPattern.FindAllString(string(content), -1) {
		variable, isDeclared := declared[ref[1:]]
		switch {
		case !isDeclared:
			violations = append(violations, fmt.Sprintf("%s: %s is referenced but no env entry declares it", path, ref[1:]))
		case repoScoped[ref[1:]]:
			violations = append(violations, fmt.Sprintf("%s: %s is repo-scoped — sibling files are filled at deploy and reference install-scoped required variables only", path, ref[1:]))
		case !variable.Required:
			violations = append(violations, fmt.Sprintf("%s: %s is optional — sibling files reference required variables only", path, ref[1:]))
		}
	}
	return violations
}

// fileLiteralViolations reports path:line of every needle in one sibling file.
func fileLiteralViolations(path string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("literal check: %v", err)}
	}

	var violations []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), len(content)+1)
	for line := 1; scanner.Scan(); line++ {
		for _, needle := range literalPattern.FindAllString(scanner.Text(), -1) {
			violations = append(violations, fmt.Sprintf("%s:%d: literal %s", path, line, needle))
		}
	}
	return violations
}

// hasUnsetReference reports whether text references a variable values lacks.
func hasUnsetReference(text string, values map[string]string) bool {
	for _, ref := range variableRefPattern.FindAllString(text, -1) {
		if values[ref[1:]] == "" {
			return true
		}
	}
	return false
}

// itemView is the entry without the items and the flags field whose text references an unset variable; a deferred reference never drops one.
func itemView(deferred *deferral, entry *Entry, values map[string]string) *Entry {
	view := *entry
	isFlagsUnset := entry.Fields != nil && hasUnsetReference(deferred.undeferred(entry.Fields.Flags), values)
	if isFlagsUnset {
		fields := *entry.Fields
		fields.Flags = ""
		view.Fields = &fields
	}
	view.Items = make([]Item, 0, len(entry.Items))
	for _, item := range entry.Items {
		text := item.Text
		if item.Fence != nil {
			text += "\n" + item.Fence.Body
		}
		if !hasUnsetReference(deferred.undeferred(text), values) {
			view.Items = append(view.Items, item)
		}
	}
	return &view
}

// siblingFiles lists every regular file below root in walk order; a missing root has none.
func siblingFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// variableDisabled returns the entries whose statement or payload reference an unset variable; an item reference drops only the item (itemView); a deferred reference drops nothing.
func variableDisabled(deferred *deferral, skill *Skill, values map[string]string) map[string]bool {
	disabled := map[string]bool{}
	for _, ref := range skill.Entries() {
		texts := []string{ref.Entry.Statement}
		if ref.Entry.Payload != nil {
			texts = append(texts, ref.Entry.Payload.Body)
		}
		if hasUnsetReference(deferred.undeferred(strings.Join(texts, "\n")), values) {
			disabled[ref.Entry.Id] = true
		}
	}
	return disabled
}

// FillPattern fills a regex pattern, quoting each value so a program path never reads as regex syntax.
func FillPattern(pattern string, values map[string]string) string {
	return variableRefPattern.ReplaceAllStringFunc(pattern, func(ref string) string {
		value, isSet := values[ref[1:]]
		if !isSet || value == "" {
			return ref
		}
		return regexp.QuoteMeta(value)
	})
}

// FillVariables replaces every set variable's reference in text; references of unset variables stay for the caller to judge.
func FillVariables(text []byte, values map[string]string) []byte {
	return variableRefPattern.ReplaceAllFunc(text, func(ref []byte) []byte {
		value, isSet := values[string(ref[1:])]
		if !isSet || value == "" {
			return ref
		}
		return []byte(value)
	})
}
