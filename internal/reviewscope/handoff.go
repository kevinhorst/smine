package reviewscope

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kevinhorst/smine/internal/workflowgen"
)

// probesDir is the round's persisted-probe dir the handoff copy carries.
const probesDir = "probes"

// Rejection is one human-rejected finding in the rejection ledger.
type Rejection struct {
	FindingId string `json:"finding_id"`
	Reason    string `json:"reason"`
	Round     int    `json:"round"`
}

// compileStageSchema resolves shared-property placeholders from the files beside the schema, as deploy does.
func compileStageSchema(schemaPath string) (*jsonschema.Schema, error) {
	document, err := loadSchemaDocument(schemaPath)
	if err != nil {
		return nil, err
	}

	replacements := make(map[string]any, len(workflowgen.SharedProps))
	for _, shared := range workflowgen.SharedProps {
		sharedPath := filepath.Join(filepath.Dir(schemaPath), shared.File)
		if _, err := os.Stat(sharedPath); err != nil {
			continue
		}

		replacements[shared.Token], err = loadSchemaDocument(sharedPath)
		if err != nil {
			return nil, err
		}
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaPath, resolvePlaceholders(document, replacements)); err != nil {
		return nil, fmt.Errorf("compileStageSchema: %s: %w", schemaPath, err)
	}

	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compileStageSchema: %s: %w", schemaPath, err)
	}
	return schema, nil
}

// loadSchemaDocument decodes one JSON schema file.
func loadSchemaDocument(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadSchemaDocument: %w", err)
	}

	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("loadSchemaDocument: %s is not JSON: %w", path, err)
	}
	return document, nil
}

func resolvePlaceholders(node any, replacements map[string]any) any {
	switch value := node.(type) {
	case string:
		if replacement, isToken := replacements[value]; isToken {
			return replacement
		}
	case map[string]any:
		for key, child := range value {
			value[key] = resolvePlaceholders(child, replacements)
		}
	case []any:
		for index, child := range value {
			value[index] = resolvePlaceholders(child, replacements)
		}
	}
	return node
}

func validateReview(reviewPath, schemaPath string) error {
	schema, err := compileStageSchema(schemaPath)
	if err != nil {
		return fmt.Errorf("validateReview: %w", err)
	}

	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return fmt.Errorf("validateReview: %w", err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("validateReview: %s is not JSON: %w", reviewPath, err)
	}

	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("validateReview: %s violates %s: %w", reviewPath, schemaPath, err)
	}
	return nil
}

// isReconfirmed detects an oscillation: a finding confirmed again after its fix landed.
func isReconfirmed(known []KnownEntry, entry rankedEntry) bool {
	for _, row := range known {
		isSame := row.FindingId == entry.FindingId || (row.File == entry.File && row.Claim == entry.Claim)
		if row.FixState == FixStateFixed && isSame {
			return true
		}
	}
	return false
}

func copyHandoff(roundDir, planDir string, round int) error {
	target := filepath.Join(planDir, "reviews", "round-"+strconv.Itoa(round))
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("copyHandoff: %w", err)
	}

	if err := copyFile(filepath.Join(roundDir, reviewFile), filepath.Join(target, reviewFile)); err != nil {
		return err
	}

	probes := filepath.Join(roundDir, probesDir)
	err := filepath.WalkDir(probes, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative := strings.TrimPrefix(path, probes)
		destination := filepath.Join(target, probesDir, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyFile(path, destination)
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("copyHandoff: %w", err)
	}
	return nil
}

// copyFile copies one regular file.
func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}

	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return fmt.Errorf("copyFile: %w", err)
	}
	return nil
}

// AppendRejection appends one human-rejected finding to the rejection ledger.
func AppendRejection(rejectedPath string, rejection Rejection) error {
	rejections := []Rejection{}
	if _, err := os.Stat(rejectedPath); err == nil {
		if err := readJson(rejectedPath, &rejections); err != nil {
			return err
		}
	}

	rejections = append(rejections, rejection)
	return writeJson(rejectedPath, rejections)
}
