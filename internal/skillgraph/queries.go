package skillgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// queriesDir holds a skill's committed query files, one JSON document each.
const queriesDir = "queries"

// Query is one committed query: assumptions, a static check or a reach target, and the expected verdict.
type Query struct {
	Name string `json:"name"`

	Assume []string `json:"assume"`
	Bound  *int     `json:"bound,omitempty"`
	Check  string   `json:"check,omitempty"`
	Expect string   `json:"expect"`
	Reach  string   `json:"reach,omitempty"`
}

// LoadQueries reads every queries/*.json of a skill dir, sorted by file name; a skill without the dir has none.
func LoadQueries(dir string) ([]Query, error) {
	paths, err := filepath.Glob(filepath.Join(dir, queriesDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadQueries: %w", err)
	}

	slices.Sort(paths)
	var queries []Query
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadQueries: %w", err)
		}

		query := Query{}
		if err := json.Unmarshal(data, &query); err != nil {
			return nil, fmt.Errorf("LoadQueries: %s: %w", path, err)
		}
		if query.Name == "" {
			query.Name = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		queries = append(queries, query)
	}
	return queries, nil
}
