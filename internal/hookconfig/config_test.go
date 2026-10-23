package hookconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurationMs(t *testing.T) {
	encoded, err := json.Marshal(DurationMs(1500 * time.Millisecond))
	require.NoError(t, err)
	assert.JSONEq(t, `1500`, string(encoded))

	var decoded DurationMs
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, 1500*time.Millisecond, decoded.Duration())

	assert.Error(t, json.Unmarshal([]byte(`"1500ms"`), &decoded))
}

func TestIndexReadOnlyCommands(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   map[string]*Listing
		_shouldPass bool

		entries []string
	}

	tests := make([]*testCase, 0)

	// pass-bare
	tests = append(tests, &testCase{_id: "pass-bare", _expected: map[string]*Listing{"cat": {IsBare: true}, "ls": {IsBare: true}}, _shouldPass: true, entries: []string{"cat", "ls"}})

	// pass-subcommands
	tests = append(tests, &testCase{_id: "pass-subcommands", _expected: map[string]*Listing{"git": {Subcommands: []string{"status", "log"}}}, _shouldPass: true, entries: []string{"git status", "git log"}})

	// pass-multiword-subcommand
	tests = append(tests, &testCase{_id: "pass-multiword-subcommand", _expected: map[string]*Listing{"git": {Subcommands: []string{"config --get"}}}, _shouldPass: true, entries: []string{"git config --get"}})

	// fail-bare-then-subcommand
	tests = append(tests, &testCase{_id: "fail-bare-then-subcommand", _expected: nil, _shouldPass: false, entries: []string{"sed", "sed -n"}})

	// fail-subcommand-then-bare
	tests = append(tests, &testCase{_id: "fail-subcommand-then-bare", _expected: nil, _shouldPass: false, entries: []string{"git status", "git"}})

	// fail-empty-entry
	tests = append(tests, &testCase{_id: "fail-empty-entry", _expected: nil, _shouldPass: false, entries: []string{" "}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			index, err := indexReadOnlyCommands(test.entries)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, index)
		})
	}
}

func TestLoad(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		home string
	}

	tests := make([]*testCase, 0)

	// pass-complete
	completeHome := t.TempDir()
	writeConfig(t, completeHome, provideCompleteConfig())
	tests = append(tests, &testCase{_id: "pass-complete", _shouldPass: true, home: completeHome})

	// fail-unknown-key
	unknownKeyHome := t.TempDir()
	unknownKeyDocument := configDocument(t, provideCompleteConfig())
	unknownKeyDocument["toolpolicy"]["unknown_key"] = true
	writeDocument(t, unknownKeyHome, unknownKeyDocument)
	tests = append(tests, &testCase{_id: "fail-unknown-key", _shouldPass: false, home: unknownKeyHome})

	// fail-wrong-type
	wrongTypeHome := t.TempDir()
	wrongTypeDocument := configDocument(t, provideCompleteConfig())
	wrongTypeDocument["globalcontext"]["inline_cap"] = "nine thousand"
	writeDocument(t, wrongTypeHome, wrongTypeDocument)
	tests = append(tests, &testCase{_id: "fail-wrong-type", _shouldPass: false, home: wrongTypeHome})

	// fail-empty-read-only-commands
	emptyReadOnlyHome := t.TempDir()
	emptyReadOnlyConfig := provideCompleteConfig()
	emptyReadOnlyConfig.Toolpolicy.ReadOnlyCommands = []string{}
	writeConfig(t, emptyReadOnlyHome, emptyReadOnlyConfig)
	tests = append(tests, &testCase{_id: "fail-empty-read-only-commands", _shouldPass: false, home: emptyReadOnlyHome})

	// fail-tool-in-two-classes
	twoClassesHome := t.TempDir()
	twoClassesConfig := provideCompleteConfig()
	twoClassesConfig.Toolpolicy.ToolClasses.Read = []string{"Read", "Bash"}
	writeConfig(t, twoClassesHome, twoClassesConfig)
	tests = append(tests, &testCase{_id: "fail-tool-in-two-classes", _shouldPass: false, home: twoClassesHome})

	// fail-bare-and-subcommand
	bareAndSubcommandHome := t.TempDir()
	bareAndSubcommandConfig := provideCompleteConfig()
	bareAndSubcommandConfig.Toolpolicy.ReadOnlyCommands = []string{"git", "git status"}
	writeConfig(t, bareAndSubcommandHome, bareAndSubcommandConfig)
	tests = append(tests, &testCase{_id: "fail-bare-and-subcommand", _shouldPass: false, home: bareAndSubcommandHome})

	// fail-empty-cause
	emptyCauseHome := t.TempDir()
	emptyCauseConfig := provideCompleteConfig()
	emptyCauseConfig.Toolpolicy.Causes.Fenced = ""
	writeConfig(t, emptyCauseHome, emptyCauseConfig)
	tests = append(tests, &testCase{_id: "fail-empty-cause", _shouldPass: false, home: emptyCauseHome})

	// fail-empty-steer
	emptySteerHome := t.TempDir()
	emptySteerConfig := provideCompleteConfig()
	emptySteerConfig.Toolpolicy.Steers.Deny = ""
	writeConfig(t, emptySteerHome, emptySteerConfig)
	tests = append(tests, &testCase{_id: "fail-empty-steer", _shouldPass: false, home: emptySteerHome})

	// fail-missing-file
	tests = append(tests, &testCase{_id: "fail-missing-file", _shouldPass: false, home: t.TempDir()})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := Load(test.home)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equalf(t, !test._shouldPass, strings.Contains(fmt.Sprint(err), Path(test.home)), "err = %v", err)
		})
	}
}

