package main

import (
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/kevinhorst/smine/internal/hookconfig"
)

func gitExecCause(causes *hookconfig.ToolpolicyCauses, rest []string) string {
	for _, sequence := range gitExecSubcommands {
		if len(rest) >= len(sequence) && slices.Equal(rest[:len(sequence)], sequence) {
			return causes.GitExec
		}
	}
	if len(rest) == 0 {
		return ""
	}

	options := gitExecOptions[rest[0]]
	for _, arg := range rest[1:] {
		if arg == gitFlagsEnd {
			return ""
		}
		for _, option := range options {
			if isGitOptionUse(arg, option) {
				return causes.GitExec
			}
		}
	}
	return ""
}

func gitGlobalsEnd(args []string, gitAdmittedFlags []string) (int, bool) {
	index := 0
	for index < len(args) {
		arg := args[index]
		switch {
		case arg == gitDirectoryFlag:
			if index+1 >= len(args) {
				return index, false
			}
			index += gitOptionWithValueTokens
		case slices.Contains(gitAdmittedFlags, arg):
			index++
		case strings.HasPrefix(arg, "-"):
			return index, false
		default:
			return index, true
		}
	}
	return index, true
}

func gitRuleCommand(command string, gitAdmittedFlags []string) string {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil || len(file.Stmts) != 1 {
		return command
	}

	call, isCall := file.Stmts[0].Cmd.(*syntax.CallExpr)
	if !isCall || len(call.Assigns) > 0 {
		return command
	}

	words := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		literal, isLiteral := wordLiteral(word)
		if !isLiteral {
			break
		}
		words = append(words, literal)
	}
	if len(words) == 0 || words[0] != gitHead {
		return command
	}

	end, isAdmitted := gitGlobalsEnd(words[1:], gitAdmittedFlags)
	subcommandIndex := end + 1
	isCuttable := isAdmitted && end > 0 && subcommandIndex < len(words)
	if !isCuttable {
		return command
	}

	cutStart := int(call.Args[1].Pos().Offset())
	cutEnd := int(call.Args[subcommandIndex].Pos().Offset())
	return command[:cutStart] + command[cutEnd:]
}

func gitSubcommandWords(args []string, gitAdmittedFlags []string) []string {
	end, isAdmitted := gitGlobalsEnd(args, gitAdmittedFlags)
	if !isAdmitted {
		return nil
	}
	return args[end:]
}

func isRefusedGitEnvironment(pair *argWord) bool {
	name, _, _ := strings.Cut(pair.text, "=")
	return strings.HasPrefix(name, gitEnvironmentPrefix) && name != gitIndexFileName
}

func isGitOptionUse(arg string, option string) bool {
	if arg == option || strings.HasPrefix(arg, option+"=") {
		return true
	}

	isShortOption := len(option) == gitShortOptionLength && !strings.HasPrefix(option, gitFlagsEnd)
	isShortCluster := strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, gitFlagsEnd)
	return isShortOption && isShortCluster && strings.Contains(arg[1:], option[1:])
}
