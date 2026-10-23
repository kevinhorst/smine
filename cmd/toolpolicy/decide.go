package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

type protectedSet struct {
	acdsl              *acdslGuard
	causes             *hookconfig.ToolpolicyCauses
	freeze             *freezeGuard
	isMakeSourceLocked bool
	makefileNames      []string
	roots              []string
}

func newProtectedSet(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string) *protectedSet {
	roots := append(settingsLayerPaths(input, grants, home), policy.ExpandedProtectedPaths(home)...)
	if input.TranscriptPath != "" {
		roots = append(roots, input.TranscriptPath)
	}

	set := &protectedSet{
		acdsl:              newAcdslGuard(input.Cwd),
		causes:             &policy.Causes,
		freeze:             grants.freeze,
		isMakeSourceLocked: !grants.Approvals[unlockMakefile],
		makefileNames:      policy.MakefileNames,
		roots:              roots,
	}
	return set
}

func (s *protectedSet) argCause(cwd string, home string, call *resolvedCall) (string, string) {
	for _, arg := range call.args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if cause := s.cause(cwd, home, arg); cause != "" {
			return cause, arg
		}
	}
	return "", ""
}

// cause names why path is protected; "" when it is not.
func (s *protectedSet) cause(cwd string, home string, path string) string {
	target := hookconfig.ExpandHome(path, home)
	if s.isMakeSourceLocked && isMakeSource(target, s.makefileNames) {
		return approvableCause(unlockMakefile)
	}
	if cause := s.freeze.fileCause(s.causes, cwd, target); cause != "" {
		return cause
	}
	if IsInsideRoots(cwd, target, s.roots) {
		if filepath.Base(target) == unlockSettings {
			return s.causes.SettingsViaCommand
		}
		return s.causes.ProtectedPath
	}
	if IsInsideGitDir(cwd, target) {
		return s.causes.GitDir
	}
	return s.acdsl.cause(s.causes, cwd, target)
}

func Decide(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string, statePath string) Verdict {
	tool := input.ToolName
	if tool == exitPlanModeToolName {
		return Verdict{Decision: decisionNone}
	}
	if policy.IsDeniedTool(tool) {
		return grants.denyVerdict(policy, tool, input, policy.Causes.DeniedTool)
	}
	if rule := grants.matchDeny(tool, &input.ToolInput); rule != nil {
		return grants.denyVerdict(policy, tool, input, "matches deny rule "+rule.Raw)
	}
	if cause := loginCause(&policy.Causes, input); cause != "" {
		return grants.denyVerdict(policy, tool, input, cause)
	}
	isEnforceOff, err := sessionstate.IsOff(statePath, sessionstate.SkillEnforce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: Decide: %v\n", err)
	}
	if cause := enforceCause(input, grants.Enforce); cause != "" && !isEnforceOff {
		return enforceVerdict(tool, cause)
	}
	if cause := relayCause(&policy.Causes, input, grants); cause != "" {
		return relayVerdict(policy, cause)
	}
	if cause := agentModelCause(&policy.Causes, tool, grants.Models, input.ToolInput.Model); cause != "" {
		return grants.denyVerdict(policy, tool, input, cause)
	}
	if policy.IsUngatedTool(tool) {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.Ungated}
	}

	if cause := fenceCause(input, policy, home); cause != "" {
		return grants.denyVerdict(policy, tool, input, cause)
	}

	roots := newSessionRoots(grants, home, input, policy)
	switch policy.ClassOf(tool) {
	case toolpolicy.ClassBash:
		return decideBash(input, policy, grants, roots, home, statePath)
	case toolpolicy.ClassRead:
		return decideRead(input, grants, roots)
	case toolpolicy.ClassWrite:
		return decideWrite(input, grants, roots)
	case toolpolicy.ClassRuleBearing:
		if tool == enterWorktreeTool {
			return decideEnterWorktree(grants, input, roots)
		}
		return decideByRules(tool, input, policy, grants)
	case toolpolicy.ClassUnknown:
		return grants.denyVerdict(policy, tool, input, policy.Causes.UnknownTool)
	}
	return grants.denyVerdict(policy, tool, input, policy.Causes.UnknownTool)
}

// In auto mode an allow is withheld so Claude Code's own flow (rules, read-only, classifier) decides; denies and the gate's own answers stand.
func deferAllowToAutoMode(input *HookInput, verdict Verdict) Verdict {
	if input.PermissionMode != permissionModeAuto {
		return verdict
	}
	isGateAnswer := verdict.IsApplied || verdict.IsReport
	if verdict.Decision != decisionAllow || isGateAnswer {
		return verdict
	}
	return Verdict{Decision: decisionNone, Reason: verdict.Reason}
}

