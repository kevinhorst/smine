package backlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	StatusDone         = "done"
	StatusImplementing = "implementing"
	StatusParked       = "parked"
	StatusPlanning     = "planning"
	StatusRaw          = "raw"
	StatusRejected     = "rejected"
	StatusReview       = "review"
	StatusShaped       = "shaped"
	StatusTesting      = "testing"

	TitleMaxRunes = 120

	dateLayout = "2006-01-02"
	idPrefix   = "B-"
	idWidth    = 3
)

// IceboxSection holds the rows nobody sequenced: unsolved problems and the nightly's oversized changes.
const IceboxSection = "Icebox"

var (
	ErrFormat         = errors.New("backlog: store format")
	ErrInvalidPri     = errors.New("backlog: invalid priority")
	ErrInvalidRoute   = errors.New("backlog: invalid route")
	ErrInvalidSection = errors.New("backlog: invalid section")
	ErrInvalidStatus  = errors.New("backlog: invalid status")
	ErrItemNotFound   = errors.New("backlog: item not found")

	Priorities = []string{"critical", "high", "medium", "low"}
	Sizes      = []string{"S", "M", "L", "XL"}
	// Statuses is in lifecycle order; done and rejected archive the item, PipelineStatuses are the stages between shaped and done.
	Statuses = []string{StatusRaw, StatusShaped, StatusPlanning, StatusImplementing, StatusReview, StatusTesting, StatusDone, StatusParked, StatusRejected}
	// ArchiveStatuses leave the active sections through MoveToArchive, never through SetStatus.
	ArchiveStatuses  = []string{StatusDone, StatusRejected}
	PipelineStatuses = []string{StatusPlanning, StatusImplementing, StatusReview, StatusTesting}

	idRe    = regexp.MustCompile(`^B-\d{3,}$`)
	skillRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

type Archive struct {
	Done     []ArchiveRow `json:"done"`
	Rejected []ArchiveRow `json:"rejected"`
}

type ArchiveRow struct {
	Id string `json:"id"`

	Commit      string `json:"commit,omitempty"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Link        string `json:"link"`
	Reason      string `json:"reason"` // resolution on done rows, reason on rejected rows
	Title       string `json:"title"`
}

type ChangelogRow struct {
	Date  string `json:"date"`
	Delta string `json:"delta"`
}

type Item struct {
	Id string `json:"id"`

	Added       string   `json:"added"`
	Deps        []string `json:"deps"`
	Description string   `json:"description"`
	Evidence    []string `json:"evidence"`
	Link        string   `json:"link"`
	Mined       *Mined   `json:"mined,omitempty"` // nil = not mined by the smine nightly
	Pri         string   `json:"pri"`
	Route       string   `json:"route"`
	Section     string   `json:"-"`
	Size        string   `json:"size"`
	Status      string   `json:"status"`
	Theme       string   `json:"theme"`
	Title       string   `json:"title"`
}

// `/fdesign change` → change; empty for bare, direct, experiment and unrouted items.
func (i *Item) RouteArgs() string {
	if !strings.HasPrefix(i.Route, "/") {
		return ""
	}
	_, args, _ := strings.Cut(strings.TrimPrefix(i.Route, "/"), " ")
	return strings.TrimSpace(args)
}

// `/fdesign change` → fdesign; empty for direct, experiment and unrouted items.
func (i *Item) RouteSkill() string {
	if !strings.HasPrefix(i.Route, "/") {
		return ""
	}
	leaf, _, _ := strings.Cut(strings.TrimPrefix(i.Route, "/"), " ")
	return leaf
}

// Id uniqueness and dep resolution are store-level, in Store.Validate.
func (i *Item) Validate() error {
	// Description
	if strings.TrimSpace(i.Description) == "" {
		return fmt.Errorf("Item.Validate: Item %s: Missing field description: %w", i.Id, ErrFormat)
	}

	// Title
	if strings.TrimSpace(i.Title) == "" {
		return fmt.Errorf("Item.Validate: Item %s: Missing field title: %w", i.Id, ErrFormat)
	}
	if utf8.RuneCountInString(i.Title) > TitleMaxRunes {
		return fmt.Errorf("Item.Validate: Item %s: Field title over %d runes: %w", i.Id, TitleMaxRunes, ErrFormat)
	}

	// Pri
	if !slices.Contains(Priorities, i.Pri) {
		return fmt.Errorf("Item.Validate: Item %s: Invalid field pri %q: %w", i.Id, i.Pri, ErrFormat)
	}

	// Size
	if !slices.Contains(Sizes, i.Size) {
		return fmt.Errorf("Item.Validate: Item %s: Invalid field size %q: %w", i.Id, i.Size, ErrFormat)
	}

	// Status
	if !slices.Contains(Statuses, i.Status) {
		return fmt.Errorf("Item.Validate: Item %s: Invalid field status %q: %w", i.Id, i.Status, ErrFormat)
	}
	return nil
}

type Mined struct {
	By       string   `json:"by,omitempty"` // session id of the run that mined the row; empty on rows mined before it was recorded
	Date     string   `json:"date"`
	Sessions []string `json:"sessions"`
}

type Section struct {
	Items []Item `json:"items"`
	Name  string `json:"name"`
}

type Store struct {
	Archive   Archive        `json:"archive"`
	Changelog []ChangelogRow `json:"changelog"`
	Sections  []Section      `json:"sections"`
	Updated   string         `json:"updated"`
}

// A missing file is the caller's os.ErrNotExist; a malformed one wraps ErrFormat.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Load: Failed to read %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	store := &Store{}
	if err := decoder.Decode(store); err != nil {
		return nil, fmt.Errorf("Load: Failed to decode %s: %w: %w", path, err, ErrFormat)
	}
	if err := store.Validate(); err != nil {
		return nil, fmt.Errorf("Load: %s: %w", path, err)
	}
	store.labelSections()
	return store, nil
}

func (s *Store) Active() int {
	return len(s.Items())
}

// Find returns the item and whether it exists; ids are exact.
func (s *Store) Find(id string) (Item, bool) {
	item := s.find(id)
	if item == nil {
		return Item{}, false
	}
	return *item, true
}

// Items flattens every section in store order.
func (s *Store) Items() []Item {
	var items []Item
	for _, section := range s.Sections {
		items = append(items, section.Items...)
	}
	return items
}

// Archived rows count too: a retired id is never reused.
func (s *Store) NextId() string {
	highest := 0
	for _, id := range s.ids() {
		number, _ := strconv.Atoi(strings.TrimPrefix(id, idPrefix))
		highest = max(highest, number)
	}
	return fmt.Sprintf("%s%0*d", idPrefix, idWidth, highest+1)
}

// SectionNames lists the epics in store order, the Icebox included.
func (s *Store) SectionNames() []string {
	names := make([]string, 0, len(s.Sections))
	for _, section := range s.Sections {
		names = append(names, section.Name)
	}
	return names
}

// Re-checks schema.json here: the server writes the file back and must not persist an unreadable shape.
func (s *Store) Validate() error {
	seen := map[string]bool{}
	for _, row := range slices.Concat(s.Archive.Done, s.Archive.Rejected) {
		if err := checkId(row.Id, seen); err != nil {
			return fmt.Errorf("Store.Validate: Archive: %w", err)
		}
	}
	names := map[string]bool{}
	for _, section := range s.Sections {
		if strings.TrimSpace(section.Name) == "" {
			return fmt.Errorf("Store.Validate: Section %q: Missing name: %w", section.Name, ErrFormat)
		}
		if names[section.Name] {
			return fmt.Errorf("Store.Validate: Section %s: Duplicate name: %w", section.Name, ErrFormat)
		}
		names[section.Name] = true
	}
	items := s.Items()
	for index := range items {
		if err := checkId(items[index].Id, seen); err != nil {
			return fmt.Errorf("Store.Validate: %w", err)
		}
		if err := items[index].Validate(); err != nil {
			return fmt.Errorf("Store.Validate: %w", err)
		}
	}
	for _, item := range items {
		if unknown := firstUnknownDep(item.Deps, seen); unknown != "" {
			return fmt.Errorf("Store.Validate: Item %s: Unknown dep %s: %w", item.Id, unknown, ErrFormat)
		}
	}
	return nil
}

func (s *Store) find(id string) *Item {
	for index := range s.Sections {
		items := s.Sections[index].Items
		position := slices.IndexFunc(items, func(item Item) bool { return item.Id == id })
		if position >= 0 {
			return &items[position]
		}
	}
	return nil
}

// findByEvidence returns the item whose evidence names pointer; nil when none does.
func (s *Store) findByEvidence(pointer string) *Item {
	for index := range s.Sections {
		items := s.Sections[index].Items
		position := slices.IndexFunc(items, func(item Item) bool { return slices.Contains(item.Evidence, pointer) })
		if position >= 0 {
			return &items[position]
		}
	}
	return nil
}

func (s *Store) ids() []string {
	var ids []string
	for _, item := range s.Items() {
		ids = append(ids, item.Id)
	}
	for _, row := range slices.Concat(s.Archive.Done, s.Archive.Rejected) {
		ids = append(ids, row.Id)
	}
	return ids
}

// labelSections stamps every item with its section name for the views.
func (s *Store) labelSections() {
	for index := range s.Sections {
		for itemIndex := range s.Sections[index].Items {
			s.Sections[index].Items[itemIndex].Section = s.Sections[index].Name
		}
	}
}

func (s *Store) log(delta string) {
	today := time.Now().Format(dateLayout)
	row := ChangelogRow{Date: today, Delta: delta}
	s.Changelog = append(s.Changelog, row)
	s.Updated = today
}

// section returns the named section, appended empty when the store lacks it.
func (s *Store) section(name string) *Section {
	position := s.sectionIndex(name)
	if position < 0 {
		s.Sections = append(s.Sections, Section{Items: []Item{}, Name: name})
		position = len(s.Sections) - 1
	}
	return &s.Sections[position]
}

func (s *Store) sectionIndex(name string) int {
	return slices.IndexFunc(s.Sections, func(section Section) bool { return section.Name == name })
}

// takeItem removes the item from its section and returns it with that section's name.
func (s *Store) takeItem(id string) (Item, string, bool) {
	for index := range s.Sections {
		items := s.Sections[index].Items
		position := slices.IndexFunc(items, func(item Item) bool { return item.Id == id })
		if position < 0 {
			continue
		}
		item := items[position]
		s.Sections[index].Items = slices.Delete(items, position, position+1)
		return item, s.Sections[index].Name, true
	}
	return Item{}, "", false
}

// AddSection inserts an empty epic before the Icebox; a blank, duplicate or Icebox name is refused.
func AddSection(name, path string) error {
	name = strings.TrimSpace(name)
	isReserved := name == "" || name == IceboxSection
	if isReserved {
		return fmt.Errorf("AddSection: Name %q: %w", name, ErrInvalidSection)
	}
	store, err := Load(path)
	if err != nil {
		return fmt.Errorf("AddSection: %w", err)
	}
	if store.sectionIndex(name) >= 0 {
		return fmt.Errorf("AddSection: Name %q: Duplicate: %w", name, ErrInvalidSection)
	}

	position := store.sectionIndex(IceboxSection)
	if position < 0 {
		position = len(store.Sections)
	}
	section := Section{Items: []Item{}, Name: name}
	store.Sections = slices.Insert(store.Sections, position, section)
	store.log("epic " + name + " added (board)")
	return Save(path, store)
}

// AppendIcebox files item under the Icebox section with the next id; an item whose first evidence pointer the store already holds is returned as is.
func AppendIcebox(item *Item, path string) (string, error) {
	store, err := Load(path)
	if err != nil {
		return "", fmt.Errorf("AppendIcebox: %w", err)
	}

	if existing := store.findByEvidence(item.Evidence[0]); existing != nil {
		return existing.Id, nil
	}

	item.Id = store.NextId()
	item.Section = IceboxSection
	if err := item.Validate(); err != nil {
		return "", fmt.Errorf("AppendIcebox: %w", err)
	}

	section := store.section(IceboxSection)
	section.Items = append(section.Items, *item)
	store.log(item.Id + " added by the nightly: " + item.Title)
	if err := Save(path, store); err != nil {
		return "", fmt.Errorf("AppendIcebox: %w", err)
	}
	return item.Id, nil
}

func AppendRejected(path string, row *ArchiveRow) error {
	store, err := Load(path)
	if err != nil {
		return fmt.Errorf("AppendRejected: %w", err)
	}

	row.Id = store.NextId()
	row.Date = time.Now().Format(dateLayout)
	store.Archive.Rejected = append(store.Archive.Rejected, *row)
	store.log(row.Id + " proposal " + row.Commit + " rejected (proposals): " + row.Reason)
	return Save(path, store)
}

// ClampTitle cuts a title over TitleMaxRunes to the cap with a trailing ellipsis; a title within the cap is returned as is.
func ClampTitle(title string) string {
	runes := []rune(title)
	if len(runes) <= TitleMaxRunes {
		return title
	}
	return string(runes[:TitleMaxRunes-1]) + "…"
}

// MoveItem files the item last under the named section; the item's own section is a no-op without a write.
func MoveItem(id, path, section string) error {
	store, err := Load(path)
	if err != nil {
		return fmt.Errorf("MoveItem: %w", err)
	}
	target := store.sectionIndex(section)
	if target < 0 {
		return fmt.Errorf("MoveItem: Section %q: %w", section, ErrInvalidSection)
	}
	current := store.find(id)
	if current == nil {
		return fmt.Errorf("MoveItem: Item %s: %w", id, ErrItemNotFound)
	}
	if current.Section == section {
		return nil
	}

	item, from, _ := store.takeItem(id)
	item.Section = section
	store.Sections[target].Items = append(store.Sections[target].Items, item)
	store.log(id + " moved " + from + " → " + section + " (board)")
	return Save(path, store)
}

// MoveToArchive files the item under done or rejected by status; resolution is the row's reason text.
func MoveToArchive(id, path, resolution, status string) error {
	if !slices.Contains(ArchiveStatuses, status) {
		return fmt.Errorf("MoveToArchive: Status %s: %w", status, ErrInvalidStatus)
	}
	store, err := Load(path)
	if err != nil {
		return fmt.Errorf("MoveToArchive: %w", err)
	}
	item, _, found := store.takeItem(id)
	if !found {
		return fmt.Errorf("MoveToArchive: Item %s: %w", id, ErrItemNotFound)
	}

	row := ArchiveRow{
		Id:          item.Id,
		Date:        time.Now().Format(dateLayout),
		Description: item.Description,
		Link:        item.Link,
		Reason:      resolution,
		Title:       item.Title,
	}
	if status == StatusRejected {
		store.Archive.Rejected = append(store.Archive.Rejected, row)
	} else {
		store.Archive.Done = append(store.Archive.Done, row)
	}
	store.log(id + " " + status + " → archive (board): " + resolution)
	return Save(path, store)
}

// Called on the run worktree's store copy.
func RecordRun(id, link, path, status string) error {
	if status != "" && !slices.Contains(PipelineStatuses, status) {
		return fmt.Errorf("RecordRun: Status %s: %w", status, ErrInvalidStatus)
	}
	err := edit(id, path, func(item *Item) string {
		delta := id + " link " + link
		isStage := status != "" && status != item.Status
		if isStage {
			delta += ", status " + item.Status + " → " + status
			item.Status = status
		}
		item.Link = link
		return delta + " (routine)"
	})
	if err != nil {
		return fmt.Errorf("RecordRun: %w", err)
	}
	return nil
}

// RemoveSection deletes an empty epic; the Icebox, an unknown name and a section holding items are refused.
func RemoveSection(name, path string) error {
	if name == IceboxSection {
		return fmt.Errorf("RemoveSection: Section %s: %w", name, ErrInvalidSection)
	}
	store, err := Load(path)
	if err != nil {
		return fmt.Errorf("RemoveSection: %w", err)
	}
	position := store.sectionIndex(name)
	if position < 0 {
		return fmt.Errorf("RemoveSection: Section %q: Unknown: %w", name, ErrInvalidSection)
	}
	if len(store.Sections[position].Items) > 0 {
		return fmt.Errorf("RemoveSection: Section %s: Not empty: %w", name, ErrInvalidSection)
	}

	store.Sections = slices.Delete(store.Sections, position, position+1)
	store.log("epic " + name + " removed (board)")
	return Save(path, store)
}

// Canonical encoding: two-space indent, struct key order, HTML escaping off, trailing newline.
func Save(path string, store *Store) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(store); err != nil {
		return fmt.Errorf("Save: Failed to encode %s: %w", path, err)
	}
	if err := fsx.WriteFileAtomic(buffer.Bytes(), path, 0o644); err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}

// SetPri rewrites the priority and logs the change; a value outside Priorities is refused.
func SetPri(id, path, pri string) error {
	if !slices.Contains(Priorities, pri) {
		return fmt.Errorf("SetPri: Priority %s: %w", pri, ErrInvalidPri)
	}
	err := edit(id, path, func(item *Item) string {
		previous := item.Pri
		item.Pri = pri
		return id + " pri " + previous + " → " + pri + " (board)"
	})
	if err != nil {
		return fmt.Errorf("SetPri: %w", err)
	}
	return nil
}

// SetRouteSkill routes the item to /<skill>; the old route's args go, the changelog row keeps the whole old route.
func SetRouteSkill(id, path, skill string) error {
	if !skillRe.MatchString(skill) {
		return fmt.Errorf("SetRouteSkill: Skill %q: %w", skill, ErrInvalidRoute)
	}
	err := edit(id, path, func(item *Item) string {
		previous := item.Route
		item.Route = "/" + skill
		return id + " route " + previous + " → " + item.Route + " (board)"
	})
	if err != nil {
		return fmt.Errorf("SetRouteSkill: %w", err)
	}
	return nil
}

// Re-reads the file so concurrent hand edits are never clobbered; done and rejected are MoveToArchive.
func SetStatus(id, path, status string) error {
	isWritable := slices.Contains(Statuses, status) && !slices.Contains(ArchiveStatuses, status)
	if !isWritable {
		return fmt.Errorf("SetStatus: Status %s: %w", status, ErrInvalidStatus)
	}
	err := edit(id, path, func(item *Item) string {
		previous := item.Status
		item.Status = status
		return id + " status " + previous + " → " + status + " (board)"
	})
	if err != nil {
		return fmt.Errorf("SetStatus: %w", err)
	}
	return nil
}

func checkId(id string, seen map[string]bool) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("checkId: Id %q: Invalid id: %w", id, ErrFormat)
	}
	if seen[id] {
		return fmt.Errorf("checkId: Id %s: Duplicate id: %w", id, ErrFormat)
	}
	seen[id] = true
	return nil
}

// edit loads the store, applies one change to the item, logs the delta the change returns and saves.
func edit(id, path string, apply func(item *Item) string) error {
	store, err := Load(path)
	if err != nil {
		return err
	}
	item := store.find(id)
	if item == nil {
		return fmt.Errorf("edit: Item %s: %w", id, ErrItemNotFound)
	}

	store.log(apply(item))
	return Save(path, store)
}

func firstUnknownDep(deps []string, known map[string]bool) string {
	for _, dep := range deps {
		if !known[dep] {
			return dep
		}
	}
	return ""
}
