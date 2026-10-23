package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// OrphanRows lists the record rows no leaf of the checkout declares — SKILL_ keys against the leaves' declared variables, PROVIDER_ keys against the interface catalog — sorted; nothing is removed (master A6). A checkout without a skills dir lists nothing.
func OrphanRows(repoDir string, rows *Rows) ([]string, error) {
	skillsRoot := filepath.Join(repoDir, skillsDirName)
	if _, err := os.Stat(skillsRoot); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	run := &deployRun{options: &SyncOptions{RepoDir: repoDir}, report: &strings.Builder{}}
	leaves, err := run.repoLeaves()
	if err != nil {
		return nil, fmt.Errorf("OrphanRows: %w", err)
	}

	catalog, err := skillgraph.LoadCatalog(filepath.Join(skillsRoot, skillgraph.FileCatalog))
	if err != nil {
		return nil, fmt.Errorf("OrphanRows: %w", err)
	}

	declared := map[string]bool{}
	for _, leaf := range leaves {
		for name := range leaf.skill.VariableNames() {
			declared[name] = true
		}
	}
	providers := map[string]bool{}
	for _, provider := range catalog.Providers {
		providers[ProviderKey(provider.Id)] = true
	}

	var orphans []string
	for key := range rows.Skills {
		if !declared[key] {
			orphans = append(orphans, key)
		}
	}
	for key := range rows.Providers {
		if !providers[key] {
			orphans = append(orphans, key)
		}
	}
	slices.Sort(orphans)
	return orphans, nil
}
