//go:build !windows

package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// WithFileLock runs body while holding an exclusive advisory lock on path+".lock"; a concurrent caller waits for it.
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

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("WithFileLock: Failed to lock %s: %w", lockPath, err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return body()
}
