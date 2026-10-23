// Package contextdocs parses, filters and packs the context entry tree and deploys it per registry repo.
package contextdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Group struct {
	Files []string
	Name  string
}

// plans/ holds design plans, not context groups.
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
