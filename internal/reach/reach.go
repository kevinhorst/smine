// Package reach defines the deployment-reach grammar shared by ACDSL rules
// and context entries: "global" binds in any repo; "none" disables the
// entry — it governs no files locally and deploys nowhere; a comma-separated
// repo-name list deploys to exactly those targets (names = target dir
// basename, the roster convention). There is no self-keyword — this repo is
// referenced by its own name, smine, which never matches a sync target and
// therefore never deploys.
package reach

import (
	"regexp"
	"strings"
)

const (
	Global = "global"
	// None disables an entry or rule: it governs no files locally and
	// deploys nowhere. A reserved token like Global — never a list member.
	None = "none"
	// ThisRepo is this repository's roster name — the reach value for
	// "binds only here". An ordinary name, not a keyword: it never matches
	// a sync target, so such entries never deploy.
	ThisRepo = "smine"
)

// nameRe matches one repo-name token; "global" and "none" are reserved.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Valid reports whether value is global, none, or a non-empty
// comma-separated list of repo-name tokens.
func Valid(value string) bool {
	if value == Global || value == None {
		return true
	}
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" || name == Global || name == None || !nameRe.MatchString(name) {
			return false
		}
	}
	return true
}

// Names returns the repo names in a list value, trimmed; empty for global,
// none, and the empty value.
func Names(value string) []string {
	if value == Global || value == None || value == "" {
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

// DeploysTo reports whether an entry with this reach ships to target:
// global always, a name list when it contains target, empty never.
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
