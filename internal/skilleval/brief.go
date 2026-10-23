package skilleval

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	BriefAdherence = "adherence"
	BriefCompare   = "compare"
	BriefGolden    = "golden"
)

const referenceLinePrefix = "\nReference change, known good: "

//go:embed briefs/adherence.txt
var adherenceBrief string

//go:embed briefs/compare.txt
var compareBrief string

//go:embed briefs/golden.txt
var goldenBrief string

func adherenceText(label string, manifest *RunManifest, runDir string) (string, error) {
	arm, err := manifest.armByLabel(label)
	if err != nil {
		return "", err
	}

	replacer := strings.NewReplacer(
		"<skill>", manifest.Skill,
		"<task>", manifest.Prompt,
		"<run dir>", runDir,
		"<label>", arm.Label,
		"<worktree>", arm.Worktree,
	)
	return replacer.Replace(adherenceBrief), nil
}

// compareText adds the reference line only when provision wrote reference.patch.
func compareText(manifest *RunManifest, runDir string) (string, error) {
	arms := slices.Clone(manifest.Arms)
	slices.SortFunc(arms, func(a, b ManifestArm) int { return strings.Compare(a.Label, b.Label) })
	lines := make([]string, 0, len(arms))
	for _, arm := range arms {
		diff := filepath.Join(runDir, runsDir, arm.Label, diffFile)
		lines = append(lines, fmt.Sprintf("- %s: diff %s, worktree %s", arm.Label, diff, arm.Worktree))
	}

	reference := ""
	referencePath := filepath.Join(runDir, referencePatchFile)
	_, err := os.Stat(referencePath)
	switch {
	case err == nil:
		reference = referenceLinePrefix + referencePath
	case !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("compareText: %w", err)
	}

	replacer := strings.NewReplacer(
		"<task>", manifest.Prompt,
		"<runs>", strings.Join(lines, "\n"),
		"<reference>", reference,
		"<run dir>", runDir,
	)
	return replacer.Replace(compareBrief), nil
}

func excludePathspec(excludePaths []string) string {
	if len(excludePaths) == 0 {
		return ""
	}

	var pathspec strings.Builder
	pathspec.WriteString(" -- .")
	for _, path := range excludePaths {
		pathspec.WriteString(" ':(exclude)" + path + "'")
	}
	return pathspec.String()
}

func goldenText(label string, manifest *RunManifest, runDir string) (string, error) {
	arm, err := manifest.armByLabel(label)
	if err != nil {
		return "", err
	}

	if manifest.Golden.kind() != GoldenTest {
		return "", fmt.Errorf("goldenText: %s is no golden test run — it has no golden judge", runDir)
	}

	judged := &manifest.Golden.Range
	replacer := strings.NewReplacer(
		"<preset skill dir>/<goldens[0].file>", filepath.Join(runDir, goldenSnapshotFile),
		"<preset skill dir>/<scoring>", filepath.Join(runDir, scoringSnapshotFile),
		"<range.baseSha>", judged.BaseSha,
		"<range.headSha>", judged.HeadSha,
		"<range.pathspec>", excludePathspec(judged.ExcludePaths),
		"<run dir>", runDir,
		"<label>", arm.Label,
		"<worktree>", arm.Worktree,
	)
	return replacer.Replace(goldenBrief), nil
}

func Brief(kind, label, runDir string) (string, error) {
	absRunDir, err := filepath.Abs(runDir)
	if err != nil {
		return "", fmt.Errorf("Brief: %w", err)
	}

	manifest, err := loadManifest(runDir)
	if err != nil {
		return "", err
	}

	switch kind {
	case BriefAdherence:
		return adherenceText(label, manifest, absRunDir)
	case BriefCompare:
		return compareText(manifest, absRunDir)
	case BriefGolden:
		return goldenText(label, manifest, absRunDir)
	}
	return "", fmt.Errorf("Brief: Unknown brief %q", kind)
}
