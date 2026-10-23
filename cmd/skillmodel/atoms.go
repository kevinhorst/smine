package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// atomRow is one atom of the census: its declaration and every reader.
type atomRow struct {
	Id string `json:"id"`

	IsShared bool                `json:"shared"`
	Kind     string              `json:"kind"`
	Readers  []skillgraph.Reader `json:"readers"`
	Source   string              `json:"source,omitempty"`
}

// atomCensus lists the atoms the skill declares plus the shared ones it reads, sorted by id.
func atomCensus(skill *skillgraph.Skill) []atomRow {
	readers := skillgraph.AtomReaders(skill)
	atoms := skill.AllAtoms()
	var rows []atomRow
	for _, id := range slices.Sorted(maps.Keys(atoms)) {
		_, isLocal := skill.Atoms[id]
		if !isLocal && len(readers[id]) == 0 {
			continue
		}
		row := atomRow{Id: id, IsShared: !isLocal, Kind: atoms[id].EffectiveKind(), Readers: readers[id], Source: atoms[id].Source}
		rows = append(rows, row)
	}
	return rows
}

func formatAtomRow(row *atomRow) string {
	kind := row.Kind
	if row.IsShared {
		kind += " shared"
	}
	source := "-"
	if row.Source != "" {
		source = row.Source
	}
	var readers []string
	var queries []string
	for _, reader := range row.Readers {
		switch reader.Role {
		case skillgraph.ReaderQuery:
			queries = append(queries, reader.Query)
		case skillgraph.ReaderSection:
			readers = append(readers, "section:"+reader.Section)
		default:
			readers = append(readers, reader.Entry+"("+reader.Role+")")
		}
	}
	return fmt.Sprintf(
		"%s  %s  source=%s  preds=%s  queries=%s",
		row.Id,
		kind,
		source,
		joinOrDash(readers),
		joinOrDash(queries),
	)
}

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}

func runAtoms(args []string) int {
	flags := flag.NewFlagSet("atoms", flag.ExitOnError)
	isJson := flags.Bool("json", false, "print the census as a JSON list")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel atoms [--json] <skill dir>")
		return exitError
	}
	skill, err := skillgraph.Load(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	rows := atomCensus(skill)
	if *isJson {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillmodel:", err)
			return exitError
		}

		fmt.Println(string(data))
		return exitClean
	}
	for index := range rows {
		fmt.Println(formatAtomRow(&rows[index]))
	}
	return exitClean
}
