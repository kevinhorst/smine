// Package fsx holds small filesystem helpers shared across packages.
package fsx

import (
	"fmt"
	"os"
	"time"
)

// CopyFile copies src to dst verbatim, executable (0o755) — every caller
// ships binaries or shims that must be runnable. Plain read/write, no
// rename dance: callers tolerate (and report) a locked destination.
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

// ReplaceFile renames tmp onto path. Go's Windows rename already replaces an
// existing destination; the retry absorbs transient sharing violations from
// antivirus/indexer holds. On unix the first attempt wins.
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
