package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/backlog"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
)

const (
	badgeAction = "badge-action"
	badgeDim    = "badge-dim"
	badgeOk     = "badge-ok"

	pageProposals     = "proposals"
	tmplProposals     = "proposals.html"
	tmplProposalsBody = "_proposals_body.html"

	// changeTarget is the branch a change is picked onto.
	changeTarget = "main"
	hoursPerDay  = 24
	// maxFileDiffLines caps the patch a card shows per file.
	maxFileDiffLines = 200
	// proposalCountTimeout bounds the nav count's git and agentwt calls.
	proposalCountTimeout = 5 * time.Second
	// historyPickLimit bounds the main log scan for picked changes.
	historyPickLimit = 200

	outcomePicked   = "picked"
	tabHistory      = "history"
	unknownNight    = "unknown night"
	verdictConflict = "conflict"
	verdictReady    = "ready"
	verdictRejected = "rejected"
	verdictUnjudged = "unjudged"
)

// commitShaPattern admits only full ids: the pick form posts the sha the card rendered, never an abbreviation.
var commitShaPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

type changeCardView struct {
	Branch       string
	Commit       repos.ChangeCommit
	Files        []changeFileView
	IsApplicable bool // applies on main right now
	IsDisabled   bool // the Welcome demo and the conflicts block: no pick
	Night        string
	Problem      string // the first task's problem sentence, else the subject
	ProbeReason  string // agentwt's reason when the dry-run pick failed
	SessionAge   string
	Sessions     []sessionLink
	ShortSha     string
	Tasks        []changeTaskView // one per change id with a record, in id order
	Verdict      changeVerdictView
}

type changeFileView struct {
	Added     int
	Deleted   int
	DiffLines []diffLine
	IsDiffCut bool
	Path      string
	Status    string
}

// changeNightView is one nightly branch's open change commits, newest first.
type changeNightView struct {
	Branch    string
	Cards     []changeCardView
	Conflicts []changeCardView // dry-run pick failed; rendered closed after the cards
	Night     string
}

type changeTaskView struct {
	Backlog  string
	Evidence []evidenceView
}

type changeVerdictView struct {
	Class  string
	Label  string
	Reason string
}

type evidenceView struct {
	Quote   string
	Session sessionLink
	URL     string // the session URL plus the finding fragment; empty without a record
}

type historyDayView struct {
	Entries []historyEntryView
	Night   string
}

type historyEntryView struct {
	Outcome      string
	OutcomeClass string
	Problem      string
	Reason       string
	ShortSha     string
	Subject      string
}

type openProposalView struct {
	Count int
}

type pickOutcomeView struct {
	NewCommit string
	Outcome   string
	Reason    string
	ShortSha  string
	Subject   string
}

type proposalsPage struct {
	History       []historyDayView
	KindCounts    []backlogFilterCount
	LoadErrors    []string
	NightCounts   []backlogFilterCount
	Nights        []changeNightView
	OpenCount     int
	Page          string
	Results       []pickOutcomeView
	Tab           string
	Title         string
	Total         int
	VerdictCounts []backlogFilterCount
}

type sessionLink struct {
	Id         string
	LastActive string // RFC3339 from the record, empty when unloaded
	URL        string // empty when the id is not in any loaded session record
}

func (s *Server) handleProposals(w http.ResponseWriter, r *http.Request) {
	if !s.hasProposals() {
		s.renderComponentOff(w, pageProposals, "Proposals", components.Mining, components.Agentwt)
		return
	}

	s.reloadSessions()
	repoPath, isNightlyKnown := s.nightlyRepo()
	if !isNightlyKnown {
		data := proposalsPage{Page: pageProposals, Title: translate(s.presentation.language(), "Proposals")}
		s.renderFragment(w, tmplProposals, data)
		return
	}
	if r.URL.Query().Get("tab") == tabHistory {
		s.renderFragment(w, tmplProposals, s.historyPage(r.Context(), repoPath))
		return
	}
	s.renderFragment(w, tmplProposals, s.proposalsPage(r.Context(), nil, repoPath))
}

