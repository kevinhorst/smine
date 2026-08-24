package server

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	audienceNonDeveloper = "non-developer"
	languageEnglish      = "en"
)

type presentationProfile struct {
	Audience string
	Language string
}

func (p *presentationProfile) isDeveloperAudience() bool {
	return p.Audience != audienceNonDeveloper
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
	}
	if profile.Language == "" {
		profile.Language = languageEnglish
	}
	return profile
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
