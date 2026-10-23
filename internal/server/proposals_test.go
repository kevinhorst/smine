package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/backlog"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	nightOlder = repos.NightBranchPrefix + "2026-10-03" + repos.ProposalsSuffix
	nightNewer = repos.NightBranchPrefix + "2026-10-04" + repos.ProposalsSuffix
)

// changesStub stands in for agentwt on the Proposals page: `unplaced` prints
// unplaced-<branch basename>.json; `probe` answers every sha with
// probe-<sha>.json when present, else an applies entry, and logs the call to
// probes; a `pick` logs its argv tail to picks and prints pick-<n>.json,
// exiting with pick-<n>.exit.
const changesStub = `#!/bin/sh
dir=$(dirname "$0")
cmd="$5"
shift 5
for last; do :; done
if [ "$cmd" = "unplaced" ]; then
	cat "$dir/unplaced-${last##*/}.json"
	exit 0
fi
if [ "$cmd" = "probe" ]; then
	shift 2
	echo "$*" >> "$dir/probes"
	printf '{"schema":1,"onto":"main","onto_sha":"0","commits":['
	sep=""
	for sha; do
		printf '%s' "$sep"
		if [ -f "$dir/probe-$sha.json" ]; then
			cat "$dir/probe-$sha.json"
		else
			printf '{"commit":"%s","outcome":"applies"}' "$sha"
		fi
		sep=","
	done
	printf ']}'
	exit 0
fi
if [ "$cmd" = "pick" ]; then
	echo "$*" >> "$dir/picks"
	n=$(wc -l < "$dir/picks" | tr -d ' ')
	echo "agentwt: picking $last" >&2
	cat "$dir/pick-$n.json"
	exit "$(cat "$dir/pick-$n.exit")"
fi
echo "agentwt: unknown command $cmd" >&2
exit 1
`

// changesFixture is a repo with smine proposals branches, the routines dir
// inside it and an agentwt stub listing the unplaced commits.
type changesFixture struct {
	backlogDir  string
	repo        string
	routinesDir string
	shas        map[string]string
	stubDir     string
}

// newChangesFixture builds the repo: the older night holds change A1, the
// trailer-less record A2 and the placed change A3 (the stub lists only A1
// and A2); the newer night holds B1 and the consolidated B2 with two ids.
// A1 conflicts with main on a dry run; A2 carries a dropped ref. Run reports
// judge the newer night only: B1 ready (short sha, newest report), B2
// rejected; the newest report also holds B1's task record.
func newChangesFixture(t *testing.T) *changesFixture {
	t.Helper()
	fixture := newRepoFixture(t)

	fixture.git(t, "switch", "-q", "-c", nightOlder)
	fixture.commit(t, "A1", "a.md", "context.actions: Added rule A\n\nSessions kept editing conflict hunks.\n\nThe rest of the finding.\n\nSmine-Change: context/rule-a\nSmine-Sessions: ghi789, unknown-session")
	fixture.commit(t, "A2", "record.md", "sessions: Recorded nightly output")
	fixture.commit(t, "A3", "placed.md", "skills.x: Placed change\n\nSmine-Change: skills/placed")
	fixture.git(t, "switch", "-q", "main")

	fixture.git(t, "switch", "-q", "-c", nightNewer)
	fixture.commit(t, "B1", "settings.json", "permissions: Allowed jq\n\nSessions were denied jq.\n\nSmine-Change: permissions/allow-jq\nSmine-Sessions: ghi789")
	fixture.commit(t, "B2", "skill.cue", "skills.y: Tightened the y and z skills\n\nSmine-Change: skills/y\nSmine-Change: skills/z")
	fixture.git(t, "switch", "-q", "main")

	fixture.writeUnplaced(t, nightOlder, "A1", "A2")
	fixture.writeUnplaced(t, nightNewer, "B1", "B2")
	fixture.writeProbeConflict(t, "A1", "a.md conflicts with main")
	fixture.git(t, "update-ref", repos.DroppedRefPrefix+fixture.shas["A2"], fixture.shas["A2"])

	runsDir := filepath.Join(fixture.routinesDir, pinnedRoutine, "runs")
	require.NoError(t, os.MkdirAll(runsDir, 0o755))
	older := routines.RunReport{
		Branch:  nightNewer,
		Changes: []routines.RunReportChange{{Id: "permissions/allow-jq", Reason: "superseded verdict", Sha: fixture.shas["B1"], Verdict: verdictRejected}},
		RunId:   "20261003T030000Z",
	}
	evidence := []routines.ChangeEvidence{{Finding: 2, Quote: "jq was denied again", Session: "ghi789"}}
	tasks := []routines.ChangeTask{{Id: "permissions/allow-jq", Backlog: "B-022", Change: "Allow jq.", Evidence: evidence, Fix: "The allow rule admits jq.", Kind: "permissions", Problem: "Sessions were denied jq.", Sessions: []string{"ghi789"}, Target: "settings.json", Title: "Allow jq"}}
	newer := routines.RunReport{
		Branch: nightNewer,
		Changes: []routines.RunReportChange{
			{Id: "permissions/allow-jq", Sha: fixture.shas["B1"][:8], Verdict: verdictReady},
			{Id: "skills/y", Reason: "breaks the skills gate", Sha: fixture.shas["B2"], Verdict: verdictRejected},
		},
		RunId: "20261004T030000Z",
		Tasks: tasks,
	}
	conflicting := routines.RunReport{
		Branch:  nightOlder,
		Changes: []routines.RunReportChange{{Id: "context/rule-a", Sha: fixture.shas["A1"], Verdict: verdictReady}},
		RunId:   "20261002T030000Z",
	}
	writeRunReport(t, runsDir, &conflicting)
	writeRunReport(t, runsDir, &older)
	writeRunReport(t, runsDir, &newer)
	return fixture
}

