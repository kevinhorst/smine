package skills

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/trash"
)

// absentContent stands for a file that does not exist in fileContents.
const absentContent = "<absent>"

// deployCatalogCue is the fixture skills root's interfaces.cue: install-scoped title-set@1, implemented by smine-rules of config-server, and the repo-scoped check@1 command without a provider.
const deployCatalogCue = `interfaces: [{id: "title-set", version: 1, kind: "program", scope: "install", summary: "Sets the title state.", invocation: {cwd: "repo-root", args: "title set <TOKEN>", stdin: "none", timeout: "1m"}, result: {exit: {"0": "set", other: "error"}, stdout: "title: set <TOKEN>"}, validate: {resolves: true}},
	{id: "check", version: 1, kind: "command", scope: "repo", example: "go vet ./...", summary: "Runs everything before a commit.", invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "30m"}, result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}, validate: {resolves: true}}]
providers: [{id: "smine-rules", program: "smine-rules", component: "config-server", implements: ["title-set@1"], probe: ["interfaces"]}]
`

// installRulesStub puts a smine-rules ahead of PATH that answers the handshake with claims; it also stands in for a fixture value naming my-rules.
func installRulesStub(t *testing.T, claims string) {
	t.Helper()
	dir := t.TempDir()
	answer := `{"interfaces":[` + claims + `]}`
	for _, program := range []string{"smine-rules", "my-rules"} {
		name, script := program, "#!/bin/sh\necho '"+answer+"'\n"
		if runtime.GOOS == "windows" {
			name, script = program+".bat", "@echo "+answer+"\r\n"
		}
		writeDeployFile(t, dir, name, script, 0o755)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// provideDeployCatalog is deployCatalogCue as the model the fixture skills carry before Save.
func provideDeployCatalog() *skillgraph.Catalog {
	titleSet := &skillgraph.Interface{
		Id:         "title-set",
		Invocation: skillgraph.Invocation{Args: "title set <TOKEN>", Cwd: "repo-root", Stdin: "none", Timeout: "1m"},
		Kind:       skillgraph.KindProgram,
		Result:     skillgraph.Result{Exit: map[string]string{"0": "set", "other": "error"}, Stdout: "title: set <TOKEN>"},
		Scope:      skillgraph.ScopeInstall,
		Summary:    "Sets the title state.",
		Validate:   skillgraph.Validate{Resolves: true},
		Version:    1,
	}
	check := &skillgraph.Interface{
		Id:         "check",
		Example:    "go vet ./...",
		Invocation: skillgraph.Invocation{Args: "none", Cwd: "repo-root", Stdin: "none", Timeout: "30m"},
		Kind:       skillgraph.KindCommand,
		Result:     skillgraph.Result{Exit: map[string]string{"0": "pass", "other": "fail"}, Stdout: "diagnostic"},
		Scope:      skillgraph.ScopeRepo,
		Summary:    "Runs everything before a commit.",
		Validate:   skillgraph.Validate{Resolves: true},
		Version:    1,
	}
	rules := &skillgraph.Provider{Id: "smine-rules", Component: components.ConfigServer, Implements: []string{"title-set@1"}, Probe: []string{"interfaces"}, Program: "smine-rules"}
	catalog := &skillgraph.Catalog{Interfaces: []*skillgraph.Interface{titleSet, check}, Providers: []*skillgraph.Provider{rules}}
	return catalog
}

// declareAliases re-saves the fixture leaf with an aliases frontmatter line
// and, when titleCode is set, a title-code line.
func declareAliases(t *testing.T, root, leaf, aliases, titleCode string) {
	t.Helper()
	skill := provideDeploySkill(leaf)
	skill.Frontmatter = append(skill.Frontmatter, skillgraph.Field{Key: "aliases", Value: " " + aliases})
	if titleCode != "" {
		skill.Frontmatter = append(skill.Frontmatter, skillgraph.Field{Key: "title-code", Value: " " + titleCode})
	}
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", leaf), skill))
}

// deployOptions is a full sync of the fixture checkout into the fixture home.
func deployOptions(root string) *SyncOptions {
	options := &SyncOptions{
		ContextDir: "ctx",
		Home:       filepath.Join(root, "home"),
		RepoDir:    filepath.Join(root, "repo"),
		Trash:      trash.New(filepath.Join(root, "home", ".claude", "smine"), 0),
	}
	return options
}

// existingFiles reports, for each path of expected, whether it exists below
// root; nil for an empty expectation.
func existingFiles(root string, expected map[string]bool) map[string]bool {
	if len(expected) == 0 {
		return nil
	}

	existing := map[string]bool{}
	for path := range expected {
		_, err := os.Lstat(filepath.Join(root, path))
		existing[path] = err == nil
	}
	return existing
}

// fileContents reads each path of expected below root, absentContent for a
// missing file; nil for an empty expectation.
func fileContents(root string, expected map[string]string) map[string]string {
	if len(expected) == 0 {
		return nil
	}

	contents := map[string]string{}
	for path := range expected {
		content, err := os.ReadFile(filepath.Join(root, path))
		contents[path] = string(content)
		if err != nil {
			contents[path] = absentContent
		}
	}
	return contents
}

// fileModes reads the permission bits of each path of expected below root;
// nil for an empty expectation.
func fileModes(root string, expected map[string]fs.FileMode) map[string]fs.FileMode {
	if len(expected) == 0 {
		return nil
	}

	modes := map[string]fs.FileMode{}
	for path := range expected {
		info, err := os.Stat(filepath.Join(root, path))
		modes[path] = 0
		if err == nil {
			modes[path] = info.Mode().Perm()
		}
	}
	return modes
}

// missingSnippets lists every snippet of snippets its file below root does
// not contain.
func missingSnippets(root string, snippets map[string][]string) []string {
	var missing []string
	for path, wanted := range snippets {
		content, _ := os.ReadFile(filepath.Join(root, path))
		for _, snippet := range wanted {
			if !strings.Contains(string(content), snippet) {
				missing = append(missing, path+": "+snippet)
			}
		}
	}
	return missing
}

// presentInReport lists every snippet of unwanted the report contains.
func presentInReport(report string, unwanted []string) []string {
	var present []string
	for _, snippet := range unwanted {
		if strings.Contains(report, snippet) {
			present = append(present, snippet)
		}
	}
	return present
}

// presentSnippets lists every snippet of snippets its file below root
// contains.
func presentSnippets(root string, snippets map[string][]string) []string {
	var present []string
	for path, unwanted := range snippets {
		content, _ := os.ReadFile(filepath.Join(root, path))
		for _, snippet := range unwanted {
			if strings.Contains(string(content), snippet) {
				present = append(present, path+": "+snippet)
			}
		}
	}
	return present
}

// projectedMarkdown is content behind a working-copy projection block.
func projectedMarkdown(content string) string {
	return "<!-- " + acdsl.ProjectionMarker + " 1 rule(s) govern this file — working-copy view, stripped before commit -->\n" +
		"<!-- - [RULE-DEMO-001] demo rule -->\n" +
		"\n" +
		content
}

// provideDeployRoot builds a temp root holding home/ and repo/ with one
// modeled leaf per name under skills/group, each with a notes.md sibling;
// deployOptions sets the context dir ctx.
func provideDeployRoot(t *testing.T, leaves ...string) string {
	t.Helper()
	root := t.TempDir()
	installRulesStub(t, `"title-set@1"`)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "home", ".claude"), 0o755))
	writeDeployFile(t, root, "repo/skills/"+skillgraph.FileVocabulary, "atoms: {}\n", 0o644)
	writeDeployFile(t, root, "repo/skills/"+skillgraph.FileCatalog, deployCatalogCue, 0o644)
	for _, leaf := range leaves {
		leafDir := filepath.Join(root, "repo", "skills", "group", leaf)
		require.NoError(t, skillgraph.Save(leafDir, provideDeploySkill(leaf)))
		writeDeployFile(t, root, "repo/skills/group/"+leaf+"/notes.md", "sibling\n", 0o644)
	}
	return root
}

// provideAcdslContextRoot is a deploy root whose base leaf declares
// acdsl-context.
func provideAcdslContextRoot(t *testing.T) string {
	t.Helper()
	root := provideDeployRoot(t, "base")
	skill := provideDeploySkill("base")
	acdslContext := skillgraph.Field{Key: "acdsl-context", Value: " RULE-*, FACT-*"}
	skill.Frontmatter = append(skill.Frontmatter, acdslContext)
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "base"), skill))
	return root
}

