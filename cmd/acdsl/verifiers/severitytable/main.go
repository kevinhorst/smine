// Command severitytable is an ACDSL verifier: a skill's severity-fixtures.json agrees with its schemas/severity-table.json.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
	minAxisValues  = 2
)

var severityValues = map[string]bool{"BLOCKER": true, "MAJOR": true, "MINOR": true, "NIT": true}

type fixture struct {
	Discriminator string `json:"discriminator"`
	Effect        string `json:"effect"`
	Example       bool   `json:"example"`
	Exposure      string `json:"exposure"`
	Id            string `json:"id"`
	Severity      string `json:"severity"`
}

type fixtures struct {
	Fixtures      []fixture `json:"fixtures"`
	SchemaVersion string    `json:"schemaVersion"`
	Table         string    `json:"table"`
}

type row struct {
	Effect   []string `json:"effect"`
	Exposure []string `json:"exposure"`
	Severity string   `json:"severity"`
}

type table struct {
	Axes          map[string][]string `json:"axes"`
	Rows          []row               `json:"rows"`
	SchemaVersion string              `json:"schemaVersion"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "severitytable: usage: <files-list>")
		return exitError
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "severitytable:", err)
		return exitError
	}
	violations := 0
	for file := range strings.FieldsSeq(string(listRaw)) {
		if filepath.Base(file) != "severity-fixtures.json" {
			continue
		}
		violations += checkFixtures(file, out)
	}
	if violations > 0 {
		return exitViolations
	}
	return exitClean
}

func checkFixtures(file string, out io.Writer) int {
	var fixturesFile fixtures
	if err := readJson(file, &fixturesFile); err != nil {
		fmt.Fprintf(out, "%s: %v\n", file, err)
		return 1
	}
	tablePath := filepath.Join(filepath.Dir(file), fixturesFile.Table)
	var severityTable table
	if err := readJson(tablePath, &severityTable); err != nil {
		fmt.Fprintf(out, "%s: table %s: %v\n", file, fixturesFile.Table, err)
		return 1
	}
	violations := checkTable(tablePath, &severityTable, out)
	seen := map[string]bool{}
	for _, fixtureCase := range fixturesFile.Fixtures {
		if seen[fixtureCase.Id] {
			fmt.Fprintf(out, "%s: duplicate fixture id %q\n", file, fixtureCase.Id)
			violations++
		}
		seen[fixtureCase.Id] = true
		got, err := lookup(&severityTable, fixtureCase.Effect, fixtureCase.Exposure)
		if err != nil {
			fmt.Fprintf(out, "%s: fixture %q: %v\n", file, fixtureCase.Id, err)
			violations++
			continue
		}
		if got != fixtureCase.Severity {
			fmt.Fprintf(out, "%s: fixture %q resolves to %s, expects %s\n", file, fixtureCase.Id, got, fixtureCase.Severity)
			violations++
		}
	}
	return violations
}

func checkTable(file string, severityTable *table, out io.Writer) int {
	violations := 0
	for axis, values := range severityTable.Axes {
		if len(values) < minAxisValues {
			fmt.Fprintf(out, "%s: axis %q has fewer than 2 values\n", file, axis)
			violations++
		}
	}
	for i, r := range severityTable.Rows {
		if !severityValues[r.Severity] {
			fmt.Fprintf(out, "%s: row %d: unknown severity %q\n", file, i, r.Severity)
			violations++
		}
		violations += checkMembers(file, i, "effect", r.Effect, severityTable.Axes["effect"], out)
		violations += checkMembers(file, i, "exposure", r.Exposure, severityTable.Axes["exposure"], out)
	}
	for _, effect := range severityTable.Axes["effect"] {
		for _, exposure := range severityTable.Axes["exposure"] {
			if _, err := lookup(severityTable, effect, exposure); err != nil {
				fmt.Fprintf(out, "%s: %v\n", file, err)
				violations++
			}
		}
	}
	return violations
}

func checkMembers(file string, rowIndex int, axis string, values []string, members []string, out io.Writer) int {
	violations := 0
	for _, v := range values {
		if !slices.Contains(members, v) {
			fmt.Fprintf(out, "%s: row %d: %s value %q is not an axis member\n", file, rowIndex, axis, v)
			violations++
		}
	}
	return violations
}

// lookup resolves one vector; an absent row axis matches every value.
func lookup(severityTable *table, effect string, exposure string) (string, error) {
	matches := make([]string, 0)
	for _, r := range severityTable.Rows {
		isEffect := len(r.Effect) == 0 || slices.Contains(r.Effect, effect)
		isExposure := len(r.Exposure) == 0 || slices.Contains(r.Exposure, exposure)
		if isEffect && isExposure {
			matches = append(matches, r.Severity)
		}
	}
	sort.Strings(matches)
	if len(matches) != 1 {
		return "", fmt.Errorf("lookup: Vector %s/%s matches %d rows, want exactly 1", effect, exposure, len(matches))
	}
	return matches[0], nil
}

func readJson(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readJson: %w", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("readJson: Invalid JSON: %w", err)
	}
	return nil
}
