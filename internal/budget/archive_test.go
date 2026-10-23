package budget

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchive_Mirror(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedCopied  int
		_expectedFiles   []string
		_shouldPass      bool

		archive *Archive
	}

	transcript := filepath.Join("projects", "-src-repo", "session-1.jsonl")
	tests := make([]*testCase, 0)

	// new-transcript-copied
	newHome := t.TempDir()
	writeTestFile(t, filepath.Join(newHome, transcript), "abc")
	tests = append(tests, &testCase{
		_id:              "new-transcript-copied",
		_expectedContent: "abc",
		_expectedCopied:  1,
		_expectedFiles:   []string{transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: newHome, Dir: t.TempDir()},
	})

	// larger-source-copied
	grownHome := t.TempDir()
	grownArchive := t.TempDir()
	writeTestFile(t, filepath.Join(grownHome, transcript), "abcdef")
	writeTestFile(t, filepath.Join(grownArchive, transcript), "abc")
	tests = append(tests, &testCase{
		_id:              "larger-source-copied",
		_expectedContent: "abcdef",
		_expectedCopied:  1,
		_expectedFiles:   []string{transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: grownHome, Dir: grownArchive},
	})

	// equal-size-kept
	unchangedHome := t.TempDir()
	unchangedArchive := t.TempDir()
	writeTestFile(t, filepath.Join(unchangedHome, transcript), "abc")
	writeTestFile(t, filepath.Join(unchangedArchive, transcript), "xyz")
	tests = append(tests, &testCase{
		_id:              "equal-size-kept",
		_expectedContent: "xyz",
		_expectedCopied:  0,
		_expectedFiles:   []string{transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: unchangedHome, Dir: unchangedArchive},
	})

	// smaller-source-kept
	shrunkHome := t.TempDir()
	shrunkArchive := t.TempDir()
	writeTestFile(t, filepath.Join(shrunkHome, transcript), "ab")
	writeTestFile(t, filepath.Join(shrunkArchive, transcript), "abcdef")
	tests = append(tests, &testCase{
		_id:              "smaller-source-kept",
		_expectedContent: "abcdef",
		_expectedCopied:  0,
		_expectedFiles:   []string{transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: shrunkHome, Dir: shrunkArchive},
	})

	// subagent-and-workflow-files-copied
	subagentHome := t.TempDir()
	subagentFile := filepath.Join("projects", "-src-repo", "session-1", "subagents", "agent-a.jsonl")
	workflowFile := filepath.Join("projects", "-src-repo", "session-1", "subagents", "workflows", "wf_abc", "agent-b.jsonl")
	writeTestFile(t, filepath.Join(subagentHome, transcript), "abc")
	writeTestFile(t, filepath.Join(subagentHome, subagentFile), "sub")
	writeTestFile(t, filepath.Join(subagentHome, workflowFile), "wf")
	tests = append(tests, &testCase{
		_id:              "subagent-and-workflow-files-copied",
		_expectedContent: "abc",
		_expectedCopied:  3,
		_expectedFiles:   []string{subagentFile, workflowFile, transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: subagentHome, Dir: t.TempDir()},
	})

	// meta-json-copied
	metaHome := t.TempDir()
	metaFile := filepath.Join("projects", "-src-repo", "session-1", "subagents", "agent-a.meta.json")
	writeTestFile(t, filepath.Join(metaHome, transcript), "abc")
	writeTestFile(t, filepath.Join(metaHome, metaFile), `{"agentType":"Explore"}`)
	tests = append(tests, &testCase{
		_id:              "meta-json-copied",
		_expectedContent: "abc",
		_expectedCopied:  2,
		_expectedFiles:   []string{metaFile, transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: metaHome, Dir: t.TempDir()},
	})

	// tool-results-not-copied
	toolResultsHome := t.TempDir()
	writeTestFile(t, filepath.Join(toolResultsHome, transcript), "abc")
	writeTestFile(t, filepath.Join(toolResultsHome, "projects", "-src-repo", "session-1", "tool-results", "toolu_1.txt"), "result")
	writeTestFile(t, filepath.Join(toolResultsHome, "projects", "-src-repo", "session-1", "subagents", "notes.txt"), "notes")
	tests = append(tests, &testCase{
		_id:              "tool-results-not-copied",
		_expectedContent: "abc",
		_expectedCopied:  1,
		_expectedFiles:   []string{transcript},
		_shouldPass:      true,

		archive: &Archive{ClaudeHome: toolResultsHome, Dir: t.TempDir()},
	})

	// no-projects-dir-error
	tests = append(tests, &testCase{
		_id:              "no-projects-dir-error",
		_expectedContent: "",
		_expectedCopied:  0,
		_expectedFiles:   nil,
		_shouldPass:      false,

		archive: &Archive{ClaudeHome: t.TempDir(), Dir: t.TempDir()},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			copied, err := test.archive.Mirror()
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedCopied, copied)
			assert.Equal(t, test._expectedFiles, archivedFiles(t, test.archive.Dir))
			assert.Equal(t, test._expectedContent, readTestFile(filepath.Join(test.archive.Dir, transcript)))
		})
	}
}

func TestTranscriptPath(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		id string
	}

	claudeHome := t.TempDir()
	found := filepath.Join(claudeHome, "projects", "-src-first-cwd", "session-1.jsonl")
	writeTestFile(t, found, "abc")
	writeTestFile(t, filepath.Join(claudeHome, "projects", "x.jsonl"), "abc")
	tests := make([]*testCase, 0)

	// found
	tests = append(tests, &testCase{
		_id:       "found",
		_expected: found,

		id: "session-1",
	})

	// absent
	tests = append(tests, &testCase{
		_id:       "absent",
		_expected: "",

		id: "session-2",
	})

	// invalid-id
	tests = append(tests, &testCase{
		_id:       "invalid-id",
		_expected: "",

		id: "../x",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, TranscriptPath(claudeHome, test.id))
		})
	}
}

// archivedFiles lists every file under root as a relative path, in walk order.
func archivedFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relative, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)
		files = append(files, relative)
		return nil
	})
	require.NoError(t, err)
	return files
}

// readTestFile returns "" for a missing file.
func readTestFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
