package skillimport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const legacySchemaVersion = 1

type legacyAtom struct {
	Id string `json:"id"`

	Aliases []string `json:"aliases,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Members []string `json:"members,omitempty"`
	Source  string   `json:"source,omitempty"`
}

type legacyAtomFile struct {
	SchemaVersion int `json:"schemaVersion"`

	Exclusive [][]string              `json:"exclusive,omitempty"`
	Sorts     map[string][]legacyAtom `json:"sorts"`
}

func addSort(atoms map[string]*skillgraph.Atom, records []legacyAtom, sortName string) error {
	for _, record := range records {
		if record.Id == "" {
			return fmt.Errorf("addSort: Sort %s: Atom without id", sortName)
		}

		if _, isDuplicate := atoms[record.Id]; isDuplicate {
			return fmt.Errorf("addSort: Atom %s: Duplicate id", record.Id)
		}

		atom := &skillgraph.Atom{
			Aliases: record.Aliases,
			Kind:    record.Kind,
			Members: record.Members,
			Sort:    sortName,
			Source:  record.Source,
		}
		if len(atom.Aliases) == 0 {
			atom.Aliases = nil
		}
		atoms[record.Id] = atom
	}
	return nil
}

// ImportVocabulary refuses unknown keys, another schemaVersion and bad ids: nothing is dropped silently.
func ImportVocabulary(path string) (*skillgraph.Vocabulary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ImportVocabulary: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	file := &legacyAtomFile{}
	if err := decoder.Decode(file); err != nil {
		return nil, fmt.Errorf("ImportVocabulary: %s: %w", path, err)
	}

	if file.SchemaVersion != legacySchemaVersion {
		return nil, fmt.Errorf("ImportVocabulary: %s: Unsupported schemaVersion %d", path, file.SchemaVersion)
	}

	atoms := map[string]*skillgraph.Atom{}
	for _, sortName := range slices.Sorted(maps.Keys(file.Sorts)) {
		if err := addSort(atoms, file.Sorts[sortName], sortName); err != nil {
			return nil, fmt.Errorf("ImportVocabulary: %s: %w", path, err)
		}
	}

	vocabulary := &skillgraph.Vocabulary{Atoms: atoms, Exclusive: file.Exclusive}
	return vocabulary, nil
}
