package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	keyFallbackModel = "fallbackModel"
	tmplModel        = "_model.html"
)

// modelSettingKeys are the claude settings keys that choose a model.
var modelSettingKeys = []string{"advisorModel", keyFallbackModel, "model"}

// modelData.RepoModel is the fragment's model, shown beside the live one when they differ.
type modelData struct {
	Model      string
	Models     []string
	Overridden bool
	RepoModel  string
}

func (s *Server) vocabulary() (*models.Vocabulary, error) {
	heartbeatPath := filepath.Join(filepath.Dir(s.settingsPath), "heartbeat", "config.json")
	return models.Load(s.settingsPath, heartbeatPath)
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

	value := r.FormValue("model")
	if value != "" {
		chosen := []string{value}
		if err := s.requireAvailableModels(chosen); err != nil {
			respond.WithBadRequest(err.Error(), w)
			return
		}
	}

	if err := main.SetModel(value); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderModel(w, main)
}

func (s *Server) renderModel(w http.ResponseWriter, main *config.Settings) {
	model, err := main.Model()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	models, err := main.AvailableModels()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := modelData{Model: model, Models: models, Overridden: s.sectionOverridden(main, "model")}
	if data.Overridden {
		data.RepoModel = s.fragmentModel()
	}
	s.renderFragment(w, tmplModel, data)
}

// fragmentModel degrades a fragment load error to no value.
func (s *Server) fragmentModel() string {
	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return ""
	}

	model, err := fragment.Model()
	if err != nil {
		return ""
	}
	return model
}

func (s *Server) requireAvailableModels(chosen []string) error {
	vocabulary, err := s.vocabulary()
	if err != nil {
		return err
	}

	for _, model := range chosen {
		if err := vocabulary.Check(model); err != nil {
			return fmt.Errorf("Server.requireAvailableModels: %w", err)
		}
	}
	return nil
}
