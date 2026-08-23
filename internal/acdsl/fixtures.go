package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// FixturesDir is the per-rule example tree: <root>/acdsl/testdata/<rule-id>/
// pass/ and fail/. The testdata name keeps go tooling and every anchor
// (by the /testdata/ exclusion convention) away from the example files.
const FixturesDir = "acdsl/testdata"

// FixturesReport is the outcome of a fixtures run: verdict mismatches plus
// run metadata — rules with examples, files exercised, wall-clock elapsed.
type FixturesReport struct {
	Failures []string
	Checked  int
	Files    int
	Skipped  int // rules with examples whose needs= artifacts are absent
	Elapsed  time.Duration
}

// Meta renders the metadata suffix shared by the CLI and the UI.
func (r FixturesReport) Meta() string {
	meta := fmt.Sprintf("%d file(s) · %s", r.Files, r.Elapsed.Round(time.Millisecond))
	if r.Skipped > 0 {
		meta = fmt.Sprintf("%d skipped (needs) · %s", r.Skipped, meta)
	}
	return meta
}

// RunFixtures proves each rule's verifier against its committed examples:
// every pass file set must exit 0, every fail set must exit 1 — the examples
// are the verifier's ground truth. Rules without a fixtures dir are skipped.
func RunFixtures(ctx context.Context, root string, rules []Rule, registry map[string]RegistryEntry) (FixturesReport, error) {
	start := time.Now()
	var report FixturesReport
	for _, rule := range rules {
		ruleDir := filepath.Join(root, FixturesDir, rule.Id)
		if _, statErr := os.Stat(ruleDir); statErr != nil {
			continue
		}
		// Same applicability as Check: a rule whose needs= artifacts are
		// absent (private artifacts on a public tree) cannot exercise its
		// fixtures either — the verifier would read the missing artifact.
		if len(missingNeeds(root, rule)) > 0 {
			report.Skipped++
			continue
		}
		entry, ok := registry[rule.Verifier]
		if !ok {
			return FixturesReport{}, fmt.Errorf("RunFixtures: %s: verifier %q not registered", rule.Id, rule.Verifier)
		}
		report.Checked++
		passFailures, passFiles, err := runFixtureSet(ctx, root, ruleDir, rule, entry, "pass", true)
		if err != nil {
			return FixturesReport{}, err
		}
		report.Failures = append(report.Failures, passFailures...)
		report.Files += passFiles
		failFailures, failFiles, err := runFixtureSet(ctx, root, ruleDir, rule, entry, "fail", false)
		if err != nil {
			return FixturesReport{}, err
		}
		report.Failures = append(report.Failures, failFailures...)
		report.Files += failFiles
	}
	report.Elapsed = time.Since(start)
	return report, nil
}

// runFixtureSet runs one rule's verifier over a single example set (pass or
// fail) and returns any verdict mismatches. A pass set flags nothing; a fail
// set must flag; an empty set is itself a failure.
func runFixtureSet(ctx context.Context, root, ruleDir string, rule Rule, entry RegistryEntry, set string, wantGreen bool) ([]string, int, error) {
	files, err := fixtureFiles(ruleDir, set)
	if err != nil {
		return nil, 0, err
	}
	if len(files) == 0 {
		return []string{fmt.Sprintf("%s: fixtures/%s is empty", rule.Id, set)}, 0, nil
	}
	diagnostics, err := runVerifier(ctx, root, rule, entry, files)
	if err != nil {
		return nil, 0, err
	}
	var failures []string
	if wantGreen && len(diagnostics) > 0 {
		failures = append(failures, fmt.Sprintf("%s: pass fixture flagged: %s", rule.Id, diagnostics[0].Message))
	}
	if !wantGreen && len(diagnostics) == 0 {
		failures = append(failures, fmt.Sprintf("%s: fail fixture not flagged", rule.Id))
	}
	return failures, len(files), nil
}

func fixtureFiles(ruleDir, set string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(ruleDir, set))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("fixtureFiles: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(ruleDir, set, entry.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}
