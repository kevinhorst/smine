package repos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeUnplaced(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Unplaced
		_shouldPass bool

		output string
	}

	commit := UnplacedCommit{CommittedAt: "2026-10-04T03:00:00Z", IsMerge: false, Sha: "0123456789abcdef0123456789abcdef01234567", ShortSha: "01234567", Subject: "context: Added x"}
	expected := &Unplaced{
		Branch:       "claude-routines/smine-nightly-2026-10-04",
		Commits:      []UnplacedCommit{commit},
		Onto:         "main",
		OntoSha:      "89abcdef0123456789abcdef0123456789abcdef",
		OntoShortSha: "89abcdef",
		Schema:       1,
	}
	tests := make([]*testCase, 0)

	// pass-schema-1
	tests = append(tests, &testCase{
		_id:         "pass-schema-1",
		_expected:   expected,
		_shouldPass: true,

		output: `{"schema":1,"branch":"claude-routines/smine-nightly-2026-10-04","onto":"main","onto_sha":"89abcdef0123456789abcdef0123456789abcdef","onto_short_sha":"89abcdef","commits":[{"sha":"0123456789abcdef0123456789abcdef01234567","short_sha":"01234567","subject":"context: Added x","committed_at":"2026-10-04T03:00:00Z","is_merge":false}]}`,
	})

	// fail-unknown-schema
	tests = append(tests, &testCase{
		_id:         "fail-unknown-schema",
		_expected:   nil,
		_shouldPass: false,

		output: `{"schema":2,"branch":"b","onto":"main","onto_sha":"x","onto_short_sha":"x","commits":[]}`,
	})

	// fail-not-json
	tests = append(tests, &testCase{
		_id:         "fail-not-json",
		_expected:   nil,
		_shouldPass: false,

		output: "agentwt: unknown branch",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := decodeUnplaced(test.output)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, document)
		})
	}
}

func TestListUnplacedExecShape(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 0, `printf '{"schema":1,"branch":"%s","onto":"main","onto_sha":"x","onto_short_sha":"x","commits":[]}' "$*"`)

	document, err := ListUnplaced(context.Background(), stub, "claude-routines/nightly-2026-08-12", "main", repo)

	require.NoError(t, err)
	assert.Equal(t, "--format json -C "+repo+" unplaced --onto main claude-routines/nightly-2026-08-12", document.Branch)
}

func TestListUnplacedRejectsUnknownBranchBeforeExec(t *testing.T) {
	repo, _ := initGitRepo(t)
	stub := writeOpStub(t, 0, `echo "must not run" >&2`)

	_, err := ListUnplaced(context.Background(), stub, "claude-routines/missing", "main", repo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown branch")
}
