package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessions"
)

func TestOverview(t *testing.T) {
	// The routine and backlog tiles render only where the Routines component can run, the proposals tile where Mining can (darwin, windows).
	hasRoutines := components.Available(components.Routines, runtime.GOOS)
	hasProposals := components.Available(components.Mining, runtime.GOOS)

	t.Run("tiles-render-with-fixtures", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")
		require.NoError(t, os.WriteFile(settingsPath, []byte(`{
			"model": "opus",
			"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]},
			"disabledMcpjsonServers": ["idle-server"]
		}`), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.disabled.json"), []byte(`{}`), 0644))
		// parked-count-from-sidecar: the tile counts the groups of hooks.disabled.json
		require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.disabled.json"),
			[]byte(`{"Stop": [{"hooks": [{"type": "command", "command": "parked"}]}]}`), 0644))

		claudeJsonPath := filepath.Join(dir, ".claude.json")
		require.NoError(t, os.WriteFile(claudeJsonPath, []byte(`{
			"mcpServers": {"goland": {}, "idle-server": {}}
		}`), 0644))

		backlogDir := writeBacklogStore(t, backlogFixture)

		skillsRepo := filepath.Join(dir, "skills-repo")
		writeRepoSkill(t, skillsRepo, "jq", "1.0")
		require.NoError(t, os.MkdirAll(filepath.Join(skillsRepo, "jq", "workflows"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, "jq", "workflows", "run.js"),
			[]byte("export const meta = {}\n"), 0644))

		contextDir := filepath.Join(dir, "context")
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "actions"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "rules"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "actions", "navigation.md"), []byte("# x\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "rules", "go.md"), []byte("# x\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(contextDir, "context.json"),
			[]byte(`{"entries": [{"id": "ACTION-NAV-001", "kind": "action", "scope": "NAV"}, {"id": "RULE-NAV-001", "kind": "rule", "scope": "NAV"}], "aspects": []}`), 0644))

		sessionsDir := filepath.Join(dir, "sessions")
		writeOverviewSessions(t, sessionsDir)

		server := newTestServer(t, &Options{
			BacklogDir:     backlogDir,
			ClaudeJsonPath: claudeJsonPath,
			ContextDir:     contextDir,
			SessionsDir:    sessionsDir,
			SettingsPath:   settingsPath,
			SkillsHome:     filepath.Join(dir, "skills-home"),
			SkillsRepo:     skillsRepo,
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, "Sessions analyzed")
		assert.Contains(t, body, "1/2 enabled") // hooks tile: one live, one parked
		assert.Contains(t, body, "disabled: 1 hooks · 0 permissions · 1 MCP")
		assert.NotContains(t, body, "Disabled entries")
		assert.Contains(t, body, "1/2 enabled")            // MCP tile: idle-server disabled in live list
		assert.Contains(t, body, "goland")                 // MCP detail
		assert.Contains(t, body, "Frustration / Positive") // merged entries card
		assert.Contains(t, body, `class="split-frustration"`)
		assert.Contains(t, body, `class="split-positive"`)
		assert.NotContains(t, body, "Positive entries")
		assert.Equal(t, hasProposals, strings.Contains(body, "Proposals"))                                                  // proposals tile
		assert.Equal(t, hasProposals, strings.Contains(body, ">0 open<"))                                                   // no nightly repo, no change commits
		assert.Equal(t, hasProposals, strings.Contains(body, "open change commits — pick or reject on the Proposals page")) // proposals tile detail
		assert.NotContains(t, body, `data-tile-id="model"`)
		assert.NotContains(t, body, `data-tile-id="last-analysis"`)
		assert.Equal(t, hasRoutines, strings.Contains(body, `data-tile-id="usage"`)) // usage tile: empty store, no weighed session
		assert.Equal(t, hasRoutines, strings.Contains(body, `<form class="tile-plan" method="post" action="/usage/plan">`))
		assert.Equal(t, hasRoutines, strings.Contains(body, `<option value="team-premium" selected>Team Premium</option>`))
		assert.Equal(t, hasRoutines, strings.Contains(body, `href="/usage"`))
		assert.Equal(t, hasRoutines, strings.Contains(body, `class="tile-filter-link tile-filter-active" href="/">7 days<`))
		assert.Equal(t, hasRoutines, strings.Contains(body, `href="/?usage=1">today<`))
		assert.Equal(t, hasRoutines, strings.Contains(body, `href="/?usage=30">30 days<`))
		assert.Contains(t, body, "2 weeks · last 2026-07-21") // sessions tile: W29 + W30, newest session 2026-07-21
		assert.Contains(t, body, `id="sync-form" hx-post="/sync"`)
		assert.Contains(t, body, `id="sync-all" name="target" value="all" checked`)
		assert.Contains(t, body, `name="target" value="hooks" checked`)
		assert.Contains(t, body, `name="target" value="settings" checked`)
		assert.Contains(t, body, `name="target" value="skills" checked`)
		assert.NotContains(t, body, "Skills out of sync")
		assert.Contains(t, body, ">0/1<")                                       // merged skills tile: 0 active / 1 repo
		assert.Contains(t, body, "active / repo · 1 out of sync · 1 workflows") // workflows folded into the skills tile
		assert.NotContains(t, body, `data-tile-id="workflows"`)
		assert.Contains(t, body, "2 entries") // context tile: entry count + gate coverage
		assert.Contains(t, body, "coverage")
		assert.Contains(t, body, "1 scopes ·") // context tile: distinct scope count
		assert.NotContains(t, body, `data-tile-id="tools"`)
		assert.Contains(t, body, `data-tile-id="skills"`)
		assert.Contains(t, body, "Repos") // repo tile (empty registry)
		assert.Contains(t, body, "0 synced · 0 worktrees")
		assert.Equal(t, hasRoutines, strings.Contains(body, "Routines")) // routine tile (no routines dir)
		assert.Equal(t, hasRoutines, strings.Contains(body, "0/0 active"))
		assert.Equal(t, hasRoutines, strings.Contains(body, "4 active"))                   // backlog tile: fixture rows
		assert.Equal(t, hasRoutines, strings.Contains(body, "3 critical · 1 in pipeline")) // backlog tile detail
		assert.Contains(t, body, `href="/sessions/personal/2026-W30"`)                     // sessions tile link: week 30 holds the newest session
		assert.Contains(t, body, `href="/sessions/personal/2026-W30?dimension=frustration&amp;open=ghi789#session-ghi789"`)
		assert.Contains(t, body, `href="/sessions/personal/2026-W29?dimension=positive&amp;open=def456#session-def456"`)
		// merged card: newest signal date joined with the trend delta;
		// template escapes the plus
		assert.Contains(t, body, "last signal 2026-07-21 · &#43;1 vs week 2026-W29")

		// default row order: sessions row, config row, repo row, orga row
		assert.Less(t, strings.Index(body, `data-tile-id="sessions"`), strings.Index(body, `data-tile-id="hooks"`))
		assert.Less(t, strings.Index(body, `data-tile-id="hooks"`), strings.Index(body, `data-tile-id="skills"`))
		assert.Equal(t, hasProposals, strings.Index(body, `data-tile-id="skills"`) < strings.Index(body, `data-tile-id="proposals"`))
		assert.NotContains(t, body, "Sentiment trend") // chart lives inside the split card now
		assert.Contains(t, body, `<rect class="trend-frustration"`)
		assert.Contains(t, body, `<rect class="trend-positive"`)
		assert.NotContains(t, body, "<polyline")
		// window filter row: default 10 active (links to the bare URL), others carry the param
		assert.Contains(t, body, `class="tile-filter-link tile-filter-active" href="/"`)
		assert.Contains(t, body, `href="/?window=last"`)
		assert.Contains(t, body, `href="/?window=all"`)
	})

	t.Run("window-filters-counts", func(t *testing.T) {
		dir := t.TempDir()
		sessionsDir := filepath.Join(dir, "sessions")
		writeOverviewSessions(t, sessionsDir)
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, backlogFixture),
			SessionsDir: sessionsDir,
		})

		// last: only batch 2 counts (2 frustration, 0 positive)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?window=last", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `#session-ghi789">2</a>`)
		assert.Contains(t, body, `#session-def456">0</a>`)

		// all: both batches (3 frustration, 1 positive) — the pre-filter totals
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?window=all", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, `#session-ghi789">3</a>`)
		assert.Contains(t, body, `#session-def456">1</a>`)
	})

	t.Run("filters-keep-each-other", func(t *testing.T) {
		dir := t.TempDir()
		sessionsDir := filepath.Join(dir, "sessions")
		writeOverviewSessions(t, sessionsDir)
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, backlogFixture),
			SessionsDir: sessionsDir,
		})
		pinRoutinesHost(server)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?usage=30&window=last", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `class="tile-filter-link tile-filter-active" href="/?usage=30&amp;window=last">30 days<`)
		assert.Contains(t, body, `href="/?window=last">7 days<`)
		assert.Contains(t, body, `href="/?usage=30">10<`)
		assert.Contains(t, body, `href="/usage?days=30"`)
	})

	t.Run("unknown-usage-falls-back-to-default", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), SessionsDir: t.TempDir()})
		pinRoutinesHost(server)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?usage=90", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `class="tile-filter-link tile-filter-active" href="/">7 days<`)
	})

	t.Run("missing-backlog-renders-empty-tile", func(t *testing.T) {
		// A fresh clone has no backlog store (private artifact) — the tile
		// shows zero items instead of degrading to an error tile.
		server := newTestServer(t, &Options{
			BacklogDir:  filepath.Join(t.TempDir(), "nope"),
			SessionsDir: t.TempDir(),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, hasRoutines, strings.Contains(response.Body.String(), "0 active"))
		assert.NotContains(t, response.Body.String(), "no such file")
	})

	t.Run("broken-backlog-degrades-to-error-tile", func(t *testing.T) {
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, `{"sections": [`),
			SessionsDir: t.TempDir(),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, hasRoutines, strings.Contains(response.Body.String(), "store format"))
	})

	t.Run("empty-everything-still-200", func(t *testing.T) {
		dir := t.TempDir()
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, backlogFixture),
			SessionsDir: filepath.Join(dir, "no-sessions"),
			SkillsHome:  filepath.Join(dir, "no-home"),
			SkillsRepo:  filepath.Join(dir, "no-repo"),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "0/0 enabled")
		assert.Contains(t, response.Body.String(), "disabled: 0 hooks · 0 permissions · 0 MCP")
		// no batches → no chart, no filter row and no analyzedDate detail on
		// the merged card
		assert.NotContains(t, response.Body.String(), "tile-trend")
		assert.NotContains(t, response.Body.String(), "window=last") // no batches → no window filter row; the usage row stays
		assert.NotContains(t, response.Body.String(), "last signal")
	})
}

