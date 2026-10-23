//go:build !darwin && !windows

package routines

import (
	"context"
	"errors"
)

// PauseSupported is false off darwin and windows.
const PauseSupported = false

// Pause is unsupported off darwin.
func Pause(ctx context.Context, label string) (int, error) {
	return 0, errors.New("Pause: Not supported on this platform")
}

// Resume is unsupported off darwin.
func Resume(ctx context.Context, label string) error {
	return errors.New("Resume: Not supported on this platform")
}
