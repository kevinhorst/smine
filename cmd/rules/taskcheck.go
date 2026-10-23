package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/register"
	"github.com/kevinhorst/smine/internal/routines"
)

// problemViolations runs the sentence register over a schema-valid task's problem; a read or decode error is reported as one line.
func problemViolations(file string) []string {
	task, err := routines.ReadChangeTaskFile(file)
	if err != nil {
		return []string{file + ":1: " + err.Error()}
	}

	violations := make([]string, 0)
	for _, violation := range register.SentenceViolations(task.Problem) {
		violations = append(violations, file+":1: problem: "+violation)
	}
	return violations
}

func runTaskCheck(args []string) int {
	flags := flag.NewFlagSet("task-check", flag.ExitOnError)
	flags.Parse(args)
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: rules task-check <task file>...")
		return exitError
	}

	schema, err := routines.ChangeTaskSchema()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations := make([]string, 0)
	for _, file := range flags.Args() {
		lines, err := schema.Violations(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}
		violations = append(violations, lines...)
		if len(lines) == 0 {
			violations = append(violations, problemViolations(file)...)
		}
	}

	for _, violation := range violations {
		fmt.Println(violation)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "rules: %d violation(s) in %d task file(s)\n", len(violations), flags.NArg())
		return exitViolations
	}

	fmt.Printf("task-check: %d file(s) valid\n", flags.NArg())
	return exitClean
}
