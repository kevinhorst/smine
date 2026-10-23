package skilleval

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goldenMaster = `{
  "goldenVersion": 2,
  "provenance": {"method": "hand-maintained"},
  "findings": [
    {"golden_id": "G-1", "file": "a.go", "line": 3, "severity": "MAJOR", "claim": "nil map write", "contextOnly": false, "contextReason": "visible in the code"},
    {"golden_id": "G-2", "file": "b.go", "line": 7, "severity": "NIT", "claim": "prefix convention", "contextOnly": true, "contextReason": "needs the context pack"}
  ]
}`

const goldenFull = `{
  "findings": [
    {
      "claim": "nil map write",
      "file": "a.go",
      "golden_id": "G-1",
      "line": 3,
      "severity": "MAJOR"
    },
    {
      "claim": "prefix convention",
      "file": "b.go",
      "golden_id": "G-2",
      "line": 7,
      "severity": "NIT"
    }
  ],
  "goldenVersion": 2,
  "provenance": {
    "method": "hand-maintained",
    "role": "GENERATED from x.master.json by go run ./cmd/skilleval goldens gen (scope full) — do not edit"
  },
  "scope": "full"
}
`

const goldenContextless = `{
  "findings": [
    {
      "claim": "nil map write",
      "file": "a.go",
      "golden_id": "G-1",
      "line": 3,
      "severity": "MAJOR"
    }
  ],
  "goldenVersion": 2,
  "provenance": {
    "method": "hand-maintained",
    "role": "GENERATED from x.master.json by go run ./cmd/skilleval goldens gen (scope contextless) — do not edit"
  },
  "scope": "contextless"
}
`

const goldenKnown = `[
  {
    "claim": "nil map write",
    "file": "a.go",
    "finding_id": "G-1",
    "fix": "",
    "line": 3,
    "round": 0,
    "severity": "MAJOR"
  },
  {
    "claim": "prefix convention",
    "file": "b.go",
    "finding_id": "G-2",
    "fix": "",
    "line": 7,
    "round": 0,
    "severity": "NIT"
  }
]
`

func TestDerivedGoldens(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    string
		_file        string
		_isBadMaster bool
		_shouldPass  bool

		masterPath string
	}

	masterDir := t.TempDir()
	validPath := filepath.Join(masterDir, "x.master.json")
	writeTestFile(t, validPath, goldenMaster)
	tests := make([]*testCase, 0)

	// full-strips-context-and-sets-scope
	tests = append(tests, &testCase{
		_id:          "full-strips-context-and-sets-scope",
		_expected:    goldenFull,
		_file:        "x.json",
		_isBadMaster: false,
		_shouldPass:  true,

		masterPath: validPath,
	})

	// contextless-drops-context-only
	tests = append(tests, &testCase{
		_id:          "contextless-drops-context-only",
		_expected:    goldenContextless,
		_file:        "x.contextless.json",
		_isBadMaster: false,
		_shouldPass:  true,

		masterPath: validPath,
	})

	// known-ledger-rows
	tests = append(tests, &testCase{
		_id:          "known-ledger-rows",
		_expected:    goldenKnown,
		_file:        "x.known.json",
		_isBadMaster: false,
		_shouldPass:  true,

		masterPath: validPath,
	})

	// unclassified-finding
	unclassifiedPath := filepath.Join(masterDir, "unclassified.master.json")
	writeTestFile(t, unclassifiedPath, `{"provenance":{},"findings":[{"golden_id":"G-1","file":"a.go"}]}`)
	tests = append(tests, &testCase{
		_id:          "unclassified-finding",
		_expected:    "",
		_file:        "unclassified.json",
		_isBadMaster: true,
		_shouldPass:  false,

		masterPath: unclassifiedPath,
	})

	// empty-findings
	emptyPath := filepath.Join(masterDir, "empty.master.json")
	writeTestFile(t, emptyPath, `{"provenance":{},"findings":[]}`)
	tests = append(tests, &testCase{
		_id:          "empty-findings",
		_expected:    "",
		_file:        "empty.json",
		_isBadMaster: true,
		_shouldPass:  false,

		masterPath: emptyPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			derived, err := derivedGoldens(test.masterPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isBadMaster, errors.Is(err, ErrBadMaster))
			assert.Equal(t, test._expected, string(derived[test._file]))
		})
	}
}

func TestCheckGoldens(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skillsRoot string
	}

	tests := make([]*testCase, 0)

	// current-clean
	cleanRoot := t.TempDir()
	cleanMaster := filepath.Join(cleanRoot, "quality", "railroad-review", "eval", "golden", "x.master.json")
	writeTestFile(t, cleanMaster, goldenMaster)
	require.NoError(t, GenerateGoldens(cleanMaster))
	tests = append(tests, &testCase{
		_id:       "current-clean",
		_expected: nil,

		skillsRoot: cleanRoot,
	})

	// stale-reported
	staleRoot := t.TempDir()
	staleMaster := filepath.Join(staleRoot, "quality", "railroad-review", "eval", "golden", "x.master.json")
	writeTestFile(t, staleMaster, goldenMaster)
	require.NoError(t, GenerateGoldens(staleMaster))
	staleKnown := filepath.Join(filepath.Dir(staleMaster), "x.known.json")
	writeTestFile(t, staleKnown, "[]\n")
	tests = append(tests, &testCase{
		_id:       "stale-reported",
		_expected: []string{staleKnown},

		skillsRoot: staleRoot,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stale, err := CheckGoldens(test.skillsRoot)
			require.NoError(t, err)
			assert.Equal(t, test._expected, stale)
		})
	}
}
