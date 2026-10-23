package reviewscope

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/trash"
)

const (
	checklistPrefix = "checklist-"
	jsonSuffix      = ".json"
	salvageDirName  = "salvage"
	verdictsDir     = "verdicts"
)

// Path depths of the keep list, counted in segments below the artifact dir.
const (
	topFileDepth     = 1
	roundFileDepth   = 2
	verdictFileDepth = 3
)

var (
	keptRoundFiles = []string{"chunks.json", "known.json", "review.json", "risk-map.json"}
	keptTopFiles   = []string{"coverage.json", "known.json", "review-record.md"}
	laneReportName = regexp.MustCompile(`-lane[0-9]+\.json$`)
	roundDirName   = regexp.MustCompile(`^round-[0-9]+$`)
)

type PruneRequest struct {
	ArtifactsDir string
	IsVerbose    bool
	Trash        *trash.Store
}

// PruneResult is the prune subcommand's output: files kept and moved to the trash, Stash the first move's stamp.
type PruneResult struct {
	Kept    int    `json:"kept"`
	Removed int    `json:"removed"`
	Stash   string `json:"stash,omitempty"`
}

func isKeptPath(relative string, isVerbose bool) bool {
	parts := strings.Split(relative, "/")
	if parts[0] == salvageDirName {
		return true
	}

	switch len(parts) {
	case topFileDepth:
		return slices.Contains(keptTopFiles, parts[0])
	case roundFileDepth:
		return roundDirName.MatchString(parts[0]) && isKeptRoundFile(parts[1], isVerbose)
	case verdictFileDepth:
		isVerdict := parts[1] == verdictsDir && isNamedJson(parts[2], "")
		isRoundVerdict := roundDirName.MatchString(parts[0]) && isVerdict
		return isVerbose && isRoundVerdict
	}
	return false
}

func isKeptRoundFile(name string, isVerbose bool) bool {
	if slices.Contains(keptRoundFiles, name) || isNamedJson(name, checklistPrefix) {
		return true
	}
	return isVerbose && laneReportName.MatchString(name)
}

// isNamedJson reports whether name is prefix, a non-empty stem and .json.
func isNamedJson(name, prefix string) bool {
	hasAffixes := strings.HasPrefix(name, prefix) && strings.HasSuffix(name, jsonSuffix)
	return hasAffixes && len(name) > len(prefix)+len(jsonSuffix)
}

// removeEmptyDirs removes every listed dir left empty, deepest first.
func removeEmptyDirs(dirs []string) error {
	for _, dir := range slices.Backward(dirs) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("removeEmptyDirs: %w", err)
		}

		if len(entries) > 0 {
			continue
		}

		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("removeEmptyDirs: %w", err)
		}
	}
	return nil
}

// Prune moves non-kept files and symlinks, unfollowed, to the trash and never removes the artifact dir itself.
func Prune(request *PruneRequest) (*PruneResult, error) {
	root := request.ArtifactsDir
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("Prune: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("Prune: Artifacts dir %s is not a directory", root)
	}

	result := &PruneResult{}
	dirs := make([]string, 0)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root {
			return nil
		}

		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}

		isRegular := entry.Type().IsRegular()
		if isRegular && isKeptPath(filepath.ToSlash(relative), request.IsVerbose) {
			result.Kept++
			return nil
		}

		removal := &trash.Entry{Action: "review prune", Reason: "outside the keep list"}
		stash, err := request.Trash.Move(path, removal)
		if err != nil {
			return fmt.Errorf("move %s: %w", path, err)
		}

		if result.Stash == "" {
			result.Stash, _, _ = strings.Cut(filepath.ToSlash(stash), "/")
		}
		result.Removed++
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("Prune: Failed to walk %s: %w", root, walkErr)
	}

	if err := removeEmptyDirs(dirs); err != nil {
		return nil, err
	}
	return result, nil
}
