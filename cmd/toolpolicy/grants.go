package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skillenv"
	"github.com/kevinhorst/smine/internal/transcript"
	"github.com/kevinhorst/smine/internal/trash"
)

// The command fdesign's enforce block requires after every plan edit; group 1 is the plan file.
var sectionCheckPattern = regexp.MustCompile(`^smine-rules plan-sections (\S+)$`)

type Grants struct {
	anchor           sessionAnchor
	freeze           *freezeGuard
	gateRepo         string
	gitAdmittedFlags []string
	repoPaths        []string
	trash            *trash.Store // keeps the previous bytes of every settings file the gate writes; nil keeps nothing
	values           map[string]string
	worktreesDir     string

	ActiveSkills          []string
	AdditionalDirectories []string
	Allow                 []*Rule
	Approvals             map[string]bool
	Deny                  []*Rule
	Enforce               []*GateRules
	HasHumanTurn          bool
	LastTurnOrigin        string
	Models                *models.Vocabulary // nil: the vocabulary is unreadable, every Agent call is denied
	PromptSkills          []string           // the skills the user's newest message names as "/<skill>" words
	SettingsMessages      []settingsMessage  // the #add, #set, #disable and #enable messages since the last #reset, transcript order
	SkillAllow            []*Rule
}

// Subagent calls resolve their skills through the parent transcript the envelope carries.
func LoadGrants(input *HookInput, cfg *hookconfig.Config, home string, gateRepo string, reposPath string, frozenLedger string) (*Grants, error) {
	policy := &cfg.Toolpolicy
	scan := scanTranscript(input.TranscriptPath, policy.DesignSkill)
	worktreesDir := cfg.Worktreecreate.WorktreesDir
	if worktreesDir == "" {
		worktreesDir = harnessWorktreesDir
	}
	grants := &Grants{
		anchor:           resolveAnchor(input, scan.trackedAnchor, worktreesDir),
		gitAdmittedFlags: policy.GitAdmittedFlags,
		worktreesDir:     worktreesDir,
	}
	if gateRepo != "" {
		grants.gateRepo = filepath.Clean(gateRepo)
	}
	anchor := grants.anchorRoot(input)
	grants.freeze = newFreezeGuard(
		anchor,
		designPrompts(&scan, policy.DesignSkill),
		frozenLedger,
		scan.sectionedPlans,
		input.SessionId,
	)
	registry := loadRegistry(reposPath)
	grants.repoPaths = foreignRepoPaths(anchor, registeredRepoPaths(registry))
	grants.values = sessionValues(anchor, filepath.Join(home, ".claude"), scan.skillEvents, registry)
	layers := []string{
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "settings.local.json"),
		policy.ManagedSettingsPath,
	}
	if anchor != "" {
		layers = append(layers,
			filepath.Join(anchor, ".claude", "settings.json"),
			filepath.Join(anchor, ".claude", "settings.local.json"),
		)
	}
	for _, layer := range layers {
		grants.loadLayer(layer, home)
	}

	vocabulary, err := models.LoadHome(filepath.Join(home, ".claude"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: LoadGrants: Model vocabulary unreadable, every Agent call is denied: %v\n", err)
	}
	grants.Models = vocabulary

	grants.Approvals = scan.approvals
	grants.HasHumanTurn = scan.hasHumanTurn
	grants.LastTurnOrigin = scan.lastTurnOrigin
	grants.PromptSkills = scan.promptSkills
	grants.SettingsMessages = scan.settingsMessages
	grants.activateSkills(scan.skillEvents, home)
	return grants, nil
}

func (g *Grants) activateSkills(events []skillEvent, home string) {
	g.ActiveSkills = make([]string, 0)
	for _, event := range events {
		if slices.Contains(g.ActiveSkills, event.skill) {
			continue
		}

		isAdmittedCall := event.isPromptNamed || g.admitsSkillCall(event.skill)
		if event.isToolCall && !isAdmittedCall {
			continue
		}

		g.ActiveSkills = append(g.ActiveSkills, event.skill)
		if manifest := readSkillManifest(home, event.skill, g.values); manifest != nil {
			g.SkillAllow = append(g.SkillAllow, manifest.Rules...)
		}
		if rules := loadGateRules(home, event.skill); rules != nil {
			g.Enforce = append(g.Enforce, rules)
		}
	}
}

func (g *Grants) admitsSkillCall(skill string) bool {
	call := &ToolInput{Skill: skill}
	isSettingsAllowed := g.matchAllow(skillToolName, call) != nil
	isSkillAllowed := g.matchSkillAllow(skillToolName, call) != nil
	return isSettingsAllowed || isSkillAllowed
}

func (g *Grants) anchorRoot(input *HookInput) string {
	switch {
	case g.anchor.isMissing:
		return ""
	case g.anchor.path == "":
		return input.Cwd
	}
	return g.anchor.path
}