// newRepoFixture is a repo with one base commit on main and the agentwt stub.
func newRepoFixture(t *testing.T) *changesFixture {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	fixture := &changesFixture{
		backlogDir:  writeBacklogStore(t, `{"archive": {"done": [], "rejected": []}, "changelog": [], "sections": [], "updated": "2000-01-01"}`),
		repo:        repo,
		routinesDir: filepath.Join(repo, "routines"),
		shas:        map[string]string{},
		stubDir:     t.TempDir(),
	}
	fixture.git(t, "init", "-q", "-b", "main")
	fixture.git(t, "commit", "-q", "--allow-empty", "-m", "base")

	stub := filepath.Join(fixture.stubDir, "agentwt.sh")
	require.NoError(t, os.WriteFile(stub, []byte(changesStub), 0o755))
	return fixture
}

func (f *changesFixture) commit(t *testing.T, label, file, message string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.repo, file), []byte(label+"\n"), 0o644))
	f.git(t, "add", file)
	f.git(t, "commit", "-q", "-m", message)
	f.shas[label] = strings.TrimSpace(f.git(t, "rev-parse", "HEAD"))
}

func (f *changesFixture) git(t *testing.T, args ...string) string {
	t.Helper()
	argv := append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)
	output, err := shell.Run(context.Background(), f.repo, "git", argv...)
	require.NoError(t, err, output)
	return output
}

// picks returns the stub's logged pick argv tails; empty when none ran.
func (f *changesFixture) picks() string {
	content, err := os.ReadFile(filepath.Join(f.stubDir, "picks"))
	if err != nil {
		return ""
	}
	return string(content)
}

// probes counts the stub's logged probe calls; zero before the first.
func (f *changesFixture) probes() int {
	content, err := os.ReadFile(filepath.Join(f.stubDir, "probes"))
	if err != nil {
		return 0
	}
	return strings.Count(string(content), "\n")
}

// rejectedRefs lists the repo's rejected refs as "<refname> <sha>" lines.
func (f *changesFixture) rejectedRefs(t *testing.T) string {
	t.Helper()
	return f.git(t, "for-each-ref", "--format=%(refname) %(objectname)", repos.RejectedRefPrefix)
}

// server builds a config server whose routines dir sits in the fixture repo.
func (f *changesFixture) server(t *testing.T) *Server {
	t.Helper()
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	writeOverviewSessions(t, sessionsDir)
	options := &Options{
		AgentwtPath: filepath.Join(f.stubDir, "agentwt.sh"),
		BacklogDir:  f.backlogDir,
		RoutinesDir: f.routinesDir,
		SessionsDir: sessionsDir,
	}
	return newTestServer(t, options)
}

// writeProbeConflict makes the stub's probe answer conflict for the labelled commit.
func (f *changesFixture) writeProbeConflict(t *testing.T, label, reason string) {
	t.Helper()
	sha := f.shas[label]
	entry := `{"commit":"` + sha + `","outcome":"conflict","reason":"` + reason + `"}`
	require.NoError(t, os.WriteFile(filepath.Join(f.stubDir, "probe-"+sha+".json"), []byte(entry), 0o644))
}

// writePick pins the stub's document and exit code for its n-th real pick.
func (f *changesFixture) writePick(t *testing.T, n int, document string, exitCode int) {
	t.Helper()
	name := "pick-" + strconv.Itoa(n)
	require.NoError(t, os.WriteFile(filepath.Join(f.stubDir, name+".json"), []byte(document), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.stubDir, name+".exit"), []byte(strconv.Itoa(exitCode)), 0o644))
}

