package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installRecordPath is install.json in the state dir of env's Claude home.
func installRecordPath(env install.Env) string {
	return filepath.Join(env.ClaudeHome, "smine", config.StateInstallFile)
}

// minimalInstallRecord is a valid stored record of the Minimal preset.
func minimalInstallRecord(goos string) install.Record {
	record := install.FullRecord(goos)
	record.Components = components.Minimal()
	record.Preset = install.PresetMinimal
	return record
}

// newInstallationServer builds a server whose Claude home is a temp dir, so
// the installation record lives at <temp>/smine/install.json.
func newInstallationServer(t *testing.T, isDevInstance bool) *Server {
	t.Helper()
	dir := t.TempDir()
	return newTestServer(t, &Options{
		IsDevInstance:    isDevInstance,
		PresentationPath: filepath.Join(dir, "presentation-profile.md"),
		RepoDir:          t.TempDir(),
		SettingsPath:     filepath.Join(dir, "settings.json"),
		StylePath:        filepath.Join(dir, "style-profile.md"),
	})
}

// patternMatches reports for every pattern whether it matches body.
func patternMatches(body string, patterns map[string]bool) map[string]bool {
	matches := make(map[string]bool, len(patterns))
	for pattern := range patterns {
		matches[pattern] = regexp.MustCompile(pattern).MatchString(body)
	}
	return matches
}

