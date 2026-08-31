package acdsl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/shell"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot is the module root — dist tests run against the real rule set so
// the shipped slice, subset, and rewrite are pinned to production data.
const repoRoot = "../.."

// newDistDest materializes a minimal target: a git repo with one Go file,
// so .go-anchored global rules match and ship.
func newDistDest(t *testing.T) string {
	t.Helper()
	dest := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dest, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), dest, "git", args...)
		require.NoError(t, err)
	}
	return dest
}

func TestDistShipsReachCoveredSliceWithBinaries(t *testing.T) {
	dest := newDistDest(t)
	lines, err := Dist(context.Background(), repoRoot, "aqms", dest, false)
	require.NoError(t, err)
	require.NotEmpty(t, lines)

	// target-match filter: no .sh files in dest, so SHELL-002 is skipped
	assert.Contains(t, strings.Join(lines, "\n"), "skipped ACDSL-SHELL-002")

	// rules: header + only reach-covered (global) rules, marker lines verbatim
	rulesRaw, err := os.ReadFile(filepath.Join(dest, "acdsl", "rules.acdsl"))
	require.NoError(t, err)
	shippedRules := string(rulesRaw)
	assert.True(t, strings.HasPrefix(shippedRules, DistHeader))
	assert.Contains(t, shippedRules, "ACDSL-GOLANG-FMT-001")
	assert.NotContains(t, shippedRules, "ACDSL-GOLANG-STATE-001") // reach smine, never ships
	assert.NotContains(t, shippedRules, "ACDSL-SHELL-002")        // reaches, but nothing to govern in dest
	sourceRaw, err := os.ReadFile(filepath.Join(repoRoot, "acdsl", "rules.acdsl"))
	require.NoError(t, err)
	for _, line := range strings.Split(shippedRules, "\n") {
		if strings.HasPrefix(line, "//acdsl:") {
			assert.Contains(t, string(sourceRaw), line, "shipped marker line must be byte-identical to source")
		}
	}

	// registry: subset of referenced names, go-run argv rewritten
	registryRaw, err := os.ReadFile(filepath.Join(dest, "acdsl", "registry.json"))
	require.NoError(t, err)
	var subset map[string]RegistryEntry
	require.NoError(t, json.Unmarshal(registryRaw, &subset))
	require.Contains(t, subset, "gofmt")
	assert.Equal(t, []string{"bin/verifiers/gofmt"}, subset["gofmt"].Argv)
	assert.NotContains(t, subset, "rules-check") // only smine-reach rules name it
	if entry, exists := subset["exhaustive-switch"]; exists {
		assert.Contains(t, entry.Description, "exhaustive tool dependency")
	}

	// binaries: runner + one per rewritten entry
	runner, err := os.Stat(filepath.Join(dest, "bin", "acdsl"))
	require.NoError(t, err)
	assert.NotZero(t, runner.Mode()&0o111)
	gofmtBin, err := os.Stat(filepath.Join(dest, "bin", "verifiers", "gofmt"))
	require.NoError(t, err)
	assert.NotZero(t, gofmtBin.Mode()&0o111)
}

func TestDistNoCoverageWritesNothing(t *testing.T) {
	// A root whose only rule keeps the smine default never reaches a target.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "rules.acdsl"),
		[]byte("//"+`acdsl:X-001 gofmt anchor="\.go$" why="local only"`+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "registry.json"),
		[]byte(`{"gofmt": {"argv": ["true"], "timeout_s": 5, "description": "noop"}}`), 0o644))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), root, "git", args...)
		require.NoError(t, err)
	}

	dest := t.TempDir()
	lines, err := Dist(context.Background(), root, "aqms", dest, false)
	require.NoError(t, err)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "no rules reach")
	_, statErr := os.Stat(filepath.Join(dest, "acdsl"))
	assert.True(t, os.IsNotExist(statErr))
}

