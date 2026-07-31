package routines

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// BootstrapAll loads every non-degraded routine that is not already loaded
// and not disabled in launchd's per-service override store. Called once at
// server startup: a logout tears down the gui domain and unloads every
// routine, and the configserver LaunchAgent restarting at login is the
// re-bootstrap trigger (D1). Per-routine failures are logged, never fatal (D7).
func BootstrapAll(ctx context.Context, dir string) {
	list, err := Scan(dir)
	if err != nil {
		log.Printf("routines: bootstrap scan failed: %v", err)
		return
	}

	output, err := shell.Run(ctx, "", "launchctl", "print-disabled", fmt.Sprintf("gui/%d", os.Getuid()))
	if err != nil {
		log.Printf("routines: print-disabled failed, assuming none disabled: %v", err)
	}
	disabled := disabledLabels(output)

	for index := range list {
		routine := &list[index]
		if routine.LoadError != "" {
			continue
		}
		if disabled[routine.Label] {
			log.Printf("routines: %s disabled, not bootstrapping", routine.Name)
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

// disabledLabels parses `launchctl print-disabled` output — lines of the form
// `"<label>" => disabled`. Absent labels default to enabled. The output is
// not a stable API (same caveat as IsLoaded, D23); anything unmatched is
// treated as enabled (D8).
func disabledLabels(output string) map[string]bool {
	disabled := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), `"`)
		if !found {
			continue
		}
		label, rest, found := strings.Cut(rest, `"`)
		if !found || strings.TrimSpace(rest) != "=> disabled" {
			continue
		}
		disabled[label] = true
	}
	return disabled
}
