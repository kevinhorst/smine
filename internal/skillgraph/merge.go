package skillgraph

import (
	"errors"
	"fmt"
	"slices"
)

// ErrUnmergeable marks a side whose changelog is not the base changelog with rows added on top.
var ErrUnmergeable = errors.New("changelog is not the base changelog with rows added")

// ChangelogMerge is one model file merge read from its three sides: the rows each side added and their bump kinds.
type ChangelogMerge struct {
	base        *Skill
	ours        *Skill
	oursAdded   []ChangelogEntry
	oursKinds   []int
	theirs      *Skill
	theirsAdded []ChangelogEntry
	theirsKinds []int

	OursNewest   string
	TheirsNewest string
}

// Merged holds the rows and version to apply plus the three sides re-encoded with the base changelog and version.
type Merged struct {
	Base    []byte
	Ours    []byte
	Rows    []ChangelogEntry
	Theirs  []byte
	Version string
}

// ParseMerge decodes the three sides and reads each side's added rows; a side that added none has an empty newest date.
func ParseMerge(base, ours, theirs []byte) (*ChangelogMerge, error) {
	baseSkill, err := DecodeSkill(base, "base")
	if err != nil {
		return nil, fmt.Errorf("ParseMerge: %w", err)
	}

	oursSkill, err := DecodeSkill(ours, "ours")
	if err != nil {
		return nil, fmt.Errorf("ParseMerge: %w", err)
	}

	theirsSkill, err := DecodeSkill(theirs, "theirs")
	if err != nil {
		return nil, fmt.Errorf("ParseMerge: %w", err)
	}

	oursAdded, oursKinds, err := addedRows(baseSkill.Changelog, oursSkill.Changelog)
	if err != nil {
		return nil, fmt.Errorf("ParseMerge: ours: %w", err)
	}

	theirsAdded, theirsKinds, err := addedRows(baseSkill.Changelog, theirsSkill.Changelog)
	if err != nil {
		return nil, fmt.Errorf("ParseMerge: theirs: %w", err)
	}

	merge := &ChangelogMerge{
		base:         baseSkill,
		ours:         oursSkill,
		oursAdded:    oursAdded,
		oursKinds:    oursKinds,
		theirs:       theirsSkill,
		theirsAdded:  theirsAdded,
		theirsKinds:  theirsKinds,
		OursNewest:   newestDate(oursAdded),
		TheirsNewest: newestDate(theirsAdded),
	}
	return merge, nil
}

// Resolve unions the added rows, fresher side on top, renumbers them and strips every side to the base changelog.
func (m *ChangelogMerge) Resolve(isOursFresher bool) (*Merged, error) {
	older, olderKinds := m.theirsAdded, m.theirsKinds
	fresher, fresherKinds := m.oursAdded, m.oursKinds
	if !isOursFresher {
		older, olderKinds, fresher, fresherKinds = m.oursAdded, m.oursKinds, m.theirsAdded, m.theirsKinds
	}
	fresher, fresherKinds = dropShared(fresher, fresherKinds, older)

	baseKinds, err := rowKinds(m.base.Changelog)
	if err != nil {
		return nil, fmt.Errorf("Resolve: %w", err)
	}

	rows := slices.Concat(fresher, older, m.base.Changelog)
	kinds := slices.Concat(fresherKinds, olderKinds, baseKinds)
	renumbered, err := renumberRows(rows, kinds)
	if err != nil {
		return nil, fmt.Errorf("Resolve: %w", err)
	}

	baseVersion := m.base.Version()
	baseBytes, err := stripped(m.base, m.base.Changelog, baseVersion)
	if err != nil {
		return nil, fmt.Errorf("Resolve: base: %w", err)
	}

	oursBytes, err := stripped(m.ours, m.base.Changelog, baseVersion)
	if err != nil {
		return nil, fmt.Errorf("Resolve: ours: %w", err)
	}

	theirsBytes, err := stripped(m.theirs, m.base.Changelog, baseVersion)
	if err != nil {
		return nil, fmt.Errorf("Resolve: theirs: %w", err)
	}

	merged := &Merged{
		Base:    baseBytes,
		Ours:    oursBytes,
		Rows:    renumbered,
		Theirs:  theirsBytes,
		Version: renumbered[0].Version,
	}
	return merged, nil
}

// ApplyChangelog decodes a merged remainder, sets the merged rows and version and encodes it canonically.
func ApplyChangelog(data []byte, rows []ChangelogEntry, version string) ([]byte, error) {
	skill, err := DecodeSkill(data, modelDecodeLabel)
	if err != nil {
		return nil, fmt.Errorf("ApplyChangelog: %w", err)
	}

	skill.Changelog = rows
	setVersion(skill, version)
	encoded, err := EncodeSkill(skill)
	if err != nil {
		return nil, fmt.Errorf("ApplyChangelog: %w", err)
	}
	return encoded, nil
}

// addedRows is the prefix a side holds above the base rows, with each row's bump kind on that side.
func addedRows(base, side []ChangelogEntry) ([]ChangelogEntry, []int, error) {
	count := len(side) - len(base)
	if count < 0 || !sameRows(side[count:], base) {
		return nil, nil, ErrUnmergeable
	}

	kinds, err := rowKinds(side)
	if err != nil {
		return nil, nil, err
	}
	return slices.Clone(side[:count]), kinds[:count], nil
}

// sameRows compares rows by date, version, text and changes; tokens are a recount, never identity.
func sameRows(rows, others []ChangelogEntry) bool {
	if len(rows) != len(others) {
		return false
	}

	for index := range rows {
		if !sameRow(rows[index], others[index]) {
			return false
		}
	}
	return true
}

// sameRow is sameRows for one pair.
func sameRow(a, b ChangelogEntry) bool {
	isSameText := a.Date == b.Date && a.Version == b.Version && a.Text == b.Text
	return isSameText && slices.Equal(a.Changes, b.Changes)
}

// dropShared removes from the fresher rows the ones the older side added as well, by date, text and changes.
func dropShared(fresher []ChangelogEntry, kinds []int, older []ChangelogEntry) ([]ChangelogEntry, []int) {
	keptRows := make([]ChangelogEntry, 0, len(fresher))
	keptKinds := make([]int, 0, len(kinds))
	for index, row := range fresher {
		isShared := slices.ContainsFunc(older, func(other ChangelogEntry) bool {
			return row.Date == other.Date && row.Text == other.Text && slices.Equal(row.Changes, other.Changes)
		})
		if isShared {
			continue
		}

		keptRows = append(keptRows, row)
		keptKinds = append(keptKinds, kinds[index])
	}
	return keptRows, keptKinds
}

// newestDate is the first row's date, empty without rows.
func newestDate(rows []ChangelogEntry) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[0].Date
}

// stripped encodes a side with the base changelog and version in place of its own.
func stripped(skill *Skill, rows []ChangelogEntry, version string) ([]byte, error) {
	copied := *skill
	copied.Changelog = rows
	copied.Frontmatter = slices.Clone(skill.Frontmatter)
	setVersion(&copied, version)
	return EncodeSkill(&copied)
}
