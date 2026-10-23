package skillgraph

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
)

// claimLiteralPattern is one "<id>@<n>" string literal in a provider's claim source.
var claimLiteralPattern = regexp.MustCompile(`"([a-z][a-z0-9-]*@[0-9]+)"`)

// catalogCommandCue is one valid command interface record as inline CUE.
const catalogCommandCue = `{id: "test", version: 1, kind: "command", scope: "repo", summary: "Runs the suite.", example: "go test ./...", invocation: {cwd: "repo-root", args: "none", stdin: "none", timeout: "20m"}, result: {exit: {"0": "pass", other: "fail"}, stdout: "diagnostic"}, validate: {resolves: true}}`

// catalogProgramCue is one valid program interface record as inline CUE.
const catalogProgramCue = `{id: "lanes", version: 1, kind: "program", scope: "install", summary: "Writes lanes.", invocation: {cwd: "repo-root", args: "lanes --round-dir <dir>", stdin: "none", timeout: "5m"}, result: {exit: {"0": "printed", other: "error"}, stdout: "the plan"}, validate: {resolves: true}}`

// catalogPathCue is one valid path interface record as inline CUE.
const catalogPathCue = `{id: "plans-archive", version: 1, kind: "path", scope: "repo", summary: "The archive root.", example: "~/plans", invocation: {cwd: "none", args: "none", stdin: "none", timeout: "none"}, result: {exit: {"0": "n/a", other: "n/a"}, stdout: "n/a"}, validate: {exists: "dir"}}`

// catalogSkillCue is one valid skill interface record as inline CUE.
const catalogSkillCue = `{id: "commit-skill", version: 1, kind: "skill", scope: "repo", summary: "The commit hand-off.", example: "package-commit", invocation: {cwd: "none", args: "none", stdin: "none", timeout: "none"}, result: {exit: {"0": "n/a", other: "n/a"}, stdout: "n/a"}, validate: {}}`

// catalogTitleSetCue is the title-set program record as inline CUE.
const catalogTitleSetCue = `{id: "title-set", version: 1, kind: "program", scope: "install", summary: "Sets the title state.", invocation: {cwd: "repo-root", args: "title set <TOKEN>", stdin: "none", timeout: "1m"}, result: {exit: {"0": "pass", other: "fail"}, stdout: "title: set <TOKEN>"}, validate: {resolves: true}}`

// catalogProviderCue is one valid provider record as inline CUE.
const catalogProviderCue = `{id: "smine-rules", program: "smine-rules", component: "config-server", implements: ["lanes@1"], probe: ["interfaces"]}`

// bindEnv sets the skill's env block and attaches the test catalog its keys bind into.
func bindEnv(skill *Skill, env map[string]*Variable) {
	skill.Env = env
	skill.Catalog = provideTestCatalog()
}

// catalogCue joins interface and provider records into one interfaces.cue text.
func catalogCue(interfaces []string, providers []string) string {
	return "interfaces: [" + strings.Join(interfaces, ", ") + "]\nproviders: [" + strings.Join(providers, ", ") + "]\n"
}

