package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/backlog"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

// backlogFixture is a trimmed copy of the real store: 4 items, 3 critical,
// B-005 in the review stage, B-004 routed direct, one done row.
const backlogFixture = `{
  "archive": {
    "done": [
      {
        "id": "B-023",
        "date": "2026-09-18",
        "description": "Workflow invocation\nDone when: already done.",
        "link": "",
        "reason": "already done",
        "title": "Workflow invocation"
      }
    ],
    "rejected": []
  },
  "changelog": [
    {
      "date": "2026-09-18",
      "delta": "Initial"
    }
  ],
  "sections": [
    {
      "items": [
        {
          "id": "B-002",
          "added": "2026-09-18",
          "deps": [],
          "description": "Letting the agent write tests willy nilly is a massive problem\nDone when: test-writing rules exist.",
          "evidence": [
            "session 98641eb1",
            "abc123"
          ],
          "link": "",
          "pri": "critical",
          "route": "/fdesign",
          "size": "M",
          "status": "shaped",
          "theme": "quality",
          "title": "Agent test-writing discipline"
        },
        {
          "id": "B-004",
          "added": "2026-09-18",
          "deps": [],
          "description": "/package-commit MUST only commit\nDone when: validation is a separate step.",
          "evidence": [],
          "link": "",
          "pri": "critical",
          "route": "direct",
          "size": "S",
          "status": "shaped",
          "theme": "skills",
          "title": "/package-commit MUST only commit"
        }
      ],
      "name": "Defense"
    },
    {
      "items": [
        {
          "id": "B-005",
          "added": "2026-09-18",
          "deps": [
            "B-002"
          ],
          "description": "Railroad-review lanes refactor\nDone when: lanes are refactored.",
          "evidence": [],
          "link": "plans/x",
          "pri": "critical",
          "route": "/fdesign change",
          "size": "L",
          "status": "review",
          "theme": "rr",
          "title": "Railroad-review lanes refactor"
        },
        {
          "id": "B-013",
          "added": "2026-09-18",
          "deps": [],
          "description": "Smine improvements, finally mine AQMS sessions again\nDone when: smine proposals carry value again.",
          "evidence": [],
          "link": "",
          "mined": {
            "date": "2026-10-03",
            "sessions": ["98641eb1-0000-0000-0000-000000000000"]
          },
          "pri": "high",
          "route": "/smine",
          "size": "L",
          "status": "shaped",
          "theme": "smine",
          "title": "Smine raised to the next level"
        }
      ],
      "name": "Railroad"
    }
  ],
  "updated": "2026-09-18"
}
`

// backlogEpicsFixture holds a dep on an archived row, an empty epic and an Icebox spread over three days.
const backlogEpicsFixture = `{
  "archive": {
    "done": [
      {
        "id": "B-023",
        "date": "2026-09-18",
        "description": "Workflow invocation\nDone when: already done.",
        "link": "",
        "reason": "already done",
        "title": "Workflow invocation"
      }
    ],
    "rejected": []
  },
  "changelog": [],
  "sections": [
    {
      "items": [
        {
          "id": "B-002",
          "added": "2026-09-18",
          "deps": ["B-023"],
          "description": "Letting the agent write tests willy nilly\nDone when: test-writing rules exist.",
          "evidence": [],
          "link": "",
          "pri": "critical",
          "route": "/idea critical",
          "size": "M",
          "status": "shaped",
          "theme": "quality",
          "title": "Agent test-writing discipline"
        }
      ],
      "name": "Defense"
    },
    {
      "items": [],
      "name": "Empty"
    },
    {
      "items": [
        {
          "id": "B-030",
          "added": "2026-10-04",
          "deps": [],
          "description": "Mined from a recorded session",
          "evidence": [],
          "link": "",
          "mined": {
            "date": "2026-10-03",
            "sessions": ["98641eb1-0000-0000-0000-000000000000"]
          },
          "pri": "medium",
          "route": "direct",
          "size": "S",
          "status": "shaped",
          "theme": "smine",
          "title": "Mined from a recorded session"
        },
        {
          "id": "B-031",
          "added": "2026-10-01",
          "deps": [],
          "description": "Filed by hand",
          "evidence": [],
          "link": "",
          "pri": "low",
          "route": "direct",
          "size": "S",
          "status": "shaped",
          "theme": "ui",
          "title": "Filed by hand"
        },
        {
          "id": "B-032",
          "added": "2026-10-05",
          "deps": [],
          "description": "Mined from an unrecorded session",
          "evidence": [],
          "link": "",
          "mined": {
            "by": "facefeed-0000-0000-0000-000000000000",
            "date": "2026-10-05",
            "sessions": ["cafebabe-0000-0000-0000-000000000000"]
          },
          "pri": "low",
          "route": "direct",
          "size": "S",
          "status": "shaped",
          "theme": "smine",
          "title": "Mined from an unrecorded session"
        }
      ],
      "name": "Icebox"
    }
  ],
  "updated": "2026-10-05"
}
`

// minedSessionId is the full id behind the fixtures' "session 98641eb1" evidence and B-013's mined session.
const minedSessionId = "98641eb1-0000-0000-0000-000000000000"

// writeMinedSessionRecord files the record minedSessionId resolves to, in scope work of the fixture week.
func writeMinedSessionRecord(t *testing.T, sessionsDir string) {
	t.Helper()
	weekDir := filepath.Join(sessionsDir, "work", fixtureWeek)
	require.NoError(t, os.MkdirAll(weekDir, 0o755))
	record := `{"id": "` + minedSessionId + `", "agent": "claude", "title": "Mined session", "repo": "work", "week": "` + fixtureWeek + `", "lastActive": "2026-07-16T10:00:00Z", "findings": []}`
	require.NoError(t, os.WriteFile(filepath.Join(weekDir, minedSessionId+".json"), []byte(record), 0o644))
}

// backlogLayout reads the store back as one "section=id,id" line per section, in store order.
func backlogLayout(t *testing.T, dir string) []string {
	t.Helper()
	store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
	require.NoError(t, err)
	layout := make([]string, 0, len(store.Sections))
	for _, section := range store.Sections {
		ids := make([]string, 0, len(section.Items))
		for _, item := range section.Items {
			ids = append(ids, item.Id)
		}
		layout = append(layout, section.Name+"="+strings.Join(ids, ","))
	}
	return layout
}

func writeBacklogStore(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "backlog.json"), []byte(content), 0o644))
	return dir
}

// writeBacklogRoutines mints one manifest-only backlog-<skill> routine per prompt, plus a non-backlog routine whose skill must not be offered.
func writeBacklogRoutines(t *testing.T, prompts ...string) string {
	t.Helper()
	dir := t.TempDir()
	smineDir := filepath.Join(dir, "smine-nightly")
	require.NoError(t, os.MkdirAll(smineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(smineDir, "routine.json"), []byte(`{"kind":"smine-nightly","prompt":"/smine --nightly"}`), 0o644))
	for _, prompt := range prompts {
		skill, _, _ := strings.Cut(strings.TrimPrefix(prompt, "/"), " ")
		routineDir := filepath.Join(dir, "backlog-"+skill)
		require.NoError(t, os.MkdirAll(routineDir, 0o755))
		manifest := `{"kind":"single","prompt":"` + prompt + `"}`
		require.NoError(t, os.WriteFile(filepath.Join(routineDir, "routine.json"), []byte(manifest), 0o644))
	}
	return dir
}

