package contextdocs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideFilterAspects() []RuleAspect {
	return []RuleAspect{
		{Name: "GOLANG", Scope: "Go", Class: "scope", Lang: "go"},
		{Name: "PYTHON", Scope: "Python", Class: "scope", Lang: "python"},
		{Name: "IMPL", Scope: "Implementing", Class: "scope"},
		{Name: "INTEG", Scope: "Integrity", Class: "topic"},
	}
}

func TestFilterRulesFileDropsSmineReachEntriesWithBullets(t *testing.T) {
	content := "# Chapter\n\n" +
		"**ACTION-IMPL-001** `[review]` — Keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[review]` — Drop.\n\n* Applies: always.\n* Reach: smine\n\n" +
		"**ACTION-IMPL-003** `[review]` — Keep too.\n\n* Applies: always.\n"
	out := FilterRulesFile(content, "aqms", provideFilterAspects(), nil)
	assert.Contains(t, out, "ACTION-IMPL-001")
	assert.NotContains(t, out, "ACTION-IMPL-002")
	assert.NotContains(t, out, "Reach: smine")
	assert.Contains(t, out, "ACTION-IMPL-003")
}

func TestFilterRulesFileListReachCoversOnlyNamedTargets(t *testing.T) {
	content := "**ACTION-IMPL-001** `[review]` — Listed.\n\n* Applies: always.\n* Reach: aqms, peek-mcp\n"
	kept := FilterRulesFile(content, "aqms", provideFilterAspects(), nil)
	assert.Contains(t, kept, "ACTION-IMPL-001")
	dropped := FilterRulesFile(content, "otherrepo", provideFilterAspects(), nil)
	assert.NotContains(t, dropped, "ACTION-IMPL-001")
}

func TestFilterRulesFileLangBoundScopes(t *testing.T) {
	content := "**RULE-GOLANG-INTEG-001** `[review]` — Go rule.\n\n* Applies: go files.\n\n" +
		"**RULE-PYTHON-INTEG-001** `[review]` — Python rule.\n\n* Applies: py files.\n\n" +
		"**ACTION-IMPL-001** `[review]` — Lang-free.\n\n* Applies: always.\n"
	out := FilterRulesFile(content, "aqms", provideFilterAspects(), []string{"go"})
	assert.Contains(t, out, "RULE-GOLANG-INTEG-001")
	assert.NotContains(t, out, "RULE-PYTHON-INTEG-001")
	assert.Contains(t, out, "ACTION-IMPL-001")
}

func TestFilterRulesFilePassesNonEntryContentVerbatim(t *testing.T) {
	content := "# Heading\n\nProse paragraph.\n\n```markdown\n**ACTION-IMPL-099** `[review]` — Example in fence.\n\n* Reach: smine\n```\n\n" +
		"## Tombstones\n\n| Retired | Replacement | Date |\n| ACTION-IMPL-009 | — | 2026-01-01 |\n"
	out := FilterRulesFile(content, "aqms", provideFilterAspects(), nil)
	assert.Equal(t, content, out)
}

func TestFilterRulesFileOutputReparsesGreen(t *testing.T) {
	content := "**ACTION-IMPL-001** `[review]` — Keep.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[review]` — Drop.\n\n* Applies: always.\n* Reach: smine\n\n" +
		"**RULE-PYTHON-INTEG-001** `[review]` — Drop lang.\n\n* Applies: py.\n"
	out := FilterRulesFile(content, "aqms", provideFilterAspects(), []string{"go"})
	dir := writeRulesFixture(t, map[string]string{"chapter.md": out})
	set, err := ParseRulesDir(dir, false)
	require.NoError(t, err)
	require.Len(t, set.Entries, 1)
	assert.Equal(t, "ACTION-IMPL-001", set.Entries[0].Id)
	assert.False(t, strings.Contains(out, "\n\n\n"), "no double blank lines in filtered output")
}
