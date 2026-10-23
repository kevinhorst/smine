package planrules

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	roleDerived = "derived"
	roleMaster  = "master"
)

// The master link; group 1 is the path.
var argsMasterPattern = regexp.MustCompile(`master: \[[^\]]*\]\(([^)#]+)\)`)

// The role on the args line; group 1 is the role.
var argsRolePattern = regexp.MustCompile("role: `([a-z]+)`")

// The first non-empty line after the title; "" when unreadable or absent.
func argsLine(planFile string) string {
	file, err := os.Open(planFile)
	if err != nil {
		return ""
	}
	defer file.Close()

	isPastTitle := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "":
			continue
		case !isPastTitle:
			isPastTitle = strings.HasPrefix(line, "# ")
			continue
		}
		return line
	}
	return ""
}

// planRole is the role the plan file's args line names; "" when it names none.
func planRole(planFile string) string {
	match := argsRolePattern.FindStringSubmatch(argsLine(planFile))
	if match == nil {
		return ""
	}
	return match[1]
}

// In plan dir order; nil when masterFile is no master plan.
func DerivedPlans(masterFile string) []string {
	if planRole(masterFile) != roleMaster {
		return nil
	}

	master := filepath.Clean(masterFile)
	featureFolder := filepath.Dir(filepath.Dir(filepath.Dir(master)))
	candidates, err := filepath.Glob(filepath.Join(featureFolder, "*-wip-*", "design", "*"+planFileSuffix))
	if err != nil {
		return nil
	}

	derived := make([]string, 0)
	for _, candidate := range candidates {
		if candidate == master || planRole(candidate) != roleDerived {
			continue
		}

		link := argsMasterPattern.FindStringSubmatch(argsLine(candidate))
		isLinked := link != nil && filepath.Join(filepath.Dir(candidate), link[1]) == master
		if isLinked {
			derived = append(derived, candidate)
		}
	}
	return derived
}
