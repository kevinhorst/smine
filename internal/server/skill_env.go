package server

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	exitCodeOther   = "other"
	exitCodeZero    = "0"
	gateTimeout     = 10 * time.Second
	noVerdict       = "—"
	placeholderWord = "x"
	providerPrefix  = "PROVIDER_"
	sourceCatalog   = "catalog"
	variablePrefix  = "SKILL_"
)

var (
	gateVerdictPattern = regexp.MustCompile(`^\d+\. (.*) → (.*)$`)
	placeholderPattern = regexp.MustCompile(`<[^>]*>`)
)

// defaultRow is one repo-scoped variable on the skill page: the record default, the global default beneath it, every repo's own value and, with a repo selected, that repo's value.
type defaultRow struct {
	Name string

	Default   string
	Global    string
	Interface *skillgraph.Interface
	Override  string
	Overrides []repoOverride
	Required  bool
}

// exitRow is one result.exit entry, "0" first, "other" last.
type exitRow struct {
	Code    string
	Meaning string
}

// providerRow is one provider in effect: its program, source, claims and the rows it fills.
type providerRow struct {
	Id string

	Cause     string
	Claims    []string
	Override  string
	Program   string
	Source    string
	Variables []variableRow
}

// repoOverride is one registered repo's own value of a repo-scoped variable.
type repoOverride struct {
	Repo  string
	Value string
}

// skillEnvView is the Environment widget: the repo-scoped rows for the default or the selected repo and the provider states, then provider rows with their capability rows and the standalone rows under advanced; a save answers with the widget and its notice.
type skillEnvView struct {
	Defaults      []defaultRow
	GateError     string
	HasGate       bool
	IsError       bool
	Notice        string
	Orphans       []string // record rows no leaf of the checkout declares, each with a Forget button
	Origin        string
	Output        string
	Providers     []providerRow
	Repo          string // the selected registered repo, "" for the defaults
	Repos         []string
	Skill         string
	Standalone    []variableRow
	States        []providerState
	Unset         []string      // the required variables without a value — the deploy's skip reason
	UnsetInstalls []variableRow // the install-scoped ones among them, shown in the visible card; their inputs stay under advanced
}

// count is the number of keys the section's form posts.
func (v *skillEnvView) count() int {
	count := len(v.Providers) + len(v.Standalone)
	for _, provider := range v.Providers {
		count += len(provider.Variables)
	}
	return count
}

// rows lists every variable row of the view for in-place updates.
func (v *skillEnvView) rows() []*variableRow {
	var rows []*variableRow
	for index := range v.Providers {
		for variableIndex := range v.Providers[index].Variables {
			rows = append(rows, &v.Providers[index].Variables[variableIndex])
		}
	}
	for index := range v.Standalone {
		rows = append(rows, &v.Standalone[index])
	}
	return rows
}

// variableRow is one declared variable rendered from its interface.
type variableRow struct {
	Name string

	Cause      string
	Exit       []exitRow
	Gate       string
	Interface  *skillgraph.Interface
	IsDeferred bool
	Override   string
	Readers    []string
	Required   bool
	Source     string
	Value      string
}

// gateTarget is what the gate judges for the row: a command as it stands, a program with its interface's args, placeholders as x; "" when nothing is checkable.
func (r *variableRow) gateTarget() string {
	if r.Interface == nil || r.Value == "" {
		return ""
	}

	switch r.Interface.Kind {
	case skillgraph.KindCommand:
		return r.Value
	case skillgraph.KindProgram:
		return r.Value + " " + placeholderPattern.ReplaceAllString(r.Interface.Invocation.Args, placeholderWord)
	case skillgraph.KindPath, skillgraph.KindSkill:
	}
	return ""
}

// isUnjudged reports a row the gate never judges: a path or a skill name is no command.
func (r *variableRow) isUnjudged() bool {
	if r.Interface == nil {
		return false
	}

	switch r.Interface.Kind {
	case skillgraph.KindCommand, skillgraph.KindProgram:
		return false
	case skillgraph.KindPath, skillgraph.KindSkill:
		return true
	}
	return false
}

// fillGate runs one gate check over every command and program value; path and skill rows read "—", a failed check fills GateError and leaves every verdict "—".
func (s *Server) fillGate(ctx context.Context, leaf string, view *skillEnvView) {
	targets := map[*variableRow]string{}
	for _, row := range view.rows() {
		if row.isUnjudged() {
			row.Gate = noVerdict
		}
		if target := row.gateTarget(); target != "" {
			targets[row] = target
		}
	}
	if len(targets) == 0 {
		return
	}

	uniqueTargets := slices.Sorted(maps.Values(targets))
	verdicts, err := gateVerdicts(ctx, s.machine.Home, s.repoDir, leaf, slices.Compact(uniqueTargets))
	if err != nil {
		view.GateError = err.Error()
	}
	for row, target := range targets {
		verdict, isJudged := verdicts[target]
		if !isJudged {
			verdict = noVerdict
		}
		row.Gate = verdict
	}
}

