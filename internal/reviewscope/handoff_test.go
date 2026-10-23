package reviewscope

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateLedger_Gates(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedErr         bool
		_expectedOscillating []string

		fixture *ledgerFixture
		schema  string
	}

	tests := make([]*testCase, 0)

	carried := provideRankedEntry("a.go", "f1", "BLOCKER")
	carried.Known = true

	// oscillation-fixed-row
	tests = append(tests, &testCase{
		_id:                  "oscillation-fixed-row",
		_expectedErr:         false,
		_expectedOscillating: []string{"f1"},

		fixture: newLedgerFixture(t, "2", carried).known(provideKnownEntry("a.go", "f1", "BLOCKER", FixStateFixed, 1)),
		schema:  "",
	})

	// carried-open-row
	tests = append(tests, &testCase{
		_id:                  "carried-open-row",
		_expectedErr:         false,
		_expectedOscillating: []string{},

		fixture: newLedgerFixture(t, "2", carried).known(provideKnownEntry("a.go", "f1", "BLOCKER", FixStateOpen, 1)),
		schema:  "",
	})

	// carried-applied-row
	tests = append(tests, &testCase{
		_id:                  "carried-applied-row",
		_expectedErr:         false,
		_expectedOscillating: []string{},

		fixture: newLedgerFixture(t, "2", carried).known(provideKnownEntry("a.go", "f1", "BLOCKER", FixStateApplied, 1)),
		schema:  "",
	})

	// schema-violation
	tests = append(tests, &testCase{
		_id:                  "schema-violation",
		_expectedErr:         true,
		_expectedOscillating: nil,

		fixture: newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER")),
		schema:  `{"type": "object", "required": ["definition_of_done"]}`,
	})

	// schema-held
	tests = append(tests, &testCase{
		_id:                  "schema-held",
		_expectedErr:         false,
		_expectedOscillating: []string{},

		fixture: newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER")),
		schema:  `{"type": "object", "required": ["ranked_plan"]}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			request := test.fixture.request("blocker", 0)
			if test.schema != "" {
				request.SchemaPath = filepath.Join(test.fixture.artifactsDir, "consolidation.json")
				require.NoError(t, os.WriteFile(request.SchemaPath, []byte(test.schema), 0o644))
			}

			result, err := UpdateLedger(request)

			if test._expectedErr {
				require.Error(t, err)
				assert.NoFileExists(t, test.fixture.knownPath(), "nothing is written before the validation holds")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test._expectedOscillating, result.Oscillating)
		})
	}
}

// provideSchemaDir writes the named files into a fresh dir and returns it.
func provideSchemaDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	return dir
}

func TestValidateReview(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		reviewPath string
		schemaPath string
	}

	deployedSchema := filepath.Join("..", "..", "skills", "quality", "railroad-review", "schemas", "consolidation.json")
	tokenSchema := `{"type": "object", "required": ["finding"], "properties": {"finding": {"type": "object", "required": ["id"], "properties": "$FINDING_PROPS"}}}`
	sharedDir := provideSchemaDir(t, map[string]string{
		"consolidation.json": tokenSchema,
		"finding.json":       `{"id": {"type": "string"}}`,
		"held.json":          `{"finding": {"id": "tests-MAJ-1-r1"}}`,
		"violated.json":      `{"finding": {"id": 7}}`,
	})
	bareDir := provideSchemaDir(t, map[string]string{
		"consolidation.json": tokenSchema,
		"empty.json":         `{}`,
		"held.json":          `{"finding": {"id": "tests-MAJ-1-r1"}}`,
		"not-json.json":      `{"type": `,
	})
	tests := make([]*testCase, 0)

	// pass-deployed-schema-real-handoff
	tests = append(tests, &testCase{
		_id:         "pass-deployed-schema-real-handoff",
		_shouldPass: true,

		reviewPath: filepath.Join("testdata", "review-clean.json"),
		schemaPath: deployedSchema,
	})

	// fail-deployed-schema-empty-handoff
	tests = append(tests, &testCase{
		_id:         "fail-deployed-schema-empty-handoff",
		_shouldPass: false,

		reviewPath: filepath.Join(bareDir, "empty.json"),
		schemaPath: deployedSchema,
	})

	// pass-placeholder-resolved-from-sibling
	tests = append(tests, &testCase{
		_id:         "pass-placeholder-resolved-from-sibling",
		_shouldPass: true,

		reviewPath: filepath.Join(sharedDir, "held.json"),
		schemaPath: filepath.Join(sharedDir, "consolidation.json"),
	})

	// fail-shared-property-enforced
	tests = append(tests, &testCase{
		_id:         "fail-shared-property-enforced",
		_shouldPass: false,

		reviewPath: filepath.Join(sharedDir, "violated.json"),
		schemaPath: filepath.Join(sharedDir, "consolidation.json"),
	})

	// fail-placeholder-without-sibling
	tests = append(tests, &testCase{
		_id:         "fail-placeholder-without-sibling",
		_shouldPass: false,

		reviewPath: filepath.Join(bareDir, "held.json"),
		schemaPath: filepath.Join(bareDir, "consolidation.json"),
	})

	// fail-schema-not-json
	tests = append(tests, &testCase{
		_id:         "fail-schema-not-json",
		_shouldPass: false,

		reviewPath: filepath.Join(bareDir, "held.json"),
		schemaPath: filepath.Join(bareDir, "not-json.json"),
	})

	// fail-schema-missing
	tests = append(tests, &testCase{
		_id:         "fail-schema-missing",
		_shouldPass: false,

		reviewPath: filepath.Join(bareDir, "held.json"),
		schemaPath: filepath.Join(bareDir, "absent.json"),
	})

	// fail-review-missing
	tests = append(tests, &testCase{
		_id:         "fail-review-missing",
		_shouldPass: false,

		reviewPath: filepath.Join(bareDir, "absent.json"),
		schemaPath: deployedSchema,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := validateReview(test.reviewPath, test.schemaPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestUpdateLedger_PlanDirCopy(t *testing.T) {
	fixture := newLedgerFixture(t, "3", provideRankedEntry("a.go", "f1", "BLOCKER"))
	require.NoError(t, os.MkdirAll(filepath.Join(fixture.roundDir, probesDir, "fixtures"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fixture.roundDir, probesDir, "fixtures", "f1.go"), []byte("x"), 0o644))
	planDir := t.TempDir()
	request := fixture.request("blocker", 0)
	request.PlanDir = planDir

	_, err := UpdateLedger(request)

	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(planDir, "reviews", "round-3", reviewFile))
	assert.FileExists(t, filepath.Join(planDir, "reviews", "round-3", probesDir, "fixtures", "f1.go"))
}

func TestAppendRejection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rejected.json")

	require.NoError(t, AppendRejection(path, Rejection{FindingId: "f1", Reason: "intended", Round: 1}))
	require.NoError(t, AppendRejection(path, Rejection{FindingId: "f2", Reason: "out of scope", Round: 2}))

	var rejections []Rejection
	require.NoError(t, readJson(path, &rejections))
	assert.Equal(t, []Rejection{
		{FindingId: "f1", Reason: "intended", Round: 1},
		{FindingId: "f2", Reason: "out of scope", Round: 2},
	}, rejections)
}
