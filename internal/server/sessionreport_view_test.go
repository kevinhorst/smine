package server

import (
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/stretchr/testify/assert"
)

func TestItemTokenCell(t *testing.T) {
	type testCase struct {
		_id       string
		_expected tokenCell

		item *contextrecord.Item
	}

	tests := make([]*testCase, 0)

	// exact
	tests = append(tests, &testCase{
		_id:       "exact",
		_expected: tokenCell{Text: "1 234"},

		item: &contextrecord.Item{Counter: contextrecord.CounterAnthropic, Tokens: 1234},
	})

	// approx
	tests = append(tests, &testCase{
		_id:       "approx",
		_expected: tokenCell{Text: "≈1 234"},

		item: &contextrecord.Item{Approx: true, Counter: contextrecord.CounterO200k, Tokens: 1234},
	})

	// error
	tests = append(tests, &testCase{
		_id:       "error",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordCountFailed, Title: "api 500"},

		item: &contextrecord.Item{Error: "api 500"},
	})

	// rate
	tests = append(tests, &testCase{
		_id:       "rate",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordPending, Title: "rate limit — reload to count"},

		item: &contextrecord.Item{Uncounted: contextrecord.UncountedRate},
	})

	// cap
	tests = append(tests, &testCase{
		_id:       "cap",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordPending, Title: "load cap — reload to count"},

		item: &contextrecord.Item{Uncounted: contextrecord.UncountedCap},
	})

	// queued
	tests = append(tests, &testCase{
		_id:       "queued",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordPending, Title: "not counted yet — a background run counts it"},

		item: &contextrecord.Item{Uncounted: contextrecord.UncountedQueued},
	})

	// size
	tests = append(tests, &testCase{
		_id:       "size",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordSkipped, Title: "over 200 000 characters"},

		item: &contextrecord.Item{Uncounted: contextrecord.UncountedSize},
	})

	// disabled
	tests = append(tests, &testCase{
		_id:       "disabled",
		_expected: tokenCell{Class: "badge badge-dim", Text: wordOff, Title: "token counting is off"},

		item: &contextrecord.Item{Uncounted: contextrecord.UncountedDisabled},
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: tokenCell{Text: "—"},

		item: &contextrecord.Item{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, itemTokenCell(test.item))
		})
	}
}

func TestPendingCount(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		items []contextrecord.Item
	}

	tests := make([]*testCase, 0)

	// none
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: 0,

		items: []contextrecord.Item{
			{Counter: contextrecord.CounterAnthropic, Tokens: 10},
		},
	})

	// rate-cap-and-queued-only
	tests = append(tests, &testCase{
		_id:       "rate-cap-and-queued-only",
		_expected: 3,

		items: []contextrecord.Item{
			{Uncounted: contextrecord.UncountedRate},
			{Uncounted: contextrecord.UncountedCap},
			{Uncounted: contextrecord.UncountedQueued},
			{Counter: contextrecord.CounterAnthropic, Tokens: 4, Uncounted: contextrecord.UncountedQueued},
			{Uncounted: contextrecord.UncountedDisabled},
			{Error: "api 500"},
			{Counter: contextrecord.CounterAnthropic, Tokens: 10},
		},
	})

	// size-not-pending
	tests = append(tests, &testCase{
		_id:       "size-not-pending",
		_expected: 0,

		items: []contextrecord.Item{
			{Uncounted: contextrecord.UncountedSize},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, pendingCount(test.items))
		})
	}
}

func TestKindTag(t *testing.T) {
	for _, kind := range contextrecord.Kinds {
		tag := kindTag(kind, "sessiontitle")
		assert.NotEmptyf(t, tag.Class, "kind %s", kind)
		assert.NotEmptyf(t, tag.Text, "kind %s", kind)
	}

	hookTag := kindTag(contextrecord.KindHookContext, "sessiontitle")
	assert.Equal(t, mechanismTag{Class: tagKindHook, Text: "sessiontitle"}, hookTag)
}

