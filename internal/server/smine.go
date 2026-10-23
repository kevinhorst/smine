package server

import (
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	// formItemSuffix follows the row key in the name of its add field: <key>.item.
	formItemSuffix = ".item"
	pageSmine      = "config-smine"
	queryComponent = "component"
	tmplSmine      = "smine.html"
	tmplSminePanel = "_smine_panel.html"
)

// settingRow is one setting with its schema and effective value; a list kind carries its items.
type settingRow struct {
	Applies      components.Applies
	Component    components.ID
	Current      string
	Default      string
	DefaultItems []string
	Help         string
	IsOverridden bool
	Items        []string
	Key          string
	Kind         components.Kind
	Label        string
	Sink         string
	Values       []string
}

func newSettingRow(record install.Record, id components.ID, setting *components.Setting) (settingRow, error) {
	segments := strings.Split(setting.Key, ".")
	row := settingRow{
		Applies:      setting.Applies,
		Component:    id,
		Current:      setting.Default,
		Default:      setting.Default,
		Help:         setting.Help,
		IsOverridden: install.IsOverridden(record, id, setting.Key),
		Key:          setting.Key,
		Kind:         setting.Kind,
		Label:        strings.ReplaceAll(segments[len(segments)-1], "_", " "),
		Sink:         string(setting.Sink.Kind) + "(" + setting.Sink.Target + ")",
		Values:       setting.Values,
	}
	if row.IsList() {
		defaultValue, err := components.ParseText(setting, setting.Default)
		if err != nil {
			return row, fmt.Errorf("newSettingRow: %w", err)
		}
		row.DefaultItems = defaultValue.List
		row.Items = defaultValue.List
	}
	if !row.IsOverridden {
		return row, nil
	}

	value, err := install.Resolve(record, id, setting.Key)
	if err != nil {
		return row, fmt.Errorf("newSettingRow: %w", err)
	}
	row.Current = value.String()
	row.Items = value.List
	return row, nil
}

// IsList reports a list kind, edited item by item.
func (r settingRow) IsList() bool {
	return components.IsListKind(r.Kind)
}

// sminePage is the Smine tab: one tile per component with settings, the active tile's settings in the panel.
type sminePage struct {
	Page  string
	Panel sminePanel
	Tiles []smineTile
	Title string
}

// sminePanel is the active component's settings with the last Save's apply report.
type sminePanel struct {
	Component  components.Component
	IsSelected bool
	Report     string
	Rows       []settingRow
}

// smineTile is one component in the selector grid.
type smineTile struct {
	Id         components.ID
	IsActive   bool // the tile whose settings the panel shows
	IsSelected bool // the component is on in the record
	Overridden int
	Settings   int
	Title      string
}

// applySettings stores the form's rows under the sync lock, applies the component and answers with the panel; a restart-class apply restarts the server after the answer — the one write path, list edits included.
func (s *Server) applySettings(w http.ResponseWriter, r *http.Request, id components.ID, form map[string][]string) {
	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	report, isRestartDue, err := s.saveComponentSettings(r, id, form)
	s.repoLocks.Release(syncLockKey)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	s.renderSminePanel(w, id, report)
	if !isRestartDue {
		return
	}

	if err := restartServerAgent(s.repoDir); err != nil {
		log.Printf("Server.applySettings: %v", err)
	}
}

// handleSminePage renders the tiles and the panel of ?component=, else of the first component with settings.
func (s *Server) handleSminePage(w http.ResponseWriter, r *http.Request) {
	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.handleSminePage: %v", err)
	}

	page := sminePage{Page: pageSmine, Title: "Smine"}
	page.Tiles, err = s.smineTiles(record)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if len(page.Tiles) == 0 {
		s.renderFragment(w, tmplSmine, page)
		return
	}

	active := components.ID(r.URL.Query().Get(queryComponent))
	index := slices.IndexFunc(page.Tiles, func(tile smineTile) bool { return tile.IsSelected && tile.Id == active })
	if index < 0 {
		index = max(slices.IndexFunc(page.Tiles, func(tile smineTile) bool { return tile.IsSelected }), 0)
	}
	page.Tiles[index].IsActive = true

	page.Panel, err = s.sminePanel(record, page.Tiles[index].Id, "")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplSmine, page)
}

