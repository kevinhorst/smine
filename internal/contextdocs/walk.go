package contextdocs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// File-first answers once per file × group, group-first once per group over the whole diff.
const (
	WalkShapeFileFirst  = "file-first"
	WalkShapeGroupFirst = "group-first"
)

// Style rules live in the language guides, not in the injection map.
const codeStyleDirection = "code-style"

type LaneMap struct {
	Lanes map[string][]string `json:"lanes"`
}

// LoadLaneMap reads a context-map.json.
func LoadLaneMap(path string) (*LaneMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadLaneMap: %w", err)
	}
	var laneMap LaneMap
	if err := json.Unmarshal(data, &laneMap); err != nil {
		return nil, fmt.Errorf("LoadLaneMap: %w", err)
	}
	return &laneMap, nil
}

// The read-gate hook denies the walk_complete write while a pair has no answer.
type Walk struct {
	Groups []WalkGroup `json:"groups"`
	Shape  string      `json:"shape"`
}

// Named by its family, "/n"-suffixed when the family was split.
type WalkGroup struct {
	Id    string   `json:"id"`
	Items []string `json:"items"`
}

type WalkRequest struct {
	Ceiling   int
	Direction string
	Files     []string
	Tokens    []string
}

// entryFamily is the id without its number: KIND-SCOPE[-TOPIC].
func entryFamily(id string) string {
	if cut := strings.LastIndex(id, "-"); cut > 0 {
		return id[:cut]
	}
	return id
}

// guidePathsFor returns the paths of the guides governing any of the files.
func guidePathsFor(guides []RuleGuide, files []string) []string {
	var paths []string
	for _, guide := range guides {
		if isGuideApplicable(guide, files) {
			paths = append(paths, guide.Path)
		}
	}
	return paths
}

func isGuideApplicable(guide RuleGuide, files []string) bool {
	for _, glob := range guide.Files {
		for _, file := range files {
			if GlobMatches(glob, file) {
				return true
			}
		}
	}
	return false
}

// Lint, gate, hook and schema entries are the tools' job, not review effort.
func isProseEntry(entry RuleEntry) bool {
	return entry.Enforcement == "" || entry.Enforcement == channelProse
}

// matchesGuide reports whether the entry source is one of the guide paths.
func matchesGuide(source string, guidePaths []string) bool {
	for _, guidePath := range guidePaths {
		if source == guidePath || strings.HasSuffix(source, "/"+guidePath) {
			return true
		}
	}
	return false
}

func matchesToken(id string, tokens []string) bool {
	for _, token := range tokens {
		if TokenMatches(id, token) {
			return true
		}
	}
	return false
}

// Reach is the injection's predicate; a guide entry joins a code-style walk only through a guide governing a lane file.
func selectWalkEntries(set RuleSet, request *WalkRequest) []string {
	var guidePaths, allGuidePaths []string
	if request.Direction == codeStyleDirection {
		guidePaths = guidePathsFor(set.Guides, request.Files)
		for _, guide := range set.Guides {
			allGuidePaths = append(allGuidePaths, guide.Path)
		}
	}
	selected := map[string]bool{}
	for _, entry := range set.Entries {
		if !isProseEntry(entry) || !IsReachable(entry.Reach, set.Self) {
			continue
		}
		isTokenSelected := !matchesGuide(entry.Source, allGuidePaths) && matchesToken(entry.Id, request.Tokens)
		isSelected := isTokenSelected || matchesGuide(entry.Source, guidePaths)
		if isSelected {
			selected[entry.Id] = true
		}
	}
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func splitFamily(family string, ids []string, ceiling int) []WalkGroup {
	if len(ids) <= ceiling {
		return []WalkGroup{{Id: family, Items: ids}}
	}
	groups := make([]WalkGroup, 0, (len(ids)+ceiling-1)/ceiling)
	for start := 0; start < len(ids); {
		chunk := ids[start:min(start+ceiling, len(ids))]
		group := WalkGroup{Id: fmt.Sprintf("%s/%d", family, len(groups)+1), Items: chunk}
		groups = append(groups, group)
		start += len(chunk)
	}
	return groups
}

// An empty walk is the caller's stop condition.
func BuildWalk(set RuleSet, request *WalkRequest) (*Walk, error) {
	if request.Ceiling <= 0 {
		return nil, fmt.Errorf("BuildWalk: Ceiling must be positive, got %d", request.Ceiling)
	}
	if request.Direction == "" {
		return nil, errors.New("BuildWalk: Missing field Direction")
	}
	byFamily := map[string][]string{}
	for _, id := range selectWalkEntries(set, request) {
		family := entryFamily(id)
		byFamily[family] = append(byFamily[family], id)
	}
	families := make([]string, 0, len(byFamily))
	for family := range byFamily {
		families = append(families, family)
	}
	sort.Strings(families)
	walk := &Walk{Groups: make([]WalkGroup, 0, len(families)), Shape: WalkShapeFileFirst}
	for _, family := range families {
		walk.Groups = append(walk.Groups, splitFamily(family, byFamily[family], request.Ceiling)...)
	}
	return walk, nil
}

// Matches the way the read gate does: * spans directories, ? one character.
func GlobMatches(glob, file string) bool {
	pattern := strings.ReplaceAll(regexp.QuoteMeta(glob), `\*`, ".*")
	pattern = "^" + strings.ReplaceAll(pattern, `\?`, ".") + "$"
	return regexp.MustCompile(pattern).MatchString(file)
}

func TokenMatches(id, token string) bool {
	prefix, isGlob := strings.CutSuffix(token, "*")
	if isGlob {
		return strings.HasPrefix(id, prefix)
	}
	return id == token
}