// provideDeploySkill is a small model that passes Check: a preamble naming
// the context dir, two steps joined by a formal clause, a payload, an Eval
// body, an enforce and a title declaration.
func provideDeploySkill(name string) *skillgraph.Skill {
	prefix := "SKILL-" + strings.ToUpper(name)
	return &skillgraph.Skill{
		Changelog: []skillgraph.ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: []skillgraph.Field{
			{Key: "name", Value: " " + name},
			{Key: "description", Value: " " + name + " fixture. Trigger on nothing."},
			{Key: "version", Value: " 1.0"},
			{Key: "format", Value: " 2"},
		},
		Preamble: "# " + name + "\n\nRead $AGENT_CONTEXT_DIR_DEFAULT/rules/plan.md.",
		Sections: []*skillgraph.Section{
			{
				Title: "Routing",
				Blocks: []skillgraph.Block{
					{Routing: &skillgraph.Routing{
						Pipeline:      skillgraph.Pipeline{Standalone: true},
						Preconditions: []string{"a demo request"},
						Purpose:       "demo.",
						Routes:        []skillgraph.Route{{For: "planning", Use: "/fdesign"}},
					}},
				},
			},
			{
				Title: "Flow",
				Blocks: []skillgraph.Block{
					{Entry: &skillgraph.Entry{Id: prefix + "-FLOW-001", Post: []string{"first_done"}, Role: skillgraph.RoleStep, Statement: "RUN the first step.", Title: "First step"}},
					{Entry: &skillgraph.Entry{Id: prefix + "-FLOW-002", Pred: "first_done", Role: skillgraph.RoleStep, Statement: "RUN `$" + skillgraph.VariableName(name, "TITLE_SET") + " title set DESIGN-READY`, then the second step.", Title: "Second step"}},
				},
			},
			{
				Title: "Templates",
				Blocks: []skillgraph.Block{
					{Entry: &skillgraph.Entry{
						Id:        prefix + "-TPL-001",
						Payload:   &skillgraph.Payload{Body: "# Concept\n", Info: "markdown", Marker: "```"},
						Role:      skillgraph.RolePayload,
						Statement: "Overview template.",
						Title:     "Overview template",
					}},
				},
			},
			{
				Title: "Model",
				Blocks: []skillgraph.Block{
					{Model: &skillgraph.Model{Effort: "high", Reason: "demo reasoning", Tier: "frontier"}},
				},
			},
		},
		Atoms:   map[string]*skillgraph.Atom{"first_done": {Sort: "state"}},
		Catalog: provideDeployCatalog(),
		Env:     map[string]*skillgraph.Variable{"TITLE_SET": {Interface: "title-set@1"}},
		Smoke:   &skillgraph.Smoke{Skip: "fixture"},
		Enforce: &skillgraph.Enforce{
			Anchors: map[string]*skillgraph.Anchor{"produced": {Tool: "^Bash$"}, "stage": {Tool: "^Workflow$"}},
			Checks: map[string]*skillgraph.EnforceCheck{
				"producer_first": {After: "produced", Kind: skillgraph.CheckRequires, On: "stage", Reason: "r", Recovery: "x"},
			},
		},
		Title: &sessiontitle.Declaration{
			States: map[string]*sessiontitle.DeclaredState{
				"DESIGN-READY": {
					Entry:   prefix + "-FLOW-002",
					Meaning: "the design waits for approval",
				},
			},
		},
	}
}

// saveRequiredSkill writes a leaf named name whose title-set variable is required.
func saveRequiredSkill(t *testing.T, root, name string) {
	t.Helper()
	skill := provideDeploySkill(name)
	skill.Env["TITLE_SET"].Required = true
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", name), skill))
}

// saveCheckSkill writes a leaf named name whose first step runs the required repo-scoped check command beside the required title-set program.
func saveCheckSkill(t *testing.T, root, name string) {
	t.Helper()
	skill := provideDeploySkill(name)
	skill.Env["TITLE_SET"].Required = true
	skill.Env["CHECK"] = &skillgraph.Variable{Interface: "check@1", Required: true}
	first := skill.Sections[1].Blocks[0].Entry
	first.Statement = "RUN `$" + skillgraph.VariableName(name, "CHECK") + "` first."
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", name), skill))
}

// stashedPaths reports, for each path of expected below root, whether a journal line names it with its stash present under the fixture trash; nil for an empty expectation.
func stashedPaths(root string, expected map[string]bool) map[string]bool {
	if len(expected) == 0 {
		return nil
	}

	stateDir := filepath.Join(root, "home", ".claude", "smine")
	entries, _ := trash.New(stateDir, 0).Tail(1000)
	stashed := map[string]bool{}
	for path := range expected {
		stashed[path] = false
		absolute := filepath.Join(root, filepath.FromSlash(path))
		for _, entry := range entries {
			_, err := os.Lstat(filepath.Join(stateDir, trash.TrashDir, filepath.FromSlash(entry.Stash)))
			isStashed := entry.Path == absolute && err == nil
			stashed[path] = stashed[path] || isStashed
		}
	}
	return stashed
}

// syncFixture runs one sync of the fixture root before the case under test.
func syncFixture(t *testing.T, options *SyncOptions) {
	t.Helper()
	_, err := Sync(options)
	require.NoError(t, err)
}

// writeDeployFile writes content below root, creating its dirs.
func writeDeployFile(t *testing.T, root, path, content string, mode fs.FileMode) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), mode))
	require.NoError(t, os.Chmod(full, mode))
}

