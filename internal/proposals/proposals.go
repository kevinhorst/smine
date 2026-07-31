// Package proposals loads sessions/proposals/<kind>.json — the sole
// authoritative proposal format (schema: sessions/proposals/schema.json).
package proposals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type AutoApplyHold struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

type Evidence struct {
	Dimension string    `json:"dimension"`
	Sessions  []Session `json:"sessions"`
	Snippets  []Snippet `json:"snippets"`
	Title     string    `json:"title"`
}

type Field struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type Proposal struct {
	AutoApplyHeld AutoApplyHold `json:"autoApplyHeld"`
	Change        string        `json:"change"`
	Decided       string        `json:"decided"`
	Evidence      []Evidence    `json:"evidence"`
	Fields        []Field       `json:"fields"`
	Id            string        `json:"id"`
	Proposed      string        `json:"proposed"`
	Status        string        `json:"status"`
	Tags          []string      `json:"tags"`
	Target        string        `json:"target"`
	Title         string        `json:"title"`
}

type Session struct {
	Id   string `json:"id"`
	Note string `json:"note"`
}

type Snippet struct {
	Code   string `json:"code"`
	Kind   string `json:"kind"`
	Lang   string `json:"lang"`
	Source string `json:"source"`
}

type Group struct {
	Proposals []Proposal `json:"proposals"`
	Title     string     `json:"title"`
}

type File struct {
	Groups  []Group `json:"groups"`
	Kind    string  `json:"kind"`
	Source  string  `json:"source"`
	Updated string  `json:"updated"`
}

// Load reads every dir/<kind>.json, sorted by kind. schema.json is skipped —
// it is the contract, not a proposal file. Malformed files are returned as
// load errors, never fatal.
func Load(dir string) ([]File, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("proposals.Load: Failed to read %s: %w", dir, err)
	}

	var files []File
	var loadErrors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || entry.Name() == "schema.json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		var file File
		if err := json.Unmarshal(data, &file); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Kind < files[j].Kind })
	return files, loadErrors, nil
}
