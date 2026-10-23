//go:build windows

package fsx

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// WithFileLock runs body while holding an exclusive lock on path+".lock"; a concurrent caller waits for it.
func WithFileLock(path string, body func() error) error {
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("WithFileLock: Failed to create %s: %w", filepath.Dir(lockPath), err)
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("WithFileLock: Failed to open %s: %w", lockPath, err)
	}
	defer file.Close()

	handle := windows.Handle(file.Fd())
	overlapped := &windows.Overlapped{}
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, overlapped); err != nil {
		return fmt.Errorf("WithFileLock: Failed to lock %s: %w", lockPath, err)
	}
	defer windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
	return body()
}
