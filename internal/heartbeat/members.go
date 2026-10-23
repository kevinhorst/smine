package heartbeat

import "strings"

// workerBranchPrefix is the namespace of a fleet worker's branch.
const workerBranchPrefix = "claude/"

// branchKey addresses one branch row; P2 keys branches per repo.
type branchKey struct {
	branch string
	repo   string
}

// MemberRow is one fleet member joined to its state, branch and work items.
type MemberRow struct {
	Id          string
	Branch      string
	BranchState BranchState
	Delivered   bool
	LastActive  string
	MemberId    string
	Name        string
	Role        string
	SessionId   string
	Since       string
	Skill       string
	State       string
	Title       string
	Token       string
	Tone        Tone
	Via         Via
	WorkItem    string
}

func (d *Document) branchIndex() map[branchKey]*Branch {
	index := make(map[branchKey]*Branch, len(d.Branches))
	for i := range d.Branches {
		if d.Branches[i].Branch == "" {
			continue
		}

		key := branchKey{branch: d.Branches[i].Branch, repo: d.Branches[i].Repo}
		index[key] = &d.Branches[i]
	}
	return index
}

// fleetSessionIds holds joined and subscriber members' sessions and the root sessions in a worker's branch.
func (d *Document) fleetSessionIds() map[string]bool {
	repos := make(map[string]string, len(d.Fleets))
	for i := range d.Fleets {
		repos[d.Fleets[i].Id] = d.Fleets[i].Repo
	}

	branches := d.branchIndex()
	ids := make(map[string]bool)
	for _, member := range d.Members {
		switch member.Via {
		case ViaJoined, ViaSubscriber:
			ids[member.SessionId] = true
		case ViaWorker:
			key := branchKey{branch: member.Branch, repo: repos[member.Fleet]}
			addBranchSessions(ids, branches[key])
		}
	}
	return ids
}

// membersOf lists the members of fleet in document order.
func (d *Document) membersOf(fleet string) []Member {
	members := make([]Member, 0)
	for _, member := range d.Members {
		if member.Fleet == fleet {
			members = append(members, member)
		}
	}
	return members
}

func (d *Document) sessionIndex() map[string]*Session {
	index := make(map[string]*Session, len(d.Sessions))
	for i := range d.Sessions {
		index[d.Sessions[i].Id] = &d.Sessions[i]
	}
	return index
}

// FleetSessions keeps the document's session order.
func (d *Document) FleetSessions() []Session {
	ids := d.fleetSessionIds()
	sessions := make([]Session, 0, len(ids))
	for i := range d.Sessions {
		if ids[d.Sessions[i].Id] {
			sessions = append(sessions, d.Sessions[i])
		}
	}
	return sessions
}

// A worker has no session row of its own.
func (d *Document) MemberRows(fleet *Fleet) []MemberRow {
	branches := d.branchIndex()
	sessions := d.sessionIndex()

	members := d.membersOf(fleet.Id)
	rows := make([]MemberRow, 0, len(members))
	for _, member := range members {
		row := MemberRow{
			Id:        member.SessionId,
			Branch:    member.Branch,
			Delivered: member.Delivered,
			MemberId:  member.MemberId,
			Role:      member.Via.Role(),
			SessionId: member.SessionId,
			Token:     member.Token,
			Via:       member.Via,
		}
		switch member.Via {
		case ViaJoined, ViaSubscriber:
			fillSession(&row, sessions[member.SessionId])
		case ViaWorker:
			fillWorker(&row, &member)
		}
		if row.Name == "" {
			row.Name = member.Token
		}
		row.Name = withoutToken(row.Name, member.Token)

		key := branchKey{branch: member.Branch, repo: fleet.Repo}
		fillBranch(&row, branches[key])
		rows = append(rows, row)
	}
	return rows
}

func addBranchSessions(ids map[string]bool, branch *Branch) {
	if branch == nil {
		return
	}

	for _, id := range branch.Sessions {
		ids[id] = true
	}
}

func fillBranch(row *MemberRow, branch *Branch) {
	if branch == nil {
		return
	}

	row.BranchState = branch.State
	row.WorkItem = strings.Join(branch.WorkItems, ", ")
}

func fillSession(row *MemberRow, session *Session) {
	if session == nil {
		return
	}

	row.Name = session.Title
	row.Since = session.StateSince
	if session.LastEvent != nil {
		row.LastActive = session.LastEvent.At
	}
	row.Skill = session.ActiveSkill
	row.State = string(session.State)
	row.Title = session.Title
	row.Tone = session.State.Tone()
}

// A worker runs until the orchestrator reported it delivered, ended after.
func fillWorker(row *MemberRow, member *Member) {
	state := SessionStateRunning
	if member.Delivered {
		state = SessionStateEnded
	}

	row.Id = member.Branch
	row.Name = strings.TrimPrefix(member.Branch, workerBranchPrefix)
	row.State = string(state)
	row.Tone = state.Tone()
}

// Cuts only the session-title hook's exact suffix, which the token badge already shows.
func withoutToken(title string, token string) string {
	marker := " - " + token + "["
	index := strings.LastIndex(title, marker)
	isSuffix := index >= 0 && strings.HasSuffix(title, "]")
	if !isSuffix || token == "" {
		return title
	}
	return title[:index]
}