// handleProposalPick picks the selected open changes in branch order and re-renders the body with each outcome.
func (s *Server) handleProposalPick(w http.ResponseWriter, r *http.Request) {
	repoPath, isNightlyKnown := s.nightlyRepo()
	if !isNightlyKnown {
		respond.WithNotFound("no nightly routine configured", w)
		return
	}
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	snapshot, err := s.proposals.Snapshot(r.Context(), s.agentwt, repoPath, changeTarget)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	selected, err := selectedChanges(snapshot.Open, r.Form["sha"])
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	results, err := s.pickSelected(r.Context(), repoPath, selected)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplProposalsBody, s.proposalsPage(r.Context(), results, repoPath))
}

// handleProposalPickOne picks the one change the path names: the card's own Pick button.
func (s *Server) handleProposalPickOne(w http.ResponseWriter, r *http.Request) {
	repoPath, isNightlyKnown := s.nightlyRepo()
	if !isNightlyKnown {
		respond.WithNotFound("no nightly routine configured", w)
		return
	}

	snapshot, err := s.proposals.Snapshot(r.Context(), s.agentwt, repoPath, changeTarget)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	selected, err := selectedChanges(snapshot.Open, []string{r.PathValue("sha")})
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	results, err := s.pickSelected(r.Context(), repoPath, selected)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplProposalsBody, s.proposalsPage(r.Context(), results, repoPath))
}

// handleProposalReject writes the rejected ref of every selected change; they leave the page and the count.
func (s *Server) handleProposalReject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	s.rejectChanges(w, r, r.Form["sha"])
}

// handleProposalRejectOne rejects the one change the path names: the card's own Reject button.
func (s *Server) handleProposalRejectOne(w http.ResponseWriter, r *http.Request) {
	s.rejectChanges(w, r, []string{r.PathValue("sha")})
}

// rejectChanges writes one rejected ref per open change the shas name, with the prompt's reason as a backlog row, and re-renders the body.
func (s *Server) rejectChanges(w http.ResponseWriter, r *http.Request, shas []string) {
	repoPath, isNightlyKnown := s.nightlyRepo()
	if !isNightlyKnown {
		respond.WithNotFound("no nightly routine configured", w)
		return
	}

	snapshot, err := s.proposals.Snapshot(r.Context(), s.agentwt, repoPath, changeTarget)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	selected, err := selectedChanges(snapshot.Open, shas)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	reason := strings.TrimSpace(r.Header.Get("HX-Prompt"))
	reports := s.nightlyReports()
	for _, change := range selected {
		if reason != "" {
			row := &backlog.ArchiveRow{
				Commit:      change.Commit.Sha,
				Description: strings.TrimSpace(change.Message.Subject + "\n\n" + change.Message.Body),
				Link:        change.Branch,
				Reason:      reason,
				Title:       cardProblem(change.Message.ChangeIds, reports, change.Message.Subject),
			}
			if err := backlog.AppendRejected(s.backlogStorePath(), row); err != nil {
				respond.WithInternalServerError(err, w)
				return
			}
		}
		if err := repos.Reject(r.Context(), repoPath, change.Commit.Sha); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
	}
	s.renderFragment(w, tmplProposalsBody, s.proposalsPage(r.Context(), nil, repoPath))
}

// historyPage carries the open count from the store and the settled changes; it builds no card.
func (s *Server) historyPage(ctx context.Context, repoPath string) proposalsPage {
	data := s.newProposalsPage(nil)
	data.Tab = tabHistory
	snapshot, err := s.proposals.Snapshot(ctx, s.agentwt, repoPath, changeTarget)
	if err != nil {
		data.LoadErrors = append(data.LoadErrors, err.Error())
	}
	data.OpenCount = snapshot.Count()
	data.History = s.proposalHistory(ctx, repoPath)
	return data
}

