// Package hookconfig decodes the rendered hooks config every Go hook reads.
package hookconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File is the rendered file's name under <claude home>/hooks.
const File = "config.json"

// Config is the whole file: one section per binary plus the shared context section and the sounds section.
type Config struct {
	Acdslhook          Acdslhook          `json:"acdslhook"`
	Context            Context            `json:"context"`
	Globalcontext      Globalcontext      `json:"globalcontext"`
	Planmodecontext    Planmodecontext    `json:"planmodecontext"`
	Readgate           Readgate           `json:"readgate"`
	Sessiontitle       Sessiontitle       `json:"sessiontitle"`
	Skillcontext       Skillcontext       `json:"skillcontext"`
	Skillhelp          Skillhelp          `json:"skillhelp"`
	Skillinvokegate    Skillinvokegate    `json:"skillinvokegate"`
	Sounds             Sounds             `json:"sounds"`
	Toolpolicy         Toolpolicy         `json:"toolpolicy"`
	Worktreecreate     Worktreecreate     `json:"worktreecreate"`
	Worktreewriteguard Worktreewriteguard `json:"worktreewriteguard"`
}

// DurationMs is a duration the file carries as integer milliseconds.
type DurationMs time.Duration

// Duration is the Go duration.
func (d DurationMs) Duration() time.Duration {
	return time.Duration(d)
}

func (d DurationMs) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(time.Duration(d).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("DurationMs.MarshalJSON: %w", err)
	}
	return data, nil
}

func (d *DurationMs) UnmarshalJSON(data []byte) error {
	var millis int64
	if err := json.Unmarshal(data, &millis); err != nil {
		return fmt.Errorf("DurationMs.UnmarshalJSON: Not an integer millisecond count: %s", string(data))
	}

	*d = DurationMs(time.Duration(millis) * time.Millisecond)
	return nil
}

// Load decodes the file strictly: an unknown key, a wrong type or an invalid toolpolicy listing is an error naming the file.
func Load(claudeHome string) (*Config, error) {
	path := Path(claudeHome)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Load: Failed to read %s (the install renders it): %w", path, err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	config := &Config{}
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("Load: Failed to decode %s: %w", path, err)
	}

	if err := config.Toolpolicy.index(); err != nil {
		return nil, fmt.Errorf("Load: %s: %w", path, err)
	}

	return config, nil
}

// Path is the file's location for a Claude home.
func Path(claudeHome string) string {
	return filepath.Join(claudeHome, "hooks", File)
}