// writeSmineRoutine mints a loadable routine dir whose manifest declares /smine and joins the backlog family by its explicit group.
func writeSmineRoutine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	routineDir := filepath.Join(dir, "smine-nightly")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "routine.json"), []byte(`{"group":"backlog","kind":"smine-nightly","prompt":"/smine --nightly"}`), 0o644))
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>com.smine.routine.test-fixture.smine-nightly</string>
<key>ProgramArguments</key><array><string>/repo/bin/routine</string><string>run</string><string>` + routineDir + `</string></array></dict></plist>`
	// A fixture-only label: the host's real com.smine.routine.smine-nightly may be loaded and must not leak in.
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.smine.routine.test-fixture.smine-nightly.plist"), []byte(plist), 0o644))
	return dir
}

func writeSkillRoot(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		writeRepoSkill(t, root, name, "1.0")
	}
	return root
}

// backlogBlobJson cuts the board's #backlog-data script body out of the page.
func backlogBlobJson(t *testing.T, body string) string {
	t.Helper()
	const open = `<script type="application/json" id="backlog-data">`
	start := strings.Index(body, open)
	require.GreaterOrEqual(t, start, 0, "no backlog-data script")
	rest := body[start+len(open):]
	end := strings.Index(rest, "</script>")
	require.GreaterOrEqual(t, end, 0)
	return rest[:end]
}

// decodeItemEvent reads the backlog:item event an in-place write sends after its swap; the header must stay ASCII.
func decodeItemEvent(t *testing.T, response *httptest.ResponseRecorder) backlogPanelItem {
	t.Helper()
	header := response.Header().Get("HX-Trigger-After-Swap")
	for _, char := range header {
		require.Less(t, char, rune(128), "non-ASCII byte in the event header")
	}
	var events map[string]backlogPanelItem
	require.NoError(t, json.Unmarshal([]byte(header), &events))
	item, found := events["backlog:item"]
	require.True(t, found, "no backlog:item event in %q", header)
	return item
}

// decodeBacklogBlob reads the board's #backlog-data script back into its Go type.
func decodeBacklogBlob(t *testing.T, body string) backlogPanelBlob {
	t.Helper()
	var blob backlogPanelBlob
	require.NoError(t, json.Unmarshal([]byte(backlogBlobJson(t, body)), &blob))
	return blob
}

func TestHandleBacklogPage(t *testing.T) {
	t.Run("renders-sections-and-rows", func(t *testing.T) {
		sessionsDir := t.TempDir()
		writeWeekFixture(t, sessionsDir, "work")
		writeMinedSessionRecord(t, sessionsDir)
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, backlogFixture),
			RoutinesDir: writeSmineRoutine(t),
			SessionsDir: sessionsDir,
			SkillsHome:  t.TempDir(),
			SkillsRepo:  writeSkillRoot(t, "fdesign"),
		})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		// a Back navigation refetches the board instead of showing a cached copy with stale state
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		assert.Contains(t, body, "<h2>Defense</h2>")
		assert.Contains(t, body, "<h2>Railroad</h2>")
		assert.Contains(t, body, `id="item-B-005"`)
		assert.Contains(t, body, `<option selected>review</option>`)
		assert.Contains(t, body, "ROUTINE MISSING")
		assert.Contains(t, body, `href="/scripts/skills/repo/fdesign"`)
		assert.Contains(t, body, "routine missing 2")
		assert.Contains(t, body, `data-group="live" data-value="1">live 0</a>`)
		assert.Contains(t, body, "Active (4)")
		assert.NotContains(t, body, "backlog-head")
		assert.Equal(t, 2, strings.Count(body, `<colgroup><col class="col-id"><col class="col-title"><col class="col-route"><col class="col-status"><col class="col-activity"></colgroup>`))
		assert.Contains(t, body, `<button type="button" class="bl-chip" data-item-chip="B-005"`)
		assert.Contains(t, body, `<script src="/assets/backlog-panel.js?v=`)
		// the badge line leads, the title follows
		tagsAt := strings.Index(body, `<div class="bl-tags">`)
		titleAt := strings.Index(body, `<div class="bl-title">Agent test-writing discipline</div>`)
		assert.Positive(t, tagsAt)
		assert.Less(t, tagsAt, titleAt)
		assert.Contains(t, body, `<a class="dep-link" href="#item-B-002" title="depends on B-002">needs B-002</a>`)
		// autocomplete off: a history navigation never restores a stale select value over the store's
		assert.Contains(t, body, `<select name="pri" class="badge badge-select badge-error" autocomplete="off" hx-post="/api/backlog/B-002/pri"`)
		assert.Contains(t, body, `<select name="status" autocomplete="off" hx-post="/api/backlog/B-002/status"`)
		assert.Contains(t, body, `<option value="critical" selected>crit</option>`)
		assert.Contains(t, body, `<option value="medium" >med</option>`)
		// descriptions live in the panel blob and on the item page — the row stays title-only
		assert.NotContains(t, body, `>Letting the agent write tests willy nilly`)
		// activity names no routine and no never-run line
		assert.NotContains(t, body, "runs via")
		assert.NotContains(t, body, "never run for this item")
		// the mined marker badges only the mined item; the filter counts it
		assert.Contains(t, body, `<tr id="item-B-013" class="row-mined"`)
		assert.Contains(t, body, `title="smined 2026-10-03 from 1 session(s) by the smine nightly">smined</span>`)
		assert.Equal(t, 1, strings.Count(body, ">smined</span>"))
		assert.Contains(t, body, `data-group="mined" data-value="1">smined 1</a>`)

		blob := decodeBacklogBlob(t, body)
		assert.NotContains(t, backlogBlobJson(t, body), `"prompt"`)
		assert.Len(t, blob.Items, 4)
		assert.Equal(t, []string{"Defense", "Railroad"}, blob.Epics)
		assert.Equal(t, backlog.Priorities, blob.Priorities)
		assert.Equal(t, backlog.Statuses, blob.Statuses)
		agentTests := blob.Items["B-002"]
		assert.Equal(t, "Letting the agent write tests willy nilly is a massive problem", agentTests.Description)
		assert.Equal(t, "test-writing rules exist.", agentTests.DoneWhen)
		assert.Equal(t, "Defense", agentTests.Epic)
		assert.Equal(t, badgeError, agentTests.PriClass)
		minedWeekURL := "/sessions/work/" + fixtureWeek + "?session=" + minedSessionId
		expectedEvidence := []evidenceLink{
			{Text: "session 98641eb1", WeekURL: minedWeekURL},
			{Text: "abc123"},
		}
		assert.Equal(t, expectedEvidence, agentTests.Evidence)
		assert.Empty(t, agentTests.Routine)
		assert.Empty(t, agentTests.Routines)
		assert.Empty(t, agentTests.MinedByURL)
		lanes := blob.Items["B-005"]
		assert.Equal(t, []depView{{Id: "B-002", IsActive: true}}, lanes.Deps)
		assert.Equal(t, "Railroad", lanes.Epic)
		mined := blob.Items["B-013"]
		require.NotNil(t, mined.Mined)
		assert.Equal(t, "2026-10-03", mined.Mined.Date)
		// mined before the miner was recorded: no session to link
		assert.Empty(t, mined.MinedByURL)
		// the fixture routine joins the backlog family by its manifest group; no launchd job runs its label
		assert.Equal(t, "smine-nightly", mined.Routine)
		assert.Equal(t, []backlogPanelRoutine{{Name: "smine-nightly"}}, mined.Routines)
	})

	t.Run("epic-form-and-delete-button", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogEpicsFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, `<form class="card-row epic-form" hx-post="/api/backlog/epics" hx-swap="none">`)
		assert.Contains(t, body, `<input type="text" name="name" placeholder="new epic" required>`)
		// only the empty epic offers a delete; the Icebox never does
		assert.Equal(t, 1, strings.Count(body, ">delete epic</button>"))
		assert.Contains(t, body, `hx-delete="/api/backlog/epics/Empty"`)
		assert.Contains(t, body, "No items in this epic.")

		blob := decodeBacklogBlob(t, body)
		assert.Equal(t, []string{"Defense", "Empty", "Icebox"}, blob.Epics)
	})

	t.Run("icebox-day-groups", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogEpicsFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Equal(t, 3, strings.Count(body, `<details class="section session-day"`))
		assert.Equal(t, 1, strings.Count(body, `<details class="section session-day" open>`))
		newestAt := strings.Index(body, `<details class="section session-day" open>
    <summary><code>Mon 2026-10-05</code> <span class="count-note">1</span></summary>`)
		minedAt := strings.Index(body, "<code>Sat 2026-10-03</code>")
		addedAt := strings.Index(body, "<code>Thu 2026-10-01</code>")
		assert.Positive(t, newestAt)
		assert.Less(t, newestAt, minedAt)
		assert.Less(t, minedAt, addedAt)
		// Defense, then one table per Icebox day
		assert.Equal(t, 4, strings.Count(body, "<colgroup>"))
	})

	t.Run("mined-by-link", func(t *testing.T) {
		server := newTestServer(t, &Options{
			BacklogDir:       writeBacklogStore(t, backlogEpicsFixture),
			PeekDashboardURL: "http://127.0.0.1:4243/",
		})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, `<div class="bl-origin meta">smined 2026-10-05 by <a href="http://127.0.0.1:4243/sessions/facefeed-0000-0000-0000-000000000000" title="facefeed-0000-0000-0000-000000000000 — the run that mined this row, in peek">facefeed</a></div>`)
		// a row mined before the miner was recorded has no origin line, nor does the hand-filed row
		assert.Equal(t, 1, strings.Count(body, `<div class="bl-origin meta">`))

		blob := decodeBacklogBlob(t, body)
		var minedItem backlogPanelItem
		for _, item := range blob.Items {
			if item.Mined != nil && item.Mined.By != "" {
				minedItem = item
			}
		}
		assert.Equal(t, "http://127.0.0.1:4243/sessions/facefeed-0000-0000-0000-000000000000", minedItem.MinedByURL)
	})

	t.Run("route-select-over-backlog-routines", func(t *testing.T) {
		server := newTestServer(t, &Options{
			BacklogDir:  writeBacklogStore(t, backlogFixture),
			RoutinesDir: writeBacklogRoutines(t, "/idea --nightly", "/fdesign --nightly"),
		})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		// B-005 routes /fdesign change: listed, its args beside the select
		assert.Contains(t, body, `<select name="skill" class="bl-route" autocomplete="off" hx-post="/api/backlog/B-005/route"`)
		assert.Contains(t, body, `<option value="fdesign" selected>/fdesign</option>`)
		assert.Contains(t, body, ` <span class="meta"> change</span>`)
		// B-004 routes direct: shown as a disabled current option, never posted
		assert.Contains(t, body, `<option value="" selected disabled>direct</option>`)
		// only backlog-* routines offer their skill, sorted
		fdesignAt := strings.Index(body, `<option value="fdesign"`)
		ideaAt := strings.Index(body, `<option value="idea"`)
		assert.Positive(t, fdesignAt)
		assert.Less(t, fdesignAt, ideaAt)
		assert.NotContains(t, body, `<option value="smine"`)
	})

	t.Run("archived-dep-as-text", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogEpicsFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, `<span class="meta" title="depends on B-023, which is archived">needs B-023 · archived</span>`)
		assert.NotContains(t, body, `href="#item-B-023"`)

		blob := decodeBacklogBlob(t, body)
		assert.Equal(t, []depView{{Id: "B-023", IsActive: false}}, blob.Items["B-002"].Deps)
	})

	t.Run("missing-store-empty-board", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: filepath.Join(t.TempDir(), "nope")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "No backlog store")
		assert.NotContains(t, response.Body.String(), "no such file")
		assert.NotContains(t, response.Body.String(), `id="backlog-data"`)
	})

	t.Run("empty-sections-zero-counters", func(t *testing.T) {
		empty := `{"archive": {"done": [], "rejected": []}, "changelog": [], "sections": [], "updated": "2026-09-18"}`
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, empty)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Active (0)")
		assert.Contains(t, body, "mined 0")
	})

	t.Run("broken-store-error-card", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, `{"sections": [`)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "store format")
	})

	t.Run("archive-tab", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog?tab=archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Workflow invocation")
		assert.Contains(t, body, "1 done · 0 rejected")
		assert.NotContains(t, body, `id="item-B-005"`)
	})
}