func (s *Server) newProposalsPage(results []pickOutcomeView) proposalsPage {
	data := proposalsPage{
		Page:    pageProposals,
		Results: results,
		Title:   translate(s.presentation.language(), "Proposals"),
	}
	return data
}

// nightlyReports degrades an unreadable runs dir to no verdicts; cards render unjudged.
func (s *Server) nightlyReports() []routines.RunReport {
	reports, err := routines.Reports(filepath.Join(s.routinesDir, pinnedRoutine), routines.HistoryLimit())
	if err != nil {
		return nil
	}
	return reports
}

// nightlyRepo derives the checkout from the routines dir, as the routine runner does.
func (s *Server) nightlyRepo() (string, bool) {
	if s.routinesDir == "" {
		return "", false
	}

	routinesDir, err := filepath.Abs(s.routinesDir)
	if err != nil {
		return "", false
	}
	return filepath.Dir(routinesDir), true
}

// openProposals serves the nav closure and the Overview tile: a bounded context, errors read as zero.
func (s *Server) openProposals() openProposalView {
	repoPath, isNightlyKnown := s.nightlyRepo()
	if !isNightlyKnown {
		return openProposalView{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), proposalCountTimeout)
	defer cancel()

	snapshot, err := s.proposals.Snapshot(ctx, s.agentwt, repoPath, changeTarget)
	if err != nil {
		log.Printf("Server.openProposals: %v", err)
	}
	return openProposalView{Count: snapshot.Count()}
}

// pickSelected picks the resolved changes in their given order and collects one outcome each.
func (s *Server) pickSelected(ctx context.Context, repoPath string, selected []*repos.OpenChange) ([]pickOutcomeView, error) {
	results := make([]pickOutcomeView, 0, len(selected))
	for _, change := range selected {
		pick, err := repos.Pick(ctx, s.agentwt, changeTarget, repoPath, change.Commit.Sha)
		if err != nil {
			return nil, fmt.Errorf("Server.pickSelected: %w", err)
		}
		results = append(results, pickOutcome(change, pick))
	}
	return results, nil
}

// proposalHistory lists every picked, rejected and dropped change per night, newest night first.
func (s *Server) proposalHistory(ctx context.Context, repoPath string) []historyDayView {
	reports := s.nightlyReports()
	entries := map[string][]historyEntryView{}
	order := make([]string, 0)
	add := func(night string, entry historyEntryView) {
		if _, seen := entries[night]; !seen {
			order = append(order, night)
		}
		entries[night] = append(entries[night], entry)
	}

	picks, err := repos.PickedChanges(ctx, historyPickLimit, repoPath, changeTarget)
	if err != nil {
		log.Printf("Server.proposalHistory: %v", err)
	}
	for index := range picks {
		pick := &picks[index]
		add(historyNight(pick.SourceSha, pick.Date, reports), pickedEntry(pick, reports))
	}

	settled, err := repos.SettledChanges(ctx, repoPath)
	if err != nil {
		log.Printf("Server.proposalHistory: %v", err)
	}
	store, err := s.loadBacklog()
	if err != nil {
		log.Printf("Server.proposalHistory: %v", err)
		store = &backlog.Store{}
	}
	reasons := map[string]string{}
	for _, row := range store.Archive.Rejected {
		reasons[row.Commit] = row.Reason
	}
	for index := range settled {
		change := &settled[index]
		add(historyNight(change.Sha, change.Date, reports), settledEntry(change, reasons, reports))
	}

	slices.Sort(order)
	slices.Reverse(order)
	days := make([]historyDayView, 0, len(order))
	for _, night := range order {
		days = append(days, historyDayView{Entries: entries[night], Night: night})
	}
	return days
}