func decideBash(input *HookInput, policy *toolpolicy.Policy, grants *Grants, roots *sessionRoots, home string, statePath string) Verdict {
	if isSelfServiceCommand(input.ToolInput.Command) {
		return decideSelfServiceCommand(input, policy, grants, home, statePath)
	}
	if input.ToolName == toolNameBash {
		if cause := fenceBash(input, policy, roots); cause != "" {
			return grants.denyVerdict(policy, toolNameBash, input, cause)
		}
	}
	for _, target := range cdTargets(input.ToolInput.Command, policy) {
		if roots.protected.cause(input.Cwd, home, target) != "" {
			return grants.denyVerdict(policy, toolNameBash, input, policy.Causes.CdProtected, pathDetail(input.Cwd, home, target))
		}
	}
	classified := Classify(input.ToolInput.Command, policy)
	if classified.Decision == decisionAllow {
		return classified
	}
	if classified.Decision == decisionDeny {
		return decideCompound(classified, input, policy, grants, roots, home)
	}
	if isExecCapableFind(input.ToolInput.Command, policy) {
		verdict := Verdict{
			Cause:    policy.Causes.FindExec,
			Decision: decisionDeny,
			Reason:   "bash-policy: " + policy.Causes.FindExec + ". " + policy.Steers.Find,
		}
		return verdict
	}
	if call, isMake := singleMakeCall(input.ToolInput.Command, policy); isMake {
		cause := makeCause(
			call,
			input.Cwd,
			home,
			roots.anchor,
			policy,
		)
		if cause != "" {
			return grants.denyVerdict(policy, toolNameBash, input, cause)
		}
	}
	if call, isSingle := singleLiteralCall(input.ToolInput.Command, policy); isSingle {
		if cause, arg := roots.protected.argCause(input.Cwd, home, call); cause != "" {
			return grants.denyVerdict(policy, toolNameBash, input, cause, pathDetail(input.Cwd, home, arg))
		}
		if call.head == "sed" {
			return Verdict{Decision: decisionAllow, Reason: policy.Reasons.SedWrite}
		}
		isPathFlagAllowed := isPathFlagCallAllowed(
			input,
			grants,
			roots,
			call,
			policy.FileCommands,
		)
		if isPathFlagAllowed {
			return Verdict{Decision: decisionAllow, Reason: policy.Reasons.PathFlag}
		}
	}
	if isAcceptEditsFileCommand(input, policy) {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.AcceptEdits}
	}
	return decideByRules(toolNameBash, input, policy, grants)
}

func decideCompound(classified Verdict, input *HookInput, policy *toolpolicy.Policy, grants *Grants, roots *sessionRoots, home string) Verdict {
	if classified.Cause != policy.Causes.Compound {
		return classified
	}
	if !isAllowedPipeline(input, policy, grants, roots, home) {
		return classified
	}
	verdict := Verdict{
		Decision: decisionAllow,
		Reason:   policy.Reasons.Pipeline,
	}
	return verdict
}

func decideByRules(tool string, input *HookInput, policy *toolpolicy.Policy, grants *Grants) Verdict {
	if rule := grants.matchAllow(tool, &input.ToolInput); rule != nil {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.SettingsRule + rule.Raw}
	}
	if rule := grants.matchSkillAllow(tool, &input.ToolInput); rule != nil {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.SkillGrant + rule.Raw}
	}
	if tool == toolNameSkill && slices.Contains(grants.ActiveSkills, input.ToolInput.Skill) {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.ActiveSkill + input.ToolInput.Skill}
	}
	if tool == toolNameSkill && slices.Contains(grants.PromptSkills, input.ToolInput.Skill) {
		return Verdict{Decision: decisionAllow, Reason: policy.Reasons.PromptSkill + input.ToolInput.Skill}
	}
	return grants.denyVerdict(policy, tool, input, policy.Causes.NoAllowRule)
}

