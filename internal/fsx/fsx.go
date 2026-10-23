// Package fsx holds small filesystem helpers shared across packages.
package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Always 0o755: every caller ships binaries or shims; callers tolerate a locked destination.
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("CopyFile: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		return fmt.Errorf("CopyFile: %w", err)
	}
	return nil
}

// HasContent reports a file that exists and is non-empty.
func HasContent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// The retry absorbs transient Windows sharing violations from antivirus or indexer holds.
func ReplaceFile(tmp, path string) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = os.Rename(tmp, path); err == nil {
			return nil
		}
		time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
	}
	return fmt.Errorf("ReplaceFile: %w", err)
}

// A reader sees the old or the new content, never half of it.
func WriteFileAtomic(data []byte, path string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("WriteFileAtomic: Failed to create %s: %w", dir, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("WriteFileAtomic: Failed to write %s: %w", tmp, err)
	}

	if err := ReplaceFile(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("WriteFileAtomic: Failed to replace %s: %w", path, err)
	}
	return nil
}
