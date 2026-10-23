package sessionstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        []Feature
		_expectedIsKnown bool

		word string
	}

	tests := make([]*testCase, 0)

	// member
	tests = append(tests, &testCase{
		_id:              "member",
		_expected:        []Feature{RgateGlobal},
		_expectedIsKnown: true,

		word: "rgate:global",
	})

	// member-list
	tests = append(tests, &testCase{
		_id:              "member-list",
		_expected:        []Feature{Ext("go"), Ext("rs"), RgateGlobal},
		_expectedIsKnown: true,

		word: "rgate:go,rs,global",
	})

	// bare-group
	tests = append(tests, &testCase{
		_id:              "bare-group",
		_expected:        []Feature{Feature(GroupInject)},
		_expectedIsKnown: true,

		word: "inject",
	})

	// inject-env
	tests = append(tests, &testCase{
		_id:              "inject-env",
		_expected:        []Feature{InjectEnv},
		_expectedIsKnown: true,

		word: "inject:env",
	})

	// all
	tests = append(tests, &testCase{
		_id:              "all",
		_expected:        []Feature{Feature(All)},
		_expectedIsKnown: true,

		word: "all",
	})

	// heartbeat-short
	tests = append(tests, &testCase{
		_id:              "heartbeat-short",
		_expected:        []Feature{Heartbeat},
		_expectedIsKnown: true,

		word: "hb",
	})

	// heartbeat-long
	tests = append(tests, &testCase{
		_id:              "heartbeat-long",
		_expected:        []Feature{Heartbeat},
		_expectedIsKnown: true,

		word: "heartbeat",
	})

	// unknown-group
	tests = append(tests, &testCase{
		_id:              "unknown-group",
		_expected:        nil,
		_expectedIsKnown: false,

		word: "gate:read",
	})

	// old-hyphen-name
	tests = append(tests, &testCase{
		_id:              "old-hyphen-name",
		_expected:        nil,
		_expectedIsKnown: false,

		word: "read-gate",
	})

	// empty-member
	tests = append(tests, &testCase{
		_id:              "empty-member",
		_expected:        nil,
		_expectedIsKnown: false,

		word: "skill:enforce,",
	})

	// empty-word
	tests = append(tests, &testCase{
		_id:              "empty-word",
		_expected:        nil,
		_expectedIsKnown: false,

		word: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			features, isKnown := Parse(test.word)
			assert.Equal(t, test._expected, features)
			assert.Equal(t, test._expectedIsKnown, isKnown)
		})
	}
}

func TestExpand(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        []Feature
		_expectedUnknown string

		features []Feature
		guides   []string
	}

	tests := make([]*testCase, 0)

	// all-with-guides
	tests = append(tests, &testCase{
		_id:              "all-with-guides",
		_expected:        []Feature{RgateGlobal, RgateInvoked, RgatePlan, Ext("go"), SkillEnforce, SkillInvokegate, InjectGlobal, InjectSkill, InjectAcdsl, InjectEnv, InjectPlan, SessionTitle, Heartbeat},
		_expectedUnknown: "",

		features: []Feature{Feature(All)},
		guides:   []string{"go"},
	})

	// group-word
	tests = append(tests, &testCase{
		_id:              "group-word",
		_expected:        []Feature{SkillEnforce, SkillInvokegate},
		_expectedUnknown: "",

		features: []Feature{Feature(GroupSkill)},
		guides:   nil,
	})

	// inject-group-holds-env
	tests = append(tests, &testCase{
		_id:              "inject-group-holds-env",
		_expected:        []Feature{InjectGlobal, InjectSkill, InjectAcdsl, InjectEnv, InjectPlan},
		_expectedUnknown: "",

		features: []Feature{Feature(GroupInject)},
		guides:   nil,
	})

	// known-extension-member
	tests = append(tests, &testCase{
		_id:              "known-extension-member",
		_expected:        []Feature{Ext("rs"), Heartbeat},
		_expectedUnknown: "",

		features: []Feature{Ext("rs"), Heartbeat},
		guides:   []string{"go", "rs"},
	})

	// unknown-extension-member
	tests = append(tests, &testCase{
		_id:              "unknown-extension-member",
		_expected:        nil,
		_expectedUnknown: "zig",

		features: []Feature{Ext("zig")},
		guides:   []string{"go"},
	})

	// unknown-fixed-member
	tests = append(tests, &testCase{
		_id:              "unknown-fixed-member",
		_expected:        nil,
		_expectedUnknown: "nope",

		features: []Feature{Feature("inject:nope")},
		guides:   nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			features, unknown := Expand(test.features, test.guides)
			assert.Equal(t, test._expected, features)
			assert.Equal(t, test._expectedUnknown, unknown)
		})
	}
}

