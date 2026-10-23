package main

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/hookconfig"
)

// entryIds lists the entries' ids in order.
func entryIds(entries []contextdocs.RuleEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Id)
	}
	return ids
}

func header(delta []contextdocs.RuleEntry, lane string, repeat []contextdocs.RuleEntry, section *hookconfig.Skillcontext, skill string, version string) string {
	laneSuffix := ""
	if lane != "" {
		laneSuffix = " " + lane
	}

	lines := []string{fmt.Sprintf(section.BannerFormat, skill, laneSuffix, cmp.Or(version, missingVersion))}
	for _, entry := range delta {
		lines = append(lines, "- **"+entry.Id+"**")
	}
	if len(repeat) > 0 {
		lines = append(lines, fmt.Sprintf(section.RepeatFormat, len(repeat), strings.Join(entryIds(repeat), ", ")))
	}
	return strings.Join(lines, "\n")
}