// skillEnv builds the Environment section of a repo skill for the stored record and its skill rows, opened on repo when it names a registered one; a home-only skill has none.
func (s *Server) skillEnv(ctx context.Context, repo string, skill *skills.Skill) skillEnvView {
	if skill.Origin != skills.OriginRepo {
		return skillEnvView{}
	}
	model, err := skillgraph.Load(skill.Path)
	if err != nil {
		log.Printf("Server.skillEnv: %v", err)
		return skillEnvView{}
	}

	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.skillEnv: %v", err)
	}
	rows := install.SkillRows(record)

	set := install.ExpandedComponents(record, s.machine.GOOS)
	prober := skills.NewProber()
	catalog := model.Catalog
	if catalog == nil {
		catalog = &skillgraph.Catalog{}
	}

	registered := s.repoRegistry.Repos()
	view := skillEnvView{HasGate: slices.Contains(set, components.Gate), Origin: skill.Origin, Repos: repoNames(registered), Skill: skill.Name}
	if slices.Contains(view.Repos, repo) {
		view.Repo = repo
	}
	view.Orphans, err = skills.OrphanRows(s.repoDir, rows)
	if err != nil {
		log.Printf("Server.skillEnv: %v", err)
	}

	declared := model.VariableNames()
	providerRows := map[string]*providerRow{}
	values := skills.Values(ctx, s.repoDir, rows, true, prober, set, model)
	view.Unset = unsetRequired(model, values, rows, registered)
	for _, value := range values {
		if value.IsDeferred {
			view.Defaults = append(view.Defaults, newDefaultRow(rows, registered, view.Repo, &value))
		}

		row := newVariableRow(rows, model, &value)
		isUnsetInstall := !value.IsDeferred && slices.Contains(view.Unset, value.Name)
		if isUnsetInstall {
			view.UnsetInstalls = append(view.UnsetInstalls, row)
		}
		provider := catalog.ProviderFor(declared[value.Name].Interface)
		if provider == nil {
			view.Standalone = append(view.Standalone, row)
			continue
		}
		if providerRows[provider.Id] == nil {
			isInstalled := slices.Contains(set, provider.Component)
			providerRows[provider.Id] = newProviderRow(ctx, rows, isInstalled, prober, provider)
		}
		providerRows[provider.Id].Variables = append(providerRows[provider.Id].Variables, row)
	}

	providerIds := slices.Sorted(maps.Keys(providerRows))
	for _, id := range providerIds {
		view.Providers = append(view.Providers, *providerRows[id])
	}
	view.States = s.providerStates(ctx, rows, []*skillgraph.Skill{model}, prober, record)
	if view.HasGate {
		s.fillGate(ctx, model.Name(), &view)
	}
	return view
}

