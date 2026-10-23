package contextdocs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/reach"
)

const rulesFixture = `# Data Integrity

Intro prose.

**ACTION-IMPL-INTEG-001** ` + "`[prose]`" + ` — Never edit an applied migration.

* Why: history is append-only.
* Applies: diff touches migration files.

**ACTION-IMPL-INTEG-004** ` + "`[prose]`" + ` — Claim side effects transactionally.

* Applies: diff adds a send or charge.

` + "```markdown" + `
**NEVER-INTEG-099** ` + "`[prose]`" + ` — Fenced example, never parsed.
` + "```" + `

## Tombstones

| Retired | Replacement | Date |
| :--- | :--- | :--- |
| RULE-INTEG-001 | ACTION-IMPL-INTEG-001 | 2026-07-30 |
`

func writeRulesFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	return dir
}

// writeContextFixture is a context root with one entry and the test aspects saved by the production writer.
func writeContextFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "nav.md"),
		[]byte("**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n"), 0644))
	require.NoError(t, SaveAspects(dir, provideTestAspects()))
	return dir
}

func provideTestAspects() []RuleAspect {
	return []RuleAspect{
		{Name: "IMPL", Scope: "Implementing activity", Class: "scope"},
		{Name: "INTEG", Scope: "Data integrity", Class: "topic"},
		{Name: "NAV", Scope: "Navigation", Class: "scope"},
		{Name: "REPO", Scope: "This repository", Class: "scope"},
		{Name: "STACK", Scope: "Stack", Class: "topic"},
	}
}

