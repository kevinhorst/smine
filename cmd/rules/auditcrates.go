package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/reviewscope"
)

const (
	auditScopeAll     = "all"
	auditScopeChanged = "changed"

	crateAgentwt   = "agentwt"
	crateHeartbeat = "heartbeat"
)

// auditCrates lists the crates in dependency order; heartbeat links agentwt by path.
var auditCrates = []string{crateAgentwt, crateHeartbeat}

// crateDependents maps a changed crate to every crate the change reaches, itself included.
var crateDependents = map[string][]string{
	crateAgentwt:   {crateAgentwt, crateHeartbeat},
	crateHeartbeat: {crateHeartbeat},
}

// changedPaths is the branch's change set against its derived base plus the working tree; hasBase is false when no base resolves.
func changedPaths(ctx context.Context, mainline, bracket string) ([]string, bool) {
	base, err := reviewscope.DeriveBase(ctx, ".", "HEAD", mainline, bracket)
	if err != nil {
		return nil, false
	}
	changed, err := acdsl.GitChangedFiles(ctx, ".", base.Commit)
	if err != nil {
		return nil, false
	}
	return changed, true
}

func runAuditCrates(args []string, out io.Writer) int {
	flags := flag.NewFlagSet("audit-crates", flag.ContinueOnError)
	bracket := flags.String("base", "", "explicit base ref; absent = the upstream fork point, else the merge-base with the mainline")
	mainline := flags.String("mainline", "main", "mainline branch")
	scope := flags.String("scope", auditScopeChanged, "changed | all")
	if err := flags.Parse(args); err != nil {
		return parseExit(err)
	}
	isKnownScope := *scope == auditScopeAll || *scope == auditScopeChanged
	if !isKnownScope {
		fmt.Fprintf(os.Stderr, "rules: audit-crates: -scope must be changed or all, got %q\n", *scope)
		return exitError
	}

	changed, hasBase := changedPaths(context.Background(), *mainline, *bracket)
	fmt.Fprintln(out, strings.Join(selectCrates(*scope, changed, hasBase), " "))
	return exitClean
}

// selectCrates is fail-closed: every case the tool cannot judge is every crate.
func selectCrates(scope string, changed []string, hasBase bool) []string {
	isUnjudgeable := scope == auditScopeAll || !hasBase || len(changed) == 0
	if isUnjudgeable {
		return auditCrates
	}

	selected := map[string]bool{}
	for _, path := range changed {
		for crate, dependents := range crateDependents {
			if !strings.HasPrefix(path, "cmd/"+crate+"/") {
				continue
			}
			for _, dependent := range dependents {
				selected[dependent] = true
			}
		}
	}

	crates := make([]string, 0, len(auditCrates))
	for _, crate := range auditCrates {
		if selected[crate] {
			crates = append(crates, crate)
		}
	}
	return crates
}
