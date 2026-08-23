package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStubPeek(t *testing.T, sessionItems []map[string]any) *httptest.Server {
	t.Helper()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Id     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result map[string]any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": request.Params["protocolVersion"],
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "stub-peek", "version": "1.0.5"},
			}
		case "tools/call":
			result = map[string]any{
				"content":           []any{},
				"isError":           false,
				"structuredContent": map[string]any{"sessions": sessionItems},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": request.Id, "result": result}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(stub.Close)
	return stub
}

func TestBatchPagePeekLinks(t *testing.T) {
	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "personal", "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))

	server := newTestServer(t, &Options{
		PeekDashboardURL: "http://127.0.0.1:4243/",
		SessionsDir:      sessionsDir,
	})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `href="http://127.0.0.1:4243/sessions/abc123"`, "session id links to peek")
	assert.Contains(t, body, `href="http://127.0.0.1:4243/sessions/def456"`, "stale/unknown session id links to peek too")
}

func TestBatchPageNoDashboardNoLinks(t *testing.T) {
	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "personal", "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "open in peek")
}

const batchFixture = `{
  "batch": {"scope": "personal", "number": 1, "file": "batch1.md", "theme": "Test theme",
            "analyzedDate": "2026-07-16", "dateRange": "2026-07-10 → 2026-07-16"},
  "sessions": [
    {
      "id": "abc123",
      "title": "First session",
      "signal": "yes (high)",
      "findings": [{"dimension": "memory", "summary": "a finding", "quotes": ["a quote"], "snippets": [{"kind": "violation", "lang": "go", "code": "func broken() {}", "source": "transcript"}]}],
      "frustration": [{"quote": "argh", "trigger": "the trigger"}]
    },
    {
      "id": "def456",
      "title": "Second session",
      "findings": [{"dimension": "rule", "summary": "another finding", "quotes": []}],
      "positive": [{"quote": "worked great", "trigger": "clean run"}]
    }
  ],
  "arcs": []
}`

func TestBatchPageDivergentScopeURLs(t *testing.T) {
	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "work", "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	divergent := `{
	  "batch": {"scope": "aqms", "number": 1, "file": "batch1.md"},
	  "sessions": [{"id": "abc123", "title": "First session", "findings": [{"dimension": "memory", "summary": "a finding"}]}],
	  "arcs": []
	}`
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(divergent), 0644))
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/work/1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `hx-get="/sessions/work/1`, "htmx URLs key off the directory scope")
	assert.NotContains(t, body, "aqms", "the JSON scope value never reaches the page")
}

func TestSessionsPages(t *testing.T) {
	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "personal", "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "batch1.json"), []byte(batchFixture), 0644))
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	t.Run("index-redirects-to-first-scope", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions", nil))
		require.Equal(t, http.StatusFound, response.Code)
		assert.Equal(t, "/sessions/personal", response.Header().Get("Location"))
	})

	t.Run("scope-page-lists-batches", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "personal")
		assert.Contains(t, response.Body.String(), "Test theme")
		// band summary carries the range spanning its batches
		assert.Contains(t, response.Body.String(), `(1) <span class="meta">2026-07-10 → 2026-07-16</span>`)
	})

	t.Run("batch-page-renders", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, "memory")
		assert.Contains(t, body, "a quote")
		assert.Contains(t, body, "violation · go · from transcript")
		assert.Contains(t, body, "<pre><code>func broken() {}</code></pre>")
		assert.Contains(t, body, "<html")
		assert.Contains(t, body, `id="batch-body"`)
		assert.Contains(t, body, `class="session-select"`)
		assert.Contains(t, body, `title="yes (high)">analyzed<`)
	})

	t.Run("reload-renders-ok-and-triggers-refresh", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions/reload", url.Values{}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), ">Ok</span>")
		assert.Equal(t, "sessions-reload", response.Header().Get("HX-Trigger"))
	})

	t.Run("session-filter-single-id", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?session=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.NotContains(t, body, `id="session-def456"`)
	})

	t.Run("collapsed-by-default-expanded-when-selected", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), `id="session-abc123" class="card card-column session-card" open`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?session=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `id="session-abc123" class="card card-column session-card" open`)
	})

	t.Run("open-param-expands-only-target-card", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?open=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123" class="card card-column session-card" open`)
		assert.NotContains(t, body, `id="session-def456" class="card card-column session-card" open`)
		assert.Contains(t, body, `id="session-def456"`)
	})

	t.Run("session-filter-multi-id", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?session=abc123,def456", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, `id="session-def456"`)
	})

	t.Run("dimension-filter-fragment", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/sessions/personal/1?dimension=memory", nil)
		request.Header.Set("HX-Request", "true")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="batch-body"`)
		assert.Contains(t, body, `id="session-list"`)
		assert.NotContains(t, body, "<html")
	})

	t.Run("unknown-batch-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/99", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("pseudo-dimensions-in-filter-bar", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "?dimension=frustration")
		assert.Contains(t, body, "?dimension=positive")
	})

	t.Run("positive-filter-keeps-only-positive-sessions", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?dimension=positive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-def456"`)
		assert.NotContains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, `<span class="badge badge-ok">positive</span>`)
		assert.Contains(t, body, `class="meta positive"`)
		assert.Contains(t, body, "worked great")
	})

	t.Run("frustration-filter-shows-quote-block", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?dimension=frustration", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.NotContains(t, body, `id="session-def456"`)
		assert.Contains(t, body, `<span class="badge badge-error">frustration</span>`)
		assert.Contains(t, body, "argh")
		// the finding-dimension filter still hides the quote blocks
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/1?dimension=memory", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "argh")
	})
}

