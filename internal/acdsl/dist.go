package acdsl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

// DistHeader marks a synced gate file — baseline-owned, overwritten on
// every sync; target-owned rules live in any other *.acdsl file and
// registry overrides in acdsl/registry.local.json.
const DistHeader = "# synced from smine — do not edit; add repo-owned rules in any *.acdsl file, registry overrides in acdsl/registry.local.json"

// verifierBinDir is where shipped verifier binaries land in a target,
// relative to its root — the rewritten registry argv points here.
const verifierBinDir = "bin/verifiers"

// verifierArgvRe matches smine's own go-run argv package path — the only
// shape the rewrite understands; anything else ships verbatim.
var verifierArgvRe = regexp.MustCompile(`^\./cmd/acdsl/verifiers/[a-z0-9]+$`)

// exhaustiveToolNote documents the one shipped verifier that needs a
// target-side toolchain dependency (DEC-7): the binary runs go tool
// exhaustive against the target module.
const exhaustiveToolNote = " — requires the exhaustive tool dependency in the target go.mod (go tool exhaustive), or override in registry.local.json"

// Dist writes a target's gate slice into dest: the reach-covered doctrine
// rules as dest/acdsl/rules.acdsl, the registry subset those rules name
// (argv rewritten to shipped binaries), and the prebuilt bin/acdsl plus one
// binary per rewritten entry. No rule reaching target writes nothing.
// includeTask additionally ships task-lifetime rules; doctrine always ships.
func Dist(ctx context.Context, root, target, dest string, includeTask bool) ([]string, error) {
	discovery, err := DiscoverRules(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("Dist: %w", err)
	}
	if len(discovery.Violations) > 0 {
		return nil, fmt.Errorf("Dist: %d authoring violation(s) — fix before distributing", len(discovery.Violations))
	}
	candidates, err := FilterLifetime(discovery.Rules, LifetimeDoctrine)
	if err != nil {
		return nil, fmt.Errorf("Dist: %w", err)
	}
	if includeTask {
		task, err := FilterLifetime(discovery.Rules, LifetimeTask)
		if err != nil {
			return nil, fmt.Errorf("Dist: %w", err)
		}
		candidates = append(candidates, task...)
	}
	var covered []Rule
	for _, rule := range candidates {
		if reach.DeploysTo(rule.Reach, target) {
			covered = append(covered, rule)
		}
	}
	if len(covered) == 0 {
		return []string{fmt.Sprintf("no rules reach %s", target)}, nil
	}

	// The no-match contract stands everywhere: a doctrine anchor matching
	// nothing is tool breakage. A rule ships only when the target has files
	// it governs; skipped rules land on a later re-sync once files exist.
	destUniverse, err := FileUniverse(ctx, dest)
	if err != nil {
		return nil, fmt.Errorf("Dist: %w", err)
	}
	var shipped []Rule
	var skipped []string
	for _, rule := range covered {
		if _, matchErr := ResolveAnchor(destUniverse, rule); matchErr != nil {
			skipped = append(skipped, fmt.Sprintf("  skipped %s — no matching files in %s (re-sync picks it up)", rule.Id, target))
			continue
		}
		shipped = append(shipped, rule)
	}
	if len(shipped) == 0 {
		return append(skipped, fmt.Sprintf("no reaching rule matches files in %s — nothing shipped", target)), nil
	}

	registry, err := LoadRegistry(filepath.Join(root, "acdsl", "registry.json"))
	if err != nil {
		return nil, fmt.Errorf("Dist: %w", err)
	}

	if err := writeRules(root, dest, shipped); err != nil {
		return nil, err
	}
	built, err := writeDistRegistry(dest, shipped, registry)
	if err != nil {
		return nil, err
	}
	if err := buildBinaries(ctx, root, dest, built); err != nil {
		return nil, err
	}
	distMode, err := writePolicy(root, dest)
	if err != nil {
		return nil, err
	}

	lines := skipped
	lines = append(lines,
		fmt.Sprintf("  acdsl/rules.acdsl -> %d rule(s) reach %s", len(shipped), target),
		fmt.Sprintf("  acdsl/registry.json -> %d verifier contract(s)", len(built)+countVerbatim(shipped, registry, built)),
		"  bin/acdsl -> check/project/fixtures runner",
	)
	if distMode != "" {
		lines = append(lines, fmt.Sprintf("  acdsl/policy.json -> mode: %s", distMode))
	}
	names := make([]string, 0, len(built))
	for name := range built {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("  %s/%s -> verifier binary", verifierBinDir, name))
	}
	return lines, nil
}

