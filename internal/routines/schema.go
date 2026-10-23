package routines

import (
	_ "embed"
	"sync"

	"github.com/kevinhorst/smine/internal/jsonschemacheck"
)

// changeTaskSchemaUrl is absolute so a violation names the schema the same way from every working directory.
const changeTaskSchemaUrl = "urn:smine:change-task.schema.json"

//go:embed change-task.schema.json
var changeTaskSchemaDocument []byte

var changeTaskSchema = sync.OnceValues(func() (*jsonschemacheck.Schema, error) {
	return jsonschemacheck.NewSchemaDocument(changeTaskSchemaUrl, changeTaskSchemaDocument)
})

// ChangeTaskSchema is the one definition of a task file; the rules binary and the task reader share it.
func ChangeTaskSchema() (*jsonschemacheck.Schema, error) {
	return changeTaskSchema()
}
