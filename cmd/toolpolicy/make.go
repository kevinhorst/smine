package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/hookconfig"
)

var (
	// makeHelpLine is a target's help comment, "## <target>: <text>".
	makeHelpLine = regexp.MustCompile(`^## ([A-Za-z0-9][A-Za-z0-9_.-]*): (.*)$`)
	// makeParamWord is a NAME= parameter named in a help text.
	makeParamWord = regexp.MustCompile(`\b([A-Z][A-Z0-9_]*)=`)
	// makeTargetPattern admits a bare make target word.
	makeTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	// makeValuePattern admits a value that cannot break out of the recipe line it expands into.
	makeValuePattern = regexp.MustCompile(`^[A-Za-z0-9 _./,:=@+-]*$`)
	// makeVariablePattern admits a make variable name.
	makeVariablePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type makeInvocation struct {
	assignments map[string]string
	dir         string
	targets     []string
}

func (m *makeInvocation) readAssignment(arg string) bool {
	name, value, _ := strings.Cut(arg, "=")
	if !makeVariablePattern.MatchString(name) || !makeValuePattern.MatchString(value) {
		return false
	}
	m.assignments[name] = value
	return true
}

func (m *makeInvocation) readLongFlag(arg string, home string, next []string) (int, bool) {
	name, value, hasValue := strings.Cut(arg, "=")
	isLongName := len(name) > len("--d")
	isCodeFlag := isLongName && slices.ContainsFunc(makeCodeLongFlags, func(flag string) bool {
		return strings.HasPrefix(flag, name)
	})
	if isCodeFlag {
		return 0, false
	}
	if !isLongName || !strings.HasPrefix(makeDirectoryFlag, name) {
		return 0, true
	}

	consumed := 0
	if !hasValue {
		if len(next) == 0 {
			return 0, false
		}
		value = next[0]
		consumed = 1
	}
	m.dir = resolveMakeDir(m.dir, value, home)
	return consumed, true
}

func (m *makeInvocation) readShortFlags(cluster string, home string, next []string) (int, bool) {
	for index, letter := range cluster {
		if strings.ContainsRune(makeCodeShortFlags, letter) {
			return 0, false
		}
		if !strings.ContainsRune(makeValueShortFlags, letter) {
			continue
		}

		value := cluster[index+1:]
		consumed := 0
		if value == "" {
			if len(next) == 0 {
				return 0, false
			}
			value = next[0]
			consumed = 1
		}
		if letter == 'C' {
			m.dir = resolveMakeDir(m.dir, value, home)
		}
		return consumed, true
	}
	return 0, true
}

func isDeclaredMakeParam(params map[string][]string, targets []string, name string) bool {
	return slices.ContainsFunc(targets, func(target string) bool {
		return slices.Contains(params[target], name)
	})
}

func isMakeSource(path string, makefileNames []string) bool {
	base := filepath.Base(path)
	return slices.Contains(makefileNames, base) || strings.HasSuffix(base, makeIncludeSuffix)
}

func isOperatorMakeCall(call *resolvedCall, cwd string, home string, makefileNames []string) bool {
	invocation, isParsed := parseMakeArgs(call.args, cwd, home)
	if !isParsed {
		return false
	}

	operators := makeOperatorTargets(readMakefile(invocation.dir, makefileNames))
	return slices.ContainsFunc(invocation.targets, func(target string) bool {
		return slices.Contains(operators, target)
	})
}

func isSafeMakeCall(call *resolvedCall, cwd string, home string, makefileNames []string) bool {
	if call.hasNonLiteralArg {
		return false
	}

	invocation, isParsed := parseMakeArgs(call.args, cwd, home)
	if !isParsed {
		return false
	}
	if len(invocation.assignments) == 0 {
		return true
	}

	params := makeParams(readMakefile(invocation.dir, makefileNames))
	for name := range invocation.assignments {
		if !isDeclaredMakeParam(params, invocation.targets, name) {
			return false
		}
	}
	return true
}

func makeParams(makefile string) map[string][]string {
	params := map[string][]string{}
	for line := range strings.SplitSeq(makefile, "\n") {
		match := makeHelpLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, word := range makeParamWord.FindAllStringSubmatch(match[2], -1) {
			params[match[1]] = append(params[match[1]], word[1])
		}
	}
	return params
}

func makeOperatorTargets(makefile string) []string {
	targets := make([]string, 0)
	for line := range strings.SplitSeq(makefile, "\n") {
		match := makeHelpLine.FindStringSubmatch(line)
		isOperator := match != nil && strings.Contains(match[2], makeOperatorTag)
		if isOperator {
			targets = append(targets, match[1])
		}
	}
	return targets
}

func parseMakeArgs(args []string, cwd string, home string) (*makeInvocation, bool) {
	invocation := &makeInvocation{
		assignments: map[string]string{},
		dir:         cwd,
		targets:     make([]string, 0, len(args)),
	}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		consumed := 0
		isSafe := true
		switch {
		case strings.HasPrefix(arg, "--"):
			consumed, isSafe = invocation.readLongFlag(arg, home, args[index+1:])
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			consumed, isSafe = invocation.readShortFlags(arg[1:], home, args[index+1:])
		case strings.Contains(arg, "="):
			isSafe = invocation.readAssignment(arg)
		case makeTargetPattern.MatchString(arg):
			invocation.targets = append(invocation.targets, arg)
		default:
			isSafe = false
		}
		if !isSafe {
			return nil, false
		}
		index += consumed
	}
	return invocation, true
}

func readMakefile(dir string, makefileNames []string) string {
	for _, name := range makefileNames {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return string(raw)
		}
	}
	return ""
}

func resolveMakeDir(base string, value string, home string) string {
	target := hookconfig.ExpandHome(value, home)
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(base, target)
}
