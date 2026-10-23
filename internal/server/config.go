package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/creachadair/tomledit/parser"
	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skills"
)

// sectionOwnedKeys render in a top section of the page and never as a generic row (D6).
var sectionOwnedKeys = map[string]bool{
	"hooks":             true,
	"permissions":       true,
	"permissions.allow": true,
	"permissions.ask":   true,
	"permissions.deny":  true,
}

// keySkillOverrides is the one map key with a preset list.
const keySkillOverrides = "skillOverrides"

const (
	tmplConfig    = "config.html"
	tmplConfigRow = "_config_row.html"
)

const (
	categoryUndocumented = "undocumented"

	stateDisabled = "disabled"
	stateSet      = "set"
	stateUnset    = "unset"
)

type configGroup struct {
	Category   string
	Overridden int
	Rows       []configRow
}

type configPageData struct {
	// Entries is always nil; the shared _config_tabs template tests {{if .Entries}} on both pages.
	Entries             []catalog.Entry
	Available           []configGroup
	AvailableCount      int
	AvailableOverridden int
	FragmentMissing     bool
	FragmentPath        string
	Groups              []configGroup
	Open                string
	Page                string
	Target              string
	Title               string
}

type configRow struct {
	CanToggle   bool
	Category    string
	Diff        []diffLine
	Documented  bool
	Entries     []configEntry
	Explanation string
	GroupBadges []groupBadge
	IsTable     bool
	Items       []configItem
	Key         string
	Overridden  bool
	PathArray   bool
	Presets     []string
	RepoValue   string
	Source      string
	State       string // stateSet | stateUnset | stateDisabled
	Subtables   []string
	Target      string
	Type        string
	Value       string
	Values      []string
}

// configEntry is one named member of a map key, shown as a select.
type configEntry struct {
	Name  string
	Value string
}

// configItem keeps the storage index so display can sort while delete URLs stay positional.
type configItem struct {
	Index int
	Value string
}

// groupBadge is one group summary's overridden count, swapped out of band by a row answer.
type groupBadge struct {
	Id    string
	Count int
}

func (s *Server) catalogEntry(target, key string) *catalog.Entry {
	for index := range s.catalog {
		if s.catalog[index].Target == target && s.catalog[index].Key == key {
			return &s.catalog[index]
		}
	}
	return nil
}

func (s *Server) claudeRows() ([]configRow, error) {
	main, disabled, err := s.loadBoth()
	if err != nil {
		return nil, err
	}

	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return nil, err
	}

	models, err := main.AvailableModels()
	if err != nil {
		return nil, err
	}

	var rows []configRow
	covered := make(map[string]bool)
	for index := range s.catalog {
		entry := &s.catalog[index]
		if entry.Target != catalog.TargetClaude {
			continue
		}

		if strings.Contains(entry.Key, "<") {
			continue
		}

		path := strings.Split(entry.Key, ".")
		covered[path[0]] = true
		if sectionOwnedKeys[entry.Key] {
			continue
		}

		row := rowFromEntry(entry)
		state, raw, ok := settingsValue(disabled.Doc(), main.Doc(), path)
		if ok {
			row.State = state
			row.Value = displayJSON(entry.Type, raw)
			if entry.Type == catalog.TypeArray {
				row.Items = jsonArrayItems(raw)
			}
			if entry.Type == catalog.TypeMap {
				row.Entries = jsonMapEntries(raw)
			}
		}
		if entry.Type == catalog.TypeArray {
			row.Presets = s.arrayPresets(entry.Key, row.Items, models)
			row.PathArray = pathArrayKeys[entry.Key]
		}
		if entry.Type == catalog.TypeMap {
			row.Presets = s.mapPresets(entry.Key, row.Entries)
		}
		if entry.Type == catalog.TypeString && slices.Contains(modelSettingKeys, entry.Key) {
			row.Values = models
		}
		row.Overridden = claudeOverridden(main.Doc(), fragment.Doc(), path)
		if row.Overridden {
			if fragRaw, ok := fragment.Doc().Get(path); ok {
				row.RepoValue = displayJSON(entry.Type, fragRaw)
			}
			blockType := isBlockType(entry.Type)
			if blockType && row.RepoValue != "" && row.Value != "" {
				row.Diff = compactDiff(diffLines(row.RepoValue, row.Value), 2)
			}
		}
		rows = append(rows, row)
	}

	rows = append(rows, undocumentedClaudeRows(covered, main.Doc(), fragment.Doc(), stateSet)...)
	rows = append(rows, undocumentedClaudeRows(covered, disabled.Doc(), fragment.Doc(), stateDisabled)...)
	rows = append(rows, undocumentedClaudeRows(covered, fragment.Doc(), fragment.Doc(), stateUnset)...)
	return rows, nil
}