func TestSync(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedContents   map[string]string
		_expectedError      string
		_expectedFiles      map[string]bool
		_expectedModes      map[string]fs.FileMode
		_expectedReport     string
		_expectedSnippets   map[string][]string
		_expectedStashed    map[string]bool
		_shouldPass         bool
		_unexpectedReport   []string
		_unexpectedSnippets map[string][]string

		envKeepFormal string
		options       *SyncOptions
		root          string
	}

	t.Setenv(envKeepFormal, "")
	helperBody := "helper agent\n"
	markedHelper := trash.MarkerLine([]byte(helperBody)) + "\n" + helperBody
	withoutContext := []components.ID{components.ConfigServer, components.Skills}
	withoutRules := []components.ID{components.Skills}
	tests := make([]*testCase, 0)

	// nil-selection-deploys-every-leaf
	root := provideDeployRoot(t, "base", "extra")
	tests = append(tests, &testCase{
		_id: "nil-selection-deploys-every-leaf",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base":  true,
			"home/.claude/skills/extra": true,
			"home/.codex/skills/extra":  true,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// chosen-mode-deploys-listed
	root = provideDeployRoot(t, "base", "extra")
	options := deployOptions(root)
	options.Selection = &Selection{Chosen: []string{"base"}, Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id: "chosen-mode-deploys-listed",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base":  true,
			"home/.claude/skills/extra": false,
		},
		_expectedReport: "  skip (not chosen): extra",
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// chosen-unknown-leaf-ignored
	root = provideDeployRoot(t, "base")
	options = deployOptions(root)
	options.Selection = &Selection{Chosen: []string{"base", "nope"}, Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:            "chosen-unknown-leaf-ignored",
		_expectedFiles: map[string]bool{"home/.claude/skills/base": true, "home/.claude/skills/nope": false},
		_shouldPass:    true,

		options: options,
		root:    root,
	})

	// none-mode-writes-nothing
	root = provideDeployRoot(t, "base")
	options = deployOptions(root)
	options.Selection = &Selection{Mode: SelectionModeNone}
	tests = append(tests, &testCase{
		_id:            "none-mode-writes-nothing",
		_expectedFiles: map[string]bool{"home/.claude/skills/base": false, "home/.codex/skills/base": false},
		_shouldPass:    true,

		options: options,
		root:    root,
	})

	// none-mode-prunes-every-marked-dir
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/stale/SKILL.md", "stale\n", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/stale/"+markerFileName, "", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/own/SKILL.md", "mine\n", 0o644)
	options = deployOptions(root)
	options.Prune = true
	options.Selection = &Selection{Mode: SelectionModeNone}
	tests = append(tests, &testCase{
		_id:              "none-mode-prunes-every-marked-dir",
		_expectedFiles:   map[string]bool{"home/.claude/skills/stale": false, "home/.claude/skills/own": true},
		_expectedStashed: map[string]bool{"home/.claude/skills/stale": true},
		_shouldPass:      true,

		options: options,
		root:    root,
	})

	// unknown-mode-fails
	root = provideDeployRoot(t, "base")
	options = deployOptions(root)
	options.Selection = &Selection{Mode: "some"}
	tests = append(tests, &testCase{
		_id:            "unknown-mode-fails",
		_expectedError: `Unknown skills mode "some"`,
		_expectedFiles: map[string]bool{"home/.claude/skills": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// requires-unmet-skipped-with-log
	root = provideDeployRoot(t, "base", "peeky")
	peeky := provideDeploySkill("peeky")
	peeky.Requires = []components.ID{components.Peek, components.Heartbeat}
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "peeky"), peeky))
	options = deployOptions(root)
	options.Components = []components.ID{components.Context, components.Heartbeat}
	tests = append(tests, &testCase{
		_id:             "requires-unmet-skipped-with-log",
		_expectedFiles:  map[string]bool{"home/.claude/skills/base": true, "home/.claude/skills/peeky": false},
		_expectedReport: "  skip peeky unavailable (peek)",
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// required-unset-skipped-unconfigured
	root = provideDeployRoot(t, "base")
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Components = withoutRules
	tests = append(tests, &testCase{
		_id:             "required-unset-skipped-unconfigured",
		_expectedFiles:  map[string]bool{"home/.claude/skills/base": true, "home/.claude/skills/needy": false},
		_expectedReport: "  skip needy unconfigured (SKILL_NEEDY_TITLE_SET)",
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// unconfigured-cause-in-report
	root = provideDeployRoot(t, "base")
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Rows = &Rows{Skills: map[string]string{"SKILL_NEEDY_TITLE_SET": "missing-rules"}}
	tests = append(tests, &testCase{
		_id:             "unconfigured-cause-in-report",
		_expectedFiles:  map[string]bool{"home/.claude/skills/base": true, "home/.claude/skills/needy": false},
		_expectedReport: "  skip needy unconfigured (SKILL_NEEDY_TITLE_SET: missing-rules does not resolve to an executable)",
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// grant-to-unconfigured-leaf-dropped
	root = provideDeployRoot(t, "base")
	saveRequiredSkill(t, root, "needy")
	granting := provideDeploySkill("base")
	grants := skillgraph.Field{Key: "allowed-tools", Value: " Bash(ls *), Skill(needy)"}
	granting.Frontmatter = append(granting.Frontmatter, grants)
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "base"), granting))
	options = deployOptions(root)
	options.Components = withoutRules
	tests = append(tests, &testCase{
		_id: "grant-to-unconfigured-leaf-dropped",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"Bash(ls *)"},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"Skill(needy)"},
		},

		options: options,
		root:    root,
	})

	// required-set-from-env-deployed
	root = provideDeployRoot(t)
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Components = withoutRules
	options.Rows = &Rows{Skills: map[string]string{"SKILL_NEEDY_TITLE_SET": "my-rules"}}
	tests = append(tests, &testCase{
		_id: "required-set-from-env-deployed",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/needy/SKILL.md": {"RUN `my-rules title set DESIGN-READY`"},
		},
		_shouldPass: true,

		options: options,
		root:    root,
	})

	// sibling-workflow-and-agent-filled
	root = provideDeployRoot(t)
	saveRequiredSkill(t, root, "needy")
	writeDeployFile(t, root, "repo/skills/group/needy/workflows/x.js", "run('$SKILL_NEEDY_TITLE_SET check')\n", 0o644)
	writeDeployFile(t, root, "repo/skills/group/needy/agents/x.md", "RUN `$SKILL_NEEDY_TITLE_SET lanes`.\n", 0o644)
	filledAgent := "RUN `smine-rules lanes`.\n"
	markedFilledAgent := trash.MarkerLine([]byte(filledAgent)) + "\n" + filledAgent
	tests = append(tests, &testCase{
		_id: "sibling-workflow-and-agent-filled",
		_expectedContents: map[string]string{
			"home/.claude/agents/x.md":                 markedFilledAgent,
			"home/.claude/skills/needy/workflows/x.js": "run('smine-rules check')\n",
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// claude-home-placeholder-codex-home-filled
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	options = deployOptions(root)
	options.Rows = &Rows{Skills: map[string]string{"SKILL_CHECKY_CHECK": "my-rules audit"}}
	tests = append(tests, &testCase{
		_id: "claude-home-placeholder-codex-home-filled",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/checky/SKILL.md": {
				"```!\n",
				`if (ENVIRON["SKILL_CHECKY_CHECK"] == "") { print "SKILL_CHECKY_CHECK: not set - runs everything before a commit, e.g. go vet ./... - set it for this repo on the skill page, or export it"; exit 2 }; `,
			},
			"home/.codex/skills/checky/SKILL.md": {"RUN `my-rules audit` first."},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/checky/SKILL.md": {"my-rules audit"},
			"home/.codex/skills/checky/SKILL.md":  {"SKILL_CHECKY_CHECK"},
		},

		options: options,
		root:    root,
	})

	// env-json-beside-placeholder
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	options = deployOptions(root)
	options.Rows = &Rows{Skills: map[string]string{"SKILL_CHECKY_CHECK": "my-rules audit"}}
	tests = append(tests, &testCase{
		_id: "env-json-beside-placeholder",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/checky/" + skillenv.FileName: true,
			"home/.codex/skills/checky/SKILL.md":              true,
			"home/.codex/skills/checky/" + skillenv.FileName:  false,
		},
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/checky/" + skillenv.FileName: {`"name": "SKILL_CHECKY_CHECK"`, `"interface": "check"`, `"required": true`},
		},
		_shouldPass: true,

		options: options,
		root:    root,
	})

	// codex-only-skip-logged
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	tests = append(tests, &testCase{
		_id: "codex-only-skip-logged",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/checky/SKILL.md": true,
			"home/.codex/skills/checky":           false,
		},
		_expectedReport: "  skip checky unconfigured (SKILL_CHECKY_CHECK) in " + filepath.Join(root, "home", ".codex", "skills"),
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// deferred-required-not-skipped
	root = provideDeployRoot(t, "base")
	saveCheckSkill(t, root, "checky")
	granting = provideDeploySkill("base")
	grants = skillgraph.Field{Key: "allowed-tools", Value: " Bash(ls *), Skill(checky)"}
	granting.Frontmatter = append(granting.Frontmatter, grants)
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "base"), granting))
	tests = append(tests, &testCase{
		_id:            "deferred-required-not-skipped",
		_expectedFiles: map[string]bool{"home/.claude/skills/checky/SKILL.md": true},
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"Skill(checky)"},
		},
		_shouldPass:       true,
		_unexpectedReport: []string{"  skip checky unconfigured (SKILL_CHECKY_CHECK)\n"},

		options: deployOptions(root),
		root:    root,
	})

	// agents-filled-without-deferred
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	writeDeployFile(t, root, "repo/skills/group/checky/agents/x.md", "RUN `$SKILL_CHECKY_TITLE_SET lanes`.\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "agents-filled-without-deferred",
		_expectedContents: map[string]string{"home/.claude/agents/x.md": markedFilledAgent},
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// marker-written-both-homes
	root = provideDeployRoot(t, "base")
	baseHash, err := sourceHash(filepath.Join(root, "repo", "skills", "group", "base"))
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id: "marker-written-both-homes",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/" + markerFileName: {markerSourcePrefix + baseHash + "\n" + markerRenderedPrefix},
			"home/.codex/skills/base/" + markerFileName:  {markerSourcePrefix + baseHash + "\n" + markerRenderedPrefix},
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// marker-holds-rendered-line
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	renderedLine, err := deployedHash(filepath.Join(root, "home", ".claude", "skills", "base"))
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id: "marker-holds-rendered-line",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/" + markerFileName: {"\n" + markerRenderedPrefix + renderedLine + "\n"},
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// unchanged-dir-rewritten-without-stash
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	syncFixture(t, deployOptions(root))
	tests = append(tests, &testCase{
		_id:               "unchanged-dir-rewritten-without-stash",
		_expectedFiles:    map[string]bool{"home/.claude/smine/trash": false},
		_shouldPass:       true,
		_unexpectedReport: []string{"previous copy stashed"},

		options: deployOptions(root),
		root:    root,
	})

	// unmarked-dir-under-deployed-name-stashed
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/base/mine.md", "mine\n", 0o644)
	tests = append(tests, &testCase{
		_id:              "unmarked-dir-under-deployed-name-stashed",
		_expectedFiles:   map[string]bool{"home/.claude/skills/base/mine.md": false, "home/.claude/skills/base/SKILL.md": true},
		_expectedReport:  filepath.Join(root, "home", ".claude", "skills", "base") + ": unmarked dir under a deployed name, previous copy stashed at ",
		_expectedStashed: map[string]bool{"home/.claude/skills/base": true},
		_shouldPass:      true,

		options: deployOptions(root),
		root:    root,
	})

	// locally-modified-dir-stashed
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	writeDeployFile(t, root, "home/.claude/skills/base/notes.md", "edited\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "locally-modified-dir-stashed",
		_expectedContents: map[string]string{"home/.claude/skills/base/notes.md": "sibling\n"},
		_expectedReport:   filepath.Join(root, "home", ".claude", "skills", "base") + ": locally modified, previous copy stashed at ",
		_expectedStashed:  map[string]bool{"home/.claude/skills/base": true},
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// prune-removes-marked-unwanted
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/stale/SKILL.md", "stale\n", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/stale/"+markerFileName, "", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id:              "prune-removes-marked-unwanted",
		_expectedFiles:   map[string]bool{"home/.claude/skills/stale": false},
		_expectedReport:  "  pruned: " + filepath.Join(root, "home", ".claude", "skills", "stale") + " -> trash ",
		_expectedStashed: map[string]bool{"home/.claude/skills/stale": true},
		_shouldPass:      true,

		options: options,
		root:    root,
	})

	// prune-keeps-unmarked-foreign
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/foreign/SKILL.md", "foreign\n", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id:             "prune-keeps-unmarked-foreign",
		_expectedFiles:  map[string]bool{"home/.claude/skills/foreign": true},
		_expectedReport: "  kept (not deployed by smine): " + filepath.Join(root, "home", ".claude", "skills", "foreign"),
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// no-prune-keeps-marked
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/stale/SKILL.md", "stale\n", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/stale/"+markerFileName, "", 0o644)
	tests = append(tests, &testCase{
		_id:             "no-prune-keeps-marked",
		_expectedFiles:  map[string]bool{"home/.claude/skills/stale": true},
		_expectedReport: "  kept (not deployed, prune removes it): " + filepath.Join(root, "home", ".claude", "skills", "stale"),
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// missing-codex-home-no-error
	root = provideDeployRoot(t, "base")
	options = deployOptions(root)
	options.Prune = true
	options.Selection = &Selection{Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:            "missing-codex-home-no-error",
		_expectedFiles: map[string]bool{"home/.codex/skills": false},
		_shouldPass:    true,

		options: options,
		root:    root,
	})

	// agents-deployed-with-owner
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", projectedMarkdown("helper agent\n"), 0o644)
	tests = append(tests, &testCase{
		_id:               "agents-deployed-with-owner",
		_expectedContents: map[string]string{"home/.claude/agents/helper.md": markedHelper},
		_expectedReport:   "  agent helper.md -> " + filepath.Join(root, "home", ".claude", "agents"),
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// agents-adopted-without-marker
	root = provideDeployRoot(t, "base")
	frontMatterAgent := "---\nname: helper\n---\nhelper agent\n"
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", frontMatterAgent, 0o644)
	writeDeployFile(t, root, "home/.claude/agents/helper.md", frontMatterAgent, 0o644)
	tests = append(tests, &testCase{
		_id: "agents-adopted-without-marker",
		_expectedContents: map[string]string{
			"home/.claude/agents/helper.md": "---\n" + trash.MarkerLine([]byte(frontMatterAgent)) + "\nname: helper\n---\nhelper agent\n",
		},
		_expectedReport: "  agent helper.md -> " + filepath.Join(root, "home", ".claude", "agents") + " (adopted, marker added)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// agents-foreign-kept
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", helperBody, 0o644)
	writeDeployFile(t, root, "home/.claude/agents/helper.md", "my own helper\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "agents-foreign-kept",
		_expectedContents: map[string]string{"home/.claude/agents/helper.md": "my own helper\n"},
		_expectedReport:   "  agent " + filepath.Join(root, "home", ".claude", "agents", "helper.md") + " kept: no smine marker",
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// agents-removed-under-prune
	root = provideDeployRoot(t, "base", "extra")
	writeDeployFile(t, root, "repo/skills/group/extra/agents/helper.md", helperBody, 0o644)
	writeDeployFile(t, root, "home/.claude/agents/helper.md", markedHelper, 0o644)
	options = deployOptions(root)
	options.Prune = true
	options.Selection = &Selection{Chosen: []string{"base"}, Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:              "agents-removed-under-prune",
		_expectedFiles:   map[string]bool{"home/.claude/agents/helper.md": false},
		_expectedReport:  "  agent removed: " + filepath.Join(root, "home", ".claude", "agents", "helper.md") + " -> trash",
		_expectedStashed: map[string]bool{"home/.claude/agents/helper.md": true},
		_shouldPass:      true,

		options: options,
		root:    root,
	})

	// agents-unmarked-kept-under-prune
	root = provideDeployRoot(t, "base", "extra")
	writeDeployFile(t, root, "repo/skills/group/extra/agents/helper.md", helperBody, 0o644)
	writeDeployFile(t, root, "home/.claude/agents/helper.md", helperBody, 0o644)
	options = deployOptions(root)
	options.Prune = true
	options.Selection = &Selection{Chosen: []string{"base"}, Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:             "agents-unmarked-kept-under-prune",
		_expectedFiles:  map[string]bool{"home/.claude/agents/helper.md": true},
		_expectedReport: "  agent " + filepath.Join(root, "home", ".claude", "agents", "helper.md") + " kept: no smine marker",
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// agents-kept-without-prune
	root = provideDeployRoot(t, "base", "extra")
	writeDeployFile(t, root, "repo/skills/group/extra/agents/helper.md", "helper agent\n", 0o644)
	writeDeployFile(t, root, "home/.claude/agents/helper.md", "helper agent\n", 0o644)
	options = deployOptions(root)
	options.Selection = &Selection{Chosen: []string{"base"}, Mode: SelectionModeChosen}
	tests = append(tests, &testCase{
		_id:            "agents-kept-without-prune",
		_expectedFiles: map[string]bool{"home/.claude/agents/helper.md": true},
		_shouldPass:    true,

		options: options,
		root:    root,
	})

	// agents-dir-not-in-skill-dir
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", "helper agent\n", 0o644)
	tests = append(tests, &testCase{
		_id: "agents-dir-not-in-skill-dir",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/agents": false,
			"home/.codex/skills/base/agents":  false,
			"home/.codex/agents":              false,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// acdsl-context-kept-without-context
	root = provideAcdslContextRoot(t)
	options = deployOptions(root)
	options.Components = withoutContext
	tests = append(tests, &testCase{
		_id:            "acdsl-context-kept-without-context",
		_expectedFiles: map[string]bool{"home/.claude/skills/base/SKILL.md": true},
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"\nacdsl-context: RULE-*, FACT-*\n"},
		},
		_shouldPass: true,

		options: options,
		root:    root,
	})

	// broken-model-fails
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/broken/broken.cue", "not cue {\n", 0o644)
	tests = append(tests, &testCase{
		_id:            "broken-model-fails",
		_expectedError: "deployRun.repoLeaves",
		_expectedFiles: map[string]bool{"home/.claude/skills/base": false, "home/.claude/skills/broken": false},
		_shouldPass:    false,

		options: deployOptions(root),
		root:    root,
	})

	// stale-tool-scripts-stashed
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/agents/tools/remove_agent_worktrees.sh", "#!/usr/bin/env bash\n", 0o755)
	writeDeployFile(t, root, "home/.claude/agents/tools/_lib/verdict.sh", "#!/usr/bin/env bash\n", 0o755)
	tests = append(tests, &testCase{
		_id: "stale-tool-scripts-stashed",
		_expectedFiles: map[string]bool{
			"home/.claude/agents/tools/_lib/verdict.sh":           false,
			"home/.claude/agents/tools/remove_agent_worktrees.sh": false,
		},
		_expectedStashed: map[string]bool{
			"home/.claude/agents/tools/_lib/verdict.sh":           true,
			"home/.claude/agents/tools/remove_agent_worktrees.sh": true,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// no-model-or-payloads-deployed
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/eval/case.md", "eval\n", 0o644)
	tests = append(tests, &testCase{
		_id: "no-model-or-payloads-deployed",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/SKILL.md": true,
			"home/.claude/skills/base/eval":     false,
			"home/.claude/skills/base/base.cue": false,
			"home/.claude/skills/base/payloads": false,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// declarations-deployed
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id: "declarations-deployed",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/enforce.json": {`"producer_first"`},
			"home/.claude/skills/base/title.json":   {`"DESIGN-READY"`},
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// help-test-deployed
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/smoke/tree/notes/plan.md", "plan\n", 0o644)
	writeDeployFile(t, root, "repo/skills/group/base/eval/definition.json", "{}\n", 0o644)
	tests = append(tests, &testCase{
		_id: "help-test-deployed",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/eval":  false,
			"home/.claude/skills/base/smoke": false,
		},
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/help-test.md": {"Smoke: skipped — fixture", "Eval: /skillroutine-eval base"},
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// context-dir-substituted
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id:             "context-dir-substituted",
		_expectedReport: "using context dir: ctx",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"Read ctx/rules/plan.md."},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {contextDirToken, "{pred: first_done}"},
		},

		options: deployOptions(root),
		root:    root,
	})

	// context-dir-default-when-unset
	root = provideDeployRoot(t, "base")
	options = deployOptions(root)
	options.ContextDir = ""
	tests = append(tests, &testCase{
		_id:             "context-dir-default-when-unset",
		_expectedReport: "using context dir: docs",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"Read docs/rules/plan.md."},
		},
		_shouldPass: true,

		options: options,
		root:    root,
	})

	// sibling-mode-kept
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/scripts/run.sh", "#!/bin/sh\n", 0o755)
	tests = append(tests, &testCase{
		_id: "sibling-mode-kept",
		_expectedModes: map[string]fs.FileMode{
			"home/.claude/skills/base/notes.md":       0o644,
			"home/.claude/skills/base/scripts/run.sh": 0o755,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// projection-block-stripped
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/notes.md", projectedMarkdown("sibling\n"), 0o644)
	tests = append(tests, &testCase{
		_id:               "projection-block-stripped",
		_expectedContents: map[string]string{"home/.claude/skills/base/notes.md": "sibling\n"},
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// deleted-sibling-pruned
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/old/gone.md", "old\n", 0o644)
	syncFixture(t, deployOptions(root))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "repo", "skills", "group", "base", "old")))
	tests = append(tests, &testCase{
		_id: "deleted-sibling-pruned",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/notes.md": true,
			"home/.claude/skills/base/old":      false,
		},
		_expectedStashed: map[string]bool{"home/.claude/skills/base/old/gone.md": true},
		_shouldPass:      true,

		options: deployOptions(root),
		root:    root,
	})

	// version-note-new
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id:             "version-note-new",
		_expectedReport: "  base -> " + filepath.Join(root, "home", ".claude", "skills", "base") + " (new v1.0)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// version-note-unchanged
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/base/SKILL.md", "---\nname: base\nversion: 1.0\n---\n", 0o644)
	tests = append(tests, &testCase{
		_id:             "version-note-unchanged",
		_expectedReport: " (unchanged v1.0)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// version-note-bumped
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/base/SKILL.md", "---\nname: base\nversion: 0.9\n---\n", 0o644)
	tests = append(tests, &testCase{
		_id:             "version-note-bumped",
		_expectedReport: " (bumped v0.9 -> v1.0)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// skip-list-kept
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.codex/skills/codex-primary-runtime/SKILL.md", "runtime\n", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id:             "skip-list-kept",
		_expectedFiles:  map[string]bool{"home/.codex/skills/codex-primary-runtime": true},
		_expectedReport: "  skipped (prune skip-list): " + filepath.Join(root, "home", ".codex", "skills", "codex-primary-runtime"),
		_shouldPass:     true,

		options: options,
		root:    root,
	})

	// symlinked-root-refused
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "linked-skills/foreign/SKILL.md", "foreign\n", 0o644)
	writeDeployFile(t, root, "linked-skills/foreign/"+markerFileName, "", 0o644)
	require.NoError(t, os.Symlink(filepath.Join(root, "linked-skills"), filepath.Join(root, "home", ".claude", "skills")))
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id:            "symlinked-root-refused",
		_expectedError: "Refusing to prune under the symlinked root",
		_expectedFiles: map[string]bool{"linked-skills/foreign/SKILL.md": true},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// empty-skills-root
	root = provideDeployRoot(t)
	tests = append(tests, &testCase{
		_id:            "empty-skills-root",
		_expectedError: "No skill model file found under " + filepath.Join(root, "repo", "skills"),
		_expectedFiles: map[string]bool{"home/.claude/skills": false},
		_shouldPass:    false,

		options: deployOptions(root),
		root:    root,
	})

	// keep-formal-canonical
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id: "keep-formal-canonical",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"{pred: first_done}", "Read ctx/rules/plan.md."},
		},
		_shouldPass: true,

		envKeepFormal: "1",
		options:       deployOptions(root),
		root:          root,
	})

	// both-homes-written
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id: "both-homes-written",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/SKILL.md": true,
			"home/.codex/skills/base/SKILL.md":  true,
		},
		_expectedReport: "synced to " + filepath.Join(root, "home", ".codex", "skills"),
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// alias-copy-written
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id: "alias-copy-written",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/pk/SKILL.md": true,
			"home/.codex/skills/pk/SKILL.md":  true,
		},
		_expectedReport: "  pk -> " + filepath.Join(root, "home", ".claude", "skills", "pk") + " (alias of base)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// alias-name-and-title-code
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id: "alias-name-and-title-code",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/pk/SKILL.md": {"\nname: pk\n", "\ntitle-code: base\n"},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/pk/SKILL.md": {"\nname: base\n"},
		},

		options: deployOptions(root),
		root:    root,
	})

	// alias-keeps-declared-title-code
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "BS")
	tests = append(tests, &testCase{
		_id: "alias-keeps-declared-title-code",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/pk/SKILL.md": {"\ntitle-code: BS\n"},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/pk/SKILL.md": {"title-code: base"},
		},

		options: deployOptions(root),
		root:    root,
	})

	// alias-skill-manifest-unchanged
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id: "alias-skill-manifest-unchanged",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"\nname: base\n", "\naliases: pk\n"},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"home/.claude/skills/base/SKILL.md": {"title-code: base"},
		},

		options: deployOptions(root),
		root:    root,
	})

	// two-aliases
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "p, pk", "")
	tests = append(tests, &testCase{
		_id: "two-aliases",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/p/SKILL.md":  true,
			"home/.claude/skills/pk/SKILL.md": true,
		},
		_shouldPass: true,

		options: deployOptions(root),
		root:    root,
	})

	// no-alias-no-copy
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id:               "no-alias-no-copy",
		_expectedFiles:    map[string]bool{"home/.claude/skills/pk": false},
		_shouldPass:       true,
		_unexpectedReport: []string{"alias of"},

		options: deployOptions(root),
		root:    root,
	})

	// alias-kept-under-prune
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	writeDeployFile(t, root, "home/.claude/skills/foreign/SKILL.md", "foreign\n", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/foreign/"+markerFileName, "", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id: "alias-kept-under-prune",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/foreign":     false,
			"home/.claude/skills/pk/SKILL.md": true,
		},
		_shouldPass: true,

		options: options,
		root:    root,
	})

	// alias-not-listed-as-foreign
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id:               "alias-not-listed-as-foreign",
		_shouldPass:       true,
		_unexpectedReport: []string{"  kept (not deployed, prune removes it): " + filepath.Join(root, "home", ".claude", "skills", "pk")},

		options: deployOptions(root),
		root:    root,
	})

	// removed-alias-pruned
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/pk/SKILL.md", "---\nname: pk\n---\n", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/pk/"+markerFileName, "", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id:              "removed-alias-pruned",
		_expectedFiles:   map[string]bool{"home/.claude/skills/pk": false},
		_expectedReport:  "  pruned: " + filepath.Join(root, "home", ".claude", "skills", "pk") + " -> trash ",
		_expectedStashed: map[string]bool{"home/.claude/skills/pk": true},
		_shouldPass:      true,

		options: options,
		root:    root,
	})

	// alias-stale-file-removed
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	writeDeployFile(t, root, "repo/skills/group/base/stale.md", "stale\n", 0o644)
	syncFixture(t, deployOptions(root))
	require.NoError(t, os.Remove(filepath.Join(root, "repo", "skills", "group", "base", "stale.md")))
	tests = append(tests, &testCase{
		_id: "alias-stale-file-removed",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/pk/SKILL.md": true,
			"home/.claude/skills/pk/stale.md": false,
		},
		_expectedStashed: map[string]bool{"home/.claude/skills/pk/stale.md": true},
		_shouldPass:      true,

		options: deployOptions(root),
		root:    root,
	})

	// alias-second-run-identical
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	syncFixture(t, deployOptions(root))
	firstRun := fileContents(root, map[string]string{"home/.claude/skills/pk/SKILL.md": ""})
	tests = append(tests, &testCase{
		_id:               "alias-second-run-identical",
		_expectedContents: firstRun,
		_shouldPass:       true,
		_unexpectedReport: []string{"previous copy stashed"},

		options: deployOptions(root),
		root:    root,
	})

	// alias-keep-formal
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id: "alias-keep-formal",
		_expectedSnippets: map[string][]string{
			"home/.claude/skills/pk/SKILL.md": {"\nname: pk\n", "{pred: first_done}"},
		},
		_shouldPass: true,

		envKeepFormal: "1",
		options:       deployOptions(root),
		root:          root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(envKeepFormal, test.envKeepFormal)
			report, err := Sync(test.options)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Contains(t, report, test._expectedReport)
			assert.Empty(t, presentInReport(report, test._unexpectedReport))
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
			assert.Equal(t, test._expectedContents, fileContents(test.root, test._expectedContents))
			assert.Equal(t, test._expectedModes, fileModes(test.root, test._expectedModes))
			assert.Empty(t, missingSnippets(test.root, test._expectedSnippets))
			assert.Empty(t, presentSnippets(test.root, test._unexpectedSnippets))
			assert.Equal(t, test._expectedStashed, stashedPaths(test.root, test._expectedStashed))
		})
	}
}

