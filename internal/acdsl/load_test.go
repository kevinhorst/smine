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
	write("evals/run/probes/fixture.acdsl", marker(`X-002 fake anchor="x" why="w"`)+"\n")
	write("evals/demo/2026-10-06-abc123/interpretation.md", "- a\n")
	write("sessions/work/2026-W42/abc123.json", "{}\n")
	write("plans/2026-01-01-x-wip-abc1234/reviews/round-1/probes/probes.acdsl", marker(`PROBE-X-001 absent anchor="x" lifetime="task" why="w"`)+"\n")
	return root
}

func TestFileUniverseAllAndPathspecs(t *testing.T) {
	root := gitFixture(t)
	ctx := context.Background()

	all, err := FileUniverse(ctx, root)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"internal/a/a.go", "tasks/contract.acdsl", "tasks/contract.acdsl.archived", "docs/note.md",
		"evals/run/probes/fixture.acdsl",
		"evals/demo/2026-10-06-abc123/interpretation.md",
		"sessions/work/2026-W42/abc123.json",
		"plans/2026-01-01-x-wip-abc1234/reviews/round-1/probes/probes.acdsl",
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
	// The renamed .acdsl.archived file is data, not a declaration surface;
	// so is a persisted review round's probes.acdsl mirror.
	assert.ElementsMatch(t, []string{"X-001", "TASK-X-001"}, ids)
	// The anchor universe is every file — anchors alone decide scope.
	assert.Contains(t, discovery.Universe, "docs/note.md")
	assert.Contains(t, discovery.Universe, "internal/a/a.go")
	// Recorded-artifact trees stay out, except the files rules gate.
	assert.NotContains(t, discovery.Universe, "evals/run/probes/fixture.acdsl")
	assert.Contains(t, discovery.Universe, "evals/demo/2026-10-06-abc123/interpretation.md")
	assert.Contains(t, discovery.Universe, "sessions/work/2026-W42/abc123.json")
}