func decideEnterWorktree(grants *Grants, input *HookInput, roots *sessionRoots) Verdict {
	if input.ToolInput.Path == "" {
		return decideByRules(enterWorktreeTool, input, roots.policy, grants)
	}
	if roots.anchor == "" {
		return grants.denyVerdict(roots.policy, enterWorktreeTool, input, roots.policy.Causes.AnchorRemoved)
	}

	container := filepath.Join(gitroot.MainRoot(roots.anchor), grants.worktreesDir)
	containers := []string{container}
	target := resolvePath(input.Cwd, input.ToolInput.Path)
	isContainerItself := target == resolvePath(input.Cwd, container)
	isInside := IsInsideRoots(input.Cwd, target, containers) && !isContainerItself
	if !isInside {
		return grants.denyVerdict(roots.policy, enterWorktreeTool, input, roots.policy.Causes.WorktreeOutside)
	}
	return decideByRules(enterWorktreeTool, input, roots.policy, grants)
}

func decideRead(input *HookInput, grants *Grants, roots *sessionRoots) Verdict {
	access := roots.canRead(input.Cwd, readPath(&input.ToolInput))
	if access.cause != "" {
		return grants.denyVerdict(roots.policy, input.ToolName, input, access.cause)
	}
	if pattern := globPatternBase(input); pattern != "" {
		if cause := roots.canRead(input.Cwd, pattern).cause; cause != "" {
			return grants.denyVerdict(roots.policy, input.ToolName, input, cause)
		}
	}
	return Verdict{Decision: decisionAllow, Reason: access.reason}
}

func applySelfService(input *HookInput, policy *toolpolicy.Policy, grants *Grants, statePath string, state *sessionstate.SessionState, target string, settings *config.Settings, allow []string, outcome string) Verdict {
	if err := sessionstate.Save(statePath, state); err != nil {
		return grants.denyVerdict(policy, toolNameBash, input, "tool-policy state unwritable: "+err.Error())
	}
	if err := writePermissionList(target, settings, allowKey, allow, grants.trash); err != nil {
		return grants.denyVerdict(policy, toolNameBash, input, unlockSettings+" write failed after validation: "+err.Error())
	}
	return appliedVerdict(outcome)
}

func decideSelfServiceCommand(input *HookInput, policy *toolpolicy.Policy, grants *Grants, home string, statePath string) Verdict {
	causes := &policy.Causes
	call, isSingle := singleLiteralCall(input.ToolInput.Command, policy)
	isValidShape := isSingle && len(call.args) > 0 && call.head == commandHead
	if !isValidShape {
		return grants.denyVerdict(policy, toolNameBash, input, causes.SettingsUsage)
	}
	if call.args[0] == commandCheck {
		return decideCheckCommand(input, policy, grants, home, call.args[1:])
	}
	if call.args[0] == commandFreeze {
		return freezePlanRules(input, policy, grants, statePath, call.args[1:])
	}
	if !grants.HasHumanTurn {
		return grants.denyVerdict(policy, toolNameBash, input, causes.SettingsInteractive)
	}

	if statePath == "" {
		return grants.denyVerdict(policy, toolNameBash, input, causes.StateUnavailable+"no state path (ledger or session id missing)")
	}
	state, err := sessionstate.Load(statePath)
	if err != nil {
		return grants.denyVerdict(policy, toolNameBash, input, causes.StateUnavailable+err.Error())
	}

	anchor := grants.anchorRoot(input)
	if anchor == "" {
		return grants.denyVerdict(policy, toolNameBash, input, causes.AnchorRemoved)
	}

	target := filepath.Join(anchor, ".claude", unlockSettings)
	settings, allow, err := allowedRules(target)
	if err != nil {
		return grants.denyVerdict(policy, toolNameBash, input, unlockSettings+" current file does not load — the user fixes it: "+err.Error())
	}

	local := &state.SettingsLocal
	switch call.args[0] {
	case commandRevert:
		return revertSelfService(input, policy, grants, statePath, state, target, settings, allow)
	case commandAllow:
		rules := call.args[1:]
		if isAllow, hasLine := grants.Approvals[unlockSettings]; hasLine && !isAllow {
			return grants.denyVerdict(policy, toolNameBash, input, causes.SettingsRevoked)
		}
		fresh, err := checkNewRules(rules, allow, policy)
		if err != nil {
			return grants.denyVerdict(policy, toolNameBash, input, unlockSettings+" rules rejected: "+err.Error())
		}
		if uncovered := uncoveredRules(fresh, grants); len(uncovered) > 0 {
			return grants.denyVerdict(policy, toolNameBash, input, causes.SettingsUncovered+strings.Join(uncovered, " "))
		}

		outcome := fmt.Sprintf("appended %d rule(s), %d already present", len(fresh), len(rules)-len(fresh))
		if len(fresh) == 0 {
			return appliedVerdict(outcome)
		}
		local.IsReverted = false
		local.LastAppended = fresh
		return applySelfService(input, policy, grants, statePath, state, target, settings, append(allow, fresh...), outcome)
	}
	return grants.denyVerdict(policy, toolNameBash, input, causes.SettingsUsage)
}

