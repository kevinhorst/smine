package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	journalTailLimit = 200
	pageTrash        = "trash"
	tmplTrash        = "trash.html"
	trashLockKey     = "_trash"
)

type trashPage struct {
	Journal []*trash.Entry
	LoadErr string
	Page    string
	Stashes []*trash.Stash
	Title   string
}

func (s *Server) handleTrashPage(w http.ResponseWriter, r *http.Request) {
	page := trashPage{Page: pageTrash, Title: "Trash"}
	stashes, err := s.trash.List()
	if err != nil {
		page.LoadErr = err.Error()
	}

	page.Stashes = stashes
	page.Journal, err = s.trash.Tail(journalTailLimit)
	if err != nil {
		page.LoadErr = err.Error()
	}

	s.renderFragment(w, tmplTrash, page)
}

func (s *Server) handleTrashRestore(w http.ResponseWriter, r *http.Request) {
	stamp := r.PathValue("stamp")
	rel := r.FormValue("rel")
	if stamp == "" || rel == "" {
		respond.WithBadRequest("stamp and rel are required", w)
		return
	}

	isReplacing := r.FormValue("replace") == "on"
	restore := func(ctx context.Context) (string, error) {
		if err := s.trash.Restore(stamp, rel, isReplacing); err != nil {
			return "", err
		}
		return fmt.Sprintf("restored %s from %s", rel, stamp), nil
	}
	s.runRepoOp(trashLockKey, "trash-op", restore, w, r)
}
