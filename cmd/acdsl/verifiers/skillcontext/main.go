// Command skillcontext is an ACDSL verifier: a skill's acdsl-context: declaration resolves against the context index.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillcontext: usage: <files-list> index=<index path>")
		return 2
	}
	index := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "index="); ok {
			index = value
		}
	}
	if index == "" {
		fmt.Fprintln(os.Stderr, "skillcontext: index= param missing")
		return 2
	}
	ids, err := indexIds(index)
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillcontext:", err)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillcontext:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		base := filepath.Base(file)
		if !skillgraph.IsModelFile(file) && base != skillgraph.FileManifest {
			continue
		}

		raw, err := skillgraph.ReadManifest(file)
		if err != nil {
			fmt.Fprintf(out, "%s:1: SKILL.md text unavailable — the model does not load or render (violations: ACDSL-SKILL-005)\n", file)
			violations++
			continue
		}
		violations += checkDeclaration(skillgraph.ManifestLabel(file), strings.Split(string(raw), "\n"), ids, out)
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

var (
	idTokenRe   = regexp.MustCompile(`^(ACTION|RULE|FACT)-[A-Z0-9]+(-[A-Z0-9]+)*-[0-9]{3}$`)
	globTokenRe = regexp.MustCompile(`^(ACTION|RULE|FACT)-([A-Z0-9]+-)*\*$`)
)

func looksLikeDeclaration(value string) bool {
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if idTokenRe.MatchString(token) || globTokenRe.MatchString(token) {
			return true
		}
	}
	return false
}

func frontmatter(lines []string) (map[string]string, map[string]int) {
	values, lineNos := map[string]string{}, map[string]int{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return values, lineNos
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok || strings.Contains(key, " ") {
			continue
		}
		values[key] = strings.TrimSpace(value)
		lineNos[key] = i + 1
	}
	return values, lineNos
}

func checkDeclaration(file string, lines []string, ids map[string]bool, out io.Writer) int {
	values, lineNos := frontmatter(lines)
	violations := 0
	if legacy, ok := values["context"]; ok && looksLikeDeclaration(legacy) {
		fmt.Fprintf(out, "%s:%d: declaration under legacy key \"context:\" — renamed to \"acdsl-context:\", nothing injects from here\n", file, lineNos["context"])
		violations++
	}
	value, present := values["acdsl-context"]
	if !present || value == "" {
		return violations
	}
	line := lineNos["acdsl-context"]
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		switch {
		case idTokenRe.MatchString(token):
			if !ids[token] {
				fmt.Fprintf(out, "%s:%d: declared entry %q not in the context index\n", file, line, token)
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
				fmt.Fprintf(out, "%s:%d: glob %q matches no context entry\n", file, line, token)
				violations++
			}
		default:
			fmt.Fprintf(out, "%s:%d: token %q is neither an entry ID nor a trailing-* glob\n", file, line, token)
			violations++
		}
	}
	return violations
}
