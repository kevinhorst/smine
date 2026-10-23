package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	// maxPathRunes caps one collision path line; the tail is kept.
	maxPathRunes = 60
	pageFleet    = "fleet"
	// sensorAgentwt's degradation carries the repos' scan errors.
	sensorAgentwt = "agentwt"
	// shownPaths is how many collision paths show before the rest fold.
	shownPaths       = 10
	tmplFleet        = "fleet.html"
	tmplFleetArchive = "fleet_archive.html"
	tmplFleetStatus  = "_fleet_status.html"
)

// handleFleet paints the shell instantly; the fragment endpoint owns the heartbeat call.
func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	data := fleetPage{
		Fleet: r.URL.Query().Get("fleet"),
		Page:  pageFleet,
		Title: "Fleet",
	}
	s.renderFragment(w, tmplFleet, data)
}

func (s *Server) handleFleetMemberRemove(w http.ResponseWriter, r *http.Request) {
	fleet := r.PathValue("fleet")
	member := r.PathValue("member")
	if err := s.heartbeat.Remove(r.Context(), fleet, member); err != nil {
		log.Printf("Server.handleFleetMemberRemove: Fleet %s member %s: %v", fleet, member, err)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("HX-Trigger", "fleet-op")
	w.WriteHeader(http.StatusNoContent)
}

// handleFleetStatus renders a banner for an unreachable heartbeat, never a blank page.
func (s *Server) handleFleetStatus(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("fleet")
	data := fleetStatusPage{Filter: filter}
	archived, err := heartbeat.OpenArchive(s.archiveDir)
	if err != nil {
		log.Printf("Server.handleFleetStatus: Fleet %q: Failed to read the archive: %v", filter, err)
		data.ArchiveErr = err.Error()
	}
	data.Archived = archivedFleets(archived, filter)

	document, err := s.heartbeat.Status(r.Context(), filter)
	if err != nil {
		log.Printf("Server.handleFleetStatus: Fleet %q: Failed to read status: %v", filter, err)
		data.StatusErr = err.Error()
		s.renderFragment(w, tmplFleetStatus, data)
		return
	}

	data.Document = document
	data.Fleets = fleetViews(document)
	data.Problems = fleetProblems(document)
	data.Queue = heartbeat.Queue(document)
	data.Sessions = document.FleetSessions()
	s.renderFragment(w, tmplFleetStatus, data)
}

// handleFleetArchive renders one dissolved fleet run from its archive file.
func (s *Server) handleFleetArchive(w http.ResponseWriter, r *http.Request) {
	archived, err := heartbeat.ReadArchive(s.archiveDir, r.PathValue("instance"))
	switch {
	case err == nil:
	case errors.Is(err, heartbeat.ErrInvalidInstance):
		respond.WithBadRequest(err.Error(), w)
		return
	case errors.Is(err, heartbeat.ErrUnknownArchive):
		respond.WithNotFound(err.Error(), w)
		return
	default:
		respond.WithInternalServerError(err, w)
		return
	}

	data := fleetArchivePage{
		Archived: archived,
		Page:     pageFleet,
		Title:    "Fleet " + archived.Fleet + " " + archived.ShortInstance(),
	}
	s.renderFragment(w, tmplFleetArchive, data)
}

func archivedFleets(archived []heartbeat.ArchivedFleet, filter string) []heartbeat.ArchivedFleet {
	if filter == "" {
		return archived
	}

	kept := make([]heartbeat.ArchivedFleet, 0, len(archived))
	for _, fleet := range archived {
		if fleet.Fleet == filter {
			kept = append(kept, fleet)
		}
	}
	return kept
}

func degradedSensors(document *heartbeat.Document) []heartbeat.Sensor {
	degraded := make([]heartbeat.Sensor, 0)
	for _, sensor := range document.Sensors {
		if sensor.State == heartbeat.SensorStateDegraded {
			degraded = append(degraded, sensor)
		}
	}
	return degraded
}

// fleetProblems gives a failed repo its own card even without a degraded sensor.
func fleetProblems(document *heartbeat.Document) []fleetProblem {
	failed := scanErrors(document)
	problems := make([]fleetProblem, 0)
	isAgentwtDegraded := false
	for _, sensor := range degradedSensors(document) {
		problem := fleetProblem{Summary: sensor.Name + " degraded since " + stampTime(sensor.Since)}
		// A --fleet document carries only the bound repo; the sensor detail still names every failed repo.
		isRepoFailure := sensor.Name == sensorAgentwt && len(failed) > 0
		isAgentwtDegraded = isAgentwtDegraded || sensor.Name == sensorAgentwt
		switch {
		case isRepoFailure:
			problem.Summary += fmt.Sprintf(" — %d repo(s) failed to scan, git rows are from the last good scan", len(failed))
			problem.Details = repoDetails(failed)
		default:
			problem.Details = []string{sensor.Detail}
		}
		problems = append(problems, problem)
	}
	if isAgentwtDegraded {
		return problems
	}

	for _, repo := range failed {
		problem := fleetProblem{
			Details: repoDetails([]heartbeat.Repo{repo}),
			Summary: "repo " + repo.Name + " scan failed — git rows are from the last good scan",
		}
		problems = append(problems, problem)
	}
	return problems
}

func fleetViews(document *heartbeat.Document) []fleetView {
	views := make([]fleetView, 0, len(document.Fleets))
	for i := range document.Fleets {
		fleet := &document.Fleets[i]
		view := fleetView{Fleet: *fleet}
		view.Members = document.MemberRows(fleet)
		view.Train = trainSteps(document, fleet, view.Members)
		views = append(views, view)
	}
	return views
}

// headPaths is the first shownPaths paths of a collision.
func headPaths(paths []string) []string {
	return paths[:min(len(paths), shownPaths)]
}

func repoDetails(repos []heartbeat.Repo) []string {
	details := make([]string, 0, len(repos))
	for _, repo := range repos {
		details = append(details, repo.Name+": "+repo.ScanError)
		for _, warning := range repo.Warnings {
			details = append(details, repo.Name+" warning: "+warning)
		}
	}
	return details
}

func scanErrors(document *heartbeat.Document) []heartbeat.Repo {
	failed := make([]heartbeat.Repo, 0)
	for _, repo := range document.Repos {
		if repo.ScanError != "" {
			failed = append(failed, repo)
		}
	}
	return failed
}

// shortPath cuts from the left because a path's tail is the informative part.
func shortPath(path string) string {
	runes := []rune(path)
	if len(runes) <= maxPathRunes {
		return path
	}
	return "…" + string(runes[len(runes)-maxPathRunes+1:])
}

// tailPaths is the paths of a collision past the first shownPaths.
func tailPaths(paths []string) []string {
	return paths[min(len(paths), shownPaths):]
}

// trainSteps picks the train of the fleet's repo whose base is the fleet's target.
func trainSteps(document *heartbeat.Document, fleet *heartbeat.Fleet, members []heartbeat.MemberRow) []heartbeat.TrainStep {
	own := map[string]bool{fleet.MergeBranch: fleet.MergeBranch != ""}
	for _, member := range members {
		own[member.Branch] = member.Branch != ""
	}

	steps := make([]heartbeat.TrainStep, 0)
	for _, train := range document.Trains {
		isFleetTrain := train.Repo == fleet.Repo && train.Base == fleet.Target
		if !isFleetTrain {
			continue
		}

		for _, step := range train.Steps {
			if own[step.Branch] {
				steps = append(steps, step)
			}
		}
	}
	return steps
}
