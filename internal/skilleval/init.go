package skilleval

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	defaultCheckTimeoutS = 1800
	definitionSchema     = "4.0"
	defaultEffort        = "high"
	defaultJudge         = "opus"
	defaultModel         = "opus"
	defaultReplicas      = 1
	defaultWindowUsd     = 150
	handshake            = "Persist all run artifacts under {artifactsDir} (your artifact directory)."
	labelAlphabet        = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	overlayFile          = "overlay.patch"
	overlayVariantFile   = "overlay-variant.patch"
	runHashLength        = 6
	skillMdFile          = "SKILL.md"
	windowBudgetEnv      = "SKILLEVAL_WINDOW_BUDGET_USD"
)

// Init refusals cmd/skilleval maps to their own exit codes.
var (
	ErrBadRunArg    = errors.New("run argument does not fit the definition")
	ErrClosedArg    = errors.New("the definition does not open this arg")
	ErrNoDefinition = errors.New("skill has no eval/definition.json")
	ErrNoGolden     = errors.New("no minted golden")
	ErrOverWindow   = errors.New("planned spend exceeds the window budget")
	ErrUnknownToken = errors.New("token label is not in the token store")
)

var skillNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type armConfig struct {
	disabledServers []string
	effort          string
	key             string
	model           string
	overlayPatch    string
	variantPatch    string
}

func newBaseConfig(def *evals.Definition) *armConfig {
	model := def.Model
	if model == "" {
		model = defaultModel
	}

	effort := def.Effort
	if effort == "" {
		effort = defaultEffort
	}

	config := &armConfig{
		disabledServers: []string{},
		effort:          effort,
		key:             configKey(nil, effort, model),
		model:           model,
	}
	return config
}

// InitRequest's Replicas 0 takes the definition's count.
type InitRequest struct {
	Date       string
	EvalsRoot  string
	Golden     string
	Overrides  []evals.DefinitionArg
	Replicas   int
	Skill      string
	SkillsRoot string
	Token      string
}

// runSpec is everything one run dir is written from.
type runSpec struct {
	configs   []*armConfig
	date      string
	def       *evals.Definition
	golden    *ManifestGolden
	overrides []evals.DefinitionArg
	replicas  int
	runDir    string
	skill     string
	skillDir  string
	token     string
}

// arms lays out every configuration × replica under shuffled blind labels.
func (s *runSpec) arms() ([]ManifestArm, error) {
	labels, err := blindLabels(len(s.configs) * s.replicas)
	if err != nil {
		return nil, err
	}

	arms := make([]ManifestArm, 0, len(labels))
	for _, config := range s.configs {
		for replica := 1; replica <= s.replicas; replica++ {
			arm := ManifestArm{
				Id:              config.key + "-r" + strconv.Itoa(replica),
				DisabledServers: config.disabledServers,
				Effort:          config.effort,
				Key:             config.key,
				Label:           labels[len(arms)],
				Model:           config.model,
				OverlayPatch:    config.overlayPatch,
				Replica:         replica,
			}
			arms = append(arms, arm)
		}
	}
	return arms, nil
}

// copyOverlays appends an overlay variant's patch to the run overlay as the variant's own.
func (s *runSpec) copyOverlays() (string, error) {
	var base []byte
	runOverlay := ""
	if s.def.Environment.Overlay != "" {
		content, err := os.ReadFile(filepath.Join(s.skillDir, s.def.Environment.Overlay))
		if err != nil {
			return "", fmt.Errorf("runSpec.copyOverlays: %w", err)
		}

		base = content
		runOverlay, err = writeOverlay(base, filepath.Join(s.runDir, overlayFile))
		if err != nil {
			return "", err
		}
	}

	for _, config := range s.configs {
		if config.variantPatch == "" {
			continue
		}

		patch, err := os.ReadFile(config.variantPatch)
		if err != nil {
			return "", fmt.Errorf("runSpec.copyOverlays: %w", err)
		}

		combined := slices.Concat(base, patch)
		config.overlayPatch, err = writeOverlay(combined, filepath.Join(s.runDir, overlayVariantFile))
		if err != nil {
			return "", err
		}
	}
	return runOverlay, nil
}