func writeInstallFile(t *testing.T, env install.Env, data []byte) {
	t.Helper()
	path := installRecordPath(env)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

func TestFormRecord(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      install.Record
		_expectedNotes []string

		form   url.Values
		stored install.Record
	}

	goos := "darwin"
	allSkills := install.Selection{Mode: skills.SelectionModeAll}
	full := install.FullRecord(goos)
	minimal := minimalInstallRecord(goos)
	formWith := func(mode string, chosen []string, ids ...components.ID) url.Values {
		form := url.Values{"skill": chosen, "skills_mode": {mode}}
		for _, id := range ids {
			form.Add("component", string(id))
		}
		return form
	}
	tests := make([]*testCase, 0)

	// tick-context-adds-session-hooks-and-gate
	tests = append(tests, &testCase{
		_id: "tick-context-adds-session-hooks-and-gate",
		_expected: install.Record{
			Applied:    true,
			Components: []components.ID{components.ConfigServer, components.Skills, components.SessionHooks, components.Context, components.Gate},
			Preset:     install.PresetCustom,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, nil, components.Skills, components.Context),
		stored: minimal,
	})

	// untick-session-hooks-removes-dependents
	hooked := install.Record{
		Components: []components.ID{
			components.ConfigServer,
			components.Skills,
			components.SessionHooks,
			components.Context,
			components.Gate,
			components.Worktrees,
			components.SessionHelpers,
			components.Heartbeat,
		},
		Preset: install.PresetCustom,
		Schema: full.Schema,
		Skills: allSkills,
	}
	tests = append(tests, &testCase{
		_id: "untick-session-hooks-removes-dependents",
		_expected: install.Record{
			Components: components.Minimal(),
			Preset:     install.PresetMinimal,
			Schema:     full.Schema,
			Skills:     allSkills,
			Suspended: install.Suspended{Components: []components.ID{
				components.Context,
				components.Gate,
				components.Worktrees,
				components.SessionHelpers,
				components.Heartbeat,
			}},
		},
		_expectedNotes: []string{"turning session-hooks off also suspends context, gate, worktrees, session-helpers, heartbeat"},

		form: formWith(
			skills.SelectionModeAll,
			nil,
			components.Skills,
			components.Context,
			components.Gate,
			components.Worktrees,
			components.SessionHelpers,
			components.Heartbeat,
		),
		stored: hooked,
	})

	// config-server-always-present
	tests = append(tests, &testCase{
		_id: "config-server-always-present",
		_expected: install.Record{
			Applied:    true,
			Components: []components.ID{components.ConfigServer},
			Preset:     install.PresetCustom,
			Schema:     full.Schema,
			Skills:     install.Selection{Mode: skills.SelectionModeNone},
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeNone, nil),
		stored: minimal,
	})

	// full-set-preset-full
	tests = append(tests, &testCase{
		_id: "full-set-preset-full",
		_expected: install.Record{
			Applied:    true,
			Components: components.Full(goos),
			Preset:     install.PresetFull,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, nil, components.Full(goos)[1:]...),
		stored: minimal,
	})

	// minimal-set-preset-minimal
	tests = append(tests, &testCase{
		_id: "minimal-set-preset-minimal",
		_expected: install.Record{
			Applied:    true,
			Components: components.Minimal(),
			Preset:     install.PresetMinimal,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, nil, components.Skills),
		stored: full,
	})

	// mode-chosen-keeps-trimmed-leaves
	tests = append(tests, &testCase{
		_id: "mode-chosen-keeps-trimmed-leaves",
		_expected: install.Record{
			Applied:    true,
			Components: components.Minimal(),
			Preset:     install.PresetMinimal,
			Schema:     full.Schema,
			Skills:     install.Selection{Chosen: []string{"concept", "fdesign"}, Mode: skills.SelectionModeChosen},
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeChosen, []string{" concept ", "", "fdesign "}, components.Skills),
		stored: minimal,
	})

	// mode-all-drops-picked-skills
	tests = append(tests, &testCase{
		_id: "mode-all-drops-picked-skills",
		_expected: install.Record{
			Applied:    true,
			Components: components.Minimal(),
			Preset:     install.PresetMinimal,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, []string{"concept"}, components.Skills),
		stored: minimal,
	})

	// confirmed-tick-marks-the-selection
	confirmedForm := formWith(skills.SelectionModeChosen, []string{"concept"}, components.Skills)
	confirmedForm.Set("skills_confirmed", "1")
	tests = append(tests, &testCase{
		_id: "confirmed-tick-marks-the-selection",
		_expected: install.Record{
			Applied:    true,
			Components: components.Minimal(),
			Preset:     install.PresetMinimal,
			Schema:     full.Schema,
			Skills:     install.Selection{Chosen: []string{"concept"}, IsConfirmed: true, Mode: skills.SelectionModeChosen},
		},
		_expectedNotes: nil,

		form:   confirmedForm,
		stored: minimal,
	})

	// unknown-id-kept-for-validate — catalog ids in catalog order, then the unknown id
	tests = append(tests, &testCase{
		_id: "unknown-id-kept-for-validate",
		_expected: install.Record{
			Applied:    true,
			Components: []components.ID{components.ConfigServer, components.Skills, "bogus"},
			Preset:     install.PresetCustom,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, nil, components.Skills, "bogus"),
		stored: minimal,
	})

	// keeps-settings-variables-providers
	kept := minimalInstallRecord(goos)
	kept.Retired = []components.ID{"retired-one"}
	kept.Settings = install.Settings{components.Context: {"global_inline_cap": json.RawMessage(`1000`)}}
	kept.Skills = install.Selection{
		Mode:      skills.SelectionModeAll,
		Providers: map[string]string{"PROVIDER_SMINE_RULES": "my-rules"},
		Suspended: []string{"done"},
		Variables: map[string]string{"SKILL_ENVSKILL_BUILD": "make build"},
	}
	kept.Updated = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	keptExpected := kept
	keptExpected.Skills.Chosen = []string{"concept"}
	keptExpected.Skills.Mode = skills.SelectionModeChosen
	tests = append(tests, &testCase{
		_id:            "keeps-settings-variables-providers",
		_expected:      keptExpected,
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeChosen, []string{"concept"}, components.Skills),
		stored: kept,
	})

	// suspends-dependents
	tests = append(tests, &testCase{
		_id: "suspends-dependents",
		_expected: install.Record{
			Components: components.Close([]components.ID{
				components.ConfigServer,
				components.Skills,
				components.SessionHooks,
				components.Worktrees,
				components.SessionHelpers,
				components.Heartbeat,
			}),
			Preset:    install.PresetCustom,
			Schema:    full.Schema,
			Skills:    allSkills,
			Suspended: install.Suspended{Components: []components.ID{components.Context}},
		},
		_expectedNotes: []string{"turning gate off also suspends context"},

		form: formWith(
			skills.SelectionModeAll,
			nil,
			components.Skills,
			components.SessionHooks,
			components.Context,
			components.Worktrees,
			components.SessionHelpers,
			components.Heartbeat,
		),
		stored: hooked,
	})

	// restores-suspended-on-retick — context needs gate, so the restore runs until nothing more returns
	suspended := minimalInstallRecord(goos)
	suspended.Suspended = install.Suspended{Components: []components.ID{components.Context, components.Gate}}
	tests = append(tests, &testCase{
		_id: "restores-suspended-on-retick",
		_expected: install.Record{
			Applied:    true,
			Components: []components.ID{components.ConfigServer, components.Skills, components.SessionHooks, components.Context, components.Gate},
			Preset:     install.PresetCustom,
			Schema:     full.Schema,
			Skills:     allSkills,
		},
		_expectedNotes: nil,

		form:   formWith(skills.SelectionModeAll, nil, components.Skills, components.SessionHooks),
		stored: suspended,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, notes := formRecord(test.form, test.stored, goos)

			assert.Equal(t, test._expected, record)
			assert.Equal(t, test._expectedNotes, notes)
		})
	}
}

