package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	titleSetArgCount = 2 // the verb and its token
	titleUsage       = "usage: rules title [-skills <dir>] set <WIP|READY|declared state>"
)

// runTitle confirms a skill step's state request; the sessiontitle hook records the printed line and renames at the turn end.
func runTitle(args []string) int {
	flags := flag.NewFlagSet("title", flag.ExitOnError)
	skillsDir := flags.String("skills", skills.DefaultHomePath(), "deployed skills dir whose title declarations name the settable states")
	flags.Parse(args)
	rest := flags.Args()
	isSet := len(rest) == titleSetArgCount && rest[0] == "set"
	if !isSet {
		fmt.Fprintln(os.Stderr, titleUsage)
		return exitError
	}

	vocabulary, err := sessiontitle.LoadVocabulary(*skillsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	state := sessiontitle.State(rest[1])
	if !vocabulary.IsSettable(state) {
		fmt.Fprintf(os.Stderr, "rules: title set %s: not WIP, READY or a state a deployed skill declares\n", state)
		return exitError
	}
	fmt.Println(sessiontitle.SetOutput(state))
	return exitClean
}
