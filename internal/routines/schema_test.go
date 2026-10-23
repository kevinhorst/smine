package routines

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plannerSchemaPath is the planner workflow's return schema, whose task object copies the embedded one.
const plannerSchemaPath = "skills/smine/smine-problems/schemas/planner.json"

func TestChangeTaskSchema(t *testing.T) {
	schema, err := ChangeTaskSchema()
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

func TestPlannerSchemaMatchesChangeTask(t *testing.T) {
	var embedded map[string]any
	require.NoError(t, json.Unmarshal(changeTaskSchemaDocument, &embedded))

	data, err := os.ReadFile(filepath.Join("..", "..", plannerSchemaPath))
	require.NoError(t, err)
	var planner map[string]any
	require.NoError(t, json.Unmarshal(data, &planner))

	task := planner["properties"].(map[string]any)["task"].(map[string]any)
	assert.Equal(t, embedded["required"], task["required"])
	assert.Equal(t, embedded["properties"], task["properties"])
}
