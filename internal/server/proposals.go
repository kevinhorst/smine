package server

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/proposals"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
)

const (
	pageProposals    = "proposals"
	tmplProposals    = "proposals.html"
	tmplProposalCard = "_proposal_card.html"

	maxVoteCommentLength = 500
	voteAccept           = "+"
	votePostpone         = "p"
	voteReject           = "-"

	stateFilterPrefix = "state:"
)

// kindDimension is the default deep-link dimension per proposals kind;
// evidence-level dimension overrides it.
var kindDimension = map[string]string{
	"context":  "rule",
	"routines": "routine-candidate",
	"skills":   "skill-candidate",
}

type proposalsPage struct {
	Files      []fileView
	HappyWord  string
	HasGroups  bool
	LoadErrors []string
	Page       string
	Tab        string
	Title      string
}

// happyWords feeds the Open tab's empty state — things that happen
// inside the LLM while there is nothing left to vote on.
var happyWords = []string{
	"Combobulating", "Confabulating", "Percolating", "Ruminating",
	"Cogitating", "Tokenizing", "Attending", "Embedding",
	"Extrapolating", "Recalibrating",
}

type fileView struct {
	AllURL     string
	Categories []categoryView
	File       proposals.File
	Filter     string
	FilterRows []proposalFilterRow
	Groups     []groupView
}

// proposalFilterRow is one labeled chip row of a kind's filter card — the
// skills kind splits into a skills row and a workflows row (plan D10).
type proposalFilterRow struct {
	Filters []proposalFilter
	Label   string
}

// categoryView is one derived top-level card. A Solo category wraps exactly
// one group and renders it bare — kinds without a taxonomy stay flat.
type categoryView struct {
	CountLabel     string
	DataSection    string
	Groups         []groupView
	IsFilterTarget bool
	Solo           bool
	Title          string
}

// subgroupView is one derived per-target bar inside an authored group.
type subgroupView struct {
	CountLabel     string
	DataSection    string
	IsFilterTarget bool
	Proposals      []proposalView
	Title          string
}

// proposalFilter is one badge in a kind's filter row (D3/D4). Label is the
// chip text (the value minus a wf: namespace); Value stays the URL token.
type proposalFilter struct {
	Active bool
	Label  string
	URL    string
	Value  string
}

type groupView struct {
	Accepted       stateNote
	Count          int
	CountLabel     string
	DataSection    string
	IsFilterTarget bool
	Postponed      stateNote
	Proposals      []proposalView
	Rejected       stateNote
	Solo           bool
	Subgroups      []subgroupView
	Title          string
	Total          int
}

// stateNote is one element of a group's compact summary notation
// "(accepted/total)[rejected][postponed]" — itself the clickable state
// filter (the global filter row carries only id/RULE badges). An active
// note links to the cleared filter; the tooltip decodes the notation.
type stateNote struct {
	Active  bool
	Count   int
	Tooltip string
	URL     string
}

type proposalView struct {
	Evidence       []evidenceView
	IsRevert       bool
	Kind           string
	PendingComment string
	PendingDate    string // date part of the pending vote's Ts
	PendingVote    string
	Proposal       proposals.Proposal
}

type evidenceView struct {
	Item  proposals.Evidence
	Links []sessionLink
}

type sessionLink struct {
	Id   string
	Note string
	URL  string // empty when the id is not in any loaded batch
}

