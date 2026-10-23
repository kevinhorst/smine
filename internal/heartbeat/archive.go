package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	ErrInvalidInstance = errors.New("ReadArchive: Invalid instance id")
	ErrUnknownArchive  = errors.New("ReadArchive: Unknown archived fleet")
	instancePattern    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Mirrors cmd/heartbeat/src/wire.rs FleetArchive, schema archive-v1.json.
type ArchivedFleet struct {
	DissolvedAt   string           `json:"dissolved_at"`
	Fleet         string           `json:"fleet"`
	Instance      string           `json:"instance"`
	Journal       []JournalEntry   `json:"journal"`
	Members       []ArchivedMember `json:"members"`
	MergeBranch   string           `json:"merge_branch"`
	Repo          string           `json:"repo"`
	StartedAt     string           `json:"started_at"`
	Subscriptions []Subscription   `json:"subscriptions"`
	Synced        bool             `json:"synced"`
	Target        string           `json:"target"`
	Version       int              `json:"v"`
}

// ArchivedMember is one member of a dissolved run, as last seen.
type ArchivedMember struct {
	Branch    string `json:"branch"`
	Delivered bool   `json:"delivered"`
	JoinedAt  string `json:"joined_at"`
	LeftAt    string `json:"left_at"`
	SessionId string `json:"session_id"`
	Token     string `json:"token"`
	Via       Via    `json:"via"`
}

// shortInstanceLen is the length of a UUID's first block.
const shortInstanceLen = 8

// ShortInstance is the first block of the run's UUID, for tables.
func (f *ArchivedFleet) ShortInstance() string {
	if len(f.Instance) < shortInstanceLen {
		return f.Instance
	}
	return f.Instance[:shortInstanceLen]
}

// Newest dissolve first; a missing dir is empty.
func OpenArchive(dir string) ([]ArchivedFleet, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("OpenArchive: Invalid dir %s: %w", dir, err)
	}

	fleets := make([]ArchivedFleet, 0, len(paths))
	for _, path := range paths {
		archived, err := readArchive(path)
		if err != nil {
			return nil, fmt.Errorf("OpenArchive: %w", err)
		}
		fleets = append(fleets, *archived)
	}
	sort.SliceStable(fleets, func(i, j int) bool { return fleets[i].DissolvedAt > fleets[j].DissolvedAt })
	return fleets, nil
}

// ReadArchive reads the archived run instance from dir.
func ReadArchive(dir string, instance string) (*ArchivedFleet, error) {
	if !instancePattern.MatchString(instance) {
		return nil, fmt.Errorf("%w %q", ErrInvalidInstance, instance)
	}

	path := filepath.Join(dir, instance+".json")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrUnknownArchive, instance, err)
	}
	return readArchive(path)
}

func readArchive(path string) (*ArchivedFleet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readArchive: Failed to read %s: %w", path, err)
	}

	archived := &ArchivedFleet{}
	if err := json.Unmarshal(raw, archived); err != nil {
		return nil, fmt.Errorf("readArchive: Failed to decode %s: %w", path, err)
	}

	if archived.Instance+".json" != filepath.Base(path) {
		return nil, fmt.Errorf("readArchive: %s names instance %q", path, archived.Instance)
	}
	return archived, nil
}
