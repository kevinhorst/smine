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

// distTarget is the repo name the fixture rules reach.
const distTarget = "target-repo"

// distRules holds one rule per dist outcome, assembled so no source line starts with the live marker.
var distRules = "//" + `acdsl:DIST-001 gofmt reach="target-repo" anchor="\.go$" why="reaches and matches"` + "\n" +
	"//" + `acdsl:DIST-002 rules-check anchor="\.go$" why="default reach stays home"` + "\n" +
	"//" + `acdsl:DIST-003 gofmt reach="global" anchor="\.sh$" why="reaches with nothing to govern"` + "\n" +
	"//" + `acdsl:DIST-004 exhaustive-switch reach="global" anchor="\.go$" why="description note"` + "\n"

// newDistRoot materializes a distributing repo with the given rules, a registry for every fixture verifier and prebuilt binaries, so Dist never builds.
func newDistRoot(t *testing.T, rules string) string {
	t.Helper()
	root := t.TempDir()
	registry := `{
  "exhaustive-switch": {"argv": ["true"], "timeout_s": 5, "description": "noop"},
  "gofmt": {"argv": ["go", "run", "./cmd/acdsl/verifiers/gofmtwrap"], "timeout_s": 5, "description": "noop"},
  "rules-check": {"argv": ["true"], "timeout_s": 5, "description": "noop"}
}
`
	files := map[string]string{
		"acdsl/registry.json": registry,
		"acdsl/rules.acdsl":   rules,
		"bin/acdsl":           "prebuilt runner\n",
		"bin/verifiers/gofmt": "prebuilt gofmt\n",
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), root, "git", args...)
		require.NoError(t, err)
	}
	return root
}

// newDistDest materializes a minimal target: a git repo with one Go file under cmd/,
// so the Go-anchored fixture rules match.
func newDistDest(t *testing.T) string {
	t.Helper()
	dest := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "cmd", "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "cmd", "app", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		_, err := shell.Run(context.Background(), dest, "git", args...)
		require.NoError(t, err)
	}
	return dest
}

