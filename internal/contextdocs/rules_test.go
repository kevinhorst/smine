package contextdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rulesFixture = `# Data Integrity

Intro prose.

**NEVER-INTEG-001** ` + "`[review]`" + ` — Never edit an applied migration.

* Why: history is append-only.
* Applies: diff touches migration files.

**ALWAYS-INTEG-001** ` + "`[review]`" + ` — Claim side effects transactionally.

* Applies: diff adds a send or charge.

` + "```markdown" + `
**NEVER-INTEG-099** ` + "`[review]`" + ` — Fenced example, never parsed.
` + "```" + `

## Tombstones

| Retired | Replacement | Date |
| :--- | :--- | :--- |
| RULE-INTEG-001 | NEVER-INTEG-001 | 2026-07-30 |
`

func writeRulesFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	return dir
}

func provideTestAspects() []RuleAspect {
	return []RuleAspect{
		{Name: "INTEG", Scope: "Data integrity"},
		{Name: "NAV", Scope: "Navigation"},
		{Name: "STACK", Scope: "Stack"},
	}
}

func TestParseRulesDir(t *testing.T) {
	t.Run("parses-entries-bullets-and-tombstones", func(t *testing.T) {
		set, err := ParseRulesDir(writeRulesFixture(t, map[string]string{"integrity.md": rulesFixture}), false)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)

		assert.Equal(t, "NEVER-INTEG-001", set.Entries[0].Id)
		assert.Equal(t, RuleClassNever, set.Entries[0].Class)
		assert.Equal(t, "INTEG", set.Entries[0].Aspect)
		assert.Equal(t, 1, set.Entries[0].Number)
		assert.Equal(t, "Never edit an applied migration.", set.Entries[0].Statement)
		assert.Equal(t, "review", set.Entries[0].Enforcement)
		assert.Equal(t, "diff touches migration files.", set.Entries[0].Applies)
		assert.Equal(t, RuleOriginBaseline, set.Entries[0].Origin)

		assert.Equal(t, "ALWAYS-INTEG-001", set.Entries[1].Id)

		require.Len(t, set.Tombstones, 1)
		assert.Equal(t, "RULE-INTEG-001", set.Tombstones[0].Retired)
		assert.Equal(t, "NEVER-INTEG-001", set.Tombstones[0].Replacement)
	})

	t.Run("malformed-entry-line-errors", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"broken.md": "**NEVER-INTEG-1** `[review]` — Two-digit number.\n",
		})
		_, err := ParseRulesDir(dir, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "malformed entry line")
	})

	t.Run("detects-origin-by-header", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"baseline.md": "<!-- synced from smine — do not edit -->\n\n**NEVER-NAV-001** `[review]` — Never scan.\n\n* Applies: everywhere.\n",
			"overlay.md":  "**NEVER-NAV-100** `[review]` — Repo-local rule.\n\n* Applies: this repo.\n",
		})
		set, err := ParseRulesDir(dir, true)
		require.NoError(t, err)
		require.Len(t, set.Entries, 2)
		assert.Equal(t, RuleOriginBaseline, set.Entries[0].Origin)
		assert.Equal(t, RuleOriginOverlay, set.Entries[1].Origin)
	})

	t.Run("repo-rules-parse-clean", func(t *testing.T) {
		repoRulesDir := filepath.Join("..", "..", "context", "rules")
		set, err := ParseRulesDir(repoRulesDir, false)
		require.NoError(t, err)
		assert.NotEmpty(t, set.Entries)
		factsSet, err := ParseFactsDir(filepath.Join("..", "..", "context", "facts"))
		require.NoError(t, err)
		assert.NotEmpty(t, factsSet.Entries)
		set.Entries = append(set.Entries, factsSet.Entries...)
		aspects, err := LoadAspects(repoRulesDir)
		require.NoError(t, err)
		assert.Empty(t, ValidateRules(set, aspects))
	})

	t.Run("non-entry-rule-ids-are-ignored", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"reviewing.md": "# Reviewing — Definition of Done\n\n- [ ] **RULE-DOD-101** — Feature matches the agreed spec\n",
		})
		set, err := ParseRulesDir(dir, false)
		require.NoError(t, err)
		assert.Empty(t, set.Entries)
	})
}

func TestParseFactsDir(t *testing.T) {
	t.Run("parses-overlay-origin", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"repo.md": "**FACT-STACK-001** — Go services.\n\n* Location: go.mod\n",
		})
		set, err := ParseFactsDir(dir)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, RuleOriginOverlay, set.Entries[0].Origin)
	})

	t.Run("facts-violation-surfaces", func(t *testing.T) {
		dir := writeRulesFixture(t, map[string]string{
			"repo.md": "**FACT-STACK-001** — Fact without location.\n",
		})
		set, err := ParseFactsDir(dir)
		require.NoError(t, err)
		violations := ValidateRules(set, provideTestAspects())
		require.NotEmpty(t, violations)
		assert.Contains(t, violations[0], "Location bullet")
	})
}

