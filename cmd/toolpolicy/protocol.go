package main

// The gate's protocol facts: the user-facing command grammar (/smine-help documents it), the permission mode names Claude Code sends, the process-env names.
const (
	commandCheck  = "check"
	usageExitCode = 2
)

// The # command words of the user's own permission channel (docs/permissions.html).
const (
	wordAdd     = "#add"
	wordAllow   = "#allow"
	wordDeny    = "#deny"
	wordDisable = "#disable"
	wordEnable  = "#enable"
	wordReset   = "#reset"
	wordSet     = "#set"
)

// The two unlock words #allow takes.
const (
	unlockMakefile = "Makefile"
	unlockSettings = "settings.local.json"
)

// Permission modes as Claude Code names them.
const (
	permissionModeAcceptEdits = "acceptEdits"
	permissionModeAuto        = "auto"
	permissionModePlan        = "plan"
)

// Process-env facts the runners set.
const (
	headlessEnv      = "SMINE_HEADLESS"
	headlessEnvValue = "1"
)

// The toolpolicy command grammar and the field counts of the # message grammar.
const (
	addFieldCount      = 2
	approvalFieldCount = 2
	checkSkillFlag     = "--skill"
	commandAllow       = "allow"
	commandFreeze      = "freeze"
	commandHead        = "toolpolicy"
	commandRevert      = "revert"
	switchFieldCount   = 2
)

// Claude Code's tool names, hook events and hook decisions.
const (
	browserPanePrefix    = "mcp__Claude_Browser__"
	enterWorktreeTool    = "EnterWorktree"
	exitPlanModeToolName = "ExitPlanMode"
	exitWorktreeTool     = "ExitWorktree"
	mcpPrefix            = "mcp__"
	skillToolName        = "Skill"
	toolNameAgent        = "Agent"
	toolNameBash         = "Bash"
	toolNameEdit         = "Edit"
	toolNameNotebookEdit = "NotebookEdit"
	toolNameRead         = "Read"
	toolNameSkill        = "Skill"
	toolNameWebFetch     = "WebFetch"
	toolNameWebSearch    = "WebSearch"
	toolNameWorkflow     = "Workflow"
	toolNameWrite        = "Write"

	decisionAllow = "allow"
	decisionDeny  = "deny"
	decisionNone  = "none"

	hookEventDirectoryAdded    = "DirectoryAdded"
	hookEventPermissionRequest = "PermissionRequest"
	hookEventPostToolUse       = "PostToolUse"
	hookEventPreToolUse        = "PreToolUse"
	hookEventUserPromptSubmit  = "UserPromptSubmit"
)

// Claude Code's transcript layout and the settings file format, rule grammar included.
const (
	additionalDirectoriesKey = "additionalDirectories"
	allowKey                 = "allow"
	colonWildcard            = ":*"
	domainSpecPrefix         = "domain:"
	globMarker               = "*"
	pastedContentOpen        = "<pasted_content"
	pathRuleRecursiveSuffix  = "/**"
	permissionsKey           = "permissions"
	reposRuleRoot            = "@repos"
	sessionTempPrefix        = "claude-"
	subagentTranscriptPrefix = "agent-"
	subagentsDir             = "subagents"
	trailingWildcard         = " *"
	transcriptExtension      = ".jsonl"
	workflowsDir             = "workflows"

	// settingsLayersPerRoot: settings.json and settings.local.json.
	settingsLayersPerRoot = 2
)

// smine file formats the gate reads or writes: the deployed skill files, the ledger, the report line prefixes.
const (
	causeCheckReport     = "tool-policy: CHECK"
	causeFreezeApplied   = "tool-policy: FROZEN"
	causeSettingsApplied = "tool-policy: APPLIED"
	enforceFile          = "enforce.json"
	frontmatterFence     = "---"
	// variableReferencePrefix opens a skill variable reference in a deployed grant.
	variableReferencePrefix = "$SKILL_"
	// commandInterfaces is the handshake subcommand every provider answers.
	commandInterfaces    = "interfaces"
	keyAllowedTools      = "allowed-tools"
	ledgerDirMode        = 0o700
	ledgerFileMode       = 0o600
	localSettingsDirMode = 0o755
	planHookHeader       = "tool-policy /fimplement hook:"
	relayMarker          = "SMINE-PRIMER"
	reportPrefix         = "tool-policy "

	makeOperatorTag = "[operator]"
)

// The system paths the OS defines: read-only system trees, the device files every shell writes, the temp root.
const sessionTempBase = "/tmp"

var (
	// claimedInterfaces is what this build implements; TestProviderClaims pins it to skills/interfaces.cue.
	claimedInterfaces = []string{
		"permission-check@1",
		"permission-freeze@1",
	}

	systemReadDirs = []string{
		"/Applications/Xcode.app",
		"/Library/Developer",
		"/System",
		"/bin",
		"/dev",
		"/opt/homebrew",
		"/sbin",
		"/usr",
	}

	// systemWriteFiles are the device files every shell writes.
	systemWriteFiles = []string{"/dev/fd", "/dev/null", "/dev/stderr", "/dev/stdout", "/dev/tty"}
)