// writeUnplaced writes the stub's unplaced document for one branch, the
// labelled commits in parent-first order.
func (f *changesFixture) writeUnplaced(t *testing.T, branch string, labels ...string) {
	t.Helper()
	commits := make([]repos.UnplacedCommit, 0, len(labels))
	for _, label := range labels {
		sha := f.shas[label]
		commit := repos.UnplacedCommit{CommittedAt: "2026-10-04T03:00:00Z", Sha: sha, ShortSha: sha[:8], Subject: label}
		commits = append(commits, commit)
	}
	document := repos.Unplaced{Branch: branch, Commits: commits, Onto: changeTarget, OntoSha: "0", OntoShortSha: "0", Schema: 1}
	encoded, err := json.Marshal(document)
	require.NoError(t, err)
	path := filepath.Join(f.stubDir, "unplaced-"+filepath.Base(branch)+".json")
	require.NoError(t, os.WriteFile(path, encoded, 0o644))
}

func writeRunReport(t *testing.T, runsDir string, report *routines.RunReport) {
	t.Helper()
	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runsDir, report.RunId+".json"), encoded, 0o644))
}

// requireProposalsPage skips where the Proposals page cannot exist: it needs Mining, which runs on darwin and windows only.
func requireProposalsPage(t *testing.T) {
	t.Helper()
	if !components.Available(components.Mining, runtime.GOOS) {
		t.Skip("the Proposals page needs Mining, unavailable on " + runtime.GOOS)
	}
}

func TestHandleProposals_ChangeCards(t *testing.T) {
	requireProposalsPage(t)
	fixture := newChangesFixture(t)
	server := fixture.server(t)

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()

	lastActive, err := time.Parse(time.RFC3339, "2026-07-21T10:00:00Z")
	require.NoError(t, err)
	sessionDays := int(time.Since(lastActive).Hours() / 24)

	// nights group the cards, newest first
	assert.Less(t, strings.Index(body, `data-section="`+nightNewer+`"`), strings.Index(body, `data-section="`+nightOlder+`"`))
	assert.Contains(t, body, "<code>2026-10-04</code>")
	// only unplaced trailer commits become cards
	assert.Contains(t, body, "context.actions: Added rule A")
	assert.Contains(t, body, "permissions: Allowed jq")
	assert.NotContains(t, body, "sessions: Recorded nightly output")
	assert.NotContains(t, body, "skills.x: Placed change")
	// every open card is selectable, so a conflict can still be rejected; the conflict card shows the reason
	assert.Contains(t, body, `name="sha" value="`+fixture.shas["B1"]+`"`)
	assert.Contains(t, body, `name="sha" value="`+fixture.shas["A1"]+`"`)
	assert.Contains(t, body, "a.md conflicts with main")
	// a recorded change leads with its problem sentence, the subject as the second line, the backlog chip, quotes linked to the finding
	assert.Contains(t, body, `<span class="proposal-title">Sessions were denied jq. <span class="badge badge-dim">B-022</span></span>`)
	assert.Contains(t, body, `<span class="proposal-subject">permissions: Allowed jq</span>`)
	assert.Contains(t, body, `<blockquote class="evidence-quote">jq was denied again <a class="badge badge-dim" href="/sessions/personal/2026-W30?session=ghi789#f-ghi789-2" title="ghi789">ghi789</a></blockquote>`)
	assert.NotContains(t, body, "The allow rule admits jq.")
	assert.NotContains(t, body, "proposal-solution")
	// a change without a record shows its subject and its body as the finding; trailers hidden
	assert.Contains(t, body, `<span class="proposal-title">context.actions: Added rule A</span>`)
	assert.Contains(t, body, `<span class="badge badge-dim">finding</span><span class="finding-content label change-body">Sessions kept editing conflict hunks.`)
	assert.NotContains(t, body, "Smine-Change: permissions/allow-jq")
	// the changes row holds one toggle per file, path first, counts and status marker on the right
	assert.Contains(t, body, `<div class="finding change-files"><span class="badge badge-dim">changes</span>`)
	assert.Contains(t, body, `<summary><span class="change-path">settings.json</span><span class="meta">+1 −0</span><code class="change-status">A</code></summary>`)
	assert.Contains(t, body, " · +1 −0 · ")
	// the short sha closes the header, the caret alone toggles the card
	assert.Contains(t, body, `<code class="proposal-sha">`+fixture.shas["B1"][:8]+`</code>`)
	assert.Contains(t, body, `<span class="proposal-caret"`)
	assert.Contains(t, body, `onclick="if (!event.target.closest('.proposal-caret')) event.preventDefault()"`)
	// every applying card carries its own pick button posting its sha in the path, no form fields; the conflict card has none
	assert.Equal(t, 2, strings.Count(body, "pick-one"))
	assert.NotContains(t, body, `hx-post="/api/proposals/pick/`+fixture.shas["A1"]+`"`)
	assert.Contains(t, body, `hx-post="/api/proposals/pick/`+fixture.shas["B1"]+`" hx-params="none"`)
	// every open card, conflicts included, carries its own reject button
	assert.Equal(t, 3, strings.Count(body, "reject-one"))
	assert.Less(t, strings.Index(body, `hx-post="/api/proposals/pick/`+fixture.shas["B1"]+`"`), strings.Index(body, `hx-post="/api/proposals/reject/`+fixture.shas["B1"]+`"`))
	assert.Contains(t, body, `hx-post="/api/proposals/reject/`+fixture.shas["A1"]+`" hx-params="none"`)
	assert.Contains(t, body, `<span class="diff-add">`)
	assert.NotContains(t, body, `<details class="snippet">`)
	// verdicts: newest report wins by short sha, rejected carries its reason, a conflict overrides the judge's ready
	assert.Equal(t, 1, strings.Count(body, `<span class="badge badge-ok">ready</span>`))
	assert.Contains(t, body, `<span class="badge badge-action" title="breaks the skills gate">rejected</span>`)
	assert.Contains(t, body, `<span class="badge badge-action" title="a.md conflicts with main">conflict</span>`)
	assert.NotContains(t, body, "superseded verdict")
	// the conflicting change sits in its night's closed conflicts block with the agentwt reason; no state badge
	conflicts := strings.Index(body, `<details class="conflicts">`)
	require.GreaterOrEqual(t, conflicts, 0)
	assert.Less(t, conflicts, strings.Index(body, `id="change-`+fixture.shas["A1"][:8]+`"`))
	assert.Contains(t, body, `<span class="badge badge-action">conflict</span><span class="finding-content label">a.md conflicts with main</span>`)
	assert.NotContains(t, body, ">stale</span>")
	assert.NotContains(t, body, "reapply failed")
	assert.NotContains(t, body, " · skills/y · skills/z · "+nightNewer)
	// sessions link to the mined record and carry its age
	assert.Contains(t, body, `<a href="/sessions/personal/2026-W30?session=ghi789" title="ghi789">`)
	assert.Contains(t, body, `<code title="unknown-session">unknown-</code>`)
	assert.Contains(t, body, fmt.Sprintf(" · ~%dd ago · <code class=\"proposal-sha\">", sessionDays))
	// chip counts
	assert.NotContains(t, body, `data-group="state"`)
	assert.Contains(t, body, `data-group="verdict" data-value="">all 3</a>`)
	assert.Contains(t, body, `data-group="verdict" data-value="ready">ready 1</a>`)
	assert.Contains(t, body, `data-group="verdict" data-value="rejected">rejected 1</a>`)
	assert.NotContains(t, body, `data-group="verdict" data-value="ready">ready 2</a>`)
	assert.Contains(t, body, `data-group="kind" data-value="skills">skills 1</a>`)
	assert.Contains(t, body, `data-group="night" data-value="2026-10-04">2026-10-04 2</a>`)
	assert.Contains(t, body, `data-group="night" data-value="2026-10-03">2026-10-03 1</a>`)
	// one pick and one reject action per page, both over the selection; no paused line below the limit
	assert.Equal(t, 1, strings.Count(body, `hx-post="/api/proposals/reject"`))
	assert.Equal(t, 1, strings.Count(body, `hx-post="/api/proposals/pick"`))
	assert.NotContains(t, body, "proposals-paused")
	assert.NotContains(t, body, "No open changes.")
	// one probe call served the page and the nav count
	assert.Equal(t, 1, fixture.probes())

	// a second request renders from the store: no new probe
	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, fixture.probes())
}

