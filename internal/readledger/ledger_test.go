package readledger

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRetention = time.Hour
	testSessionId = "session-1"
)

func TestLedger_Load_SkipsUnknownKind(t *testing.T) {
	claudeHome := t.TempDir()
	ledger := New(MainAgent, claudeHome, testRetention, testSessionId)
	provideFile(t, ledger.Path(), []byte("{\"kind\":\"other\"}\n{\"ids\":[\"A-X-001\"],\"kind\":\"context\"}\n"))

	assert.Equal(t, []Record{provideContext("", 0, 0, "A-X-001")}, readRecords(t, ledger))
}

func TestLedger_Prune(t *testing.T) {
	claudeHome := t.TempDir()
	stale := filepath.Join(claudeHome, stateDirName, "old__main.jsonl")
	provideFile(t, stale, []byte("{}\n"))
	expired := time.Now().Add(-testRetention - time.Minute)
	err := os.Chtimes(stale, expired, expired)
	require.NoError(t, err)

	recent := filepath.Join(claudeHome, stateDirName, "recent__main.jsonl")
	provideFile(t, recent, []byte("{}\n"))
	kept := time.Now().Add(-testRetention + time.Minute)
	err = os.Chtimes(recent, kept, kept)
	require.NoError(t, err)

	provideLedger(t, claudeHome, provideRead("/repo/a.go", 1, 10))

	_, err = os.Stat(stale)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.FileExists(t, recent)
}

func TestLedger_Reset(t *testing.T) {
	type testCase struct {
		_id                    string
		_expectedIsFilePresent bool
		_expectedRecords       []Record

		ledger *Ledger
		source string
	}

	tests := make([]*testCase, 0)
	read := provideRead("/repo/docs/rules/go.md", 1, 10)
	invocation := provideSkill("/fdesign", "fdesign")
	inlineContext := provideContext("", 0, 0, "A-X-001")
	filedContext := provideContext("/home/context-required/session-1/fdesign--main.md", 1, 40, "B-X-002")

	// compact-drops-read-and-inline-context
	tests = append(tests, &testCase{
		_id:                    "compact-drops-read-and-inline-context",
		_expectedIsFilePresent: true,
		_expectedRecords:       []Record{invocation, filedContext},

		ledger: provideLedger(t, t.TempDir(), read, invocation, inlineContext, filedContext),
		source: SourceCompact,
	})

	// clear-same-as-compact
	tests = append(tests, &testCase{
		_id:                    "clear-same-as-compact",
		_expectedIsFilePresent: true,
		_expectedRecords:       []Record{invocation, filedContext},

		ledger: provideLedger(t, t.TempDir(), read, invocation, inlineContext, filedContext),
		source: SourceClear,
	})

	// startup-keeps-everything
	tests = append(tests, &testCase{
		_id:                    "startup-keeps-everything",
		_expectedIsFilePresent: true,
		_expectedRecords:       []Record{read, invocation, inlineContext, filedContext},

		ledger: provideLedger(t, t.TempDir(), read, invocation, inlineContext, filedContext),
		source: "startup",
	})

	// resume-keeps-everything
	tests = append(tests, &testCase{
		_id:                    "resume-keeps-everything",
		_expectedIsFilePresent: true,
		_expectedRecords:       []Record{read, invocation, inlineContext, filedContext},

		ledger: provideLedger(t, t.TempDir(), read, invocation, inlineContext, filedContext),
		source: "resume",
	})

	// compact-on-empty-ledger-writes-nothing
	tests = append(tests, &testCase{
		_id:                    "compact-on-empty-ledger-writes-nothing",
		_expectedIsFilePresent: false,
		_expectedRecords:       nil,

		ledger: provideLedger(t, t.TempDir()),
		source: SourceCompact,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.ledger.Reset(test.source)

			require.NoError(t, err)
			assert.Equal(t, test._expectedRecords, readRecords(t, test.ledger))
			assert.Equal(t, test._expectedIsFilePresent, isFilePresent(t, test.ledger.Path()))
		})
	}
}

func TestSeenIds(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedIds []string

		records []Record
	}

	tests := make([]*testCase, 0)

	// no-context-records
	tests = append(tests, &testCase{
		_id:          "no-context-records",
		_expectedIds: []string{},

		records: []Record{provideRead("/repo/a.go", 1, 10)},
	})

	// two-context-records-union-sorted-unique
	tests = append(tests, &testCase{
		_id:          "two-context-records-union-sorted-unique",
		_expectedIds: []string{"A-X-001", "B-X-002", "C-X-003"},

		records: []Record{
			provideContext("", 0, 0, "C-X-003", "A-X-001"),
			provideContext("/home/context-required/session-1/fdesign--main.md", 1, 40, "B-X-002", "A-X-001"),
		},
	})

	// read-and-skill-ignored
	tests = append(tests, &testCase{
		_id:          "read-and-skill-ignored",
		_expectedIds: []string{"A-X-001"},

		records: []Record{
			provideRead("/repo/a.go", 1, 10),
			provideSkill("/fdesign", "fdesign"),
			provideContext("", 0, 0, "A-X-001"),
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedIds, SeenIds(test.records))
		})
	}
}

// isFilePresent reports whether the path exists.
func isFilePresent(t *testing.T, path string) bool {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	require.NoError(t, err)
	return true
}

func provideContext(file string, start int, end int, ids ...string) Record {
	return Record{End: end, File: file, Ids: ids, Kind: KindContext, Start: start}
}

func provideFile(t *testing.T, path string, content []byte) {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, content, 0o644)
	require.NoError(t, err)
}

// provideLedger appends the records to the root ledger and returns it.
func provideLedger(t *testing.T, claudeHome string, records ...Record) *Ledger {
	ledger := New(MainAgent, claudeHome, testRetention, testSessionId)
	for _, record := range records {
		err := ledger.Append(&record)
		require.NoError(t, err)
	}
	return ledger
}

func provideRead(file string, start int, end int) Record {
	return Record{End: end, File: file, Kind: KindRead, Start: start}
}

func provideSkill(args string, skill string) Record {
	return Record{Args: args, Kind: KindSkill, Skill: skill}
}

// readRecords returns the ledger's records, nil when it has none.
func readRecords(t *testing.T, ledger *Ledger) []Record {
	records, err := ledger.Load()
	require.NoError(t, err)
	if len(records) == 0 {
		return nil
	}
	return records
}
