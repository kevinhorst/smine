// Package reach defines the deployment-reach grammar shared by ACDSL rules and context entries.
package reach

import (
	"regexp"
	"strings"
)

const (
	// Always ships only into the machine-global always pack, never into a repo pack.
	Always = "always"
	Global = "global"
	// None governs no files locally and deploys nowhere.
	None = "none"
	// ThisRepo is an ordinary name that never matches a sync target, so its entries never deploy.
	ThisRepo = "smine"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func isReservedToken(name string) bool {
	return name == Always || name == Global || name == None
}

func Valid(value string) bool {
	if isReservedToken(value) {
		return true
	}
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" || isReservedToken(name) || !nameRe.MatchString(name) {
			return false
		}
	}
	return true
}

func Names(value string) []string {
	if isReservedToken(value) || value == "" {
		return nil
	}
	var names []string
	for _, name := range strings.Split(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func DeploysTo(value, target string) bool {
	if value == Global {
		return true
	}
	for _, name := range Names(value) {
		if name == target {
			return true
		}
	}
	return false
}