func TestParseRulesDir(t *testing.T) {
	t.Run("parses-entries-bullets-and-tombstones", func(t *testing.T) {
		set, err := ParseRulesDir(writeRulesFixture(t, map[string]string{"integrity.md": rulesFixture}), false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)

		assert.Equal(t, "ACTION-IMPL-INTEG-001", set.Entries[0].Id)
		assert.Equal(t, RuleKindAction, set.Entries[0].Kind)
		assert.Equal(t, "IMPL", set.Entries[0].Scope)
		assert.Equal(t, "INTEG", set.Entries[0].Topic)
		assert.Equal(t, 1, set.Entries[0].Number)
		assert.Equal(t, "Never edit an applied migration.", set.Entries[0].Content.Statement)
		assert.Equal(t, "prose", set.Entries[0].Enforcement)
		assert.Equal(t, "diff touches migration files.", set.Entries[0].Content.Applies)
		assert.Equal(t, RuleOriginBaseline, set.Entries[0].Origin)

		assert.Equal(t, "ACTION-IMPL-INTEG-004", set.Entries[1].Id)

		require.Len(t, set.Tombstones, 1)
		assert.Equal(t, "RULE-INTEG-001", set.Tombstones[0].Retired)
		assert.Equal(t, "ACTION-IMPL-INTEG-001", set.Tombstones[0].Replacement)
	})

	t.Run("marker-tag-parses", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"integrity.md": "# Data Integrity\n\n**ACTION-IMPL-INTEG-001** `[prose]` `[DoD]` — Never edit an applied migration.\n\n* Applies: diff touches migration files.\n\n**ACTION-IMPL-INTEG-004** `[prose]` — Claim side effects transactionally.\n\n* Applies: diff adds a send or charge.\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)
		assert.Equal(t, []string{"DoD"}, set.Entries[0].Markers)
		assert.Equal(t, "prose", set.Entries[0].Enforcement)
		assert.Equal(t, "Never edit an applied migration.", set.Entries[0].Content.Statement)
		assert.Nil(t, set.Entries[1].Markers)
	})

	t.Run("soft-wrapped-headline-joins-statement", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"runbooks.md": "# Runbooks\n\n**RULE-NAV-001** `[prose]` — A runbook is started from the repo's\ntemplate, never from scratch.\n\n* Applies: everywhere.\n\nA prose note after a blank line.\n\n**RULE-NAV-002** `[prose]` — Second rule\n1. numbered item stops the join\n\n**RULE-NAV-003** `[prose]` — Third rule\n| a table row stops the join |\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 3)
		assert.Equal(t, "A runbook is started from the repo's template, never from scratch.", set.Entries[0].Content.Statement)
		assert.Equal(t, []string{"A prose note after a blank line."}, set.Entries[0].Content.Notes)
		assert.Equal(t, "Second rule", set.Entries[1].Content.Statement)
		assert.Equal(t, []string{"1. numbered item stops the join"}, set.Entries[1].Content.Notes)
		assert.Equal(t, "Third rule", set.Entries[2].Content.Statement)
	})

	t.Run("malformed-entry-line-still-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"integrity.md": "**ACTION-IMPL-INTEG-001** `[prose]` `[DoD]` no dash separator\n",
		})
		_, err := ParseRulesDir(dir, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed entry line")
	})

	t.Run("content-is-typed-fields", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"go.md": "# GO\n\n**RULE-NAV-001** `[prose]` — Names are verbose.\n\n* Applies: everywhere.\n* Why: clarity.\n* Evidence: measured drift.\n* Version: 1.1\n* Names MUST be verbose.\n  Continuation of the bullet.\n* No `i` loops.\n\nA prose note.\n\n```go\n// GOOD\n---\nvar hookGroup int\n```\n\n```\nbare fence\n```\n\n**RULE-NAV-002** `[prose]` — Second rule.\n\n## SECTION\n\nprose\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)
		content := set.Entries[0].Content
		assert.Equal(t, "Names are verbose.", content.Statement)
		assert.Equal(t, "everywhere.", content.Applies)
		assert.Equal(t, "clarity.", content.Why)
		assert.Equal(t, "measured drift.", content.Evidence)
		assert.Equal(t, "1.1", set.Entries[0].Version)
		assert.Equal(t, []string{"Names MUST be verbose.\n  Continuation of the bullet.", "No `i` loops."}, content.Bullets)
		assert.Equal(t, []string{"A prose note."}, content.Notes)
		require.Len(t, content.Examples, 2)
		assert.Equal(t, RuleExample{Lang: "go", Code: "// GOOD\n---\nvar hookGroup int"}, content.Examples[0])
		assert.Equal(t, RuleExample{Lang: "", Code: "bare fence"}, content.Examples[1])
		assert.Equal(t, RuleContent{Statement: "Second rule."}, set.Entries[1].Content)
	})

	t.Run("files-line-declares-a-guide", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"go.md":   "# GO\n\n**Files:** `*.go`, `*_test.go`\n\n**RULE-NAV-001** `[prose]` — Names are verbose.\n",
			"plan.md": "# PLAN\n\n**RULE-NAV-002** `[prose]` — No Files line here.\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Guides, 1)
		assert.Equal(t, "go", set.Guides[0].Name)
		assert.Equal(t, []string{"*.go", "*_test.go"}, set.Guides[0].Files)
		assert.True(t, strings.HasSuffix(set.Guides[0].Path, "/go.md"))
	})

	t.Run("second-files-line-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"go.md": "**Files:** `*.go`\n\n**Files:** `*.py`\n",
		})
		_, err := ParseRulesDir(dir, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "second Files line")
	})

	t.Run("files-line-without-globs-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"go.md": "**Files:** all go files\n",
		})
		_, err := ParseRulesDir(dir, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no backticked glob")
	})

	t.Run("malformed-entry-line-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"broken.md": "**ACTION-IMPL-1** `[prose]` — Two-digit number.\n",
		})
		_, err := ParseRulesDir(dir, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed entry line")
	})

	t.Run("detects-origin-by-header", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"baseline.md": "<!-- synced from smine — do not edit -->\n\n**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n",
			"overlay.md":  "**ACTION-NAV-100** `[prose]` — Repo-local rule.\n\n* Applies: this repo.\n",
		})
		set, err := ParseRulesDir(dir, true)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)
		assert.Equal(t, RuleOriginBaseline, set.Entries[0].Origin)
		assert.Equal(t, RuleOriginOverlay, set.Entries[1].Origin)
	})

	t.Run("rule-kind-headline-parses", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"go.md": "**RULE-NAV-INTEG-001** `[prose]` — Names are verbose.\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, RuleKindRule, set.Entries[0].Kind)
		assert.Equal(t, "NAV", set.Entries[0].Scope)
		assert.Equal(t, "INTEG", set.Entries[0].Topic)
		assert.Equal(t, "prose", set.Entries[0].Enforcement)
	})

	t.Run("version-bullet-overrides-default", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"nav.md": "**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n* Version: 1.2\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, "1.2", set.Entries[0].Version)
	})

	t.Run("version-defaults-to-1.0", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"nav.md": "**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, "1.0", set.Entries[0].Version)
	})

	t.Run("reach-bullet-parses-smine-and-list", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"nav.md": "**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n* Reach: smine\n\n**ACTION-NAV-002** `[prose]` — Always look.\n\n* Applies: everywhere.\n* Reach: work, peek-mcp\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)
		assert.Equal(t, "smine", set.Entries[0].Reach)
		assert.Equal(t, "work, peek-mcp", set.Entries[1].Reach)
	})

	t.Run("reach-defaults-to-global", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"nav.md": "**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, "global", set.Entries[0].Reach)
	})

	t.Run("non-entry-rule-ids-are-ignored", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"reviewing.md": "# Reviewing — Definition of Done\n\n- [ ] **ACTION-REVIEW-101** — Feature matches the agreed spec\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		assert.Empty(t, set.Entries)
	})
}