func (s *runSpec) freezeGolden() error {
	switch s.golden.kind() {
	case GoldenProduce:
		return copyFile(filepath.Join(s.skillDir, s.golden.SeedFile), filepath.Join(s.runDir, seedSnapshotFile))
	case GoldenTest:
		if err := copyFile(filepath.Join(s.skillDir, s.def.Goldens[0].File), filepath.Join(s.runDir, goldenSnapshotFile)); err != nil {
			return err
		}
		return copyFile(filepath.Join(s.skillDir, s.def.ScoringFile), filepath.Join(s.runDir, scoringSnapshotFile))
	}
	return nil
}

func (s *runSpec) write() error {
	prompt, err := renderInvocation(s.def, s.golden, s.overrides)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(s.runDir, runsDir), 0o755); err != nil {
		return fmt.Errorf("runSpec.write: %w", err)
	}

	deployedDir := filepath.Join(skills.DefaultHomePath(), s.skill)
	if err := copyFile(filepath.Join(deployedDir, skillMdFile), filepath.Join(s.runDir, skillMdFile)); err != nil {
		return err
	}

	runOverlay, err := s.copyOverlays()
	if err != nil {
		return err
	}

	arms, err := s.arms()
	if err != nil {
		return err
	}

	if err := s.freezeGolden(); err != nil {
		return err
	}

	manifest := &RunManifest{
		Arms:           arms,
		At:             checkoutRef(s.def),
		BudgetUsd:      s.def.Budgets.ArmUsd,
		Check:          s.def.Check,
		CheckTimeoutS:  defaultCheckTimeoutS,
		ContextIndex:   s.def.Environment.ContextIndex,
		Date:           s.date,
		Findings:       s.def.Findings,
		Golden:         s.golden,
		Judge:          defaultJudge,
		Layer:          LayerEval,
		Mode:           ModeNightly,
		OverlayPatch:   runOverlay,
		Overrides:      s.overrides,
		Prebuild:       s.def.Environment.Prebuild,
		PresetSkillDir: s.skillDir,
		Prompt:         prompt,
		Reference:      s.def.Reference,
		RepoName:       s.def.Environment.TargetRepo,
		Scorer:         s.def.Scorer.Command,
		Skill:          s.skill,
		SkillMd:        filepath.Join(s.runDir, skillMdFile),
		SkillVersion:   skills.Version(deployedDir),
		TimeoutS:       s.def.Budgets.ArmTimeoutS,
		TokenLabel:     s.token,
	}
	return writeManifest(s.runDir, manifest)
}

// blindLabels are count shuffled single letters — the run names judges see.
func blindLabels(count int) ([]string, error) {
	if count > len(labelAlphabet) {
		return nil, fmt.Errorf("blindLabels: %d runs exceed the %d blind labels", count, len(labelAlphabet))
	}

	labels := strings.Split(labelAlphabet[:count], "")
	//nolint:gosec // G404: the order only hides run identity from judges; no secret depends on it
	rand.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
	return labels, nil
}

// An open arg with a placeholder would let a run replace what the runner fills in.
func checkOpenArgs(def *evals.Definition) error {
	declared := def.Invocation.Args
	for _, golden := range def.Goldens {
		declared = slices.Concat(declared, golden.Produce.Args, golden.Test.Args)
	}
	for _, arg := range declared {
		isOpen := slices.Contains(def.Invocation.Open, arg.Name)
		if isOpen && strings.Contains(string(arg.Value), "{") {
			return fmt.Errorf("checkOpenArgs: Open arg %q holds a placeholder", arg.Name)
		}
	}
	return nil
}

