package skillgraph

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/semver"
)

// bumpKind is the index of the first component where next differs from previous; equal versions answer -1.
func bumpKind(previous, next string) (int, error) {
	left, err := semver.Parse(previous)
	if err != nil {
		return 0, err
	}

	right, err := semver.Parse(next)
	if err != nil {
		return 0, err
	}

	for index := range semver.Parts {
		if semver.ComponentAt(left, index) != semver.ComponentAt(right, index) {
			return index, nil
		}
	}
	return -1, nil
}

// bump increments the component at kind and zeroes the ones after it.
func bump(version string, kind int) (string, error) {
	components, err := semver.Parse(version)
	if err != nil {
		return "", err
	}

	for len(components) <= kind {
		components = append(components, 0)
	}
	components[kind]++
	for index := kind + 1; index < len(components); index++ {
		components[index] = 0
	}

	parts := make([]string, 0, len(components))
	for _, component := range components {
		parts = append(parts, strconv.Itoa(component))
	}
	return strings.Join(parts, "."), nil
}

// rowKinds is each row's bump kind against the row below it; an equal pair inherits the lower row's kind, the bottom row is a patch.
func rowKinds(rows []ChangelogEntry) ([]int, error) {
	kinds := make([]int, len(rows))
	for index := len(rows) - 1; index >= 0; index-- {
		if index == len(rows)-1 {
			kinds[index] = semver.Parts - 1
			continue
		}

		kind, err := bumpKind(rows[index+1].Version, rows[index].Version)
		if err != nil {
			return nil, err
		}

		if kind < 0 {
			kind = kinds[index+1]
		}
		kinds[index] = kind
	}
	return kinds, nil
}

// renumberRows lifts, bottom-up, every row that is not above the row below it by its own kind; the input stays untouched.
func renumberRows(rows []ChangelogEntry, kinds []int) ([]ChangelogEntry, error) {
	if len(kinds) != len(rows) {
		return nil, errors.New("renumberRows: One kind per row required")
	}

	renumbered := slices.Clone(rows)
	for below := len(renumbered) - 1; below > 0; below-- {
		upper := below - 1
		running := renumbered[below].Version
		order, err := semver.Compare(renumbered[upper].Version, running)
		if err != nil {
			return nil, err
		}

		if order > 0 {
			continue
		}

		lifted, err := bump(running, kinds[upper])
		if err != nil {
			return nil, err
		}
		renumbered[upper].Version = lifted
	}
	return renumbered, nil
}

// setVersion writes the frontmatter version, keeping the authored leading whitespace and quoting.
func setVersion(skill *Skill, version string) {
	for index, field := range skill.Frontmatter {
		if field.Key != "version" {
			continue
		}

		trimmed := strings.TrimLeft(field.Value, " \t")
		prefix := field.Value[:len(field.Value)-len(trimmed)]
		quote := ""
		if strings.HasPrefix(trimmed, `"`) || strings.HasPrefix(trimmed, "'") {
			quote = trimmed[:1]
		}
		skill.Frontmatter[index].Value = prefix + quote + version + quote
	}
}

// RenumberChangelog rewrites one model file so its versions strictly descend, the frontmatter version following the top row.
func RenumberChangelog(data []byte) ([]byte, error) {
	skill, err := DecodeSkill(data, modelDecodeLabel)
	if err != nil {
		return nil, fmt.Errorf("RenumberChangelog: %w", err)
	}

	kinds, err := rowKinds(skill.Changelog)
	if err != nil {
		return nil, fmt.Errorf("RenumberChangelog: %w", err)
	}

	rows, err := renumberRows(skill.Changelog, kinds)
	if err != nil {
		return nil, fmt.Errorf("RenumberChangelog: %w", err)
	}

	skill.Changelog = rows
	setVersion(skill, rows[0].Version)
	encoded, err := EncodeSkill(skill)
	if err != nil {
		return nil, fmt.Errorf("RenumberChangelog: %w", err)
	}
	return encoded, nil
}