func TestHandleBacklogActivity(t *testing.T) {
	hanging := newHangingPeek(t)
	server := newTestServer(t, &Options{
		BacklogDir:   writeBacklogStore(t, backlogFixture),
		PeekEndpoint: hanging.url,
		RoutinesDir:  writeSmineRoutine(t),
	})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-013/activity", nil))

	// a hanging peek would hold the cell for the peek timeout
	assert.Zero(t, hanging.calls.Load(), "the cell never asks peek")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	body := response.Body.String()
	assert.Contains(t, body, `<td id="activity-B-013"`)
	// the poll answer is the cell alone: no select of the row is ever swapped by it
	assert.NotContains(t, body, "<select")
	assert.NotContains(t, body, "<tr")
}

func TestHandleBacklogActivity_dispatch(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedContains string
		_expectedPolls    bool

		server *Server
	}

	sinceUnix := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC).Unix()
	tests := make([]*testCase, 0)

	// no-run-yet-shows-starting
	tests = append(tests, &testCase{
		_id:               "no-run-yet-shows-starting",
		_expectedCode:     http.StatusOK,
		_expectedContains: `<span class="badge badge-action">starting</span>`,
		_expectedPolls:    true,

		server: newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)}),
	})

	// older-run-still-starting
	olderRoutines := writeSmineRoutine(t)
	olderRow := `{"exit_status":0,"is_error":false,"item":"B-013","num_turns":3,"result":"done","session_id":"s-old","subtype":"success","timestamp":"2026-10-10T08:00:00Z","total_cost_usd":0.1}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(olderRoutines, "smine-nightly", "results.jsonl"), []byte(olderRow), 0o644))
	tests = append(tests, &testCase{
		_id:               "older-run-still-starting",
		_expectedCode:     http.StatusOK,
		_expectedContains: `<span class="badge badge-action">starting</span>`,
		_expectedPolls:    true,

		server: newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: olderRoutines}),
	})

	// finished-run-stops-polling
	finishedRoutines := writeSmineRoutine(t)
	finishedRow := `{"branch":"claude-routines/smine-nightly-2026-10-10","exit_status":0,"is_error":false,"item":"B-013","num_turns":3,"result":"done","session_id":"s-new","subtype":"success","timestamp":"2026-10-10T09:05:00Z","total_cost_usd":0.1}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(finishedRoutines, "smine-nightly", "results.jsonl"), []byte(finishedRow), 0o644))
	tests = append(tests, &testCase{
		_id:               "finished-run-stops-polling",
		_expectedCode:     http.StatusOK,
		_expectedContains: `<span class="badge badge-dim">ok</span>`,
		_expectedPolls:    false,

		server: newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: finishedRoutines}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-013/activity?since="+strconv.FormatInt(sinceUnix, 10), nil))
			assert.Equal(t, test._expectedCode, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedContains)
			assert.Equal(t, test._expectedPolls, strings.Contains(response.Body.String(), `hx-get="/backlog/B-013/activity?since=`))
		})
	}
}

