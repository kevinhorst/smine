package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

func runRenumber(args []string) int {
	flags := flag.NewFlagSet("renumber", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel renumber <skill dir>…")
		return exitError
	}

	for _, dir := range flags.Args() {
		line, err := renumberSkill(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillmodel:", err)
			return exitError
		}
		fmt.Println(line)
	}
	return exitClean
}

// renumberSkill rewrites one model file so its changelog versions strictly descend and reports the top version's move.
func renumberSkill(dir string) (string, error) {
	path := skillgraph.ModelPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("renumberSkill: %w", err)
	}

	before, err := skillgraph.DecodeSkill(data, path)
	if err != nil {
		return "", fmt.Errorf("renumberSkill: %w", err)
	}

	renumbered, err := skillgraph.RenumberChangelog(data)
	if err != nil {
		return "", fmt.Errorf("renumberSkill: %s: %w", dir, err)
	}

	if bytes.Equal(renumbered, data) {
		return fmt.Sprintf("skillmodel: unchanged %s: %s", dir, before.Version()), nil
	}

	after, err := skillgraph.DecodeSkill(renumbered, path)
	if err != nil {
		return "", fmt.Errorf("renumberSkill: %w", err)
	}

	if err := fsx.WriteFileAtomic(renumbered, path, 0o644); err != nil {
		return "", fmt.Errorf("renumberSkill: %w", err)
	}
	return fmt.Sprintf("skillmodel: renumbered %s: %s → %s", dir, before.Version(), after.Version()), nil
}