func TestCompact(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		features []Feature
		guides   []string
	}

	tests := make([]*testCase, 0)

	// whole-group-reads-as-word
	tests = append(tests, &testCase{
		_id:       "whole-group-reads-as-word",
		_expected: []string{"skill", "hb"},

		features: []Feature{Heartbeat, SkillEnforce, SkillInvokegate},
		guides:   nil,
	})

	// partial-group-lists-members
	tests = append(tests, &testCase{
		_id:       "partial-group-lists-members",
		_expected: []string{"rgate:go", "rgate:global"},

		features: []Feature{Ext("go"), RgateGlobal},
		guides:   []string{"go"},
	})

	// rgate-needs-every-extension
	tests = append(tests, &testCase{
		_id:       "rgate-needs-every-extension",
		_expected: []string{"rgate:global", "rgate:invoked", "rgate:plan"},

		features: []Feature{RgateGlobal, RgateInvoked, RgatePlan},
		guides:   []string{"go"},
	})

	// everything
	tests = append(tests, &testCase{
		_id:       "everything",
		_expected: []string{"rgate", "skill", "inject", "session", "hb"},

		features: Features(nil),
		guides:   nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Compact(test.features, test.guides))
		})
	}
}

func TestCatalog(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		actual []string
	}

	tests := make([]*testCase, 0)

	// features-in-help-order
	tests = append(tests, &testCase{
		_id: "features-in-help-order",
		_expected: []string{
			"rgate:global",
			"rgate:invoked",
			"rgate:plan",
			"skill:enforce",
			"skill:invokegate",
			"inject:global",
			"inject:skill",
			"inject:acdsl",
			"inject:env",
			"inject:plan",
			"session:title",
			"hb",
		},

		actual: catalogFeatureNames(Catalog()),
	})

	// features-equal-catalog
	tests = append(tests, &testCase{
		_id:       "features-equal-catalog",
		_expected: catalogFeatureNames(Catalog()),

		actual: featureNames(Features(nil)),
	})

	// every-entry-has-text
	tests = append(tests, &testCase{
		_id:       "every-entry-has-text",
		_expected: []string{},

		actual: entriesWithoutText(append(Catalog(), GuideEntry)),
	})

	// help-closes-rgate-with-guides
	tests = append(tests, &testCase{
		_id: "help-closes-rgate-with-guides",
		_expected: []string{
			"rgate:global",
			"rgate:invoked",
			"rgate:plan",
			"rgate:<ext|guide>",
			"skill:enforce",
			"skill:invokegate",
			"inject:global",
			"inject:skill",
			"inject:acdsl",
			"inject:env",
			"inject:plan",
			"session:title",
			"hb",
		},

		actual: catalogFeatureNames(Help()),
	})

	// names-grammar-line
	tests = append(tests, &testCase{
		_id:       "names-grammar-line",
		_expected: []string{"rgate[:global|invoked|plan|<ext|guide>], skill[:enforce|invokegate], inject[:global|skill|acdsl|env|plan], session[:title], hb, all"},

		actual: []string{Names()},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.actual)
		})
	}
}