func (g *Grants) loadLayer(path string, home string) {
	settings, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: Grants.loadLayer: Skipping unparseable layer %s: %v\n", path, err)
		return
	}

	perms, err := settings.Permissions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: Grants.loadLayer: Skipping layer %s with unreadable permissions: %v\n", path, err)
		return
	}

	g.Allow = append(g.Allow, parseRules(perms.Allow, path)...)
	g.Deny = append(g.Deny, parseRules(perms.Deny, path)...)
	for _, dir := range perms.AdditionalDirectories {
		g.AdditionalDirectories = append(g.AdditionalDirectories, hookconfig.ExpandHome(dir, home))
	}
}

func (g *Grants) matchAllow(toolName string, input *ToolInput) *Rule {
	return firstMatch(g.Allow, toolName, input, g.gitAdmittedFlags)
}

func (g *Grants) matchDeny(toolName string, input *ToolInput) *Rule {
	return firstMatch(g.Deny, toolName, input, g.gitAdmittedFlags)
}

func (g *Grants) matchSkillAllow(toolName string, input *ToolInput) *Rule {
	return firstMatch(g.SkillAllow, toolName, input, g.gitAdmittedFlags)
}

func (g *Grants) skillPathGrants(home string, tools []string) []*pathGrant {
	grants := make([]*pathGrant, 0)
	for _, rule := range g.SkillAllow {
		isPathRule := rule.Kind == ruleKindPathPattern && slices.Contains(tools, rule.Tool)
		if !isPathRule {
			continue
		}
		for _, root := range pathRuleRoots(home, g.repoPaths, rule.Spec) {
			grants = append(grants, &pathGrant{root: root, rule: rule})
		}
	}
	return grants
}

func firstMatch(rules []*Rule, toolName string, input *ToolInput, gitAdmittedFlags []string) *Rule {
	matched := input
	if toolName == toolNameBash {
		ruleForm := *input
		ruleForm.Command = gitRuleCommand(input.Command, gitAdmittedFlags)
		matched = &ruleForm
	}
	for _, rule := range rules {
		if rule.Matches(toolName, matched) {
			return rule
		}
	}
	return nil
}

func foreignRepoPaths(anchor string, paths []string) []string {
	if anchor == "" {
		return paths
	}

	own := resolvePath(anchor, gitroot.MainRoot(anchor))
	return slices.DeleteFunc(slices.Clone(paths), func(path string) bool {
		return resolvePath(anchor, path) == own
	})
}

// loadRegistry is nil without a readable registry; every reader treats nil as empty.
func loadRegistry(reposPath string) *repos.Registry {
	if reposPath == "" {
		return nil
	}
	if _, err := os.Stat(reposPath); err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: loadRegistry: Skipping repo registry %s: %v\n", reposPath, err)
		return nil
	}

	registry := repos.NewRegistry(reposPath)
	if err := registry.Reload(); err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: loadRegistry: Skipping repo registry %s: %v\n", reposPath, err)
		return nil
	}
	return registry
}

func registeredRepoPaths(registry *repos.Registry) []string {
	if registry == nil {
		return nil
	}

	paths := make([]string, 0)
	for _, repo := range registry.Repos() {
		paths = append(paths, repo.Path)
	}
	return paths
}

// sessionValues resolves the anchor's repo values the way the skillenv hook exports them, the events' sidecars binding the global defaults; an unreadable record yields the registry row alone.
func sessionValues(anchor string, claudeHome string, events []skillEvent, registry *repos.Registry) map[string]string {
	rows := &skillenv.Rows{}
	if record, err := install.LoadRecordAt(claudeHome); err == nil {
		rows = &skillenv.Rows{Defaults: record.Skills.Defaults, Variables: record.Skills.Variables}
	}

	documents := make([]*skillenv.Document, 0, len(events))
	for _, event := range events {
		path := filepath.Join(claudeHome, "skills", event.skill, skillenv.FileName)
		document, err := skillenv.LoadDocument(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "toolpolicy: sessionValues: %v\n", err)
			continue
		}
		documents = append(documents, document)
	}

	bindings := skillenv.Bindings(documents)
	values := skillenv.Resolve(bindings, anchor, registry, rows)
	return skillenv.ValueMap(values)
}

type skillEvent struct {
	isPromptNamed bool
	isToolCall    bool
	skill         string
}

type transcriptScan struct {
	approvals        map[string]bool
	designPrompts    []string
	designSkill      string
	firstPrompt      string
	hasHumanTurn     bool
	hasPrompt        bool
	lastTurnOrigin   string
	promptSkills     []string
	settingsMessages []settingsMessage
	skillEvents      []skillEvent
	sectionedPlans   []string
	trackedAnchor    string
}

func (s *transcriptScan) observePrompt(line *transcript.Line) {
	if !isPromptAuthor(line) {
		return
	}

	text := line.Message.Content.LeadingText()
	if !s.hasPrompt {
		s.hasPrompt = true
		s.firstPrompt = text
	}

	skill, args, isCommand := transcript.SlashCommand(text)
	isDesignPrompt := isCommand && skill == s.designSkill && transcript.IsHuman(line)
	if isDesignPrompt {
		s.designPrompts = append(s.designPrompts, args)
	}
}