func TestSync_AliasFilesIdentical(t *testing.T) {
	t.Setenv(envKeepFormal, "")
	root := provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	syncFixture(t, deployOptions(root))

	skillsDir := filepath.Join(root, "home", ".claude", "skills")
	for _, name := range []string{"enforce.json", "help-test.md", "notes.md", "title.json"} {
		skillContent, err := os.ReadFile(filepath.Join(skillsDir, "base", name))
		require.NoError(t, err)
		aliasContent, err := os.ReadFile(filepath.Join(skillsDir, "pk", name))
		require.NoError(t, err)
		assert.Equalf(t, skillContent, aliasContent, "%s differs between base and its alias pk", name)
	}

	skillMarker, err := os.ReadFile(filepath.Join(skillsDir, "base", markerFileName))
	require.NoError(t, err)
	aliasMarker, err := os.ReadFile(filepath.Join(skillsDir, "pk", markerFileName))
	require.NoError(t, err)
	skillSource, _, _ := strings.Cut(string(skillMarker), "\n")
	aliasSource, _, _ := strings.Cut(string(aliasMarker), "\n")
	aliasHash, err := deployedHash(filepath.Join(skillsDir, "pk"))
	require.NoError(t, err)
	assert.Equal(t, skillSource, aliasSource, "source-line-shared")
	assert.Contains(t, string(aliasMarker), markerRenderedPrefix+aliasHash+"\n", "alias-rendered-line-own")
}

