//go:build !windows

package routinerun

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const lockPoll = 10 * time.Second

// acquireLock relies on the kernel releasing the flock when the process dies.
func acquireLock(path string, wait time.Duration) (*os.File, bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, false, fmt.Errorf("acquireLock: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("acquireLock: %w", err)
	}

	deadline := time.Now().Add(wait)
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return file, true, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			file.Close()
			return nil, false, fmt.Errorf("acquireLock: Failed to lock %s: %w", path, err)
		}
		if !time.Now().Before(deadline) {
			file.Close()
			return nil, false, nil
		}
		time.Sleep(lockPoll)
	}
}
