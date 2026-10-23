package acdsl

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Rules reference entries by name and never carry code.
type RegistryEntry struct {
	Argv        []string `json:"argv"`
	Cache       string   `json:"cache,omitempty"` // files | tree | none; absent = tree
	Description string   `json:"description"`
	Reads       []string `json:"reads,omitempty"` // directory prefixes ending in /, files scope only
	TimeoutSec  int      `json:"timeout_s"`
}

func (e RegistryEntry) Timeout() time.Duration { return time.Duration(e.TimeoutSec) * time.Second }

// Target-owned and never synced; its entries replace baseline entries by name.
const LocalRegistryName = "registry.local.json"

// MaxVerifierTimeout bounds timeout_s: a cold go tool exhaustive under CI's parallel audit outran the 60s shell.Run ceiling.
const MaxVerifierTimeout = 5 * time.Minute

// RegistryPath is the committed registry's path under a module root.
const RegistryPath = "acdsl/registry.json"

// Overlay entries win by name, so a railroad probe registry holds only its own entries.
func LoadRegistryOverlay(overlayPath, root string) (map[string]RegistryEntry, error) {
	basePath := filepath.Join(root, RegistryPath)
	registry, err := LoadRegistry(basePath)
	if err != nil {
		return nil, fmt.Errorf("LoadRegistryOverlay: %w", err)
	}

	isBase := overlayPath == "" || filepath.Clean(overlayPath) == filepath.Clean(basePath)
	if isBase {
		return registry, nil
	}

	overlay, err := LoadRegistry(overlayPath)
	if err != nil {
		return nil, fmt.Errorf("LoadRegistryOverlay: %w", err)
	}

	maps.Copy(registry, overlay)
	return registry, nil
}

// A sibling registry.local.json overlays it, local entries winning by name.
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
		if entry.TimeoutSec <= 0 || entry.Timeout() > MaxVerifierTimeout {
			return nil, fmt.Errorf("LoadRegistry: %s: %s: timeout_s must be 1..%d (verifier ceiling)", path, name, int(MaxVerifierTimeout.Seconds()))
		}
		if err := checkCacheScope(entry); err != nil {
			return nil, fmt.Errorf("LoadRegistry: %s: %s: %w", path, name, err)
		}
	}
	return registry, nil
}

// checkCacheScope refuses an unknown scope and reads outside the files scope or not shaped as a directory prefix.
func checkCacheScope(entry RegistryEntry) error {
	switch entry.Cache {
	case "", CacheFiles, CacheNone, CacheTree:
	default:
		return fmt.Errorf("checkCacheScope: Unknown cache scope %q", entry.Cache)
	}
	if len(entry.Reads) > 0 && entry.Cache != CacheFiles {
		return errors.New("checkCacheScope: reads needs cache files")
	}
	for _, prefix := range entry.Reads {
		if !strings.HasSuffix(prefix, "/") {
			return fmt.Errorf("checkCacheScope: reads entry %q is not a directory prefix", prefix)
		}
	}
	return nil
}

// Substituted at spawn only, so LoadRegistry stays pure and Dist ships the pristine go-run argv.
func resolveVerifierBinary(root string, entry RegistryEntry) RegistryEntry {
	isStockGoRun := len(entry.Argv) == 3 && entry.Argv[0] == "go" && entry.Argv[1] == "run"
	if !isStockGoRun || !verifierArgvRe.MatchString(entry.Argv[2]) {
		return entry
	}
	binary := filepath.Join(root, verifierBinDir, verifierBinaryName(entry.Argv[2]))
	if _, err := os.Stat(binary); err != nil {
		return entry
	}
	entry.Argv = []string{binary}
	return entry
}

// go build -o <dir> names each binary after its package dir, not the registry name.
func verifierBinaryName(pkg string) string {
	name := filepath.Base(pkg)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}