func TestRelativePath(t *testing.T) {
	type testCase struct {
		_id       string
		_expected pathCell

		claudeHome string
		cwd        string
		path       string
	}

	tests := make([]*testCase, 0)

	// under-cwd
	tests = append(tests, &testCase{
		_id:       "under-cwd",
		_expected: pathCell{Prefix: filepath.FromSlash("/repo/app/"), Rel: filepath.FromSlash("internal/a.go")},

		claudeHome: filepath.FromSlash("/home/.claude"),
		cwd:        filepath.FromSlash("/repo/app"),
		path:       filepath.FromSlash("/repo/app/internal/a.go"),
	})

	// under-claude-home
	tests = append(tests, &testCase{
		_id:       "under-claude-home",
		_expected: pathCell{Prefix: filepath.FromSlash("/home/.claude/"), Rel: filepath.FromSlash("context/rules/go.md")},

		claudeHome: filepath.FromSlash("/home/.claude"),
		cwd:        filepath.FromSlash("/repo/app"),
		path:       filepath.FromSlash("/home/.claude/context/rules/go.md"),
	})

	// elsewhere
	tests = append(tests, &testCase{
		_id:       "elsewhere",
		_expected: pathCell{Rel: filepath.FromSlash("/etc/hosts")},

		claudeHome: filepath.FromSlash("/home/.claude"),
		cwd:        filepath.FromSlash("/repo/app"),
		path:       filepath.FromSlash("/etc/hosts"),
	})

	// empty-cwd
	tests = append(tests, &testCase{
		_id:       "empty-cwd",
		_expected: pathCell{Prefix: filepath.FromSlash("/home/.claude/"), Rel: filepath.FromSlash("skills/x/SKILL.md")},

		claudeHome: filepath.FromSlash("/home/.claude"),
		cwd:        "",
		path:       filepath.FromSlash("/home/.claude/skills/x/SKILL.md"),
	})

	// cwd-with-trailing-slash
	tests = append(tests, &testCase{
		_id:       "cwd-with-trailing-slash",
		_expected: pathCell{Prefix: filepath.FromSlash("/repo/app/"), Rel: filepath.FromSlash("internal/a.go")},

		claudeHome: filepath.FromSlash("/home/.claude"),
		cwd:        filepath.FromSlash("/repo/app/"),
		path:       filepath.FromSlash("/repo/app/internal/a.go"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			got := relativePath(test.path, test.cwd, test.claudeHome)
			assert.Equal(t, test._expected, got)
			assert.Equal(t, test.path, got.String())
		})
	}
}

func TestSizeBadge(t *testing.T) {
	type testCase struct {
		_id             string
		_expected       sectionSize
		_expectedString string

		pending int
		totals  *contextrecord.Totals
	}

	tests := make([]*testCase, 0)

	// none-pending
	tests = append(tests, &testCase{
		_id:             "none-pending",
		_expected:       sectionSize{Chars: "12 345 chars", Tokens: "3 000 tokens"},
		_expectedString: "12 345 chars · 3 000 tokens",

		pending: 0,
		totals:  &contextrecord.Totals{Chars: 12345, Items: 4, Tokens: 3000},
	})

	// pending
	tests = append(tests, &testCase{
		_id:             "pending",
		_expected:       sectionSize{Chars: "12 345 chars", Pending: "2 pending", Tokens: "1 500 tokens"},
		_expectedString: "12 345 chars · 1 500 tokens so far · 2 pending",

		pending: 2,
		totals:  &contextrecord.Totals{Chars: 12345, Items: 4, Tokens: 1500, Uncounted: 2},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			size := sizeBadge(test.totals, test.pending)
			assert.Equal(t, test._expected, size)
			assert.Equal(t, test._expectedString, size.String())
			assert.Empty(t, sectionSize{}.String())
		})
	}
}

func TestTotalsTokenCell(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		totals *contextrecord.Totals
	}

	tests := make([]*testCase, 0)

	// counted
	tests = append(tests, &testCase{
		_id:       "counted",
		_expected: "4 200",

		totals: &contextrecord.Totals{Items: 3, Tokens: 4200},
	})

	// none-counted
	tests = append(tests, &testCase{
		_id:       "none-counted",
		_expected: "—",

		totals: &contextrecord.Totals{Items: 3, Uncounted: 3},
	})

	// mixed
	tests = append(tests, &testCase{
		_id:       "mixed",
		_expected: "1 000",

		totals: &contextrecord.Totals{Items: 3, Tokens: 1000, Uncounted: 2},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, totalsTokenCell(test.totals))
		})
	}
}