func TestDropUnavailableSkills(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedChosen    []string
		_expectedNotes     []string
		_expectedSuspended []string

		record *install.Record
	}

	list := []skills.Skill{
		{Name: "concept"},
		{Name: "done", Requires: []components.ID{components.Agentwt, components.Heartbeat}},
		{Name: "peek", Requires: []components.ID{components.Peek}},
	}
	withPeek := []components.ID{components.ConfigServer, components.Skills, components.Peek}
	suspendedNote := "skill done needs a component that is off — suspended until it returns"
	tests := make([]*testCase, 0)

	// suspends-unavailable-keeps-unknown
	tests = append(tests, &testCase{
		_id:                "suspends-unavailable-keeps-unknown",
		_expectedChosen:    []string{"concept", "peek", "unknown"},
		_expectedNotes:     []string{suspendedNote},
		_expectedSuspended: []string{"done"},

		record: &install.Record{
			Components: withPeek,
			Skills:     install.Selection{Chosen: []string{"concept", "done", "peek", "unknown"}, Mode: skills.SelectionModeChosen},
		},
	})

	// suspends-and-restores
	tests = append(tests, &testCase{
		_id:                "suspends-and-restores",
		_expectedChosen:    []string{"concept", "peek"},
		_expectedNotes:     []string{"skill peek restored — its components are back", suspendedNote},
		_expectedSuspended: []string{"done"},

		record: &install.Record{
			Components: withPeek,
			Skills:     install.Selection{Chosen: []string{"concept", "done"}, Mode: skills.SelectionModeChosen, Suspended: []string{"peek"}},
		},
	})

	// mode-all-restores-nothing
	tests = append(tests, &testCase{
		_id:                "mode-all-restores-nothing",
		_expectedChosen:    nil,
		_expectedNotes:     nil,
		_expectedSuspended: []string{"peek"},

		record: &install.Record{
			Components: withPeek,
			Skills:     install.Selection{Mode: skills.SelectionModeAll, Suspended: []string{"peek"}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			notes := dropUnavailableSkills(test.record, list)

			assert.Equal(t, test._expectedNotes, notes)
			assert.Equal(t, test._expectedChosen, test.record.Skills.Chosen)
			assert.Equal(t, test._expectedSuspended, test.record.Skills.Suspended)
		})
	}
}