func TestExtensions(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		globs []string
	}

	tests := make([]*testCase, 0)

	// one-guide-many-globs-one-extension
	tests = append(tests, &testCase{
		_id:       "one-guide-many-globs-one-extension",
		_expected: []string{"go"},

		globs: []string{"*_test.go", "*_test_data.go"},
	})

	// one-guide-two-extensions
	tests = append(tests, &testCase{
		_id:       "one-guide-two-extensions",
		_expected: []string{"cue", "md"},

		globs: []string{"skills/**/*.cue", "skills/**/payloads/*.md"},
	})

	// literal-file-name
	tests = append(tests, &testCase{
		_id:       "literal-file-name",
		_expected: []string{"rs", "toml"},

		globs: []string{"*.rs", "Cargo.toml"},
	})

	// glob-without-extension-adds-nothing
	tests = append(tests, &testCase{
		_id:       "glob-without-extension-adds-nothing",
		_expected: []string{},

		globs: []string{"plans/*/*/runbooks/**", "Makefile", "*.mk*"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Extensions(test.globs))
		})
	}
}

func TestGuideWords(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		path string
	}

	dir := t.TempDir()
	indexPath := filepath.Join(dir, "context.json")
	index := `{"guides":[{"path":"rules/go.md","files":["*.go"]},{"path":"rules/go_test.md","files":["*_test.go"]},{"path":"rules/skills.md","files":["skills/**/*.cue","skills/**/payloads/*.md"]}]}`
	require.NoError(t, os.WriteFile(indexPath, []byte(index), 0o600))
	garbagePath := filepath.Join(dir, "garbage.json")
	require.NoError(t, os.WriteFile(garbagePath, []byte("{"), 0o600))
	tests := make([]*testCase, 0)

	// extensions-and-names-of-the-index
	tests = append(tests, &testCase{
		_id:       "extensions-and-names-of-the-index",
		_expected: []string{"cue", "go", "go_test", "md", "skills"},

		path: indexPath,
	})

	// missing-index-none
	tests = append(tests, &testCase{
		_id:       "missing-index-none",
		_expected: nil,

		path: filepath.Join(dir, "missing.json"),
	})

	// garbage-index-none
	tests = append(tests, &testCase{
		_id:       "garbage-index-none",
		_expected: nil,

		path: garbagePath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, GuideWords(test.path))
		})
	}
}

func TestFile(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		sessionId string
		stateDir  string
	}

	home := t.TempDir()
	tests := make([]*testCase, 0)

	// empty-dir
	tests = append(tests, &testCase{
		_id:       "empty-dir",
		_expected: "",

		sessionId: "session-1",
		stateDir:  "",
	})

	// empty-session
	tests = append(tests, &testCase{
		_id:       "empty-session",
		_expected: "",

		sessionId: "",
		stateDir:  filepath.Join("dir", "state"),
	})

	// joined-path
	tests = append(tests, &testCase{
		_id:       "joined-path",
		_expected: filepath.Join("dir", "state", "session-1.json"),

		sessionId: "session-1",
		stateDir:  filepath.Join("dir", "state"),
	})

	// state-dir-on-home
	tests = append(tests, &testCase{
		_id:       "state-dir-on-home",
		_expected: filepath.Join(home, "tool-policy", "state", "session-1.json"),

		sessionId: "session-1",
		stateDir:  StateDir(home),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, File(test.stateDir, test.sessionId))
		})
	}
}