// proposalsPage lists the open changes with their applicability from the store; results carries a pick's outcomes.
func (s *Server) proposalsPage(ctx context.Context, results []pickOutcomeView, repoPath string) proposalsPage {
	data := s.newProposalsPage(results)
	snapshot, err := s.proposals.Snapshot(ctx, s.agentwt, repoPath, changeTarget)
	if err != nil {
		data.LoadErrors = append(data.LoadErrors, err.Error())
	}

	reports := s.nightlyReports()
	refs := s.sessions.SessionRefs()
	nights := map[string]*changeNightView{}
	order := make([]string, 0)
	for index := range snapshot.Open {
		change := &snapshot.Open[index]
		probe := snapshot.Probes[change.Commit.Sha]
		card := changeCard(change, snapshot.Commits[change.Commit.Sha], &probe, refs, reports)

		night, ok := nights[card.Branch]
		if !ok {
			night = &changeNightView{Branch: card.Branch, Night: card.Night}
			nights[card.Branch] = night
			order = append(order, card.Branch)
		}
		if card.IsApplicable {
			night.Cards = append([]changeCardView{*card}, night.Cards...)
		} else {
			night.Conflicts = append([]changeCardView{*card}, night.Conflicts...)
		}
	}
	for _, branch := range order {
		data.Nights = append(data.Nights, *nights[branch])
	}
	data.OpenCount, data.Total = snapshot.Count(), snapshot.Count()
	cardCounts(&data)
	return data
}

// cardCounts fills the three filter rows from the rendered cards, conflicts included.
func cardCounts(data *proposalsPage) {
	verdicts, kinds, nightNames := map[string]int{}, map[string]int{}, map[string]int{}
	nightOrder := make([]string, 0, len(data.Nights))
	kindOrder := make([]string, 0)
	for _, night := range data.Nights {
		nightOrder = append(nightOrder, night.Night)
		for _, card := range slices.Concat(night.Cards, night.Conflicts) {
			verdicts[card.Verdict.Label]++
			nightNames[night.Night]++
			if _, seen := kinds[card.Commit.Kind]; !seen {
				kindOrder = append(kindOrder, card.Commit.Kind)
			}
			kinds[card.Commit.Kind]++
		}
	}
	slices.Sort(kindOrder)
	data.VerdictCounts = filterCounts([]string{verdictReady, verdictRejected, verdictUnjudged}, verdicts)
	data.KindCounts = filterCounts(kindOrder, kinds)
	data.NightCounts = filterCounts(nightOrder, nightNames)
}

// cardFiles classifies each file's capped patch and marks its cut.
func cardFiles(commit *repos.ChangeCommit) []changeFileView {
	files := make([]changeFileView, 0, len(commit.Files))
	for _, file := range commit.Files {
		lines := diffLinesFromPatch(file.Diff)
		if file.IsDiffCut {
			lines = append(lines, diffLine{Kind: diffSkip, Text: "⋯ diff truncated"})
		}
		view := changeFileView{Added: file.Added, Deleted: file.Deleted, DiffLines: lines, IsDiffCut: file.IsDiffCut, Path: file.Path, Status: file.Status}
		files = append(files, view)
	}
	return files
}

// cardProblem is the first recorded task's problem sentence; a change without a record shows its subject.
func cardProblem(ids []string, reports []routines.RunReport, subject string) string {
	for _, id := range ids {
		if task := routines.FindTask(reports, id); task != nil {
			return task.Problem
		}
	}
	return subject
}

// changeCard renders one open change from the store's payload and probe outcome; no git runs here.
func changeCard(change *repos.OpenChange, commit *repos.ChangeCommit, probe *repos.ProbeEntry, refs map[string]sessions.SessionRef, reports []routines.RunReport) *changeCardView {
	links := sessionLinks(change.Message.Sessions, refs)
	card := &changeCardView{
		Branch:       change.Branch,
		Commit:       *commit,
		Files:        cardFiles(commit),
		IsApplicable: probe.IsApplicable(),
		IsDisabled:   !probe.IsApplicable(),
		Night:        nightLabel(change.Branch),
		Problem:      cardProblem(commit.ChangeIds, reports, commit.Subject),
		SessionAge:   sessionAge(links, time.Now()),
		Sessions:     links,
		ShortSha:     change.Commit.ShortSha,
		Tasks:        cardTasks(commit.ChangeIds, reports, refs),
		Verdict:      changeVerdict(change.Branch, change.Commit.Sha, reports),
	}
	if !card.IsApplicable {
		card.ProbeReason = probe.Reason
		card.Verdict = changeVerdictView{Class: badgeAction, Label: verdictConflict, Reason: probe.Reason}
	}
	return card
}