func (s *Server) handleProposals(w http.ResponseWriter, r *http.Request) {
	files, loadErrors, err := proposals.Load(s.proposalsDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	tab := r.URL.Query().Get("tab")
	isKnownTab := tab == "notes"
	if !isKnownTab {
		tab = "open"
	}

	votes, votesErr := proposals.LoadVotes(s.votesPath())
	if votesErr != nil {
		loadErrors = append(loadErrors, votesErr.Error())
	}

	// One filter per kind, param name = kind: ?skills=<id>&rules=<prefix>…
	active := make(map[string]string, len(files))
	for _, file := range files {
		if value := r.URL.Query().Get(file.Kind); value != "" {
			active[file.Kind] = value
		}
	}

	data := proposalsPage{
		Files:      proposalViews(files, active, s.sessions.SessionRefs(), votes, tab),
		HappyWord:  happyWords[rand.IntN(len(happyWords))],
		LoadErrors: loadErrors,
		Page:       pageProposals,
		Tab:        tab,
		Title:      "Proposals",
	}
	for _, file := range data.Files {
		if len(file.Groups) > 0 {
			data.HasGroups = true
		}
	}
	s.renderFragment(w, tmplProposals, data)
}

func proposalViews(files []proposals.File, active map[string]string, refs map[string]sessions.SessionRef, votes map[string]proposals.Vote, tab string) []fileView {
	views := make([]fileView, 0, len(files))
	for _, file := range files {
		view := fileView{
			AllURL: filterURL(active, file.Kind, ""),
			File:   file,
			Filter: active[file.Kind],
			Groups: groupViews(file, active, refs, votes, tab),
		}
		view.Categories = categoryViews(file.Kind, view.Groups)
		view.FilterRows = filterRows(&file, active, view.Filter)
		views = append(views, view)
	}
	return views
}

// filterRows renders a kind's chip rows: the skills kind splits skill chips
// from wf:-namespaced workflow chips into two labeled rows, every other kind
// keeps one unlabeled row (plan D10).
func filterRows(file *proposals.File, active map[string]string, activeValue string) []proposalFilterRow {
	var skills, workflows []proposalFilter
	for _, value := range filterValues(file) {
		filter := proposalFilter{
			Active: value == activeValue,
			Label:  strings.TrimPrefix(value, workflowFilterPrefix),
			URL:    filterURL(active, file.Kind, value),
			Value:  value,
		}
		if strings.HasPrefix(value, workflowFilterPrefix) {
			workflows = append(workflows, filter)
			continue
		}
		skills = append(skills, filter)
	}
	if len(workflows) == 0 {
		if len(skills) == 0 {
			return nil
		}
		row := proposalFilterRow{Filters: skills}
		return []proposalFilterRow{row}
	}
	rows := []proposalFilterRow{
		{Filters: skills, Label: "skills"},
		{Filters: workflows, Label: "workflows"},
	}
	return rows
}

// filterURL builds /proposals with this kind's filter set to value ("" to
// clear), preserving every other kind's active filter.
func filterURL(active map[string]string, kind, value string) string {
	query := url.Values{}
	for activeKind, activeValue := range active {
		if activeKind != kind {
			query.Set(activeKind, activeValue)
		}
	}
	if value != "" {
		query.Set(kind, value)
	}
	if len(query) == 0 {
		return "/proposals"
	}
	return "/proposals?" + query.Encode()
}

// filterValues lists the file's selectable values: proposal ids for named
// kinds, band + entry prefixes for context (D3/D4) — one level, never deeper.
func filterValues(file *proposals.File) []string {
	values := make(map[string]bool)
	for _, group := range file.Groups {
		for index := range group.Proposals {
			for _, value := range proposalFilterKeys(&group.Proposals[index], file.Kind) {
				values[value] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(values))
}

// effectiveState is the state a proposal counts and filters under: a
// pending vote wins over the stored status.
func effectiveState(proposal *proposals.Proposal, kind string, votes map[string]proposals.Vote) string {
	vote, ok := votes[proposals.VoteKey(kind, proposal.Id)]
	if !ok {
		return proposal.Status
	}

	switch vote.Vote {
	case voteAccept:
		return "accepted"
	case voteReject:
		return "rejected"
	case votePostpone:
		return "postponed"
	}
	return proposal.Status
}

// stateFilterTarget parses a state filter token "state:<group>:<state>"
// into its group index and state; ok is false for non-state filters.
func stateFilterTarget(filter string) (int, string, bool) {
	rest, isStateFilter := strings.CutPrefix(filter, stateFilterPrefix)
	if !isStateFilter {
		return 0, "", false
	}

	indexPart, state, isTwoPart := strings.Cut(rest, ":")
	if !isTwoPart {
		return 0, "", false
	}
	index, err := strconv.Atoi(indexPart)
	if err != nil {
		return 0, "", false
	}
	return index, state, true
}

// visibleUnderStateFilter scopes a state filter to the group it was
// clicked on: that group narrows to the state, statusless (rerouted /
// considered) entries hide everywhere, every other group stays complete.
func visibleUnderStateFilter(proposal *proposals.Proposal, kind string, isTargetGroup bool, state string, votes map[string]proposals.Vote) bool {
	if proposal.Status == "" {
		return false
	}
	if !isTargetGroup {
		return true
	}
	return effectiveState(proposal, kind, votes) == state
}

var (
	// entryPrefixPattern captures a canon entry id minus its number
	// (identity grammar: ACTION|RULE|FACT-SCOPE[-TOPIC]) — the context
	// kind's filter category.
	entryPrefixPattern = regexp.MustCompile(`((?:ACTION|RULE|FACT)-[A-Z]+(?:-[A-Z]+)?)-[0-9]`)
	splitIdPattern     = regexp.MustCompile(`--\d+$`)
)

// workflowFilterPrefix namespaces workflow chips inside a kind's one filter
// param, so skills and workflows can render as separate rows.
const workflowFilterPrefix = "wf:"

// namedProposalFilterKey is what a skills/routines proposal filters under:
// workflow proposals by their bundled script (wf: namespace), edits by their
// target skill, everything else by its split-stripped id (plan D9).
func namedProposalFilterKey(proposal *proposals.Proposal) string {
	if strings.Contains(proposal.Target, "/workflows/") {
		base := path.Base(proposal.Target)
		return workflowFilterPrefix + strings.TrimSuffix(base, ".js")
	}
	if proposal.Target != "" && !strings.Contains(proposal.Target, "/") {
		return proposal.Target
	}
	return splitIdPattern.ReplaceAllString(proposal.Id, "")
}

// proposalFilterKeys returns what a proposal matches on: its target skill or
// workflow (falling back to the base id — split siblings share their
// candidate's badge), or for context the ACDSL-vs-prose band key plus every
// canon entry prefix mentioned in title and fields. Only actual proposals
// participate — rerouted/considered entries carry no status and never become
// filter values (nor match one).
func proposalFilterKeys(proposal *proposals.Proposal, kind string) []string {
	if proposal.Status == "" {
		return nil
	}
	if kind != "context" {
		key := namedProposalFilterKey(proposal)
		if key == "" {
			return nil
		}
		return []string{key}
	}

	keys := []string{gateFilterKey(&proposal.Gate)}
	seen := make(map[string]bool)
	scan := func(text string) {
		for _, match := range entryPrefixPattern.FindAllStringSubmatch(text, -1) {
			if prefix := match[1]; !seen[prefix] {
				seen[prefix] = true
				keys = append(keys, prefix)
			}
		}
	}
	scan(proposal.Title)
	for _, field := range proposal.Fields {
		scan(field.Label)
		scan(field.Text)
	}
	return keys
}

// gateFilterKey is the ACDSL-vs-prose axis: bands F/A/D are gate-backed
// ("acdsl"), band J and a missing gate are prose.
func gateFilterKey(gate *proposals.Gate) string {
	switch gate.Band {
	case "F", "A", "D":
		return "acdsl"
	}
	return "prose"
}

func groupViews(file proposals.File, active map[string]string, refs map[string]sessions.SessionRef, votes map[string]proposals.Vote, tab string) []groupView {
	filter := active[file.Kind]
	stateGroup, filterState, isStateFilter := stateFilterTarget(filter)

	groups := make([]groupView, 0, len(file.Groups))
	for groupIndex := range file.Groups {
		group := &file.Groups[groupIndex]
		view := groupView{Title: group.Title}
		counts := make(map[string]int)
		for index := range group.Proposals {
			proposal := &group.Proposals[index]
			// Counts and total always cover the FULL group — an active
			// filter narrows the cards, never the numbers.
			if proposal.Status != "" {
				counts[effectiveState(proposal, file.Kind, votes)]++
				view.Total++
			}

			isVisible := true
			switch {
			case filter == "":
			case isStateFilter:
				isVisible = visibleUnderStateFilter(proposal, file.Kind, groupIndex == stateGroup, filterState, votes)
			default:
				isVisible = slices.Contains(proposalFilterKeys(proposal, file.Kind), filter)
			}
			if !isVisible {
				continue
			}
			view.Proposals = append(view.Proposals, proposalCardView(file.Kind, proposal, refs, votes))
		}
		// Statusless groups (rerouted/blocked/considered notes) live on
		// the Notes tab; votable groups on Open.
		if (tab == "notes") != (view.Total == 0) {
			continue
		}
		if len(view.Proposals) == 0 {
			continue
		}

		// A filter narrows to what the user asked for — its sections open
		// (id filters everywhere, a state-note only its own group); plain
		// swaps keep the client-side state.
		switch {
		case filter == "":
		case isStateFilter:
			view.IsFilterTarget = groupIndex == stateGroup
		default:
			view.IsFilterTarget = true
		}

		acceptedCount := counts["accepted"] + counts["applied"] + counts["building"]
		acceptedTooltip := fmt.Sprintf("%d of %d accepted", acceptedCount, view.Total)
		view.Accepted = stateNoteFor("accepted", acceptedCount, acceptedTooltip, active, file.Kind, groupIndex)
		view.Rejected = stateNoteFor("rejected", counts["rejected"], fmt.Sprintf("%d rejected", counts["rejected"]), active, file.Kind, groupIndex)
		view.Postponed = stateNoteFor("postponed", counts["postponed"], fmt.Sprintf("%d postponed", counts["postponed"]), active, file.Kind, groupIndex)

		// The count pill mirrors what the bar contains: votable proposals
		// on Open, notes on Notes.
		view.Count = view.Total
		if tab == "notes" {
			view.Count = len(view.Proposals)
		}
		view.CountLabel = countLabel(view.Count)
		view.DataSection = file.Kind + "/" + view.Title
		view.Subgroups = subgroupViews(file.Kind, &view)
		groups = append(groups, view)
	}
	return groups
}

// countLabel keeps the summary pill two digits wide.
func countLabel(n int) string {
	if n > 99 {
		return "99+"
	}
	return strconv.Itoa(n)
}

// contextCategory maps a context group title onto the context-surface
// taxonomy; unmatched titles (project-local, external, reroutes) become
// solo cards.
func contextCategory(title string) (name string, solo bool) {
	switch {
	case strings.HasPrefix(title, "context/rules/"):
		return "rules", false
	case strings.HasPrefix(title, "context/actions/"):
		return "actions", false
	case strings.HasPrefix(title, "context/facts/"):
		return "facts", false
	case strings.HasPrefix(title, "acdsl/"):
		return "acdsl", false
	}
	return title, true
}

// categoryViews wraps the authored groups in derived top-level cards: the
// context kind buckets by contextCategory in first-appearance order, every
// other kind gets one Solo card per group (rendered bare).
func categoryViews(kind string, groups []groupView) []categoryView {
	categories := make([]categoryView, 0, len(groups))
	byName := make(map[string]int)
	for index := range groups {
		group := &groups[index]
		name, solo := group.Title, true
		if kind == "context" {
			name, solo = contextCategory(group.Title)
		}
		group.Solo = solo

		position, exists := byName[name]
		if !exists || solo {
			position = len(categories)
			byName[name] = position
			categories = append(categories, categoryView{
				DataSection: kind + "/cat:" + name,
				Solo:        solo,
				Title:       name,
			})
		}
		category := &categories[position]
		category.Groups = append(category.Groups, *group)
		category.IsFilterTarget = category.IsFilterTarget || group.IsFilterTarget
	}
	for index := range categories {
		category := &categories[index]
		count := 0
		for _, group := range category.Groups {
			count += group.Count
		}
		category.CountLabel = countLabel(count)
	}
	return categories
}

// subgroupKey is what a proposal separates on inside its authored group:
// the target skill/surface, else the split-id base.
func subgroupKey(p *proposals.Proposal) string {
	if p.Target != "" {
		return p.Target
	}
	return splitIdPattern.ReplaceAllString(p.Id, "")
}

// subgroupViews derives per-target bars inside a votable authored group.
// The context kind already separates per surface via its group titles, and
// a group with one distinct key stays flat.
func subgroupViews(kind string, group *groupView) []subgroupView {
	if kind == "context" || group.Total == 0 {
		return nil
	}

	subgroups := make([]subgroupView, 0, len(group.Proposals))
	byKey := make(map[string]int)
	for _, proposal := range group.Proposals {
		key := subgroupKey(&proposal.Proposal)
		position, exists := byKey[key]
		if !exists {
			position = len(subgroups)
			byKey[key] = position
			subgroups = append(subgroups, subgroupView{
				DataSection: kind + "/" + group.Title + "/" + key,
				Title:       key,
			})
		}
		subgroups[position].Proposals = append(subgroups[position].Proposals, proposal)
	}
	if len(subgroups) < 2 {
		return nil
	}
	for index := range subgroups {
		subgroups[index].CountLabel = countLabel(len(subgroups[index].Proposals))
		// A filter narrows to what the user asked for — the bars holding
		// the matches open with their group.
		subgroups[index].IsFilterTarget = group.IsFilterTarget
	}
	return subgroups
}

// stateNoteFor builds one summary-notation element: count, decoding
// tooltip, and the group-scoped toggle URL (active note links to the
// cleared filter).
func stateNoteFor(state string, count int, tooltip string, active map[string]string, kind string, groupIndex int) stateNote {
	key := fmt.Sprintf("%s%d:%s", stateFilterPrefix, groupIndex, state)
	isActive := key == active[kind]
	urlValue := key
	action := "click to filter this set"
	if isActive {
		urlValue = ""
		action = "click to clear the filter"
	}

	note := stateNote{
		Active: isActive,
		Count:  count,
	}
	// A zero count is information, not a control — filtering on it would
	// empty the set and take the clear link with it.
	if count == 0 && !isActive {
		note.Tooltip = tooltip
		return note
	}

	note.Tooltip = tooltip + " — " + action
	note.URL = filterURL(active, kind, urlValue)
	return note
}

// votesPath is the sidecar location next to the proposal JSONs.
func (s *Server) votesPath() string {
	return filepath.Join(s.proposalsDir, "votes.jsonl")
}

func (s *Server) handleProposalVote(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if _, ok := kindDimension[kind]; !ok {
		respond.WithNotFound("unknown proposal kind", w)
		return
	}

	vote := r.FormValue("vote")
	isKnownVote := vote == voteAccept || vote == voteReject || vote == votePostpone
	if !isKnownVote {
		respond.WithBadRequest("vote must be +, - or p", w)
		return
	}

	comment := r.FormValue("comment")
	if len(comment) > maxVoteCommentLength {
		respond.WithBadRequest("comment exceeds 500 characters", w)
		return
	}

	files, _, err := proposals.Load(s.proposalsDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	proposal := findProposal(files, kind, r.PathValue("id"))
	if proposal == nil {
		respond.WithNotFound("unknown proposal id", w)
		return
	}

	decision := &proposals.Vote{
		Id:      proposal.Id,
		Comment: comment,
		Kind:    kind,
		Title:   proposal.Title,
		Ts:      time.Now().UTC().Format(time.RFC3339),
		Vote:    vote,
	}
	if err := proposals.SetVote(s.votesPath(), decision); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	votes := map[string]proposals.Vote{proposals.VoteKey(kind, proposal.Id): *decision}
	card := proposalCardView(kind, proposal, s.sessions.SessionRefs(), votes)
	s.renderFragment(w, tmplProposalCard, card)
}

// findProposal resolves a votable proposal by kind and id.
func findProposal(files []proposals.File, kind, id string) *proposals.Proposal {
	for index := range files {
		if files[index].Kind == kind {
			return findProposalInFile(&files[index], id)
		}
	}
	return nil
}

func findProposalInFile(file *proposals.File, id string) *proposals.Proposal {
	if id == "" {
		return nil
	}

	for index := range file.Groups {
		proposal := findProposalInGroup(&file.Groups[index], id)
		if proposal != nil {
			return proposal
		}
	}
	return nil
}

// findProposalInGroup matches votable proposals only — statusless
// (rerouted/considered) entries are not proposals and never match (D5).
func findProposalInGroup(group *proposals.Group, id string) *proposals.Proposal {
	for index := range group.Proposals {
		proposal := &group.Proposals[index]
		if proposal.Status != "" && proposal.Id == id {
			return proposal
		}
	}
	return nil
}

func proposalCardView(kind string, proposal *proposals.Proposal, refs map[string]sessions.SessionRef, votes map[string]proposals.Vote) proposalView {
	view := proposalView{
		Evidence: evidenceViews(*proposal, kind, refs),
		Kind:     kind,
		Proposal: *proposal,
	}

	vote, ok := votes[proposals.VoteKey(kind, proposal.Id)]
	if !ok {
		return view
	}

	isRevertTarget := proposal.Status == "accepted" || proposal.Status == "building"
	view.IsRevert = vote.Vote != voteAccept && isRevertTarget
	view.PendingComment = vote.Comment
	view.PendingDate = datePart(vote.Ts)
	view.PendingVote = vote.Vote
	return view
}

// datePart reduces an RFC3339 timestamp to its date.
func datePart(ts string) string {
	if len(ts) < len("2006-01-02") {
		return ts
	}
	return ts[:len("2006-01-02")]
}

func evidenceViews(proposal proposals.Proposal, kind string, refs map[string]sessions.SessionRef) []evidenceView {
	views := make([]evidenceView, 0, len(proposal.Evidence))
	for _, item := range proposal.Evidence {
		views = append(views, evidenceView{Item: item, Links: sessionLinks(item, kind, refs)})
	}
	return views
}

func sessionLinks(item proposals.Evidence, kind string, refs map[string]sessions.SessionRef) []sessionLink {
	dimension := item.Dimension
	if dimension == "" {
		dimension = kindDimension[kind]
	}

	links := make([]sessionLink, 0, len(item.Sessions))
	for _, session := range item.Sessions {
		link := sessionLink{Id: session.Id, Note: session.Note}
		if ref, ok := refs[session.Id]; ok {
			query := url.Values{"session": {session.Id}}
			if dimension != "" {
				query.Set("dimension", dimension)
			}
			link.URL = fmt.Sprintf("/sessions/%s/%d?%s", ref.Scope, ref.BatchNumber, query.Encode())
		}
		links = append(links, link)
	}
	return links
}