func TestParsePack(t *testing.T) {
	t.Run("facts-only-pack-parses", func(t *testing.T) {
		pack := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(pack, "facts"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(pack, "facts", "repo.md"),
			[]byte("**FACT-STACK-001** — Go services.\n\n* Location: go.mod\n"), 0644))

		set, err := ParsePack(pack)
		require.NoError(t, err)
		require.Len(t, set.Entries, 1)
		assert.Equal(t, RuleOriginOverlay, set.Entries[0].Origin)
		assert.Empty(t, ValidateRules(set, provideTestAspects()))
	})

	t.Run("missing-pack-dir-errors", func(t *testing.T) {
		_, err := ParsePack(filepath.Join(t.TempDir(), "nope"))
		assert.Error(t, err)
	})
}

func TestValidateRules(t *testing.T) {
	provideEntry := func(mutate func(entry *RuleEntry)) RuleEntry {
		entry := RuleEntry{
			Id:          "NEVER-INTEG-001",
			Class:       RuleClassNever,
			Aspect:      "INTEG",
			Number:      1,
			Statement:   "Never do the thing.",
			Enforcement: "review",
			Applies:     "always",
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
			entry.Id = "FACT-STACK-001"
			entry.Class = RuleClassFact
			entry.Aspect = "STACK"
			entry.Enforcement = ""
			entry.Applies = ""
			entry.Location = "go.mod"
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// duplicate-id
	tests = append(tests, &testCase{
		_id:               "duplicate-id",
		_expectedFragment: "duplicate id NEVER-INTEG-001",
		set: RuleSet{Entries: []RuleEntry{
			provideEntry(func(entry *RuleEntry) {}),
			provideEntry(func(entry *RuleEntry) {}),
		}},
	})

	// unknown-aspect
	tests = append(tests, &testCase{
		_id:               "unknown-aspect",
		_expectedFragment: "unknown aspect BOGUS",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Aspect = "BOGUS"
			entry.Id = "NEVER-BOGUS-001"
		})}},
	})

	// missing-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "missing-enforcement-tag",
		_expectedFragment: "require an enforcement tag",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Enforcement = ""
		})}},
	})

	// unknown-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "unknown-enforcement-tag",
		_expectedFragment: "unknown enforcement tag [vibes]",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Enforcement = "vibes"
		})}},
	})

	// fact-with-enforcement-tag
	tests = append(tests, &testCase{
		_id:               "fact-with-enforcement-tag",
		_expectedFragment: "FACT entries carry no enforcement tag",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-INTEG-001"
			entry.Class = RuleClassFact
			entry.Location = "go.mod"
			entry.Origin = RuleOriginOverlay
		})}},
	})

	// fact-missing-location
	tests = append(tests, &testCase{
		_id:               "fact-missing-location",
		_expectedFragment: "FACT entries require a Location bullet",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Id = "FACT-INTEG-001"
			entry.Class = RuleClassFact
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
			entry.Class = RuleClassFact
			entry.Enforcement = ""
			entry.Location = "go.mod"
		})}},
	})

	// missing-applies
	tests = append(tests, &testCase{
		_id:               "missing-applies",
		_expectedFragment: "require an Applies bullet",
		set: RuleSet{Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {
			entry.Applies = ""
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

	// tombstoned-number-reused
	tests = append(tests, &testCase{
		_id:               "tombstoned-number-reused",
		_expectedFragment: "numbers are never reused",
		set: RuleSet{
			Entries: []RuleEntry{provideEntry(func(entry *RuleEntry) {})},
			Tombstones: []RuleTombstone{{
				Retired:     "NEVER-INTEG-001",
				Replacement: "NEVER-INTEG-002",
				Date:        "2026-07-30",
				Source:      "rules/integrity.md",
			}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := ValidateRules(test.set, provideTestAspects())
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
	t.Run("loads-seed-file", func(t *testing.T) {
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
	t.Run("sorts-and-round-trips", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveAspects(dir, []RuleAspect{
			{Name: "STACK", Scope: "Stack"},
			{Name: "ARCH", Scope: "Architecture"},
		}))

		aspects, err := LoadAspects(dir)
		require.NoError(t, err)
		require.Len(t, aspects, 2)
		assert.Equal(t, "ARCH", aspects[0].Name)
		assert.Equal(t, "STACK", aspects[1].Name)
	})

	t.Run("trailing-newline", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveAspects(dir, []RuleAspect{{Name: "NAV", Scope: "n"}}))
		data, err := os.ReadFile(filepath.Join(dir, AspectsFileName))
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(string(data), "\n"))
	})
}

func TestRenderRulesJson(t *testing.T) {
	rendered, err := RenderRulesJson(RuleSet{Entries: []RuleEntry{{
		Id:     "FACT-STACK-001",
		Class:  RuleClassFact,
		Aspect: "STACK",
	}}})
	require.NoError(t, err)
	assert.Contains(t, string(rendered), `"id": "FACT-STACK-001"`)
	assert.True(t, strings.HasSuffix(string(rendered), "\n"))
}