// cardTasks resolves one record per change id over the reports, newest first; an id without a record is skipped.
func cardTasks(ids []string, reports []routines.RunReport, refs map[string]sessions.SessionRef) []changeTaskView {
	tasks := make([]changeTaskView, 0, len(ids))
	for _, id := range ids {
		task := routines.FindTask(reports, id)
		if task == nil {
			continue
		}
		tasks = append(tasks, taskView(task, refs))
	}
	return tasks
}

// changeVerdict matches shas by prefix either way: a report may record a short sha.
func changeVerdict(branch, sha string, reports []routines.RunReport) changeVerdictView {
	unjudged := changeVerdictView{Class: badgeDim, Label: verdictUnjudged}
	for index := range reports {
		if reports[index].Branch != branch {
			continue
		}

		change := findReportChange(reports[index].Changes, sha)
		if change == nil {
			return unjudged
		}
		return verdictView(change)
	}
	return unjudged
}

// evidenceURL points at the finding the quote came from; without a record the session page alone.
func evidenceURL(item *routines.ChangeEvidence, link sessionLink) string {
	if link.URL == "" {
		return ""
	}
	return fmt.Sprintf("%s#f-%s-%d", link.URL, item.Session, item.Finding)
}

func findReportChange(changes []routines.RunReportChange, sha string) *routines.RunReportChange {
	for index := range changes {
		if shaMatches(changes[index].Sha, sha) {
			return &changes[index]
		}
	}
	return nil
}

// historyNight resolves the night of a settled sha: the newest report holding it, else the commit date it came with.
func historyNight(sha, date string, reports []routines.RunReport) string {
	for index := range reports {
		if findReportChange(reports[index].Changes, sha) != nil {
			return nightLabel(reports[index].Branch)
		}
	}
	if date == "" {
		return unknownNight
	}
	return date
}

// nightLabel is the date stem: "2026-10-05" or "2026-10-05-2".
func nightLabel(branch string) string {
	return strings.TrimSuffix(strings.TrimPrefix(branch, repos.NightBranchPrefix), repos.ProposalsSuffix)
}

func outcomeClass(outcome string) string {
	switch outcome {
	case outcomePicked:
		return badgeOk
	case repos.SettledOutcomeRejected:
		return badgeAction
	case repos.SettledOutcomeDropped:
		return badgeDim
	}
	return badgeDim
}

// pickedEntry shows a placed change under the proposal's sha, so it matches the card it came from.
func pickedEntry(pick *repos.PickedChange, reports []routines.RunReport) historyEntryView {
	return historyEntryView{
		Outcome:      outcomePicked,
		OutcomeClass: outcomeClass(outcomePicked),
		Problem:      cardProblem(pick.Message.ChangeIds, reports, pick.Message.Subject),
		ShortSha:     shortId(pick.SourceSha),
		Subject:      pick.Message.Subject,
	}
}

func pickOutcome(change *repos.OpenChange, pick *repos.PickResult) pickOutcomeView {
	return pickOutcomeView{
		NewCommit: pick.NewCommit,
		Outcome:   pick.Outcome,
		Reason:    pick.Reason,
		ShortSha:  change.Commit.ShortSha,
		Subject:   change.Message.Subject,
	}
}