func TestNewComponentRow(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedDetail   string
		_expectedFix      string
		_expectedRequires []string
		_expectedState    string

		goos   string
		status install.ComponentStatus
	}

	heartbeat, _ := components.Lookup(components.Heartbeat)
	mining, _ := components.Lookup(components.Mining)
	token, _ := components.Lookup(components.Token)
	tests := make([]*testCase, 0)

	// unavailable-wins-over-selected
	tests = append(tests, &testCase{
		_id:               "unavailable-wins-over-selected",
		_expectedDetail:   "not available on linux",
		_expectedFix:      "",
		_expectedRequires: []string{"Session hooks"},
		_expectedState:    componentUnavailable,

		goos: "linux",
		status: install.ComponentStatus{
			Available: false,
			Component: heartbeat,
			Health:    install.Health{Detail: "daemon answers", Ok: true},
			Selected:  true,
		},
	})

	// off
	tests = append(tests, &testCase{
		_id:               "off",
		_expectedDetail:   "",
		_expectedFix:      "",
		_expectedRequires: nil,
		_expectedState:    componentOff,

		goos: "darwin",
		status: install.ComponentStatus{
			Available: true,
			Component: token,
			Health:    install.Health{},
			Selected:  false,
		},
	})

	// on-with-detail
	tests = append(tests, &testCase{
		_id:               "on-with-detail",
		_expectedDetail:   "token file present",
		_expectedFix:      "",
		_expectedRequires: nil,
		_expectedState:    componentOn,

		goos: "darwin",
		status: install.ComponentStatus{
			Available: true,
			Component: token,
			Health:    install.Health{Detail: "token file present", Fix: "unused", Ok: true},
			Selected:  true,
		},
	})

	// down-with-detail-and-fix
	tests = append(tests, &testCase{
		_id:               "down-with-detail-and-fix",
		_expectedDetail:   "no token file",
		_expectedFix:      "save a token on Profile",
		_expectedRequires: nil,
		_expectedState:    componentDown,

		goos: "darwin",
		status: install.ComponentStatus{
			Available: true,
			Component: token,
			Health:    install.Health{Detail: "no token file", Fix: "save a token on Profile", Ok: false},
			Selected:  true,
		},
	})

	// requires-mapped-to-titles
	tests = append(tests, &testCase{
		_id:               "requires-mapped-to-titles",
		_expectedDetail:   "",
		_expectedFix:      "",
		_expectedRequires: []string{"peek", "Context system"},
		_expectedState:    componentOff,

		goos: "darwin",
		status: install.ComponentStatus{
			Available: true,
			Component: mining,
			Health:    install.Health{},
			Selected:  false,
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			row := newComponentRow(test.status, test.goos)

			assert.Equal(t, test._expectedState, row.State)
			assert.Equal(t, test._expectedDetail, row.Detail)
			assert.Equal(t, test._expectedFix, row.Fix)
			assert.Equal(t, test._expectedRequires, row.Requires)
		})
	}
}

func TestRecordCache(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedComponents []components.ID
		_shouldPass         bool

		cache *recordCache
		env   install.Env
	}

	goos := runtime.GOOS
	tests := make([]*testCase, 0)

	// missing-file-full
	tests = append(tests, &testCase{
		_id:                 "missing-file-full",
		_expectedComponents: components.Full(goos),
		_shouldPass:         true,

		cache: &recordCache{},
		env:   install.Env{ClaudeHome: t.TempDir(), GOOS: goos},
	})

	// written-record-read
	env := install.Env{ClaudeHome: t.TempDir(), GOOS: goos}
	require.NoError(t, install.Save(env, minimalInstallRecord(goos)))
	tests = append(tests, &testCase{
		_id:                 "written-record-read",
		_expectedComponents: components.Minimal(),
		_shouldPass:         true,

		cache: &recordCache{},
		env:   env,
	})

	// rewrite-with-new-mtime-re-read
	env = install.Env{ClaudeHome: t.TempDir(), GOOS: goos}
	require.NoError(t, install.Save(env, minimalInstallRecord(goos)))
	info, err := os.Stat(installRecordPath(env))
	require.NoError(t, err)
	cache := &recordCache{}
	loaded, err := cache.load(env)
	require.NoError(t, err)
	rewritten := install.FullRecord(goos)
	rewritten.Updated = loaded.Updated
	require.NoError(t, install.Save(env, rewritten))
	movedTime := info.ModTime().Add(time.Hour)
	require.NoError(t, os.Chtimes(installRecordPath(env), movedTime, movedTime))
	tests = append(tests, &testCase{
		_id:                 "rewrite-with-new-mtime-re-read",
		_expectedComponents: components.Full(goos),
		_shouldPass:         true,

		cache: cache,
		env:   env,
	})

	// same-mtime-served-from-cache
	env = install.Env{ClaudeHome: t.TempDir(), GOOS: goos}
	require.NoError(t, install.Save(env, minimalInstallRecord(goos)))
	info, err = os.Stat(installRecordPath(env))
	require.NoError(t, err)
	cache = &recordCache{}
	loaded, err = cache.load(env)
	require.NoError(t, err)
	rewritten = install.FullRecord(goos)
	rewritten.Updated = loaded.Updated
	require.NoError(t, install.Save(env, rewritten))
	require.NoError(t, os.Chtimes(installRecordPath(env), info.ModTime(), info.ModTime()))
	tests = append(tests, &testCase{
		_id:                 "same-mtime-served-from-cache",
		_expectedComponents: components.Minimal(),
		_shouldPass:         true,

		cache: cache,
		env:   env,
	})

	// invalid-json-error-and-empty
	env = install.Env{ClaudeHome: t.TempDir(), GOOS: goos}
	writeInstallFile(t, env, []byte("{not json"))
	tests = append(tests, &testCase{
		_id:                 "invalid-json-error-and-empty",
		_expectedComponents: nil,
		_shouldPass:         false,

		cache: &recordCache{},
		env:   env,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := test.cache.load(test.env)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedComponents, record.Components)
		})
	}
}

