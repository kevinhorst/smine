package secretscan

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

func isBinary(content []byte) bool {
	sniffLength := min(len(content), binarySniffLength)

	return bytes.IndexByte(content[:sniffLength], 0) >= 0
}

func matchesAnyGlob(globs []string, name string) bool {
	for _, glob := range globs {
		matched, err := filepath.Match(glob, name)
		if err == nil && matched {
			return true
		}
	}

	return false
}

func scanFile(path, relPath string, entry fs.DirEntry, patterns []compiledPattern, result *Result) error {
	if matchesAnyGlob(denyFileGlobs, entry.Name()) {
		result.treeFindings = append(result.treeFindings, Finding{
			value:      entry.Name(),
			Confidence: ConfidenceHigh,
			Detector:   detectorDenyFile,
			Excerpt:    entry.Name(),
			Line:       0,
			Path:       relPath,
		})
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("scanFile: Failed to stat %s: %w", relPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if info.Size() > maxFileSize {
		result.OversizeFiles = append(result.OversizeFiles, relPath)
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("scanFile: Failed to read %s: %w", relPath, err)
	}
	if isBinary(content) {
		result.BinarySkipCount++
		return nil
	}

	result.treeFindings = append(result.treeFindings, scanContent(patterns, relPath, content)...)
	return nil
}

func scanTree(repoPath string, patterns []compiledPattern, result *Result) error {
	walkErr := filepath.WalkDir(repoPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath, relErr := filepath.Rel(repoPath, path)
		if relErr != nil {
			return relErr
		}
		relPath = filepath.ToSlash(relPath)

		if entry.IsDir() {
			if slices.Contains(skipDirNames, entry.Name()) {
				return filepath.SkipDir
			}
			if slices.Contains(skipDirPaths, relPath) {
				return filepath.SkipDir
			}
			return nil
		}
		if matchesAnyGlob(skipFileGlobs, entry.Name()) {
			return nil
		}

		return scanFile(path, relPath, entry, patterns, result)
	})
	if walkErr != nil {
		return fmt.Errorf("scanTree: Failed to walk %s: %w", repoPath, walkErr)
	}

	sortFindings(result.treeFindings)
	return nil
}
