package routinerun

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strconv"
)

// PreventSleep holds `caffeinate -is -w <own pid>` until the returned release runs.
func PreventSleep() func() {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "caffeinate", "-is", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		log.Printf("PreventSleep: Failed to start caffeinate: %v", err)
		cancel()
		return func() {}
	}

	return func() {
		cancel()
		_ = cmd.Wait()
	}
}