func TestParseFactsDir(t *testing.T) {
	t.Run("parses-overlay-origin", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"repo.md": "**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n",
		})
		set, err := ParseFactsDir(dir)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, RuleOriginOverlay, set.Entries[0].Origin)
	})

	t.Run("facts-violation-surfaces", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"repo.md": "**FACT-REPO-STACK-001** — Fact without location.\n",
		})
		set, err := ParseFactsDir(dir)
		require.NoError(t, err)
		violations := ValidateRules(set, provideTestAspects(), "")
		require.NotEmpty(t, violations)
		assert.Contains(t, violations[0], "Location bullet")
	})
}

func TestParseContext(t *testing.T) {
	t.Run("facts-only-pack-parses", func(t *testing.T) {
		pack := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(pack, "facts"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(pack, "facts", "repo.md"),
			[]byte("**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: smine\n"), 0644))

		set, err := ParseContext(pack, true)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, RuleOriginOverlay, set.Entries[0].Origin)
		assert.Empty(t, ValidateRules(set, provideTestAspects(), ""))
	})

	t.Run("missing-pack-dir-errors", func(t *testing.T) {
		_, err := ParseContext(filepath.Join(t.TempDir(), "nope"), true)
		assert.Error(t, err)
	})

	t.Run("merges-actions-rules-facts", func(t *testing.T) {
		contextDir := t.TempDir()
		for dir, content := range map[string]string{
			"actions": "**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n",
			"rules":   "**RULE-NAV-001** `[prose]` — Names are verbose.\n",
			"facts":   "**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n",
		} {
			require.NoError(t, os.MkdirAll(filepath.Join(contextDir, dir), 0755))
			require.NoError(t, os.WriteFile(filepath.Join(contextDir, dir, "one.md"), []byte(content), 0644))
		}
		set, err := ParseContext(contextDir, false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 3)
		kinds := []string{set.Entries[0].Kind, set.Entries[1].Kind, set.Entries[2].Kind}
		assert.Equal(t, []string{RuleKindAction, RuleKindRule, RuleKindFact}, kinds)
	})

	t.Run("guide-path-is-relative-to-the-context-dir", func(t *testing.T) {
		contextDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "rules"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "rules", "go.md"),
			[]byte("**Files:** `*.go`\n\n**RULE-NAV-001** `[prose]` — Names are verbose.\n"), 0644))
		set, err := ParseContext(contextDir, false)
		require.NoError(t, err)
		require.Len(t, set.Guides, 1)
		assert.Equal(t, "rules/go.md", set.Guides[0].Path)
		assert.Equal(t, "go", set.Guides[0].Name)
	})

	t.Run("self-names-the-source-repo", func(t *testing.T) {
		contextDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "facts"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "facts", "repo.md"),
			[]byte("**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n"), 0644))

		source, err := ParseContext(contextDir, false)
		require.NoError(t, err)
		assert.Equal(t, reach.ThisRepo, source.Self)

		deployed, err := ParseContext(contextDir, true)
		require.NoError(t, err)
		assert.Empty(t, deployed.Self)
	})
}

