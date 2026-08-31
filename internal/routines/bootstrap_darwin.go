package routines

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// SyncAll loads every non-degraded routine that is not already loaded
// and not disabled in launchd's per-service override store; a routine with a
// default-disabled marker additionally needs an explicit enabled override
// (the config server's Start writes it) before it is ever bootstrapped.
// Called once at
// server startup: a logout tears down the gui domain and unloads every
// routine, and the configserver LaunchAgent restarting at login is the
// re-bootstrap trigger (D1). Per-routine failures are logged, never fatal
// (D7). Windows implements the same name as a registration reconcile
// (sched_windows.go) — registrations persist there, the gui domain does not.
func SyncAll(ctx context.Context, dir string) {
	list, err := Scan(dir)
	if err != nil {
		log.Printf("routines: bootstrap scan failed: %v", err)
		return
	}

	output, err := shell.Run(ctx, "", "launchctl", "print-disabled", fmt.Sprintf("gui/%d", os.Getuid()))
	if err != nil {
		log.Printf("routines: print-disabled failed, assuming none disabled: %v", err)
	}
	overrides := overrideLabels(output)

	for index := range list {
		routine := &list[index]
		if routine.LoadError != "" {
			continue
		}
		if overrides[routine.Label] == "disabled" {
			log.Printf("routines: %s disabled, not bootstrapping", routine.Name)
			continue
		}
		if routine.DefaultDisabled && overrides[routine.Label] != "enabled" {
			log.Printf("routines: %s default-disabled, awaiting explicit enable", routine.Name)
			continue
		}
		loaded, err := IsLoaded(ctx, routine.Label)
		if err != nil {
			log.Printf("routines: %s probe failed: %v", routine.Name, err)
			continue
		}
		if loaded {
			continue
		}
		if _, err := Start(ctx, routine.Label, routine.PlistPath); err != nil {
			log.Printf("routines: %s bootstrap failed: %v", routine.Name, err)
			continue
		}
		log.Printf("routines: %s bootstrapped", routine.Name)
	}
}

// overrideLabels parses `launchctl print-disabled` output — lines of the form
// `"<label>" => disabled` or `"<label>" => enabled` — into label → state.
// Absent labels have no explicit override. The output is not a stable API
// (same caveat as IsLoaded, D23); anything unmatched carries no entry (D8).
func overrideLabels(output string) map[string]string {
	overrides := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), `"`)
		if !found {
			continue
		}
		label, rest, found := strings.Cut(rest, `"`)
		if !found {
			continue
		}
		state, found := strings.CutPrefix(strings.TrimSpace(rest), "=> ")
		if !found || (state != "enabled" && state != "disabled") {
			continue
		}
		overrides[label] = state
	}
	return overrides
}
