// Command skillenv exports a session's repo-scoped skill values into the harness's environment file and refuses an invoked skill whose required value is unset.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/skillenv"
)

// HookInput is the slice of the SessionStart, CwdChanged, UserPromptExpansion and PreToolUse envelopes the hook reads.
type HookInput struct {
	CommandName   string    `json:"command_name"`
	Cwd           string    `json:"cwd"`
	ExpansionType string    `json:"expansion_type"`
	HookEventName string    `json:"hook_event_name"`
	SessionId     string    `json:"session_id"`
	ToolInput     ToolInput `json:"tool_input"`
}

// skillName is the skill the envelope invokes: a typed command's name or a Skill call's input, "" for anything else.
func (i *HookInput) skillName() string {
	switch i.HookEventName {
	case eventUserPromptExpansion:
		if i.ExpansionType == expansionSlashCommand {
			return i.CommandName
		}
	case eventPreToolUse:
		return i.ToolInput.Skill
	}
	return ""
}

// HookOutput is the check's answer: a typed command's block, or the specific output of a Skill call's deny or of either event's context.
type HookOutput struct {
	Decision           string              `json:"decision,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
	Reason             string              `json:"reason,omitempty"`
}

// HookSpecificOutput carries a Skill call's deny or the context lines; an empty decision leaves the call to the other hooks.
type HookSpecificOutput struct {
	AdditionalContext        string `json:"additionalContext,omitempty"`
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

// ToolInput is the Skill tool's input; its args stay unread.
type ToolInput struct {
	Skill string `json:"skill"`
}

// sources is what both modes resolve from; notes keeps each read failure for the refusal.
type sources struct {
	claudeHome string
	notes      []string
	record     install.Record
	registry   *repos.Registry
	rows       *skillenv.Rows
}

// appendExports writes the lines to the env file; the harness sources it before every Bash command.
func appendExports(envFile string, lines []string) error {
	file, err := os.OpenFile(envFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, envFileMode)
	if err != nil {
		return fmt.Errorf("appendExports: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return fmt.Errorf("appendExports: %w", err)
	}
	return nil
}

// check refuses an invoked skill whose required repo value is unset, else adds one context line per value its body reads; a skill without env.json passes silently.
func check(input *HookInput, loaded *sources, stderr io.Writer, stdout io.Writer) int {
	name := input.skillName()
	if !leafPattern.MatchString(name) {
		return 0
	}

	path := filepath.Join(loaded.claudeHome, skillsDirName, name, skillenv.FileName)
	document, err := skillenv.LoadDocument(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "check: %v\n", err)
		return 0
	}

	documents := []*skillenv.Document{document}
	bindings := skillenv.Bindings(documents)
	values := skillenv.Resolve(bindings, input.Cwd, loaded.registry, loaded.rows)
	missing := unsetRequired(document, values)
	if len(missing) > 0 {
		reason := refusalReason(input, loaded, missing, document.Skill)
		output := refusal(input.HookEventName, reason)
		return writeOutput(output, stderr, stdout)
	}

	lines := contextLines(document, values)
	output := contextOutput(input.HookEventName, lines)
	return writeOutput(output, stderr, stdout)
}

// contextLines is one line per variable the document lists: its value and source, or not set.
func contextLines(document *skillenv.Document, values map[string]skillenv.Value) []string {
	lines := make([]string, 0, len(document.Variables))
	for _, entry := range document.Variables {
		value, isSet := values[entry.Name]
		if !isSet {
			lines = append(lines, fmt.Sprintf(contextUnsetFormat, entry.Name))
			continue
		}

		line := fmt.Sprintf(
			contextValueFormat,
			entry.Name,
			strconv.Quote(value.Value),
			sourceLabel(value),
		)
		lines = append(lines, line)
	}
	return lines
}

// contextOutput adds the lines as context beside the expanded command or the tool result.
func contextOutput(event string, lines []string) *HookOutput {
	specific := &HookSpecificOutput{AdditionalContext: strings.Join(lines, "\n"), HookEventName: event}
	return &HookOutput{HookSpecificOutput: specific}
}

// export appends the session repo's values to the env file, the deployed sidecars binding the global defaults.
func export(cwd string, envFile string, loaded *sources, stderr io.Writer) int {
	skillsDir := filepath.Join(loaded.claudeHome, skillsDirName)
	documents, err := skillenv.LoadDocuments(skillsDir)
	if err != nil {
		fmt.Fprintf(stderr, "export: %v\n", err)
	}

	bindings := skillenv.Bindings(documents)
	values := skillenv.Resolve(bindings, cwd, loaded.registry, loaded.rows)
	lines := skillenv.ExportLines(values)
	if len(lines) == 0 {
		return 0
	}
	if err := appendExports(envFile, lines); err != nil {
		fmt.Fprintf(stderr, "export: %v\n", err)
	}
	return 0
}

// fixLines is the skill page link, or one export line per variable when the record installs no config server.
func fixLines(link string, missing []*skillenv.Entry) []string {
	if link != "" {
		return []string{fmt.Sprintf(linkHintFormat, link)}
	}

	lines := make([]string, 0, len(missing))
	for _, entry := range missing {
		lines = append(lines, fmt.Sprintf(exportHintFormat, entry.Name))
	}
	return lines
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillenv: isOff: %v\n", err)
	}
	return isOff
}

// loadSources reads the record and the registry once for either mode; a failure leaves that source empty and its note for the refusal.
func loadSources(claudeHome string, reposPath string, stderr io.Writer) *sources {
	loaded := &sources{claudeHome: claudeHome}
	record, err := install.LoadRecordAt(claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "loadSources: %v\n", err)
		loaded.notes = append(loaded.notes, err.Error())
		record = install.Record{}
	}
	loaded.record = record
	loaded.rows = &skillenv.Rows{Defaults: record.Skills.Defaults, Variables: record.Skills.Variables}

	registry := repos.NewRegistry(reposPath)
	if err := registry.Reload(); err != nil {
		fmt.Fprintf(stderr, "loadSources: %v\n", err)
		loaded.notes = append(loaded.notes, err.Error())
		return loaded
	}
	loaded.registry = registry
	return loaded
}

// main answers nothing in a smoke session: the runner set the smoke's own values and SkillEnvOffEnv, which neither mode may override.
func main() {
	if os.Getenv(config.SkillEnvOffEnv) != "" {
		os.Exit(0)
	}
	os.Exit(run(os.Args[1:], os.Getenv(envFileVariable), os.Stderr, os.Stdin, os.Stdout))
}

// refusal blocks a typed command or denies a Skill call; the harness shows a block's reason to the user, a deny's to the agent.
func refusal(event string, reason string) *HookOutput {
	if event != eventPreToolUse {
		return &HookOutput{Decision: decisionBlock, Reason: reason}
	}

	specific := &HookSpecificOutput{HookEventName: eventPreToolUse, PermissionDecision: decisionDeny, PermissionDecisionReason: reason}
	return &HookOutput{HookSpecificOutput: specific}
}

// refusalReason names each unset required variable, then the fix, the shell hint and the read failures.
func refusalReason(input *HookInput, loaded *sources, missing []*skillenv.Entry, skill string) string {
	repo, isRegistered := registeredRepo(input.Cwd, loaded.registry)
	where := unregisteredWhere
	if isRegistered {
		where = fmt.Sprintf(registeredWhereFormat, repo)
	}

	var lines []string
	for _, entry := range missing {
		line := fmt.Sprintf(
			unsetFormat,
			entry.Name,
			where,
			skillenv.Phrase(entry.Summary),
			entry.Example,
		)
		lines = append(lines, line)
	}
	link := skillPageLink(loaded.record, repo, skill)
	lines = append(lines, fixLines(link, missing)...)
	lines = append(lines, fmt.Sprintf(shellHintFormat, input.skillName()))
	for _, note := range loaded.notes {
		lines = append(lines, fmt.Sprintf(noteFormat, note))
	}
	return strings.Join(lines, "\n")
}

// registeredRepo is the registry row name of cwd's main checkout.
func registeredRepo(cwd string, registry *repos.Registry) (string, bool) {
	if registry == nil {
		return "", false
	}

	row, isRegistered := registry.FindByPath(gitroot.MainRoot(cwd))
	if !isRegistered {
		return "", false
	}
	return row.Name, true
}

// run exits 0 whatever it finds: a hook that fails must never stop a session or an invocation.
func run(args []string, envFile string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	home, err := config.ClaudeHomeDir("")
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 0
	}
	stateDir, err := config.StateDir(home)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 0
	}

	flags := flag.NewFlagSet("skillenv", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", home, "the Claude home the record, the deployed skills and the session switches are read from")
	envPath := flags.String("env-file", envFile, "the file the exports are appended to; default $CLAUDE_ENV_FILE")
	reposPath := flags.String("repos", filepath.Join(stateDir, config.StateRegistryFile), "repo registry")
	if err := flags.Parse(args); err != nil {
		return 0
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 0
	}
	isExport := input.HookEventName == eventSessionStart || input.HookEventName == eventCwdChanged
	isCheck := input.HookEventName == eventUserPromptExpansion || input.HookEventName == eventPreToolUse
	isExportDue := isExport && *envPath != ""
	isDue := isExportDue || isCheck
	if !isDue || input.Cwd == "" {
		return 0
	}
	if isOff(*claudeHome, input.SessionId, sessionstate.InjectEnv) {
		return 0
	}

	loaded := loadSources(*claudeHome, *reposPath, stderr)
	if isExport {
		return export(input.Cwd, *envPath, loaded, stderr)
	}
	return check(input, loaded, stderr, stdout)
}

// skillPageLink is the config server's page of the skill, opened on repo when registered; "" when the record installs no config server.
func skillPageLink(record install.Record, repo string, skill string) string {
	if !install.Selects(record, runtime.GOOS, components.ConfigServer) {
		return ""
	}
	port, err := install.Resolve(record, components.ConfigServer, components.PortKey)
	if err != nil {
		return ""
	}

	link := fmt.Sprintf(skillPageFormat, port.Int, url.PathEscape(skill))
	if repo == "" {
		return link
	}
	return link + repoQuery + url.QueryEscape(repo)
}

// sourceLabel names where a value came from, a repo value by its registry name.
func sourceLabel(value skillenv.Value) string {
	if value.Source == skillenv.SourceRepo {
		return string(value.Source) + " " + value.Repo
	}
	return string(value.Source)
}

// unsetRequired lists the document's required variables the resolution left unset.
func unsetRequired(document *skillenv.Document, values map[string]skillenv.Value) []*skillenv.Entry {
	var missing []*skillenv.Entry
	for _, entry := range document.Variables {
		_, isSet := values[entry.Name]
		if entry.Required && !isSet {
			missing = append(missing, entry)
		}
	}
	return missing
}

// writeOutput encodes the answer on stdout; a failure is logged and the call passes.
func writeOutput(output *HookOutput, stderr io.Writer, stdout io.Writer) int {
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "writeOutput: Failed to encode the answer: %v\n", err)
	}
	return 0
}
