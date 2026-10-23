package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/shell"
)

func TestNewPendingRequest(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedRoster []repos.Repo

		registryPath string
	}

	repo := initFixtureRepo(t)
	registryPath := filepath.Join(t.TempDir(), "repos.json")
	registryJson := `{"repos":[{"name":"alpha","path":"` + repo + `"}]}`
	require.NoError(t, os.WriteFile(registryPath, []byte(registryJson), 0o644))
	recordsRoot := t.TempDir()

	tests := make([]*testCase, 0)

	// registry-roster-and-repo-root
	tests = append(tests, &testCase{
		_id:             "registry-roster-and-repo-root",
		_expectedRoster: []repos.Repo{{Name: "alpha", Path: repo}},

		registryPath: registryPath,
	})

	// missing-registry-empty-roster
	tests = append(tests, &testCase{
		_id:             "missing-registry-empty-roster",
		_expectedRoster: []repos.Repo{},

		registryPath: filepath.Join(t.TempDir(), "missing.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			repoRoot, err := mainCheckoutRoot(context.Background())
			require.NoError(t, err)
			request, err := sessions.NewPendingRequest(
				false,
				0,
				test.registryPath,
				repoRoot,
				recordsRoot,
			)
			require.NoError(t, err)
			assert.Equal(t, repo, request.RepoRoot)
			assert.Equal(t, test._expectedRoster, request.Roster)
		})
	}
}

func TestRunContextRecord(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedExit      int
		_expectedSessionId string

		args []string
	}

	transcriptPath := provideTranscript(t)
	missingPath := filepath.Join(t.TempDir(), "missing.jsonl")

	tests := make([]*testCase, 0)

	// pass-transcript
	tests = append(tests, &testCase{
		_id:                "pass-transcript",
		_expectedExit:      exitClean,
		_expectedSessionId: "sess-1",

		args: []string{transcriptPath, "docs"},
	})

	// fail-missing-file
	tests = append(tests, &testCase{
		_id:                "fail-missing-file",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args: []string{missingPath, "docs"},
	})

	// fail-zero-args
	tests = append(tests, &testCase{
		_id:                "fail-zero-args",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args: []string{},
	})

	// fail-three-args
	tests = append(tests, &testCase{
		_id:                "fail-three-args",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args: []string{transcriptPath, "docs", "extra"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exitCode, sessionId := runContextRecordCaptured(t, test.args)
			assert.Equal(t, test._expectedExit, exitCode)
			assert.Equal(t, test._expectedSessionId, sessionId)
		})
	}
}

func TestRunContextRecord_EmptyAgentItems(t *testing.T) {
	transcriptPath := filepath.Join(t.TempDir(), "empty.jsonl")
	transcriptLine := `{"type":"user","cwd":"/repo/x","gitBranch":"claude/x","sessionId":"sess-empty","message":{"role":"user","content":"hi"}}`
	require.NoError(t, os.WriteFile(transcriptPath, []byte(transcriptLine+"\n"), 0o644))

	exitCode, output := captureStdout(t, func() int { return runContextRecord([]string{transcriptPath, "docs"}) })

	assert.Equal(t, exitClean, exitCode)
	assert.Contains(t, output, `"items": []`)
	assert.NotContains(t, output, `"items": null`)
}

