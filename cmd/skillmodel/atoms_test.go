package main

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// censusSkill builds a one-section skill whose single entry carries role and pred.
func censusSkill(atoms map[string]*skillgraph.Atom, role, pred, sectionPred string) *skillgraph.Skill {
	entry := &skillgraph.Entry{
		Id:        "SKILL-DEMO-RUN-001",
		Pred:      pred,
		Role:      role,
		Statement: "Run the tools.",
		Title:     "Tool run",
	}
	skill := &skillgraph.Skill{
		Atoms: atoms,
		Sections: []*skillgraph.Section{{
			Blocks: []skillgraph.Block{{Entry: entry}},
			Pred:   sectionPred,
			Title:  "Run",
		}},
	}
	return skill
}

// runAtomsCaptured runs the atoms command and answers its exit code and stdout.
func runAtomsCaptured(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runAtoms(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(output)
}

func TestAtomCensus(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []atomRow

		skill *skillgraph.Skill
	}

	tests := make([]*testCase, 0)

	// local-with-entry-reader
	localAtoms := map[string]*skillgraph.Atom{"ready": {Sort: "state"}}
	tests = append(tests, &testCase{
		_id: "local-with-entry-reader",
		_expected: []atomRow{{
			Id:       "ready",
			IsShared: false,
			Kind:     skillgraph.KindAtomBool,
			Readers:  []skillgraph.Reader{{Entry: "SKILL-DEMO-RUN-001", Role: skillgraph.RoleStop, Section: "Run"}},
		}},

		skill: censusSkill(localAtoms, skillgraph.RoleStop, "ready", ""),
	})

	// shared-read-is-listed
	sharedSkill := censusSkill(nil, skillgraph.RoleStep, "", "panic")
	sharedSkill.Vocabulary = &skillgraph.Vocabulary{Atoms: map[string]*skillgraph.Atom{"panic": {Sort: "shared", Source: "world"}}}
	tests = append(tests, &testCase{
		_id: "shared-read-is-listed",
		_expected: []atomRow{{
			Id:       "panic",
			IsShared: true,
			Kind:     skillgraph.KindAtomBool,
			Readers:  []skillgraph.Reader{{Role: skillgraph.ReaderSection, Section: "Run"}},
			Source:   "world",
		}},

		skill: sharedSkill,
	})

	// shared-unread-is-left-out
	unreadSkill := censusSkill(nil, skillgraph.RoleStep, "", "")
	unreadSkill.Vocabulary = &skillgraph.Vocabulary{Atoms: map[string]*skillgraph.Atom{"idle": {Sort: "shared"}}}
	tests = append(tests, &testCase{
		_id:       "shared-unread-is-left-out",
		_expected: nil,

		skill: unreadSkill,
	})

	// query-reader-listed
	queriedSkill := censusSkill(map[string]*skillgraph.Atom{"routed": {Sort: "state"}}, skillgraph.RoleStep, "", "")
	queriedSkill.Queries = []skillgraph.Query{{Check: "routed", Expect: "unsat", Name: "q"}}
	tests = append(tests, &testCase{
		_id: "query-reader-listed",
		_expected: []atomRow{{
			Id:       "routed",
			IsShared: false,
			Kind:     skillgraph.KindAtomBool,
			Readers:  []skillgraph.Reader{{Query: "q", Role: skillgraph.ReaderQuery}},
		}},

		skill: queriedSkill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, atomCensus(test.skill))
		})
	}
}

func TestFormatAtomRow(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		row *atomRow
	}

	tests := make([]*testCase, 0)

	// no-readers-dashes
	tests = append(tests, &testCase{
		_id:       "no-readers-dashes",
		_expected: "ready  bool  source=-  preds=-  queries=-",

		row: &atomRow{Id: "ready", Kind: skillgraph.KindAtomBool},
	})

	// section-and-entry-readers
	readers := []skillgraph.Reader{
		{Role: skillgraph.ReaderSection, Section: "Run"},
		{Entry: "SKILL-DEMO-RUN-001", Role: skillgraph.RoleStop, Section: "Run"},
		{Query: "q", Role: skillgraph.ReaderQuery},
	}
	tests = append(tests, &testCase{
		_id:       "section-and-entry-readers",
		_expected: "panic  bool shared  source=world  preds=section:Run,SKILL-DEMO-RUN-001(stop)  queries=q",

		row: &atomRow{Id: "panic", IsShared: true, Kind: skillgraph.KindAtomBool, Readers: readers, Source: "world"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, formatAtomRow(test.row))
		})
	}
}

func TestRunAtoms(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCode   int
		_expectedOutput string

		args []string
	}

	modelDir := saveFixture(t, modelAtoms(), "tools_loaded")
	tests := make([]*testCase, 0)

	// text-output
	tests = append(tests, &testCase{
		_id:             "text-output",
		_expectedCode:   exitClean,
		_expectedOutput: "tools_loaded  bool  source=-  preds=SKILL-DEMO-RUN-002(step)  queries=-\n",

		args: []string{modelDir},
	})

	// json-output
	tests = append(tests, &testCase{
		_id:           "json-output",
		_expectedCode: exitClean,
		_expectedOutput: `[
  {
    "id": "tools_loaded",
    "shared": false,
    "kind": "bool",
    "readers": [
      {
        "entry": "SKILL-DEMO-RUN-002",
        "role": "step",
        "section": "Run"
      }
    ]
  }
]
`,

		args: []string{"--json", modelDir},
	})

	// usage-without-dir
	tests = append(tests, &testCase{
		_id:             "usage-without-dir",
		_expectedCode:   exitError,
		_expectedOutput: "",

		args: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, output := runAtomsCaptured(t, test.args)
			assert.Equal(t, test._expectedCode, code)
			assert.Equal(t, test._expectedOutput, output)
		})
	}
}