// handleSmineSave stores every posted row of one component; an empty field resets its row.
func (s *Server) handleSmineSave(w http.ResponseWriter, r *http.Request) {
	id, ok := hookCardComponent(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	s.applySettings(w, r, id, r.PostForm)
}

// renderSminePanel re-renders one component's panel with the report under its title; an unreadable record joins the report.
func (s *Server) renderSminePanel(w http.ResponseWriter, id components.ID, report string) {
	record, err := s.records.load(s.machine)
	if err != nil {
		report += "FAILED: " + err.Error() + "\n"
	}

	panel, err := s.sminePanel(record, id, report)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplSminePanel, &panel)
}

// saveComponentSettings stores every row of form in a copy of the record and applies the component; a refused value is the error, naming the row's help.
func (s *Server) saveComponentSettings(r *http.Request, id components.ID, form map[string][]string) (string, bool, error) {
	stored, err := s.records.load(s.machine)
	if err != nil {
		return "", false, fmt.Errorf("Server.saveComponentSettings: %w", err)
	}

	record := cloneRecordSettings(stored)

	rows, err := install.Schema(s.machine, id)
	if err != nil {
		return "", false, fmt.Errorf("Server.saveComponentSettings: %w", err)
	}

	var restartKeys []string
	for _, row := range rows {
		texts, isPosted := form[row.Key]
		if pending := pendingItem(form, &row); pending != "" {
			// A Save counts a typed, never-added list item: the Add button is not the only way in.
			text, err := withItem(record, id, &row, texts, pending)
			if err != nil {
				return "", false, fmt.Errorf("Server.saveComponentSettings: %w", err)
			}
			texts, isPosted = []string{text}, true
		}
		if !isPosted {
			continue
		}

		before := install.IsOverridden(record, id, row.Key)
		if err := storeOverride(&record, id, &row, texts[len(texts)-1]); err != nil {
			return "", false, fmt.Errorf("%w — %s", err, row.Help)
		}

		isRestartRow := row.Applies == components.AppliesRestart
		isTouched := before || install.IsOverridden(record, id, row.Key)
		if isRestartRow && isTouched {
			restartKeys = append(restartKeys, row.Key)
		}
	}

	results, err := install.ApplyComponent(r.Context(), s.machine, record, id)
	report := panelSummary(results)
	if err != nil {
		report += "FAILED: " + err.Error() + "\n"
	}
	if values, valuesErr := install.InstallValues(record); valuesErr == nil {
		s.trash.SetRetention(values.TrashRetentionDays)
	}
	if len(restartKeys) > 0 {
		report += "restart needed: " + strings.Join(restartKeys, ", ") + " — the changed values reach their consumer after a restart; the apply restarts it when its rendered config changed\n"
	}

	isRestartDue := slices.ContainsFunc(results, isRestartResult)
	if isRestartDue {
		report += "the config server restarts in a moment — reload the page\n"
	}
	return report, isRestartDue, nil
}

// sminePanel builds one component's rows from its schema.
func (s *Server) sminePanel(record install.Record, id components.ID, report string) (sminePanel, error) {
	var panel sminePanel
	component, ok := components.Lookup(id)
	if !ok {
		return panel, fmt.Errorf("Server.sminePanel: Unknown component %q", id)
	}

	panel.Component = component
	panel.IsSelected = slices.Contains(record.Components, id)
	panel.Report = report
	schema, err := install.Schema(s.machine, id)
	if err != nil {
		return panel, fmt.Errorf("Server.sminePanel: %w", err)
	}
	for _, setting := range pageSettings(schema) {
		row, err := newSettingRow(record, id, &setting)
		if err != nil {
			return panel, fmt.Errorf("Server.sminePanel: %w", err)
		}
		panel.Rows = append(panel.Rows, row)
	}
	return panel, nil
}

