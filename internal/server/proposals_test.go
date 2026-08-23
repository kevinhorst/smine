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

	"github.com/kevinhorst/smine/internal/proposals"
)

const proposalsFixture = `{
  "kind": "routines",
  "updated": "2026-07-11",
  "source": "proposals/routines.json",
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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		assert.Contains(t, body, `<h2>routines <span class="meta">updated 2026-07-11 · proposals/routines.json</span></h2>`)
		assert.NotContains(t, body, "Cumulative, cross-scope")

		// evidence table: kind-default dimension link, override link, unresolved id unlinked,
		// one bullet per session with its note
		assert.Contains(t, body, "Manual nightly trigger")
		assert.Contains(t, body, `<li><a href="/sessions/work/3?dimension=routine-candidate&amp;session=65a26e92-4c98-4879-82ce-35e644cd0ab5" title="65a26e92-4c98-4879-82ce-35e644cd0ab5"><code>65a26e92</code></a>: triggered by hand every evening</li>`)
		assert.Contains(t, body, `<a href="/sessions/work/3?dimension=skill-report-card&amp;session=65a26e92-4c98-4879-82ce-35e644cd0ab5"`)
		assert.Contains(t, body, `<li><code class="meta" title="00000000-0000-0000-0000-000000000000">00000000</code>: pruned session</li>`)
		assert.Contains(t, body, "Wrap smine in a nightly schedule.")
		assert.Contains(t, body, `<span class="meta">2026-07-20</span>`)
		assert.Contains(t, body, `<details class="card-more">`)
		assert.Contains(t, body, "details · 1 fields · 2 evidence")
		assert.NotContains(t, body, "<th>Evidence</th>")
		assert.NotContains(t, body, "tab=auto-apply")
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
		rulesFixture := `{
		  "kind": "context",
		  "groups": [
		    {"title": "context/rules/go.md", "proposals": [
		      {"id": "G1", "title": "io idiom", "status": "proposed", "gate": {"band": "J"}, "autoApplyHeld": {"date": "2026-07-30", "reason": "touches guard logic"}, "fields": [{"label": "Proposed rule (new, e.g. RULE-GOLANG-INTERFACE-002)", "text": "split on io idiom"}]},
		      {"id": "G3", "title": "naming", "status": "proposed", "gate": {"band": "J"}, "fields": [{"label": "Proposed rule (amend RULE-GOLANG-NAME-003)", "text": "full names"}]}
		    ]},
		    {"title": "Reroutes (not rules)", "proposals": [{"id": "R1", "title": "rerouted rule", "fields": [{"label": "Note", "text": "see RULE-GOLANG-TEST-001"}]}]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "skills.json"), []byte(skillsFixture), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "context.json"), []byte(rulesFixture), 0644))
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

		// No filter: everything renders, badge rows list the values sorted.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, ">contract-drift-audit</a>")
		assert.Contains(t, body, ">RULE-GOLANG-INTERFACE</a>")
		assert.Contains(t, body, ">RULE-GOLANG-NAME</a>")
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
			"/proposals?skills=contract-drift-audit&context=RULE-GOLANG-NAME", nil))
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
		// Context prefix filter: only the RULE-NAME proposal remains; the
		// statusless rerouted entry is hidden under any active filter.
		assert.Contains(t, body, "<strong>naming</strong>")
		assert.NotContains(t, body, "io idiom")
		assert.NotContains(t, body, "rerouted rule")
		assert.NotContains(t, body, "rejected-idea")
		// Badge URL for a context value carries the active skills filter.
		assert.Contains(t, body, "context=RULE-GOLANG-INTERFACE&amp;skills=contract-drift-audit")
		// Only the filter's own target sections arrive open, marked so the
		// client pins them; unfiltered kinds' sections stay untouched.
		assert.Contains(t, body, `data-section="skills/New skills" open data-filter-target="1">`)
		assert.NotContains(t, body, `data-section="skills/Edits" open`)
	})

	t.Run("filters-context-by-band-and-entry-prefix", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		contextFixture := `{
		  "kind": "context",
		  "groups": [
		    {"title": "context/actions/implementing.md", "proposals": [
		      {"id": "C1", "title": "judgment rule", "status": "proposed", "change": "Amend ACTION-IMPL-002: write a plan.", "gate": {"band": "J"}, "fields": [{"label": "Proposed rule (amend ACTION-IMPL-002)", "text": "write a plan first"}]},
		      {"id": "C2", "title": "gated rule", "status": "accepted", "change": "Add a checkable rule.", "gate": {"band": "A", "verifier": "test-schema", "anchor": "_test\\.go$"}},
		      {"id": "C3", "title": "ungated legacy rule", "status": "proposed", "change": "Add without a gate."}
		    ]}
		  ]
		}`
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "context.json"), []byte(contextFixture), 0644))
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

		// No filter: band-derived badges plus the canon entry prefix.
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, ">acdsl</a>")
		assert.Contains(t, body, ">prose</a>")
		assert.Contains(t, body, ">ACTION-IMPL</a>")

		// prose narrows to band J and gateless proposals.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?context=prose", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "judgment rule")
		assert.Contains(t, body, "ungated legacy rule")
		assert.NotContains(t, body, "<strong>gated rule</strong>")

		// acdsl narrows to the F/A/D-banded proposal.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?context=acdsl", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "gated rule")
		assert.NotContains(t, body, "<strong>judgment rule</strong>")
		assert.NotContains(t, body, "<strong>ungated legacy rule</strong>")

		// Entry-prefix filter matches the citing proposal only.
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/proposals?context=ACTION-IMPL", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body = response.Body.String()
		assert.Contains(t, body, "judgment rule")
		assert.NotContains(t, body, "<strong>gated rule</strong>")
	})

	t.Run("overlays-pending-vote-badge-and-comment", func(t *testing.T) {
		sessionsDir := t.TempDir()
		jsonDir := filepath.Join(sessionsDir, "proposals")
		require.NoError(t, os.MkdirAll(jsonDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, "routines.json"), []byte(proposalsFixture), 0644))
		votesLine := `{"kind":"routines","id":"nightly-session-analysis","title":"nightly-session-analysis","vote":"+","comment":"do it","ts":"2026-07-22T00:00:00Z"}` + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "proposals", "votes.jsonl"), []byte(votesLine), 0644))
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
		server := newTestServer(t, &Options{ProposalsDir: t.TempDir(), SessionsDir: t.TempDir()})

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
		server := newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir})

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
	return newTestServer(t, &Options{ProposalsDir: jsonDir, SessionsDir: sessionsDir}), sessionsDir
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

