package acdsl

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitFixture initializes a throwaway repo with one governed Go file, one
// active .acdsl contract, one retired contract (renamed away from .acdsl),
// and one plain data file. Files stay untracked — the universe deliberately
// includes untracked unignored files.
func gitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	init := exec.Command("git", "init", "-q", root)
	require.NoError(t, init.Run())

	write := func(rel, content string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644))
	}
	write("internal/a/a.go", "package a\n"+marker(`X-001 fake anchor="\.go$" why="w"`)+"\n")
	write("tasks/contract.acdsl", marker(`TASK-X-001 fake anchor="^internal/a/" lifetime="task" why="w"`)+"\n")
	write("tasks/contract.acdsl.archived", marker(`TASK-X-999 fake anchor="x" lifetime="task" why="w"`)+"\n")
	write("docs/note.md", "just data\n")
	return root
}

func TestFileUniverseAllAndPathspecs(t *testing.T) {
	root := gitFixture(t)
	ctx := context.Background()

	all, err := FileUniverse(ctx, root)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"internal/a/a.go", "tasks/contract.acdsl", "tasks/contract.acdsl.archived", "docs/note.md",
	}, all)

	goFiles, err := FileUniverse(ctx, root, "*.go")
	require.NoError(t, err)
	assert.Equal(t, []string{"internal/a/a.go"}, goFiles)

	declarations, err := FileUniverse(ctx, root, declarationPathspecs()...)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"internal/a/a.go", "tasks/contract.acdsl", "docs/note.md"}, declarations)

	none, err := FileUniverse(ctx, root, "*.nothing")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestDeclarationPathspecsCoverEverySyntaxClass(t *testing.T) {
	specs := declarationPathspecs()
	assert.Contains(t, specs, "*.acdsl")
	assert.Contains(t, specs, "Makefile")
	for ext := range projectionSyntaxes {
		assert.Contains(t, specs, "*"+ext)
	}
	assert.NotContains(t, specs, "*.json")
}

func TestDiscoverRulesScansGoAndActiveAcdslOnly(t *testing.T) {
	root := gitFixture(t)
	discovery, err := DiscoverRules(context.Background(), root)
	require.NoError(t, err)
	require.Empty(t, discovery.Violations)

	ids := make([]string, 0, len(discovery.Rules))
	for _, rule := range discovery.Rules {
		ids = append(ids, rule.Id)
	}
	// The renamed .acdsl.archived file is data, not a declaration surface.
	assert.ElementsMatch(t, []string{"X-001", "TASK-X-001"}, ids)
	// The anchor universe is every file — anchors alone decide scope.
	assert.Contains(t, discovery.Universe, "docs/note.md")
	assert.Contains(t, discovery.Universe, "internal/a/a.go")
}