var docURLRe = regexp.MustCompile(`https://code\.claude\.com/docs/\S+`)

// docHref falls back to .Source: all Codex keys and some Claude keys embed no deep link.
func docHref(explanation, source string) string {
	if match := docURLRe.FindString(explanation); match != "" {
		return strings.TrimRight(match, ".,;)")
	}
	return source
}

// claudeOverridden counts one side missing as a difference.
func claudeOverridden(mainDoc, fragmentDoc *config.Document, path []string) bool {
	liveRaw, liveOk := mainDoc.Get(path)
	fragRaw, fragOk := fragmentDoc.Get(path)
	if !liveOk && !fragOk {
		return false
	}
	if liveOk != fragOk {
		return true
	}

	var live, frag any
	if json.Unmarshal(liveRaw, &live) != nil || json.Unmarshal(fragRaw, &frag) != nil {
		return !bytes.Equal(liveRaw, fragRaw)
	}
	return !reflect.DeepEqual(canonicalJSON(live), canonicalJSON(frag))
}

// canonicalJSON compares arrays as multisets: the toggles re-append entries on enable, so order is not drift.
func canonicalJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, element := range typed {
			typed[key] = canonicalJSON(element)
		}
	case []any:
		for index, element := range typed {
			typed[index] = canonicalJSON(element)
		}
		slices.SortFunc(typed, func(left, right any) int {
			leftJSON, _ := json.Marshal(left)
			rightJSON, _ := json.Marshal(right)
			return bytes.Compare(leftJSON, rightJSON)
		})
	}
	return value
}

func (s *Server) codexFileRow(entry *codex.Entry, fragLiterals map[string]string, state string) configRow {
	row := configRow{
		CanToggle:  true,
		Category:   categoryUndocumented,
		IsTable:    entry.Value == "",
		Key:        entry.Key,
		Overridden: entry.Value != fragLiterals[entry.Key],
		State:      state,
		Subtables:  entry.Subtables,
		Target:     catalog.TargetCodex,
		Value:      entry.Value,
	}
	catalogEntry := s.catalogEntry(catalog.TargetCodex, entry.Key)
	if catalogEntry == nil {
		return row
	}

	row.Category = catalogEntry.Category
	row.Documented = true
	row.Explanation = catalogEntry.Explanation
	row.Source = catalogEntry.Source
	row.Type = catalogEntry.Type
	row.Value = codexDisplayValue(catalogEntry, entry.Value)
	row.Values = catalogEntry.Values
	if catalogEntry.Type == catalog.TypeArray {
		row.Items = codexArrayItems(entry.Value)
	}
	return row
}

func (s *Server) codexFileRows(cfg *codex.Config, seen map[string]bool, fragLiterals map[string]string, state string) []configRow {
	var rows []configRow
	for _, entry := range cfg.Entries() {
		if seen[entry.Key] {
			continue
		}

		seen[entry.Key] = true
		rows = append(rows, s.codexFileRow(&entry, fragLiterals, state))
	}
	return rows
}