func TestRecordCacheHasToleratesBrokenFile(t *testing.T) {
	env := install.Env{ClaudeHome: t.TempDir(), GOOS: runtime.GOOS}
	writeInstallFile(t, env, []byte("{not json"))
	cache := &recordCache{}

	assert.True(t, cache.has(env, components.Skills), "page gating keeps the Full fallback")
}

func TestInstallationRoutes(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedStatus int

		form   url.Values
		path   string
		server *Server
	}

	t.Setenv("HOME", t.TempDir())
	chosenEmpty := url.Values{
		"component":   {string(components.Skills)},
		"skills_mode": {skills.SelectionModeChosen},
	}
	tests := make([]*testCase, 0)

	// dev-instance-preview
	tests = append(tests, &testCase{
		_id:             "dev-instance-preview",
		_expectedBody:   readOnlyInstallation,
		_expectedStatus: http.StatusBadRequest,

		form:   chosenEmpty,
		path:   "/profile/installation/preview",
		server: newInstallationServer(t, true),
	})

	// dev-instance-apply
	tests = append(tests, &testCase{
		_id:             "dev-instance-apply",
		_expectedBody:   readOnlyInstallation,
		_expectedStatus: http.StatusBadRequest,

		form:   chosenEmpty,
		path:   "/profile/installation/apply",
		server: newInstallationServer(t, true),
	})

	// preview-mode-chosen-empty-error-card
	tests = append(tests, &testCase{
		_id:             "preview-mode-chosen-empty-error-card",
		_expectedBody:   `<div class="card card-error"><span class="label">Validate: validateSkills: Mode chosen lists no skill</span></div>`,
		_expectedStatus: http.StatusOK,

		form:   chosenEmpty,
		path:   "/profile/installation/preview",
		server: newInstallationServer(t, false),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost(test.path, test.form))

			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
		})
	}
}

func TestProfileShowsInstallation(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedMatches map[string]bool

		server *Server
	}

	t.Setenv("HOME", t.TempDir())
	previewButton := `<button type="submit">Preview changes</button>`
	skillsDisabled := `value="skills"\s*(checked)?\s*disabled`
	tests := make([]*testCase, 0)

	// rows-for-every-catalog-id
	rowMatches := map[string]bool{previewButton: true, skillsDisabled: false}
	for _, component := range components.Catalog() {
		rowMatches[regexp.QuoteMeta(`id="installation-`+string(component.ID)+`"`)] = true
	}
	tests = append(tests, &testCase{
		_id:              "rows-for-every-catalog-id",
		_expectedMatches: rowMatches,

		server: newInstallationServer(t, false),
	})

	// dev-instance-inputs-disabled-no-preview
	server := newInstallationServer(t, true)
	require.NoError(t, install.Save(server.machine, minimalInstallRecord(server.machine.GOOS)))
	tests = append(tests, &testCase{
		_id: "dev-instance-inputs-disabled-no-preview",
		_expectedMatches: map[string]bool{
			`name="skills_mode"\s*disabled`: true,
			previewButton:                   false,
			skillsDisabled:                  true,
		},

		server: server,
	})

	// stored-minimal-peek-unchecked
	server = newInstallationServer(t, false)
	require.NoError(t, install.Save(server.machine, minimalInstallRecord(server.machine.GOOS)))
	tests = append(tests, &testCase{
		_id: "stored-minimal-peek-unchecked",
		_expectedMatches: map[string]bool{
			`value="peek"\s*checked`:   false,
			`value="skills"\s*checked`: true,
		},

		server: server,
	})

	// chosen-skills-picked-and-confirmed
	server = newInstallationServer(t, false)
	server.skillsRepo = writeSkillRoot(t, "concept", "fdesign")
	chosen := minimalInstallRecord(server.machine.GOOS)
	chosen.Skills = install.Selection{Chosen: []string{"fdesign"}, IsConfirmed: true, Mode: skills.SelectionModeChosen}
	require.NoError(t, install.Save(server.machine, chosen))
	tests = append(tests, &testCase{
		_id: "chosen-skills-picked-and-confirmed",
		_expectedMatches: map[string]bool{
			`name="skill" value="concept"\s*onchange`:          true,
			`name="skill" value="fdesign" checked`:             true,
			`name="skills_confirmed" checked`:                  true,
			`<option value="chosen" selected>`:                 true,
			`<details class="reach-picker skills-pick">`:       true,
			`<details class="reach-picker skills-pick" hidden`: false,
		},

		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, test._expectedMatches, patternMatches(recorder.Body.String(), test._expectedMatches))
		})
	}
}

