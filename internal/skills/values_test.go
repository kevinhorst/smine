package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// valuesStubs are the programs the values fixture puts on PATH; nothing runs them.
var valuesStubs = []string{"fake-alt", "fake-env", "fake-file", "fake-make", "fake-rules"}

// fakeProber answers every handshake with its claims or its error and counts the calls.
type fakeProber struct {
	calls  int
	claims []string
	err    error
}

func (p *fakeProber) Claims(ctx context.Context, provider *skillgraph.Provider) ([]string, error) {
	p.calls++
	return p.claims, p.err
}

// provideValuesCatalog is the repo-scoped check command, the install-scoped lanes program of smine-rules (fake-rules), the repo-scoped acdsl-check program without a program, and the repo-scoped archive path.
func provideValuesCatalog() *skillgraph.Catalog {
	exit := map[string]string{"0": "pass", "other": "fail"}
	resolves := skillgraph.Validate{Resolves: true}
	interfaces := []*skillgraph.Interface{
		{Id: "acdsl-check", Example: "bin/acdsl", Invocation: skillgraph.Invocation{Args: "check", Cwd: "repo-root", Stdin: "none", Timeout: "1m"}, Kind: skillgraph.KindProgram, Result: skillgraph.Result{Exit: exit, Stdout: "violations"}, Scope: skillgraph.ScopeRepo, Summary: "Runs the gates.", Validate: resolves, Version: 1},
		{Id: "check", Example: "go vet ./...", Invocation: skillgraph.Invocation{Args: "none", Cwd: "repo-root", Stdin: "none", Timeout: "30m"}, Kind: skillgraph.KindCommand, Result: skillgraph.Result{Exit: exit, Stdout: "diagnostic"}, Scope: skillgraph.ScopeRepo, Summary: "Runs everything before a commit.", Validate: resolves, Version: 1},
		{Id: "lanes", Invocation: skillgraph.Invocation{Args: "lanes --round-dir <dir>", Cwd: "repo-root", Stdin: "none", Timeout: "5m"}, Kind: skillgraph.KindProgram, Result: skillgraph.Result{Exit: exit, Stdout: "the plan"}, Scope: skillgraph.ScopeInstall, Summary: "Writes the lanes.", Validate: resolves, Version: 1},
		{Id: "plans-archive", Example: "~/archive", Invocation: skillgraph.Invocation{Args: "none", Cwd: "none", Stdin: "none", Timeout: "none"}, Kind: skillgraph.KindPath, Result: skillgraph.Result{Exit: exit, Stdout: "n/a"}, Scope: skillgraph.ScopeRepo, Summary: "The archive root.", Validate: skillgraph.Validate{Exists: "dir"}, Version: 1},
	}
	providers := []*skillgraph.Provider{
		{Id: "acdsl", Implements: []string{"acdsl-check@1"}, Probe: []string{"interfaces"}},
		{Id: "smine-rules", Component: components.ConfigServer, Implements: []string{"lanes@1"}, Probe: []string{"interfaces"}, Program: "fake-rules"},
	}
	return &skillgraph.Catalog{Interfaces: interfaces, Providers: providers}
}

// provideValuesSkill is a skill named demo declaring the given keys bound to the values catalog; LANES is required.
func provideValuesSkill(catalog *skillgraph.Catalog, keys ...string) *skillgraph.Skill {
	bindings := map[string]*skillgraph.Variable{
		"ACDSL_CHECK":   {Interface: "acdsl-check@1"},
		"CHECK":         {Interface: "check@1"},
		"LANES":         {Interface: "lanes@1", Required: true},
		"PLANS_ARCHIVE": {Interface: "plans-archive@1"},
	}
	env := map[string]*skillgraph.Variable{}
	for _, key := range keys {
		env[key] = bindings[key]
	}
	skill := &skillgraph.Skill{Catalog: catalog, Env: env, Frontmatter: []skillgraph.Field{{Key: "name", Value: " demo"}}}
	return skill
}

