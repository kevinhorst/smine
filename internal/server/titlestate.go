package server

import (
	"log"
	"sync"

	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// titleStore is reloaded by a sync op.
type titleStore struct {
	mu sync.RWMutex

	skillsDir  string
	vocabulary *sessiontitle.Vocabulary
}

func newTitleStore(skillsDir string) *titleStore {
	store := &titleStore{skillsDir: skillsDir, vocabulary: &sessiontitle.Vocabulary{}}
	store.reload()
	return store
}

// reload keeps the in-memory vocabulary on a failed read.
func (st *titleStore) reload() {
	vocabulary, err := sessiontitle.LoadVocabulary(st.skillsDir)
	if err != nil {
		log.Printf("titleStore.reload: Skills dir %s: Failed to load title declarations: %v", st.skillsDir, err)
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	st.vocabulary = vocabulary
}

func (st *titleStore) view(title string) titleView {
	st.mu.RLock()
	defer st.mu.RUnlock()

	parsed := sessiontitle.ParseTitle(title, st.vocabulary)
	if parsed.State == sessiontitle.StateNone {
		return titleView{Text: title}
	}

	view := titleView{Badge: titleBadge(parsed.State), State: string(parsed.State), Text: parsed.Label()}
	if declared, isDeclared := st.vocabulary.Declared(parsed.State); isDeclared {
		view.Meaning = declared.Meaning
		view.Skill = declared.Skill
	}
	return view
}

// titleBadge gives the action tone to every state waiting on a human.
func titleBadge(state sessiontitle.State) string {
	switch state {
	case sessiontitle.StateDone:
		return badgeOk
	case sessiontitle.StateNone, sessiontitle.StateClosed, sessiontitle.StateWip:
		return badgeDim
	case sessiontitle.StateReady, sessiontitle.StateTesting:
		return badgeAction
	}
	return badgeAction
}