func TestLoadSave(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *SessionState
		_shouldPass bool

		loadPath string
		saved    *SessionState
		savePath string
	}

	dir := t.TempDir()
	tests := make([]*testCase, 0)

	// missing-file-is-zero
	tests = append(tests, &testCase{
		_id:         "missing-file-is-zero",
		_expected:   &SessionState{},
		_shouldPass: true,

		loadPath: filepath.Join(dir, "missing.json"),
		saved:    nil,
		savePath: "",
	})

	// empty-path-is-zero
	tests = append(tests, &testCase{
		_id:         "empty-path-is-zero",
		_expected:   &SessionState{},
		_shouldPass: true,

		loadPath: "",
		saved:    nil,
		savePath: "",
	})

	// record-without-switches-key
	oldRecordPath := filepath.Join(dir, "old.json")
	oldRecord := []byte(`{"settings_local": {"is_reverted": true, "last_appended": ["Bash(lsof *)"]}}`)
	require.NoError(t, os.WriteFile(oldRecordPath, oldRecord, 0o600))
	tests = append(tests, &testCase{
		_id: "record-without-switches-key",
		_expected: &SessionState{
			SettingsLocal: SelfServiceState{IsReverted: true, LastAppended: []string{"Bash(lsof *)"}},
			Switches:      Switches{},
		},
		_shouldPass: true,

		loadPath: oldRecordPath,
		saved:    nil,
		savePath: "",
	})

	// garbage-is-error
	garbagePath := filepath.Join(dir, "garbage.json")
	require.NoError(t, os.WriteFile(garbagePath, []byte("not json"), 0o600))
	tests = append(tests, &testCase{
		_id:         "garbage-is-error",
		_expected:   nil,
		_shouldPass: false,

		loadPath: garbagePath,
		saved:    nil,
		savePath: "",
	})

	// round-trip
	roundTripPath := filepath.Join(dir, "round-trip.json")
	roundTripState := &SessionState{
		SettingsLocal: SelfServiceState{IsReverted: false, LastAppended: []string{"Bash(lsof *)"}},
		Switches: Switches{
			History: []SwitchEvent{{Action: SwitchDisabled, At: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Feature: Heartbeat}},
			Off:     []Feature{Heartbeat, RgateGlobal},
		},
	}
	tests = append(tests, &testCase{
		_id:         "round-trip",
		_expected:   roundTripState,
		_shouldPass: true,

		loadPath: roundTripPath,
		saved:    roundTripState,
		savePath: roundTripPath,
	})

	// save-empty-path-is-error
	tests = append(tests, &testCase{
		_id:         "save-empty-path-is-error",
		_expected:   nil,
		_shouldPass: false,

		loadPath: "",
		saved:    &SessionState{},
		savePath: "",
	})

	// save-creates-dir
	nestedPath := filepath.Join(dir, "home", "tool-policy", "state", "session-1.json")
	nestedState := &SessionState{
		SettingsLocal: SelfServiceState{IsReverted: true, LastAppended: []string{}},
		Switches:      Switches{Off: []Feature{SkillEnforce}},
	}
	tests = append(tests, &testCase{
		_id:         "save-creates-dir",
		_expected:   nestedState,
		_shouldPass: true,

		loadPath: nestedPath,
		saved:    nestedState,
		savePath: nestedPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			loaded, err := saveThenLoad(test.savePath, test.saved, test.loadPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, loaded)
		})
	}
}

func TestSwitches_Set(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Switches

		off      []Feature
		switches *Switches
	}

	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	earlier := syncNow.Add(-time.Hour)
	tests := make([]*testCase, 0)

	// first-disable-records-one-event
	tests = append(tests, &testCase{
		_id: "first-disable-records-one-event",
		_expected: &Switches{
			History: []SwitchEvent{{Action: SwitchDisabled, At: syncNow, Feature: InjectGlobal}},
			Off:     []Feature{InjectGlobal},
		},

		off:      []Feature{InjectGlobal},
		switches: &Switches{},
	})

	// enable-appends-after-history
	tests = append(tests, &testCase{
		_id: "enable-appends-after-history",
		_expected: &Switches{
			History: []SwitchEvent{
				{Action: SwitchDisabled, At: earlier, Feature: InjectGlobal},
				{Action: SwitchEnabled, At: syncNow, Feature: InjectGlobal},
			},
			Off: []Feature{},
		},

		off: []Feature{},
		switches: &Switches{
			History: []SwitchEvent{{Action: SwitchDisabled, At: earlier, Feature: InjectGlobal}},
			Off:     []Feature{InjectGlobal},
		},
	})

	// swap-records-disable-before-enable
	tests = append(tests, &testCase{
		_id: "swap-records-disable-before-enable",
		_expected: &Switches{
			History: []SwitchEvent{
				{Action: SwitchDisabled, At: syncNow, Feature: Heartbeat},
				{Action: SwitchEnabled, At: syncNow, Feature: SessionTitle},
			},
			Off: []Feature{Heartbeat, RgateGlobal},
		},

		off:      []Feature{Heartbeat, RgateGlobal},
		switches: &Switches{Off: []Feature{RgateGlobal, SessionTitle}},
	})

	// unchanged-set-adds-nothing
	tests = append(tests, &testCase{
		_id:       "unchanged-set-adds-nothing",
		_expected: &Switches{Off: []Feature{SkillEnforce}},

		off:      []Feature{SkillEnforce},
		switches: &Switches{Off: []Feature{SkillEnforce}},
	})

	// reset-to-nil-enables-every-feature
	tests = append(tests, &testCase{
		_id: "reset-to-nil-enables-every-feature",
		_expected: &Switches{
			History: []SwitchEvent{
				{Action: SwitchEnabled, At: syncNow, Feature: Heartbeat},
				{Action: SwitchEnabled, At: syncNow, Feature: RgateGlobal},
			},
			Off: nil,
		},

		off:      nil,
		switches: &Switches{Off: []Feature{Heartbeat, RgateGlobal}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.switches.Set(test.off, syncNow)
			assert.Equal(t, test._expected, test.switches)
		})
	}
}

