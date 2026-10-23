package main

import (
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/toolpolicy"
)

var sedWriteCommand = regexp.MustCompile(`(^|[;[:space:]])[wW]([[:space:]]|$)`)

// hasNonLiteralArg fails flag-checked heads closed: an expansion could smuggle a write flag past the literal checks.
func isReadOnlyCall(args []string, hasNonLiteralArg bool, head string, policy *toolpolicy.Policy) bool {
	listing, isListed := policy.ReadOnlyListing(head)
	if !isListed {
		return false
	}
	if !listing.IsBare {
		return isReadOnlySubcommandCall(args, hasNonLiteralArg, head, listing.Subcommands, policy.GitAdmittedFlags)
	}

	switch head {
	case "find":
		// find's write/exec switches are always literal; a non-literal positional (find $DIR) stays benign.
		return isReadOnlyFind(args)
	case "sed":
		if hasNonLiteralArg {
			return false
		}
		return isReadOnlySed(args)
	}
	return true
}

// isReadOnlyFind rejects every find flag that executes or writes.
func isReadOnlyFind(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-exec", "-execdir", "-ok", "-okdir", "-delete":
			return false
		}
		if strings.HasPrefix(arg, "-fprint") {
			return false
		}
	}
	return true
}

func isReadOnlySed(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-i") {
			return false
		}
		if !strings.HasPrefix(arg, "-") && sedWriteCommand.MatchString(arg) {
			return false
		}
	}
	return true
}

func isReadOnlySubcommandCall(args []string, hasNonLiteralArg bool, head string, subcommands []string, gitAdmittedFlags []string) bool {
	if hasNonLiteralArg {
		return false
	}
	if !matchesReadOnlySubcommand(head, subcommands, args, gitAdmittedFlags) {
		return false
	}

	switch head {
	case "go":
		return isReadOnlyGoEnv(args)
	case "git":
		return isReadOnlyGitBranch(gitSubcommandWords(args, gitAdmittedFlags))
	}
	return true
}

// isReadOnlyGoEnv rejects go env write flags that persist to the GOENV file.
func isReadOnlyGoEnv(args []string) bool {
	if len(args) == 0 || args[0] != "env" {
		return true
	}
	for _, arg := range args[1:] {
		if slices.Contains(goEnvWriteFlags, arg) {
			return false
		}
	}
	return true
}

func isReadOnlyGitBranch(args []string) bool {
	if len(args) == 0 || args[0] != "branch" {
		return true
	}
	hasListingFlag := false
	hasPositional := false
	for _, arg := range args[1:] {
		flag, _, _ := strings.Cut(arg, "=")
		if slices.Contains(gitBranchMutatingFlags, flag) {
			return false
		}
		if slices.Contains(gitBranchListingFlags, flag) {
			hasListingFlag = true
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			hasPositional = true
		}
	}
	return !hasPositional || hasListingFlag
}

func isSafePathFlagCall(call *resolvedCall) bool {
	letters, isPathFlagHead := fileCommandSafeFlags[call.head]
	if !isPathFlagHead {
		return false
	}

	hasFlag := false
	for _, arg := range call.args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		cluster := strings.TrimPrefix(arg, "-")
		if cluster == "" || strings.Trim(cluster, letters) != "" {
			return false
		}
		hasFlag = true
	}
	return hasFlag
}

func matchesReadOnlySubcommand(head string, subcommands []string, args []string, gitAdmittedFlags []string) bool {
	remaining := args
	if head == "git" {
		remaining = gitSubcommandWords(args, gitAdmittedFlags)
	}
	if len(remaining) == 0 {
		return false
	}

	for _, entry := range subcommands {
		entryTokens := strings.Fields(entry)
		if len(entryTokens) > len(remaining) {
			continue
		}

		matches := true
		for i, token := range entryTokens {
			if remaining[i] != token {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}