func TestHandleProposals_History(t *testing.T) {
	requireProposalsPage(t)
	fixture := newChangesFixture(t)
	fixture.git(t, "update-ref", repos.RejectedRefPrefix+fixture.shas["B1"], fixture.shas["B1"])
	fixture.commit(t, "B2-picked", "picked.md", "skills.y: Tightened the y and z skills\n\nSmine-Change: skills/y\nSmine-Change: skills/z\n(cherry picked from commit "+fixture.shas["B2"]+")")
	droppedNight := strings.TrimSpace(fixture.git(t, "show", "--no-patch", "--format=%cs", fixture.shas["A2"]))
	recorder := httptest.NewRecorder()

	fixture.server(t).Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals?tab=history", nil))
	body := recorder.Body.String()

	assert.Equal(t, http.StatusOK, recorder.Code)
	// the open count comes from the store, no card is built
	assert.Contains(t, body, `>Open <span class="count-note">2</span></a>`)
	assert.NotContains(t, body, "card-proposal\" id=")
	// the picked change sits under its proposal's night beside the rejected one, the proposal sha and the badge at the right
	assert.Contains(t, body, `<summary><code>2026-10-04</code> <span class="count-note">2</span></summary>`)
	assert.Contains(t, body, `<span class="proposal-title">skills.y: Tightened the y and z skills</span>`)
	assert.NotContains(t, body, `<span class="proposal-subject">skills.y: Tightened the y and z skills</span>`)
	assert.Contains(t, body, `<code class="proposal-sha">`+fixture.shas["B2"][:8]+"</code>\n        "+`<span class="badge badge-ok">picked</span>`)
	// the rejected change under its report's night, problem sentence and subject
	assert.Contains(t, body, `<code class="proposal-sha">`+fixture.shas["B1"][:8]+"</code>\n        "+`<span class="badge badge-action">rejected</span>`)
	assert.Contains(t, body, `<span class="proposal-title">Sessions were denied jq.</span>`)
	assert.Contains(t, body, `<span class="proposal-subject">permissions: Allowed jq</span>`)
	// the dropped change no report names falls back to its commit date and its subject
	assert.Contains(t, body, `<summary><code>`+droppedNight+`</code> <span class="count-note">1</span></summary>`)
	assert.Contains(t, body, `<span class="badge badge-dim">dropped</span>`)
	assert.Contains(t, body, `<span class="proposal-title">sessions: Recorded nightly output</span>`)
	assert.Contains(t, body, `<a href="/proposals?tab=history" class="active"`)
	assert.NotContains(t, body, "No decisions yet.")
}