// provideTestCatalog is a small catalog: the build, check and test commands, five programs, the archive path and the commit skill.
func provideTestCatalog() *Catalog {
	exit := map[string]string{exitOther: "fail", exitZero: "pass"}
	resolves := Validate{Resolves: true}
	interfaces := []*Interface{
		{Id: "agentwt-status", Invocation: Invocation{Args: "status [--format json]", Cwd: "repo-root", Stdin: "none", Timeout: "5m"}, Kind: KindProgram, Result: Result{Exit: exit, Stdout: "document"}, Scope: ScopeInstall, Summary: "Prints the status.", Validate: resolves, Version: 1},
		{Id: "build", Example: "go build ./...", Invocation: Invocation{Args: "none", Cwd: "repo-root", Stdin: "none", Timeout: "20m"}, Kind: KindCommand, Result: Result{Exit: exit, Stdout: "diagnostic"}, Scope: ScopeRepo, Summary: "Compiles the repo.", Validate: resolves, Version: 1},
		{Id: "check", Example: "go vet ./...", Invocation: Invocation{Args: "none", Cwd: "repo-root", Stdin: "none", Timeout: "30m"}, Kind: KindCommand, Result: Result{Exit: exit, Stdout: "diagnostic"}, Scope: ScopeRepo, Summary: "Runs everything before a commit.", Validate: resolves, Version: 1},
		{Id: "commit-skill", Example: "package-commit", Invocation: Invocation{Args: "none", Cwd: "none", Stdin: "none", Timeout: noTimeout}, Kind: KindSkill, Result: Result{Exit: exit, Stdout: "n/a"}, Scope: ScopeRepo, Summary: "The commit hand-off.", Validate: Validate{}, Version: 1},
		{Id: "heartbeat-status", Invocation: Invocation{Args: "status [--format json]", Cwd: "repo-root", Stdin: "none", Timeout: "1m"}, Kind: KindProgram, Result: Result{Exit: exit, Stdout: "document"}, Scope: ScopeInstall, Summary: "Prints the heartbeat status.", Validate: resolves, Version: 1},
		{Id: "lanes", Invocation: Invocation{Args: "lanes --round-dir <dir>", Cwd: "repo-root", Stdin: "none", Timeout: "5m"}, Kind: KindProgram, Result: Result{Exit: exit, Stdout: "the plan"}, Scope: ScopeInstall, Summary: "Writes the lanes.", Validate: resolves, Version: 1},
		{Id: "plan-sections", Invocation: Invocation{Args: "plan-sections <plan file>", Cwd: "repo-root", Stdin: "none", Timeout: "1m"}, Kind: KindProgram, Result: Result{Exit: exit, Stdout: "violations"}, Scope: ScopeInstall, Summary: "Checks a plan's sections.", Validate: resolves, Version: 1},
		{Id: "plans-archive", Example: "~/plans", Invocation: Invocation{Args: "none", Cwd: "none", Stdin: "none", Timeout: noTimeout}, Kind: KindPath, Result: Result{Exit: exit, Stdout: "n/a"}, Scope: ScopeRepo, Summary: "The archive root.", Validate: Validate{Exists: "dir"}, Version: 1},
		{Id: "test", Example: "go test ./...", Invocation: Invocation{Args: "none", Cwd: "repo-root", Stdin: "none", Timeout: "20m"}, Kind: KindCommand, Result: Result{Exit: exit, Stdout: "diagnostic"}, Scope: ScopeRepo, Summary: "Runs the suite.", Validate: resolves, Version: 1},
		{Id: "title-set", Invocation: Invocation{Args: "title set <TOKEN>", Cwd: "repo-root", Stdin: "none", Timeout: "1m"}, Kind: KindProgram, Result: Result{Exit: exit, Stdout: "title: set <TOKEN>"}, Scope: ScopeInstall, Summary: "Sets the title state.", Validate: resolves, Version: 1},
	}
	providers := []*Provider{
		{Id: "smine-agentwt", Component: components.Agentwt, Implements: []string{"agentwt-status@1"}, Probe: []string{"interfaces"}, Program: "smine-agentwt"},
		{Id: "smine-heartbeat", Component: components.Heartbeat, Implements: []string{"heartbeat-status@1"}, Probe: []string{"interfaces"}, Program: "smine-heartbeat"},
		{Id: "smine-rules", Component: components.ConfigServer, Implements: []string{"lanes@1", "plan-sections@1", "title-set@1"}, Probe: []string{"interfaces"}, Program: "smine-rules"},
	}
	return &Catalog{Interfaces: interfaces, Providers: providers}
}

