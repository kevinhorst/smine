package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const proposalsFixture = `{
  "kind": "routines",
  "updated": "2026-07-11",
  "source": "sessions/proposals/routines.json",
  "note": "Cumulative, cross-scope, ranked scheduling proposals",
  "groups": [
    {
      "title": "Proposals",
      "proposals": [
        {
          "id": "nightly-session-analysis",
          "title": "nightly-session-analysis",
          "target": "smine",
          "status": "proposed",
          "proposed": "2026-07-20",
          "change": "Wrap smine in a nightly schedule.",
          "fields": [{"label": "Purpose", "text": "Run the retrospective pipeline."}],
          "evidence": [
            {
              "title": "Manual nightly trigger",
              "sessions": [
                {"id": "65a26e92-4c98-4879-82ce-35e644cd0ab5", "note": "triggered by hand every evening"}
              ],
              "snippets": [{"kind": "fix", "lang": "go", "code": "func fixed() {}", "source": "plan"}]
            },
            {
              "title": "Dimension override",
              "dimension": "skill-report-card",
              "sessions": [
                {"id": "65a26e92-4c98-4879-82ce-35e644cd0ab5", "note": "report-card style evidence"},
                {"id": "00000000-0000-0000-0000-000000000000", "note": "pruned session"}
              ]
            }
          ]
        }
      ]
    }
  ]
}`

// proposalsBatchFixture backs the session-link resolution: one work batch
// holding the session id the evidence cites.
const proposalsBatchFixture = `{
  "batch": {"scope": "work", "number": 3, "file": "b3.md"},
  "sessions": [{"id": "65a26e92-4c98-4879-82ce-35e644cd0ab5", "title": "linked session"}]
}`

