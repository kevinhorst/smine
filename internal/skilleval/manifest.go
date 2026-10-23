package skilleval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
)

const (
	digestFile          = "run.json.sha256"
	goldenSnapshotFile  = "golden.json"
	manifestFile        = "run.json"
	provisionFile       = "provision.json"
	runsDir             = "runs"
	scoringSnapshotFile = "scoring.json"
	seedSnapshotFile    = "seed.json"
)

const (
	GoldenProduce = "produce"
	GoldenTest    = "test"
)

var ErrManifestChanged = errors.New("run.json changed after init")

// ManifestArm's OverlayPatch, when set, replaces the run overlay (an A/B overlay variant).
type ManifestArm struct {
	Id              string   `json:"id"`
	DisabledServers []string `json:"disabledServers"`
	Effort          string   `json:"effort"`
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Model           string   `json:"model"`
	OverlayPatch    string   `json:"overlayPatch,omitempty"`
	Replica         int      `json:"replica"`
	Worktree        string   `json:"worktree"`
}

type ManifestGolden struct {
	Id       string                `json:"id"`
	Kind     string                `json:"kind"`
	Range    evals.DefinitionRange `json:"range"`
	Scope    string                `json:"scope"`
	SeedFile string                `json:"seedFile"`
}

func newManifestGolden(def *evals.Definition, requested, skillDir string) (*ManifestGolden, error) {
	if len(def.Goldens) == 0 {
		if requested != "" {
			return nil, fmt.Errorf("newManifestGolden: %s declares no golden: %w", def.Skill, ErrBadRunArg)
		}
		return nil, nil
	}

	first := &def.Goldens[0]
	golden := &ManifestGolden{Id: first.Id, Kind: requested, Range: def.Environment.Range, Scope: first.Scope}
	if golden.Kind == "" {
		golden.Kind = GoldenTest
	}

	switch golden.Kind {
	case GoldenProduce:
		golden.SeedFile = first.Produce.SeedFile
		if _, err := os.Stat(filepath.Join(skillDir, golden.SeedFile)); err != nil {
			return nil, fmt.Errorf("newManifestGolden: Seed file: %w", err)
		}
	case GoldenTest:
		if _, err := os.Stat(filepath.Join(skillDir, first.File)); err != nil {
			return nil, fmt.Errorf("newManifestGolden: %s is not minted — run golden: produce: %w", first.File, ErrNoGolden)
		}
	}
	return golden, nil
}

// A run without a golden has a nil block and kind "".
func (g *ManifestGolden) kind() string {
	if g == nil {
		return ""
	}
	return g.Kind
}

type provisionRecord struct {
	At        string            `json:"at"`
	Reference string            `json:"reference"`
	Repo      string            `json:"repo"`
	Worktrees map[string]string `json:"worktrees"`
}

func newProvisionRecord(manifest *RunManifest) *provisionRecord {
	record := &provisionRecord{
		At:        manifest.At,
		Reference: manifest.Reference,
		Repo:      manifest.Repo,
		Worktrees: make(map[string]string, len(manifest.Arms)),
	}
	for _, arm := range manifest.Arms {
		record.Worktrees[arm.Label] = arm.Worktree
	}
	return record
}

// RunManifest is run.json; provision, the arm runner, the briefs and the fold derive everything else from it.
type RunManifest struct {
	Arms           []ManifestArm             `json:"arms"`
	At             string                    `json:"at"`
	BudgetUsd      float64                   `json:"budgetUsd"`
	Check          []string                  `json:"check"`
	CheckTimeoutS  int                       `json:"checkTimeoutS"`
	ContextIndex   string                    `json:"contextIndex"`
	Date           string                    `json:"date"`
	Findings       evals.DefinitionFindings  `json:"findings"`
	Golden         *ManifestGolden           `json:"golden,omitempty"`
	Judge          string                    `json:"judge"`
	Layer          string                    `json:"layer"`
	Mode           string                    `json:"mode"`
	OverlayPatch   string                    `json:"overlayPatch"`
	Overrides      []evals.DefinitionArg     `json:"overrides,omitempty"`
	Prebuild       *evals.DefinitionPrebuild `json:"prebuild"`
	PresetSkillDir string                    `json:"presetSkillDir"`
	Prompt         string                    `json:"prompt"`
	Reference      string                    `json:"reference"`
	Repo           string                    `json:"repo"`
	RepoName       string                    `json:"repoName"`
	Scorer         []string                  `json:"scorer"`
	Skill          string                    `json:"skill"`
	SkillMd        string                    `json:"skillMd"`
	SkillVersion   string                    `json:"skillVersion"`
	TimeoutS       int                       `json:"timeoutS"`
	TokenLabel     string                    `json:"token"`
}

