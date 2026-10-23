//go:build !darwin && !windows

package routinerun

// PreventSleep has no sleep management to suppress off darwin/windows.
func PreventSleep() func() {
	return func() {}
}
