package secretscan

import (
	"encoding/json"
	"fmt"
	"strings"
)

func RenderJson(result *Result) ([]byte, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("RenderJson: Failed to marshal result: %w", err)
	}

	return append(data, '\n'), nil
}

func RenderText(result *Result) string {
	builder := &strings.Builder{}
	for _, finding := range result.NewFindings {
		fmt.Fprintf(builder, "%s:%d  [%s]  %s  %s\n",
			finding.Path, finding.Line, finding.Confidence, finding.Detector, finding.Excerpt)
	}

	if len(result.HistoryFindings) > 0 {
		fmt.Fprintf(builder, "\nhistory:\n")
	}
	for _, finding := range result.HistoryFindings {
		fmt.Fprintf(builder, "%s:%d  [%s]  %s  %s  blob=%.12s  commits=%s\n",
			finding.Path, finding.Line, finding.Confidence, finding.Detector,
			finding.Excerpt, finding.BlobOid, strings.Join(finding.Commits, ","))
	}

	fmt.Fprintf(builder, "\n%d new, %d baselined, %d history findings; skipped: %d binary, %d oversize\n",
		len(result.NewFindings), len(result.BaselinedFindings), len(result.HistoryFindings),
		result.BinarySkipCount, len(result.OversizeFiles))
	return builder.String()
}
