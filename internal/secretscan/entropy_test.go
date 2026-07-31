package secretscan

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShannonEntropy(t *testing.T) {
	assert.InDelta(t, 0.0, shannonEntropy("aaaa"), 0.0001)
	assert.InDelta(t, 2.0, shannonEntropy("abcd"), 0.0001)
}

// TestEntropyCorpus is the hand-built calibration corpus: thresholds and
// minimum lengths in entropy.go change only together with these fixtures.
func TestEntropyCorpus(t *testing.T) {
	type testCase struct {
		_detector   string
		_id         string
		_shouldFind bool
		content     string
	}

	tests := make([]*testCase, 0)

	// random-base64-secret
	tests = append(tests, &testCase{
		_detector:   detectorEntropyBase64,
		_id:         "random-base64-secret",
		_shouldFind: true,
		content:     "blob = QwErTyUiOpAsDfGhJkLzXcVbNm123456\n",
	})

	// random-hex-key
	tests = append(tests, &testCase{
		_detector:   detectorEntropyHex,
		_id:         "random-hex-key",
		_shouldFind: true,
		content:     "value = f3a9c1e7b2d84065a1c9e3f7b5d20486\n",
	})

	// english-prose
	tests = append(tests, &testCase{
		_id:         "english-prose",
		_shouldFind: false,
		content:     "the quick brown fox jumps over the lazy dog repeatedly\n",
	})

	// long-camelcase-identifier
	tests = append(tests, &testCase{
		_id:         "long-camelcase-identifier",
		_shouldFind: false,
		content:     "ProvideWebEventsServiceConfiguration\n",
	})

	// uuid-with-dashes
	tests = append(tests, &testCase{
		_id:         "uuid-with-dashes",
		_shouldFind: false,
		content:     "id = 550e8400-e29b-41d4-a716-446655440000\n",
	})

	// sha256-with-keyword-context
	tests = append(tests, &testCase{
		_id:         "sha256-with-keyword-context",
		_shouldFind: false,
		content:     "sha256:2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae\n",
	})

	// repeated-char-padding
	tests = append(tests, &testCase{
		_id:         "repeated-char-padding",
		_shouldFind: false,
		content:     "pad = AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n",
	})

	// short-token
	tests = append(tests, &testCase{
		_id:         "short-token",
		_shouldFind: false,
		content:     "value = f3a9c1e7b2d8406\n",
	})

	// data-uri-image-suppressed
	tests = append(tests, &testCase{
		_id:         "data-uri-image-suppressed",
		_shouldFind: false,
		content:     "src=\"data:image/png;base64,QwErTyUiOpAsDfGhJkLzXcVbNm123456\"\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			findings := entropyFindings("corpus.txt", []byte(test.content), nil)
			if !test._shouldFind {
				assert.Empty(t, findings)
				return
			}

			assert.Len(t, findings, 1)
			assert.Equal(t, test._detector, findings[0].Detector)
		})
	}
}

func TestHasLineContext(t *testing.T) {
	t.Run("keyword-on-line", func(t *testing.T) {
		content := []byte("checksum: f3a9c1e7b2d84065a1c9e3f7b5d20486\n")
		assert.True(t, hasLineContext(content, 10, hashContextKeywords))
	})

	t.Run("keyword-absent", func(t *testing.T) {
		content := []byte("value = f3a9c1e7b2d84065a1c9e3f7b5d20486\n")
		assert.False(t, hasLineContext(content, 8, hashContextKeywords))
	})

	t.Run("token-at-eof", func(t *testing.T) {
		content := []byte("digest f3a9c1e7b2d84065a1c9e3f7b5d20486")
		assert.True(t, hasLineContext(content, 7, hashContextKeywords))
	})

	t.Run("keyword-outside-window-ignored", func(t *testing.T) {
		content := []byte("digest " + strings.Repeat("x", lineContextWindow+10) + "f3a9c1e7b2d84065a1c9e3f7b5d20486")
		assert.False(t, hasLineContext(content, lineContextWindow+17, hashContextKeywords))
	})
}
