// Command skillentries is an ACDSL verifier: a skill model loads and its entries pass the checks Load cannot make.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
)

// acdslIdPattern matches a declaration id in acdsl/*.acdsl files.
var acdslIdPattern = regexp.MustCompile(`//acdsl:(\S+)`)

func checkChannelClaim(entry *skillgraph.Entry, acdslIds map[string]bool) string {
	switch entry.Channel {
	case "gate":
		gate := entry.Bullet(skillgraph.LabelGate)
		if gate == "" {
			return entry.Id + ": [gate] requires a Gate: bullet naming the ACDSL rule id"
		}
		if !acdslIds[gate] {
			return fmt.Sprintf("%s: Gate: %s not declared in acdsl/*.acdsl", entry.Id, gate)
		}
	case "hook":
		hook := entry.Bullet(skillgraph.LabelHook)
		if hook == "" {
			return entry.Id + ": [hook] requires a Hook: bullet naming the enforcing hook source under cmd/"
		}
		if !strings.HasPrefix(hook, "cmd/") {
			return fmt.Sprintf("%s: Hook: %s is not under cmd/", entry.Id, hook)
		}
		if _, err := os.Stat(hook); err != nil {
			return fmt.Sprintf("%s: Hook: %s not found", entry.Id, hook)
		}
	case "schema":
		if entry.Fields != nil {
			return ""
		}
		schema := entry.SchemaPointer()
		if schema == "" {
			return entry.Id + ": [schema] requires a Schema: bullet naming the schema file"
		}
		if _, err := os.Stat(schema); err != nil {
			return fmt.Sprintf("%s: Schema: %s not found", entry.Id, schema)
		}
	}
	return ""
}

func checkEnumerations(dir string, skill *skillgraph.Skill, out io.Writer) (int, error) {
	rendered, err := skillgraph.RenderManifest(skill)
	if err != nil {
		return 0, fmt.Errorf("checkEnumerations: %w", err)
	}

	manifest := filepath.Join(dir, skillgraph.FileManifest)
	lines := strings.Split(string(rendered), "\n")
	violations := skills.CheckEnumerations(lines)
	for _, violation := range violations {
		fmt.Fprintf(out, "%s:%d: %s\n", manifest, violation.Line, violation.Message)
	}
	return len(violations), nil
}

func checkSkill(acdslIds map[string]bool, file string, out io.Writer) (int, error) {
	dir := filepath.Dir(file)
	skill, err := skillgraph.Load(dir)
	if err != nil {
		return printLoadError(err, file, out), nil
	}

	violations := 0
	wantName := skillgraph.SkillIdName(filepath.Base(dir))
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		if name := idSkillName(entry.Id); name != wantName {
			fmt.Fprintf(out, "%s:1: entry %s names skill %s, leaf dir is %s\n", file, entry.Id, name, wantName)
			violations++
		}
		if claimViolation := checkChannelClaim(entry, acdslIds); claimViolation != "" {
			fmt.Fprintf(out, "%s:1: %s\n", file, claimViolation)
			violations++
		}
	}
	spans, err := checkEnumerations(dir, skill, out)
	if err != nil {
		return 0, err
	}

	return violations + spans, nil
}

// The CUE schema has already proven the SKILL-<NAME>-<TOPIC>-NNN id shape.
func idSkillName(id string) string {
	name, _, _ := strings.Cut(strings.TrimPrefix(id, "SKILL-"), "-")
	return name
}

func loadAcdslIds() map[string]bool {
	ids := map[string]bool{}
	entries, err := os.ReadDir("acdsl")
	if err != nil {
		return ids
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".acdsl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("acdsl", entry.Name()))
		if err != nil {
			continue
		}
		for _, match := range acdslIdPattern.FindAllStringSubmatch(string(data), -1) {
			ids[match[1]] = true
		}
	}
	return ids
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

// Load joins its schema and Check violations with newlines.
func printLoadError(err error, file string, out io.Writer) int {
	violations := 0
	for line := range strings.Lines(err.Error()) {
		message := strings.TrimSpace(line)
		if message == "" {
			continue
		}
		fmt.Fprintf(out, "%s:1: %s\n", file, message)
		violations++
	}
	return violations
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillentries: usage: <files-list>")
		return exitError
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillentries:", err)
		return exitError
	}

	acdslIds := loadAcdslIds()
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		if !skillgraph.IsModelFile(file) {
			continue
		}
		count, err := checkSkill(acdslIds, file, out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillentries:", err)
			return exitError
		}
		violations += count
	}
	if violations > 0 {
		return exitViolations
	}
	return exitClean
}