func TestDistShipsReachCoveredSliceWithBinaries(t *testing.T) {
	root := newDistRoot(t, distRules)
	dest := newDistDest(t)
	lines, err := Dist(context.Background(), root, distTarget, dest, false)
	require.NoError(t, err)
	require.NotEmpty(t, lines)

	// target-match filter: no .sh files in dest, so DIST-003 is skipped
	assert.Contains(t, strings.Join(lines, "\n"), "skipped DIST-003")

	// rules: header + only reach-covered rules, marker lines verbatim
	rulesRaw, err := os.ReadFile(filepath.Join(dest, "acdsl", "rules.acdsl"))
	require.NoError(t, err)
	shippedRules := string(rulesRaw)
	assert.True(t, strings.HasPrefix(shippedRules, DistHeader))
	assert.Contains(t, shippedRules, "DIST-001")
	assert.Contains(t, shippedRules, "DIST-004")
	assert.NotContains(t, shippedRules, "DIST-002") // matches cmd/app/main.go, default reach, never ships
	assert.NotContains(t, shippedRules, "DIST-003") // reaches, but nothing to govern in dest
	for _, line := range strings.Split(shippedRules, "\n") {
		if strings.HasPrefix(line, "//acdsl:") {
			assert.Contains(t, distRules, line, "shipped marker line must be byte-identical to source")
		}
	}

	// registry: subset of referenced names, go-run argv rewritten
	registryRaw, err := os.ReadFile(filepath.Join(dest, "acdsl", "registry.json"))
	require.NoError(t, err)
	var subset map[string]RegistryEntry
	require.NoError(t, json.Unmarshal(registryRaw, &subset))
	require.Contains(t, subset, "gofmt")
	assert.Equal(t, []string{"bin/verifiers/gofmt"}, subset["gofmt"].Argv)
	assert.NotContains(t, subset, "rules-check") // only the home-only rule names it
	assert.Contains(t, subset["exhaustive-switch"].Description, "exhaustive tool dependency")

	// binaries: the prebuilt runner and the prebuilt verifier of the rewritten entry
	runner, err := os.ReadFile(filepath.Join(dest, "bin", "acdsl"))
	require.NoError(t, err)
	assert.Equal(t, "prebuilt runner\n", string(runner))
	gofmtBin, err := os.ReadFile(filepath.Join(dest, "bin", "verifiers", "gofmt"))
	require.NoError(t, err)
	assert.Equal(t, "prebuilt gofmt\n", string(gofmtBin))
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
		pkg       string            // package shipped under the registry name
		prebuilts map[string]string // relative name -> content
		wantFile  string            // file expected in destDir
		wantBuild bool              // go-build fallback expected
		wantFrom  string            // prebuilt whose content lands in wantFile; "" = wantFile itself
	}{
		{name: "prebuilt-copied-verbatim", pkg: "./cmd/acdsl", prebuilts: map[string]string{"acdsl": "prebuilt-plain"}, wantFile: "acdsl"},
		{name: "exe-preferred-over-extensionless", pkg: "./cmd/acdsl", prebuilts: map[string]string{"acdsl": "prebuilt-plain", "acdsl.exe": "prebuilt-exe"}, wantFile: "acdsl.exe"},
		{name: "built-by-package-dir-copied", pkg: "./cmd/acdsl/verifiers/acdslwrap", prebuilts: map[string]string{"acdslwrap": "built-by-make"}, wantFile: "acdsl", wantFrom: "acdslwrap"},
		{name: "absent-falls-back-to-go-build", pkg: "./cmd/acdsl", prebuilts: nil, wantFile: "acdsl", wantBuild: true},
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

			require.NoError(t, shipBinary(context.Background(), srcDir, srcDir, destDir, "acdsl", test.pkg))

			shipped, err := os.ReadFile(filepath.Join(destDir, test.wantFile))
			require.NoError(t, err)
			_, invokedErr := os.Stat(invocations)
			wantFrom := test.wantFrom
			if wantFrom == "" {
				wantFrom = test.wantFile
			}
			if test.wantBuild {
				assert.NoError(t, invokedErr, "go build fallback expected")
			} else {
				assert.True(t, os.IsNotExist(invokedErr), "prebuilt present — go must not run")
				assert.Equal(t, test.prebuilts[wantFrom], string(shipped))
			}
		})
	}
}

func TestDistRefusesRepoOwnedRulesFile(t *testing.T) {
	root := newDistRoot(t, distRules)
	dest := newDistDest(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "acdsl"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dest, "acdsl", "rules.acdsl"),
		[]byte("# my own rules\n"), 0o644))
	_, err := Dist(context.Background(), root, distTarget, dest, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo-owned")
}

func TestDistShipsPolicyModeRewritten(t *testing.T) {
	root := newDistRoot(t, distRules)
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "policy.json"), []byte(`{"mode": "free", "dist_mode": "gated"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "policy.schema.json"), []byte("{}\n"), 0o644))
	dest := newDistDest(t)
	lines, err := Dist(context.Background(), root, distTarget, dest, false)
	require.NoError(t, err)
	assert.Contains(t, strings.Join(lines, "\n"), "acdsl/policy.json -> mode: gated")

	shipped, err := LoadPolicy(dest)
	require.NoError(t, err)
	assert.Equal(t, PolicyModeGated, shipped.Mode)
	assert.Empty(t, shipped.DistMode)

	_, err = os.Stat(filepath.Join(dest, "acdsl", "policy.schema.json"))
	assert.NoError(t, err)
}

func TestWritePolicyWithoutSchemaStripsBaseRef(t *testing.T) {
	root := t.TempDir()
	dest := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dest, "acdsl"), 0o755))
	sourcePolicy := `{"mode": "free", "dist_mode": "gated", "base_ref": "origin/develop"}`
	require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "policy.json"), []byte(sourcePolicy), 0o644))

	mode, err := writePolicy(root, dest)

	require.NoError(t, err)
	assert.Equal(t, PolicyModeGated, mode)
	shipped, loadErr := LoadPolicy(dest)
	require.NoError(t, loadErr)
	assert.Empty(t, shipped.BaseRef)
	_, statErr := os.Stat(filepath.Join(dest, "acdsl", "policy.schema.json"))
	assert.True(t, os.IsNotExist(statErr))
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