// writeIndexDirs is a temp cwd root holding an empty context.json under each named dir.
func writeIndexDirs(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(root, dir, ContextFileName), []byte("{}\n"), 0644))
	}
	return root
}

func TestResolveContextDir(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedDir        string
		_expectedErrorPart  string
		_expectedIsDeployed bool
		_shouldPass         bool

		deployedName string
		explicit     string
		root         string
	}

	tests := make([]*testCase, 0)

	// explicit-wins
	tests = append(tests, &testCase{
		_id:                 "explicit-wins",
		_expectedDir:        "custom",
		_expectedErrorPart:  "",
		_expectedIsDeployed: false,
		_shouldPass:         true,

		deployedName: "",
		explicit:     "custom",
		root:         writeIndexDirs(t, "context", "docs"),
	})

	// source-context-dir
	tests = append(tests, &testCase{
		_id:                 "source-context-dir",
		_expectedDir:        "context",
		_expectedErrorPart:  "",
		_expectedIsDeployed: false,
		_shouldPass:         true,

		deployedName: "",
		explicit:     "",
		root:         writeIndexDirs(t, "context", "docs"),
	})

	// deployed-docs-dir
	tests = append(tests, &testCase{
		_id:                 "deployed-docs-dir",
		_expectedDir:        "docs",
		_expectedErrorPart:  "",
		_expectedIsDeployed: true,
		_shouldPass:         true,

		deployedName: "",
		explicit:     "",
		root:         writeIndexDirs(t, "docs"),
	})

	// deployed-name-from-environment
	tests = append(tests, &testCase{
		_id:                 "deployed-name-from-environment",
		_expectedDir:        "site",
		_expectedErrorPart:  "",
		_expectedIsDeployed: true,
		_shouldPass:         true,

		deployedName: "site",
		explicit:     "",
		root:         writeIndexDirs(t, "docs", "site"),
	})

	// neither-errors
	tests = append(tests, &testCase{
		_id:                 "neither-errors",
		_expectedDir:        "",
		_expectedErrorPart:  "neither context/context.json nor docs/context.json",
		_expectedIsDeployed: false,
		_shouldPass:         false,

		deployedName: "",
		explicit:     "",
		root:         writeIndexDirs(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Chdir(test.root)
			dir, isDeployed, err := ResolveContextDir(test.explicit, test.deployedName)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDir, dir)
			assert.Equal(t, test._expectedIsDeployed, isDeployed)
			assert.Contains(t, fmt.Sprint(err), test._expectedErrorPart)
		})
	}
}

