package secretscan

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportResult() *Result {
	return &Result{
		BaselinedFindings: []Finding{},
		HistoryFindings:   []HistoryFinding{},
		NewFindings: []Finding{{
			value:      "raw-secret-value",
			Confidence: ConfidenceHigh,
			Detector:   "aws-access-key-id",
			Excerpt:    "shown-excerpt",
			Line:       3,
			Path:       "a.py",
		}},
		OversizeFiles: []string{},
		RepoPath:      "/tmp/repo",
	}
}

func TestRenderText(t *testing.T) {
	t.Run("findings-lines-formatted", func(t *testing.T) {
		text := RenderText(reportResult())
		assert.Contains(t, text, "a.py:3  [high]  aws-access-key-id  shown-excerpt\n")
	})

	t.Run("summary-line", func(t *testing.T) {
		text := RenderText(reportResult())
		assert.Contains(t, text, "1 new, 0 baselined, 0 history findings; skipped: 0 binary, 0 oversize\n")
	})

	t.Run("history-section-only-when-present", func(t *testing.T) {
		result := reportResult()
		assert.NotContains(t, RenderText(result), "history:")

		result.HistoryFindings = []HistoryFinding{{
			BlobOid:    "abcdef0123456789abcdef0123456789abcdef01",
			Commits:    []string{"3e49439", "b5b8a7b"},
			Confidence: ConfidenceHigh,
			Detector:   "aws-access-key-id",
			Excerpt:    "shown-excerpt",
			Line:       1,
			Path:       "old.py",
		}}
		text := RenderText(result)
		assert.Contains(t, text, "history:")
		assert.Contains(t, text, "blob=abcdef012345")
		assert.Contains(t, text, "commits=3e49439,b5b8a7b")
	})
}

func TestRenderJson(t *testing.T) {
	data, err := RenderJson(reportResult())
	require.NoError(t, err)

	t.Run("expected-keys-present", func(t *testing.T) {
		decoded := map[string]any{}
		require.NoError(t, json.Unmarshal(data, &decoded))
		for _, key := range []string{"newFindings", "baselinedFindings", "historyFindings", "oversizeFiles", "binarySkipCount", "repoPath"} {
			assert.Contains(t, decoded, key)
		}
	})

	t.Run("raw-value-not-serialized", func(t *testing.T) {
		assert.NotContains(t, string(data), "raw-secret-value")
	})

	t.Run("trailing-newline", func(t *testing.T) {
		assert.Equal(t, byte('\n'), data[len(data)-1])
	})
}
