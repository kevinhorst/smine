package server

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
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
	"routines":  "routine-candidate",
	"skills":    "skill-candidate",
	"style":     "rule",
	"workflows": "workflow-candidate",
}

type proposalsPage struct {
	AutoApplyContent string
	AutoApplyError   string
	AutoApplyPath    string
	Files            []fileView
	HappyWord        string
	HasGroups        bool
	LoadErrors       []string
	Page             string
	Tab              string
	Title            string
}

// happyWords feeds the Open tab's empty state — things that happen
// inside the LLM while there is nothing left to vote on.
var happyWords = []string{
	"Combobulating", "Confabulating", "Percolating", "Ruminating",
	"Cogitating", "Tokenizing", "Attending", "Embedding",
	"Extrapolating", "Recalibrating",
}

type fileView struct {
	AllURL  string
	File    proposals.File
	Filter  string
	Filters []proposalFilter
	Groups  []groupView
}

// proposalFilter is one badge in a kind's filter row (D3/D4).
type proposalFilter struct {
	Active bool
	URL    string
	Value  string
}

type groupView struct {
	Accepted       stateNote
	IsFilterTarget bool
	Postponed      stateNote
	Proposals      []proposalView
	Rejected       stateNote
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
	files, loadErrors, err := proposals.Load(filepath.Join(s.sessionsDir, "proposals"))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	tab := r.URL.Query().Get("tab")
	isKnownTab := tab == "notes" || tab == "auto-apply"
	if !isKnownTab {
		tab = "open"
	}

	// The auto-apply tab is the decide-rules editor — no proposal rendering.
	if tab == "auto-apply" {
		data := proposalsPage{
			AutoApplyPath: s.autoApplyRulesPath,
			Page:          pageProposals,
			Tab:           tab,
			Title:         "Proposals",
		}
		content, err := os.ReadFile(s.autoApplyRulesPath)
		if err != nil {
			data.AutoApplyError = err.Error()
		}

		data.AutoApplyContent = string(content)
		s.renderFragment(w, tmplProposals, data)
		return
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
		for _, value := range filterValues(&file) {
			view.Filters = append(view.Filters, proposalFilter{
				Active: value == view.Filter,
				URL:    filterURL(active, file.Kind, value),
				Value:  value,
			})
		}
		views = append(views, view)
	}
	return views
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
// kinds, RULE-<CATEGORY> prefixes for style (D3/D4) — one level, never deeper.
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
	rulePrefixPattern = regexp.MustCompile(`RULE-[A-Z]+`)
	splitIdPattern    = regexp.MustCompile(`--\d+$`)
)

// proposalFilterKeys returns what a proposal matches on: its base id (split
// siblings share their candidate's badge), or for style every RULE-<CATEGORY>
// prefix mentioned in title and fields. Only actual proposals participate —
// rerouted/considered entries carry no status and never become filter values
// (nor match one).
func proposalFilterKeys(proposal *proposals.Proposal, kind string) []string {
	if proposal.Status == "" {
		return nil
	}
	if kind != "style" {
		if proposal.Id == "" {
			return nil
		}
		return []string{splitIdPattern.ReplaceAllString(proposal.Id, "")}
	}

	seen := make(map[string]bool)
	var keys []string
	scan := func(text string) {
		for _, match := range rulePrefixPattern.FindAllString(text, -1) {
			if !seen[match] {
				seen[match] = true
				keys = append(keys, match)
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
		groups = append(groups, view)
	}
	return groups
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
	return filepath.Join(s.sessionsDir, "proposals", "votes.jsonl")
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

	files, _, err := proposals.Load(filepath.Join(s.sessionsDir, "proposals"))
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