func TestBacklogActivityOOB_dispatchAnswer(t *testing.T) {
	server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
	store, err := server.loadBacklog()
	require.NoError(t, err)
	item, _ := store.Find("B-013")
	view := server.itemView(item, maxBoardRuns, store)
	view.IsActivityOOB = true
	view.PollSince = 1791590000

	response := httptest.NewRecorder()
	server.renderFragment(response, tmplBacklogCellsOOB, view)
	body := response.Body.String()
	assert.Contains(t, body, "<template>")
	assert.NotContains(t, body, `id="status-B-013"`)
	assert.Contains(t, body, `<td id="activity-B-013" hx-swap-oob="true"`)
	assert.Contains(t, body, `hx-get="/backlog/B-013/activity?since=1791590000" hx-trigger="every 3s" hx-swap="outerHTML"`)
	assert.Contains(t, body, `<span class="badge badge-action">starting</span>`)
	// the dispatch answer swaps the cell only, never the row and its selects
	assert.NotContains(t, body, "<tr")
}

func Test_asciiJson(t *testing.T) {
	backslash := string(rune(92))
	dash := `{"title":"RR init — review-config"}`
	emoji := `{"emoji":"🚀"}`
	plain := `{"plain":"ok"}`

	// the escaped form decodes to the same value and carries no byte above ASCII
	assert.JSONEq(t, dash, asciiJson([]byte(dash)))
	assert.Contains(t, asciiJson([]byte(dash)), backslash+"u2014")
	assert.NotContains(t, asciiJson([]byte(dash)), "—")
	assert.JSONEq(t, emoji, asciiJson([]byte(emoji)))
	assert.Contains(t, asciiJson([]byte(emoji)), backslash+"ud83d"+backslash+"ude80")
	assert.JSONEq(t, plain, asciiJson([]byte(plain)))
	assert.Len(t, asciiJson([]byte(plain)), len(plain))
}

func Test_hasRunSince(t *testing.T) {
	sinceUnix := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC).Unix()
	started := &routines.RunResult{Subtype: routines.SubtypeStarted, Timestamp: "2026-10-10T09:00:01Z"}
	older := &routines.RunResult{Subtype: "success", Timestamp: "2026-10-10T08:59:59Z"}
	unparseable := &routines.RunResult{Subtype: "success", Timestamp: "yesterday"}

	assert.False(t, hasRunSince(nil, sinceUnix))
	assert.True(t, hasRunSince(started, sinceUnix))
	assert.False(t, hasRunSince(older, sinceUnix))
	assert.False(t, hasRunSince(unparseable, sinceUnix))
}

func TestBacklogItem(t *testing.T) {
	t.Run("header-selects-and-acceptance", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-005", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		headAt := strings.Index(body, `<div class="bl-head">`)
		statusAt := strings.Index(body, `<select name="status" autocomplete="off" hx-post="/api/backlog/B-005/status"`)
		priAt := strings.Index(body, `<select name="pri" class="badge badge-select badge-error" autocomplete="off" hx-post="/api/backlog/B-005/pri"`)
		fieldsAt := strings.Index(body, `<div class="item-fields">`)
		assert.Positive(t, headAt)
		assert.Less(t, headAt, statusAt)
		assert.Less(t, statusAt, priAt)
		assert.Less(t, priAt, fieldsAt)
		assert.Contains(t, body, `<select name="epic" autocomplete="off" hx-post="/api/backlog/B-005/epic"`)
		assert.Contains(t, body, `<option selected>Railroad</option>`)
		assert.Contains(t, body, `<option >Defense</option>`)
		assert.Contains(t, body, `<div class="item-desc">Railroad-review lanes refactor</div>`)
		assert.Contains(t, body, `<span class="section-label">acceptance</span>
  <div class="item-desc">lanes are refactored.</div>`)
		assert.NotContains(t, body, `<span class="section-label">description</span>`)
		assert.NotContains(t, body, "done when")
	})

	t.Run("no-prompt-block", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-013", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, `id="dispatch-form"`)
		assert.Contains(t, body, `id="routine-smine-nightly"`)
		assert.NotContains(t, body, "dispatch-prompt")
		assert.NotContains(t, body, "B-013: Smine raised to the next level\n\n")
		// the dispatch names its routine, chosen from the backlog routines running the route skill
		assert.Contains(t, body, `<option value="smine-nightly" selected>smine-nightly</option>`)
	})

	t.Run("routine-missing-card", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-005", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Contains(t, body, "B-005 Railroad-review lanes refactor")
		assert.Contains(t, body, "ROUTINE MISSING.")
		assert.NotContains(t, body, `id="dispatch-form"`)
		assert.Contains(t, body, `<a class="dep-link" href="/backlog/B-002">needs B-002</a>`)
	})

	t.Run("evidence-list", func(t *testing.T) {
		sessionsDir := t.TempDir()
		writeWeekFixture(t, sessionsDir, "work")
		writeMinedSessionRecord(t, sessionsDir)
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), SessionsDir: sessionsDir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-002", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<li><a href="/sessions/work/`+fixtureWeek+`?session=`+minedSessionId+`" title="open the session findings">session 98641eb1</a></li>`)
		// under eight characters is no session pointer, even when a record carries the id
		assert.Contains(t, body, `<li><span>abc123</span></li>`)
	})

	t.Run("sessions-table-from-peek", func(t *testing.T) {
		skillsHome := t.TempDir()
		writeTitleDeclaration(t, skillsHome, "fdesign")
		sessionsByAgent := map[string][]map[string]any{
			"claude": {{"id": "s-design", "last_active": "2000-01-01T00:00:00Z", "meta": map[string]any{}, "title": "CC,FD - B-005 lanes - [DESIGN-READY]"}},
		}
		stub := newAgentStubPeek(t, nil, sessionsByAgent, nil)
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), PeekEndpoint: stub.URL, SkillsHome: skillsHome})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-005", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<td>CC,FD - B-005 lanes <span class="badge badge-action" title="design gate presented — /fdesign">DESIGN-READY</span></td>`)
		assert.NotContains(t, body, "[DESIGN-READY]")
	})

	t.Run("unknown-id-404", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-999", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestHandleBacklogStatus(t *testing.T) {
	t.Run("rejected-moves-to-rejected-archive", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-013/status", url.Values{"status": {"rejected"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.Equal(t, "true", response.Header().Get("HX-Refresh"))

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		_, found := store.Find("B-013")
		assert.False(t, found)
		require.Len(t, store.Archive.Rejected, 1)
		assert.Equal(t, "B-013", store.Archive.Rejected[0].Id)
		assert.Equal(t, "rejected (board)", store.Archive.Rejected[0].Reason)
		assert.Len(t, store.Archive.Done, 1)
	})

	t.Run("parked-stays-active", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/status", url.Values{"status": {"parked"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		item, found := store.Find("B-002")
		assert.True(t, found)
		assert.Equal(t, "parked", item.Status)
		assert.Empty(t, store.Archive.Rejected)
	})

	t.Run("status-changed-in-place", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/status", url.Values{"status": {"planning"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		// no reload: only the status cell swaps out of band, so the row's other selects and the Activity poll stay; the card reads the event
		body := response.Body.String()
		assert.Empty(t, response.Header().Get("HX-Refresh"))
		assert.Contains(t, body, `<td id="status-B-002" hx-swap-oob="true">`)
		assert.Contains(t, body, `<option selected>planning</option>`)
		assert.NotContains(t, body, "<tr")
		assert.NotContains(t, body, `id="title-B-002"`)
		assert.NotContains(t, body, `id="activity-B-002"`)
		assert.Equal(t, "planning", decodeItemEvent(t, response).Status)

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		item, _ := store.Find("B-002")
		assert.Equal(t, "planning", item.Status)
	})

	t.Run("done-moves-to-archive", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-004/status", url.Values{"status": {"done"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		_, found := store.Find("B-004")
		assert.False(t, found)
		assert.Equal(t, 3, store.Active())
		require.Len(t, store.Archive.Done, 2)
		assert.Equal(t, "B-004", store.Archive.Done[1].Id)
		assert.Equal(t, "/package-commit MUST only commit\nDone when: validation is a separate step.", store.Archive.Done[1].Description)
		assert.Equal(t, "done (board)", store.Archive.Done[1].Reason)
	})

	t.Run("invalid-400", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/status", url.Values{"status": {"in-pipeline"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("unknown-409", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-999/status", url.Values{"status": {"shaped"}}))
		assert.Equal(t, http.StatusConflict, response.Code)
	})
}

