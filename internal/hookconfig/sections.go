package hookconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

const (
	ClassBash Class = iota
	ClassRead
	ClassWrite
	ClassRuleBearing
	ClassUnknown
)

const prefixWildcard = "*"

type Acdslhook struct {
	Binaries      []string   `json:"binaries"`
	ContextText   string     `json:"context_text"`
	GoRunFallback bool       `json:"go_run_fallback"`
	TimeoutMs     DurationMs `json:"timeout_ms"`
}

// Class is the closed tool taxonomy the gates dispatch on.
type Class int

// Context is the section every context hook shares.
type Context struct {
	ContextDir string `json:"context_dir"`
}

type Globalcontext struct {
	Banner        string `json:"banner"`
	InlineCap     int    `json:"inline_cap"`
	PointerFormat string `json:"pointer_format"`
	ReadBudget    int    `json:"read_budget"`
}

// Listing is one read_only_commands head: bare admits every subcommand, else only the listed ones.
type Listing struct {
	IsBare      bool
	Subcommands []string
}

// add folds a later entry of the same head in; false when the head would be both bare and listed with subcommands.
func (l *Listing) add(subcommand string) bool {
	isBareEntry := subcommand == ""
	if isBareEntry != l.IsBare {
		return false
	}
	if isBareEntry {
		return true
	}

	l.Subcommands = append(l.Subcommands, subcommand)
	return true
}

type Planmodecontext struct {
	Banner        string `json:"banner"`
	EntryFormat   string `json:"entry_format"`
	EntryIdPrefix string `json:"entry_id_prefix"`
	Heading       string `json:"heading"`
	PackFormat    string `json:"pack_format"`
	PointerFormat string `json:"pointer_format"`
	RootOnly      bool   `json:"root_only"`
	Trailer       string `json:"trailer"`
}

type Readgate struct {
	BashAddendum        string     `json:"bash_addendum"`
	DefaultSliceSuffix  string     `json:"default_slice_suffix"`
	DenyFormat          string     `json:"deny_format"`
	DisabledGuides      []string   `json:"disabled_guides"`
	GateSubagents       bool       `json:"gate_subagents"`
	HeadMismatchText    string     `json:"head_mismatch_text"`
	LedgerRetentionMs   DurationMs `json:"ledger_retention_ms"`
	ManifestWriteReason string     `json:"manifest_write_reason"`
}

type Sessiontitle struct {
	InstructionPlanMode string   `json:"instruction_plan_mode"`
	InstructionTail     string   `json:"instruction_tail"`
	MaxStates           int      `json:"max_states"`
	MaxTitleRunes       int      `json:"max_title_runes"`
	MaxTokenRunes       int      `json:"max_token_runes"`
	RenameAllowReason   string   `json:"rename_allow_reason"`
	TriggerPatterns     []string `json:"trigger_patterns"`
}

type Skillcontext struct {
	BannerFormat     string `json:"banner_format"`
	DefaultSlice     string `json:"default_slice"`
	HelpPattern      string `json:"help_pattern"`
	InlineCap        int    `json:"inline_cap"`
	NoContextPattern string `json:"no_context_pattern"`
	PointerFormat    string `json:"pointer_format"`
	RepeatFormat     string `json:"repeat_format"`
}

type Skillhelp struct {
	FeaturesLead string `json:"features_lead"`
	HelpPattern  string `json:"help_pattern"`
}

type Skillinvokegate struct {
	DenyTexts      []string `json:"deny_texts"`
	EntryChapters  []string `json:"entry_chapters"`
	ShellMetaChars string   `json:"shell_meta_chars"`
}

type Sounds struct {
	Notification string `json:"notification"`
	Stop         string `json:"stop"`
}

// ToolClasses lists the tools of each class; an entry ending in * matches by prefix.
type ToolClasses struct {
	Bash        []string `json:"bash"`
	Read        []string `json:"read"`
	RuleBearing []string `json:"rule_bearing"`
	Write       []string `json:"write"`
}

// all returns the class lists in lookup order: bash, read, write, rule-bearing.
func (c *ToolClasses) all() [][]string {
	return [][]string{c.Bash, c.Read, c.Write, c.RuleBearing}
}

