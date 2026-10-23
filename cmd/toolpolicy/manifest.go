package main

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// skillManifest is the slice of a deployed skill's frontmatter the gate reads; the runtime placement lines are declarative and never read.
type skillManifest struct {
	values map[string]string

	Rules []*Rule
	Skill string
}

// filled substitutes the session's values into each rule, a multi-line command value as one rule per line, and drops a rule a reference survives in.
func (m *skillManifest) filled(rules []string) []string {
	kept := make([]string, 0, len(rules))
	for _, rule := range rules {
		for _, lineValues := range m.lineValues(rule) {
			filledRule := string(skillgraph.FillVariables([]byte(rule), lineValues))
			if strings.Contains(filledRule, variableReferencePrefix) {
				continue
			}
			kept = append(kept, filledRule)
		}
	}
	return kept
}

// lineValues is one value map per line of the first multi-line value the rule references, else the session's values alone.
func (m *skillManifest) lineValues(rule string) []map[string]string {
	names := slices.Sorted(maps.Keys(m.values))
	for _, name := range names {
		value := m.values[name]
		isReferencedMultiLine := strings.Contains(value, "\n") && strings.Contains(rule, "$"+name)
		if !isReferencedMultiLine {
			continue
		}
		perLine := make([]map[string]string, 0)
		for line := range strings.SplitSeq(value, "\n") {
			lineMap := maps.Clone(m.values)
			lineMap[name] = line
			perLine = append(perLine, lineMap)
		}
		return perLine
	}
	return []map[string]string{m.values}
}

// read folds one frontmatter line into the manifest; other keys are ignored.
func (m *skillManifest) read(line string, source string) {
	key, value, isPair := strings.Cut(line, ":")
	if !isPair || key != keyAllowedTools {
		return
	}
	m.Rules = parseRules(m.filled(strings.Split(strings.TrimSpace(value), ",")), source)
}

// readSkillManifest reads the grants of a deployed skill's frontmatter; nil without a SKILL.md.
func readSkillManifest(home string, skill string, values map[string]string) *skillManifest {
	path := filepath.Join(home, ".claude", "skills", skill, "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	manifest := &skillManifest{Skill: skill, values: values}
	inFrontmatter := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == frontmatterFence {
			if inFrontmatter {
				return manifest
			}
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			manifest.read(trimmed, path)
		}
	}
	return manifest
}
