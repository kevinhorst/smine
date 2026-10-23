package budget

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
)

func TestNewPeekSide(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *PeekSide

		events *peek.SessionEvents
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// negative-seconds-clamped
	tests = append(tests, &testCase{
		_id: "negative-seconds-clamped",
		_expected: &PeekSide{
			ActiveSeconds: 0,
			CostUsd:       1.5,
			HasCost:       true,
			IdleSeconds:   0,
			LastActive:    anchor.Add(time.Hour),
			Root:          TokenClasses{CacheCreation: 1, CacheRead: 2, Input: 3, Output: 4},
			StartedAt:     anchor,
			WallSeconds:   0,
		},

		events: &peek.SessionEvents{
			ActiveSeconds: -5,
			CostUsd:       1.5,
			HasTelemetry:  true,
			IdleSeconds:   -7,
			LastActive:    anchor.Add(time.Hour),
			StartedAt:     anchor,
			Usage:         peek.TokenUsage{CacheCreationInputTokens: 1, CacheReadInputTokens: 2, InputTokens: 3, OutputTokens: 4},
			WallSeconds:   -9,
		},
	})

	// subagents-summed
	tests = append(tests, &testCase{
		_id: "subagents-summed",
		_expected: &PeekSide{
			ActiveSeconds: 60,
			CostUsd:       2,
			HasCost:       true,
			IdleSeconds:   30,
			LastActive:    anchor.Add(time.Hour),
			Root:          TokenClasses{Input: 10, Output: 20},
			SkillRuns: []SkillRun{
				{Seconds: 40, Skill: "fdesign", Usage: TokenClasses{Input: 4, Output: 8}},
			},
			StartedAt:   anchor,
			Subagents:   TokenClasses{CacheCreation: 3, CacheRead: 30, Input: 3, Output: 7},
			WallSeconds: 90,
		},

		events: &peek.SessionEvents{
			ActiveSeconds: 60,
			CostUsd:       2,
			HasTelemetry:  true,
			IdleSeconds:   30,
			LastActive:    anchor.Add(time.Hour),
			SkillRuns: []peek.SkillRun{
				{Seconds: 40, Skill: "fdesign", Usage: peek.TokenUsage{InputTokens: 4, OutputTokens: 8}},
			},
			StartedAt: anchor,
			Subagents: []peek.SubagentUsage{
				{Id: "a1", Usage: peek.TokenUsage{CacheCreationInputTokens: 1, CacheReadInputTokens: 10, InputTokens: 1, OutputTokens: 2}},
				{Id: "a2", Usage: peek.TokenUsage{CacheCreationInputTokens: 2, CacheReadInputTokens: 20, InputTokens: 2, OutputTokens: 5}},
			},
			Usage:       peek.TokenUsage{InputTokens: 10, OutputTokens: 20},
			WallSeconds: 90,
		},
	})

	// no-telemetry-no-cost
	tests = append(tests, &testCase{
		_id: "no-telemetry-no-cost",
		_expected: &PeekSide{
			ActiveSeconds: 5,
			CostUsd:       0,
			HasCost:       false,
			IdleSeconds:   1,
			LastActive:    anchor,
			StartedAt:     anchor,
			WallSeconds:   6,
		},

		events: &peek.SessionEvents{
			ActiveSeconds: 5,
			HasTelemetry:  false,
			IdleSeconds:   1,
			LastActive:    anchor,
			StartedAt:     anchor,
			WallSeconds:   6,
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, newPeekSide(test.events))
		})
	}
}

func TestPeekSide_OutsideSkills(t *testing.T) {
	type testCase struct {
		_id       string
		_expected TokenClasses

		side *PeekSide
	}

	root := TokenClasses{CacheCreation: 10, CacheRead: 100, Input: 20, Output: 40}
	tests := make([]*testCase, 0)

	// runs-equal-root
	tests = append(tests, &testCase{
		_id:       "runs-equal-root",
		_expected: TokenClasses{},

		side: &PeekSide{
			Root: root,
			SkillRuns: []SkillRun{
				{Skill: "a", Usage: TokenClasses{CacheCreation: 4, CacheRead: 60, Input: 5, Output: 10}},
				{Skill: "b", Usage: TokenClasses{CacheCreation: 6, CacheRead: 40, Input: 15, Output: 30}},
			},
		},
	})

	// runs-below-root
	tests = append(tests, &testCase{
		_id:       "runs-below-root",
		_expected: TokenClasses{CacheCreation: 9, CacheRead: 90, Input: 18, Output: 36},

		side: &PeekSide{
			Root: root,
			SkillRuns: []SkillRun{
				{Skill: "a", Usage: TokenClasses{CacheCreation: 1, CacheRead: 10, Input: 2, Output: 4}},
			},
		},
	})

	// runs-above-root-clamped
	tests = append(tests, &testCase{
		_id:       "runs-above-root-clamped",
		_expected: TokenClasses{CacheCreation: 0, CacheRead: 50, Input: 0, Output: 40},

		side: &PeekSide{
			Root: root,
			SkillRuns: []SkillRun{
				{Skill: "a", Usage: TokenClasses{CacheCreation: 25, CacheRead: 50, Input: 21}},
			},
		},
	})

	// no-runs
	tests = append(tests, &testCase{
		_id:       "no-runs",
		_expected: root,

		side: &PeekSide{Root: root},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.side.OutsideSkills())
		})
	}
}