// TestDistTaskLifetime pins the sync-task opt-in: a reach-covered task rule
// ships only with includeTask, doctrine ships either way.
func TestDistTaskLifetime(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	rules := "//" + `acdsl:ACDSL-GOLANG-FMT-001 gofmt reach="global" anchor="\.go$" why="doctrine rule"` + "\n" +
		"//" + `acdsl:ACDSL-GOLANG-FMT-002 gofmt reach="global" lifetime="task" anchor="\.go$" why="task contract"` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "rules.acdsl"), []byte(rules), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "registry.json"),
		[]byte(`{"gofmt": {"argv": ["true"], "timeout_s": 5, "description": "noop"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644))
	// Prebuilt runner so Dist never needs a Go module in the synthetic root.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin", "acdsl"), []byte("prebuilt\n"), 0o755))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), root, "git", args...)
		require.NoError(t, err)
	}

	// without includeTask the task rule stays home
	dest := newDistDest(t)
	_, err := Dist(context.Background(), root, "aqms", dest, false)
	require.NoError(t, err)
	shipped, err := os.ReadFile(filepath.Join(dest, "acdsl", "rules.acdsl"))
	require.NoError(t, err)
	assert.Contains(t, string(shipped), "ACDSL-GOLANG-FMT-001")
	assert.NotContains(t, string(shipped), "ACDSL-GOLANG-FMT-002")

	// with includeTask the reach-covered task rule ships too
	dest = newDistDest(t)
	_, err = Dist(context.Background(), root, "aqms", dest, true)
	require.NoError(t, err)
	shipped, err = os.ReadFile(filepath.Join(dest, "acdsl", "rules.acdsl"))
	require.NoError(t, err)
	assert.Contains(t, string(shipped), "ACDSL-GOLANG-FMT-001")
	assert.Contains(t, string(shipped), "ACDSL-GOLANG-FMT-002")
}

// TestShipBinary pins the prebuilt-first contract: an installer-shipped exe
// is copied verbatim (no toolchain), the go-build fallback covers dev
// machines, and .exe wins over an extension-less sibling.
func TestShipBinary(t *testing.T) {
	tests := []struct {
		name      string
		prebuilts map[string]string // relative name -> content
		wantFile  string            // file expected in destDir
		wantBuild bool              // go-build fallback expected
	}{
		{name: "prebuilt-copied-verbatim", prebuilts: map[string]string{"acdsl": "prebuilt-plain"}, wantFile: "acdsl"},
		{name: "exe-preferred-over-extensionless", prebuilts: map[string]string{"acdsl": "prebuilt-plain", "acdsl.exe": "prebuilt-exe"}, wantFile: "acdsl.exe"},
		{name: "absent-falls-back-to-go-build", prebuilts: nil, wantFile: "acdsl", wantBuild: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srcDir := t.TempDir()
			destDir := t.TempDir()
			for name, content := range test.prebuilts {
				require.NoError(t, os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0o755))
			}

			// Stub go on PATH: records the invocation, creates the -o file.
			stubDir := t.TempDir()
			invocations := filepath.Join(stubDir, "invoked")
			stub := "#!/bin/sh\necho \"$@\" >> " + invocations + "\n: > \"$3\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(stubDir, "go"), []byte(stub), 0o755))
			t.Setenv("PATH", stubDir)

			require.NoError(t, shipBinary(context.Background(), srcDir, srcDir, destDir, "acdsl", "./cmd/acdsl"))

			shipped, err := os.ReadFile(filepath.Join(destDir, test.wantFile))
			require.NoError(t, err)
			_, invokedErr := os.Stat(invocations)
			if test.wantBuild {
				assert.NoError(t, invokedErr, "go build fallback expected")
			} else {
				assert.True(t, os.IsNotExist(invokedErr), "prebuilt present — go must not run")
				assert.Equal(t, test.prebuilts[test.wantFile], string(shipped))
			}
		})
	}
}

func TestDistRefusesRepoOwnedRulesFile(t *testing.T) {
	dest := newDistDest(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "acdsl"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "acdsl", "rules.acdsl"),
		[]byte("# my own rules\n"), 0o644))
	_, err := Dist(context.Background(), repoRoot, "aqms", dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo-owned")
}

func TestDistShipsPolicyModeRewritten(t *testing.T) {
	dest := newDistDest(t)
	lines, err := Dist(context.Background(), repoRoot, "aqms", dest, false)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(lines, "\n"), "acdsl/policy.json -> mode: gated")

	shipped, err := LoadPolicy(dest)
	require.NoError(t, err)
	assert.Equal(t, PolicyModeGated, shipped.Mode)
	assert.Empty(t, shipped.DistMode)

	_, err = os.Stat(filepath.Join(dest, "acdsl", "policy.schema.json"))
	assert.NoError(t, err)
}

func TestWritePolicyNoSourceShipsNothing(t *testing.T) {
	root := t.TempDir()
	dest := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "acdsl"), 0o755))

	mode, err := writePolicy(root, dest)

	require.NoError(t, err)
	assert.Empty(t, mode)
	_, statErr := os.Stat(filepath.Join(dest, "acdsl", "policy.json"))
	assert.True(t, os.IsNotExist(statErr))
}
