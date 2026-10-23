package server

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/trash"
)

const maxStyleProfileBytes = 65536

// presentationStore re-reads after each save so the running server and the disk never diverge.
type presentationStore struct {
	mu sync.RWMutex

	path         string
	profile      *presentation.Profile
	stylePath    string
	templatePath string
	trash        *trash.Store
}

func newPresentationStore(path, stylePath, templatePath string, store *trash.Store) *presentationStore {
	opened := &presentationStore{
		path:         path,
		profile:      presentation.Load(path),
		stylePath:    stylePath,
		templatePath: templatePath,
		trash:        store,
	}
	return opened
}

func (st *presentationStore) audience() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.Audience
}

func (st *presentationStore) autoMerge() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.AutoMerge
}

func (st *presentationStore) isDeveloperAudience() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.IsDeveloperAudience()
}

func (st *presentationStore) isDevMode() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.DevMode
}

func (st *presentationStore) language() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.Language
}

func (st *presentationStore) reload() {
	profile := presentation.Load(st.path)
	st.mu.Lock()
	st.profile = profile
	st.mu.Unlock()
}

// saveProfileSelection persists what it is told; the casual devMode force-off lives in the handler.
func (st *presentationStore) saveProfileSelection(audience, language string, devMode, autoMerge bool) error {
	body := profileBody(st.path)
	if body == "" && audience == presentation.AudienceCasual && language == languageGerman {
		if template, err := os.ReadFile(st.templatePath); err == nil {
			body = stripFrontMatter(string(template))
		}
	}

	profile := &presentation.Profile{
		Audience:  audience,
		AutoMerge: autoMerge,
		DevMode:   devMode,
		Language:  language,
	}
	entry := &trash.Entry{Action: "profile selection save", Reason: "presentation profile settings saved on the Profile page"}
	if _, err := st.trash.Keep(st.path, entry); err != nil {
		return fmt.Errorf("presentationStore.saveProfileSelection: %w", err)
	}
	if err := fsx.WriteFileAtomic([]byte(formatProfile(profile, body)), st.path, 0o644); err != nil {
		return fmt.Errorf("presentationStore.saveProfileSelection: %w", err)
	}

	st.reload()
	return nil
}

func (st *presentationStore) savePresentationBody(content string) error {
	st.mu.RLock()
	profile := *st.profile
	st.mu.RUnlock()

	entry := &trash.Entry{Action: "profile body save", Reason: "presentation profile edited on the Profile page"}
	if _, err := st.trash.Keep(st.path, entry); err != nil {
		return fmt.Errorf("presentationStore.savePresentationBody: %w", err)
	}
	if err := fsx.WriteFileAtomic([]byte(formatProfile(&profile, content)), st.path, 0o644); err != nil {
		return fmt.Errorf("presentationStore.savePresentationBody: %w", err)
	}

	st.reload()
	return nil
}

func (st *presentationStore) presentationContent() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return profileBody(st.path)
}

func (st *presentationStore) saveStyle(content string) error {
	entry := &trash.Entry{Action: "profile style save", Reason: "style profile edited on the Profile page"}
	if _, err := st.trash.Keep(st.stylePath, entry); err != nil {
		return fmt.Errorf("presentationStore.saveStyle: %w", err)
	}
	if err := fsx.WriteFileAtomic([]byte(content), st.stylePath, 0o644); err != nil {
		return fmt.Errorf("presentationStore.saveStyle: %w", err)
	}
	return nil
}

func (st *presentationStore) styleContent() (string, error) {
	content, err := os.ReadFile(st.stylePath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("presentationStore.styleContent: %w", err)
	}
	return string(content), nil
}

// formatProfile is the single writer of the frontmatter shape, so every consumer parses one format.
func formatProfile(profile *presentation.Profile, body string) string {
	return fmt.Sprintf(
		"---\nlanguage: %s\naudience: %s\ndev-mode: %t\nauto-merge: %t\n---\n%s",
		profile.Language,
		profile.Audience,
		profile.DevMode,
		profile.AutoMerge,
		body,
	)
}

func profileBody(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return stripFrontMatter(string(content))
}

func stripFrontMatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			return strings.Join(lines[index+1:], "\n")
		}
	}
	return content
}