func TestPeekSide_SkillUsage(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedUsage  TokenClasses
		_expectedSecond int

		side  *PeekSide
		skill string
	}

	side := &PeekSide{
		SkillRuns: []SkillRun{
			{Seconds: 60, Skill: "a", Usage: TokenClasses{CacheCreation: 4, CacheRead: 60, Input: 5, Output: 10}},
			{Seconds: 5, Skill: "b", Usage: TokenClasses{CacheCreation: 6, CacheRead: 40, Input: 15, Output: 30}},
			{Seconds: 30, Skill: "a", Usage: TokenClasses{CacheCreation: 1, CacheRead: 10, Input: 2, Output: 4}},
		},
	}
	tests := make([]*testCase, 0)

	// two-runs-summed
	tests = append(tests, &testCase{
		_id:             "two-runs-summed",
		_expectedUsage:  TokenClasses{CacheCreation: 5, CacheRead: 70, Input: 7, Output: 14},
		_expectedSecond: 90,

		side:  side,
		skill: "a",
	})

	// one-run
	tests = append(tests, &testCase{
		_id:             "one-run",
		_expectedUsage:  TokenClasses{CacheCreation: 6, CacheRead: 40, Input: 15, Output: 30},
		_expectedSecond: 5,

		side:  side,
		skill: "b",
	})

	// no-run-zero
	tests = append(tests, &testCase{
		_id:             "no-run-zero",
		_expectedUsage:  TokenClasses{},
		_expectedSecond: 0,

		side:  side,
		skill: "c",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			usage, seconds := test.side.SkillUsage(test.skill)
			assert.Equal(t, test._expectedUsage, usage)
			assert.Equal(t, test._expectedSecond, seconds)
		})
	}
}

func TestPeekSide_Total(t *testing.T) {
	side := &PeekSide{
		Root:      TokenClasses{CacheCreation: 1, CacheRead: 2, Input: 3, Output: 4},
		Subagents: TokenClasses{CacheCreation: 10, CacheRead: 20, Input: 30, Output: 40},
	}

	assert.Equal(t, TokenClasses{CacheCreation: 11, CacheRead: 22, Input: 33, Output: 44}, side.Total())
	assert.Equal(t, 110, side.Total().Total())
}