func TestSync_Overrides(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedContents map[string]string
		_expectedFiles    map[string]bool
		_expectedReport   string
		_expectedStashed  map[string]bool
		_shouldPass       bool

		options *SyncOptions
		root    string
	}

	t.Setenv(envKeepFormal, "")
	casualProfile := "---\nlanguage: de\naudience: casual\n---\n"
	profilePath := "home/.claude/context/global/presentation-profile.md"
	settingsPath := "home/.claude/settings.json"
	overlayPath := "home/.config/claude-routine/skill-overrides.json"
	repoLocalPath := "repo/.claude/settings.local.json"
	tests := make([]*testCase, 0)

	// casual-writes-overlay-only
	root := provideDeployRoot(t, "demo")
	declareAliases(t, root, "demo", "pk", "")
	writeDeployFile(t, root, profilePath, casualProfile, 0o644)
	writeDeployFile(t, root, settingsPath, `{"model": "opus"}`, 0o644)
	tests = append(tests, &testCase{
		_id: "casual-writes-overlay-only",
		_expectedContents: map[string]string{
			overlayPath:  "{\n  \"skillOverrides\": {\n    \"demo\": \"on\",\n    \"pk\": \"on\"\n  }\n}\n",
			settingsPath: `{"model": "opus"}`,
		},
		_expectedFiles:  map[string]bool{repoLocalPath: false},
		_expectedReport: "  routine overlay -> " + filepath.Join(root, filepath.FromSlash(overlayPath)) + " (2 leaves; hide them in your sessions from the Skills page)",
		_shouldPass:     true,

		options: deployOptions(root),
		root:    root,
	})

	// developer-removes-overlay-only
	root = provideDeployRoot(t, "demo")
	writeDeployFile(t, root, profilePath, casualProfile, 0o644)
	writeDeployFile(t, root, settingsPath, `{"model": "opus", "skillOverrides": {"demo": "off"}}`, 0o644)
	syncFixture(t, deployOptions(root))
	writeDeployFile(t, root, profilePath, "---\nlanguage: en\naudience: developer\n---\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "developer-removes-overlay-only",
		_expectedContents: map[string]string{settingsPath: `{"model": "opus", "skillOverrides": {"demo": "off"}}`},
		_expectedFiles:    map[string]bool{overlayPath: false, repoLocalPath: false},
		_shouldPass:       true,

		options: deployOptions(root),
		root:    root,
	})

	// legacy-non-developer-migrated
	root = provideDeployRoot(t, "demo")
	writeDeployFile(t, root, profilePath, "---\nlanguage: de\naudience: non-developer\n---\n", 0o644)
	writeDeployFile(t, root, settingsPath, `{"model": "opus"}`, 0o644)
	tests = append(tests, &testCase{
		_id: "legacy-non-developer-migrated",
		_expectedContents: map[string]string{
			profilePath:  casualProfile,
			settingsPath: `{"model": "opus"}`,
		},
		_expectedStashed: map[string]bool{profilePath: true},
		_shouldPass:      true,

		options: deployOptions(root),
		root:    root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := Sync(test.options)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, report, test._expectedReport)
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
			assert.Equal(t, test._expectedContents, fileContents(test.root, test._expectedContents))
			assert.Equal(t, test._expectedStashed, stashedPaths(test.root, test._expectedStashed))
		})
	}
}

