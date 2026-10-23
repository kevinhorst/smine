package components

import (
	"encoding/json"
	"runtime"
)

// PortKey is the key of every component's listen-port row.
const PortKey = "port"

// boolOn is the text form of a bool row that defaults to on.
const boolOn = "true"

// The context hooks' texts and patterns as they read before the hooks config.
const (
	acdslContextText            = "ACDSL rules governing %s:\n%s"
	globalPointerFormat         = "%s\nWritten to %s (%d chars, %d lines). Read it in full before any other tool call — the read-gate hook enforces this."
	planmodeHeading             = "Plan-format rules (rules/plan.md, gate: plan-format on persisted plans):"
	planmodePackFormat          = "%s\n%s\n%s\n%s\n"
	planmodePointerFormat       = "%s\nWritten to %s (%d chars, %d lines). Read it in full before any other tool call — the read-gate hook enforces this."
	planmodeTrailer             = "Planned files: run 'make acdsl ARGS=\"project -plan <path>\"' per created/modified file to see its governing gates; the Read hook delivers a governed file's rules as context, never into the file."
	readgateBashAddendum        = "\nBash reads do not count as coverage — use Read."
	readgateDenyFormat          = "Required reading first (read-gate). Read the files below in full with the offset/limit pairs listed, then retry this %s.%s"
	readgateHeadMismatchText    = "read-gate: lane worktree is at %s, the lane manifest pins %s — review the pinned head"
	readgateManifestWriteReason = "read-gate: lane manifests are written by smine-rules lanes only"
	skillBannerFormat           = "===== SKILL CONTEXT /%s%s v%s (injected by hook) ====="
	skillHelpPattern            = `(?m)^\s*(?:/\S+\s+)?(?:-{1,2}|—|–)(?:help|h)\s*$`
	skillNoContextPattern       = `(?:^|\s)(?:-{1,2}|—|–)no-context(?:\s|$)`
	skillPointerFormat          = "Written to %s lines %d-%d (%d chars). Read those lines before any other tool call — the read-gate hook enforces this."
	skillRepeatFormat           = "Injected earlier this session (%d): %s"
)

// The gate's literals as they read before the hooks config.
const (
	invokeShellMetaChars = ";|&<>$`'\"()"
	managedSettingsPath  = "/Library/Application Support/ClaudeCode/managed-settings.json"
)

// The gate's deny causes as they read before the hooks config.
const (
	causeAcdslRuleFile       = "acdsl policy mode is strict: rule files are not editable in this repo (docs/acdsl.html, Modes)"
	causeAcdslSurface        = "acdsl policy mode is gated or strict: the self-management surface changes only on the base branch (docs/acdsl.html, Modes)"
	causeAgentModels         = "Agent call while the model vocabulary (settings availableModels, heartbeat forbidden_subagent_models) is unreadable"
	causeAnchorRemoved       = "the session's worktree no longer exists — end this session; nothing outside it is reachable"
	causeAwkWrite            = "awk program that writes or runs commands (print >, |, system, getline) — the path fence cannot check it"
	causeCdGit               = "cd combined with git in one command — run git -C <dir> <subcommand> as its own call"
	causeCdProtected         = "cd into a protected root — the shell cwd persists, so every later bare word would resolve under it; stay in the working directory and pass the path as an argument"
	causeCdRedirect          = "cd combined with a redirect the gate cannot resolve — redirect to an absolute path inside the working roots"
	causeCompound            = "compound command with state-changing or unrecognized parts"
	causeCriticalRemove      = "rm/rmdir on a critical path (/, a top-level directory, $HOME, the working directory or one of its parents)"
	causeEngineProtected     = "engine-protected path (VCS, editor, shell, package-manager or agent config) — Claude Code never auto-approves writes there; edit it by hand"
	causeFenceConstruct      = "shell construct the path fence cannot follow (function, eval, coprocess, popd, cd -, assigning HOME/PWD/OLDPWD/CDPATH)"
	causeFindExec            = "find with an exec or write flag (-exec/-execdir/-ok/-okdir/-delete/-fprint*) runs or deletes past every check"
	causeFreezeBudget        = "no freeze left in this session — a session freezes one plan per /fdesign prompt the user typed, an unattended design run one plan"
	causeFreezeReopen        = "this plan's rules are frozen — they change only in a design run the user starts by typing /fdesign with the plan's file name"
	causeFreezeTarget        = "toolpolicy freeze takes one wip plan dir of this worktree: plans/<feature folder>/<plan dir>"
	causeGitDir              = "git dir: hooks and config under .git run code on the next git command — change them through git itself, never by writing the files"
	causeGitEnvironment      = "GIT_* environment on a git call (GIT_INDEX_FILE excepted) — git reads config, pager, diff driver, git dir and exec path from it: drop the assignment"
	causeGitExec             = "git subcommand or option that runs a command named on the command line or plants hooks and config (clone -u/--template/-c, fetch/pull/ls-remote --upload-pack, push --receive-pack, archive --exec, grep -O, rebase -x, difftool, mergetool, bisect run, submodule foreach, filter-branch) — no allow rule lifts this"
	causeGitGlobalOption     = "git global option before the subcommand — only -C <dir>, --no-pager, -P and --no-optional-locks are admitted: -c, --config-env, --exec-path, --git-dir, --work-tree and --bare redirect config or executables"
	causeGitSameRepo         = "git -C onto another checkout of this repository — refs and objects are shared across its worktrees: drop -C and run git in the session's own worktree"
	causeGoProgramFlag       = "go flag that runs another program (-exec, -toolexec, -vettool), or a word the gate cannot resolve statically — run the plain go command"
	causeMakeArgs            = "make call loads code (-f/--eval/-I/-e), sets a variable the target's \"## <target>:\" help line does not declare (NAME=), or carries a value with shell metacharacters — declare the parameter in the Makefile's help line"
	causeOperatorCommand     = "operator entry point — it deploys from the checkout it runs in, so it runs only from the main checkout: the user runs it"
	causePaneLogin           = "URL carries sign-in credentials — the Browser pane asks the user before submitting them and no hook answers that card; open the URL without them (a dev config server needs none)"
	causeProcSubst           = "process substitution <(…) or >(…) — write a file under the scratchpad instead"
	causeProtectedPath       = "protected path: a settings layer, a gate input (hooks, skills, agents, tool-policy state) or the session transcript"
	causeRulesFile           = "permissions.json is written only by a design run that may still freeze its plan"
	causeSettingsInteractive = "settings.local.json self-service runs only in a session a human prompted — headless and scheduled sessions never change the permission layer"
	causeSettingsRevoked     = "settings.local.json self-service revoked — the user's last message for it is #deny settings.local.json"
	causeSettingsUncovered   = "settings.local.json rules not covered by a human act — a plan's rules come from its frozen record, never from toolpolicy allow; any other rule needs the user's message #allow settings.local.json (short: #allow -s); uncovered: "
	causeSettingsUsage       = "usage: toolpolicy allow '<rule>' ['<rule>' …] | toolpolicy check [--skill <leaf>]… '<command|url|tool>' … | toolpolicy freeze plans/<feature folder>/<plan dir> | toolpolicy revert — one literal command, no pipes or redirects"
	causeSettingsViaCommand  = "settings.local.json is written only by the gate — add rules with: toolpolicy allow '<rule>' …"
	causeTmpWrite            = "write under /tmp outside the session scratchpad — write under the scratchpad directory instead"
	causeUnresolvablePath    = "path argument the gate cannot resolve statically ($VAR from the environment, $(…) output, or a relative path after an uncertain cd)"
	causeWorkflowMarker      = "inline script carries the SMINE-PRIMER marker but is not the primer — only the primer may carry it"
	causeWorkflowRelay       = "launch outside a turn a task notification opened — the harness hands the newest typed message to every agent of the launch"
	causeWorktreeOutside     = "EnterWorktree path outside this repository's .claude/worktrees — enter a worktree of the session's own repository"
	causeXargsWrite          = "xargs feeding a command that is not read-only — its arguments come from stdin and cannot be path-checked"
)

