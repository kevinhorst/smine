package server

import (
	"bytes"
	"encoding/json"
	"errors"
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
)

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
	// Entries is always nil; it exists so the shared _config_tabs template
	// can test {{if .Entries}} on both the Active and Docs page data.
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
	Explanation string
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

// configItem pairs a display value with its index in the stored array so
// display order can be alphabetical while delete URLs stay positional (D7).
type configItem struct {
	Index int
	Value string
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

	fragment, err := config.LoadFragment(s.claudeFragmentPath)
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
		row := rowFromEntry(entry)
		state, raw, ok := settingsValue(disabled.Doc(), main.Doc(), path)
		if ok {
			row.State = state
			row.Value = displayJSON(entry.Type, raw)
			if entry.Type == catalog.TypeArray {
				row.Items = jsonArrayItems(raw)
			}
		}
		if entry.Type == catalog.TypeArray {
			row.Presets = s.arrayPresets(entry.Key, row.Items)
			row.PathArray = pathArrayKeys[entry.Key]
		}
		row.Overridden = claudeOverridden(main.Doc(), fragment.Doc(), path)
		if row.Overridden {
			if fragRaw, ok := fragment.Doc().Get(path); ok {
				row.RepoValue = displayJSON(entry.Type, fragRaw)
			}
			blockType := entry.Type == catalog.TypeArray || entry.Type == catalog.TypeObject || entry.Type == catalog.TypeTable
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

// docURLRe matches the canonical per-key documentation deep link embedded in a
// catalog explanation (a trailing "See https://code.claude.com/docs/...").
var docURLRe = regexp.MustCompile(`https://code\.claude\.com/docs/\S+`)

// docHref returns the per-key documentation URL: the code.claude.com deep link
// from the explanation when present, else the generic schema source. Keys with
// no embedded URL (all Codex keys, ~58 Claude keys) keep their .Source.
func docHref(explanation, source string) string {
	if match := docURLRe.FindString(explanation); match != "" {
		return strings.TrimRight(match, ".,;)")
	}
	return source
}

// claudeOverridden reports whether the live value at path differs from the
// repo fragment's — including one side missing. Equality is canonical JSON:
// key order in objects is irrelevant, array order matters (H1).
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

// canonicalJSON normalizes a decoded value for drift comparison: arrays
// compare as multisets — the toggles re-append entries on enable, so
// storage order is presentation noise, not drift.
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

	fragment, err := codex.Load(s.codexFragmentPath)
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

	byCategory := make(map[string][]configRow)
	availableByCategory := make(map[string][]configRow)
	for _, row := range rows {
		if row.State == stateUnset {
			availableByCategory[row.Category] = append(availableByCategory[row.Category], row)
			continue
		}
		byCategory[row.Category] = append(byCategory[row.Category], row)
	}

	data := configPageData{Open: r.URL.Query().Get("open"), Page: "config-" + target, Target: target, Title: target + " config"}
	_, fragment := s.syncPaths(target)
	data.FragmentPath = fragment
	if _, err := os.Stat(fragment); err != nil {
		data.FragmentMissing = true
	}
	data.Groups = sortedGroups(byCategory)
	data.Available = sortedGroups(availableByCategory)
	for _, group := range data.Available {
		data.AvailableCount += len(group.Rows)
		data.AvailableOverridden += group.Overridden
	}
	s.renderFragment(w, tmplConfig, data)
}

// sortedGroups orders categories alphabetically with undocumented last and
// each group's rows by key.
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
	http.Redirect(w, r, "/config/claude", http.StatusFound)
}

func (s *Server) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	value := r.FormValue("value")
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return
	}

	// Catalog lookup: typed validation for documented keys, strict
	// structured parse for everything else (D8)
	entry := s.catalogEntry(target, key)
	if entry != nil {
		if err := catalog.Validate(entry, value); err != nil {
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
	// Only mutating handlers render single rows — the trigger re-pulls
	// #config-body so group badges and row placement stay fresh (D3).
	w.Header().Set("HX-Trigger", "config-op")
	rows, err := s.targetRows(target)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	for _, row := range rows {
		if row.Key == key {
			s.renderFragment(w, tmplConfigRow, row)
			return
		}
	}

	// Undocumented key no longer present anywhere: render a bare unset row.
	row := configRow{Key: key, State: stateUnset, Target: target}
	s.renderFragment(w, tmplConfigRow, row)
}

func (s *Server) setClaudeValue(entry *catalog.Entry, key, value string, w http.ResponseWriter) bool {
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	// String/enum input is JSON-quoted; bool/number/array/object and
	// undocumented keys must be valid JSON as submitted (D8)
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

	if err := config.Save(s.settingsPath, settings); err != nil {
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

	// String/enum input becomes a quoted TOML string; everything else
	// must already be a valid TOML literal (D8)
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

	if err := config.Save(s.settingsPath, settings); err != nil {
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

// jsonArrayItems renders each array element for the list editor: strings
// bare, everything else as compact JSON. Items are display-sorted but keep
// their storage index for the delete URL (D7).
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

func sortConfigItems(items []configItem) {
	slices.SortFunc(items, func(left, right configItem) int {
		return strings.Compare(left.Value, right.Value)
	})
}

// arrayPresets offers enumerable item values for MCP-server-name keys,
// excluding items already present (D6).
func (s *Server) arrayPresets(key string, items []configItem) []string {
	if key != "disabledMcpjsonServers" && key != "enabledMcpjsonServers" {
		return nil
	}

	known, err := config.McpServerNames(s.claudeJsonPath)
	if err != nil {
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

// codexArrayItems lists a TOML array literal's item literals; non-arrays
// yield nil (row falls back to display-only). Display-sorted with storage
// indexes kept, like jsonArrayItems (D7).
func codexArrayItems(literal string) []configItem {
	value, err := parser.ParseValue(literal)
	if err != nil {
		return nil
	}
	array, ok := value.X.(parser.Array)
	if !ok {
		return nil
	}

	// Index counts parser.Value elements only — mutateCodexItems rebuilds
	// the array from exactly those, so delete positions must match.
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

// displayJSON renders a raw JSON value for the row's control: strings and
// enums show the bare string, structured values show indented JSON.
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

// settingsValue resolves a key path against the live settings first, then the
// disabled file.
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
			// Fragment-only pass: the key exists nowhere live, so there is
			// no live value to show — only the drift marker (D4).
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