func (s *Server) codexRows() ([]configRow, error) {
	main, err := codex.Load(s.codexPath)
	if err != nil {
		return nil, err
	}

	disabled, err := codex.Load(codex.DisabledPath(s.codexPath))
	if err != nil {
		return nil, err
	}

	fragmentData, err := os.ReadFile(s.codexFragmentPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("codexRows: Failed to read %s: %w", s.codexFragmentPath, err)
	}
	fragment, err := codex.Parse(s.markers().Expand(fragmentData))
	if err != nil {
		return nil, err
	}

	fragLiterals := make(map[string]string)
	for _, entry := range fragment.Entries() {
		fragLiterals[entry.Key] = entry.Value
	}

	seen := make(map[string]bool)
	rows := s.codexFileRows(main, seen, fragLiterals, stateSet)
	rows = append(rows, s.codexFileRows(disabled, seen, fragLiterals, stateDisabled)...)

	for index := range s.catalog {
		entry := &s.catalog[index]
		if entry.Target != catalog.TargetCodex {
			continue
		}

		if strings.Contains(entry.Key, "<") {
			continue
		}

		if seen[entry.Key] {
			continue
		}

		seen[entry.Key] = true
		row := rowFromEntry(entry)
		_, row.Overridden = fragLiterals[entry.Key]
		rows = append(rows, row)
	}

	// Fragment-only keys: exist nowhere live — drift marker only (D4).
	for _, entry := range fragment.Entries() {
		if seen[entry.Key] {
			continue
		}

		seen[entry.Key] = true
		row := configRow{
			Category:   categoryUndocumented,
			Key:        entry.Key,
			Overridden: true,
			State:      stateUnset,
			Target:     catalog.TargetCodex,
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Server) handleConfigCodexToggle(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := codex.Toggle(s.codexPath, key); err != nil {
		if errors.Is(err, codex.ErrNotFound) {
			respond.WithNotFound(err.Error(), w)
			return
		}

		respond.WithInternalServerError(err, w)
		return
	}

	s.renderConfigRow(w, catalog.TargetCodex, key)
}

func (s *Server) handleConfigPage(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	if !catalog.IsTarget(target) {
		http.NotFound(w, r)
		return
	}

	rows, err := s.targetRows(target)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := configPageData{Open: r.URL.Query().Get("open"), Page: "config-" + target, Target: target, Title: target + " config"}
	fragment := s.claudeFragmentPath
	if target == catalog.TargetCodex {
		fragment = s.codexFragmentPath
	}
	data.FragmentPath = fragment
	if _, err := os.Stat(fragment); err != nil {
		data.FragmentMissing = true
	}
	data.Groups, data.Available = groupedRows(rows)
	for _, group := range data.Available {
		data.AvailableCount += len(group.Rows)
		data.AvailableOverridden += group.Overridden
	}
	s.renderFragment(w, tmplConfig, data)
}

// groupBadges is one badge per category group plus the available section's, in the ids config.html renders.
func groupBadges(target string, groups, available []configGroup) []groupBadge {
	badges := make([]groupBadge, 0, len(groups)+1)
	for _, group := range groups {
		badge := groupBadge{Id: "ovr-" + domID(target, group.Category), Count: group.Overridden}
		badges = append(badges, badge)
	}
	availableOverridden := 0
	for _, group := range available {
		availableOverridden += group.Overridden
	}
	availableBadge := groupBadge{Id: "ovr-available-" + target, Count: availableOverridden}
	return append(badges, availableBadge)
}

// groupedRows splits the rows into the category groups (set rows) and the available groups (unset rows).
func groupedRows(rows []configRow) ([]configGroup, []configGroup) {
	byCategory := make(map[string][]configRow)
	availableByCategory := make(map[string][]configRow)
	for _, row := range rows {
		if row.State == stateUnset {
			availableByCategory[row.Category] = append(availableByCategory[row.Category], row)
			continue
		}
		byCategory[row.Category] = append(byCategory[row.Category], row)
	}
	return sortedGroups(byCategory), sortedGroups(availableByCategory)
}

func sortedGroups(byCategory map[string][]configRow) []configGroup {
	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		if category != categoryUndocumented {
			categories = append(categories, category)
		}
	}
	slices.Sort(categories)
	if _, ok := byCategory[categoryUndocumented]; ok {
		categories = append(categories, categoryUndocumented)
	}

	var groups []configGroup
	for _, category := range categories {
		group := configGroup{Category: category, Rows: byCategory[category]}
		slices.SortFunc(group.Rows, func(left, right configRow) int {
			return strings.Compare(left.Key, right.Key)
		})
		for _, row := range group.Rows {
			if row.Overridden {
				group.Overridden++
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func (s *Server) handleConfigRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/config/smine", http.StatusFound)
}

func (s *Server) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	value := r.FormValue("value")
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return
	}

	// Catalog lookup: typed validation for documented keys, strict structured parse for the rest
	entry := s.catalogEntry(target, key)
	if entry != nil {
		if err := catalog.Validate(entry, value); err != nil {
			respond.WithBadRequest(err.Error(), w)
			return
		}
	}

	if target == catalog.TargetClaude && slices.Contains(modelSettingKeys, key) {
		chosen := []string{value}
		if key == keyFallbackModel && json.Unmarshal([]byte(value), &chosen) != nil {
			respond.WithBadRequest("fallbackModel must be a JSON array of model IDs", w)
			return
		}
		if err := s.requireAvailableModels(chosen); err != nil {
			respond.WithBadRequest(err.Error(), w)
			return
		}
	}

	if target == catalog.TargetCodex && !s.setCodexValue(entry, key, value, w) {
		return
	}
	if target == catalog.TargetClaude && !s.setClaudeValue(entry, key, value, w) {
		return
	}

	s.renderConfigRow(w, target, key)
}

func (s *Server) handleConfigUnset(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return
	}

	if target == catalog.TargetCodex && !s.unsetCodexValue(key, w) {
		return
	}
	if target == catalog.TargetClaude && !s.unsetClaudeValue(key, w) {
		return
	}

	s.renderConfigRow(w, target, key)
}

func (s *Server) renderConfigRow(w http.ResponseWriter, target, key string) {
	rows, err := s.targetRows(target)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	groups, available := groupedRows(rows)
	badges := groupBadges(target, groups, available)
	for _, row := range rows {
		if row.Key == key {
			row.GroupBadges = badges
			s.renderFragment(w, tmplConfigRow, row)
			return
		}
	}

	// Undocumented key no longer present anywhere: render a bare unset row.
	row := configRow{GroupBadges: badges, Key: key, State: stateUnset, Target: target}
	s.renderFragment(w, tmplConfigRow, row)
}

func (s *Server) setClaudeValue(entry *catalog.Entry, key, value string, w http.ResponseWriter) bool {
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	// String/enum input is JSON-quoted; everything else must be valid JSON as submitted
	raw := json.RawMessage(value)
	if entry != nil && catalog.IsTextualType(entry.Type) {
		quoted, _ := json.Marshal(value)
		raw = quoted
	}
	if !json.Valid(raw) {
		respond.WithBadRequest("value is not valid JSON", w)
		return false
	}

	if err := settings.Doc().Set(strings.Split(key, "."), raw); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return false
	}

	if err := config.Save(s.settingsPath, settings, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	return true
}

func (s *Server) setCodexValue(entry *catalog.Entry, key, value string, w http.ResponseWriter) bool {
	cfg, err := codex.Load(s.codexPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	// String/enum input becomes a quoted TOML string; everything else must be a valid TOML literal
	literal := value
	if entry != nil && catalog.IsTextualType(entry.Type) {
		literal = strconv.Quote(value)
	}
	if err := cfg.Set(key, literal); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return false
	}

	if err := codex.Save(s.codexPath, cfg); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	return true
}

func (s *Server) targetRows(target string) ([]configRow, error) {
	if target == catalog.TargetCodex {
		return s.codexRows()
	}
	return s.claudeRows()
}

func (s *Server) unsetClaudeValue(key string, w http.ResponseWriter) bool {
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	if !settings.Doc().Unset(strings.Split(key, ".")) {
		respond.WithNotFound("key not found: "+key, w)
		return false
	}

	if err := config.Save(s.settingsPath, settings, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	return true
}

func (s *Server) unsetCodexValue(key string, w http.ResponseWriter) bool {
	cfg, err := codex.Load(s.codexPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	if !cfg.Unset(key) {
		respond.WithNotFound("key not found: "+key, w)
		return false
	}

	if err := codex.Save(s.codexPath, cfg); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	return true
}

func jsonArrayItems(raw json.RawMessage) []configItem {
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return nil
	}

	items := make([]configItem, 0, len(elements))
	for index, element := range elements {
		var text string
		if err := json.Unmarshal(element, &text); err == nil {
			items = append(items, configItem{Index: index, Value: text})
			continue
		}
		items = append(items, configItem{Index: index, Value: string(element)})
	}
	sortConfigItems(items)
	return items
}

func jsonMapEntries(raw json.RawMessage) []configEntry {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil
	}

	entries := make([]configEntry, 0, len(members))
	for name, element := range members {
		var text string
		if err := json.Unmarshal(element, &text); err != nil {
			text = string(element)
		}
		entries = append(entries, configEntry{Name: name, Value: text})
	}
	slices.SortFunc(entries, func(left, right configEntry) int {
		return strings.Compare(left.Name, right.Name)
	})
	return entries
}

func sortConfigItems(items []configItem) {
	slices.SortFunc(items, func(left, right configItem) int {
		return strings.Compare(left.Value, right.Value)
	})
}

func (s *Server) arrayPresets(key string, items []configItem, models []string) []string {
	var known []string
	switch key {
	case "disabledMcpjsonServers", "enabledMcpjsonServers":
		names, err := config.McpServerNames(s.claudeJsonPath)
		if err != nil {
			return nil
		}
		known = names
	case keyFallbackModel:
		known = models
	default:
		return nil
	}

	var presets []string
	for _, name := range known {
		if !slices.ContainsFunc(items, func(item configItem) bool { return item.Value == name }) {
			presets = append(presets, name)
		}
	}
	return presets
}

// mapPresets lists the names a map key can still gain; only the skill overrides have a known name set.
func (s *Server) mapPresets(key string, entries []configEntry) []string {
	if key != keySkillOverrides {
		return nil
	}

	scanned, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return nil
	}

	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		present[entry.Name] = true
	}

	var presets []string
	for _, skill := range scanned {
		names := slices.Clone(skill.Aliases)
		names = append(names, skill.Name)
		for _, name := range names {
			if present[name] {
				continue
			}

			present[name] = true
			presets = append(presets, name)
		}
	}
	slices.Sort(presets)
	return presets
}

