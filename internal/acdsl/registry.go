package acdsl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// RegistryEntry is one predefined verifier: the only form computation may
// take in ACDSL. Rules reference entries by name and can never carry code.
type RegistryEntry struct {
	Argv        []string `json:"argv"`
	TimeoutSec  int      `json:"timeout_s"`
	Description string   `json:"description"`
}

func (e RegistryEntry) Timeout() time.Duration { return time.Duration(e.TimeoutSec) * time.Second }

// LocalRegistryName is the target-owned override file beside the baseline
// registry — never synced; entries replace baseline entries by name, so a
// target re-points any verifier contract at its own implementation.
const LocalRegistryName = "registry.local.json"

// LoadRegistry reads and validates the committed registry file. A sibling
// registry.local.json overlays it, local entries winning by name.
func LoadRegistry(path string) (map[string]RegistryEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadRegistry: %w", err)
	}
	var registry map[string]RegistryEntry
	if err := json.Unmarshal(raw, &registry); err != nil {
		return nil, fmt.Errorf("LoadRegistry: %s: %w", path, err)
	}
	localPath := filepath.Join(filepath.Dir(path), LocalRegistryName)
	if localRaw, localErr := os.ReadFile(localPath); localErr == nil {
		var local map[string]RegistryEntry
		if err := json.Unmarshal(localRaw, &local); err != nil {
			return nil, fmt.Errorf("LoadRegistry: %s: %w", localPath, err)
		}
		for name, entry := range local {
			registry[name] = entry
		}
	}
	for name, entry := range registry {
		if len(entry.Argv) == 0 {
			return nil, fmt.Errorf("LoadRegistry: %s: %s: empty argv", path, name)
		}
		if entry.TimeoutSec <= 0 || entry.TimeoutSec > 60 {
			return nil, fmt.Errorf("LoadRegistry: %s: %s: timeout_s must be 1..60 (shell.Run ceiling)", path, name)
		}
	}
	return registry, nil
}

// resolveVerifierBinary substitutes a prebuilt bin/verifiers binary for the
// stock go-run argv at spawn time — every go-run spawn pays the toolchain's
// cache check and link, which dominates a gate run. Substitution happens at
// execution only: LoadRegistry stays pure so Dist ships the pristine argv.
// The entry is returned untouched when no binary exists, so a source-only
// checkout keeps working.
func resolveVerifierBinary(root string, entry RegistryEntry) RegistryEntry {
	isStockGoRun := len(entry.Argv) == 3 && entry.Argv[0] == "go" && entry.Argv[1] == "run"
	if !isStockGoRun || !verifierArgvRe.MatchString(entry.Argv[2]) {
		return entry
	}
	// go build -o <dir> names each binary after its package, not the
	// registry name (skill-context runs package skillcontext).
	binary := filepath.Join(root, "bin", "verifiers", filepath.Base(entry.Argv[2]))
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if _, err := os.Stat(binary); err != nil {
		return entry
	}
	entry.Argv = []string{binary}
	return entry
}