func (c *ToolClasses) Validate() error {
	// Bash
	if len(c.Bash) == 0 {
		return errors.New("ToolClasses.Validate: Missing field bash")
	}

	// Read
	if len(c.Read) == 0 {
		return errors.New("ToolClasses.Validate: Missing field read")
	}

	// Write
	if len(c.Write) == 0 {
		return errors.New("ToolClasses.Validate: Missing field write")
	}

	// every entry in one class only
	seen := map[string]bool{}
	for _, entries := range c.all() {
		if duplicate := firstSeen(entries, seen); duplicate != "" {
			return fmt.Errorf("ToolClasses.Validate: %q is listed in two classes", duplicate)
		}
	}
	return nil
}

// Toolpolicy is the former tool-policy.json plus the gate's own literals; ReadFence stays process env.
type Toolpolicy struct {
	readOnlyListings map[string]*Listing

	Causes                      ToolpolicyCauses  `json:"causes"`
	DeniedTools                 []string          `json:"denied_tools"`
	DesignSkill                 string            `json:"design_skill"`
	EngineProtectedDirs         []string          `json:"engine_protected_dirs"`
	EngineProtectedFilePatterns []string          `json:"engine_protected_file_patterns"`
	EngineProtectedFiles        []string          `json:"engine_protected_files"`
	ExtraReadDirs               []string          `json:"extra_read_dirs"`
	ExtraWriteDirs              []string          `json:"extra_write_dirs"`
	FileCommands                []string          `json:"file_commands"`
	FimplementSkill             string            `json:"fimplement_skill"`
	GitAdmittedFlags            []string          `json:"git_admitted_flags"`
	MakefileNames               []string          `json:"makefile_names"`
	ManagedSettingsPath         string            `json:"managed_settings_path"`
	MaxLoopItems                int               `json:"max_loop_items"`
	OperatorPaths               []string          `json:"operator_paths"`
	ProtectedPaths              []string          `json:"protected_paths"`
	ReadFence                   []string          `json:"-"`
	ReadOnlyCommands            []string          `json:"read_only_commands"`
	Reasons                     ToolpolicyReasons `json:"reasons"`
	Steers                      ToolpolicySteers  `json:"steers"`
	TmpDirs                     []string          `json:"tmp_dirs"`
	ToolClasses                 ToolClasses       `json:"tool_classes"`
	UngatedTools                []string          `json:"ungated_tools"`
	WrapperCommands             []string          `json:"wrapper_commands"`
}

// index runs the listing checks on a decoded section and builds the read-only lookup.
func (p *Toolpolicy) index() error {
	if len(p.ReadOnlyCommands) == 0 {
		return errors.New("Toolpolicy.index: Section has no read_only_commands")
	}
	if len(p.UngatedTools) == 0 {
		return errors.New("Toolpolicy.index: Section has no ungated_tools")
	}
	if err := p.ToolClasses.Validate(); err != nil {
		return fmt.Errorf("Toolpolicy.index: %w", err)
	}

	// An empty cause reads as no cause, so a blank text would turn its deny into a pass.
	if key := firstEmptyText(p.Causes); key != "" {
		return fmt.Errorf("Toolpolicy.index: Section has an empty causes.%s", key)
	}
	if key := firstEmptyText(p.Reasons); key != "" {
		return fmt.Errorf("Toolpolicy.index: Section has an empty reasons.%s", key)
	}
	if key := firstEmptyText(p.Steers); key != "" {
		return fmt.Errorf("Toolpolicy.index: Section has an empty steers.%s", key)
	}

	listings, err := indexReadOnlyCommands(p.ReadOnlyCommands)
	if err != nil {
		return fmt.Errorf("Toolpolicy.index: %w", err)
	}

	p.readOnlyListings = listings
	return nil
}

func (p *Toolpolicy) ClassOf(tool string) Class {
	switch {
	case matchesClass(p.ToolClasses.Bash, tool):
		return ClassBash
	case matchesClass(p.ToolClasses.Read, tool):
		return ClassRead
	case matchesClass(p.ToolClasses.Write, tool):
		return ClassWrite
	case matchesClass(p.ToolClasses.RuleBearing, tool):
		return ClassRuleBearing
	}
	return ClassUnknown
}