func TestTrendBars(t *testing.T) {
	bars := trendBars([]int{6, 12}, 12, []int{3, 0})

	require.Len(t, bars, 4)
	// two batches → slot 50; frustration at 0.1·slot, positive at 0.55·slot
	assert.Equal(t, trendBar{Class: "trend-frustration", Height: "12.0", Width: "17.5", X: "5.0", Y: "14.0"}, bars[0])
	assert.Equal(t, trendBar{Class: "trend-positive", Height: "6.0", Width: "17.5", X: "27.5", Y: "20.0"}, bars[1])
	// maxValue bar spans the full padded height
	assert.Equal(t, trendBar{Class: "trend-frustration", Height: "24.0", Width: "17.5", X: "55.0", Y: "2.0"}, bars[2])
	// zero value → zero-height rect
	assert.Equal(t, trendBar{Class: "trend-positive", Height: "0.0", Width: "17.5", X: "77.5", Y: "26.0"}, bars[3])
}

func TestWindowPoints(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFirst int
		_expectedLen   int

		points []sessions.SentimentPoint
		window string
	}

	tests := make([]*testCase, 0)

	// last-of-three
	tests = append(tests, &testCase{
		_id:            "last-of-three",
		_expectedFirst: 3,
		_expectedLen:   1,

		points: provideSentimentPoints(3),
		window: windowLast,
	})

	// ten-of-twelve
	tests = append(tests, &testCase{
		_id:            "ten-of-twelve",
		_expectedFirst: 3,
		_expectedLen:   10,

		points: provideSentimentPoints(12),
		window: windowTen,
	})

	// all-unchanged
	tests = append(tests, &testCase{
		_id:            "all-unchanged",
		_expectedFirst: 1,
		_expectedLen:   12,

		points: provideSentimentPoints(12),
		window: windowAll,
	})

	// short-series-last
	tests = append(tests, &testCase{
		_id:            "short-series-last",
		_expectedFirst: 1,
		_expectedLen:   1,

		points: provideSentimentPoints(1),
		window: windowLast,
	})

	// short-series-ten
	tests = append(tests, &testCase{
		_id:            "short-series-ten",
		_expectedFirst: 1,
		_expectedLen:   3,

		points: provideSentimentPoints(3),
		window: windowTen,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			windowed := windowPoints(test.points, test.window)

			assert.Len(t, windowed, test._expectedLen)
			assert.Equal(t, fmt.Sprintf("2026-W%02d", test._expectedFirst), windowed[0].Week)
		})
	}
}