func codexArrayItems(literal string) []configItem {
	value, err := parser.ParseValue(literal)
	if err != nil {
		return nil
	}
	array, ok := value.X.(parser.Array)
	if !ok {
		return nil
	}

	// Index counts parser.Value elements only, matching how mutateCodexItems rebuilds the array.
	var items []configItem
	for _, element := range array {
		if item, ok := element.(parser.Value); ok {
			items = append(items, configItem{Index: len(items), Value: codexItemDisplay(item.String())})
		}
	}
	sortConfigItems(items)
	return items
}

// codexItemDisplay unquotes TOML string items for display.
func codexItemDisplay(literal string) string {
	if unquoted, err := strconv.Unquote(literal); err == nil {
		return unquoted
	}
	return literal
}

func codexDisplayValue(entry *catalog.Entry, value string) string {
	if entry != nil && catalog.IsTextualType(entry.Type) {
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
	}
	return value
}

// isBlockType names the types whose value renders as a block, so the repo value diffs line by line.
func isBlockType(configType string) bool {
	switch configType {
	case catalog.TypeArray, catalog.TypeMap, catalog.TypeObject, catalog.TypeTable:
		return true
	case catalog.TypeBool, catalog.TypeEnum, catalog.TypeNumber, catalog.TypeString:
		return false
	}
	return false
}

