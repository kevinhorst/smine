package main

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Case-folded: the default macOS and Windows filesystems treat ".GIT" as the same directory.
func IsInsideGitDir(cwd string, path string) bool {
	segments := strings.Split(resolvePath(cwd, path), string(os.PathSeparator))
	return slices.ContainsFunc(segments, func(segment string) bool {
		return strings.EqualFold(segment, gitDirName)
	})
}

func IsInsideRoots(cwd string, path string, roots []string) bool {
	target := resolvePath(cwd, path)
	for _, root := range roots {
		if root == "" {
			continue
		}
		if strings.Contains(root, globMarker) {
			if insideGlobRoot(cwd, target, root) {
				return true
			}
			continue
		}
		resolvedRoot := resolvePath(cwd, root)
		if target == resolvedRoot {
			return true
		}
		if strings.HasPrefix(target, resolvedRoot+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func insideGlobRoot(cwd string, target string, root string) bool {
	separator := string(os.PathSeparator)
	segments := strings.Split(filepath.Clean(root), separator)
	firstGlob := slices.IndexFunc(segments, func(segment string) bool {
		return strings.Contains(segment, globMarker)
	})
	prefix := resolvePath(cwd, strings.Join(segments[:firstGlob], separator))
	rest, isInside := strings.CutPrefix(target, prefix+separator)
	if !isInside {
		return false
	}

	patterns := segments[firstGlob:]
	targetSegments := strings.Split(rest, separator)
	if len(targetSegments) < len(patterns) {
		return false
	}
	for index, pattern := range patterns {
		isMatch, err := path.Match(pattern, targetSegments[index])
		if err != nil || !isMatch {
			return false
		}
	}
	return true
}

func resolveExisting(path string) string {
	remainder := ""
	current := path
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(resolved, remainder)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

func resolvePath(cwd string, path string) string {
	if path == "" {
		path = cwd
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	return resolveExisting(filepath.Clean(path))
}