func TestHandleBacklogPri(t *testing.T) {
	t.Run("writes-in-place", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/pri", url.Values{"pri": {"high"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Empty(t, response.Header().Get("HX-Refresh"))
		assert.Contains(t, body, `<td id="title-B-002" hx-swap-oob="true">`)
		assert.Contains(t, body, `<option value="high" selected>high</option>`)
		assert.NotContains(t, body, "<tr")
		assert.NotContains(t, body, `id="status-B-002"`)
		assert.NotContains(t, body, `id="activity-B-002"`)
		event := decodeItemEvent(t, response)
		assert.Equal(t, "high", event.Pri)
		assert.Equal(t, badgeAction, event.PriClass)

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		item, _ := store.Find("B-002")
		assert.Equal(t, "high", item.Pri)
		assert.Equal(t, "B-002 pri critical → high (board)", store.Changelog[len(store.Changelog)-1].Delta)
	})

	t.Run("invalid-pri-400", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/pri", url.Values{"pri": {"urgent"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Empty(t, response.Header().Get("HX-Refresh"))
	})

	t.Run("unknown-item-409", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-999/pri", url.Values{"pri": {"low"}}))
		assert.Equal(t, http.StatusConflict, response.Code)
	})
}

func TestHandleBacklogRoute(t *testing.T) {
	t.Run("writes-in-place", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir, RoutinesDir: writeBacklogRoutines(t, "/idea --nightly", "/fdesign --nightly")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-005/route", url.Values{"skill": {"idea"}}))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.Empty(t, response.Header().Get("HX-Refresh"))
		assert.Contains(t, body, `<td id="route-B-005" hx-swap-oob="true">`)
		assert.Contains(t, body, `<option value="idea" selected>/idea</option>`)
		assert.Contains(t, body, `<td id="activity-B-005" hx-swap-oob="true"`)
		assert.NotContains(t, body, "<tr")
		assert.NotContains(t, body, `id="status-B-005"`)
		assert.Equal(t, "idea", decodeItemEvent(t, response).RouteSkill)

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		item, _ := store.Find("B-005")
		assert.Equal(t, "/idea", item.Route)
		assert.Equal(t, "B-005 route /fdesign change → /idea (board)", store.Changelog[len(store.Changelog)-1].Delta)
	})

	t.Run("skill-without-backlog-routine-400", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir, RoutinesDir: writeBacklogRoutines(t, "/idea --nightly")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-005/route", url.Values{"skill": {"smine"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Empty(t, response.Header().Get("HX-Refresh"))

		store, err := backlog.Load(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		item, _ := store.Find("B-005")
		assert.Equal(t, "/fdesign change", item.Route)
	})

	t.Run("blank-skill-400", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeBacklogRoutines(t, "/idea --nightly")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-005/route", url.Values{"skill": {""}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("unknown-item-409", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeBacklogRoutines(t, "/idea --nightly")})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-999/route", url.Values{"skill": {"idea"}}))
		assert.Equal(t, http.StatusConflict, response.Code)
	})
}

func TestHandleBacklogEpics(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCode    int
		_expectedLayout  []string
		_expectedRefresh string

		dir     string
		request *http.Request
		server  *Server
	}

	unchanged := []string{"Defense=B-002", "Empty=", "Icebox=B-030,B-031,B-032"}
	tests := make([]*testCase, 0)

	// add-refreshes
	addDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "add-refreshes",
		_expectedCode:    http.StatusOK,
		_expectedLayout:  []string{"Defense=B-002", "Empty=", "Offense=", "Icebox=B-030,B-031,B-032"},
		_expectedRefresh: "true",

		dir:     addDir,
		request: formPost("/api/backlog/epics", url.Values{"name": {"Offense"}}),
		server:  newTestServer(t, &Options{BacklogDir: addDir}),
	})

	// add-blank-400
	addBlankDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "add-blank-400",
		_expectedCode:    http.StatusBadRequest,
		_expectedLayout:  unchanged,
		_expectedRefresh: "",

		dir:     addBlankDir,
		request: formPost("/api/backlog/epics", url.Values{"name": {"  "}}),
		server:  newTestServer(t, &Options{BacklogDir: addBlankDir}),
	})

	// move-refreshes
	moveDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "move-refreshes",
		_expectedCode:    http.StatusOK,
		_expectedLayout:  []string{"Defense=", "Empty=B-002", "Icebox=B-030,B-031,B-032"},
		_expectedRefresh: "true",

		dir:     moveDir,
		request: formPost("/api/backlog/B-002/epic", url.Values{"epic": {"Empty"}}),
		server:  newTestServer(t, &Options{BacklogDir: moveDir}),
	})

	// move-unknown-epic-400
	moveUnknownEpicDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "move-unknown-epic-400",
		_expectedCode:    http.StatusBadRequest,
		_expectedLayout:  unchanged,
		_expectedRefresh: "",

		dir:     moveUnknownEpicDir,
		request: formPost("/api/backlog/B-002/epic", url.Values{"epic": {"Nowhere"}}),
		server:  newTestServer(t, &Options{BacklogDir: moveUnknownEpicDir}),
	})

	// move-unknown-item-409
	moveUnknownItemDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "move-unknown-item-409",
		_expectedCode:    http.StatusConflict,
		_expectedLayout:  unchanged,
		_expectedRefresh: "",

		dir:     moveUnknownItemDir,
		request: formPost("/api/backlog/B-999/epic", url.Values{"epic": {"Empty"}}),
		server:  newTestServer(t, &Options{BacklogDir: moveUnknownItemDir}),
	})

	// remove-empty-refreshes
	removeDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "remove-empty-refreshes",
		_expectedCode:    http.StatusOK,
		_expectedLayout:  []string{"Defense=B-002", "Icebox=B-030,B-031,B-032"},
		_expectedRefresh: "true",

		dir:     removeDir,
		request: httptest.NewRequest(http.MethodDelete, "/api/backlog/epics/Empty", nil),
		server:  newTestServer(t, &Options{BacklogDir: removeDir}),
	})

	// remove-non-empty-400
	removeNonEmptyDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "remove-non-empty-400",
		_expectedCode:    http.StatusBadRequest,
		_expectedLayout:  unchanged,
		_expectedRefresh: "",

		dir:     removeNonEmptyDir,
		request: httptest.NewRequest(http.MethodDelete, "/api/backlog/epics/Defense", nil),
		server:  newTestServer(t, &Options{BacklogDir: removeNonEmptyDir}),
	})

	// remove-icebox-400
	removeIceboxDir := writeBacklogStore(t, backlogEpicsFixture)
	tests = append(tests, &testCase{
		_id:              "remove-icebox-400",
		_expectedCode:    http.StatusBadRequest,
		_expectedLayout:  unchanged,
		_expectedRefresh: "",

		dir:     removeIceboxDir,
		request: httptest.NewRequest(http.MethodDelete, "/api/backlog/epics/Icebox", nil),
		server:  newTestServer(t, &Options{BacklogDir: removeIceboxDir}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, test.request)

			assert.Equal(t, test._expectedCode, response.Code, response.Body.String())
			assert.Equal(t, test._expectedRefresh, response.Header().Get("HX-Refresh"))
			assert.Equal(t, test._expectedLayout, backlogLayout(t, test.dir))
		})
	}
}

