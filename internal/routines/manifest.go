package routines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ManifestFileName's presence makes a routines/ subdir a routine.
const ManifestFileName = "routine.json"

type Kind string

const (
	KindBudget       Kind = "budget"
	KindSingle       Kind = "single"
	KindSmineNightly Kind = "smine-nightly"
)

var promptSkillRe = regexp.MustCompile(`^/([A-Za-z0-9-]+)`)

type Manifest struct {
	Group  string `json:"group,omitempty"`
	Kind   Kind   `json:"kind"`
	Prompt string `json:"prompt,omitempty"`
}

func LoadManifest(routineDir string) (*Manifest, error) {
	path := filepath.Join(routineDir, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadManifest: Failed to read %s: %w", path, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("LoadManifest: Failed to parse %s: %w", path, err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("LoadManifest: %s: %w", path, err)
	}
	return &manifest, nil
}

func (m *Manifest) Skill() string {
	match := promptSkillRe.FindStringSubmatch(m.Prompt)
	if match == nil {
		return ""
	}
	return match[1]
}

func (m *Manifest) Validate() error {
	// Group
	if m.Group != "" && !namePattern.MatchString(m.Group) {
		return fmt.Errorf("Manifest.Validate: Invalid field group %q: only lowercase letters, digits and '-'", m.Group)
	}

	// Kind
	switch m.Kind {
	case KindBudget:
		// Prompt
		if m.Prompt != "" {
			return fmt.Errorf("Manifest.Validate: Field prompt is not allowed for kind %s", m.Kind)
		}
	case KindSingle, KindSmineNightly:
		// Prompt
		if m.Prompt == "" {
			return fmt.Errorf("Manifest.Validate: Missing field prompt for kind %s", m.Kind)
		}
	default:
		return fmt.Errorf("Manifest.Validate: Invalid field kind %q", m.Kind)
	}
	return nil
}
