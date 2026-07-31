// Package contextdocs lists the repo's context source tree (context/) and
// wraps its deployment script, cmd/sync/sync_context.sh.
package contextdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Group is one context source subdirectory (general, go, ...) with its
// markdown files, both name-sorted.
type Group struct {
	Files []string
	Name  string
}

// Scan lists the context source subdirectories and their .md files. The
// plans skip mirrors the script's language enumeration (sync_context.sh) so
// the form never offers a dir the script would reject.
func Scan(srcDir string) ([]Group, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("Scan: %w", err)
	}

	var groups []Group
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "plans" {
			continue
		}

		files, err := os.ReadDir(filepath.Join(srcDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("Scan: %w", err)
		}
		group := Group{Name: entry.Name()}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}
			group.Files = append(group.Files, file.Name())
		}
		groups = append(groups, group)
	}
	return groups, nil
}