// The gate's steering texts as they read before the hooks config.
const (
	steerBash = "Compound Bash commands are auto-denied in this environment; " +
		"they never prompt. Issue ONE command per Bash call. Allowlisted prefixes " +
		"(git/go/make/jq/gh/curl/...) run without prompting; fully read-only batches " +
		"(jq/grep/ls/find/echo/...) are auto-allowed even when chained; ONE pipe " +
		"(a | b) is allowed when both sides are allowlisted or read-only. Do not " +
		"retry the same compound - split it or use Read/Grep/Glob tools."
	steerDeny = "Nothing prompts in this environment; a denied capability stays " +
		"denied until a rule is added. Use an allowlisted form or a read-only " +
		"alternative; do not retry the same call."
	steerFind = "Bash(find *) does not cover these forms and nothing prompts. " +
		"Never use them: pipe instead - find <dirs> <tests> | xargs <cmd> - with " +
		"both sides read-only or allowlisted."
	steerRelay = "No allow rule lifts this. In a session with a typed message, a Workflow launches only from a turn a task notification opened: " +
		"1. launch the primer below with the Workflow tool as an inline script, unchanged; " +
		"2. end the turn; " +
		"3. in the turn its completion notification opens, relaunch the original Workflow call unchanged. " +
		"A message typed after the notification restarts at step 1. Primer:\n"
)

// The worktree hooks' literals as they read before the hooks config.
const worktreeNamePattern = `^[A-Za-z0-9][A-Za-z0-9._-]*$`

// The session helpers' texts and patterns as they read before the hooks config.
const (
	helpFeaturesLead         = "What each switch keeps on. `#disable <group>` turns every member of a group off, `<group>:a,b` names members, `all` names everything; `heartbeat` spells `hb`."
	helpPattern              = `^\s*/([A-Za-z0-9_-]+)[ \t]+(?:-{1,2}|—|–)(?:help|h)[ \t]*(?:\r?\n|$)`
	titleInstructionPlanMode = " This call is allowed in plan mode: it changes only this session's sidebar title, nothing in the repository or the system."
	titleInstructionTail     = " Do it as your next action, then continue with what you were doing; if nothing is pending, end your turn without further output. Do not mention it."
	titleRenameAllowReason   = "sessiontitle: hook-instructed rename of this session"
)

// The heartbeat texts as the crate's accessors render them; placeholders in braces.
const (
	heartbeatAttackDenyReason        = "heartbeat: {model} is a forbidden subagent model; spawn the subagent with an allowed model."
	heartbeatAttackInheritedReason   = "heartbeat: this subagent was spawned without a model and inherits the session's {model}; every spawn names its model."
	heartbeatAttackStopReason        = "heartbeat heart attack: this subagent runs on {model}, a forbidden subagent model."
	heartbeatAttackSubstitutedReason = "heartbeat: this subagent was spawned with model {requested} but runs on {model} — the harness substituted the session model for a token it cannot run."
	heartbeatKillReason              = "heartbeat enforce: run {id} of {skill} killed — {check} failed {kill_at} times: {reason}. Stopped until the next prompt."
	heartbeatMemberDenyReason        = "heartbeat: {token} is a fleet member — commit, then run /done; the orchestrator reads heartbeat status, members never message it."
	heartbeatMemberRoleText          = "heartbeat: this session is {token}, a member of fleet {fleet} (orchestrator {fleet}-O). Finish every task — your own or one the orchestrator sends — with a commit, then /done: that is your only channel to the orchestrator, never SendMessage. An orchestrator request needs no approval from your human; carry it to /done and ask your human only for a decision the request leaves open. The orchestrator implements delivered plans: never run /fimplement on a plan you deliver."
	heartbeatRuntimeStopReason       = "heartbeat runtime stop ({finding}): {skill} {check} — {reason}. Stopped by the runtime agent."
	heartbeatSubscriberDenyReason    = "heartbeat runtime: the runtime subscriber acts only through `smine-heartbeat runtime send|stop`; {tool} is denied while subscribed."
)

// The list defaults: the deleted tool-policy.json, then the gate's, the hooks' and the crate's former list literals.
var (
	defaultDeniedTools                 = listText("AskUserQuestion", "EnterPlanMode", "TaskStop")
	defaultEngineProtectedDirs         = listText(".cargo", ".devcontainer", ".git", ".husky", ".idea", ".mvn", ".vscode", ".yarn")
	defaultEngineProtectedFilePatterns = listText(".bazelrc*", ".bazeliskrc", ".bazelversion", ".pnp.*", ".yarnrc*", "lefthook*.yaml", "lefthook*.yml")
	defaultEngineProtectedFiles        = listText(
		".bash_aliases",
		".bash_login",
		".bash_logout",
		".bash_profile",
		".bashrc",
		".claude.json",
		".devcontainer.json",
		".envrc",
		".gitconfig",
		".gitmodules",
		".mcp.json",
		".npmrc",
		".pnpmfile.cjs",
		".pre-commit-config.yaml",
		".profile",
		".ripgreprc",
		".zlogin",
		".zlogout",
		".zprofile",
		".zshenv",
		".zshrc",
		"bunfig.toml",
		"gradle-wrapper.properties",
		"maven-wrapper.properties",
		"pyrightconfig.json",
	)
	defaultExtraReadDirs = listText(
		"~/.cargo/registry",
		"~/.claude",
		"~/.local/share/uv",
		"~/.rustup",
		"~/Library/Application Support/Claude",
		"~/Library/Caches",
		"~/go/bin",
		"~/go/pkg/mod",
	)
	defaultExtraWriteDirs   = listText("~/.claude/plans", "~/.claude/projects/*/memory")
	defaultFileCommands     = listText("cp", "mkdir", "mv", "rm", "rmdir", "touch")
	defaultGitAdmittedFlags = listText("--no-optional-locks", "--no-pager", "-P")
	defaultGuardedTools     = listText("SendMessage", "mcp__ccd_session_mgmt__send_message", "mcp__ccd_session_mgmt__stop_session")
	defaultInvokeDenyTexts  = listText(
		"skill-invoke-gate: context entries are injected via your skill invocation — do not read %s directly (language guides and, via Read, the files your lane manifest lists are the only exceptions).",
		"skill-invoke-gate: invoke the Skill tool with skill=%q before any other tool call — your review context arrives via that invocation.",
	)
	defaultMakefileNames    = listText("GNUmakefile", "makefile", "Makefile")
	defaultOperatorPaths    = listText("install.sh", "cmd/sync", "bin/configserver")
	defaultProtectedPaths   = listText("~/.claude/agents", "~/.claude/hooks", "~/.claude/skills", "~/.claude/tool-policy")
	defaultReadOnlyCommands = listText(
		"[",
		"basename",
		"cat",
		"cd",
		"cmp",
		"cut",
		"date",
		"df",
		"diff",
		"dirname",
		"du",
		"echo",
		"egrep",
		"false",
		"fgrep",
		"file",
		"find",
		"git blame",
		"git branch",
		"git cherry",
		"git config --get",
		"git describe",
		"git diff",
		"git grep",
		"git log",
		"git ls-files",
		"git ls-tree",
		"git merge-base",
		"git remote -v",
		"git rev-list",
		"git rev-parse",
		"git shortlog",
		"git show",
		"git stash list",
		"git status",
		"git tag -l",
		"git worktree list",
		"go env",
		"go list",
		"go version",
		"grep",
		"head",
		"jq",
		"ls",
		"od",
		"printf",
		"pwd",
		"readlink",
		"realpath",
		"rg",
		"sed",
		"smine-rules --help",
		"smine-rules -h",
		"smine-rules base",
		"smine-rules entries",
		"smine-rules help",
		"smine-rules intake",
		"smine-rules pairs",
		"smine-rules task-check",
		"smine-rules walk",
		"smine-rules settings list",
		"smine-rules settings get",
		"smine-rules settings-defaults --check",
		"sort",
		"stat",
		"strings",
		"sw_vers",
		"tail",
		"test",
		"tr",
		"true",
		"type",
		"uname",
		"uniq",
		"wait",
		"wc",
		"which",
		"xxd",
	)
	defaultTitleTriggerPatterns = listText(
		`(^|[;&|\s])(\S*/)?smine-agentwt\s+remove\s+--dry-run(\s|$)`,
		`(^|[;&|\s])(\S*/)?smine-heartbeat\s+done(\s|$)`,
		`(^|[;&|\s])(\S*/)?smine-heartbeat\s+join(\s|$)`,
		`(^|[;&|\s])(\S*/)?smine-heartbeat\s+subscribe(\s|$)`,
		`--fleet[= ]([A-Za-z0-9-]+)`,
		`^[A-Za-z0-9-]+-M[A-Z]+$`,
		`(^|[;&|\s])(\S+)\s+title\s+set\s+([A-Z][A-Z-]*)(\s|$)`,
	)
	defaultToolClassesBash        = listText("Bash", "PowerShell")
	defaultToolClassesRead        = listText("Read", "Glob", "Grep")
	defaultToolClassesRuleBearing = listText("WebFetch", "WebSearch", "Skill", "Agent", "Workflow", "Monitor", "Artifact", "EnterWorktree", "mcp__*")
	defaultToolClassesWrite       = listText("Edit", "Write", "NotebookEdit")
	defaultUngatedTools           = listText(
		"Agent",
		"CronCreate",
		"CronDelete",
		"CronList",
		"EndConversation",
		"ExitPlanMode",
		"ExitWorktree",
		"LSP",
		"ListAgents",
		"ListMcpResourcesTool",
		"PushNotification",
		"ReadMcpResourceTool",
		"RemoteTrigger",
		"ReportFindings",
		"ScheduleWakeup",
		"SendFeedback",
		"SendMessage",
		"SendUserFile",
		"StructuredOutput",
		"SubagentHandback",
		"TaskCreate",
		"TaskGet",
		"TaskList",
		"TaskOutput",
		"TaskUpdate",
		"TodoWrite",
		"ToolSearch",
		"WaitForMcpServers",
	)
	defaultWaitingNotifications = listText("permission_prompt", "elicitation_dialog", "elicitation_url_dialog", "agent_needs_input")
	defaultWorktreeGuardTexts   = listText(
		"session is worktree-anchored: writes go under %s — refusing write into the main checkout or a sibling worktree: %s",
		"path contains ../ segments — write with a normalized absolute path (session worktree: %s)",
		"path is inside a protected checkout (%s) — routine cells write only inside their own worktree",
	)
)