func TestSetOverrides(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedStashed map[string]bool

		leaves []string
		path   string
		root   string
		store  *trash.Store
		value  string
	}

	settingsPath := "home/.claude/settings.json"
	tests := make([]*testCase, 0)

	// off-sets-overrides
	root := t.TempDir()
	writeDeployFile(t, root, settingsPath, `{"model": "opus"}`, 0o644)
	tests = append(tests, &testCase{
		_id:              "off-sets-overrides",
		_expectedContent: "{\n  \"model\": \"opus\",\n  \"skillOverrides\": {\n    \"demo\": \"off\",\n    \"pk\": \"off\"\n  }\n}\n",
		_expectedStashed: map[string]bool{settingsPath: true},

		leaves: []string{"demo", "pk"},
		path:   filepath.Join(root, filepath.FromSlash(settingsPath)),
		root:   root,
		store:  trash.New(filepath.Join(root, "home", ".claude", "smine"), 0),
		value:  "off",
	})

	// empty-clears-overrides
	root = t.TempDir()
	writeDeployFile(t, root, settingsPath, `{"model": "opus", "skillOverrides": {"demo": "off", "foreign": "on"}}`, 0o644)
	tests = append(tests, &testCase{
		_id:              "empty-clears-overrides",
		_expectedContent: "{\n  \"model\": \"opus\",\n  \"skillOverrides\": {\n    \"foreign\": \"on\"\n  }\n}\n",
		_expectedStashed: map[string]bool{settingsPath: true},

		leaves: []string{"demo"},
		path:   filepath.Join(root, filepath.FromSlash(settingsPath)),
		root:   root,
		store:  trash.New(filepath.Join(root, "home", ".claude", "smine"), 0),
		value:  "",
	})

	// missing-file-writes-nothing
	root = t.TempDir()
	tests = append(tests, &testCase{
		_id:              "missing-file-writes-nothing",
		_expectedContent: absentContent,
		_expectedStashed: map[string]bool{settingsPath: false},

		leaves: []string{"demo"},
		path:   filepath.Join(root, filepath.FromSlash(settingsPath)),
		root:   root,
		store:  trash.New(filepath.Join(root, "home", ".claude", "smine"), 0),
		value:  "off",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := SetOverrides(test.leaves, test.path, test.value, test.store)

			assert.NoError(t, err)
			assert.Equal(t, test._expectedContent, fileContents(test.root, map[string]string{settingsPath: ""})[settingsPath])
			assert.Equal(t, test._expectedStashed, stashedPaths(test.root, test._expectedStashed))
		})
	}
}