func (p *Toolpolicy) ExpandedExtraReadDirs(home string) []string {
	roots := make([]string, 0, len(p.ExtraReadDirs))
	for _, root := range p.ExtraReadDirs {
		roots = append(roots, ExpandHome(root, home))
	}
	return roots
}

func (p *Toolpolicy) ExpandedExtraWriteDirs(home string) []string {
	roots := make([]string, 0, len(p.ExtraWriteDirs))
	for _, root := range p.ExtraWriteDirs {
		roots = append(roots, ExpandHome(root, home))
	}
	return roots
}

// ExpandedProtectedPaths resolves the ~/ prefix of every protected subtree.
func (p *Toolpolicy) ExpandedProtectedPaths(home string) []string {
	roots := make([]string, 0, len(p.ProtectedPaths))
	for _, root := range p.ProtectedPaths {
		roots = append(roots, ExpandHome(root, home))
	}
	return roots
}

func (p *Toolpolicy) IsDeniedTool(tool string) bool {
	return slices.Contains(p.DeniedTools, tool)
}

func (p *Toolpolicy) IsUngatedTool(tool string) bool {
	return slices.Contains(p.UngatedTools, tool)
}

// ReadOnlyListing returns a head's read_only_commands listing; false for an unlisted head.
func (p *Toolpolicy) ReadOnlyListing(head string) (*Listing, bool) {
	listing, isListed := p.readOnlyListings[head]
	return listing, isListed
}

// ToolpolicyCauses are the gate's deny causes; a fence step returns one, "" meaning no deny.
type ToolpolicyCauses struct {
	AcdslRuleFile       string `json:"acdsl_rule_file"`
	AcdslSurface        string `json:"acdsl_surface"`
	AgentModels         string `json:"agent_models"`
	AnchorRemoved       string `json:"anchor_removed"`
	AwkWrite            string `json:"awk_write"`
	CdGit               string `json:"cd_git"`
	CdProtected         string `json:"cd_protected"`
	CdRedirect          string `json:"cd_redirect"`
	Compound            string `json:"compound"`
	CriticalRemove      string `json:"critical_remove"`
	DeniedTool          string `json:"denied_tool"`
	EngineProtected     string `json:"engine_protected"`
	FenceConstruct      string `json:"fence_construct"`
	FenceUnparseable    string `json:"fence_unparseable"`
	Fenced              string `json:"fenced"`
	FindExec            string `json:"find_exec"`
	FreezeBudget        string `json:"freeze_budget"`
	FreezeReopen        string `json:"freeze_reopen"`
	FreezeTarget        string `json:"freeze_target"`
	GitDir              string `json:"git_dir"`
	GitEnvironment      string `json:"git_environment"`
	GitExec             string `json:"git_exec"`
	GitGlobalOption     string `json:"git_global_option"`
	GitSameRepo         string `json:"git_same_repo"`
	GoProgramFlag       string `json:"go_program_flag"`
	MakeArgs            string `json:"make_args"`
	NoAllowRule         string `json:"no_allow_rule"`
	OperatorCommand     string `json:"operator_command"`
	PaneLogin           string `json:"pane_login"`
	ProcSubst           string `json:"proc_subst"`
	ProtectedPath       string `json:"protected_path"`
	ReadOutside         string `json:"read_outside"`
	RulesFile           string `json:"rules_file"`
	SedScriptWrite      string `json:"sed_script_write"`
	SettingsInteractive string `json:"settings_interactive"`
	SettingsRevoked     string `json:"settings_revoked"`
	SettingsUncovered   string `json:"settings_uncovered"`
	SettingsUsage       string `json:"settings_usage"`
	SettingsViaCommand  string `json:"settings_via_command"`
	StateUnavailable    string `json:"state_unavailable"`
	TildeUser           string `json:"tilde_user"`
	TmpWrite            string `json:"tmp_write"`
	UnknownTool         string `json:"unknown_tool"`
	UnparseableCompound string `json:"unparseable_compound"`
	UnresolvableHead    string `json:"unresolvable_head"`
	UnresolvablePath    string `json:"unresolvable_path"`
	WorkflowMarker      string `json:"workflow_marker"`
	WorkflowRelay       string `json:"workflow_relay"`
	WorktreeOutside     string `json:"worktree_outside"`
	WriteOutside        string `json:"write_outside"`
	XargsWrite          string `json:"xargs_write"`
}