// provideCatalogRoot is a skills root holding an empty vocabulary and a catalog with the title-set record.
func provideCatalogRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, FileVocabulary), []byte("atoms: {}\n"), 0o644))
	catalog := catalogCue([]string{catalogTitleSetCue}, nil)
	require.NoError(t, os.WriteFile(filepath.Join(root, FileCatalog), []byte(catalog), 0o644))
	return root
}

// repoCatalog loads the repo's skills/interfaces.cue.
func repoCatalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := LoadCatalog(filepath.Join("..", "..", "skills", FileCatalog))
	require.NoError(t, err)
	return catalog
}

func TestCheckValue(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedError string

		iface *Interface
		value string
	}

	catalog := provideTestCatalog()
	command, _ := catalog.Interface("test@1")
	program, _ := catalog.Interface("lanes@1")
	path, _ := catalog.Interface("plans-archive@1")
	skill, _ := catalog.Interface("commit-skill@1")
	tests := make([]*testCase, 0)

	// fail-skill-two-words
	tests = append(tests, &testCase{_id: "fail-skill-two-words", _expectedError: "value: a skill is one leaf name — lowercase letters, digits and hyphens", iface: skill, value: "package commit"})

	// fail-skill-uppercase
	tests = append(tests, &testCase{_id: "fail-skill-uppercase", _expectedError: "value: a skill is one leaf name — lowercase letters, digits and hyphens", iface: skill, value: "Package-Commit"})

	// pass-skill-leaf
	tests = append(tests, &testCase{_id: "pass-skill-leaf", _expectedError: "", iface: skill, value: "package-commit"})

	// command-two-lines-pass
	tests = append(tests, &testCase{_id: "command-two-lines-pass", _expectedError: "", iface: command, value: "python manage.py check\npytest"})

	// command-blank-lines-only-refused
	tests = append(tests, &testCase{_id: "command-blank-lines-only-refused", _expectedError: "value: cmdline.SplitLines: No command line in the value", iface: command, value: "\n  \n"})

	// program-two-lines-refused
	tests = append(tests, &testCase{_id: "program-two-lines-refused", _expectedError: "value: spans more than one line", iface: program, value: "smine-rules\nlanes"})

	// command-carriage-return-refused
	tests = append(tests, &testCase{_id: "command-carriage-return-refused", _expectedError: "value: holds a carriage return", iface: command, value: "make build\r\nmake test"})

	// fail-backtick
	tests = append(tests, &testCase{_id: "fail-backtick", _expectedError: "value: holds a backtick", iface: command, value: "make `x`"})

	// fail-skill-reference
	tests = append(tests, &testCase{_id: "fail-skill-reference", _expectedError: "value: references $SKILL_", iface: command, value: "$SKILL_DEMO_BUILD"})

	// fail-parenthesis
	tests = append(tests, &testCase{_id: "fail-parenthesis", _expectedError: "value: holds a parenthesis — grants splice values into Bash(…)", iface: command, value: "make (audit)"})

	// fail-command-with-operator
	tests = append(tests, &testCase{_id: "fail-command-with-operator", _expectedError: `value: cmdline.SplitLines: cmdline.Split: Shell operator "&&"`, iface: command, value: "make build && make test"})

	// fail-command-blank
	tests = append(tests, &testCase{_id: "fail-command-blank", _expectedError: "value: cmdline.SplitLines: No command line in the value", iface: command, value: "  "})

	// fail-program-with-two-words
	tests = append(tests, &testCase{_id: "fail-program-with-two-words", _expectedError: "value: a program is one word, got 2", iface: program, value: "smine-rules lanes"})

	// fail-path-relative
	tests = append(tests, &testCase{_id: "fail-path-relative", _expectedError: "value: a path opens with ~/ or /", iface: path, value: "plans"})

	// pass-command
	tests = append(tests, &testCase{_id: "pass-command", _expectedError: "", iface: command, value: `go test -run "A B" ./...`})

	// pass-program-path
	tests = append(tests, &testCase{_id: "pass-program-path", _expectedError: "", iface: program, value: "/opt/bin/smine-rules"})

	// pass-path-home
	tests = append(tests, &testCase{_id: "pass-path-home", _expectedError: "", iface: path, value: "~/plans"})

	// pass-path-absolute
	tests = append(tests, &testCase{_id: "pass-path-absolute", _expectedError: "", iface: path, value: "/srv/plans"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := CheckValue(test.iface, test.value)

			assert.Equalf(t, test._expectedError == "", err == nil, "err = %v", err)
			assert.True(t, err == nil || strings.Contains(err.Error(), test._expectedError), "err = %v", err)
		})
	}
}

