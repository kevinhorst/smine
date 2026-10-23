package contextdocs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideSourceIndex() *ContextFile {
	index := &ContextFile{
		Self: "smine",
		Entries: []RuleEntry{
			{Id: "ACTION-IMPL-001", Kind: "action", Origin: "baseline", Reach: "global", Scope: "IMPL", Source: "/abs/context/actions/implementing.md"},
			{Id: "ACTION-IMPL-002", Kind: "action", Origin: "baseline", Reach: "smine", Scope: "IMPL", Source: "context/actions/implementing.md"},
			{Id: "ACTION-IMPL-003", Kind: "action", Origin: "baseline", Reach: "work, peek-mcp", Scope: "IMPL", Source: "context/actions/implementing.md"},
			{Id: "ACTION-IMPL-004", Kind: "action", Origin: "baseline", Reach: "always", Scope: "IMPL", Source: "context/actions/implementing.md"},
			{Id: "RULE-GOLANG-ERR-001", Kind: "rule", Origin: "baseline", Reach: "global", Scope: "GOLANG", Source: "context/rules/go.md"},
			{Id: "RULE-PYTHON-ERR-001", Kind: "rule", Origin: "baseline", Reach: "global", Scope: "PYTHON", Source: "context/rules/python.md"},
			{Id: "RULE-SKILL-001", Kind: "rule", Origin: "baseline", Reach: "global", Scope: "SKILL", Source: "context/rules/skills.md"},
		},
		Tombstones: []RuleTombstone{{Retired: "ACTION-IMPL-009", Replacement: "—", Date: "2026-01-01", Source: "/abs/context/actions/implementing.md"}},
		Guides: []RuleGuide{
			{Files: []string{"*.go"}, Name: "go", Path: "rules/go.md", Source: "context/rules/go.md"},
			{Files: []string{"*_test.go"}, Name: "go_test", Path: "rules/go_test.md", Source: "context/rules/go_test.md"},
			{Files: []string{"*.py"}, Name: "python", Path: "rules/python.md", Source: "context/rules/python.md"},
			{Files: []string{"skills/**/*.cue"}, Name: "skills", Path: "rules/skills.md", Source: "context/rules/skills.md"},
		},
		Aspects: provideFilterAspects(),
	}
	index.Aspects = append(index.Aspects, RuleAspect{Name: "SKILL", Scope: "Skills", Class: "scope"})
	return index
}

func TestFilterIndex(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedEntries []string
		_expectedGuides  []string

		langs []string
		name  string
	}

	tests := make([]*testCase, 0)

	// work-with-go
	tests = append(tests, &testCase{
		_id:              "work-with-go",
		_expectedEntries: []string{"ACTION-IMPL-001", "ACTION-IMPL-003", "RULE-GOLANG-ERR-001", "RULE-SKILL-001"},
		_expectedGuides:  []string{"go", "go_test", "skills"},

		langs: []string{"go"},
		name:  "work",
	})

	// other-repo-no-langs
	tests = append(tests, &testCase{
		_id:              "other-repo-no-langs",
		_expectedEntries: []string{"ACTION-IMPL-001", "RULE-SKILL-001"},
		_expectedGuides:  []string{"skills"},

		langs: nil,
		name:  "otherrepo",
	})

	// smine-named-target-gets-smine-entries
	tests = append(tests, &testCase{
		_id:              "smine-named-target-gets-smine-entries",
		_expectedEntries: []string{"ACTION-IMPL-001", "ACTION-IMPL-002", "RULE-GOLANG-ERR-001", "RULE-PYTHON-ERR-001", "RULE-SKILL-001"},
		_expectedGuides:  []string{"go", "go_test", "python", "skills"},

		langs: []string{"go", "python"},
		name:  "smine",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			filtered := FilterIndex(provideSourceIndex(), test.name, test.langs, "docs")
			var ids, guides []string
			for _, entry := range filtered.Entries {
				ids = append(ids, entry.Id)
			}
			for _, guide := range filtered.Guides {
				guides = append(guides, guide.Name)
			}
			assert.Equal(t, test._expectedEntries, ids)
			assert.Equal(t, test._expectedGuides, guides)
			assert.Empty(t, filtered.Self)
			assert.Len(t, filtered.Aspects, 5)
			assert.Len(t, filtered.Tombstones, 1)
		})
	}
}

func TestFilterIndexRewritesSources(t *testing.T) {
	filtered := FilterIndex(provideSourceIndex(), "work", []string{"go"}, "docs")

	assert.Equal(t, "docs/actions/implementing.md", filtered.Entries[0].Source)
	assert.Equal(t, "docs/actions/implementing.md", filtered.Tombstones[0].Source)
	assert.Equal(t, "docs/rules/go.md", filtered.Guides[0].Source)
	assert.Equal(t, "rules/go.md", filtered.Guides[0].Path)
}

func TestLangs(t *testing.T) {
	assert.Equal(t, []string{"go", "python"}, Langs(provideFilterAspects()))
	assert.Empty(t, Langs(nil))
}

func TestMergeRepoOwned(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "actions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "facts"), 0o755))
	baseline := "<!-- synced from smine — do not edit -->\n\n**ACTION-IMPL-001** `[prose]` — Copy.\n\n* Applies: always.\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "actions", "implementing.md"), []byte(baseline), 0o644))
	own := "**ACTION-IMPL-101** `[prose]` — Own.\n\n* Applies: here.\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "actions", "local.md"), []byte(own), 0o644))
	fact := "**FACT-REPO-ARCH-101** — Own fact.\n\n* Location: here\n* Reach: work\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "facts", "repo.md"), []byte(fact), 0o644))

	aspects := append(provideFilterAspects(),
		RuleAspect{Name: "REPO", Scope: "Repository", Class: "scope"},
		RuleAspect{Name: "ARCH", Scope: "Architecture", Class: "topic"},
	)
	index := &ContextFile{Aspects: aspects, Entries: []RuleEntry{{Id: "ACTION-IMPL-001", Origin: "baseline"}}}
	require.NoError(t, MergeRepoOwned(index, root, t.TempDir()))

	var ids []string
	for _, entry := range index.Entries {
		ids = append(ids, entry.Id)
	}
	assert.Equal(t, []string{"ACTION-IMPL-001", "ACTION-IMPL-101", "FACT-REPO-ARCH-101"}, ids)
}

func TestMergeRepoOwnedMissingDir(t *testing.T) {
	index := &ContextFile{}
	require.NoError(t, MergeRepoOwned(index, filepath.Join(t.TempDir(), "absent"), t.TempDir()))
	assert.Empty(t, index.Entries)
}

func TestMergeRepoOwnedParseError(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "actions"), 0o755))
	broken := "**ACTION-IMPL-101** `[prose]` — Own.\n\n**Files:** `*.go`\n**Files:** `*.py`\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "actions", "local.md"), []byte(broken), 0o644))

	err := MergeRepoOwned(&ContextFile{}, root, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MergeRepoOwned")
}

func TestMergeRepoOwnedRefusesBaselineNumber(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "actions"), 0o755))
	own := "**ACTION-IMPL-099** `[prose]` — Overlay with a baseline number.\n\n* Applies: here.\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "actions", "local.md"), []byte(own), 0o644))

	index := &ContextFile{Aspects: provideFilterAspects()}
	err := MergeRepoOwned(index, root, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overlay entries use numbers 100+")
	assert.Empty(t, index.Entries)
}