func TestValidateRules(t *testing.T) {
	provideEntry := func(mutate func(entry *RuleEntry)) RuleEntry {
		entry := RuleEntry{
			Id:          "ACTION-IMPL-INTEG-001",
			Kind:        RuleKindAction,
			Scope:       "IMPL",
			Topic:       "INTEG",
			Number:      1,
			Content:     RuleContent{Statement: "Never do the thing.", Applies: "always"},
			Enforcement: "prose",
			Reach:       "global",
			Source:      "rules/integrity.md",
			Origin:      RuleOriginBaseline,
		}
		mutate(&entry)
		return entry
	}

	type testCase struct {
		_id               string
		_expectedFragment string
		set               RuleSet
	}

	tests := make([]*testCase, 0)

	// clean-set
	tests = append(tests, &testCase{
		_id:               "clean-set",
		_expectedFragment: "",
		set:               RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {})}},
	})

	// clean-fact-overlay
	tests = append(tests, &testCase{
		_id:               "clean-fact-overlay",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-REPO-STACK-001"
			entry.Kind = RuleKindFact
			entry.Scope = "REPO"
			entry.Topic = "STACK"
			entry.Enforcement = ""
			entry.Content.Applies = ""
			entry.Content.Location = "go.mod"
			entry.Reach = "smine"
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// duplicate-id
	tests = append(tests, &testCase{
		_id:               "duplicate-id",
		_expectedFragment: "duplicate id ACTION-IMPL-INTEG-001",
		set: RuleSet{Entries: []RuleEntry{
			provideEntry(func(entry *RuleEntry) {}),
			provideEntry(func(entry *RuleEntry) {}),
		}},
	})

	// unknown-scope
	tests = append(tests, &testCase{
		_id:               "unknown-scope",
		_expectedFragment: "scope BOGUS is not a registered class-scope taxonomy entry",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Scope = "BOGUS"
			entry.Id = "ACTION-BOGUS-001"
		})}},
	})

	// missing-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "missing-enforcement-tag",
		_expectedFragment: "require a channel tag",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Enforcement = ""
		})}},
	})

	// unknown-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "unknown-enforcement-tag",
		_expectedFragment: "unknown channel tag [vibes]",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Enforcement = "vibes"
		})}},
	})

	// clean-marker-tag
	tests = append(tests, &testCase{
		_id:               "clean-marker-tag",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Markers = []string{"DoD"}
		})}},
	})

	// unknown-marker-tag
	tests = append(tests, &testCase{
		_id:               "unknown-marker-tag",
		_expectedFragment: "unknown marker tag [Todo]",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Markers = []string{"Todo"}
		})}},
	})

	// fact-with-marker-tag
	tests = append(tests, &testCase{
		_id:               "fact-with-marker-tag",
		_expectedFragment: "FACT entries carry no marker tag",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-REPO-STACK-001"
			entry.Kind = RuleKindFact
			entry.Scope = "REPO"
			entry.Topic = "STACK"
			entry.Enforcement = ""
			entry.Content.Applies = ""
			entry.Content.Location = "go.mod"
			entry.Origin = RuleOriginOverlay
			entry.Markers = []string{"DoD"}
		})}},
	})

	// fact-with-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "fact-with-enforcement-tag",
		_expectedFragment: "FACT entries carry no role or channel tag",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-INTEG-001"
			entry.Kind = RuleKindFact
			entry.Content.Location = "go.mod"
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// fact-missing-location
	tests = append(tests, &testCase{
		_id:               "fact-missing-location",
		_expectedFragment: "FACT entries require a Location bullet",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-INTEG-001"
			entry.Kind = RuleKindFact
			entry.Enforcement = ""
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// fact-in-baseline
	tests = append(tests, &testCase{
		_id:               "fact-in-baseline",
		_expectedFragment: "facts never ship in the synced baseline",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-INTEG-001"
			entry.Kind = RuleKindFact
			entry.Enforcement = ""
			entry.Content.Location = "go.mod"
		})}},
	})

	// missing-applies
	tests = append(tests, &testCase{
		_id:               "missing-applies",
		_expectedFragment: "require an Applies bullet",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Content.Applies = ""
		})}},
	})

	// baseline-number-out-of-range
	tests = append(tests, &testCase{
		_id:               "baseline-number-out-of-range",
		_expectedFragment: "baseline entries use numbers 001-099",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "NEVER-INTEG-100"
			entry.Number = 100
		})}},
	})

	// overlay-number-out-of-range
	tests = append(tests, &testCase{
		_id:               "overlay-number-out-of-range",
		_expectedFragment: "overlay entries use numbers 100+",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// invalid-reach-value
	tests = append(tests, &testCase{
		_id:               "invalid-reach-value",
		_expectedFragment: "Reach must be global, always, none, or a comma-separated repo-name list",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Reach = "bogus name"
		})}},
	})

	// always-reach-is-valid
	tests = append(tests, &testCase{
		_id:               "always-reach-is-valid",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Reach = "always"
		})}},
	})

	// none-reach-is-valid
	tests = append(tests, &testCase{
		_id:               "none-reach-is-valid",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Reach = "none"
		})}},
	})

	// list-reach-is-valid
	tests = append(tests, &testCase{
		_id:               "list-reach-is-valid",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Reach = "work, peek-mcp"
		})}},
	})

	// repo-scope-rejects-global-reach
	tests = append(tests, &testCase{
		_id:               "repo-scope-rejects-global-reach",
		_expectedFragment: "never global",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-REPO-STACK-001"
			entry.Kind = RuleKindFact
			entry.Scope = "REPO"
			entry.Topic = "STACK"
			entry.Content.Location = "go.mod"
			entry.Enforcement = ""
			entry.Origin = RuleOriginOverlay
			entry.Source = "facts/repo.md"
			entry.Reach = "global"
		})}},
	})

	// repo-scope-rejects-always-reach
	tests = append(tests, &testCase{
		_id:               "repo-scope-rejects-always-reach",
		_expectedFragment: "never always-injected",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-REPO-STACK-001"
			entry.Kind = RuleKindFact
			entry.Scope = "REPO"
			entry.Topic = "STACK"
			entry.Content.Location = "go.mod"
			entry.Enforcement = ""
			entry.Origin = RuleOriginOverlay
			entry.Source = "facts/repo.md"
			entry.Reach = "always"
		})}},
	})

	// repo-scope-accepts-repo-reach
	tests = append(tests, &testCase{
		_id:               "repo-scope-accepts-repo-reach",
		_expectedFragment: "",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-REPO-STACK-001"
			entry.Kind = RuleKindFact
			entry.Scope = "REPO"
			entry.Topic = "STACK"
			entry.Content.Location = "go.mod"
			entry.Enforcement = ""
			entry.Origin = RuleOriginOverlay
			entry.Source = "facts/repo.md"
			entry.Reach = "smine"
		})}},
	})

	// smine-scope-requires-smine-reach
	tests = append(tests, &testCase{
		_id:               "smine-scope-requires-smine-reach",
		_expectedFragment: "Reach: smine required",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "ACTION-SMINE-001"
			entry.Scope = "SMINE"
			entry.Topic = ""
		})}},
	})

	// tombstoned-number-reused
	tests = append(tests, &testCase{
		_id:               "tombstoned-number-reused",
		_expectedFragment: "numbers are never reused",
		set: RuleSet{
			Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {})},
			Tombstones: []RuleTombstone{{
				Retired:     "ACTION-IMPL-INTEG-001",
				Replacement: "ACTION-IMPL-INTEG-002",
				Date:        "2026-07-30",
				Source:      "rules/integrity.md",
			}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := ValidateRules(test.set, provideTestAspects(), "")
			if test._expectedFragment == "" {
				assert.Empty(t, violations)
				return
			}
			require.NotEmpty(t, violations)
			assert.Contains(t, strings.Join(violations, "\n"), test._expectedFragment)
		})
	}
}

