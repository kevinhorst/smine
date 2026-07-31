package secretscan

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBaseline(t *testing.T) {
	t.Run("missing-file-empty", func(t *testing.T) {
		baseline, err := loadBaseline(t.TempDir())
		require.NoError(t, err)
		assert.Empty(t, baseline.Entries)
		assert.Empty(t, baseline.Salt)
	})

	t.Run("malformed-json-errors", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, baselineFileName, "{not json")
		_, err := loadBaseline(root)
		assert.ErrorContains(t, err, "Malformed")
	})

	t.Run("malformed-salt-errors", func(t *testing.T) {
		baseline := &baselineFile{Salt: "zz"}
		_, err := baseline.saltBytes()
		assert.ErrorContains(t, err, "Malformed salt")
	})
}

func TestSplitByBaseline(t *testing.T) {
	salt := []byte{0x0a, 0x0b}
	finding := func(path, value string) Finding {
		return Finding{value: value, Detector: "aws-access-key-id", Path: path}
	}
	baselineFor := func(entries ...baselineEntry) *baselineFile {
		return &baselineFile{Entries: entries, Salt: hex.EncodeToString(salt)}
	}
	splitResult := func(t *testing.T, baseline *baselineFile, findings ...Finding) *Result {
		t.Helper()
		result := &Result{
			BaselinedFindings: make([]Finding, 0),
			NewFindings:       make([]Finding, 0),
			baseline:          baseline,
			treeFindings:      findings,
		}
		require.NoError(t, splitByBaseline(result))
		return result
	}

	t.Run("new-vs-baselined-split", func(t *testing.T) {
		baseline := baselineFor(baselineEntry{
			Detector:  "aws-access-key-id",
			Path:      "a.py",
			ValueHash: valueHash(salt, fakeAwsKey),
		})
		result := splitResult(t, baseline, finding("a.py", fakeAwsKey), finding("b.py", fakeAwsKey))

		require.Len(t, result.BaselinedFindings, 1)
		assert.Equal(t, "a.py", result.BaselinedFindings[0].Path)
		require.Len(t, result.NewFindings, 1)
		assert.Equal(t, "b.py", result.NewFindings[0].Path)
	})

	t.Run("renamed-path-resurfaces", func(t *testing.T) {
		baseline := baselineFor(baselineEntry{
			Detector:  "aws-access-key-id",
			Path:      "old.py",
			ValueHash: valueHash(salt, fakeAwsKey),
		})
		result := splitResult(t, baseline, finding("renamed.py", fakeAwsKey))

		assert.Empty(t, result.BaselinedFindings)
		assert.Len(t, result.NewFindings, 1)
	})

	t.Run("changed-value-resurfaces", func(t *testing.T) {
		baseline := baselineFor(baselineEntry{
			Detector:  "aws-access-key-id",
			Path:      "a.py",
			ValueHash: valueHash(salt, "some-other-value"),
		})
		result := splitResult(t, baseline, finding("a.py", fakeAwsKey))

		assert.Empty(t, result.BaselinedFindings)
		assert.Len(t, result.NewFindings, 1)
	})
}

func TestWriteBaseline(t *testing.T) {
	finding := func(path, value string) Finding {
		return Finding{value: value, Detector: "aws-access-key-id", Path: path}
	}
	resultFor := func(baseline *baselineFile, findings ...Finding) *Result {
		return &Result{
			BaselinedFindings: make([]Finding, 0),
			NewFindings:       append([]Finding{}, findings...),
			baseline:          baseline,
			treeFindings:      findings,
		}
	}

	t.Run("creates-salt-once", func(t *testing.T) {
		repo := t.TempDir()
		result := resultFor(&baselineFile{}, finding("a.py", fakeAwsKey))
		require.NoError(t, WriteBaseline(repo, result))

		written, err := loadBaseline(repo)
		require.NoError(t, err)
		assert.Len(t, written.Salt, saltLength*2)
		assert.Len(t, written.Entries, 1)
	})

	t.Run("salt-stable-across-writes", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, WriteBaseline(repo, resultFor(&baselineFile{}, finding("a.py", fakeAwsKey))))
		first, err := loadBaseline(repo)
		require.NoError(t, err)

		require.NoError(t, WriteBaseline(repo, resultFor(first, finding("a.py", fakeAwsKey))))
		second, err := loadBaseline(repo)
		require.NoError(t, err)

		assert.Equal(t, first.Salt, second.Salt)
		assert.Equal(t, first.Entries, second.Entries)
	})

	t.Run("prunes-stale-entries", func(t *testing.T) {
		repo := t.TempDir()
		stale := &baselineFile{
			Entries: []baselineEntry{{Detector: "deny-file", Path: "gone.pem", ValueHash: "0000"}},
			Salt:    "0a0b",
		}
		require.NoError(t, WriteBaseline(repo, resultFor(stale, finding("a.py", fakeAwsKey))))

		written, err := loadBaseline(repo)
		require.NoError(t, err)
		require.Len(t, written.Entries, 1)
		assert.Equal(t, "a.py", written.Entries[0].Path)
	})

	t.Run("entries-sorted-and-deduped", func(t *testing.T) {
		repo := t.TempDir()
		result := resultFor(&baselineFile{},
			finding("b.py", fakeAwsKey), finding("a.py", fakeAwsKey), finding("a.py", fakeAwsKey))
		require.NoError(t, WriteBaseline(repo, result))

		written, err := loadBaseline(repo)
		require.NoError(t, err)
		require.Len(t, written.Entries, 2)
		assert.Equal(t, "a.py", written.Entries[0].Path)
		assert.Equal(t, "b.py", written.Entries[1].Path)
	})

	t.Run("moves-new-findings-to-baselined", func(t *testing.T) {
		repo := t.TempDir()
		result := resultFor(&baselineFile{}, finding("a.py", fakeAwsKey))
		require.NoError(t, WriteBaseline(repo, result))

		assert.Empty(t, result.NewFindings)
		assert.Len(t, result.BaselinedFindings, 1)
	})

	t.Run("trailing-newline", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, WriteBaseline(repo, resultFor(&baselineFile{}, finding("a.py", fakeAwsKey))))

		data, err := os.ReadFile(filepath.Join(repo, baselineFileName))
		require.NoError(t, err)
		assert.Equal(t, byte('\n'), data[len(data)-1])
	})
}