// countVerbatim counts shipped registry entries that were not rewritten —
// argv outside the go-run shape ships verbatim and builds nothing.
func countVerbatim(shipped []Rule, registry map[string]RegistryEntry, built map[string]string) int {
	seen := map[string]bool{}
	count := 0
	for _, rule := range shipped {
		if seen[rule.Verifier] {
			continue
		}
		seen[rule.Verifier] = true
		if _, isBuilt := built[rule.Verifier]; !isBuilt {
			if _, exists := registry[rule.Verifier]; exists {
				count++
			}
		}
	}
	return count
}

// writeRules renders DistHeader plus the shipped rules' original marker
// lines (verbatim from their declaration files) into dest/acdsl/rules.acdsl.
// A pre-existing unheadered file is target-owned — refuse to overwrite it.
func writeRules(root, dest string, shipped []Rule) error {
	dst := filepath.Join(dest, "acdsl", "rules.acdsl")
	if existing, err := os.ReadFile(dst); err == nil {
		headline, _, _ := strings.Cut(string(existing), "\n")
		if !strings.Contains(headline, "synced from smine") {
			return fmt.Errorf("writeRules: %s is repo-owned (no sync header) — refusing to overwrite", dst)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("writeRules: %w", err)
	}

	var builder strings.Builder
	builder.WriteString(DistHeader + "\n\n")
	for _, rule := range shipped {
		line, err := markerLine(root, rule)
		if err != nil {
			return err
		}
		builder.WriteString(line + "\n")
	}
	if err := os.WriteFile(dst, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("writeRules: %w", err)
	}
	return nil
}

// markerLine reads a rule's original declaration line — shipped byte-
// identical so reach labels, params, and whys survive verbatim.
func markerLine(root string, rule Rule) (string, error) {
	raw, err := os.ReadFile(filepath.Join(root, rule.File))
	if err != nil {
		return "", fmt.Errorf("markerLine: %s: %w", rule.Id, err)
	}
	lines := strings.Split(string(raw), "\n")
	if rule.Line < 1 || rule.Line > len(lines) {
		return "", fmt.Errorf("markerLine: %s: line %d out of range in %s", rule.Id, rule.Line, rule.File)
	}
	return strings.TrimSpace(lines[rule.Line-1]), nil
}

// writeDistRegistry writes the subset registry into dest/acdsl/registry.json:
// every verifier a shipped rule names. Argv matching the go-run shape is
// rewritten to the shipped binary path and returned in built (name → source
// package); anything else ships verbatim.
func writeDistRegistry(dest string, shipped []Rule, registry map[string]RegistryEntry) (map[string]string, error) {
	subset := map[string]RegistryEntry{}
	built := map[string]string{}
	for _, rule := range shipped {
		entry, exists := registry[rule.Verifier]
		if !exists {
			return nil, fmt.Errorf("writeDistRegistry: %s: verifier %q not in registry", rule.Id, rule.Verifier)
		}
		if len(entry.Argv) == 3 && entry.Argv[0] == "go" && entry.Argv[1] == "run" && verifierArgvRe.MatchString(entry.Argv[2]) {
			built[rule.Verifier] = entry.Argv[2]
			entry.Argv = []string{verifierBinDir + "/" + rule.Verifier}
		}
		if rule.Verifier == "exhaustive-switch" && !strings.Contains(entry.Description, "exhaustive tool dependency") {
			entry.Description += exhaustiveToolNote
		}
		subset[rule.Verifier] = entry
	}

	data, err := json.MarshalIndent(subset, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("writeDistRegistry: %w", err)
	}
	dst := filepath.Join(dest, "acdsl", "registry.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("writeDistRegistry: %w", err)
	}
	if err := os.WriteFile(dst, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("writeDistRegistry: %w", err)
	}
	return built, nil
}

// writePolicy ships the self-management policy into dest: mode rewritten
// from dist_mode (falling back to the source mode), dist_mode stripped — a
// target never re-distributes. The schema ships verbatim beside it, and both
// copies are baseline-owned like the registry subset. No source policy ships
// nothing and returns "".
func writePolicy(root, dest string) (PolicyMode, error) {
	policy, err := LoadPolicy(root)
	if err != nil {
		return "", fmt.Errorf("writePolicy: %w", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "acdsl", "policy.json")); statErr != nil {
		return "", nil
	}
	if policy.DistMode != "" {
		policy.Mode = policy.DistMode
	}
	policy.DistMode = ""
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return "", fmt.Errorf("writePolicy: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "acdsl", "policy.json"), append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writePolicy: %w", err)
	}
	schemaSrc := filepath.Join(root, "acdsl", "policy.schema.json")
	if _, statErr := os.Stat(schemaSrc); statErr == nil {
		if err := fsx.CopyFile(schemaSrc, filepath.Join(dest, "acdsl", "policy.schema.json")); err != nil {
			return "", fmt.Errorf("writePolicy: %w", err)
		}
	}
	return policy.Mode, nil
}

