package proposals

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Vote is one decision line in sessions/proposals/votes.jsonl — the sidecar
// the config server writes and the smine-nightly apply stage consumes. The
// analyze skills never touch it.
type Vote struct {
	Id      string `json:"id"`
	Comment string `json:"comment"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Ts      string `json:"ts"`
	Vote    string `json:"vote"`
}

// LoadVotes reads path as JSONL; the last line per (kind, id) wins. A
// missing file means no votes.
func LoadVotes(path string) (map[string]Vote, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Vote{}, nil
		}
		return nil, fmt.Errorf("LoadVotes: Failed to read %s: %w", path, err)
	}

	votes := make(map[string]Vote)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var vote Vote
		if err := json.Unmarshal([]byte(line), &vote); err != nil {
			return nil, fmt.Errorf("LoadVotes: Malformed line in %s: %w", path, err)
		}
		votes[VoteKey(vote.Kind, vote.Id)] = vote
	}
	return votes, nil
}

// SetVote drops any stored line for the vote's (kind, id), appends the new
// line, and saves atomically (.tmp + rename), mirroring checklist.SetStatus.
func SetVote(path string, vote *Vote) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("SetVote: Failed to read %s: %w", path, err)
	}

	kept := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var existing Vote
		if err := json.Unmarshal([]byte(line), &existing); err != nil {
			return fmt.Errorf("SetVote: Malformed line in %s: %w", path, err)
		}
		if VoteKey(existing.Kind, existing.Id) == VoteKey(vote.Kind, vote.Id) {
			continue
		}
		kept = append(kept, line)
	}

	encoded, err := json.Marshal(vote)
	if err != nil {
		return fmt.Errorf("SetVote: Failed to encode vote: %w", err)
	}
	kept = append(kept, string(encoded))

	out := []byte(strings.Join(kept, "\n") + "\n")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return fmt.Errorf("SetVote: Failed to write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("SetVote: Failed to rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

// VoteKey is the unique handle a vote replaces on: one pending decision per
// (kind, id).
func VoteKey(kind, id string) string {
	return kind + "/" + id
}
