// Package presentation reads the per-install presentation profile.
package presentation

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	AudienceCasual  = "casual"
	LanguageEnglish = "en"
)

type Profile struct {
	Audience  string
	AutoMerge bool
	DevMode   bool
	Language  string
}

func (p *Profile) IsDeveloperAudience() bool {
	return p.Audience != AudienceCasual
}

func applyField(profile *Profile, line string) {
	if value, ok := strings.CutPrefix(line, "language:"); ok {
		profile.Language = strings.TrimSpace(value)
	}
	if value, ok := strings.CutPrefix(line, "audience:"); ok {
		profile.Audience = strings.TrimSpace(value)
	}
	if value, ok := strings.CutPrefix(line, "dev-mode:"); ok {
		profile.DevMode = strings.TrimSpace(value) == "true"
	}
	// auto-merge: false disables the orchestrate merge; absent/other keeps it on
	if value, ok := strings.CutPrefix(line, "auto-merge:"); ok {
		profile.AutoMerge = strings.TrimSpace(value) != "false"
	}
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "context", "global", "presentation-profile.md")
}

// Load falls back to defaults on any absence or parse problem; unknown keys are ignored.
func Load(path string) *Profile {
	profile := &Profile{
		Audience:  "",
		AutoMerge: true,
		Language:  LanguageEnglish,
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return profile
	}

	isInFrontMatter := false
	for line := range strings.SplitSeq(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if isInFrontMatter {
				break
			}
			isInFrontMatter = true
			continue
		}
		if !isInFrontMatter {
			continue
		}
		applyField(profile, trimmed)
	}
	if profile.Language == "" {
		profile.Language = LanguageEnglish
	}
	return profile
}