// smineTiles lists one tile per catalog component with page settings, in catalog order.
func (s *Server) smineTiles(record install.Record) ([]smineTile, error) {
	var tiles []smineTile
	for _, component := range components.Catalog() {
		full, err := install.Schema(s.machine, component.ID)
		if err != nil {
			return nil, fmt.Errorf("Server.smineTiles: %w", err)
		}
		schema := pageSettings(full)
		if len(schema) == 0 {
			continue
		}

		tile := smineTile{
			Id:         component.ID,
			IsSelected: slices.Contains(record.Components, component.ID),
			Overridden: overriddenCount(record, component.ID, schema),
			Settings:   len(schema),
			Title:      component.Title,
		}
		tiles = append(tiles, tile)
	}
	return tiles, nil
}

// cloneRecordSettings copies the overrides so a refused Save never mutates the cached record.
func cloneRecordSettings(record install.Record) install.Record {
	if record.Settings == nil {
		return record
	}

	settings := make(install.Settings, len(record.Settings))
	for id, values := range record.Settings {
		settings[id] = maps.Clone(values)
	}
	record.Settings = settings
	return record
}

// panelSummary words each result as what the Save did — the installer's action words (install, unchanged) read as a rebuild or a lost save.
func panelSummary(results []install.Result) string {
	var summary strings.Builder
	for _, result := range results {
		summary.WriteString(string(result.Component) + ": ")
		switch {
		case result.Err != nil:
			summary.WriteString("FAILED: " + result.Err.Error())
		case result.Action == install.ActionUnchanged:
			summary.WriteString("saved, nothing to apply — every value was already in effect")
		case result.Action == install.ActionRemove:
			summary.WriteString("removed")
		case slices.ContainsFunc(result.Steps, install.IsDaemonRestart):
			summary.WriteString("changes applied: daemon restarted")
		default:
			summary.WriteString("changes applied")
		}
		summary.WriteString("\n")
	}
	return summary.String()
}

// pendingItem is the text in a list row's add field, trimmed; empty for a scalar row or an untouched field.
func pendingItem(form map[string][]string, row *components.Setting) string {
	if !components.IsListKind(row.Kind) {
		return ""
	}
	texts := form[row.Key+formItemSuffix]
	if len(texts) == 0 {
		return ""
	}
	return strings.TrimSpace(texts[len(texts)-1])
}

// withItem is the row's list with item appended, as the text storeOverride parses: the posted list when the form carries one, else the record's.
func withItem(record install.Record, id components.ID, row *components.Setting, posted []string, item string) (string, error) {
	var current components.Value
	var err error
	if len(posted) > 0 {
		current, err = components.ParseText(row, posted[len(posted)-1])
	} else {
		current, err = install.Resolve(record, id, row.Key)
	}
	if err != nil {
		return "", fmt.Errorf("withItem: %w", err)
	}
	edited := components.Value{Kind: row.Kind, List: append(slices.Clone(current.List), item)}
	return edited.String(), nil
}

// pageSettings is the schema without its hidden rows, the ones the Smine page leaves to the CLI.
func pageSettings(schema []components.Setting) []components.Setting {
	isHidden := func(setting components.Setting) bool { return setting.IsHidden }
	return slices.DeleteFunc(slices.Clone(schema), isHidden)
}

// overriddenCount is the number of schema rows the record overrides for the component.
func overriddenCount(record install.Record, id components.ID, schema []components.Setting) int {
	count := 0
	for _, setting := range schema {
		if install.IsOverridden(record, id, setting.Key) {
			count++
		}
	}
	return count
}

// storeOverride parses one posted value into the record; empty, or equal to the row's default (a group row's from its fragment), clears the override.
func storeOverride(record *install.Record, id components.ID, row *components.Setting, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		install.ClearOverride(record, id, row.Key)
		return nil
	}

	value, err := components.ParseText(row, text)
	if err != nil {
		return fmt.Errorf("storeOverride: %w", err)
	}

	defaultValue, err := components.ParseText(row, row.Default)
	if err != nil {
		return fmt.Errorf("storeOverride: %w", err)
	}
	if value.String() == defaultValue.String() {
		install.ClearOverride(record, id, row.Key)
		return nil
	}

	if err := install.SetOverride(record, id, row.Key, value); err != nil {
		return fmt.Errorf("storeOverride: %w", err)
	}
	return nil
}