// ToolpolicyReasons are the gate's allow reasons; some lead the rule or skill they name.
type ToolpolicyReasons struct {
	AcceptEdits  string `json:"accept_edits"`
	ActiveSkill  string `json:"active_skill"`
	PathFlag     string `json:"path_flag"`
	Pipeline     string `json:"pipeline"`
	PromptSkill  string `json:"prompt_skill"`
	ReadOnly     string `json:"read_only"`
	Roots        string `json:"roots"`
	SedWrite     string `json:"sed_write"`
	SettingsRule string `json:"settings_rule"`
	SkillGrant   string `json:"skill_grant"`
	SystemRead   string `json:"system_read"`
	Ungated      string `json:"ungated"`
}

// ToolpolicySteers are the steering texts a deny appends.
type ToolpolicySteers struct {
	Bash  string `json:"bash"`
	Deny  string `json:"deny"`
	Find  string `json:"find"`
	Relay string `json:"relay"`
}

type Worktreecreate struct {
	AcdslBuildTimeoutMs   DurationMs `json:"acdsl_build_timeout_ms"`
	BranchPrefix          string     `json:"branch_prefix"`
	DefaultBranchFallback []string   `json:"default_branch_fallback"`
	NamePattern           string     `json:"name_pattern"`
	ResetAttempts         int        `json:"reset_attempts"`
	ResetRetryDelayMs     DurationMs `json:"reset_retry_delay_ms"`
	ShortShaLength        int        `json:"short_sha_length"`
	WorktreesDir          string     `json:"worktrees_dir"`
}

type Worktreewriteguard struct {
	DenyTexts   []string `json:"deny_texts"`
	PoolSegment string   `json:"pool_segment"`
}

// firstEmptyText returns the json key of the first empty string field of a text struct; "" when every field holds text.
func firstEmptyText(texts any) string {
	value := reflect.ValueOf(texts)
	for index := range value.NumField() {
		if value.Field(index).String() == "" {
			key, _, _ := strings.Cut(value.Type().Field(index).Tag.Get("json"), ",")
			return key
		}
	}
	return ""
}

// firstSeen marks entries as seen and returns the first one that already was.
func firstSeen(entries []string, seen map[string]bool) string {
	for _, entry := range entries {
		if seen[entry] {
			return entry
		}
		seen[entry] = true
	}
	return ""
}

func indexReadOnlyCommands(entries []string) (map[string]*Listing, error) {
	listings := make(map[string]*Listing, len(entries))
	for _, entry := range entries {
		head, subcommand, _ := strings.Cut(strings.TrimSpace(entry), " ")
		if head == "" {
			return nil, errors.New("indexReadOnlyCommands: Empty read_only_commands entry")
		}

		listing, isKnown := listings[head]
		if !isKnown {
			listings[head] = newListing(subcommand)
			continue
		}

		if !listing.add(subcommand) {
			return nil, fmt.Errorf("indexReadOnlyCommands: %q is listed both bare and with subcommands", head)
		}
	}
	return listings, nil
}

func matchesClass(entries []string, tool string) bool {
	isMatch := func(entry string) bool {
		prefix, isGlob := strings.CutSuffix(entry, prefixWildcard)
		if isGlob {
			return strings.HasPrefix(tool, prefix)
		}
		return entry == tool
	}
	return slices.ContainsFunc(entries, isMatch)
}

func newListing(subcommand string) *Listing {
	if subcommand == "" {
		return &Listing{IsBare: true}
	}
	return &Listing{Subcommands: []string{subcommand}}
}

// ExpandHome rewrites a leading ~/ (or a bare ~) to the home directory.
func ExpandHome(path string, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
