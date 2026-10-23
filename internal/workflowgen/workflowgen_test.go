package workflowgen

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/models"
)

const (
	testModels    = `const DEFAULT_SUBAGENT_MODEL = "opus";`
	testStageFile = "stage.js"
)

func TestGenerator_Run(t *testing.T) {
	type testCase struct {
		_id                     string
		_expectedContent        string
		_expectedFindings       []string
		_expectedIsNoStageFiles bool
		_shouldPass             bool

		generator *Generator
		isCheck   bool
		stagePath string
	}

	tests := make([]*testCase, 0)

	staleSchemaStage := "// head\n// GEN:schemas lane\nconst OLD = 1;\n// GEN:end\n// tail\n"
	freshSchemaStage := "// head\n// GEN:schemas lane\nconst LANE_SCHEMA = {\n  \"type\": \"object\",\n  \"required\": [\n    \"a\"\n  ]\n}\n// GEN:end\n// tail\n"
	modelsBlock := "// GEN:models\n" + testModels + "\n// GEN:end\n"
	payloadText := "Brief on \"<base>\".\n- <question>\n"
	freshPayloadStage := "// GEN:payload SKILL-X-TPL-001\nconst PAYLOAD_SKILL_X_TPL_001 = \"Brief on \\\"<base>\\\".\\n- <question>\"\n// GEN:end\n"
	stalePayloadStage := "// GEN:payload SKILL-X-TPL-001\nconst PAYLOAD_SKILL_X_TPL_001 = \"hand edited\"\n// GEN:end\n"

	// schemas-block-rebuilt
	files := map[string]string{
		"schemas/lane.json":  `{"type": "object", "required": ["a"]}`,
		"workflows/stage.js": staleSchemaStage,
	}
	root := writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "schemas-block-rebuilt",
		_expectedContent:        freshSchemaStage,
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// placeholders-resolved
	files = map[string]string{
		"schemas/finding.json": `{"id": {"type": "string"}}`,
		"schemas/station.json": `{"items": "$FINDING_PROPS", "verdicts": "$VERDICT_PROPS"}`,
		"schemas/verdict.json": `{"verdict": {"type": "string"}}`,
		"workflows/stage.js":   "// GEN:schemas finding,station\n// GEN:end\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id: "placeholders-resolved",
		_expectedContent: "// GEN:schemas finding,station\n" +
			"const FINDING_PROPS = {\n  \"id\": {\n    \"type\": \"string\"\n  }\n}\n" +
			"const STATION_SCHEMA = {\n  \"items\": {\n    \"id\": {\n      \"type\": \"string\"\n    }\n  },\n" +
			"  \"verdicts\": {\n    \"verdict\": {\n      \"type\": \"string\"\n    }\n  }\n}\n" +
			"// GEN:end\n",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// severity-examples-key-order
	files = map[string]string{
		"severity-fixtures.json": `{"fixtures": [` +
			`{"id": "a", "discriminator": "d1", "severity": "BLOCKER", "exposure": "often", "effect": "incorrect", "example": true},` +
			`{"id": "b", "discriminator": "d2", "severity": "MINOR", "exposure": "rare", "effect": "cosmetic", "example": false}]}`,
		"workflows/stage.js": "// GEN:schemas severity-fixtures\n// GEN:end\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id: "severity-examples-key-order",
		_expectedContent: "// GEN:schemas severity-fixtures\n" +
			"const SEVERITY_EXAMPLES = [\n  {\n    \"effect\": \"incorrect\",\n    \"exposure\": \"often\",\n" +
			"    \"severity\": \"BLOCKER\",\n    \"discriminator\": \"d1\"\n  }\n]\n" +
			"// GEN:end\n",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// common-and-models-rebuilt
	files = map[string]string{
		"workflows/common/common.src.js": "function shared() {}\n",
		"workflows/stage.js":             "// GEN:models\nold\n// GEN:end\n// GEN:common\nold\n// GEN:end\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "common-and-models-rebuilt",
		_expectedContent:        modelsBlock + "// GEN:common\nfunction shared() {}\n// GEN:end\n",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// payload-block-rendered
	files = map[string]string{
		"payloads/SKILL-X-TPL-001.md": payloadText,
		"workflows/stage.js":          "// GEN:payload SKILL-X-TPL-001\n// GEN:end\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "payload-block-rendered",
		_expectedContent:        freshPayloadStage,
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// payload-check-mode-reports-stale
	files = map[string]string{
		"payloads/SKILL-X-TPL-001.md": payloadText,
		"workflows/stage.js":          stalePayloadStage,
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "payload-check-mode-reports-stale",
		_expectedContent:        stalePayloadStage,
		_expectedFindings:       []string{"stage.js is stale — run 'make workflow-gen'"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   true,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// payload-check-mode-fresh
	files = map[string]string{
		"payloads/SKILL-X-TPL-001.md": payloadText,
		"workflows/stage.js":          freshPayloadStage,
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "payload-check-mode-fresh",
		_expectedContent:        freshPayloadStage,
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   true,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// payload-missing
	files = map[string]string{
		"workflows/stage.js": "// GEN:payload SKILL-X-TPL-001\n// GEN:end\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "payload-missing",
		_expectedContent:        "// GEN:payload SKILL-X-TPL-001\n// GEN:end\n",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             false,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// check-mode-reports-stale
	files = map[string]string{
		"schemas/lane.json":  `{"type": "object", "required": ["a"]}`,
		"workflows/stage.js": staleSchemaStage,
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "check-mode-reports-stale",
		_expectedContent:        staleSchemaStage,
		_expectedFindings:       []string{"stage.js is stale — run 'make workflow-gen'"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   true,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// check-mode-fresh
	files = map[string]string{
		"schemas/lane.json":  `{"type": "object", "required": ["a"]}`,
		"workflows/stage.js": freshSchemaStage,
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "check-mode-fresh",
		_expectedContent:        freshSchemaStage,
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   true,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// no-stage-files
	files = map[string]string{
		"workflows/common/common.src.js": "function shared() {}\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "no-stage-files",
		_expectedContent:        "",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: true,
		_shouldPass:             false,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// handwritten-over-cap
	files = map[string]string{
		"workflows/stage.js": strings.Repeat("x\n", 301),
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "handwritten-over-cap",
		_expectedContent:        strings.Repeat("x\n", 301),
		_expectedFindings:       []string{"stage.js: 301 handwritten lines — the stage-file cap is 300"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// handwritten-at-cap-generated-excluded
	files = map[string]string{
		"workflows/stage.js": strings.Repeat("x\n", 300) + modelsBlock,
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "handwritten-at-cap-generated-excluded",
		_expectedContent:        strings.Repeat("x\n", 300) + modelsBlock,
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// conduct-language
	files = map[string]string{
		"workflows/stage.js": "// Use as few tool calls as you can.\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "conduct-language",
		_expectedContent:        "// Use as few tool calls as you can.\n",
		_expectedFindings:       []string{"stage.js: conduct/limit language is banned — never limit a worker's tool calls or reads"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// copy-back-language
	files = map[string]string{
		"workflows/stage.js": "// Return the file byte-for-byte.\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "copy-back-language",
		_expectedContent:        "// Return the file byte-for-byte.\n",
		_expectedFindings:       []string{"stage.js: copy-back language is banned — no agent re-emits data the script already holds"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// schema-literal-outside-block
	files = map[string]string{
		"workflows/stage.js": "const LANE_SCHEMA = {};\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "schema-literal-outside-block",
		_expectedContent:        "const LANE_SCHEMA = {};\n",
		_expectedFindings:       []string{"stage.js: schema literal outside a GEN block — schemas live in schemas/*.json"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// agent-without-models-block
	files = map[string]string{
		"workflows/stage.js": "await agent(DEFAULT_SUBAGENT_MODEL);\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "agent-without-models-block",
		_expectedContent:        "await agent(DEFAULT_SUBAGENT_MODEL);\n",
		_expectedFindings:       []string{"stage.js: spawns agents without the GEN:models block — every agent() names an asserted model"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// agent-without-model-name
	files = map[string]string{
		"workflows/stage.js": modelsBlock + "await agent({});\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "agent-without-model-name",
		_expectedContent:        modelsBlock + "await agent({});\n",
		_expectedFindings:       []string{"stage.js: spawns agents but never names a model through assertModel or DEFAULT_SUBAGENT_MODEL"},
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// agent-with-assert-model
	files = map[string]string{
		"workflows/stage.js": modelsBlock + "await agent({model: assertModel(\"opus\")});\n",
	}
	root = writeSkillFiles(t, files)
	tests = append(tests, &testCase{
		_id:                     "agent-with-assert-model",
		_expectedContent:        modelsBlock + "await agent({model: assertModel(\"opus\")});\n",
		_expectedFindings:       nil,
		_expectedIsNoStageFiles: false,
		_shouldPass:             true,

		generator: NewGenerator(filepath.Join(root, "workflows"), testModels),
		isCheck:   false,
		stagePath: filepath.Join(root, "workflows", testStageFile),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			findings, err := test.generator.Run(test.isCheck)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedIsNoStageFiles, errors.Is(err, ErrNoStageFiles))
			assert.Equal(t, test._expectedFindings, findings)
			assert.Equal(t, test._expectedContent, readStage(test.stagePath))
		})
	}
}

// TestGenerator_Run_committedStageFiles pins the generator to the bytes the
// shell generator committed: every stage file of the repo is fresh and passes
// the shape rules.
func TestGenerator_Run_committedStageFiles(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedFindings []string
		_shouldPass       bool

		generator *Generator
	}

	tests := make([]*testCase, 0)

	repoRoot := findRepoRoot(t)
	vocabulary, err := models.LoadTemplates(repoRoot)
	require.NoError(t, err)

	dirs, err := filepath.Glob(filepath.Join(repoRoot, "skills", "*", "*", "workflows"))
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	for _, dir := range dirs {
		relative, err := filepath.Rel(repoRoot, dir)
		require.NoError(t, err)

		tests = append(tests, &testCase{
			_id:               relative,
			_expectedFindings: nil,
			_shouldPass:       true,

			generator: NewGenerator(dir, vocabulary.WorkflowBlock()),
		})
	}

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			findings, err := test.generator.Run(true)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFindings, findings)
		})
	}
}

func TestGenerator_Run_missingPayload(t *testing.T) {
	files := map[string]string{
		"workflows/stage.js": "// GEN:payload SKILL-X-TPL-001\n// GEN:end\n",
	}
	root := writeSkillFiles(t, files)
	generator := NewGenerator(filepath.Join(root, "workflows"), testModels)

	_, err := generator.Run(false)

	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.ErrorContains(t, err, filepath.Join("payloads", "SKILL-X-TPL-001.md"))
}

func TestPayloadConst(t *testing.T) {
	assert.Equal(t, "PAYLOAD_SKILL_FDESIGN_TPL_004", payloadConst("SKILL-FDESIGN-TPL-004"))
}

func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no go.mod above the test's working directory")
		dir = parent
	}
}

func readStage(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeSkillFiles(t *testing.T, files map[string]string) string {
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}