func TestBacklogDispatch(t *testing.T) {
	t.Run("routine-missing-409", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-002/dispatch", url.Values{"routine": {"backlog-fdesign"}}))
		assert.Equal(t, http.StatusConflict, response.Code)
		assert.Contains(t, response.Body.String(), "routine missing")
	})

	t.Run("routine-not-running-the-skill-400", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-013/dispatch", url.Values{"routine": {"backlog-fdesign-opus"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "backlog-fdesign-opus is no backlog routine running /smine")
	})

	t.Run("missing-routine-400", func(t *testing.T) {
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-013/dispatch", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "Missing field routine")
	})

	t.Run("never-writes-store", func(t *testing.T) {
		dir := writeBacklogStore(t, backlogFixture)
		server := newTestServer(t, &Options{BacklogDir: dir, RoutinesDir: writeSmineRoutine(t)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-013/dispatch", url.Values{"routine": {"smine-nightly"}, "stage": {"done"}}))
		// no launchd in tests → the routine is stopped → 409 before any run
		assert.Equal(t, http.StatusConflict, response.Code)
		after, err := os.ReadFile(filepath.Join(dir, "backlog.json"))
		require.NoError(t, err)
		assert.JSONEq(t, backlogFixture, string(after))
	})

	t.Run("orphan-started-row-never-blocks", func(t *testing.T) {
		routinesDir := writeSmineRoutine(t)
		started := `{"timestamp":"2026-09-24T10:00:00Z","exit_status":0,"subtype":"started"}` + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(routinesDir, "smine-nightly", "results.jsonl"), []byte(started), 0o644))
		server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: routinesDir})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/backlog/B-013/dispatch", url.Values{"routine": {"smine-nightly"}}))
		// the fixture job is neither running nor loaded: refused as stopped, not as busy
		assert.Equal(t, http.StatusConflict, response.Code)
		assert.NotContains(t, response.Body.String(), "run active")
		assert.Contains(t, response.Body.String(), "stopped")
	})
}

func Test_dispatchPrompt(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		extra string
		route string
	}
	tests := make([]*testCase, 0)

	// route-args-lead
	tests = append(tests, &testCase{_id: "route-args-lead", _expected: "change B-001: Title\n\nDesc", route: "/fdesign change"})

	// bare-route
	tests = append(tests, &testCase{_id: "bare-route", _expected: "B-001: Title\n\nDesc", route: "/concept"})

	// operator-extra
	tests = append(tests, &testCase{_id: "operator-extra", _expected: "critical B-001: Title focus x\n\nDesc", extra: "focus x", route: "/idea critical"})

	// direct-no-args
	tests = append(tests, &testCase{_id: "direct-no-args", _expected: "B-001: Title\n\nDesc", route: "direct"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			item := &backlog.Item{Description: "Desc", Id: "B-001", Route: test.route, Title: "Title"}
			assert.Equal(t, test._expected, dispatchPrompt(item, test.extra))
		})
	}
}

func Test_splitDoneWhen(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedDescription string
		_expectedDoneWhen    string

		description string
	}
	tests := make([]*testCase, 0)

	// done-when-line
	tests = append(tests, &testCase{
		_id:                  "done-when-line",
		_expectedDescription: "Body\nmore",
		_expectedDoneWhen:    "it ships.",

		description: "Body\nmore\nDone when: it ships.",
	})

	// done-when-only
	tests = append(tests, &testCase{
		_id:                  "done-when-only",
		_expectedDescription: "",
		_expectedDoneWhen:    "it ships.",

		description: "Done when: it ships.",
	})

	// no-done-when
	tests = append(tests, &testCase{
		_id:                  "no-done-when",
		_expectedDescription: "Body only",
		_expectedDoneWhen:    "",

		description: "Body only",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			description, doneWhen := splitDoneWhen(test.description)
			assert.Equal(t, test._expectedDescription, description)
			assert.Equal(t, test._expectedDoneWhen, doneWhen)
		})
	}
}

func Test_titleSessions(t *testing.T) {
	now := time.Now()
	sessions := []peek.Session{
		{Id: "a", LastActive: now.Add(-time.Hour), Title: "B-005 lanes"},
		{Id: "b", LastActive: now, Title: "notes (B-005)"},
		{Id: "c", LastActive: now, Title: "B-0051 other"},
		{Id: "d", LastActive: now, Title: "SUB-005 other"},
	}

	matched := titleSessions("B-005", 10, sessions)
	require.Len(t, matched, 2)
	assert.Equal(t, "b", matched[0].Id)
	assert.Equal(t, "a", matched[1].Id)
	assert.Len(t, titleSessions("B-005", 1, sessions), 1)
}

func Test_itemRuns(t *testing.T) {
	history := []routines.RunResult{{Item: "B-013", SessionId: "x"}, {SessionId: "y"}, {Item: "B-013", SessionId: "z"}, {Item: "B-001"}}

	kept := itemRuns(history, "B-013", 10)
	require.Len(t, kept, 2)
	assert.Equal(t, "x", kept[0].SessionId)
	assert.Equal(t, "z", kept[1].SessionId)
	assert.Len(t, itemRuns(history, "B-013", 1), 1)
	assert.Empty(t, itemRuns(history, "B-999", 10))
}

func Test_mergeSessions(t *testing.T) {
	now := time.Now()
	titled := []peek.Session{{Id: "a", LastActive: now.Add(-time.Hour)}}
	run := []peek.Session{{Id: "b", LastActive: now}, {Id: "a", LastActive: now.Add(-time.Hour)}}

	merged := mergeSessions(titled, run, 10)
	require.Len(t, merged, 2)
	assert.Equal(t, "b", merged[0].Id)
	assert.Equal(t, "a", merged[1].Id)
	assert.Len(t, mergeSessions(titled, run, 1), 1)
}

