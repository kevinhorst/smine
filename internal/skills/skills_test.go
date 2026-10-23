package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

func TestSplitDescription(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedArgs    []SkillArg
		_expectedSummary string

		description string
	}

	tests := make([]*testCase, 0)

	// full-convention
	tests = append(tests, &testCase{
		_id: "full-convention",
		_expectedArgs: []SkillArg{
			{Doc: "recent-turn count (default 5)", Name: "n"},
			{Doc: "mode token", Name: "list|plan|diff"},
		},
		_expectedSummary: "Show the latest session — turns, plan, diff, or list.",

		description: "Show the latest session — turns, plan, diff, or list. Trigger on /peek [n]. Args — n: recent-turn count (default 5); list|plan|diff: mode token.",
	})

	// no-args-segment
	tests = append(tests, &testCase{
		_id:              "no-args-segment",
		_expectedArgs:    nil,
		_expectedSummary: "Review code changes against project conventions.",

		description: "Review code changes against project conventions. Trigger on /railroad-review or \"review this branch\".",
	})

	// no-trigger-marker
	tests = append(tests, &testCase{
		_id:              "no-trigger-marker",
		_expectedArgs:    nil,
		_expectedSummary: "Compress prose to the bare minimum.",

		description: "Compress prose to the bare minimum.",
	})

	// trailing-whitespace-and-period
	tests = append(tests, &testCase{
		_id: "trailing-whitespace-and-period",
		_expectedArgs: []SkillArg{
			{Doc: "the one flag", Name: "flag"},
		},
		_expectedSummary: "Do the thing.",

		description: "Do the thing. Trigger on /thing. Args — flag: the one flag. ",
	})

	// arg-doc-with-colon
	tests = append(tests, &testCase{
		_id: "arg-doc-with-colon",
		_expectedArgs: []SkillArg{
			{Doc: "route selector: plan or skill", Name: "target"},
		},
		_expectedSummary: "Reformat a target.",

		description: "Reformat a target. Trigger on /fmt. Args — target: route selector: plan or skill.",
	})

	// empty-description
	tests = append(tests, &testCase{
		_id:              "empty-description",
		_expectedArgs:    nil,
		_expectedSummary: "",

		description: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			summary, args := splitDescription(test.description)
			assert.Equal(t, test._expectedSummary, summary)
			assert.Equal(t, test._expectedArgs, args)
		})
	}
}

func TestDescriptionConvention(t *testing.T) {
	const (
		maxDescriptionLen = 450
		maxSummaryLen     = 160
	)
	exempt := map[string]bool{
		"caveman": true,
		"jq":      true,
		"xlsx":    true,
	}

	dirs, err := skillgraph.Discover(filepath.Join("..", "..", "skills"))
	require.NoError(t, err)

	checked := 0
	for _, dir := range dirs {
		name := filepath.Base(dir)
		if exempt[name] {
			continue
		}

		data, readErr := skillgraph.ReadManifest(skillgraph.ModelPath(dir))
		require.NoError(t, readErr)
		fm := parseFrontmatter(string(data))
		summary, _ := splitDescription(fm.Description)
		checked++

		assert.Containsf(t, fm.Description, ". Trigger on ", "%s: description misses the '. Trigger on' marker", name)
		assert.LessOrEqualf(t, utf8.RuneCountInString(fm.Description), maxDescriptionLen, "%s: description exceeds %d chars", name, maxDescriptionLen)
		assert.LessOrEqualf(t, utf8.RuneCountInString(summary), maxSummaryLen, "%s: first sentence exceeds %d chars", name, maxSummaryLen)
		if strings.Contains(string(data), "\n## Args") {
			assert.Containsf(t, fm.Description, "Args — ", "%s: has an Args section but no 'Args —' description segment", name)
		}
	}
	assert.Greater(t, checked, 20, "repo skill sweep found implausibly few skills")
}