func TestRecord_Share(t *testing.T) {
	type testCase struct {
		_id       string
		_expected float64

		record *Record
		usage  TokenClasses
	}

	// cacheRead is a tenth of the price of output on Opus, the same on a count
	root := TokenClasses{CacheRead: 1000, Output: 100}
	cacheReads := TokenClasses{CacheRead: 1000}
	opus := ModelUsage{Model: "claude-opus-5-5-20260301", Usage: TokenClasses{Output: 1000}}
	fable := ModelUsage{Model: "claude-fable-5-1", Usage: TokenClasses{Output: 1000}}
	unknown := ModelUsage{Model: "gpt-5-codex", Usage: TokenClasses{Output: 1000}}
	tests := make([]*testCase, 0)

	// no-peek-side-zero
	tests = append(tests, &testCase{
		_id:       "no-peek-side-zero",
		_expected: 0,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{opus}}},
		usage:  cacheReads,
	})

	// empty-session-zero
	tests = append(tests, &testCase{
		_id:       "empty-session-zero",
		_expected: 0,

		record: &Record{Peek: &PeekSide{}},
		usage:  cacheReads,
	})

	// no-ccusage-by-count
	// 1 000 of 1 100 tokens
	tests = append(tests, &testCase{
		_id:       "no-ccusage-by-count",
		_expected: 1000.0 / 1100,

		record: &Record{Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// no-models-by-count
	tests = append(tests, &testCase{
		_id:       "no-models-by-count",
		_expected: 1000.0 / 1100,

		record: &Record{Ccusage: &CcusageSide{}, Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// opus-by-price
	// 0.01 × 1 000 = 10 of 10 + 1 × 100
	tests = append(tests, &testCase{
		_id:       "opus-by-price",
		_expected: 10.0 / 110,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{opus}}, Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// fable-by-price
	// 0.0125 × 1 000 = 12.5 of 12.5 + 2.5 × 100
	tests = append(tests, &testCase{
		_id:       "fable-by-price",
		_expected: 12.5 / 262.5,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{fable}}, Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// heaviest-model-decides
	// Fable drew more tokens than Opus: Fable ratios
	lighterOpus := ModelUsage{Model: opus.Model, Usage: TokenClasses{Output: 10}}
	tests = append(tests, &testCase{
		_id:       "heaviest-model-decides",
		_expected: 12.5 / 262.5,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{lighterOpus, fable}}, Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// unknown-heaviest-by-count
	tests = append(tests, &testCase{
		_id:       "unknown-heaviest-by-count",
		_expected: 1000.0 / 1100,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{unknown, lighterOpus}}, Peek: &PeekSide{Root: root}},
		usage:  cacheReads,
	})

	// subagents-in-the-whole
	// the whole is root plus subagents: 10 of 10 + 100 + 100
	tests = append(tests, &testCase{
		_id:       "subagents-in-the-whole",
		_expected: 10.0 / 210,

		record: &Record{Ccusage: &CcusageSide{Models: []ModelUsage{opus}}, Peek: &PeekSide{Root: root, Subagents: TokenClasses{Output: 100}}},
		usage:  cacheReads,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.InDelta(t, test._expected, test.record.Share(test.usage), 1e-12)
		})
	}
}

func TestRecord_LastActivity(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	record := &Record{
		Ccusage: &CcusageSide{LastActivity: anchor.Add(2 * time.Hour)},
		Peek:    &PeekSide{LastActive: anchor.Add(time.Hour)},
	}

	assert.Equal(t, anchor.Add(2*time.Hour), record.LastActivity())
	assert.True(t, (&Record{}).LastActivity().IsZero())
}

func TestCcusageSide_PlanUsage(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedAll   float64
		_expectedFable float64
		_expectedOk    bool

		side *CcusageSide
	}

	tests := make([]*testCase, 0)

	// opus-only
	tests = append(tests, &testCase{
		_id:            "opus-only",
		_expectedAll:   100*0.4 + 1000*0.01 + 10*0.2 + 50*1,
		_expectedFable: 0,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-opus-5-5", Usage: TokenClasses{CacheCreation: 100, CacheRead: 1000, Input: 10, Output: 50}},
		}},
	})

	// measured-window-fills-team-premium
	tests = append(tests, &testCase{
		_id:            "measured-window-fills-team-premium",
		_expectedAll:   PlanTeamPremium.WindowCapacity(),
		_expectedFable: 0,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-opus-5-5", Usage: TokenClasses{CacheCreation: 17216269, CacheRead: 390238659, Input: 4834, Output: 2137757}},
		}},
	})

	// switched-models-weighted-per-model
	tests = append(tests, &testCase{
		_id: "switched-models-weighted-per-model",
		_expectedAll: (200*1 + 8000*0.0125 + 40*0.5 + 100*2.5) +
			(1000*0.4 + 10000*0.01 + 100*0.2 + 500*1) +
			(500*0.2 + 2000*0.01 + 20*0.1 + 300*0.5),
		_expectedFable: 200*1 + 8000*0.0125 + 40*0.5 + 100*2.5,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{CacheCreation: 200, CacheRead: 8000, Input: 40, Output: 100}},
			{Model: "claude-opus-5-5", Usage: TokenClasses{CacheCreation: 1000, CacheRead: 10000, Input: 100, Output: 500}},
			{Model: "claude-sonnet-5-5", Usage: TokenClasses{CacheCreation: 500, CacheRead: 2000, Input: 20, Output: 300}},
		}},
	})

	// fable-output-weight
	tests = append(tests, &testCase{
		_id:            "fable-output-weight",
		_expectedAll:   2500,
		_expectedFable: 2500,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{Output: 1000}},
		}},
	})

	// fable-cache-creation-weight
	tests = append(tests, &testCase{
		_id:            "fable-cache-creation-weight",
		_expectedAll:   1000,
		_expectedFable: 1000,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{CacheCreation: 1000}},
		}},
	})

	// fable-cache-read-weight
	tests = append(tests, &testCase{
		_id:            "fable-cache-read-weight",
		_expectedAll:   12.5,
		_expectedFable: 12.5,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{CacheRead: 1000}},
		}},
	})

	// fable-input-weight
	tests = append(tests, &testCase{
		_id:            "fable-input-weight",
		_expectedAll:   500,
		_expectedFable: 500,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{Input: 1000}},
		}},
	})

	// no-models-unknown
	tests = append(tests, &testCase{
		_id:            "no-models-unknown",
		_expectedAll:   0,
		_expectedFable: 0,
		_expectedOk:    false,

		side: &CcusageSide{Usage: TokenClasses{Output: 1000}},
	})

	// unknown-model-with-tokens-unknown
	tests = append(tests, &testCase{
		_id:            "unknown-model-with-tokens-unknown",
		_expectedAll:   0,
		_expectedFable: 0,
		_expectedOk:    false,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-opus-5-5", Usage: TokenClasses{Output: 10}},
			{Model: "gpt-5", Usage: TokenClasses{Output: 5}},
		}},
	})

	// unknown-model-without-tokens-ignored
	tests = append(tests, &testCase{
		_id:            "unknown-model-without-tokens-ignored",
		_expectedAll:   10,
		_expectedFable: 0,
		_expectedOk:    true,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-opus-5-5", Usage: TokenClasses{Output: 10}},
			{Model: "gpt-5", Usage: TokenClasses{}},
		}},
	})

	// all-models-without-tokens-unknown
	tests = append(tests, &testCase{
		_id:            "all-models-without-tokens-unknown",
		_expectedAll:   0,
		_expectedFable: 0,
		_expectedOk:    false,

		side: &CcusageSide{Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{}},
			{Model: "claude-opus-5-5", Usage: TokenClasses{}},
		}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			usage, ok := test.side.PlanUsage()
			assert.Equal(t, test._expectedOk, ok)
			assert.InDelta(t, test._expectedAll, usage.All, 1e-6)
			assert.InDelta(t, test._expectedFable, usage.Fable, 1e-6)
		})
	}
}