// buildBinaries ships the runner and every rewritten entry's binary into the
// target: bin/acdsl plus bin/verifiers/<name>. A prebuilt exe in the smine
// repo's bin/ (Windows installer payload) is copied; otherwise go build —
// dev machines always build fresh.
func buildBinaries(ctx context.Context, root, dest string, built map[string]string) error {
	if err := os.MkdirAll(filepath.Join(dest, verifierBinDir), 0o755); err != nil {
		return fmt.Errorf("buildBinaries: %w", err)
	}
	if err := shipBinary(ctx, root, filepath.Join(root, "bin"), filepath.Join(dest, "bin"), "acdsl", "./cmd/acdsl"); err != nil {
		return fmt.Errorf("buildBinaries: %w", err)
	}
	for name, pkg := range built {
		if err := shipBinary(ctx, root, filepath.Join(root, "bin", "verifiers"), filepath.Join(dest, verifierBinDir), name, pkg); err != nil {
			return fmt.Errorf("buildBinaries: %w", err)
		}
	}
	return nil
}

// shipBinary copies srcDir/name(.exe) to destDir when a prebuilt exists,
// else go-builds pkg there. Windows outputs always carry .exe so CreateProcess
// and Git Bash agree the file is executable.
func shipBinary(ctx context.Context, root, srcDir, destDir, name, pkg string) error {
	out := name
	if runtime.GOOS == "windows" {
		out = name + ".exe"
	}
	for _, prebuilt := range []string{filepath.Join(srcDir, name+".exe"), filepath.Join(srcDir, name)} {
		if info, err := os.Stat(prebuilt); err == nil && !info.IsDir() {
			if err := fsx.CopyFile(prebuilt, filepath.Join(destDir, filepath.Base(prebuilt))); err != nil {
				return fmt.Errorf("shipBinary: %s: %w", name, err)
			}
			return nil
		}
	}
	if output, err := shell.Run(ctx, root, "go", "build", "-o", filepath.Join(destDir, out), pkg); err != nil {
		return fmt.Errorf("shipBinary: %s: %w\n%s", name, err, output)
	}
	return nil
}