// provideSentimentPoints builds count points for weeks 2026-W01..count.
func provideSentimentPoints(count int) []sessions.SentimentPoint {
	points := make([]sessions.SentimentPoint, 0, count)
	for number := 1; number <= count; number++ {
		points = append(points, sessions.SentimentPoint{Week: fmt.Sprintf("2026-W%02d", number)})
	}
	return points
}

// writeOverviewSessions writes the week fixture plus a newer week holding one
// frustration-only session.
func writeOverviewSessions(t *testing.T, sessionsDir string) {
	t.Helper()
	writeWeekFixture(t, sessionsDir, "personal")
	newerDir := filepath.Join(sessionsDir, "personal", "2026-W30")
	require.NoError(t, os.MkdirAll(newerDir, 0o755))
	newer := `{"id": "ghi789", "agent": "claude", "title": "Third session", "repo": "personal", "week": "2026-W30",
		"lastActive": "2026-07-21T10:00:00Z", "findings": [],
		"frustration": [{"quote": "no", "trigger": "t"}, {"quote": "still no", "trigger": "t"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(newerDir, "ghi789.json"), []byte(newer), 0o644))
}

func TestParseWindow(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		target string
	}

	tests := make([]*testCase, 0)

	// all
	tests = append(tests, &testCase{
		_id:       "all",
		_expected: windowAll,

		target: "/?window=all",
	})

	// last
	tests = append(tests, &testCase{
		_id:       "last",
		_expected: windowLast,

		target: "/?window=last",
	})

	// ten
	tests = append(tests, &testCase{
		_id:       "ten",
		_expected: windowTen,

		target: "/?window=10",
	})

	// absent-default
	tests = append(tests, &testCase{
		_id:       "absent-default",
		_expected: windowDefault,

		target: "/",
	})

	// garbage-default
	tests = append(tests, &testCase{
		_id:       "garbage-default",
		_expected: windowDefault,

		target: "/?window=nonsense",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			window := parseWindow(httptest.NewRequest(http.MethodGet, test.target, nil))

			assert.Equal(t, test._expected, window)
		})
	}
}

func TestOverviewGermanSignalStrings(t *testing.T) {
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	writeOverviewSessions(t, sessionsDir)
	options := &Options{PresentationPath: writeGermanProfile(t), SessionsDir: sessionsDir}
	server := newTestServer(t, options)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "2 Wochen")
	// the template escapes the plus
	assert.Contains(t, body, "letztes Signal 2026-07-21 · &#43;1 ggü. Woche 2026-W29")
	assert.NotContains(t, body, "vs week")
	assert.NotContains(t, body, "%!")
}

func TestOverviewCasual(t *testing.T) {
	server := newTestServer(t, &Options{PresentationPath: writeGermanProfile(t)})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Ausgewertete Sitzungen")
	assert.Contains(t, body, "Vorschläge")
	assert.NotContains(t, body, ">Backlog</span>")
	assert.NotContains(t, body, ">Repos</span>")
	assert.NotContains(t, body, ">Skills</span>")
	assert.NotContains(t, body, ">Hooks</span>")
}

func TestUsageTileText(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDetail string
		_expectedValue  string

		cost   string
		shares planShares
	}

	tests := make([]*testCase, 0)

	// nothing-weighed
	tests = append(tests, &testCase{
		_id:             "nothing-weighed",
		_expectedDetail: "",
		_expectedValue:  usageDash,

		cost:   usageDash,
		shares: planShares{claude: 2, weighed: 0},
	})

	// window-and-weekly-with-fable
	tests = append(tests, &testCase{
		_id:             "window-and-weekly-with-fable",
		_expectedDetail: "$714.87 API equivalent · Fable 25% weekly",
		_expectedValue:  "149% / 24%",

		cost:   "$714.87",
		shares: planShares{claude: 15, fable: 25.2, hasWeekly: true, weekly: 23.8, weighed: 13, window: 149.4},
	})

	// window-only-plan
	tests = append(tests, &testCase{
		_id:             "window-only-plan",
		_expectedDetail: "$12.00 API equivalent",
		_expectedValue:  "40%",

		cost:   "$12.00",
		shares: planShares{claude: 1, hasWeekly: false, weighed: 1, window: 40},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedValue, usagePlanValue(test.shares))
			assert.Equal(t, test._expectedDetail, joinDetail(usageCostDetail(test.cost), usageFableDetail(test.shares)))
		})
	}
}