func TestLoad_RoundTrip(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, provideCompleteConfig())
	expected := provideCompleteConfig()
	require.NoError(t, expected.Toolpolicy.index())

	loaded, err := Load(home)

	require.NoError(t, err)
	assert.Equal(t, expected, loaded)
}

func TestToolpolicy_ClassOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Class

		tool string
	}

	policy := &provideCompleteConfig().Toolpolicy

	tests := make([]*testCase, 0)

	// bash
	tests = append(tests, &testCase{_id: "bash", _expected: ClassBash, tool: "Bash"})

	// grep-read
	tests = append(tests, &testCase{_id: "grep-read", _expected: ClassRead, tool: "Grep"})

	// edit-write
	tests = append(tests, &testCase{_id: "edit-write", _expected: ClassWrite, tool: "Edit"})

	// mcp-prefix-rule-bearing
	tests = append(tests, &testCase{_id: "mcp-prefix-rule-bearing", _expected: ClassRuleBearing, tool: "mcp__peek-mcp__session_get"})

	// tool-search-unknown
	tests = append(tests, &testCase{_id: "tool-search-unknown", _expected: ClassUnknown, tool: "ToolSearch"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, policy.ClassOf(test.tool))
		})
	}
}

// configDocument marshals a config into its section maps, so a case can add or retype one key.
func configDocument(t *testing.T, config *Config) map[string]map[string]any {
	t.Helper()
	encoded, err := json.Marshal(config)
	require.NoError(t, err)

	document := map[string]map[string]any{}
	require.NoError(t, json.Unmarshal(encoded, &document))
	return document
}