func TestLoadCatalog(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedError string

		path string
	}

	writeCatalog := func(text string) string {
		path := filepath.Join(t.TempDir(), FileCatalog)
		require.NoError(t, os.WriteFile(path, []byte(text), 0o644))
		return path
	}
	validInterfaces := []string{catalogCommandCue, catalogProgramCue, catalogPathCue, catalogSkillCue}
	tests := make([]*testCase, 0)

	// fail-skill-with-resolves
	resolvingSkill := strings.Replace(catalogSkillCue, "validate: {}", "validate: {resolves: true}", 1)
	tests = append(tests, &testCase{_id: "fail-skill-with-resolves", _expectedError: "catalog: commit-skill@1: a skill interface validates by its leaf name — no resolves, no exists", path: writeCatalog(catalogCue([]string{resolvingSkill}, nil))})

	// repo-catalog-loads
	tests = append(tests, &testCase{_id: "repo-catalog-loads", _expectedError: "", path: filepath.Join("..", "..", "skills", FileCatalog)})

	// command-install-scoped-refused
	installCommand := strings.Replace(catalogCommandCue, `scope: "repo"`, `scope: "install"`, 1)
	tests = append(tests, &testCase{_id: "command-install-scoped-refused", _expectedError: "catalog: test@1: a command interface is repo-scoped — its value differs per repo", path: writeCatalog(catalogCue([]string{installCommand}, nil))})

	// repo-scoped-without-example-refused
	withoutExample := strings.Replace(catalogCommandCue, `, example: "go test ./..."`, "", 1)
	tests = append(tests, &testCase{_id: "repo-scoped-without-example-refused", _expectedError: "catalog: test@1: a repo-scoped interface carries an example the abort sentence shows", path: writeCatalog(catalogCue([]string{withoutExample}, nil))})

	// provider-mixed-scopes-refused
	mixedProvider := strings.Replace(catalogProviderCue, `["lanes@1"]`, `["lanes@1", "test@1"]`, 1)
	tests = append(tests, &testCase{_id: "provider-mixed-scopes-refused", _expectedError: "catalog: provider smine-rules implements install- and repo-scoped records — one scope per provider", path: writeCatalog(catalogCue(validInterfaces, []string{mixedProvider}))})

	// pass-inline
	tests = append(tests, &testCase{_id: "pass-inline", _expectedError: "", path: writeCatalog(catalogCue(validInterfaces, []string{catalogProviderCue}))})

	// fail-comment
	tests = append(tests, &testCase{_id: "fail-comment", _expectedError: "comment group", path: writeCatalog("// note\n" + catalogCue(validInterfaces, nil))})

	// fail-duplicate-interface
	duplicate := catalogCue([]string{catalogCommandCue, catalogCommandCue}, nil)
	tests = append(tests, &testCase{_id: "fail-duplicate-interface", _expectedError: "catalog: interface test@1 declared twice", path: writeCatalog(duplicate)})

	// fail-command-without-resolves
	unresolved := strings.Replace(catalogCommandCue, "validate: {resolves: true}", "validate: {}", 1)
	tests = append(tests, &testCase{_id: "fail-command-without-resolves", _expectedError: "catalog: test@1: a command interface validates with resolves: true and no exists", path: writeCatalog(catalogCue([]string{unresolved}, nil))})

	// fail-path-without-exists
	unchecked := strings.Replace(catalogPathCue, `validate: {exists: "dir"}`, "validate: {}", 1)
	tests = append(tests, &testCase{_id: "fail-path-without-exists", _expectedError: "catalog: plans-archive@1: a path interface validates with exists: dir|file and no resolves", path: writeCatalog(catalogCue([]string{unchecked}, nil))})

	// fail-exit-without-other
	partialExit := strings.Replace(catalogCommandCue, `, other: "fail"`, "", 1)
	tests = append(tests, &testCase{_id: "fail-exit-without-other", _expectedError: `catalog: test@1: result.exit needs the keys "0" and "other"`, path: writeCatalog(catalogCue([]string{partialExit}, nil))})

	// fail-bad-timeout
	badTimeout := strings.Replace(catalogCommandCue, `timeout: "20m"`, `timeout: "soon"`, 1)
	tests = append(tests, &testCase{_id: "fail-bad-timeout", _expectedError: `catalog: test@1: invocation.timeout "soon" is neither a duration nor "none"`, path: writeCatalog(catalogCue([]string{badTimeout}, nil))})

	// fail-program-args-without-head
	headless := strings.Replace(catalogProgramCue, `args: "lanes --round-dir <dir>"`, `args: " "`, 1)
	tests = append(tests, &testCase{_id: "fail-program-args-without-head", _expectedError: "catalog: lanes@1: a program interface's args open with the subcommand word", path: writeCatalog(catalogCue([]string{headless}, nil))})

	// fail-claim-without-record
	unknownClaim := strings.Replace(catalogProviderCue, `["lanes@1"]`, `["lanes@2"]`, 1)
	tests = append(tests, &testCase{_id: "fail-claim-without-record", _expectedError: "catalog: provider smine-rules implements lanes@2, which no interface record declares", path: writeCatalog(catalogCue(validInterfaces, []string{unknownClaim}))})

	// fail-two-providers-one-version
	secondProvider := strings.Replace(catalogProviderCue, `id: "smine-rules", program: "smine-rules"`, `id: "other", program: "other"`, 1)
	tests = append(tests, &testCase{_id: "fail-two-providers-one-version", _expectedError: "catalog: lanes@1 is implemented by smine-rules and other — one provider per interface version", path: writeCatalog(catalogCue(validInterfaces, []string{catalogProviderCue, secondProvider}))})

	// fail-binary-without-program
	binaryOnly := strings.Replace(catalogProviderCue, `program: "smine-rules"`, `program: "", binary: "~/bin/x"`, 1)
	tests = append(tests, &testCase{_id: "fail-binary-without-program", _expectedError: "catalog: provider smine-rules: binary without a program", path: writeCatalog(catalogCue(validInterfaces, []string{binaryOnly}))})

	// fail-unknown-kind
	unknownKind := strings.Replace(catalogCommandCue, `kind: "command"`, `kind: "value"`, 1)
	tests = append(tests, &testCase{_id: "fail-unknown-kind", _expectedError: "violates #Catalog", path: writeCatalog(catalogCue([]string{unknownKind}, nil))})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := LoadCatalog(test.path)

			assert.Equalf(t, test._expectedError == "", err == nil, "err = %v", err)
			assert.True(t, err == nil || strings.Contains(err.Error(), test._expectedError), "err = %v", err)
		})
	}
}

