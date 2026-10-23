package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/skills"
)

func runIntake(args []string) int {
	flags := flag.NewFlagSet("intake", flag.ExitOnError)
	base := flags.String("base", "", "review base commit; required")
	claudeHome := flags.String("claude-home", filepath.Dir(skills.DefaultHomePath()), "Claude home whose model vocabulary names the suggested model")
	excludeCsv := flags.String("exclude", "", "comma-separated pathspecs already excluded from the diff")
	isGate := flags.Bool("gate", false, "bare invocation: append the reply line the config gate waits on")
	head := flags.String("head", "", "review head commit; required")
	pathsCsv := flags.String("paths", "", "comma-separated pathspecs narrowing the diff; absent = whole diff")
	flags.Parse(args)
	if *base == "" || *head == "" {
		fmt.Fprintln(os.Stderr, "usage: rules intake --base <commit> --head <commit> [--paths p,q] [--exclude x,y] [--claude-home <dir>] [--gate]")
		return exitError
	}

	vocabulary, err := models.LoadHome(*claudeHome)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	request := &reviewscope.IntakeRequest{
		Base:     *base,
		Excludes: splitCsv(*excludeCsv),
		Head:     *head,
		Model:    vocabulary.DefaultSubagent(),
		Paths:    splitCsv(*pathsCsv),
		Repo:     ".",
	}
	report, err := reviewscope.Intake(context.Background(), request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Print(report.Render(*isGate))
	return exitClean
}