func TestThemeBullets(t *testing.T) {
	tests := []struct {
		name    string
		theme   string
		label   string
		bullets []string
	}{
		{
			name:    "label-and-comma-split",
			theme:   "Centerpieces: alpha review, beta incident",
			label:   "Centerpieces:",
			bullets: []string{"alpha review", "beta incident"},
		},
		{
			name:    "semicolon-wins-over-comma",
			theme:   "topics: a, b; c, d",
			label:   "topics:",
			bullets: []string{"a, b", "c, d"},
		},
		{
			name:    "no-colon",
			theme:   "one thing, another thing",
			label:   "",
			bullets: []string{"one thing", "another thing"},
		},
		{
			name:    "empty",
			theme:   "",
			label:   "",
			bullets: nil,
		},
		{
			name:    "single-fragment",
			theme:   "just one centerpiece",
			label:   "",
			bullets: []string{"just one centerpiece"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			label, bullets := themeBullets(test.theme)
			assert.Equal(t, test.label, label)
			assert.Equal(t, test.bullets, bullets)
		})
	}
}

func TestVisibleSessions(t *testing.T) {
	batch := &sessions.BatchSummary{
		Sessions: []sessions.Session{
			{
				Id: "s1",
				Findings: []sessions.Finding{
					{Dimension: "memory", Summary: "m1"},
					{Dimension: "rule", Summary: "r1"},
				},
				Frustration: []sessions.Frustration{{Quote: "argh", Trigger: "t"}},
			},
			{
				Id:       "s2",
				Findings: []sessions.Finding{{Dimension: "rule", Summary: "r2"}},
				Positive: []sessions.Positive{{Quote: "nice", Trigger: "t"}},
			},
		},
	}

	tests := []struct {
		name         string
		dimension    string
		sessionIds   []string
		wantIds      []string
		wantSelected bool
		wantSummary  map[string][]string
	}{
		{
			name:        "no-filters-all-sessions",
			wantIds:     []string{"s1", "s2"},
			wantSummary: map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:        "dimension-hides-non-matching",
			dimension:   "memory",
			wantIds:     []string{"s1"},
			wantSummary: map[string][]string{"s1": {"m1"}},
		},
		{
			name:         "session-id-filter",
			sessionIds:   []string{"s2"},
			wantIds:      []string{"s2"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s2": {"r2"}},
		},
		{
			name:         "multiple-session-ids",
			sessionIds:   []string{"s1", "s2"},
			wantIds:      []string{"s1", "s2"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:         "both-combined",
			dimension:    "rule",
			sessionIds:   []string{"s1"},
			wantIds:      []string{"s1"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s1": {"r1"}},
		},
		{
			name:       "combined-no-match",
			dimension:  "memory",
			sessionIds: []string{"s2"},
			wantIds:    nil,
		},
		{
			name:      "frustration-pseudo-dimension",
			dimension: "frustration",
			wantIds:   []string{"s1"},
		},
		{
			name:      "positive-pseudo-dimension",
			dimension: "positive",
			wantIds:   []string{"s2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			views := visibleSessions(batch, test.dimension, test.sessionIds)
			var ids []string
			for _, view := range views {
				ids = append(ids, view.Session.Id)
				assert.Equal(t, test.wantSelected, view.Selected)
				var summaries []string
				for _, finding := range view.Findings {
					summaries = append(summaries, finding.Summary)
				}
				assert.Equal(t, test.wantSummary[view.Session.Id], summaries)
			}
			assert.Equal(t, test.wantIds, ids)
		})
	}
}

func TestBatchBands(t *testing.T) {
	batch := func(n int) *sessions.BatchSummary {
		return &sessions.BatchSummary{Batch: sessions.Batch{Number: n}}
	}

	t.Run("groups-by-decade", func(t *testing.T) {
		bands := batchBands([]*sessions.BatchSummary{batch(1), batch(9), batch(10), batch(11), batch(20), batch(21)})
		require.Len(t, bands, 3)
		assert.Equal(t, "Batch 1-10", bands[0].Label)
		assert.Len(t, bands[0].Batches, 3)
		assert.Equal(t, "Batch 11-20", bands[1].Label)
		assert.Len(t, bands[1].Batches, 2)
		assert.Equal(t, "Batch 21-30", bands[2].Label)
		assert.Len(t, bands[2].Batches, 1)
	})

	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, batchBands(nil))
	})

	t.Run("bands-carry-date-range", func(t *testing.T) {
		dated := &sessions.BatchSummary{Batch: sessions.Batch{Number: 1, DateRange: "2026-07-10 → 2026-07-16"}}
		bands := batchBands([]*sessions.BatchSummary{dated})
		require.Len(t, bands, 1)
		assert.Equal(t, "2026-07-10 → 2026-07-16", bands[0].DateRange)
	})
}

