package proposals_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/proposals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustLine marshals a vote to its stored JSONL form.
func mustLine(t *testing.T, vote proposals.Vote) string {
	t.Helper()
	data, err := json.Marshal(vote)
	require.NoError(t, err)
	return string(data)
}

func ptr(s string) *string { return &s }

func TestLoadVotes(t *testing.T) {
	skillAccept := proposals.Vote{Id: "x1", Kind: "skills", Title: "X1", Ts: "2026-07-22T00:00:00Z", Vote: "+"}
	skillReject := proposals.Vote{Id: "x1", Kind: "skills", Title: "X1", Ts: "2026-07-22T01:00:00Z", Vote: "-", Comment: "no"}
	ruleAccept := proposals.Vote{Id: "G1", Kind: "style", Title: "io idiom", Ts: "2026-07-22T02:00:00Z", Vote: "+"}

	type testCase struct {
		_expectErr bool
		_expected  map[string]proposals.Vote
		_id        string
		content    *string
	}

	tests := make([]*testCase, 0)

	// missing-file-empty
	tests = append(tests, &testCase{
		_id:       "missing-file-empty",
		_expected: map[string]proposals.Vote{},
		content:   nil,
	})

	// last-line-wins
	tests = append(tests, &testCase{
		_id: "last-line-wins",
		_expected: map[string]proposals.Vote{
			"skills/x1": skillReject,
		},
		content: ptr(mustLine(t, skillAccept) + "\n" + mustLine(t, skillReject) + "\n"),
	})

	// malformed-line-error
	tests = append(tests, &testCase{
		_id:        "malformed-line-error",
		_expectErr: true,
		content:    ptr(mustLine(t, skillAccept) + "\n{not json\n"),
	})

	// blank-lines-skipped
	tests = append(tests, &testCase{
		_id: "blank-lines-skipped",
		_expected: map[string]proposals.Vote{
			"skills/x1": skillAccept,
			"style/G1":  ruleAccept,
		},
		content: ptr("\n" + mustLine(t, skillAccept) + "\n\n" + mustLine(t, ruleAccept) + "\n\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "votes.jsonl")
			if test.content != nil {
				require.NoError(t, os.WriteFile(path, []byte(*test.content), 0644))
			}

			got, err := proposals.LoadVotes(path)
			if test._expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test._expected, got)
		})
	}
}

func TestSetVote(t *testing.T) {
	skillAccept := proposals.Vote{Id: "x1", Kind: "skills", Title: "X1", Ts: "2026-07-22T00:00:00Z", Vote: "+"}
	skillReject := proposals.Vote{Id: "x1", Kind: "skills", Title: "X1", Ts: "2026-07-22T01:00:00Z", Vote: "-", Comment: "no"}
	ruleAccept := proposals.Vote{Id: "G1", Kind: "style", Title: "io idiom", Ts: "2026-07-22T02:00:00Z", Vote: "+"}
	other := proposals.Vote{Id: "y2", Kind: "skills", Title: "Y2", Ts: "2026-07-22T03:00:00Z", Vote: "+"}

	type testCase struct {
		_expectErr   bool
		_expectLines []string
		_id          string
		existing     *string
		vote         proposals.Vote
	}

	tests := make([]*testCase, 0)

	// creates-file
	tests = append(tests, &testCase{
		_id:          "creates-file",
		_expectLines: []string{mustLine(t, skillAccept)},
		existing:     nil,
		vote:         skillAccept,
	})

	// replaces-same-proposal
	tests = append(tests, &testCase{
		_id:          "replaces-same-proposal",
		_expectLines: []string{mustLine(t, skillReject)},
		existing:     ptr(mustLine(t, skillAccept) + "\n"),
		vote:         skillReject,
	})

	// keeps-other-lines-in-order
	tests = append(tests, &testCase{
		_id:          "keeps-other-lines-in-order",
		_expectLines: []string{mustLine(t, ruleAccept), mustLine(t, other), mustLine(t, skillReject)},
		existing:     ptr(mustLine(t, ruleAccept) + "\n" + mustLine(t, other) + "\n" + mustLine(t, skillAccept) + "\n"),
		vote:         skillReject,
	})

	// appends-new-vote-last
	tests = append(tests, &testCase{
		_id:          "appends-new-vote-last",
		_expectLines: []string{mustLine(t, ruleAccept), mustLine(t, skillAccept)},
		existing:     ptr(mustLine(t, ruleAccept) + "\n"),
		vote:         skillAccept,
	})

	// malformed-existing-line-error
	tests = append(tests, &testCase{
		_id:        "malformed-existing-line-error",
		_expectErr: true,
		existing:   ptr("{not json\n"),
		vote:       skillAccept,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "votes.jsonl")
			if test.existing != nil {
				require.NoError(t, os.WriteFile(path, []byte(*test.existing), 0644))
			}

			err := proposals.SetVote(path, &test.vote)
			if test._expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			var lines []string
			for _, line := range strings.Split(string(data), "\n") {
				if strings.TrimSpace(line) != "" {
					lines = append(lines, line)
				}
			}
			assert.Equal(t, test._expectLines, lines)
		})
	}
}
