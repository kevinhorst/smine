package fsx

import (
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithFileLock(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErr       error
		_expectedMaxInside int32
		_expectedRuns      int32

		bodyErr error
		callers int
		path    string
	}

	bodyFailed := errors.New("body failed")

	tests := make([]*testCase, 0)

	// body-error-propagates
	tests = append(tests, &testCase{
		_id:                "body-error-propagates",
		_expectedErr:       bodyFailed,
		_expectedMaxInside: 1,
		_expectedRuns:      1,

		bodyErr: bodyFailed,
		callers: 1,
		path:    filepath.Join(t.TempDir(), "record.json"),
	})

	// concurrent-bodies-serialize
	tests = append(tests, &testCase{
		_id:                "concurrent-bodies-serialize",
		_expectedErr:       nil,
		_expectedMaxInside: 1,
		_expectedRuns:      8,

		bodyErr: nil,
		callers: 8,
		path:    filepath.Join(t.TempDir(), "record.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			runs, maxInside, err := runLockedBodies(test.bodyErr, test.callers, test.path)

			assert.ErrorIs(t, err, test._expectedErr)
			assert.Equal(t, test._expectedRuns, runs)
			assert.Equal(t, test._expectedMaxInside, maxInside)
		})
	}
}

// runLockedBodies runs callers concurrent WithFileLock calls on path and reports the runs, the peak overlap and every error joined.
func runLockedBodies(bodyErr error, callers int, path string) (int32, int32, error) {
	var inside, maxInside, runs atomic.Int32
	body := func() error {
		current := inside.Add(1)
		for {
			peak := maxInside.Load()
			if current <= peak || maxInside.CompareAndSwap(peak, current) {
				break
			}
		}
		runs.Add(1)
		time.Sleep(time.Millisecond)
		inside.Add(-1)
		return bodyErr
	}

	errorChan := make(chan error, callers)
	var group sync.WaitGroup
	for range callers {
		group.Go(func() {
			errorChan <- WithFileLock(path, body)
		})
	}

	group.Wait()
	close(errorChan)

	var errs []error
	for err := range errorChan {
		errs = append(errs, err)
	}
	return runs.Load(), maxInside.Load(), errors.Join(errs...)
}