func decideWrite(input *HookInput, grants *Grants, roots *sessionRoots) Verdict {
	path := writePath(&input.ToolInput)
	if cause := roots.protected.cause(input.Cwd, roots.home, path); cause != "" {
		return grants.denyVerdict(roots.policy, input.ToolName, input, cause, pathDetail(input.Cwd, roots.home, path))
	}

	access := roots.canWrite(input.Cwd, path)
	if access.cause != "" {
		return grants.denyVerdict(roots.policy, input.ToolName, input, access.cause)
	}
	return Verdict{Decision: decisionAllow, Reason: access.reason}
}

func (g *Grants) denyVerdict(policy *toolpolicy.Policy, tool string, input *HookInput, cause string, details ...string) Verdict {
	candidate := NarrowestRule(tool, &input.ToolInput, g.gitAdmittedFlags)
	stated := strings.Join(append([]string{cause}, details...), ": ")
	reason := "tool-policy: denied " + tool + " — " + stated + ". candidate rule: " +
		candidate + ". " + policy.Steers.Deny
	if tool == toolNameBash {
		reason += " " + policy.Steers.Bash
	}
	verdict := Verdict{
		CandidateRule: candidate,
		Cause:         cause,
		Decision:      decisionDeny,
		Reason:        reason,
	}
	return verdict
}

func isAllowedPipeline(input *HookInput, policy *toolpolicy.Policy, grants *Grants, roots *sessionRoots, home string) bool {
	segments, isPipeline := pipelineSegments(input.ToolInput.Command, policy)
	if !isPipeline {
		return false
	}

	for _, segment := range segments {
		if !isAllowedSegment(segment, input.Cwd, home, policy, grants, roots) {
			return false
		}
	}
	return true
}

func isAllowedSegment(segment *pipelineSegment, cwd string, home string, policy *toolpolicy.Policy, grants *Grants, roots *sessionRoots) bool {
	segmentInput := &ToolInput{Command: segment.text}
	if grants.matchDeny(toolNameBash, segmentInput) != nil {
		return false
	}
	if isReadOnlyCall(segment.call.args, false, segment.call.head, policy) {
		return true
	}
	makeRefusal := ""
	if segment.call.head == makeHead {
		makeRefusal = makeCause(
			segment.call,
			cwd,
			home,
			roots.anchor,
			policy,
		)
	}
	if makeRefusal != "" {
		return false
	}
	if cause, _ := roots.protected.argCause(cwd, home, segment.call); cause != "" {
		return false
	}

	isSettingsAllowed := grants.matchAllow(toolNameBash, segmentInput) != nil
	isSkillAllowed := grants.matchSkillAllow(toolNameBash, segmentInput) != nil
	return isSettingsAllowed || isSkillAllowed
}

// Only the head is checked here, so a malformed call gets the usage cause instead of falling through.
func isSelfServiceCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	return len(fields) > 0 && fields[0] == commandHead
}

// revertSelfService removes exactly the last allow call's rules, once.
func revertSelfService(input *HookInput, policy *toolpolicy.Policy, grants *Grants, statePath string, state *sessionstate.SessionState, target string, settings *config.Settings, allow []string) Verdict {
	local := &state.SettingsLocal
	isRevertable := len(local.LastAppended) > 0 && !local.IsReverted
	if !isRevertable {
		return grants.denyVerdict(policy, toolNameBash, input, unlockSettings+" nothing to revert this session")
	}

	remaining := slices.DeleteFunc(slices.Clone(allow), func(rule string) bool {
		return slices.Contains(local.LastAppended, rule)
	})
	local.IsReverted = true
	return applySelfService(input, policy, grants, statePath, state, target, settings, remaining, "reverted the last toolpolicy allow")
}

func isAcceptEditsFileCommand(input *HookInput, policy *toolpolicy.Policy) bool {
	if !isFileCommandMode(input.PermissionMode) {
		return false
	}
	call, isSingle := singleLiteralCall(input.ToolInput.Command, policy)
	return isSingle && slices.Contains(policy.FileCommands, call.head)
}

// Auto mode keeps acceptEdits' file-command scope: the fence has placed every path inside the write roots.
func isFileCommandMode(mode string) bool {
	return mode == permissionModeAcceptEdits || mode == permissionModeAuto
}

