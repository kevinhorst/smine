package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/workflowgen"
)

const workflowGenExitNoFiles = 2

func runWorkflowGen(args []string) int {
	flags := flag.NewFlagSet("workflow-gen", flag.ExitOnError)
	isCheck := flags.Bool("check", false, "compare instead of writing: fail on a stale file")
	root := flags.String("root", ".", "repo root")
	flags.Parse(args)
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: rules workflow-gen [--check] [--root <repo>] <workflows dir>...")
		return exitError
	}

	vocabulary, err := models.LoadTemplates(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if vocabulary.DefaultSubagent() == "" {
		fmt.Fprintln(os.Stderr, "rules: workflow-gen: no subagent model resolves against the templates")
		return exitError
	}

	status := exitClean
	for _, dir := range flags.Args() {
		findings, err := workflowgen.NewGenerator(dir, vocabulary.WorkflowBlock()).Run(*isCheck)
		if errors.Is(err, workflowgen.ErrNoStageFiles) {
			fmt.Fprintln(os.Stderr, "workflow-gen:", err)
			return workflowGenExitNoFiles
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}

		for _, finding := range findings {
			fmt.Fprintln(os.Stderr, "workflow-gen:", finding)
			status = exitError
		}
	}
	return status
}