func TestCcusageSide_Json(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	withModels := &CcusageSide{
		CostUsd:      1.5,
		LastActivity: anchor,
		Models: []ModelUsage{
			{Model: "claude-fable-5-1", Usage: TokenClasses{Output: 7}},
			{Model: "claude-opus-5-5", Usage: TokenClasses{Input: 3}},
		},
		Usage: TokenClasses{Input: 3, Output: 7},
	}
	withModelsData, err := json.Marshal(withModels)
	require.NoError(t, err)
	var withModelsDecoded CcusageSide
	require.NoError(t, json.Unmarshal(withModelsData, &withModelsDecoded))
	assert.Equal(t, withModels, &withModelsDecoded)

	withoutModels := &CcusageSide{CostUsd: 1.5, LastActivity: anchor, Usage: TokenClasses{Input: 3, Output: 7}}
	withoutModelsData, err := json.Marshal(withoutModels)
	require.NoError(t, err)
	assert.NotContains(t, string(withoutModelsData), `"models"`)
	var withoutModelsDecoded CcusageSide
	require.NoError(t, json.Unmarshal(withoutModelsData, &withoutModelsDecoded))
	assert.Equal(t, withoutModels, &withoutModelsDecoded)
}

func TestCcusageSide_WallSeconds(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, 90, (&CcusageSide{FirstActivity: anchor, LastActivity: anchor.Add(90 * time.Second)}).WallSeconds())
	assert.Equal(t, 0, (&CcusageSide{LastActivity: anchor}).WallSeconds())
}

func TestBreakdown_IsCurrent(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		breakdown    *Breakdown
		lastActivity time.Time
	}

	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// is-current-equal
	tests = append(tests, &testCase{
		_id:       "is-current-equal",
		_expected: true,

		breakdown:    &Breakdown{LastActivity: anchor},
		lastActivity: anchor,
	})

	// is-current-older
	tests = append(tests, &testCase{
		_id:       "is-current-older",
		_expected: false,

		breakdown:    &Breakdown{LastActivity: anchor},
		lastActivity: anchor.Add(time.Minute),
	})

	// is-current-newer
	tests = append(tests, &testCase{
		_id:       "is-current-newer",
		_expected: true,

		breakdown:    &Breakdown{LastActivity: anchor.Add(time.Minute)},
		lastActivity: anchor,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.breakdown.IsCurrent(test.lastActivity))
		})
	}
}