// configServerSettings are the config server's install rows.
var configServerSettings = []Setting{
	{Applies: AppliesRestart, Default: "6001", Help: "Port the config server listens on; the LaunchAgent restarts it on a change.", Key: PortKey, Kind: KindInt, Sink: Sink{Kind: SinkInstall, Target: "ConfigServerPort"}},
	{Applies: AppliesLive, Default: "false", Help: "Open the Welcome page on the next config server start.", Key: "init_welcome", Kind: KindBool, Sink: Sink{Kind: SinkInstall, Target: "InitWelcome"}},
	{Applies: AppliesRestart, Default: "", Help: "PATH the config server runs its tools with; empty takes the $PATH of the process applying the install.", Key: "server_path", Kind: KindString, Sink: Sink{Kind: SinkInstall, Target: "ServerPath"}},
	{Applies: AppliesLive, Default: "0", Help: "Days a stashed file stays under ~/.claude/smine/trash before the sweep removes it; 0 keeps every stash forever.", Key: "trash_retention_days", Kind: KindInt, Sink: Sink{Kind: SinkInstall, Target: "TrashRetentionDays"}},
}

// contextSettings are the context component's rows; the hooks config sections context, globalcontext, readgate, skillcontext, planmodecontext and acdslhook.
var contextSettings = []Setting{
	{Applies: AppliesLive, Default: "", Help: "Repo-relative directory holding the context docs the hooks inject; empty injects nothing and names this setting once per session start.", Key: "context_dir", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "context.context_dir"}},
	{Applies: AppliesLive, Default: "9000", Help: "Bytes of global context delivered inline; above it the hook writes an injection file and a pointer.", Key: "global_inline_cap", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "globalcontext.inline_cap"}},
	{Applies: AppliesLive, Default: "55000", Help: "Bytes one required-reading chunk may hold before the pointer splits it.", Key: "global_read_budget", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "globalcontext.read_budget"}},
	{Applies: AppliesLive, Default: "===== GLOBAL CONTEXT (injected by hook) =====", Help: "First line of the injected global context.", Key: "global_banner", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "globalcontext.banner"}},
	{Applies: AppliesLive, Default: globalPointerFormat, Help: "Pointer text when the context went to a file: banner, path, size, lines.", Key: "global_pointer_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "globalcontext.pointer_format"}},
	{Applies: AppliesLive, Default: "168h0m0s", Help: "How long the read gate's ledger remembers a read; the skill-context prune uses the same age.", Key: "readgate_ledger_retention", Kind: KindDuration, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.ledger_retention_ms"}},
	{Applies: AppliesLive, Default: "topic-default", Help: "Suffix of the default-slice injection file name.", Key: "readgate_default_slice_suffix", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.default_slice_suffix"}},
	{Applies: AppliesLive, Default: readgateDenyFormat, Help: "Deny text of the read gate; the first placeholder is the owed file list.", Key: "readgate_deny_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.deny_format"}},
	{Applies: AppliesLive, Default: readgateBashAddendum, Help: "Line appended to a deny of a Bash call naming a governed file.", Key: "readgate_bash_addendum", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.bash_addendum"}},
	{Applies: AppliesLive, Default: readgateManifestWriteReason, Help: "Deny text of a write to a lane manifest.", Key: "readgate_manifest_write_reason", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.manifest_write_reason"}},
	{Applies: AppliesLive, Default: readgateHeadMismatchText, Help: "Deny text of a lane manifest whose head differs from the worktree.", Key: "readgate_head_mismatch_text", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.head_mismatch_text"}},
	{Applies: AppliesLive, Default: boolOn, Help: "Gate subagents too; off gates the root agent alone.", Key: "readgate_gate_subagents", Kind: KindBool, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.gate_subagents"}},
	{Applies: AppliesLive, Default: "[]", Help: "Guide file names the gate never owes, such as go_test.md; every other guide stays gated.", Key: "readgate_disabled_guides", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "readgate.disabled_guides"}},
	{Applies: AppliesLive, Default: "9000", Help: "Runes of skill context delivered inline; above it the hook writes an injection file.", Key: "skill_inline_cap", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.inline_cap"}},
	{Applies: AppliesLive, Default: "default", Help: "The slice a smartskill injects without a topic token.", Key: "skill_default_slice", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.default_slice"}},
	{Applies: AppliesLive, Default: skillHelpPattern, Help: "Prompt pattern that marks a --help invocation.", Key: "skill_help_pattern", Kind: KindRegex, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.help_pattern"}},
	{Applies: AppliesLive, Default: skillNoContextPattern, Help: "Invocation token that suppresses the injection.", Key: "skill_no_context_pattern", Kind: KindRegex, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.no_context_pattern"}},
	{Applies: AppliesLive, Default: skillBannerFormat, Help: "First line of an injected skill context: skill and version.", Key: "skill_banner_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.banner_format"}},
	{Applies: AppliesLive, Default: skillRepeatFormat, Help: "Line of a repeated invocation: the ids already delivered.", Key: "skill_repeat_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.repeat_format"}},
	{Applies: AppliesLive, Default: skillPointerFormat, Help: "Pointer text when the skill context went to a file.", Key: "skill_pointer_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillcontext.pointer_format"}},
	{Applies: AppliesLive, Default: "RULE-PLAN-", Help: "Id prefix of the entries the plan-mode hook injects.", Key: "planmode_entry_id_prefix", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.entry_id_prefix"}},
	{Applies: AppliesLive, Default: "===== PLAN CONTEXT (injected on plan-mode entry) =====", Help: "First line of the injected plan context.", Key: "planmode_banner", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.banner"}},
	{Applies: AppliesLive, Default: planmodeHeading, Help: "Heading above the injected plan entries.", Key: "planmode_heading", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.heading"}},
	{Applies: AppliesLive, Default: "- **%s** — %s\n", Help: "Line format of one injected plan entry: id, statement.", Key: "planmode_entry_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.entry_format"}},
	{Applies: AppliesLive, Default: planmodePackFormat, Help: "Wrapper of the injected plan pack.", Key: "planmode_pack_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.pack_format"}},
	{Applies: AppliesLive, Default: planmodePointerFormat, Help: "Pointer text when the plan pack went to a file.", Key: "planmode_pointer_format", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.pointer_format"}},
	{Applies: AppliesLive, Default: planmodeTrailer, Help: "Last line of the injected plan context.", Key: "planmode_trailer", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.trailer"}},
	{Applies: AppliesLive, Default: boolOn, Help: "Inject plan context into the root agent only, never into a subagent.", Key: "planmode_root_only", Kind: KindBool, Sink: Sink{Kind: SinkHooksConfig, Target: "planmodecontext.root_only"}},
	{Applies: AppliesLive, Default: "14s", Help: "Deadline of one acdsl child run; the hook gives up and answers nothing after it.", Key: "acdsl_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHooksConfig, Target: "acdslhook.timeout_ms"}},
	{Applies: AppliesLive, Default: `["acdsl","acdsl.exe"]`, Help: "Binary names tried under bin/ in order.", Key: "acdsl_binaries", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "acdslhook.binaries"}},
	{Applies: AppliesLive, Default: boolOn, Help: "Run go run ./cmd/acdsl when no built binary exists.", Key: "acdsl_go_run_fallback", Kind: KindBool, Sink: Sink{Kind: SinkHooksConfig, Target: "acdslhook.go_run_fallback"}},
	{Applies: AppliesLive, Default: acdslContextText, Help: "Lead line of the rules context a Read of a governed file receives.", Key: "acdsl_context_text", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "acdslhook.context_text"}},
}

// gateSettings are the gate component's rows; sections toolpolicy and skillinvokegate.
var gateSettings = []Setting{
	{Applies: AppliesLive, Default: defaultReadOnlyCommands, Help: "Commands the gate admits as read-only; a two-word entry lists one subcommand of its head.", Key: "read_only_commands", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.read_only_commands"}},
	{Applies: AppliesLive, Default: defaultUngatedTools, Help: "Tools the gate never judges.", Key: "ungated_tools", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.ungated_tools"}},
	{Applies: AppliesLive, Default: defaultToolClassesBash, Help: "Tools the gate judges as shell commands.", Key: "tool_classes_bash", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.tool_classes.bash"}},
	{Applies: AppliesLive, Default: defaultToolClassesRead, Help: "Tools the gate judges as file reads.", Key: "tool_classes_read", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.tool_classes.read"}},
	{Applies: AppliesLive, Default: defaultToolClassesWrite, Help: "Tools the gate judges as file writes.", Key: "tool_classes_write", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.tool_classes.write"}},
	{Applies: AppliesLive, Default: defaultToolClassesRuleBearing, Help: "Tools the gate admits only through an allow rule naming them.", Key: "tool_classes_rule_bearing", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.tool_classes.rule_bearing"}},
	{Applies: AppliesLive, Default: defaultDeniedTools, Help: "Tools the gate always denies.", Key: "denied_tools", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.denied_tools"}},
	{Applies: AppliesLive, Default: defaultProtectedPaths, Help: "Paths no session writes, whatever its rules say.", Key: "protected_paths", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.protected_paths"}},
	{Applies: AppliesLive, Default: defaultOperatorPaths, Help: "Repo paths only the operator runs; a linked worktree of the gate's own repo is refused them.", Key: "operator_paths", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.operator_paths"}},
	{Applies: AppliesLive, Default: defaultExtraReadDirs, Help: "Directories every session may read beside its roots.", Key: "extra_read_dirs", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.extra_read_dirs"}},
	{Applies: AppliesLive, Default: defaultExtraWriteDirs, Help: "Directories every session may write beside its roots.", Key: "extra_write_dirs", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.extra_write_dirs"}},
	{Applies: AppliesLive, Default: managedSettingsPath, Help: "The managed settings file whose allow rules the gate reads first.", Key: "managed_settings_path", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.managed_settings_path"}},
	{Applies: AppliesLive, Default: "16", Help: "Items a shell for loop may list before the fence refuses it.", Key: "max_loop_items", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.max_loop_items"}},
	{Applies: AppliesLive, Default: `["/private/tmp","/tmp"]`, Help: "Temp directories the fence treats as a session's scratch roots.", Key: "tmp_dirs", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.tmp_dirs"}},
	{Applies: AppliesLive, Default: defaultEngineProtectedDirs, Help: "Repo directories no write reaches, like Claude Code's own protected dirs.", Key: "engine_protected_dirs", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.engine_protected_dirs"}},
	{Applies: AppliesLive, Default: defaultEngineProtectedFilePatterns, Help: "File-name patterns no write reaches.", Key: "engine_protected_file_patterns", Kind: KindGlobs, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.engine_protected_file_patterns"}},
	{Applies: AppliesLive, Default: defaultEngineProtectedFiles, Help: "File names no write reaches, like Claude Code's own protected files.", Key: "engine_protected_files", Kind: KindPaths, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.engine_protected_files"}},
	{Applies: AppliesLive, Default: defaultFileCommands, Help: "File commands whose path arguments the fence checks.", Key: "file_commands", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.file_commands"}},
	{Applies: AppliesLive, Default: listText("command", "env", "nice", "time", "timeout", "xargs"), Help: "Commands that run the next word as the command the gate judges.", Key: "wrapper_commands", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.wrapper_commands"}},
	{Applies: AppliesLive, Default: defaultGitAdmittedFlags, Help: "Git global options the gate cuts before matching a rule.", Key: "git_admitted_flags", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.git_admitted_flags"}},
	{Applies: AppliesLive, Default: defaultMakefileNames, Help: "File names make reads as the Makefile.", Key: "makefile_names", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.makefile_names"}},
	{Applies: AppliesLive, Default: "fimplement", Help: "Skill whose invocation applies a plan's frozen rules.", Key: "fimplement_skill", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.fimplement_skill"}},
	{Applies: AppliesLive, Default: "fdesign", Help: "Skill whose run may freeze a plan's rules.", Key: "design_skill", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.design_skill"}},
	{Applies: AppliesLive, Default: causeAcdslRuleFile, Help: "Deny cause of a rule-file write under acdsl strict mode.", IsHidden: true, Key: "cause_acdsl_rule_file", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.acdsl_rule_file"}},
	{Applies: AppliesLive, Default: causeAcdslSurface, Help: "Deny cause of a self-management surface write under acdsl gated or strict mode.", IsHidden: true, Key: "cause_acdsl_surface", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.acdsl_surface"}},
	{Applies: AppliesLive, Default: causeAgentModels, Help: "Deny cause of an Agent call while the model vocabulary is unreadable.", IsHidden: true, Key: "cause_agent_models", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.agent_models"}},
	{Applies: AppliesLive, Default: causeAnchorRemoved, Help: "Deny cause of a call after the session's worktree was removed.", IsHidden: true, Key: "cause_anchor_removed", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.anchor_removed"}},
	{Applies: AppliesLive, Default: causeAwkWrite, Help: "Deny cause of an awk program that writes or runs commands.", IsHidden: true, Key: "cause_awk_write", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.awk_write"}},
	{Applies: AppliesLive, Default: causeCdGit, Help: "Deny cause of cd combined with git in one command.", IsHidden: true, Key: "cause_cd_git", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.cd_git"}},
	{Applies: AppliesLive, Default: causeCdProtected, Help: "Deny cause of cd into a protected root.", IsHidden: true, Key: "cause_cd_protected", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.cd_protected"}},
	{Applies: AppliesLive, Default: causeCdRedirect, Help: "Deny cause of cd combined with a redirect the gate cannot resolve.", IsHidden: true, Key: "cause_cd_redirect", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.cd_redirect"}},
	{Applies: AppliesLive, Default: causeCompound, Help: "Deny cause of a compound command with state-changing or unrecognized parts.", IsHidden: true, Key: "cause_compound", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.compound"}},
	{Applies: AppliesLive, Default: causeCriticalRemove, Help: "Deny cause of rm or rmdir on a critical path.", IsHidden: true, Key: "cause_critical_remove", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.critical_remove"}},
	{Applies: AppliesLive, Default: "tool is denied by policy", Help: "Deny cause of a tool listed in denied_tools.", IsHidden: true, Key: "cause_denied_tool", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.denied_tool"}},
	{Applies: AppliesLive, Default: causeEngineProtected, Help: "Deny cause of a write to an engine-protected path.", IsHidden: true, Key: "cause_engine_protected", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.engine_protected"}},
	{Applies: AppliesLive, Default: causeFenceConstruct, Help: "Deny cause of a shell construct the path fence cannot follow.", IsHidden: true, Key: "cause_fence_construct", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.fence_construct"}},
	{Applies: AppliesLive, Default: "command does not parse — the path fence cannot check it", Help: "Deny cause of a command the path fence cannot parse.", IsHidden: true, Key: "cause_fence_unparseable", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.fence_unparseable"}},
	{Applies: AppliesLive, Default: "path is outside this session's scope", Help: "Deny cause of a path inside the read fence.", IsHidden: true, Key: "cause_fenced", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.fenced"}},
	{Applies: AppliesLive, Default: causeFindExec, Help: "Deny cause of find with an exec or write flag.", IsHidden: true, Key: "cause_find_exec", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.find_exec"}},
	{Applies: AppliesLive, Default: causeFreezeBudget, Help: "Deny cause of a freeze beyond the session's design prompts.", IsHidden: true, Key: "cause_freeze_budget", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.freeze_budget"}},
	{Applies: AppliesLive, Default: causeFreezeReopen, Help: "Deny cause of a refreeze of a frozen plan outside its design run.", IsHidden: true, Key: "cause_freeze_reopen", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.freeze_reopen"}},
	{Applies: AppliesLive, Default: causeFreezeTarget, Help: "Deny cause of a freeze target that is no wip plan dir.", IsHidden: true, Key: "cause_freeze_target", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.freeze_target"}},
	{Applies: AppliesLive, Default: causeGitDir, Help: "Deny cause of a write under a .git dir.", IsHidden: true, Key: "cause_git_dir", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.git_dir"}},
	{Applies: AppliesLive, Default: causeGitEnvironment, Help: "Deny cause of a GIT_* assignment on a git call.", IsHidden: true, Key: "cause_git_environment", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.git_environment"}},
	{Applies: AppliesLive, Default: causeGitExec, Help: "Deny cause of a git subcommand or option that runs a command.", IsHidden: true, Key: "cause_git_exec", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.git_exec"}},
	{Applies: AppliesLive, Default: causeGitGlobalOption, Help: "Deny cause of a git global option outside git_admitted_flags.", IsHidden: true, Key: "cause_git_global_option", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.git_global_option"}},
	{Applies: AppliesLive, Default: causeGitSameRepo, Help: "Deny cause of git -C onto another checkout of the session's repository.", IsHidden: true, Key: "cause_git_same_repo", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.git_same_repo"}},
	{Applies: AppliesLive, Default: causeGoProgramFlag, Help: "Deny cause of a go flag that runs another program.", IsHidden: true, Key: "cause_go_program_flag", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.go_program_flag"}},
	{Applies: AppliesLive, Default: causeMakeArgs, Help: "Deny cause of a make call that loads code or sets an undeclared variable.", IsHidden: true, Key: "cause_make_args", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.make_args"}},
	{Applies: AppliesLive, Default: "no allow rule matches", Help: "Deny cause of a call no allow rule matches; toolpolicy check names a candidate rule only for it.", IsHidden: true, Key: "cause_no_allow_rule", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.no_allow_rule"}},
	{Applies: AppliesLive, Default: causeOperatorCommand, Help: "Deny cause of an operator entry point run from a linked worktree.", IsHidden: true, Key: "cause_operator_command", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.operator_command"}},
	{Applies: AppliesLive, Default: causePaneLogin, Help: "Deny cause of a Browser pane URL carrying sign-in credentials.", IsHidden: true, Key: "cause_pane_login", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.pane_login"}},
	{Applies: AppliesLive, Default: causeProcSubst, Help: "Deny cause of a process substitution.", IsHidden: true, Key: "cause_proc_subst", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.proc_subst"}},
	{Applies: AppliesLive, Default: causeProtectedPath, Help: "Deny cause of a write to a protected path.", IsHidden: true, Key: "cause_protected_path", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.protected_path"}},
	{Applies: AppliesLive, Default: "read outside the working directories", Help: "Deny cause of a read outside the session's read roots.", IsHidden: true, Key: "cause_read_outside", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.read_outside"}},
	{Applies: AppliesLive, Default: causeRulesFile, Help: "Deny cause of a write to a plan's rules file outside a freezable design run; the freeze cause follows it.", IsHidden: true, Key: "cause_rules_file", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.rules_file"}},
	{Applies: AppliesLive, Default: "sed script with a w command — write files with the Edit tool or a redirect", Help: "Deny cause of a sed script with a w command.", IsHidden: true, Key: "cause_sed_script_write", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.sed_script_write"}},
	{Applies: AppliesLive, Default: causeSettingsInteractive, Help: "Deny cause of toolpolicy allow in a session no human prompted.", IsHidden: true, Key: "cause_settings_interactive", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.settings_interactive"}},
	{Applies: AppliesLive, Default: causeSettingsRevoked, Help: "Deny cause of toolpolicy allow after the user's #deny settings.local.json.", IsHidden: true, Key: "cause_settings_revoked", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.settings_revoked"}},
	{Applies: AppliesLive, Default: causeSettingsUncovered, Help: "Deny cause of toolpolicy allow rules no human act covers; the uncovered rules follow it.", IsHidden: true, Key: "cause_settings_uncovered", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.settings_uncovered"}},
	{Applies: AppliesLive, Default: causeSettingsUsage, Help: "Usage text of a malformed toolpolicy command.", IsHidden: true, Key: "cause_settings_usage", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.settings_usage"}},
	{Applies: AppliesLive, Default: causeSettingsViaCommand, Help: "Deny cause of a direct write to settings.local.json.", IsHidden: true, Key: "cause_settings_via_command", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.settings_via_command"}},
	{Applies: AppliesLive, Default: "tool-policy state unavailable: ", Help: "Lead of the deny cause when the session state cannot be read; the error follows it.", IsHidden: true, Key: "cause_state_unavailable", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.state_unavailable"}},
	{Applies: AppliesLive, Default: "~user path — name the absolute path instead", Help: "Deny cause of a ~user path.", IsHidden: true, Key: "cause_tilde_user", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.tilde_user"}},
	{Applies: AppliesLive, Default: causeTmpWrite, Help: "Deny cause of a write under a temp dir outside the session scratchpad.", IsHidden: true, Key: "cause_tmp_write", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.tmp_write"}},
	{Applies: AppliesLive, Default: "unknown tool", Help: "Deny cause of a tool in no tool class.", IsHidden: true, Key: "cause_unknown_tool", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.unknown_tool"}},
	{Applies: AppliesLive, Default: "unparseable compound command", Help: "Deny cause of a command that does not parse and carries a separator.", IsHidden: true, Key: "cause_unparseable_compound", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.unparseable_compound"}},
	{Applies: AppliesLive, Default: "command name the gate cannot resolve statically", Help: "Deny cause of a command name the gate cannot resolve.", IsHidden: true, Key: "cause_unresolvable_head", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.unresolvable_head"}},
	{Applies: AppliesLive, Default: causeUnresolvablePath, Help: "Deny cause of a path argument the gate cannot resolve.", IsHidden: true, Key: "cause_unresolvable_path", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.unresolvable_path"}},
	{Applies: AppliesLive, Default: causeWorkflowMarker, Help: "Deny cause of an inline Workflow script carrying the primer marker without being the primer.", IsHidden: true, Key: "cause_workflow_marker", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.workflow_marker"}},
	{Applies: AppliesLive, Default: causeWorkflowRelay, Help: "Deny cause of a Workflow launch outside a turn a task notification opened.", IsHidden: true, Key: "cause_workflow_relay", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.workflow_relay"}},
	{Applies: AppliesLive, Default: causeWorktreeOutside, Help: "Deny cause of an EnterWorktree path outside the repository's worktrees dir.", IsHidden: true, Key: "cause_worktree_outside", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.worktree_outside"}},
	{Applies: AppliesLive, Default: "write outside the working directories", Help: "Deny cause of a write outside the session's write roots.", IsHidden: true, Key: "cause_write_outside", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.write_outside"}},
	{Applies: AppliesLive, Default: causeXargsWrite, Help: "Deny cause of xargs feeding a command that is not read-only.", IsHidden: true, Key: "cause_xargs_write", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.causes.xargs_write"}},
	{Applies: AppliesLive, Default: "acceptEdits file command, paths checked by the fence (bash-policy)", Help: "Allow reason of a file command in acceptEdits or auto mode.", IsHidden: true, Key: "reason_accept_edits", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.accept_edits"}},
	{Applies: AppliesLive, Default: "active skill ", Help: "Lead of the allow reason of a Skill call to an active skill; the skill follows it.", IsHidden: true, Key: "reason_active_skill", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.active_skill"}},
	{Applies: AppliesLive, Default: "flagged file command with safe flags inside the write roots (bash-policy)", Help: "Allow reason of a file command with safe flags inside the write roots.", IsHidden: true, Key: "reason_path_flag", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.path_flag"}},
	{Applies: AppliesLive, Default: "two-segment pipeline of allowed commands (bash-policy)", Help: "Allow reason of a pipe whose two sides are allowed.", IsHidden: true, Key: "reason_pipeline", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.pipeline"}},
	{Applies: AppliesLive, Default: "skill the user's newest message names: /", Help: "Lead of the allow reason of a Skill call the user's newest message names; the skill follows it.", IsHidden: true, Key: "reason_prompt_skill", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.prompt_skill"}},
	{Applies: AppliesLive, Default: "read-only command batch (bash-policy)", Help: "Allow reason of a read-only command batch.", IsHidden: true, Key: "reason_read_only", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.read_only"}},
	{Applies: AppliesLive, Default: "inside the session's working roots", Help: "Allow reason of a path inside the session's roots.", IsHidden: true, Key: "reason_roots", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.roots"}},
	{Applies: AppliesLive, Default: "in-place sed, files checked by the fence (bash-policy)", Help: "Allow reason of an in-place sed whose files the fence checked.", IsHidden: true, Key: "reason_sed_write", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.sed_write"}},
	{Applies: AppliesLive, Default: "settings rule ", Help: "Lead of the allow reason of a settings allow rule; the rule follows it.", IsHidden: true, Key: "reason_settings_rule", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.settings_rule"}},
	{Applies: AppliesLive, Default: "skill grant ", Help: "Lead of the allow reason of a skill's allowed-tools rule; the rule follows it.", IsHidden: true, Key: "reason_skill_grant", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.skill_grant"}},
	{Applies: AppliesLive, Default: "system path, read-only", Help: "Allow reason of a read under a system path.", IsHidden: true, Key: "reason_system_read", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.system_read"}},
	{Applies: AppliesLive, Default: "ungated tool", Help: "Allow reason of a tool listed in ungated_tools.", IsHidden: true, Key: "reason_ungated", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.reasons.ungated"}},
	{Applies: AppliesLive, Default: steerBash, Help: "Steering text every Bash deny appends.", IsHidden: true, Key: "steer_bash", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.steers.bash"}},
	{Applies: AppliesLive, Default: steerDeny, Help: "Steering text every deny appends after the candidate rule.", IsHidden: true, Key: "steer_deny", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.steers.deny"}},
	{Applies: AppliesLive, Default: steerFind, Help: "Steering text of a find exec or write deny.", IsHidden: true, Key: "steer_find", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.steers.find"}},
	{Applies: AppliesLive, Default: steerRelay, Help: "Relay steps of a Workflow relay deny; the primer script follows them.", IsHidden: true, Key: "steer_relay", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "toolpolicy.steers.relay"}},
	{Applies: AppliesLive, Default: listText("actions", "facts", "rules"), Help: "Context chapters whose entries reach a session only through a skill invocation.", Key: "invoke_entry_chapters", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "skillinvokegate.entry_chapters"}},
	{Applies: AppliesLive, Default: invokeShellMetaChars, Help: "Characters that make a Bash read of an entry non-literal.", Key: "invoke_shell_meta_chars", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillinvokegate.shell_meta_chars"}},
	{Applies: AppliesLive, Default: defaultInvokeDenyTexts, Help: "The two deny texts of the skill-invoke gate: a direct entry read, a call before the required invocation.", Key: "invoke_deny_texts", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "skillinvokegate.deny_texts"}},
}

// heartbeatSettings are the heartbeat component's rows; heartbeat/config.json keys, the port marker and the plist file limit.
var heartbeatSettings = []Setting{
	{Applies: AppliesRestart, Default: "6101", Help: "Port the heartbeat daemon listens on; its hook URLs and the plist follow it.", Key: PortKey, Kind: KindInt, Sink: Sink{Kind: SinkMarker, Target: "{{HEARTBEAT_PORT}}"}},
	{Applies: AppliesRestart, Default: "30s", Help: "Wait before a degraded peek round is repeated.", Key: "peek_retry", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "peek_retry_ms"}},
	{Applies: AppliesRestart, Default: "", Help: "Landing target the done skill catches up; empty means no staging target, and /done branch: refuses naming this setting.", Key: "staging_target", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "staging_target"}},
	{Applies: AppliesRestart, Default: "2", Help: "Worker threads of the daemon.", Key: "workers", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "workers"}},
	{Applies: AppliesRestart, Default: "500ms", Help: "Grace the daemon gives its threads at shutdown.", Key: "shutdown_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "shutdown_timeout_ms"}},
	{Applies: AppliesRestart, Default: "16384", Help: "Events the engine inbox holds before senders wait.", Key: "inbox_capacity", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "inbox_capacity"}},
	{Applies: AppliesLive, Default: "[]", Help: "Path patterns two fleet members may both change without a collision.", Key: "exempt_paths", Kind: KindGlobs, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "exempt_paths"}},
	{Applies: AppliesLive, Default: `["version"]`, Help: "JSON keys whose change alone is no collision.", Key: "version_keys", Kind: KindStrings, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "version_keys"}},
	{Applies: AppliesLive, Default: "[]", Help: "Model patterns a subagent may not run on; the heart attack stops one that does.", Key: "forbidden_subagent_models", Kind: KindModels, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "forbidden_subagent_models"}},
	{Applies: AppliesLive, Default: `["claude/"]`, Help: "Branch prefixes whose branches the collision check compares.", Key: "collision_prefixes", Kind: KindStrings, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "collision_prefixes"}},
	{Applies: AppliesLive, Default: heartbeatRuntimeStopReason, Help: "Stop text of a runtime finding: {finding}, {skill}, {version}, {check}, {reason}, {evidence}.", Key: "runtime_stop_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "runtime_stop_reason"}},
	{Applies: AppliesLive, Default: "8", Help: "Failures of one check after which the run is killed.", Key: "kill_at", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "kill_at"}},
	{Applies: AppliesLive, Default: heartbeatKillReason, Help: "Stop text of a killed run: {id}, {skill}, {check}, {kill_at}, {reason}.", Key: "kill_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "kill_reason"}},
	{Applies: AppliesLive, Default: "64", Help: "Ended runs the runtime keeps listed.", Key: "max_ended_runs", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "max_ended_runs"}},
	{Applies: AppliesLive, Default: "50", Help: "Lines of a runtime digest.", Key: "runtime_digest_lines", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "runtime_digest_lines"}},
	{Applies: AppliesLive, Default: defaultGuardedTools, Help: "Tools a runtime subscriber may not call.", Key: "guarded_tools", Kind: KindStrings, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "guarded_tools"}},
	{Applies: AppliesLive, Default: heartbeatSubscriberDenyReason, Help: "Deny text of a guarded tool while subscribed: {tool}.", Key: "subscriber_deny_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "subscriber_deny_reason"}},
	{Applies: AppliesRestart, Default: "256", Help: "Characters a free-text field keeps.", Key: "max_field", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "max_field"}},
	{Applies: AppliesLive, Default: "50", Help: "Lines of a fleet digest.", Key: "digest_lines", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "digest_lines"}},
	{Applies: AppliesLive, Default: "8", Help: "Paths one digest line names.", Key: "digest_line_paths", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "digest_line_paths"}},
	{Applies: AppliesLive, Default: "1000", Help: "Terminal sessions the fleet keeps listed.", Key: "max_terminal", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "max_terminal"}},
	{Applies: AppliesLive, Default: "claude-orch/", Help: "Prefix of the orchestrator's merge branches.", Key: "merge_branch_prefix", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "merge_branch_prefix"}},
	{Applies: AppliesRestart, Default: "claude/", Help: "Prefix of the worker branches the hook edge maps to their sessions.", Key: "worker_branch_prefix", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "worker_branch_prefix"}},
	{Applies: AppliesLive, Default: `["AskUserQuestion","ExitPlanMode"]`, Help: "Tools whose call marks a session as waiting on its human.", Key: "waiting_tools", Kind: KindStrings, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "waiting_tools"}},
	{Applies: AppliesLive, Default: defaultWaitingNotifications, Help: "Notification types that mark a session as waiting on its human.", Key: "waiting_notifications", Kind: KindStrings, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "waiting_notifications"}},
	{Applies: AppliesLive, Default: "1s", Help: "Window within which a denial ledger line matches a hook event.", Key: "ledger_second", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "ledger_second_ms"}},
	{Applies: AppliesLive, Default: "32", Help: "Unknown sessions whose refusal the engine remembers.", Key: "max_refused_unknown", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "max_refused_unknown"}},
	{Applies: AppliesLive, Default: heartbeatAttackDenyReason, Help: "Deny text of a spawn on a forbidden model: {model}.", Key: "attack_deny_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "attack_deny_reason"}},
	{Applies: AppliesLive, Default: heartbeatAttackStopReason, Help: "Stop text of a subagent running on a forbidden model: {model}.", Key: "attack_stop_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "attack_stop_reason"}},
	{Applies: AppliesLive, Default: heartbeatAttackInheritedReason, Help: "Stop text of a subagent spawned without a model: {model}.", Key: "attack_inherited_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "attack_inherited_reason"}},
	{Applies: AppliesLive, Default: heartbeatAttackSubstitutedReason, Help: "Stop text of a subagent whose model the harness substituted: {requested}, {model}.", Key: "attack_substituted_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "attack_substituted_reason"}},
	{Applies: AppliesLive, Default: heartbeatMemberDenyReason, Help: "Deny text of a fleet member messaging the orchestrator: {token}.", Key: "member_deny_reason", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "member_deny_reason"}},
	{Applies: AppliesLive, Default: heartbeatMemberRoleText, Help: "Role context a fleet member receives on every prompt: {token}, {fleet}.", Key: "member_role_text", Kind: KindString, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "member_role_text"}},
	{Applies: AppliesRestart, Default: "62", Help: "Characters a fleet or member id may hold.", Key: "max_id_len", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "max_id_len"}},
	{Applies: AppliesRestart, Default: "67108864", Help: "Bytes one hook request body may hold.", Key: "hook_max_body", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "hook_max_body"}},
	{Applies: AppliesRestart, Default: "256", Help: "Characters of a hook event's subject the edge keeps.", Key: "hook_max_subject", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "hook_max_subject"}},
	{Applies: AppliesRestart, Default: "2048", Help: "Characters of a hook event's detail the edge keeps.", Key: "hook_max_detail", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "hook_max_detail"}},
	{Applies: AppliesRestart, Default: "10ms", Help: "Interval at which the hook edge polls for a claimed answer.", Key: "claim_poll", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "claim_poll_ms"}},
	{Applies: AppliesRestart, Default: "200", Help: "Polls the hook edge makes before it answers without a claim.", Key: "claim_polls", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "claim_polls"}},
	{Applies: AppliesRestart, Default: "5s", Help: "Deadline of one CLI call against the daemon.", Key: "client_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "client_timeout_ms"}},
	{Applies: AppliesRestart, Default: "5s", Help: "Deadline of the peek identity probe.", Key: "peek_healthz_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "peek_healthz_timeout_ms"}},
	{Applies: AppliesRestart, Default: "10s", Help: "Deadline of one peek MCP call.", Key: "peek_call_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "peek_call_timeout_ms"}},
	{Applies: AppliesRestart, Default: "5s", Help: "Deadline of one socket write to a subscriber.", Key: "socket_write_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "socket_write_timeout_ms"}},
	{Applies: AppliesRestart, Default: "250ms", Help: "Quiet time the worktree edge waits before it rescans.", Key: "scan_settle", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "scan_settle_ms"}},
	{Applies: AppliesRestart, Default: "1048576", Help: "Bytes of the denial ledger the ledger edge reads from its end.", Key: "ledger_tail_window", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "ledger_tail_window"}},
	{Applies: AppliesRestart, Default: "64", Help: "Transcript lines the transcript edge reads per wake.", Key: "transcript_max_lines", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "transcript_max_lines"}},
	{Applies: AppliesRestart, Default: "64", Help: "Process exits the exits edge batches into one event.", Key: "exits_batch", Kind: KindInt, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "exits_batch"}},
	{Applies: AppliesRestart, Default: "1s", Help: "Interval of the polling process-exit watch off macOS (macOS watches through kqueue).", Key: "exits_poll", Kind: KindDuration, Sink: Sink{Kind: SinkHeartbeatConfig, Target: "exits_poll_ms"}},
	{Applies: AppliesRestart, Default: "16384", Help: "Open-file limit the LaunchAgent grants the daemon.", Key: "number_of_files", Kind: KindInt, Sink: Sink{Kind: SinkInstall, Target: "NumberOfFiles"}},
}

// peekSettings are peek's install rows.
var peekSettings = []Setting{
	{Applies: AppliesRestart, Default: "4242", Help: "Port of the central peek MCP server; heartbeat's peek address follows it.", Key: PortKey, Kind: KindInt, Sink: Sink{Kind: SinkInstall, Target: "PeekPort"}},
	{Applies: AppliesRestart, Default: "42442", Help: "Port of peek's control endpoint.", Key: "control_port", Kind: KindInt, Sink: Sink{Kind: SinkInstall, Target: "PeekControlPort"}},
	{Applies: AppliesRestart, Default: "", Help: "Claude home peek reads sessions from; empty takes ~/.claude.", Key: "claude_home", Kind: KindString, Sink: Sink{Kind: SinkInstall, Target: "PeekClaudeHome"}},
	{Applies: AppliesRestart, Default: "", Help: "Codex home peek reads sessions from; empty takes ~/.codex.", Key: "codex_home", Kind: KindString, Sink: Sink{Kind: SinkInstall, Target: "PeekCodexHome"}},
}

// routinesSettings are the routines component's install rows.
var routinesSettings = []Setting{
	{Applies: AppliesRestart, Default: "off", Help: "How the config server's context report and the budget routine count Claude tokens: off, or keychain for the Anthropic API key in the OS credential store — the macOS Keychain item com.smine.configserver / anthropic-api-key, or the Windows Credential Manager entry com.smine.configserver/anthropic-api-key.", Key: "count_tokens", Kind: KindEnum, Sink: Sink{Kind: SinkInstall, Target: "CountTokens"}, Values: []string{"off", "keychain"}},
}

// sessionHelpersSettings are the session-helpers component's rows; sections sessiontitle, skillhelp and sounds.
var sessionHelpersSettings = []Setting{
	{Applies: AppliesLive, Default: "62", Help: "Runes a session title may hold.", Key: "max_title_runes", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.max_title_runes"}},
	{Applies: AppliesLive, Default: "8", Help: "Title states one skill may declare.", Key: "max_states", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.max_states"}},
	{Applies: AppliesLive, Default: "16", Help: "Runes one title state token may hold.", Key: "max_token_runes", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.max_token_runes"}},
	{Applies: AppliesLive, Default: titleInstructionPlanMode, Help: "Sentence the rename instruction adds in plan mode.", Key: "title_instruction_plan_mode", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.instruction_plan_mode"}},
	{Applies: AppliesLive, Default: titleInstructionTail, Help: "Closing sentences of the rename instruction.", Key: "title_instruction_tail", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.instruction_tail"}},
	{Applies: AppliesLive, Default: titleRenameAllowReason, Help: "Allow reason of the hook-instructed rename call.", Key: "title_rename_allow_reason", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.rename_allow_reason"}},
	{Applies: AppliesLive, Default: defaultTitleTriggerPatterns, Help: "Command patterns the title hook watches for: close check, done, join, subscribe, the fleet flag, the member token, title set.", Key: "title_trigger_patterns", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "sessiontitle.trigger_patterns"}},
	{Applies: AppliesLive, Default: helpPattern, Help: "Prompt pattern that marks a /<skill> --help prompt.", Key: "help_pattern", Kind: KindRegex, Sink: Sink{Kind: SinkHooksConfig, Target: "skillhelp.help_pattern"}},
	{Applies: AppliesLive, Default: helpFeaturesLead, Help: "Lead line above the feature catalog of a help answer.", Key: "help_features_lead", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "skillhelp.features_lead"}},
	{Applies: AppliesLive, Default: soundDefault("/System/Library/Sounds/Funk.aiff", "C:/Windows/Media/notify.wav"), Help: "Sound file played on a notification (afplay on macOS, PlaySound on Windows; forward slashes, the path lands in a bash hook command).", Key: "notification_sound", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "sounds.notification"}},
	{Applies: AppliesLive, Default: soundDefault("/System/Library/Sounds/Hero.aiff", "C:/Windows/Media/ding.wav"), Help: "Sound file played when a turn ends (afplay on macOS, PlaySound on Windows; forward slashes, the path lands in a bash hook command).", Key: "stop_sound", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "sounds.stop"}},
}

// worktreesSettings are the worktrees component's rows; sections worktreecreate and worktreewriteguard.
var worktreesSettings = []Setting{
	{Applies: AppliesLive, Default: "claude/", Help: "Prefix of every session branch the worktree hook creates.", Key: "branch_prefix", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.branch_prefix"}},
	{Applies: AppliesLive, Default: ".claude/worktrees", Help: "Repo-relative directory the session worktrees live in.", Key: "worktrees_dir", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.worktrees_dir"}},
	{Applies: AppliesLive, Default: "5", Help: "Attempts of the branch reset when git reports index.lock.", Key: "reset_attempts", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.reset_attempts"}},
	{Applies: AppliesLive, Default: "200ms", Help: "Base delay between reset attempts; attempt n waits n times this.", Key: "reset_retry_delay", Kind: KindDuration, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.reset_retry_delay_ms"}},
	{Applies: AppliesLive, Default: "12s", Help: "Deadline of the acdsl build after a base move; it stays under the hook's own timeout.", Key: "acdsl_build_timeout", Kind: KindDuration, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.acdsl_build_timeout_ms"}},
	{Applies: AppliesLive, Default: "7", Help: "Hex digits of the short sha a worktree name ends in.", Key: "short_sha_length", Kind: KindInt, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.short_sha_length"}},
	{Applies: AppliesLive, Default: `["main","master"]`, Help: "Default-branch names tried when origin/HEAD is unset.", Key: "default_branch_fallback", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.default_branch_fallback"}},
	{Applies: AppliesLive, Default: worktreeNamePattern, Help: "Pattern a requested worktree name must match.", Key: "name_pattern", Kind: KindRegex, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreecreate.name_pattern"}},
	{Applies: AppliesLive, Default: "/.claude/worktrees/", Help: "Path segment that marks a session worktree for the write guard and the protected-root check.", Key: "pool_segment", Kind: KindString, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreewriteguard.pool_segment"}},
	{Applies: AppliesLive, Default: defaultWorktreeGuardTexts, Help: "The three deny texts of the write guard: outside the worktree, a parent segment, a protected checkout.", Key: "guard_deny_texts", Kind: KindStrings, Sink: Sink{Kind: SinkHooksConfig, Target: "worktreewriteguard.deny_texts"}},
}

// listText is the list kinds' text form of items: a JSON string array.
func listText(items ...string) string {
	data, _ := json.Marshal(items)
	return string(data)
}

// soundDefault picks the platform's stock sound; linux has no player, so its default is empty and the hook exits at once.
func soundDefault(darwin, windows string) string {
	switch runtime.GOOS {
	case goosDarwin:
		return darwin
	case goosWindows:
		return windows
	}
	return ""
}