func TestNamedProposalFilterKey(t *testing.T) {
	// workflow proposal → wf: + script basename
	workflow := &proposals.Proposal{Id: "review-defect-to-fix-pipeline", Target: "skills/quality/railroad-review/workflows/review-defect-to-fix.js"}
	assert.Equal(t, "wf:review-defect-to-fix", namedProposalFilterKey(workflow))

	// skill edit → the target skill, not the proposal id
	edit := &proposals.Proposal{Id: "delta-re-review", Target: "railroad-review"}
	assert.Equal(t, "railroad-review", namedProposalFilterKey(edit))

	// new-skill candidate without a target → its own id
	candidate := &proposals.Proposal{Id: "api-collection-from-codebase"}
	assert.Equal(t, "api-collection-from-codebase", namedProposalFilterKey(candidate))

	// split sibling → suffix stripped
	split := &proposals.Proposal{Id: "some-candidate--2"}
	assert.Equal(t, "some-candidate", namedProposalFilterKey(split))

	// path-shaped target outside workflows/ → falls back to the id
	pathTarget := &proposals.Proposal{Id: "doc-fix", Target: "docs/checklist.md"}
	assert.Equal(t, "doc-fix", namedProposalFilterKey(pathTarget))
}

func TestCountLabel(t *testing.T) {
	assert.Equal(t, "0", countLabel(0))
	assert.Equal(t, "9", countLabel(9))
	assert.Equal(t, "99", countLabel(99))
	assert.Equal(t, "99+", countLabel(100))
}