func TestHandleProposals_HistoryEmpty(t *testing.T) {
	requireProposalsPage(t)
	fixture := newRepoFixture(t)
	recorder := httptest.NewRecorder()

	fixture.server(t).Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals?tab=history", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "No decisions yet.")
	assert.NotContains(t, recorder.Body.String(), "history-entry")
}

func TestHandleProposals_ComponentsOff(t *testing.T) {
	server := newTestServer(t, &Options{})
	saveWelcomeRecord(t, server, components.ConfigServer, components.Peek, components.Agentwt)
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "This page needs Mining pipeline, agentwt")
	assert.NotContains(t, recorder.Body.String(), `id="proposals-body"`)
}

func TestHandleProposals_EmptyState(t *testing.T) {
	requireProposalsPage(t)
	type testCase struct {
		_id string

		server *Server
	}

	tests := make([]*testCase, 0)

	// no-routines-dir
	tests = append(tests, &testCase{
		_id: "no-routines-dir",

		server: newTestServer(t, &Options{}),
	})

	// no-night-branches
	emptyFixture := &changesFixture{repo: t.TempDir(), stubDir: t.TempDir()}
	emptyFixture.routinesDir = filepath.Join(emptyFixture.repo, "routines")
	emptyFixture.git(t, "init", "-q", "-b", "main")
	tests = append(tests, &testCase{
		_id: "no-night-branches",

		server: emptyFixture.server(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals", nil))
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "No open changes.")
			assert.NotContains(t, recorder.Body.String(), "card-proposal")
		})
	}
}

func TestHandleProposalPick(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedPicks  string
		_expectedStatus int
		_unexpectedBody string

		fixture *changesFixture
		form    url.Values
		server  *Server
	}

	unknownSha := strings.Repeat("e", 40)
	tests := make([]*testCase, 0)

	// two-picked-oldest-night-first
	two := newChangesFixture(t)
	two.writePick(t, 1, `{"schema":1,"commit":"`+two.shas["A1"]+`","onto":"main","outcome":"picked","new_commit":"abcdef0123456789abcdef0123456789abcdef01"}`, 0)
	two.writePick(t, 2, `{"schema":1,"commit":"`+two.shas["B1"]+`","onto":"main","outcome":"picked","new_commit":"1234567890abcdef1234567890abcdef12345678"}`, 0)
	tests = append(tests, &testCase{
		_id:             "two-picked-oldest-night-first",
		_expectedBody:   "→ <code>12345678</code>",
		_expectedPicks:  "--onto main " + two.shas["A1"] + "\n--onto main " + two.shas["B1"] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: "<h1>",

		fixture: two,
		form:    url.Values{"sha": {two.shas["B1"], two.shas["A1"]}},
		server:  two.server(t),
	})

	// second-pick-conflicts
	conflict := newChangesFixture(t)
	conflict.writePick(t, 1, `{"schema":1,"commit":"`+conflict.shas["B1"]+`","onto":"main","outcome":"picked","new_commit":"abcdef0123456789abcdef0123456789abcdef01"}`, 0)
	conflict.writePick(t, 2, `{"schema":1,"commit":"`+conflict.shas["B2"]+`","onto":"main","outcome":"conflict","reason":"skill.cue conflicts with main"}`, 3)
	tests = append(tests, &testCase{
		_id:             "second-pick-conflicts",
		_expectedBody:   "skill.cue conflicts with main",
		_expectedPicks:  "--onto main " + conflict.shas["B1"] + "\n--onto main " + conflict.shas["B2"] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: "<h1>",

		fixture: conflict,
		form:    url.Values{"sha": {conflict.shas["B2"], conflict.shas["B1"]}},
		server:  conflict.server(t),
	})

	// unknown-sha
	unknown := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "unknown-sha",
		_expectedBody:   "not an open change commit",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: unknown,
		form:    url.Values{"sha": {unknownSha}},
		server:  unknown.server(t),
	})

	// placed-sha-not-listed
	placed := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "placed-sha-not-listed",
		_expectedBody:   "not an open change commit",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: placed,
		form:    url.Values{"sha": {placed.shas["A3"]}},
		server:  placed.server(t),
	})

	// abbreviated-sha
	abbreviated := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "abbreviated-sha",
		_expectedBody:   "full commit id",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: abbreviated,
		form:    url.Values{"sha": {abbreviated.shas["B1"][:8]}},
		server:  abbreviated.server(t),
	})

	// no-sha
	none := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "no-sha",
		_expectedBody:   "No change selected",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: none,
		form:    url.Values{},
		server:  none.server(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/api/proposals/pick", test.form))
			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.NotContains(t, recorder.Body.String(), test._unexpectedBody)
			assert.Equal(t, test._expectedPicks, test.fixture.picks())
		})
	}
}