func TestIsOff(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   bool
		_shouldPass bool

		feature Feature
		path    string
	}

	dir := t.TempDir()
	recordPath := File(dir, "session-1")
	record := &SessionState{Switches: Switches{Off: []Feature{RgateGlobal, SessionTitle}}}
	require.NoError(t, Save(recordPath, record))
	tests := make([]*testCase, 0)

	// empty-path-false
	tests = append(tests, &testCase{
		_id:         "empty-path-false",
		_expected:   false,
		_shouldPass: true,

		feature: RgateGlobal,
		path:    "",
	})

	// missing-file-false
	tests = append(tests, &testCase{
		_id:         "missing-file-false",
		_expected:   false,
		_shouldPass: true,

		feature: RgateGlobal,
		path:    File(dir, "session-missing"),
	})

	// feature-off
	tests = append(tests, &testCase{
		_id:         "feature-off",
		_expected:   true,
		_shouldPass: true,

		feature: RgateGlobal,
		path:    recordPath,
	})

	// feature-on
	tests = append(tests, &testCase{
		_id:         "feature-on",
		_expected:   false,
		_shouldPass: true,

		feature: Heartbeat,
		path:    recordPath,
	})

	// garbage-false-with-error
	garbagePath := File(dir, "session-garbage")
	require.NoError(t, os.WriteFile(garbagePath, []byte("{"), 0o600))
	tests = append(tests, &testCase{
		_id:         "garbage-false-with-error",
		_expected:   false,
		_shouldPass: false,

		feature: RgateGlobal,
		path:    garbagePath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isOff, err := IsOff(test.path, test.feature)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, isOff)
		})
	}
}

func TestOff(t *testing.T) {
	dir := t.TempDir()
	recordPath := File(dir, "session-1")
	require.NoError(t, Save(recordPath, &SessionState{Switches: Switches{Off: []Feature{Heartbeat, RgateGlobal}}}))
	garbagePath := File(dir, "session-garbage")
	require.NoError(t, os.WriteFile(garbagePath, []byte("{"), 0o600))

	off, err := Off(recordPath)
	assert.NoError(t, err)
	assert.Equal(t, []Feature{Heartbeat, RgateGlobal}, off)

	missing, err := Off(File(dir, "session-missing"))
	assert.NoError(t, err)
	assert.Nil(t, missing)

	garbage, err := Off(garbagePath)
	assert.Error(t, err)
	assert.Nil(t, garbage)
}

func catalogFeatureNames(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, string(entry.Feature))
	}
	return names
}

func entriesWithoutText(entries []Entry) []string {
	missing := make([]string, 0)
	for _, entry := range entries {
		isMissingText := entry.Hook == "" || entry.Does == ""
		if isMissingText {
			missing = append(missing, string(entry.Feature))
		}
	}
	return missing
}

func featureNames(features []Feature) []string {
	names := make([]string, 0, len(features))
	for _, feature := range features {
		names = append(names, string(feature))
	}
	return names
}

func saveThenLoad(savePath string, saved *SessionState, loadPath string) (*SessionState, error) {
	if saved != nil {
		if err := Save(savePath, saved); err != nil {
			return nil, err
		}
	}
	return Load(loadPath)
}