func TestProposalsPage(t *testing.T) {
	t.Run("renders-loaded-files", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "routines.json"), []byte(proposalsFixture), 0644))
		batchDir := filepath.Join(sessionsDir, "work", "json")
		require.NoError(t, os.MkdirAll(batchDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(batchDir, "batch-03.json"), []byte(proposalsBatchFixture), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "routines")
		// Zero accepted renders no pill at all — nothing to filter on.
		assert.NotContains(t, body, "(0/1)")
		assert.Contains(t, body, "nightly-session-analysis")
		assert.Contains(t, body, "→ smine")
		assert.Contains(t, body, "proposed")
		assert.Contains(t, body, "Run the retrospective pipeline.")
		// meta sits inline in the headline; the note paragraph is gone
		assert.Contains(t, body, `<h2>routines <span class="meta">updated 2026-07-11 · sessions/proposals/routines.json</span></h2>`)
		assert.NotContains(t, body, "Cumulative, cross-scope")

		// evidence table: kind-default dimension link, override link, unresolved id unlinked,
		// one bullet per session with its note
		assert.Contains(t, body, "Manual nightly trigger")
		assert.Contains(t, body, `<li><a href="/sessions/work/3?dimension=routine-candidate&amp;session=65a26e92-4c98-4879-82ce-35e644cd0ab5" title="65a26e92-4c98-4879-82ce-35e644cd0ab5"><code>65a26e92</code></a>: triggered by hand every evening</li>`)
		assert.Contains(t, body, `<a href="/sessions/work/3?dimension=skill-report-card&amp;session=65a26e92-4c98-4879-82ce-35e644cd0ab5"`)
		assert.Contains(t, body, `<li><code class="meta" title="00000000-0000-0000-0000-000000000000">00000000</code>: pruned session</li>`)
		assert.Contains(t, body, "Wrap smine in a nightly schedule.")
		assert.Contains(t, body, "proposed 2026-07-20")
		assert.Contains(t, body, "fix · go · from plan")
		assert.Contains(t, body, "<pre><code>func fixed() {}</code></pre>")
		// Without a filter, sections stay collapsed.
		assert.NotContains(t, body, `data-section="routines/Proposals" open>`)
	})

	t.Run("filters-by-name-and-rule-prefix", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		skillsFixture := `{
		  "kind": "skills",
		  "groups": [
		    {"title": "New skills", "proposals": [
		      {"id": "contract-drift-audit", "title": "contract-drift-audit", "status": "accepted", "decided": "2026-07-21"},
		      {"id": "parallel-review-merge", "title": "parallel-review-merge", "status": "proposed"}
		    ]},
		    {"title": "Edits", "proposals": [
		      {"id": "fdesign--1", "title": "fdesign — a", "status": "proposed"},
		      {"id": "fdesign--2", "title": "fdesign — b", "status": "proposed"}
		    ]},
		    {"title": "Considered, not proposed", "proposals": [{"id": "rejected-idea", "title": "rejected-idea"}]}
		  ]
		}`
		styleFixture := `{
		  "kind": "style",
		  "groups": [
		    {"title": "Go", "proposals": [
		      {"id": "G1", "title": "io idiom", "status": "proposed", "autoApplyHeld": {"date": "2026-07-30", "reason": "touches guard logic"}, "fields": [{"label": "Proposed rule (new, e.g. RULE-INTERFACE-002)", "text": "split on io idiom"}]},
		      {"id": "G3", "title": "naming", "status": "proposed", "fields": [{"label": "Proposed rule (amend RULE-NAME-003)", "text": "full names"}]}
		    ]},
		    {"title": "Reroutes (not rules)", "proposals": [{"id": "R1", "title": "rerouted rule", "fields": [{"label": "Note", "text": "see RULE-TEST-001"}]}]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "skills.json"), []byte(skillsFixture), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "style.json"), []byte(styleFixture), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		// No filter: everything renders, badge rows list the values sorted.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, ">contract-drift-audit</a>")
		assert.Contains(t, body, ">RULE-INTERFACE</a>")
		assert.Contains(t, body, ">RULE-NAME</a>")
		assert.Contains(t, body, "parallel-review-merge")
		// Split siblings share one coarse badge — never per-sibling values.
		assert.Contains(t, body, ">fdesign</a>")
		// A held proposal renders the auto-apply badge with its reason.
		assert.Contains(t, body, "held by auto-apply")
		assert.Contains(t, body, "touches guard logic")
		// Stored decided date renders at the status badge.
		assert.Contains(t, body, `<span class="meta">2026-07-21</span>`)
		assert.NotContains(t, body, ">fdesign--1</a>")
		// Statusless entries never become filter values.
		assert.NotContains(t, body, ">rejected-idea</a>")
		assert.NotContains(t, body, ">RULE-TEST</a>")

		// Name filter: only the matching proposal, empty groups dropped,
		// the other kind's filter is preserved in the badge URLs.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet,
			"/proposals?skills=contract-drift-audit&style=RULE-NAME", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "New skills")
		// The accepted pill stays inactive under an id filter — count only
		// in the tooltip, still full-group: 1 accepted of 2.
		assert.Contains(t, body, ">+</a>")
		assert.Contains(t, body, `title="1 of 2 accepted — click to filter this set"`)
		assert.NotContains(t, body, "<strong>parallel-review-merge</strong>")
		assert.NotContains(t, body, "<summary>Edits")

		// A coarse base-id filter matches every split sibling.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?skills=fdesign", nil))
		require.Equal(t, http.StatusOK, response.Code)
		siblingBody := response.Body.String()
		assert.Contains(t, siblingBody, "fdesign — a")
		assert.Contains(t, siblingBody, "fdesign — b")
		assert.NotContains(t, siblingBody, "contract-drift-audit</strong>")
		// Style prefix filter: only the RULE-NAME proposal remains; the
		// statusless rerouted entry is hidden under any active filter.
		assert.Contains(t, body, "<strong>naming</strong>")
		assert.NotContains(t, body, "io idiom")
		assert.NotContains(t, body, "rerouted rule")
		assert.NotContains(t, body, "rejected-idea")
		// Badge URL for a style value carries the active skills filter.
		assert.Contains(t, body, "skills=contract-drift-audit&amp;style=RULE-INTERFACE")
		// Only the filter's own target sections arrive open, marked so the
		// client pins them; unfiltered kinds' sections stay untouched.
		assert.Contains(t, body, `data-section="skills/New skills" open data-filter-target="1">`)
		assert.NotContains(t, body, `data-section="skills/Edits" open`)
	})

	t.Run("overlays-pending-vote-badge-and-comment", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "routines.json"), []byte(proposalsFixture), 0644))
		votesLine := `{"kind":"routines","id":"nightly-session-analysis","title":"nightly-session-analysis","vote":"+","comment":"do it","ts":"2026-07-22T00:00:00Z"}` + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "proposals", "votes.jsonl"), []byte(votesLine), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Accepted (Pending)")
		assert.NotContains(t, body, `<span class="badge badge-ok">proposed</span>`)
		assert.Contains(t, body, `value="do it"`)
		// The pending vote's timestamp renders as the decided date.
		assert.Contains(t, body, `<span class="meta">2026-07-22</span>`)
	})

	t.Run("stale-vote-ignored", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "routines.json"), []byte(proposalsFixture), 0644))
		votesLine := `{"kind":"routines","id":"ghost","title":"ghost","vote":"+","comment":"gone","ts":"2026-07-22T00:00:00Z"}` + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "proposals", "votes.jsonl"), []byte(votesLine), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "nightly-session-analysis")
		assert.NotContains(t, body, "(Pending")
	})

	t.Run("filters-by-state", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		stateFixture := `{
		  "kind": "skills",
		  "groups": [
		    {"title": "New skills", "proposals": [
		      {"id": "untriaged", "title": "Untriaged", "status": "proposed"},
		      {"id": "voted-plus", "title": "Voted Plus", "status": "proposed"},
		      {"id": "voted-minus", "title": "Voted Minus", "status": "accepted"},
		      {"id": "voted-postpone", "title": "Voted Postpone", "status": "proposed"},
		      {"id": "rerouted", "title": "Rerouted"}
		    ]},
		    {"title": "Other set", "proposals": [
		      {"id": "other-one", "title": "Other One", "status": "proposed"}
		    ]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "skills.json"), []byte(stateFixture), 0644))
		votesLines := `{"kind":"skills","id":"voted-plus","title":"Voted Plus","vote":"+","comment":"","ts":"2026-07-23T00:00:00Z"}
{"kind":"skills","id":"voted-minus","title":"Voted Minus","vote":"-","comment":"","ts":"2026-07-23T00:00:00Z"}
{"kind":"skills","id":"voted-postpone","title":"Voted Postpone","vote":"p","comment":"","ts":"2026-07-23T00:00:00Z"}
`
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "proposals", "votes.jsonl"), []byte(votesLines), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		// No filter: the summary-line segments ARE the state filters; the
		// global filter row carries only id badges. Pending votes win over
		// status, the statusless entry counts nowhere.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		// Inactive pills carry no counts — the tooltip decodes them.
		assert.Contains(t, body, `class="state-note"`)
		assert.Contains(t, body, ">+</a>")
		assert.Contains(t, body, ">−</a>")
		assert.Contains(t, body, ">p</a>")
		// The other set's zero-accepted state renders no pill at all.
		assert.NotContains(t, body, "(0/1)")
		assert.Contains(t, body, `title="1 of 4 accepted — click to filter this set"`)
		assert.Contains(t, body, `title="1 rejected — click to filter this set"`)
		assert.Contains(t, body, `title="1 postponed — click to filter this set"`)
		// State tokens appear only in summary-notation URLs, never as
		// global filter-row badge labels.
		assert.NotContains(t, body, ">accepted</a>")

		// State filter, scoped to group 0: only its pending-accepted
		// proposal remains there; the OTHER group stays complete; the
		// statusless rerouted entry hides; summary numbers stay
		// full-group — filters narrow cards, never the notation.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?skills=state%3A0%3Aaccepted", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "Voted Plus")
		assert.NotContains(t, body, "Voted Minus")
		assert.NotContains(t, body, "Untriaged")
		assert.NotContains(t, body, "Rerouted")
		assert.Contains(t, body, "Other One")
		// The active accepted pill reveals its count as a badge; the
		// rejected pill stays a bare ghost control.
		assert.Contains(t, body, `class="badge badge-ok"`)
		assert.Contains(t, body, ">+ (1/4)</a>")
		assert.Contains(t, body, ">−</a>")
		// Server default: only the filter's target renders open (and marked
		// for the client-side pin); the script owns everything else.
		assert.Contains(t, body, `data-section="skills/New skills" open data-filter-target="1">`)
		assert.Contains(t, body, `data-section="skills/Other set">`)

		// proposed filter on group 0: only its unvoted proposed entry
		// remains; the other set is untouched.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?skills=state%3A0%3Aproposed", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "Untriaged")
		assert.NotContains(t, body, "Voted Plus")
		assert.NotContains(t, body, "Rerouted")
		assert.Contains(t, body, "Other One")
	})

	t.Run("notes-tab", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		tabFixture := `{
		  "kind": "skills",
		  "groups": [
		    {"title": "Votable set", "proposals": [
		      {"id": "one", "title": "Votable One", "status": "proposed"},
		      {"id": "mixed-note", "title": "Mixed Note"}
		    ]},
		    {"title": "Rerouted candidates", "proposals": [
		      {"id": "note-only", "title": "Note Only"}
		    ]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "skills.json"), []byte(tabFixture), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		// Open tab: votable groups only — mixed groups stay with their
		// note cards, fully-statusless groups move to Notes.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Votable set")
		assert.Contains(t, body, "Mixed Note")
		assert.NotContains(t, body, "Rerouted candidates")
		assert.Contains(t, body, `<a href="/proposals?tab=notes" >Notes</a>`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?tab=notes", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "Rerouted candidates")
		assert.Contains(t, body, "Note Only")
		assert.NotContains(t, body, "Votable set")
		assert.Contains(t, body, `<a href="/proposals?tab=notes" class="active">Notes</a>`)
		// Filter values only exist for votable proposals — no filter row here.
		assert.NotContains(t, body, `class="card card-filter"`)
	})

	t.Run("open-tab-empty-message", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		notesOnly := `{
		  "kind": "routines",
		  "groups": [
		    {"title": "Rerouted candidates", "proposals": [{"id": "n1", "title": "Note One"}]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "routines.json"), []byte(notesOnly), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "Everything applied, happy ")
		assert.NotContains(t, body, "Rerouted candidates")
		// The kind heading disappears with its groups.
		assert.NotContains(t, body, "<h2>routines")

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?tab=notes", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "Rerouted candidates")
		assert.NotContains(t, body, "Everything applied")
	})

	t.Run("empty-state", func(t *testing.T) {
		server := newTestServer(t, &Options{SessionsDir: t.TempDir()})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "No proposal JSONs found")
	})

	t.Run("load-error-card", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "broken.json"), []byte("{not json"), 0644))
		server := newTestServer(t, &Options{SessionsDir: sessionsDir})

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "broken.json")
	})
}