func TestValidateRulesChannelClaims(t *testing.T) {
	provideGateAction := func(mutate func(entry *RuleEntry)) RuleEntry {
		entry := RuleEntry{
			Id:          "ACTION-IMPL-001",
			Kind:        RuleKindAction,
			Scope:       "IMPL",
			Number:      1,
			Content:     RuleContent{Statement: "Never do the thing.", Applies: "always"},
			Enforcement: "gate",
			Reach:       "global",
			Source:      "actions/impl.md",
			Origin:      RuleOriginBaseline,
		}
		mutate(&entry)
		return entry
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "acdsl"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "acdsl", "rules.acdsl"),
		[]byte("//acdsl:ACDSL-GOLANG-ENUM-001 exhaustive-switch reach=\"global\" why=\"test\"\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "hooks"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "hooks", "read-gate.sh"), []byte("#!/bin/bash\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.json"), []byte("{}"), 0644))

	type testCase struct {
		_id               string
		_expectedFragment string
		entry             RuleEntry
	}
	tests := []*testCase{
		{
			_id:               "gate-resolves",
			_expectedFragment: "",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Content.Gate = "ACDSL-GOLANG-ENUM-001"
			}),
		},
		{
			_id:               "gate-missing-pointer",
			_expectedFragment: "[gate] requires a Gate: bullet",
			entry:             provideGateAction(func(entry *RuleEntry) {}),
		},
		{
			_id:               "gate-unknown-id",
			_expectedFragment: "not declared in acdsl/rules.acdsl",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Content.Gate = "ACDSL-BOGUS-001"
			}),
		},
		{
			_id:               "hook-resolves",
			_expectedFragment: "",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Enforcement = "hook"
				entry.Content.Hook = "read-gate.sh"
			}),
		},
		{
			_id:               "hook-not-found",
			_expectedFragment: "not found under cmd/hooks/",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Enforcement = "hook"
				entry.Content.Hook = "nope.sh"
			}),
		},
		{
			_id:               "schema-resolves",
			_expectedFragment: "",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Enforcement = "schema"
				entry.Content.Schema = "schema.json"
			}),
		},
		{
			_id:               "schema-not-found",
			_expectedFragment: "Schema: nope.json not found",
			entry: provideGateAction(func(entry *RuleEntry) {
				entry.Enforcement = "schema"
				entry.Content.Schema = "nope.json"
			}),
		},
	}

	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := ValidateRules(RuleSet{Entries: []RuleEntry{test.entry}}, provideTestAspects(), dir)
			if test._expectedFragment == "" {
				assert.Empty(t, violations)
				return
			}
			require.NotEmpty(t, violations)
			assert.Contains(t, strings.Join(violations, "\n"), test._expectedFragment)
		})
	}
}

