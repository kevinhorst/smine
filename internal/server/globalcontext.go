package server

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	// globalContextGroupSuffix ends the smineId of every global-context hook group in the context fragment.
	globalContextGroupSuffix = "-globalcontext"
	tmplGlobalContextDoc     = "global_context_doc.html"
)

type globalContextFile struct {
	Modified string
	Name     string
	Size     string
}

type globalContextDocPage struct {
	Content string
	Name    string
	Page    string
	Title   string
}

// parkedSlot is one peeked position of the parked store, dropped once settings.json holds its group.
type parkedSlot struct {
	event string
	index int
}

// globalContextDir is the presentation profile's parent, which the global-context hook globs.
func (s *Server) globalContextDir() string {
	return filepath.Dir(s.presentation.path)
}

// globalContextGroupIds are the context fragment's global-context hook group ids.
func (s *Server) globalContextGroupIds() ([]string, error) {
	fragment, err := config.LoadComponentFragment(s.claudeFragmentPath, string(components.Context), s.markers())
	if err != nil {
		return nil, fmt.Errorf("Server.globalContextGroupIds: %w", err)
	}

	ids, err := fragment.HookIds()
	if err != nil {
		return nil, fmt.Errorf("Server.globalContextGroupIds: %w", err)
	}
	return slices.DeleteFunc(ids, func(id string) bool { return !strings.HasSuffix(id, globalContextGroupSuffix) }), nil
}

// globalContextFiles treats a missing directory as empty: a fresh install has no global context yet.
func (s *Server) globalContextFiles() ([]globalContextFile, error) {
	entries, err := os.ReadDir(s.globalContextDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Server.globalContextFiles: %w", err)
	}

	files := make([]globalContextFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		file := globalContextFile{
			Modified: info.ModTime().Format("2006-01-02"),
			Name:     entry.Name(),
			Size:     humanSize(info.Size()),
		}
		files = append(files, file)
	}
	return files, nil
}

// globalContextEnabled reads whether none of the global-context hook groups is parked; an unreadable store or fragment reads as enabled.
func (s *Server) globalContextEnabled() bool {
	ids, err := s.globalContextGroupIds()
	if err != nil {
		return true
	}

	parked, err := s.disabledHooks.Snapshot()
	if err != nil {
		return true
	}
	for _, groups := range parked {
		isParked := slices.ContainsFunc(groups, func(group config.HookGroup) bool { return slices.Contains(ids, group.Id) })
		if isParked {
			return false
		}
	}
	return true
}

// setGlobalContextEnabled moves the global-context hook groups between settings.json and the parked store, the Hooks tab's toggle path.
func (s *Server) setGlobalContextEnabled(enabled bool) error {
	ids, err := s.globalContextGroupIds()
	if err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}

	main, err := config.Load(s.settingsPath)
	if err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}

	mainHooks, err := main.Hooks()
	if err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}
	if mainHooks == nil {
		mainHooks = make(map[string][]config.HookGroup)
	}

	var peeked []parkedSlot
	if enabled {
		peeked, err = s.unparkGroups(ids, mainHooks)
	} else {
		err = s.parkGroups(ids, mainHooks)
	}
	if err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}

	if err := main.SetHooks(mainHooks); err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}
	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}
	if err := s.dropUnparked(peeked); err != nil {
		return fmt.Errorf("Server.setGlobalContextEnabled: %w", err)
	}
	return nil
}

// parkGroups takes every live group carrying one of ids out of mainHooks into the parked store.
func (s *Server) parkGroups(ids []string, mainHooks map[string][]config.HookGroup) error {
	events := slices.Sorted(maps.Keys(mainHooks))
	for _, event := range events {
		for index, live := range slices.Backward(mainHooks[event]) {
			if !slices.Contains(ids, live.Id) {
				continue
			}

			group, _ := takeHookGroup(mainHooks, event, index)
			if err := s.disabledHooks.Add(event, group); err != nil {
				return fmt.Errorf("Server.parkGroups: %w", err)
			}
		}
	}
	return nil
}

// unparkGroups peeks every parked group carrying one of ids back into mainHooks; the caller saves settings, then runs dropUnparked on the result.
func (s *Server) unparkGroups(ids []string, mainHooks map[string][]config.HookGroup) ([]parkedSlot, error) {
	parked, err := s.disabledHooks.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("Server.unparkGroups: %w", err)
	}

	var peeked []parkedSlot
	for event, groups := range parked {
		for index, parkedGroup := range slices.Backward(groups) {
			if !slices.Contains(ids, parkedGroup.Id) {
				continue
			}

			group, ok, err := s.disabledHooks.Peek(event, index)
			if err != nil {
				return nil, fmt.Errorf("Server.unparkGroups: %w", err)
			}
			if ok {
				mainHooks[event] = append(mainHooks[event], group)
				peeked = append(peeked, parkedSlot{event: event, index: index})
			}
		}
	}
	return peeked, nil
}

// dropUnparked removes the peeked groups from the parked store after settings.json holds them; each event's indices arrive descending.
func (s *Server) dropUnparked(peeked []parkedSlot) error {
	for _, slot := range peeked {
		if err := s.disabledHooks.Drop(slot.event, slot.index); err != nil {
			return fmt.Errorf("Server.dropUnparked: %w", err)
		}
	}
	return nil
}

func (s *Server) handleContextGlobalToggle(w http.ResponseWriter, r *http.Request) {
	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	if err := s.setGlobalContextEnabled(!s.globalContextEnabled()); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	http.Redirect(w, r, "/context", http.StatusSeeOther)
}

// handleContextGlobalDoc validates the name against the listing so no path outside the directory is read.
func (s *Server) handleContextGlobalDoc(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	files, err := s.globalContextFiles()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	known := false
	for _, file := range files {
		if file.Name == name {
			known = true
			break
		}
	}
	if !known {
		respond.WithNotFound("unknown global context file", w)
		return
	}

	content, err := os.ReadFile(filepath.Join(s.globalContextDir(), name))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	page := globalContextDocPage{
		Content: string(content),
		Name:    name,
		Page:    pageContext,
		Title:   name,
	}
	s.renderFragment(w, tmplGlobalContextDoc, page)
}

// humanSize renders a byte count as a compact size for the file list.
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size) / unit
	if value < unit {
		return fmt.Sprintf("%.1f KiB", value)
	}
	return fmt.Sprintf("%.1f MiB", value/unit)
}