func displayJSON(configType string, raw json.RawMessage) string {
	if catalog.IsTextualType(configType) {
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return text
		}
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err == nil {
		return buf.String()
	}
	return string(raw)
}

func domID(target, key string) string {
	var builder strings.Builder
	builder.WriteString("row-")
	builder.WriteString(target)
	builder.WriteByte('-')
	for _, char := range key {
		if isDomIDChar(char) {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('-')
		}
	}
	return builder.String()
}

func isDomIDChar(char rune) bool {
	switch {
	case char >= 'a' && char <= 'z':
		return true
	case char >= 'A' && char <= 'Z':
		return true
	case char >= '0' && char <= '9':
		return true
	case char == '-' || char == '_':
		return true
	}
	return false
}

func rowFromEntry(entry *catalog.Entry) configRow {
	return configRow{
		Category:    entry.Category,
		Documented:  true,
		Explanation: entry.Explanation,
		IsTable:     entry.Type == catalog.TypeTable,
		Key:         entry.Key,
		Source:      entry.Source,
		State:       stateUnset,
		Target:      entry.Target,
		Type:        entry.Type,
		Values:      entry.Values,
	}
}

func settingsValue(disabledDoc, mainDoc *config.Document, path []string) (string, json.RawMessage, bool) {
	if raw, ok := mainDoc.Get(path); ok {
		return stateSet, raw, true
	}
	if raw, ok := disabledDoc.Get(path); ok {
		return stateDisabled, raw, true
	}
	return stateUnset, nil, false
}

func undocumentedClaudeRows(covered map[string]bool, doc, fragmentDoc *config.Document, state string) []configRow {
	var rows []configRow
	for _, key := range doc.Keys() {
		if covered[key] {
			continue
		}

		covered[key] = true
		row := configRow{
			Category: categoryUndocumented,
			Key:      key,
			State:    state,
			Target:   catalog.TargetClaude,
		}
		if state == stateUnset {
			// Fragment-only: no live value to show, only the drift marker.
			row.Overridden = true
			if raw, ok := fragmentDoc.Get([]string{key}); ok {
				row.RepoValue = displayJSON("", raw)
			}
		} else {
			raw, _ := doc.Get([]string{key})
			row.Value = displayJSON("", raw)
			row.Overridden = claudeOverridden(doc, fragmentDoc, []string{key})
			if row.Overridden {
				if fragRaw, ok := fragmentDoc.Get([]string{key}); ok {
					row.RepoValue = displayJSON("", fragRaw)
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}
