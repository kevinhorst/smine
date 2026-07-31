package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplEnv = "_env.html"

type envData struct {
	Env        []envRow
	Overridden bool
}

type envRow struct {
	Key   string
	Value string
}

func (s *Server) handleDeleteEnv(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	env, err := main.Env()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	delete(env, key)

	if err := main.SetEnv(env); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := config.Save(s.settingsPath, main); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger", "config-op")
	s.renderEnv(w, main)
}

func (s *Server) handleGetEnv(w http.ResponseWriter, _ *http.Request) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderEnv(w, main)
}

func (s *Server) handleSetEnv(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	value := r.FormValue("value")
	if key == "" {
		respond.WithBadRequest("key must not be empty", w)
		return
	}

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	env, err := main.Env()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if env == nil {
		env = make(map[string]string)
	}
	env[key] = value

	if err := main.SetEnv(env); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := config.Save(s.settingsPath, main); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger", "config-op")
	s.renderEnv(w, main)
}

func (s *Server) renderEnv(w http.ResponseWriter, main *config.Settings) {
	env, err := main.Env()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	rows := make([]envRow, 0, len(env))
	for key, value := range env {
		row := envRow{Key: key, Value: value}
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(left, right envRow) int {
		return strings.Compare(left.Key, right.Key)
	})

	data := envData{Env: rows, Overridden: s.sectionOverridden(main, "env")}
	s.renderFragment(w, tmplEnv, data)
}