func TestInstallationPage(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedMatches map[string]bool

		server *Server
	}

	t.Setenv("HOME", t.TempDir())
	applyPending := regexp.QuoteMeta("The last apply did not finish — apply again")
	historyList := `Record history \(1\)`
	keptUnknown := "Kept but unknown"
	suspended := "Suspended until their components return"
	tests := make([]*testCase, 0)

	// history-listed
	server := newInstallationServer(t, false)
	require.NoError(t, install.Save(server.machine, minimalInstallRecord(server.machine.GOOS)))
	resaveRecord(t, server)
	tests = append(tests, &testCase{
		_id: "history-listed",
		_expectedMatches: map[string]bool{
			`hx-get="/profile/record-history/[0-9T.-]+Z" hx-target="#record-history"`: true,
			applyPending: false,
			historyList:  true,
			keptUnknown:  false,
			suspended:    false,
		},

		server: server,
	})

	// notes-and-suspended-rendered
	server = newInstallationServer(t, false)
	noted := minimalInstallRecord(server.machine.GOOS)
	noted.Retired = []components.ID{"ghost-component"}
	noted.Suspended.Components = []components.ID{components.Peek}
	noted.Skills.Suspended = []string{"fdesign"}
	require.NoError(t, install.Save(server.machine, noted))
	tests = append(tests, &testCase{
		_id: "notes-and-suspended-rendered",
		_expectedMatches: map[string]bool{
			"component ghost-component: no longer in the catalog, kept": true,
			"<code>peek</code>":          true,
			"<code>skill fdesign</code>": true,
			applyPending:                 false,
			historyList:                  false,
			keptUnknown:                  true,
			suspended:                    true,
		},

		server: server,
	})

	// apply-pending-notice
	server = newInstallationServer(t, false)
	pending := minimalInstallRecord(server.machine.GOOS)
	pending.Applied = false
	require.NoError(t, install.Save(server.machine, pending))
	tests = append(tests, &testCase{
		_id: "apply-pending-notice",
		_expectedMatches: map[string]bool{
			applyPending: true,
			historyList:  false,
			keptUnknown:  false,
			suspended:    false,
		},

		server: server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile", nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, test._expectedMatches, patternMatches(recorder.Body.String(), test._expectedMatches))
		})
	}
}

func TestRecordHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	server := newInstallationServer(t, false)
	require.NoError(t, install.Save(server.machine, minimalInstallRecord(server.machine.GOOS)))
	resaveRecord(t, server)
	path := install.RecordPath(server.machine)
	history, err := install.History(path)
	require.NoError(t, err)
	require.Len(t, history, 1)
	stamp := strings.TrimPrefix(history[0], path+".")

	t.Run("diff-rendered-for-a-stamp", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile/record-history/"+stamp, nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "install.json."+stamp)
		assert.Contains(t, recorder.Body.String(), `<span class="diff-del">-   &#34;preset&#34;: &#34;minimal&#34;,</span>`)
		assert.Contains(t, recorder.Body.String(), `<span class="diff-add">&#43;   &#34;preset&#34;: &#34;full&#34;,</span>`)
	})

	t.Run("unknown-stamp-404", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/profile/record-history/2000-01-01T00-00-00.000000000Z", nil))
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}

// resaveRecord loads the server's install.json and saves it again with the Full preset, so one history file holds the previous bytes.
func resaveRecord(t *testing.T, server *Server) {
	t.Helper()
	record, _, err := install.Load(server.machine)
	require.NoError(t, err)
	record.Components = components.Full(server.machine.GOOS)
	record.Preset = install.PresetFull
	require.NoError(t, install.Save(server.machine, record))
}
