package server

import (
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/stretchr/testify/assert"
)

func TestFeatureRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []featureRow

		switches *sessionstate.Switches
	}

	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	globalTag := []mechanismTag{kindTag(contextrecord.KindGlobal, "")}
	tests := make([]*testCase, 0)

	// empty-record-every-feature-on
	tests = append(tests, &testCase{
		_id:       "empty-record-every-feature-on",
		_expected: catalogRows(nil),

		switches: &sessionstate.Switches{},
	})

	// disabled-then-enabled-was-off
	wasOff := featureRow{
		Events:  []featureEvent{{Action: "disabled", At: "2000-01-01 00:00:00"}, {Action: "enabled", At: "2000-01-01 01:00:00"}},
		Feature: "inject:global",
		Hint:    "inject:global was disabled at 2000-01-01 00:00:00 — injections may be missing",
		Hook:    "globalcontext (SessionStart, SubagentStart)",
		State:   "was off",
		Tags:    globalTag,
	}
	tests = append(tests, &testCase{
		_id:       "disabled-then-enabled-was-off",
		_expected: catalogRows(map[string]featureRow{"inject:global": wasOff}),

		switches: &sessionstate.Switches{History: []sessionstate.SwitchEvent{
			{Action: sessionstate.SwitchDisabled, At: syncNow, Feature: sessionstate.InjectGlobal},
			{Action: sessionstate.SwitchEnabled, At: syncNow.Add(time.Hour), Feature: sessionstate.InjectGlobal},
		}},
	})

	// off-without-history-warns
	offOnly := featureRow{
		Events:  nil,
		Feature: "rgate:global",
		Hint:    "rgate:global is off for this session — injections may be missing",
		Hook:    "readgate (PreToolUse)",
		State:   "off",
		Tags:    globalTag,
	}
	tests = append(tests, &testCase{
		_id:       "off-without-history-warns",
		_expected: catalogRows(map[string]featureRow{"rgate:global": offOnly}),

		switches: &sessionstate.Switches{Off: []sessionstate.Feature{sessionstate.RgateGlobal}},
	})

	// guide-member-listed-after-catalog
	guideRow := featureRow{
		Events:  []featureEvent{{Action: "disabled", At: "2000-01-01 00:00:00"}},
		Feature: "rgate:go",
		Hint:    "rgate:go was disabled at 2000-01-01 00:00:00 — injections may be missing",
		Hook:    "readgate (PreToolUse)",
		State:   "off",
		Tags:    []mechanismTag{kindTag(contextrecord.KindLang, "")},
	}
	tests = append(tests, &testCase{
		_id:       "guide-member-listed-after-catalog",
		_expected: append(catalogRows(nil), guideRow),

		switches: &sessionstate.Switches{
			History: []sessionstate.SwitchEvent{{Action: sessionstate.SwitchDisabled, At: syncNow, Feature: "rgate:go"}},
			Off:     []sessionstate.Feature{"rgate:go"},
		},
	})

	// pre-grammar-name-listed-without-tags
	legacyRow := featureRow{
		Events:  nil,
		Feature: "acdsl-projection",
		Hint:    "acdsl-projection is off for this session — injections may be missing",
		Hook:    "",
		State:   "off",
		Tags:    nil,
	}
	tests = append(tests, &testCase{
		_id:       "pre-grammar-name-listed-without-tags",
		_expected: append(catalogRows(nil), legacyRow),

		switches: &sessionstate.Switches{Off: []sessionstate.Feature{"acdsl-projection"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, featureRows(test.switches, time.UTC))
		})
	}
}

func TestSectionTags(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedHooks   []mechanismTag
		_expectedSession []mechanismTag

		hints    map[string]string
		hookRows []sectionRow
	}

	harness := mechanismTag{Class: tagKindHook, Text: "harness"}
	acdslContext := mechanismTag{Class: tagKindHook, Text: "acdsl-context"}
	title := mechanismTag{Class: tagKindHook, Text: "sessiontitle"}
	policy := mechanismTag{Class: tagKindHook, Text: "tool-policy"}
	global := kindTag(contextrecord.KindGlobal, "")
	tests := make([]*testCase, 0)

	// top-three-by-summed-tokens
	tests = append(tests, &testCase{
		_id:              "top-three-by-summed-tokens",
		_expectedHooks:   []mechanismTag{acdslContext, harness, title},
		_expectedSession: []mechanismTag{global},

		hints: map[string]string{},
		hookRows: []sectionRow{
			{Tag: harness, tokensValue: 500},
			{Tag: acdslContext, tokensValue: 300},
			{Tag: acdslContext, tokensValue: 300},
			{Tag: title, tokensValue: 40},
			{Tag: policy, tokensValue: 10},
		},
	})

	// warned-tags-carry-the-hint
	warnedTitle := mechanismTag{Class: tagKindHook, Text: "sessiontitle", Warn: "session:title is off"}
	warnedGlobal := mechanismTag{Class: global.Class, Text: global.Text, Warn: "inject:global is off"}
	tests = append(tests, &testCase{
		_id:              "warned-tags-carry-the-hint",
		_expectedHooks:   []mechanismTag{warnedTitle},
		_expectedSession: []mechanismTag{warnedGlobal},

		hints:    map[string]string{"global-context": "inject:global is off", "sessiontitle": "session:title is off"},
		hookRows: []sectionRow{{Tag: title, tokensValue: 40}},
	})

	// no-hook-rows-no-hook-tags
	tests = append(tests, &testCase{
		_id:              "no-hook-rows-no-hook-tags",
		_expectedHooks:   []mechanismTag{},
		_expectedSession: []mechanismTag{global},

		hints:    map[string]string{},
		hookRows: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			tags := sectionTags(test.hints, test.hookRows)
			assert.Equal(t, test._expectedHooks, tags[sizeKeyHooks])
			assert.Equal(t, test._expectedSession, tags[string(contextrecord.KindGlobal)])
		})
	}
}

func TestWarnHints(t *testing.T) {
	rows := []featureRow{
		{Feature: "inject:global", Hint: "inject:global is off", Tags: []mechanismTag{kindTag(contextrecord.KindGlobal, "")}},
		{Feature: "rgate:global", Hint: "rgate:global is off", Tags: []mechanismTag{kindTag(contextrecord.KindGlobal, "")}},
		{Feature: "hb", Hint: "", Tags: nil},
	}

	assert.Equal(t, map[string]string{"global-context": "inject:global is off; rgate:global is off"}, warnHints(rows))
}

// catalogRows is every catalog feature on, with the given rows in place of their feature's.
func catalogRows(replaced map[string]featureRow) []featureRow {
	rows := make([]featureRow, 0)
	for _, entry := range sessionstate.Catalog() {
		row := featureRow{Feature: string(entry.Feature), Hook: entry.Hook, State: "on", Tags: featureTags(entry.Feature)}
		if replacement, isReplaced := replaced[string(entry.Feature)]; isReplaced {
			row = replacement
		}
		rows = append(rows, row)
	}
	return rows
}