func TestBandDateRange(t *testing.T) {
	batch := func(dateRange, analyzedDate string) *sessions.BatchSummary {
		return &sessions.BatchSummary{Batch: sessions.Batch{AnalyzedDate: analyzedDate, DateRange: dateRange}}
	}

	tests := []struct {
		name    string
		batches []*sessions.BatchSummary
		want    string
	}{
		{
			name: "mixed-free-form-ranges-span-min-max",
			batches: []*sessions.BatchSummary{
				batch("2026-06-29 → 2026-07-07", ""),
				batch("2026-05-27 -> 2026-05-29", ""),
			},
			want: "2026-05-27 → 2026-07-07",
		},
		{
			name:    "single-date-no-arrow",
			batches: []*sessions.BatchSummary{batch("2026-06-17", "")},
			want:    "2026-06-17",
		},
		{
			name:    "fallback-to-analyzed-date",
			batches: []*sessions.BatchSummary{batch("", "2026-07-03")},
			want:    "2026-07-03",
		},
		{
			name:    "no-dates-anywhere",
			batches: []*sessions.BatchSummary{batch("", ""), batch("prose only", "")},
			want:    "",
		},
		{
			name:    "short-form-suffix-ignored",
			batches: []*sessions.BatchSummary{batch("2026-06-15/16", "")},
			want:    "2026-06-15",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, bandDateRange(test.batches))
		})
	}
}

func TestSignalLabel(t *testing.T) {
	tests := []struct {
		signal string
		want   string
	}{
		{signal: "yes", want: "analyzed"},
		{signal: "yes (high)", want: "analyzed"},
		{signal: "Yes (clean)", want: "analyzed"},
		{signal: "exemplar", want: "exemplar"},
		{signal: "HIGH VALUE — pattern", want: "HIGH VALUE — pattern"},
		{signal: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.signal, func(t *testing.T) {
			assert.Equal(t, test.want, signalLabel(test.signal))
		})
	}
}

func TestParseSessionIds(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{
			name:  "repeated-params",
			query: url.Values{"session": {"a", "b"}},
			want:  []string{"a", "b"},
		},
		{
			name:  "comma-joined",
			query: url.Values{"session": {"a,b,c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "mixed",
			query: url.Values{"session": {"a,b", "c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty-entries-dropped",
			query: url.Values{"session": {" , a ,", ""}},
			want:  []string{"a"},
		},
		{
			name:  "absent",
			query: url.Values{},
			want:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, parseSessionIds(test.query))
		})
	}
}

func TestToggleAllURL(t *testing.T) {
	batch := &sessions.BatchSummary{
		Batch:    sessions.Batch{Number: 3, Scope: "work"},
		Sessions: []sessions.Session{{Id: "s1"}, {Id: "s2"}},
	}

	t.Run("selection-active-clears", func(t *testing.T) {
		assert.Equal(t, "/sessions/work/3", toggleAllURL(batch, "", []string{"s1"}))
	})

	t.Run("empty-selection-selects-all", func(t *testing.T) {
		assert.Equal(t, "/sessions/work/3?session=s1%2Cs2", toggleAllURL(batch, "", nil))
	})

	t.Run("dimension-preserved", func(t *testing.T) {
		assert.Equal(t, "/sessions/work/3?dimension=memory", toggleAllURL(batch, "memory", []string{"s1"}))
		assert.Equal(t, "/sessions/work/3?dimension=memory&session=s1%2Cs2", toggleAllURL(batch, "memory", nil))
	})
}