func TestBreakdown_HasRetryable(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		breakdown *Breakdown
	}

	tests := make([]*testCase, 0)

	// has-retryable-cap
	tests = append(tests, &testCase{
		_id:       "has-retryable-cap",
		_expected: true,

		breakdown: breakdownWithUncounted(contextrecord.UncountedCap),
	})

	// has-retryable-queued
	tests = append(tests, &testCase{
		_id:       "has-retryable-queued",
		_expected: true,

		breakdown: breakdownWithUncounted(contextrecord.UncountedQueued),
	})

	// has-retryable-rate
	tests = append(tests, &testCase{
		_id:       "has-retryable-rate",
		_expected: true,

		breakdown: breakdownWithUncounted(contextrecord.UncountedRate),
	})

	// disabled-not-retryable
	tests = append(tests, &testCase{
		_id:       "disabled-not-retryable",
		_expected: false,

		breakdown: breakdownWithUncounted(contextrecord.UncountedDisabled),
	})

	// size-not-retryable
	tests = append(tests, &testCase{
		_id:       "size-not-retryable",
		_expected: false,

		breakdown: breakdownWithUncounted(contextrecord.UncountedSize),
	})

	// all-counted
	tests = append(tests, &testCase{
		_id:       "all-counted",
		_expected: false,

		breakdown: breakdownWithUncounted(""),
	})

	// error-breakdown-no-record
	tests = append(tests, &testCase{
		_id:       "error-breakdown-no-record",
		_expected: false,

		breakdown: &Breakdown{Error: "ExtractRecord: No transcript on disk"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.breakdown.HasRetryable())
		})
	}
}

func TestBreakdown_Summary(t *testing.T) {
	anchor := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	root := contextrecord.Agent{
		Id:   "root",
		Kind: contextrecord.AgentRoot,
		Items: []contextrecord.Item{
			contextrecord.NewItem(contextrecord.KindLang, "go", contextrecord.SourceToolResult, "go guide"),
			contextrecord.NewItem(contextrecord.KindGlobal, "global", contextrecord.SourceHookContext, "global text"),
		},
	}
	subagent := contextrecord.Agent{
		Id:          "agent-1",
		Description: "explore",
		Kind:        contextrecord.AgentSubagent,
		Model:       "sonnet",
		Type:        "Explore",
		Items: []contextrecord.Item{
			contextrecord.NewItem(contextrecord.KindSkill, "fdesign", contextrecord.SourceToolResult, "skill body"),
		},
	}
	record := &contextrecord.Record{Agent: contextrecord.SessionClaude, Agents: []contextrecord.Agent{root, subagent}}
	record.Recompute()
	breakdown := NewBreakdown(anchor, "session-1", anchor, record)

	summary := breakdown.Summary(anchor.Add(time.Minute))

	assert.Equal(t, anchor.Add(time.Minute), summary.Source)
	assert.Equal(t, record.Totals, summary.Totals)
	assert.Equal(t, []ContextAgent{
		{Id: "root", Kind: contextrecord.AgentRoot, Totals: record.Agents[0].Totals},
		{Id: "agent-1", Description: "explore", Kind: contextrecord.AgentSubagent, Model: "sonnet", Totals: record.Agents[1].Totals, Type: "Explore"},
	}, summary.Agents)
	assert.Equal(t, []contextrecord.Kind{contextrecord.KindGlobal, contextrecord.KindSkill, contextrecord.KindLang}, summaryKinds(summary))
	assert.Equal(t, "go guide", breakdown.Record.Agents[0].Items[0].Detail)
}

func breakdownWithUncounted(reason string) *Breakdown {
	item := contextrecord.NewItem(contextrecord.KindGlobal, "global", contextrecord.SourceHookContext, "text")
	item.Uncounted = reason
	agent := contextrecord.Agent{Id: "root", Items: []contextrecord.Item{item}, Kind: contextrecord.AgentRoot}
	record := &contextrecord.Record{Agents: []contextrecord.Agent{agent}}
	return &Breakdown{Record: record, Schema: RecordSchema}
}

func summaryKinds(summary *ContextSummary) []contextrecord.Kind {
	kinds := make([]contextrecord.Kind, 0, len(summary.Kinds))
	for _, row := range summary.Kinds {
		kinds = append(kinds, row.Kind)
	}
	return kinds
}
