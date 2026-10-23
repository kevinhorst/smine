package main

import (
	"net/url"
	"strings"

	"github.com/kevinhorst/smine/internal/hookconfig"
)

// An unparseable URL carries no credentials the browser would submit.
func hasLogin(target string) bool {
	parsed, err := url.Parse(target)
	if err != nil {
		return false
	}
	return parsed.User != nil
}

func loginCause(causes *hookconfig.ToolpolicyCauses, input *HookInput) string {
	if !strings.HasPrefix(input.ToolName, browserPanePrefix) {
		return ""
	}

	for _, target := range navigationTargets(&input.ToolInput) {
		if hasLogin(target) {
			return causes.PaneLogin
		}
	}
	return ""
}

func navigationTargets(input *ToolInput) []string {
	targets := make([]string, 0, len(input.Actions)+1)
	if input.Url != "" {
		targets = append(targets, input.Url)
	}
	for _, action := range input.Actions {
		if action.Input.Url != "" {
			targets = append(targets, action.Input.Url)
		}
	}
	return targets
}
