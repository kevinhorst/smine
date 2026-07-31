package server

import (
	"net/http"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplModel = "_model.html"

type modelData struct {
	Model      string
	Overridden bool
}

func (s *Server) handleGetModel(w http.ResponseWriter, _ *http.Request) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderModel(w, main)
}

func (s *Server) handleSetModel(w http.ResponseWriter, r *http.Request) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := main.SetModel(r.FormValue("model")); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := config.Save(s.settingsPath, main); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger", "config-op")
	s.renderModel(w, main)
}

func (s *Server) renderModel(w http.ResponseWriter, main *config.Settings) {
	model, err := main.Model()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := modelData{Model: model, Overridden: s.sectionOverridden(main, "model")}
	s.renderFragment(w, tmplModel, data)
}
