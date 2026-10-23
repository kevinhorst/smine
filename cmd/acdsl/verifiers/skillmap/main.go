// Command skillmap is an ACDSL verifier: a skill's context-map.json is well-formed and stays inside its acdsl-context: declaration.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

var (
	idTokenRe   = regexp.MustCompile(`^(ACTION|RULE|FACT)-[A-Z0-9]+(-[A-Z0-9]+)*-[0-9]{3}$`)
	globTokenRe = regexp.MustCompile(`^(ACTION|RULE|FACT)-([A-Z0-9]+-)*\*$`)
)

// contextMap is the context-map.json shape the skill-context hook reads.
type contextMap struct {
	SchemaVersion string              `json:"schemaVersion"`
	Skill         string              `json:"skill"`
	Lanes         map[string][]string `json:"lanes"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillmap: usage: <files-list> index=<index path>")
		return 2
	}
	index := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "index="); ok {
			index = value
		}
	}
	if index == "" {
		fmt.Fprintln(os.Stderr, "skillmap: index= param missing")
		return 2
	}
	ids, err := indexIds(index)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmap:", err)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmap:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		if filepath.Base(file) != "context-map.json" {
			continue
		}
		violations += checkMap(file, ids, out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// indexIds returns the set of entry IDs in the generated context index.
func indexIds(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var index struct {
		Entries []struct {
			Id string `json:"id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(index.Entries))
	for _, entry := range index.Entries {
		ids[entry.Id] = true
	}
	return ids, nil
}

func declarationTokens(mapPath string) ([]string, error) {
	raw, err := siblingManifest(filepath.Dir(mapPath))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if value, ok := strings.CutPrefix(line, "acdsl-context:"); ok {
			tokens := make([]string, 0)
			for _, token := range strings.Split(value, ",") {
				if token = strings.TrimSpace(token); token != "" {
					tokens = append(tokens, token)
				}
			}
			return tokens, nil
		}
	}
	return nil, nil
}

func siblingManifest(dir string) ([]byte, error) {
	modelPath := skillgraph.ModelPath(dir)
	if _, err := os.Stat(modelPath); err == nil {
		return skillgraph.ReadManifest(modelPath)
	}
	return skillgraph.ReadManifest(filepath.Join(dir, skillgraph.FileManifest))
}

// resolve returns the index entry IDs a token set selects.
func resolve(tokens []string, ids map[string]bool) map[string]bool {
	selected := map[string]bool{}
	for _, token := range tokens {
		if ids[token] {
			selected[token] = true
			continue
		}
		if !globTokenRe.MatchString(token) {
			continue
		}
		prefix := strings.TrimSuffix(token, "*")
		for id := range ids {
			if strings.HasPrefix(id, prefix) {
				selected[id] = true
			}
		}
	}
	return selected
}

func checkMap(file string, ids map[string]bool, out io.Writer) int {
	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(out, "%s: %v\n", file, err)
		return 1
	}
	var parsed contextMap
	if err := json.Unmarshal(raw, &parsed); err != nil {
		fmt.Fprintf(out, "%s: invalid JSON: %v\n", file, err)
		return 1
	}
	violations := 0
	if parsed.SchemaVersion == "" {
		fmt.Fprintf(out, "%s: schemaVersion missing\n", file)
		violations++
	}
	skillDir := filepath.Base(filepath.Dir(file))
	if parsed.Skill != skillDir {
		fmt.Fprintf(out, "%s: skill %q does not match the directory name %q\n", file, parsed.Skill, skillDir)
		violations++
	}
	declared, err := declarationTokens(file)
	if err != nil {
		fmt.Fprintf(out, "%s: sibling SKILL.md unreadable: %v\n", file, err)
		return violations + 1
	}
	declaredIds := resolve(declared, ids)

	laneNames := make([]string, 0, len(parsed.Lanes))
	for lane := range parsed.Lanes {
		laneNames = append(laneNames, lane)
	}
	sort.Strings(laneNames)
	for _, lane := range laneNames {
		for _, token := range parsed.Lanes[lane] {
			switch {
			case idTokenRe.MatchString(token):
				if !ids[token] {
					fmt.Fprintf(out, "%s: lane %q entry %q not in the context index\n", file, lane, token)
					violations++
				}
			case globTokenRe.MatchString(token):
				prefix := strings.TrimSuffix(token, "*")
				found := false
				for id := range ids {
					if strings.HasPrefix(id, prefix) {
						found = true
						break
					}
				}
				if !found {
					fmt.Fprintf(out, "%s: lane %q glob %q matches no context entry\n", file, lane, token)
					violations++
				}
			default:
				fmt.Fprintf(out, "%s: lane %q token %q is neither an entry ID nor a trailing-* glob\n", file, lane, token)
				violations++
			}
		}
		laneIds := resolve(parsed.Lanes[lane], ids)
		outside := make([]string, 0)
		for id := range laneIds {
			if !declaredIds[id] {
				outside = append(outside, id)
			}
		}
		if len(outside) > 0 {
			sort.Strings(outside)
			fmt.Fprintf(out, "%s: lane %q selects entries outside the SKILL.md declaration: %s\n", file, lane, strings.Join(outside, ", "))
			violations++
		}
	}
	return violations
}
