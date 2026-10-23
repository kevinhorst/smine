package skilleval

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	emptyCell        = "—"
	percentScale     = 100
	secondsPerHour   = 3600
	secondsPerMinute = 60
	shortShaLength   = 8
	tokensPerK       = 1_000
	tokensPerM       = 1_000_000
)

func boldFailure(failed, total int) string {
	return fmt.Sprintf("**%d%%** (%d)", failurePercent(failed, total), failed)
}

func cellText(text string) string {
	return strings.ReplaceAll(snippet(text), "|", "\\|")
}

func failurePercent(failed, total int) int {
	return int(math.Round(percent(failed, total)))
}

func formatDuration(seconds float64) string {
	total := int(math.Round(seconds))
	hours := total / secondsPerHour
	minutes := total % secondsPerHour / secondsPerMinute
	rest := total % secondsPerMinute
	switch {
	case hours > 0:
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %02ds", minutes, rest)
	}
	return fmt.Sprintf("%ds", rest)
}

func formatMetric(metricId string, value float64) string {
	switch metricId {
	case metricWallSeconds, metricActiveSeconds:
		return formatDuration(value)
	case metricOutputTokens:
		return formatTokens(value)
	case metricCostUsd:
		return fmt.Sprintf("$%.2f", value)
	case metricReviewScore:
		return fmt.Sprintf("%.0f%%", value*percentScale)
	case metricLostMinutes, metricEscapedMinutes, metricFalsePositiveMinutes:
		return fmt.Sprintf("%.0f min", value)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
}

func formatTokens(value float64) string {
	switch {
	case value >= tokensPerM:
		return fmt.Sprintf("%.1fM", value/tokensPerM)
	case value >= tokensPerK:
		return fmt.Sprintf("%.0fk", value/tokensPerK)
	}
	return fmt.Sprintf("%.0f", value)
}

func metricCell(fold *armFold, manifest *RunManifest, metricId string) string {
	for _, value := range fold.metricValues(manifest) {
		if value.MetricId != metricId {
			continue
		}

		number, ok := value.Number()
		if !ok {
			return emptyCell
		}
		return formatMetric(metricId, number)
	}
	return emptyCell
}

func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * percentScale / float64(total)
}

func renderReport(folds []*armFold, manifest *RunManifest) string {
	var out strings.Builder
	writeHeader(&out, manifest)
	writeResultTable(&out, folds, manifest, scorerMetricIds(folds))
	writeStepMisses(&out, folds)
	writeUnobserved(&out, folds)
	writeCompareNotes(&out, folds)
	writeFailures(&out, folds)
	return out.String()
}

