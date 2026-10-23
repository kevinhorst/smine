package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// The deploy marker's two lines: the hash of the source the copy was rendered from, and the hash of the rendered files as written.
const (
	markerRenderedPrefix = "rendered "
	markerSourcePrefix   = "source "
)

// unshippedSourceDirs never reach a deployed copy, so a change there leaves the copy current.
var unshippedSourceDirs = []string{evalDirName, skillgraph.DirSmoke}

// deployedHash is renderedHash over what a deployed dir holds now, marker excluded.
func deployedHash(dir string) (string, error) {
	var files []*deployedFile
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("deployedHash: %w", err)
		}
		if !entry.Type().IsRegular() || rel == markerFileName {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("deployedHash: %w", err)
		}
		files = append(files, &deployedFile{content: content, path: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("deployedHash: %w", err)
	}
	return renderedHash(files), nil
}

// isLocallyModified reports a marked dir whose files no longer hash to the marker's rendered line; a marker without the line (an older deploy) is compared with wanted, the fresh render, so only a differing copy is stashed once.
func isLocallyModified(dir, wanted string) (bool, error) {
	marker, err := os.ReadFile(filepath.Join(dir, markerFileName))
	if err != nil {
		return false, fmt.Errorf("isLocallyModified: %w", err)
	}
	current, err := deployedHash(dir)
	if err != nil {
		return false, err
	}

	_, rendered, hasRendered := strings.Cut(string(marker), markerRenderedPrefix)
	if !hasRendered {
		return current != wanted, nil
	}
	return strings.TrimSpace(rendered) != current, nil
}

// markerContent is the deploy marker of a copy rendered from leafDir as files.
func markerContent(leafDir string, files []*deployedFile) ([]byte, error) {
	hash, err := sourceHash(leafDir)
	if err != nil {
		return nil, err
	}
	return []byte(markerSourcePrefix + hash + "\n" + markerRenderedPrefix + renderedHash(files) + "\n"), nil
}

// renderedHash is the sha256 over the files a deploy writes, marker excluded, in path order: rel path NUL content NUL.
func renderedHash(files []*deployedFile) string {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(left, right *deployedFile) int { return strings.Compare(left.path, right.path) })
	hash := sha256.New()
	for _, file := range sorted {
		if file.path == markerFileName {
			continue
		}
		hash.Write([]byte(file.path))
		hash.Write([]byte{0})
		hash.Write(file.content)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// sourceHash is the sha256 over every source file of the skill but eval/ and smoke/, projection blocks stripped, in path order.
func sourceHash(leafDir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(leafDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(leafDir, path)
		if err != nil {
			return fmt.Errorf("sourceHash: %w", err)
		}
		if entry.IsDir() && slices.Contains(unshippedSourceDirs, rel) {
			return fs.SkipDir
		}
		if entry.Type().IsRegular() {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("sourceHash: %w", err)
	}

	hash := sha256.New()
	slices.Sort(paths)
	for _, rel := range paths {
		path := filepath.Join(leafDir, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("sourceHash: %w", err)
		}
		hash.Write([]byte(filepath.ToSlash(rel)))
		hash.Write([]byte{0})
		hash.Write(acdsl.StripProjection(path, content))
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// IsDeployedCurrent reports whether the copy in deployedDir was rendered from leafDir's current source; a copy without a marker is not.
func IsDeployedCurrent(leafDir, deployedDir string) (bool, error) {
	deployed, err := os.ReadFile(filepath.Join(deployedDir, markerFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("IsDeployedCurrent: %w", err)
	}

	expected, err := sourceHash(leafDir)
	if err != nil {
		return false, fmt.Errorf("IsDeployedCurrent: %w", err)
	}
	firstLine, _, _ := strings.Cut(string(deployed), "\n")
	return firstLine == markerSourcePrefix+expected, nil
}
