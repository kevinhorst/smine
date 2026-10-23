package skilleval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func Promote(label, runDir string) (string, error) {
	manifest, err := loadManifest(runDir)
	if err != nil {
		return "", err
	}

	if manifest.Golden.kind() != GoldenProduce {
		return "", fmt.Errorf("Promote: %s is no golden produce run", runDir)
	}

	if label == "" && len(manifest.Arms) == 1 {
		label = manifest.Arms[0].Label
	}

	arm, err := manifest.armByLabel(label)
	if err != nil {
		return "", err
	}

	armDir := filepath.Join(runDir, runsDir, arm.Label)
	var result ArmResult
	if err := readJsonFile(filepath.Join(armDir, resultFile), &result); err != nil {
		return "", err
	}

	isFailed := result.IsError || result.ExitStatus != 0
	if isFailed || len(result.InvalidReasons) > 0 {
		return "", fmt.Errorf("Promote: Arm %s failed or is invalid", arm.Label)
	}

	seedPath := filepath.Join(manifest.PresetSkillDir, manifest.Golden.SeedFile)
	live, err := os.ReadFile(seedPath)
	if err != nil {
		return "", fmt.Errorf("Promote: %w", err)
	}

	snapshot, err := os.ReadFile(filepath.Join(runDir, seedSnapshotFile))
	if err != nil {
		return "", fmt.Errorf("Promote: %w", err)
	}

	if !bytes.Equal(live, snapshot) {
		return "", fmt.Errorf("Promote: %s changed since the run started — this run does not hold its newer rows", seedPath)
	}

	findings, err := newestMatch(filepath.Join(armDir, artifactsDir, manifest.Findings.Glob))
	if err != nil {
		return "", err
	}

	var rows, added []json.RawMessage
	if err := json.Unmarshal(live, &rows); err != nil {
		return "", fmt.Errorf("Promote: %s: %w", seedPath, err)
	}

	if err := readJsonFile(findings, &added); err != nil {
		return "", err
	}

	// No HTML escaping: the seed's existing rows hold raw <, > and & and must not change.
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(append(rows, added...)); err != nil {
		return "", fmt.Errorf("Promote: %w", err)
	}

	if err := os.WriteFile(seedPath, out.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("Promote: %w", err)
	}
	return seedPath, nil
}