func isPathFlagCallAllowed(input *HookInput, grants *Grants, roots *sessionRoots, call *resolvedCall, fileCommands []string) bool {
	if !isSafePathFlagCall(call) {
		return false
	}
	if !arePathArgsInsideRoots(input.Cwd, call.args, roots.writeDirs) {
		return false
	}

	isAcceptEditsScope := isFileCommandMode(input.PermissionMode) && slices.Contains(fileCommands, call.head)
	isRuleAllowed := grants.matchAllow(toolNameBash, &input.ToolInput) != nil
	return isAcceptEditsScope || isRuleAllowed
}

func settingsLayerPaths(input *HookInput, grants *Grants, home string) []string {
	roots := []string{home, input.Cwd}
	if anchor := grants.anchorRoot(input); anchor != "" && anchor != input.Cwd {
		roots = append(roots, anchor)
	}
	if main := gitroot.MainRoot(input.Cwd); main != input.Cwd {
		roots = append(roots, main)
	}
	roots = append(roots, grants.AdditionalDirectories...)
	paths := make([]string, 0, settingsLayersPerRoot*len(roots))
	for _, root := range roots {
		paths = append(paths,
			filepath.Join(root, ".claude", "settings.json"),
			filepath.Join(root, ".claude", "settings.local.json"),
		)
	}
	return paths
}

// approvableCause names the exact message that unlocks target for the session.
func approvableCause(target string) string {
	return target + " is protected — the user unlocks it for this session by sending the message: " +
		wordAllow + " " + target + " (re-lock: " + wordDeny + " " + target + ")"
}

func arePathArgsInsideRoots(cwd string, args []string, roots []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !IsInsideRoots(cwd, arg, roots) {
			return false
		}
	}
	return true
}

func fenceCause(input *HookInput, policy *toolpolicy.Policy, home string) string {
	if len(policy.ReadFence) == 0 {
		return ""
	}

	for _, path := range []string{readPath(&input.ToolInput), writePath(&input.ToolInput)} {
		isFenced := path != "" && IsInsideRoots(input.Cwd, hookconfig.ExpandHome(path, home), policy.ReadFence)
		if isFenced {
			return policy.Causes.Fenced
		}
	}
	for _, root := range policy.ReadFence {
		if namesFencedRoot(input.ToolInput.Command, home, root) {
			return policy.Causes.Fenced
		}
	}
	return ""
}

func makeCause(call *resolvedCall, cwd string, home string, anchor string, policy *toolpolicy.Policy) string {
	if !isSafeMakeCall(call, cwd, home, policy.MakefileNames) {
		return policy.Causes.MakeArgs
	}

	isWorktreeSession := anchor != "" && isLinkedWorktree(anchor)
	if isWorktreeSession && isOperatorMakeCall(call, cwd, home, policy.MakefileNames) {
		return policy.Causes.OperatorCommand
	}
	return ""
}

func namesFencedRoot(command string, home string, root string) bool {
	if root == "" {
		return false
	}
	if strings.Contains(command, root) {
		return true
	}

	relative, isUnderHome := strings.CutPrefix(root, home+string(filepath.Separator))
	if !isUnderHome {
		return false
	}
	return slices.ContainsFunc(homeSpellings, func(spelling string) bool {
		return strings.Contains(command, spelling+relative)
	})
}

func pathDetail(cwd string, home string, path string) string {
	return resolvePath(cwd, hookconfig.ExpandHome(path, home)) + " (cwd " + cwd + ")"
}

func globPatternBase(input *HookInput) string {
	pattern := input.ToolInput.Pattern
	if input.ToolName != "Glob" || !isPathShaped(pattern) {
		return ""
	}
	prefix := globPrefix(pattern)
	if filepath.IsAbs(prefix) || strings.HasPrefix(prefix, "~") {
		return prefix
	}
	base := readPath(&input.ToolInput)
	if base == "" {
		base = input.Cwd
	}
	return filepath.Join(base, prefix)
}

func agentModelCause(causes *hookconfig.ToolpolicyCauses, tool string, vocabulary *models.Vocabulary, model string) string {
	if tool != toolNameAgent {
		return ""
	}
	if vocabulary == nil {
		return causes.AgentModels
	}
	if err := vocabulary.CheckSubagent(model); err != nil {
		return err.Error()
	}
	return ""
}

func readPath(input *ToolInput) string {
	if input.FilePath != "" {
		return input.FilePath
	}
	return input.Path
}

func writePath(input *ToolInput) string {
	if input.FilePath != "" {
		return input.FilePath
	}
	return input.NotebookPath
}
