// Package models is the one model-token vocabulary every gate, runner and generator asks.
package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
)

const (
	claudePrefix  = "claude-"
	contextSuffix = "[1m]"
	inheritToken  = "inherit"

	workflowBlockFormat = `// Subagent model vocabulary — generated from availableModels and
// forbidden_subagent_models by the workflow generator (internal/models).
const SUBAGENT_MODELS = [%s]
const DEFAULT_SUBAGENT_MODEL = %q
const assertModel = (name, value) => {
  if (!SUBAGENT_MODELS.includes(value)) throw new Error(` + "`${name} \"${value}\" is not a subagent model (${SUBAGENT_MODELS.join(' | ')}) — unresolved tokens silently run the session model, forbidden ones are stopped by heartbeat`" + `)
  return value
}
`
)

// Prose twin: the plan rule's effort ladder — change one, change both.
var familyOrder = []string{"fable", "opus", "sonnet", "haiku"}

// skillAuthoringForbidden is the repo's skill-authoring policy for the committed GEN:models blocks; the machine's runtime list is a setting.
var skillAuthoringForbidden = []string{"*fable*"}

type heartbeatConfig struct {
	ForbiddenSubagentModels []string `json:"forbidden_subagent_models"`
}

type Vocabulary struct {
	Available []string
	Forbidden []string
}

// A missing file contributes an empty list.
func Load(settingsPath string, heartbeatPath string) (*Vocabulary, error) {
	settings, err := config.Load(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("models.Load: %w", err)
	}

	available, err := settings.AvailableModels()
	if err != nil {
		return nil, fmt.Errorf("models.Load: Failed to read availableModels of %s: %w", settingsPath, err)
	}

	forbidden, err := loadForbidden(heartbeatPath)
	if err != nil {
		return nil, err
	}

	vocabulary := &Vocabulary{Available: available, Forbidden: forbidden}
	return vocabulary, nil
}

// LoadHome is the deployed vocabulary of a Claude home (~/.claude).
func LoadHome(claudeHome string) (*Vocabulary, error) {
	settingsPath := filepath.Join(claudeHome, "settings.json")
	heartbeatPath := filepath.Join(claudeHome, "heartbeat", "config.json")
	return Load(settingsPath, heartbeatPath)
}

// LoadTemplates is the repo's authoring vocabulary: the template's availableModels and the skill-authoring forbidden list.
func LoadTemplates(repoRoot string) (*Vocabulary, error) {
	settings, err := config.Load(filepath.Join(repoRoot, "settings", "claude_code", "base.json"))
	if err != nil {
		return nil, fmt.Errorf("models.LoadTemplates: %w", err)
	}

	available, err := settings.AvailableModels()
	if err != nil {
		return nil, fmt.Errorf("models.LoadTemplates: Failed to read availableModels: %w", err)
	}

	vocabulary := &Vocabulary{Available: available, Forbidden: slices.Clone(skillAuthoringForbidden)}
	return vocabulary, nil
}

// For root-session tokens: settings model, ROUTINE_MODEL, eval arm.
func (v *Vocabulary) Check(token string) error {
	if v.Resolves(token) {
		return nil
	}
	return fmt.Errorf("Vocabulary.Check: Model %q does not resolve against availableModels (%s) — the harness would silently run the session model", token, strings.Join(v.Available, ", "))
}

// CheckSubagent fails a token that cannot run a subagent as named.
func (v *Vocabulary) CheckSubagent(token string) error {
	if v.IsSubagent(token) {
		return nil
	}
	return fmt.Errorf("Vocabulary.CheckSubagent: Model %q cannot run a subagent — name one of: %s (an unresolved token silently runs the session model, a forbidden one is stopped by heartbeat)", token, strings.Join(v.SubagentAliases(), ", "))
}

func (v *Vocabulary) DefaultSubagent() string {
	aliases := v.SubagentAliases()
	if len(aliases) == 0 {
		return ""
	}
	return aliases[0]
}

// IsForbidden reports a token a forbidden_subagent_models glob matches.
func (v *Vocabulary) IsForbidden(token string) bool {
	normalized := normalize(token)
	return slices.ContainsFunc(v.Forbidden, func(pattern string) bool {
		isMatch, err := path.Match(strings.ToLower(pattern), normalized)
		return err == nil && isMatch
	})
}

// IsSubagent reports a token that resolves and is not forbidden.
func (v *Vocabulary) IsSubagent(token string) bool {
	isForbidden := v.IsForbidden(token)
	return v.Resolves(token) && !isForbidden
}

// Mirrors the harness as probed 2026-09-29: empty availableModels admits any model.
func (v *Vocabulary) Resolves(token string) bool {
	normalized := normalize(token)
	if normalized == "" || normalized == inheritToken {
		return false
	}
	if len(v.Available) == 0 {
		return true
	}
	return slices.ContainsFunc(v.Available, func(entry string) bool {
		return admits(normalize(entry), normalized)
	})
}

func (v *Vocabulary) SubagentAliases() []string {
	aliases := make([]string, 0, len(familyOrder))
	for _, family := range familyOrder {
		if v.IsSubagent(family) {
			aliases = append(aliases, family)
		}
	}
	return aliases
}

// WorkflowBlock is the JS body of a stage file's GEN:models block.
func (v *Vocabulary) WorkflowBlock() string {
	aliases := v.SubagentAliases()
	quoted := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		quoted = append(quoted, fmt.Sprintf("%q", alias))
	}
	return fmt.Sprintf(workflowBlockFormat, strings.Join(quoted, ", "), v.DefaultSubagent())
}

func admits(entry string, token string) bool {
	isFamily := slices.Contains(familyOrder, token)
	switch {
	case token == entry:
		return true
	case strings.HasPrefix(token, claudePrefix):
		return strings.HasPrefix(token, entry+"-")
	case isFamily:
		return strings.HasPrefix(entry, claudePrefix+token+"-")
	}
	return false
}

func loadForbidden(heartbeatPath string) ([]string, error) {
	data, err := os.ReadFile(heartbeatPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("models.loadForbidden: Failed to read %s: %w", heartbeatPath, err)
	}

	var heartbeat heartbeatConfig
	if err := json.Unmarshal(data, &heartbeat); err != nil {
		return nil, fmt.Errorf("models.loadForbidden: Failed to parse %s: %w", heartbeatPath, err)
	}
	return heartbeat.ForbiddenSubagentModels, nil
}

func normalize(token string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(token)), contextSuffix)
}
