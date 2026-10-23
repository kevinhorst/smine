//go:build !darwin && !windows

package routines

import (
	"context"
	"errors"
)

// errUnsupported lets the server build on linux, where CI runs, while start/stop/run stay honest errors.
var errUnsupported = errors.New("routines: no scheduler backend on this platform")

func IsLoaded(ctx context.Context, label string) (bool, error) {
	return false, nil
}

func IsRunning(ctx context.Context, label string) (bool, error) {
	return false, nil
}

// RegisterLogonTask and StopTask are Task Scheduler verbs; there is no scheduler backend here.
func RegisterLogonTask(ctx context.Context, label, xml string) (string, error) {
	return "", errUnsupported
}

func StopTask(ctx context.Context, label string) (string, error) {
	return "", errUnsupported
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

// Unregister has nothing to remove without a scheduler backend.
func Unregister(ctx context.Context, label string) (string, error) {
	return "", nil
}