// voteFixture holds one votable accepted proposal, one votable proposed
// proposal, and one statusless (non-votable) entry — all under kind skills.
const voteFixture = `{
  "kind": "skills",
  "groups": [
    {"title": "New skills", "proposals": [
      {"id": "accepted-one", "title": "Accepted One", "status": "accepted"},
      {"id": "proposed-one", "title": "Proposed One", "status": "proposed"},
      {"id": "statusless-one", "title": "Statusless One"}
    ]}
  ]
}`

func newVoteServer(t *testing.T) (*Server, string) {
	t.Helper()
	sessionsDir := t.TempDir()
	jsonDir := filepath.Join(sessionsDir, "proposals")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "skills.json"), []byte(voteFixture), 0644))
	return newTestServer(t, &Options{SessionsDir: sessionsDir}), sessionsDir
}

// votesLines returns the non-blank lines of the votes sidecar.
func votesLines(t *testing.T, sessionsDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(sessionsDir, "proposals", "votes.jsonl"))
	require.NoError(t, err)
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestProposalVote(t *testing.T) {
	t.Run("records-vote-and-renders-badge", func(t *testing.T) {
		server, sessionsDir := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"+"}, "comment": {"ship it"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Accepted (Pending)")

		lines := votesLines(t, sessionsDir)
		require.Len(t, lines, 1)
		assert.Contains(t, lines[0], "proposed-one")
		assert.Contains(t, lines[0], "ship it")
	})

	t.Run("replace-vote-single-line", func(t *testing.T) {
		server, sessionsDir := newVoteServer(t)

		first := httptest.NewRecorder()
		server.Handler().ServeHTTP(first, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"+"}}))
		require.Equal(t, http.StatusOK, first.Code)

		second := httptest.NewRecorder()
		server.Handler().ServeHTTP(second, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"-"}}))
		require.Equal(t, http.StatusOK, second.Code)

		lines := votesLines(t, sessionsDir)
		require.Len(t, lines, 1)
		assert.Contains(t, lines[0], `"vote":"-"`)
	})

	t.Run("revert-badge-on-accepted", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/accepted-one/vote",
			url.Values{"vote": {"-"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Rejected (Pending, revert)")
	})

	t.Run("postpone-vote-badge", func(t *testing.T) {
		server, sessionsDir := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"p"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Postponed (Pending)")

		lines := votesLines(t, sessionsDir)
		require.Len(t, lines, 1)
		assert.Contains(t, lines[0], `"vote":"p"`)
	})

	t.Run("postpone-revert-on-accepted", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/accepted-one/vote",
			url.Values{"vote": {"p"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Postponed (Pending, revert)")
	})

	t.Run("invalid-vote-400", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"x"}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("comment-too-long-400", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/proposed-one/vote",
			url.Values{"vote": {"+"}, "comment": {strings.Repeat("a", 501)}}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("unknown-kind-404", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/bogus/proposed-one/vote",
			url.Values{"vote": {"+"}}))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("unknown-id-404", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/nonexistent/vote",
			url.Values{"vote": {"+"}}))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("statusless-entry-404", func(t *testing.T) {
		server, _ := newVoteServer(t)

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/api/proposals/skills/statusless-one/vote",
			url.Values{"vote": {"+"}}))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}