func checkPrerequisites(def *evals.Definition, skillDir string) error {
	var missing []string
	for _, path := range def.Prerequisites {
		if _, err := os.Stat(filepath.Join(skillDir, path)); err != nil {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("checkPrerequisites: %s: Missing %s", skillDir, strings.Join(missing, ", "))
	}
	return nil
}

// checkToken names the stored labels in its error, never a value.
func checkToken(label string) error {
	dir, err := routines.TokenDir()
	if err != nil {
		return fmt.Errorf("checkToken: %w", err)
	}

	labels, err := routines.ListTokenLabels(dir)
	if err != nil {
		return fmt.Errorf("checkToken: %w", err)
	}

	if !slices.Contains(labels, label) {
		return fmt.Errorf("checkToken: %q — stored labels: %s: %w", label, strings.Join(labels, ", "), ErrUnknownToken)
	}
	return nil
}

func checkWindow(budgets *evals.DefinitionBudgets, runCount int) error {
	window := budgets.WindowUsd
	if window == 0 {
		window = defaultWindowUsd
	}

	if raw := os.Getenv(windowBudgetEnv); raw != "" {
		parsed, err := strconv.ParseFloat(raw, floatBits)
		if err != nil {
			return fmt.Errorf("checkWindow: %s=%q: %w", windowBudgetEnv, raw, err)
		}
		window = parsed
	}

	planned := float64(runCount) * budgets.ArmUsd
	if planned > window {
		return fmt.Errorf(
			"checkWindow: %d runs × $%g = $%g over the $%g window: %w",
			runCount,
			budgets.ArmUsd,
			planned,
			window,
			ErrOverWindow,
		)
	}
	return nil
}

func checkoutRef(def *evals.Definition) string {
	if def.Environment.CheckoutRef != "" {
		return def.Environment.CheckoutRef
	}
	return def.Environment.Range.HeadSha
}

// configKey names a configuration the way an arm token did.
func configKey(disabledServers []string, effort, model string) string {
	var key strings.Builder
	key.WriteString(model + "-" + effort)
	for _, server := range disabledServers {
		key.WriteString("-no-" + server)
	}
	return key.String()
}

func copyFile(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	return nil
}

func findSkillDir(skill, skillsRoot string) (string, error) {
	if !skillNamePattern.MatchString(skill) {
		return "", fmt.Errorf("findSkillDir: Invalid skill name %q", skill)
	}

	definition := filepath.FromSlash(evals.DefinitionFile)
	patterns := []string{
		filepath.Join(skillsRoot, skill, definition),
		filepath.Join(skillsRoot, "*", skill, definition),
	}
	var found []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("findSkillDir: %w", err)
		}
		found = append(found, matches...)
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("findSkillDir: %s: %w", skill, ErrNoDefinition)
	case 1:
		return filepath.Dir(filepath.Dir(found[0])), nil
	}
	return "", fmt.Errorf("findSkillDir: %s: %d skill dirs match: %s", skill, len(found), strings.Join(found, ", "))
}

func kindArgs(def *evals.Definition, golden *ManifestGolden) []evals.DefinitionArg {
	switch golden.kind() {
	case GoldenProduce:
		return def.Goldens[0].Produce.Args
	case GoldenTest:
		return def.Goldens[0].Test.Args
	}
	return nil
}

func loadEvalDefinition(skill, skillsRoot string) (*evals.Definition, string, error) {
	skillDir, err := findSkillDir(skill, skillsRoot)
	if err != nil {
		return nil, "", err
	}

	def, err := evals.LoadDefinition(skillDir)
	if err != nil {
		return nil, "", err
	}

	if def == nil {
		return nil, "", fmt.Errorf("loadEvalDefinition: %s: %w", skillDir, ErrNoDefinition)
	}

	if def.SchemaVersion != definitionSchema {
		return nil, "", fmt.Errorf("loadEvalDefinition: %s: Schema %q, want %s", skillDir, def.SchemaVersion, definitionSchema)
	}

	if err := checkOpenArgs(def); err != nil {
		return nil, "", err
	}

	if err := checkPrerequisites(def, skillDir); err != nil {
		return nil, "", err
	}
	return def, skillDir, nil
}

func mergeArgs(args, layer []evals.DefinitionArg) []evals.DefinitionArg {
	merged := slices.Clone(args)
	for _, arg := range layer {
		index := slices.IndexFunc(merged, func(existing evals.DefinitionArg) bool { return existing.Name == arg.Name })
		if index < 0 {
			merged = append(merged, arg)
			continue
		}
		merged[index] = arg
	}
	return merged
}

