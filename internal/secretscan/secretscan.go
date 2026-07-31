// Package secretscan detects credentials in a git repository's working tree
// and history. Production code is standard library only and deterministic:
// same input, same findings, byte-for-byte identical reports.
package secretscan

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	binarySniffLength = 8000
	excerptMaxLength  = 80
	maxFileSize       = 5 * 1024 * 1024
)

type Finding struct {
	value string

	Confidence string `json:"confidence"`
	Detector   string `json:"detector"`
	Excerpt    string `json:"excerpt"`
	Line       int    `json:"line"`
	Path       string `json:"path"`
}

type Options struct {
	ShouldScanHistory bool
}

type Result struct {
	baseline     *baselineFile
	treeFindings []Finding

	BaselinedFindings []Finding        `json:"baselinedFindings"`
	BinarySkipCount   int              `json:"binarySkipCount"`
	HistoryFindings   []HistoryFinding `json:"historyFindings"`
	NewFindings       []Finding        `json:"newFindings"`
	OversizeFiles     []string         `json:"oversizeFiles"`
	RepoPath          string           `json:"repoPath"`
}

func guardGitRepo(repoPath string) error {
	gitPath := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		return fmt.Errorf("guardGitRepo: Not a git repository (no .git in %s): %w", repoPath, err)
	}

	return nil
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}

		return findings[i].Detector < findings[j].Detector
	})
}

// Scan is read-only: it never writes to the repository or the baseline.
func Scan(repoPath string, options Options) (*Result, error) {
	repoPath = filepath.Clean(repoPath)
	if err := guardGitRepo(repoPath); err != nil {
		return nil, err
	}

	patterns, err := compilePatterns()
	if err != nil {
		return nil, err
	}

	result := &Result{
		BaselinedFindings: make([]Finding, 0),
		HistoryFindings:   make([]HistoryFinding, 0),
		NewFindings:       make([]Finding, 0),
		OversizeFiles:     make([]string, 0),
		RepoPath:          repoPath,
		treeFindings:      make([]Finding, 0),
	}
	if err := scanTree(repoPath, patterns, result); err != nil {
		return nil, err
	}

	baseline, err := loadBaseline(repoPath)
	if err != nil {
		return nil, err
	}

	result.baseline = baseline
	if err := splitByBaseline(result); err != nil {
		return nil, err
	}
	if !options.ShouldScanHistory {
		return result, nil
	}

	historyFindings, err := scanHistory(repoPath, patterns)
	if err != nil {
		return nil, err
	}

	result.HistoryFindings = historyFindings
	return result, nil
}
