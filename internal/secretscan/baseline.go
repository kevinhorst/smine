package secretscan

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	baselineFileName = ".secretscan-baseline"
	saltLength       = 16
)

type baselineEntry struct {
	Detector  string `json:"detector"`
	Path      string `json:"path"`
	ValueHash string `json:"valueHash"`
}

type baselineFile struct {
	Entries []baselineEntry `json:"entries"`
	Salt    string          `json:"salt"`
}

func (b *baselineFile) saltBytes() ([]byte, error) {
	if b.Salt == "" {
		return nil, nil
	}
	salt, err := hex.DecodeString(b.Salt)
	if err != nil {
		return nil, fmt.Errorf("baselineFile.saltBytes: Malformed salt in %s: %w", baselineFileName, err)
	}

	return salt, nil
}

func compareEntries(left, right baselineEntry) int {
	if result := strings.Compare(left.Path, right.Path); result != 0 {
		return result
	}
	if result := strings.Compare(left.Detector, right.Detector); result != 0 {
		return result
	}

	return strings.Compare(left.ValueHash, right.ValueHash)
}

func loadBaseline(repoPath string) (*baselineFile, error) {
	data, err := os.ReadFile(filepath.Join(repoPath, baselineFileName))
	if os.IsNotExist(err) {
		return &baselineFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loadBaseline: Failed to read %s: %w", baselineFileName, err)
	}

	baseline := &baselineFile{}
	if err := json.Unmarshal(data, baseline); err != nil {
		return nil, fmt.Errorf("loadBaseline: Malformed %s: %w", baselineFileName, err)
	}

	return baseline, nil
}

func saveBaseline(repoPath string, baseline *baselineFile) error {
	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("saveBaseline: Failed to marshal baseline: %w", err)
	}
	data = append(data, '\n')

	path := filepath.Join(repoPath, baselineFileName)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("saveBaseline: Failed to write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("saveBaseline: Failed to rename %s: %w", tmpPath, err)
	}

	return nil
}

func splitByBaseline(result *Result) error {
	salt, err := result.baseline.saltBytes()
	if err != nil {
		return err
	}

	baselined := make(map[baselineEntry]bool, len(result.baseline.Entries))
	for _, entry := range result.baseline.Entries {
		baselined[entry] = true
	}

	for _, finding := range result.treeFindings {
		entry := baselineEntry{
			Detector:  finding.Detector,
			Path:      finding.Path,
			ValueHash: valueHash(salt, finding.value),
		}
		if baselined[entry] {
			result.BaselinedFindings = append(result.BaselinedFindings, finding)
			continue
		}
		result.NewFindings = append(result.NewFindings, finding)
	}

	return nil
}

func valueHash(salt []byte, value string) string {
	hasher := sha256.New()
	hasher.Write(salt)
	hasher.Write([]byte(value))

	return hex.EncodeToString(hasher.Sum(nil))
}

// WriteBaseline rewrites the repo's baseline to exactly the current tree
// findings: new findings become accepted, stale entries drop out. The salt is
// kept across writes; a missing salt is created once and persisted.
func WriteBaseline(repoPath string, result *Result) error {
	baseline := result.baseline
	if baseline.Salt == "" {
		saltBytes := make([]byte, saltLength)
		if _, err := rand.Read(saltBytes); err != nil {
			return fmt.Errorf("WriteBaseline: Failed to create salt: %w", err)
		}
		baseline.Salt = hex.EncodeToString(saltBytes)
	}
	salt, err := baseline.saltBytes()
	if err != nil {
		return err
	}

	entries := make([]baselineEntry, 0, len(result.treeFindings))
	for _, finding := range result.treeFindings {
		entries = append(entries, baselineEntry{
			Detector:  finding.Detector,
			Path:      finding.Path,
			ValueHash: valueHash(salt, finding.value),
		})
	}
	slices.SortFunc(entries, compareEntries)
	baseline.Entries = slices.Compact(entries)

	if err := saveBaseline(repoPath, baseline); err != nil {
		return err
	}

	result.BaselinedFindings = append(result.BaselinedFindings, result.NewFindings...)
	sortFindings(result.BaselinedFindings)
	result.NewFindings = result.NewFindings[:0]
	return nil
}
