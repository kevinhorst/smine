package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// unplacedSchema matches cmd/agentwt/schema/unplaced-v1.json.
const unplacedSchema = 1

type Unplaced struct {
	Branch       string           `json:"branch"`
	Commits      []UnplacedCommit `json:"commits"`
	Onto         string           `json:"onto"`
	OntoSha      string           `json:"onto_sha"`
	OntoShortSha string           `json:"onto_short_sha"`
	Schema       int              `json:"schema"`
}

// UnplacedCommit is one commit line of the `unplaced` document.
type UnplacedCommit struct {
	CommittedAt string `json:"committed_at"`
	IsMerge     bool   `json:"is_merge"`
	Sha         string `json:"sha"`
	ShortSha    string `json:"short_sha"`
	Subject     string `json:"subject"`
}

func ListUnplaced(ctx context.Context, agentwtPath, branch, onto, repoPath string) (*Unplaced, error) {
	if err := ValidateBranch(ctx, branch, repoPath); err != nil {
		return nil, fmt.Errorf("ListUnplaced: %w", err)
	}

	output, err := shell.Run(ctx, repoPath, agentwtPath, agentwtArgs(repoPath, "unplaced", "--onto", onto, branch)...)
	if err != nil {
		return nil, fmt.Errorf("ListUnplaced: %s: %w", strings.TrimSpace(output), err)
	}

	return decodeUnplaced(output)
}

// decodeUnplaced refuses an unknown schema rather than guess a field mapping.
func decodeUnplaced(output string) (*Unplaced, error) {
	var document Unplaced
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, fmt.Errorf("decodeUnplaced: Decode: %w", err)
	}
	if document.Schema != unplacedSchema {
		return nil, fmt.Errorf("decodeUnplaced: Unsupported schema %d", document.Schema)
	}
	return &document, nil
}
