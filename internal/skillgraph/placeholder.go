package skillgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillenv"
)

const (
	placeholderAbortExit  = 2
	placeholderClose      = "```"
	placeholderFence      = "```!"
	placeholderGrant      = "Bash(awk *)"
	placeholderProgram    = "awk 'BEGIN { "
	placeholderProgramEnd = " }'"
	// plainPunctuation is what a text byte may be besides a letter, digit or space: everything else is octal-escaped, so the program holds no quote, backslash, dollar or operator the harness's shell parser could misread.
	plainPunctuation = " ,.:/_=+@~-"
)

// deferral is the render's view of the variables filled at invocation: their declarations, the deploy's values, the sentence tail and the names its blocks read.
type deferral struct {
	emitted   map[string]bool
	hint      string
	skill     *Skill
	values    map[string]string
	variables map[string]*Variable
}

// newDeferral is nil when the skill declares none of the names, so every caller treats nil as the canonical render.
func newDeferral(hint string, names map[string]bool, skill *Skill, values map[string]string) *deferral {
	variables := map[string]*Variable{}
	for name, variable := range skill.VariableNames() {
		if names[name] {
			variables[name] = variable
		}
	}
	if len(variables) == 0 {
		return nil
	}
	return &deferral{emitted: map[string]bool{}, hint: hint, skill: skill, values: values, variables: variables}
}

// abortSentence is what the block prints after "NAME: " when a required value is unset: the record's summary, its example and the hint, in plain characters only.
func (d *deferral) abortSentence(name string) string {
	iface, isKnown := d.skill.catalog().Interface(d.variables[name].Interface)
	if !isKnown {
		iface = &Interface{}
	}
	return plainText(fmt.Sprintf("not set - %s, e.g. %s - %s", skillenv.Phrase(iface.Summary), iface.Example, d.hint))
}

// filled replaces the references the deploy fills with their value, an unset one with its literal text; the deferred ones stay for block to split on.
func (d *deferral) filled(raw string) string {
	return variableRefPattern.ReplaceAllStringFunc(raw, func(ref string) string {
		name := ref[1:]
		if d.variables[name] != nil {
			return ref
		}
		if value := d.values[name]; value != "" {
			return value
		}
		return ref
	})
}

// guard is the statement that ends the program before it prints: a required name unset prints the abort sentence and exits 2, an optional one exits 0 with no output.
func (d *deferral) guard(name string) string {
	if d.variables[name].Required {
		return fmt.Sprintf(`if (%s == "") { print "%s: %s"; exit %d }; `, environ(name), name, d.abortSentence(name), placeholderAbortExit)
	}
	return fmt.Sprintf(`if (%s == "") exit 0; `, environ(name))
}

// readNames lists the deferred names the lines reference, the required ones first, each group sorted.
func (d *deferral) readNames(lines []string) []string {
	read := map[string]bool{}
	for _, ref := range variableRefPattern.FindAllString(strings.Join(lines, "\n"), -1) {
		if d.variables[ref[1:]] != nil {
			read[ref[1:]] = true
		}
	}
	names := slices.Sorted(maps.Keys(read))
	slices.SortStableFunc(names, func(a, b string) int {
		return requiredFirst(d.variables[a].Required, d.variables[b].Required)
	})
	return names
}

// renderEnv encodes the skill's env.json: every deferred name a block read, sorted, with its record's id, scope, summary and example.
func (d *deferral) renderEnv() ([]byte, error) {
	names := slices.Sorted(maps.Keys(d.emitted))
	entries := make([]*skillenv.Entry, 0, len(names))
	for _, name := range names {
		iface, isKnown := d.skill.catalog().Interface(d.variables[name].Interface)
		if !isKnown {
			iface = &Interface{}
		}

		entry := &skillenv.Entry{
			Example:   iface.Example,
			Interface: iface.Id,
			Name:      name,
			Required:  d.variables[name].Required,
			Scope:     iface.Scope,
			Summary:   iface.Summary,
		}
		entries = append(entries, entry)
	}

	document := &skillenv.Document{Schema: skillenv.DocumentSchema, Skill: d.skill.Name(), Variables: entries, Version: d.skill.Version()}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("deferral.renderEnv: %w", err)
	}
	return append(encoded, '\n'), nil
}

// undeferred blanks the deferred references, so the drop checks judge only what the deploy fills; nil keeps the text.
func (d *deferral) undeferred(text string) string {
	if d == nil {
		return text
	}
	return variableRefPattern.ReplaceAllStringFunc(text, func(ref string) string {
		if d.variables[ref[1:]] != nil {
			return ""
		}
		return ref
	})
}

// block wraps the lines in one fenced awk block when they read a deferred name; otherwise the lines come back as they are.
func (d *deferral) block(lines []string) []string {
	read := d.readNames(lines)
	if len(read) == 0 {
		return lines
	}

	var program strings.Builder
	program.WriteString(placeholderProgram)
	for _, name := range read {
		d.emitted[name] = true
		program.WriteString(d.guard(name))
	}
	raw := d.filled(strings.Join(lines, "\n"))
	var pieces []string
	last := 0
	for _, match := range variableRefPattern.FindAllStringIndex(raw, -1) {
		name := raw[match[0]+1 : match[1]]
		if d.variables[name] == nil {
			continue
		}
		pieces = append(pieces, awkString(raw[last:match[0]])...)
		pieces = append(pieces, environ(name))
		last = match[1]
	}
	pieces = append(pieces, awkString(raw[last:])...)
	program.WriteString(`printf "%s", `)
	program.WriteString(strings.Join(pieces, " "))
	program.WriteString(placeholderProgramEnd)
	return []string{placeholderFence, program.String(), placeholderClose}
}

// awkString renders one text run as an awk string literal, none for an empty run: plain bytes as they are, every other byte as a three-digit octal escape.
func awkString(text string) []string {
	if text == "" {
		return nil
	}
	var out strings.Builder
	out.WriteByte('"')
	for i := 0; i < len(text); i++ {
		if textByte := text[i]; isPlainByte(textByte) {
			out.WriteByte(textByte)
		} else {
			fmt.Fprintf(&out, `\%03o`, textByte)
		}
	}
	out.WriteByte('"')
	return []string{out.String()}
}

// environ is the awk expression reading the name from the session's environment.
func environ(name string) string {
	return `ENVIRON["` + name + `"]`
}

// isPlainByte reports a byte the program carries as it is: a letter, a digit or plain punctuation.
func isPlainByte(b byte) bool {
	isLetter := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
	isDigit := b >= '0' && b <= '9'
	return isLetter || isDigit || strings.IndexByte(plainPunctuation, b) >= 0
}

// plainText keeps the plain bytes of text and turns every other run into one space — the abort sentence awk prints raw.
func plainText(text string) string {
	var out strings.Builder
	isGap := false
	for i := 0; i < len(text); i++ {
		if isPlainByte(text[i]) {
			out.WriteByte(text[i])
			isGap = false
			continue
		}
		if !isGap {
			out.WriteByte(' ')
			isGap = true
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// requiredFirst orders a required name before an optional one and leaves two of a kind as they are.
func requiredFirst(isRequiredA, isRequiredB bool) int {
	switch {
	case isRequiredA == isRequiredB:
		return 0
	case isRequiredA:
		return -1
	}
	return 1
}
