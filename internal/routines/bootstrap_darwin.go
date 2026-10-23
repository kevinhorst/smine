package routines

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// SyncAll runs at server startup because a logout unloads every routine from the gui domain.
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

// overrideLabels drops unmatched lines: `launchctl print-disabled` output is not a stable API.
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
		isKnownState := state == "enabled" || state == "disabled"
		if !found || !isKnownState {
			continue
		}
		overrides[label] = state
	}
	return overrides
}
