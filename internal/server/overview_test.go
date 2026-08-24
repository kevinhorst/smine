package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessions"
)

func TestOverview(t *testing.T) {
	t.Run("tiles-render-with-fixtures", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, "settings.json")
		require.NoError(t, os.WriteFile(settingsPath, []byte(`{
			"model": "opus",
			"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]},
			"disabledMcpjsonServers": ["idle-server"]
		}`), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.disabled.json"), []byte(`{}`), 0644))

		claudeJsonPath := filepath.Join(dir, ".claude.json")
		require.NoError(t, os.WriteFile(claudeJsonPath, []byte(`{
			"mcpServers": {"goland": {}, "idle-server": {}}
		}`), 0644))

		checklistPath := filepath.Join(dir, "checklist.md")
		require.NoError(t, os.WriteFile(checklistPath, []byte(checklistFixture), 0644))

		skillsRepo := filepath.Join(dir, "skills-repo")
		require.NoError(t, os.MkdirAll(filepath.Join(skillsRepo, "jq", "workflows"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, "jq", "SKILL.md"),
			[]byte("---\nname: jq\ndescription: d\n---\n"), 0644))
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
		jsonDir := filepath.Join(sessionsDir, "personal", "json")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch2.json"), []byte(`{
			"batch": {"scope": "personal", "number": 2, "file": "batch2.md", "analyzedDate": "2026-07-17"},
			"sessions": [{
				"id": "ghi789",
				"title": "Third session",
				"frustration": [
					{"quote": "no", "trigger": "t"},
					{"quote": "still no", "trigger": "t"}
				]
			}]
		}`), 0644))

		proposalsDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(proposalsDir, 0755))
		proposalsJSON := `{"kind": "skills", "groups": [{"title": "New skills", "proposals": [
			{"id": "a", "title": "a", "status": "proposed"},
			{"id": "b", "title": "b"},
			{"id": "c", "title": "c", "status": "accepted"},
			{"id": "d", "title": "d", "status": "rejected"}
		]}]}`
		require.NoError(t, os.WriteFile(filepath.Join(proposalsDir, "skills.json"), []byte(proposalsJSON), 0644))

		server := newTestServer(t, &Options{
			ChecklistPath:  checklistPath,
			ClaudeJsonPath: claudeJsonPath,
			ContextDir:     contextDir,
			ProposalsDir:   proposalsDir,
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
		assert.Contains(t, body, "1/1 enabled") // hooks tile
		assert.Contains(t, body, "disabled: 0 hooks · 0 permissions · 1 MCP")
		assert.NotContains(t, body, "Disabled entries")
		assert.Contains(t, body, "1/2 enabled")            // MCP tile: idle-server disabled in live list
		assert.Contains(t, body, "goland")                 // MCP detail
		assert.Contains(t, body, "Frustration / Positive") // merged entries card
		assert.Contains(t, body, `class="split-frustration"`)
		assert.Contains(t, body, `class="split-positive"`)
		assert.NotContains(t, body, "Positive entries")
		assert.Contains(t, body, "Proposals")               // proposals tile
		assert.Contains(t, body, ">1 open<")                // 3 status-bearing minus accepted minus rejected
		assert.Contains(t, body, "1 accepted · 1 rejected") // state counts, status-bearing only
		assert.Contains(t, body, "opus")                    // model tile
		assert.NotContains(t, body, "Skills out of sync")
		assert.Contains(t, body, ">0/1<")                                       // merged skills tile: 0 active / 1 repo
		assert.Contains(t, body, "active / repo · 1 out of sync · 1 workflows") // workflows folded into the skills tile
		assert.NotContains(t, body, `data-tile-id="workflows"`)
		assert.Contains(t, body, "2 entries") // context tile: entry count + gate coverage
		assert.Contains(t, body, "coverage")
		assert.Contains(t, body, "1 scopes ·") // context tile: distinct scope count
		assert.Contains(t, body, "Tools")      // tools tile
		assert.Contains(t, body, "prune-jetbrains")
		assert.NotContains(t, body, "secretscan")
		assert.Contains(t, body, `data-tile-id="skills"`)
		assert.Contains(t, body, "Repos") // repo tile (empty registry)
		assert.Contains(t, body, "0 worktrees")
		assert.Contains(t, body, "Routines") // routine tile (no routines dir)
		assert.Contains(t, body, "0/0 active")
		assert.Contains(t, body, "0/1 done")                    // checklist fixture: entry 8 Open
		assert.Contains(t, body, `href="/sessions/personal/2"`) // last analysis link: batch 2 is newest
		assert.Contains(t, body, `href="/sessions/personal/2?dimension=frustration&amp;open=ghi789#session-ghi789"`)
		assert.Contains(t, body, `href="/sessions/personal/1?dimension=positive&amp;open=def456#session-def456"`)
		// merged card: newest signal batch date joined with the trend delta;
		// template escapes the plus
		assert.Contains(t, body, "last signal 2026-07-17 · &#43;1 vs batch 1")

		// default row order: sessions row, config row, repo row, orga row
		assert.Less(t, strings.Index(body, `data-tile-id="sessions"`), strings.Index(body, `data-tile-id="hooks"`))
		assert.Less(t, strings.Index(body, `data-tile-id="hooks"`), strings.Index(body, `data-tile-id="skills"`))
		assert.Less(t, strings.Index(body, `data-tile-id="skills"`), strings.Index(body, `data-tile-id="proposals"`))
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
		checklistPath := filepath.Join(dir, "checklist.md")
		require.NoError(t, os.WriteFile(checklistPath, []byte(checklistFixture), 0644))
		sessionsDir := filepath.Join(dir, "sessions")
		jsonDir := filepath.Join(sessionsDir, "personal", "json")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch2.json"), []byte(`{
			"batch": {"scope": "personal", "number": 2, "file": "batch2.md", "analyzedDate": "2026-07-17"},
			"sessions": [{
				"id": "ghi789",
				"title": "Third session",
				"frustration": [
					{"quote": "no", "trigger": "t"},
					{"quote": "still no", "trigger": "t"}
				]
			}]
		}`), 0644))
		server := newTestServer(t, &Options{
			ChecklistPath: checklistPath,
			SessionsDir:   sessionsDir,
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

	t.Run("missing-checklist-renders-empty-tile", func(t *testing.T) {
		// A fresh clone has no checklist doc (private artifact) — the tile
		// shows zero progress instead of degrading to an error tile.
		server := newTestServer(t, &Options{
			ChecklistPath: filepath.Join(t.TempDir(), "nope.md"),
			SessionsDir:   t.TempDir(),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "0/0 done")
		assert.NotContains(t, response.Body.String(), "no such file")
	})

	t.Run("empty-everything-still-200", func(t *testing.T) {
		dir := t.TempDir()
		checklistPath := filepath.Join(dir, "checklist.md")
		require.NoError(t, os.WriteFile(checklistPath, []byte(checklistFixture), 0644))
		server := newTestServer(t, &Options{
			ChecklistPath: checklistPath,
			SessionsDir:   filepath.Join(dir, "no-sessions"),
			SkillsHome:    filepath.Join(dir, "no-home"),
			SkillsRepo:    filepath.Join(dir, "no-repo"),
		})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "0/0 enabled")
		assert.Contains(t, response.Body.String(), "disabled: 0 hooks · 0 permissions · 0 MCP")
		// no batches → no chart, no filter row and no analyzedDate detail on
		// the merged card
		assert.NotContains(t, response.Body.String(), "tile-trend")
		assert.NotContains(t, response.Body.String(), "tile-filter")
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
			assert.Equal(t, test._expectedFirst, windowed[0].BatchNumber)
		})
	}
}

// provideSentimentPoints builds count points numbered 1..count.
func provideSentimentPoints(count int) []sessions.SentimentPoint {
	points := make([]sessions.SentimentPoint, 0, count)
	for number := 1; number <= count; number++ {
		points = append(points, sessions.SentimentPoint{BatchNumber: number})
	}
	return points
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

func TestOverviewNonDeveloper(t *testing.T) {
	server := newTestServer(t, &Options{PresentationPath: writeGermanProfile(t)})

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "Ausgewertete Sitzungen")
	assert.Contains(t, body, "Vorschläge")
	assert.NotContains(t, body, ">Checklist</span>")
	assert.NotContains(t, body, ">Repos</span>")
	assert.NotContains(t, body, ">Skills</span>")
	assert.NotContains(t, body, ">Hooks</span>")
}