func (m *RunManifest) applyProvision(record *provisionRecord) {
	m.At, m.Reference, m.Repo = record.At, record.Reference, record.Repo
	for index := range m.Arms {
		m.Arms[index].Worktree = record.Worktrees[m.Arms[index].Label]
	}
}

// authLabel never returns the token itself.
func (m *RunManifest) authLabel() string {
	if m.TokenLabel == "" {
		return "inherited session auth (claude CLI login unless CLAUDE_CODE_OAUTH_TOKEN is set)"
	}
	return "token " + m.TokenLabel
}

func (m *RunManifest) armByLabel(label string) (*ManifestArm, error) {
	for index := range m.Arms {
		if m.Arms[index].Label == label {
			return &m.Arms[index], nil
		}
	}
	return nil, fmt.Errorf("RunManifest.armByLabel: Unknown arm label %s", label)
}

func (m *RunManifest) armOverlay(arm *ManifestArm) string {
	if arm.OverlayPatch != "" {
		return arm.OverlayPatch
	}
	return m.OverlayPatch
}

// isCompared: only an A/B side-by-side run gets a compare judge ranking.
func (m *RunManifest) isCompared() bool {
	return len(armKeys(m)) > 1
}

// requiredServers void the comparison unless they are connected here.
func (m *RunManifest) requiredServers(arm *ManifestArm) []string {
	isRequired := make(map[string]bool)
	for _, other := range m.Arms {
		for _, server := range other.DisabledServers {
			isRequired[server] = true
		}
	}
	for _, server := range arm.DisabledServers {
		delete(isRequired, server)
	}

	required := make([]string, 0, len(isRequired))
	for server := range isRequired {
		required = append(required, server)
	}
	sort.Strings(required)
	return required
}

func digestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// A run dir written before the digest existed has none and loads unchecked.
func loadManifest(runDir string) (*RunManifest, error) {
	data, err := os.ReadFile(filepath.Join(runDir, manifestFile))
	if err != nil {
		return nil, fmt.Errorf("loadManifest: %w", err)
	}

	digest, err := os.ReadFile(filepath.Join(runDir, digestFile))
	isChanged := err == nil && strings.TrimSpace(string(digest)) != digestOf(data)
	if isChanged {
		return nil, fmt.Errorf("loadManifest: %s: %w", runDir, ErrManifestChanged)
	}

	var manifest RunManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("loadManifest: %s: %w", runDir, err)
	}

	var record provisionRecord
	err = readJsonFile(filepath.Join(runDir, provisionFile), &record)
	if errors.Is(err, fs.ErrNotExist) {
		return &manifest, nil
	}

	if err != nil {
		return nil, err
	}

	manifest.applyProvision(&record)
	return &manifest, nil
}

func readJsonFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readJsonFile: %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("readJsonFile: %s: %w", path, err)
	}

	return nil
}

func writeJsonFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("writeJsonFile: %s: %w", path, err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writeJsonFile: %s: %w", path, err)
	}

	return nil
}

func writeManifest(runDir string, manifest *RunManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("writeManifest: %w", err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(runDir, manifestFile), data, 0o644); err != nil {
		return fmt.Errorf("writeManifest: %w", err)
	}

	if err := os.WriteFile(filepath.Join(runDir, digestFile), []byte(digestOf(data)+"\n"), 0o644); err != nil {
		return fmt.Errorf("writeManifest: %w", err)
	}

	return nil
}
