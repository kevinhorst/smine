package reviewscope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
)

const (
	artifactExt        = ".json"
	LaneManifestSuffix = ".lane-manifest.json"

	directionCodeStyle     = "code-style"
	directionContracts     = "contracts"
	directionCorrectness   = "correctness"
	directionCritical      = "critical"
	directionDataIntegrity = "data-integrity"
	directionSecurity      = "security"
	directionSpecialFocus  = "special-focus"
	directionTests         = "tests"
)

var (
	chunkSlugPattern    = regexp.MustCompile(`[^a-z0-9]+`)
	docFilePattern      = regexp.MustCompile(`(^|/)(docs/|README\.md$|LICENSE)|\.txt$`)
	findingIdPattern    = regexp.MustCompile(`^(.+)-(?:BLK|MAJ|MIN|NIT|INF)-(\d+)-r\d+$`)
	knownDirections     = []string{directionCodeStyle, directionContracts, directionCorrectness, directionCritical, directionDataIntegrity, directionSecurity, directionSpecialFocus, directionTests}
	securityKeepPattern = regexp.MustCompile(`(^|/)(SKILL\.md$|hooks/|settings)`)
)

// Chunk is one dispatcher-partitioned file group.
type Chunk struct {
	Files []string `json:"files"`
	Name  string   `json:"name"`
}

type Lane struct {
	Artifact  string `json:"artifact"`
	Chunk     string `json:"chunk"`
	Direction string `json:"direction"`
	Files     int    `json:"files"`
	Index     int    `json:"lane"`
	Manifest  string `json:"manifest"`
	Slug      string `json:"slug"`
}

// LaneManifest partners are read, never reviewed.
type LaneManifest struct {
	Artifact  string            `json:"artifact"`
	Direction string            `json:"direction"`
	Files     []string          `json:"files"`
	Head      string            `json:"head,omitempty"`
	Partners  []string          `json:"partners"`
	Root      string            `json:"root"`
	Walk      *contextdocs.Walk `json:"walk"`
}

// LanePlan is the lanes subcommand's output.
type LanePlan struct {
	IdFloor        map[string]int `json:"idFloor"`
	KnownPath      string         `json:"knownPath"`
	Lanes          []Lane         `json:"lanes"`
	MissingContext []string       `json:"missingContext"`
	Skipped        []SkippedLane  `json:"skipped"`
}

// Round 1 without a handed-in list has nothing known: no file, empty path.
func (p *LanePlan) WriteKnownView(handedInPath, roundDir string) error {
	p.IdFloor = map[string]int{}
	sources := []string{}
	if handedInPath != "" {
		sources = append(sources, handedInPath)
	}

	ownPath := filepath.Join(filepath.Dir(filepath.Clean(roundDir)), knownFile)
	if _, err := os.Stat(ownPath); err == nil {
		sources = append(sources, ownPath)
	}

	if len(sources) == 0 {
		return nil
	}

	rows := []KnownEntry{}
	for _, source := range sources {
		var part []KnownEntry
		if err := readJson(source, &part); err != nil {
			return err
		}
		rows = append(rows, part...)
	}

	for _, row := range rows {
		match := findingIdPattern.FindStringSubmatch(row.FindingId)
		if match == nil {
			continue
		}

		number, err := strconv.Atoi(match[2])
		if err != nil {
			return fmt.Errorf("LanePlan.WriteKnownView: %s: %w", row.FindingId, err)
		}
		p.IdFloor[match[1]] = max(p.IdFloor[match[1]], number)
	}

	p.KnownPath = filepath.Join(roundDir, knownFile)
	return writeJson(p.KnownPath, rows)
}

type LaneRequest struct {
	Ceiling      int
	Checklists   map[string]*contextdocs.Walk
	Chunks       []Chunk
	DiffFiles    []string
	Directions   []string
	Done         map[string][]string
	Head         string
	Lanes        int
	Pairs        *TestPairs
	RoundDir     string
	RuleSet      contextdocs.RuleSet
	Tokens       map[string][]string
	WorktreeRoot string
}

// SkippedLane is a direction × chunk that spawns no lane, with the reason.
type SkippedLane struct {
	Chunk     string `json:"chunk"`
	Direction string `json:"direction"`
	Reason    string `json:"reason"`
}

