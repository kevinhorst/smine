package server

import (
	"cmp"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	featureStateOff       = "off"
	featureStateOn        = "on"
	featureStateWasOff    = "was off"
	featureTimeLayout     = "2006-01-02 15:04:05"
	hintSeparator         = "; "
	hookLabelSessionTitle = "sessiontitle"
	hookTagLimit          = 3
)

// loadSessionFeatures reads the gate's record of the session; a missing record lists every feature on.
func (s *Server) loadSessionFeatures(page *sessionReportPage, id string) {
	state, err := sessionstate.Load(sessionstate.File(sessionstate.StateDir(s.claudeHome), id))
	if err != nil {
		log.Printf("Server.loadSessionFeatures: Session %s: %v", id, err)
		page.FeaturesErr = err.Error()
		return
	}

	page.Features = featureRows(&state.Switches, time.Local)
}

// appendHint joins a second warned feature's hint onto a tag that already warns.
func appendHint(hints map[string]string, text string, hint string) {
	existing := hints[text]
	if existing == "" {
		hints[text] = hint
		return
	}

	hints[text] = existing + hintSeparator + hint
}

// featureHint names why a warned feature's injections may be missing; an unwarned feature has none.
func featureHint(disabledAt []string, feature sessionstate.Feature, isOff bool) string {
	switch {
	case len(disabledAt) > 0:
		return fmt.Sprintf("%s was disabled at %s — injections may be missing", feature, strings.Join(disabledAt, ", "))
	case isOff:
		return fmt.Sprintf("%s is off for this session — injections may be missing", feature)
	}
	return ""
}

// featureRows lists the catalog in help order, then every guide member the record names.
func featureRows(switches *sessionstate.Switches, location *time.Location) []featureRow {
	hooks := map[sessionstate.Feature]string{}
	features := make([]sessionstate.Feature, 0)
	for _, entry := range sessionstate.Catalog() {
		hooks[entry.Feature] = entry.Hook
		features = append(features, entry.Feature)
	}

	recorded := recordedFeatures(hooks, switches)
	features = append(features, recorded...)
	rows := make([]featureRow, 0, len(features))
	for _, feature := range features {
		rows = append(rows, newFeatureRow(feature, featureHook(feature, hooks), location, switches))
	}
	return rows
}

// featureHook is the catalog's hook, the read gate for a guide member, empty for a name the grammar no longer has.
func featureHook(feature sessionstate.Feature, hooks map[sessionstate.Feature]string) string {
	hook, isCatalog := hooks[feature]
	switch {
	case isCatalog:
		return hook
	case isGuideMember(feature):
		return sessionstate.GuideEntry.Hook
	}
	return ""
}

// featureTags are the report tags of the injections a feature feeds; a recorded name outside the grammar feeds none.
func featureTags(feature sessionstate.Feature) []mechanismTag {
	switch feature {
	case sessionstate.InjectGlobal, sessionstate.RgateGlobal:
		return []mechanismTag{kindTag(contextrecord.KindGlobal, "")}
	case sessionstate.InjectSkill, sessionstate.RgateInvoked:
		return []mechanismTag{kindTag(contextrecord.KindSkill, "")}
	case sessionstate.InjectAcdsl:
		return []mechanismTag{kindTag(contextrecord.KindAcdslProjection, ""), kindTag(contextrecord.KindAcdslContext, "")}
	case sessionstate.InjectPlan, sessionstate.RgatePlan:
		return []mechanismTag{kindTag(contextrecord.KindPlanContext, "")}
	case sessionstate.SessionTitle:
		return []mechanismTag{kindTag(contextrecord.KindHookContext, hookLabelSessionTitle)}
	case sessionstate.Heartbeat, sessionstate.InjectEnv, sessionstate.SkillEnforce, sessionstate.SkillInvokegate:
		return nil
	}
	if isGuideMember(feature) {
		return []mechanismTag{kindTag(contextrecord.KindLang, "")}
	}
	return nil
}

// isGuideMember: a read-gate member outside the catalog, `rgate:<ext|guide>`.
func isGuideMember(feature sessionstate.Feature) bool {
	return strings.HasPrefix(string(feature), sessionstate.GroupRgate+":")
}