// provideCompleteConfig sets every file key; ReadFence is process env and never in the file.
func provideCompleteConfig() *Config {
	config := &Config{
		Acdslhook: Acdslhook{
			Binaries:      []string{"acdsl", "acdsl.exe"},
			ContextText:   "acdsl context",
			GoRunFallback: true,
			TimeoutMs:     DurationMs(14 * time.Second),
		},
		Context: Context{
			ContextDir: "context",
		},
		Globalcontext: Globalcontext{
			Banner:        "global banner",
			InlineCap:     9000,
			PointerFormat: "global pointer %s",
			ReadBudget:    55000,
		},
		Planmodecontext: Planmodecontext{
			Banner:        "planmode banner",
			EntryFormat:   "entry %s",
			EntryIdPrefix: "RULE-PLAN-",
			Heading:       "planmode heading",
			PackFormat:    "pack %s",
			PointerFormat: "planmode pointer %s",
			RootOnly:      true,
			Trailer:       "planmode trailer",
		},
		Readgate: Readgate{
			BashAddendum:        "bash addendum",
			DefaultSliceSuffix:  "topic-default",
			DenyFormat:          "deny %s",
			HeadMismatchText:    "head mismatch",
			LedgerRetentionMs:   DurationMs(168 * time.Hour),
			ManifestWriteReason: "manifest write",
		},
		Sessiontitle: Sessiontitle{
			InstructionPlanMode: "plan mode instruction",
			InstructionTail:     "instruction tail",
			MaxStates:           8,
			MaxTitleRunes:       62,
			MaxTokenRunes:       16,
			RenameAllowReason:   "rename allowed",
			TriggerPatterns:     []string{"^/done"},
		},
		Skillcontext: Skillcontext{
			BannerFormat:     "skill banner %s",
			DefaultSlice:     "default",
			HelpPattern:      "--help$",
			InlineCap:        9000,
			NoContextPattern: "no-context",
			PointerFormat:    "skill pointer %s",
			RepeatFormat:     "repeat %s",
		},
		Skillhelp: Skillhelp{
			FeaturesLead: "features",
			HelpPattern:  "--help$",
		},
		Skillinvokegate: Skillinvokegate{
			DenyTexts:      []string{"deny after", "deny before %s"},
			EntryChapters:  []string{"actions"},
			ShellMetaChars: "|&;",
		},
		Sounds: Sounds{
			Notification: "/System/Library/Sounds/Funk.aiff",
			Stop:         "/System/Library/Sounds/Hero.aiff",
		},
		Toolpolicy: Toolpolicy{
			Causes: ToolpolicyCauses{
				AcdslRuleFile:       "acdsl rule file",
				AcdslSurface:        "acdsl surface",
				AgentModels:         "agent models",
				AnchorRemoved:       "anchor removed",
				AwkWrite:            "awk write",
				CdGit:               "cd git",
				CdProtected:         "cd protected",
				CdRedirect:          "cd redirect",
				Compound:            "compound",
				CriticalRemove:      "critical remove",
				DeniedTool:          "denied tool",
				EngineProtected:     "engine protected",
				FenceConstruct:      "fence construct",
				FenceUnparseable:    "fence unparseable",
				Fenced:              "fenced",
				FindExec:            "find exec",
				FreezeBudget:        "freeze budget",
				FreezeReopen:        "freeze reopen",
				FreezeTarget:        "freeze target",
				GitDir:              "git dir",
				GitEnvironment:      "git environment",
				GitExec:             "git exec",
				GitGlobalOption:     "git global option",
				GitSameRepo:         "git same repo",
				GoProgramFlag:       "go program flag",
				MakeArgs:            "make args",
				NoAllowRule:         "no allow rule",
				OperatorCommand:     "operator command",
				PaneLogin:           "pane login",
				ProcSubst:           "proc subst",
				ProtectedPath:       "protected path",
				ReadOutside:         "read outside",
				RulesFile:           "rules file",
				SedScriptWrite:      "sed script write",
				SettingsInteractive: "settings interactive",
				SettingsRevoked:     "settings revoked",
				SettingsUncovered:   "settings uncovered: ",
				SettingsUsage:       "settings usage",
				SettingsViaCommand:  "settings via command",
				StateUnavailable:    "state unavailable: ",
				TildeUser:           "tilde user",
				TmpWrite:            "tmp write",
				UnknownTool:         "unknown tool",
				UnparseableCompound: "unparseable compound",
				UnresolvableHead:    "unresolvable head",
				UnresolvablePath:    "unresolvable path",
				WorkflowMarker:      "workflow marker",
				WorkflowRelay:       "workflow relay",
				WorktreeOutside:     "worktree outside",
				WriteOutside:        "write outside",
				XargsWrite:          "xargs write",
			},
			DeniedTools:                 []string{"AskUserQuestion"},
			DesignSkill:                 "fdesign",
			EngineProtectedDirs:         []string{".git"},
			EngineProtectedFilePatterns: []string{"*.lock"},
			EngineProtectedFiles:        []string{".bashrc"},
			ExtraReadDirs:               []string{"~/read"},
			ExtraWriteDirs:              []string{"~/write"},
			FileCommands:                []string{"cat"},
			FimplementSkill:             "fimplement",
			GitAdmittedFlags:            []string{"--no-pager"},
			MakefileNames:               []string{"Makefile"},
			ManagedSettingsPath:         "/etc/claude/managed-settings.json",
			MaxLoopItems:                16,
			OperatorPaths:               []string{"~/operator"},
			ProtectedPaths:              []string{"~/.claude"},
			ReadOnlyCommands:            []string{"cat", "git status", "git log"},
			Reasons: ToolpolicyReasons{
				AcceptEdits:  "accept edits",
				ActiveSkill:  "active skill ",
				PathFlag:     "path flag",
				Pipeline:     "pipeline",
				PromptSkill:  "prompt skill /",
				ReadOnly:     "read only",
				Roots:        "roots",
				SedWrite:     "sed write",
				SettingsRule: "settings rule ",
				SkillGrant:   "skill grant ",
				SystemRead:   "system read",
				Ungated:      "ungated",
			},
			Steers: ToolpolicySteers{
				Bash:  "bash steer",
				Deny:  "deny steer",
				Find:  "find steer",
				Relay: "relay steer",
			},
			TmpDirs: []string{"/tmp"},
			ToolClasses: ToolClasses{
				Bash:        []string{"Bash"},
				Read:        []string{"Read", "Grep"},
				RuleBearing: []string{"mcp__*", "Skill"},
				Write:       []string{"Edit", "Write"},
			},
			UngatedTools:    []string{"Agent"},
			WrapperCommands: []string{"xargs"},
		},
		Worktreecreate: Worktreecreate{
			AcdslBuildTimeoutMs:   DurationMs(12 * time.Second),
			BranchPrefix:          "claude/",
			DefaultBranchFallback: []string{"main", "master"},
			NamePattern:           "^[a-z0-9-]+$",
			ResetAttempts:         5,
			ResetRetryDelayMs:     DurationMs(200 * time.Millisecond),
			ShortShaLength:        7,
			WorktreesDir:          ".claude/worktrees",
		},
		Worktreewriteguard: Worktreewriteguard{
			DenyTexts:   []string{"guard deny"},
			PoolSegment: "/.claude/worktrees/",
		},
	}
	return config
}

// writeConfig renders a complete config to <home>/hooks/config.json, the way the install does.
func writeConfig(t *testing.T, home string, config *Config) {
	t.Helper()
	writeDocument(t, home, configDocument(t, config))
}

func writeDocument(t *testing.T, home string, document map[string]map[string]any) {
	t.Helper()
	encoded, err := json.MarshalIndent(document, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Dir(Path(home)), 0o755))
	require.NoError(t, os.WriteFile(Path(home), encoded, 0o644))
}