func Test_withRuns(t *testing.T) {
	history := []routines.RunResult{{Item: "B-013", Subtype: routines.SubtypeStarted, Timestamp: "2026-10-10T09:00:00Z"}}
	histories := map[string][]routines.RunResult{"nightly": history}
	item := backlog.Item{Id: "B-013"}

	live := withRuns(backlogItemView{Item: item, Routines: []routines.Routine{{IsLive: true, Name: "nightly"}}}, histories, 10)
	orphan := withRuns(backlogItemView{Item: item, Routines: []routines.Routine{{IsLive: false, Name: "nightly"}}}, histories, 10)

	assert.True(t, live.IsLive)
	assert.False(t, orphan.IsLive)
	require.NotNil(t, orphan.LastRun)

	// several routines run the skill: rows merge newest first, and the newest row's routine is the item's routine
	opusRows := []routines.RunResult{{Item: "B-013", Subtype: "success", Timestamp: "2026-10-09T08:00:00Z"}, {Item: "B-013", Subtype: "success", Timestamp: "2026-10-01T08:00:00Z"}}
	fableRows := []routines.RunResult{{Item: "B-013", Subtype: routines.SubtypeStarted, Timestamp: "2026-10-10T07:00:00Z"}}
	both := map[string][]routines.RunResult{"backlog-fdesign-fable": fableRows, "backlog-fdesign-opus": opusRows}
	candidates := []routines.Routine{{Name: "backlog-fdesign-fable"}, {IsLive: true, Name: "backlog-fdesign-opus"}}
	merged := withRuns(backlogItemView{Item: item, Routine: &candidates[0], Routines: candidates}, both, 2)

	require.Len(t, merged.Runs, 2)
	assert.Equal(t, "2026-10-10T07:00:00Z", merged.Runs[0].Timestamp)
	assert.Equal(t, "2026-10-09T08:00:00Z", merged.Runs[1].Timestamp)
	assert.Equal(t, "backlog-fdesign-fable", merged.Routine.Name)
	// the newest row started under a routine that is not live: interrupted, not running — another routine's live run never counts
	assert.False(t, merged.IsLive)
}

func Test_runSessions(t *testing.T) {
	sessions := []peek.Session{{Id: "x"}, {Id: "y"}}
	runs := []routines.RunResult{{SessionId: "y"}, {SessionId: ""}, {SessionId: "gone"}}

	matched := runSessions(runs, sessions)
	require.Len(t, matched, 1)
	assert.Equal(t, "y", matched[0].Id)
}