func TestRunCheck(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedExit      int
		_expectedSessionId string

		args       []string
		recordPath string
	}

	transcriptPath := provideTranscript(t)
	recordWithoutContext := provideRecordWithoutContext(t)
	missingTranscriptPath := filepath.Join(t.TempDir(), "missing.jsonl")
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	t.Chdir(repoRoot)

	tests := make([]*testCase, 0)

	// pass-merge-into-record-without-context
	recordPath := writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "pass-merge-into-record-without-context",
		_expectedExit:      exitClean,
		_expectedSessionId: "sess-1",

		args:       []string{recordPath, "--context", transcriptPath},
		recordPath: recordPath,
	})

	// pass-context-flag-before-path
	recordPath = writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "pass-context-flag-before-path",
		_expectedExit:      exitClean,
		_expectedSessionId: "sess-1",

		args:       []string{"--context", transcriptPath, recordPath},
		recordPath: recordPath,
	})

	// pass-without-context-flag
	recordPath = writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "pass-without-context-flag",
		_expectedExit:      exitClean,
		_expectedSessionId: "",

		args:       []string{recordPath},
		recordPath: recordPath,
	})

	// fail-context-with-two-records
	recordPath = writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "fail-context-with-two-records",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args:       []string{recordPath, writeRecord(t, recordWithoutContext), "--context", transcriptPath},
		recordPath: recordPath,
	})

	// fail-context-without-record
	recordPath = writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "fail-context-without-record",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args:       []string{"--context", transcriptPath},
		recordPath: recordPath,
	})

	// fail-missing-transcript
	recordPath = writeRecord(t, recordWithoutContext)
	tests = append(tests, &testCase{
		_id:                "fail-missing-transcript",
		_expectedExit:      exitError,
		_expectedSessionId: "",

		args:       []string{recordPath, "--context", missingTranscriptPath},
		recordPath: recordPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exitCode, sessionId := runCheckRecorded(t, test.args, test.recordPath)
			assert.Equal(t, test._expectedExit, exitCode)
			assert.Equal(t, test._expectedSessionId, sessionId)
		})
	}
}

// captureStdout runs the command with stdout redirected to a temp file and returns its exit code and output.
func captureStdout(t *testing.T, command func() int) (int, string) {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), "stdout")
	outputFile, err := os.Create(outputPath)
	require.NoError(t, err)

	originalStdout := os.Stdout
	os.Stdout = outputFile
	exitCode := command()
	os.Stdout = originalStdout
	require.NoError(t, outputFile.Close())

	output, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	return exitCode, string(output)
}

// initFixtureRepo creates an empty git repo in a temp dir, makes it the
// process cwd and returns its resolved path.
func initFixtureRepo(t *testing.T) string {
	t.Helper()

	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	_, err = shell.Run(context.Background(), repo, "git", "init", "--quiet")
	require.NoError(t, err)

	t.Chdir(repo)
	return repo
}

// provideTranscript copies the contextrecord fixture tree into a temp dir.
func provideTranscript(t *testing.T) string {
	t.Helper()

	sourceDir := filepath.Join("..", "..", "internal", "contextrecord", "testdata")
	targetDir := t.TempDir()
	err := os.CopyFS(targetDir, os.DirFS(sourceDir))
	require.NoError(t, err)

	return filepath.Join(targetDir, "root.jsonl")
}

// provideRecordWithoutContext returns the sessions fixture record with its context key removed.
func provideRecordWithoutContext(t *testing.T) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", "internal", "sessions", "testdata", "record.json"))
	require.NoError(t, err)

	contextBlock := "  \"context\": {\n    \"honored\": 1,\n    \"pack_reads\": [\n      \"context/actions/implementing.md\"\n    ]\n  },\n"
	require.Contains(t, string(content), contextBlock)
	return strings.Replace(string(content), contextBlock, "", 1)
}

// runCheckRecorded runs check and returns the exit code and the session id of the record's context key.
func runCheckRecorded(t *testing.T, args []string, recordPath string) (int, string) {
	t.Helper()

	exitCode, _ := captureStdout(t, func() int { return runCheck(args) })
	session, violations, err := sessions.ReadRecord(recordPath)
	require.NoError(t, err)
	require.Empty(t, violations)

	if session.Context == nil {
		return exitCode, ""
	}
	return exitCode, session.Context.SessionId
}

// runContextRecordCaptured runs the subcommand and returns the exit code and the printed record's session id.
func runContextRecordCaptured(t *testing.T, args []string) (int, string) {
	t.Helper()

	exitCode, output := captureStdout(t, func() int { return runContextRecord(args) })
	if output == "" {
		return exitCode, ""
	}

	record := &contextrecord.Record{}
	require.NoError(t, json.Unmarshal([]byte(output), record))
	return exitCode, record.SessionId
}

func writeRecord(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "c67a1a4c-ebdd-47fc-a84b-677e4eee8e41.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
