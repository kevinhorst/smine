// Command stylecheck replaces the learned style profile with the withheld stub on any secret or PII match, the profile kept in the trash.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	exitClean = 0
	exitHit   = 1
	exitError = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	path := flag.String("path", presentation.DefaultStylePath(), "style profile to check")
	flag.Parse()

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "stylecheck:", err)
		return exitError
	}

	hits, err := presentation.GuardStyleProfile(*path, trash.New(stateDir, 0))
	for _, hit := range hits {
		fmt.Printf("%s: %s line %d\n", *path, hit.Kind, hit.Line)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "stylecheck:", err)
		return exitError
	}
	if len(hits) > 0 {
		fmt.Printf("%s: withheld — replaced by the stub\n", *path)
		return exitHit
	}
	fmt.Printf("%s: ok\n", *path)
	return exitClean
}
