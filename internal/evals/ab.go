package evals

// AbFileName is the A/B dir's record file.
const AbFileName = "ab.json"

// Taken from the statistics tool's stats.json (Welch, method of cmd/evalstats).
type AbDifference struct {
	BaselineMean   float64 `json:"baselineMean"`
	BaselineN      int     `json:"baselineN"`
	BaselineStdDev float64 `json:"baselineStdDev"`
	DeltaMean      float64 `json:"deltaMean"`
	Df             float64 `json:"df"`
	Measure        string  `json:"measure"`
	Note           string  `json:"note"`
	Significant    bool    `json:"significant"`
	VariantMean    float64 `json:"variantMean"`
	VariantN       int     `json:"variantN"`
	VariantStdDev  float64 `json:"variantStdDev"`
	WelchT         float64 `json:"welchT"`
}

type AbFile struct {
	Baseline      AbSide         `json:"baseline"`
	Date          string         `json:"date"`
	Descriptor    string         `json:"descriptor"`
	Differences   []AbDifference `json:"differences"`
	Layer         string         `json:"layer"`
	Notes         []string       `json:"notes"`
	SchemaVersion string         `json:"schemaVersion"`
	Skill         string         `json:"skill"`
	Variant       AbSide         `json:"variant"`
}

func (f *AbFile) SideOf(key string) *AbSide {
	switch key {
	case f.Baseline.Key:
		return &f.Baseline
	case f.Variant.Key:
		return &f.Variant
	}
	return nil
}

// Measures are an eval record's numeric eval.json metrics or a smoke record's own numbers.
type AbRecord struct {
	Measures     map[string]float64 `json:"measures"`
	Path         string             `json:"path"`
	SkillVersion string             `json:"skillVersion"`
}

// Key is the configuration key of a side-by-side run or the skill version of a version variant.
type AbSide struct {
	Key     string     `json:"key"`
	Records []AbRecord `json:"records"`
}