func TestFindCatalog(t *testing.T) {
	outside, err := FindCatalog(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, outside.Interfaces, "outside-a-root-empty")

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, FileVocabulary), []byte("atoms: {}\n"), 0o644))
	leafDir := filepath.Join(root, "group", "leaf")
	withoutFile, err := FindCatalog(leafDir)
	require.NoError(t, err)
	assert.Empty(t, withoutFile.Interfaces, "root-without-file-empty")

	require.NoError(t, os.WriteFile(filepath.Join(root, FileCatalog), []byte(catalogCue([]string{catalogCommandCue}, nil)), 0o644))
	withFile, err := FindCatalog(leafDir)
	require.NoError(t, err)
	_, isKnown := withFile.Interface("test@1")
	assert.True(t, isKnown, "root-with-file-loaded")
}

func TestProviderClaims(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		claimed []string
	}

	repoRoot := filepath.Join("..", "..")
	sources := map[string]string{
		"acdsl":           "cmd/acdsl/interfaces.go",
		"smine-agentwt":   "cmd/agentwt/src/interfaces.rs",
		"smine-heartbeat": "cmd/heartbeat/src/interfaces.rs",
		"smine-routine":   "cmd/routine/interfaces.go",
		"smine-rules":     "cmd/rules/interfaces.go",
		"toolpolicy":      "cmd/toolpolicy/protocol.go",
	}
	catalog := repoCatalog(t)
	tests := make([]*testCase, 0)

	for _, provider := range catalog.Providers {
		source, hasSource := sources[provider.Id]
		require.True(t, hasSource, "provider %s has no claim source", provider.Id)
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(source)))
		require.NoError(t, err)

		var claimed []string
		for _, match := range claimLiteralPattern.FindAllStringSubmatch(string(content), -1) {
			claimed = append(claimed, match[1])
		}
		slices.Sort(claimed)
		tests = append(tests, &testCase{_id: provider.Id, _expected: provider.Claims(), claimed: slices.Compact(claimed)})
	}

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.claimed)
		})
	}
}