// Shell and coreutils grammar: command heads, wrappers, reserved variables, flag shapes.
const (
	envWrapper      = "env"
	findHead        = "find"
	globMetaChars   = `*?[\`
	goArgsSeparator = "-args"
	goHead          = "go"
	sedHead         = "sed"
	shortFlagLength = 2
	timeoutWrapper  = "timeout"
	xargsWrapper    = "xargs"

	directoryFlag  = "--directory"
	fileFlag       = "--file"
	outputFlag     = "--output"
	tarModeLetters = "AcrtuxzjJvfpPkKmOSWBhlLiC0123456789"
)

var (
	cdCommands = []string{"cd", "pushd"}
	// fenceOnlyWrappers run the next word as the command, like the configured wrapper commands.
	fenceOnlyWrappers = []string{"builtin", "exec"}
	// The fence resolves ~ and relative paths through these, so a fenced command never assigns them.
	fenceReservedVars     = []string{"CDPATH", "HOME", "OLDPWD", "PWD"}
	findExpressionOpeners = []string{"!", "(", ")"}
	findLeadingOptions    = []string{"-E", "-H", "-L", "-P", "-X", "-d", "-s", "-x"}
	// goEnvWriteFlags are the go env flags that persist changes to the GOENV file.
	goEnvWriteFlags = []string{"-w", "-u", "--write"}
	goProgramFlags  = []string{"exec", "toolexec", "vettool"}
	homeSpellings   = []string{"~/", "$HOME/", "${HOME}/"}
	tarLongModes    = map[string]string{"--append": "r", "--create": "c", "--extract": "x", "--get": "x", "--update": "u"}
	// wrapperValueFlags are wrapper options whose value is the next word.
	wrapperValueFlags = map[string][]string{
		envWrapper:     {"-u"},
		"nice":         {"-n"},
		timeoutWrapper: {"-k", "-s"},
		xargsWrapper:   {"-E", "-I", "-J", "-L", "-P", "-R", "-S", "-a", "-d", "-n", "-s"},
	}
)

// The engine stops path-checking these commands once they carry a flag.
var fileCommandSafeFlags = map[string]string{"cp": "HLPRXacfnprv"}

// make's command grammar.
const (
	makeDirectoryFlag = "--directory"
	makeHead          = "make"
	makeIncludeSuffix = ".mk"

	makeCodeShortFlags  = "EIef"
	makeValueShortFlags = "CWo"
)

var makeCodeLongFlags = []string{"--environment-overrides", "--eval", "--file", "--include-dir", "--makefile"}

// git's command grammar: the repository layout, the global options, the options and subcommands that run code.
const (
	configGitSegment         = "git"
	gitConfigOption          = "--config"
	gitDirName               = ".git"
	gitDirectoryFlag         = "-C"
	gitEnvironmentPrefix     = "GIT_"
	gitExecOption            = "--exec"
	gitFlagsEnd              = "--"
	gitHead                  = "git"
	gitIndexFileName         = "GIT_INDEX_FILE"
	gitOptionWithValueTokens = 2
	gitReceivePackOption     = "--receive-pack"
	gitSeparateDirOption     = "--separate-git-dir"
	gitShortOptionLength     = 2
	gitTemplateOption        = "--template"
	gitUploadPackOption      = "--upload-pack"
)

var (
	gitExecOptions = map[string][]string{
		"archive":    {gitExecOption},
		"clone":      {gitConfigOption, gitSeparateDirOption, gitTemplateOption, gitUploadPackOption, "-c", "-u"},
		"fetch":      {gitUploadPackOption},
		"fetch-pack": {gitExecOption, gitUploadPackOption},
		"grep":       {"--open-files-in-pager", "-O"},
		"init":       {gitSeparateDirOption, gitTemplateOption},
		"ls-remote":  {gitUploadPackOption},
		"pull":       {gitUploadPackOption},
		"push":       {gitExecOption, gitReceivePackOption},
		"rebase":     {gitExecOption, "-x"},
		"send-pack":  {gitExecOption, gitReceivePackOption},
	}

	gitExecSubcommands = [][]string{
		{"bisect", "run"},
		{"difftool"},
		{"filter-branch"},
		{"mergetool"},
		{"submodule", "foreach"},
	}

	gitBranchMutatingFlags = []string{
		"-c", "-C", "--copy",
		"-d", "-D", "--delete",
		"-f", "--force",
		"-m", "-M", "--move",
		"-u", "--set-upstream-to", "--unset-upstream",
		"--edit-description",
	}

	gitBranchListingFlags = []string{
		"-a", "--all", "-l", "--list", "-r", "--remotes", "-v", "-vv", "--verbose",
		"--show-current", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at",
	}
)

// Internal labels and model bounds of the gate, never shown as configuration: no catalog row exists for them (plan gap).
const (
	anchorSourceEnvelope   = "envelope"
	anchorSourceSubagent   = "subagent-transcript"
	anchorSourceTranscript = "transcript"
	loopIterationsModeled  = 2
	pipelineSegmentCount   = 2
	rootsBeyondDirectories = 2
)

// A protocol fact: the deny hands the agent this script verbatim and isRelayPrimer admits only it, so both sides must byte-match.
const relayPrimerScript = `export const meta = { name: 'relay-primer', description: 'SMINE-PRIMER — opens a notification turn' }
await new Promise(r => setTimeout(r, 10000))
return 'primed'`

// Claude Code's own EnterWorktree location, which holds while the worktrees component renders no worktrees_dir.
const harnessWorktreesDir = ".claude/worktrees"