func TestHandleProposalPick_BothOutcomesRendered(t *testing.T) {
	fixture := newChangesFixture(t)
	fixture.writePick(t, 1, `{"schema":1,"commit":"`+fixture.shas["B1"]+`","onto":"main","outcome":"picked","new_commit":"abcdef0123456789abcdef0123456789abcdef01"}`, 0)
	fixture.writePick(t, 2, `{"schema":1,"commit":"`+fixture.shas["B2"]+`","onto":"main","outcome":"conflict","reason":"skill.cue conflicts with main"}`, 3)
	form := url.Values{"sha": {fixture.shas["B1"], fixture.shas["B2"]}}

	recorder := httptest.NewRecorder()
	fixture.server(t).Handler().ServeHTTP(recorder, formPost("/api/proposals/pick", form))

	body := recorder.Body.String()
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, `<span class="badge badge-ok">picked</span><span class="label"><code>`+fixture.shas["B1"][:8]+`</code> permissions: Allowed jq → <code>abcdef01</code>`)
	assert.Contains(t, body, `<span class="badge badge-action">conflict</span><span class="label"><code>`+fixture.shas["B2"][:8]+`</code> skills.y: Tightened the y and z skills — skill.cue conflicts with main`)
}

func TestHandleProposalPickOne(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedPicks  string
		_expectedStatus int
		_unexpectedBody string

		fixture *changesFixture
		path    string
		server  *Server
	}

	tests := make([]*testCase, 0)

	// picked-by-path
	picked := newChangesFixture(t)
	picked.writePick(t, 1, `{"schema":1,"commit":"`+picked.shas["B1"]+`","onto":"main","outcome":"picked","new_commit":"abcdef0123456789abcdef0123456789abcdef01"}`, 0)
	tests = append(tests, &testCase{
		_id:             "picked-by-path",
		_expectedBody:   `<span class="badge badge-ok">picked</span><span class="label"><code>` + picked.shas["B1"][:8] + `</code> permissions: Allowed jq → <code>abcdef01</code>`,
		_expectedPicks:  "--onto main " + picked.shas["B1"] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: "<h1>",

		fixture: picked,
		path:    "/api/proposals/pick/" + picked.shas["B1"],
		server:  picked.server(t),
	})

	// unknown-sha
	unknown := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "unknown-sha",
		_expectedBody:   "not an open change commit",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: unknown,
		path:    "/api/proposals/pick/" + strings.Repeat("e", 40),
		server:  unknown.server(t),
	})

	// abbreviated-sha
	abbreviated := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "abbreviated-sha",
		_expectedBody:   "full commit id",
		_expectedPicks:  "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "picked",

		fixture: abbreviated,
		path:    "/api/proposals/pick/" + abbreviated.shas["B1"][:8],
		server:  abbreviated.server(t),
	})

	// no-nightly-dir
	noNightly := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "no-nightly-dir",
		_expectedBody:   "no nightly routine configured",
		_expectedPicks:  "",
		_expectedStatus: http.StatusNotFound,
		_unexpectedBody: "picked",

		fixture: noNightly,
		path:    "/api/proposals/pick/" + noNightly.shas["B1"],
		server:  newTestServer(t, &Options{}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, test.path, nil))
			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.NotContains(t, recorder.Body.String(), test._unexpectedBody)
			assert.Equal(t, test._expectedPicks, test.fixture.picks())
		})
	}
}

