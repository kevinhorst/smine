package server

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"

	"github.com/kevinhorst/smine/internal/checklist"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageChecklist = "checklist"
	tmplChecklist = "checklist.html"
)

type checklistEntryView struct {
	Body   template.HTML
	Number int
	Status string
	Tags   []string
	Title  string
}

type checklistPageData struct {
	CountDone int
	CountOpen int
	Entries   []checklistEntryView
	Error     string
	Page      string
	Tab       string
	Title     string
}

func (s *Server) handleChecklistPage(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "done" {
		tab = "open"
	}
	data := checklistPageData{Page: pageChecklist, Tab: tab, Title: "Workflow Improvements Checklist"}
	parsedChecklist, err := checklist.Parse(s.checklistPath)
	if err != nil {
		data.Error = err.Error()
	} else {
		for _, entry := range parsedChecklist.Entries {
			isDone := entry.Status == statusDone
			if isDone {
				data.CountDone++
			} else {
				data.CountOpen++
			}
			if isDone == (tab == "done") {
				data.Entries = append(data.Entries, checklistEntryData(parsedChecklist, entry))
			}
		}
	}
	s.renderFragment(w, tmplChecklist, data)
}

func (s *Server) handleChecklistStatus(w http.ResponseWriter, r *http.Request) {
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil || number < 1 {
		respond.WithBadRequest("invalid entry number", w)
		return
	}

	status := r.FormValue("status")
	err = checklist.SetStatus(s.checklistPath, number, status)
	switch {
	case errors.Is(err, checklist.ErrInvalidTag):
		respond.WithBadRequest(err.Error(), w)
		return
	case errors.Is(err, checklist.ErrEntryNotFound):
		respond.WithConflict("file changed underneath, reload", w)
		return
	case err != nil:
		respond.WithInternalServerError(err, w)
		return
	}

	// Full reload: tab counters update and the entry moves to its new tab;
	// this also removes the OOB <tr> that browsers stripped into a stray
	// text line (D6).
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func checklistEntryData(parsedChecklist *checklist.Checklist, entry checklist.Entry) checklistEntryView {
	body, err := renderMarkdown([]byte(entry.Body))
	if err != nil {
		body = template.HTML("")
	}

	return checklistEntryView{
		Body:   body,
		Number: entry.Number,
		Status: entry.Status,
		Tags:   parsedChecklist.Tags,
		Title:  entry.Title,
	}
}