// provideValuesRepo is a repo dir holding the bin/acdsl stub a relative program value resolves to.
func provideValuesRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "bin"), 0o755))
	writeValuesStub(t, filepath.Join(repoDir, "bin"), "acdsl")
	return repoDir
}

// setValuesEnvironment puts the stubs alone on PATH, points the home at a dir holding archive/ and clears every demo and provider variable.
func setValuesEnvironment(t *testing.T) {
	t.Helper()
	stubDir := t.TempDir()
	for _, stub := range valuesStubs {
		writeValuesStub(t, stubDir, stub)
	}
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "archive"), 0o755))
	t.Setenv("PATH", stubDir)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{"SKILL_DEMO_ACDSL_CHECK", "SKILL_DEMO_CHECK", "SKILL_DEMO_LANES", "SKILL_DEMO_PLANS_ARCHIVE", "PROVIDER_SMINE_RULES"} {
		t.Setenv(name, "")
	}
}

// writeValuesStub writes an executable stub named stub into dir, with the .exe suffix on Windows.
func writeValuesStub(t *testing.T, dir, stub string) {
	t.Helper()
	name := stub
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755))
}

func TestValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []VariableValue

		dir         string
		env         *Rows
		environment map[string]string
		isDeferring bool
		prober      Prober
		set         []components.ID
		skill       *skillgraph.Skill
	}

	setValuesEnvironment(t)
	repoDir := provideValuesRepo(t)
	catalog := provideValuesCatalog()
	check, _ := catalog.Interface("check@1")
	lanes, _ := catalog.Interface("lanes@1")
	acdslCheck, _ := catalog.Interface("acdsl-check@1")
	archive, _ := catalog.Interface("plans-archive@1")
	claimsLanes := &fakeProber{claims: []string{"lanes@1"}}
	unsetLanes := VariableValue{Interface: lanes, Name: "SKILL_DEMO_LANES", Required: true, Source: SourceUnset}
	tests := make([]*testCase, 0)

	// environment-over-install-env-over-provider
	tests = append(tests, &testCase{
		_id: "environment-over-install-env-over-provider",
		_expected: []VariableValue{
			{Interface: acdslCheck, Name: "SKILL_DEMO_ACDSL_CHECK", Source: SourceUnset},
			{Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceRecord, Value: "fake-make audit"},
			{Interface: lanes, Name: "SKILL_DEMO_LANES", Required: true, Source: SourceEnvironment, Value: "fake-env"},
		},

		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit", "SKILL_DEMO_LANES": "fake-file"}},
		environment: map[string]string{"SKILL_DEMO_LANES": "fake-env"},
		prober:      claimsLanes,
		set:         nil,
		skill:       provideValuesSkill(catalog, "ACDSL_CHECK", "CHECK", "LANES"),
	})

	// provider-row-over-catalog-program
	tests = append(tests, &testCase{
		_id:       "provider-row-over-catalog-program",
		_expected: []VariableValue{{Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: "provider smine-rules", Value: "fake-alt"}},

		env:    &Rows{Providers: map[string]string{"PROVIDER_SMINE_RULES": "fake-alt"}},
		prober: claimsLanes,
		set:    nil,
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// capability-row-over-provider-row
	tests = append(tests, &testCase{
		_id:       "capability-row-over-provider-row",
		_expected: []VariableValue{{Interface: lanes, Name: "SKILL_DEMO_LANES", Required: true, Source: SourceRecord, Value: "fake-file"}},

		env:    &Rows{Providers: map[string]string{"PROVIDER_SMINE_RULES": "fake-alt"}, Skills: map[string]string{"SKILL_DEMO_LANES": "fake-file"}},
		prober: claimsLanes,
		set:    nil,
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// provider-without-component-unset
	tests = append(tests, &testCase{
		_id:       "provider-without-component-unset",
		_expected: []VariableValue{unsetLanes},

		prober: claimsLanes,
		set:    []components.ID{components.Skills},
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// command-with-operator-unset-with-cause
	tests = append(tests, &testCase{
		_id:       "command-with-operator-unset-with-cause",
		_expected: []VariableValue{{Cause: `value: cmdline.SplitLines: cmdline.Split: Shell operator "&&" in "fake-make && fake-make"`, Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceUnset}},

		env:   &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make && fake-make"}},
		set:   nil,
		skill: provideValuesSkill(catalog, "CHECK"),
	})

	// program-not-on-path-unset-with-cause
	tests = append(tests, &testCase{
		_id:       "program-not-on-path-unset-with-cause",
		_expected: []VariableValue{{Cause: "missing-rules does not resolve to an executable", Interface: lanes, Name: "SKILL_DEMO_LANES", Required: true, Source: SourceUnset}},

		env:   &Rows{Skills: map[string]string{"SKILL_DEMO_LANES": "missing-rules"}},
		set:   nil,
		skill: provideValuesSkill(catalog, "LANES"),
	})

	// path-missing-unset-with-cause
	tests = append(tests, &testCase{
		_id:       "path-missing-unset-with-cause",
		_expected: []VariableValue{{Cause: "~/missing is not an existing dir", Interface: archive, Name: "SKILL_DEMO_PLANS_ARCHIVE", Source: SourceUnset}},

		env:   &Rows{Skills: map[string]string{"SKILL_DEMO_PLANS_ARCHIVE": "~/missing"}},
		set:   nil,
		skill: provideValuesSkill(catalog, "PLANS_ARCHIVE"),
	})

	// path-present-set
	tests = append(tests, &testCase{
		_id:       "path-present-set",
		_expected: []VariableValue{{Interface: archive, Name: "SKILL_DEMO_PLANS_ARCHIVE", Source: SourceRecord, Value: "~/archive"}},

		env:   &Rows{Skills: map[string]string{"SKILL_DEMO_PLANS_ARCHIVE": "~/archive"}},
		set:   nil,
		skill: provideValuesSkill(catalog, "PLANS_ARCHIVE"),
	})

	// probe-claims-version-set
	tests = append(tests, &testCase{
		_id:       "probe-claims-version-set",
		_expected: []VariableValue{{Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: "provider smine-rules", Value: "fake-rules"}},

		prober: claimsLanes,
		set:    []components.ID{components.ConfigServer},
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// probe-lacks-version-unset-with-cause
	tests = append(tests, &testCase{
		_id:       "probe-lacks-version-unset-with-cause",
		_expected: []VariableValue{{Cause: "smine-rules does not implement lanes@1 (answered: lanes@0, title-set@1)", Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: SourceUnset}},

		prober: &fakeProber{claims: []string{"lanes@0", "title-set@1"}},
		set:    nil,
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// probe-error-unset-with-cause
	tests = append(tests, &testCase{
		_id:       "probe-error-unset-with-cause",
		_expected: []VariableValue{{Cause: "smine-rules does not answer `fake-rules interfaces`: exit status 1", Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: SourceUnset}},

		prober: &fakeProber{err: errors.New("exit status 1")},
		set:    nil,
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// nil-prober-skips-the-probe
	tests = append(tests, &testCase{
		_id:       "nil-prober-skips-the-probe",
		_expected: []VariableValue{{Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: "provider smine-rules", Value: "fake-rules"}},

		prober: nil,
		set:    nil,
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// no-env-block
	tests = append(tests, &testCase{
		_id:       "no-env-block",
		_expected: nil,

		set:   nil,
		skill: provideValuesSkill(catalog),
	})

	// deferring-repo-scoped-invocation
	tests = append(tests, &testCase{
		_id: "deferring-repo-scoped-invocation",
		_expected: []VariableValue{
			{Interface: acdslCheck, IsDeferred: true, Name: "SKILL_DEMO_ACDSL_CHECK", Source: SourceInvocation},
			{Interface: check, IsDeferred: true, Name: "SKILL_DEMO_CHECK", Source: SourceInvocation},
			{Interface: archive, IsDeferred: true, Name: "SKILL_DEMO_PLANS_ARCHIVE", Source: SourceInvocation},
		},

		dir:         repoDir,
		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit", "SKILL_DEMO_PLANS_ARCHIVE": "~/archive"}},
		isDeferring: true,
		set:         nil,
		skill:       provideValuesSkill(catalog, "ACDSL_CHECK", "CHECK", "PLANS_ARCHIVE"),
	})

	// deferring-install-scoped-resolved
	tests = append(tests, &testCase{
		_id: "deferring-install-scoped-resolved",
		_expected: []VariableValue{
			{Interface: check, IsDeferred: true, Name: "SKILL_DEMO_CHECK", Source: SourceInvocation},
			{Interface: lanes, Name: "SKILL_DEMO_LANES", Provider: "smine-rules", Required: true, Source: "provider smine-rules", Value: "fake-rules"},
		},

		dir:         repoDir,
		isDeferring: true,
		prober:      claimsLanes,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK", "LANES"),
	})

	// not-deferring-repo-scoped-resolved
	tests = append(tests, &testCase{
		_id:       "not-deferring-repo-scoped-resolved",
		_expected: []VariableValue{{Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceRecord, Value: "fake-make audit"}},

		dir:         repoDir,
		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// relative-program-resolved-under-dir
	tests = append(tests, &testCase{
		_id:       "relative-program-resolved-under-dir",
		_expected: []VariableValue{{Interface: acdslCheck, Name: "SKILL_DEMO_ACDSL_CHECK", Source: SourceRecord, Value: "bin/acdsl"}},

		dir:         repoDir,
		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_ACDSL_CHECK": "bin/acdsl"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "ACDSL_CHECK"),
	})

	// multi-line-command-first-word-each-line
	tests = append(tests, &testCase{
		_id:       "multi-line-command-first-word-each-line",
		_expected: []VariableValue{{Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceRecord, Value: "fake-make audit\nbin/acdsl check"}},

		dir:         repoDir,
		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit\nbin/acdsl check"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// multi-line-second-word-missing-cause
	tests = append(tests, &testCase{
		_id:       "multi-line-second-word-missing-cause",
		_expected: []VariableValue{{Cause: "missing-tool does not resolve to an executable", Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceUnset}},

		dir:         repoDir,
		env:         &Rows{Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit\nmissing-tool run"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// global-default-after-record-row
	tests = append(tests, &testCase{
		_id:       "global-default-after-record-row",
		_expected: []VariableValue{{Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceGlobalDefault, Value: "fake-make test"}},

		dir:         repoDir,
		env:         &Rows{Defaults: map[string]string{"check": "fake-make test"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// record-row-over-global-default
	tests = append(tests, &testCase{
		_id:       "record-row-over-global-default",
		_expected: []VariableValue{{Interface: check, Name: "SKILL_DEMO_CHECK", Source: SourceRecord, Value: "fake-make audit"}},

		dir:         repoDir,
		env:         &Rows{Defaults: map[string]string{"check": "fake-make test"}, Skills: map[string]string{"SKILL_DEMO_CHECK": "fake-make audit"}},
		isDeferring: false,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// install-scoped-ignores-global-default
	tests = append(tests, &testCase{
		_id:       "install-scoped-ignores-global-default",
		_expected: []VariableValue{unsetLanes},

		env:    &Rows{Defaults: map[string]string{"lanes": "fake-alt"}},
		prober: claimsLanes,
		set:    []components.ID{components.Skills},
		skill:  provideValuesSkill(catalog, "LANES"),
	})

	// deferring-keeps-invocation
	tests = append(tests, &testCase{
		_id:       "deferring-keeps-invocation",
		_expected: []VariableValue{{Interface: check, IsDeferred: true, Name: "SKILL_DEMO_CHECK", Source: SourceInvocation}},

		dir:         repoDir,
		env:         &Rows{Defaults: map[string]string{"check": "fake-make test"}},
		isDeferring: true,
		set:         nil,
		skill:       provideValuesSkill(catalog, "CHECK"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv("SKILL_DEMO_LANES", test.environment["SKILL_DEMO_LANES"])
			assert.Equal(t, test._expected, Values(context.Background(), test.dir, test.env, test.isDeferring, test.prober, test.set, test.skill))
		})
	}
}

func TestGlobalDefault(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		iface *skillgraph.Interface
		rows  *Rows
	}

	catalog := provideValuesCatalog()
	check, _ := catalog.Interface("check@1")
	lanes, _ := catalog.Interface("lanes@1")
	rows := &Rows{Defaults: map[string]string{"check": "make audit", "lanes": "fake-alt"}}
	tests := make([]*testCase, 0)

	// repo-scoped-reads-default
	tests = append(tests, &testCase{
		_id:       "repo-scoped-reads-default",
		_expected: "make audit",

		iface: check,
		rows:  rows,
	})

	// install-scoped-empty
	tests = append(tests, &testCase{
		_id:       "install-scoped-empty",
		_expected: "",

		iface: lanes,
		rows:  rows,
	})

	// nil-interface-empty
	tests = append(tests, &testCase{
		_id:       "nil-interface-empty",
		_expected: "",

		iface: nil,
		rows:  rows,
	})

	// nil-rows-empty
	tests = append(tests, &testCase{
		_id:       "nil-rows-empty",
		_expected: "",

		iface: check,
		rows:  nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, GlobalDefault(test.iface, test.rows))
		})
	}
}

func TestUnconfigured(t *testing.T) {
	values := []VariableValue{
		{Name: "SKILL_DEMO_CHECK", Source: SourceUnset},
		{Cause: "fake-rules does not resolve to an executable", Name: "SKILL_DEMO_LANES", Required: true, Source: SourceUnset},
		{Name: "SKILL_DEMO_PLAN_SECTIONS", Required: true, Source: SourceUnset},
		{Name: "SKILL_DEMO_TEST", Required: true, Source: SourceRecord, Value: "make test"},
	}

	expected := []string{"SKILL_DEMO_LANES: fake-rules does not resolve to an executable", "SKILL_DEMO_PLAN_SECTIONS"}
	assert.Equal(t, expected, Unconfigured(values))
	assert.Empty(t, Unconfigured(nil), "no-values-no-reasons")

	deferred := []VariableValue{{IsDeferred: true, Name: "SKILL_DEMO_BUILD", Required: true, Source: SourceInvocation}}
	assert.Empty(t, Unconfigured(deferred), "deferred-required-not-listed")
}

func TestDeferredNames(t *testing.T) {
	values := []VariableValue{
		{IsDeferred: true, Name: "SKILL_DEMO_BUILD", Required: true, Source: SourceInvocation},
		{Name: "SKILL_DEMO_LANES", Required: true, Source: SourceRecord, Value: "fake-rules"},
		{IsDeferred: true, Name: "SKILL_DEMO_TEST", Source: SourceInvocation},
	}

	expected := map[string]bool{"SKILL_DEMO_BUILD": true, "SKILL_DEMO_TEST": true}
	assert.Equal(t, expected, DeferredNames(values), "names-of-deferred-only")
	assert.Equal(t, map[string]bool{}, DeferredNames(nil))
}

func TestValueMap(t *testing.T) {
	values := []VariableValue{
		{Name: "SKILL_DEMO_CHECK", Source: SourceRecord, Value: "make audit"},
		{Name: "SKILL_DEMO_RULES", Source: SourceUnset},
	}

	assert.Equal(t, map[string]string{"SKILL_DEMO_CHECK": "make audit"}, ValueMap(values))
	assert.Equal(t, map[string]string{}, ValueMap(nil))
}