func TestParseFrontmatterFolded(t *testing.T) {
	folded := "---\nname: caveman\ndescription: >\n  Compress prose to the\n  bare minimum.\nauthor: X\n---\nbody\n"
	fm := parseFrontmatter(folded)
	assert.Equal(t, "caveman", fm.Name)
	assert.Equal(t, "Compress prose to the bare minimum.", fm.Description)
	assert.Equal(t, "X", fm.Author, "key after the fold still parsed")

	literal := "---\ndescription: |\n  Line one.\n  Line two.\n---\n"
	fm = parseFrontmatter(literal)
	assert.Equal(t, "Line one. Line two.", fm.Description)

	single := "---\ndescription: Plain single-line.\n---\n"
	fm = parseFrontmatter(single)
	assert.Equal(t, "Plain single-line.", fm.Description)

	terminated := "---\ndescription: >\nversion: 1.0\n---\n"
	fm = parseFrontmatter(terminated)
	assert.Empty(t, fm.Description, "next key at column 0 terminates an empty fold")
	assert.Equal(t, "1.0", fm.Version)
}

func TestParseFrontmatterArgumentHint(t *testing.T) {
	present := "---\nname: x\nargument-hint: \"/x foo: bar\"\n---\n"
	fm := parseFrontmatter(present)
	assert.Equal(t, "/x foo: bar", fm.ArgumentHint)

	absent := "---\nname: x\n---\n"
	fm = parseFrontmatter(absent)
	assert.Empty(t, fm.ArgumentHint)
}

func TestParseFrontmatterRuntime(t *testing.T) {
	present := "---\nname: x\ndocker: always\nenable-delegation: never\nenforce-permissions: acceptEdits, planMode\n---\n"
	parsed := parseFrontmatter(present)
	assert.Equal(t, "always", parsed.Docker)
	assert.Equal(t, "never", parsed.EnableDelegation)
	assert.Equal(t, "acceptEdits, planMode", parsed.EnforcePermissions)

	absent := "---\nname: x\n---\n"
	parsed = parseFrontmatter(absent)
	assert.Empty(t, parsed.Docker)
	assert.Empty(t, parsed.EnableDelegation)
	assert.Empty(t, parsed.EnforcePermissions)
}

func TestGroupTools(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ToolGroup

		raw string
	}

	tests := make([]*testCase, 0)

	// git-group-from-bash
	tests = append(tests, &testCase{
		_id: "git-group-from-bash",
		_expected: []ToolGroup{
			{Key: "git", Tools: []string{"git diff", "git log"}},
		},

		raw: "Bash(git log*), Bash(git diff*)",
	})

	// go-make-jq-groups-alphabetical
	tests = append(tests, &testCase{
		_id: "go-make-jq-groups-alphabetical",
		_expected: []ToolGroup{
			{Key: "go", Tools: []string{"go run ./cmd/acdsl"}},
			{Key: "jq", Tools: []string{"jq"}},
			{Key: "make", Tools: []string{"make build", "make test"}},
		},

		raw: "Bash(make test*), Bash(go run ./cmd/acdsl *), Bash(jq *), Bash(make build*)",
	})

	// plain-tools-first
	tests = append(tests, &testCase{
		_id: "plain-tools-first",
		_expected: []ToolGroup{
			{Key: "tools", Tools: []string{"Edit", "Read", "Write"}},
			{Key: "git", Tools: []string{"git diff"}},
		},

		raw: "Read, Write, Edit, Bash(git diff*)",
	})

	// mcp-server-group
	tests = append(tests, &testCase{
		_id: "mcp-server-group",
		_expected: []ToolGroup{
			{Key: "Peek_MCP", Tools: []string{"session_diff", "session_list"}},
		},

		raw: "mcp__Peek_MCP__session_list, mcp__Peek_MCP__session_diff",
	})

	// empty-string
	tests = append(tests, &testCase{
		_id:       "empty-string",
		_expected: []ToolGroup{},

		raw: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, GroupTools(test.raw))
		})
	}
}

func TestTitleCode(t *testing.T) {
	assert.Equal(t, "FD", TitleCode("---\nname: fdesign\ntitle-code: FD\nversion: 1.0.0\n---\n"))
	assert.Empty(t, TitleCode("---\nname: fdesign\n---\n"))
	assert.Empty(t, TitleCode("# no frontmatter\ntitle-code: FD\n"))
}

func TestVersion(t *testing.T) {
	writeManifest := func(t *testing.T, manifest string) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
		return dir
	}

	assert.Equal(t, "1.2.0", Version(writeManifest(t, "---\nname: fdesign\nversion: 1.2.0\n---\n")))
	assert.Empty(t, Version(writeManifest(t, "---\nname: fdesign\n---\n")))
	assert.Empty(t, Version(t.TempDir()))
	assert.Empty(t, Version(writeManifest(t, "# no frontmatter\nversion: 1.2.0\n")))
}
