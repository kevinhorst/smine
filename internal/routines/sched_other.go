//go:build !darwin && !windows

package routines

import (
	"context"
	"errors"
)

// errUnsupported degrades the scheduler surface on platforms without a
// backend (launchd on darwin, Task Scheduler on windows). Exists so the
// server package builds everywhere — CI runs the test suite on linux —
// while routine start/stop/run stay honest errors there.
var errUnsupported = errors.New("routines: no scheduler backend on this platform")

// IsLoaded reports false everywhere: nothing can be loaded without a
// scheduler backend.
func IsLoaded(ctx context.Context, label string) (bool, error) {
	return false, nil
}

// RunNow is unsupported without a scheduler backend.
func RunNow(ctx context.Context, label string) (string, error) {
	return "", errUnsupported
}

// Start is unsupported without a scheduler backend.
func Start(ctx context.Context, label, plistPath string) (string, error) {
	return "", errUnsupported
}

// Stop is unsupported without a scheduler backend.
func Stop(ctx context.Context, label string) (string, error) {
	return "", errUnsupported
}

// SyncAll is a no-op without a scheduler backend; the server still starts.
func SyncAll(ctx context.Context, dir string) {}