// chunkSlug lowers a chunk name to its artifact and branch token.
func chunkSlug(name string) string {
	return strings.Trim(chunkSlugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// chunksFor gives contracts the whole diff, every other direction the dispatcher's partition.
func chunksFor(direction string, request *LaneRequest) []Chunk {
	if direction == directionContracts || len(request.Chunks) == 0 {
		whole := Chunk{Files: request.DiffFiles}
		return []Chunk{whole}
	}
	return request.Chunks
}

// filterFiles keeps the files the predicate accepts.
func filterFiles(files []string, keep func(string) bool) []string {
	kept := make([]string, 0, len(files))
	for _, file := range files {
		if keep(file) {
			kept = append(kept, file)
		}
	}
	return kept
}

func laneFiles(direction string, files []string, request *LaneRequest) []string {
	done := request.Done[direction]
	isOpen := func(file string) bool { return !slices.Contains(done, file) }
	return filterFiles(relevantFiles(direction, files, request.Pairs), isOpen)
}

// laneSlug names a lane's artifact, manifest and worktree.
func laneSlug(chunkName, direction string, index int) string {
	if chunkName == "" {
		return fmt.Sprintf("%s-lane%d", direction, index)
	}
	return fmt.Sprintf("%s-%s-lane%d", direction, chunkSlug(chunkName), index)
}

// partnersOf lists the counterparts a tests lane reads beside its files.
func partnersOf(direction string, files []string, pairs *TestPairs) []string {
	partners := make([]string, 0)
	if direction != directionTests {
		return partners
	}

	for _, file := range files {
		for _, partner := range pairs.Partners[file] {
			isNew := !slices.Contains(files, partner) && !slices.Contains(partners, partner)
			if isNew {
				partners = append(partners, partner)
			}
		}
	}
	return partners
}

// planDirection adds one direction's lanes to the plan.
func planDirection(direction string, plan *LanePlan, request *LaneRequest) error {
	for _, chunk := range chunksFor(direction, request) {
		files := laneFiles(direction, chunk.Files, request)
		if len(files) == 0 {
			skipped := SkippedLane{Chunk: chunk.Name, Direction: direction, Reason: "no relevant files left"}
			plan.Skipped = append(plan.Skipped, skipped)
			continue
		}

		walk, err := walkFor(direction, files, request)
		if err != nil {
			return err
		}

		if walk != nil && len(walk.Groups) == 0 {
			if !slices.Contains(plan.MissingContext, direction) {
				plan.MissingContext = append(plan.MissingContext, direction)
			}
			continue
		}

		for index := 1; index <= request.Lanes; index++ {
			lane, err := writeLane(chunk.Name, direction, files, index, request, walk)
			if err != nil {
				return err
			}
			plan.Lanes = append(plan.Lanes, *lane)
		}
	}
	return nil
}

func relevantFiles(direction string, files []string, pairs *TestPairs) []string {
	isNoDoc := func(file string) bool { return !docFilePattern.MatchString(file) }
	switch direction {
	case directionCorrectness:
		isProduction := func(file string) bool { return !slices.Contains(pairs.Tests, file) }
		return filterFiles(files, isProduction)
	case directionSecurity:
		isSecurityRelevant := func(file string) bool { return isNoDoc(file) || securityKeepPattern.MatchString(file) }
		return filterFiles(files, isSecurityRelevant)
	case directionTests:
		return filterFiles(files, isNoDoc)
	case directionCodeStyle, directionContracts, directionCritical, directionDataIntegrity, directionSpecialFocus:
		return files
	}
	return files
}

// walkFor returns none on special-focus: its subject is the focus argument, not a context slice.
func walkFor(direction string, files []string, request *LaneRequest) (*contextdocs.Walk, error) {
	switch direction {
	case directionContracts, directionCorrectness:
		walk, isPresent := request.Checklists[direction]
		if !isPresent {
			return nil, fmt.Errorf("walkFor: Missing checklist for direction %s", direction)
		}
		return walk, nil
	case directionSpecialFocus:
		return nil, nil
	case directionCodeStyle, directionCritical, directionDataIntegrity, directionSecurity, directionTests:
		walkRequest := &contextdocs.WalkRequest{Ceiling: request.Ceiling, Direction: direction, Files: files, Tokens: request.Tokens[direction]}
		return contextdocs.BuildWalk(request.RuleSet, walkRequest)
	}
	return nil, fmt.Errorf("walkFor: Unknown direction %s", direction)
}

// writeLane writes one lane's manifest and returns its plan entry.
func writeLane(chunkName, direction string, files []string, index int, request *LaneRequest, walk *contextdocs.Walk) (*Lane, error) {
	slug := laneSlug(chunkName, direction, index)
	artifact := filepath.Join(request.RoundDir, slug+artifactExt)
	partners := partnersOf(direction, files, request.Pairs)
	root := ""
	if request.WorktreeRoot != "" {
		root = filepath.Join(request.WorktreeRoot, slug)
	}

	manifest := &LaneManifest{
		Artifact:  artifact,
		Direction: direction,
		Files:     append(slices.Clone(files), partners...),
		Head:      request.Head,
		Partners:  partners,
		Root:      root,
		Walk:      walk,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("writeLane: %w", err)
	}

	manifestPath := artifact + LaneManifestSuffix
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("writeLane: %w", err)
	}

	lane := &Lane{Artifact: artifact, Chunk: chunkName, Direction: direction, Files: len(files), Index: index, Manifest: manifestPath, Slug: slug}
	return lane, nil
}

// PlanLanes spawns no lane for a rule direction whose walk selects no entries; it lands in MissingContext.
func PlanLanes(request *LaneRequest) (*LanePlan, error) {
	for _, direction := range request.Directions {
		if !slices.Contains(knownDirections, direction) {
			return nil, fmt.Errorf("PlanLanes: Unknown direction %s", direction)
		}
	}

	if err := os.MkdirAll(request.RoundDir, 0o755); err != nil {
		return nil, fmt.Errorf("PlanLanes: %w", err)
	}

	plan := &LanePlan{Lanes: []Lane{}, MissingContext: []string{}, Skipped: []SkippedLane{}}
	for _, direction := range request.Directions {
		if err := planDirection(direction, plan, request); err != nil {
			return nil, err
		}
	}
	return plan, nil
}
