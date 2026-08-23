package acdsl

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/reach"
)

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func checkSetup(script string) ([]Rule, map[string]RegistryEntry, []string) {
	rules := []Rule{{Id: "X-001", Verifier: "fake", Anchor: `\.go$`, Why: "the why", Params: map[string]string{"b": "2", "a": "1"}}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	tracked := []string{"a.go", "b.go"}
	return rules, registry, tracked
}

func TestCheckExitZeroMeansPass(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nexit 0\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckExitOneYieldsDiagnostics(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho 'a.go:1: first'\necho 'b.go:2: second'\nexit 1\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	require.Len(t, diagnostics, 2)
	assert.Equal(t, "X-001", diagnostics[0].RuleId)
	assert.Equal(t, "fake", diagnostics[0].Verifier)
	assert.Equal(t, "a.go:1: first", diagnostics[0].Message)
	assert.Equal(t, "the why", diagnostics[0].Why)
	assert.Equal(t, "b.go:2: second", diagnostics[1].Message)
}

func TestCheckExitTwoIsError(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho broken >&2\nexit 2\n")
	rules, registry, tracked := checkSetup(script)
	_, err := Check(context.Background(), "", rules, registry, tracked)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-001")
	assert.Contains(t, err.Error(), "failed")
}

func TestCheckUnregisteredVerifierIsError(t *testing.T) {
	rules := []Rule{{Id: "X-001", Verifier: "absent", Anchor: `\.go$`, Why: "w"}}
	_, err := Check(context.Background(), "", rules, map[string]RegistryEntry{}, []string{"a.go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `verifier "absent" not registered`)
}

func TestCheckSkipsDisabledRules(t *testing.T) {
	// The skip precedes the registry lookup: an unregistered verifier on a
	// disabled rule is not an error, and no verifier runs.
	rules := []Rule{{Id: "X-001", Verifier: "absent", Anchor: `\.go$`, Why: "w", Reach: reach.None}}
	diagnostics, err := Check(context.Background(), "", rules, map[string]RegistryEntry{}, []string{"a.go"})
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckParamsPassedSorted(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho \"args: $2 $3\"\nexit 1\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	require.Len(t, diagnostics, 1)
	assert.Equal(t, "args: a=1 b=2", diagnostics[0].Message)
}

func TestCheckFilesListPassedAsFirstArg(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nawk '{print $0 \":1: seen\"}' \"$1\"\nexit 1\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	require.Len(t, diagnostics, 2)
	assert.Equal(t, "a.go:1: seen", diagnostics[0].Message)
	assert.Equal(t, "b.go:1: seen", diagnostics[1].Message)
}

func TestCheckToolNoiseLinesFiltered(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho 'a.go:1: real finding'\necho 'exit status 1'\nexit 1\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	require.Len(t, diagnostics, 1)
	assert.Equal(t, "a.go:1: real finding", diagnostics[0].Message)
}

func TestCheckExitOneWithoutContractLinesFallsBack(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho 'something went sideways'\nexit 1\n")
	rules, registry, tracked := checkSetup(script)
	diagnostics, err := Check(context.Background(), "", rules, registry, tracked)
	require.NoError(t, err)
	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Message, "something went sideways")
}

func TestCheckTaskZeroMatchYieldsDiagnostic(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nexit 0\n")
	rules := []Rule{{Id: "TASK-X-001", Verifier: "fake", Anchor: `^internal/planned/`, Why: "the why", Lifetime: LifetimeTask, File: "tasks/contract.acdsl", Line: 3}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	diagnostics, err := Check(context.Background(), "", rules, registry, []string{"a.go"})
	require.NoError(t, err)
	require.Len(t, diagnostics, 1)
	assert.Equal(t, "TASK-X-001", diagnostics[0].RuleId)
	assert.Equal(t, "tasks/contract.acdsl:3: anchor matched no files — planned artifact not yet present", diagnostics[0].Message)
}

func TestCheckAllowEmptyZeroMatchPasses(t *testing.T) {
	// empty="ok" marks a transient artifact class (e.g. active design plans,
	// all archived) — zero matches is a legitimate state, not tool breakage.
	script := writeScript(t, "#!/bin/sh\nexit 1\n")
	rules := []Rule{{Id: "X-001", Verifier: "fake", Anchor: `^plans/[^/]+/design/`, AllowEmpty: true, Why: "w", Lifetime: LifetimeDoctrine}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	diagnostics, err := Check(context.Background(), "", rules, registry, []string{"a.go"})
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckNeedsMissingSkipsRule(t *testing.T) {
	// needs= names artifacts the verifier reads; a tree without them (a
	// public clone whose private artifacts are excluded) skips the rule
	// instead of running a verifier that would fail on the absent file.
	script := writeScript(t, "#!/bin/sh\nexit 1\n")
	rules := []Rule{{Id: "X-001", Verifier: "fake", Anchor: `\.go$`, Needs: []string{"context/context.json"}, Why: "w", Lifetime: LifetimeDoctrine}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	diagnostics, err := Check(context.Background(), t.TempDir(), rules, registry, []string{"a.go"})
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckDoctrineZeroMatchIsError(t *testing.T) {
	// The no-match contract stands: a doctrine anchor matching nothing is
	// tool breakage (typo guard). Distribution keeps target gates runnable
	// by never shipping a rule without matching target files (Dist filter).
	script := writeScript(t, "#!/bin/sh\nexit 0\n")
	rules := []Rule{{Id: "X-001", Verifier: "fake", Anchor: `^nothing/`, Why: "w", Lifetime: LifetimeDoctrine}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	_, err := Check(context.Background(), "", rules, registry, []string{"a.go"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnchorEmpty)
}

func TestCheckNonGoAnchorResolvesInFullUniverse(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nexit 0\n")
	rules := []Rule{{Id: "TASK-X-001", Verifier: "fake", Anchor: `\.golden\.json$`, Why: "w", Lifetime: LifetimeTask}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{script}, TimeoutSec: 10}}
	universe := []string{"a.go", "plans/x/observations/resp.golden.json"}
	diagnostics, err := Check(context.Background(), "", rules, registry, universe)
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckTimeoutKillIsError(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nsleep 5\nexit 0\n")
	rules, registry, tracked := checkSetup(script)
	registry["fake"] = RegistryEntry{Argv: []string{script}, TimeoutSec: 1}
	_, err := Check(context.Background(), "", rules, registry, tracked)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}
