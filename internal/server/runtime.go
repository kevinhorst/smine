package server

import (
	"log"
	"net/http"

	"github.com/kevinhorst/smine/internal/heartbeat"
)

const (
	pageRuntime       = "runtime"
	tmplRuntime       = "runtime.html"
	tmplRuntimeStatus = "_runtime_status.html"
)

// anchorStep is one run of consecutive equal anchor hits: Count folds the repeats, Seq and LastSeq bound them.
type anchorStep struct {
	Id      string
	Count   int
	LastSeq uint64
	Seq     uint64
}

// anchorTrack folds consecutive equal anchor hits into steps so a long run reads as its shape, not as its list.
func anchorTrack(hits []heartbeat.AnchorHit) []anchorStep {
	var steps []anchorStep
	for index := range hits {
		hit := &hits[index]
		isRepeat := len(steps) > 0 && steps[len(steps)-1].Id == hit.Id
		if isRepeat {
			last := &steps[len(steps)-1]
			last.Count++
			last.LastSeq = hit.Seq
			continue
		}

		step := anchorStep{Id: hit.Id, Count: 1, LastSeq: hit.Seq, Seq: hit.Seq}
		steps = append(steps, step)
	}
	return steps
}

// handleRuntime paints the shell; the fragment owns the heartbeat call.
func (s *Server) handleRuntime(w http.ResponseWriter, r *http.Request) {
	data := runtimePage{Page: pageRuntime, Title: "Fleet — Runtime"}
	s.renderFragment(w, tmplRuntime, data)
}

// handleRuntimeStatus renders a banner for an unreachable heartbeat, never a blank page.
func (s *Server) handleRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	data := runtimeStatusPage{}
	document, err := s.heartbeat.Runtime(r.Context())
	if err != nil {
		log.Printf("Server.handleRuntimeStatus: Failed to read the runtime document: %v", err)
		data.StatusErr = err.Error()
		s.renderFragment(w, tmplRuntimeStatus, data)
		return
	}

	data.Document = document
	s.renderFragment(w, tmplRuntimeStatus, data)
}
