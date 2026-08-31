package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	audienceCasual  = "casual"
	languageEnglish = "en"

	maxStyleProfileBytes = 65536
)

type presentationProfile struct {
	Audience string
	DevMode  bool
	Language string
}

func (p *presentationProfile) isDeveloperAudience() bool {
	return p.Audience != audienceCasual
}

// presentationStore holds the live profile behind the file it mirrors: saves
// write the file atomically and re-read it, so the running server and the
// disk never diverge and template funcs pick up edits without a restart
// (plan D3).
type presentationStore struct {
	mu sync.RWMutex

	path      string
	profile   *presentationProfile
	stylePath string
}

func newPresentationStore(path, stylePath string) *presentationStore {
	store := &presentationStore{
		path:      path,
		profile:   loadPresentationProfile(path),
		stylePath: stylePath,
	}
	return store
}

func (st *presentationStore) audience() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.Audience
}

func (st *presentationStore) isDeveloperAudience() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.profile.isDeveloperAudience()
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
	profile := loadPresentationProfile(st.path)
	st.mu.Lock()
	st.profile = profile
	st.mu.Unlock()
}

// saveProfileSelection persists the declared audience/language/dev-mode:
// all-default (developer, English, dev-mode off) deletes the file, the de
// casual combo seeds from the repo template on first activation, anything
// else rewrites only the frontmatter and keeps the body (plan D6). The
// casual force-off for devMode lives in the handler — the store persists
// what it is told.
func (st *presentationStore) saveProfileSelection(audience, language string, devMode bool) error {
	if audience == "" && language == languageEnglish && !devMode {
		if err := os.Remove(st.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("presentationStore.saveProfileSelection: Failed to remove %s: %w", st.path, err)
		}
		st.reload()
		return nil
	}

	body := profileBody(st.path)
	if body == "" && audience == audienceCasual && language == languageGerman {
		templatePath := filepath.Join("settings", "claude_code", "presentation-profile.de.md")
		if template, err := os.ReadFile(templatePath); err == nil {
			body = stripFrontMatter(string(template))
		}
	}

	content := fmt.Sprintf("---\nlanguage: %s\naudience: %s\ndev-mode: %t\n---\n%s", language, audience, devMode, body)
	if err := writeFileAtomic(st.path, content); err != nil {
		return fmt.Errorf("presentationStore.saveProfileSelection: %w", err)
	}

	st.reload()
	return nil
}

func (st *presentationStore) saveStyle(content string) error {
	if err := writeFileAtomic(st.stylePath, content); err != nil {
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

// loadPresentationProfile reads the per-install profile; any absence or
// parse problem falls back to defaults (English, developer) so machines
// without a profile behave exactly as before (plan D2).
func loadPresentationProfile(path string) *presentationProfile {
	profile := &presentationProfile{
		Audience: "",
		Language: languageEnglish,
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return profile
	}

	inFrontMatter := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontMatter {
				break
			}
			inFrontMatter = true
			continue
		}
		if !inFrontMatter {
			continue
		}
		if value, ok := strings.CutPrefix(trimmed, "language:"); ok {
			profile.Language = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(trimmed, "audience:"); ok {
			profile.Audience = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(trimmed, "dev-mode:"); ok {
			profile.DevMode = strings.TrimSpace(value) == "true"
		}
	}
	if profile.Language == "" {
		profile.Language = languageEnglish
	}
	return profile
}

// profileBody returns the profile file's content after its frontmatter block;
// empty when the file is absent.
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

// writeFileAtomic writes tmp + rename — the saveAutoApplyRules discipline —
// creating the parent directory first (a fresh install has no
// ~/.claude/context/global yet).
func writeFileAtomic(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("writeFileAtomic: Failed to create %s: %w", filepath.Dir(path), err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writeFileAtomic: Failed to write %s: %w", tmp, err)
	}

	if err := fsx.ReplaceFile(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("writeFileAtomic: Failed to rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

// DefaultPresentationPath is the per-install profile location every consumer
// shares (the global-context hook, the nightly routine, this server).
func DefaultPresentationPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "context", "global", "presentation-profile.md")
}

// DefaultStylePath is the learned style profile's location — a sibling of the
// presentation profile, injected by the same global-context hook.
func DefaultStylePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "context", "global", "style-profile.md")
}
