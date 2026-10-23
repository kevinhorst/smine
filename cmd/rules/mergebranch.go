package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	mergeBranchPrefix = "claude-merge/"
	mergeBranchUsage  = "usage: rules merge-branch create <ours> <theirs> | cleanup <ours>"
	mergeExitUsage    = 2
)

// slugBreakPattern is every run of characters a branch slug replaces.
var slugBreakPattern = regexp.MustCompile(`[^a-z0-9]+`)

func runMergeBranch(args []string) int {
	ctx := context.Background()
	switch {
	case len(args) == 3 && args[0] == "create":
		return createMergeBranch(ctx, args[1], args[2])
	case len(args) == 2 && args[0] == "cleanup":
		return cleanupMergeBranches(ctx, args[1])
	}
	fmt.Fprintln(os.Stderr, mergeBranchUsage)
	return mergeExitUsage
}

func cleanupMergeBranches(ctx context.Context, ours string) int {
	if output, err := shell.Run(ctx, "", "git", "checkout", ours); err != nil {
		fmt.Fprint(os.Stderr, output)
		return exitError
	}

	refs, err := shell.Run(ctx, "", "git", "for-each-ref", "--format=%(refname:short)", "refs/heads/"+mergeBranchPrefix+"*")
	if err != nil {
		fmt.Fprint(os.Stderr, refs)
		return exitError
	}

	for branch := range strings.FieldsSeq(refs) {
		if _, err := shell.Run(ctx, "", "git", "merge-base", "--is-ancestor", branch, ours); err != nil {
			fmt.Printf("kept (not merged into %s): %s\n", ours, branch)
			continue
		}

		if output, err := shell.Run(ctx, "", "git", "branch", "-d", branch); err != nil {
			fmt.Fprint(os.Stderr, output)
			return exitError
		}
		fmt.Printf("deleted: %s\n", branch)
	}
	return exitClean
}

func createMergeBranch(ctx context.Context, ours string, theirs string) int {
	branch := mergeBranchPrefix + slug(theirs) + "-" + slug(ours)
	output, err := shell.Run(ctx, "", "git", "checkout", "-b", branch, ours)
	fmt.Fprint(os.Stderr, output)
	if err != nil {
		return exitError
	}

	fmt.Println(branch)
	return exitClean
}

// slug lowercases a branch name and joins its alphanumeric runs with "-".
func slug(name string) string {
	return strings.Trim(slugBreakPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
}
