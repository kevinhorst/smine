package budget

import (
	"slices"
	"strings"
)

const (
	// DefaultPlan is the plan the one measured window belongs to.
	DefaultPlan = PlanTeamPremium
	// Models sharing the Fable part of the weekly limit.
	fableModelPrefix = modelNamePrefix + "fable"
	// FableWeeklyShare is the part of the weekly limit Fable models may use.
	FableWeeklyShare = 0.5
	// The measured window took the all-models weekly limit from 100% to 84%.
	measuredWeeklyShare = 0.16
	// modelNamePrefix opens every model id; family and version follow it.
	modelNamePrefix = "claude-"
)

// One Team Premium 5-hour window driven to its limit on Opus 5.5 alone (docs/token-usage.html, 2026-09-28).
const (
	measuredCacheCreation = 17216269
	measuredCacheRead     = 390238659
	measuredInput         = 4834
	measuredOutput        = 2137757
)

// In Opus 5.5 output tokens: Anthropic's list-price ratios, cache writes at the 1-hour rate.
const (
	fableCacheCreation, fableCacheRead, fableInput, fableOutput     = 1, 0.0125, 0.5, 2.5
	haikuCacheCreation, haikuCacheRead, haikuInput, haikuOutput     = 0.1, 0.005, 0.05, 0.25
	opusCacheCreation, opusCacheRead, opusInput, opusOutput         = 0.4, 0.01, 0.2, 1
	sonnetCacheCreation, sonnetCacheRead, sonnetInput, sonnetOutput = 0.2, 0.01, 0.1, 0.5
)

// One Opus 5.5 session on a Max 20x account: 28% on its 5-hour meter, about 10 points on its weekly meter (docs/token-usage.html, 2026-10-05).
const (
	max20xSessionCacheCreation = 15156419
	max20xSessionCacheRead     = 353337991
	max20xSessionInput         = 4308
	max20xSessionOutput        = 1876747
	// max20xSessionWeekShare is eyeballed, to be confirmed by a before-and-after reading.
	max20xSessionWeekShare   = 0.10
	max20xSessionWindowShare = 0.28
)

var max20xSession = TokenClasses{
	CacheCreation: max20xSessionCacheCreation,
	CacheRead:     max20xSessionCacheRead,
	Input:         max20xSessionInput,
	Output:        max20xSessionOutput,
}

var measuredWindow = TokenClasses{
	CacheCreation: measuredCacheCreation,
	CacheRead:     measuredCacheRead,
	Input:         measuredInput,
	Output:        measuredOutput,
}

// countWeights weigh every class alike: a share by token count.
var countWeights = classWeights{cacheCreation: 1, cacheRead: 1, input: 1, output: 1}

// measuredWeights are the ratios of the model the measured window ran on.
var measuredWeights = classWeights{cacheCreation: opusCacheCreation, cacheRead: opusCacheRead, input: opusInput, output: opusOutput}

// Keyed by family and version so a dated model id finds its entry.
var modelWeights = []modelWeight{
	{family: "fable", version: "5-1", weights: classWeights{cacheCreation: fableCacheCreation, cacheRead: fableCacheRead, input: fableInput, output: fableOutput}},
	{family: "haiku", version: "4-5", weights: classWeights{cacheCreation: haikuCacheCreation, cacheRead: haikuCacheRead, input: haikuInput, output: haikuOutput}},
	{family: "opus", version: "5-5", weights: measuredWeights},
	{family: "sonnet", version: "5-5", weights: classWeights{cacheCreation: sonnetCacheCreation, cacheRead: sonnetCacheRead, input: sonnetInput, output: sonnetOutput}},
}

// planLabels names each plan on the page.
var planLabels = map[Plan]string{
	PlanMax20x:      "Max 20x",
	PlanTeamPremium: "Team Premium",
}

// The part of the weekly limit one full window takes; Max 20x from the measured session's two meter moves.
var planWeeklyShares = map[Plan]float64{
	PlanMax20x:      max20xSessionWeekShare / max20xSessionWindowShare,
	PlanTeamPremium: measuredWeeklyShare,
}

// In measured Team Premium windows; Max 20x is the measured session's share of its window.
var planWindows = map[Plan]float64{
	PlanMax20x:      measuredWeights.equivalents(max20xSession) / max20xSessionWindowShare / measuredWeights.equivalents(measuredWindow),
	PlanTeamPremium: 1,
}

// Plans lists the plans in the order the Usage page offers them.
var Plans = []Plan{PlanTeamPremium, PlanMax20x}

type classWeights struct {
	cacheCreation float64
	cacheRead     float64
	input         float64
	output        float64
}

func (w classWeights) equivalents(usage TokenClasses) float64 {
	return w.cacheCreation*float64(usage.CacheCreation) +
		w.cacheRead*float64(usage.CacheRead) +
		w.input*float64(usage.Input) +
		w.output*float64(usage.Output)
}

type modelWeight struct {
	family  string
	version string
	weights classWeights
}

// prefix is what every id of the model starts with, dated ids included.
func (m *modelWeight) prefix() string {
	return modelNamePrefix + m.family + "-" + m.version
}

// Plan is a Claude subscription plan.
type Plan string

const (
	PlanMax20x      Plan = "max-20x"
	PlanTeamPremium Plan = "team-premium"
)

// NewPlan reads a stored plan name; false for an unknown one.
func NewPlan(value string) (Plan, bool) {
	plan := Plan(value)
	return plan, slices.Contains(Plans, plan)
}

func (p Plan) Label() string {
	return planLabels[p]
}

func (p Plan) WeeklyShare() (float64, bool) {
	share, ok := planWeeklyShares[p]
	return share, ok
}

// In Opus 5.5 output-token equivalents: the measured window times the plan's size.
func (p Plan) WindowCapacity() float64 {
	return measuredWeights.equivalents(measuredWindow) * planWindows[p]
}

// In Opus 5.5 output-token equivalents; Fable is the part Fable models drew.
type PlanUsage struct {
	All   float64
	Fable float64
}

func (u PlanUsage) Add(other PlanUsage) PlanUsage {
	sum := PlanUsage{All: u.All + other.All, Fable: u.Fable + other.Fable}
	return sum
}

// Scale is the usage times factor: a session's plan usage narrowed to a share of it.
func (u PlanUsage) Scale(factor float64) PlanUsage {
	scaled := PlanUsage{All: u.All * factor, Fable: u.Fable * factor}
	return scaled
}

// weightsFor finds a model's ratios by its name prefix.
func weightsFor(model string) (classWeights, bool) {
	for index := range modelWeights {
		if strings.HasPrefix(model, modelWeights[index].prefix()) {
			return modelWeights[index].weights, true
		}
	}

	var none classWeights
	return none, false
}
