package skillgraph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
)

// payloadExtension is the file extension of a payload body file.
const payloadExtension = ".md"

func discoverChild(dir string) ([]string, error) {
	isSkill, err := isModelDir(dir)
	if err != nil {
		return nil, err
	}

	if isSkill {
		found := []string{dir}
		return found, nil
	}

	children, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("discoverChild: Failed to read %s: %w", dir, err)
	}

	var found []string
	var problems []error
	for _, child := range children {
		if !child.IsDir() {
			continue
		}

		childDir := filepath.Join(dir, child.Name())
		isChildSkill, err := isModelDir(childDir)
		switch {
		case err != nil:
			problems = append(problems, err)
		case isChildSkill:
			found = append(found, childDir)
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return found, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("fileExists: Failed to stat %s: %w", path, err)
	}
	return true, nil
}

// isModelDir errors on a legacy skill.cue and on a SKILL.md without a model: every repo skill is model-backed.
func isModelDir(dir string) (bool, error) {
	hasModel, err := fileExists(ModelPath(dir))
	if err != nil {
		return false, err
	}

	if hasModel {
		return true, nil
	}

	legacyPath := filepath.Join(dir, LegacyModelFile)
	hasLegacy, err := fileExists(legacyPath)
	if err != nil {
		return false, err
	}

	if hasLegacy {
		return false, fmt.Errorf("isModelDir: %s holds the legacy model file — run: git mv %s %s", dir, legacyPath, ModelPath(dir))
	}

	manifestPath := filepath.Join(dir, FileManifest)
	hasManifest, err := fileExists(manifestPath)
	if err != nil {
		return false, err
	}

	if hasManifest {
		return false, fmt.Errorf("isModelDir: %s has SKILL.md but no %s — run: make skill-import DIR=%s", dir, ModelFile(filepath.Base(dir)), dir)
	}
	return false, nil
}

// loadViolations joins Check's violations, each prefixed with the skill dir.
func loadViolations(dir string, violations []string) error {
	problems := make([]error, 0, len(violations))
	for _, violation := range violations {
		problems = append(problems, errors.New(dir+": "+violation))
	}
	return errors.Join(problems...)
}

func payloadFile(id string) string {
	return path.Join(DirPayloads, id+payloadExtension)
}

// readPayloads relies on schema-validated ids to keep the path inside dir.
func readPayloads(dir string, skill *Skill) error {
	for _, ref := range skill.Entries() {
		if ref.Entry.Payload == nil {
			continue
		}

		bodyPath := filepath.Join(dir, payloadFile(ref.Entry.Id))
		body, err := os.ReadFile(bodyPath)
		if err != nil {
			return fmt.Errorf("readPayloads: Payload entry %s: Failed to read %s: %w", ref.Entry.Id, bodyPath, err)
		}

		ref.Entry.Payload.Body = string(body)
	}
	return nil
}

// Discover errors on a missing root so the gate never passes vacuously.
func Discover(root string) ([]string, error) {
	children, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("Discover: Failed to read %s: %w", root, err)
	}

	var dirs []string
	var problems []error
	for _, child := range children {
		if !child.IsDir() {
			continue
		}

		childDir := filepath.Join(root, child.Name())
		found, err := discoverChild(childDir)
		if err != nil {
			problems = append(problems, err)
			continue
		}

		dirs = append(dirs, found...)
	}
	if len(problems) > 0 {
		joined := errors.Join(problems...)
		return nil, fmt.Errorf("Discover: %w", joined)
	}

	slices.Sort(dirs)
	return dirs, nil
}

func FindRoot(dir string) (string, error) {
	candidates := []string{filepath.Join(dir, ".."), filepath.Join(dir, "..", "..")}
	for _, candidate := range candidates {
		hasVocabulary, err := fileExists(filepath.Join(candidate, FileVocabulary))
		if err != nil {
			return "", fmt.Errorf("FindRoot: %w", err)
		}

		if hasVocabulary {
			return candidate, nil
		}
	}
	return "", nil
}

func FindVocabulary(dir string) (*Vocabulary, error) {
	root, err := FindRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("FindVocabulary: %w", err)
	}

	if root == "" {
		empty := &Vocabulary{}
		return empty, nil
	}
	return LoadVocabulary(filepath.Join(root, FileVocabulary))
}

func Load(dir string) (*Skill, error) {
	modelPath := ModelPath(dir)
	data, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("Load: Failed to read %s: %w", modelPath, err)
	}

	skill, err := DecodeSkill(data, modelPath)
	if err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	if skill.Name() != filepath.Base(dir) {
		return nil, fmt.Errorf("Load: %s: Frontmatter name %q differs from the skill dir", modelPath, skill.Name())
	}

	if err := readPayloads(dir, skill); err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	vocabulary, err := FindVocabulary(dir)
	if err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	skill.Vocabulary = vocabulary
	catalog, err := FindCatalog(dir)
	if err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	skill.Catalog = catalog
	skill.Dir = dir
	queries, err := LoadQueries(dir)
	if err != nil {
		return nil, fmt.Errorf("Load: %w", err)
	}

	skill.Queries = queries
	violations := Check(skill)
	violations = append(violations, smokeFixtureViolations(dir, skill.Smoke)...)
	violations = append(violations, checkFieldFiles(dir, skill)...)
	violations = append(violations, checkLiterals(dir, skill)...)
	if len(violations) > 0 {
		return nil, loadViolations(dir, violations)
	}
	return skill, nil
}

// LoadVocabulary reads and schema-validates a shared vocabulary.cue.
func LoadVocabulary(vocabularyPath string) (*Vocabulary, error) {
	data, err := os.ReadFile(vocabularyPath)
	if err != nil {
		return nil, fmt.Errorf("LoadVocabulary: Failed to read %s: %w", vocabularyPath, err)
	}

	vocabulary, err := decodeVocabulary(data, vocabularyPath)
	if err != nil {
		return nil, fmt.Errorf("LoadVocabulary: %w", err)
	}
	return vocabulary, nil
}
