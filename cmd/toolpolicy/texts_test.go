package main

import (
	"encoding/json"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

// defaultTexts is the rendered gate section; the names below bind its texts, so an asserted verdict text is the hooks config's.
var defaultTexts = renderedTextsPolicy()

var (
	causeAcdslRuleFile       = defaultTexts.Causes.AcdslRuleFile
	causeAcdslSurface        = defaultTexts.Causes.AcdslSurface
	causeAgentModels         = defaultTexts.Causes.AgentModels
	causeAnchorRemoved       = defaultTexts.Causes.AnchorRemoved
	causeAwkWrite            = defaultTexts.Causes.AwkWrite
	causeCdGit               = defaultTexts.Causes.CdGit
	causeCdProtected         = defaultTexts.Causes.CdProtected
	causeCdRedirect          = defaultTexts.Causes.CdRedirect
	causeCompound            = defaultTexts.Causes.Compound
	causeCriticalRemove      = defaultTexts.Causes.CriticalRemove
	causeDeniedTool          = defaultTexts.Causes.DeniedTool
	causeEngineProtected     = defaultTexts.Causes.EngineProtected
	causeFenceConstruct      = defaultTexts.Causes.FenceConstruct
	causeFenceUnparseable    = defaultTexts.Causes.FenceUnparseable
	causeFenced              = defaultTexts.Causes.Fenced
	causeFindExec            = defaultTexts.Causes.FindExec
	causeFreezeBudget        = defaultTexts.Causes.FreezeBudget
	causeFreezeReopen        = defaultTexts.Causes.FreezeReopen
	causeFreezeTarget        = defaultTexts.Causes.FreezeTarget
	causeGitDir              = defaultTexts.Causes.GitDir
	causeGitEnvironment      = defaultTexts.Causes.GitEnvironment
	causeGitExec             = defaultTexts.Causes.GitExec
	causeGitGlobalOption     = defaultTexts.Causes.GitGlobalOption
	causeGitSameRepo         = defaultTexts.Causes.GitSameRepo
	causeGoProgramFlag       = defaultTexts.Causes.GoProgramFlag
	causeMakeArgs            = defaultTexts.Causes.MakeArgs
	causeNoAllowRule         = defaultTexts.Causes.NoAllowRule
	causeOperatorCommand     = defaultTexts.Causes.OperatorCommand
	causePaneLogin           = defaultTexts.Causes.PaneLogin
	causeProcSubst           = defaultTexts.Causes.ProcSubst
	causeProtectedPath       = defaultTexts.Causes.ProtectedPath
	causeReadOutside         = defaultTexts.Causes.ReadOutside
	causeRulesFile           = defaultTexts.Causes.RulesFile
	causeSedScriptWrite      = defaultTexts.Causes.SedScriptWrite
	causeSettingsInteractive = defaultTexts.Causes.SettingsInteractive
	causeSettingsRevoked     = defaultTexts.Causes.SettingsRevoked
	causeSettingsUncovered   = defaultTexts.Causes.SettingsUncovered
	causeSettingsUsage       = defaultTexts.Causes.SettingsUsage
	causeSettingsViaCommand  = defaultTexts.Causes.SettingsViaCommand
	causeTildeUser           = defaultTexts.Causes.TildeUser
	causeTmpWrite            = defaultTexts.Causes.TmpWrite
	causeUnknownTool         = defaultTexts.Causes.UnknownTool
	causeUnresolvableHead    = defaultTexts.Causes.UnresolvableHead
	causeUnresolvablePath    = defaultTexts.Causes.UnresolvablePath
	causeWorkflowMarker      = defaultTexts.Causes.WorkflowMarker
	causeWorkflowRelay       = defaultTexts.Causes.WorkflowRelay
	causeWorktreeOutside     = defaultTexts.Causes.WorktreeOutside
	causeWriteOutside        = defaultTexts.Causes.WriteOutside
	causeXargsWrite          = defaultTexts.Causes.XargsWrite

	acceptEditsAllowReason = defaultTexts.Reasons.AcceptEdits
	activeSkillReason      = defaultTexts.Reasons.ActiveSkill
	allowReason            = defaultTexts.Reasons.ReadOnly
	pathFlagAllowReason    = defaultTexts.Reasons.PathFlag
	pipelineAllowReason    = defaultTexts.Reasons.Pipeline
	promptSkillReason      = defaultTexts.Reasons.PromptSkill
	rootsAllowReason       = defaultTexts.Reasons.Roots
	sedWriteAllowReason    = defaultTexts.Reasons.SedWrite
	settingsRuleReason     = defaultTexts.Reasons.SettingsRule
	skillGrantReason       = defaultTexts.Reasons.SkillGrant
	systemReadReason       = defaultTexts.Reasons.SystemRead
	ungatedAllowReason     = defaultTexts.Reasons.Ungated

	denySteer  = defaultTexts.Steers.Deny
	relaySteps = defaultTexts.Steers.Relay
)

// renderedTextsPolicy decodes the gate's rendered defaults once for the package-level text names; a render failure is a broken catalog.
func renderedTextsPolicy() *toolpolicy.Policy {
	defaults, err := install.RenderDefaults(components.Gate, components.SinkHooksConfig)
	if err != nil {
		panic(err)
	}
	cfg := &hookconfig.Config{}
	if err := json.Unmarshal(defaults, cfg); err != nil {
		panic(err)
	}
	return &cfg.Toolpolicy
}
