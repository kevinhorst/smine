package server

import (
	"net/http"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplEnv = "_env.html"

// envData.OverriddenCount is the row count behind the section badge: every row not byte-equal to the fragment.
type envData struct {
	Env             []envRow
	Overridden      bool
	OverriddenCount int
}

// envRow.RepoValue is the fragment's value of a changed key; Source marks the row against the fragment.
type envRow struct {
	Key       string
	RepoValue string
	Source    string
	Value     string
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

	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

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

	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderEnv(w, main)
}

func (s *Server) renderEnv(w http.ResponseWriter, main *config.Settings) {
	env, err := main.Env()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	fragmentEnv, known := s.loadFragmentEnv()
	rows := envRows(env, fragmentEnv, known)
	data := envData{Env: rows, Overridden: s.sectionOverridden(main, "env")}
	for index := range rows {
		if rows[index].Source != "" && rows[index].Source != sourceShared {
			data.OverriddenCount++
		}
	}
	s.renderFragment(w, tmplEnv, data)
}

// loadFragmentEnv degrades a fragment load error to an unknown fragment: the rows then carry no source.
func (s *Server) loadFragmentEnv() (map[string]string, bool) {
	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return nil, false
	}

	env, err := fragment.Env()
	if err != nil {
		return nil, false
	}
	return env, true
}

// envRows marks each live key against the fragment and appends the fragment keys the live settings lack, sorted by key.
func envRows(env, fragmentEnv map[string]string, known bool) []envRow {
	rows := make([]envRow, 0, len(env)+len(fragmentEnv))
	for key, value := range env {
		row := envRow{Key: key, Value: value}
		if known {
			row.Source, row.RepoValue = envSource(fragmentEnv, key, value)
		}
		rows = append(rows, row)
	}
	for key, value := range fragmentEnv {
		if _, isLive := env[key]; !isLive && known {
			rows = append(rows, envRow{Key: key, RepoValue: value, Source: sourceRepoOnly})
		}
	}
	slices.SortFunc(rows, func(left, right envRow) int {
		return strings.Compare(left.Key, right.Key)
	})
	return rows
}

// envSource is shared for an equal value, changed for another value, local for a key the fragment lacks.
func envSource(fragmentEnv map[string]string, key, value string) (string, string) {
	repoValue, inFragment := fragmentEnv[key]
	switch {
	case !inFragment:
		return sourceLocal, ""
	case repoValue == value:
		return sourceShared, ""
	}
	return sourceChanged, repoValue
}
