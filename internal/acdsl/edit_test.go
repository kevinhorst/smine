package acdsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/reach"
)

func writeRulesFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rules.acdsl")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestSetRuleReachReplacesExistingAttr(t *testing.T) {
	content := "# header comment\n" +
		`//acdsl:R-A gofmt reach="global" anchor="\.go$" why="a"` + "\n" +
		`//acdsl:R-B gofmt anchor="\.go$" why="b"` + "\n"
	path := writeRulesFile(t, content)

	require.NoError(t, SetRuleReach(path, 2, "aqms,peek-mcp"))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	want := "# header comment\n" +
		`//acdsl:R-A gofmt reach="aqms,peek-mcp" anchor="\.go$" why="a"` + "\n" +
		`//acdsl:R-B gofmt anchor="\.go$" why="b"` + "\n"
	assert.Equal(t, want, string(got))
}

func TestSetRuleReachInsertsAttrAfterVerifier(t *testing.T) {
	content := `//acdsl:R-B gofmt anchor="\.go$" why="b"` + "\n"
	path := writeRulesFile(t, content)

	require.NoError(t, SetRuleReach(path, 1, reach.None))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, `//acdsl:R-B gofmt reach="none" anchor="\.go$" why="b"`+"\n", string(got))
}

func TestSetRuleReachRoundTripsThroughParse(t *testing.T) {
	content := `//acdsl:R-B gofmt anchor="\.go$" why="b"` + "\n"
	path := writeRulesFile(t, content)
	require.NoError(t, SetRuleReach(path, 1, reach.None))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	rules, violations := ParseRules(map[string][]string{path: strings.Split(string(data), "\n")})
	require.Empty(t, violations)
	require.Len(t, rules, 1)
	assert.Equal(t, reach.None, rules[0].Reach)
}

func TestSetRuleReachErrors(t *testing.T) {
	path := writeRulesFile(t, "# not a marker\n"+`//acdsl:R-A gofmt anchor="a" why="w"`+"\n")

	assert.Error(t, SetRuleReach(path, 1, "smine"), "non-marker line")
	assert.Error(t, SetRuleReach(path, 99, "smine"), "line out of range")
	assert.Error(t, SetRuleReach(path, 2, "aqms,global"), "invalid reach value")
	assert.Error(t, SetRuleReach(filepath.Join(t.TempDir(), "missing.acdsl"), 1, "smine"), "missing file")
}