func TestBacklogRunRows(t *testing.T) {
	routinesDir := writeSmineRoutine(t)
	rows := `{"timestamp":"2026-09-27T10:00:00Z","exit_status":70,"subtype":"success","item":"B-013","branch":"claude-routines/smine-nightly-2026-09-26","session_id":"s1"}` + "\n" +
		`{"timestamp":"2026-09-27T11:00:00Z","exit_status":0,"subtype":"started","item":"B-013","branch":"claude-routines/smine-nightly-2026-09-27"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(routinesDir, "smine-nightly", "results.jsonl"), []byte(rows), 0o644))
	server := newTestServer(t, &Options{BacklogDir: writeBacklogStore(t, backlogFixture), RoutinesDir: routinesDir})

	// No launchd job runs the fixture label: the started row is an orphan.
	t.Run("orphan-row-reads-interrupted", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-013/activity", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.NotContains(t, body, "hx-get=")
		assert.Contains(t, body, `<span class="badge badge-error" title="the run died without a final row">interrupted</span>`)
		assert.Contains(t, body, `<span class="meta">2026-09-27 11:00:00Z</span>`)
		assert.Contains(t, body, "<code>claude-routines/smine-nightly-2026-09-27</code>")
	})

	t.Run("runs-table-filters-by-item", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-013/runs", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		body := response.Body.String()
		assert.NotContains(t, body, "hx-get=")
		assert.Contains(t, body, "INTERRUPTED")
		assert.Contains(t, body, "claude-routines/smine-nightly-2026-09-27")
	})

	t.Run("other-item-never-run", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/backlog/B-002/activity", nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		assert.NotContains(t, response.Body.String(), "hx-get=")
	})
}

func Test_routineFamily(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		routine *routines.Routine
	}

	tests := make([]*testCase, 0)

	// group-wins-over-name
	tests = append(tests, &testCase{
		_id:       "group-wins-over-name",
		_expected: "backlog",

		routine: &routines.Routine{Group: "backlog", Name: "smine-nightly"},
	})

	// name-prefix-without-group
	tests = append(tests, &testCase{
		_id:       "name-prefix-without-group",
		_expected: "backlog",

		routine: &routines.Routine{Name: "backlog-fdesign-opus"},
	})

	// whole-name-without-dash
	tests = append(tests, &testCase{
		_id:       "whole-name-without-dash",
		_expected: "backlog",

		routine: &routines.Routine{Name: "backlog"},
	})

	// other-family
	tests = append(tests, &testCase{
		_id:       "other-family",
		_expected: "smine",

		routine: &routines.Routine{Name: "smine-nightly"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, routineFamily(test.routine))
		})
	}
}

func Test_backlogRoutinesFor(t *testing.T) {
	list := []routines.Routine{
		{Name: "backlog-fdesign-opus", Skill: "fdesign"},
		{Group: "backlog", Name: "design-fable", Skill: "fdesign"},
		{Name: "fdesign-nightly", Skill: "fdesign"},
		{Name: "backlog-idea", Skill: "idea"},
		{Name: "backlog-budget"},
	}

	matched := backlogRoutinesFor(list, "fdesign")
	names := make([]string, 0, len(matched))
	for _, routine := range matched {
		names = append(names, routine.Name)
	}
	// by name; a routine outside the backlog family never matches, whatever its skill
	assert.Equal(t, []string{"backlog-fdesign-opus", "design-fable"}, names)
	assert.Empty(t, backlogRoutinesFor(list, "smine"))
	assert.Empty(t, backlogRoutinesFor(list, ""))
}

func Test_skillHref(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		installed []skills.Skill
		name      string
	}
	tests := make([]*testCase, 0)

	// repo-preferred
	tests = append(tests, &testCase{
		_id:       "repo-preferred",
		_expected: "/scripts/skills/repo/fdesign",
		installed: []skills.Skill{{Name: "fdesign", Origin: skills.OriginHome}, {Name: "fdesign", Origin: skills.OriginRepo}},
		name:      "fdesign",
	})

	// home-only
	tests = append(tests, &testCase{
		_id:       "home-only",
		_expected: "/scripts/skills/home/fdesign",
		installed: []skills.Skill{{Name: "fdesign", Origin: skills.OriginHome}},
		name:      "fdesign",
	})

	// not-installed
	tests = append(tests, &testCase{
		_id:       "not-installed",
		_expected: "",
		installed: []skills.Skill{{Name: "idea", Origin: skills.OriginRepo}},
		name:      "fdesign",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, skillHref(test.installed, test.name))
		})
	}
}

func Test_resolveSessionPointer(t *testing.T) {
	type testCase struct {
		_id       string
		_expected evidenceLink

		entry string
	}

	refs := map[string]sessions.SessionRef{
		minedSessionId:                         {Scope: "work", Week: fixtureWeek},
		"aaaa1111-0000-0000-0000-000000000000": {Scope: "work", Week: fixtureWeek},
		"aaaa1111-1111-0000-0000-000000000000": {Scope: "work", Week: fixtureWeek},
	}
	minedWeekURL := "/sessions/work/" + fixtureWeek + "?session=" + minedSessionId
	tests := make([]*testCase, 0)

	// bare-id-in-records
	tests = append(tests, &testCase{
		_id:       "bare-id-in-records",
		_expected: evidenceLink{Text: minedSessionId, WeekURL: minedWeekURL},

		entry: minedSessionId,
	})

	// session-prefix-in-records
	tests = append(tests, &testCase{
		_id:       "session-prefix-in-records",
		_expected: evidenceLink{Text: "session 98641eb1", WeekURL: minedWeekURL},

		entry: "session 98641eb1",
	})

	// finding-anchor
	tests = append(tests, &testCase{
		_id:       "finding-anchor",
		_expected: evidenceLink{Text: minedSessionId + "#3", WeekURL: minedWeekURL + "#f-" + minedSessionId + "-3"},

		entry: minedSessionId + "#3",
	})

	// prefix-finding-anchor
	tests = append(tests, &testCase{
		_id:       "prefix-finding-anchor",
		_expected: evidenceLink{Text: "session 98641eb1#0", WeekURL: minedWeekURL + "#f-" + minedSessionId + "-0"},

		entry: "session 98641eb1#0",
	})

	// malformed-finding-text
	tests = append(tests, &testCase{
		_id:       "malformed-finding-text",
		_expected: evidenceLink{Text: minedSessionId + "#x"},

		entry: minedSessionId + "#x",
	})

	// ambiguous-prefix-text
	tests = append(tests, &testCase{
		_id:       "ambiguous-prefix-text",
		_expected: evidenceLink{Text: "session aaaa1111"},

		entry: "session aaaa1111",
	})

	// full-id-unknown-text
	tests = append(tests, &testCase{
		_id:       "full-id-unknown-text",
		_expected: evidenceLink{Text: "cafebabe-0000-0000-0000-000000000000"},

		entry: "cafebabe-0000-0000-0000-000000000000",
	})

	// short-prefix-unknown-text
	tests = append(tests, &testCase{
		_id:       "short-prefix-unknown-text",
		_expected: evidenceLink{Text: "session deadbeef"},

		entry: "session deadbeef",
	})

	// commit-text
	tests = append(tests, &testCase{
		_id:       "commit-text",
		_expected: evidenceLink{Text: "254d9b893"},

		entry: "254d9b893",
	})

	// path-text
	tests = append(tests, &testCase{
		_id:       "path-text",
		_expected: evidenceLink{Text: "internal/server/backlog.go"},

		entry: "internal/server/backlog.go",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, resolveSessionPointer(test.entry, refs))
		})
	}
}

func Test_iceboxDays(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []backlogDayView

		items []backlogItemView
	}

	minedEarly := backlogItemView{Item: backlog.Item{Id: "B-030", Added: "2026-10-04", Mined: &backlog.Mined{Date: "2026-10-03"}}}
	minedLate := backlogItemView{Item: backlog.Item{Id: "B-033", Added: "2026-10-06", Mined: &backlog.Mined{Date: "2026-10-03"}}}
	handFiled := backlogItemView{Item: backlog.Item{Id: "B-031", Added: "2026-10-01"}}
	newest := backlogItemView{Item: backlog.Item{Id: "B-032", Added: "2026-10-05"}}
	tests := make([]*testCase, 0)

	// mined-date-wins
	tests = append(tests, &testCase{
		_id:       "mined-date-wins",
		_expected: []backlogDayView{{IsOpen: true, Items: []backlogItemView{minedEarly}, Key: "2026-10-03", Label: "Sat 2026-10-03"}},

		items: []backlogItemView{minedEarly},
	})

	// added-date-fallback
	tests = append(tests, &testCase{
		_id:       "added-date-fallback",
		_expected: []backlogDayView{{IsOpen: true, Items: []backlogItemView{handFiled}, Key: "2026-10-01", Label: "Thu 2026-10-01"}},

		items: []backlogItemView{handFiled},
	})

	// newest-first-open
	tests = append(tests, &testCase{
		_id: "newest-first-open",
		_expected: []backlogDayView{
			{IsOpen: true, Items: []backlogItemView{newest}, Key: "2026-10-05", Label: "Mon 2026-10-05"},
			{IsOpen: false, Items: []backlogItemView{minedEarly, minedLate}, Key: "2026-10-03", Label: "Sat 2026-10-03"},
			{IsOpen: false, Items: []backlogItemView{handFiled}, Key: "2026-10-01", Label: "Thu 2026-10-01"},
		},

		items: []backlogItemView{minedEarly, handFiled, newest, minedLate},
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: []backlogDayView{},

		items: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, iceboxDays(test.items))
		})
	}
}

func Test_minedByURL(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		base  string
		mined *backlog.Mined
	}

	tests := make([]*testCase, 0)

	// nil-mined
	tests = append(tests, &testCase{
		_id:       "nil-mined",
		_expected: "",

		base:  "http://127.0.0.1:4243/",
		mined: nil,
	})

	// no-miner-recorded
	tests = append(tests, &testCase{
		_id:       "no-miner-recorded",
		_expected: "",

		base:  "http://127.0.0.1:4243/",
		mined: &backlog.Mined{Date: "2026-10-03", Sessions: []string{minedSessionId}},
	})

	// no-dashboard
	tests = append(tests, &testCase{
		_id:       "no-dashboard",
		_expected: "",

		base:  "",
		mined: &backlog.Mined{By: "facefeed-0000-0000-0000-000000000000", Date: "2026-10-03", Sessions: []string{minedSessionId}},
	})

	// miner-linked
	tests = append(tests, &testCase{
		_id:       "miner-linked",
		_expected: "http://127.0.0.1:4243/sessions/facefeed-0000-0000-0000-000000000000",

		base:  "http://127.0.0.1:4243/",
		mined: &backlog.Mined{By: "facefeed-0000-0000-0000-000000000000", Date: "2026-10-03", Sessions: []string{minedSessionId}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, minedByURL(test.base, test.mined))
		})
	}
}

func Test_backlogRouteSkills(t *testing.T) {
	list := []routines.Routine{
		{Name: "backlog-idea", Skill: "idea"},
		{LoadError: "expected exactly one .plist, found 0", Name: "backlog-fdesign", Skill: "fdesign"},
		{Name: "backlog-empty", Skill: ""},
		{Name: "smine-nightly", Skill: "smine"},
		{Name: "backlog-idea-copy", Skill: "idea"},
		{Group: "backlog", Name: "concept-weekly", Skill: "concept"},
		{Group: "smine", Name: "backlog-skills", Skill: "smine-skills"},
	}
	// the explicit group decides before the name prefix
	assert.Equal(t, []string{"concept", "fdesign", "idea"}, backlogRouteSkills(list))
}

func Test_loadedRoutines(t *testing.T) {
	// the host never loads the fixture label
	label := "com.smine.routine.test-fixture.smine-nightly"
	list := []routines.Routine{{Label: label, Name: "smine-nightly"}}

	assert.Equal(t, map[string]bool{label: false}, loadedRoutines(t.Context(), list))
}