// recordedFeatures are the features the record names outside the catalog — guide members and pre-grammar names — sorted.
func recordedFeatures(catalog map[sessionstate.Feature]string, switches *sessionstate.Switches) []sessionstate.Feature {
	named := map[sessionstate.Feature]bool{}
	for _, feature := range switches.Off {
		named[feature] = true
	}
	for _, event := range switches.History {
		named[event.Feature] = true
	}

	recorded := make([]sessionstate.Feature, 0)
	for feature := range named {
		_, isCatalog := catalog[feature]
		if !isCatalog {
			recorded = append(recorded, feature)
		}
	}
	slices.Sort(recorded)
	return recorded
}

// largestHookTags: the hook rows' distinct tags by summed tokens, largest first, at most three.
func largestHookTags(rows []sectionRow) []mechanismTag {
	tokens := map[string]int{}
	byText := map[string]mechanismTag{}
	for _, row := range rows {
		tokens[row.Tag.Text] += row.tokensValue
		byText[row.Tag.Text] = row.Tag
	}

	texts := slices.Collect(maps.Keys(byText))
	slices.SortFunc(texts, func(a, b string) int { return cmp.Or(cmp.Compare(tokens[b], tokens[a]), strings.Compare(a, b)) })
	largest := texts[:min(len(texts), hookTagLimit)]
	tags := make([]mechanismTag, 0, len(largest))
	for _, text := range largest {
		tags = append(tags, byText[text])
	}
	return tags
}

// newFeatureRow folds the feature's history; one disable or the off set warns, a later enable never clears it.
func newFeatureRow(feature sessionstate.Feature, hook string, location *time.Location, switches *sessionstate.Switches) featureRow {
	row := featureRow{Feature: string(feature), Hook: hook, State: featureStateOn, Tags: featureTags(feature)}
	disabledAt := make([]string, 0)
	for _, event := range switches.History {
		if event.Feature != feature {
			continue
		}

		at := event.At.In(location).Format(featureTimeLayout)
		change := featureEvent{Action: string(event.Action), At: at}
		row.Events = append(row.Events, change)
		if event.Action == sessionstate.SwitchDisabled {
			disabledAt = append(disabledAt, at)
		}
	}

	isOff := slices.Contains(switches.Off, feature)
	switch {
	case isOff:
		row.State = featureStateOff
	case len(disabledAt) > 0:
		row.State = featureStateWasOff
	}
	row.Hint = featureHint(disabledAt, feature, isOff)
	return row
}

// sectionTags are each section header's tags with their warnings; the hook header shows its three largest hooks.
func sectionTags(hints map[string]string, hookRows []sectionRow) map[string][]mechanismTag {
	tags := map[string][]mechanismTag{
		string(contextrecord.KindAcdslProjection): {kindTag(contextrecord.KindAcdslProjection, "")},
		string(contextrecord.KindGlobal):          {kindTag(contextrecord.KindGlobal, "")},
		string(contextrecord.KindPlanRules):       {kindTag(contextrecord.KindPlanRules, "")},
		string(contextrecord.KindSkill):           {kindTag(contextrecord.KindSkill, "")},
		string(contextrecord.KindSubagentPrompt):  {kindTag(contextrecord.KindSubagentPrompt, "")},
		sizeKeyFileReads:                          {kindTag(contextrecord.KindPackRead, ""), kindTag(contextrecord.KindLang, "")},
		sizeKeyHooks:                              largestHookTags(hookRows),
	}
	for _, list := range tags {
		for index := range list {
			list[index].Warn = hints[list[index].Text]
		}
	}
	return tags
}

// warnHints maps each tag text to the hints of every warned feature feeding it.
func warnHints(rows []featureRow) map[string]string {
	hints := map[string]string{}
	for _, row := range rows {
		if row.Hint == "" {
			continue
		}

		for _, tag := range row.Tags {
			appendHint(hints, tag.Text, row.Hint)
		}
	}
	return hints
}

// warnHookRows marks each hook row whose tag a warned feature feeds.
func warnHookRows(hints map[string]string, rows []sectionRow) {
	for index := range rows {
		rows[index].Tag.Warn = hints[rows[index].Tag.Text]
	}
}