func resultRow(fold *armFold, isGolden bool, manifest *RunManifest, scoreIds []string) string {
	smoke := "pass"
	if len(fold.smoke) > 0 {
		smoke = "**fail**"
	}
	steps := emptyCell
	misses, applicable := fold.stepMisses()
	if applicable > 0 {
		steps = boldFailure(misses, applicable)
	}
	cells := []string{
		fold.arm.Id,
		smoke,
		metricCell(fold, manifest, metricCheckPass),
		metricCell(fold, manifest, metricWallSeconds),
		metricCell(fold, manifest, metricActiveSeconds),
		metricCell(fold, manifest, metricOutputTokens),
		metricCell(fold, manifest, metricCostUsd),
		metricCell(fold, manifest, metricPermissionDenials),
		steps,
		metricCell(fold, manifest, metricDefectsMajor),
		metricCell(fold, manifest, metricQualityRank),
	}
	if isGolden {
		cells = append(cells, metricCell(fold, manifest, metricReviewScore), metricCell(fold, manifest, metricLostMinutes))
	}
	for _, scoreId := range scoreIds {
		cells = append(cells, scoreCell(fold, scoreId))
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

// scoreCell is one run's score cell; empty when its scorer reported none.
func scoreCell(fold *armFold, scoreId string) string {
	score, hasScore := fold.result.Scores[strings.TrimPrefix(scoreId, scorePrefix)]
	if !hasScore {
		return emptyCell
	}
	return formatMetric(scoreId, score)
}

func shortSha(sha string) string {
	if len(sha) <= shortShaLength {
		return sha
	}
	return sha[:shortShaLength]
}

func writeCompareNotes(out *strings.Builder, folds []*armFold) {
	var ranked []*armFold
	for _, fold := range folds {
		if fold.compare != nil {
			ranked = append(ranked, fold)
		}
	}
	if len(ranked) == 0 {
		return
	}

	sort.Slice(ranked, func(i, j int) bool { return ranked[i].rank < ranked[j].rank })
	out.WriteString("## Compare judge\n\n")
	for _, fold := range ranked {
		fmt.Fprintf(out, "- `%s` (rank %d): %s\n", fold.arm.Id, fold.rank, cellText(fold.compare.Summary))
		for _, defect := range fold.compare.Defects {
			fmt.Fprintf(out, "  - %s `%s`: %s\n", defect.Severity, defect.File, cellText(defect.Claim))
		}
	}
	out.WriteString("\n")
}

func writeFailures(out *strings.Builder, folds []*armFold) {
	out.WriteString("## Smoke and variant failures\n\n")
	isClean := true
	for _, fold := range folds {
		reasons := make([]string, 0, len(fold.smoke)+len(fold.result.InvalidReasons))
		reasons = append(reasons, fold.smoke...)
		reasons = append(reasons, fold.result.InvalidReasons...)
		if len(reasons) == 0 {
			continue
		}

		isClean = false
		fmt.Fprintf(out, "- `%s`: %s\n", fold.arm.Id, strings.Join(reasons, "; "))
	}
	if isClean {
		out.WriteString("None.\n")
	}
}

func writeHeader(out *strings.Builder, manifest *RunManifest) {
	fmt.Fprintf(out, "# %s — eval %s\n\n", manifest.Skill, manifest.Date)
	fmt.Fprintf(out, "- prompt: %s\n", cellText(manifest.Prompt))
	fmt.Fprintf(out, "- start commit: `%s` in `%s`\n", shortSha(manifest.At), manifest.Repo)
	fmt.Fprintf(out, "- auth: %s\n", manifest.authLabel())
	fmt.Fprintf(out, "- judge: `%s`\n\n", manifest.Judge)
}

func writeResultTable(out *strings.Builder, folds []*armFold, manifest *RunManifest, scoreIds []string) {
	isGolden := false
	for _, fold := range folds {
		if fold.golden != nil {
			isGolden = true
		}
	}

	var header, divider strings.Builder
	header.WriteString("| Run | Smoke | Check pass | Wall | Main-loop active | Output tokens (all agents) | Cost | Permission denials (all agents) | Step misses | Major+ defects | Rank |")
	divider.WriteString("|---|---|---|---|---|---|---|---|---|---|---|")
	if isGolden {
		header.WriteString(" Review score | Lost minutes |")
		divider.WriteString("---|---|")
	}
	for _, scoreId := range scoreIds {
		header.WriteString(" " + strings.TrimPrefix(scoreId, scorePrefix) + " |")
		divider.WriteString("---|")
	}
	out.WriteString(header.String() + "\n" + divider.String() + "\n")
	for _, fold := range folds {
		out.WriteString(resultRow(fold, isGolden, manifest, scoreIds) + "\n")
	}
	out.WriteString("\n")
}

func writeStepMisses(out *strings.Builder, folds []*armFold) {
	out.WriteString("## Step misses\n\n")
	isClean := true
	for _, fold := range folds {
		for _, scored := range fold.adherence {
			isMiss := scored.isApplicable && scored.score < 1
			if !isMiss {
				continue
			}

			isClean = false
			entry := scored.entry
			fmt.Fprintf(
				out,
				"- `%s` %s — %s (%s): %s — %s\n",
				fold.arm.Id,
				entry.Id,
				entry.Title,
				entry.Verdict,
				cellText(entry.Evidence),
				cellText(entry.Note),
			)
		}
	}
	if isClean {
		out.WriteString("None.\n")
	}
	out.WriteString("\n")
}

func writeUnobserved(out *strings.Builder, folds []*armFold) {
	out.WriteString("## Unobservable entries\n\n")
	isClean := true
	for _, fold := range folds {
		for _, scored := range fold.adherence {
			if scored.entry.Verdict != verdictNotObservable {
				continue
			}

			isClean = false
			entry := scored.entry
			fmt.Fprintf(out, "- `%s` %s — %s: %s\n", fold.arm.Id, entry.Id, entry.Title, cellText(entry.Note))
		}
	}
	if isClean {
		out.WriteString("None.\n")
	}
	out.WriteString("\n")
}