func TestHandleProposalReject(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedRefs   string
		_expectedStatus int
		_unexpectedBody string

		form   url.Values
		refs   func(t *testing.T) string
		server *Server
	}

	tests := make([]*testCase, 0)

	// ref-written-card-gone
	rejected := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "ref-written-card-gone",
		_expectedBody:   `id="change-` + rejected.shas["B2"][:8] + `"`,
		_expectedRefs:   repos.RejectedRefPrefix + rejected.shas["B1"] + " " + rejected.shas["B1"] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: `id="change-` + rejected.shas["B1"][:8] + `"`,

		form:   url.Values{"branch": {nightNewer}, "sha": {rejected.shas["B1"]}},
		refs:   rejected.rejectedRefs,
		server: rejected.server(t),
	})

	// two-selected-both-rejected
	both := newChangesFixture(t)
	bothShas := []string{both.shas["A1"], both.shas["B1"]}
	slices.Sort(bothShas)
	tests = append(tests, &testCase{
		_id:             "two-selected-both-rejected",
		_expectedBody:   `id="change-` + both.shas["B2"][:8] + `"`,
		_expectedRefs:   repos.RejectedRefPrefix + bothShas[0] + " " + bothShas[0] + "\n" + repos.RejectedRefPrefix + bothShas[1] + " " + bothShas[1] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: `id="change-` + both.shas["A1"][:8] + `"`,

		form:   url.Values{"sha": {both.shas["B1"], both.shas["A1"]}},
		refs:   both.rejectedRefs,
		server: both.server(t),
	})

	// unknown-sha
	unknown := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "unknown-sha",
		_expectedBody:   "not an open change commit",
		_expectedRefs:   "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "card-proposal",

		form:   url.Values{"branch": {nightNewer}, "sha": {strings.Repeat("e", 40)}},
		refs:   unknown.rejectedRefs,
		server: unknown.server(t),
	})

	// no-nightly-dir
	noNightly := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "no-nightly-dir",
		_expectedBody:   "no nightly routine configured",
		_expectedRefs:   "",
		_expectedStatus: http.StatusNotFound,
		_unexpectedBody: "card-proposal",

		form:   url.Values{"branch": {nightNewer}, "sha": {noNightly.shas["B1"]}},
		refs:   noNightly.rejectedRefs,
		server: newTestServer(t, &Options{}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/api/proposals/reject", test.form))
			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.NotContains(t, recorder.Body.String(), test._unexpectedBody)
			assert.Equal(t, test._expectedRefs, test.refs(t))
		})
	}
}

func TestHandleProposalReject_Reason(t *testing.T) {
	requireProposalsPage(t)
	fixture := newChangesFixture(t)
	server := fixture.server(t)
	request := formPost("/api/proposals/reject", url.Values{"sha": {fixture.shas["B1"]}})
	request.Header.Set("HX-Prompt", " jq stays prompted ")

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	store, err := backlog.Load(filepath.Join(fixture.backlogDir, "backlog.json"))
	require.NoError(t, err)
	require.Len(t, store.Archive.Rejected, 1)
	assert.Equal(t, fixture.shas["B1"], store.Archive.Rejected[0].Commit)
	assert.Equal(t, "jq stays prompted", store.Archive.Rejected[0].Reason)
	assert.Equal(t, "Sessions were denied jq.", store.Archive.Rejected[0].Title)

	recorder = httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/proposals?tab=history", nil))
	assert.Contains(t, recorder.Body.String(), `<span class="badge badge-action" title="jq stays prompted">rejected</span>`)
}

func TestHandleProposalRejectOne(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedRefs   string
		_expectedStatus int
		_unexpectedBody string

		path   string
		refs   func(t *testing.T) string
		server *Server
	}

	tests := make([]*testCase, 0)

	// conflict-card-rejected
	conflicting := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "conflict-card-rejected",
		_expectedBody:   `id="change-` + conflicting.shas["B1"][:8] + `"`,
		_expectedRefs:   repos.RejectedRefPrefix + conflicting.shas["A1"] + " " + conflicting.shas["A1"] + "\n",
		_expectedStatus: http.StatusOK,
		_unexpectedBody: `id="change-` + conflicting.shas["A1"][:8] + `"`,

		path:   "/api/proposals/reject/" + conflicting.shas["A1"],
		refs:   conflicting.rejectedRefs,
		server: conflicting.server(t),
	})

	// abbreviated-sha-refused
	abbreviated := newChangesFixture(t)
	tests = append(tests, &testCase{
		_id:             "abbreviated-sha-refused",
		_expectedBody:   "sha must be a full commit id",
		_expectedRefs:   "",
		_expectedStatus: http.StatusBadRequest,
		_unexpectedBody: "card-proposal",

		path:   "/api/proposals/reject/" + abbreviated.shas["B1"][:8],
		refs:   abbreviated.rejectedRefs,
		server: abbreviated.server(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, test.path, nil))
			assert.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.NotContains(t, recorder.Body.String(), test._unexpectedBody)
			assert.Equal(t, test._expectedRefs, test.refs(t))
		})
	}
}