func (s *transcriptScan) observeTurn(line *transcript.Line) {
	isStampedTurn := line.Type == transcript.TypeUser && line.TurnOrigin != ""
	switch {
	case isStampedTurn:
		s.lastTurnOrigin = line.TurnOrigin
	case transcript.IsHumanQueuedPrompt(line):
		s.lastTurnOrigin = transcript.OriginHuman
	}
}

// observeSectionChecks keeps the plan file every `smine-rules plan-sections` call of the session names.
func (s *transcriptScan) observeSectionChecks(line *transcript.Line) {
	if line.Type != transcript.TypeAssistant {
		return
	}

	for _, block := range line.Message.Content.Blocks {
		isBashCall := block.Name == toolNameBash && block.Type == transcript.BlockToolUse
		if !isBashCall {
			continue
		}
		if match := sectionCheckPattern.FindStringSubmatch(strings.TrimSpace(block.Input.Command)); match != nil {
			s.sectionedPlans = append(s.sectionedPlans, match[1])
		}
	}
}

// observeHashtags folds every command line of a human message: approvals last-wins per target, settings messages in order.
func (s *transcriptScan) observeHashtags(line *transcript.Line) {
	for _, text := range transcript.HumanTexts(line) {
		commands := parseHashtags(text)
		for _, grant := range commands.approvals {
			s.approvals[grant.target] = grant.isAllow
		}
		s.settingsMessages = foldMessages(s.settingsMessages, commands.messages, func(message *settingsMessage) bool { return message.hasEffect() })
	}
}

func approvalTarget(target string) string {
	switch target {
	case "-s", "--s", "-settings", "--settings":
		return unlockSettings
	}
	return target
}

func scanTranscript(transcriptPath string, designSkill string) transcriptScan {
	scan := transcriptScan{approvals: map[string]bool{}, designSkill: designSkill, settingsMessages: make([]settingsMessage, 0), skillEvents: make([]skillEvent, 0)}
	if transcriptPath == "" {
		return scan
	}
	file, err := os.Open(transcriptPath)
	if err != nil {
		return scan
	}
	defer file.Close()

	anchor := newAnchorTracker()
	sawFirstUser := false
	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		line := transcript.Line{}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		anchor.observe(&line)
		if transcript.IsHuman(&line) {
			scan.hasHumanTurn = true
		}
		scan.observePrompt(&line)
		scan.observeTurn(&line)
		scan.observeSectionChecks(&line)
		scan.observeHashtags(&line)
		if skills, isHuman := transcript.HumanSlashSkills(&line); isHuman {
			scan.promptSkills = skills
		}
		scan.skillEvents = append(scan.skillEvents, lineSkillEvents(&line, scan.promptSkills, &sawFirstUser)...)
	}
	scan.trackedAnchor = anchor.current
	return scan
}

// isPromptAuthor: a prompt the user typed, or a headless cell's own prompt; every row the harness writes is neither.
func isPromptAuthor(line *transcript.Line) bool {
	kind := transcript.Classify(line)
	return kind == transcript.KindHuman || kind == transcript.KindHeadless
}

func lineSkillEvents(line *transcript.Line, promptSkills []string, sawFirstUser *bool) []skillEvent {
	events := make([]skillEvent, 0)
	switch line.Type {
	case transcript.TypeUser:
		if !isPromptAuthor(line) {
			return events
		}
		text := line.Message.Content.LeadingText()
		typed := transcript.CommandNameSkill(text)
		if !*sawFirstUser {
			*sawFirstUser = true
			typed = append(typed, transcript.PromptStartSkill(text)...)
		}
		for _, skill := range typed {
			event := skillEvent{isToolCall: false, skill: skill}
			events = append(events, event)
		}
	case transcript.TypeAssistant:
		for _, block := range line.Message.Content.Blocks {
			isSkillCall := block.Type == transcript.BlockToolUse && block.Name == skillToolName
			if isSkillCall && block.Input.Skill != "" {
				event := skillEvent{
					isPromptNamed: slices.Contains(promptSkills, block.Input.Skill),
					isToolCall:    true,
					skill:         block.Input.Skill,
				}
				events = append(events, event)
			}
		}
	}
	return events
}

func parseRules(raw []string, source string) []*Rule {
	rules := make([]*Rule, 0, len(raw))
	for _, entry := range raw {
		rule, err := ParseRule(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "toolpolicy: %s: %v\n", source, err)
			continue
		}
		if rule.Kind == ruleKindPathPattern {
			fmt.Fprintf(os.Stderr, "toolpolicy: %s: path pattern rule %q is matched only as a skill grant\n", source, rule.Raw)
		}
		rules = append(rules, rule)
	}
	return rules
}
