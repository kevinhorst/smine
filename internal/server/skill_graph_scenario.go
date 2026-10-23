package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

// Query keys of the scenario endpoint.
const (
	scenarioParamFlip = "flip"
	scenarioParamOn   = "on"
)

// handleSkillGraphScenario answers 409 with the load error when the model does not load.
func (s *Server) handleSkillGraphScenario(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	skillModel, derived, err := loadStructure(skill.Path)
	if err != nil {
		respond.WithConflict(err.Error(), w)
		return
	}

	query := r.URL.Query()
	scenario := skillmodel.NewScenario(skillmodel.Inputs(skillModel), query[scenarioParamOn])
	isFlipped := false
	if pred := query.Get(scenarioParamFlip); pred != "" {
		isFlipped = scenario.Flip(derived, pred)
	}
	result := skillmodel.Evaluate(derived, scenario)
	result.Flipped = isFlipped
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Server.handleSkillGraphScenario: Skill %s: Failed to write response: %v", skill.Name, err)
	}
}
