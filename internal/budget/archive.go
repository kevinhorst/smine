package budget

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	archiveDirMode   = 0o700
	archiveFileMode  = 0o600
	metaSuffix       = ".meta.json"
	projectsDirName  = "projects"
	subagentsDirName = "subagents"
	transcriptSuffix = ".jsonl"
)

// Relative paths are kept so ccusage reads Dir as a Claude directory of its own.
type Archive struct {
	ClaudeHome string
	Dir        string
}

// mirrorProject copies every session of one project directory.
func (a *Archive) mirrorProject(project string) (int, error) {
	source := filepath.Join(a.ClaudeHome, projectsDirName, project)
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, fmt.Errorf("Archive.mirrorProject: %w", err)
	}

	copied := 0
	for _, entry := range entries {
		isTranscript := !entry.IsDir() && strings.HasSuffix(entry.Name(), transcriptSuffix)
		if !isTranscript {
			continue
		}

		count, err := a.mirrorSession(project, entry.Name())
		copied += count
		if err != nil {
			return copied, err
		}
	}
	return copied, nil
}

// mirrorSession copies one root transcript and its subagent files.
func (a *Archive) mirrorSession(project, transcript string) (int, error) {
	relative := filepath.Join(projectsDirName, project, transcript)
	copied := 0
	isCopied, err := copyChanged(filepath.Join(a.ClaudeHome, relative), filepath.Join(a.Dir, relative))
	if err != nil {
		return copied, err
	}
	if isCopied {
		copied++
	}

	stem := strings.TrimSuffix(transcript, transcriptSuffix)
	subagents := filepath.Join(a.ClaudeHome, projectsDirName, project, stem, subagentsDirName)
	walkErr := filepath.WalkDir(subagents, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipAll
		}
		if !isArchived(entry) {
			return nil
		}

		tail, err := filepath.Rel(a.ClaudeHome, path)
		if err != nil {
			return fmt.Errorf("Archive.mirrorSession: %w", err)
		}
		isCopied, err := copyChanged(path, filepath.Join(a.Dir, tail))
		if isCopied {
			copied++
		}
		return err
	})
	if walkErr != nil {
		return copied, fmt.Errorf("Archive.mirrorSession: %w", walkErr)
	}
	return copied, nil
}

// The first failing file ends the run with the count so far.
func (a *Archive) Mirror() (int, error) {
	projects, err := os.ReadDir(filepath.Join(a.ClaudeHome, projectsDirName))
	if err != nil {
		return 0, fmt.Errorf("Archive.Mirror: %w", err)
	}

	copied := 0
	for _, project := range projects {
		if !project.IsDir() {
			continue
		}

		count, err := a.mirrorProject(project.Name())
		copied += count
		if err != nil {
			return copied, err
		}
	}
	return copied, nil
}

// copyChanged copies source over target only when the source grew; an equal or smaller source keeps the archived copy (master E4).
func copyChanged(source, target string) (bool, error) {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	targetInfo, err := os.Stat(target)
	if err == nil && targetInfo.Size() >= sourceInfo.Size() {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(target), archiveDirMode); err != nil {
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	input, err := os.Open(source)
	if err != nil {
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	defer input.Close()

	temp := target + ".tmp"
	output, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, archiveFileMode)
	if err != nil {
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		os.Remove(temp)
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	if err := output.Close(); err != nil {
		os.Remove(temp)
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	if err := fsx.ReplaceFile(temp, target); err != nil {
		return false, fmt.Errorf("copyChanged: %w", err)
	}
	return true, nil
}

// isArchived keeps transcripts and agent meta files.
func isArchived(entry fs.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	name := entry.Name()
	return strings.HasSuffix(name, transcriptSuffix) || strings.HasSuffix(name, metaSuffix)
}

func TranscriptPath(claudeHome, id string) string {
	if !sessionIdRe.MatchString(id) {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(claudeHome, projectsDirName, "*", id+transcriptSuffix))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}
