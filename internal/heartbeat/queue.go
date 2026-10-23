package heartbeat

import "strings"

const (
	journalKindMerged = "merged"
	journalKindNote   = "note"
	// Starts the orchestrator's note when git refuses the target's fast-forward.
	syncBlockedPrefix = "sync blocked"
)

// QueueItem is one human-layer item (concept D28).
type QueueItem struct {
	Id      string
	Detail  string
	Fleet   string
	Kind    QueueKind
	Since   string
	Subject string
}

type QueueKind string

const (
	QueueKindAwaitingDone  QueueKind = "awaiting_done"
	QueueKindPermission    QueueKind = "permission"
	QueueKindReadyForMain  QueueKind = "ready_for_main"
	QueueKindReadyToSync   QueueKind = "ready_to_sync"
	QueueKindSyncBlocked   QueueKind = "sync_blocked"
	QueueKindTargetMissing QueueKind = "target_missing"
	QueueKindWaiting       QueueKind = "waiting"
)

func (k QueueKind) Label() string {
	switch k {
	case QueueKindAwaitingDone:
		return "awaiting /done"
	case QueueKindPermission:
		return "permission candidate"
	case QueueKindReadyForMain:
		return "ready for main"
	case QueueKindReadyToSync:
		return "ready to sync"
	case QueueKindSyncBlocked:
		return "sync blocked"
	case QueueKindTargetMissing:
		return "target missing"
	case QueueKindWaiting:
		return "waiting on human"
	}
	return string(k)
}

// A derived item disappears when its condition clears.
func Queue(document *Document) []QueueItem {
	branches := document.branchIndex()
	sessions := document.sessionIndex()

	items := make([]QueueItem, 0)
	for i := range document.Fleets {
		fleet := &document.Fleets[i]
		items = append(items, fleetItems(fleet, document.membersOf(fleet.Id), sessions, branches)...)
	}
	return items
}

// An idle member with a merge-ready branch but no /done is never merged by the orchestrator.
func awaitingDoneItems(fleet *Fleet, member *Member, session *Session, branch *Branch) []QueueItem {
	items := make([]QueueItem, 0)
	isUndelivered := member.Via == ViaJoined && !member.Delivered
	isIdle := session != nil && session.State == SessionStateIdle
	isMergeReady := branch != nil && branch.State == BranchStateMergeReady
	if !isUndelivered || !isIdle {
		return items
	}
	if !isMergeReady {
		return items
	}

	item := QueueItem{
		Id:      fleet.Id + "/awaiting_done/" + member.SessionId,
		Detail:  "branch " + member.Branch + " is merge-ready; the member has not run /done",
		Fleet:   fleet.Id,
		Kind:    QueueKindAwaitingDone,
		Since:   session.StateSince,
		Subject: member.Token,
	}
	items = append(items, item)
	return items
}

func fleetItems(fleet *Fleet, members []Member, sessions map[string]*Session, branches map[branchKey]*Branch) []QueueItem {
	items := targetItems(fleet)
	for i := range members {
		member := &members[i]
		if member.Via == ViaWorker {
			continue
		}

		session := sessions[member.SessionId]
		branch := branches[branchKey{branch: member.Branch, repo: fleet.Repo}]
		items = append(items, sessionItems(fleet.Id, session)...)
		items = append(items, awaitingDoneItems(fleet, member, session, branch)...)
	}
	return items
}

func sessionItems(fleet string, session *Session) []QueueItem {
	items := make([]QueueItem, 0)
	if session == nil {
		return items
	}

	if session.Blocked != nil {
		item := QueueItem{
			Id:      fleet + "/permission/" + session.Id,
			Detail:  session.Blocked.CandidateRule,
			Fleet:   fleet,
			Kind:    QueueKindPermission,
			Since:   session.Blocked.DeniedAt,
			Subject: session.Title,
		}
		items = append(items, item)
	}

	if session.State == SessionStateWaiting {
		item := QueueItem{
			Id:      fleet + "/waiting/" + session.Id,
			Detail:  session.WaitingFor,
			Fleet:   fleet,
			Kind:    QueueKindWaiting,
			Since:   session.StateSince,
			Subject: session.Title,
		}
		items = append(items, item)
	}
	return items
}

func syncBlockedNote(fleet *Fleet) *JournalEntry {
	for i := len(fleet.Journal) - 1; i >= 0; i-- {
		entry := &fleet.Journal[i]
		switch {
		case entry.Kind == journalKindMerged:
			return nil
		case entry.Kind == journalKindNote && strings.HasPrefix(entry.Subject, syncBlockedPrefix):
			return entry
		}
	}
	return nil
}

func syncItems(fleet *Fleet) []QueueItem {
	items := make([]QueueItem, 0)
	blocked := syncBlockedNote(fleet)
	switch {
	case fleet.Synced:
		item := QueueItem{
			Id:      fleet.Id + "/ready_for_main",
			Detail:  "all fleet work synced to the target — landing on main is human",
			Fleet:   fleet.Id,
			Kind:    QueueKindReadyForMain,
			Subject: fleet.Target,
		}
		items = append(items, item)
	case blocked != nil:
		item := QueueItem{
			Id:      fleet.Id + "/sync_blocked",
			Detail:  blocked.Subject,
			Fleet:   fleet.Id,
			Kind:    QueueKindSyncBlocked,
			Since:   blocked.At,
			Subject: fleet.Target,
		}
		items = append(items, item)
	default:
		item := QueueItem{
			Id:      fleet.Id + "/ready_to_sync",
			Detail:  "all member work merged into " + fleet.MergeBranch + " — the orchestrator syncs the target next",
			Fleet:   fleet.Id,
			Kind:    QueueKindReadyToSync,
			Subject: fleet.Target,
		}
		items = append(items, item)
	}
	return items
}

func targetItems(fleet *Fleet) []QueueItem {
	items := make([]QueueItem, 0)
	switch fleet.TargetState {
	case TargetStateIntegrated:
		items = append(items, syncItems(fleet)...)
	case TargetStateMissing:
		item := QueueItem{
			Id:      fleet.Id + "/target_missing",
			Detail:  "integration branch is no local branch in the repo's last scan",
			Fleet:   fleet.Id,
			Kind:    QueueKindTargetMissing,
			Subject: fleet.Target,
		}
		items = append(items, item)
	case TargetStateOpen:
	}
	return items
}