func TestLoadAspects(t *testing.T) {
	t.Run("loads-aspects-json", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			AspectsFileName: `[{"name": "NAV", "scope": "Navigation"}]`,
		})
		aspects, err := LoadAspects(dir)
		require.NoError(t, err)
		require.Len(t, aspects, 1)
		assert.Equal(t, "NAV", aspects[0].Name)
		assert.Equal(t, "Navigation", aspects[0].Scope)
	})

	t.Run("missing-file-errors", func(t *testing.T) {
		_, err := LoadAspects(t.TempDir())
		require.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("malformed-json-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{AspectsFileName: "{not json"})
		_, err := LoadAspects(dir)
		assert.Error(t, err)
	})
}

func TestSaveAspects(t *testing.T) {
	dir := t.TempDir()
	aspects := []RuleAspect{
		{Name: "STACK", Scope: "Stack & tooling"},
		{Name: "ARCH", Scope: "Architecture"},
	}
	require.NoError(t, SaveAspects(dir, aspects))

	loaded, err := LoadAspects(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "ARCH", loaded[0].Name)
	assert.Equal(t, "STACK", loaded[1].Name)

	data, err := os.ReadFile(filepath.Join(dir, AspectsFileName))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"scope": "Stack & tooling"`)
	assert.True(t, strings.HasSuffix(string(data), "]\n"))
}

func TestWriteContextFile(t *testing.T) {
	dir := writeContextFixture(t)
	require.NoError(t, WriteContextFile(dir))

	data, err := os.ReadFile(filepath.Join(dir, ContextFileName))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id": "ACTION-NAV-001"`)
	assert.Contains(t, string(data), `"name": "NAV"`)
	assert.True(t, strings.HasSuffix(string(data), "\n"))
}

func TestRegenerate(t *testing.T) {
	t.Run("writes-context-file", func(t *testing.T) {
		dir := writeContextFixture(t)
		require.NoError(t, Regenerate(dir))
		assert.FileExists(t, filepath.Join(dir, ContextFileName))
	})

	t.Run("no-aspects-source-is-left-alone", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, Regenerate(dir))
		assert.NoFileExists(t, filepath.Join(dir, ContextFileName))
	})
}

