//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// quarantineAttr is the flag macOS puts on a download and on everything extracted from it; Gatekeeper kills an unsigned binary that carries it.
const quarantineAttr = "com.apple.quarantine"

// clearQuarantine drops the flag from every file under the install dir, since launchd and the installer run the unsigned bin/ binaries in place.
func clearQuarantine(out io.Writer, repoDir string) error {
	cleared := 0
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		err := unix.Lremovexattr(path, quarantineAttr)
		if errors.Is(err, unix.ENOATTR) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		cleared++
		return nil
	}
	if err := filepath.WalkDir(repoDir, walk); err != nil {
		return fmt.Errorf("clearQuarantine: %w", err)
	}

	if cleared > 0 {
		fmt.Fprintf(out, "-> Cleared the macOS quarantine flag from %d files under %s\n", cleared, repoDir)
	}
	return nil
}
