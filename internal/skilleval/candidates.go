package skilleval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/reviewscope"
)

const (
	candidatesFile = "candidates.json"
	claimKey       = "claim"
	jsonNull       = "null"
)

// candidateKeys are the finding fields copied verbatim into a candidate.
var candidateKeys = []string{"effect", "exposure", "file", "finding_id", "fix", "fix_state", "line", "round", "route", "severity"}

// candidate without a fix state reads as open.
type candidate struct {
	FindingId string               `json:"finding_id"`
	FixState  reviewscope.FixState `json:"fix_state"`
	Severity  string               `json:"severity"`
}

// isMachineFix counts a landed fix whether or not the fix check confirmed it.
func (c candidate) isMachineFix() bool {
	return c.FixState == reviewscope.FixStateApplied || c.FixState == reviewscope.FixStateFixed
}

// extractCandidates reads only the newest artifact matching the declared glob.
func extractCandidates(artifactsPath string, findings *evals.DefinitionFindings) ([]map[string]json.RawMessage, error) {
	candidates := []map[string]json.RawMessage{}
	artifact, err := newestMatch(filepath.Join(artifactsPath, findings.Glob))
	if err != nil {
		return nil, err
	}

	if artifact == "" {
		return candidates, nil
	}

	items, err := readItems(findings.ArrayKey, artifact)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		reduced, err := reduceCandidate(findings.ClaimKeys, item)
		if err != nil {
			return nil, fmt.Errorf("extractCandidates: %s: %w", artifact, err)
		}
		candidates = append(candidates, reduced)
	}
	return candidates, nil
}

func isJsonNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == jsonNull
}

// itemRound is an item's round and whether it carries one.
func itemRound(item map[string]json.RawMessage) (int, bool, error) {
	rawRound, hasRound := item["round"]
	if !hasRound {
		return 0, false, nil
	}

	var round int
	if err := json.Unmarshal(rawRound, &round); err != nil {
		return 0, false, fmt.Errorf("itemRound: %w", err)
	}
	return round, true, nil
}

func loadCandidates(path string) (map[string]candidate, error) {
	var rows []candidate
	if err := readJsonFile(path, &rows); err != nil {
		return nil, err
	}

	candidateById := make(map[string]candidate, len(rows))
	for _, row := range rows {
		candidateById[row.FindingId] = row
	}
	return candidateById, nil
}

func newestMatch(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("newestMatch: %s: %w", pattern, err)
	}

	newest := ""
	var newestInfo os.FileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return "", fmt.Errorf("newestMatch: %w", err)
		}
		if newestInfo == nil || info.ModTime().After(newestInfo.ModTime()) {
			newest, newestInfo = match, info
		}
	}
	return newest, nil
}

func readItems(arrayKey, artifact string) ([]map[string]json.RawMessage, error) {
	items := []map[string]json.RawMessage{}
	if arrayKey == "" {
		if err := readJsonFile(artifact, &items); err != nil {
			return nil, err
		}
		return items, nil
	}

	var document map[string]json.RawMessage
	if err := readJsonFile(artifact, &document); err != nil {
		return nil, err
	}

	raw, ok := document[arrayKey]
	if !ok || isJsonNull(raw) {
		return items, nil
	}

	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("readItems: %s: %s: %w", artifact, arrayKey, err)
	}
	return items, nil
}

func reduceCandidate(claimKeys []string, item map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	candidate := make(map[string]json.RawMessage, len(candidateKeys)+1)
	for _, key := range candidateKeys {
		if value, ok := item[key]; ok {
			candidate[key] = value
		}
	}
	for _, key := range claimKeys {
		value, ok := item[key]
		if ok && !isJsonNull(value) {
			candidate[claimKey] = value
			break
		}
	}

	findingId, err := roundedId(item)
	if err != nil {
		return nil, err
	}

	candidate["finding_id"] = findingId
	return candidate, nil
}

// roundedId prefixes the round (r2-tests-MIN-1) so the ids of a multi-round ledger stay unique.
func roundedId(item map[string]json.RawMessage) (json.RawMessage, error) {
	round, hasRound, err := itemRound(item)
	if err != nil {
		return nil, err
	}

	if !hasRound {
		return item["finding_id"], nil
	}

	var findingId string
	if err := json.Unmarshal(item["finding_id"], &findingId); err != nil {
		return nil, fmt.Errorf("roundedId: %w", err)
	}

	rounded, err := json.Marshal("r" + strconv.Itoa(round) + "-" + findingId)
	if err != nil {
		return nil, fmt.Errorf("roundedId: %w", err)
	}
	return rounded, nil
}
