package skilleval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

const (
	masterSuffix     = ".master.json"
	provenanceRole   = "GENERATED from %s by go run ./cmd/skilleval goldens gen (scope %s) — do not edit"
	scopeContextless = "contextless"
	scopeFull        = "full"
)

// ErrBadMaster marks a master whose findings are empty or unclassified.
var ErrBadMaster = errors.New("bad golden master")

// knownRow is one seed-ledger row of <stem>.known.json.
type knownRow struct {
	Claim     json.RawMessage `json:"claim"`
	File      json.RawMessage `json:"file"`
	FindingId json.RawMessage `json:"finding_id"`
	Fix       string          `json:"fix"`
	Line      json.RawMessage `json:"line"`
	Round     int             `json:"round"`
	Severity  json.RawMessage `json:"severity"`
}

func derivedGoldens(masterPath string) (map[string][]byte, error) {
	var master map[string]json.RawMessage
	if err := readJsonFile(masterPath, &master); err != nil {
		return nil, err
	}

	var findings []map[string]json.RawMessage
	if err := json.Unmarshal(master["findings"], &findings); err != nil || len(findings) == 0 {
		return nil, fmt.Errorf("derivedGoldens: %s: findings not a non-empty array: %w", masterPath, ErrBadMaster)
	}

	for _, finding := range findings {
		if _, ok := finding["contextOnly"]; !ok {
			return nil, fmt.Errorf("derivedGoldens: %s: unclassified finding %s: %w", masterPath, finding["golden_id"], ErrBadMaster)
		}
	}

	masterName := filepath.Base(masterPath)
	stem := strings.TrimSuffix(masterName, masterSuffix)
	full, err := scopedGolden(findings, master, masterName, scopeFull)
	if err != nil {
		return nil, err
	}

	contextless, err := scopedGolden(findings, master, masterName, scopeContextless)
	if err != nil {
		return nil, err
	}

	known, err := knownLedger(findings)
	if err != nil {
		return nil, err
	}

	derived := map[string][]byte{
		stem + ".json":             full,
		stem + ".contextless.json": contextless,
		stem + ".known.json":       known,
	}
	return derived, nil
}

func knownLedger(findings []map[string]json.RawMessage) ([]byte, error) {
	rows := make([]knownRow, 0, len(findings))
	for _, finding := range findings {
		row := knownRow{
			Claim:     finding["claim"],
			File:      finding["file"],
			FindingId: finding["golden_id"],
			Line:      finding["line"],
			Severity:  finding["severity"],
		}
		rows = append(rows, row)
	}
	return marshalGolden(rows)
}

func marshalGolden(value any) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("marshalGolden: %w", err)
	}

	return out.Bytes(), nil
}

func scopedGolden(findings []map[string]json.RawMessage, master map[string]json.RawMessage, masterName, scope string) ([]byte, error) {
	kept := make([]map[string]json.RawMessage, 0, len(findings))
	for _, finding := range findings {
		isContextOnly := string(bytes.TrimSpace(finding["contextOnly"])) == "true"
		if scope == scopeContextless && isContextOnly {
			continue
		}

		stripped := maps.Clone(finding)
		delete(stripped, "contextOnly")
		delete(stripped, "contextReason")
		kept = append(kept, stripped)
	}

	var provenance map[string]json.RawMessage
	if err := json.Unmarshal(master["provenance"], &provenance); err != nil {
		return nil, fmt.Errorf("scopedGolden: %s: provenance: %w", masterName, err)
	}

	role, err := json.Marshal(fmt.Sprintf(provenanceRole, masterName, scope))
	if err != nil {
		return nil, fmt.Errorf("scopedGolden: %w", err)
	}

	provenance["role"] = role
	golden := make(map[string]any, len(master)+1)
	for key, value := range master {
		golden[key] = value
	}
	golden["findings"] = kept
	golden["provenance"] = provenance
	golden["scope"] = scope
	return marshalGolden(golden)
}

func CheckGoldens(skillsRoot string) ([]string, error) {
	masters, err := filepath.Glob(filepath.Join(skillsRoot, "*", "*", "eval", "golden", "*"+masterSuffix))
	if err != nil {
		return nil, fmt.Errorf("CheckGoldens: %w", err)
	}

	var stale []string
	for _, masterPath := range masters {
		derived, err := derivedGoldens(masterPath)
		if err != nil {
			return nil, err
		}

		for name, want := range derived {
			path := filepath.Join(filepath.Dir(masterPath), name)
			have, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(have, want) {
				stale = append(stale, path)
			}
		}
	}
	return stale, nil
}

// GenerateGoldens writes the three derived files of one master next to it.
func GenerateGoldens(masterPath string) error {
	if !strings.HasSuffix(masterPath, masterSuffix) {
		return fmt.Errorf("GenerateGoldens: %s: not a %s file", masterPath, masterSuffix)
	}

	derived, err := derivedGoldens(masterPath)
	if err != nil {
		return err
	}

	for name, data := range derived {
		if err := os.WriteFile(filepath.Join(filepath.Dir(masterPath), name), data, 0o644); err != nil {
			return fmt.Errorf("GenerateGoldens: %s: %w", name, err)
		}
	}
	return nil
}