// selectedChanges resolves posted shas against the open list, oldest night first, parent before child.
func selectedChanges(open []repos.OpenChange, shas []string) ([]*repos.OpenChange, error) {
	selected := make([]*repos.OpenChange, 0, len(shas))
	for _, sha := range shas {
		if !commitShaPattern.MatchString(sha) {
			return nil, errors.New("selectedChanges: sha must be a full commit id")
		}

		index := slices.IndexFunc(open, func(change repos.OpenChange) bool { return change.Commit.Sha == sha })
		if index < 0 {
			return nil, fmt.Errorf("selectedChanges: %s is not an open change commit", sha)
		}
		selected = append(selected, &open[index])
	}
	if len(selected) == 0 {
		return nil, errors.New("selectedChanges: No change selected")
	}

	position := func(change *repos.OpenChange) int {
		return slices.IndexFunc(open, func(candidate repos.OpenChange) bool { return candidate.Commit.Sha == change.Commit.Sha })
	}
	slices.SortFunc(selected, func(a, b *repos.OpenChange) int {
		if byBranch := strings.Compare(a.Branch, b.Branch); byBranch != 0 {
			return byBranch
		}
		return position(a) - position(b)
	})
	return selected, nil
}

// settledEntry shows a rejected or dropped change under its own sha with the problem sentence and the subject.
func settledEntry(change *repos.SettledChange, reasons map[string]string, reports []routines.RunReport) historyEntryView {
	return historyEntryView{
		Outcome:      change.Outcome,
		OutcomeClass: outcomeClass(change.Outcome),
		Problem:      cardProblem(change.Message.ChangeIds, reports, change.Message.Subject),
		Reason:       reasons[change.Sha],
		ShortSha:     shortId(change.Sha),
		Subject:      change.Message.Subject,
	}
}

// sessionAge is the newest evidence session's last activity in whole days; empty when no record is loaded.
func sessionAge(links []sessionLink, now time.Time) string {
	youngest := -1
	for _, link := range links {
		stamp, err := time.Parse(time.RFC3339, link.LastActive)
		if err != nil {
			continue
		}

		days := int(now.Sub(stamp).Hours() / hoursPerDay)
		if youngest < 0 || days < youngest {
			youngest = days
		}
	}
	if youngest < 0 {
		return ""
	}
	return fmt.Sprintf("~%dd ago", youngest)
}

func sessionLinks(ids []string, refs map[string]sessions.SessionRef) []sessionLink {
	links := make([]sessionLink, 0, len(ids))
	for _, id := range ids {
		link := sessionLink{Id: id}
		if ref, ok := refs[id]; ok {
			query := url.Values{"session": {id}}
			link.LastActive = ref.LastActive
			link.URL = fmt.Sprintf("/sessions/%s/%s?%s", url.PathEscape(ref.Scope), url.PathEscape(ref.Week), query.Encode())
		}
		links = append(links, link)
	}
	return links
}

// shaMatches compares a recorded sha with a full one either way: a report may record a short sha.
func shaMatches(recorded, sha string) bool {
	if recorded == "" {
		return false
	}
	return strings.HasPrefix(sha, recorded) || strings.HasPrefix(recorded, sha)
}

func taskView(task *routines.ChangeTask, refs map[string]sessions.SessionRef) changeTaskView {
	evidence := make([]evidenceView, 0, len(task.Evidence))
	for _, item := range task.Evidence {
		link := sessionLinks([]string{item.Session}, refs)[0]
		view := evidenceView{Quote: item.Quote, Session: link, URL: evidenceURL(&item, link)}
		evidence = append(evidence, view)
	}
	return changeTaskView{Backlog: task.Backlog, Evidence: evidence}
}

// verdictView renders a verdict outside the ready/rejected contract verbatim and dim.
func verdictView(change *routines.RunReportChange) changeVerdictView {
	switch change.Verdict {
	case verdictReady:
		return changeVerdictView{Class: badgeOk, Label: verdictReady}
	case verdictRejected:
		return changeVerdictView{Class: badgeAction, Label: verdictRejected, Reason: change.Reason}
	}
	return changeVerdictView{Class: badgeDim, Label: change.Verdict, Reason: change.Reason}
}
