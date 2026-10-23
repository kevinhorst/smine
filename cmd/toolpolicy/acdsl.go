package main

import (
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/hookconfig"
)

// acdslGuard is the write guard of the repo cwd sits in.
type acdslGuard struct {
	isTaskContractAllowed bool
	mode                  acdsl.PolicyMode
	root                  string
	surface               []string
}

func newAcdslGuard(cwd string) *acdslGuard {
	guard := &acdslGuard{mode: acdsl.PolicyModeFree}
	root, isFound := acdslPolicyRoot(cwd)
	if !isFound {
		return guard
	}
	policy, err := acdsl.LoadPolicy(root)
	if err != nil {
		return guard
	}

	guarded := acdsl.GuardedSurface()
	surface := make([]string, 0, len(guarded))
	for _, path := range guarded {
		surface = append(surface, filepath.Join(root, path))
	}
	guard.isTaskContractAllowed = policy.AllowTaskContracts
	guard.mode = policy.Mode
	guard.root = root
	guard.surface = surface
	return guard
}

func (g *acdslGuard) isRuleFile(cwd string, target string) bool {
	rulesDir := []string{filepath.Join(g.root, "acdsl")}
	if IsInsideRoots(cwd, target, rulesDir) {
		return true
	}
	isAcdslFile := filepath.Ext(target) == ".acdsl" && IsInsideRoots(cwd, target, []string{g.root})
	return isAcdslFile && !g.isTaskContractAllowed
}

// cause names why the guard denies a write to target; "" when it does not.
func (g *acdslGuard) cause(causes *hookconfig.ToolpolicyCauses, cwd string, target string) string {
	switch g.mode {
	case acdsl.PolicyModeFree:
		return ""
	case acdsl.PolicyModeGated:
		if IsInsideRoots(cwd, target, g.surface) {
			return causes.AcdslSurface
		}
		return ""
	case acdsl.PolicyModeStrict:
		if IsInsideRoots(cwd, target, g.surface) {
			return causes.AcdslSurface
		}
		if g.isRuleFile(cwd, target) {
			return causes.AcdslRuleFile
		}
		return ""
	}
	return ""
}

func acdslPolicyRoot(cwd string) (string, bool) {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "acdsl", "policy.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