func TestRegenerateSkipsDeployedIndex(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, SaveAspects(dir, nil))
	deployed := `{"deploy": {"contextDir": "context", "langs": [], "role": "r", "acdsl": false, "markdown": false, "task": false, "writeRepo": true}, "entries": [], "tombstones": [], "guides": [], "aspects": []}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ContextFileName), []byte(deployed), 0o644))

	require.NoError(t, Regenerate(dir))

	kept, err := os.ReadFile(filepath.Join(dir, ContextFileName))
	require.NoError(t, err)
	assert.Equal(t, deployed, string(kept))
}

func TestContextFileRenderCarriesDeployFirst(t *testing.T) {
	index := &ContextFile{Deploy: &ContextDeploy{ContextDir: "docs", Langs: []string{"go"}, Role: "r"}}

	data, err := index.Render()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(string(data), "{\n  \"deploy\": {"))
	reloaded := &ContextFile{}
	require.NoError(t, json.Unmarshal(data, reloaded))
	assert.Equal(t, index.Deploy, reloaded.Deploy)
}

func TestIsDeployedIndex(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsDeployedIndex(filepath.Join(dir, "missing.json")))

	source := filepath.Join(dir, "source.json")
	require.NoError(t, os.WriteFile(source, []byte(`{"self": "smine", "entries": []}`), 0o644))
	assert.False(t, IsDeployedIndex(source))

	deployed := filepath.Join(dir, "deployed.json")
	require.NoError(t, os.WriteFile(deployed, []byte(`{"deploy": {"contextDir": "docs"}, "entries": []}`), 0o644))
	assert.True(t, IsDeployedIndex(deployed))
}

func TestLoadContextFile(t *testing.T) {
	set := RuleSet{
		Entries:    []RuleEntry{{Id: "ACTION-REVIEW-001", Kind: RuleKindAction, Origin: RuleOriginBaseline, Scope: "REVIEW", Content: RuleContent{Statement: "Review."}}},
		Guides:     []RuleGuide{{Name: "go", Path: "rules/go.md", Files: []string{"*.go"}, Source: "rules/go.md"}},
		Tombstones: []RuleTombstone{{Retired: "ACTION-REVIEW-000", Replacement: "ACTION-REVIEW-001", Date: "2026-10-12", Source: "actions/reviewing.md"}},
	}
	rendered, err := RenderContextJson(set, nil, "")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), ContextFileName)
	require.NoError(t, os.WriteFile(path, rendered, 0o644))

	index, err := LoadContextFile(path)
	require.NoError(t, err)
	assert.Equal(t, set, index.RuleSet())

	_, err = LoadContextFile(filepath.Join(t.TempDir(), ContextFileName))
	assert.Error(t, err)
}

func TestRenderContextJson(t *testing.T) {
	rendered, err := RenderContextJson(RuleSet{
		Entries: []RuleEntry{{
			Id:    "FACT-REPO-STACK-001",
			Kind:  RuleKindFact,
			Scope: "REPO",
			Topic: "STACK",
		}},
		Guides: []RuleGuide{
			{Name: "python", Path: "rules/python.md", Files: []string{"*.py"}, Source: "rules/python.md"},
			{Name: "go", Path: "rules/go.md", Files: []string{"*.go"}, Source: "rules/go.md"},
		},
	}, []RuleAspect{{Name: "REPO", Scope: "repo facts", Class: "scope"}}, "smine")
	require.NoError(t, err)
	assert.Contains(t, string(rendered), `"self": "smine"`)
	assert.Contains(t, string(rendered), `"id": "FACT-REPO-STACK-001"`)
	assert.Contains(t, string(rendered), `"name": "REPO"`)
	assert.Contains(t, string(rendered), `"guides": [`)
	assert.Contains(t, string(rendered), `"content": {`)
	assert.Less(t, strings.Index(string(rendered), `"name": "go"`), strings.Index(string(rendered), `"name": "python"`))
	assert.True(t, strings.HasSuffix(string(rendered), "\n"))
}