func TestSkillRepoScoped(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]bool

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// bound-repo-and-install
	mixed := provideCompleteSkill()
	bindEnv(mixed, map[string]*Variable{"BUILD": {Interface: "build@1"}, "LANES": {Interface: "lanes@1"}, "PLANS_ARCHIVE": {Interface: "plans-archive@1"}})
	tests = append(tests, &testCase{_id: "bound-repo-and-install", _expected: map[string]bool{"SKILL_DEMO_BUILD": true, "SKILL_DEMO_PLANS_ARCHIVE": true}, skill: mixed})

	// unknown-interface-ignored
	unknown := provideCompleteSkill()
	bindEnv(unknown, map[string]*Variable{"BUILD": {Interface: "build@9"}, "TEST": {Interface: "test@1"}})
	tests = append(tests, &testCase{_id: "unknown-interface-ignored", _expected: map[string]bool{"SKILL_DEMO_TEST": true}, skill: unknown})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.skill.RepoScoped())
		})
	}
}

func TestVariableReaders(t *testing.T) {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"LANES": {Interface: "lanes@1"}})
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_LANES lanes`."
	gate := flowSection(skill).Blocks[1].Entry
	gate.Items = append(gate.Items, Item{Label: LabelParam, Marker: "*", Text: "lanes: `$SKILL_DEMO_LANES lanes --round-dir x`."})
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_LANES lanes *), Read"})
	anchors := map[string]*Anchor{
		"lanes": {Input: []InputMatch{{Field: "command", Pattern: "^$SKILL_DEMO_LANES lanes "}}, Tool: "^Bash$"},
		"stage": {Tool: "^Workflow$"},
	}
	checks := map[string]*EnforceCheck{"no_lanes": {Deny: "lanes", Kind: "phase", Reason: "run $SKILL_DEMO_LANES lanes first", Recovery: "run it"}}
	skill.Enforce = &Enforce{Anchors: anchors, Checks: checks}

	readers := VariableReaders(skill, "SKILL_DEMO_LANES")

	expected := []string{"param of SKILL-DEMO-FLOW-002", "entry SKILL-DEMO-FLOW-003", "grant Bash($SKILL_DEMO_LANES lanes *)", "anchor lanes", "check no_lanes"}
	assert.Equal(t, expected, readers)
	assert.Empty(t, VariableReaders(skill, "SKILL_DEMO_OTHER"), "unread-name-has-no-readers")
}