// applyEnvForm writes the posted SKILL_ and PROVIDER_ fields into rows, an empty field deleting the row; it refuses the first undeclared key or invalid value, other keys pass untouched.
func applyEnvForm(rows *skills.Rows, form url.Values, model *skillgraph.Skill) error {
	catalog := model.Catalog
	if catalog == nil {
		catalog = &skillgraph.Catalog{}
	}

	declared := model.VariableNames()
	providers := map[string]*skillgraph.Provider{}
	for _, variable := range declared {
		if provider := catalog.ProviderFor(variable.Interface); provider != nil {
			providers[skills.ProviderKey(provider.Id)] = provider
		}
	}

	keys := slices.Sorted(maps.Keys(form))
	for _, key := range keys {
		value := formValue(form, key)
		var err error
		switch {
		case strings.HasPrefix(key, variablePrefix):
			err = applyVariableField(rows, catalog, declared[key], key, value)
		case strings.HasPrefix(key, providerPrefix):
			err = applyProviderField(rows, providers[key] != nil, key, value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// applyProviderField writes one PROVIDER_ row; the key must name a provider the skill resolves through, the value a program.
func applyProviderField(rows *skills.Rows, isUsed bool, key, value string) error {
	if !isUsed {
		return fmt.Errorf("unknown provider %s", key)
	}
	if value == "" {
		delete(rows.Providers, key)
		return nil
	}

	programInterface := &skillgraph.Interface{Kind: skillgraph.KindProgram}
	if err := skillgraph.CheckValue(programInterface, value); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if rows.Providers == nil {
		rows.Providers = map[string]string{}
	}
	rows.Providers[key] = value
	return nil
}

// applyVariableField writes one SKILL_ row; the key must be declared, the value pass its interface's value rules.
func applyVariableField(rows *skills.Rows, catalog *skillgraph.Catalog, variable *skillgraph.Variable, key, value string) error {
	if variable == nil {
		return fmt.Errorf("unknown variable %s", key)
	}
	if value == "" {
		delete(rows.Skills, key)
		return nil
	}

	iface, isKnown := catalog.Interface(variable.Interface)
	if !isKnown {
		return fmt.Errorf("%s: interface %s is not in %s", key, variable.Interface, skillgraph.FileCatalog)
	}
	if err := skillgraph.CheckValue(iface, value); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if rows.Skills == nil {
		rows.Skills = map[string]string{}
	}
	rows.Skills[key] = value
	return nil
}

// compareExitCodes orders "0" first, "other" last, the codes between alphabetically.
func compareExitCodes(a, b string) int {
	if byRank := cmp.Compare(exitRank(a), exitRank(b)); byRank != 0 {
		return byRank
	}
	return strings.Compare(a, b)
}

// Ranks of exit codes in an exit table: "0" first, "other" last.
const (
	exitRankFirst = iota
	exitRankMiddle
	exitRankLast
)

// exitRank places "0" before and "other" after every other code.
func exitRank(code string) int {
	switch code {
	case exitCodeZero:
		return exitRankFirst
	case exitCodeOther:
		return exitRankLast
	}
	return exitRankMiddle
}

// exitRows orders an interface's exit table "0" first, then the rest alphabetically, "other" last.
func exitRows(iface *skillgraph.Interface) []exitRow {
	if iface == nil {
		return nil
	}

	codes := slices.SortedFunc(maps.Keys(iface.Result.Exit), compareExitCodes)
	rows := make([]exitRow, 0, len(codes))
	for _, code := range codes {
		rows = append(rows, exitRow{Code: code, Meaning: iface.Result.Exit[code]})
	}
	return rows
}

// gateVerdicts runs the installed gate's check over targets from repoDir and maps each target to its verdict line.
func gateVerdicts(ctx context.Context, home, repoDir, leaf string, targets []string) (map[string]string, error) {
	hooksDir := filepath.Join(home, ".claude", "hooks")
	gateCtx, cancel := context.WithTimeout(ctx, gateTimeout)
	defer cancel()

	args := append([]string{"check", "--skill", leaf}, targets...)
	output, err := shell.Run(gateCtx, repoDir, filepath.Join(hooksDir, "bin", "toolpolicy"), args...)
	if err != nil {
		return nil, fmt.Errorf("gateVerdicts: %w", err)
	}

	verdicts := map[string]string{}
	for line := range strings.SplitSeq(output, "\n") {
		match := gateVerdictPattern.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if match != nil {
			verdicts[match[1]] = match[2]
		}
	}
	return verdicts, nil
}

// installRow is the record row a SKILL_ or PROVIDER_ form key writes, "" for any other key.
func installRow(rows *skills.Rows, key string) string {
	switch {
	case strings.HasPrefix(key, variablePrefix):
		return rows.Skills[key]
	case strings.HasPrefix(key, providerPrefix):
		return rows.Providers[key]
	}
	return ""
}

// isInstallChange reports whether a posted key other than a repo-scoped variable changed its record row, the one change a redeploy fills in.
func isInstallChange(before, after *skills.Rows, form url.Values, repoScoped map[string]bool) bool {
	for key := range form {
		isChanged := installRow(before, key) != installRow(after, key)
		if isChanged && !repoScoped[key] {
			return true
		}
	}
	return false
}

// newDefaultRow renders one deferred variable with its record default, every registered repo holding its own value and the selected repo's value.
func newDefaultRow(rows *skills.Rows, registered []repos.Repo, selected string, value *skills.VariableValue) defaultRow {
	row := defaultRow{Name: value.Name, Default: rows.Skills[value.Name], Interface: value.Interface, Required: value.Required}
	row.Global = skills.GlobalDefault(value.Interface, rows)
	for _, repo := range registered {
		override := repo.Env[value.Name]
		if repo.Name == selected {
			row.Override = override
		}
		if override != "" {
			row.Overrides = append(row.Overrides, repoOverride{Repo: repo.Name, Value: override})
		}
	}
	return row
}

// newProviderRow resolves the provider's program — PROVIDER_ environment, its record row, the catalog program when installed here — and probes it once.
func newProviderRow(ctx context.Context, rows *skills.Rows, isInstalled bool, prober skills.Prober, provider *skillgraph.Provider) *providerRow {
	key := skills.ProviderKey(provider.Id)
	row := &providerRow{Id: provider.Id, Override: rows.Providers[key], Source: skills.SourceUnset}
	isCatalogProgram := isInstalled && provider.IsProvided()
	switch {
	case os.Getenv(key) != "":
		row.Program, row.Source = os.Getenv(key), skills.SourceEnvironment
	case row.Override != "":
		row.Program, row.Source = row.Override, skills.SourceRecord
	case isCatalogProgram:
		row.Program, row.Source = provider.Program, sourceCatalog
		if provider.Component != "" {
			row.Source = string(provider.Component)
		}
	}
	if row.Program == "" {
		return row
	}

	claims, err := prober.Claims(ctx, provider)
	if err != nil {
		row.Cause = err.Error()
		return row
	}
	row.Claims = claims
	return row
}

// newVariableRow renders one resolved value with its record override, readers and exit table.
func newVariableRow(rows *skills.Rows, model *skillgraph.Skill, value *skills.VariableValue) variableRow {
	row := variableRow{
		Name:       value.Name,
		Cause:      value.Cause,
		Exit:       exitRows(value.Interface),
		Interface:  value.Interface,
		IsDeferred: value.IsDeferred,
		Override:   rows.Skills[value.Name],
		Readers:    skillgraph.VariableReaders(model, value.Name),
		Required:   value.Required,
		Source:     value.Source,
		Value:      value.Value,
	}
	return row
}
