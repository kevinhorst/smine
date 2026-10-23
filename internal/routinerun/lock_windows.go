package routinerun

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

const lockPoll = 10 * time.Second

// acquireLock relies on the OS releasing the LockFileEx lock on process death.
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
		overlapped := new(windows.Overlapped)
		err = windows.LockFileEx(
			windows.Handle(file.Fd()),
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0,
			1,
			0,
			overlapped,
		)
		if err == nil {
			return file, true, nil
		}
		if !time.Now().Before(deadline) {
			file.Close()
			return nil, false, nil
		}
		time.Sleep(lockPoll)
	}
}