// Layers merge by name: definition, golden kind, run overrides; the per-arm placeholders stay unfilled.
func renderInvocation(def *evals.Definition, golden *ManifestGolden, overrides []evals.DefinitionArg) (string, error) {
	for _, arg := range overrides {
		if !slices.Contains(def.Invocation.Open, arg.Name) {
			return "", fmt.Errorf("renderInvocation: Arg %q: %w", arg.Name, ErrClosedArg)
		}
	}

	args := mergeArgs(def.Invocation.Args, kindArgs(def, golden))
	args = mergeArgs(args, overrides)
	words := []string{"/" + def.Skill}
	if def.Invocation.Text != "" {
		words = append(words, def.Invocation.Text)
	}
	for _, arg := range args {
		words = append(words, arg.String())
	}

	prebuilt := []string{}
	if def.Environment.Prebuild != nil {
		prebuilt = def.Environment.Prebuild.Outputs
	}

	replacer := strings.NewReplacer(
		"{base}", def.Environment.Range.BaseSha,
		"{exclude}", strings.Join(def.Environment.Range.ExcludePaths, " "),
		"{head}", def.Environment.Range.HeadSha,
		"{prebuilt}", strings.Join(prebuilt, " "),
	)
	prompt := replacer.Replace(strings.Join(words, " "))
	if def.Findings.Glob == "" {
		return prompt, nil
	}
	return prompt + " " + handshake, nil
}

// replicaCount is the requested replicas, else the definition's, else 1.
func replicaCount(def *evals.Definition, requested int) int {
	switch {
	case requested > 0:
		return requested
	case def.Replicas > 0:
		return def.Replicas
	}
	return defaultReplicas
}

func shortRef(ref string) (string, error) {
	if len(ref) < runHashLength {
		return "", fmt.Errorf("shortRef: Checkout commit %q is shorter than %d digits", ref, runHashLength)
	}
	return strings.ToLower(ref[:runHashLength]), nil
}

// uniqueRunDir is base, else base-2, base-3, … — the first that does not exist.
func uniqueRunDir(base string) (string, error) {
	candidate := base
	for suffix := 2; ; suffix++ {
		_, err := os.Stat(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("uniqueRunDir: %w", err)
		}
		candidate = base + "-" + strconv.Itoa(suffix)
	}
}

// writeOverlay returns an absolute path, the form provision and the arm runner read.
func writeOverlay(content []byte, path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("writeOverlay: %w", err)
	}

	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		return "", fmt.Errorf("writeOverlay: %w", err)
	}
	return absolute, nil
}

func Compose(request *InitRequest) (string, error) {
	def, skillDir, err := loadEvalDefinition(request.Skill, request.SkillsRoot)
	if err != nil {
		return "", err
	}

	golden, err := newManifestGolden(def, request.Golden, skillDir)
	if err != nil {
		return "", err
	}
	return renderInvocation(def, golden, request.Overrides)
}

func Init(request *InitRequest) (string, error) {
	def, skillDir, err := loadEvalDefinition(request.Skill, request.SkillsRoot)
	if err != nil {
		return "", err
	}

	golden, err := newManifestGolden(def, request.Golden, skillDir)
	if err != nil {
		return "", err
	}

	if err := checkToken(request.Token); err != nil {
		return "", err
	}

	hash, err := shortRef(checkoutRef(def))
	if err != nil {
		return "", err
	}

	runDir, err := uniqueRunDir(filepath.Join(request.EvalsRoot, request.Skill, request.Date+"-"+hash))
	if err != nil {
		return "", err
	}

	spec := &runSpec{
		configs:   []*armConfig{newBaseConfig(def)},
		date:      request.Date,
		def:       def,
		golden:    golden,
		overrides: request.Overrides,
		replicas:  replicaCount(def, request.Replicas),
		runDir:    runDir,
		skill:     request.Skill,
		skillDir:  skillDir,
		token:     request.Token,
	}
	if err := checkWindow(&def.Budgets, len(spec.configs)*spec.replicas); err != nil {
		return "", err
	}

	if err := spec.write(); err != nil {
		return "", err
	}
	return runDir, nil
}

func ParseSets(sets []string) []evals.DefinitionArg {
	overrides := make([]evals.DefinitionArg, 0, len(sets))
	for _, set := range sets {
		name, value, _ := strings.Cut(set, "=")
		arg := evals.DefinitionArg{Name: name, Value: evals.DefinitionArgValue(value)}
		overrides = append(overrides, arg)
	}
	return overrides
}
