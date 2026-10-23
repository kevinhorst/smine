package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/skills"
)

// runBundle exits 1 when a written file carries a smine reference, 2 on any other failure.
func runBundle(args []string) int {
	flags := flag.NewFlagSet("bundle", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillmodel bundle <dir>")
		return exitError
	}

	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}

	report, err := skills.Bundle(flags.Arg(0), repoDir)
	fmt.Print(report)
	switch {
	case errors.Is(err, skills.ErrBundleReferences):
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitViolations
	case err != nil:
		fmt.Fprintln(os.Stderr, "skillmodel:", err)
		return exitError
	}
	return exitClean
}