func TestChangeVerdict(t *testing.T) {
	type testCase struct {
		_id       string
		_expected changeVerdictView

		branch  string
		reports []routines.RunReport
		sha     string
	}

	sha := "0123456789abcdef0123456789abcdef01234567"
	readyShort := routines.RunReport{Branch: nightNewer, Changes: []routines.RunReportChange{{Sha: sha[:8], Verdict: verdictReady}}}
	rejectedFull := routines.RunReport{Branch: nightNewer, Changes: []routines.RunReportChange{{Reason: "breaks a gate", Sha: sha, Verdict: verdictRejected}}}
	otherBranch := routines.RunReport{Branch: nightOlder, Changes: []routines.RunReportChange{{Sha: sha, Verdict: verdictReady}}}
	emptySha := routines.RunReport{Branch: nightNewer, Changes: []routines.RunReportChange{{Sha: "", Verdict: verdictReady}}}
	tests := make([]*testCase, 0)

	// ready-by-short-sha
	tests = append(tests, &testCase{
		_id:       "ready-by-short-sha",
		_expected: changeVerdictView{Class: badgeOk, Label: verdictReady},

		branch:  nightNewer,
		reports: []routines.RunReport{readyShort},
		sha:     sha,
	})

	// rejected-with-reason
	tests = append(tests, &testCase{
		_id:       "rejected-with-reason",
		_expected: changeVerdictView{Class: badgeAction, Label: verdictRejected, Reason: "breaks a gate"},

		branch:  nightNewer,
		reports: []routines.RunReport{rejectedFull},
		sha:     sha,
	})

	// report-full-sha-card-short
	tests = append(tests, &testCase{
		_id:       "report-full-sha-card-short",
		_expected: changeVerdictView{Class: badgeAction, Label: verdictRejected, Reason: "breaks a gate"},

		branch:  nightNewer,
		reports: []routines.RunReport{rejectedFull},
		sha:     sha[:8],
	})

	// newest-report-wins
	tests = append(tests, &testCase{
		_id:       "newest-report-wins",
		_expected: changeVerdictView{Class: badgeOk, Label: verdictReady},

		branch:  nightNewer,
		reports: []routines.RunReport{readyShort, rejectedFull},
		sha:     sha,
	})

	// missing-report
	tests = append(tests, &testCase{
		_id:       "missing-report",
		_expected: changeVerdictView{Class: badgeDim, Label: verdictUnjudged},

		branch:  nightNewer,
		reports: []routines.RunReport{},
		sha:     sha,
	})

	// report-of-other-branch
	tests = append(tests, &testCase{
		_id:       "report-of-other-branch",
		_expected: changeVerdictView{Class: badgeDim, Label: verdictUnjudged},

		branch:  nightNewer,
		reports: []routines.RunReport{otherBranch},
		sha:     sha,
	})

	// empty-recorded-sha-never-matches
	tests = append(tests, &testCase{
		_id:       "empty-recorded-sha-never-matches",
		_expected: changeVerdictView{Class: badgeDim, Label: verdictUnjudged},

		branch:  nightNewer,
		reports: []routines.RunReport{emptySha},
		sha:     sha,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, changeVerdict(test.branch, test.sha, test.reports))
		})
	}
}

func TestServer_OpenProposals(t *testing.T) {
	type testCase struct {
		_id       string
		_expected openProposalView

		server *Server
	}

	tests := make([]*testCase, 0)

	// count-equals-open-changes
	tests = append(tests, &testCase{
		_id:       "count-equals-open-changes",
		_expected: openProposalView{Count: 3},

		server: newChangesFixture(t).server(t),
	})

	// no-routines-dir
	tests = append(tests, &testCase{
		_id:       "no-routines-dir",
		_expected: openProposalView{Count: 0},

		server: newTestServer(t, &Options{}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.server.openProposals())
		})
	}
}

func TestSessionAge(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		links []sessionLink
	}

	now := time.Date(2000, 1, 10, 12, 0, 0, 0, time.UTC)
	tests := make([]*testCase, 0)

	// newest-session-wins
	tests = append(tests, &testCase{
		_id:       "newest-session-wins",
		_expected: "~2d ago",

		links: []sessionLink{{Id: "old", LastActive: "2000-01-03T12:00:00Z"}, {Id: "new", LastActive: "2000-01-08T12:00:00Z"}},
	})

	// unparseable-skipped
	tests = append(tests, &testCase{
		_id:       "unparseable-skipped",
		_expected: "~0d ago",

		links: []sessionLink{{Id: "bad", LastActive: "yesterday"}, {Id: "today", LastActive: "2000-01-10T08:00:00Z"}},
	})

	// no-record-empty
	tests = append(tests, &testCase{
		_id:       "no-record-empty",
		_expected: "",

		links: []sessionLink{{Id: "unknown"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionAge(test.links, now))
		})
	}
}