func TestCategoryViews(t *testing.T) {
	t.Run("context-buckets-by-surface", func(t *testing.T) {
		groups := []groupView{
			{Title: "context/rules/go.md", Count: 11},
			{Title: "context/actions/implementing.md", Count: 25, IsFilterTarget: true},
			{Title: "context/rules/sql.md", Count: 1},
			{Title: "context/facts/repo.md", Count: 2},
			{Title: "acdsl/rules.acdsl", Count: 3},
			{Title: "Project-local: tooling", Count: 4},
		}
		categories := categoryViews("context", groups)

		require.Len(t, categories, 5)
		assert.Equal(t, "rules", categories[0].Title)
		assert.Equal(t, "12", categories[0].CountLabel)
		assert.Len(t, categories[0].Groups, 2)
		assert.False(t, categories[0].Solo)
		assert.False(t, categories[0].Groups[0].Solo)
		assert.Equal(t, "context/cat:rules", categories[0].DataSection)
		assert.Equal(t, "actions", categories[1].Title)
		assert.True(t, categories[1].IsFilterTarget)
		assert.Equal(t, "facts", categories[2].Title)
		assert.Equal(t, "acdsl", categories[3].Title)
		assert.Equal(t, "Project-local: tooling", categories[4].Title)
		assert.True(t, categories[4].Solo)
		assert.True(t, categories[4].Groups[0].Solo)
	})

	t.Run("other-kinds-stay-flat", func(t *testing.T) {
		groups := []groupView{
			{Title: "New skills", Count: 0},
			{Title: "Edits to existing skills", Count: 52},
		}
		categories := categoryViews("skills", groups)

		require.Len(t, categories, 2)
		for _, category := range categories {
			assert.True(t, category.Solo)
			assert.Len(t, category.Groups, 1)
		}
	})
}

func TestSubgroupViews(t *testing.T) {
	cardFor := func(id, target string) proposalView {
		card := proposalView{Kind: "skills"}
		card.Proposal.Id = id
		card.Proposal.Target = target
		return card
	}

	t.Run("splits-by-target-in-first-appearance-order", func(t *testing.T) {
		group := groupView{Title: "Edits to existing skills", Total: 3, Proposals: []proposalView{
			cardFor("fdesign--1", "fdesign"),
			cardFor("fchange--1", "fchange"),
			cardFor("fdesign--2", "fdesign"),
		}}
		subgroups := subgroupViews("skills", &group)

		require.Len(t, subgroups, 2)
		assert.Equal(t, "fdesign", subgroups[0].Title)
		assert.Equal(t, "2", subgroups[0].CountLabel)
		assert.Equal(t, "skills/Edits to existing skills/fdesign", subgroups[0].DataSection)
		assert.Equal(t, "fchange", subgroups[1].Title)
		assert.Len(t, subgroups[1].Proposals, 1)
	})

	t.Run("empty-target-falls-back-to-split-id-base", func(t *testing.T) {
		group := groupView{Title: "Proposals", Total: 2, Proposals: []proposalView{
			cardFor("analyze-ledger-drift-check", ""),
			cardFor("analyze-parameterized-tier--1", ""),
		}}
		subgroups := subgroupViews("skills", &group)

		require.Len(t, subgroups, 2)
		assert.Equal(t, "analyze-ledger-drift-check", subgroups[0].Title)
		assert.Equal(t, "analyze-parameterized-tier", subgroups[1].Title)
	})

	t.Run("single-key-stays-flat", func(t *testing.T) {
		group := groupView{Title: "Edits", Total: 2, Proposals: []proposalView{
			cardFor("fdesign--1", "fdesign"),
			cardFor("fdesign--2", "fdesign"),
		}}
		assert.Nil(t, subgroupViews("skills", &group))
	})

	t.Run("context-and-statusless-groups-never-subgroup", func(t *testing.T) {
		votable := groupView{Title: "context/rules/go.md", Total: 2, Proposals: []proposalView{
			cardFor("G9--1", "a"), cardFor("G9--2", "b"),
		}}
		assert.Nil(t, subgroupViews("context", &votable))

		notes := groupView{Title: "Reroute notes", Total: 0, Proposals: []proposalView{
			cardFor("n1", "a"), cardFor("n2", "b"),
		}}
		assert.Nil(t, subgroupViews("skills", &notes))
	})
}
