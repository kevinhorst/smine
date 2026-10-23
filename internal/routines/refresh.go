package routines

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/fsx"
	"howett.net/plist"
)

// scheduleKey is wholesale operator-owned: the installed schedule always wins, presence and absence alike.
const scheduleKey = "StartCalendarInterval"

// RefreshResult reports one template's reconcile outcome.
type RefreshResult struct {
	Changed bool
	Name    string
}

// RefreshPlists marks a changed plist .pending-reload so the deferred reload never kills a live run.
func RefreshPlists(homeDir, repoDir string) ([]RefreshResult, error) {
	templates, err := filepath.Glob(filepath.Join(repoDir, "routines", "*", "*.plist.template"))
	if err != nil {
		return nil, fmt.Errorf("RefreshPlists: Failed to glob templates: %w", err)
	}

	var results []RefreshResult
	for _, templatePath := range templates {
		result, err := refreshPlist(homeDir, repoDir, templatePath)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func refreshPlist(homeDir, repoDir, templatePath string) (RefreshResult, error) {
	installedPath := strings.TrimSuffix(templatePath, ".template")
	result := RefreshResult{Name: filepath.Base(installedPath)}

	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to read %s: %w", templatePath, err)
	}
	rendered := strings.ReplaceAll(string(raw), "__REPO_DIR__", repoDir)
	rendered = strings.ReplaceAll(rendered, "__HOME__", homeDir)

	var content map[string]any
	if _, err := plist.Unmarshal([]byte(rendered), &content); err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to parse %s: %w", templatePath, err)
	}

	installedRaw, err := os.ReadFile(installedPath)
	if os.IsNotExist(err) {
		// First install: materialize the rendered template as-is.
		if err := writePlistAtomic(content, installedPath); err != nil {
			return result, err
		}
		result.Changed = true
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to read %s: %w", installedPath, err)
	}

	var installed map[string]any
	if _, err := plist.Unmarshal(installedRaw, &installed); err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to parse %s: %w", installedPath, err)
	}

	if schedule, present := installed[scheduleKey]; present {
		content[scheduleKey] = schedule
	} else {
		delete(content, scheduleKey)
	}

	merged := mergeEnv(content["EnvironmentVariables"], installed["EnvironmentVariables"])
	if len(merged) > 0 {
		content["EnvironmentVariables"] = merged
	} else {
		delete(content, "EnvironmentVariables")
	}

	// Compare through the encoder so formatting differences never count as a change.
	target, err := plist.MarshalIndent(content, plist.XMLFormat, "\t")
	if err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to encode %s: %w", installedPath, err)
	}
	current, err := plist.MarshalIndent(installed, plist.XMLFormat, "\t")
	if err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to encode %s: %w", installedPath, err)
	}
	if string(target) == string(current) {
		return result, nil
	}

	if err := writePlistAtomic(content, installedPath); err != nil {
		return result, err
	}

	markerPath := filepath.Join(filepath.Dir(installedPath), PendingReloadMarker)
	if err := os.WriteFile(markerPath, []byte("template-refresh\n"), 0o644); err != nil {
		return result, fmt.Errorf("refreshPlist: Failed to write %s: %w", markerPath, err)
	}
	result.Changed = true
	return result, nil
}

// mergeEnv lets a new template knob deploy on refresh while operator values and keys survive.
func mergeEnv(templateEnv, installedEnv any) map[string]any {
	merged := make(map[string]any)
	if templateDict, ok := templateEnv.(map[string]any); ok {
		for key, value := range templateDict {
			merged[key] = value
		}
	}
	if installedDict, ok := installedEnv.(map[string]any); ok {
		for key, value := range installedDict {
			merged[key] = value
		}
	}
	return merged
}

func writePlistAtomic(content map[string]any, plistPath string) error {
	encoded, err := plist.MarshalIndent(content, plist.XMLFormat, "\t")
	if err != nil {
		return fmt.Errorf("writePlistAtomic: Failed to encode %s: %w", plistPath, err)
	}

	if err := fsx.WriteFileAtomic(encoded, plistPath, 0o644); err != nil {
		return fmt.Errorf("writePlistAtomic: %w", err)
	}
	return nil
}