func TestSync_Variant(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedError      string
		_expectedFiles      map[string]bool
		_expectedSnippets   map[string][]string
		_expectedStashed    map[string]bool
		_shouldPass         bool
		_unexpectedSnippets map[string][]string

		options *SyncOptions
		root    string
	}

	t.Setenv(envKeepFormal, "")
	tests := make([]*testCase, 0)

	// renders-under-suffixed-name
	root := provideDeployRoot(t, "demo")
	options := deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo=no-tpl:SKILL-DEMO-TPL-*"
	tests = append(tests, &testCase{
		_id: "renders-under-suffixed-name",
		_expectedFiles: map[string]bool{
			"variants/demo--no-tpl/demo.cue":     false,
			"variants/demo--no-tpl/help-test.md": true,
			"variants/demo--no-tpl/notes.md":     true,
			"variants/demo--no-tpl/payloads":     false,
		},
		_expectedSnippets: map[string][]string{
			"variants/demo--no-tpl/SKILL.md": {"\nname: demo--no-tpl\n", "SKILL-DEMO-FLOW-001", "Read ctx/rules/plan.md."},
		},
		_shouldPass: true,
		_unexpectedSnippets: map[string][]string{
			"variants/demo--no-tpl/SKILL.md": {"SKILL-DEMO-TPL-001", "```markdown", "## Templates"},
		},

		options: options,
		root:    root,
	})

	// bad-name
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo=No_Tpl:SKILL-DEMO-TPL-*"
	tests = append(tests, &testCase{
		_id:            "bad-name",
		_expectedError: "Variant name must be kebab-case",
		_expectedFiles: map[string]bool{"variants": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// name-too-long
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo=" + strings.Repeat("a", maxVariantNameLen+1) + ":SKILL-DEMO-TPL-*"
	tests = append(tests, &testCase{
		_id:            "name-too-long",
		_expectedError: "Variant name longer than 24 chars",
		_expectedFiles: map[string]bool{"variants": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// spec-without-separators
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo"
	tests = append(tests, &testCase{
		_id:            "spec-without-separators",
		_expectedError: "is not <leaf>=<name>:<disable>",
		_expectedFiles: map[string]bool{"variants": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// unknown-leaf
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "nope=x:SKILL-NOPE-A-001"
	tests = append(tests, &testCase{
		_id:            "unknown-leaf",
		_expectedError: "No skill leaf named nope",
		_expectedFiles: map[string]bool{"variants": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// unconfigured-variant-refused
	root = provideDeployRoot(t, "demo")
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Components = []components.ID{components.Skills}
	options.Into = filepath.Join(root, "variants")
	options.Variant = "needy=lean:"
	tests = append(tests, &testCase{
		_id:            "unconfigured-variant-refused",
		_expectedError: "Skill needy unconfigured (SKILL_NEEDY_TITLE_SET)",
		_expectedFiles: map[string]bool{"variants": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// unknown-id-fails
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo=bad:SKILL-DEMO-NOPE-001"
	tests = append(tests, &testCase{
		_id:            "unknown-id-fails",
		_expectedError: "matches no entry",
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// unknown-id-leaves-no-dir
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Into = filepath.Join(root, "variants")
	options.Variant = "demo=bad:SKILL-DEMO-NOPE-001"
	tests = append(tests, &testCase{
		_id:            "unknown-id-leaves-no-dir",
		_expectedFiles: map[string]bool{"variants/demo--bad": false},
		_shouldPass:    false,

		options: options,
		root:    root,
	})

	// into-root
	root = provideDeployRoot(t, "demo")
	options = deployOptions(root)
	options.Variant = "demo=lean:SKILL-DEMO-TPL-001"
	tests = append(tests, &testCase{
		_id:            "into-root",
		_expectedFiles: map[string]bool{"home/.claude/skills/demo--lean/SKILL.md": true, "home/.codex/skills/demo--lean": false},
		_shouldPass:    true,

		options: options,
		root:    root,
	})

	// prune-removes-variant-leftovers
	root = provideDeployRoot(t, "demo")
	writeDeployFile(t, root, "home/.claude/skills/demo--old/SKILL.md", "", 0o644)
	writeDeployFile(t, root, "home/.claude/skills/demo--old/"+markerFileName, "", 0o644)
	options = deployOptions(root)
	options.Prune = true
	tests = append(tests, &testCase{
		_id: "prune-removes-variant-leftovers",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/demo--old":         false,
			"home/.claude/skills/demo/SKILL.md":     true,
			"home/.claude/skills/demo/payloads":     false,
			"home/.claude/skills/demo/demo.cue":     false,
			"home/.codex/skills/demo/SKILL.md":      true,
			"home/.claude/skills/demo/help-test.md": true,
			"home/.claude/skills/demo/enforce.json": true,
			"home/.claude/skills/demo/title.json":   true,
		},
		_expectedStashed: map[string]bool{"home/.claude/skills/demo--old": true},
		_shouldPass:      true,

		options: options,
		root:    root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := Sync(test.options)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
			assert.Empty(t, missingSnippets(test.root, test._expectedSnippets))
			assert.Empty(t, presentSnippets(test.root, test._unexpectedSnippets))
			assert.Equal(t, test._expectedStashed, stashedPaths(test.root, test._expectedStashed))
		})
	}
}

func TestDeployLeaf(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedError  string
		_expectedFiles  map[string]bool
		_expectedReport string
		_shouldPass     bool

		leaf    string
		options *SyncOptions
		root    string
	}

	tests := make([]*testCase, 0)

	// deploys-both-homes
	root := provideDeployRoot(t, "base", "extra")
	tests = append(tests, &testCase{
		_id: "deploys-both-homes",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/SKILL.md": true,
			"home/.codex/skills/base/SKILL.md":  true,
			"home/.claude/skills/extra":         false,
		},
		_expectedReport: "  base -> " + filepath.Join(root, "home", ".codex", "skills", "base") + " (new v1.0)",
		_shouldPass:     true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// deploys-agents
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", "helper agent\n", 0o644)
	tests = append(tests, &testCase{
		_id:             "deploys-agents",
		_expectedFiles:  map[string]bool{"home/.claude/agents/helper.md": true, "home/.claude/skills/base/agents": false},
		_expectedReport: "  agent helper.md -> " + filepath.Join(root, "home", ".claude", "agents"),
		_shouldPass:     true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// unavailable-refused
	root = provideDeployRoot(t, "base")
	requiring := provideDeploySkill("base")
	requiring.Requires = []components.ID{components.Heartbeat}
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "base"), requiring))
	options := deployOptions(root)
	options.Components = []components.ID{components.Context}
	tests = append(tests, &testCase{
		_id:            "unavailable-refused",
		_expectedError: "Skill base unavailable (heartbeat)",
		_expectedFiles: map[string]bool{"home/.claude/skills/base": false},
		_shouldPass:    false,

		leaf:    "base",
		options: options,
		root:    root,
	})

	// unconfigured-refused
	root = provideDeployRoot(t)
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Components = []components.ID{components.Skills}
	tests = append(tests, &testCase{
		_id:            "unconfigured-refused",
		_expectedError: "Skill needy unconfigured (SKILL_NEEDY_TITLE_SET)",
		_expectedFiles: map[string]bool{"home/.claude/skills/needy": false},
		_shouldPass:    false,

		leaf:    "needy",
		options: options,
		root:    root,
	})

	// unconfigured-refused-with-cause
	root = provideDeployRoot(t)
	saveRequiredSkill(t, root, "needy")
	options = deployOptions(root)
	options.Rows = &Rows{Skills: map[string]string{"SKILL_NEEDY_TITLE_SET": "my-rules && x"}}
	tests = append(tests, &testCase{
		_id:            "unconfigured-refused-with-cause",
		_expectedError: `Skill needy unconfigured (SKILL_NEEDY_TITLE_SET: value: cmdline.Split: Shell operator "&&" in "my-rules && x")`,
		_expectedFiles: map[string]bool{"home/.claude/skills/needy": false},
		_shouldPass:    false,

		leaf:    "needy",
		options: options,
		root:    root,
	})

	// deferred-required-deployed
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	options = deployOptions(root)
	options.Rows = &Rows{Skills: map[string]string{"SKILL_CHECKY_CHECK": "my-rules audit"}}
	tests = append(tests, &testCase{
		_id: "deferred-required-deployed",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/checky/SKILL.md": true,
			"home/.codex/skills/checky/SKILL.md":  true,
		},
		_expectedReport: "  checky -> " + filepath.Join(root, "home", ".claude", "skills", "checky") + " (new v1.0)",
		_shouldPass:     true,

		leaf:    "checky",
		options: options,
		root:    root,
	})

	// codex-home-skipped-when-default-missing
	root = provideDeployRoot(t)
	saveCheckSkill(t, root, "checky")
	tests = append(tests, &testCase{
		_id: "codex-home-skipped-when-default-missing",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/checky/SKILL.md": true,
			"home/.codex/skills/checky":           false,
		},
		_expectedReport: "  skip checky unconfigured (SKILL_CHECKY_CHECK) in " + filepath.Join(root, "home", ".codex", "skills"),
		_shouldPass:     true,

		leaf:    "checky",
		options: deployOptions(root),
		root:    root,
	})

	// unknown-leaf
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id:            "unknown-leaf",
		_expectedError: "No skill leaf named nope",
		_expectedFiles: map[string]bool{"home/.claude/skills": false},
		_shouldPass:    false,

		leaf:    "nope",
		options: deployOptions(root),
		root:    root,
	})

	// writes-marker
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id: "writes-marker",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base/" + markerFileName: true,
			"home/.codex/skills/base/" + markerFileName:  true,
		},
		_shouldPass: true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// deploys-marked-alias
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	tests = append(tests, &testCase{
		_id: "deploys-marked-alias",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/pk/" + markerFileName: true,
			"home/.codex/skills/pk/SKILL.md":           true,
		},
		_expectedReport: "  pk -> " + filepath.Join(root, "home", ".claude", "skills", "pk") + " (alias of base)",
		_shouldPass:     true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := DeployLeaf(test.leaf, test.options)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Contains(t, report, test._expectedReport)
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
		})
	}
}

func TestDeployRun_Claims(t *testing.T) {
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	name, script := "counting-rules", "#!/bin/sh\necho x >> '"+countPath+"'\necho '{\"interfaces\":[\"title-set@1\"]}'\n"
	if runtime.GOOS == "windows" {
		name, script = "counting-rules.bat", "@echo x>>\""+countPath+"\"\r\n@echo {\"interfaces\":[\"title-set@1\"]}\r\n"
	}
	writeDeployFile(t, dir, name, script, 0o755)
	provider := &skillgraph.Provider{Id: "smine-rules", Implements: []string{"title-set@1"}, Program: filepath.Join(dir, name)}
	run := newDeployRun(deployOptions(t.TempDir()))

	first, firstErr := run.Claims(context.Background(), provider)
	second, secondErr := run.Claims(context.Background(), provider)
	count, err := os.ReadFile(countPath)
	require.NoError(t, err)

	assert.NoError(t, firstErr)
	assert.NoError(t, secondErr)
	assert.Equal(t, []string{"title-set@1"}, first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, strings.Count(string(count), "x"), "probe-runs-once-per-executable")

	missing := &skillgraph.Provider{Id: "ghost", Program: filepath.Join(dir, "ghost")}
	_, missingErr := run.Claims(context.Background(), missing)
	assert.Error(t, missingErr, "missing-executable-errors")
}

func TestRemoveLeaf(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedError   string
		_expectedFiles   map[string]bool
		_expectedReport  string
		_expectedStashed map[string]bool
		_shouldPass      bool

		leaf    string
		options *SyncOptions
		root    string
	}

	tests := make([]*testCase, 0)

	// removes-marked
	root := provideDeployRoot(t, "base", "extra")
	syncFixture(t, deployOptions(root))
	tests = append(tests, &testCase{
		_id: "removes-marked",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/base":  false,
			"home/.codex/skills/base":   false,
			"home/.claude/skills/extra": true,
		},
		_expectedReport: "  removed: " + filepath.Join(root, "home", ".claude", "skills", "base") + " -> trash ",
		_expectedStashed: map[string]bool{
			"home/.claude/skills/base": true,
			"home/.codex/skills/base":  true,
		},
		_shouldPass: true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// refuses-unmarked
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/base/SKILL.md", "user copy\n", 0o644)
	tests = append(tests, &testCase{
		_id:            "refuses-unmarked",
		_expectedError: "not deployed by smine",
		_expectedFiles: map[string]bool{"home/.claude/skills/base/SKILL.md": true},
		_shouldPass:    false,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// missing-home-skipped
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	require.NoError(t, os.RemoveAll(filepath.Join(root, "home", ".codex")))
	tests = append(tests, &testCase{
		_id:            "missing-home-skipped",
		_expectedFiles: map[string]bool{"home/.claude/skills/base": false, "home/.codex": false},
		_shouldPass:    true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// removes-agents
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", "helper agent\n", 0o644)
	syncFixture(t, deployOptions(root))
	tests = append(tests, &testCase{
		_id:              "removes-agents",
		_expectedFiles:   map[string]bool{"home/.claude/agents/helper.md": false},
		_expectedReport:  "  agent removed: " + filepath.Join(root, "home", ".claude", "agents", "helper.md") + " -> trash",
		_expectedStashed: map[string]bool{"home/.claude/agents/helper.md": true},
		_shouldPass:      true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// removes-alias
	root = provideDeployRoot(t, "base")
	declareAliases(t, root, "base", "pk", "")
	syncFixture(t, deployOptions(root))
	tests = append(tests, &testCase{
		_id: "removes-alias",
		_expectedFiles: map[string]bool{
			"home/.claude/skills/pk": false,
			"home/.codex/skills/pk":  false,
		},
		_expectedReport: "  removed: " + filepath.Join(root, "home", ".claude", "skills", "pk"),
		_shouldPass:     true,

		leaf:    "base",
		options: deployOptions(root),
		root:    root,
	})

	// unknown-leaf
	root = provideDeployRoot(t, "base")
	tests = append(tests, &testCase{
		_id:            "unknown-leaf",
		_expectedError: "No skill leaf named nope",
		_shouldPass:    false,

		leaf:    "nope",
		options: deployOptions(root),
		root:    root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := RemoveLeaf(test.leaf, test.options)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Contains(t, report, test._expectedReport)
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
			assert.Equal(t, test._expectedStashed, stashedPaths(test.root, test._expectedStashed))
		})
	}
}

func TestIsDeployedCurrent(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   bool
		_shouldPass bool

		deployedDir string
		leafDir     string
	}

	const leafPath = "repo/skills/group/base"
	// provideDeployedBase deploys the base skill with a README and returns the fixture root, its source and deployed dirs.
	provideDeployedBase := func() (string, string, string) {
		root := provideDeployRoot(t, "base")
		writeDeployFile(t, root, leafPath+"/README.md", "notes\n", 0o644)
		syncFixture(t, deployOptions(root))
		return root, filepath.Join(root, filepath.FromSlash(leafPath)), filepath.Join(root, "home", ".claude", "skills", "base")
	}
	tests := make([]*testCase, 0)

	// fresh-deploy-current
	_, leafDir, deployedDir := provideDeployedBase()
	tests = append(tests, &testCase{
		_id:         "fresh-deploy-current",
		_expected:   true,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// unshipped-edits-current
	root, leafDir, deployedDir := provideDeployedBase()
	writeDeployFile(t, root, leafPath+"/eval/definition.json", "{}\n", 0o644)
	writeDeployFile(t, root, leafPath+"/smoke/tree/notes/plan.md", "plan\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "unshipped-edits-current",
		_expected:   true,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// projection-block-current
	root, leafDir, deployedDir = provideDeployedBase()
	projected := "<!-- [ACDSL-PROJECTION] 1 rule(s) govern this file — working-copy view, stripped before commit -->\n<!-- - [ACDSL-DEMO-001] demo rule -->\n\nnotes\n"
	writeDeployFile(t, root, leafPath+"/README.md", projected, 0o644)
	tests = append(tests, &testCase{
		_id:         "projection-block-current",
		_expected:   true,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// source-edit-stale
	root, leafDir, deployedDir = provideDeployedBase()
	writeDeployFile(t, root, leafPath+"/README.md", "edited\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "source-edit-stale",
		_expected:   false,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// deployed-edit-current
	root, leafDir, deployedDir = provideDeployedBase()
	writeDeployFile(t, root, "home/.claude/skills/base/README.md", "edited in place\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "deployed-edit-current",
		_expected:   true,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// marker-missing-stale
	_, leafDir, deployedDir = provideDeployedBase()
	require.NoError(t, os.Remove(filepath.Join(deployedDir, markerFileName)))
	tests = append(tests, &testCase{
		_id:         "marker-missing-stale",
		_expected:   false,
		_shouldPass: true,

		deployedDir: deployedDir,
		leafDir:     leafDir,
	})

	// source-missing-fails
	_, _, deployedDir = provideDeployedBase()
	tests = append(tests, &testCase{
		_id:         "source-missing-fails",
		_expected:   false,
		_shouldPass: false,

		deployedDir: deployedDir,
		leafDir:     filepath.Join(t.TempDir(), "missing"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isCurrent, err := IsDeployedCurrent(test.leafDir, test.deployedDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, isCurrent)
		})
	}
}
